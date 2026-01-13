DROP INDEX IF EXISTS orders_replaced_by_idx;
ALTER TABLE orders DROP COLUMN IF EXISTS replaced_by_order_id;
