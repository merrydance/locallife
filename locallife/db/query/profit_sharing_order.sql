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
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
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

-- name: GetProfitSharingOrder :one
SELECT * FROM profit_sharing_orders
WHERE id = $1 LIMIT 1;

-- name: GetProfitSharingOrderForUpdate :one
SELECT * FROM profit_sharing_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetProfitSharingOrderByOutOrderNo :one
SELECT * FROM profit_sharing_orders
WHERE out_order_no = $1 LIMIT 1;

-- name: GetProfitSharingOrderByPaymentOrder :one
SELECT * FROM profit_sharing_orders
WHERE payment_order_id = $1 LIMIT 1;

-- name: ListProfitSharingOrdersByMerchant :many
SELECT * FROM profit_sharing_orders
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListProfitSharingOrdersByOperator :many
SELECT * FROM profit_sharing_orders
WHERE operator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListProfitSharingOrdersByStatus :many
SELECT * FROM profit_sharing_orders
WHERE status = $1
ORDER BY created_at
LIMIT $2 OFFSET $3;

-- name: UpdateProfitSharingOrderToProcessing :one
UPDATE profit_sharing_orders
SET
    status = 'processing',
    sharing_order_id = $2
WHERE id = $1 AND status = 'pending'
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
WHERE id = $1
RETURNING *;

-- name: GetMerchantProfitSharingStats :one
SELECT 
    COUNT(*) as total_orders,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(merchant_amount), 0)::bigint as total_merchant_amount,
    COALESCE(SUM(platform_commission), 0)::bigint as total_platform_commission,
    COALESCE(SUM(operator_commission), 0)::bigint as total_operator_commission
FROM profit_sharing_orders
WHERE merchant_id = $1 AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3;

-- name: GetOperatorProfitSharingStats :one
SELECT 
    COUNT(*) as total_orders,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(operator_commission), 0)::bigint as total_operator_commission
FROM profit_sharing_orders
WHERE operator_id = $1 AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3;

-- name: ListMerchantFinanceOrders :many
-- 商户财务订单明细（带分账信息）
SELECT 
    p.id,
    p.payment_order_id,
    p.order_source,
    p.total_amount,
    p.platform_commission,
    p.operator_commission,
    p.merchant_amount,
    p.status,
    p.created_at,
    p.finished_at,
    po.order_id
FROM profit_sharing_orders p
JOIN payment_orders po ON po.id = p.payment_order_id
WHERE p.merchant_id = $1
  AND p.created_at >= $2 AND p.created_at <= $3
ORDER BY p.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountMerchantFinanceOrders :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND created_at >= $2 AND created_at <= $3;

-- name: GetMerchantFinanceOverview :one
-- 商户财务概览：统计收入、服务费、净收入
SELECT 
    COUNT(*) FILTER (WHERE status = 'finished') as completed_orders,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_orders,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN total_amount ELSE 0 END), 0)::bigint as total_gmv,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN merchant_amount ELSE 0 END), 0)::bigint as total_income,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN platform_commission ELSE 0 END), 0)::bigint as total_platform_fee,
    COALESCE(SUM(CASE WHEN status = 'finished' THEN operator_commission ELSE 0 END), 0)::bigint as total_operator_fee,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN merchant_amount ELSE 0 END), 0)::bigint as pending_income
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND created_at >= $2 AND created_at <= $3;

-- name: GetMerchantServiceFeeDetail :many
-- 商户服务费明细
SELECT 
    DATE(created_at) AS date,
    order_source,
    COUNT(*) as order_count,
    COALESCE(SUM(total_amount), 0)::bigint as total_amount,
    COALESCE(SUM(platform_commission), 0)::bigint as platform_fee,
    COALESCE(SUM(operator_commission), 0)::bigint as operator_fee
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3
GROUP BY DATE(created_at), order_source
ORDER BY date DESC, order_source;

-- name: GetMerchantDailyFinance :many
-- 商户每日财务汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as order_count,
    COALESCE(SUM(total_amount), 0)::bigint as total_gmv,
    COALESCE(SUM(merchant_amount), 0)::bigint as merchant_income,
    COALESCE(SUM(platform_commission + operator_commission), 0)::bigint as total_fee
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- name: ListMerchantSettlements :many
-- 商户结算记录（带日期范围和状态筛选）
SELECT *
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND created_at >= $2 AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: ListMerchantSettlementsByStatus :many
-- 商户结算记录（带日期范围和状态筛选）
SELECT *
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND status = $2
  AND created_at >= $3 AND created_at <= $4
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountMerchantSettlements :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND created_at >= $2 AND created_at <= $3;

-- name: CountMerchantSettlementsByStatus :one
SELECT COUNT(*)::bigint
FROM profit_sharing_orders
WHERE merchant_id = $1
  AND status = $2
  AND created_at >= $3 AND created_at <= $4;

-- ==================== 骑手分账查询 ====================

-- name: GetRiderProfitSharingStats :one
-- 骑手配送费收入统计
SELECT 
    COUNT(*) as total_deliveries,
    COALESCE(SUM(rider_amount), 0)::bigint as total_rider_income,
    COALESCE(SUM(delivery_fee), 0)::bigint as total_delivery_fee
FROM profit_sharing_orders
WHERE rider_id = $1 AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3;

-- name: ListRiderProfitSharingOrders :many
-- 骑手配送费明细
SELECT 
    p.*,
    po.order_id,
    o.order_no,
    m.name as merchant_name
FROM profit_sharing_orders p
JOIN payment_orders po ON po.id = p.payment_order_id
JOIN orders o ON o.id = po.order_id
JOIN merchants m ON m.id = p.merchant_id
WHERE p.rider_id = $1
  AND p.created_at >= $2 AND p.created_at <= $3
ORDER BY p.created_at DESC
LIMIT $4 OFFSET $5;

-- name: GetRiderDailyIncome :many
-- 骑手每日收入汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as delivery_count,
    COALESCE(SUM(rider_amount), 0)::bigint as daily_income
FROM profit_sharing_orders
WHERE rider_id = $1
  AND status = 'finished'
  AND created_at >= $2 AND created_at <= $3
GROUP BY DATE(created_at)
ORDER BY date DESC;
