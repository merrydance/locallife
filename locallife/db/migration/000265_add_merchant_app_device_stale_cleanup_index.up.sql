CREATE INDEX IF NOT EXISTS idx_merchant_app_devices_active_last_active_at
    ON merchant_app_devices(last_active_at)
    WHERE status = 'active';
