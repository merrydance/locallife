CREATE TABLE IF NOT EXISTS trust_score_changes (
    id BIGSERIAL PRIMARY KEY,

    -- 角色关联
    entity_type TEXT NOT NULL CHECK (entity_type IN ('customer', 'merchant', 'rider')),
    entity_id BIGINT NOT NULL,

    -- 分数变化
    old_score SMALLINT NOT NULL,
    new_score SMALLINT NOT NULL,
    score_change SMALLINT NOT NULL,

    -- 变更原因
    reason_type TEXT NOT NULL,
    reason_description TEXT NOT NULL,

    -- 关联事件
    related_type TEXT,
    related_id BIGINT,

    -- 变更元数据
    is_auto BOOLEAN NOT NULL DEFAULT TRUE,
    operator_id BIGINT,

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
