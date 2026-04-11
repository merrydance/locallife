ALTER TABLE orders
    DROP COLUMN IF EXISTS delivery_latitude_snapshot,
    DROP COLUMN IF EXISTS delivery_longitude_snapshot,
    DROP COLUMN IF EXISTS delivery_address_snapshot,
    DROP COLUMN IF EXISTS delivery_contact_phone_snapshot,
    DROP COLUMN IF EXISTS delivery_contact_name_snapshot;