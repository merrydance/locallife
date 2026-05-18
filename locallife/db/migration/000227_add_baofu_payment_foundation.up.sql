ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_commands
    DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
    ADD CONSTRAINT external_payment_commands_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_facts
    DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
    ADD CONSTRAINT external_payment_facts_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE profit_sharing_orders
    ADD COLUMN IF NOT EXISTS payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payment_fee_rate_bps INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'wechat',
    ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'ecommerce',
    ADD COLUMN IF NOT EXISTS merchant_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS rider_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS operator_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS platform_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS sharing_detail_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_provider_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_provider_check CHECK (provider IN ('wechat', 'baofu'));

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_channel_check CHECK (channel IN ('ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_payment_fee_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_payment_fee_check CHECK (payment_fee >= 0 AND payment_fee_rate_bps >= 0);

CREATE TABLE IF NOT EXISTS baofu_account_bindings (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_type TEXT NOT NULL,
    contract_no TEXT,
    sharing_mer_id TEXT,
    login_no TEXT,
    open_state TEXT NOT NULL DEFAULT 'processing',
    wechat_sub_mch_id TEXT,
    bank_card_last4 TEXT,
    last_open_trans_serial_no TEXT,
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_account_bindings_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_account_bindings_account_type_check CHECK (account_type IN ('personal', 'business', 'platform')),
    CONSTRAINT baofu_account_bindings_open_state_check CHECK (open_state IN ('processing', 'active', 'failed', 'abnormal')),
    CONSTRAINT baofu_account_bindings_owner_uidx UNIQUE (owner_type, owner_id),
    CONSTRAINT baofu_account_bindings_contract_uidx UNIQUE (contract_no),
    CONSTRAINT baofu_account_bindings_sharing_uidx UNIQUE (sharing_mer_id),
    CONSTRAINT baofu_account_bindings_active_receiver_check CHECK (
        open_state <> 'active' OR length(trim(COALESCE(sharing_mer_id, contract_no, ''))) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_baofu_account_bindings_open_state
    ON baofu_account_bindings(open_state, updated_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS baofu_fee_ledger (
    id BIGSERIAL PRIMARY KEY,
    fee_type TEXT NOT NULL,
    payer_type TEXT NOT NULL,
    payer_id BIGINT,
    business_object_type TEXT NOT NULL,
    business_object_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    fee_rate_bps INTEGER,
    provider_bill_no TEXT,
    status TEXT NOT NULL DEFAULT 'recorded',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_fee_ledger_fee_type_check CHECK (fee_type IN ('payment_fee', 'account_open_verify_fee')),
    CONSTRAINT baofu_fee_ledger_payer_type_check CHECK (payer_type IN ('merchant', 'platform')),
    CONSTRAINT baofu_fee_ledger_status_check CHECK (status IN ('recorded', 'reconciled', 'adjusted')),
    CONSTRAINT baofu_fee_ledger_amount_check CHECK (amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_baofu_fee_ledger_business_uidx
    ON baofu_fee_ledger(fee_type, business_object_type, business_object_id);

CREATE TABLE IF NOT EXISTS baofu_withdrawal_orders (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_binding_id BIGINT NOT NULL REFERENCES baofu_account_bindings(id),
    out_request_no TEXT NOT NULL,
    baofu_withdraw_no TEXT,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'processing',
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_withdrawal_orders_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_withdrawal_orders_status_check CHECK (status IN ('processing', 'succeeded', 'failed', 'returned')),
    CONSTRAINT baofu_withdrawal_orders_amount_check CHECK (amount > 0),
    CONSTRAINT baofu_withdrawal_orders_out_request_no_uidx UNIQUE (out_request_no)
);

CREATE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_owner
    ON baofu_withdrawal_orders(owner_type, owner_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_status
    ON baofu_withdrawal_orders(status, created_at ASC, id ASC);
