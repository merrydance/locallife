-- name: CreateDeliveryTimeoutAlert :one
INSERT INTO delivery_timeout_alerts (
    delivery_id,
    alert_key
) VALUES (
    $1,
    $2
) RETURNING *;

-- name: DeleteDeliveryTimeoutAlert :exec
DELETE FROM delivery_timeout_alerts
WHERE delivery_id = $1
    AND alert_key = $2;

-- name: ListPendingDeliveriesBeforeWithoutAlert :many
SELECT d.id, d.order_id, d.rider_id, d.pickup_address, d.pickup_longitude, d.pickup_latitude, d.pickup_contact, d.pickup_phone, d.picked_at, d.delivery_address, d.delivery_longitude, d.delivery_latitude, d.delivery_contact, d.delivery_phone, d.delivered_at, d.distance, d.delivery_fee, d.rider_earnings, d.status, d.estimated_pickup_at, d.estimated_delivery_at, d.is_damaged, d.is_delayed, d.damage_amount, d.damage_reason, d.created_at, d.assigned_at, d.completed_at, d.rider_delivered_at
FROM deliveries d
WHERE d.status = sqlc.arg('status')
    AND d.created_at < sqlc.arg('created_at')
    AND NOT EXISTS (
        SELECT 1
        FROM delivery_timeout_alerts a
        WHERE a.delivery_id = d.id
            AND a.alert_key = sqlc.arg('alert_key')
    )
ORDER BY d.created_at ASC, d.id ASC
LIMIT sqlc.arg('limit');