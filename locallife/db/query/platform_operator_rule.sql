-- name: GetPlatformOperatorRuleBaselineFromRegion :one
SELECT
  commission_rate,
  merchant_deposit,
  rider_deposit
FROM region_rule_configs
ORDER BY id
LIMIT 1;

-- name: GetPlatformOperatorRuleBaselineFromOperator :one
SELECT
  commission_rate,
  merchant_deposit,
  rider_deposit
FROM operators
ORDER BY id
LIMIT 1;

-- name: UpdateAllRegionRuleConfigCommissionRate :exec
UPDATE region_rule_configs
SET
  commission_rate = $1,
  updated_at = NOW();

-- name: UpdateAllRegionRuleConfigMerchantDeposit :exec
UPDATE region_rule_configs
SET
  merchant_deposit = $1,
  updated_at = NOW();

-- name: UpdateAllRegionRuleConfigRiderDeposit :exec
UPDATE region_rule_configs
SET
  rider_deposit = $1,
  updated_at = NOW();

-- name: UpdateAllOperatorsCommissionRate :exec
UPDATE operators
SET
  commission_rate = $1,
  updated_at = NOW();

-- name: UpdateAllOperatorsMerchantDeposit :exec
UPDATE operators
SET
  merchant_deposit = $1,
  updated_at = NOW();

-- name: UpdateAllOperatorsRiderDeposit :exec
UPDATE operators
SET
  rider_deposit = $1,
  updated_at = NOW();
