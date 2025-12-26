-- name: CreateRider :one
INSERT INTO riders (
    user_id,
    real_name,
    id_card_no,
    phone,
    region_id,
    status
) VALUES (
    $1, $2, $3, $4, $5, 'pending'
) RETURNING *;

-- name: GetRider :one
SELECT * FROM riders
WHERE id = $1 LIMIT 1;

-- name: GetRiderByUserID :one
SELECT * FROM riders
WHERE user_id = $1 LIMIT 1;

-- name: GetRiderForUpdate :one
SELECT * FROM riders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: UpdateRiderRegion :one
-- 更新骑手所属区域
UPDATE riders
SET region_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRiderStatus :one
UPDATE riders
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRiderOnlineStatus :one
UPDATE riders
SET is_online = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRiderLocation :one
UPDATE riders
SET 
    current_longitude = $2,
    current_latitude = $3,
    location_updated_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRiderDeposit :one
UPDATE riders
SET 
    deposit_amount = $2,
    frozen_deposit = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeductRiderDeposit :one
-- 从骑手押金扣款（原子操作：检查余额 + 扣款）
UPDATE riders
SET 
    deposit_amount = deposit_amount - $2,
    updated_at = now()
WHERE id = $1
  AND deposit_amount >= $2  -- 确保余额充足
RETURNING *;

-- name: GetRiderForDeposit :one
-- 获取骑手押金信息（用于扣款前检查）
SELECT id, deposit_amount, frozen_deposit
FROM riders
WHERE id = $1
FOR UPDATE;

-- name: UpdateRiderStats :one
UPDATE riders
SET 
    total_orders = total_orders + $2,
    total_earnings = total_earnings + $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRiderOnlineDuration :one
UPDATE riders
SET 
    online_duration = online_duration + $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListOnlineRiders :many
SELECT * FROM riders
WHERE is_online = true AND status = 'active'
ORDER BY location_updated_at DESC;

-- name: ListRidersByStatus :many
SELECT * FROM riders
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListNearbyRiders :many
SELECT *, 
    (6371000 * acos(
        LEAST(1::float8, GREATEST(-1::float8,
            cos(radians(sqlc.arg(center_lat)::float8)) * cos(radians(current_latitude::float8)) * 
            cos(radians(current_longitude::float8) - radians(sqlc.arg(center_lng)::float8)) + 
            sin(radians(sqlc.arg(center_lat)::float8)) * sin(radians(current_latitude::float8))
        ))
    ))::int AS distance
FROM riders
WHERE is_online = true 
    AND status = 'active'
    AND current_latitude IS NOT NULL
    AND current_longitude IS NOT NULL
    AND (6371000 * acos(
        LEAST(1::float8, GREATEST(-1::float8,
            cos(radians(sqlc.arg(center_lat)::float8)) * cos(radians(current_latitude::float8)) * 
            cos(radians(current_longitude::float8) - radians(sqlc.arg(center_lng)::float8)) + 
            sin(radians(sqlc.arg(center_lat)::float8)) * sin(radians(current_latitude::float8))
        ))
    )) < sqlc.arg(max_distance)::float8
ORDER BY distance
LIMIT sqlc.arg(limit_count)::int;

-- name: CountRidersByStatus :one
SELECT COUNT(*) FROM riders
WHERE status = $1;

-- name: CountOnlineRiders :one
SELECT COUNT(*) FROM riders
WHERE is_online = true AND status = 'active';

-- name: ListRidersByRegion :many
-- 按区域列出骑手（供运营商管理使用）
SELECT * FROM riders
WHERE region_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRidersByRegion :one
-- 统计区域内骑手数量
SELECT COUNT(*) FROM riders
WHERE region_id = $1;

-- name: ListRidersByRegionWithStatus :many
-- 按区域和状态列出骑手
SELECT * FROM riders
WHERE region_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountRidersByRegionWithStatus :one
-- 按区域和状态统计骑手数量
SELECT COUNT(*) FROM riders
WHERE region_id = $1 AND status = $2;

-- name: ListOnlineRidersByRegion :many
-- 列出区域内在线骑手
SELECT * FROM riders
WHERE region_id = $1 
    AND is_online = true 
    AND status = 'active'
ORDER BY location_updated_at DESC;

-- name: UpdateRiderSubMchID :one
-- 更新骑手的微信二级商户号
UPDATE riders
SET 
    sub_mch_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetRiderBySubMchID :one
-- 通过二级商户号查找骑手
SELECT * FROM riders
WHERE sub_mch_id = $1 LIMIT 1;
