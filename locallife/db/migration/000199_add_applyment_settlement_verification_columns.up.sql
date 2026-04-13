ALTER TABLE ecommerce_applyments
ADD COLUMN settlement_verify_first_trade_at TIMESTAMPTZ,
ADD COLUMN settlement_verify_last_checked_at TIMESTAMPTZ,
ADD COLUMN settlement_verify_check_count INT NOT NULL DEFAULT 0,
ADD COLUMN settlement_verify_status TEXT,
ADD COLUMN settlement_verify_fail_reason TEXT,
ADD COLUMN settlement_verify_failed_notified_at TIMESTAMPTZ,
ADD CONSTRAINT ecommerce_applyments_settlement_verify_status_check
CHECK (
    settlement_verify_status IS NULL OR
    settlement_verify_status IN ('verifying', 'success', 'fail')
);

CREATE INDEX idx_ecommerce_applyments_settlement_verify
ON ecommerce_applyments (
    subject_type,
    status,
    settlement_verify_status,
    settlement_verify_last_checked_at,
    id
)
WHERE subject_type = 'merchant';

COMMENT ON COLUMN ecommerce_applyments.settlement_verify_first_trade_at IS '商户首笔支付成功时间，用于 0.01 结算卡验卡三日巡检';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_last_checked_at IS '最近一次查询微信结算卡验卡结果的时间';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_check_count IS '已执行的每日验卡巡检次数，最多三次';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_status IS '结算卡验卡巡检状态: verifying-巡检中, success-巡检结束且未发现失败, fail-巡检发现失败';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_fail_reason IS '微信返回的结算卡验卡失败原因';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_failed_notified_at IS '运营商已收到结算卡失败通知的时间';