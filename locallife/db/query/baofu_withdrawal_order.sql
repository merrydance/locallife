-- name: CreateBaofuWithdrawalOrder :one
INSERT INTO baofu_withdrawal_orders (
    owner_type,
    owner_id,
    account_binding_id,
    out_request_no,
    amount,
    status,
    raw_snapshot
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_binding_id),
    sqlc.arg(out_request_no),
    sqlc.arg(amount),
    sqlc.arg(status),
    COALESCE(sqlc.narg(raw_snapshot), '{}'::jsonb)
) RETURNING *;

-- name: GetBaofuWithdrawalOrder :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at
FROM baofu_withdrawal_orders
WHERE id = $1
LIMIT 1;

-- name: GetBaofuWithdrawalOrderByOutRequestNo :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at
FROM baofu_withdrawal_orders
WHERE out_request_no = $1
LIMIT 1;

-- name: ListBaofuWithdrawalOrdersByOwner :many
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at
FROM baofu_withdrawal_orders
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count)::int
OFFSET sqlc.arg(offset_count)::int;

-- name: CountBaofuWithdrawalOrdersByOwner :one
SELECT COUNT(*)::bigint
FROM baofu_withdrawal_orders
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id);

-- name: ListProcessingBaofuWithdrawalOrdersForRecovery :many
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at
FROM baofu_withdrawal_orders
WHERE status = 'processing'
  AND created_at <= sqlc.arg(created_before)
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg(limit_count)::int;

-- name: UpdateBaofuWithdrawalOrderToProcessing :one
UPDATE baofu_withdrawal_orders
SET
    status = 'processing',
    baofu_withdraw_no = sqlc.narg(baofu_withdraw_no),
    raw_snapshot = COALESCE(sqlc.narg(raw_snapshot), raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'processing'
RETURNING *;

-- name: UpdateBaofuWithdrawalOrderStatus :one
UPDATE baofu_withdrawal_orders
SET
    status = sqlc.arg(status),
    baofu_withdraw_no = COALESCE(sqlc.narg(baofu_withdraw_no), baofu_withdraw_no),
    raw_snapshot = COALESCE(sqlc.narg(raw_snapshot), raw_snapshot),
    finished_at = CASE WHEN sqlc.arg(status) IN ('succeeded', 'failed', 'returned') THEN now() ELSE finished_at END,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'processing'
RETURNING *;
