-- name: CreateWithdrawalRecord :one
INSERT INTO withdrawal_records (
    user_id,
    amount,
    status,
    channel,
    account_info,
    out_request_no
) VALUES (
    $1, $2, $3, $4, $5, sqlc.narg(out_request_no)
) RETURNING *;

-- name: GetWithdrawalRecord :one
SELECT id, user_id, amount, status, channel, account_info, reason, created_at, updated_at, out_request_no FROM withdrawal_records
WHERE id = $1 LIMIT 1;

-- name: ListWithdrawalRecords :many
SELECT id, user_id, amount, status, channel, account_info, reason, created_at, updated_at, out_request_no FROM withdrawal_records
WHERE user_id = $1
    AND channel = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountWithdrawalRecords :one
SELECT count(*) FROM withdrawal_records
WHERE user_id = $1
  AND channel = $2;

-- name: GetWithdrawalRecordByOutRequestNo :one
SELECT id, user_id, amount, status, channel, account_info, reason, created_at, updated_at, out_request_no FROM withdrawal_records
WHERE out_request_no = $1 LIMIT 1;

-- name: ListPendingWithdrawalRecordsByChannel :many
SELECT id, user_id, amount, status, channel, account_info, reason, created_at, updated_at, out_request_no FROM withdrawal_records
WHERE channel = $1
    AND status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

-- name: UpdateWithdrawalStatus :one
UPDATE withdrawal_records
SET 
    status = sqlc.arg('status'),
    reason = CASE
        WHEN sqlc.arg('clear_reason')::bool THEN NULL
        WHEN sqlc.narg('reason')::text IS NOT NULL THEN sqlc.narg('reason')::text
        ELSE reason
    END,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UpdateWithdrawalAccountInfo :one
UPDATE withdrawal_records
SET
    account_info = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
