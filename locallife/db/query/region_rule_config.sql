-- name: GetRegionRuleConfigByRegion :one
SELECT id, region_id, commission_rate, merchant_deposit, rider_deposit, weather_coeff_extreme, weather_coeff_heavy, weather_coeff_moderate, weather_coeff_light, created_at, updated_at
FROM region_rule_configs
WHERE region_id = $1
LIMIT 1;

-- name: UpsertRegionRuleConfig :one
INSERT INTO region_rule_configs (
  region_id,
  commission_rate,
  merchant_deposit,
  rider_deposit,
  weather_coeff_extreme,
  weather_coeff_heavy,
  weather_coeff_moderate,
  weather_coeff_light
)
VALUES (
  $1,
  COALESCE(sqlc.narg('commission_rate')::numeric, 0.0300),
  COALESCE(sqlc.narg('merchant_deposit')::bigint, 500000),
  COALESCE(sqlc.narg('rider_deposit')::bigint, 20000),
  COALESCE(sqlc.narg('weather_coeff_extreme')::numeric, 2.00),
  COALESCE(sqlc.narg('weather_coeff_heavy')::numeric, 1.80),
  COALESCE(sqlc.narg('weather_coeff_moderate')::numeric, 1.30),
  COALESCE(sqlc.narg('weather_coeff_light')::numeric, 1.10)
)
ON CONFLICT (region_id) DO UPDATE
SET
  commission_rate = COALESCE(sqlc.narg('commission_rate')::numeric, region_rule_configs.commission_rate),
  merchant_deposit = COALESCE(sqlc.narg('merchant_deposit')::bigint, region_rule_configs.merchant_deposit),
  rider_deposit = COALESCE(sqlc.narg('rider_deposit')::bigint, region_rule_configs.rider_deposit),
  weather_coeff_extreme = COALESCE(sqlc.narg('weather_coeff_extreme')::numeric, region_rule_configs.weather_coeff_extreme),
  weather_coeff_heavy = COALESCE(sqlc.narg('weather_coeff_heavy')::numeric, region_rule_configs.weather_coeff_heavy),
  weather_coeff_moderate = COALESCE(sqlc.narg('weather_coeff_moderate')::numeric, region_rule_configs.weather_coeff_moderate),
  weather_coeff_light = COALESCE(sqlc.narg('weather_coeff_light')::numeric, region_rule_configs.weather_coeff_light),
  updated_at = NOW()
RETURNING id, region_id, commission_rate, merchant_deposit, rider_deposit, weather_coeff_extreme, weather_coeff_heavy, weather_coeff_moderate, weather_coeff_light, created_at, updated_at;
