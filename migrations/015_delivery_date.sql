-- Data de entrega do pedido (só a data, sem hora). O planejamento de rotas agrupa por ela:
-- cada rota (delivery_plans) carrega a data das entregas que atende.
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS delivery_date DATE;
ALTER TABLE delivery_plans ADD COLUMN IF NOT EXISTS delivery_date DATE;

CREATE INDEX IF NOT EXISTS sales_orders_delivery_date_idx ON sales_orders (delivery_date);
