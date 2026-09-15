package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"erp-schema/model"
	"erp/pkg/httpserver"
	cashclient "erp/services/sales-service/internal/adapters/cashflow"
	configclient "erp/services/sales-service/internal/adapters/config"
	stockclient "erp/services/sales-service/internal/adapters/stock"
	"erp/services/sales-service/internal/application"
	"erp/services/sales-service/internal/domain"

	"github.com/gin-gonic/gin"
)

// orderIn shadows domain.Order's promoted Address field with the shared erp-schema type,
// so address validation (required fields) runs against the single cross-service schema
// before mapping down into sales-service's own OrderAddress.
type orderIn struct {
	domain.Order
	Address model.Address `json:"address"`
}

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.Engine, jwt gin.HandlerFunc) {
	api := r.Group("/", jwt)
	api.GET("/sales-orders", h.listOrders)
	api.POST("/sales-orders", h.createOrder)
	api.GET("/sales-orders/:id", h.getOrder)
	api.PUT("/sales-orders/:id", h.updateOrder)
	api.POST("/sales-orders/:id/cancel", h.cancel)
	api.DELETE("/sales-orders/:id", h.deleteOrder)
	api.PUT("/sales-orders/:id/payment-status", h.setPaymentStatus)
	api.POST("/sales-orders/:id/picking/scan", h.scanPick)
	api.POST("/sales-orders/:id/picking/complete", h.completePicking)
	api.POST("/sales-orders/:id/picking/bypass", h.bypassPicking)
	api.POST("/sales-orders/:id/picking/undo", h.undoPicking)
	api.POST("/sales-orders/:id/deliver", h.deliver)
	api.POST("/sales-orders/:id/fail", h.failDelivery)
	api.POST("/sales-orders/:id/undeliver", h.undoDeliver)
	api.GET("/delivery-candidates", h.listCandidates)
	api.GET("/delivery-plans", h.listPlans)
	api.POST("/delivery-plans", h.createPlans)
	api.GET("/delivery-plans/:id", h.getPlan)
	api.POST("/delivery-plans/:id/confirm", h.confirmPlan)
}

func (h *Handler) listOrders(c *gin.Context) {
	from, err := parseTimeQuery(c, "from")
	if err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	to, err := parseTimeQuery(c, "to")
	if err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.ListOrders(c.Request.Context(), from, to)
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseTimeQuery(c *gin.Context, key string) (*time.Time, error) {
	v := c.Query(key)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (h *Handler) createOrder(c *gin.Context) {
	var in orderIn
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	in.Order.Address = toDomainAddress(in.Address)
	out, err := h.svc.CreateOrder(h.withAuth(c), in.Order)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrWarehouseRequired) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) updateOrder(c *gin.Context) {
	var in orderIn
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	in.Order.Address = toDomainAddress(in.Address)
	out, err := h.svc.UpdateOrder(h.withAuth(c), c.Param("id"), in.Order)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) getOrder(c *gin.Context) {
	out, err := h.svc.GetOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) cancel(c *gin.Context) {
	if err := h.svc.Cancel(h.withAuth(c), c.Param("id")); err != nil {
		httpserver.Error(c, cancelStatus(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) deleteOrder(c *gin.Context) {
	if err := h.svc.DeleteOrder(h.withAuth(c), c.Param("id")); err != nil {
		httpserver.Error(c, cancelStatus(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// cancelStatus maps Cancel/DeleteOrder's errors: both already committed their own change
// (status flip or row delete) before ever calling cashflow — so a cashflow error here is a
// downstream dependency failure, not "not found", and must not be reported as 404.
func cancelStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (h *Handler) setPaymentStatus(c *gin.Context) {
	var in struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.SetPaymentStatus(c.Request.Context(), c.Param("id"), in.Status); err != nil {
		if errors.Is(err, domain.ErrInvalid) {
			httpserver.Error(c, http.StatusBadRequest, err)
			return
		}
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) withAuth(c *gin.Context) context.Context {
	tok := c.GetHeader("Authorization")
	ctx := context.WithValue(c.Request.Context(), configclient.AuthHeaderKey, tok)
	ctx = context.WithValue(ctx, cashclient.AuthHeaderKey, tok)
	return context.WithValue(ctx, stockclient.AuthHeaderKey, tok)
}

func (h *Handler) scanPick(c *gin.Context) {
	var in struct {
		ProductID   string  `json:"product_id"`
		WarehouseID string  `json:"warehouse_id"`
		Quantity    float64 `json:"quantity"`
	}
	_ = c.ShouldBindJSON(&in)
	out, err := h.svc.ScanPick(h.withAuth(c), c.Param("id"), in.ProductID, in.WarehouseID, in.Quantity)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrWarehouseRequired) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) completePicking(c *gin.Context) {
	var in struct {
		VolumeCount int `json:"volume_count"`
	}
	_ = c.ShouldBindJSON(&in)
	out, err := h.svc.CompletePicking(c.Request.Context(), c.Param("id"), in.VolumeCount)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) bypassPicking(c *gin.Context) {
	var in struct {
		VolumeCount      int    `json:"volume_count"`
		Reason           string `json:"reason"`
		ApproverEmail    string `json:"approver_email"`
		ApproverPassword string `json:"approver_password"`
	}
	_ = c.ShouldBindJSON(&in)
	out, err := h.svc.BypassPicking(c.Request.Context(), c.Param("id"), in.VolumeCount, in.Reason,
		c.GetString("email"), c.GetString("role"), in.ApproverEmail, in.ApproverPassword)
	if err != nil {
		status := http.StatusConflict
		switch {
		case errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, domain.ErrApprovalRequired):
			status = http.StatusForbidden
		case errors.Is(err, domain.ErrApprovalDenied):
			status = http.StatusUnauthorized
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) undoPicking(c *gin.Context) {
	if err := h.svc.UndoPicking(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusConflict
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) deliver(c *gin.Context) {
	if err := h.svc.Deliver(h.withAuth(c), c.Param("id")); err != nil {
		status := http.StatusConflict
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) failDelivery(c *gin.Context) {
	var in struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.FailDelivery(h.withAuth(c), c.Param("id"), in.Note); err != nil {
		status := http.StatusConflict
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) undoDeliver(c *gin.Context) {
	if err := h.svc.UndoDeliver(h.withAuth(c), c.Param("id")); err != nil {
		status := http.StatusConflict
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listCandidates(c *gin.Context) {
	out, err := h.svc.ListCandidates(h.withAuth(c))
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) listPlans(c *gin.Context) {
	out, err := h.svc.ListPlans(h.withAuth(c))
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) getPlan(c *gin.Context) {
	out, err := h.svc.GetPlan(h.withAuth(c), c.Param("id"))
	if err != nil {
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) confirmPlan(c *gin.Context) {
	var in domain.RouteOption
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&in); err != nil {
			httpserver.Error(c, http.StatusBadRequest, err)
			return
		}
	}
	out, err := h.svc.ConfirmPlan(h.withAuth(c), c.Param("id"), in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) createPlans(c *gin.Context) {
	var in struct {
		CenterID   string   `json:"center_id"`
		VehicleIDs []string `json:"vehicle_ids"`
		OrderIDs   []string `json:"order_ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.CreatePlans(h.withAuth(c), in.CenterID, in.VehicleIDs, in.OrderIDs)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrRouting) {
			status = http.StatusBadGateway
		} else if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrConflict) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
