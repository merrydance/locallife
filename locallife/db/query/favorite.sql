-- name: AddFavoriteMerchant :one
INSERT INTO favorites (user_id, favorite_type, merchant_id)
VALUES ($1, 'merchant', $2)
ON CONFLICT (user_id, favorite_type, merchant_id) WHERE merchant_id IS NOT NULL 
DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: AddFavoriteDish :one
INSERT INTO favorites (user_id, favorite_type, dish_id)
VALUES ($1, 'dish', $2)
ON CONFLICT (user_id, favorite_type, dish_id) WHERE dish_id IS NOT NULL 
DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: RemoveFavoriteMerchant :exec
DELETE FROM favorites
WHERE user_id = $1 AND favorite_type = 'merchant' AND merchant_id = $2;

-- name: RemoveFavoriteDish :exec
DELETE FROM favorites
WHERE user_id = $1 AND favorite_type = 'dish' AND dish_id = $2;

-- name: ListFavoriteMerchants :many
SELECT 
    f.id,
    f.created_at,
    m.id AS merchant_id,
    m.name AS merchant_name,
    m.logo_media_asset_id AS merchant_logo_media_asset_id,
    m.address AS merchant_address,
    m.status AS merchant_status,
    COALESCE(mp.is_takeout_suspended, false) AS merchant_is_ordering_suspended
FROM favorites f
JOIN merchants m ON m.id = f.merchant_id
LEFT JOIN merchant_profiles mp ON mp.merchant_id = m.id
WHERE f.user_id = $1 AND f.favorite_type = 'merchant'
ORDER BY f.created_at DESC, f.id DESC
LIMIT $2 OFFSET $3;

-- name: CountFavoriteMerchants :one
SELECT COUNT(*) FROM favorites
WHERE user_id = $1 AND favorite_type = 'merchant';

-- name: ListFavoriteDishes :many
SELECT 
    f.id,
    f.created_at,
    d.id AS dish_id,
    d.name AS dish_name,
    d.description AS dish_description,
    d.image_media_asset_id AS dish_image_media_asset_id,
    d.price AS dish_price,
    d.member_price AS dish_member_price,
    d.is_available AS dish_is_available,
    d.is_online AS dish_is_online,
    m.id AS merchant_id,
    m.name AS merchant_name
FROM favorites f
JOIN dishes d ON d.id = f.dish_id
JOIN merchants m ON m.id = d.merchant_id
WHERE f.user_id = $1 AND f.favorite_type = 'dish'
  AND (NOT sqlc.arg('exclude_packaging')::boolean OR d.is_packaging = false)
ORDER BY f.created_at DESC, f.id DESC
LIMIT $2 OFFSET $3;

-- name: CountFavoriteDishes :one
SELECT COUNT(*) FROM favorites
JOIN dishes d ON d.id = favorites.dish_id
WHERE favorites.user_id = $1 AND favorites.favorite_type = 'dish'
  AND (NOT sqlc.arg('exclude_packaging')::boolean OR d.is_packaging = false);

-- name: IsMerchantFavorited :one
SELECT EXISTS(
    SELECT 1 FROM favorites
    WHERE user_id = $1 AND favorite_type = 'merchant' AND merchant_id = $2
) AS is_favorited;

-- name: IsDishFavorited :one
SELECT EXISTS(
    SELECT 1
    FROM favorites f
    JOIN dishes d ON d.id = f.dish_id
    WHERE f.user_id = sqlc.arg('user_id')
      AND f.favorite_type = 'dish'
      AND f.dish_id = sqlc.arg('dish_id')
      AND (NOT sqlc.arg('exclude_packaging')::boolean OR d.is_packaging = false)
) AS is_favorited;
