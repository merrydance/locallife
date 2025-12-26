-- M9: TrustScore信任分系统（信用驱动，非证据驱动）
-- 设计理念：
--   1. 人性本善，初始高信任（850分）
--   2. 只有负面行为才扣分
--   3. 信用分驱动异常处理，无需证据（食安除外）
--   4. 回溯检查决定是否自动通过
--   5. 团伙欺诈纯规则引擎检测（无AI）

-- 用户信任画像表
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

CREATE INDEX idx_user_profiles_user_id ON user_profiles(user_id);
CREATE INDEX idx_user_profiles_trust_score ON user_profiles(trust_score);
CREATE INDEX idx_user_profiles_blacklisted ON user_profiles(is_blacklisted);

-- 商户信任画像表
CREATE TABLE IF NOT EXISTS merchant_profiles (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL UNIQUE REFERENCES merchants(id) ON DELETE CASCADE,
    
    -- TrustScore信任分（初始850分）
    trust_score SMALLINT NOT NULL DEFAULT 850 CHECK (trust_score >= 300 AND trust_score <= 850),
    
    -- 经营统计（全局）
    total_orders INTEGER NOT NULL DEFAULT 0,
    total_sales BIGINT NOT NULL DEFAULT 0, -- 总销售额（分）
    completed_orders INTEGER NOT NULL DEFAULT 0,
    
    -- 负面事件统计（全局累计）
    total_claims INTEGER NOT NULL DEFAULT 0,
    foreign_object_claims INTEGER NOT NULL DEFAULT 0, -- 异物索赔次数
    food_safety_incidents INTEGER NOT NULL DEFAULT 0, -- 食安事件次数
    timeout_count INTEGER NOT NULL DEFAULT 0, -- 超时出餐次数
    refuse_order_count INTEGER NOT NULL DEFAULT 0, -- 拒单次数
    
    -- 多时间窗口统计
    recent_7d_claims INTEGER NOT NULL DEFAULT 0,
    recent_7d_incidents INTEGER NOT NULL DEFAULT 0,
    
    recent_30d_claims INTEGER NOT NULL DEFAULT 0,
    recent_30d_incidents INTEGER NOT NULL DEFAULT 0,
    recent_30d_timeouts INTEGER NOT NULL DEFAULT 0,
    
    recent_90d_claims INTEGER NOT NULL DEFAULT 0,
    recent_90d_incidents INTEGER NOT NULL DEFAULT 0,
    
    -- 熔断状态
    is_suspended BOOLEAN NOT NULL DEFAULT FALSE,
    suspend_reason TEXT,
    suspended_at TIMESTAMPTZ,
    suspend_until TIMESTAMPTZ,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE merchant_profiles IS '商户信任画像表 - 信用分驱动食安熔断';
COMMENT ON COLUMN merchant_profiles.trust_score IS '商户信任分，400以下停业整顿';
COMMENT ON COLUMN merchant_profiles.foreign_object_claims IS '异物索赔：一周3次触发限流';
COMMENT ON COLUMN merchant_profiles.food_safety_incidents IS '食安事件：需整改和人工审核';
COMMENT ON COLUMN merchant_profiles.is_suspended IS '是否已熔断（食安事件/信用分过低）';

CREATE INDEX idx_merchant_profiles_trust_score ON merchant_profiles(trust_score);
CREATE INDEX idx_merchant_profiles_suspended ON merchant_profiles(is_suspended);

-- 骑手信任画像表
CREATE TABLE IF NOT EXISTS rider_profiles (
    id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL UNIQUE REFERENCES riders(id) ON DELETE CASCADE,
    
    -- TrustScore信任分（初始850分）
    trust_score SMALLINT NOT NULL DEFAULT 850 CHECK (trust_score >= 300 AND trust_score <= 850),
    
    -- 配送统计（全局）
    total_deliveries INTEGER NOT NULL DEFAULT 0,
    completed_deliveries INTEGER NOT NULL DEFAULT 0,
    on_time_deliveries INTEGER NOT NULL DEFAULT 0,
    delayed_deliveries INTEGER NOT NULL DEFAULT 0,
    cancelled_deliveries INTEGER NOT NULL DEFAULT 0,
    
    -- 负面事件统计（全局累计）
    total_damage_incidents INTEGER NOT NULL DEFAULT 0, -- 餐损事件总次数
    customer_complaints INTEGER NOT NULL DEFAULT 0, -- 顾客投诉次数
    timeout_incidents INTEGER NOT NULL DEFAULT 0, -- 严重超时次数
    
    -- 多时间窗口统计
    recent_7d_damages INTEGER NOT NULL DEFAULT 0,
    recent_7d_delays INTEGER NOT NULL DEFAULT 0,
    
    recent_30d_damages INTEGER NOT NULL DEFAULT 0,
    recent_30d_delays INTEGER NOT NULL DEFAULT 0,
    recent_30d_complaints INTEGER NOT NULL DEFAULT 0,
    
    recent_90d_damages INTEGER NOT NULL DEFAULT 0,
    recent_90d_delays INTEGER NOT NULL DEFAULT 0,
    
    -- 工作时长（小时）
    total_online_hours INTEGER NOT NULL DEFAULT 0,
    
    -- 暂停状态
    is_suspended BOOLEAN NOT NULL DEFAULT FALSE,
    suspend_reason TEXT,
    suspended_at TIMESTAMPTZ,
    suspend_until TIMESTAMPTZ,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE rider_profiles IS '骑手信任画像表 - 餐损索赔由押金扣除';
COMMENT ON COLUMN rider_profiles.trust_score IS '骑手信任分，350以下暂停接单';
COMMENT ON COLUMN rider_profiles.total_damage_incidents IS '餐损事件：一周3次触发扣分';
COMMENT ON COLUMN rider_profiles.is_suspended IS '是否暂停接单';

CREATE INDEX idx_rider_profiles_trust_score ON rider_profiles(trust_score);
CREATE INDEX idx_rider_profiles_suspended ON rider_profiles(is_suspended);

-- 索赔记录表（增强版 - 信用驱动）
CREATE TABLE IF NOT EXISTS claims (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 索赔类型
    claim_type TEXT NOT NULL CHECK (claim_type IN ('foreign-object', 'damage', 'delay', 'quality', 'missing-item', 'other')),
    description TEXT NOT NULL,
    evidence_urls TEXT[], -- 证据图片URLs（仅食安必需）
    
    -- 金额
    claim_amount BIGINT NOT NULL, -- 申请索赔金额（分）
    approved_amount BIGINT, -- 实际赔付金额（分）
    
    -- 审核状态和决策
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'auto-approved', 'manual-review', 'approved', 'rejected')),
    approval_type TEXT CHECK (approval_type IN ('instant', 'auto', 'manual')),
    -- instant: 秒赔（高信用+小额）
    -- auto: 回溯检查自动通过
    -- manual: 人工审核（低信用）
    
    -- 信用驱动决策
    trust_score_snapshot SMALLINT, -- 用户提交索赔时的信用分快照
    is_malicious BOOLEAN NOT NULL DEFAULT FALSE, -- 是否恶意索赔（回溯检查/团伙检测确认）
    
    -- 回溯检查结果
    lookback_result JSONB,
    -- 示例: {"period": "30d", "orders_checked": 5, "claims_found": 1, "merchants": [123,456], "riders": [789]}
    
    -- 自动审核依据
    auto_approval_reason TEXT,
    -- high_trust: 高信用秒赔
    -- lookback_passed: 回溯检查通过
    -- low_amount: 小额免证
    rejection_reason TEXT,
    
    -- 审核信息（仅低信用需要）
    reviewer_id BIGINT,
    review_notes TEXT,
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ
);

COMMENT ON TABLE claims IS '索赔记录表 - 信用驱动免证索赔';
COMMENT ON COLUMN claims.trust_score_snapshot IS '用户提交索赔时的信用分快照（决策依据）';
COMMENT ON COLUMN claims.lookback_result IS '回溯检查：最近5笔订单（30天→90天→1年）的索赔历史';
COMMENT ON COLUMN claims.approval_type IS 'instant=秒赔(>=750分+<=50元), auto=回溯通过, manual=人工审核';

CREATE INDEX idx_claims_order_id ON claims(order_id);
CREATE INDEX idx_claims_user_id ON claims(user_id);
CREATE INDEX idx_claims_status ON claims(status);
CREATE INDEX idx_claims_malicious ON claims(is_malicious);
CREATE INDEX idx_claims_created_at ON claims(created_at);
CREATE INDEX idx_claims_user_status ON claims(user_id, status);
CREATE INDEX idx_claims_user_created ON claims(user_id, created_at); -- 回溯检查专用
CREATE INDEX idx_claims_trust_snapshot ON claims(trust_score_snapshot);
CREATE INDEX idx_claims_approval_type ON claims(approval_type);

-- 食品安全事件表
CREATE TABLE IF NOT EXISTS food_safety_incidents (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 事件描述
    incident_type TEXT NOT NULL,
    description TEXT NOT NULL,
    evidence_urls TEXT[] NOT NULL, -- 食安必须有证据
    
    -- 快照数据（事故溯源）
    order_snapshot JSONB NOT NULL, -- 订单完整信息
    merchant_snapshot JSONB NOT NULL, -- 商户完整信息（菜单、当班员工）
    rider_snapshot JSONB, -- 骑手信息快照（配送路线、时间）
    
    -- 状态流转
    status TEXT NOT NULL DEFAULT 'reported' CHECK (status IN ('reported', 'investigating', 'merchant-suspended', 'resolved')),
    
    -- 处理结果
    investigation_report TEXT,
    resolution TEXT,
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

COMMENT ON TABLE food_safety_incidents IS '食品安全事件表 - 唯一需要证据的场景';
COMMENT ON COLUMN food_safety_incidents.evidence_urls IS '食安必须有证据（照片、描述）';
COMMENT ON COLUMN food_safety_incidents.order_snapshot IS '订单完整快照（所有字段）用于事故溯源';
COMMENT ON COLUMN food_safety_incidents.merchant_snapshot IS '商户快照（菜单、当班员工）';
COMMENT ON COLUMN food_safety_incidents.rider_snapshot IS '骑手快照（配送路线、时间）';
COMMENT ON COLUMN food_safety_incidents.status IS 'merchant-suspended: 商户已熔断，需整改和人工审核';

CREATE INDEX idx_food_safety_merchant_id ON food_safety_incidents(merchant_id);
CREATE INDEX idx_food_safety_user_id ON food_safety_incidents(user_id);
CREATE INDEX idx_food_safety_status ON food_safety_incidents(status);
CREATE INDEX idx_food_safety_created_at ON food_safety_incidents(created_at);
CREATE INDEX idx_food_safety_merchant_status ON food_safety_incidents(merchant_id, status);

-- 欺诈模式检测表（团伙欺诈、设备复用等 - 纯规则引擎）
CREATE TABLE IF NOT EXISTS fraud_patterns (
    id BIGSERIAL PRIMARY KEY,
    
    -- 模式类型
    pattern_type TEXT NOT NULL CHECK (pattern_type IN ('device-reuse', 'address-cluster', 'coordinated-claims', 'payment-link', 'time-anomaly')),
    
    -- 关联数据
    related_user_ids BIGINT[] NOT NULL,
    related_order_ids BIGINT[],
    related_claim_ids BIGINT[],
    
    -- 特征数据（欺诈证据）
    device_fingerprints TEXT[],
    address_ids BIGINT[],
    ip_addresses TEXT[],
    
    -- 模式描述
    pattern_description TEXT, -- 如：5个账号使用同一设备在1小时内下单
    match_count SMALLINT NOT NULL DEFAULT 1, -- 匹配规则数量（如：同设备+同地址=2）
    
    -- 审核状态
    is_confirmed BOOLEAN NOT NULL DEFAULT FALSE, -- 最终确认为欺诈团伙
    
    -- 审核信息（仅复杂情况需要）
    reviewer_id BIGINT,
    review_notes TEXT,
    action_taken TEXT, -- block_users=拉黑用户/return_funds=返还商户骑手损失/warning=警告
    
    -- 时间戳
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ
);

COMMENT ON TABLE fraud_patterns IS '欺诈模式检测表 - 纯规则引擎（设备指纹+地址聚类+协同索赔）';
COMMENT ON COLUMN fraud_patterns.pattern_type IS 'device-reuse: 同设备多账号, address-cluster: 同地址多账号, coordinated-claims: 协同索赔';
COMMENT ON COLUMN fraud_patterns.match_count IS '匹配规则数量（同设备+同地址=2），>=2确认为欺诈';
COMMENT ON COLUMN fraud_patterns.is_confirmed IS '确认后拉黑用户，平台返还商户/骑手损失';

CREATE INDEX idx_fraud_patterns_type ON fraud_patterns(pattern_type);
CREATE INDEX idx_fraud_patterns_detected ON fraud_patterns(detected_at);
CREATE INDEX idx_fraud_patterns_confirmed ON fraud_patterns(is_confirmed);
CREATE INDEX idx_fraud_patterns_match_count ON fraud_patterns(match_count);

-- TrustScore变更记录表（审计日志）
CREATE TABLE IF NOT EXISTS trust_score_changes (
    id BIGSERIAL PRIMARY KEY,
    
    -- 角色关联
    entity_type TEXT NOT NULL CHECK (entity_type IN ('customer', 'merchant', 'rider')),
    entity_id BIGINT NOT NULL,
    
    -- 分数变化
    old_score SMALLINT NOT NULL,
    new_score SMALLINT NOT NULL,
    score_change SMALLINT NOT NULL, -- 变化值，负数=扣分
    
    -- 变更原因
    reason_type TEXT NOT NULL, -- claim/cancel/timeout/damage/incident/fraud等
    reason_description TEXT NOT NULL,
    
    -- 关联事件
    related_type TEXT, -- claim/order/delivery/incident
    related_id BIGINT,
    
    -- 变更元数据
    is_auto BOOLEAN NOT NULL DEFAULT TRUE, -- 是否系统自动变更
    operator_id BIGINT, -- 操作人ID（人工调整时）
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE trust_score_changes IS 'TrustScore变更记录表（审计日志）- 所有信用分变更可追溯';
COMMENT ON COLUMN trust_score_changes.score_change IS '变化值：负数=扣分，正数=加分（第一版不实现加分）';
COMMENT ON COLUMN trust_score_changes.reason_type IS '变更原因类型：用于统计分析';
COMMENT ON COLUMN trust_score_changes.is_auto IS '系统自动变更 vs 人工调整';

CREATE INDEX idx_trust_score_changes_entity ON trust_score_changes(entity_type, entity_id);
CREATE INDEX idx_trust_score_changes_created ON trust_score_changes(created_at);
CREATE INDEX idx_trust_score_changes_reason ON trust_score_changes(reason_type);
CREATE INDEX idx_trust_score_changes_entity_created ON trust_score_changes(entity_type, entity_id, created_at);
