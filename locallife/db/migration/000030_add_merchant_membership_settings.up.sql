-- 商户会员使用场景配置表
-- 控制会员余额/赠送金额在哪些场景下可用

CREATE TABLE IF NOT EXISTS merchant_membership_settings (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE UNIQUE,
    
    -- 余额可用场景 (JSON数组): ["dine_in", "takeout", "reservation"]
    balance_usable_scenes TEXT[] NOT NULL DEFAULT ARRAY['dine_in', 'takeout', 'reservation'],
    
    -- 赠送金额可用场景 (可比余额更严格)
    bonus_usable_scenes TEXT[] NOT NULL DEFAULT ARRAY['dine_in'],
    
    -- 是否允许与优惠券叠加使用
    allow_with_voucher BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- 是否允许与满减活动叠加使用
    allow_with_discount BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- 单笔订单最大抵扣比例 (1-100, 100表示可全额抵扣)
    max_deduction_percent INT NOT NULL DEFAULT 100 CHECK (max_deduction_percent >= 1 AND max_deduction_percent <= 100),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

-- 索引
CREATE INDEX idx_merchant_membership_settings_merchant ON merchant_membership_settings(merchant_id);

-- 触发器：更新 updated_at
CREATE TRIGGER update_merchant_membership_settings_updated_at 
BEFORE UPDATE ON merchant_membership_settings
FOR EACH ROW 
EXECUTE FUNCTION update_updated_at_column();

-- 注释
COMMENT ON TABLE merchant_membership_settings IS '商户会员使用场景配置';
COMMENT ON COLUMN merchant_membership_settings.balance_usable_scenes IS '余额可用场景: dine_in(堂食), takeout(外卖), reservation(预定)';
COMMENT ON COLUMN merchant_membership_settings.bonus_usable_scenes IS '赠送金额可用场景，可比余额更严格';
COMMENT ON COLUMN merchant_membership_settings.allow_with_voucher IS '是否允许与优惠券叠加';
COMMENT ON COLUMN merchant_membership_settings.allow_with_discount IS '是否允许与满减叠加';
COMMENT ON COLUMN merchant_membership_settings.max_deduction_percent IS '单笔最大抵扣比例(1-100)';
