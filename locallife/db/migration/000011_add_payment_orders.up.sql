-- M7.5: 微信支付服务
-- 支持微信小程序支付（押金、充值等）和微信收付通分账（商户交易）
-- 退款需区分支付类型：miniprogram走小程序退款，profit_sharing走收付通退款

-- 运营商表（每个区县一个运营商）
CREATE TABLE operators (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    region_id BIGINT UNIQUE NOT NULL REFERENCES regions(id),
    
    name TEXT NOT NULL,
    contact_name TEXT NOT NULL,
    contact_phone TEXT NOT NULL,
    
    -- 微信支付分账信息
    wechat_mch_id TEXT,
    
    -- 分账比例配置（支持灵活调整）
    commission_rate DECIMAL(5,4) NOT NULL DEFAULT 0.0300,
    
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT operators_status_check CHECK (status IN ('active', 'suspended')),
    CONSTRAINT operators_commission_rate_check CHECK (commission_rate >= 0 AND commission_rate <= 1)
);

-- 索引
CREATE UNIQUE INDEX operators_user_id_idx ON operators(user_id);
CREATE UNIQUE INDEX operators_region_id_idx ON operators(region_id);
CREATE INDEX operators_wechat_mch_id_idx ON operators(wechat_mch_id);

-- 支付订单表
CREATE TABLE payment_orders (
    id BIGSERIAL PRIMARY KEY,
    
    -- 关联
    order_id BIGINT REFERENCES orders(id),
    reservation_id BIGINT REFERENCES table_reservations(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    
    -- 支付类型
    payment_type TEXT NOT NULL,
    business_type TEXT NOT NULL,
    
    -- 金额
    amount BIGINT NOT NULL,
    
    -- 微信支付信息
    out_trade_no TEXT UNIQUE NOT NULL,
    transaction_id TEXT,
    prepay_id TEXT,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- 时间戳
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT payment_orders_payment_type_check CHECK (payment_type IN ('miniprogram', 'profit_sharing')),
    CONSTRAINT payment_orders_business_type_check CHECK (business_type IN ('order', 'deposit', 'recharge', 'reservation')),
    CONSTRAINT payment_orders_status_check CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'closed')),
    CONSTRAINT payment_orders_amount_check CHECK (amount > 0)
);

-- 索引
CREATE INDEX payment_orders_order_id_idx ON payment_orders(order_id);
CREATE INDEX payment_orders_reservation_id_idx ON payment_orders(reservation_id);
CREATE INDEX payment_orders_user_id_idx ON payment_orders(user_id);
CREATE UNIQUE INDEX payment_orders_out_trade_no_idx ON payment_orders(out_trade_no);
CREATE INDEX payment_orders_transaction_id_idx ON payment_orders(transaction_id);
CREATE INDEX payment_orders_status_idx ON payment_orders(status);
CREATE INDEX payment_orders_created_at_idx ON payment_orders(created_at);

-- 收付通分账订单表
-- 分账规则：
-- - 外卖/预定：platform_commission=2%, operator_commission=3%, merchant_amount=95%
-- - 堂食/打包：platform_commission=0, operator_commission=0, merchant_amount=100%
CREATE TABLE profit_sharing_orders (
    id BIGSERIAL PRIMARY KEY,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    
    -- 关联
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    operator_id BIGINT REFERENCES operators(id),
    
    -- 订单来源（决定分账比例）
    order_source TEXT NOT NULL,
    
    -- 分账金额（单位：分）
    total_amount BIGINT NOT NULL,
    platform_commission BIGINT NOT NULL DEFAULT 0,
    operator_commission BIGINT NOT NULL DEFAULT 0,
    merchant_amount BIGINT NOT NULL,
    
    -- 微信分账信息
    out_order_no TEXT UNIQUE NOT NULL,
    sharing_order_id TEXT,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT profit_sharing_orders_order_source_check CHECK (order_source IN ('takeout', 'dine_in', 'takeaway', 'reservation')),
    CONSTRAINT profit_sharing_orders_status_check CHECK (status IN ('pending', 'processing', 'finished', 'failed')),
    CONSTRAINT profit_sharing_orders_amount_check CHECK (total_amount > 0),
    CONSTRAINT profit_sharing_orders_commission_check CHECK (platform_commission >= 0 AND operator_commission >= 0 AND merchant_amount >= 0)
);

-- 索引
CREATE INDEX profit_sharing_orders_payment_order_id_idx ON profit_sharing_orders(payment_order_id);
CREATE INDEX profit_sharing_orders_merchant_id_idx ON profit_sharing_orders(merchant_id);
CREATE INDEX profit_sharing_orders_operator_id_idx ON profit_sharing_orders(operator_id);
CREATE INDEX profit_sharing_orders_status_idx ON profit_sharing_orders(status);
CREATE UNIQUE INDEX profit_sharing_orders_out_order_no_idx ON profit_sharing_orders(out_order_no);

-- 退款订单表
-- 退款类型区分：
-- - miniprogram：小程序支付退款（押金、充值等）
-- - profit_sharing：收付通支付退款（订单支付），需要从各方账户扣回
CREATE TABLE refund_orders (
    id BIGSERIAL PRIMARY KEY,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    
    -- 退款类型（继承自payment_orders.payment_type）
    refund_type TEXT NOT NULL,
    
    -- 退款信息
    refund_amount BIGINT NOT NULL,
    refund_reason TEXT,
    
    -- 微信退款信息
    out_refund_no TEXT UNIQUE NOT NULL,
    refund_id TEXT,
    
    -- 收付通退款时的分账回退信息
    platform_refund BIGINT,
    operator_refund BIGINT,
    merchant_refund BIGINT,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- 时间戳
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT refund_orders_refund_type_check CHECK (refund_type IN ('miniprogram', 'profit_sharing')),
    CONSTRAINT refund_orders_status_check CHECK (status IN ('pending', 'processing', 'success', 'failed', 'closed')),
    CONSTRAINT refund_orders_refund_amount_check CHECK (refund_amount > 0)
);

-- 索引
CREATE INDEX refund_orders_payment_order_id_idx ON refund_orders(payment_order_id);
CREATE INDEX refund_orders_refund_type_idx ON refund_orders(refund_type);
CREATE UNIQUE INDEX refund_orders_out_refund_no_idx ON refund_orders(out_refund_no);
CREATE INDEX refund_orders_refund_id_idx ON refund_orders(refund_id);
CREATE INDEX refund_orders_status_idx ON refund_orders(status);
