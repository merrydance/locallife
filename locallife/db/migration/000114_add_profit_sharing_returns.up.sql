-- 新增分账回退记录表
CREATE TABLE IF NOT EXISTS profit_sharing_returns (
    id BIGSERIAL PRIMARY KEY,

    refund_order_id BIGINT NOT NULL REFERENCES refund_orders(id) ON DELETE CASCADE,
    profit_sharing_order_id BIGINT NOT NULL REFERENCES profit_sharing_orders(id),
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),

    sub_mchid TEXT NOT NULL,
    out_order_no TEXT NOT NULL,
    out_return_no TEXT UNIQUE NOT NULL,
    return_mchid TEXT NOT NULL,
    amount BIGINT NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending',
    return_id TEXT,
    fail_reason TEXT,
    finished_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT profit_sharing_returns_status_check
        CHECK (status IN ('pending', 'processing', 'success', 'failed')),
    CONSTRAINT profit_sharing_returns_amount_check
        CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS profit_sharing_returns_refund_order_id_idx ON profit_sharing_returns(refund_order_id);
CREATE INDEX IF NOT EXISTS profit_sharing_returns_profit_sharing_order_id_idx ON profit_sharing_returns(profit_sharing_order_id);
CREATE INDEX IF NOT EXISTS profit_sharing_returns_payment_order_id_idx ON profit_sharing_returns(payment_order_id);
CREATE INDEX IF NOT EXISTS profit_sharing_returns_status_idx ON profit_sharing_returns(status);

COMMENT ON TABLE profit_sharing_returns IS '分账回退记录表，退款前的分账回退流水';
COMMENT ON COLUMN profit_sharing_returns.out_return_no IS '商户分账回退单号';
COMMENT ON COLUMN profit_sharing_returns.return_mchid IS '回退接收方商户号（平台/运营商）';
COMMENT ON COLUMN profit_sharing_returns.status IS '回退状态：pending/processing/success/failed';
