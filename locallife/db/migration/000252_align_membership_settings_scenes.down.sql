ALTER TABLE merchant_membership_settings
    DROP CONSTRAINT IF EXISTS merchant_membership_settings_balance_scenes_check,
    DROP CONSTRAINT IF EXISTS merchant_membership_settings_bonus_scenes_check;

ALTER TABLE merchant_membership_settings
    ALTER COLUMN balance_usable_scenes SET DEFAULT ARRAY['dine_in', 'takeout', 'reservation']::TEXT[],
    ALTER COLUMN bonus_usable_scenes SET DEFAULT ARRAY['dine_in']::TEXT[];

COMMENT ON COLUMN merchant_membership_settings.balance_usable_scenes IS '余额可用场景: dine_in(堂食), takeout(外卖), reservation(预定)';
COMMENT ON COLUMN merchant_membership_settings.bonus_usable_scenes IS '赠送金额可用场景，可比余额更严格';
