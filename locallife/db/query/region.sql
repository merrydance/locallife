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

-- name: ListRegions :many
SELECT * FROM regions
WHERE 
  CASE 
    WHEN sqlc.narg(parent_id)::bigint IS NULL THEN parent_id IS NULL
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
-- 获取未被运营商占用的区域列表（优化：避免 N+1 查询）
SELECT 
  r.*,
  p.name as parent_name
FROM regions r
LEFT JOIN regions p ON r.parent_id = p.id
WHERE 
  NOT EXISTS (
    SELECT 1 FROM operators o WHERE o.region_id = r.id
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
WHERE level = 3
ORDER BY (
    6371000 * acos(
        cos(radians(sqlc.arg(lat)::float8)) * cos(radians(latitude)) * 
        cos(radians(longitude) - radians(sqlc.arg(lon)::float8)) + 
        sin(radians(sqlc.arg(lat)::float8)) * sin(radians(latitude))
    )
) ASC
LIMIT 1;
