ALTER TABLE behavior_decisions
    ADD COLUMN claim_id BIGINT REFERENCES claims(id) ON DELETE SET NULL,
    ADD COLUMN decision_mode TEXT,
    ADD COLUMN responsibility_domain TEXT,
    ADD COLUMN payout_mode TEXT,
    ADD COLUMN effective_status TEXT NOT NULL DEFAULT 'effective',
    ADD COLUMN confidence_score INTEGER,
    ADD COLUMN user_risk_score INTEGER,
    ADD COLUMN merchant_liability_score INTEGER,
    ADD COLUMN rider_liability_score INTEGER,
    ADD COLUMN fallback_reason TEXT,
    ADD COLUMN restriction_reason TEXT,
    ADD COLUMN liability_shares JSONB,
    ADD COLUMN score_breakdown JSONB,
    ADD COLUMN graph_hits JSONB,
    ADD COLUMN fact_snapshot JSONB,
    ADD COLUMN supersedes_decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    ADD COLUMN overturned_by_decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    ADD COLUMN profile_effect_applied BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE behavior_decisions
    ADD CONSTRAINT behavior_decisions_decision_mode_check CHECK (
        decision_mode IS NULL OR decision_mode IN (
            'merchant_recovery',
            'rider_recovery',
            'platform_fallback',
            'user_restricted',
            'split'
        )
    ),
    ADD CONSTRAINT behavior_decisions_responsibility_domain_check CHECK (
        responsibility_domain IS NULL OR responsibility_domain IN (
            'merchant_domain',
            'rider_domain',
            'user_domain',
            'unknown'
        )
    ),
    ADD CONSTRAINT behavior_decisions_payout_mode_check CHECK (
        payout_mode IS NULL OR payout_mode IN (
            'instant_paid',
            'limited_paid',
            'rejected'
        )
    ),
    ADD CONSTRAINT behavior_decisions_effective_status_check CHECK (
        effective_status IN ('effective', 'overturned', 'superseded', 'archived')
    ),
    ADD CONSTRAINT behavior_decisions_mode_reason_check CHECK (
        (decision_mode IS DISTINCT FROM 'platform_fallback' OR fallback_reason IS NOT NULL)
        AND
        (decision_mode IS DISTINCT FROM 'user_restricted' OR restriction_reason IS NOT NULL)
    );

CREATE INDEX idx_behavior_decisions_claim_id ON behavior_decisions(claim_id);
CREATE UNIQUE INDEX idx_behavior_decisions_effective_claim_id ON behavior_decisions(claim_id)
WHERE claim_id IS NOT NULL AND effective_status = 'effective';
CREATE INDEX idx_behavior_decisions_mode_status ON behavior_decisions(decision_mode, effective_status, created_at DESC);

ALTER TABLE behavior_trace_snapshots
    ADD COLUMN actor_type TEXT,
    ADD COLUMN actor_id BIGINT,
    ADD COLUMN window_key TEXT,
    ADD COLUMN stats_scope TEXT,
    ADD COLUMN metric_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN association_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN snapshot_version TEXT;

ALTER TABLE behavior_trace_snapshots
    ADD CONSTRAINT behavior_trace_snapshots_actor_type_check CHECK (
        actor_type IS NULL OR actor_type IN ('user', 'merchant', 'rider')
    ),
    ADD CONSTRAINT behavior_trace_snapshots_window_key_check CHECK (
        window_key IS NULL OR window_key IN ('7d', '30d', '90d', 'lifetime')
    ),
    ADD CONSTRAINT behavior_trace_snapshots_stats_scope_check CHECK (
        stats_scope IS NULL OR stats_scope IN ('raw', 'net_effective')
    );

CREATE INDEX idx_behavior_trace_snapshots_actor_window ON behavior_trace_snapshots(actor_type, actor_id, window_key);

ALTER TABLE claim_recoveries
    ADD COLUMN decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    ADD COLUMN recovery_basis TEXT;

ALTER TABLE claim_recoveries
    ADD CONSTRAINT claim_recoveries_recovery_basis_check CHECK (
        recovery_basis IS NULL OR recovery_basis IN (
            'merchant_recovery',
            'rider_recovery',
            'platform_fallback',
            'user_restricted'
        )
    );

CREATE INDEX idx_claim_recoveries_decision_id ON claim_recoveries(decision_id);

CREATE TABLE claim_recovery_events (
    id BIGSERIAL PRIMARY KEY,
    recovery_id BIGINT NOT NULL REFERENCES claim_recoveries(id) ON DELETE CASCADE,
    decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('created', 'paid', 'waived', 'resumed', 'overturned')),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_claim_recovery_events_recovery_id ON claim_recovery_events(recovery_id);
CREATE INDEX idx_claim_recovery_events_decision_id ON claim_recovery_events(decision_id);

CREATE TABLE behavior_decision_effects (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('user', 'merchant', 'rider')),
    entity_id BIGINT NOT NULL,
    metric_key TEXT NOT NULL,
    delta_value BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'applied' CHECK (status IN ('applied', 'reverted')),
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reverted_at TIMESTAMPTZ,
    reverted_by_decision_id BIGINT REFERENCES behavior_decisions(id) ON DELETE SET NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_behavior_decision_effects_decision_id ON behavior_decision_effects(decision_id);
CREATE INDEX idx_behavior_decision_effects_entity ON behavior_decision_effects(entity_type, entity_id);
CREATE INDEX idx_behavior_decision_effects_status ON behavior_decision_effects(status);