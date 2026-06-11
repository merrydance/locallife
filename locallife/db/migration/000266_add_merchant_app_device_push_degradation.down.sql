ALTER TABLE merchant_app_devices
    DROP CONSTRAINT IF EXISTS merchant_app_devices_push_failure_count_check,
    DROP COLUMN IF EXISTS push_degraded_at,
    DROP COLUMN IF EXISTS last_push_failure_at,
    DROP COLUMN IF EXISTS last_push_failure_reason,
    DROP COLUMN IF EXISTS push_failure_count;
