-- 用户索赔警告状态表
-- 设计理念："你的行为决定你是谁"
-- 记录用户的索赔行为模式，用于判定是否需要提交证据

CREATE TABLE IF NOT EXISTS user_claim_warnings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    
    -- 警告状态
    warning_count INT NOT NULL DEFAULT 0,          -- 被警告次数
    last_warning_at TIMESTAMPTZ,                   -- 最后警告时间
    last_warning_reason TEXT,                      -- 最后警告原因
    
    -- 索赔限制
    requires_evidence BOOLEAN NOT NULL DEFAULT false, -- 是否需要提交证据
    platform_pay_count INT NOT NULL DEFAULT 0,     -- 平台垫付次数
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- 唯一约束：每个用户只有一条记录
    CONSTRAINT user_claim_warnings_user_id_unique UNIQUE (user_id)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_user_claim_warnings_user_id ON user_claim_warnings(user_id);
CREATE INDEX IF NOT EXISTS idx_user_claim_warnings_requires_evidence ON user_claim_warnings(requires_evidence) WHERE requires_evidence = true;

-- 注释
COMMENT ON TABLE user_claim_warnings IS '用户索赔警告状态表，记录用户的索赔行为模式';
COMMENT ON COLUMN user_claim_warnings.warning_count IS '被警告次数：5单3索赔首次警告+1';
COMMENT ON COLUMN user_claim_warnings.requires_evidence IS '是否需要提交证据：被警告后再次索赔时需要';
COMMENT ON COLUMN user_claim_warnings.platform_pay_count IS '平台垫付次数：问题用户的索赔由平台承担';
