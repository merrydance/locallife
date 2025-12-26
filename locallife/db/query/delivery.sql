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
SELECT * FROM deliveries
WHERE id = $1 LIMIT 1;

-- name: GetDeliveryByOrderID :one
SELECT * FROM deliveries
WHERE order_id = $1 LIMIT 1;

-- name: GetDeliveryForUpdate :one
SELECT * FROM deliveries
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
WHERE id = $1 AND rider_id = $2
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
    delivered_at = now()
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
SELECT * FROM deliveries
WHERE rider_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListDeliveriesByRiderAndStatus :many
SELECT * FROM deliveries
WHERE rider_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListRiderActiveDeliveries :many
SELECT * FROM deliveries
WHERE rider_id = $1 AND status IN ('assigned', 'picking', 'picked', 'delivering')
ORDER BY created_at ASC;

-- name: ListPendingDeliveries :many
SELECT * FROM deliveries
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: CountRiderDeliveries :one
SELECT COUNT(*) FROM deliveries
WHERE rider_id = $1;

-- name: CountRiderCompletedDeliveries :one
SELECT COUNT(*) FROM deliveries
WHERE rider_id = $1 AND status = 'completed';

-- name: GetRiderEarnings :one
SELECT COALESCE(SUM(rider_earnings), 0) AS total_earnings
FROM deliveries
WHERE rider_id = $1 AND status = 'completed';

-- name: GetRiderDailyEarnings :one
SELECT COALESCE(SUM(rider_earnings), 0) AS daily_earnings
FROM deliveries
WHERE rider_id = $1 
    AND status = 'completed'
    AND completed_at >= $2
    AND completed_at < $3;

-- name: UpdateDeliveryEstimatedTime :one
UPDATE deliveries
SET 
    estimated_delivery_at = $2
WHERE id = $1
RETURNING *;
