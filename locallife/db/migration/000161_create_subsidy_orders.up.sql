-- 补差订单表
-- 对应微信平台收付通补差 API：/v3/ecommerce/subsidies/create
-- 用于平台营销活动（满减、折扣等）出资补贴给二级商户

CREATE TABLE subsidy_orders (
    id                  BIGSERIAL PRIMARY KEY,

    -- 关联支付主订单（收付通子单对应的 payment_order）
    payment_order_id    BIGINT NOT NULL REFERENCES payment_orders(id),

    -- 子单商户号
    sub_mch_id          TEXT NOT NULL,

    -- 子单微信支付订单号（支付成功后写入，发起补差时必须）
    transaction_id      TEXT,

    -- 商户补差单号（全局唯一，用于幂等）
    out_subsidy_no      TEXT NOT NULL UNIQUE,

    -- 用户实际支付金额（分）
    payer_amount        BIGINT NOT NULL,

    -- 补差金额（分）
    amount              BIGINT NOT NULL
                            CHECK (amount > 0),

    -- 补差说明（最长80字）
    description         TEXT NOT NULL,

    -- 补差状态
    -- pending: 待发起  success: 补差成功  failed: 补差失败  canceled: 已取消
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'success', 'failed', 'canceled')),

    -- 微信补差单号（成功后由微信返回）
    wxpay_subsidy_id    TEXT,

    -- 失败原因（failed 时记录）
    fail_reason         TEXT,

    -- ==================== 补差退回（退款时使用） ====================

    -- 补差退回商户单号
    out_return_no       TEXT UNIQUE,

    -- 退回金额（分），≤ amount
    return_amount       BIGINT,

    -- 退回状态：pending_return / return_success / return_failed
    return_status       TEXT
                            CHECK (return_status IN ('pending_return', 'return_success', 'return_failed')),

    -- 微信退回单号
    return_wxpay_id     TEXT,

    -- 退回失败原因
    return_fail_reason  TEXT,

    -- ==================== 时间 ====================

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ
);

CREATE INDEX idx_subsidy_orders_payment_order  ON subsidy_orders(payment_order_id);
CREATE INDEX idx_subsidy_orders_status         ON subsidy_orders(status);
CREATE INDEX idx_subsidy_orders_sub_mch_id     ON subsidy_orders(sub_mch_id);
CREATE INDEX idx_subsidy_orders_created_at     ON subsidy_orders(created_at DESC);

COMMENT ON TABLE subsidy_orders IS '平台收付通补差订单；平台出资补贴给二级商户，对应微信支付 /v3/ecommerce/subsidies/create';
