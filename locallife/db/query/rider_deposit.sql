-- name: CreateRiderDeposit :one
INSERT INTO rider_deposits (
    rider_id,
    amount,
    type,
    related_order_id,
    balance_after,
    remark
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetRiderDeposit :one
SELECT * FROM rider_deposits
WHERE id = $1 LIMIT 1;

-- name: ListRiderDeposits :many
SELECT * FROM rider_deposits
WHERE rider_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRiderDepositsByType :many
SELECT * FROM rider_deposits
WHERE rider_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetRiderDepositStats :one
SELECT 
    COALESCE(SUM(CASE WHEN type = 'deposit' THEN amount ELSE 0 END), 0) AS total_deposit,
    COALESCE(SUM(CASE WHEN type = 'withdraw' THEN amount ELSE 0 END), 0) AS total_withdraw,
    COALESCE(SUM(CASE WHEN type = 'deduct' THEN amount ELSE 0 END), 0) AS total_deduct
FROM rider_deposits
WHERE rider_id = $1;
