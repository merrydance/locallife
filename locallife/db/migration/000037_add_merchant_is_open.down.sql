-- 回滚商户营业状态字段
DROP INDEX IF EXISTS idx_merchants_is_open;
ALTER TABLE merchants DROP COLUMN IF EXISTS auto_close_at;
ALTER TABLE merchants DROP COLUMN IF EXISTS is_open;
