-- 移除唯一约束
ALTER TABLE "user_devices" DROP CONSTRAINT IF EXISTS "user_devices_user_id_device_id_key";

-- 恢复索引
CREATE INDEX IF NOT EXISTS "user_devices_user_id_device_id_idx" ON "user_devices" ("user_id", "device_id");
