DROP INDEX IF EXISTS idx_baofu_withdrawal_orders_idempotency_uq;

ALTER TABLE baofu_withdrawal_orders
    DROP COLUMN IF EXISTS idempotency_request_hash,
    DROP COLUMN IF EXISTS idempotency_key;
