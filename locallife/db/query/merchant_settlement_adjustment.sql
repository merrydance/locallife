-- Merchant settlement adjustment queries

-- name: CreateMerchantSettlementAdjustment :one
INSERT INTO merchant_settlement_adjustments (
  merchant_id,
  adjustment_type,
  amount,
  status,
  related_type,
  related_id,
  note,
  posted_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetMerchantSettlementAdjustmentByRelatedAndType :one
SELECT * FROM merchant_settlement_adjustments
WHERE related_type = $1 AND related_id = $2 AND adjustment_type = $3
LIMIT 1;

-- name: ListMerchantSettlementAdjustments :many
SELECT * FROM merchant_settlement_adjustments
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantSettlementAdjustments :one
SELECT COUNT(*)::bigint
FROM merchant_settlement_adjustments
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: SumMerchantSettlementAdjustments :one
SELECT COALESCE(SUM(amount), 0)::bigint AS total_adjustment
FROM merchant_settlement_adjustments
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: ListMerchantDailySettlementAdjustments :many
SELECT DATE(created_at) AS date,
       COALESCE(SUM(amount), 0)::bigint AS total_adjustment
FROM merchant_settlement_adjustments
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- name: ListMerchantSettlementTimeline :many
SELECT
  'profit_sharing'::text AS record_type,
  p.id,
  p.payment_order_id,
  p.order_source,
  p.total_amount,
  p.platform_commission,
  p.operator_commission,
  p.merchant_amount,
  p.out_order_no,
  p.sharing_order_id,
  p.status,
  p.created_at,
  p.finished_at,
  NULL::text AS adjustment_type,
  NULL::text AS related_type,
  NULL::bigint AS related_id
FROM profit_sharing_orders p
WHERE p.merchant_id = sqlc.arg('merchant_id')
  AND p.created_at >= sqlc.arg('start_at') AND p.created_at <= sqlc.arg('end_at')
UNION ALL
SELECT
  'adjustment'::text AS record_type,
  a.id,
  0 AS payment_order_id,
  'adjustment'::text AS order_source,
  0 AS total_amount,
  0 AS platform_commission,
  0 AS operator_commission,
  a.amount AS merchant_amount,
  NULL::text AS out_order_no,
  NULL::text AS sharing_order_id,
  a.status,
  a.created_at,
  a.posted_at AS finished_at,
  a.adjustment_type,
  a.related_type,
  a.related_id
FROM merchant_settlement_adjustments a
WHERE a.merchant_id = sqlc.arg('merchant_id')
  AND a.created_at >= sqlc.arg('start_at') AND a.created_at <= sqlc.arg('end_at')
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantSettlementTimeline :one
SELECT (
  (SELECT COUNT(*) FROM profit_sharing_orders p
   WHERE p.merchant_id = sqlc.arg('merchant_id')
     AND p.created_at >= sqlc.arg('start_at') AND p.created_at <= sqlc.arg('end_at'))
  +
  (SELECT COUNT(*) FROM merchant_settlement_adjustments a
   WHERE a.merchant_id = sqlc.arg('merchant_id')
     AND a.created_at >= sqlc.arg('start_at') AND a.created_at <= sqlc.arg('end_at'))
)::bigint AS total_count;
