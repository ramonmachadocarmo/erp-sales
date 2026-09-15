ALTER TABLE sales_orders
    ADD COLUMN payment_method_id VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN payment_term_id VARCHAR(50) NOT NULL DEFAULT '';
