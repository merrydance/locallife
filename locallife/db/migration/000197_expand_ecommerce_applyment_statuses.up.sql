ALTER TABLE ecommerce_applyments
DROP CONSTRAINT IF EXISTS ecommerce_applyments_status_check;

ALTER TABLE ecommerce_applyments
ADD CONSTRAINT ecommerce_applyments_status_check
CHECK (status IN (
    'pending',
    'submitted',
    'checking',
    'auditing',
    'account_need_verify',
    'to_be_confirmed',
    'rejected',
    'frozen',
    'to_be_signed',
    'signing',
    'rejected_sign',
    'finish',
    'canceled'
));

COMMENT ON COLUMN ecommerce_applyments.status IS '进件状态: pending-待提交, submitted-已提交, checking-资料校验中, auditing-审核中, account_need_verify-待账户验证, to_be_confirmed-待确认, rejected-已驳回, frozen-冻结, to_be_signed-待签约, signing-签约中, rejected_sign-签约失败, finish-完成, canceled-已作废';