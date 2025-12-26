-- M11: 千人千面推荐引擎 - SQLC查询

-- ============================================================================
-- 用户行为埋点
-- ============================================================================

-- name: TrackBehavior :one
INSERT INTO user_behaviors (
  user_id,
  behavior_type,
  dish_id,
  combo_id,
  merchant_id,
  duration
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetUserRecentBehaviors :many
SELECT * FROM user_behaviors
WHERE user_id = $1
  AND created_at >= $2
ORDER BY created_at DESC
LIMIT $3;

-- name: GetUserBehaviorsByType :many
SELECT * FROM user_behaviors
WHERE user_id = $1
  AND behavior_type = $2
  AND created_at >= $3
ORDER BY created_at DESC
LIMIT $4;

-- ============================================================================
-- 用户偏好管理
-- ============================================================================

-- name: GetUserPreferences :one
SELECT * FROM user_preferences
WHERE user_id = $1;

-- name: UpsertUserPreferences :one
INSERT INTO user_preferences (
  user_id,
  cuisine_preferences,
  price_range_min,
  price_range_max,
  avg_order_amount,
  favorite_time_slots,
  purchase_frequency,
  last_order_date,
  top_cuisines,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, now()
)
ON CONFLICT (user_id)
DO UPDATE SET
  cuisine_preferences = EXCLUDED.cuisine_preferences,
  price_range_min = EXCLUDED.price_range_min,
  price_range_max = EXCLUDED.price_range_max,
  avg_order_amount = EXCLUDED.avg_order_amount,
  favorite_time_slots = EXCLUDED.favorite_time_slots,
  purchase_frequency = EXCLUDED.purchase_frequency,
  last_order_date = EXCLUDED.last_order_date,
  top_cuisines = EXCLUDED.top_cuisines,
  updated_at = now()
RETURNING *;

-- ============================================================================
-- 推荐结果管理
-- ============================================================================

-- name: SaveRecommendations :one
INSERT INTO recommendations (
  user_id,
  dish_ids,
  combo_ids,
  merchant_ids,
  algorithm,
  score,
  generated_at,
  expired_at
) VALUES (
  $1, $2, $3, $4, $5, $6, now(), $7
) RETURNING *;

-- name: GetLatestRecommendations :one
SELECT * FROM recommendations
WHERE user_id = $1
  AND expired_at > now()
ORDER BY generated_at DESC
LIMIT 1;

-- name: DeleteExpiredRecommendations :exec
DELETE FROM recommendations
WHERE expired_at <= now();

-- ============================================================================
-- 推荐配置管理（运营商）
-- ============================================================================

-- name: GetRecommendationConfig :one
SELECT * FROM recommendation_configs
WHERE region_id = $1;

-- name: UpsertRecommendationConfig :one
INSERT INTO recommendation_configs (
  region_id,
  exploitation_ratio,
  exploration_ratio,
  random_ratio,
  auto_adjust,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, now()
)
ON CONFLICT (region_id)
DO UPDATE SET
  exploitation_ratio = EXCLUDED.exploitation_ratio,
  exploration_ratio = EXCLUDED.exploration_ratio,
  random_ratio = EXCLUDED.random_ratio,
  auto_adjust = EXCLUDED.auto_adjust,
  updated_at = now()
RETURNING *;

-- name: GetAllRecommendationConfigs :many
SELECT * FROM recommendation_configs
ORDER BY region_id;
