ALTER TABLE membership_transactions
ADD COLUMN IF NOT EXISTS payment_order_id BIGINT REFERENCES payment_orders(id);

CREATE UNIQUE INDEX IF NOT EXISTS membership_transactions_payment_order_id_idx
    ON membership_transactions(payment_order_id)
    WHERE payment_order_id IS NOT NULL;
