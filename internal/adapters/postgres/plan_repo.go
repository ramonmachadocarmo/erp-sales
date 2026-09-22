package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"erp/services/sales-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlanRepo struct{ pool *pgxpool.Pool }

func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo { return &PlanRepo{pool: pool} }

func (r *PlanRepo) CreateMany(ctx context.Context, plans []domain.DeliveryPlan) ([]domain.DeliveryPlan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	out := make([]domain.DeliveryPlan, 0, len(plans))
	for _, p := range plans {
		geom, err := json.Marshal(p.Geometry)
		if err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO delivery_plans (center_id, vehicle_id, status, distance_m, duration_s, weight_kg, volume_m3, occupancy_pct, geometry, delivery_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::date)
			RETURNING id, created_at
		`, p.CenterID, p.VehicleID, p.Status, p.DistanceM, p.DurationS, p.WeightKg, p.VolumeM3, p.OccupancyPct, geom, p.DeliveryDate).
			Scan(&p.ID, &p.CreatedAt); err != nil {
			return nil, err
		}
		for i, s := range p.Stops {
			if _, err := tx.Exec(ctx, `
				INSERT INTO delivery_plan_stops (plan_id, seq, sales_order_id, distance_m, duration_s)
				VALUES ($1,$2,$3,$4,$5)
			`, p.ID, s.Seq, s.SalesOrderID, s.DistanceM, s.DurationS); err != nil {
				return nil, err
			}
			p.Stops[i] = s
		}
		out = append(out, p)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PlanRepo) List(ctx context.Context) ([]domain.DeliveryPlan, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, center_id::text, vehicle_id::text, status, distance_m, duration_s, weight_kg, volume_m3, occupancy_pct, geometry, COALESCE(to_char(delivery_date, 'YYYY-MM-DD'), ''), created_at
		FROM delivery_plans WHERE status <> 'CANCELLED' ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DeliveryPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		stops, err := r.stops(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Stops = stops
		out = append(out, p)
	}
	if out == nil {
		out = []domain.DeliveryPlan{}
	}
	return out, rows.Err()
}

func (r *PlanRepo) Get(ctx context.Context, id string) (domain.DeliveryPlan, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, center_id::text, vehicle_id::text, status, distance_m, duration_s, weight_kg, volume_m3, occupancy_pct, geometry, COALESCE(to_char(delivery_date, 'YYYY-MM-DD'), ''), created_at
		FROM delivery_plans WHERE id=$1
	`, id)
	p, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DeliveryPlan{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	p.Stops, err = r.stops(ctx, p.ID)
	return p, err
}

func (r *PlanRepo) PlannedOrderIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.sales_order_id::text
		FROM delivery_plan_stops s
		JOIN delivery_plans p ON p.id = s.plan_id
		WHERE p.status IN ('PLANNED', 'CONFIRMED', 'IN_PROGRESS')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (r *PlanRepo) ReleaseOrders(ctx context.Context, orderIDs []string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	if _, err := r.pool.Exec(ctx, `
		DELETE FROM delivery_plan_stops s
		USING delivery_plans p
		WHERE s.plan_id = p.id AND p.status = 'PLANNED' AND s.sales_order_id = ANY($1::uuid[])
	`, orderIDs); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE delivery_plans p SET status='CANCELLED'
		WHERE p.status='PLANNED'
		  AND NOT EXISTS (SELECT 1 FROM delivery_plan_stops s WHERE s.plan_id = p.id)
	`)
	return err
}

func (r *PlanRepo) ByOrder(ctx context.Context, orderID string) (domain.DeliveryPlan, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT p.id::text FROM delivery_plan_stops s
		JOIN delivery_plans p ON p.id = s.plan_id
		WHERE s.sales_order_id=$1
		ORDER BY p.created_at DESC LIMIT 1
	`, orderID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DeliveryPlan{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	return r.Get(ctx, id)
}

func (r *PlanRepo) UpdateStatus(ctx context.Context, id, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE delivery_plans SET status=$2 WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PlanRepo) ReplaceRoute(ctx context.Context, p domain.DeliveryPlan) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	geom, err := json.Marshal(p.Geometry)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE delivery_plans SET distance_m=$2, duration_s=$3, geometry=$4 WHERE id=$1
	`, p.ID, p.DistanceM, p.DurationS, geom)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM delivery_plan_stops WHERE plan_id=$1`, p.ID); err != nil {
		return err
	}
	for _, s := range p.Stops {
		if _, err := tx.Exec(ctx, `
			INSERT INTO delivery_plan_stops (plan_id, seq, sales_order_id, distance_m, duration_s)
			VALUES ($1,$2,$3,$4,$5)
		`, p.ID, s.Seq, s.SalesOrderID, s.DistanceM, s.DurationS); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PlanRepo) stops(ctx context.Context, planID string) ([]domain.PlanStop, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT seq, sales_order_id::text, distance_m, duration_s
		FROM delivery_plan_stops WHERE plan_id=$1 ORDER BY seq
	`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PlanStop
	for rows.Next() {
		var s domain.PlanStop
		if err := rows.Scan(&s.Seq, &s.SalesOrderID, &s.DistanceM, &s.DurationS); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []domain.PlanStop{}
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPlan(row rowScanner) (domain.DeliveryPlan, error) {
	var p domain.DeliveryPlan
	var geom []byte
	err := row.Scan(&p.ID, &p.CenterID, &p.VehicleID, &p.Status, &p.DistanceM, &p.DurationS, &p.WeightKg, &p.VolumeM3, &p.OccupancyPct, &geom, &p.DeliveryDate, &p.CreatedAt)
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	if len(geom) > 0 {
		_ = json.Unmarshal(geom, &p.Geometry)
	}
	if p.Geometry == nil {
		p.Geometry = [][]float64{}
	}
	return p, nil
}
