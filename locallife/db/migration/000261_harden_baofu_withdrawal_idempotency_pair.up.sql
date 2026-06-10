UPDATE baofu_withdrawal_orders
SET
    idempotency_key = NULL,
    idempotency_request_hash = NULL,
    updated_at = now()
WHERE idempotency_key IS NOT NULL
  AND length(trim(idempotency_key)) = 0;

UPDATE baofu_withdrawal_orders
SET
    idempotency_request_hash = NULL,
    updated_at = now()
WHERE idempotency_request_hash IS NOT NULL
  AND length(trim(idempotency_request_hash)) = 0;

UPDATE baofu_withdrawal_orders
SET
    idempotency_request_hash = 'legacy_missing_hash:' || id::text,
    updated_at = now()
WHERE idempotency_key IS NOT NULL
  AND idempotency_request_hash IS NULL;

UPDATE baofu_withdrawal_orders
SET
    idempotency_key = NULL,
    idempotency_request_hash = NULL,
    updated_at = now()
WHERE idempotency_key IS NULL
  AND idempotency_request_hash IS NOT NULL;

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
