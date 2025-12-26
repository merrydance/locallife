-- name: GetRecommendConfig :one
SELECT * FROM recommend_configs
WHERE name = $1 LIMIT 1;

-- name: GetActiveRecommendConfig :one
SELECT * FROM recommend_configs
WHERE is_active = true
ORDER BY id DESC
LIMIT 1;

-- name: CreateRecommendConfig :one
INSERT INTO recommend_configs (
    name,
    distance_weight,
    route_weight,
    urgency_weight,
    profit_weight,
    max_distance,
    max_results,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: UpdateRecommendConfig :one
UPDATE recommend_configs
SET 
    distance_weight = $2,
    route_weight = $3,
    urgency_weight = $4,
    profit_weight = $5,
    max_distance = $6,
    max_results = $7,
    is_active = $8,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRecommendConfigs :many
SELECT * FROM recommend_configs
ORDER BY created_at DESC;
