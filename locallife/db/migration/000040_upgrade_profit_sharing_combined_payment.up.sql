-- 升级分账系统：支持配送费分账给骑手 + 合单支付
-- 分账公式：
--   骑手收入 = 配送费
--   可分账金额 = 支付总额 - 配送费
--   平台收入 = 可分账金额 × platform_rate%
--   运营商收入 = 可分账金额 × operator_rate%
--   商户收入 = 可分账金额 - 平台收入 - 运营商收入

-- 1. 升级 profit_sharing_orders 表，添加配送费和骑手分账字段
ALTER TABLE profit_sharing_orders 
    ADD COLUMN IF NOT EXISTS delivery_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_id BIGINT REFERENCES riders(id),
    ADD COLUMN IF NOT EXISTS rider_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS distributable_amount BIGINT,
    ADD COLUMN IF NOT EXISTS platform_rate INT NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS operator_rate INT NOT NULL DEFAULT 3;

-- 更新约束：骑手分账金额必须等于配送费
ALTER TABLE profit_sharing_orders 
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_rider_amount_check;
ALTER TABLE profit_sharing_orders 
    ADD CONSTRAINT profit_sharing_orders_rider_amount_check 
    CHECK (rider_amount >= 0 AND (rider_id IS NULL OR rider_amount = delivery_fee));

-- 添加骑手索引
CREATE INDEX IF NOT EXISTS profit_sharing_orders_rider_id_idx ON profit_sharing_orders(rider_id);

COMMENT ON COLUMN profit_sharing_orders.delivery_fee IS '配送费（分），外卖订单专用';
COMMENT ON COLUMN profit_sharing_orders.rider_id IS '骑手ID，配送订单关联';
COMMENT ON COLUMN profit_sharing_orders.rider_amount IS '骑手分账金额（分），等于配送费';
COMMENT ON COLUMN profit_sharing_orders.distributable_amount IS '可分账金额（分）= total_amount - delivery_fee';
COMMENT ON COLUMN profit_sharing_orders.platform_rate IS '平台分账比例（百分比），默认2%';
COMMENT ON COLUMN profit_sharing_orders.operator_rate IS '运营商分账比例（百分比），默认3%';


-- 2. 创建合单支付主表
CREATE TABLE IF NOT EXISTS combined_payment_orders (
    id BIGSERIAL PRIMARY KEY,
    
    -- 用户信息
    user_id BIGINT NOT NULL REFERENCES users(id),
    
    -- 合单商户订单号（微信合单支付使用）
    combine_out_trade_no TEXT UNIQUE NOT NULL,
    
    -- 合计金额（所有子订单金额之和）
    total_amount BIGINT NOT NULL,
    
    -- 微信支付信息
    prepay_id TEXT,
    transaction_id TEXT,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- 时间戳
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT combined_payment_orders_status_check 
        CHECK (status IN ('pending', 'paid', 'failed', 'closed')),
    CONSTRAINT combined_payment_orders_amount_check 
        CHECK (total_amount > 0)
);

-- 索引
CREATE INDEX combined_payment_orders_user_id_idx ON combined_payment_orders(user_id);
CREATE INDEX combined_payment_orders_status_idx ON combined_payment_orders(status);
CREATE INDEX combined_payment_orders_created_at_idx ON combined_payment_orders(created_at);

COMMENT ON TABLE combined_payment_orders IS '合单支付主表，支持多商户一次支付';
COMMENT ON COLUMN combined_payment_orders.combine_out_trade_no IS '合单商户订单号，微信合单支付接口使用';
COMMENT ON COLUMN combined_payment_orders.total_amount IS '合计支付金额（分），所有子订单之和';


-- 3. 创建合单支付子订单表（每个商户一个子单）
CREATE TABLE IF NOT EXISTS combined_payment_sub_orders (
    id BIGSERIAL PRIMARY KEY,
    
    -- 关联合单主表
    combined_payment_id BIGINT NOT NULL REFERENCES combined_payment_orders(id) ON DELETE CASCADE,
    
    -- 关联订单和商户
    order_id BIGINT NOT NULL REFERENCES orders(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    
    -- 微信子商户信息
    sub_mchid TEXT NOT NULL,
    
    -- 子单金额
    amount BIGINT NOT NULL,
    
    -- 子单商户订单号
    out_trade_no TEXT UNIQUE NOT NULL,
    
    -- 子单描述
    description TEXT NOT NULL,
    
    -- 子单状态（跟随主单，但可能有独立的分账状态）
    profit_sharing_status TEXT NOT NULL DEFAULT 'pending',
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT combined_payment_sub_orders_amount_check CHECK (amount > 0),
    CONSTRAINT combined_payment_sub_orders_profit_sharing_status_check 
        CHECK (profit_sharing_status IN ('pending', 'processing', 'finished', 'failed'))
);

-- 索引
CREATE INDEX combined_payment_sub_orders_combined_id_idx ON combined_payment_sub_orders(combined_payment_id);
CREATE INDEX combined_payment_sub_orders_order_id_idx ON combined_payment_sub_orders(order_id);
CREATE INDEX combined_payment_sub_orders_merchant_id_idx ON combined_payment_sub_orders(merchant_id);

COMMENT ON TABLE combined_payment_sub_orders IS '合单支付子订单表，每个商户一个子单';
COMMENT ON COLUMN combined_payment_sub_orders.sub_mchid IS '微信子商户号';
COMMENT ON COLUMN combined_payment_sub_orders.out_trade_no IS '子单商户订单号，对应微信的sub_out_trade_no';


-- 4. 扩展 payment_orders 表，支持关联合单支付
ALTER TABLE payment_orders 
    ADD COLUMN IF NOT EXISTS combined_payment_id BIGINT REFERENCES combined_payment_orders(id);

CREATE INDEX IF NOT EXISTS payment_orders_combined_payment_id_idx ON payment_orders(combined_payment_id);

COMMENT ON COLUMN payment_orders.combined_payment_id IS '关联的合单支付ID，单商户支付时为NULL';


-- 5. 更新现有分账数据的 distributable_amount（可分账金额 = 总金额 - 配送费）
UPDATE profit_sharing_orders 
SET distributable_amount = total_amount - delivery_fee
WHERE distributable_amount IS NULL;

-- 设置 distributable_amount 非空约束
ALTER TABLE profit_sharing_orders 
    ALTER COLUMN distributable_amount SET NOT NULL;
ALTER TABLE profit_sharing_orders 
    ALTER COLUMN distributable_amount SET DEFAULT 0;
