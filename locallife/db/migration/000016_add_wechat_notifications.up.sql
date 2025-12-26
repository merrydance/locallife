-- 微信支付回调通知记录表（用于幂等性检查）
CREATE TABLE IF NOT EXISTS "wechat_notifications" (
    "id" VARCHAR(64) PRIMARY KEY,                    -- 微信通知ID（微信保证唯一）
    "event_type" VARCHAR(64) NOT NULL,               -- 事件类型（TRANSACTION.SUCCESS, REFUND.SUCCESS等）
    "resource_type" VARCHAR(64),                     -- 资源类型（encrypt-resource）
    "summary" TEXT,                                  -- 事件摘要
    "out_trade_no" VARCHAR(64),                      -- 商户订单号（用于快速查询）
    "transaction_id" VARCHAR(64),                    -- 微信支付订单号
    "processed_at" TIMESTAMP NOT NULL DEFAULT NOW(), -- 处理时间
    "created_at" TIMESTAMP NOT NULL DEFAULT NOW()    -- 创建时间
);

-- 索引：按创建时间查询（用于清理历史数据）
CREATE INDEX idx_wechat_notifications_created_at ON wechat_notifications(created_at);

-- 索引：按商户订单号查询
CREATE INDEX idx_wechat_notifications_out_trade_no ON wechat_notifications(out_trade_no) WHERE out_trade_no IS NOT NULL;

-- 索引：按事件类型查询
CREATE INDEX idx_wechat_notifications_event_type ON wechat_notifications(event_type);

-- 表注释
COMMENT ON TABLE wechat_notifications IS '微信支付回调通知记录表，用于防止重复处理（幂等性）';
COMMENT ON COLUMN wechat_notifications.id IS '微信通知ID，微信保证全局唯一';
COMMENT ON COLUMN wechat_notifications.event_type IS '通知事件类型';
COMMENT ON COLUMN wechat_notifications.out_trade_no IS '商户订单号，用于关联业务订单';
COMMENT ON COLUMN wechat_notifications.processed_at IS '通知处理完成时间';
