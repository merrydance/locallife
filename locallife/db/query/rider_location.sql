-- name: CreateRiderLocation :one
INSERT INTO rider_locations (
    rider_id,
    delivery_id,
    longitude,
    latitude,
    accuracy,
    speed,
    heading,
    recorded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: BatchCreateRiderLocations :copyfrom
INSERT INTO rider_locations (
    rider_id,
    delivery_id,
    longitude,
    latitude,
    accuracy,
    speed,
    heading,
    recorded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetRiderLatestLocation :one
SELECT id, rider_id, delivery_id, longitude, latitude, accuracy, speed, heading, recorded_at FROM rider_locations
WHERE rider_id = $1
ORDER BY recorded_at DESC, id DESC
LIMIT 1;

-- name: GetDeliveryLatestLocation :one
SELECT id, rider_id, delivery_id, longitude, latitude, accuracy, speed, heading, recorded_at FROM rider_locations
WHERE delivery_id = $1
ORDER BY recorded_at DESC, id DESC
LIMIT 1;

-- name: ListRiderLocations :many
SELECT id, rider_id, delivery_id, longitude, latitude, accuracy, speed, heading, recorded_at FROM rider_locations
WHERE rider_id = sqlc.arg('rider_id')
    AND recorded_at >= sqlc.arg('start_at')
    AND recorded_at <= sqlc.arg('end_at')
ORDER BY recorded_at ASC;

-- name: ListDeliveryLocations :many
SELECT id, rider_id, delivery_id, longitude, latitude, accuracy, speed, heading, recorded_at FROM rider_locations
WHERE delivery_id = $1
ORDER BY recorded_at ASC;

-- name: ListDeliveryLocationsSince :many
SELECT id, rider_id, delivery_id, longitude, latitude, accuracy, speed, heading, recorded_at FROM rider_locations
WHERE delivery_id = $1
    AND recorded_at > $2
ORDER BY recorded_at ASC;

-- name: DeleteOldRiderLocations :exec
DELETE FROM rider_locations
WHERE recorded_at < $1;

-- name: CountRiderLocations :one
SELECT COUNT(*) FROM rider_locations
WHERE rider_id = $1;
