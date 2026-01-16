DROP INDEX IF EXISTS rider_deposits_payment_order_id_idx;
ALTER TABLE rider_deposits DROP COLUMN IF EXISTS payment_order_id;
