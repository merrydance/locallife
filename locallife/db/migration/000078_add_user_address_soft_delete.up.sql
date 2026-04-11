-- 为用户地址添加软删除字段，避免历史订单外键阻塞删除
ALTER TABLE user_addresses ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_user_addresses_deleted_at ON user_addresses(deleted_at) WHERE deleted_at IS NULL;
