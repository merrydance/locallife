ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS payment_channel TEXT,
    ADD COLUMN IF NOT EXISTS requires_profit_sharing BOOLEAN;

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IS NULL OR payment_channel IN ('direct', 'ecommerce'));

CREATE INDEX IF NOT EXISTS payment_orders_payment_channel_idx ON payment_orders(payment_channel);
CREATE INDEX IF NOT EXISTS payment_orders_requires_profit_sharing_idx ON payment_orders(requires_profit_sharing) WHERE requires_profit_sharing = TRUE;