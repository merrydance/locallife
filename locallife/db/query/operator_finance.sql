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
SELECT * FROM withdrawal_records
WHERE id = $1 LIMIT 1;

-- name: ListWithdrawalRecords :many
SELECT * FROM withdrawal_records
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWithdrawalRecords :one
SELECT count(*) FROM withdrawal_records
WHERE user_id = $1
  AND channel = $2;

-- name: GetWithdrawalRecordByOutRequestNo :one
SELECT * FROM withdrawal_records
WHERE out_request_no = $1 LIMIT 1;

-- name: ListPendingWithdrawalRecordsByChannel :many
SELECT * FROM withdrawal_records
WHERE channel = $1
    AND status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

-- name: UpdateWithdrawalStatus :one
UPDATE withdrawal_records
SET 
    status = $2,
    reason = COALESCE(sqlc.narg(reason), reason),
    updated_at = now()
WHERE id = $1
RETURNING *;
