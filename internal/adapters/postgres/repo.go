package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"erp/pkg/outbox"
	"erp/services/sales-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateOrder(ctx context.Context, o domain.Order, _ []byte) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback(ctx)
	var warehouseID any
	if o.WarehouseID != "" {
		warehouseID = o.WarehouseID
	}
	var addressID any
	if o.Address.ID != "" {
		addressID = o.Address.ID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO sales_orders (customer_id, warehouse_id, status, payment_status, subtotal_amount, discount_amount, total_amount, payment_method_id, payment_term_id,
			address_id, address_alias, address_zip, address_street, address_number, address_complement, address_district, address_city, address_state, address_lat, address_lng, delivery_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,NULLIF($21,'')::date)
		RETURNING id, created_at, updated_at
	`, o.CustomerID, warehouseID, o.Status, o.PaymentStatus, o.SubtotalAmount, o.DiscountAmount, o.TotalAmount, o.PaymentMethodID, o.PaymentTermID,
		addressID, o.Address.Alias, o.Address.Zip, o.Address.Street, o.Address.Number, o.Address.Complement, o.Address.District, o.Address.City, o.Address.State,
		nullFloat(o.Address.Lat), nullFloat(o.Address.Lng), o.DeliveryDate).
		Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return domain.Order{}, err
	}
	for i, item := range o.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO sales_order_items (sales_order_id, product_id, quantity, unit_price, subtotal)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, o.ID, item.ProductID, item.Quantity, item.UnitPrice, item.Subtotal).Scan(&o.Items[i].ID); err != nil {
			return domain.Order{}, err
		}
		if err := insertItemComponents(ctx, tx, o.Items[i].ID, item.Components); err != nil {
			return domain.Order{}, err
		}
	}
	if err := enqueueOrder(ctx, tx, "sales.order.created", o); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

func (r *Repo) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	var o domain.Order
	var lat, lng sql.NullFloat64
	err := r.pool.QueryRow(ctx, `
		SELECT o.id, o.customer_id,
			COALESCE(pk.warehouse_id::text, o.warehouse_id::text, ''),
			o.status, o.payment_status, COALESCE(o.delivery_note, ''), o.subtotal_amount, o.discount_amount, o.total_amount, o.payment_method_id, o.payment_term_id,
			COALESCE(o.address_id::text, ''), o.address_alias, o.address_zip, o.address_street, o.address_number, o.address_complement, o.address_district, o.address_city, o.address_state,
			o.address_lat, o.address_lng, COALESCE(to_char(o.delivery_date, 'YYYY-MM-DD'), ''),
			o.created_at, o.updated_at
		FROM sales_orders o
		LEFT JOIN sales_pickings pk ON pk.sales_order_id = o.id
		WHERE o.id=$1
	`, id).Scan(&o.ID, &o.CustomerID, &o.WarehouseID, &o.Status, &o.PaymentStatus, &o.DeliveryNote, &o.SubtotalAmount, &o.DiscountAmount, &o.TotalAmount, &o.PaymentMethodID, &o.PaymentTermID,
		&o.Address.ID, &o.Address.Alias, &o.Address.Zip, &o.Address.Street, &o.Address.Number, &o.Address.Complement, &o.Address.District, &o.Address.City, &o.Address.State,
		&lat, &lng, &o.DeliveryDate, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, err
	}
	applyGeo(&o.Address, lat, lng)
	items, err := r.items(ctx, o.ID)
	if err != nil {
		return domain.Order{}, err
	}
	o.Items = items
	picks, err := r.picks(ctx, o.ID)
	if err != nil {
		return domain.Order{}, err
	}
	o.Picks = picks
	o.Picking, err = r.picking(ctx, o.ID)
	return o, err
}

func (r *Repo) ListOrders(ctx context.Context, from, to *time.Time) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.customer_id,
			COALESCE(pk.warehouse_id::text, o.warehouse_id::text, ''),
			o.status, o.payment_status, COALESCE(o.delivery_note, ''), o.subtotal_amount, o.discount_amount, o.total_amount, o.payment_method_id, o.payment_term_id,
			COALESCE(o.address_id::text, ''), o.address_alias, o.address_zip, o.address_street, o.address_number, o.address_complement, o.address_district, o.address_city, o.address_state,
			o.address_lat, o.address_lng, COALESCE(to_char(o.delivery_date, 'YYYY-MM-DD'), ''),
			o.created_at, o.updated_at
		FROM sales_orders o
		LEFT JOIN sales_pickings pk ON pk.sales_order_id = o.id
		WHERE ($1::timestamptz IS NULL OR o.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR o.created_at <= $2)
		ORDER BY o.created_at DESC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.WarehouseID, &o.Status, &o.PaymentStatus, &o.DeliveryNote, &o.SubtotalAmount, &o.DiscountAmount, &o.TotalAmount, &o.PaymentMethodID, &o.PaymentTermID,
			&o.Address.ID, &o.Address.Alias, &o.Address.Zip, &o.Address.Street, &o.Address.Number, &o.Address.Complement, &o.Address.District, &o.Address.City, &o.Address.State,
			&lat, &lng, &o.DeliveryDate, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		applyGeo(&o.Address, lat, lng)
		items, err := r.items(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		o.Items = items
		picks, err := r.picks(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		o.Picks = picks
		o.Picking, err = r.picking(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if out == nil {
		out = []domain.Order{}
	}
	return out, rows.Err()
}

func (r *Repo) UpdateStatus(ctx context.Context, id, status string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE sales_orders SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if status == "CANCELLED" {
		if err := enqueueOrder(ctx, tx, "sales.order.cancelled", domain.Order{ID: id}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Delete used to just DELETE the row — unlike UpdateStatus's CANCELLED branch, it never told
// stock-service the order was gone, so any OPEN stock_reservations row for it (created back
// at ReserveOrder time) was orphaned forever, permanently inflating that product's "Reservado"
// figure on the Saldos screen. Enqueuing the same "sales.order.cancelled" event UpdateStatus
// already sends (empty Items, per ReserveOrder's "len(ev.Items)==0 -> ReleaseReservations"
// branch) fixes that — harmless to send even for an order whose reservations were already
// confirmed/released, since ReleaseReservations only ever touches rows still OPEN.
func (r *Repo) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM sales_orders WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err := enqueueOrder(ctx, tx, "sales.order.cancelled", domain.Order{ID: id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) SetPaymentStatus(ctx context.Context, id, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sales_orders SET payment_status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) SetDeliveryNote(ctx context.Context, id, note string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sales_orders SET delivery_note=$2, updated_at=NOW() WHERE id=$1`, id, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateAddressGeo(ctx context.Context, id string, lat, lng float64) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sales_orders SET address_lat=$2, address_lng=$3, updated_at=NOW() WHERE id=$1`, id, lat, lng)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) Replace(ctx context.Context, o domain.Order) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback(ctx)
	var addressID any
	if o.Address.ID != "" {
		addressID = o.Address.ID
	}
	tag, err := tx.Exec(ctx, `
		UPDATE sales_orders SET customer_id=$2, payment_method_id=$3, payment_term_id=$4,
			subtotal_amount=$5, discount_amount=$6, total_amount=$7,
			address_id=$8, address_alias=$9, address_zip=$10, address_street=$11, address_number=$12,
			address_complement=$13, address_district=$14, address_city=$15, address_state=$16,
			address_lat=$17, address_lng=$18, payment_status=$19, delivery_date=NULLIF($20,'')::date, updated_at=NOW()
		WHERE id=$1
	`, o.ID, o.CustomerID, o.PaymentMethodID, o.PaymentTermID, o.SubtotalAmount, o.DiscountAmount, o.TotalAmount,
		addressID, o.Address.Alias, o.Address.Zip, o.Address.Street, o.Address.Number, o.Address.Complement,
		o.Address.District, o.Address.City, o.Address.State, nullFloat(o.Address.Lat), nullFloat(o.Address.Lng), o.PaymentStatus, o.DeliveryDate)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sales_order_items WHERE sales_order_id=$1`, o.ID); err != nil {
		return domain.Order{}, err
	}
	for i, item := range o.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO sales_order_items (sales_order_id, product_id, quantity, unit_price, subtotal)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, o.ID, item.ProductID, item.Quantity, item.UnitPrice, item.Subtotal).Scan(&o.Items[i].ID); err != nil {
			return domain.Order{}, err
		}
		if err := insertItemComponents(ctx, tx, o.Items[i].ID, item.Components); err != nil {
			return domain.Order{}, err
		}
	}
	if err := enqueueOrder(ctx, tx, "sales.order.updated", o); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return r.GetOrder(ctx, o.ID)
}

func (r *Repo) EnsurePicking(ctx context.Context, orderID, warehouseID string) (domain.Picking, error) {
	o, err := r.GetOrder(ctx, orderID)
	if err != nil {
		return domain.Picking{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Picking{}, err
	}
	defer tx.Rollback(ctx)
	var exists int
	if err := tx.QueryRow(ctx, `SELECT COUNT(1) FROM sales_pickings WHERE sales_order_id=$1`, orderID).Scan(&exists); err != nil {
		return domain.Picking{}, err
	}
	var p domain.Picking
	err = tx.QueryRow(ctx, `
		INSERT INTO sales_pickings (sales_order_id, warehouse_id, status)
		VALUES ($1,$2,'OPEN')
		ON CONFLICT (sales_order_id) DO UPDATE SET warehouse_id = sales_pickings.warehouse_id
		RETURNING id, number, sales_order_id, warehouse_id::text, status, volume_count, created_at, completed_at
	`, orderID, warehouseID).Scan(&p.ID, &p.Number, &p.SalesOrderID, &p.WarehouseID, &p.Status, &p.VolumeCount, &p.CreatedAt, &p.CompletedAt)
	if err != nil {
		return domain.Picking{}, err
	}
	var n int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1) FROM sales_outbox_events
		WHERE aggregate_id=$1 AND event_type='sales.order.created'
	`, orderID).Scan(&n); err != nil {
		return domain.Picking{}, err
	}
	if n == 0 {
		o.WarehouseID = p.WarehouseID
		if err := enqueueOrder(ctx, tx, "sales.order.created", o); err != nil {
			return domain.Picking{}, err
		}
	} else if exists == 0 {
		o.WarehouseID = p.WarehouseID
		if err := enqueueOrder(ctx, tx, "sales.order.updated", o); err != nil {
			return domain.Picking{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Picking{}, err
	}
	return p, nil
}

func (r *Repo) MarkPickingDone(ctx context.Context, orderID string, volumeCount int) error {
	_, err := r.pool.Exec(ctx, `UPDATE sales_pickings SET status='DONE', volume_count=$2, completed_at=COALESCE(completed_at, NOW()) WHERE sales_order_id=$1`, orderID, volumeCount)
	return err
}

func (r *Repo) ResetPicking(ctx context.Context, orderID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM sales_order_picks WHERE sales_order_id=$1`, orderID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sales_pickings SET status='OPEN', completed_at=NULL, volume_count=0 WHERE sales_order_id=$1`, orderID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE sales_orders SET status='APPROVED', updated_at=NOW() WHERE id=$1`, orderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *Repo) AddPick(ctx context.Context, orderID string, p domain.Pick) (domain.Pick, error) {
	var pickingID any
	if p.PickingID != "" {
		pickingID = p.PickingID
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO sales_order_picks (sales_order_id, picking_id, product_id, warehouse_id, quantity)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at
	`, orderID, pickingID, p.ProductID, p.WarehouseID, p.Quantity).Scan(&p.ID, &p.CreatedAt)
	return p, err
}

func (r *Repo) ListPicks(ctx context.Context, orderID string) ([]domain.Pick, error) {
	return r.picks(ctx, orderID)
}

func (r *Repo) picking(ctx context.Context, orderID string) (*domain.Picking, error) {
	var p domain.Picking
	err := r.pool.QueryRow(ctx, `
		SELECT id, number, sales_order_id, warehouse_id::text, status, volume_count, created_at, completed_at
		FROM sales_pickings WHERE sales_order_id=$1
	`, orderID).Scan(&p.ID, &p.Number, &p.SalesOrderID, &p.WarehouseID, &p.Status, &p.VolumeCount, &p.CreatedAt, &p.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repo) picks(ctx context.Context, orderID string) ([]domain.Pick, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(picking_id::text, ''), product_id, warehouse_id, quantity, created_at
		FROM sales_order_picks WHERE sales_order_id=$1 ORDER BY created_at
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Pick
	for rows.Next() {
		var p domain.Pick
		if err := rows.Scan(&p.ID, &p.PickingID, &p.ProductID, &p.WarehouseID, &p.Quantity, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []domain.Pick{}
	}
	return out, rows.Err()
}

func insertItemComponents(ctx context.Context, tx pgx.Tx, itemID string, components []domain.OrderItemComponent) error {
	for _, c := range components {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sales_order_item_components (order_item_id, product_id, quantity)
			VALUES ($1,$2,$3)
		`, itemID, c.ProductID, c.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) items(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, quantity, unit_price, subtotal FROM sales_order_items WHERE sales_order_id=$1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrderItem
	for rows.Next() {
		var i domain.OrderItem
		if err := rows.Scan(&i.ID, &i.ProductID, &i.Quantity, &i.UnitPrice, &i.Subtotal); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []domain.OrderItem{}, nil
	}
	componentsByItem, err := r.itemComponents(ctx, orderID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Components = componentsByItem[out[i].ID]
	}
	return out, nil
}

// itemComponents loads every substitution row for an order's items in one query, keyed by
// order_item_id — avoids an N+1 query per line item.
func (r *Repo) itemComponents(ctx context.Context, orderID string) (map[string][]domain.OrderItemComponent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.order_item_id, c.product_id, c.quantity
		FROM sales_order_item_components c
		INNER JOIN sales_order_items i ON i.id = c.order_item_id
		WHERE i.sales_order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]domain.OrderItemComponent{}
	for rows.Next() {
		var itemID string
		var c domain.OrderItemComponent
		if err := rows.Scan(&itemID, &c.ProductID, &c.Quantity); err != nil {
			return nil, err
		}
		out[itemID] = append(out[itemID], c)
	}
	return out, rows.Err()
}

func (r *Repo) FetchPending(ctx context.Context, limit int) ([]outbox.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_type, payload FROM sales_outbox_events
		WHERE status='PENDING' ORDER BY created_at LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []outbox.Event
	for rows.Next() {
		var e outbox.Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repo) MarkProcessed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sales_outbox_events SET status='PROCESSED', processed_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *Repo) MarkFailed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sales_outbox_events SET status='FAILED' WHERE id=$1`, id)
	return err
}

type Orders struct{ *Repo }

func (o Orders) Create(ctx context.Context, order domain.Order, payload []byte) (domain.Order, error) {
	return o.CreateOrder(ctx, order, payload)
}

func (o Orders) Get(ctx context.Context, id string) (domain.Order, error) {
	return o.GetOrder(ctx, id)
}

func (o Orders) List(ctx context.Context, from, to *time.Time) ([]domain.Order, error) {
	return o.ListOrders(ctx, from, to)
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func applyGeo(a *domain.OrderAddress, lat, lng sql.NullFloat64) {
	if lat.Valid {
		v := lat.Float64
		a.Lat = &v
	}
	if lng.Valid {
		v := lng.Float64
		a.Lng = &v
	}
}

func enqueueOrder(ctx context.Context, tx pgx.Tx, eventType string, o domain.Order) error {
	payload, err := json.Marshal(domain.OrderEvent{
		OrderID: o.ID, WarehouseID: o.WarehouseID, CustomerID: o.CustomerID,
		Items: o.Items, TotalAmount: o.TotalAmount,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO sales_outbox_events (aggregate_type, aggregate_id, event_type, payload, status)
		VALUES ('SALES_ORDER', $1, $2, $3, 'PENDING')
	`, o.ID, eventType, payload)
	return err
}
