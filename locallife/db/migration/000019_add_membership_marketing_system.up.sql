-- M10: 商户会员与营销系统

-- 创建自动更新 updated_at 的触发器函数（如果不存在）
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 商户会员表
CREATE TABLE IF NOT EXISTS merchant_memberships (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 余额信息（单位：分）
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    total_recharged BIGINT NOT NULL DEFAULT 0 CHECK (total_recharged >= 0),
    total_consumed BIGINT NOT NULL DEFAULT 0 CHECK (total_consumed >= 0),
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    
    -- 约束：一个用户在一个商户只能有一个会员账户
    CONSTRAINT unique_merchant_user UNIQUE (merchant_id, user_id)
);

-- 索引
CREATE INDEX idx_merchant_memberships_merchant_id ON merchant_memberships(merchant_id);
CREATE INDEX idx_merchant_memberships_user_id ON merchant_memberships(user_id);
CREATE INDEX idx_merchant_memberships_balance ON merchant_memberships(balance);

-- 触发器：更新 updated_at
CREATE TRIGGER update_merchant_memberships_updated_at 
BEFORE UPDATE ON merchant_memberships
FOR EACH ROW 
EXECUTE FUNCTION update_updated_at_column();

-- 充值规则表（充100送20等）
CREATE TABLE IF NOT EXISTS recharge_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    
    -- 充值规则
    recharge_amount BIGINT NOT NULL CHECK (recharge_amount > 0),
    bonus_amount BIGINT NOT NULL CHECK (bonus_amount >= 0),
    
    -- 状态与有效期
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    
    -- 约束：有效期检查
    CONSTRAINT check_valid_period CHECK (valid_until > valid_from)
);

-- 索引
CREATE INDEX idx_recharge_rules_merchant_id ON recharge_rules(merchant_id);
CREATE INDEX idx_recharge_rules_merchant_active ON recharge_rules(merchant_id, is_active);
CREATE INDEX idx_recharge_rules_valid_period ON recharge_rules(valid_from, valid_until);

-- 触发器：更新 updated_at
CREATE TRIGGER update_recharge_rules_updated_at 
BEFORE UPDATE ON recharge_rules
FOR EACH ROW 
EXECUTE FUNCTION update_updated_at_column();

-- 会员交易流水表
CREATE TABLE IF NOT EXISTS membership_transactions (
    id BIGSERIAL PRIMARY KEY,
    membership_id BIGINT NOT NULL REFERENCES merchant_memberships(id) ON DELETE CASCADE,
    
    -- 交易类型与金额
    type TEXT NOT NULL CHECK (type IN ('recharge', 'consume', 'refund', 'bonus')),
    amount BIGINT NOT NULL,
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    
    -- 关联信息
    related_order_id BIGINT,
    recharge_rule_id BIGINT REFERENCES recharge_rules(id) ON DELETE SET NULL,
    
    -- 备注
    notes TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_membership_transactions_membership_id ON membership_transactions(membership_id);
CREATE INDEX idx_membership_transactions_membership_created ON membership_transactions(membership_id, created_at DESC);
CREATE INDEX idx_membership_transactions_type ON membership_transactions(type);
CREATE INDEX idx_membership_transactions_created_at ON membership_transactions(created_at);

-- 代金券模板表
CREATE TABLE IF NOT EXISTS vouchers (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    
    -- 券码与名称
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    
    -- 金额与门槛
    amount BIGINT NOT NULL CHECK (amount > 0),
    min_order_amount BIGINT NOT NULL DEFAULT 0 CHECK (min_order_amount >= 0),
    
    -- 库存
    total_quantity INT NOT NULL CHECK (total_quantity > 0),
    claimed_quantity INT NOT NULL DEFAULT 0 CHECK (claimed_quantity >= 0),
    used_quantity INT NOT NULL DEFAULT 0 CHECK (used_quantity >= 0),
    
    -- 有效期
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL,
    
    -- 状态
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT check_voucher_valid_period CHECK (valid_until > valid_from),
    CONSTRAINT check_voucher_quantities CHECK (
        claimed_quantity <= total_quantity AND 
        used_quantity <= claimed_quantity
    )
);

-- 索引
CREATE INDEX idx_vouchers_merchant_id ON vouchers(merchant_id);
CREATE UNIQUE INDEX idx_vouchers_code ON vouchers(code);
CREATE INDEX idx_vouchers_merchant_active ON vouchers(merchant_id, is_active);
CREATE INDEX idx_vouchers_valid_period ON vouchers(valid_from, valid_until);

-- 触发器：更新 updated_at
CREATE TRIGGER update_vouchers_updated_at 
BEFORE UPDATE ON vouchers
FOR EACH ROW 
EXECUTE FUNCTION update_updated_at_column();

-- 用户代金券表
CREATE TABLE IF NOT EXISTS user_vouchers (
    id BIGSERIAL PRIMARY KEY,
    voucher_id BIGINT NOT NULL REFERENCES vouchers(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 使用状态
    status TEXT NOT NULL DEFAULT 'unused' CHECK (status IN ('unused', 'used', 'expired')),
    
    -- 使用信息
    order_id BIGINT,
    used_at TIMESTAMPTZ,
    
    -- 时间戳
    obtained_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- 索引
CREATE INDEX idx_user_vouchers_user_id ON user_vouchers(user_id);
CREATE INDEX idx_user_vouchers_voucher_id ON user_vouchers(voucher_id);
CREATE INDEX idx_user_vouchers_user_status ON user_vouchers(user_id, status);
CREATE INDEX idx_user_vouchers_voucher_status ON user_vouchers(voucher_id, status);
CREATE INDEX idx_user_vouchers_status ON user_vouchers(status);
CREATE INDEX idx_user_vouchers_expires_at ON user_vouchers(expires_at);

-- 满减规则表
CREATE TABLE IF NOT EXISTS discount_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    
    -- 规则信息
    name TEXT NOT NULL,
    description TEXT,
    
    -- 满减条件
    min_order_amount BIGINT NOT NULL CHECK (min_order_amount > 0),
    discount_amount BIGINT NOT NULL CHECK (discount_amount > 0),
    
    -- 叠加规则
    can_stack_with_voucher BOOLEAN NOT NULL DEFAULT FALSE,
    can_stack_with_membership BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- 有效期
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL,
    
    -- 状态
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT check_discount_valid_period CHECK (valid_until > valid_from),
    CONSTRAINT check_discount_amount_valid CHECK (discount_amount < min_order_amount)
);

-- 索引
CREATE INDEX idx_discount_rules_merchant_id ON discount_rules(merchant_id);
CREATE INDEX idx_discount_rules_merchant_active ON discount_rules(merchant_id, is_active);
CREATE INDEX idx_discount_rules_valid_period ON discount_rules(valid_from, valid_until);

-- 触发器：更新 updated_at
CREATE TRIGGER update_discount_rules_updated_at 
BEFORE UPDATE ON discount_rules
FOR EACH ROW 
EXECUTE FUNCTION update_updated_at_column();

-- 注释
COMMENT ON TABLE merchant_memberships IS 'M10: 商户会员账户表';
COMMENT ON TABLE recharge_rules IS 'M10: 充值规则表（充100送20等）';
COMMENT ON TABLE membership_transactions IS 'M10: 会员交易流水表';
COMMENT ON TABLE vouchers IS 'M10: 代金券模板表';
COMMENT ON TABLE user_vouchers IS 'M10: 用户代金券表';
COMMENT ON TABLE discount_rules IS 'M10: 满减规则表';
