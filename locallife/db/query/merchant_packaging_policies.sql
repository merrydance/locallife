-- 商户包装策略查询

-- name: GetMerchantPackagingPolicy :one
SELECT * FROM merchant_packaging_policies
WHERE merchant_id = $1
LIMIT 1;

-- name: UpsertMerchantPackagingPolicy :one
INSERT INTO merchant_packaging_policies (
    merchant_id,
    applicable_order_types,
    candidate_dish_ids
) VALUES (
    $1, $2, $3
)
ON CONFLICT (merchant_id) DO UPDATE SET
    applicable_order_types = EXCLUDED.applicable_order_types,
    candidate_dish_ids = EXCLUDED.candidate_dish_ids,
    updated_at = NOW()
RETURNING *;