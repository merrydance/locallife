-- name: CreateMerchantMembership :one
INSERT INTO merchant_memberships (
    merchant_id,
    user_id
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetMerchantMembership :one
SELECT * FROM merchant_memberships
WHERE id = $1 LIMIT 1;

-- name: GetMembershipByMerchantAndUser :one
SELECT * FROM merchant_memberships
WHERE merchant_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListUserMemberships :many
SELECT m.*, mer.name as merchant_name, mer.logo_url
FROM merchant_memberships m
JOIN merchants mer ON mer.id = m.merchant_id
WHERE m.user_id = $1
ORDER BY m.balance DESC
LIMIT $2 OFFSET $3;

-- name: ListMerchantMembers :many
SELECT m.*, u.full_name, u.phone, u.avatar_url
FROM merchant_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.merchant_id = $1
ORDER BY m.total_consumed DESC
LIMIT $2 OFFSET $3;

-- name: UpdateMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = $2,
    total_recharged = $3,
    total_consumed = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = balance + $2,
    total_recharged = total_recharged + $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DecrementMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = balance - $2,
    total_consumed = total_consumed + $2,
    updated_at = NOW()
WHERE id = $1 AND balance >= $2
RETURNING *;

-- name: GetMembershipForUpdate :one
SELECT * FROM merchant_memberships
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetMembershipByMerchantAndUserForUpdate :one
SELECT * FROM merchant_memberships
WHERE merchant_id = $1 AND user_id = $2 LIMIT 1
FOR UPDATE;

-- Recharge Rules

-- name: CreateRechargeRule :one
INSERT INTO recharge_rules (
    merchant_id,
    recharge_amount,
    bonus_amount,
    is_active,
    valid_from,
    valid_until
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetRechargeRule :one
SELECT * FROM recharge_rules
WHERE id = $1 LIMIT 1;

-- name: ListMerchantRechargeRules :many
SELECT * FROM recharge_rules
WHERE merchant_id = $1
ORDER BY recharge_amount ASC;

-- name: ListActiveRechargeRules :many
SELECT * FROM recharge_rules
WHERE merchant_id = $1 
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
ORDER BY recharge_amount ASC;

-- name: GetMatchingRechargeRule :one
SELECT * FROM recharge_rules
WHERE merchant_id = $1 
    AND recharge_amount = $2
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
LIMIT 1;

-- name: UpdateRechargeRule :one
UPDATE recharge_rules
SET 
    recharge_amount = COALESCE(sqlc.narg('recharge_amount'), recharge_amount),
    bonus_amount = COALESCE(sqlc.narg('bonus_amount'), bonus_amount),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    valid_from = COALESCE(sqlc.narg('valid_from'), valid_from),
    valid_until = COALESCE(sqlc.narg('valid_until'), valid_until),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteRechargeRule :exec
DELETE FROM recharge_rules
WHERE id = $1;

-- Membership Transactions

-- name: CreateMembershipTransaction :one
INSERT INTO membership_transactions (
    membership_id,
    type,
    amount,
    balance_after,
    related_order_id,
    recharge_rule_id,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetMembershipTransaction :one
SELECT * FROM membership_transactions
WHERE id = $1 LIMIT 1;

-- name: ListMembershipTransactions :many
SELECT * FROM membership_transactions
WHERE membership_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMembershipTransactionsByType :many
SELECT * FROM membership_transactions
WHERE membership_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetMembershipTransactionStats :one
SELECT 
    COUNT(*) as total_count,
    SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END) as total_recharge,
    SUM(CASE WHEN amount < 0 THEN ABS(amount) ELSE 0 END) as total_consume
FROM membership_transactions
WHERE membership_id = $1;
