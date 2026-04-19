-- name: CreatePaymentOrder :one
INSERT INTO payment_orders (
    order_id,
    reservation_id,
    user_id,
    payment_type,
    payment_channel,
    requires_profit_sharing,
    business_type,
    amount,
    out_trade_no,
    expires_at,
    attach
) VALUES (
    sqlc.arg(order_id),
    sqlc.arg(reservation_id),
    sqlc.arg(user_id),
    sqlc.arg(payment_type),
    sqlc.arg(payment_channel),
    sqlc.arg(requires_profit_sharing),
    sqlc.arg(business_type),
    sqlc.arg(amount),
    sqlc.arg(out_trade_no),
    sqlc.arg(expires_at),
    sqlc.arg(attach)
) RETURNING *;

-- name: GetPaymentOrder :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE id = $1 LIMIT 1;

-- name: GetPaymentOrderForUpdate :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetPaymentOrderByOutTradeNo :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE out_trade_no = $1 LIMIT 1;

-- name: GetPaymentOrderByTransactionId :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE transaction_id = $1 LIMIT 1;

-- name: GetPaymentOrdersByOrder :many
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: GetPaymentOrdersByReservation :many
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE reservation_id = $1
ORDER BY created_at DESC;

-- name: GetLatestPaymentOrderByReservation :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE reservation_id = $1
    AND business_type = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestPaymentOrderByOrder :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE order_id = $1
    AND business_type = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestPaymentOrderByBusinessTypeAndAttach :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE business_type = $1
    AND attach = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPaymentOrdersByUser :many
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListPaymentLedgerEntriesByUser :many
SELECT id, entry_type, payment_order_id, refund_order_id, order_id, business_type, amount, status, occurred_at, created_at
FROM (
    SELECT
        po.id AS id,
        'payment'::text AS entry_type,
        po.id AS payment_order_id,
        NULL::bigint AS refund_order_id,
        po.order_id,
        po.business_type,
        po.amount,
        po.status,
        COALESCE(po.paid_at, po.created_at) AS occurred_at,
        po.created_at
    FROM payment_orders po
    WHERE po.user_id = $1

    UNION ALL

    SELECT
        ro.id AS id,
        'refund'::text AS entry_type,
        ro.payment_order_id,
        ro.id AS refund_order_id,
        po.order_id,
        po.business_type,
        ro.refund_amount AS amount,
        ro.status,
        COALESCE(ro.refunded_at, ro.created_at) AS occurred_at,
        ro.created_at
    FROM refund_orders ro
    JOIN payment_orders po ON po.id = ro.payment_order_id
    WHERE po.user_id = $1
) AS ledger_entries
ORDER BY occurred_at DESC, created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountPaymentLedgerEntriesByUser :one
SELECT COUNT(*)::bigint
FROM (
    SELECT po.id
    FROM payment_orders po
    WHERE po.user_id = $1

    UNION ALL

    SELECT ro.id
    FROM refund_orders ro
    JOIN payment_orders po ON po.id = ro.payment_order_id
    WHERE po.user_id = $1
) AS ledger_entries;

-- name: ListPaymentOrdersByUserAndStatus :many
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE user_id = $1 AND status = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: UpdatePaymentOrderPrepayId :one
UPDATE payment_orders
SET
    prepay_id = $2
WHERE id = $1
    AND status = 'pending'
    AND prepay_id IS NULL
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
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE status = 'pending' AND expires_at < now()
ORDER BY created_at
LIMIT $1;

-- name: ListPaidUnprocessedPaymentOrders :many
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE status = 'paid'
    AND processed_at IS NULL
    AND paid_at <= $1
    AND NOT EXISTS (
            SELECT 1 FROM refund_orders ro
            WHERE ro.payment_order_id = payment_orders.id
    )
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
SELECT po.id, po.order_id, po.reservation_id, po.user_id, po.payment_type, po.business_type, po.amount, po.out_trade_no, po.transaction_id, po.prepay_id, po.status, po.paid_at, po.created_at, po.expires_at, po.attach, po.combined_payment_id, po.processed_at, po.payment_channel, po.requires_profit_sharing
FROM payment_orders po
JOIN orders o ON po.order_id = o.id
WHERE 
    po.status = 'paid' 
    AND po.business_type = 'order' 
    AND o.status = 'cancelled'
    AND po.created_at > now() - INTERVAL '7 days'
    AND NOT EXISTS (
        SELECT 1 FROM refund_orders ro 
        WHERE ro.payment_order_id = po.id 
        AND ro.status IN ('pending', 'processing', 'success')
    )
ORDER BY po.created_at
LIMIT $1;

-- name: ListPaidUnrefundedReservationPaymentOrders :many
SELECT po.id, po.order_id, po.reservation_id, po.user_id, po.payment_type, po.business_type, po.amount, po.out_trade_no, po.transaction_id, po.prepay_id, po.status, po.paid_at, po.created_at, po.expires_at, po.attach, po.combined_payment_id, po.processed_at, po.payment_channel, po.requires_profit_sharing
FROM payment_orders po
JOIN table_reservations r ON po.reservation_id = r.id
WHERE 
    po.status = 'paid' 
    AND po.business_type = 'reservation' 
    AND r.status = 'cancelled'
    AND po.created_at > now() - INTERVAL '7 days'
    AND NOT EXISTS (
        SELECT 1 FROM refund_orders ro 
        WHERE ro.payment_order_id = po.id 
        AND ro.status IN ('pending', 'processing', 'success')
    )
ORDER BY po.created_at
LIMIT $1;

-- name: GetPendingPaymentOrderByUserAndBusinessType :one
SELECT id, order_id, reservation_id, user_id, payment_type, business_type, amount, out_trade_no, transaction_id, prepay_id, status, paid_at, created_at, expires_at, attach, combined_payment_id, processed_at, payment_channel, requires_profit_sharing FROM payment_orders
WHERE user_id = $1
    AND business_type = $2
    AND amount = $3
    AND status = 'pending'
    AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1;

-- name: SetPaymentOrderCombinedID :one
UPDATE payment_orders
SET combined_payment_id = $2
WHERE id = $1
RETURNING *;

-- name: ListMiniprogramPaymentOrdersForReconciliation :many
-- 获取指定日期范围内所有小程序直连支付订单（用于每日对账）
SELECT id, out_trade_no, transaction_id, amount, status
FROM payment_orders
WHERE payment_channel = 'direct'
  AND status IN ('paid', 'refunded')
  AND paid_at >= $1
  AND paid_at < $2;

