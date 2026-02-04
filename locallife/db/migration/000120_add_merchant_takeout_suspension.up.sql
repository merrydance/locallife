-- Add takeout-only suspension fields to merchant_profiles

ALTER TABLE merchant_profiles
  ADD COLUMN is_takeout_suspended BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN takeout_suspend_reason TEXT,
  ADD COLUMN takeout_suspended_at TIMESTAMPTZ,
  ADD COLUMN takeout_suspend_until TIMESTAMPTZ;

CREATE INDEX idx_merchant_profiles_takeout_suspended ON merchant_profiles(is_takeout_suspended);

COMMENT ON COLUMN merchant_profiles.is_takeout_suspended IS '是否暂停外卖接单（追偿逾期/经营限制）';
COMMENT ON COLUMN merchant_profiles.takeout_suspend_reason IS '外卖暂停原因';
COMMENT ON COLUMN merchant_profiles.takeout_suspend_until IS '外卖暂停截止时间';
