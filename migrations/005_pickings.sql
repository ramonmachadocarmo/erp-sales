ALTER TABLE sales_orders ALTER COLUMN warehouse_id DROP NOT NULL;

CREATE TABLE sales_pickings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_order_id UUID NOT NULL UNIQUE REFERENCES sales_orders(id) ON DELETE CASCADE,
    warehouse_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sales_pickings_order ON sales_pickings(sales_order_id);

INSERT INTO sales_pickings (sales_order_id, warehouse_id, status)
SELECT id, warehouse_id,
    CASE WHEN status IN ('PICKING', 'PICKED', 'INVOICED') THEN 'DONE' ELSE 'OPEN' END
FROM sales_orders
WHERE warehouse_id IS NOT NULL
ON CONFLICT (sales_order_id) DO NOTHING;

ALTER TABLE sales_order_picks
    ADD COLUMN picking_id UUID REFERENCES sales_pickings(id) ON DELETE CASCADE;

UPDATE sales_order_picks p
SET picking_id = s.id
FROM sales_pickings s
WHERE s.sales_order_id = p.sales_order_id;
