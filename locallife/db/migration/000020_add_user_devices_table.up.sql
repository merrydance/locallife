-- 用户设备指纹表（用于检测团伙欺诈）
CREATE TABLE IF NOT EXISTS user_devices (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL, -- 设备指纹（设备唯一标识）
    device_type VARCHAR(50), -- ios/android/web
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_user_devices_user_id ON user_devices(user_id);
CREATE INDEX idx_user_devices_device_id ON user_devices(device_id);
CREATE UNIQUE INDEX idx_user_devices_user_device ON user_devices(user_id, device_id);

-- 触发器：自动更新 updated_at
CREATE TRIGGER update_user_devices_updated_at
    BEFORE UPDATE ON user_devices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE user_devices IS '用户设备指纹表，用于检测设备复用欺诈';
COMMENT ON COLUMN user_devices.device_id IS '设备指纹，可以是设备IMEI、UUID或浏览器指纹';
