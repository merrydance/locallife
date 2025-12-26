-- name: CreateOrder :one
INSERT INTO orders (
    order_no,
    user_id,
    merchant_id,
    order_type,
    address_id,
    delivery_fee,
    delivery_distance,
    table_id,
    reservation_id,
    subtotal,
    discount_amount,
    delivery_fee_discount,
    total_amount,
    status,
    notes,
    user_voucher_id,
    voucher_amount,
    balance_paid,
    membership_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
) RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders
WHERE id = $1 LIMIT 1;

-- name: GetOrderForUpdate :one
SELECT * FROM orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetOrderByOrderNo :one
SELECT * FROM orders
WHERE order_no = $1 LIMIT 1;

-- name: GetOrderWithDetails :one
SELECT 
    o.*,
    m.name as merchant_name,
    m.phone as merchant_phone,
    m.address as merchant_address
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
WHERE o.id = $1;

-- name: ListOrdersByUser :many
SELECT 
    o.*,
    m.name as merchant_name
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
WHERE o.user_id = $1
ORDER BY o.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListOrdersByUserAndStatus :many
SELECT 
    o.*,
    m.name as merchant_name
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
WHERE o.user_id = $1 AND o.status = $2
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListOrdersByMerchant :many
SELECT * FROM orders
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListOrdersByMerchantAndStatus :many
SELECT * FROM orders
WHERE merchant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListOrdersByMerchantAndStatuses :many
SELECT * FROM orders
WHERE merchant_id = $1 AND status = ANY($2::text[])
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountOrdersByMerchant :one
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1;

-- name: CountOrdersByMerchantAndStatus :one
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1 AND status = $2;

-- name: UpdateOrderStatus :one
UPDATE orders
SET 
    status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOrderToPaid :one
UPDATE orders
SET 
    status = 'paid',
    payment_method = $2,
    paid_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateOrderToCompleted :one
UPDATE orders
SET 
    status = 'completed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOrderToCancelled :one
UPDATE orders
SET 
    status = 'cancelled',
    cancelled_at = now(),
    cancel_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetOrderStats :one
SELECT 
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'paid') as paid_count,
    COUNT(*) FILTER (WHERE status = 'preparing') as preparing_count,
    COUNT(*) FILTER (WHERE status = 'ready') as ready_count,
    COUNT(*) FILTER (WHERE status = 'delivering') as delivering_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_count
FROM orders
WHERE merchant_id = $1 AND created_at >= $2 AND created_at <= $3;

-- ==================== KDS 厨房显示系统查询 ====================

-- name: ListMerchantOrdersByStatus :many
-- 根据商户ID和状态查询订单（用于厨房显示）
SELECT * FROM orders
WHERE merchant_id = $1 AND status = $2
ORDER BY created_at ASC
LIMIT $3 OFFSET $4;

-- name: CountMerchantOrdersByStatusAfterTime :one
-- 统计商户在某时间后特定状态的订单数
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1 
  AND status = $2 
  AND created_at >= $3;

-- name: CountOrderUrges :one
-- 统计订单被催单次数（从状态日志表查询催单记录）
SELECT COUNT(*)::bigint FROM order_status_logs
WHERE order_id = $1 AND notes LIKE '%催单%';

-- name: GetMerchantAvgPrepareTime :one
-- 计算商户近N天的平均出餐时间（分钟）
-- 通过订单支付时间到状态变为ready的时间差计算
SELECT COALESCE(
    ROUND(AVG(
        EXTRACT(EPOCH FROM (osl.created_at - o.paid_at)) / 60
    )),
    0
)::bigint as avg_prepare_minutes
FROM orders o
INNER JOIN order_status_logs osl ON o.id = osl.order_id
WHERE o.merchant_id = $1
  AND o.paid_at IS NOT NULL
  AND osl.to_status = 'ready'
  AND o.created_at >= $2;

-- ==================== 商户财务相关查询 ====================

-- name: GetMerchantPromotionExpenses :one
-- 统计商户满返运费支出
SELECT 
    COUNT(*) FILTER (WHERE delivery_fee_discount > 0) as promo_order_count,
    COALESCE(SUM(delivery_fee_discount), 0)::bigint as total_discount
FROM orders
WHERE merchant_id = $1
  AND status IN ('delivered', 'completed')
  AND created_at >= $2 AND created_at <= $3;

-- name: ListMerchantPromotionOrders :many
-- 商户满返支出明细
SELECT 
    id,
    order_no,
    order_type,
    subtotal,
    delivery_fee,
    delivery_fee_discount,
    total_amount,
    created_at,
    completed_at
FROM orders
WHERE merchant_id = $1
  AND delivery_fee_discount > 0
  AND status IN ('delivered', 'completed')
  AND created_at >= $2 AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountMerchantPromotionOrders :one
SELECT COUNT(*)::bigint
FROM orders
WHERE merchant_id = $1
  AND delivery_fee_discount > 0
  AND status IN ('delivered', 'completed')
  AND created_at >= $2 AND created_at <= $3;

-- name: GetMerchantDailyPromotionExpenses :many
-- 商户每日满返支出汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as order_count,
    COALESCE(SUM(delivery_fee_discount), 0)::bigint as promotion_amount
FROM orders
WHERE merchant_id = $1
  AND delivery_fee_discount > 0
  AND status IN ('delivered', 'completed')
  AND created_at >= $2 AND created_at <= $3
GROUP BY DATE(created_at)
ORDER BY date DESC;
