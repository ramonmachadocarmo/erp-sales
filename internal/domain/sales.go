package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalid           = errors.New("invalid status")
	ErrWarehouseRequired = errors.New("warehouse required")
	ErrRouting           = errors.New("osrm unavailable")
	ErrConflict          = errors.New("already planned")
	// ErrApprovalRequired: the caller isn't an admin, and no admin credentials were supplied
	// to approve a picking bypass. ErrApprovalDenied: admin credentials were supplied but
	// don't belong to an admin (or are wrong). See Service.BypassPicking.
	ErrApprovalRequired = errors.New("admin approval required")
	ErrApprovalDenied   = errors.New("invalid admin credentials")
)

// AdminVerifier checks a set of credentials against identity-service, live — used only to
// approve a picking bypass requested by a non-admin (see Service.BypassPicking). Deliberately
// not a token/session check: the approving admin may not be the one logged into this browser
// session at all.
type AdminVerifier interface {
	VerifyAdmin(ctx context.Context, email, password string) (bool, error)
}

// MasterRoleCode mirrors pkg/rbac.MasterRoleCode — sales-service has no reason to depend on
// the rbac package just for this one constant.
const MasterRoleCode = "MASTER"

// OrderItemComponent overrides one product a kit line consumes from stock, when the
// customer substitutes an item — see OrderItem.Components.
type OrderItemComponent struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
}

type OrderItem struct {
	ID        string  `json:"id,omitempty"`
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
	// Components is set only when this line is a kit (ProductID is some assembly's own
	// linked product) AND the customer substituted one or more of its recipe items —
	// the sale price never changes, only what stock-service actually consumes. Empty
	// means "fulfill from the kit's recipe as registered in Montagem" (stock-service's
	// concern, not this service's — sales-service never looks up the recipe itself).
	Components []OrderItemComponent `json:"components,omitempty"`
}

type Picking struct {
	ID           string     `json:"id,omitempty"`
	Number       int        `json:"number"`
	SalesOrderID string     `json:"sales_order_id"`
	WarehouseID  string     `json:"warehouse_id"`
	Status       string     `json:"status"`
	VolumeCount  int        `json:"volume_count"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type OrderAddress struct {
	ID         string   `json:"id,omitempty"`
	Alias      string   `json:"alias"`
	Zip        string   `json:"zip"`
	Street     string   `json:"street"`
	Number     string   `json:"number"`
	Complement string   `json:"complement"`
	District   string   `json:"district"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	Lat        *float64 `json:"lat,omitempty"`
	Lng        *float64 `json:"lng,omitempty"`
}

func (a OrderAddress) Empty() bool {
	return a.Alias == "" && a.Street == "" && a.City == "" && a.Zip == ""
}

const (
	PaymentStatusPending = "PENDING"
	PaymentStatusPaid    = "PAID"
)

type Order struct {
	ID              string       `json:"id"`
	CustomerID      string       `json:"customer_id"`
	WarehouseID     string       `json:"warehouse_id,omitempty"`
	PaymentMethodID string       `json:"payment_method_id"`
	PaymentTermID   string       `json:"payment_term_id"`
	Status          string       `json:"status"`
	PaymentStatus   string       `json:"payment_status"`
	SubtotalAmount  float64      `json:"subtotal_amount"`
	DiscountAmount  float64      `json:"discount_amount"`
	TotalAmount     float64      `json:"total_amount"`
	Address         OrderAddress `json:"address"`
	Items           []OrderItem  `json:"items"`
	Picking         *Picking     `json:"picking,omitempty"`
	Picks           []Pick       `json:"picks"`
	DeliveryNote    string       `json:"delivery_note,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type Pick struct {
	ID          string    `json:"id,omitempty"`
	PickingID   string    `json:"picking_id,omitempty"`
	ProductID   string    `json:"product_id"`
	WarehouseID string    `json:"warehouse_id"`
	Quantity    float64   `json:"quantity"`
	CreatedAt   time.Time `json:"created_at"`
}

type OrderEvent struct {
	OrderID     string      `json:"order_id"`
	WarehouseID string      `json:"warehouse_id"`
	CustomerID  string      `json:"customer_id"`
	Items       []OrderItem `json:"items"`
	TotalAmount float64     `json:"total_amount"`
}

type InvoiceEvent struct {
	InvoiceID    string `json:"invoice_id"`
	SalesOrderID string `json:"sales_order_id"`
	AccessKey    string `json:"access_key"`
}

type Directory interface {
	EnsureCustomer(ctx context.Context, id string) error
	DefaultWarehouse(ctx context.Context) (string, error)
	GetCenter(ctx context.Context, id string) (Center, error)
	GetVehicle(ctx context.Context, id string) (Vehicle, error)
	SearchAddress(ctx context.Context, a OrderAddress) (lat, lng float64, err error)
}

type Catalog interface {
	Product(ctx context.Context, id string) (ProductLoad, error)
}

type ProductLoad struct {
	ID       string
	WeightKg float64
	VolumeM3 float64
}

type Center struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	WarehouseID string  `json:"warehouse_id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
}

type Vehicle struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	CapacityM3 float64 `json:"capacity_m3"`
	CapacityKg float64 `json:"capacity_kg"`
	Active     bool    `json:"active"`
}

type Coord struct {
	Lat float64
	Lng float64
}

type Router interface {
	Table(ctx context.Context, coords []Coord) (durations, distances [][]float64, err error)
	Geometry(ctx context.Context, coords []Coord) ([][]float64, error)
}

type PlanStop struct {
	Seq          int          `json:"seq"`
	SalesOrderID string       `json:"sales_order_id"`
	DistanceM    float64      `json:"distance_m"`
	DurationS    float64      `json:"duration_s"`
	CustomerID   string       `json:"customer_id,omitempty"`
	Address      OrderAddress `json:"address,omitempty"`
	WeightKg     float64      `json:"weight_kg,omitempty"`
	VolumeM3     float64      `json:"volume_m3,omitempty"`
	Lat          float64      `json:"lat,omitempty"`
	Lng          float64      `json:"lng,omitempty"`
}

type DeliveryPlan struct {
	ID           string        `json:"id"`
	CenterID     string        `json:"center_id"`
	VehicleID    string        `json:"vehicle_id"`
	Status       string        `json:"status"`
	DistanceM    float64       `json:"distance_m"`
	DurationS    float64       `json:"duration_s"`
	WeightKg     float64       `json:"weight_kg"`
	VolumeM3     float64       `json:"volume_m3"`
	OccupancyPct float64       `json:"occupancy_pct"`
	Geometry     [][]float64   `json:"geometry"`
	Stops        []PlanStop    `json:"stops"`
	CenterName   string        `json:"center_name,omitempty"`
	CenterLat    float64       `json:"center_lat,omitempty"`
	CenterLng    float64       `json:"center_lng,omitempty"`
	VehicleName  string        `json:"vehicle_name,omitempty"`
	VehicleCode  string        `json:"vehicle_code,omitempty"`
	CapacityKg   float64       `json:"capacity_kg,omitempty"`
	CapacityM3   float64       `json:"capacity_m3,omitempty"`
	Options      []RouteOption `json:"options,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

type RouteOption struct {
	Label     string      `json:"label"`
	DistanceM float64     `json:"distance_m"`
	DurationS float64     `json:"duration_s"`
	Geometry  [][]float64 `json:"geometry"`
	Stops     []PlanStop  `json:"stops"`
	Selected  bool        `json:"selected,omitempty"`
}

type SkippedStop struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type PlanResult struct {
	Plans   []DeliveryPlan `json:"plans"`
	Skipped []SkippedStop  `json:"skipped"`
}

type DeliveryCandidate struct {
	Order
	WeightKg float64 `json:"weight_kg"`
	VolumeM3 float64 `json:"volume_m3"`
	HasGeo   bool    `json:"has_geo"`
	Planned  bool    `json:"planned"`
}

type PlanRepository interface {
	CreateMany(ctx context.Context, plans []DeliveryPlan) ([]DeliveryPlan, error)
	List(ctx context.Context) ([]DeliveryPlan, error)
	Get(ctx context.Context, id string) (DeliveryPlan, error)
	PlannedOrderIDs(ctx context.Context) (map[string]struct{}, error)
	ReleaseOrders(ctx context.Context, orderIDs []string) error
	ByOrder(ctx context.Context, orderID string) (DeliveryPlan, error)
	UpdateStatus(ctx context.Context, id, status string) error
	ReplaceRoute(ctx context.Context, p DeliveryPlan) error
}

type Cashflow interface {
	ScheduleSale(ctx context.Context, orderID, customerID, methodID, termID string, amount float64, at time.Time) error
	CancelSchedule(ctx context.Context, orderID string) error
}

type OrderRepository interface {
	Create(ctx context.Context, o Order, payload []byte) (Order, error)
	Get(ctx context.Context, id string) (Order, error)
	List(ctx context.Context, from, to *time.Time) ([]Order, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
	SetPaymentStatus(ctx context.Context, id, status string) error
	SetDeliveryNote(ctx context.Context, id, note string) error
	UpdateAddressGeo(ctx context.Context, id string, lat, lng float64) error
	Replace(ctx context.Context, o Order) (Order, error)
	EnsurePicking(ctx context.Context, orderID, warehouseID string) (Picking, error)
	MarkPickingDone(ctx context.Context, orderID string, volumeCount int) error
	AddPick(ctx context.Context, orderID string, p Pick) (Pick, error)
	ListPicks(ctx context.Context, orderID string) ([]Pick, error)
	ResetPicking(ctx context.Context, orderID string) error
}

func PickedQty(picks []Pick, productID string) float64 {
	var n float64
	for _, p := range picks {
		if p.ProductID == productID {
			n += p.Quantity
		}
	}
	return n
}
