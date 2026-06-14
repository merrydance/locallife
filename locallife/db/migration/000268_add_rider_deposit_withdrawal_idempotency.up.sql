CREATE TABLE rider_deposit_withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    requested_amount BIGINT NOT NULL,
    accepted_amount BIGINT NOT NULL DEFAULT 0,
    refund_order_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rider_deposit_withdrawal_requests_user_key_uq
        UNIQUE (user_id, idempotency_key),
    CONSTRAINT rider_deposit_withdrawal_requests_idempotency_key_check
        CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 256),
    CONSTRAINT rider_deposit_withdrawal_requests_request_hash_check
        CHECK (btrim(request_hash) <> ''),
    CONSTRAINT rider_deposit_withdrawal_requests_requested_amount_check
        CHECK (requested_amount > 0),
    CONSTRAINT rider_deposit_withdrawal_requests_accepted_amount_check
        CHECK (accepted_amount >= 0 AND accepted_amount <= requested_amount),
    CONSTRAINT rider_deposit_withdrawal_requests_refund_order_ids_array_check
        CHECK (jsonb_typeof(refund_order_ids) = 'array')
);
