DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ecommerce_applyments
        WHERE status IN ('checking', 'account_need_verify', 'to_be_confirmed', 'canceled')
    ) THEN
        RAISE EXCEPTION 'cannot downgrade ecommerce_applyments status constraint while rows use checking/account_need_verify/to_be_confirmed/canceled';
    END IF;
END
$$;

ALTER TABLE ecommerce_applyments
DROP CONSTRAINT IF EXISTS ecommerce_applyments_status_check;

ALTER TABLE ecommerce_applyments
ADD CONSTRAINT ecommerce_applyments_status_check
CHECK (status IN (
    'pending',
    'submitted',
    'auditing',
    'rejected',
    'frozen',
    'to_be_signed',
    'signing',
    'rejected_sign',
    'finish'
));

COMMENT ON COLUMN ecommerce_applyments.status IS '进件状态: pending-待提交, submitted-已提交, auditing-审核中, rejected-已驳回, frozen-冻结, to_be_signed-待签约, signing-签约中, rejected_sign-签约失败, finish-完成';