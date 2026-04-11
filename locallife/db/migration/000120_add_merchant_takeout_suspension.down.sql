-- Drop takeout-only suspension fields from merchant_profiles

DROP INDEX IF EXISTS idx_merchant_profiles_takeout_suspended;

ALTER TABLE merchant_profiles
  DROP COLUMN IF EXISTS is_takeout_suspended,
  DROP COLUMN IF EXISTS takeout_suspend_reason,
  DROP COLUMN IF EXISTS takeout_suspended_at,
  DROP COLUMN IF EXISTS takeout_suspend_until;
