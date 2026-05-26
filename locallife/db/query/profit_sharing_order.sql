-- name: CreateProfitSharingOrder :one
INSERT INTO profit_sharing_orders (
    payment_order_id,
    merchant_id,
    operator_id,
    order_source,
    total_amount,
    delivery_fee,
    rider_id,
    rider_amount,
    distributable_amount,
    platform_rate,
    operator_rate,
    platform_commission,
    operator_commission,
    merchant_amount,
    out_order_no,
    status,
    payment_fee,
    payment_fee_rate_bps,
    provider,
    channel,
    merchant_sharing_mer_id,
    rider_sharing_mer_id,
    operator_sharing_mer_id,
    platform_sharing_mer_id,
    sharing_detail_snapshot
) VALUES (
    sqlc.arg(payment_order_id),
    sqlc.arg(merchant_id),
    sqlc.narg(operator_id),
    sqlc.arg(order_source),
    sqlc.arg(total_amount),
    sqlc.arg(delivery_fee),
    sqlc.narg(rider_id),
    sqlc.arg(rider_amount),
    sqlc.arg(distributable_amount),
    sqlc.arg(platform_rate),
    sqlc.arg(operator_rate),
    sqlc.arg(platform_commission),
    sqlc.arg(operator_commission),
    sqlc.arg(merchant_amount),
    sqlc.arg(out_order_no),
    sqlc.arg(status),
    sqlc.arg(payment_fee)::bigint,
    COALESCE(NULLIF(sqlc.arg(payment_fee_rate_bps)::integer, 0), 30),
    COALESCE(sqlc.narg(provider), 'wechat'),
    COALESCE(sqlc.narg(channel), 'baofu_aggregate'),
    sqlc.narg(merchant_sharing_mer_id),
    sqlc.narg(rider_sharing_mer_id),
    sqlc.narg(operator_sharing_mer_id),
    sqlc.narg(platform_sharing_mer_id),
    COALESCE(sqlc.narg(sharing_detail_snapshot), '{}'::jsonb)
) RETURNING *;

-- name: CreateProfitSharingOrderSimple :one
-- 简化版创建（不含骑手分账，用于堂食/自提订单）
INSERT INTO profit_sharing_orders (
    payment_order_id,
    merchant_id,
    operator_id,
    order_source,
    total_amount,
    delivery_fee,
    distributable_amount,
    platform_rate,
    operator_rate,
    platform_commission,
    operator_commission,
    merchant_amount,
    out_order_no,
    status
) VALUES (
    $1, $2, $3, $4, $5, 0, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: UpdateProfitSharingOrderFeeBreakdown :one
UPDATE profit_sharing_orders
SET
    calculation_version = sqlc.arg(calculation_version),
    settlement_mode = sqlc.arg(settlement_mode),
    provider_payment_fee = sqlc.arg(provider_payment_fee),
    provider_payment_fee_rate_bps = sqlc.arg(provider_payment_fee_rate_bps),
    provider_payment_fee_base_amount = sqlc.arg(provider_payment_fee_base_amount),
    provider_payment_fee_source = sqlc.arg(provider_payment_fee_source),
    merchant_payment_fee = sqlc.arg(merchant_payment_fee),
    merchant_payment_fee_rate_bps = sqlc.arg(merchant_payment_fee_rate_bps),
    merchant_payment_fee_base_amount = sqlc.arg(merchant_payment_fee_base_amount),
    rider_gross_amount = sqlc.arg(rider_gross_amount),
    rider_payment_fee = sqlc.arg(rider_payment_fee),
    rider_payment_fee_rate_bps = sqlc.arg(rider_payment_fee_rate_bps),
    rider_payment_fee_base_amount = sqlc.arg(rider_payment_fee_base_amount),
    commission_base_amount = sqlc.arg(commission_base_amount),
    platform_receiver_amount = sqlc.arg(platform_receiver_amount)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateProfitSharingOrderRiderBillByPaymentOrder :one
UPDATE profit_sharing_orders
SET
    rider_id = sqlc.arg(rider_id),
    rider_sharing_mer_id = sqlc.arg(rider_sharing_mer_id),
    rider_amount = sqlc.arg(rider_amount),
    distributable_amount = sqlc.arg(distributable_amount),
    platform_commission = sqlc.arg(platform_commission),
    operator_commission = sqlc.arg(operator_commission),
    merchant_amount = sqlc.arg(merchant_amount),
    sharing_detail_snapshot = sqlc.arg(sharing_detail_snapshot),
    rider_gross_amount = sqlc.arg(rider_gross_amount),
    rider_payment_fee = sqlc.arg(rider_payment_fee),
    rider_payment_fee_rate_bps = sqlc.arg(rider_payment_fee_rate_bps),
    rider_payment_fee_base_amount = sqlc.arg(rider_payment_fee_base_amount),
    merchant_payment_fee = sqlc.arg(merchant_payment_fee),
    merchant_payment_fee_base_amount = sqlc.arg(merchant_payment_fee_base_amount),
    commission_base_amount = sqlc.arg(commission_base_amount),
    platform_receiver_amount = sqlc.arg(platform_receiver_amount)
WHERE payment_order_id = sqlc.arg(payment_order_id)
  AND status = 'pending'
RETURNING *;

-- name: GetProfitSharingOrder :one
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE id = $1 LIMIT 1;

-- name: GetProfitSharingOrderForUpdate :one
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetProfitSharingOrderByOutOrderNo :one
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE out_order_no = $1 LIMIT 1;

-- name: GetProfitSharingOrderByPaymentOrder :one
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE payment_order_id = $1 LIMIT 1;

-- name: ListProfitSharingOrdersByOrderIDsForMerchant :many
SELECT
    COALESCE(po.order_id, 0)::bigint AS order_id,
    sqlc.embed(p)
FROM profit_sharing_orders p
JOIN payment_orders po ON po.id = p.payment_order_id
WHERE p.merchant_id = sqlc.arg('merchant_id')
  AND po.order_id = ANY(sqlc.arg('order_ids')::bigint[])
ORDER BY po.order_id, p.created_at DESC, p.id DESC;

-- name: ListProfitSharingOrdersByMerchant :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE merchant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListProfitSharingOrdersByOperator :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE operator_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListProfitSharingOrdersByStatus :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE status = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: ListPlatformProfitSharingReconciliationDetails :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE COALESCE(finished_at, created_at) >= sqlc.arg('start_at')
  AND COALESCE(finished_at, created_at) <= sqlc.arg('end_at')
ORDER BY COALESCE(finished_at, created_at) DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPlatformProfitSharingReconciliationDetails :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE COALESCE(finished_at, created_at) >= sqlc.arg('start_at')
  AND COALESCE(finished_at, created_at) <= sqlc.arg('end_at');

-- name: ListProfitSharingOrdersForRetry :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount FROM profit_sharing_orders
WHERE status IN ('pending', 'failed', 'processing')
  AND created_at <= $1
ORDER BY created_at ASC, id ASC
LIMIT $2;

-- name: ListBaofuProfitSharingOrdersReadyForCommand :many
SELECT pso.id, pso.payment_order_id, pso.merchant_id, pso.operator_id, pso.order_source, pso.total_amount, pso.platform_commission, pso.operator_commission, pso.merchant_amount, pso.out_order_no, pso.sharing_order_id, pso.status, pso.finished_at, pso.created_at, pso.delivery_fee, pso.rider_id, pso.rider_amount, pso.distributable_amount, pso.platform_rate, pso.operator_rate, pso.payment_fee, pso.payment_fee_rate_bps, pso.provider, pso.channel, pso.merchant_sharing_mer_id, pso.rider_sharing_mer_id, pso.operator_sharing_mer_id, pso.platform_sharing_mer_id, pso.sharing_detail_snapshot, pso.calculation_version, pso.settlement_mode, pso.provider_payment_fee, pso.provider_payment_fee_rate_bps, pso.provider_payment_fee_base_amount, pso.provider_payment_fee_source, pso.merchant_payment_fee, pso.merchant_payment_fee_rate_bps, pso.merchant_payment_fee_base_amount, pso.rider_gross_amount, pso.rider_payment_fee, pso.rider_payment_fee_rate_bps, pso.rider_payment_fee_base_amount, pso.commission_base_amount, pso.platform_receiver_amount
FROM profit_sharing_orders pso
JOIN payment_orders po ON po.id = pso.payment_order_id
LEFT JOIN orders o ON po.business_type = 'order' AND po.order_id = o.id
LEFT JOIN table_reservations r ON po.business_type IN ('reservation', 'reservation_addon') AND po.reservation_id = r.id
WHERE pso.provider = 'baofu'
  AND pso.channel = 'baofu_aggregate'
  AND pso.status IN ('pending', 'failed')
  AND pso.created_at <= sqlc.arg(created_before)
  AND po.status = 'paid'
  AND po.payment_channel = 'baofu_aggregate'
  AND po.requires_profit_sharing = TRUE
  AND (
      (
          po.business_type = 'order'
          AND o.status = 'completed'
      )
      OR (
          po.business_type IN ('reservation', 'reservation_addon')
          AND r.status IN ('paid', 'confirmed', 'checked_in', 'completed')
      )
  )
  AND NOT EXISTS (
      SELECT 1 FROM refund_orders ro
      WHERE ro.payment_order_id = po.id
        AND ro.status IN ('pending', 'processing', 'success')
  )
ORDER BY pso.created_at ASC, pso.id ASC
LIMIT sqlc.arg('limit')::int;

-- name: GetProfitSharingReconciliationSummary :many
SELECT
    status,
    COUNT(*) as total_orders,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(distributable_amount), 0)::bigint as total_merchant_flow,
    COALESCE(SUM(rider_gross_amount), 0)::bigint as total_rider_flow,
    COALESCE(SUM(platform_commission), 0)::bigint as total_platform_commission,
    COALESCE(SUM(operator_commission), 0)::bigint as total_operator_commission,
    COALESCE(SUM(merchant_amount), 0)::bigint as total_merchant_amount,
    COALESCE(SUM(rider_amount), 0)::bigint as total_rider_amount
FROM profit_sharing_orders
WHERE COALESCE(finished_at, created_at) >= sqlc.arg('start_at')
  AND COALESCE(finished_at, created_at) <= sqlc.arg('end_at')
GROUP BY status
ORDER BY status;

-- name: GetProfitSharingSlaSummary :one
SELECT
  COUNT(*) as total_orders,
  COUNT(*) FILTER (WHERE status = 'finished') as finished_orders,
  COUNT(*) FILTER (WHERE status = 'failed') as failed_orders,
  COUNT(*) FILTER (WHERE status IN ('pending', 'processing')) as pending_orders,
  COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - created_at))) FILTER (WHERE status = 'finished' AND finished_at IS NOT NULL), 0)::bigint as avg_finish_seconds,
  COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (finished_at - created_at))) FILTER (WHERE status = 'finished' AND finished_at IS NOT NULL), 0)::bigint as p95_finish_seconds
FROM profit_sharing_orders
WHERE COALESCE(finished_at, created_at) >= sqlc.arg('start_at')
  AND COALESCE(finished_at, created_at) <= sqlc.arg('end_at');

-- name: UpdateProfitSharingOrderToProcessing :one
UPDATE profit_sharing_orders
SET
    status = 'processing',
    sharing_order_id = $2
WHERE id = $1 AND status IN ('pending', 'failed')
RETURNING *;

-- name: UpdateProfitSharingOrderToFinished :one
UPDATE profit_sharing_orders
SET
    status = 'finished',
    finished_at = now()
WHERE id = $1 AND status = 'processing'
RETURNING *;

-- name: UpdateProfitSharingOrderToFailed :one
UPDATE profit_sharing_orders
SET
    status = 'failed'
WHERE id = $1 AND status = 'processing'
RETURNING *;

-- name: GetMerchantProfitSharingStats :one
SELECT 
    COUNT(*) as total_orders,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(merchant_amount), 0)::bigint as total_merchant_receivable_amount,
    COALESCE(SUM(platform_commission + operator_commission), 0)::bigint as total_platform_service_fee_amount,
    COALESCE(SUM(CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END), 0)::bigint as total_payment_channel_fee_amount
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id') AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: GetOperatorProfitSharingStats :one
SELECT 
    COUNT(*) as total_orders,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(operator_commission), 0)::bigint as total_operator_commission
FROM profit_sharing_orders
WHERE operator_id = sqlc.arg('operator_id') AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: GetOperatorProfitSharingStatsByRegion :one
-- 按区域过滤的运营商分账统计（多区域运营商区域维度财务概览）
SELECT 
    COUNT(*) as total_orders,
    COALESCE(SUM(ps.total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(ps.operator_commission), 0)::bigint as total_operator_commission
FROM profit_sharing_orders ps
JOIN merchants m ON m.id = ps.merchant_id
WHERE ps.operator_id = sqlc.arg('operator_id') AND ps.status = 'finished'
  AND m.region_id = sqlc.arg('region_id')
  AND ps.created_at >= sqlc.arg('start_at') AND ps.created_at <= sqlc.arg('end_at');

-- name: ListMerchantFinanceOrders :many
-- 商户财务订单明细（带分账信息）
SELECT 
    p.id,
    p.payment_order_id,
    p.order_source,
    p.total_amount,
    (p.platform_commission + p.operator_commission)::bigint AS platform_service_fee_amount,
    (CASE WHEN p.calculation_version = 'baofu_fee_v2' THEN p.merchant_payment_fee ELSE p.payment_fee END)::bigint AS payment_channel_fee_amount,
    p.merchant_amount AS merchant_receivable_amount,
    p.status,
    p.created_at,
    p.finished_at,
    po.order_id,
    po.reservation_id
FROM profit_sharing_orders p
JOIN payment_orders po ON po.id = p.payment_order_id
WHERE p.merchant_id = sqlc.arg('merchant_id')
  AND p.created_at >= sqlc.arg('start_at') AND p.created_at <= sqlc.arg('end_at')
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantFinanceOrders :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: GetMerchantFinanceOverview :one
-- 商户财务概览：统计收入、服务费、净收入
SELECT 
    COUNT(*) FILTER (WHERE status = 'finished') as completed_orders,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_orders,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN total_amount ELSE 0 END), 0)::bigint as total_gmv,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN merchant_amount ELSE 0 END), 0)::bigint as total_merchant_receivable_amount,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN platform_commission + operator_commission ELSE 0 END), 0)::bigint as total_platform_service_fee_amount,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN
        CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END
        ELSE 0 END), 0)::bigint as total_payment_channel_fee_amount,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN merchant_amount ELSE 0 END), 0)::bigint as pending_merchant_receivable_amount
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: GetMerchantServiceFeeDetail :many
-- 商户服务费明细
SELECT 
    DATE(created_at) AS date,
    order_source,
    COUNT(*) as order_count,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(platform_commission + operator_commission), 0)::bigint as platform_service_fee_amount,
    COALESCE(SUM(CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END), 0)::bigint as payment_channel_fee_amount,
    COALESCE(SUM(platform_commission + operator_commission + CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END), 0)::bigint as total_fee_amount
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
GROUP BY DATE(created_at), order_source
ORDER BY date DESC, order_source;

-- name: GetMerchantDailyFinance :many
-- 商户每日财务汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as order_count,
    COALESCE(SUM(total_amount), 0)::bigint as total_gmv,
    COALESCE(SUM(merchant_amount), 0)::bigint as merchant_receivable_amount,
    COALESCE(SUM(CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END), 0)::bigint as payment_channel_fee_amount,
    COALESCE(SUM(platform_commission + operator_commission), 0)::bigint as platform_service_fee_amount,
    COALESCE(SUM(platform_commission + operator_commission + CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END), 0)::bigint as total_deduction_fee_amount
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- name: ListMerchantSettlements :many
-- 商户结算记录（带日期范围和状态筛选）
SELECT
  id,
  payment_order_id,
  order_source,
  total_amount,
  (platform_commission + operator_commission)::bigint AS platform_service_fee_amount,
  (CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END)::bigint AS payment_channel_fee_amount,
  merchant_amount AS merchant_receivable_amount,
  out_order_no,
  sharing_order_id,
  status,
  finished_at,
  created_at
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListMerchantSettlementsByStatus :many
-- 商户结算记录（带日期范围和状态筛选）
SELECT
  id,
  payment_order_id,
  order_source,
  total_amount,
  (platform_commission + operator_commission)::bigint AS platform_service_fee_amount,
  (CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END)::bigint AS payment_channel_fee_amount,
  merchant_amount AS merchant_receivable_amount,
  out_order_no,
  sharing_order_id,
  status,
  finished_at,
  created_at
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = sqlc.arg('status')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantSettlements :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: CountMerchantSettlementsByStatus :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = sqlc.arg('status')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- ==================== 骑手分账查询 ====================

-- name: GetRiderProfitSharingStats :one
-- 骑手代取费收入统计
SELECT 
    COUNT(*) as total_deliveries,
    COALESCE(SUM(rider_amount), 0)::bigint as total_rider_income,
    COALESCE(SUM(delivery_fee), 0)::bigint as total_delivery_fee,
    COALESCE(SUM(CASE WHEN rider_gross_amount > 0 THEN rider_gross_amount ELSE delivery_fee END), 0)::bigint as total_rider_gross_amount,
    COALESCE(SUM(rider_payment_fee), 0)::bigint as total_rider_payment_fee
FROM profit_sharing_orders
WHERE rider_id = sqlc.arg('rider_id') AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: ListRiderProfitSharingOrders :many
-- 骑手代取费明细
SELECT 
  p.id, p.payment_order_id, p.merchant_id, p.operator_id, p.order_source, p.total_amount, p.platform_commission, p.operator_commission, p.merchant_amount, p.out_order_no, p.sharing_order_id, p.status, p.finished_at, p.created_at, p.delivery_fee, p.rider_id, p.rider_amount, p.distributable_amount, p.platform_rate, p.operator_rate, p.payment_fee, p.payment_fee_rate_bps, p.provider, p.channel, p.merchant_sharing_mer_id, p.rider_sharing_mer_id, p.operator_sharing_mer_id, p.platform_sharing_mer_id, p.sharing_detail_snapshot, p.calculation_version, p.settlement_mode, p.provider_payment_fee, p.provider_payment_fee_rate_bps, p.provider_payment_fee_base_amount, p.provider_payment_fee_source, p.merchant_payment_fee, p.merchant_payment_fee_rate_bps, p.merchant_payment_fee_base_amount, p.rider_gross_amount, p.rider_payment_fee, p.rider_payment_fee_rate_bps, p.rider_payment_fee_base_amount, p.commission_base_amount, p.platform_receiver_amount,
    po.order_id,
    o.order_no,
    m.name as merchant_name
FROM profit_sharing_orders p
JOIN payment_orders po ON po.id = p.payment_order_id
JOIN orders o ON o.id = po.order_id
JOIN merchants m ON m.id = p.merchant_id
WHERE p.rider_id = sqlc.arg('rider_id')
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND p.created_at >= sqlc.arg('start_at') AND p.created_at <= sqlc.arg('end_at')
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountRiderProfitSharingOrders :one
-- 骑手代取费明细总数
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE rider_id = sqlc.arg('rider_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at');

-- name: GetRiderProfitSharingStatusSummary :many
-- 骑手代取费按分账状态汇总
SELECT
    status,
    COUNT(*)::bigint as order_count,
    COALESCE(SUM(rider_amount), 0)::bigint as rider_amount,
    COALESCE(SUM(delivery_fee), 0)::bigint as delivery_fee,
    COALESCE(SUM(CASE WHEN rider_gross_amount > 0 THEN rider_gross_amount ELSE delivery_fee END), 0)::bigint as rider_gross_amount,
    COALESCE(SUM(rider_payment_fee), 0)::bigint as rider_payment_fee
FROM profit_sharing_orders
WHERE rider_id = sqlc.arg('rider_id')
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
GROUP BY status
ORDER BY status;

-- name: GetRiderDailyIncome :many
-- 骑手每日收入汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as delivery_count,
    COALESCE(SUM(rider_amount), 0)::bigint as daily_income,
    COALESCE(SUM(CASE WHEN rider_gross_amount > 0 THEN rider_gross_amount ELSE delivery_fee END), 0)::bigint as rider_gross_amount,
    COALESCE(SUM(rider_payment_fee), 0)::bigint as rider_payment_fee
FROM profit_sharing_orders
WHERE rider_id = sqlc.arg('rider_id')
  AND status = 'finished'
  AND created_at >= sqlc.arg('start_at') AND created_at <= sqlc.arg('end_at')
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- name: ListCompletedOrdersMissingProfitSharing :many
SELECT po.id AS payment_order_id, po.order_id
FROM payment_orders po
JOIN orders o ON po.order_id = o.id
LEFT JOIN profit_sharing_orders pso ON po.id = pso.payment_order_id
WHERE 
    po.status = 'paid' 
    AND po.payment_channel = 'baofu_aggregate'
    AND po.requires_profit_sharing = TRUE
    AND o.status = 'completed'
  AND o.order_type <> 'takeout'
    AND pso.id IS NULL
    AND o.updated_at > now() - INTERVAL '7 days'
ORDER BY o.updated_at ASC, po.id ASC
LIMIT $1;

-- name: ListBaofuOrdersReadyForProfitSharing :many
SELECT
    po.id AS payment_order_id,
    po.order_id,
    po.reservation_id,
    po.business_type
FROM payment_orders po
LEFT JOIN orders o ON po.business_type = 'order' AND po.order_id = o.id
LEFT JOIN table_reservations r ON po.business_type IN ('reservation', 'reservation_addon') AND po.reservation_id = r.id
WHERE po.status = 'paid'
  AND po.payment_channel = 'baofu_aggregate'
  AND po.requires_profit_sharing = TRUE
  AND (
      (
          po.business_type = 'order'
          AND o.status = 'completed'
          AND COALESCE(o.completed_at, o.updated_at) <= sqlc.arg(refund_closed_before)
      )
      OR (
          po.business_type IN ('reservation', 'reservation_addon')
          AND r.status IN ('paid', 'confirmed', 'checked_in', 'completed')
          AND COALESCE(r.paid_at, r.updated_at, po.paid_at, po.created_at) <= sqlc.arg(refund_closed_before)
      )
  )
  AND NOT EXISTS (
      SELECT 1 FROM refund_orders ro
      WHERE ro.payment_order_id = po.id
        AND ro.status IN ('pending', 'processing', 'success')
  )
  AND NOT EXISTS (
      SELECT 1 FROM profit_sharing_orders pso
      WHERE pso.payment_order_id = po.id
  )
ORDER BY COALESCE(o.completed_at, o.updated_at, r.paid_at, r.updated_at, po.paid_at, po.created_at) ASC, po.id ASC
LIMIT sqlc.arg('limit')::int;

-- name: ListBaofuProcessingProfitSharingOrdersForRecovery :many
SELECT id, payment_order_id, merchant_id, operator_id, order_source, total_amount, platform_commission, operator_commission, merchant_amount, out_order_no, sharing_order_id, status, finished_at, created_at, delivery_fee, rider_id, rider_amount, distributable_amount, platform_rate, operator_rate, payment_fee, payment_fee_rate_bps, provider, channel, merchant_sharing_mer_id, rider_sharing_mer_id, operator_sharing_mer_id, platform_sharing_mer_id, sharing_detail_snapshot, calculation_version, settlement_mode, provider_payment_fee, provider_payment_fee_rate_bps, provider_payment_fee_base_amount, provider_payment_fee_source, merchant_payment_fee, merchant_payment_fee_rate_bps, merchant_payment_fee_base_amount, rider_gross_amount, rider_payment_fee, rider_payment_fee_rate_bps, rider_payment_fee_base_amount, commission_base_amount, platform_receiver_amount
FROM profit_sharing_orders
WHERE provider = 'baofu'
  AND channel = 'baofu_aggregate'
  AND status = 'processing'
  AND created_at <= sqlc.arg(created_before)
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg('limit')::int;
