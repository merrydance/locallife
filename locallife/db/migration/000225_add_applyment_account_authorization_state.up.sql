ALTER TABLE ecommerce_applyments
ADD COLUMN account_willingness_business_code TEXT,
ADD COLUMN account_willingness_applyment_id BIGINT,
ADD COLUMN account_willingness_state TEXT,
ADD COLUMN account_willingness_qrcode_data TEXT,
ADD COLUMN account_willingness_reject_reason TEXT,
ADD COLUMN account_authorize_state TEXT,
ADD COLUMN account_authorize_state_checked_at TIMESTAMPTZ;

COMMENT ON COLUMN ecommerce_applyments.account_willingness_business_code IS '普通服务商开户意愿确认业务申请编号';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_applyment_id IS '普通服务商开户意愿确认微信申请单号';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_state IS '普通服务商开户意愿确认审核状态';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_qrcode_data IS '普通服务商开户意愿确认二维码数据';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_reject_reason IS '普通服务商开户意愿确认驳回原因';
COMMENT ON COLUMN ecommerce_applyments.account_authorize_state IS '普通服务商特约商户开户意愿授权状态';
COMMENT ON COLUMN ecommerce_applyments.account_authorize_state_checked_at IS '最近一次查询普通服务商开户意愿授权状态的时间';

CREATE INDEX idx_ecommerce_applyments_account_authorize_state
ON ecommerce_applyments (account_authorize_state, account_authorize_state_checked_at)
WHERE account_authorize_state IS NOT NULL;
