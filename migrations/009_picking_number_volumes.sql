CREATE SEQUENCE IF NOT EXISTS sales_picking_number_seq START 1;

ALTER TABLE sales_pickings
    ADD COLUMN IF NOT EXISTS number INTEGER,
    ADD COLUMN IF NOT EXISTS volume_count INTEGER NOT NULL DEFAULT 0;

UPDATE sales_pickings
SET number = nextval('sales_picking_number_seq')
WHERE number IS NULL;

ALTER TABLE sales_pickings
    ALTER COLUMN number SET DEFAULT nextval('sales_picking_number_seq'),
    ALTER COLUMN number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sales_pickings_number ON sales_pickings(number);
