ALTER TABLE ecommerce_applyments
    DROP COLUMN IF EXISTS bank_branch_id,
    DROP COLUMN IF EXISTS bank_alias_code,
    DROP COLUMN IF EXISTS bank_alias,
    DROP COLUMN IF EXISTS account_bank_code;