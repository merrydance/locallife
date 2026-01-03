-- Boss 店铺认领系统
-- Boss 是独立于 merchant_staff 的角色，可以认领多个店铺
-- 只有查看经营分析和管理员工的权限

-- 1. 创建 merchant_bosses 表
CREATE TABLE merchant_bosses (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    UNIQUE(user_id, merchant_id)
);

CREATE INDEX idx_merchant_bosses_user ON merchant_bosses(user_id);
CREATE INDEX idx_merchant_bosses_merchant ON merchant_bosses(merchant_id);

COMMENT ON TABLE merchant_bosses IS 'Boss 店铺认领关系表 - Boss 可以认领多个店铺，只有分析和员工管理权限';
COMMENT ON COLUMN merchant_bosses.status IS '状态: active=有效, disabled=已解除';

-- 2. 商户表添加 Boss 认领码字段
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS boss_bind_code VARCHAR(32);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS boss_bind_code_expires_at TIMESTAMPTZ;

COMMENT ON COLUMN merchants.boss_bind_code IS 'Boss 认领码';
COMMENT ON COLUMN merchants.boss_bind_code_expires_at IS 'Boss 认领码过期时间';
