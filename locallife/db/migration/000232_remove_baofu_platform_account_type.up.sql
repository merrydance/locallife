UPDATE baofu_account_bindings
SET account_type = 'business'
WHERE account_type = 'platform';

ALTER TABLE baofu_account_bindings
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_account_type_check;

ALTER TABLE baofu_account_bindings
    ADD CONSTRAINT baofu_account_bindings_account_type_check CHECK (account_type IN ('personal', 'business'));
