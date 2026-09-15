CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cpf_cnpj VARCHAR(18) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    trade_name VARCHAR(150),
    email VARCHAR(100) NOT NULL,
    phone VARCHAR(20),
    credit_limit NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sales_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL,
    status VARCHAR(30) NOT NULL,
    subtotal_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    total_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sales_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_order_id UUID NOT NULL REFERENCES sales_orders(id) ON DELETE CASCADE,
    product_id VARCHAR(50) NOT NULL,
    quantity NUMERIC(15,4) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(15,4) NOT NULL CHECK (unit_price >= 0),
    subtotal NUMERIC(15,2) NOT NULL
);

CREATE TABLE sales_outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(50) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_customers_cpf_cnpj ON customers(cpf_cnpj);
CREATE INDEX idx_sales_orders_customer ON sales_orders(customer_id);
CREATE INDEX idx_sales_orders_status ON sales_orders(status, created_at DESC);
CREATE INDEX idx_sales_order_items_order ON sales_order_items(sales_order_id);
CREATE INDEX idx_sales_outbox_pending ON sales_outbox_events(status, created_at) WHERE status = 'PENDING';
