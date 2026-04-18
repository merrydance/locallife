-- name: CreateMerchantMembership :one
INSERT INTO merchant_memberships (
    merchant_id,
    user_id
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetMerchantMembership :one
SELECT id, merchant_id, user_id, balance, total_recharged, total_consumed, created_at, updated_at, principal_balance, bonus_balance FROM merchant_memberships
WHERE id = $1 LIMIT 1;

-- name: GetMembershipByMerchantAndUser :one
SELECT id, merchant_id, user_id, balance, total_recharged, total_consumed, created_at, updated_at, principal_balance, bonus_balance FROM merchant_memberships
WHERE merchant_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListUserMemberships :many
SELECT m.id, m.merchant_id, m.user_id, m.balance, m.total_recharged, m.total_consumed, m.created_at, m.updated_at, m.principal_balance, m.bonus_balance, mer.name as merchant_name, mer.logo_media_asset_id
FROM merchant_memberships m
JOIN merchants mer ON mer.id = m.merchant_id
WHERE m.user_id = $1
ORDER BY m.balance DESC
LIMIT $2 OFFSET $3;

-- name: ListMerchantMembers :many
SELECT m.id, m.merchant_id, m.user_id, m.balance, m.total_recharged, m.total_consumed, m.created_at, m.updated_at, m.principal_balance, m.bonus_balance, u.full_name, u.phone, u.avatar_url, u.avatar_media_asset_id
FROM merchant_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.merchant_id = $1
ORDER BY m.total_consumed DESC
LIMIT $2 OFFSET $3;

-- name: UpdateMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = $2,
    principal_balance = $3,
    bonus_balance = $4,
    total_recharged = $5,
    total_consumed = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = balance + $2,
    principal_balance = principal_balance + $2,
    total_recharged = total_recharged + $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DecrementMembershipBalance :one
UPDATE merchant_memberships
SET 
    balance = balance - $2,
    principal_balance = principal_balance - $2,
    total_consumed = total_consumed + $2,
    updated_at = NOW()
WHERE id = $1 AND balance >= $2
RETURNING *;

-- name: GetMembershipForUpdate :one
SELECT id, merchant_id, user_id, balance, total_recharged, total_consumed, created_at, updated_at, principal_balance, bonus_balance FROM merchant_memberships
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetMembershipByMerchantAndUserForUpdate :one
SELECT id, merchant_id, user_id, balance, total_recharged, total_consumed, created_at, updated_at, principal_balance, bonus_balance FROM merchant_memberships
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
SELECT id, merchant_id, recharge_amount, bonus_amount, is_active, valid_from, valid_until, created_at, updated_at FROM recharge_rules
WHERE id = $1 LIMIT 1;

-- name: ListMerchantRechargeRules :many
SELECT id, merchant_id, recharge_amount, bonus_amount, is_active, valid_from, valid_until, created_at, updated_at FROM recharge_rules
WHERE merchant_id = $1
ORDER BY recharge_amount ASC;

-- name: ListActiveRechargeRules :many
SELECT id, merchant_id, recharge_amount, bonus_amount, is_active, valid_from, valid_until, created_at, updated_at FROM recharge_rules
WHERE merchant_id = $1 
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
ORDER BY recharge_amount ASC;

-- name: GetMatchingRechargeRule :one
SELECT id, merchant_id, recharge_amount, bonus_amount, is_active, valid_from, valid_until, created_at, updated_at FROM recharge_rules
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
    principal_amount,
    bonus_amount,
    balance_after,
    related_order_id,
    recharge_rule_id,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: CreateMembershipTransactionWithPaymentOrderID :one
INSERT INTO membership_transactions (
    membership_id,
    type,
    amount,
    principal_amount,
    bonus_amount,
    balance_after,
    related_order_id,
    recharge_rule_id,
    notes,
    payment_order_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: CreateMembershipRechargeTransaction :one
INSERT INTO membership_transactions (
    membership_id,
    type,
    amount,
    principal_amount,
    bonus_amount,
    balance_after,
    related_order_id,
    recharge_rule_id,
    notes,
    idempotency_key
) VALUES (
    $1, 'recharge', $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetMembershipTransaction :one
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key FROM membership_transactions
WHERE id = $1 LIMIT 1;

-- name: GetMembershipTransactionByPaymentOrderID :one
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key FROM membership_transactions
WHERE payment_order_id = $1 LIMIT 1;

-- name: GetMembershipConsumeByOrder :one
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key FROM membership_transactions
WHERE membership_id = $1 AND related_order_id = $2 AND type = 'consume'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetMembershipRechargeTransactionByIdempotencyKey :one
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key
FROM membership_transactions
WHERE membership_id = sqlc.arg('membership_id')
    AND type = 'recharge'
    AND idempotency_key = sqlc.arg('idempotency_key')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListMembershipTransactions :many
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key FROM membership_transactions
WHERE membership_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMembershipTransactionsByType :many
SELECT id, membership_id, type, amount, balance_after, related_order_id, recharge_rule_id, notes, created_at, payment_order_id, principal_amount, bonus_amount, idempotency_key FROM membership_transactions
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
