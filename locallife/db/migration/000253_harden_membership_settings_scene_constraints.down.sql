ALTER TABLE merchant_membership_settings
    DROP CONSTRAINT IF EXISTS merchant_membership_settings_balance_scenes_check,
    DROP CONSTRAINT IF EXISTS merchant_membership_settings_bonus_scenes_check;

UPDATE merchant_membership_settings
SET
    balance_usable_scenes = COALESCE((
        SELECT array_agg(scene ORDER BY ord)
        FROM unnest(balance_usable_scenes) WITH ORDINALITY AS existing(scene, ord)
        WHERE scene = ANY (ARRAY['dine_in', 'takeaway']::TEXT[])
    ), ARRAY[]::TEXT[]),
    bonus_usable_scenes = COALESCE((
        SELECT array_agg(scene ORDER BY ord)
        FROM unnest(bonus_usable_scenes) WITH ORDINALITY AS existing(scene, ord)
        WHERE scene = ANY (ARRAY['dine_in', 'takeaway']::TEXT[])
    ), ARRAY[]::TEXT[]),
    updated_at = NOW()
WHERE NOT balance_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]
   OR array_position(balance_usable_scenes, NULL) IS NOT NULL
   OR NOT bonus_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]
   OR array_position(bonus_usable_scenes, NULL) IS NOT NULL;

ALTER TABLE merchant_membership_settings
    ALTER COLUMN balance_usable_scenes SET DEFAULT ARRAY['dine_in', 'takeaway']::TEXT[],
    ALTER COLUMN bonus_usable_scenes SET DEFAULT ARRAY['dine_in']::TEXT[];

ALTER TABLE merchant_membership_settings
    ADD CONSTRAINT merchant_membership_settings_balance_scenes_check
        CHECK (
            array_position(balance_usable_scenes, NULL) IS NULL
            AND balance_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]
        ),
    ADD CONSTRAINT merchant_membership_settings_bonus_scenes_check
        CHECK (
            array_position(bonus_usable_scenes, NULL) IS NULL
            AND bonus_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]
        );

COMMENT ON COLUMN merchant_membership_settings.balance_usable_scenes IS '余额可用场景: dine_in(堂食), takeaway(外带自取)';
COMMENT ON COLUMN merchant_membership_settings.bonus_usable_scenes IS '赠送金额可用场景: dine_in(堂食), takeaway(外带自取)，可比余额更严格';
