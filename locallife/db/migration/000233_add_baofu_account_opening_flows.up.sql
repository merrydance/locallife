ALTER TABLE payment_orders
DROP CONSTRAINT IF EXISTS payment_orders_business_type_check;

ALTER TABLE payment_orders
ADD CONSTRAINT payment_orders_business_type_check
CHECK (business_type IN (
  'order',
  'reservation',
  'reservation_addon',
  'membership_recharge',
  'rider_deposit',
  'claim_recovery',
  'baofu_account_verify_fee',
  'deposit',
  'recharge'
));

CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_baofu_verify_fee_active_uidx
    ON payment_orders (business_type, attach)
    WHERE business_type = 'baofu_account_verify_fee'
      AND status IN ('pending', 'paid');

CREATE TABLE IF NOT EXISTS baofu_account_opening_profiles (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_type TEXT NOT NULL,
    profile_status TEXT NOT NULL DEFAULT 'incomplete',
    legal_name TEXT,
    certificate_type TEXT,
    certificate_no_ciphertext TEXT,
    certificate_no_mask TEXT,
    email_ciphertext TEXT,
    email_mask TEXT,
    customer_name TEXT,
    alias_name TEXT,
    corporate_name TEXT,
    corporate_cert_type TEXT,
    corporate_cert_id_ciphertext TEXT,
    corporate_cert_id_mask TEXT,
    corporate_mobile_ciphertext TEXT,
    corporate_mobile_mask TEXT,
    industry_id TEXT,
    contact_name TEXT,
    contact_mobile_ciphertext TEXT,
    contact_mobile_mask TEXT,
    bank_account_no_ciphertext TEXT,
    bank_account_no_mask TEXT,
    bank_mobile_ciphertext TEXT,
    bank_mobile_mask TEXT,
    bank_name TEXT,
    deposit_bank_province TEXT,
    deposit_bank_city TEXT,
    deposit_bank_name TEXT,
    card_user_name TEXT,
    source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_account_opening_profiles_owner_type_check CHECK (owner_type IN ('merchant', 'platform', 'rider', 'operator')),
    CONSTRAINT baofu_account_opening_profiles_account_type_check CHECK (account_type IN ('personal', 'business')),
    CONSTRAINT baofu_account_opening_profiles_status_check CHECK (profile_status IN ('incomplete', 'complete')),
    CONSTRAINT baofu_account_opening_profiles_owner_uidx UNIQUE (owner_type, owner_id)
);

CREATE INDEX IF NOT EXISTS baofu_account_opening_profiles_status_idx
    ON baofu_account_opening_profiles(profile_status, updated_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS baofu_account_opening_flows (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_type TEXT NOT NULL,
    profile_id BIGINT REFERENCES baofu_account_opening_profiles(id),
    state TEXT NOT NULL,
    verify_fee_amount BIGINT NOT NULL DEFAULT 0,
    verify_fee_payment_order_id BIGINT REFERENCES payment_orders(id),
    open_trans_serial_no TEXT,
    login_no TEXT,
    account_binding_id BIGINT REFERENCES baofu_account_bindings(id),
    merchant_report_id BIGINT REFERENCES baofu_merchant_reports(id),
    failure_code TEXT,
    failure_message TEXT,
    provider_request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_account_opening_flows_owner_type_check CHECK (owner_type IN ('merchant', 'platform', 'rider', 'operator')),
    CONSTRAINT baofu_account_opening_flows_account_type_check CHECK (account_type IN ('personal', 'business')),
    CONSTRAINT baofu_account_opening_flows_state_check CHECK (state IN (
        'profile_pending',
        'verify_fee_pending',
        'verify_fee_processing',
        'opening_processing',
        'merchant_report_processing',
        'applet_auth_pending',
        'ready',
        'failed',
        'voided'
    )),
    CONSTRAINT baofu_account_opening_flows_amount_check CHECK (verify_fee_amount >= 0),
    CONSTRAINT baofu_account_opening_flows_open_login_check CHECK (
        state <> 'opening_processing'
        OR length(trim(COALESCE(login_no, ''))) > 0
    ),
    CONSTRAINT baofu_account_opening_flows_rider_operator_fee_check CHECK (
        owner_type NOT IN ('rider', 'operator')
        OR state IN ('profile_pending', 'verify_fee_pending', 'verify_fee_processing', 'voided')
        OR verify_fee_payment_order_id IS NOT NULL
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS baofu_account_opening_flows_active_owner_uidx
    ON baofu_account_opening_flows(owner_type, owner_id)
    WHERE state IN (
        'profile_pending',
        'verify_fee_pending',
        'verify_fee_processing',
        'opening_processing',
        'merchant_report_processing',
        'applet_auth_pending'
    );

CREATE UNIQUE INDEX IF NOT EXISTS baofu_account_opening_flows_open_trans_uidx
    ON baofu_account_opening_flows(open_trans_serial_no)
    WHERE open_trans_serial_no IS NOT NULL;

CREATE INDEX IF NOT EXISTS baofu_account_opening_flows_state_idx
    ON baofu_account_opening_flows(state, updated_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS baofu_account_opening_flows_payment_order_idx
    ON baofu_account_opening_flows(verify_fee_payment_order_id)
    WHERE verify_fee_payment_order_id IS NOT NULL;
