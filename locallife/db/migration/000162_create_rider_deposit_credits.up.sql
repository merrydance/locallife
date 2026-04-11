CREATE TABLE rider_deposit_credits (
    id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES riders(id) ON DELETE CASCADE,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    original_amount BIGINT NOT NULL CHECK (original_amount >= 0),
    refundable_amount BIGINT NOT NULL CHECK (refundable_amount >= 0),
    refunded_amount BIGINT NOT NULL DEFAULT 0 CHECK (refunded_amount >= 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('active', 'partially_refunded', 'fully_refunded', 'expired', 'legacy')),
    paid_at TIMESTAMPTZ NOT NULL,
    refundable_until TIMESTAMPTZ NOT NULL,
    last_reminded_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rider_deposit_credits_payment_order_unique UNIQUE (payment_order_id),
    CONSTRAINT rider_deposit_credits_amount_consistency CHECK (refundable_amount + refunded_amount <= original_amount)
);

CREATE INDEX idx_rider_deposit_credits_rider_status_until
    ON rider_deposit_credits (rider_id, status, refundable_until);

CREATE INDEX idx_rider_deposit_credits_status_until
    ON rider_deposit_credits (status, refundable_until);

COMMENT ON TABLE rider_deposit_credits IS '骑手押金可退款凭证，记录每笔押金支付单在微信退款窗口内的可退款状态';
COMMENT ON COLUMN rider_deposit_credits.payment_order_id IS '原始押金支付单 ID，后续提现应基于该支付单走退款';
COMMENT ON COLUMN rider_deposit_credits.refundable_until IS '微信退款有效截止时间，通常为支付成功后 365 天';
COMMENT ON COLUMN rider_deposit_credits.status IS 'active=可退款, partially_refunded=部分退款, fully_refunded=已全部退款, expired=已过期, legacy=历史兼容数据';