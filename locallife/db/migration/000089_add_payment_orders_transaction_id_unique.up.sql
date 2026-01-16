CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_transaction_id_unique_idx
    ON payment_orders(transaction_id)
    WHERE transaction_id IS NOT NULL;
