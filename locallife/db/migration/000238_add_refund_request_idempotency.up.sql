CREATE TABLE refund_request_idempotency (
    id BIGSERIAL PRIMARY KEY,
    operation_scope TEXT NOT NULL,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    refund_order_id BIGINT NOT NULL REFERENCES refund_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refund_request_idempotency_scope_key_uq
        UNIQUE (operation_scope, actor_user_id, idempotency_key)
);

CREATE INDEX refund_request_idempotency_refund_order_id_idx
    ON refund_request_idempotency(refund_order_id);
