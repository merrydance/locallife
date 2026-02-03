-- name: CreateProfitSharingReturn :one
INSERT INTO profit_sharing_returns (
    refund_order_id,
    profit_sharing_order_id,
    payment_order_id,
    sub_mchid,
    out_order_no,
    out_return_no,
    return_mchid,
    amount,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetProfitSharingReturn :one
SELECT * FROM profit_sharing_returns
WHERE id = $1 LIMIT 1;

-- name: GetProfitSharingReturnByOutReturnNo :one
SELECT * FROM profit_sharing_returns
WHERE out_return_no = $1 LIMIT 1;

-- name: ListProfitSharingReturnsByRefundOrder :many
SELECT * FROM profit_sharing_returns
WHERE refund_order_id = $1
ORDER BY created_at;

-- name: CountProfitSharingReturnsByRefundOrder :one
SELECT COUNT(*)::int FROM profit_sharing_returns
WHERE refund_order_id = $1;

-- name: CountProfitSharingReturnsByRefundOrderStatus :one
SELECT COUNT(*)::int FROM profit_sharing_returns
WHERE refund_order_id = $1 AND status = $2;

-- name: UpdateProfitSharingReturnToProcessing :one
UPDATE profit_sharing_returns
SET
    status = 'processing',
    return_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateProfitSharingReturnToSuccess :one
UPDATE profit_sharing_returns
SET
    status = 'success',
    finished_at = now(),
    fail_reason = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateProfitSharingReturnToFailed :one
UPDATE profit_sharing_returns
SET
    status = 'failed',
    fail_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
