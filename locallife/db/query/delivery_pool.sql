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
    expected_delivery_at,
    expires_at,
    priority
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetDeliveryPoolItem :one
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at FROM delivery_pool
WHERE id = $1 LIMIT 1;

-- name: GetDeliveryPoolByOrderID :one
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at FROM delivery_pool
WHERE order_id = $1 LIMIT 1;

-- name: GetDeliveryPoolItemForUpdate :one
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at FROM delivery_pool
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetDeliveryPoolByOrderIDForUpdate :one
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at FROM delivery_pool
WHERE order_id = $1 LIMIT 1
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
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at,
    (priority + EXTRACT(EPOCH FROM (now() - created_at)) / 600)::int AS effective_priority
FROM delivery_pool
ORDER BY effective_priority DESC, created_at ASC
LIMIT $1 OFFSET $2;

-- name: ListDeliveryPoolNearby :many
-- 按骑手位置获取附近的可接订单
-- 动态优先级：等待越久优先级越高
SELECT id, order_id, merchant_id, pickup_longitude, pickup_latitude, delivery_longitude, delivery_latitude, distance, delivery_fee, expected_pickup_at, expires_at, priority, created_at, expected_delivery_at,
    (6371000 * acos(LEAST(1, GREATEST(-1,
        cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(pickup_latitude::float8)) * 
        cos(radians(pickup_longitude::float8) - radians(sqlc.arg(rider_lng)::float8)) + 
        sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(pickup_latitude::float8))
    ))))::int AS distance_to_rider,
    (priority + EXTRACT(EPOCH FROM (now() - created_at)) / 600)::int AS effective_priority
FROM delivery_pool
WHERE (6371000 * acos(LEAST(1, GREATEST(-1,
        cos(radians(sqlc.arg(rider_lat)::float8)) * cos(radians(pickup_latitude::float8)) * 
        cos(radians(pickup_longitude::float8) - radians(sqlc.arg(rider_lng)::float8)) + 
        sin(radians(sqlc.arg(rider_lat)::float8)) * sin(radians(pickup_latitude::float8))
    )))) < sqlc.arg(max_distance)::float8
ORDER BY distance_to_rider ASC
LIMIT sqlc.arg(result_limit)::int;

-- name: CountDeliveryPool :one
SELECT COUNT(*) FROM delivery_pool;
