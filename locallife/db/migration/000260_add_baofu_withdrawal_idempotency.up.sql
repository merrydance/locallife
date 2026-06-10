ALTER TABLE baofu_withdrawal_orders
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS idempotency_request_hash TEXT;

ALTER TABLE baofu_withdrawal_orders
    DROP CONSTRAINT IF EXISTS baofu_withdrawal_orders_idempotency_pair_check;

ALTER TABLE baofu_withdrawal_orders
    ADD CONSTRAINT baofu_withdrawal_orders_idempotency_pair_check
    CHECK (
        (idempotency_key IS NULL AND idempotency_request_hash IS NULL)
        OR
        (
            idempotency_key IS NOT NULL
            AND idempotency_request_hash IS NOT NULL
            AND length(trim(idempotency_key)) > 0
            AND length(trim(idempotency_request_hash)) > 0
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_idempotency_uq
    ON baofu_withdrawal_orders(owner_type, owner_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
