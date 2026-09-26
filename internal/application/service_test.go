package application

import (
	"context"
	"testing"
	"time"

	"erp/services/sales-service/internal/domain"
)

type memOrders struct{ byID map[string]domain.Order }

func (m *memOrders) Create(_ context.Context, o domain.Order, _ []byte) (domain.Order, error) {
	o.ID = "so1"
	o.CreatedAt = time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	if m.byID == nil {
		m.byID = map[string]domain.Order{}
	}
	m.byID[o.ID] = o
	return o, nil
}

func (m *memOrders) Get(_ context.Context, id string) (domain.Order, error) {
	o, ok := m.byID[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}

func (m *memOrders) List(context.Context, *time.Time, *time.Time) ([]domain.Order, error) {
	return nil, nil
}

func (m *memOrders) UpdateStatus(_ context.Context, id, status string) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.Status = status
	m.byID[id] = o
	return nil
}

func (m *memOrders) Delete(_ context.Context, id string) error {
	if _, ok := m.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

func (m *memOrders) SetPaymentStatus(_ context.Context, id, status string) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.PaymentStatus = status
	m.byID[id] = o
	return nil
}

func (m *memOrders) SetDeliveryNote(_ context.Context, id, note string) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.DeliveryNote = note
	m.byID[id] = o
	return nil
}

func (m *memOrders) UpdateAddressGeo(_ context.Context, id string, lat, lng float64) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.Address.Lat = &lat
	o.Address.Lng = &lng
	m.byID[id] = o
	return nil
}

func (m *memOrders) Replace(_ context.Context, o domain.Order) (domain.Order, error) {
	cur, ok := m.byID[o.ID]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	o.CreatedAt = cur.CreatedAt
	o.Status = cur.Status
	m.byID[o.ID] = o
	return o, nil
}

func (m *memOrders) EnsurePicking(_ context.Context, orderID, warehouseID string) (domain.Picking, error) {
	o, ok := m.byID[orderID]
	if !ok {
		return domain.Picking{}, domain.ErrNotFound
	}
	if o.Picking != nil {
		return *o.Picking, nil
	}
	p := domain.Picking{ID: "pick1", Number: 1, SalesOrderID: orderID, WarehouseID: warehouseID, Status: "OPEN"}
	o.Picking = &p
	o.WarehouseID = warehouseID
	m.byID[orderID] = o
	return p, nil
}

func (m *memOrders) MarkPickingDone(_ context.Context, orderID string, volumeCount int) error {
	o, ok := m.byID[orderID]
	if !ok {
		return domain.ErrNotFound
	}
	if o.Picking != nil {
		p := *o.Picking
		p.Status = "DONE"
		p.VolumeCount = volumeCount
		o.Picking = &p
		m.byID[orderID] = o
	}
	return nil
}

func (m *memOrders) AddPick(_ context.Context, orderID string, p domain.Pick) (domain.Pick, error) {
	o, ok := m.byID[orderID]
	if !ok {
		return domain.Pick{}, domain.ErrNotFound
	}
	p.ID = "pk" + string(rune('0'+len(o.Picks)+1))
	o.Picks = append(o.Picks, p)
	m.byID[orderID] = o
	return p, nil
}

func (m *memOrders) ListPicks(_ context.Context, orderID string) ([]domain.Pick, error) {
	o, ok := m.byID[orderID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return o.Picks, nil
}

func (m *memOrders) ResetPicking(_ context.Context, orderID string) error {
	o, ok := m.byID[orderID]
	if !ok {
		return domain.ErrNotFound
	}
	o.Picks = nil
	o.Status = "APPROVED"
	if o.Picking != nil {
		p := *o.Picking
		p.Status = "OPEN"
		p.CompletedAt = nil
		p.VolumeCount = 0
		o.Picking = &p
	}
	m.byID[orderID] = o
	return nil
}

type okDir struct{ warehouse string }

func (okDir) EnsureCustomer(context.Context, string) error { return nil }

func (d okDir) DefaultWarehouse(context.Context) (string, error) { return d.warehouse, nil }

func (okDir) GetCenter(context.Context, string) (domain.Center, error) {
	return domain.Center{}, domain.ErrNotFound
}

func (okDir) GetVehicle(context.Context, string) (domain.Vehicle, error) {
	return domain.Vehicle{}, domain.ErrNotFound
}

func (okDir) SearchAddress(context.Context, domain.OrderAddress) (float64, float64, error) {
	return 0, 0, domain.ErrNotFound
}

type memPlans struct {
	byID map[string]domain.DeliveryPlan
}

func (m *memPlans) CreateMany(_ context.Context, plans []domain.DeliveryPlan) ([]domain.DeliveryPlan, error) {
	return plans, nil
}
func (m *memPlans) List(context.Context) ([]domain.DeliveryPlan, error) { return nil, nil }
func (m *memPlans) Get(_ context.Context, id string) (domain.DeliveryPlan, error) {
	p, ok := m.byID[id]
	if !ok {
		return domain.DeliveryPlan{}, domain.ErrNotFound
	}
	return p, nil
}
func (m *memPlans) PlannedOrderIDs(context.Context) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (m *memPlans) ReleaseOrders(context.Context, []string) error { return nil }
func (m *memPlans) ByOrder(context.Context, string) (domain.DeliveryPlan, error) {
	return domain.DeliveryPlan{}, domain.ErrNotFound
}
func (m *memPlans) UpdateStatus(_ context.Context, id, status string) error {
	p, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	p.Status = status
	m.byID[id] = p
	return nil
}
func (m *memPlans) ReplaceRoute(_ context.Context, p domain.DeliveryPlan) error {
	cur, ok := m.byID[p.ID]
	if !ok {
		return domain.ErrNotFound
	}
	cur.Stops = p.Stops
	cur.Geometry = p.Geometry
	cur.DistanceM = p.DistanceM
	cur.DurationS = p.DurationS
	m.byID[p.ID] = cur
	return nil
}

func testSvc(orders domain.OrderRepository, dir domain.Directory, cash domain.Cashflow) *Service {
	return New(orders, dir, cash, nil, nil, nil, nil, nil)
}

func TestScanPick(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "APPROVED", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 10}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	got, err := svc.ScanPick(context.Background(), "so1", "p", "", 1)
	if err != nil || got.Status != "PICKING" {
		t.Fatalf("%v %+v", err, got)
	}
	got, err = svc.ScanPick(context.Background(), "so1", "p", "w1", 1)
	if err != nil || got.Status != "PICKING" {
		t.Fatalf("%v %+v", err, got)
	}
	got, err = svc.CompletePicking(context.Background(), "so1", 2)
	if err != nil || got.Status != "PICKED" || got.Picking == nil || got.Picking.VolumeCount != 2 {
		t.Fatalf("%v %+v", err, got)
	}
}

type cashSpy struct {
	n         int
	amount    float64
	method    string
	term      string
	cancelled []string
}

func (c *cashSpy) ScheduleSale(_ context.Context, _, _, methodID, termID string, amount float64, _ time.Time) error {
	c.n++
	c.method = methodID
	c.term = termID
	c.amount = amount
	return nil
}

func (c *cashSpy) CancelSchedule(_ context.Context, orderID string) error {
	c.cancelled = append(c.cancelled, orderID)
	return nil
}

func TestCreateOrderRequiresPayment(t *testing.T) {
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	_, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", WarehouseID: "w1",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCreateOrderSchedulesCashflow(t *testing.T) {
	cash := &cashSpy{}
	svc := testSvc(&memOrders{}, okDir{}, cash)
	got, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", WarehouseID: "w1", PaymentMethodID: "m", PaymentTermID: "t",
		Address: domain.OrderAddress{Alias: "Casa", Street: "Rua", City: "SP"},
		Items:   []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 500}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalAmount != 1000 || got.Status != "APPROVED" || got.WarehouseID != "" {
		t.Fatalf("%+v", got)
	}
	if cash.n != 1 || cash.amount != 1000 || cash.method != "m" || cash.term != "t" {
		t.Fatalf("%+v", cash)
	}
}

func TestUpdateOrderApproved(t *testing.T) {
	orders := &memOrders{}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	got, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", PaymentMethodID: "m", PaymentTermID: "t",
		Address: domain.OrderAddress{Alias: "Casa", Street: "Rua", City: "SP"},
		Items:   []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 500}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err = svc.UpdateOrder(context.Background(), got.ID, domain.Order{
		CustomerID: "c1", PaymentMethodID: "m2", PaymentTermID: "t",
		Address: domain.OrderAddress{Alias: "Casa", Street: "Rua", City: "SP"},
		Items:   []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 100}},
	})
	if err != nil || got.TotalAmount != 100 || got.PaymentMethodID != "m2" {
		t.Fatalf("%v %+v", err, got)
	}
	orders.byID[got.ID] = domain.Order{ID: got.ID, Status: "PICKED", Items: got.Items}
	if _, err := svc.UpdateOrder(context.Background(), got.ID, got); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCreateOrderWithoutWarehouse(t *testing.T) {
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	got, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", WarehouseID: "w1", PaymentMethodID: "m", PaymentTermID: "t",
		Address: domain.OrderAddress{Alias: "Casa", City: "SP"},
		Items:   []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != nil || got.WarehouseID != "" || got.Status != "APPROVED" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestScanPickRequiresWarehouse(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {ID: "so1", Status: "APPROVED", Items: []domain.OrderItem{{ProductID: "p", Quantity: 1}}},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if _, err := svc.ScanPick(context.Background(), "so1", "p", "", 1); err != domain.ErrWarehouseRequired {
		t.Fatalf("%v", err)
	}
}

func TestScanPickAllowsOverQty(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "APPROVED", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	got, err := svc.ScanPick(context.Background(), "so1", "p", "w1", 2)
	if err != nil || got.Status != "PICKING" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestScanPickWrongStatus(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {ID: "so1", Status: "PENDING_RESERVATION", WarehouseID: "w1", Items: []domain.OrderItem{{ProductID: "p", Quantity: 1}}},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if _, err := svc.ScanPick(context.Background(), "so1", "p", "w1", 1); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCompletePicking(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "PICKING", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 1}},
			Picks: []domain.Pick{{ProductID: "p", Quantity: 1}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	got, err := svc.CompletePicking(context.Background(), "so1", 1)
	if err != nil || got.Status != "PICKED" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestCompletePickingIncomplete(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "PICKING", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 2}},
			Picks: []domain.Pick{{ProductID: "p", Quantity: 1}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if _, err := svc.CompletePicking(context.Background(), "so1", 1); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCompletePickingRequiresVolumes(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "PICKING", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 1}},
			Picks: []domain.Pick{{ProductID: "p", Quantity: 1}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if _, err := svc.CompletePicking(context.Background(), "so1", 0); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

type fakeAdminVerifier struct {
	ok  bool
	err error
}

func (f *fakeAdminVerifier) VerifyAdmin(context.Context, string, string) (bool, error) {
	return f.ok, f.err
}

func incompletePickingOrder() *memOrders {
	return &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "PICKING", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 2}},
			Picks: []domain.Pick{{ProductID: "p", Quantity: 1}},
		},
	}}
}

func TestBypassPickingMasterCallerNeedsNoApproval(t *testing.T) {
	orders := incompletePickingOrder()
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, nil, nil, nil)
	got, err := svc.BypassPicking(context.Background(), "so1", 1, "cliente aceitou falta", "boss@erp.com", domain.MasterRoleCode, "", "")
	if err != nil || got.Status != "PICKED" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestBypassPickingNonMasterWithoutApproverRequiresApproval(t *testing.T) {
	orders := incompletePickingOrder()
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, nil, &fakeAdminVerifier{ok: true}, nil)
	if _, err := svc.BypassPicking(context.Background(), "so1", 1, "", "clerk@erp.com", "SEM_ACESSO", "", ""); err != domain.ErrApprovalRequired {
		t.Fatalf("%v", err)
	}
}

func TestBypassPickingNonMasterWrongApproverCredentialsDenied(t *testing.T) {
	orders := incompletePickingOrder()
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, nil, &fakeAdminVerifier{ok: false}, nil)
	_, err := svc.BypassPicking(context.Background(), "so1", 1, "", "clerk@erp.com", "SEM_ACESSO", "boss@erp.com", "wrong")
	if err != domain.ErrApprovalDenied {
		t.Fatalf("%v", err)
	}
}

func TestBypassPickingNonMasterWithValidApproverSucceeds(t *testing.T) {
	orders := incompletePickingOrder()
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, nil, &fakeAdminVerifier{ok: true}, nil)
	got, err := svc.BypassPicking(context.Background(), "so1", 1, "", "clerk@erp.com", "SEM_ACESSO", "boss@erp.com", "right")
	if err != nil || got.Status != "PICKED" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestBypassPickingRequiresVolumes(t *testing.T) {
	orders := incompletePickingOrder()
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, nil, nil, nil)
	if _, err := svc.BypassPicking(context.Background(), "so1", 0, "", "boss@erp.com", domain.MasterRoleCode, "", ""); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

// Address is optional: a counter/PDV sale is picked up on the spot and never gets one.
func TestCreateOrderAllowsNoAddress(t *testing.T) {
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	o, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if o.PaymentStatus != domain.PaymentStatusPending {
		t.Fatalf("expected default payment status PENDING, got %q", o.PaymentStatus)
	}
}

func TestCreateOrderWithImmediatePayment(t *testing.T) {
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	o, err := svc.CreateOrder(context.Background(), domain.Order{
		CustomerID: "c1", PaymentMethodID: "m", PaymentTermID: "t", PaymentStatus: domain.PaymentStatusPaid,
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if o.PaymentStatus != domain.PaymentStatusPaid {
		t.Fatalf("expected payment status PAID, got %q", o.PaymentStatus)
	}
}

func TestUndoPicking(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {
			ID: "so1", Status: "PICKED", WarehouseID: "w1",
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 1}},
			Picks: []domain.Pick{{ProductID: "p", Quantity: 1}},
		},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if err := svc.UndoPicking(context.Background(), "so1"); err != nil {
		t.Fatal(err)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "APPROVED" || len(got.Picks) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestDeliver(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {ID: "so1", Status: "PICKED"},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if err := svc.Deliver(context.Background(), "so1"); err != nil {
		t.Fatal(err)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "DELIVERED" {
		t.Fatalf("%+v", got)
	}
}

func TestFailAndUndoDeliver(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{
		"so1": {ID: "so1", Status: "PICKED"},
	}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if err := svc.FailDelivery(context.Background(), "so1", "cliente ausente"); err != nil {
		t.Fatal(err)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "UNDELIVERED" || got.DeliveryNote != "cliente ausente" {
		t.Fatalf("%+v", got)
	}
	if err := svc.Deliver(context.Background(), "so1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.UndoDeliver(context.Background(), "so1"); err != nil {
		t.Fatal(err)
	}
	got, _ = orders.Get(context.Background(), "so1")
	if got.Status != "PICKED" {
		t.Fatalf("%+v", got)
	}
	if err := svc.FailDelivery(context.Background(), "so1", "  "); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestConfirmPlan(t *testing.T) {
	plans := &memPlans{byID: map[string]domain.DeliveryPlan{
		"p1": {ID: "p1", Status: "PLANNED", Stops: []domain.PlanStop{{Seq: 1, SalesOrderID: "so1"}}},
	}}
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "PICKED"}}}
	svc := New(orders, okDir{}, &cashSpy{}, nil, nil, plans, nil, nil)
	got, err := svc.ConfirmPlan(context.Background(), "p1", domain.RouteOption{
		DistanceM: 1500, DurationS: 120,
		Stops: []domain.PlanStop{{Seq: 1, SalesOrderID: "so1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "CONFIRMED" || got.DistanceM != 1500 {
		t.Fatalf("%+v", got)
	}
	plans.byID["p1"] = domain.DeliveryPlan{ID: "p1", Status: "IN_PROGRESS"}
	if _, err := svc.ConfirmPlan(context.Background(), "p1", domain.RouteOption{}); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCancelClearsCashflowSchedule(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "APPROVED"}}}
	cash := &cashSpy{}
	svc := testSvc(orders, okDir{}, cash)
	if err := svc.Cancel(context.Background(), "so1"); err != nil {
		t.Fatalf("%v", err)
	}
	if len(cash.cancelled) != 1 || cash.cancelled[0] != "so1" {
		t.Fatalf("expected cashflow schedule cancelled for so1, got: %+v", cash.cancelled)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "CANCELLED" {
		t.Fatalf("%+v", got)
	}
}

func TestDeleteOrderClearsCashflowSchedule(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "APPROVED"}}}
	cash := &cashSpy{}
	svc := testSvc(orders, okDir{}, cash)
	if err := svc.DeleteOrder(context.Background(), "so1"); err != nil {
		t.Fatalf("%v", err)
	}
	if len(cash.cancelled) != 1 || cash.cancelled[0] != "so1" {
		t.Fatalf("expected cashflow schedule cancelled for so1, got: %+v", cash.cancelled)
	}
	if _, err := orders.Get(context.Background(), "so1"); err != domain.ErrNotFound {
		t.Fatalf("expected order deleted, got: %v", err)
	}
}

func TestDeleteOrderInvoicedRefusedBeforeTouchingCashflow(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "INVOICED"}}}
	cash := &cashSpy{}
	svc := testSvc(orders, okDir{}, cash)
	if err := svc.DeleteOrder(context.Background(), "so1"); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
	if len(cash.cancelled) != 0 {
		t.Fatalf("cashflow should not be touched when delete is refused, got: %+v", cash.cancelled)
	}
}

func TestOnNFeIssuedDoesNotResurrectCancelledOrder(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "CANCELLED"}}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if err := svc.OnNFeIssued(context.Background(), domain.InvoiceEvent{SalesOrderID: "so1", InvoiceID: "inv1"}); err != nil {
		t.Fatalf("%v", err)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "CANCELLED" {
		t.Fatalf("expected order to stay CANCELLED, got: %+v", got)
	}
}

func TestOnNFeIssuedIgnoresDeletedOrder(t *testing.T) {
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	if err := svc.OnNFeIssued(context.Background(), domain.InvoiceEvent{SalesOrderID: "gone", InvoiceID: "inv1"}); err != nil {
		t.Fatalf("expected nil (idempotent no-op) for a deleted order, got: %v", err)
	}
}

func TestOnNFeIssuedSetsInvoicedForActiveOrder(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.Order{"so1": {ID: "so1", Status: "APPROVED"}}}
	svc := testSvc(orders, okDir{}, &cashSpy{})
	if err := svc.OnNFeIssued(context.Background(), domain.InvoiceEvent{SalesOrderID: "so1", InvoiceID: "inv1"}); err != nil {
		t.Fatalf("%v", err)
	}
	got, _ := orders.Get(context.Background(), "so1")
	if got.Status != "INVOICED" {
		t.Fatalf("expected order to become INVOICED, got: %+v", got)
	}
}

func TestCreateOrderDeliveryDate(t *testing.T) {
	newOrder := func(date string) domain.Order {
		return domain.Order{
			CustomerID: "c1", PaymentMethodID: "m", PaymentTermID: "t", DeliveryDate: date,
			Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
		}
	}
	svc := testSvc(&memOrders{}, okDir{}, &cashSpy{})
	o, err := svc.CreateOrder(context.Background(), newOrder("2026-09-25"))
	if err != nil || o.DeliveryDate != "2026-09-25" {
		t.Fatalf("valid date: %v %+v", err, o.DeliveryDate)
	}
	if _, err := svc.CreateOrder(context.Background(), newOrder("")); err != nil {
		t.Fatalf("empty date (PDV) must be accepted: %v", err)
	}
	for _, bad := range []string{"25/09/2026", "2026-13-01", "amanhã"} {
		if _, err := svc.CreateOrder(context.Background(), newOrder(bad)); err != domain.ErrInvalid {
			t.Fatalf("%q must be rejected, got %v", bad, err)
		}
	}
}
