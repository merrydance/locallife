ALTER TABLE baofu_withdrawal_orders
    DROP CONSTRAINT IF EXISTS baofu_withdrawal_orders_idempotency_pair_check;

ALTER TABLE baofu_withdrawal_orders
    ADD CONSTRAINT baofu_withdrawal_orders_idempotency_pair_check
    CHECK (
        (idempotency_key IS NULL AND idempotency_request_hash IS NULL)
        OR
        (length(trim(idempotency_key)) > 0 AND length(trim(idempotency_request_hash)) > 0)
    );
