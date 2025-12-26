-- 用户余额账户
-- 设计理念：
-- 1. 用于接收索赔退款、活动奖励等
-- 2. 可用于下单抵扣
-- 3. 支持提现到微信零钱（后续实现）

CREATE TABLE IF NOT EXISTS user_balances (
    user_id BIGINT PRIMARY KEY REFERENCES users(id),
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),  -- 可用余额（分）
    frozen_balance BIGINT NOT NULL DEFAULT 0 CHECK (frozen_balance >= 0),  -- 冻结余额（提现中）
    total_income BIGINT NOT NULL DEFAULT 0,  -- 累计收入（分）
    total_expense BIGINT NOT NULL DEFAULT 0,  -- 累计支出（分）
    total_withdraw BIGINT NOT NULL DEFAULT 0,  -- 累计提现（分）
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 余额变动日志
CREATE TABLE IF NOT EXISTS user_balance_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    
    -- 变动类型
    type VARCHAR(32) NOT NULL,  -- claim_refund(索赔退款), order_pay(订单支付), withdraw(提现), recharge(充值), adjustment(调整)
    
    -- 金额信息
    amount BIGINT NOT NULL,  -- 变动金额（正数入账，负数出账）
    balance_before BIGINT NOT NULL,  -- 变动前余额
    balance_after BIGINT NOT NULL,  -- 变动后余额
    
    -- 关联信息
    related_type VARCHAR(32),  -- claim, order, withdraw_request 等
    related_id BIGINT,  -- 关联ID
    
    -- 来源信息（索赔退款场景）
    source_type VARCHAR(32),  -- rider_deposit(骑手押金), merchant_refund(商户退款), platform(平台垫付)
    source_id BIGINT,  -- 来源ID（如骑手ID、商户ID）
    
    remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_user_balance_logs_user_id ON user_balance_logs(user_id);
CREATE INDEX idx_user_balance_logs_type ON user_balance_logs(type);
CREATE INDEX idx_user_balance_logs_related ON user_balance_logs(related_type, related_id);
CREATE INDEX idx_user_balance_logs_created_at ON user_balance_logs(created_at);

-- 添加注释
COMMENT ON TABLE user_balances IS '用户余额账户';
COMMENT ON COLUMN user_balances.balance IS '可用余额（分）';
COMMENT ON COLUMN user_balances.frozen_balance IS '冻结余额，提现处理中（分）';
COMMENT ON TABLE user_balance_logs IS '用户余额变动日志';
COMMENT ON COLUMN user_balance_logs.type IS '变动类型：claim_refund/order_pay/withdraw/recharge/adjustment';
COMMENT ON COLUMN user_balance_logs.source_type IS '资金来源：rider_deposit/merchant_refund/platform';
