-- Discount Rules (满减规则)

-- name: CreateDiscountRule :one
INSERT INTO discount_rules (
    merchant_id,
    name,
    description,
    min_order_amount,
    discount_amount,
    can_stack_with_voucher,
    can_stack_with_membership,
    valid_from,
    valid_until,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetDiscountRule :one
SELECT * FROM discount_rules
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListMerchantDiscountRules :many
SELECT * FROM discount_rules
WHERE merchant_id = $1 AND deleted_at IS NULL
ORDER BY min_order_amount ASC
LIMIT $2 OFFSET $3;

-- name: ListActiveDiscountRules :many
SELECT * FROM discount_rules
WHERE merchant_id = $1 
    AND deleted_at IS NULL
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
ORDER BY min_order_amount ASC;

-- name: GetApplicableDiscountRules :many
SELECT * FROM discount_rules
WHERE merchant_id = $1 
    AND deleted_at IS NULL
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
    AND min_order_amount <= $2
ORDER BY discount_amount DESC;

-- name: GetBestDiscountRule :one
SELECT * FROM discount_rules
WHERE merchant_id = $1 
    AND deleted_at IS NULL
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW()
    AND min_order_amount <= $2
ORDER BY discount_amount DESC
LIMIT 1;

-- name: UpdateDiscountRule :one
UPDATE discount_rules
SET 
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    min_order_amount = COALESCE(sqlc.narg('min_order_amount'), min_order_amount),
    discount_amount = COALESCE(sqlc.narg('discount_amount'), discount_amount),
    can_stack_with_voucher = COALESCE(sqlc.narg('can_stack_with_voucher'), can_stack_with_voucher),
    can_stack_with_membership = COALESCE(sqlc.narg('can_stack_with_membership'), can_stack_with_membership),
    valid_from = COALESCE(sqlc.narg('valid_from'), valid_from),
    valid_until = COALESCE(sqlc.narg('valid_until'), valid_until),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: DeleteDiscountRule :exec
-- 软删除满减规则
UPDATE discount_rules SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountActiveDiscountRules :one
SELECT COUNT(*) FROM discount_rules
WHERE merchant_id = $1 
    AND deleted_at IS NULL
    AND is_active = TRUE
    AND valid_from <= NOW()
    AND valid_until >= NOW();
