-- name: CreateWeatherCoefficient :one
INSERT INTO weather_coefficients (
  region_id,
  recorded_at,
  weather_data,
  warning_data,
  weather_type,
  weather_code,
  temperature,
  feels_like,
  humidity,
  wind_speed,
  wind_scale,
  precip,
  visibility,
  has_warning,
  warning_type,
  warning_level,
  warning_severity,
  warning_text,
  weather_coefficient,
  warning_coefficient,
  final_coefficient,
  delivery_suspended,
  suspend_reason
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
) RETURNING *;

-- name: GetWeatherCoefficient :one
SELECT * FROM weather_coefficients
WHERE id = $1 LIMIT 1;

-- name: GetLatestWeatherCoefficient :one
SELECT * FROM weather_coefficients
WHERE region_id = $1
ORDER BY recorded_at DESC
LIMIT 1;

-- name: ListWeatherCoefficients :many
SELECT * FROM weather_coefficients
WHERE region_id = $1
ORDER BY recorded_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRecentWeatherCoefficients :many
SELECT * FROM weather_coefficients
WHERE region_id = $1 AND recorded_at >= $2
ORDER BY recorded_at DESC;

-- name: ListRegionsWithWarning :many
SELECT DISTINCT region_id FROM weather_coefficients
WHERE has_warning = true
  AND recorded_at = (
    SELECT MAX(recorded_at) FROM weather_coefficients wc2
    WHERE wc2.region_id = weather_coefficients.region_id
  );

-- name: ListSuspendedRegions :many
SELECT * FROM weather_coefficients w1
WHERE delivery_suspended = true
  AND recorded_at = (
    SELECT MAX(recorded_at) FROM weather_coefficients w2
    WHERE w2.region_id = w1.region_id
  );

-- name: DeleteOldWeatherCoefficients :exec
DELETE FROM weather_coefficients
WHERE recorded_at < $1;
