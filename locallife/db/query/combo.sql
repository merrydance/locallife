-- ============================================
-- 套餐查询 (Combo Queries)
-- ============================================

-- name: CreateComboSet :one
INSERT INTO combo_sets (
  merchant_id,
  name,
  description,
  image_url,
  original_price,
  combo_price,
  is_online
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetComboSet :one
SELECT * FROM combo_sets
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetComboSetWithDetails :one
SELECT 
  cs.*,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'dish_id', cd.dish_id,
        'dish_name', d.name,
        'dish_price', d.price,
        'dish_image_url', d.image_url,
        'quantity', cd.quantity
      )
    ) FILTER (WHERE cd.dish_id IS NOT NULL),
    '[]'
  ) as dishes,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'id', t.id,
        'name', t.name
      )
    ) FILTER (WHERE t.id IS NOT NULL),
    '[]'
  ) as tags
FROM combo_sets cs
LEFT JOIN combo_dishes cd ON cs.id = cd.combo_id
LEFT JOIN dishes d ON cd.dish_id = d.id
LEFT JOIN combo_tags ct ON cs.id = ct.combo_id
LEFT JOIN tags t ON ct.tag_id = t.id
WHERE cs.id = $1 AND cs.deleted_at IS NULL
GROUP BY cs.id;

-- name: ListComboSetsByMerchant :many
SELECT * FROM combo_sets
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('is_online')::boolean IS NULL OR is_online = sqlc.narg('is_online'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateComboSet :one
UPDATE combo_sets
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  image_url = COALESCE(sqlc.narg('image_url'), image_url),
  original_price = COALESCE(sqlc.narg('original_price'), original_price),
  combo_price = COALESCE(sqlc.narg('combo_price'), combo_price),
  is_online = COALESCE(sqlc.narg('is_online'), is_online),
  updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: UpdateComboSetOnlineStatus :exec
UPDATE combo_sets
SET 
  is_online = $2,
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteComboSet :exec
-- 软删除套餐
UPDATE combo_sets SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountComboSetsByMerchant :one
SELECT COUNT(*) FROM combo_sets
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('is_online')::boolean IS NULL OR is_online = sqlc.narg('is_online'));

-- ============================================
-- 套餐菜品关联查询 (Combo Dish Queries)
-- ============================================

-- name: AddComboDish :one
INSERT INTO combo_dishes (
  combo_id,
  dish_id,
  quantity
) VALUES (
  $1, $2, $3
) RETURNING *;

-- name: ListComboDishes :many
SELECT 
  d.*,
  cd.quantity
FROM dishes d
JOIN combo_dishes cd ON d.id = cd.dish_id
WHERE cd.combo_id = $1
ORDER BY cd.id ASC;

-- name: RemoveComboDish :exec
DELETE FROM combo_dishes
WHERE combo_id = $1 AND dish_id = $2;

-- name: RemoveAllComboDishes :exec
DELETE FROM combo_dishes
WHERE combo_id = $1;

-- ============================================
-- 套餐标签关联查询 (Combo Tag Queries)
-- ============================================

-- name: AddComboTag :one
INSERT INTO combo_tags (
  combo_id,
  tag_id
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListComboTags :many
SELECT 
  t.id,
  t.name,
  t.type,
  t.sort_order,
  t.status,
  t.created_at
FROM tags t
JOIN combo_tags ct ON t.id = ct.tag_id
WHERE ct.combo_id = $1
ORDER BY t.sort_order ASC;

-- name: RemoveComboTag :exec
DELETE FROM combo_tags
WHERE combo_id = $1 AND tag_id = $2;

-- name: RemoveAllComboTags :exec
DELETE FROM combo_tags
WHERE combo_id = $1;

-- ============================================
-- 套餐推荐查询
-- ============================================

-- name: GetPopularCombos :many
-- 获取热门套餐（基于销量）
SELECT 
    cs.id,
    cs.merchant_id,
    cs.name,
    cs.description,
    cs.image_url,
    cs.original_price,
    cs.combo_price,
    COALESCE(SUM(oi.quantity), 0)::int AS total_sold
FROM combo_sets cs
LEFT JOIN order_items oi ON cs.id = oi.combo_id
LEFT JOIN orders o ON o.id = oi.order_id AND o.status IN ('delivered', 'completed')
WHERE cs.is_online = true AND cs.deleted_at IS NULL
GROUP BY cs.id
ORDER BY total_sold DESC, cs.created_at DESC
LIMIT $1;

-- name: GetCombosByIDs :many
-- 批量获取套餐详情
SELECT 
    id,
    merchant_id,
    name,
    description,
    image_url,
    original_price,
    combo_price,
    is_online
FROM combo_sets
WHERE id = ANY($1::bigint[])
  AND deleted_at IS NULL
  AND is_online = true;

-- name: GetCombosWithMerchantByIDs :many
-- 批量获取套餐详情及商户信息（用于推荐流展示）
SELECT 
    cs.id,
    cs.merchant_id,
    cs.name,
    cs.description,
    cs.image_url,
    cs.original_price,
    cs.combo_price,
    cs.is_online,
    m.name AS merchant_name,
    m.logo_url AS merchant_logo,
    m.latitude AS merchant_latitude,
    m.longitude AS merchant_longitude,
    m.region_id AS merchant_region_id,
    COALESCE(
        (SELECT SUM(oi.quantity)
         FROM order_items oi 
         JOIN orders o ON o.id = oi.order_id 
         WHERE oi.combo_id = cs.id 
           AND o.status IN ('delivered', 'completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_sales
FROM combo_sets cs
JOIN merchants m ON m.id = cs.merchant_id
WHERE cs.id = ANY($1::bigint[])
  AND cs.deleted_at IS NULL
  AND cs.is_online = true
  AND m.status = 'approved';

-- name: ListOnlineCombosByMerchant :many
-- 获取商户上架套餐（用于扫码点餐菜单展示）
SELECT 
    id,
    merchant_id,
    name,
    description,
    image_url,
    original_price,
    combo_price AS price,
    is_online
FROM combo_sets
WHERE merchant_id = $1
  AND deleted_at IS NULL
  AND is_online = true
ORDER BY created_at DESC;
