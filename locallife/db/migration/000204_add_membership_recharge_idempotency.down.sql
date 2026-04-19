DROP INDEX IF EXISTS membership_transactions_recharge_idempotency_uq;

ALTER TABLE membership_transactions
DROP COLUMN IF EXISTS idempotency_key;