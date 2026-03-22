-- 微信支付用户投诉表
-- 对应微信支付「用户投诉 v2」API：/v3/merchant-service/complaints-v2
-- 平台有义务在规定时效内回复并完结投诉，否则影响商户评级

CREATE TABLE wechat_complaints (
    id                  BIGSERIAL PRIMARY KEY,

    -- 微信支付投诉单号（全局唯一）
    complaint_id        TEXT NOT NULL UNIQUE,

    -- 投诉时间（由微信返回）
    complaint_time      TIMESTAMPTZ NOT NULL,

    -- 投诉人微信 openid（可能为空，部分场景微信不返回）
    payer_openid        TEXT,

    -- 投诉详情文本
    complaint_detail    TEXT NOT NULL DEFAULT '',

    -- 投诉状态（与微信侧同步）
    -- PENDING_RESPONSE: 待回复  PROCESSING: 处理中  PROCESSED: 已完结
    complaint_state     TEXT NOT NULL DEFAULT 'PENDING_RESPONSE'
                            CHECK (complaint_state IN (
                                'PENDING_RESPONSE',
                                'PROCESSING',
                                'PROCESSED'
                            )),

    -- 相关微信订单号（子单或合单主单）
    transaction_id      TEXT,

    -- 相关商户订单号
    out_trade_no        TEXT,

    -- 二级商户号（收付通场景）
    sub_mch_id          TEXT,

    -- 关联本地商户（可为空，入驻未完成时可能无关联）
    merchant_id         BIGINT REFERENCES merchants(id) ON DELETE SET NULL,

    -- 订单金额（分），方便展示
    payer_complaint_full_info BOOLEAN NOT NULL DEFAULT false,

    -- 投诉金额（分）
    amount              BIGINT NOT NULL DEFAULT 0,

    -- ==================== 我方处理记录 ====================

    -- 平台/商户最近一次回复内容
    response_content    TEXT,

    -- 最近一次回复时间
    responded_at        TIMESTAMPTZ,

    -- 上传给微信的凭证 media_id 列表（JSON 数组）
    media_ids           JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- 投诉完结时间
    completed_at        TIMESTAMPTZ,

    -- ==================== 同步信息 ====================

    -- 最近一次从微信同步的时间
    last_synced_at      TIMESTAMPTZ,

    -- 微信侧更新时间（用于增量同步判断）
    wxpay_update_time   TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ
);

CREATE INDEX idx_wechat_complaints_state       ON wechat_complaints(complaint_state);
CREATE INDEX idx_wechat_complaints_merchant    ON wechat_complaints(merchant_id);
CREATE INDEX idx_wechat_complaints_sub_mch_id  ON wechat_complaints(sub_mch_id);
CREATE INDEX idx_wechat_complaints_time        ON wechat_complaints(complaint_time DESC);
CREATE INDEX idx_wechat_complaints_out_trade   ON wechat_complaints(out_trade_no)
    WHERE out_trade_no IS NOT NULL;

COMMENT ON TABLE wechat_complaints IS '微信支付用户投诉记录；由每日同步任务从微信拉取，商户可通过平台回复并完结';
