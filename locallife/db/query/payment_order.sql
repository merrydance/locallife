-- name: CreatePaymentOrder :one
INSERT INTO payment_orders (
    order_id,
    reservation_id,
    user_id,
    payment_type,
    business_type,
    amount,
    out_trade_no,
    expires_at,
    attach
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetPaymentOrder :one
SELECT * FROM payment_orders
WHERE id = $1 LIMIT 1;

-- name: GetPaymentOrderForUpdate :one
SELECT * FROM payment_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetPaymentOrderByOutTradeNo :one
SELECT * FROM payment_orders
WHERE out_trade_no = $1 LIMIT 1;

-- name: GetPaymentOrderByTransactionId :one
SELECT * FROM payment_orders
WHERE transaction_id = $1 LIMIT 1;

-- name: GetPaymentOrdersByOrder :many
SELECT * FROM payment_orders
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: GetPaymentOrdersByReservation :many
SELECT * FROM payment_orders
WHERE reservation_id = $1
ORDER BY created_at DESC;

-- name: GetLatestPaymentOrderByReservation :one
SELECT * FROM payment_orders
WHERE reservation_id = $1
    AND business_type = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestPaymentOrderByOrder :one
SELECT * FROM payment_orders
WHERE order_id = $1
    AND business_type = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPaymentOrdersByUser :many
SELECT * FROM payment_orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPaymentOrdersByUserAndStatus :many
SELECT * FROM payment_orders
WHERE user_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdatePaymentOrderPrepayId :one
UPDATE payment_orders
SET
    prepay_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdatePaymentOrderToPaid :one
UPDATE payment_orders
SET
    status = 'paid',
    transaction_id = $2,
    paid_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdatePaymentOrderToFailed :one
UPDATE payment_orders
SET
    status = 'failed'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdatePaymentOrderToClosed :one
UPDATE payment_orders
SET
    status = 'closed'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdatePaymentOrderToRefunded :one
UPDATE payment_orders
SET
    status = 'refunded'
WHERE id = $1
RETURNING *;

-- name: ListExpiredPaymentOrders :many
SELECT * FROM payment_orders
WHERE status = 'pending' AND expires_at < now()
ORDER BY created_at
LIMIT $1;

-- name: ListPaidUnprocessedPaymentOrders :many
SELECT * FROM payment_orders
WHERE status = 'paid' AND processed_at IS NULL AND paid_at <= $1
ORDER BY paid_at
LIMIT $2;

-- name: UpdatePaymentOrderProcessedAt :one
UPDATE payment_orders
SET
    processed_at = now()
WHERE id = $1 AND status = 'paid' AND processed_at IS NULL
RETURNING *;

-- name: CloseExpiredPaymentOrders :execrows
-- 批量关闭过期的 pending 支付订单
UPDATE payment_orders
SET status = 'closed'
WHERE status = 'pending' AND expires_at < now();

-- name: ListPaidUnrefundedPaymentOrders :many
SELECT po.*
FROM payment_orders po
JOIN orders o ON po.order_id = o.id
WHERE 
    po.status = 'paid' 
    AND po.business_type = 'order' 
    AND o.status = 'cancelled'
    AND po.created_at > now() - INTERVAL '7 days'
ORDER BY po.created_at
LIMIT $1;

-- name: SetPaymentOrderCombinedID :one
UPDATE payment_orders
SET combined_payment_id = $2
WHERE id = $1
RETURNING *;

