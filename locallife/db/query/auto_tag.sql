-- name: GetHotSellingDishIDs :many
-- 获取热卖菜品ID列表（近7天销量 >= 指定阈值）
SELECT d.id
FROM dishes d
JOIN order_items oi ON d.id = oi.dish_id
JOIN orders o ON oi.order_id = o.id
WHERE o.status = 'completed'
  AND o.created_at >= NOW() - INTERVAL '7 days'
  AND d.is_online = true
  AND d.is_available = true
  AND d.deleted_at IS NULL
GROUP BY d.id
HAVING SUM(oi.quantity) >= $1;

-- name: GetQualityDishIDs :many
-- 获取无投诉的高质量菜品ID列表
-- 条件: 销量>=指定阈值, 近30天无投诉, 商户无食品安全事故
SELECT d.id
FROM dishes d
JOIN order_items oi ON d.id = oi.dish_id
JOIN orders o ON oi.order_id = o.id
WHERE o.status = 'completed'
  AND d.is_online = true
  AND d.is_available = true
  AND d.deleted_at IS NULL
  -- 排除近30天内有投诉的菜品
  AND d.id NOT IN (
      SELECT DISTINCT oi2.dish_id
      FROM claims c
      JOIN orders o2 ON c.order_id = o2.id
      JOIN order_items oi2 ON o2.id = oi2.order_id
      WHERE c.created_at >= NOW() - INTERVAL '30 days'
        AND c.status IN ('approved', 'pending')
  )
  -- 排除近30天内有食品安全事故的商户的菜品
  AND d.merchant_id NOT IN (
      SELECT DISTINCT merchant_id
      FROM food_safety_incidents
      WHERE created_at >= NOW() - INTERVAL '30 days'
  )
GROUP BY d.id
HAVING SUM(oi.quantity) >= $1;

-- name: GetDishRepurchaseRate :one
-- 获取单个菜品的复购率（用于过滤）
SELECT 
    COUNT(DISTINCT o.user_id) AS total_users,
    COUNT(DISTINCT CASE 
        WHEN (
            SELECT COUNT(*) FROM order_items oi3 
            JOIN orders o3 ON oi3.order_id = o3.id 
            WHERE oi3.dish_id = $1 AND o3.user_id = o.user_id AND o3.status = 'completed'
        ) >= 2 THEN o.user_id 
    END) AS repurchase_users
FROM order_items oi
JOIN orders o ON oi.order_id = o.id
WHERE oi.dish_id = $1 AND o.status = 'completed';

-- name: GetSystemTagByName :one
-- 根据名称获取系统标签
SELECT * FROM tags WHERE name = $1 AND type = 'system' LIMIT 1;

-- name: UpsertDishTag :exec
-- 添加或更新菜品标签关联
INSERT INTO dish_tags (dish_id, tag_id, created_at)
VALUES ($1, $2, NOW())
ON CONFLICT (dish_id, tag_id) DO NOTHING;

-- name: DeleteDishTagByTagID :exec
-- 删除指定标签的所有菜品关联（用于清理过期的自动标签）
DELETE FROM dish_tags WHERE tag_id = $1;

-- name: GetDishIDsWithTag :many
-- 获取有指定标签的所有菜品ID
SELECT dish_id FROM dish_tags WHERE tag_id = $1;
