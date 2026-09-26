CREATE SEQUENCE IF NOT EXISTS sales_order_number_seq START 1;

ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS number INTEGER;

UPDATE sales_orders
SET number = nextval('sales_order_number_seq')
WHERE number IS NULL;

ALTER TABLE sales_orders
    ALTER COLUMN number SET DEFAULT nextval('sales_order_number_seq'),
    ALTER COLUMN number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sales_orders_number ON sales_orders(number);
