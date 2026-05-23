-- Keep historical WeChat platform payment rows under their original channel.
-- Active runtime surfaces are removed by code and route cleanup; silently
-- rewriting old provider/channel values as baofu would corrupt audit semantics.

ALTER TABLE profit_sharing_receiver_targets
    DROP CONSTRAINT IF EXISTS profit_sharing_receiver_targets_channel_check;

ALTER TABLE profit_sharing_receiver_targets
    ADD CONSTRAINT profit_sharing_receiver_targets_channel_check
    CHECK (channel IN ('ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

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
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_channel_check CHECK (channel IN ('ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

DROP TABLE IF EXISTS profit_sharing_receiver_attempts CASCADE;
DROP TABLE IF EXISTS profit_sharing_receiver_targets CASCADE;
DROP TABLE IF EXISTS merchant_cancel_withdraw_applications CASCADE;
DROP TABLE IF EXISTS wechat_merchant_violations CASCADE;
DROP TABLE IF EXISTS subsidy_orders CASCADE;
DROP TABLE IF EXISTS wechat_complaints CASCADE;
DROP TABLE IF EXISTS ecommerce_applyments CASCADE;
