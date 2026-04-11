-- Phase1: 规则读取查询（草案）

-- name: ListRules :many
SELECT * FROM rules
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: GetRule :one
SELECT * FROM rules WHERE id = $1;

-- name: GetRuleVersion :one
SELECT * FROM rule_versions WHERE id = $1;

-- name: ListRuleVersionsByRule :many
SELECT * FROM rule_versions WHERE rule_id = $1 ORDER BY version DESC;
