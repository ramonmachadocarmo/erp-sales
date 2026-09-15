ALTER TABLE sales_pickings ADD COLUMN completed_at TIMESTAMPTZ;

UPDATE sales_pickings s
SET completed_at = COALESCE(
    (SELECT MAX(p.created_at) FROM sales_order_picks p WHERE p.sales_order_id = s.sales_order_id),
    s.created_at
)
WHERE s.status = 'DONE' AND s.completed_at IS NULL;
