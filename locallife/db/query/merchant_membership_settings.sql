-- 商户会员设置查询

-- name: GetMerchantMembershipSettings :one
SELECT * FROM merchant_membership_settings
WHERE merchant_id = $1 LIMIT 1;

-- name: CreateMerchantMembershipSettings :one
INSERT INTO merchant_membership_settings (
    merchant_id,
    balance_usable_scenes,
    bonus_usable_scenes,
    allow_with_voucher,
    allow_with_discount,
    max_deduction_percent
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: UpdateMerchantMembershipSettings :one
UPDATE merchant_membership_settings
SET
    balance_usable_scenes = COALESCE(sqlc.narg('balance_usable_scenes'), balance_usable_scenes),
    bonus_usable_scenes = COALESCE(sqlc.narg('bonus_usable_scenes'), bonus_usable_scenes),
    allow_with_voucher = COALESCE(sqlc.narg('allow_with_voucher'), allow_with_voucher),
    allow_with_discount = COALESCE(sqlc.narg('allow_with_discount'), allow_with_discount),
    max_deduction_percent = COALESCE(sqlc.narg('max_deduction_percent'), max_deduction_percent),
    updated_at = NOW()
WHERE merchant_id = sqlc.arg('merchant_id')
RETURNING *;

-- name: UpsertMerchantMembershipSettings :one
INSERT INTO merchant_membership_settings (
    merchant_id,
    balance_usable_scenes,
    bonus_usable_scenes,
    allow_with_voucher,
    allow_with_discount,
    max_deduction_percent
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (merchant_id) DO UPDATE SET
    balance_usable_scenes = EXCLUDED.balance_usable_scenes,
    bonus_usable_scenes = EXCLUDED.bonus_usable_scenes,
    allow_with_voucher = EXCLUDED.allow_with_voucher,
    allow_with_discount = EXCLUDED.allow_with_discount,
    max_deduction_percent = EXCLUDED.max_deduction_percent,
    updated_at = NOW()
RETURNING *;
