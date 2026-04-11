DROP INDEX IF EXISTS rider_deposits_payment_order_id_idx;

CREATE UNIQUE INDEX IF NOT EXISTS rider_deposits_payment_order_id_idx
    ON rider_deposits(payment_order_id)
    WHERE payment_order_id IS NOT NULL;