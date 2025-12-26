-- name: CreateDeliveryFeeConfig :one
INSERT INTO delivery_fee_configs (
  region_id,
  base_fee,
  base_distance,
  extra_fee_per_km,
  value_ratio,
  max_fee,
  min_fee,
  is_active
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetDeliveryFeeConfig :one
SELECT * FROM delivery_fee_configs
WHERE id = $1 LIMIT 1;

-- name: GetDeliveryFeeConfigByRegion :one
SELECT * FROM delivery_fee_configs
WHERE region_id = $1 LIMIT 1;

-- name: GetActiveDeliveryFeeConfigByRegion :one
SELECT * FROM delivery_fee_configs
WHERE region_id = $1 AND is_active = true LIMIT 1;

-- name: ListDeliveryFeeConfigs :many
SELECT * FROM delivery_fee_configs
ORDER BY region_id
LIMIT $1 OFFSET $2;

-- name: ListActiveDeliveryFeeConfigs :many
SELECT * FROM delivery_fee_configs
WHERE is_active = true
ORDER BY region_id;

-- name: UpdateDeliveryFeeConfig :one
UPDATE delivery_fee_configs
SET
  base_fee = COALESCE(sqlc.narg(base_fee), base_fee),
  base_distance = COALESCE(sqlc.narg(base_distance), base_distance),
  extra_fee_per_km = COALESCE(sqlc.narg(extra_fee_per_km), extra_fee_per_km),
  value_ratio = COALESCE(sqlc.narg(value_ratio), value_ratio),
  max_fee = sqlc.narg(max_fee),
  min_fee = COALESCE(sqlc.narg(min_fee), min_fee),
  is_active = COALESCE(sqlc.narg(is_active), is_active),
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDeliveryFeeConfig :exec
DELETE FROM delivery_fee_configs
WHERE id = $1;
