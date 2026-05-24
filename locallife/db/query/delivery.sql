-- name: CreateDelivery :one
INSERT INTO deliveries (
    order_id,
    pickup_address,
    pickup_longitude,
    pickup_latitude,
    pickup_contact,
    pickup_phone,
    delivery_address,
    delivery_longitude,
    delivery_latitude,
    delivery_contact,
    delivery_phone,
    distance,
    delivery_fee,
    estimated_pickup_at,
    estimated_delivery_at,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'pending'
) RETURNING *;

-- name: GetDelivery :one
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE id = $1 LIMIT 1;

-- name: GetDeliveryByOrderID :one
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE order_id = $1 LIMIT 1;

-- name: GetDeliveryForUpdate :one
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: AssignDelivery :one
UPDATE deliveries
SET 
    rider_id = $2,
    status = 'assigned',
    assigned_at = now()
WHERE id = $1 AND rider_id IS NULL
RETURNING *;

-- name: UpdateDeliveryToPickup :one
UPDATE deliveries
SET 
    status = 'picking'
WHERE id = $1 AND rider_id = $2 AND status = 'assigned'
RETURNING *;

-- name: UpdateDeliveryToPicked :one
UPDATE deliveries
SET 
    status = 'picked',
    picked_at = now()
WHERE id = $1 AND rider_id = $2 AND status = 'picking'
RETURNING *;

-- name: UpdateDeliveryToDelivering :one
UPDATE deliveries
SET 
    status = 'delivering'
WHERE id = $1 AND rider_id = $2 AND status = 'picked'
RETURNING *;

-- name: UpdateDeliveryToDelivered :one
UPDATE deliveries
SET 
    status = 'delivered',
    delivered_at = now(),
    rider_delivered_at = now()
WHERE id = $1 AND rider_id = $2 AND status = 'delivering'
RETURNING *;

-- name: UpdateDeliveryToCompleted :one
UPDATE deliveries
SET 
    status = 'completed',
    rider_earnings = $2,
    completed_at = now()
WHERE id = $1 AND status = 'delivered'
RETURNING *;

-- name: UpdateDeliveryToCancelled :one
UPDATE deliveries
SET 
    status = 'cancelled'
WHERE id = $1
RETURNING *;

-- name: UpdateDeliveryDamage :one
UPDATE deliveries
SET 
    is_damaged = true,
    damage_amount = $2,
    damage_reason = $3
WHERE id = $1
RETURNING *;

-- name: UpdateDeliveryDelayed :one
UPDATE deliveries
SET is_delayed = true
WHERE id = $1
RETURNING *;

-- name: ListDeliveriesByRider :many
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE rider_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListDeliveriesByRiderAndStatus :many
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE rider_id = $1 AND status = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: ListRiderActiveDeliveries :many
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE rider_id = $1 AND status IN ('assigned', 'picking', 'picked', 'delivering')
ORDER BY created_at ASC;

-- name: ListPendingDeliveries :many
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE status = 'pending'
ORDER BY created_at ASC, id ASC
LIMIT $1;

-- name: CountRiderDeliveries :one
SELECT COUNT(*) FROM deliveries
WHERE rider_id = $1;

-- name: CountRiderCompletedDeliveries :one
SELECT COUNT(*) FROM deliveries
WHERE rider_id = $1 AND status = 'completed';

-- name: CountRiderCompletedDeliveriesInRange :one
SELECT COUNT(*) FROM deliveries
WHERE rider_id = sqlc.arg('rider_id')
    AND status = 'completed'
    AND completed_at >= sqlc.arg('start_at')
    AND completed_at < sqlc.arg('end_at');

-- name: GetRiderEarnings :one
SELECT COALESCE(SUM(rider_earnings), 0) AS total_earnings
FROM deliveries
WHERE rider_id = $1 AND status = 'completed';

-- name: GetRiderDailyEarnings :one
SELECT COALESCE(SUM(rider_earnings), 0) AS daily_earnings
FROM deliveries
WHERE rider_id = sqlc.arg('rider_id')
    AND status = 'completed'
    AND completed_at >= sqlc.arg('start_at')
    AND completed_at < sqlc.arg('end_at');

-- name: UpdateDeliveryEstimatedTime :one
UPDATE deliveries
SET 
    estimated_delivery_at = $2
WHERE id = $1
RETURNING *;

-- name: ListPendingDeliveriesBefore :many
-- 获取超时未接单的代取单
SELECT id, order_id, rider_id, pickup_address, pickup_longitude, pickup_latitude, pickup_contact, pickup_phone, picked_at, delivery_address, delivery_longitude, delivery_latitude, delivery_contact, delivery_phone, delivered_at, distance, delivery_fee, rider_earnings, status, estimated_pickup_at, estimated_delivery_at, is_damaged, is_delayed, damage_amount, damage_reason, created_at, assigned_at, completed_at, rider_delivered_at FROM deliveries
WHERE status = sqlc.arg('status')
  AND created_at < sqlc.arg('created_at')
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg('limit');
