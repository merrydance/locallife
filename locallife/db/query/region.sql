-- name: CreateRegion :one
INSERT INTO regions (
  code,
  name,
  level,
  parent_id,
  longitude,
  latitude
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetRegion :one
SELECT * FROM regions
WHERE id = $1 LIMIT 1;

-- name: GetRegionByCode :one
SELECT * FROM regions
WHERE code = $1 LIMIT 1;

-- name: GetRegionByProviderCode :one
SELECT r.* FROM region_external_mappings rem
JOIN regions r ON r.id = rem.region_id
WHERE rem.provider = $1 AND rem.external_code = $2
LIMIT 1;

-- name: UpsertRegionExternalMapping :one
INSERT INTO region_external_mappings (
  region_id,
  provider,
  external_code,
  external_name
) VALUES (
  $1, $2, $3, $4
)
ON CONFLICT (provider, external_code)
DO UPDATE SET
  region_id = EXCLUDED.region_id,
  external_name = EXCLUDED.external_name
RETURNING *;

-- name: GetRegionByNameAndLevel :one
SELECT * FROM regions
WHERE name = $1 AND level = $2 LIMIT 1;

-- name: GetRegionByNameAndParent :one
SELECT * FROM regions
WHERE name = $1 AND parent_id = $2 LIMIT 1;

-- name: ListRegions :many
SELECT * FROM regions
WHERE 
  CASE 
    WHEN sqlc.narg(parent_id)::bigint IS NULL THEN TRUE
    ELSE parent_id = sqlc.narg(parent_id)
  END
  AND CASE
    WHEN sqlc.narg(level)::smallint IS NULL THEN TRUE
    ELSE level = sqlc.narg(level)
  END
ORDER BY id
LIMIT $1
OFFSET $2;

-- name: ListRegionChildren :many
SELECT * FROM regions
WHERE parent_id = $1
ORDER BY id;

-- name: SearchRegionsByName :many
SELECT * FROM regions
WHERE name LIKE '%' || $1 || '%'
ORDER BY level, id
LIMIT $2;

-- name: UpdateRegion :one
UPDATE regions
SET
  name = COALESCE(sqlc.narg(name), name),
  longitude = COALESCE(sqlc.narg(longitude), longitude),
  latitude = COALESCE(sqlc.narg(latitude), latitude)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteRegion :exec
DELETE FROM regions
WHERE id = $1;

-- name: GetRegionsWithDeliveryFeeConfig :many
-- 获取已开通运费配置的区县（带市名，用于天气抓取）
SELECT 
  r.id,
  r.name,
  r.qweather_location_id,
  p.name as city_name
FROM regions r
JOIN delivery_fee_configs dfc ON r.id = dfc.region_id
LEFT JOIN regions p ON r.parent_id = p.id
WHERE dfc.is_active = true;

-- name: UpdateRegionQweatherLocationID :exec
-- 更新区县的和风天气 LocationID（首次查询后缓存）
UPDATE regions 
SET qweather_location_id = $2 
WHERE id = $1;

-- name: ListAvailableRegions :many
-- 获取可申请区域列表：排除已被有效运营商占用，且排除已提交/已通过的申请占坑
SELECT 
  r.*,
  p.name as parent_name
FROM regions r
LEFT JOIN regions p ON r.parent_id = p.id
WHERE 
  NOT EXISTS (
    SELECT 1
    FROM operator_regions or_t
    JOIN operators o ON o.id = or_t.operator_id
    WHERE or_t.region_id = r.id
      AND or_t.status = 'active'
      AND o.status = 'active'
  )
  AND
  NOT EXISTS (
    SELECT 1 FROM operators o WHERE o.region_id = r.id
  )
  AND
  NOT EXISTS (
    SELECT 1
    FROM operator_applications oa
    WHERE oa.region_id = r.id
      AND oa.status IN ('submitted', 'approved')
  )
  AND CASE 
    WHEN sqlc.narg(parent_id)::bigint IS NULL THEN TRUE
    ELSE r.parent_id = sqlc.narg(parent_id)
  END
  AND CASE
    WHEN sqlc.narg(level)::smallint IS NULL THEN r.level = 3
    ELSE r.level = sqlc.narg(level)
  END
ORDER BY r.id
LIMIT $1
OFFSET $2;

-- name: GetClosestRegion :one
-- 根据经纬度获取最近的区县级区域
SELECT * FROM regions
WHERE level IN (3, 4)
  AND longitude IS NOT NULL
  AND latitude IS NOT NULL
ORDER BY (
    6371000 * acos(
        cos(radians(sqlc.arg(lat)::float8)) * cos(radians(latitude)) * 
        cos(radians(longitude) - radians(sqlc.arg(lon)::float8)) + 
        sin(radians(sqlc.arg(lat)::float8)) * sin(radians(latitude))
    )
) ASC
LIMIT 1;
