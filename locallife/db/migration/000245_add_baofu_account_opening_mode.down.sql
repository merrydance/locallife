ALTER TABLE baofu_account_bindings
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_owner_check,
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_opening_mode_check;

ALTER TABLE baofu_account_opening_flows
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_owner_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_flows_opening_mode_check;

ALTER TABLE baofu_account_opening_profiles
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_owner_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_account_type_check,
    DROP CONSTRAINT IF EXISTS baofu_account_opening_profiles_opening_mode_check;

ALTER TABLE baofu_account_bindings
    DROP COLUMN IF EXISTS opening_mode;

ALTER TABLE baofu_account_opening_flows
    DROP COLUMN IF EXISTS opening_mode;

ALTER TABLE baofu_account_opening_profiles
    DROP COLUMN IF EXISTS opening_mode;
