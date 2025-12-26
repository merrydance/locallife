-- name: CreatePeakHourConfig :one
INSERT INTO peak_hour_configs (
  region_id,
  name,
  start_time,
  end_time,
  coefficient,
  days_of_week,
  is_active
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetPeakHourConfig :one
SELECT * FROM peak_hour_configs
WHERE id = $1 LIMIT 1;

-- name: ListPeakHourConfigsByRegion :many
SELECT * FROM peak_hour_configs
WHERE region_id = $1
ORDER BY start_time;

-- name: ListActivePeakHourConfigsByRegion :many
SELECT * FROM peak_hour_configs
WHERE region_id = $1 AND is_active = true
ORDER BY start_time;

-- name: UpdatePeakHourConfig :one
UPDATE peak_hour_configs
SET
  name = COALESCE(sqlc.narg(name), name),
  start_time = COALESCE(sqlc.narg(start_time), start_time),
  end_time = COALESCE(sqlc.narg(end_time), end_time),
  coefficient = COALESCE(sqlc.narg(coefficient), coefficient),
  days_of_week = COALESCE(sqlc.narg(days_of_week), days_of_week),
  is_active = COALESCE(sqlc.narg(is_active), is_active),
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeletePeakHourConfig :exec
DELETE FROM peak_hour_configs
WHERE id = $1;

-- name: DeletePeakHourConfigsByRegion :exec
DELETE FROM peak_hour_configs
WHERE region_id = $1;
