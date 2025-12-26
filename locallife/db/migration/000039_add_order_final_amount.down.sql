-- 回滚: 删除 final_amount 和 platform_commission 字段
DROP INDEX IF EXISTS idx_orders_final_amount;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_platform_commission_check;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_final_amount_check;
ALTER TABLE orders DROP COLUMN IF EXISTS platform_commission;
ALTER TABLE orders DROP COLUMN IF EXISTS final_amount;
