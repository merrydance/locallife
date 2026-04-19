DROP INDEX IF EXISTS payment_orders_requires_profit_sharing_idx;
DROP INDEX IF EXISTS payment_orders_payment_channel_idx;

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    DROP COLUMN IF EXISTS requires_profit_sharing,
    DROP COLUMN IF EXISTS payment_channel;