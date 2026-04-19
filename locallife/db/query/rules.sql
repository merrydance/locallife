-- Phase1: 规则引擎基础查询（草案）

-- name: CreateRule :one
INSERT INTO rules (name, category, status, current_version_id, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreateRuleVersion :one
INSERT INTO rule_versions (rule_id, version, status, priority, scope, condition, action, gray_config, effective_at, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateRuleCurrentVersion :one
UPDATE rules
SET current_version_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRuleStatus :one
UPDATE rules
SET status = $2, current_version_id = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListActiveRuleVersions :many
SELECT rv.id, rv.rule_id, rv.version, rv.status, rv.priority, rv.scope, rv.condition, rv.action, rv.gray_config, rv.effective_at, rv.expires_at, rv.created_by, rv.created_at FROM rule_versions rv
JOIN rules r ON r.id = rv.rule_id
WHERE r.status = 'active'
  AND rv.status = 'published'
  AND (rv.effective_at IS NULL OR rv.effective_at <= now())
  AND (rv.expires_at IS NULL OR rv.expires_at > now())
ORDER BY rv.priority ASC, rv.id ASC;

-- name: CreateRuleAudit :one
INSERT INTO rule_audits (rule_id, rule_version_id, action, actor_id, actor_role, detail)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
