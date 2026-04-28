CREATE TABLE IF NOT EXISTS merchant_app_devices (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    platform VARCHAR(32) NOT NULL DEFAULT 'android',
    provider VARCHAR(32) NOT NULL DEFAULT 'unknown',
    push_token VARCHAR(512) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    device_model VARCHAR(100),
    os_version VARCHAR(50),
    app_version VARCHAR(32),
    last_registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unregistered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT merchant_app_devices_platform_check CHECK (platform IN ('android')),
    CONSTRAINT merchant_app_devices_provider_check CHECK (provider IN ('huawei', 'honor', 'xiaomi', 'oppo', 'vivo', 'unknown')),
    CONSTRAINT merchant_app_devices_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT merchant_app_devices_device_id_not_blank CHECK (btrim(device_id) <> ''),
    CONSTRAINT merchant_app_devices_push_token_not_blank CHECK (btrim(push_token) <> '')
);

CREATE UNIQUE INDEX idx_merchant_app_devices_active_device_id
    ON merchant_app_devices(device_id)
    WHERE status = 'active';

CREATE UNIQUE INDEX idx_merchant_app_devices_active_push_token
    ON merchant_app_devices(push_token)
    WHERE status = 'active';

CREATE INDEX idx_merchant_app_devices_merchant_active
    ON merchant_app_devices(merchant_id, status, last_active_at DESC);

CREATE INDEX idx_merchant_app_devices_provider_active
    ON merchant_app_devices(provider, status, last_active_at DESC);

CREATE INDEX idx_merchant_app_devices_user_active
    ON merchant_app_devices(user_id, status, last_active_at DESC);

COMMENT ON TABLE merchant_app_devices IS 'Android merchant app native push device registry';
COMMENT ON COLUMN merchant_app_devices.device_id IS 'Client-persisted app install identifier';
COMMENT ON COLUMN merchant_app_devices.provider IS 'Native push provider: huawei, honor, xiaomi, oppo, vivo, or unknown';
COMMENT ON COLUMN merchant_app_devices.push_token IS 'Native vendor push token; never log raw value';