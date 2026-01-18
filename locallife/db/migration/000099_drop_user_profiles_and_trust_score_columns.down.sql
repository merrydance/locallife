-- Restore TrustScore user profiles and trust_score columns

CREATE TABLE IF NOT EXISTS user_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('customer', 'merchant', 'rider')),

    -- TrustScore信任分（初始850分，范围300-850）
    -- 设计理念：人性本善，初始高信任，只有负面行为才扣分
    trust_score SMALLINT NOT NULL DEFAULT 850 CHECK (trust_score >= 300 AND trust_score <= 850),

    -- 订单统计（全局）
    total_orders INTEGER NOT NULL DEFAULT 0,
    completed_orders INTEGER NOT NULL DEFAULT 0,
    cancelled_orders INTEGER NOT NULL DEFAULT 0,

    -- 负面行为统计（全局累计）
    total_claims INTEGER NOT NULL DEFAULT 0, -- 总索赔次数
    malicious_claims INTEGER NOT NULL DEFAULT 0, -- 恶意索赔次数（已确认）
    food_safety_reports INTEGER NOT NULL DEFAULT 0, -- 食安上报次数
    verified_violations INTEGER NOT NULL DEFAULT 0, -- 已核实违规次数

    -- 多时间窗口统计（滚动窗口）
    recent_7d_claims INTEGER NOT NULL DEFAULT 0,
    recent_7d_orders INTEGER NOT NULL DEFAULT 0,

    recent_30d_claims INTEGER NOT NULL DEFAULT 0,
    recent_30d_orders INTEGER NOT NULL DEFAULT 0,
    recent_30d_cancels INTEGER NOT NULL DEFAULT 0,

    recent_90d_claims INTEGER NOT NULL DEFAULT 0,
    recent_90d_orders INTEGER NOT NULL DEFAULT 0,

    -- 状态标记
    is_blacklisted BOOLEAN NOT NULL DEFAULT FALSE,
    blacklist_reason TEXT,
    blacklisted_at TIMESTAMPTZ,

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_user_role UNIQUE (user_id, role)
);

COMMENT ON TABLE user_profiles IS '用户信任画像表（顾客）- 信用驱动异常处理';
COMMENT ON COLUMN user_profiles.trust_score IS '信任分，初始850（高信任），只有负面行为才扣分';
COMMENT ON COLUMN user_profiles.recent_7d_claims IS '近7天索赔次数（快速识别异常）';
COMMENT ON COLUMN user_profiles.recent_30d_claims IS '近30天索赔次数（回溯检查）';
COMMENT ON COLUMN user_profiles.recent_90d_claims IS '近90天索赔次数（长期趋势）';

CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON user_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_profiles_trust_score ON user_profiles(trust_score);
CREATE INDEX IF NOT EXISTS idx_user_profiles_blacklisted ON user_profiles(is_blacklisted);

ALTER TABLE merchant_profiles
ADD COLUMN IF NOT EXISTS trust_score SMALLINT NOT NULL DEFAULT 850 CHECK (trust_score >= 300 AND trust_score <= 850);

COMMENT ON COLUMN merchant_profiles.trust_score IS '商户信任分，400以下停业整顿';
CREATE INDEX IF NOT EXISTS idx_merchant_profiles_trust_score ON merchant_profiles(trust_score);

ALTER TABLE rider_profiles
ADD COLUMN IF NOT EXISTS trust_score SMALLINT NOT NULL DEFAULT 850 CHECK (trust_score >= 300 AND trust_score <= 850);

COMMENT ON COLUMN rider_profiles.trust_score IS '骑手信任分，350以下暂停接单';
CREATE INDEX IF NOT EXISTS idx_rider_profiles_trust_score ON rider_profiles(trust_score);
