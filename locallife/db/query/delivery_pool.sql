-- name: AddToDeliveryPool :one
INSERT INTO delivery_pool (
    order_id,
    merchant_id,
    pickup_longitude,
    pickup_latitude,
    delivery_longitude,
    delivery_latitude,
    distance,
    delivery_fee,
    expected_pickup_at,
    expires_at,
    priority
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetDeliveryPoolItem :one
SELECT * FROM delivery_pool
WHERE id = $1 LIMIT 1;

-- name: GetDeliveryPoolByOrderID :one
SELECT * FROM delivery_pool
WHERE order_id = $1 LIMIT 1;

-- name: GetDeliveryPoolItemForUpdate :one
SELECT * FROM delivery_pool
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: RemoveFromDeliveryPool :exec
DELETE FROM delivery_pool
WHERE order_id = $1;

-- name: RemoveExpiredFromDeliveryPool :exec
-- 清理已过期订单池项（用于订单取消等情况，expires_at不再用于可见性过滤）
DELETE FROM delivery_pool
WHERE expires_at < now();

-- name: ListDeliveryPool :many
-- 列出所有待接单的配送池订单
-- 外卖订单始终可见直到被接单或取消
-- 动态优先级 = 基础优先级 + 等待时间加成（每等待10分钟加1级）
SELECT *,
    (priority + EXTRACT(EPOCH FROM (now() - created_at)) / 600)::int AS effective_priority
FROM delivery_pool
ORDER BY effective_priority DESC, created_at ASC
LIMIT $1 OFFSET $2;

-- name: ListDeliveryPoolNearby :many
-- 按骑手位置获取附近的可接订单
-- 动态优先级：等待越久优先级越高
SELECT *, 
    (6371000 * acos(cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(pickup_latitude)) * 
    cos(radians(pickup_longitude) - radians(sqlc.arg(rider_lng)::float8)) + 
    sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(pickup_latitude))))::int AS distance_to_rider,
    (priority + EXTRACT(EPOCH FROM (now() - created_at)) / 600)::int AS effective_priority
FROM delivery_pool
WHERE (6371000 * acos(cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(pickup_latitude)) * 
        cos(radians(pickup_longitude) - radians(sqlc.arg(rider_lng)::float8)) + 
        sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(pickup_latitude)))) < sqlc.arg(max_distance)::float8
ORDER BY distance_to_rider ASC
LIMIT sqlc.arg(result_limit)::int;

-- name: ListDeliveryPoolNearbyByRegion :many
-- 按区域过滤的骑手可接订单列表，实现多租户隔离
-- 骑手只能看到其所属区域内商户的订单
-- 距离越近排名越靠前，同时返回动态优先级供前端展示
SELECT dp.*, 
    (6371000 * acos(cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(dp.pickup_latitude)) * 
    cos(radians(dp.pickup_longitude) - radians(sqlc.arg(rider_lng)::float8)) + 
    sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(dp.pickup_latitude))))::int AS distance_to_rider,
    (dp.priority + EXTRACT(EPOCH FROM (now() - dp.created_at)) / 600)::int AS effective_priority
FROM delivery_pool dp
JOIN merchants m ON dp.merchant_id = m.id
WHERE m.region_id = sqlc.arg(region_id)::bigint
    AND (6371000 * acos(cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(dp.pickup_latitude)) * 
        cos(radians(dp.pickup_longitude) - radians(sqlc.arg(rider_lng)::float8)) + 
        sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(dp.pickup_latitude)))) < sqlc.arg(max_distance)::float8
ORDER BY distance_to_rider ASC
LIMIT sqlc.arg(result_limit)::int;

-- name: CountDeliveryPool :one
SELECT COUNT(*) FROM delivery_pool;
