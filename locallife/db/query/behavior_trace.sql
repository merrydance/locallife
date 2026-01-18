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
SELECT * FROM platform_configs
WHERE config_key = $1 AND scope_type = $2 AND scope_id IS NOT DISTINCT FROM $3
LIMIT 1;

-- name: ListPlatformConfigsByKey :many
SELECT * FROM platform_configs
WHERE config_key = $1
ORDER BY scope_type, scope_id;

-- ==============================
-- behavior_decisions
-- ==============================

-- name: CreateBehaviorDecision :one
INSERT INTO behavior_decisions (
    order_id,
    user_id,
    merchant_id,
    rider_id,
    decision_version,
    reason_codes,
    responsible_party,
    compensation_source,
    decision_status,
    trace_summary
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetBehaviorDecision :one
SELECT * FROM behavior_decisions
WHERE id = $1
LIMIT 1;

-- name: ListBehaviorDecisionsByOrder :many
SELECT * FROM behavior_decisions
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: UpdateBehaviorDecisionStatus :exec
UPDATE behavior_decisions
SET decision_status = $2,
    updated_at = NOW()
WHERE id = $1;

-- ==============================
-- behavior_evidence
-- ==============================

-- name: CreateBehaviorEvidence :one
INSERT INTO behavior_evidence (
    decision_id,
    evidence_type,
    payload
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: ListBehaviorEvidenceByDecision :many
SELECT * FROM behavior_evidence
WHERE decision_id = $1
ORDER BY created_at ASC;

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
    association_hits
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListBehaviorTraceSnapshotsByDecision :many
SELECT * FROM behavior_trace_snapshots
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

-- name: ListBehaviorActionsByDecision :many
SELECT * FROM behavior_actions
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
SELECT * FROM behavior_appeals
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
SELECT * FROM behavior_blocklist
WHERE entity_type = $1 AND entity_id = $2 AND status = 'active'
LIMIT 1;

-- name: UpdateBehaviorBlocklistStatus :exec
UPDATE behavior_blocklist
SET status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListActiveBehaviorBlocklists :many
SELECT * FROM behavior_blocklist
WHERE status = 'active'
ORDER BY created_at DESC;
