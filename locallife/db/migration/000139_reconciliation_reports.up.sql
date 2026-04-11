-- 每日对账报告表
-- 记录与微信支付账单的自动比对结果，供运营和财务团队审查

CREATE TABLE reconciliation_reports (
    id              BIGSERIAL PRIMARY KEY,
    -- 对账日期（账单日期，T+1可用，我们通常跑 T-1 日账单）
    bill_date       DATE NOT NULL,
    -- 账单类型：trade=小程序直连支付账单，ecommerce_trade=收付通合单账单，refund=退款账单
    bill_type       TEXT NOT NULL
                        CHECK (bill_type IN ('trade', 'ecommerce_trade', 'refund')),
    -- 对账状态
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    -- 微信账单总笔数
    wxpay_count     INT NOT NULL DEFAULT 0,
    -- 本地数据库记录总笔数（同日期范围）
    local_count     INT NOT NULL DEFAULT 0,
    -- 差异笔数（missing_local + missing_wxpay + amount_mismatch 之和）
    mismatch_count  INT NOT NULL DEFAULT 0,
    -- 微信有记录、本地无记录的订单列表：[{"out_trade_no":"...", "amount":100}]
    missing_local   JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- 本地有记录、微信账单无记录的订单列表：[{"out_trade_no":"...", "amount":100}]
    missing_wxpay   JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- 金额不一致的订单列表：[{"out_trade_no":"...", "wxpay_amount":100, "local_amount":99}]
    amount_mismatch JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- 对账失败时的错误信息
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ,
    -- 同一天同一类型只保留一条报告（重新跑时覆盖）
    UNIQUE (bill_date, bill_type)
);

CREATE INDEX idx_reconciliation_reports_bill_date ON reconciliation_reports(bill_date DESC);
CREATE INDEX idx_reconciliation_reports_status    ON reconciliation_reports(status);

COMMENT ON TABLE reconciliation_reports IS '每日微信支付对账报告，由 bill-reconciliation 调度器自动生成';
