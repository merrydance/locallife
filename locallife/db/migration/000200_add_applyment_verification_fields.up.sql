ALTER TABLE ecommerce_applyments
ADD COLUMN legal_validation_url TEXT,
ADD COLUMN account_validation JSONB;

COMMENT ON COLUMN ecommerce_applyments.legal_validation_url IS '微信进件返回的法人扫码验证链接';
COMMENT ON COLUMN ecommerce_applyments.account_validation IS '微信进件返回的汇款账户验证原始信息，敏感字段保持微信返回密文';