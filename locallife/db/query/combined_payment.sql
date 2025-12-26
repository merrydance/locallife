-- ==================== 合单支付主表查询 ====================

-- name: CreateCombinedPaymentOrder :one
INSERT INTO combined_payment_orders (
    user_id,
    combine_out_trade_no,
    total_amount,
    status,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetCombinedPaymentOrder :one
SELECT * FROM combined_payment_orders
WHERE id = $1 LIMIT 1;

-- name: GetCombinedPaymentOrderForUpdate :one
SELECT * FROM combined_payment_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetCombinedPaymentOrderByOutTradeNo :one
SELECT * FROM combined_payment_orders
WHERE combine_out_trade_no = $1 LIMIT 1;

-- name: UpdateCombinedPaymentOrderPrepay :one
UPDATE combined_payment_orders
SET prepay_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdateCombinedPaymentOrderToPaid :one
UPDATE combined_payment_orders
SET 
    status = 'paid',
    transaction_id = $2,
    paid_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateCombinedPaymentOrderToFailed :one
UPDATE combined_payment_orders
SET status = 'failed'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateCombinedPaymentOrderToClosed :one
UPDATE combined_payment_orders
SET status = 'closed'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: ListUserCombinedPaymentOrders :many
SELECT * FROM combined_payment_orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingCombinedPaymentOrders :many
-- 查询待支付且已过期的合单（用于定时关闭）
SELECT * FROM combined_payment_orders
WHERE status = 'pending'
  AND expires_at < now()
ORDER BY created_at
LIMIT $1;


-- ==================== 合单支付子订单查询 ====================

-- name: CreateCombinedPaymentSubOrder :one
INSERT INTO combined_payment_sub_orders (
    combined_payment_id,
    order_id,
    merchant_id,
    sub_mchid,
    amount,
    out_trade_no,
    description
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetCombinedPaymentSubOrder :one
SELECT * FROM combined_payment_sub_orders
WHERE id = $1 LIMIT 1;

-- name: GetCombinedPaymentSubOrderByOutTradeNo :one
SELECT * FROM combined_payment_sub_orders
WHERE out_trade_no = $1 LIMIT 1;

-- name: ListCombinedPaymentSubOrders :many
SELECT * FROM combined_payment_sub_orders
WHERE combined_payment_id = $1
ORDER BY created_at;

-- name: ListCombinedPaymentSubOrdersWithMerchant :many
SELECT 
    s.*,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    o.order_no
FROM combined_payment_sub_orders s
JOIN merchants m ON m.id = s.merchant_id
JOIN orders o ON o.id = s.order_id
WHERE s.combined_payment_id = $1
ORDER BY s.created_at;

-- name: UpdateSubOrderProfitSharingStatus :one
UPDATE combined_payment_sub_orders
SET profit_sharing_status = $2
WHERE id = $1
RETURNING *;

-- name: CountCombinedPaymentSubOrders :one
SELECT COUNT(*)::int FROM combined_payment_sub_orders
WHERE combined_payment_id = $1;

-- name: GetCombinedPaymentSubOrdersByOrder :many
-- 根据订单ID查询其所有合单子单（一个订单可能参与多次合单支付尝试）
SELECT * FROM combined_payment_sub_orders
WHERE order_id = $1
ORDER BY created_at DESC;


-- ==================== 合单支付完整信息查询 ====================

-- name: GetCombinedPaymentOrderWithSubOrders :one
-- 获取合单支付完整信息（主单+所有子单）
SELECT 
    c.*,
    COALESCE(
        json_agg(
            json_build_object(
                'id', s.id,
                'order_id', s.order_id,
                'merchant_id', s.merchant_id,
                'sub_mchid', s.sub_mchid,
                'amount', s.amount,
                'out_trade_no', s.out_trade_no,
                'description', s.description,
                'profit_sharing_status', s.profit_sharing_status,
                'merchant_name', m.name,
                'merchant_logo', m.logo_url,
                'order_no', o.order_no
            ) ORDER BY s.created_at
        ) FILTER (WHERE s.id IS NOT NULL), '[]'
    )::json AS sub_orders
FROM combined_payment_orders c
LEFT JOIN combined_payment_sub_orders s ON s.combined_payment_id = c.id
LEFT JOIN merchants m ON m.id = s.merchant_id
LEFT JOIN orders o ON o.id = s.order_id
WHERE c.id = $1
GROUP BY c.id;
