-- A kit line item (product_id points at an assembly's own linked product, e.g. "Cesta 3
-- pessoas") is, by default, fulfilled from its recipe in stock-service — this table only
-- holds an override when the customer substitutes one of the recipe's products for another
-- (e.g. grapes instead of apples). Absent rows for an order item means "use the recipe as-is".
CREATE TABLE sales_order_item_components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES sales_order_items(id) ON DELETE CASCADE,
    product_id VARCHAR(50) NOT NULL,
    quantity NUMERIC(15,4) NOT NULL CHECK (quantity > 0)
);

CREATE INDEX idx_sales_order_item_components_item ON sales_order_item_components(order_item_id);
