-- 修复 user_devices 表，将 VARCHAR(n) 改为 TEXT（遵循 PostgreSQL 最佳实践）
ALTER TABLE user_devices 
    ALTER COLUMN device_id TYPE TEXT,
    ALTER COLUMN device_type TYPE TEXT;

COMMENT ON COLUMN user_devices.device_id IS '设备指纹，可以是设备IMEI、UUID或浏览器指纹（TEXT类型，遵循PostgreSQL最佳实践）';
COMMENT ON COLUMN user_devices.device_type IS '设备类型：ios/android/web等';
