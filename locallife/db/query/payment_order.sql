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
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestPaymentOrderByOrder :one
SELECT * FROM payment_orders
WHERE order_id = $1
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
