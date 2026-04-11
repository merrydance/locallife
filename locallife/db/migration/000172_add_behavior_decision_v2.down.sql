DROP INDEX IF EXISTS idx_behavior_decision_effects_status;
DROP INDEX IF EXISTS idx_behavior_decision_effects_entity;
DROP INDEX IF EXISTS idx_behavior_decision_effects_decision_id;
DROP TABLE IF EXISTS behavior_decision_effects;

DROP INDEX IF EXISTS idx_claim_recovery_events_decision_id;
DROP INDEX IF EXISTS idx_claim_recovery_events_recovery_id;
DROP TABLE IF EXISTS claim_recovery_events;

DROP INDEX IF EXISTS idx_claim_recoveries_decision_id;
ALTER TABLE claim_recoveries
    DROP CONSTRAINT IF EXISTS claim_recoveries_recovery_basis_check,
    DROP COLUMN IF EXISTS recovery_basis,
    DROP COLUMN IF EXISTS decision_id;

DROP INDEX IF EXISTS idx_behavior_trace_snapshots_actor_window;
ALTER TABLE behavior_trace_snapshots
    DROP CONSTRAINT IF EXISTS behavior_trace_snapshots_stats_scope_check,
    DROP CONSTRAINT IF EXISTS behavior_trace_snapshots_window_key_check,
    DROP CONSTRAINT IF EXISTS behavior_trace_snapshots_actor_type_check,
    DROP COLUMN IF EXISTS snapshot_version,
    DROP COLUMN IF EXISTS association_payload,
    DROP COLUMN IF EXISTS metric_payload,
    DROP COLUMN IF EXISTS stats_scope,
    DROP COLUMN IF EXISTS window_key,
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS actor_type;

DROP INDEX IF EXISTS idx_behavior_decisions_mode_status;
DROP INDEX IF EXISTS idx_behavior_decisions_effective_claim_id;
DROP INDEX IF EXISTS idx_behavior_decisions_claim_id;
ALTER TABLE behavior_decisions
    DROP CONSTRAINT IF EXISTS behavior_decisions_mode_reason_check,
    DROP CONSTRAINT IF EXISTS behavior_decisions_effective_status_check,
    DROP CONSTRAINT IF EXISTS behavior_decisions_payout_mode_check,
    DROP CONSTRAINT IF EXISTS behavior_decisions_responsibility_domain_check,
    DROP CONSTRAINT IF EXISTS behavior_decisions_decision_mode_check,
    DROP COLUMN IF EXISTS profile_effect_applied,
    DROP COLUMN IF EXISTS overturned_by_decision_id,
    DROP COLUMN IF EXISTS supersedes_decision_id,
    DROP COLUMN IF EXISTS fact_snapshot,
    DROP COLUMN IF EXISTS graph_hits,
    DROP COLUMN IF EXISTS score_breakdown,
    DROP COLUMN IF EXISTS liability_shares,
    DROP COLUMN IF EXISTS restriction_reason,
    DROP COLUMN IF EXISTS fallback_reason,
    DROP COLUMN IF EXISTS rider_liability_score,
    DROP COLUMN IF EXISTS merchant_liability_score,
    DROP COLUMN IF EXISTS user_risk_score,
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS effective_status,
    DROP COLUMN IF EXISTS payout_mode,
    DROP COLUMN IF EXISTS responsibility_domain,
    DROP COLUMN IF EXISTS decision_mode,
    DROP COLUMN IF EXISTS claim_id;