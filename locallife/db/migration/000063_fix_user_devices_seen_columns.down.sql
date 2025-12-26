-- Revert user_devices schema drift fix.

-- Drop trigger first since it may reference updated_at.
DROP TRIGGER IF EXISTS update_user_devices_updated_at ON user_devices;

ALTER TABLE user_devices
    DROP COLUMN IF EXISTS first_seen,
    DROP COLUMN IF EXISTS last_seen,
    DROP COLUMN IF EXISTS updated_at;
