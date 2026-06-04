ALTER TABLE baofu_account_opening_profiles
    ADD COLUMN IF NOT EXISTS opening_mode TEXT;

ALTER TABLE baofu_account_opening_flows
    ADD COLUMN IF NOT EXISTS opening_mode TEXT;

ALTER TABLE baofu_account_bindings
    ADD COLUMN IF NOT EXISTS opening_mode TEXT;

UPDATE baofu_account_opening_profiles
SET opening_mode = CASE
    WHEN account_type = 'personal' AND owner_type = 'merchant' THEN 'merchant_personal_micro'
    WHEN account_type = 'personal' THEN 'personal'
    WHEN account_type = 'business'
         AND owner_type = 'merchant'
         AND source_snapshot ->> 'self_employed' = 'true'
         AND length(trim(COALESCE(card_user_name, ''))) > 0 THEN 'individual_business_private'
    ELSE 'business_public'
END
WHERE opening_mode IS NULL;

UPDATE baofu_account_opening_flows AS flow
SET opening_mode = COALESCE(
    (
        SELECT profile.opening_mode
        FROM baofu_account_opening_profiles AS profile
        WHERE profile.id = flow.profile_id
          AND profile.account_type = flow.account_type
        LIMIT 1
    ),
    (
        SELECT profile.opening_mode
        FROM baofu_account_opening_profiles AS profile
        WHERE profile.owner_type = flow.owner_type
          AND profile.owner_id = flow.owner_id
          AND profile.account_type = flow.account_type
        ORDER BY profile.updated_at DESC, profile.id DESC
        LIMIT 1
    ),
    CASE
        WHEN flow.account_type = 'personal' AND flow.owner_type = 'merchant' THEN 'merchant_personal_micro'
        WHEN flow.account_type = 'personal' THEN 'personal'
        ELSE 'business_public'
    END
)
WHERE flow.opening_mode IS NULL;

UPDATE baofu_account_bindings AS binding
SET opening_mode = COALESCE(
    (
        SELECT flow.opening_mode
        FROM baofu_account_opening_flows AS flow
        WHERE flow.owner_type = binding.owner_type
          AND flow.owner_id = binding.owner_id
          AND flow.account_type = binding.account_type
        ORDER BY flow.created_at DESC, flow.id DESC
        LIMIT 1
    ),
    (
        SELECT profile.opening_mode
        FROM baofu_account_opening_profiles AS profile
        WHERE profile.owner_type = binding.owner_type
          AND profile.owner_id = binding.owner_id
          AND profile.account_type = binding.account_type
        ORDER BY profile.updated_at DESC, profile.id DESC
        LIMIT 1
    ),
    CASE
        WHEN binding.account_type = 'personal' AND binding.owner_type = 'merchant' THEN 'merchant_personal_micro'
        WHEN binding.account_type = 'personal' THEN 'personal'
        ELSE 'business_public'
    END
)
WHERE binding.opening_mode IS NULL;

ALTER TABLE baofu_account_opening_profiles
    ALTER COLUMN opening_mode SET NOT NULL;

ALTER TABLE baofu_account_opening_flows
    ALTER COLUMN opening_mode SET NOT NULL;

ALTER TABLE baofu_account_bindings
    ALTER COLUMN opening_mode SET NOT NULL;

ALTER TABLE baofu_account_opening_profiles
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_owner_check;

ALTER TABLE baofu_account_opening_profiles
    ADD CONSTRAINT baofu_account_opening_profiles_opening_mode_check CHECK (opening_mode IN ('personal', 'merchant_personal_micro', 'business_public', 'individual_business_private')),
    ADD CONSTRAINT baofu_account_opening_profiles_opening_mode_account_type_check CHECK (
        (account_type = 'personal' AND opening_mode IN ('personal', 'merchant_personal_micro'))
        OR (account_type = 'business' AND opening_mode IN ('business_public', 'individual_business_private'))
    ),
    ADD CONSTRAINT baofu_account_opening_profiles_opening_mode_owner_check CHECK (
        (opening_mode = 'merchant_personal_micro' AND owner_type = 'merchant')
        OR (opening_mode = 'individual_business_private' AND owner_type = 'merchant')
        OR opening_mode IN ('personal', 'business_public')
    );

ALTER TABLE baofu_account_opening_flows
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_owner_check;

ALTER TABLE baofu_account_opening_flows
    ADD CONSTRAINT baofu_account_opening_flows_opening_mode_check CHECK (opening_mode IN ('personal', 'merchant_personal_micro', 'business_public', 'individual_business_private')),
    ADD CONSTRAINT baofu_account_opening_flows_opening_mode_account_type_check CHECK (
        (account_type = 'personal' AND opening_mode IN ('personal', 'merchant_personal_micro'))
        OR (account_type = 'business' AND opening_mode IN ('business_public', 'individual_business_private'))
    ),
    ADD CONSTRAINT baofu_account_opening_flows_opening_mode_owner_check CHECK (
        (opening_mode = 'merchant_personal_micro' AND owner_type = 'merchant')
        OR (opening_mode = 'individual_business_private' AND owner_type = 'merchant')
        OR opening_mode IN ('personal', 'business_public')
    );

ALTER TABLE baofu_account_bindings
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_check,
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_owner_check;

ALTER TABLE baofu_account_bindings
    ADD CONSTRAINT baofu_account_bindings_opening_mode_check CHECK (opening_mode IN ('personal', 'merchant_personal_micro', 'business_public', 'individual_business_private')),
    ADD CONSTRAINT baofu_account_bindings_opening_mode_account_type_check CHECK (
        (account_type = 'personal' AND opening_mode IN ('personal', 'merchant_personal_micro'))
        OR (account_type = 'business' AND opening_mode IN ('business_public', 'individual_business_private'))
    ),
    ADD CONSTRAINT baofu_account_bindings_opening_mode_owner_check CHECK (
        (opening_mode = 'merchant_personal_micro' AND owner_type = 'merchant')
        OR (opening_mode = 'individual_business_private' AND owner_type = 'merchant')
        OR opening_mode IN ('personal', 'business_public')
    );
