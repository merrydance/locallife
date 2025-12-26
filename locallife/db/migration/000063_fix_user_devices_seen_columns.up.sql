-- Fix schema drift for user_devices between early (M1) and later (M20) definitions.
-- Some deployments already have user_devices created by 000002, so 000020's
-- CREATE TABLE IF NOT EXISTS will not add the new columns.
--
-- This migration makes the table compatible with the sqlc query UpsertUserDevice
-- (first_seen/last_seen/updated_at).

ALTER TABLE user_devices
    ADD COLUMN IF NOT EXISTS first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Best-effort backfill for existing rows created by the old schema.
-- created_at and last_login_at exist in the old schema.
UPDATE user_devices
SET first_seen = created_at
WHERE created_at IS NOT NULL;

UPDATE user_devices
SET last_seen = last_login_at
WHERE last_login_at IS NOT NULL;
