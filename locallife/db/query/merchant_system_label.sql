-- name: UpsertMerchantCapabilitiesDefaults :exec
INSERT INTO merchant_capabilities (merchant_id)
VALUES ($1)
ON CONFLICT (merchant_id) DO NOTHING;

-- name: GetMerchantCapabilities :one
SELECT merchant_id, open_kitchen_status, dine_in_status, source, note, created_at, updated_at FROM merchant_capabilities
WHERE merchant_id = $1
LIMIT 1;

-- name: UpsertMerchantCapabilities :one
INSERT INTO merchant_capabilities (
  merchant_id,
  open_kitchen_status,
  dine_in_status,
  source,
  note
) VALUES (
  sqlc.arg('merchant_id'),
  COALESCE(sqlc.narg('open_kitchen_status'), 'unknown'),
  COALESCE(sqlc.narg('dine_in_status'), 'unknown'),
  COALESCE(sqlc.narg('source'), 'system_default'),
  sqlc.narg('note')
)
ON CONFLICT (merchant_id) DO UPDATE SET
  open_kitchen_status = COALESCE(sqlc.narg('open_kitchen_status'), merchant_capabilities.open_kitchen_status),
  dine_in_status = COALESCE(sqlc.narg('dine_in_status'), merchant_capabilities.dine_in_status),
  source = COALESCE(sqlc.narg('source'), merchant_capabilities.source),
  note = COALESCE(sqlc.narg('note'), merchant_capabilities.note),
  updated_at = NOW()
RETURNING *;

-- name: ListMerchantSystemLabels :many
SELECT t.id, t.name, t.type, t.sort_order, t.status, t.created_at, t.icon
FROM tags t
INNER JOIN merchant_system_labels msl ON t.id = msl.tag_id
WHERE msl.merchant_id = $1
  AND t.type = 'system'
  AND t.status = 'active'
ORDER BY t.sort_order ASC, t.name ASC;

-- name: ListMerchantSystemLabelLinks :many
SELECT merchant_id, tag_id, source, created_at, updated_at
FROM merchant_system_labels
WHERE merchant_id = $1
ORDER BY tag_id ASC;

-- name: UpsertMerchantSystemLabel :exec
INSERT INTO merchant_system_labels (
  merchant_id,
  tag_id,
  source
) VALUES (
  $1, $2, $3
)
ON CONFLICT (merchant_id, tag_id) DO UPDATE SET
  source = EXCLUDED.source,
  updated_at = NOW();

-- name: RemoveMerchantSystemLabel :exec
DELETE FROM merchant_system_labels
WHERE merchant_id = $1
  AND tag_id = $2;
