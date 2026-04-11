-- orders.replaced_by_order_id to track superseded orders
ALTER TABLE orders
ADD COLUMN replaced_by_order_id BIGINT REFERENCES orders(id);

-- optional index to speed lookups for replaced chains
CREATE INDEX IF NOT EXISTS orders_replaced_by_idx ON orders(replaced_by_order_id) WHERE replaced_by_order_id IS NOT NULL;
