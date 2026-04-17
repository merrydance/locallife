ALTER TABLE payment_orders
    ALTER COLUMN payment_channel SET NOT NULL,
    ALTER COLUMN requires_profit_sharing SET NOT NULL;

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce'));