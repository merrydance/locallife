-- 多店铺员工管理系统
-- 支持一个老板管理多个店铺，邀请员工（店长、厨师长、收银员）

-- 1. 创建商户员工表
CREATE TABLE merchant_staff (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'manager', 'chef', 'cashier')),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    invited_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    UNIQUE(merchant_id, user_id)
);

CREATE INDEX idx_merchant_staff_merchant ON merchant_staff(merchant_id);
CREATE INDEX idx_merchant_staff_user ON merchant_staff(user_id);

COMMENT ON TABLE merchant_staff IS '商户员工表 - 管理商户与用户的关联关系及角色';
COMMENT ON COLUMN merchant_staff.role IS '员工角色: owner=店主, manager=店长, chef=厨师长, cashier=收银员';
COMMENT ON COLUMN merchant_staff.status IS '状态: active=启用, disabled=禁用';
COMMENT ON COLUMN merchant_staff.invited_by IS '邀请人（店主ID）';

-- 2. 商户表添加老板绑定相关字段（支持店长代入驻）
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS pending_owner_bind BOOLEAN DEFAULT FALSE;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS bind_code VARCHAR(32);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS bind_code_expires_at TIMESTAMPTZ;

COMMENT ON COLUMN merchants.pending_owner_bind IS '是否等待老板绑定（店长代入驻时为true）';
COMMENT ON COLUMN merchants.bind_code IS '老板绑定码';
COMMENT ON COLUMN merchants.bind_code_expires_at IS '绑定码过期时间';

-- 3. 为现有商户创建 owner 记录（数据迁移）
INSERT INTO merchant_staff (merchant_id, user_id, role, created_at)
SELECT id, owner_user_id, 'owner', created_at
FROM merchants
WHERE owner_user_id IS NOT NULL
ON CONFLICT (merchant_id, user_id) DO NOTHING;
