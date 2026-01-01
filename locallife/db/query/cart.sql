-- name: CreateCart :one
INSERT INTO carts (user_id, merchant_id)
VALUES ($1, $2)
ON CONFLICT (user_id, merchant_id) DO UPDATE SET updated_at = now()
RETURNING *;

-- name: GetCart :one
SELECT * FROM carts
WHERE id = $1;

-- name: GetCartByUserAndMerchant :one
SELECT * FROM carts
WHERE user_id = $1 AND merchant_id = $2;

-- name: GetCartWithItems :one
SELECT 
    c.*,
    COALESCE(
        json_agg(
            json_build_object(
                'id', ci.id,
                'dish_id', ci.dish_id,
                'combo_id', ci.combo_id,
                'quantity', ci.quantity,
                'customizations', ci.customizations,
                'dish_name', d.name,
                'dish_image_url', d.image_url,
                'dish_price', d.price,
                'dish_member_price', d.member_price,
                'dish_is_available', d.is_available,
                'combo_name', cs.name,
                'combo_image_url', cs.image_url,
                'combo_original_price', cs.original_price,
                'combo_price', cs.combo_price,
                'combo_is_available', cs.is_online
            ) ORDER BY ci.created_at
        ) FILTER (WHERE ci.id IS NOT NULL), '[]'
    )::json AS items
FROM carts c
LEFT JOIN cart_items ci ON ci.cart_id = c.id
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE c.user_id = $1 AND c.merchant_id = $2
GROUP BY c.id;

-- name: AddCartItem :one
INSERT INTO cart_items (cart_id, dish_id, combo_id, quantity, customizations)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateCartItem :one
UPDATE cart_items
SET 
    quantity = COALESCE(sqlc.narg('quantity'), quantity),
    customizations = COALESCE(sqlc.narg('customizations'), customizations),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteCartItem :exec
DELETE FROM cart_items
WHERE id = $1;

-- name: ClearCart :exec
DELETE FROM cart_items
WHERE cart_id = $1;

-- name: DeleteCart :exec
DELETE FROM carts
WHERE id = $1;

-- name: GetCartItem :one
SELECT 
    ci.*,
    d.name AS dish_name,
    d.price AS dish_price,
    d.member_price AS dish_member_price,
    d.is_available AS dish_is_available,
    cs.name AS combo_name,
    cs.combo_price AS combo_price,
    cs.is_online AS combo_is_available
FROM cart_items ci
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE ci.id = $1;

-- name: ListCartItems :many
SELECT 
    ci.*,
    d.name AS dish_name,
    d.image_url AS dish_image_url,
    d.price AS dish_price,
    d.member_price AS dish_member_price,
    d.is_available AS dish_is_available,
    cs.name AS combo_name,
    cs.image_url AS combo_image_url,
    cs.original_price AS combo_original_price,
    cs.combo_price AS combo_price,
    cs.is_online AS combo_is_available
FROM cart_items ci
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE ci.cart_id = $1
ORDER BY ci.created_at;

-- name: GetUserCarts :many
SELECT 
    c.*,
    m.name AS merchant_name,
    m.logo_url AS merchant_logo,
    COUNT(ci.id) AS item_count
FROM carts c
JOIN merchants m ON m.id = c.merchant_id
LEFT JOIN cart_items ci ON ci.cart_id = c.id
WHERE c.user_id = $1
GROUP BY c.id, m.id
ORDER BY c.updated_at DESC;

-- name: GetCartItemByDishAndCustomizations :one
SELECT * FROM cart_items
WHERE cart_id = $1 AND dish_id = $2 AND customizations IS NOT DISTINCT FROM $3;

-- name: GetCartItemByCombo :one
SELECT * FROM cart_items
WHERE cart_id = $1 AND combo_id = $2;


-- ==================== 多商户购物车汇总查询 ====================

-- name: GetUserCartsSummary :one
-- 获取用户所有购物车的汇总统计
SELECT 
    COUNT(DISTINCT c.id)::int AS cart_count,
    COUNT(ci.id)::int AS total_items,
    COALESCE(SUM(
        CASE 
            WHEN ci.dish_id IS NOT NULL THEN d.price * ci.quantity
            WHEN ci.combo_id IS NOT NULL THEN cs.combo_price * ci.quantity
            ELSE 0
        END
    ), 0)::bigint AS total_amount
FROM carts c
LEFT JOIN cart_items ci ON ci.cart_id = c.id
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE c.user_id = $1;

-- name: GetUserCartsWithDetails :many
-- 获取用户所有购物车及其商品详情（用于合单结算）
SELECT 
    c.id AS cart_id,
    c.merchant_id,
    m.name AS merchant_name,
    m.logo_url AS merchant_logo,
    mpc.sub_mch_id AS sub_mchid,
    COUNT(ci.id)::int AS item_count,
    COALESCE(SUM(
        CASE 
            WHEN ci.dish_id IS NOT NULL THEN d.price * ci.quantity
            WHEN ci.combo_id IS NOT NULL THEN cs.combo_price * ci.quantity
            ELSE 0
        END
    ), 0)::bigint AS subtotal,
    -- 检查商品可用性
    BOOL_AND(
        CASE 
            WHEN ci.dish_id IS NOT NULL THEN d.is_available AND d.is_online
            WHEN ci.combo_id IS NOT NULL THEN cs.is_online
            ELSE true
        END
    ) AS all_available,
    c.updated_at
FROM carts c
JOIN merchants m ON m.id = c.merchant_id
LEFT JOIN merchant_payment_configs mpc ON mpc.merchant_id = m.id
LEFT JOIN cart_items ci ON ci.cart_id = c.id
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE c.user_id = $1
GROUP BY c.id, c.merchant_id, m.id, mpc.sub_mch_id
HAVING COUNT(ci.id) > 0  -- 只返回有商品的购物车
ORDER BY c.updated_at DESC;

-- name: GetUserCartsByMerchantIDs :many
-- 根据商户ID列表获取用户购物车（用于合单结算时验证）
SELECT 
    c.*,
    m.name AS merchant_name,
    mpc.sub_mch_id AS sub_mchid,
    m.status AS merchant_status
FROM carts c
JOIN merchants m ON m.id = c.merchant_id
LEFT JOIN merchant_payment_configs mpc ON mpc.merchant_id = m.id
WHERE c.user_id = $1 AND c.merchant_id = ANY($2::bigint[])
ORDER BY c.updated_at DESC;

-- name: ListCartItemsForCheckout :many
-- 获取购物车商品详情（用于结算时校验价格和可用性）
SELECT 
    ci.*,
    d.name AS dish_name,
    d.image_url AS dish_image_url,
    d.price AS dish_price,
    d.member_price AS dish_member_price,
    d.is_available AS dish_is_available,
    d.is_online AS dish_is_online,
    d.merchant_id AS dish_merchant_id,
    cs.name AS combo_name,
    cs.image_url AS combo_image_url,
    cs.combo_price AS combo_price,
    cs.is_online AS combo_is_online,
    cs.merchant_id AS combo_merchant_id
FROM cart_items ci
LEFT JOIN dishes d ON d.id = ci.dish_id
LEFT JOIN combo_sets cs ON cs.id = ci.combo_id
WHERE ci.cart_id = ANY($1::bigint[])
ORDER BY ci.cart_id, ci.created_at;

-- name: ClearMultipleCarts :exec
-- 批量清空购物车（合单支付成功后）
DELETE FROM cart_items
WHERE cart_id = ANY($1::bigint[]);

-- name: GetUserCartsByCartIDs :many
-- 根据购物车ID列表获取用户购物车详情（用于合单结算）
SELECT 
    c.id,
    c.user_id,
    c.merchant_id,
    c.updated_at,
    c.created_at,
    m.name AS merchant_name,
    m.logo_url AS merchant_logo,
    m.region_id AS region_id,
    mpc.sub_mch_id AS sub_mchid,
    m.status AS merchant_status,
    m.latitude AS merchant_latitude,
    m.longitude AS merchant_longitude
FROM carts c
JOIN merchants m ON m.id = c.merchant_id
LEFT JOIN merchant_payment_configs mpc ON mpc.merchant_id = m.id
WHERE c.user_id = $1 AND c.id = ANY($2::bigint[])
ORDER BY c.updated_at DESC;
