-- Phase2: 分账规则配置查询（草案）

-- name: GetActiveProfitSharingConfig :one
SELECT id, status, order_source, region_id, merchant_id, platform_rate, operator_rate, rider_enabled, priority, effective_at, expires_at, created_by, created_at, updated_at FROM profit_sharing_configs
WHERE status = 'active'
  AND (order_source = $1 OR order_source = 'all')
  AND (merchant_id IS NULL OR merchant_id = $2)
  AND (region_id IS NULL OR region_id = $3)
  AND (effective_at IS NULL OR effective_at <= now())
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY
  CASE WHEN merchant_id IS NOT NULL THEN 0 ELSE 1 END,
  CASE WHEN region_id IS NOT NULL THEN 0 ELSE 1 END,
  priority ASC,
  id DESC
LIMIT 1;

-- name: CreateProfitSharingConfig :one
INSERT INTO profit_sharing_configs (
  status,
  order_source,
  region_id,
  merchant_id,
  platform_rate,
  operator_rate,
  rider_enabled,
  priority,
  effective_at,
  expires_at,
  created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: SetProfitSharingAuditActor :exec
SELECT set_config('app.actor_id', $1::text, true),
  set_config('app.actor_role', $2, true),
  set_config('app.actor_detail', $3, true);

-- name: UpdateProfitSharingConfig :one
UPDATE profit_sharing_configs
SET
  status = COALESCE(sqlc.narg('status'), status),
  order_source = COALESCE(sqlc.narg('order_source'), order_source),
  region_id = COALESCE(sqlc.narg('region_id'), region_id),
  merchant_id = COALESCE(sqlc.narg('merchant_id'), merchant_id),
  platform_rate = COALESCE(sqlc.narg('platform_rate'), platform_rate),
  operator_rate = COALESCE(sqlc.narg('operator_rate'), operator_rate),
  rider_enabled = COALESCE(sqlc.narg('rider_enabled'), rider_enabled),
  priority = COALESCE(sqlc.narg('priority'), priority),
  effective_at = COALESCE(sqlc.narg('effective_at'), effective_at),
  expires_at = COALESCE(sqlc.narg('expires_at'), expires_at),
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateProfitSharingConfigStatus :one
UPDATE profit_sharing_configs
SET
  status = $2,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListProfitSharingConfigs :many
SELECT id, status, order_source, region_id, merchant_id, platform_rate, operator_rate, rider_enabled, priority, effective_at, expires_at, created_by, created_at, updated_at FROM profit_sharing_configs
WHERE (NULLIF($1::text, '') IS NULL OR status = $1)
  AND (NULLIF($2::text, '') IS NULL OR order_source = $2)
  AND ($3::bigint = 0 OR region_id = $3)
  AND ($4::bigint = 0 OR merchant_id = $4)
ORDER BY priority ASC, id DESC
LIMIT $5 OFFSET $6;

-- name: ListProfitSharingConfigsForRegion :many
SELECT psc.id, psc.status, psc.order_source, psc.region_id, psc.merchant_id, psc.platform_rate, psc.operator_rate, psc.rider_enabled, psc.priority, psc.effective_at, psc.expires_at, psc.created_by, psc.created_at, psc.updated_at
FROM profit_sharing_configs psc
LEFT JOIN merchants m ON m.id = psc.merchant_id
WHERE (NULLIF($1::text, '') IS NULL OR psc.status = $1)
  AND (NULLIF($2::text, '') IS NULL OR psc.order_source = $2)
  AND (psc.region_id IS NULL OR psc.region_id = $3)
  AND ($4::bigint = 0 OR psc.merchant_id = $4)
  AND (psc.merchant_id IS NULL OR m.region_id = $3)
ORDER BY psc.priority ASC, psc.id DESC
LIMIT $5 OFFSET $6;

-- name: ListProfitSharingConfigsForRegions :many
SELECT psc.id, psc.status, psc.order_source, psc.region_id, psc.merchant_id, psc.platform_rate, psc.operator_rate, psc.rider_enabled, psc.priority, psc.effective_at, psc.expires_at, psc.created_by, psc.created_at, psc.updated_at
FROM profit_sharing_configs psc
LEFT JOIN merchants m ON m.id = psc.merchant_id
WHERE (NULLIF(sqlc.arg('status')::text, '') IS NULL OR psc.status = sqlc.arg('status'))
  AND (NULLIF(sqlc.arg('order_source')::text, '') IS NULL OR psc.order_source = sqlc.arg('order_source'))
  AND (psc.region_id IS NULL OR psc.region_id = ANY(sqlc.arg('region_ids')::bigint[]))
  AND (sqlc.arg('merchant_id')::bigint = 0 OR psc.merchant_id = sqlc.arg('merchant_id'))
  AND (psc.merchant_id IS NULL OR m.region_id = ANY(sqlc.arg('region_ids')::bigint[]))
ORDER BY psc.priority ASC, psc.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListProfitSharingConfigAudits :many
SELECT id, config_id, action, actor_id, actor_role, detail, created_at
FROM profit_sharing_config_audits
WHERE ($1::bigint = 0 OR config_id = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
