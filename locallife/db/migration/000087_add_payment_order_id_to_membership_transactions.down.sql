DROP INDEX IF EXISTS membership_transactions_payment_order_id_idx;
ALTER TABLE membership_transactions DROP COLUMN IF EXISTS payment_order_id;
