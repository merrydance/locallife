-- ============================================
-- 菜品分类查询 (Dish Category Queries)
-- ============================================

-- name: GetDishCategoryByName :one
SELECT * FROM dish_categories
WHERE name = $1 LIMIT 1;

-- name: CreateDishCategory :one
INSERT INTO dish_categories (
  name
) VALUES (
  $1
) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: LinkMerchantDishCategory :one
INSERT INTO merchant_dish_categories (
  merchant_id,
  category_id,
  sort_order
) VALUES (
  $1, $2, $3
) ON CONFLICT (merchant_id, category_id) DO UPDATE SET 
  sort_order = EXCLUDED.sort_order
RETURNING *;

-- name: GetDishCategory :one
SELECT * FROM dish_categories
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListDishCategories :many
SELECT c.*, mdc.sort_order FROM dish_categories c
JOIN merchant_dish_categories mdc ON c.id = mdc.category_id
WHERE mdc.merchant_id = $1
ORDER BY mdc.sort_order ASC, c.name ASC;

-- name: UpdateMerchantDishCategoryOrder :one
UPDATE merchant_dish_categories
SET
  sort_order = $3
WHERE merchant_id = $1 AND category_id = $2
RETURNING *;

-- name: GetMerchantDishCategory :one
SELECT * FROM merchant_dish_categories
WHERE merchant_id = $1 AND category_id = $2;

-- name: UnlinkMerchantDishCategory :exec
DELETE FROM merchant_dish_categories
WHERE merchant_id = $1 AND category_id = $2;

-- name: UpdateDishesCategory :exec
UPDATE dishes
SET category_id = sqlc.arg(new_category_id)
WHERE merchant_id = sqlc.arg(merchant_id) AND category_id = sqlc.arg(old_category_id);

-- ============================================
-- 菜品查询 (Dish Queries)
-- ============================================

-- name: CreateDish :one
INSERT INTO dishes (
  merchant_id,
  category_id,
  name,
  description,
  image_url,
  price,
  member_price,
  is_available,
  is_online,
  sort_order,
  prepare_time
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetDish :one
SELECT * FROM dishes
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetDishWithDetails :one
SELECT 
  d.*,
  dc.name as category_name,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'id', i.id,
        'name', i.name,
        'category', i.category,
        'is_allergen', i.is_allergen
      )
    ) FILTER (WHERE i.id IS NOT NULL),
    '[]'
  ) as ingredients,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'id', t.id,
        'name', t.name
      )
    ) FILTER (WHERE t.id IS NOT NULL),
    '[]'
  ) as tags
FROM dishes d
LEFT JOIN dish_categories dc ON d.category_id = dc.id
LEFT JOIN dish_ingredients di ON d.id = di.dish_id
LEFT JOIN ingredients i ON di.ingredient_id = i.id
LEFT JOIN dish_tags dt ON d.id = dt.dish_id
LEFT JOIN tags t ON dt.tag_id = t.id
WHERE d.id = $1 AND d.deleted_at IS NULL
GROUP BY d.id, dc.name;

-- name: ListDishesByMerchant :many
SELECT * FROM dishes
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('category_id')::bigint IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_online')::boolean IS NULL OR is_online = sqlc.narg('is_online'))
  AND (sqlc.narg('is_available')::boolean IS NULL OR is_available = sqlc.narg('is_available'))
ORDER BY sort_order ASC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchDishesByName :many
SELECT * FROM dishes
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND name ILIKE '%' || $2 || '%'
  AND is_online = true
ORDER BY sort_order ASC, name ASC
LIMIT $3 OFFSET $4;

-- name: CountSearchDishesByName :one
-- 统计商户内菜品搜索结果总数
SELECT COUNT(*) FROM dishes
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND name ILIKE '%' || $2 || '%'
  AND is_online = true;

-- name: SearchDishesGlobal :many
-- 全局菜品搜索（跨商户），只搜索已激活商户的上架菜品
SELECT d.* FROM dishes d
JOIN merchants m ON d.merchant_id = m.id
WHERE 
  m.status = 'active'
  AND m.deleted_at IS NULL
  AND d.deleted_at IS NULL
  AND d.is_online = true
  AND d.name ILIKE '%' || $1 || '%'
ORDER BY d.sort_order ASC, d.name ASC
LIMIT $2 OFFSET $3;

-- name: CountSearchDishesGlobal :one
-- 统计全局菜品搜索结果总数
SELECT COUNT(*) FROM dishes d
JOIN merchants m ON d.merchant_id = m.id
WHERE 
  m.status = 'active'
  AND m.deleted_at IS NULL
  AND d.deleted_at IS NULL
  AND d.is_online = true
  AND d.name ILIKE '%' || $1 || '%';

-- name: SearchDishIDsGlobal :many
-- 全局菜品搜索，只返回菜品ID（用于推荐接口的关键词过滤）
SELECT d.id FROM dishes d
JOIN merchants m ON d.merchant_id = m.id
WHERE 
  m.status = 'active'
  AND m.deleted_at IS NULL
  AND d.deleted_at IS NULL
  AND d.is_online = true
  AND d.name ILIKE '%' || $1 || '%'
ORDER BY d.sort_order ASC, d.name ASC;

-- name: UpdateDish :one
UPDATE dishes
SET
  category_id = COALESCE(sqlc.narg('category_id'), category_id),
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  image_url = COALESCE(sqlc.narg('image_url'), image_url),
  price = COALESCE(sqlc.narg('price'), price),
  member_price = COALESCE(sqlc.narg('member_price'), member_price),
  is_available = COALESCE(sqlc.narg('is_available'), is_available),
  is_online = COALESCE(sqlc.narg('is_online'), is_online),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
  prepare_time = COALESCE(sqlc.narg('prepare_time'), prepare_time),
  updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDishAvailability :exec
UPDATE dishes
SET 
  is_available = $2,
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateDishOnlineStatus :exec
UPDATE dishes
SET 
  is_online = $2,
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteDish :exec
-- 软删除菜品
UPDATE dishes SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountDishesByMerchant :one
SELECT COUNT(*) FROM dishes
WHERE 
  merchant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('is_online')::boolean IS NULL OR is_online = sqlc.narg('is_online'));

-- ============================================
-- 菜品食材关联查询 (Dish Ingredient Queries)
-- ============================================

-- name: AddDishIngredient :one
INSERT INTO dish_ingredients (
  dish_id,
  ingredient_id
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListDishIngredients :many
SELECT 
  i.*
FROM ingredients i
JOIN dish_ingredients di ON i.id = di.ingredient_id
WHERE di.dish_id = $1
ORDER BY i.name ASC;

-- name: RemoveDishIngredient :exec
DELETE FROM dish_ingredients
WHERE dish_id = $1 AND ingredient_id = $2;

-- name: RemoveAllDishIngredients :exec
DELETE FROM dish_ingredients
WHERE dish_id = $1;

-- ============================================
-- 菜品标签关联查询 (Dish Tag Queries)
-- ============================================

-- name: AddDishTag :one
INSERT INTO dish_tags (
  dish_id,
  tag_id
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListDishTags :many
SELECT 
  t.id,
  t.name,
  t.type,
  t.sort_order,
  t.status,
  t.created_at
FROM tags t
JOIN dish_tags dt ON t.id = dt.tag_id
WHERE dt.dish_id = $1
ORDER BY t.sort_order ASC;

-- name: RemoveDishTag :exec
DELETE FROM dish_tags
WHERE dish_id = $1 AND tag_id = $2;

-- name: RemoveAllDishTags :exec
DELETE FROM dish_tags
WHERE dish_id = $1;

-- ============================================
-- 菜品定制选项查询 (Dish Customization Queries)
-- ============================================

-- name: CreateDishCustomizationGroup :one
INSERT INTO dish_customization_groups (
  dish_id,
  name,
  is_required,
  sort_order
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: ListDishCustomizationGroups :many
SELECT * FROM dish_customization_groups
WHERE dish_id = $1
ORDER BY sort_order ASC;

-- name: UpdateDishCustomizationGroup :one
UPDATE dish_customization_groups
SET
  name = COALESCE(sqlc.narg('name'), name),
  is_required = COALESCE(sqlc.narg('is_required'), is_required),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteDishCustomizationGroup :exec
DELETE FROM dish_customization_groups
WHERE id = $1;

-- name: DeleteAllDishCustomizationGroups :exec
DELETE FROM dish_customization_groups
WHERE dish_id = $1;

-- name: CreateDishCustomizationOption :one
INSERT INTO dish_customization_options (
  group_id,
  tag_id,
  extra_price,
  sort_order
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: ListDishCustomizationOptions :many
SELECT 
  dco.*,
  t.name as tag_name,
  t.type as tag_type
FROM dish_customization_options dco
JOIN tags t ON dco.tag_id = t.id
WHERE dco.group_id = $1
ORDER BY dco.sort_order ASC;

-- name: GetDishWithCustomizations :one
SELECT 
  d.*,
  COALESCE(
    json_agg(
      json_build_object(
        'id', dcg.id,
        'name', dcg.name,
        'is_required', dcg.is_required,
        'sort_order', dcg.sort_order,
        'options', (
          SELECT json_agg(
            json_build_object(
              'id', dco.id,
              'tag_id', dco.tag_id,
              'tag_name', t.name,
              'extra_price', dco.extra_price,
              'sort_order', dco.sort_order
            ) ORDER BY dco.sort_order
          )
          FROM dish_customization_options dco
          JOIN tags t ON dco.tag_id = t.id
          WHERE dco.group_id = dcg.id
        )
      ) ORDER BY dcg.sort_order
    ) FILTER (WHERE dcg.id IS NOT NULL),
    '[]'
  ) as customization_groups
FROM dishes d
LEFT JOIN dish_customization_groups dcg ON d.id = dcg.dish_id
WHERE d.id = $1 AND d.deleted_at IS NULL
GROUP BY d.id;

-- name: GetDishComplete :one
SELECT 
  d.*,
  dc.name as category_name,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'id', i.id,
        'name', i.name,
        'category', i.category,
        'is_allergen', i.is_allergen
      )
    ) FILTER (WHERE i.id IS NOT NULL),
    '[]'
  ) as ingredients,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'id', t.id,
        'name', t.name
      )
    ) FILTER (WHERE t.id IS NOT NULL),
    '[]'
  ) as tags,
  COALESCE(
    (
      SELECT json_agg(
        json_build_object(
          'id', dcg.id,
          'name', dcg.name,
          'is_required', dcg.is_required,
          'sort_order', dcg.sort_order,
          'options', (
            SELECT json_agg(
              json_build_object(
                'id', dco.id,
                'tag_id', dco.tag_id,
                'tag_name', opt_tag.name,
                'extra_price', dco.extra_price,
                'sort_order', dco.sort_order
              ) ORDER BY dco.sort_order
            )
            FROM dish_customization_options dco
            JOIN tags opt_tag ON dco.tag_id = opt_tag.id
            WHERE dco.group_id = dcg.id
          )
        ) ORDER BY dcg.sort_order
      )
      FROM dish_customization_groups dcg
      WHERE dcg.dish_id = d.id
    ),
    '[]'
  ) as customization_groups
FROM dishes d
LEFT JOIN dish_categories dc ON d.category_id = dc.id
LEFT JOIN dish_ingredients di ON d.id = di.dish_id
LEFT JOIN ingredients i ON di.ingredient_id = i.id
LEFT JOIN dish_tags dt ON d.id = dt.dish_id
LEFT JOIN tags t ON dt.tag_id = t.id
WHERE d.id = $1 AND d.deleted_at IS NULL
GROUP BY d.id, dc.name;

-- name: DeleteDishCustomizationOption :exec
DELETE FROM dish_customization_options
WHERE id = $1;

-- ============================================
-- 推荐系统查询 (Recommendation Queries)
-- ============================================

-- name: GetPopularDishes :many
-- 获取全平台热门菜品（基于销量）
SELECT 
    d.id,
    d.merchant_id,
    d.name,
    d.description,
    d.image_url,
    d.price,
    d.member_price,
    COALESCE(SUM(oi.quantity), 0)::int AS total_sold
FROM dishes d
LEFT JOIN order_items oi ON d.id = oi.dish_id
LEFT JOIN orders o ON o.id = oi.order_id AND o.status IN ('delivered', 'completed')
WHERE d.is_online = true 
  AND d.is_available = true
  AND d.deleted_at IS NULL
GROUP BY d.id
ORDER BY total_sold DESC, d.created_at DESC
LIMIT $1;

-- name: GetRandomDishes :many
-- 获取随机菜品（用于推荐探索）
SELECT id FROM dishes
WHERE is_online = true 
  AND is_available = true
  AND deleted_at IS NULL
ORDER BY RANDOM()
LIMIT $1;

-- name: GetDishIDsByCuisines :many
-- 根据菜系获取菜品ID（用于基于偏好推荐）
-- 注：当前merchants表无cuisine_type字段，简化为按价格区间查询热门菜品
SELECT d.id FROM dishes d
WHERE d.is_online = true 
  AND d.is_available = true
  AND d.deleted_at IS NULL
  AND d.price >= $1
  AND d.price <= $2
ORDER BY d.created_at DESC
LIMIT $3;

-- name: GetUserPurchasedDishIDs :many
-- 获取用户购买过的菜品ID（用于排除已购买）
SELECT DISTINCT oi.dish_id 
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE o.user_id = $1
  AND o.status IN ('delivered', 'completed')
  AND o.created_at >= $2;

-- name: GetExploreDishes :many
-- 获取用户未购买过的热门菜品（探索推荐）
SELECT 
    d.id,
    COALESCE(SUM(oi.quantity), 0)::int AS total_sold
FROM dishes d
LEFT JOIN order_items oi ON d.id = oi.dish_id
LEFT JOIN orders o ON o.id = oi.order_id AND o.status IN ('delivered', 'completed')
WHERE d.is_online = true 
  AND d.is_available = true
  AND d.deleted_at IS NULL
  AND d.id NOT IN (
    SELECT DISTINCT oi2.dish_id 
    FROM order_items oi2
    JOIN orders o2 ON o2.id = oi2.order_id
    WHERE o2.user_id = $1
      AND o2.status IN ('delivered', 'completed')
  )
GROUP BY d.id
ORDER BY total_sold DESC
LIMIT $2;

-- name: GetDishesByIDs :many
-- 批量获取菜品详情（用于推荐结果）
SELECT 
    id,
    merchant_id,
    name,
    description,
    image_url,
    price,
    member_price,
    is_available,
    is_online
FROM dishes
WHERE id = ANY($1::bigint[])
  AND deleted_at IS NULL
  AND is_online = true;

-- name: GetDishesWithMerchantByIDs :many
-- 批量获取菜品详情及商户信息（用于推荐流展示）
-- 返回菜品信息、商户信息、近30天销量
SELECT 
    d.id,
    d.merchant_id,
    d.name,
    d.description,
    d.image_url,
    d.price,
    d.member_price,
    d.is_available,
    d.is_online,
    m.name AS merchant_name,
    m.logo_url AS merchant_logo,
    m.latitude AS merchant_latitude,
    m.longitude AS merchant_longitude,
    m.region_id AS merchant_region_id,
    m.is_open AS merchant_is_open,
    COALESCE(
        (SELECT SUM(oi.quantity)
         FROM order_items oi 
         JOIN orders o ON o.id = oi.order_id 
         WHERE oi.dish_id = d.id 
           AND o.status IN ('delivered', 'completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_sales
FROM dishes d
JOIN merchants m ON m.id = d.merchant_id
WHERE d.id = ANY($1::bigint[])
  AND d.deleted_at IS NULL
  AND d.is_online = true
  AND m.status = 'active';

-- name: GetDishesByIDsAll :many
-- 批量获取菜品（不过滤上下架状态，用于权限验证等）
SELECT 
    id,
    merchant_id,
    name,
    description,
    image_url,
    price,
    member_price,
    is_available,
    is_online
FROM dishes
WHERE id = ANY($1::bigint[])
  AND deleted_at IS NULL;

-- name: BatchUpdateDishOnlineStatus :execrows
-- 批量更新菜品上下架状态（只更新属于指定商户的菜品）
UPDATE dishes
SET is_online = $1, updated_at = NOW()
WHERE id = ANY($2::bigint[])
  AND merchant_id = $3
  AND deleted_at IS NULL
RETURNING id;

-- name: ListDishesForMenu :many
-- 获取商户上架菜品（用于扫码点餐菜单展示）
SELECT 
    d.id,
    d.category_id,
    d.name,
    d.description,
    d.image_url,
    d.price,
    d.member_price,
    d.is_available,
    d.sort_order
FROM dishes d
WHERE d.merchant_id = $1
  AND d.is_online = true
  AND d.deleted_at IS NULL
ORDER BY d.category_id, d.sort_order ASC, d.id ASC;

-- name: GetDishIDsByTagID :many
-- 获取带有指定标签的菜品ID列表（用于推荐过滤）
SELECT DISTINCT d.id
FROM dishes d
JOIN dish_tags dt ON d.id = dt.dish_id
WHERE dt.tag_id = $1
  AND d.is_online = true
  AND d.is_available = true
  AND d.deleted_at IS NULL;
