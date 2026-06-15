CREATE TABLE order_create_request_idempotency (
    id BIGSERIAL PRIMARY KEY,
    operation_scope TEXT NOT NULL,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    order_id BIGINT REFERENCES orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_create_request_idempotency_scope_key_uq
        UNIQUE (operation_scope, actor_user_id, idempotency_key),
    CONSTRAINT order_create_request_idempotency_key_check
        CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 256),
    CONSTRAINT order_create_request_idempotency_hash_check
        CHECK (btrim(request_hash) <> '')
);

CREATE INDEX order_create_request_idempotency_order_id_idx
    ON order_create_request_idempotency(order_id);
