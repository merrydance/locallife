-- Vouchers (代金券模板)

-- name: CreateVoucher :one
INSERT INTO vouchers (
    merchant_id,
    code,
    name,
    description,
    amount,
    min_order_amount,
    total_quantity,
    valid_from,
    valid_until,
    is_active,
    allowed_order_types
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetVoucher :one
SELECT * FROM vouchers
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetVoucherByCode :one
SELECT * FROM vouchers
WHERE code = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetVoucherForUpdate :one
SELECT * FROM vouchers
WHERE id = $1 AND deleted_at IS NULL LIMIT 1
FOR UPDATE;

-- name: ListMerchantVouchers :many
SELECT * FROM vouchers
WHERE merchant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActiveVouchers :many
SELECT * FROM vouchers
WHERE merchant_id = $1 
    AND deleted_at IS NULL
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
    AND claimed_quantity < total_quantity
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateVoucher :one
UPDATE vouchers
SET 
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    amount = COALESCE(sqlc.narg('amount'), amount),
    min_order_amount = COALESCE(sqlc.narg('min_order_amount'), min_order_amount),
    total_quantity = COALESCE(sqlc.narg('total_quantity'), total_quantity),
    valid_from = COALESCE(sqlc.narg('valid_from'), valid_from),
    valid_until = COALESCE(sqlc.narg('valid_until'), valid_until),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    allowed_order_types = COALESCE(sqlc.narg('allowed_order_types'), allowed_order_types),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: IncrementVoucherClaimedQuantity :one
UPDATE vouchers
SET 
    claimed_quantity = claimed_quantity + 1,
    updated_at = NOW()
WHERE id = $1 
    AND claimed_quantity < total_quantity
RETURNING *;

-- name: IncrementVoucherUsedQuantity :one
UPDATE vouchers
SET 
    used_quantity = used_quantity + 1,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteVoucher :exec
-- 软删除代金券模板
UPDATE vouchers SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- User Vouchers (用户已领取的代金券)

-- name: CreateUserVoucher :one
INSERT INTO user_vouchers (
    voucher_id,
    user_id,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetUserVoucher :one
SELECT uv.*, v.merchant_id, v.code, v.name, v.amount, v.min_order_amount, v.allowed_order_types
FROM user_vouchers uv
JOIN vouchers v ON v.id = uv.voucher_id
WHERE uv.id = $1 LIMIT 1;

-- name: GetUserVoucherForUpdate :one
SELECT * FROM user_vouchers
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: ListUserVouchers :many
SELECT uv.*, v.merchant_id, v.code, v.name, v.amount, v.min_order_amount, v.allowed_order_types, m.name as merchant_name
FROM user_vouchers uv
JOIN vouchers v ON v.id = uv.voucher_id
JOIN merchants m ON m.id = v.merchant_id
WHERE uv.user_id = $1
ORDER BY uv.obtained_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUserAvailableVouchers :many
SELECT uv.*, v.merchant_id, v.code, v.name, v.amount, v.min_order_amount, v.allowed_order_types, m.name as merchant_name
FROM user_vouchers uv
JOIN vouchers v ON v.id = uv.voucher_id
JOIN merchants m ON m.id = v.merchant_id
WHERE uv.user_id = $1 
    AND uv.status = 'unused'
    AND uv.expires_at > NOW()
ORDER BY v.amount DESC
LIMIT $2 OFFSET $3;

-- name: ListUserAvailableVouchersForMerchant :many
SELECT uv.*, v.code, v.name, v.amount, v.min_order_amount, v.allowed_order_types
FROM user_vouchers uv
JOIN vouchers v ON v.id = uv.voucher_id
WHERE uv.user_id = $1 
    AND v.merchant_id = $2
    AND uv.status = 'unused'
    AND uv.expires_at > NOW()
    AND v.min_order_amount <= $3
ORDER BY v.amount DESC;

-- name: CheckUserVoucherExists :one
SELECT COUNT(*) > 0 as exists
FROM user_vouchers
WHERE voucher_id = $1 AND user_id = $2;

-- name: MarkUserVoucherAsUsed :one
UPDATE user_vouchers
SET 
    status = 'used',
    order_id = $2,
    used_at = NOW()
WHERE id = $1 AND status = 'unused'
RETURNING *;

-- name: MarkExpiredVouchers :exec
UPDATE user_vouchers
SET status = 'expired'
WHERE status = 'unused' 
    AND expires_at <= NOW();

-- name: CountUserVouchersByStatus :one
SELECT 
    COUNT(*) FILTER (WHERE status = 'unused' AND expires_at > NOW()) as available_count,
    COUNT(*) FILTER (WHERE status = 'used') as used_count,
    COUNT(*) FILTER (WHERE status = 'expired' OR (status = 'unused' AND expires_at <= NOW())) as expired_count
FROM user_vouchers
WHERE user_id = $1;

-- name: CountUnusedVouchersByVoucherID :one
SELECT COUNT(*) FROM user_vouchers
WHERE voucher_id = $1 AND status = 'unused' AND expires_at > NOW();

-- name: GetVoucherUsageStats :one
SELECT 
    v.total_quantity,
    v.claimed_quantity,
    v.used_quantity,
    COUNT(CASE WHEN uv.status = 'unused' AND uv.expires_at > NOW() THEN 1 END) as active_count,
    COUNT(CASE WHEN uv.status = 'used' THEN 1 END) as used_count_verified,
    COUNT(CASE WHEN uv.status = 'expired' OR (uv.status = 'unused' AND uv.expires_at <= NOW()) THEN 1 END) as expired_count
FROM vouchers v
LEFT JOIN user_vouchers uv ON uv.voucher_id = v.id
WHERE v.id = $1
GROUP BY v.id, v.total_quantity, v.claimed_quantity, v.used_quantity;
