package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"erp/pkg/audit"
	"erp/services/sales-service/internal/domain"
)

type Service struct {
	orders   domain.OrderRepository
	dir      domain.Directory
	cashflow domain.Cashflow
	catalog  domain.Catalog
	router   domain.Router
	plans    domain.PlanRepository
	identity domain.AdminVerifier
	audit    *audit.Logger
}

func New(orders domain.OrderRepository, dir domain.Directory, cashflow domain.Cashflow, catalog domain.Catalog, router domain.Router, plans domain.PlanRepository, identity domain.AdminVerifier, auditLogger *audit.Logger) *Service {
	return &Service{orders: orders, dir: dir, cashflow: cashflow, catalog: catalog, router: router, plans: plans, identity: identity, audit: auditLogger}
}

// validDeliveryDate accepts an empty date (PDV/counter sales) or a YYYY-MM-DD one.
func validDeliveryDate(d string) bool {
	if d == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

func (s *Service) CreateOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	if o.PaymentMethodID == "" || o.PaymentTermID == "" {
		return domain.Order{}, domain.ErrInvalid
	}
	// Address is optional: a counter/PDV sale has no delivery, so it never gets one — Picking/
	// Delivery/Route planning are separate, explicit actions nobody is forced to run on it.
	if o.PaymentStatus != domain.PaymentStatusPaid {
		o.PaymentStatus = domain.PaymentStatusPending
	}
		if err := s.dir.EnsureCustomer(ctx, o.CustomerID); err != nil {
		return domain.Order{}, err
	}
	if o.DeliveryDate = strings.TrimSpace(o.DeliveryDate); !validDeliveryDate(o.DeliveryDate) {
		return domain.Order{}, domain.ErrInvalid
	}
	if len(o.Items) == 0 {
		return domain.Order{}, domain.ErrInvalid
	}
	var subtotal float64
	for i := range o.Items {
		o.Items[i].Subtotal = o.Items[i].Quantity * o.Items[i].UnitPrice
		subtotal += o.Items[i].Subtotal
	}
	o.SubtotalAmount = subtotal
	o.TotalAmount = subtotal - o.DiscountAmount
	o.WarehouseID = ""
	o.Status = "APPROVED"
	if o.Address.Lat == nil || o.Address.Lng == nil {
		if lat, lng, ok := s.resolveGeo(ctx, o); ok {
			o.Address.Lat = &lat
			o.Address.Lng = &lng
		}
	}
	created, err := s.orders.Create(ctx, o, nil)
	if err != nil {
		return domain.Order{}, err
	}
	if err := s.cashflow.ScheduleSale(ctx, created.ID, created.CustomerID, created.PaymentMethodID, created.PaymentTermID, created.TotalAmount, created.CreatedAt); err != nil {
		return domain.Order{}, err
	}
	return created, nil
}

func (s *Service) UpdateOrder(ctx context.Context, id string, in domain.Order) (domain.Order, error) {
	cur, err := s.orders.Get(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if cur.Status != "APPROVED" || len(cur.Picks) > 0 {
		return domain.Order{}, domain.ErrInvalid
	}
	in.ID = id
	if in.PaymentMethodID == "" || in.PaymentTermID == "" || len(in.Items) == 0 {
		return domain.Order{}, domain.ErrInvalid
	}
		if err := s.dir.EnsureCustomer(ctx, in.CustomerID); err != nil {
		return domain.Order{}, err
	}
	if in.DeliveryDate = strings.TrimSpace(in.DeliveryDate); !validDeliveryDate(in.DeliveryDate) {
		return domain.Order{}, domain.ErrInvalid
	}
	var subtotal float64
	for i := range in.Items {
		in.Items[i].Subtotal = in.Items[i].Quantity * in.Items[i].UnitPrice
		subtotal += in.Items[i].Subtotal
	}
	in.SubtotalAmount = subtotal
	in.TotalAmount = subtotal - in.DiscountAmount
	in.Status = cur.Status
	if in.PaymentStatus != domain.PaymentStatusPending && in.PaymentStatus != domain.PaymentStatusPaid {
		in.PaymentStatus = cur.PaymentStatus
	}
	if in.Address.Lat == nil || in.Address.Lng == nil {
		if lat, lng, ok := s.resolveGeo(ctx, in); ok {
			in.Address.Lat = &lat
			in.Address.Lng = &lng
		}
	}
	updated, err := s.orders.Replace(ctx, in)
	if err != nil {
		return domain.Order{}, err
	}
	if err := s.cashflow.ScheduleSale(ctx, updated.ID, updated.CustomerID, updated.PaymentMethodID, updated.PaymentTermID, updated.TotalAmount, updated.CreatedAt); err != nil {
		return domain.Order{}, err
	}
	return updated, nil
}

func (s *Service) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	return s.orders.Get(ctx, id)
}

func (s *Service) ListOrders(ctx context.Context, from, to *time.Time) ([]domain.Order, error) {
	return s.orders.List(ctx, from, to)
}

func (s *Service) OnStockReserved(ctx context.Context, ev domain.OrderEvent) error {
	o, err := s.orders.Get(ctx, ev.OrderID)
	if err != nil {
		return err
	}
	if o.Status != "PENDING_RESERVATION" {
		return nil
	}
	return s.orders.UpdateStatus(ctx, ev.OrderID, "APPROVED")
}

func (s *Service) OnNFeIssued(ctx context.Context, ev domain.InvoiceEvent) error {
	if ev.SalesOrderID == "" {
		return nil
	}
	return s.orders.UpdateStatus(ctx, ev.SalesOrderID, "INVOICED")
}

func (s *Service) Cancel(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status == "INVOICED" || o.Status == "PICKING" || o.Status == "PICKED" {
		return domain.ErrInvalid
	}
	if o.Status != "APPROVED" && o.Status != "PENDING_RESERVATION" {
		return domain.ErrInvalid
	}
	if len(o.Picks) > 0 {
		return domain.ErrInvalid
	}
	if err := s.orders.UpdateStatus(ctx, id, "CANCELLED"); err != nil {
		return err
	}
	return s.cashflow.CancelSchedule(ctx, id)
}

// DeleteOrder permanently removes an order — unlike Cancel, which only flips status and
// keeps the record for history. Blocked once a fiscal document exists (INVOICED), since
// that's a real external document a delete here can't unwind. Deleting a picked/delivered
// order is allowed but does NOT reverse the stock that was already deducted when it was
// picked — only the sales record and its own pick history disappear (Picks cascade-delete
// with the order), the balance change in stock-service stands. Callers who need the stock
// back have to adjust it separately.
func (s *Service) DeleteOrder(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status == "INVOICED" {
		return domain.ErrInvalid
	}
	if err := s.orders.Delete(ctx, id); err != nil {
		return err
	}
	return s.cashflow.CancelSchedule(ctx, id)
}

func (s *Service) SetPaymentStatus(ctx context.Context, id, status string) error {
	if status != domain.PaymentStatusPending && status != domain.PaymentStatusPaid {
		return domain.ErrInvalid
	}
	if _, err := s.orders.Get(ctx, id); err != nil {
		return err
	}
	return s.orders.SetPaymentStatus(ctx, id, status)
}

func (s *Service) UndoPicking(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != "PICKING" && o.Status != "PICKED" {
		return domain.ErrInvalid
	}
	return s.orders.ResetPicking(ctx, id)
}

func (s *Service) Deliver(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != "PICKED" && o.Status != "UNDELIVERED" {
		return domain.ErrInvalid
	}
	if err := s.orders.UpdateStatus(ctx, id, "DELIVERED"); err != nil {
		return err
	}
	_ = s.orders.SetDeliveryNote(ctx, id, "")
	s.refreshPlanStatus(ctx, id)
	return nil
}

func (s *Service) FailDelivery(ctx context.Context, id, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return domain.ErrInvalid
	}
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != "PICKED" && o.Status != "UNDELIVERED" {
		return domain.ErrInvalid
	}
	if err := s.orders.UpdateStatus(ctx, id, "UNDELIVERED"); err != nil {
		return err
	}
	if err := s.orders.SetDeliveryNote(ctx, id, note); err != nil {
		return err
	}
	s.refreshPlanStatus(ctx, id)
	return nil
}

func (s *Service) UndoDeliver(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != "DELIVERED" {
		return domain.ErrInvalid
	}
	if err := s.orders.UpdateStatus(ctx, id, "PICKED"); err != nil {
		return err
	}
	_ = s.orders.SetDeliveryNote(ctx, id, "")
	s.refreshPlanStatus(ctx, id)
	return nil
}

func (s *Service) ScanPick(ctx context.Context, orderID, productID, warehouseID string, qty float64) (domain.Order, error) {
	o, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if o.Status != "APPROVED" && o.Status != "PICKING" {
		return domain.Order{}, domain.ErrInvalid
	}
	if qty <= 0 {
		qty = 1
	}
	if warehouseID == "" && o.Picking != nil {
		warehouseID = o.Picking.WarehouseID
	}
	if warehouseID == "" {
		warehouseID = o.WarehouseID
	}
	if warehouseID == "" {
		warehouseID, err = s.dir.DefaultWarehouse(ctx)
		if err != nil {
			return domain.Order{}, err
		}
	}
	if warehouseID == "" {
		return domain.Order{}, domain.ErrWarehouseRequired
	}
	picking, err := s.orders.EnsurePicking(ctx, orderID, warehouseID)
	if err != nil {
		return domain.Order{}, err
	}
	warehouseID = picking.WarehouseID
	recipes, err := s.kitRecipes(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	var ordered float64
	for _, it := range o.Items {
		if it.ProductID == productID {
			ordered += it.Quantity
		}
	}
	if ordered <= 0 && domain.PickRequirements(o.Items, nil, recipes)[productID] <= 0 {
		return domain.Order{}, domain.ErrInvalid
	}
	if _, isKit := recipes[productID]; isKit && ordered > 0 && !domain.KitComponentsPicked(o.Items, o.Picks, recipes, productID) {
		return domain.Order{}, domain.ErrInvalid
	}
	if _, err := s.orders.AddPick(ctx, orderID, domain.Pick{PickingID: picking.ID, ProductID: productID, WarehouseID: warehouseID, Quantity: qty}); err != nil {
		return domain.Order{}, err
	}
	if o.Status == "APPROVED" {
		if err := s.orders.UpdateStatus(ctx, orderID, "PICKING"); err != nil {
			return domain.Order{}, err
		}
	}
	return s.orders.Get(ctx, orderID)
}

func (s *Service) CompletePicking(ctx context.Context, orderID string, volumeCount int) (domain.Order, error) {
	if volumeCount < 1 {
		return domain.Order{}, domain.ErrInvalid
	}
	o, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if o.Status != "APPROVED" && o.Status != "PICKING" {
		return domain.Order{}, domain.ErrInvalid
	}
	recipes, err := s.kitRecipes(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	if !pickingDone(o, recipes) {
		return domain.Order{}, domain.ErrInvalid
	}
	return s.finishPicking(ctx, orderID, volumeCount)
}

// BypassPicking force-completes a picking that isn't fully separated yet — e.g. a missing
// item the customer agreed to receive later. Anyone can request it, but it only actually
// runs once approved: a MASTER caller approves it just by being MASTER; anyone else must
// supply a real admin's live credentials (approverEmail/approverPassword), checked against
// identity-service — not this browser's own session, since the approver is often a
// different person than whoever's at the counter. Every bypass is written to the audit log
// (see pkg/audit) regardless of who approved it, recording both the requester and approver.
func (s *Service) BypassPicking(ctx context.Context, orderID string, volumeCount int, reason, callerEmail, callerRole, approverEmail, approverPassword string) (domain.Order, error) {
	if volumeCount < 1 {
		return domain.Order{}, domain.ErrInvalid
	}
	approvedBy := callerEmail
	if callerRole != domain.MasterRoleCode {
		if approverEmail == "" || approverPassword == "" {
			return domain.Order{}, domain.ErrApprovalRequired
		}
		if s.identity == nil {
			return domain.Order{}, domain.ErrApprovalRequired
		}
		ok, err := s.identity.VerifyAdmin(ctx, approverEmail, approverPassword)
		if err != nil {
			return domain.Order{}, err
		}
		if !ok {
			return domain.Order{}, domain.ErrApprovalDenied
		}
		approvedBy = approverEmail
	}
	o, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if o.Status != "APPROVED" && o.Status != "PICKING" {
		return domain.Order{}, domain.ErrInvalid
	}
	// Deliberately no pickingDone check here — bypassing it is the entire point.
	out, err := s.finishPicking(ctx, orderID, volumeCount)
	if err != nil {
		return domain.Order{}, err
	}
	if s.audit != nil {
		body, _ := json.Marshal(map[string]string{"order_id": orderID, "requested_by": callerEmail, "approved_by": approvedBy, "reason": reason})
		go s.audit.Log(audit.Entry{
			Method: "BYPASS", Module: "sales", Path: "/sales-orders/" + orderID + "/picking/bypass",
			UserEmail: callerEmail, UserRole: callerRole, Body: string(body),
		})
	}
	return out, nil
}

func (s *Service) finishPicking(ctx context.Context, orderID string, volumeCount int) (domain.Order, error) {
	if err := s.orders.UpdateStatus(ctx, orderID, "PICKED"); err != nil {
		return domain.Order{}, err
	}
	if err := s.orders.MarkPickingDone(ctx, orderID, volumeCount); err != nil {
		return domain.Order{}, err
	}
	return s.orders.Get(ctx, orderID)
}

// kitRecipes returns nil (no kit expansion) when no catalog is wired.
func (s *Service) kitRecipes(ctx context.Context) (map[string][]domain.OrderItemComponent, error) {
	if s.catalog == nil {
		return nil, nil
	}
	return s.catalog.KitRecipes(ctx)
}

func pickingDone(o domain.Order, recipes map[string][]domain.OrderItemComponent) bool {
	if len(o.Items) == 0 {
		return false
	}
	for productID, qty := range domain.PickRequirements(o.Items, o.Picks, recipes) {
		if domain.PickedQty(o.Picks, productID) < qty-1e-9 {
			return false
		}
	}
	return true
}
