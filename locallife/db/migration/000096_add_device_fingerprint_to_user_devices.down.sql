DROP INDEX IF EXISTS idx_user_devices_fingerprint;
ALTER TABLE user_devices
DROP COLUMN IF EXISTS device_fingerprint;
