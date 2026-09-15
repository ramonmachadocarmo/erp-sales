ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS address_lat DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS address_lng DOUBLE PRECISION;

CREATE TABLE IF NOT EXISTS delivery_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    center_id UUID NOT NULL,
    vehicle_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PLANNED',
    distance_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_s DOUBLE PRECISION NOT NULL DEFAULT 0,
    weight_kg DOUBLE PRECISION NOT NULL DEFAULT 0,
    volume_m3 DOUBLE PRECISION NOT NULL DEFAULT 0,
    occupancy_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    geometry JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS delivery_plan_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES delivery_plans(id) ON DELETE CASCADE,
    seq INT NOT NULL,
    sales_order_id UUID NOT NULL UNIQUE,
    distance_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_s DOUBLE PRECISION NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS delivery_plans_status_idx ON delivery_plans (status);
CREATE INDEX IF NOT EXISTS delivery_plan_stops_plan_idx ON delivery_plan_stops (plan_id, seq);
