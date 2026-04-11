-- name: CreateDeliveryLocationEvent :one
INSERT INTO delivery_location_events (
    delivery_id,
    order_id,
    rider_id,
    longitude,
    latitude,
    accuracy,
    speed,
    event_type,
    source,
    recorded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (delivery_id, event_type) DO NOTHING
RETURNING id, delivery_id, order_id, rider_id, longitude, latitude, accuracy, speed, event_type, source, recorded_at, created_at;
