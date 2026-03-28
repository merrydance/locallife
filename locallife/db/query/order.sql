-- name: CreateOrder :one
INSERT INTO orders (
    order_no,
    user_id,
    merchant_id,
    order_type,
    address_id,
    delivery_fee,
    delivery_distance,
    delivery_duration,
    table_id,
    reservation_id,
    subtotal,
    discount_amount,
    delivery_fee_discount,
    total_amount,
    status,
    fulfillment_status,
    notes,
    user_voucher_id,
    voucher_amount,
    balance_paid,
    membership_id,
    replaced_by_order_id,
    pickup_code
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
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
    m.address as merchant_address,
    ua.contact_name as delivery_contact_name,
    ua.contact_phone as delivery_contact_phone,
    ua.detail_address as delivery_address
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
LEFT JOIN user_addresses ua ON o.address_id = ua.id
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

-- name: ListOrdersByUserWithFilters :many
SELECT
        o.*,
        m.name as merchant_name
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
WHERE o.user_id = sqlc.arg('user_id')
    AND (sqlc.narg('status')::text IS NULL OR o.status = sqlc.narg('status'))
    AND (sqlc.narg('order_type')::text IS NULL OR o.order_type = sqlc.narg('order_type'))
    AND (sqlc.narg('reservation_id')::bigint IS NULL OR o.reservation_id IS NOT DISTINCT FROM sqlc.narg('reservation_id'))
ORDER BY o.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListOrdersByUserAndStatus :many
SELECT 
    o.*,
    m.name as merchant_name
FROM orders o
INNER JOIN merchants m ON o.merchant_id = m.id
WHERE o.user_id = $1 AND o.status = $2
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4;

-- name: HasUserOrderedFromMerchant :one
SELECT EXISTS(
    SELECT 1 FROM orders
    WHERE user_id = $1 AND merchant_id = $2
        AND status NOT IN ('cancelled')
) AS has_ordered;

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

-- name: ListOrdersByMerchantWithFilters :many
SELECT * FROM orders
WHERE merchant_id = sqlc.arg('merchant_id')
    AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
    AND (sqlc.narg('order_type')::text IS NULL OR order_type = sqlc.narg('order_type')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOrdersByMerchant :one
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1;

-- name: CountOrdersByMerchantAndStatus :one
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1 AND status = $2;

-- name: CountOrdersByMerchantWithFilters :one
SELECT COUNT(*) FROM orders
WHERE merchant_id = sqlc.arg('merchant_id')
    AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
    AND (sqlc.narg('order_type')::text IS NULL OR order_type = sqlc.narg('order_type')::text);

-- name: GetLatestOrderByReservation :one
SELECT * FROM orders
WHERE reservation_id = $1
    AND replaced_by_order_id IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateOrderStatus :one
UPDATE orders
SET 
    status = sqlc.arg('status'),
    fulfillment_status = COALESCE(sqlc.narg('fulfillment_status'), fulfillment_status),
    updated_at = now()
WHERE id = sqlc.arg('id')
    AND status = sqlc.arg('expected_status')
RETURNING *;

-- name: MarkOrderReplaced :one
UPDATE orders
SET 
    status = 'cancelled',
    fulfillment_status = 'cancelled',
    cancelled_at = now(),
    cancel_reason = COALESCE(sqlc.narg('cancel_reason'), cancel_reason),
    replaced_by_order_id = sqlc.arg('replaced_by_order_id'),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UpdateOrderToPaid :one
UPDATE orders
SET 
    status = 'paid',
    payment_method = sqlc.arg('payment_method'),
    paid_at = now(),
    fulfillment_status = COALESCE(sqlc.narg('fulfillment_status'), fulfillment_status),
    updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'pending'
RETURNING *;

-- name: UpdateOrderToCompleted :one
UPDATE orders
SET 
    status = 'completed',
    fulfillment_status = 'completed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1
    AND status NOT IN ('cancelled', 'completed')
RETURNING *;

-- name: UpdateOrderToCourierAccepted :one
UPDATE orders
SET 
    status = 'courier_accepted',
    courier_accept_at = COALESCE(courier_accept_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('ready', 'courier_accepted')
RETURNING *;

-- name: UpdateOrderToPicked :one
UPDATE orders
SET 
    status = 'picked',
    picked_at = COALESCE(picked_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('ready', 'courier_accepted', 'picked')
RETURNING *;

-- name: UpdateOrderToDelivering :one
UPDATE orders
SET 
    status = 'delivering',
    updated_at = now()
WHERE id = $1 AND status IN ('picked', 'delivering')
RETURNING *;

-- name: UpdateOrderToRiderDelivered :one
UPDATE orders
SET 
    status = 'rider_delivered',
    rider_delivered_at = COALESCE(rider_delivered_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('delivering', 'rider_delivered')
RETURNING *;

-- name: UpdateOrderToUserDelivered :one
UPDATE orders
SET 
    status = 'user_delivered',
    user_delivered_at = COALESCE(user_delivered_at, now()),
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('rider_delivered', 'user_delivered')
RETURNING *;

-- name: CompleteTakeoutOrderByUser :one
-- 用户点击完成（外卖）：直接进入 completed，并补齐 user_delivered_at
UPDATE orders
SET
        status = 'completed',
        fulfillment_status = 'completed',
        user_delivered_at = COALESCE(user_delivered_at, now()),
        completed_at = COALESCE(completed_at, now()),
        updated_at = now()
WHERE id = $1
    AND order_type = 'takeout'
    AND status IN ('rider_delivered', 'user_delivered')
RETURNING *;

-- name: AutoCompleteTakeoutOrder :one
-- 系统自动完成（外卖）：1h 未手动完成且无索赔时触发，记录 auto_user_delivered_at
UPDATE orders
SET
        status = 'completed',
        fulfillment_status = 'completed',
        user_delivered_at = COALESCE(user_delivered_at, now()),
        auto_user_delivered_at = COALESCE(auto_user_delivered_at, now()),
        completed_at = COALESCE(completed_at, now()),
        updated_at = now()
WHERE id = $1
    AND order_type = 'takeout'
    AND status IN ('rider_delivered', 'user_delivered')
RETURNING *;

-- name: UpdateOrderExceptionState :one
UPDATE orders
SET 
    exception_state = sqlc.arg('exception_state'),
    claim_channel = sqlc.arg('claim_channel'),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UpdateOrderToCancelled :one
UPDATE orders
SET 
    status = 'cancelled',
    fulfillment_status = 'cancelled',
    cancelled_at = now(),
    cancel_reason = sqlc.arg('cancel_reason'),
    updated_at = now()
WHERE id = sqlc.arg('id')
    AND status = sqlc.arg('expected_status')
RETURNING *;

-- name: GetOrderStats :one
SELECT 
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'paid') as paid_count,
    COUNT(*) FILTER (WHERE status = 'preparing') as preparing_count,
    COUNT(*) FILTER (WHERE status = 'ready') as ready_count,
    COUNT(*) FILTER (WHERE status IN ('courier_accepted', 'picked', 'delivering', 'rider_delivered')) as delivering_count,
    COUNT(*) FILTER (WHERE status IN ('completed', 'user_delivered')) as completed_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_count
FROM orders
WHERE merchant_id = sqlc.arg(merchant_id)
    AND created_at >= sqlc.arg(start_at)
    AND created_at <= sqlc.arg(end_at);

-- ==================== KDS 厨房显示系统查询 ====================

-- name: ListMerchantOrdersByStatus :many
-- 根据商户ID和状态查询订单（用于厨房显示）
SELECT * FROM orders
WHERE merchant_id = $1 AND status = $2 AND replaced_by_order_id IS NULL
ORDER BY created_at ASC
LIMIT $3 OFFSET $4;

-- name: CountMerchantOrdersByStatusAfterTime :one
-- 统计商户在某时间后特定状态的订单数
SELECT COUNT(*) FROM orders
WHERE merchant_id = $1 
    AND status = $2 
    AND created_at >= $3
    AND replaced_by_order_id IS NULL;

-- name: CountOrderUrges :one
-- 统计订单被催单次数（从状态日志表查询催单记录）
SELECT COUNT(*)::bigint FROM order_status_logs
WHERE order_id = $1 AND notes LIKE '%催单%';

-- name: UpdateOrderToPreparing :one
-- P1-035 修复：带状态前置条件的厨房状态变更，防止并发竞态
UPDATE orders
SET 
    status = 'preparing',
    fulfillment_status = 'preparing',
    prep_start_at = COALESCE(prep_start_at, now()),
    updated_at = now()
WHERE id = $1 AND status = 'paid'
RETURNING *;

-- name: UpdateOrderToReady :one
-- P1-035 修复：带状态前置条件的厨房状态变更，防止并发竞态
UPDATE orders
SET 
    status = 'ready',
    fulfillment_status = 'ready',
    ready_at = COALESCE(ready_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('paid', 'preparing')
RETURNING *;

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
        AND o.created_at >= sqlc.arg('start_at');

-- ==================== 商户财务相关查询 ====================

-- name: GetMerchantPromotionExpenses :one
-- 统计商户满返运费支出
SELECT 
    COUNT(*) FILTER (WHERE delivery_fee_discount > 0) as promo_order_count,
    COALESCE(SUM(delivery_fee_discount), 0)::bigint as total_discount
FROM orders
WHERE merchant_id = sqlc.arg(merchant_id)
    AND status IN ('user_delivered', 'completed')
    AND created_at >= sqlc.arg(start_at)
    AND created_at <= sqlc.arg(end_at);

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
WHERE merchant_id = sqlc.arg('merchant_id')
        AND delivery_fee_discount > 0
        AND status IN ('user_delivered', 'completed')
    AND created_at >= sqlc.arg('start_at')
    AND created_at <= sqlc.arg('end_at')
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantPromotionOrders :one
SELECT COUNT(*)::bigint
FROM orders
WHERE merchant_id = sqlc.arg('merchant_id')
    AND delivery_fee_discount > 0
    AND status IN ('user_delivered', 'completed')
    AND created_at >= sqlc.arg('start_at')
    AND created_at <= sqlc.arg('end_at');

-- name: GetMerchantDailyPromotionExpenses :many
-- 商户每日满返支出汇总
SELECT 
    DATE(created_at) AS date,
    COUNT(*) as order_count,
    COALESCE(SUM(delivery_fee_discount), 0)::bigint as promotion_amount
FROM orders
WHERE merchant_id = sqlc.arg('merchant_id')
        AND delivery_fee_discount > 0
        AND status IN ('user_delivered', 'completed')
    AND created_at >= sqlc.arg('start_at')
    AND created_at <= sqlc.arg('end_at')
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- ==================== 订单超时清理 ====================

-- name: ListPendingOrdersBefore :many
-- 获取超时未支付的 pending 订单（创建时间早于指定时间）
SELECT * FROM orders
WHERE status = sqlc.arg('status')
  AND created_at < sqlc.arg('created_at')
ORDER BY created_at ASC
LIMIT sqlc.arg('limit');

-- name: ListTakeoutOrdersDeliveredBefore :many
-- 获取已送达但未完成超过一定时间的外卖订单（用于自动完成）
SELECT * FROM orders
WHERE order_type = 'takeout'
    AND status IN ('rider_delivered', 'user_delivered')
    AND rider_delivered_at IS NOT NULL
    AND rider_delivered_at < sqlc.arg('delivered_before')
    AND replaced_by_order_id IS NULL
ORDER BY rider_delivered_at ASC
LIMIT sqlc.arg('limit');
