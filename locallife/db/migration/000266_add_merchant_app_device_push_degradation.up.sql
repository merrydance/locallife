ALTER TABLE merchant_app_devices
    ADD COLUMN IF NOT EXISTS push_failure_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_push_failure_reason VARCHAR(255),
    ADD COLUMN IF NOT EXISTS last_push_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS push_degraded_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'merchant_app_devices_push_failure_count_check'
          AND conrelid = 'merchant_app_devices'::regclass
    ) THEN
        ALTER TABLE merchant_app_devices
            ADD CONSTRAINT merchant_app_devices_push_failure_count_check
            CHECK (push_failure_count >= 0);
    END IF;
END
$$;

COMMENT ON COLUMN merchant_app_devices.push_failure_count IS 'Consecutive terminal native-push provider failures for this active device token';
COMMENT ON COLUMN merchant_app_devices.last_push_failure_reason IS 'Sanitized terminal native-push provider failure reason';
COMMENT ON COLUMN merchant_app_devices.last_push_failure_at IS 'Last terminal native-push provider failure timestamp';
COMMENT ON COLUMN merchant_app_devices.push_degraded_at IS 'First timestamp when this active device token entered degraded native-push state';
