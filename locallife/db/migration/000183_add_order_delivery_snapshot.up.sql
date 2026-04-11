ALTER TABLE orders
    ADD COLUMN delivery_contact_name_snapshot TEXT,
    ADD COLUMN delivery_contact_phone_snapshot TEXT,
    ADD COLUMN delivery_address_snapshot TEXT,
    ADD COLUMN delivery_longitude_snapshot NUMERIC,
    ADD COLUMN delivery_latitude_snapshot NUMERIC;