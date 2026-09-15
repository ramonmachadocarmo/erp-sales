ALTER TABLE sales_orders
    ADD COLUMN address_id UUID,
    ADD COLUMN address_alias VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN address_zip VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN address_street VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN address_number VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN address_complement VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN address_district VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN address_city VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN address_state VARCHAR(2) NOT NULL DEFAULT '';
