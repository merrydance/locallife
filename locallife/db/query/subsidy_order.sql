-- name: CreateSubsidyOrder :one
INSERT INTO subsidy_orders (
    payment_order_id,
    sub_mch_id,
    transaction_id,
    out_subsidy_no,
    payer_amount,
    amount,
    description,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, 'pending'
) RETURNING *;

-- name: GetSubsidyOrder :one
SELECT * FROM subsidy_orders
WHERE id = $1 LIMIT 1;

-- name: GetSubsidyOrderByOutSubsidyNo :one
SELECT * FROM subsidy_orders
WHERE out_subsidy_no = $1 LIMIT 1;

-- name: GetSubsidyOrderByPaymentOrderID :one
-- 一个 payment_order 只应有一条补差
SELECT * FROM subsidy_orders
WHERE payment_order_id = $1 LIMIT 1;

-- name: GetSubsidyOrderForUpdate :one
SELECT * FROM subsidy_orders
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateSubsidyOrderToSuccess :one
UPDATE subsidy_orders
SET
    status           = 'success',
    wxpay_subsidy_id = $2,
    transaction_id   = COALESCE($3, transaction_id),
    updated_at       = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubsidyOrderToFailed :one
UPDATE subsidy_orders
SET
    status      = 'failed',
    fail_reason = $2,
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubsidyOrderToCanceled :one
UPDATE subsidy_orders
SET
    status     = 'canceled',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: InitiateSubsidyReturn :one
-- 发起补差退回，写入退回单号和金额
UPDATE subsidy_orders
SET
    out_return_no  = $2,
    return_amount  = $3,
    return_status  = 'pending_return',
    updated_at     = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubsidyReturnToSuccess :one
UPDATE subsidy_orders
SET
    return_status   = 'return_success',
    return_wxpay_id = $2,
    updated_at      = now()
WHERE out_return_no = $1
RETURNING *;

-- name: UpdateSubsidyReturnToFailed :one
UPDATE subsidy_orders
SET
    return_status      = 'return_failed',
    return_fail_reason = $2,
    updated_at         = now()
WHERE out_return_no = $1
RETURNING *;

-- name: ListSubsidyOrdersByPaymentIDs :many
-- 批量查询（用于退款流程判断是否需要退回补差）
SELECT * FROM subsidy_orders
WHERE payment_order_id = ANY(@payment_order_ids::bigint[])
  AND status = 'success';
