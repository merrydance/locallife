-- Add payment_order_id to rider_deposits for idempotency
ALTER TABLE rider_deposits
    ADD COLUMN IF NOT EXISTS payment_order_id BIGINT REFERENCES payment_orders(id);

CREATE UNIQUE INDEX IF NOT EXISTS rider_deposits_payment_order_id_idx
    ON rider_deposits(payment_order_id)
    WHERE payment_order_id IS NOT NULL;
