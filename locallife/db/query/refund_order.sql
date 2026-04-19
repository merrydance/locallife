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
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE id = $1 LIMIT 1;

-- name: GetRefundOrderForUpdate :one
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetRefundOrderByOutRefundNo :one
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE out_refund_no = $1 LIMIT 1;

-- name: GetRefundOrderByRefundId :one
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE refund_id = $1 LIMIT 1;

-- name: ListRefundOrdersByPaymentOrder :many
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE payment_order_id = $1
ORDER BY created_at DESC;

-- name: ListRefundOrdersByStatus :many
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE status = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: ListPendingReservationRefundOrdersForRecovery :many
SELECT
    ro.id,
    ro.payment_order_id,
    ro.refund_amount,
    ro.refund_reason,
    ro.out_refund_no,
    po.reservation_id,
    po.business_type
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE ro.status = 'pending'
    AND po.status = 'paid'
    AND po.reservation_id IS NOT NULL
    AND po.business_type IN ('reservation', 'reservation_addon')
    AND ro.created_at < sqlc.arg('created_before')
ORDER BY ro.created_at ASC, ro.id ASC
LIMIT sqlc.arg('limit')::int;

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
WHERE id = $1 AND status IN ('pending', 'processing')
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
WHERE id = $1 AND status IN ('pending', 'processing')
RETURNING *;

-- name: GetTotalRefundedByPaymentOrder :one
SELECT COALESCE(SUM(refund_amount), 0)::bigint as total_refunded
FROM refund_orders
WHERE payment_order_id = $1 AND status IN ('pending', 'processing', 'success');

-- name: GetPendingRiderDepositRefundAmountByUserID :one
SELECT COALESCE(SUM(ro.refund_amount), 0)::bigint AS pending_rider_deposit_refund_amount
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE po.user_id = $1
    AND po.business_type = 'rider_deposit'
    AND ro.refund_type = 'rider_deposit'
    AND ro.status IN ('pending', 'processing');

-- name: ListRefundOrdersForReconciliation :many
-- 获取指定日期范围内直连支付（miniprogram/deposit等）成功退款订单（用于每日对账）
-- 通过 JOIN payment_orders 过滤 payment_channel，排除收付通退款（已单独对账）
SELECT r.id, r.out_refund_no, r.refund_id, r.refund_amount, r.status
FROM refund_orders r
JOIN payment_orders p ON p.id = r.payment_order_id
WHERE r.status = 'success'
  AND r.refunded_at >= $1
  AND r.refunded_at < $2
    AND p.payment_channel = 'direct';

-- name: ListEcommerceRefundOrdersForReconciliation :many
-- 获取指定日期范围内收付通退款成功记录（payment_channel='ecommerce'）
-- 对应微信 /v3/ecommerce/refunds/apply 产生的退款账单
SELECT r.id, r.out_refund_no, r.refund_id, r.refund_amount, r.status
FROM refund_orders r
JOIN payment_orders p ON p.id = r.payment_order_id
WHERE r.status = 'success'
  AND r.refunded_at >= $1
  AND r.refunded_at < $2
    AND p.payment_channel = 'ecommerce';

-- name: ListStuckProcessingRefundOrders :many
-- 查找持续处于 processing 状态超过阈值时间的退款单（微信回调可能永久丢失）
-- 用于运营告警，让人工核查微信商户平台退款结果
SELECT ro.id, ro.out_refund_no, ro.refund_id, ro.refund_amount, ro.status, ro.created_at,
       po.payment_type
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE ro.status = 'processing'
  AND ro.created_at < sqlc.arg('created_before')
ORDER BY ro.created_at ASC, ro.id ASC
LIMIT sqlc.arg('limit')::int;
