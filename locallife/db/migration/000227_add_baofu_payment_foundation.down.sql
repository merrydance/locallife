DROP TABLE IF EXISTS baofu_withdrawal_orders;
DROP TABLE IF EXISTS baofu_fee_ledger;
DROP TABLE IF EXISTS baofu_account_bindings;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_payment_fee_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_provider_check;

ALTER TABLE profit_sharing_orders
    DROP COLUMN IF EXISTS sharing_detail_snapshot,
    DROP COLUMN IF EXISTS platform_sharing_mer_id,
    DROP COLUMN IF EXISTS operator_sharing_mer_id,
    DROP COLUMN IF EXISTS rider_sharing_mer_id,
    DROP COLUMN IF EXISTS merchant_sharing_mer_id,
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS payment_fee_rate_bps,
    DROP COLUMN IF EXISTS payment_fee;

ALTER TABLE external_payment_facts
    DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
    ADD CONSTRAINT external_payment_facts_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));

ALTER TABLE external_payment_commands
    DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
    ADD CONSTRAINT external_payment_commands_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));
