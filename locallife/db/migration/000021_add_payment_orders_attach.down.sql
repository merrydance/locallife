-- 移除attach字段
DROP INDEX IF EXISTS payment_orders_attach_idx;
ALTER TABLE payment_orders DROP COLUMN IF EXISTS attach;
