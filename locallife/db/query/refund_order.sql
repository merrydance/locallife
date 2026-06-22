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

-- name: GetRefundRequestIdempotency :one
SELECT id, operation_scope, actor_user_id, idempotency_key, request_hash, refund_order_id, created_at, updated_at
FROM refund_request_idempotency
WHERE operation_scope = $1
  AND actor_user_id = $2
  AND idempotency_key = $3
LIMIT 1;

-- name: GetRefundRequestIdempotencyForUpdate :one
SELECT id, operation_scope, actor_user_id, idempotency_key, request_hash, refund_order_id, created_at, updated_at
FROM refund_request_idempotency
WHERE operation_scope = $1
  AND actor_user_id = $2
  AND idempotency_key = $3
LIMIT 1
FOR UPDATE;

-- name: CreateRefundRequestIdempotency :one
INSERT INTO refund_request_idempotency (
    operation_scope,
    actor_user_id,
    idempotency_key,
    request_hash,
    refund_order_id
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (operation_scope, actor_user_id, idempotency_key) DO UPDATE
SET updated_at = refund_request_idempotency.updated_at
WHERE refund_request_idempotency.request_hash = EXCLUDED.request_hash
RETURNING id, operation_scope, actor_user_id, idempotency_key, request_hash, refund_order_id, created_at, updated_at;

-- name: CreateRiderDepositWithdrawalRequest :one
INSERT INTO rider_deposit_withdrawal_requests (
    user_id,
    idempotency_key,
    request_hash,
    requested_amount,
    accepted_amount,
    refund_order_ids
) VALUES (
    sqlc.arg(user_id),
    sqlc.arg(idempotency_key),
    sqlc.arg(request_hash),
    sqlc.arg(requested_amount),
    COALESCE(sqlc.narg(accepted_amount), 0),
    COALESCE(sqlc.narg(refund_order_ids), '[]'::jsonb)
)
RETURNING id, user_id, idempotency_key, request_hash, requested_amount, accepted_amount, refund_order_ids, created_at, updated_at;

-- name: GetRiderDepositWithdrawalRequestForUpdate :one
SELECT id, user_id, idempotency_key, request_hash, requested_amount, accepted_amount, refund_order_ids, created_at, updated_at
FROM rider_deposit_withdrawal_requests
WHERE user_id = sqlc.arg(user_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
LIMIT 1
FOR UPDATE;

-- name: UpdateRiderDepositWithdrawalRequestRefundOrders :one
UPDATE rider_deposit_withdrawal_requests
SET accepted_amount = sqlc.arg(accepted_amount),
    refund_order_ids = sqlc.arg(refund_order_ids),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, user_id, idempotency_key, request_hash, requested_amount, accepted_amount, refund_order_ids, created_at, updated_at;

-- name: ListRefundOrdersByPaymentOrder :many
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE payment_order_id = $1
ORDER BY created_at DESC;

-- name: ListRefundOrdersByStatus :many
SELECT id, payment_order_id, refund_type, refund_amount, refund_reason, out_refund_no, refund_id, platform_refund, operator_refund, merchant_refund, status, refunded_at, created_at FROM refund_orders
WHERE status = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: ListPendingOrderRefundOrdersForRecovery :many
SELECT
    ro.id,
    ro.payment_order_id,
    ro.refund_amount,
    ro.refund_reason,
    ro.out_refund_no,
    po.order_id,
    po.business_type
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
JOIN orders o ON o.id = po.order_id
WHERE ro.status = 'pending'
    AND po.status = 'paid'
    AND po.order_id IS NOT NULL
    AND po.business_type = 'order'
    AND o.status = 'cancelled'
    AND ro.created_at < sqlc.arg('created_before')
ORDER BY ro.created_at ASC, ro.id ASC
LIMIT sqlc.arg('limit')::int;

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

-- name: ListPendingRiderDepositRefundOrdersForRecovery :many
SELECT
    ro.id,
    ro.payment_order_id,
    ro.refund_amount,
    ro.out_refund_no,
    po.business_type,
    po.payment_channel
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
LEFT JOIN external_payment_commands epc
    ON epc.provider = 'wechat'
   AND epc.channel = 'direct'
   AND epc.capability = 'direct_refund'
   AND epc.command_type = 'create_refund'
   AND epc.external_object_type = 'refund'
   AND epc.external_object_key = ro.out_refund_no
WHERE ro.status = 'pending'
  AND po.status = 'paid'
  AND po.business_type = 'rider_deposit'
  AND po.payment_channel = 'direct'
  AND ro.refund_type = 'rider_deposit'
  AND ro.created_at < sqlc.arg('created_before')
  AND (epc.id IS NULL OR epc.command_status = 'unknown')
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
WHERE id = $1 AND status IN ('pending', 'processing')
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

-- name: GetTotalActiveRefundedByPaymentOrder :one
SELECT COALESCE(SUM(refund_amount), 0)::bigint as total_active_refunded
FROM refund_orders
WHERE payment_order_id = $1 AND status IN ('pending', 'processing');

-- name: GetTotalSuccessfulRefundedByPaymentOrder :one
SELECT COALESCE(SUM(refund_amount), 0)::bigint as total_successful_refunded
FROM refund_orders
WHERE payment_order_id = $1 AND status = 'success';

-- name: GetBaofuPaymentOrderRefundGuardForUpdate :one
SELECT po.id,
       po.status,
       po.payment_channel,
       EXISTS (
           SELECT 1 FROM profit_sharing_orders pso
           WHERE pso.payment_order_id = po.id
             AND pso.status IN ('processing', 'finished')
       ) AS has_started_profit_sharing
FROM payment_orders po
WHERE po.id = $1
FOR UPDATE;

-- name: GetPendingRiderDepositRefundAmountByUserID :one
SELECT COALESCE(SUM(ro.refund_amount), 0)::bigint AS pending_rider_deposit_refund_amount
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE po.user_id = $1
    AND po.business_type = 'rider_deposit'
    AND ro.refund_type = 'rider_deposit'
    AND ro.status IN ('pending', 'processing');

-- name: ListRiderDepositWithdrawalRefundOrdersByIDs :many
SELECT
    ro.id AS refund_order_id,
    ro.payment_order_id,
    ro.refund_amount,
    ro.out_refund_no,
    ro.refund_id,
    ro.status,
    ro.refunded_at,
    ro.created_at,
    po.out_trade_no,
    po.amount AS source_payment_amount
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE po.user_id = sqlc.arg('user_id')::bigint
    AND po.business_type = 'rider_deposit'
    AND ro.refund_type = 'rider_deposit'
    AND ro.id = ANY(sqlc.arg('refund_order_ids')::bigint[])
ORDER BY ro.created_at ASC, ro.id ASC;

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

-- name: ListStuckProcessingRefundOrders :many
-- 查找持续处于 processing 状态超过阈值时间的退款单（支付通道回调可能永久丢失）
-- 用于运营告警，让人工核查对应支付后台退款结果
SELECT ro.id, ro.out_refund_no, ro.refund_id, ro.refund_amount, ro.status, ro.created_at,
       po.payment_type
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
WHERE ro.status = 'processing'
  AND ro.created_at < sqlc.arg('created_before')
ORDER BY ro.created_at ASC, ro.id ASC
LIMIT sqlc.arg('limit')::int;
