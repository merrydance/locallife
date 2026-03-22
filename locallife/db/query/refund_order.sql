-- name: CreateRefundOrder :one
INSERT INTO refund_orders (
    payment_order_id,
    refund_type,
    refund_amount,
    refund_reason,
    out_refund_no,
    platform_refund,
    operator_refund,
    merchant_refund,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetRefundOrder :one
SELECT * FROM refund_orders
WHERE id = $1 LIMIT 1;

-- name: GetRefundOrderForUpdate :one
SELECT * FROM refund_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetRefundOrderByOutRefundNo :one
SELECT * FROM refund_orders
WHERE out_refund_no = $1 LIMIT 1;

-- name: GetRefundOrderByRefundId :one
SELECT * FROM refund_orders
WHERE refund_id = $1 LIMIT 1;

-- name: ListRefundOrdersByPaymentOrder :many
SELECT * FROM refund_orders
WHERE payment_order_id = $1
ORDER BY created_at DESC;

-- name: ListRefundOrdersByStatus :many
SELECT * FROM refund_orders
WHERE status = $1
ORDER BY created_at
LIMIT $2 OFFSET $3;

-- name: UpdateRefundOrderToProcessing :one
UPDATE refund_orders
SET
    status = 'processing',
    refund_id = $2
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateRefundOrderToSuccess :one
UPDATE refund_orders
SET
    status = 'success',
    refunded_at = now()
WHERE id = $1 AND status = 'processing'
RETURNING *;

-- name: UpdateRefundOrderToFailed :one
UPDATE refund_orders
SET
    status = 'failed'
WHERE id = $1
RETURNING *;

-- name: UpdateRefundOrderToClosed :one
UPDATE refund_orders
SET
    status = 'closed'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: GetTotalRefundedByPaymentOrder :one
SELECT COALESCE(SUM(refund_amount), 0)::bigint as total_refunded
FROM refund_orders
WHERE payment_order_id = $1 AND status = 'success';

-- name: ListRefundOrdersForReconciliation :many
-- 获取指定日期范围内直连支付（miniprogram/deposit等）成功退款订单（用于每日对账）
-- 通过 JOIN payment_orders 过滤 payment_type，排除收付通退款（已单独对账）
SELECT r.id, r.out_refund_no, r.refund_id, r.refund_amount, r.status
FROM refund_orders r
JOIN payment_orders p ON p.id = r.payment_order_id
WHERE r.status = 'success'
  AND r.refunded_at >= $1
  AND r.refunded_at < $2
  AND p.payment_type != 'profit_sharing';

-- name: ListEcommerceRefundOrdersForReconciliation :many
-- 获取指定日期范围内收付通退款成功记录（payment_type='profit_sharing'）
-- 对应微信 /v3/ecommerce/refunds/apply 产生的退款账单
SELECT r.id, r.out_refund_no, r.refund_id, r.refund_amount, r.status
FROM refund_orders r
JOIN payment_orders p ON p.id = r.payment_order_id
WHERE r.status = 'success'
  AND r.refunded_at >= $1
  AND r.refunded_at < $2
  AND p.payment_type = 'profit_sharing';
