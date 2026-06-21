CREATE TABLE IF NOT EXISTS baofu_withdrawal_account_guards (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_binding_id BIGINT NOT NULL REFERENCES baofu_account_bindings(id),
    provider_available_amount_fen BIGINT NOT NULL DEFAULT 0,
    provider_pending_amount_fen BIGINT NOT NULL DEFAULT 0,
    provider_ledger_amount_fen BIGINT NOT NULL DEFAULT 0,
    provider_frozen_amount_fen BIGINT NOT NULL DEFAULT 0,
    provider_balance_observed_at TIMESTAMPTZ,
    reserved_amount_fen BIGINT NOT NULL DEFAULT 0,
    consumed_withdraw_amount_fen BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_withdrawal_account_guards_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_withdrawal_account_guards_owner_scope_check CHECK (
        (owner_type = 'platform' AND owner_id = 0)
        OR
        (owner_type <> 'platform' AND owner_id > 0)
    ),
    CONSTRAINT baofu_withdrawal_account_guards_provider_available_check CHECK (provider_available_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_provider_pending_check CHECK (provider_pending_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_provider_ledger_check CHECK (provider_ledger_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_provider_frozen_check CHECK (provider_frozen_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_reserved_check CHECK (reserved_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_consumed_check CHECK (consumed_withdraw_amount_fen >= 0),
    CONSTRAINT baofu_withdrawal_account_guards_owner_binding_uidx UNIQUE (owner_type, owner_id, account_binding_id)
);

CREATE TABLE IF NOT EXISTS baofu_withdrawal_reservations (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_order_id BIGINT NOT NULL REFERENCES baofu_withdrawal_orders(id),
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_binding_id BIGINT NOT NULL REFERENCES baofu_account_bindings(id),
    amount_fen BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'reserved',
    release_reason TEXT,
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_withdrawal_reservations_order_uidx UNIQUE (withdrawal_order_id),
    CONSTRAINT baofu_withdrawal_reservations_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_withdrawal_reservations_owner_scope_check CHECK (
        (owner_type = 'platform' AND owner_id = 0)
        OR
        (owner_type <> 'platform' AND owner_id > 0)
    ),
    CONSTRAINT baofu_withdrawal_reservations_amount_check CHECK (amount_fen > 0),
    CONSTRAINT baofu_withdrawal_reservations_status_check CHECK (status IN ('reserved', 'consumed', 'released')),
    CONSTRAINT baofu_withdrawal_reservations_consumed_at_check CHECK (
        (status = 'consumed' AND consumed_at IS NOT NULL AND released_at IS NULL)
        OR
        (status <> 'consumed' AND consumed_at IS NULL)
    ),
    CONSTRAINT baofu_withdrawal_reservations_released_at_check CHECK (
        (status = 'released' AND released_at IS NOT NULL AND consumed_at IS NULL)
        OR
        (status <> 'released' AND released_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_baofu_withdrawal_reservations_owner_status
    ON baofu_withdrawal_reservations(owner_type, owner_id, account_binding_id, status, id ASC);

INSERT INTO baofu_withdrawal_reservations (
    withdrawal_order_id,
    owner_type,
    owner_id,
    account_binding_id,
    amount_fen,
    status,
    reserved_at,
    created_at,
    updated_at
)
SELECT
    id,
    owner_type,
    owner_id,
    account_binding_id,
    amount,
    'reserved',
    created_at,
    created_at,
    updated_at
FROM baofu_withdrawal_orders
WHERE status = 'processing'
ON CONFLICT (withdrawal_order_id) DO NOTHING;

INSERT INTO baofu_withdrawal_account_guards (
    owner_type,
    owner_id,
    account_binding_id,
    reserved_amount_fen,
    created_at,
    updated_at
)
SELECT
    owner_type,
    owner_id,
    account_binding_id,
    COALESCE(SUM(amount), 0)::bigint,
    min(created_at),
    max(updated_at)
FROM baofu_withdrawal_orders
WHERE status = 'processing'
GROUP BY owner_type, owner_id, account_binding_id
ON CONFLICT (owner_type, owner_id, account_binding_id) DO UPDATE SET
    reserved_amount_fen = EXCLUDED.reserved_amount_fen,
    updated_at = now();
