ALTER TABLE ecommerce_applyments
    ADD COLUMN IF NOT EXISTS account_bank_code bigint,
    ADD COLUMN IF NOT EXISTS bank_alias text,
    ADD COLUMN IF NOT EXISTS bank_alias_code text,
    ADD COLUMN IF NOT EXISTS bank_branch_id text;

COMMENT ON COLUMN ecommerce_applyments.account_bank_code IS '微信收付通开户银行编码';
COMMENT ON COLUMN ecommerce_applyments.bank_alias IS '微信收付通银行别名名称';
COMMENT ON COLUMN ecommerce_applyments.bank_alias_code IS '微信收付通银行别名编码';
COMMENT ON COLUMN ecommerce_applyments.bank_branch_id IS '微信收付通支行联行号';