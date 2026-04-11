-- Behavior trace decision system (production-ready schema)

CREATE TABLE IF NOT EXISTS platform_configs (
    id BIGSERIAL PRIMARY KEY,
    config_key TEXT NOT NULL,
    config_value JSONB NOT NULL,
    scope_type TEXT NOT NULL DEFAULT 'global' CHECK (scope_type IN ('global', 'city', 'merchant')),
    scope_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_platform_config UNIQUE (config_key, scope_type, scope_id)
);

CREATE INDEX idx_platform_configs_key ON platform_configs(config_key);
CREATE INDEX idx_platform_configs_scope ON platform_configs(scope_type, scope_id);

CREATE TABLE IF NOT EXISTS behavior_decisions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    rider_id BIGINT REFERENCES riders(id) ON DELETE SET NULL,
    decision_version TEXT NOT NULL,
    reason_codes TEXT[] NOT NULL DEFAULT '{}',
    responsible_party TEXT NOT NULL CHECK (responsible_party IN ('merchant', 'rider', 'user', 'unknown')),
    compensation_source TEXT NOT NULL CHECK (compensation_source IN ('merchant', 'rider', 'platform', 'unknown')),
    decision_status TEXT NOT NULL CHECK (decision_status IN ('pending', 'decided', 'executed', 'archived')),
    trace_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_behavior_decisions_order_id ON behavior_decisions(order_id);
CREATE INDEX idx_behavior_decisions_user_id ON behavior_decisions(user_id);
CREATE INDEX idx_behavior_decisions_merchant_id ON behavior_decisions(merchant_id);
CREATE INDEX idx_behavior_decisions_rider_id ON behavior_decisions(rider_id);
CREATE INDEX idx_behavior_decisions_status ON behavior_decisions(decision_status);

CREATE TABLE IF NOT EXISTS behavior_evidence (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id) ON DELETE CASCADE,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('image', 'text', 'location', 'device', 'address', 'ip', 'user_agent', 'other')),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_behavior_evidence_decision_id ON behavior_evidence(decision_id);

CREATE TABLE IF NOT EXISTS behavior_trace_snapshots (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id) ON DELETE CASCADE,
    window_days INTEGER NOT NULL,
    abnormal_count INTEGER NOT NULL,
    total_count INTEGER NOT NULL,
    abnormal_rate NUMERIC(6, 4) NOT NULL,
    association_hits TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_behavior_trace_snapshots_decision_id ON behavior_trace_snapshots(decision_id);

CREATE TABLE IF NOT EXISTS behavior_actions (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL CHECK (action_type IN ('block', 'refund', 'notify', 'observe')),
    target_entity TEXT NOT NULL CHECK (target_entity IN ('merchant', 'rider', 'user', 'platform')),
    status TEXT NOT NULL CHECK (status IN ('created', 'running', 'success', 'failed')),
    detail JSONB,
    executed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_behavior_actions_decision_id ON behavior_actions(decision_id);
CREATE INDEX idx_behavior_actions_status ON behavior_actions(status);

CREATE TABLE IF NOT EXISTS behavior_appeals (
    id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('merchant', 'rider')),
    entity_id BIGINT NOT NULL,
    decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    reason TEXT NOT NULL,
    evidence TEXT,
    status TEXT NOT NULL CHECK (status IN ('submitted', 'reevaluating', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reeval_at TIMESTAMPTZ
);

CREATE INDEX idx_behavior_appeals_entity ON behavior_appeals(entity_type, entity_id);
CREATE INDEX idx_behavior_appeals_status ON behavior_appeals(status);

CREATE TABLE IF NOT EXISTS behavior_blocklist (
    id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('user')),
    entity_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason_code TEXT NOT NULL,
    block_until TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('active', 'expired', 'removed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_behavior_blocklist_active ON behavior_blocklist(entity_type, entity_id)
WHERE status = 'active';

CREATE INDEX idx_behavior_blocklist_status ON behavior_blocklist(status);
