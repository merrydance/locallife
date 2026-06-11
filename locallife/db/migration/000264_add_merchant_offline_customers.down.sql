ALTER TABLE table_reservations
    DROP COLUMN IF EXISTS created_by_user_id,
    DROP COLUMN IF EXISTS offline_customer_id;

DROP INDEX IF EXISTS table_reservations_offline_customer_id_idx;
DROP INDEX IF EXISTS merchant_offline_customers_merchant_idx;
DROP INDEX IF EXISTS merchant_offline_customers_merchant_phone_uidx;
DROP TABLE IF EXISTS merchant_offline_customers;
