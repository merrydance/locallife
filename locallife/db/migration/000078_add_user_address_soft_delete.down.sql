DROP INDEX IF EXISTS idx_user_addresses_deleted_at;
ALTER TABLE user_addresses DROP COLUMN IF EXISTS deleted_at;
