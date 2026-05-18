ALTER TABLE profit_sharing_orders
    ADD COLUMN IF NOT EXISTS calculation_version TEXT NOT NULL DEFAULT 'legacy_v1',
    ADD COLUMN IF NOT EXISTS settlement_mode TEXT NOT NULL DEFAULT 'commission_share',
    ADD COLUMN IF NOT EXISTS provider_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_source TEXT NOT NULL DEFAULT 'estimated',
    ADD COLUMN IF NOT EXISTS merchant_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS merchant_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS merchant_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_gross_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS rider_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS commission_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_receiver_amount BIGINT NOT NULL DEFAULT 0;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_settlement_mode_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_settlement_mode_check
    CHECK (settlement_mode IN ('commission_share', 'fee_only_share', 'direct_no_share'));

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_fee_breakdown_amounts_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_fee_breakdown_amounts_check
    CHECK (
        provider_payment_fee >= 0
        AND provider_payment_fee_rate_bps >= 0
        AND provider_payment_fee_base_amount >= 0
        AND merchant_payment_fee >= 0
        AND merchant_payment_fee_rate_bps >= 0
        AND merchant_payment_fee_base_amount >= 0
        AND rider_gross_amount >= 0
        AND rider_payment_fee >= 0
        AND rider_payment_fee_rate_bps >= 0
        AND rider_payment_fee_base_amount >= 0
        AND commission_base_amount >= 0
        AND platform_receiver_amount >= 0
    );

UPDATE profit_sharing_orders
SET
    calculation_version = 'baofu_legacy_provider_fee_v1',
    settlement_mode = 'commission_share',
    provider_payment_fee = payment_fee,
    provider_payment_fee_rate_bps = payment_fee_rate_bps,
    provider_payment_fee_base_amount = total_amount,
    provider_payment_fee_source = 'estimated',
    merchant_payment_fee = 0,
    merchant_payment_fee_rate_bps = 60,
    merchant_payment_fee_base_amount = 0,
    rider_gross_amount = rider_amount,
    rider_payment_fee = 0,
    rider_payment_fee_rate_bps = 60,
    rider_payment_fee_base_amount = 0,
    commission_base_amount = total_amount,
    platform_receiver_amount = platform_commission
WHERE provider = 'baofu'
  AND calculation_version = 'legacy_v1';

CREATE TABLE IF NOT EXISTS order_payment_fee_ledgers (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    channel TEXT NOT NULL,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    profit_sharing_order_id BIGINT REFERENCES profit_sharing_orders(id),
    fee_type TEXT NOT NULL,
    payer_type TEXT NOT NULL,
    payer_id BIGINT,
    payee_type TEXT NOT NULL,
    base_amount BIGINT NOT NULL,
    rate_bps INTEGER NOT NULL,
    amount BIGINT NOT NULL,
    amount_source TEXT NOT NULL DEFAULT 'calculated',
    external_payment_fact_id BIGINT REFERENCES external_payment_facts(id),
    status TEXT NOT NULL DEFAULT 'recorded',
    calculation_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_payment_fee_ledgers_fee_type_check CHECK (fee_type IN (
        'provider_payment_fee',
        'merchant_payment_service_fee',
        'rider_payment_service_fee'
    )),
    CONSTRAINT order_payment_fee_ledgers_payer_type_check CHECK (payer_type IN ('platform', 'merchant', 'rider')),
    CONSTRAINT order_payment_fee_ledgers_payee_type_check CHECK (payee_type IN ('baofu', 'platform')),
    CONSTRAINT order_payment_fee_ledgers_status_check CHECK (status IN ('recorded', 'reconciled', 'adjusted')),
    CONSTRAINT order_payment_fee_ledgers_amount_check CHECK (base_amount >= 0 AND rate_bps >= 0 AND amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS order_payment_fee_ledgers_once_per_payer_uidx
    ON order_payment_fee_ledgers(payment_order_id, fee_type, payer_type, COALESCE(payer_id, 0));

CREATE INDEX IF NOT EXISTS order_payment_fee_ledgers_payer_idx
    ON order_payment_fee_ledgers(payer_type, payer_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS order_payment_fee_ledgers_payment_order_idx
    ON order_payment_fee_ledgers(payment_order_id, created_at DESC, id DESC);
