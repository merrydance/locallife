-- Behavior trace system queries

-- ==============================
-- platform_configs
-- ==============================

-- name: CreatePlatformConfig :one
INSERT INTO platform_configs (
    config_key,
    config_value,
    scope_type,
    scope_id
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: UpsertPlatformConfig :one
INSERT INTO platform_configs (
    config_key,
    config_value,
    scope_type,
    scope_id
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (config_key, scope_type, scope_id) DO UPDATE
SET config_value = EXCLUDED.config_value,
    updated_at = NOW()
RETURNING *;

-- name: GetPlatformConfig :one
SELECT id, config_key, config_value, scope_type, scope_id, created_at, updated_at FROM platform_configs
WHERE config_key = $1 AND scope_type = $2 AND scope_id IS NOT DISTINCT FROM $3
ORDER BY updated_at DESC, id DESC
LIMIT 1;

-- name: ListPlatformConfigsByKey :many
SELECT id, config_key, config_value, scope_type, scope_id, created_at, updated_at FROM platform_configs
WHERE config_key = $1
ORDER BY scope_type, scope_id;

-- ==============================
-- behavior_decisions
-- ==============================

-- name: CreateBehaviorDecision :one
INSERT INTO behavior_decisions (
    order_id,
    reservation_id,
    claim_id,
    user_id,
    merchant_id,
    rider_id,
    decision_version,
    reason_codes,
    responsible_party,
    compensation_source,
    decision_status,
    trace_summary,
    decision_mode,
    responsibility_domain,
    payout_mode,
    confidence_score,
    user_risk_score,
    merchant_liability_score,
    rider_liability_score,
    fallback_reason,
    restriction_reason,
    liability_shares,
    score_breakdown,
    graph_hits,
    fact_snapshot,
    supersedes_decision_id,
    overturned_by_decision_id
) VALUES (
    sqlc.narg('order_id'), sqlc.narg('reservation_id'), sqlc.narg('claim_id'), $1, $2, $3, $4, $5, $6, $7, $8, $9,
    sqlc.narg('decision_mode'),
    sqlc.narg('responsibility_domain'),
    sqlc.narg('payout_mode'),
    sqlc.narg('confidence_score'),
    sqlc.narg('user_risk_score'),
    sqlc.narg('merchant_liability_score'),
    sqlc.narg('rider_liability_score'),
    sqlc.narg('fallback_reason'),
    sqlc.narg('restriction_reason'),
    sqlc.narg('liability_shares'),
    sqlc.narg('score_breakdown'),
    sqlc.narg('graph_hits'),
    sqlc.narg('fact_snapshot'),
    sqlc.narg('supersedes_decision_id'),
    sqlc.narg('overturned_by_decision_id')
) RETURNING *;

-- name: GetBehaviorDecision :one
SELECT id, order_id, user_id, merchant_id, rider_id, decision_version, reason_codes, responsible_party, compensation_source, decision_status, trace_summary, created_at, updated_at, reservation_id, claim_id, decision_mode, responsibility_domain, payout_mode, effective_status, confidence_score, user_risk_score, merchant_liability_score, rider_liability_score, fallback_reason, restriction_reason, liability_shares, score_breakdown, graph_hits, fact_snapshot, supersedes_decision_id, overturned_by_decision_id, profile_effect_applied FROM behavior_decisions
WHERE id = $1
LIMIT 1;

-- name: ListBehaviorDecisionsByOrder :many
SELECT id, order_id, user_id, merchant_id, rider_id, decision_version, reason_codes, responsible_party, compensation_source, decision_status, trace_summary, created_at, updated_at, reservation_id, claim_id, decision_mode, responsibility_domain, payout_mode, effective_status, confidence_score, user_risk_score, merchant_liability_score, rider_liability_score, fallback_reason, restriction_reason, liability_shares, score_breakdown, graph_hits, fact_snapshot, supersedes_decision_id, overturned_by_decision_id, profile_effect_applied FROM behavior_decisions
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: UpdateBehaviorDecisionStatus :exec
UPDATE behavior_decisions
SET decision_status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateBehaviorDecisionProfileEffectApplied :exec
UPDATE behavior_decisions
SET profile_effect_applied = $2,
    updated_at = NOW()
WHERE id = $1;

-- ==============================
-- behavior_trace_snapshots
-- ==============================

-- name: CountUserOrders :one
SELECT COUNT(*)::INT AS total_orders
FROM orders
WHERE user_id = $1;

-- name: GetUserClaimWindowStats :one
SELECT 
    (SELECT COUNT(*) FROM orders o WHERE o.user_id = $1 AND o.order_type = 'takeout'
     AND o.created_at >= NOW() - INTERVAL '7 days')::INT as takeout_orders_7d,
    (SELECT COUNT(*) FROM claims c WHERE c.user_id = $1 
     AND c.created_at >= NOW() - INTERVAL '7 days')::INT as claims_7d,
    (SELECT COUNT(*) FROM orders o WHERE o.user_id = $1 AND o.order_type = 'takeout'
     AND o.created_at >= NOW() - INTERVAL '30 days')::INT as takeout_orders_30d,
    (SELECT COUNT(*) FROM claims c WHERE c.user_id = $1 
     AND c.created_at >= NOW() - INTERVAL '30 days')::INT as claims_30d;

-- name: CreateBehaviorTraceSnapshot :one
INSERT INTO behavior_trace_snapshots (
    decision_id,
    window_days,
    abnormal_count,
    total_count,
    abnormal_rate,
    association_hits,
    actor_type,
    actor_id,
    window_key,
    stats_scope,
    metric_payload,
    association_payload,
    snapshot_version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    sqlc.narg('actor_type'),
    sqlc.narg('actor_id'),
    sqlc.narg('window_key'),
    sqlc.narg('stats_scope'),
    COALESCE(sqlc.narg('metric_payload'), '{}'::jsonb),
    COALESCE(sqlc.narg('association_payload'), '{}'::jsonb),
    COALESCE(sqlc.narg('snapshot_version'), 'v2')
) RETURNING *;

-- ==============================
-- behavior_decision_effects
-- ==============================

-- name: CreateBehaviorDecisionEffect :one
INSERT INTO behavior_decision_effects (
    decision_id,
    entity_type,
    entity_id,
    metric_key,
    delta_value,
    status,
    note
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListBehaviorDecisionEffectsByDecision :many
SELECT id, decision_id, entity_type, entity_id, metric_key, delta_value, status, applied_at, reverted_at, reverted_by_decision_id, note, created_at FROM behavior_decision_effects
WHERE decision_id = $1
ORDER BY created_at ASC;

-- name: GetBehaviorEffectSummary :one
SELECT
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'claim_attempts' AND status = 'applied'), 0)::BIGINT AS claim_attempts,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'effective_claims' AND status = 'applied'), 0)::BIGINT AS effective_claims,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'effective_liability_claims' AND status = 'applied'), 0)::BIGINT AS effective_liability_claims,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'merchant_recovered_claims' AND status = 'applied'), 0)::BIGINT AS merchant_recovered_claims,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'rider_recovered_claims' AND status = 'applied'), 0)::BIGINT AS rider_recovered_claims,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'platform_fallback_claims' AND status = 'applied'), 0)::BIGINT AS platform_fallback_claims,
        COALESCE(SUM(delta_value) FILTER (WHERE metric_key = 'malicious_confirmed_claims' AND status = 'applied'), 0)::BIGINT AS malicious_confirmed_claims
FROM behavior_decision_effects
WHERE entity_type = sqlc.arg('entity_type')
    AND entity_id = sqlc.arg('entity_id')
    AND created_at >= sqlc.arg('start_at')
    AND created_at <= sqlc.arg('end_at');

-- name: RevertBehaviorDecisionEffectsByDecision :exec
UPDATE behavior_decision_effects
SET status = 'reverted',
    reverted_at = COALESCE(sqlc.narg('reverted_at'), NOW()),
    reverted_by_decision_id = sqlc.narg('reverted_by_decision_id')
WHERE decision_id = $1
  AND status = 'applied';

-- name: ListBehaviorTraceSnapshotsByDecision :many
SELECT id, decision_id, window_days, abnormal_count, total_count, abnormal_rate, association_hits, created_at, actor_type, actor_id, window_key, stats_scope, metric_payload, association_payload, snapshot_version FROM behavior_trace_snapshots
WHERE decision_id = $1
ORDER BY created_at ASC;

-- ==============================
-- behavior_actions
-- ==============================

-- name: CreateBehaviorAction :one
INSERT INTO behavior_actions (
    decision_id,
    action_type,
    target_entity,
    status,
    detail
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateBehaviorActionStatus :exec
UPDATE behavior_actions
SET status = $2,
    executed_at = COALESCE(sqlc.narg('executed_at'), executed_at)
WHERE id = $1;

-- name: UpdateBehaviorActionExecution :exec
UPDATE behavior_actions
SET status = $2,
    detail = $3,
    executed_at = COALESCE(sqlc.narg('executed_at'), executed_at)
WHERE id = $1;

-- name: GetBehaviorAction :one
SELECT id, decision_id, action_type, target_entity, status, detail, executed_at, created_at FROM behavior_actions
WHERE id = $1
LIMIT 1;

-- name: ListBehaviorActionsByStatusAndType :many
SELECT id, decision_id, action_type, target_entity, status, detail, executed_at, created_at FROM behavior_actions
WHERE status = $1
    AND action_type = $2
    AND target_entity = $3
ORDER BY created_at ASC
LIMIT $4;

-- name: ListBehaviorActionsByDecision :many
SELECT id, decision_id, action_type, target_entity, status, detail, executed_at, created_at FROM behavior_actions
WHERE decision_id = $1
ORDER BY created_at ASC;

-- ==============================
-- behavior_appeals
-- ==============================

-- name: CreateBehaviorAppeal :one
INSERT INTO behavior_appeals (
    entity_type,
    entity_id,
    decision_id,
    reason,
    evidence,
    status,
    reeval_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: UpdateBehaviorAppealStatus :exec
UPDATE behavior_appeals
SET status = $2,
    reeval_at = COALESCE(sqlc.narg('reeval_at'), reeval_at)
WHERE id = $1;

-- name: ListBehaviorAppealsByEntity :many
SELECT id, entity_type, entity_id, decision_id, reason, evidence, status, created_at, reeval_at FROM behavior_appeals
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC;

-- ==============================
-- behavior_blocklist
-- ==============================

-- name: CreateBehaviorBlocklist :one
INSERT INTO behavior_blocklist (
    entity_type,
    entity_id,
    reason_code,
    block_until,
    status
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetActiveBehaviorBlocklist :one
SELECT id, entity_type, entity_id, reason_code, block_until, status, created_at, updated_at FROM behavior_blocklist
WHERE entity_type = $1 AND entity_id = $2 AND status = 'active'
LIMIT 1;

-- name: UpdateBehaviorBlocklistStatus :exec
UPDATE behavior_blocklist
SET status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListActiveBehaviorBlocklists :many
SELECT id, entity_type, entity_id, reason_code, block_until, status, created_at, updated_at FROM behavior_blocklist
WHERE status = 'active'
ORDER BY created_at DESC;
