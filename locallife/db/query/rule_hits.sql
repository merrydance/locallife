-- Phase1: 规则命中审计查询（草案）

-- name: CreateRuleHit :one
INSERT INTO rule_hits (rule_id, rule_version_id, domain, decision, reason, inputs, outputs, actor_id, actor_role, region_id, merchant_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListRuleHitsByRule :many
SELECT id, rule_id, rule_version_id, domain, decision, reason, inputs, outputs, actor_id, actor_role, region_id, merchant_id, created_at FROM rule_hits
WHERE rule_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListRuleHitsByRuleAndRegion :many
SELECT id, rule_id, rule_version_id, domain, decision, reason, inputs, outputs, actor_id, actor_role, region_id, merchant_id, created_at FROM rule_hits
WHERE rule_id = $1 AND region_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;
