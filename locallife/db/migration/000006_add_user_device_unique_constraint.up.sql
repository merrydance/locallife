-- 为 user_devices 表添加 (user_id, device_id) 唯一约束
-- 这是 upsert 操作所需的

-- 先删除现有的索引
DROP INDEX IF EXISTS "user_devices_user_id_device_id_idx";

-- 创建唯一约束
ALTER TABLE "user_devices" ADD CONSTRAINT "user_devices_user_id_device_id_key" UNIQUE ("user_id", "device_id");
