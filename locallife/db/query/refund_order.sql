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
