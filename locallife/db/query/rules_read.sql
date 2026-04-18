-- Phase1: 规则读取查询（草案）

-- name: ListRules :many
SELECT id, name, category, status, current_version_id, created_by, created_at, updated_at FROM rules
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: GetRule :one
SELECT id, name, category, status, current_version_id, created_by, created_at, updated_at FROM rules WHERE id = $1;

-- name: GetRuleVersion :one
SELECT id, rule_id, version, status, priority, scope, condition, action, gray_config, effective_at, expires_at, created_by, created_at FROM rule_versions WHERE id = $1;

-- name: ListRuleVersionsByRule :many
SELECT id, rule_id, version, status, priority, scope, condition, action, gray_config, effective_at, expires_at, created_by, created_at FROM rule_versions WHERE rule_id = $1 ORDER BY version DESC;
