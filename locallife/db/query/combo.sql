-- ============================================
-- 套餐查询 (Combo Queries)
-- ============================================

-- name: CreateComboSet :one
INSERT INTO combo_sets (
  merchant_id,
  name,
  description,
  image_media_asset_id,
  original_price,
  combo_price,
  is_online
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetComboSet :one
SELECT id, merchant_id, name, description, original_price, combo_price, is_online, created_at, updated_at, deleted_at, image_media_asset_id FROM combo_sets
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetComboSetWithDetails :one
SELECT 
  cs.id, cs.merchant_id, cs.name, cs.description, cs.original_price, cs.combo_price, cs.is_online, cs.created_at, cs.updated_at, cs.deleted_at, cs.image_media_asset_id,
  COALESCE(
    json_agg(DISTINCT
      jsonb_build_object(
        'dish_id', cd.dish_id,
        'dish_name', d.name,
        'dish_price', COALESCE(cd.dish_base_price_snapshot, d.price) + COALESCE(cd.customization_extra_price, 0),
        'dish_image_media_asset_id', d.image_media_asset_id,
        'quantity', cd.quantity,
        'customizations', COALESCE(cd.customizations, '{}'::jsonb),
        'customization_extra_price', COALESCE(cd.customization_extra_price, 0),
        'customization_summary', COALESCE(cd.customizations ->> 'meta_specs', '')
      )
    ) FILTER (WHERE d.id IS NOT NULL),
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
LEFT JOIN dishes d ON cd.dish_id = d.id AND d.deleted_at IS NULL
LEFT JOIN combo_tags ct ON cs.id = ct.combo_id
LEFT JOIN tags t ON ct.tag_id = t.id
WHERE cs.id = $1 AND cs.deleted_at IS NULL
GROUP BY cs.id;

-- name: ListComboSetsByMerchant :many
SELECT
  cs.id,
  cs.name,
  cs.description,
  cs.original_price,
  cs.combo_price,
  cs.is_online,
  COALESCE(dish_stats.dish_count, 0)::bigint AS dish_count,
  COALESCE(dish_stats.dish_total_quantity, 0)::bigint AS dish_total_quantity,
  COALESCE(tag_stats.tags, '[]'::json) AS tags
FROM combo_sets cs
LEFT JOIN LATERAL (
  SELECT
    COUNT(d.id)::bigint AS dish_count,
    COALESCE(SUM(cd.quantity), 0)::bigint AS dish_total_quantity
  FROM combo_dishes cd
  JOIN dishes d ON d.id = cd.dish_id AND d.deleted_at IS NULL
  WHERE cd.combo_id = cs.id
) AS dish_stats ON TRUE
LEFT JOIN LATERAL (
  SELECT COALESCE(
    json_agg(
      jsonb_build_object(
        'id', t.id,
        'name', t.name
      )
      ORDER BY t.sort_order ASC, t.id ASC
    ),
    '[]'::json
  ) AS tags
  FROM combo_tags ct
  JOIN tags t ON t.id = ct.tag_id
  WHERE ct.combo_id = cs.id
) AS tag_stats ON TRUE
WHERE 
  cs.merchant_id = $1
  AND cs.deleted_at IS NULL
  AND (sqlc.narg('is_online')::boolean IS NULL OR cs.is_online = sqlc.narg('is_online'))
ORDER BY cs.created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateComboSet :one
UPDATE combo_sets
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  image_media_asset_id = COALESCE(sqlc.narg('image_media_asset_id'), image_media_asset_id),
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
  quantity,
  dish_base_price_snapshot,
  customizations,
  customization_extra_price
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListComboDishes :many
SELECT 
  d.id, d.merchant_id, d.category_id, d.name, d.description, d.price, d.member_price, d.is_available, d.is_online, d.sort_order, d.created_at, d.updated_at, d.prepare_time, d.deleted_at, d.monthly_sales, d.repurchase_rate, d.image_media_asset_id, d.is_packaging,
  cd.quantity,
  cd.dish_base_price_snapshot,
  cd.customizations,
  cd.customization_extra_price
FROM dishes d
JOIN combo_dishes cd ON d.id = cd.dish_id
WHERE cd.combo_id = $1 AND d.deleted_at IS NULL
ORDER BY cd.id ASC;

-- name: RemoveComboDish :exec
DELETE FROM combo_dishes
WHERE combo_id = $1 AND dish_id = $2;

-- name: RemoveDishFromAllCombos :exec
DELETE FROM combo_dishes
WHERE dish_id = $1;

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
    cs.image_media_asset_id,
    cs.original_price,
    cs.combo_price,
    COALESCE(SUM(oi.quantity), 0)::int AS total_sold
FROM combo_sets cs
LEFT JOIN order_items oi ON cs.id = oi.combo_id
LEFT JOIN orders o ON o.id = oi.order_id AND o.status IN ('user_delivered', 'completed')
LEFT JOIN merchants m ON cs.merchant_id = m.id
WHERE cs.is_online = true AND cs.deleted_at IS NULL AND m.status = 'active'
GROUP BY cs.id, m.is_open, m.latitude, m.longitude
ORDER BY 
    m.is_open DESC, 
    total_sold DESC, 
    cs.created_at DESC,
    earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth($2::float8, $3::float8)) ASC
LIMIT $1;

-- name: GetCombosByIDs :many
-- 批量获取套餐详情
SELECT 
    id,
    merchant_id,
    name,
    description,
    image_media_asset_id,
    original_price,
    combo_price,
    is_online
FROM combo_sets
WHERE id = ANY($1::bigint[])
  AND deleted_at IS NULL
  AND is_online = true;

-- name: GetCombosWithMerchantByIDs :many
-- 批量获取套餐详情及商户信息（用于推荐流展示）
-- 当套餐没有专属图片时，使用套餐内第一个菜品的图片作为展示图
SELECT 
    cs.id,
    cs.merchant_id,
    cs.name,
    cs.description,
    COALESCE(
        NULLIF(cs.image_media_asset_id::text, ''),
        (SELECT d.image_media_asset_id::text
         FROM combo_dishes cd 
         JOIN dishes d ON cd.dish_id = d.id 
         WHERE cd.combo_id = cs.id 
           AND d.image_media_asset_id IS NOT NULL
           AND d.deleted_at IS NULL
         ORDER BY cd.id ASC 
         LIMIT 1)
    ) AS image_media_asset_id,
    cs.original_price,
    cs.combo_price,
    cs.is_online,
    m.name AS merchant_name,
    m.logo_media_asset_id AS merchant_logo_media_asset_id,
    m.latitude AS merchant_latitude,
    m.longitude AS merchant_longitude,
    m.region_id AS merchant_region_id,
    m.is_open AS merchant_is_open,
    COALESCE(
        (SELECT SUM(oi.quantity)
         FROM order_items oi 
         JOIN orders o ON o.id = oi.order_id 
         WHERE oi.combo_id = cs.id 
           AND o.status IN ('user_delivered', 'completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_sales
FROM combo_sets cs
JOIN merchants m ON m.id = cs.merchant_id
WHERE cs.id = ANY($1::bigint[])
  AND cs.deleted_at IS NULL
  AND cs.is_online = true
  AND m.status = 'active';

-- name: ListOnlineCombosByMerchant :many
-- 获取商户上架套餐（用于扫码点餐菜单展示）
SELECT 
    cs.id,
    cs.merchant_id,
    cs.name,
    cs.description,
    cs.image_media_asset_id,
    cs.original_price,
    cs.combo_price AS price,
    cs.is_online,
    COALESCE(
      (SELECT json_agg(t.name)
       FROM combo_tags ct
       JOIN tags t ON ct.tag_id = t.id
       WHERE ct.combo_id = cs.id),
      '[]'
    ) as tags
FROM combo_sets cs
WHERE merchant_id = $1
  AND deleted_at IS NULL
  AND is_online = true
ORDER BY created_at DESC;

-- name: SearchComboIDsGlobal :many
-- 全局套餐搜索，只返回套餐ID（用于推荐接口的关键词过滤）
SELECT cs.id FROM combo_sets cs
JOIN merchants m ON cs.merchant_id = m.id
WHERE 
  m.status = 'active'
  AND m.deleted_at IS NULL
  AND cs.deleted_at IS NULL
  AND cs.is_online = true
  AND cs.name ILIKE '%' || $1 || '%'
ORDER BY cs.created_at DESC;

-- name: SearchCombosGlobal :many
-- Consumer-Facing Global Combo Search
-- Returns enriched data for the home feed or search page.
-- Strict filters: Online combos only, Active merchants only.
-- Sorting: Open Merchants First > Sales (Weighted) > Distance.
SELECT 
    cs.id,
    cs.merchant_id,
    cs.name,
    cs.description,
    cs.image_media_asset_id,
  dish_img.image_media_asset_id AS fallback_image_media_asset_id,
    cs.original_price,
    cs.combo_price,
    cs.is_online,
    m.name AS merchant_name,
    m.logo_media_asset_id AS merchant_logo_media_asset_id,
    m.latitude AS merchant_latitude,
    m.longitude AS merchant_longitude,
    m.region_id AS merchant_region_id,
    m.is_open AS merchant_is_open,
    -- Calculate Sales using Order Items (Last 30 days) for Relevance
    COALESCE(
        (SELECT SUM(oi.quantity)
         FROM order_items oi 
         JOIN orders o ON o.id = oi.order_id 
         WHERE oi.combo_id = cs.id 
           AND o.status IN ('user_delivered', 'completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_sales,
    -- Distance Calculation
    COALESCE(earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth($4::float8, $5::float8)), 9999999)::float8 AS distance,
    COALESCE(
      (SELECT json_agg(t.name)
       FROM combo_tags ct
       JOIN tags t ON ct.tag_id = t.id
       WHERE ct.combo_id = cs.id),
      '[]'
    ) as tags
FROM combo_sets cs
JOIN merchants m ON cs.merchant_id = m.id
LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
LEFT JOIN LATERAL (
    SELECT d.image_media_asset_id
    FROM combo_dishes cd
    JOIN dishes d ON cd.dish_id = d.id
    WHERE cd.combo_id = cs.id
      AND d.image_media_asset_id IS NOT NULL
      AND d.deleted_at IS NULL
    ORDER BY cd.id ASC
    LIMIT 1
) AS dish_img ON TRUE
WHERE 
    m.status = 'active'
    AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
    AND cs.deleted_at IS NULL
    AND cs.is_online = true
    AND (
        $1::text = '' OR 
        cs.name ILIKE '%' || $1 || '%' OR 
        m.name ILIKE '%' || $1 || '%'
    )
ORDER BY 
    m.is_open DESC, 
    monthly_sales DESC,
    distance ASC
LIMIT $2 OFFSET $3;

-- name: CountSearchCombosGlobal :one
-- Count for pagination
SELECT COUNT(*)
FROM combo_sets cs
JOIN merchants m ON cs.merchant_id = m.id
LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
WHERE 
    m.status = 'active'
    AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
    AND cs.deleted_at IS NULL
    AND cs.is_online = true
    AND (
        $1::text = '' OR 
        cs.name ILIKE '%' || $1 || '%' OR 
        m.name ILIKE '%' || $1 || '%'
    );

-- name: GetComboMemberImagesByCombos :many
-- 批量获取多个套餐的成员图片
SELECT cd.combo_id, d.image_media_asset_id
FROM combo_dishes cd
JOIN dishes d ON cd.dish_id = d.id
WHERE cd.combo_id = ANY($1::bigint[])
  AND d.deleted_at IS NULL
ORDER BY cd.combo_id, cd.id ASC;
