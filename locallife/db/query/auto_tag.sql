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
SELECT id, name, type, sort_order, status, created_at FROM tags WHERE name = $1 AND type = 'system' LIMIT 1;

-- name: UpsertDishTag :exec
-- 添加或更新菜品标签关联
INSERT INTO dish_tags (dish_id, tag_id, created_at)
VALUES ($1, $2, NOW())
ON CONFLICT (dish_id, tag_id) DO NOTHING;

-- name: GetDishSales :one
-- 获取单个菜品近30天销量
SELECT COALESCE(SUM(oi.quantity), 0)::int
FROM order_items oi
JOIN orders o ON oi.order_id = o.id
WHERE oi.dish_id = $1
  AND o.status IN ('user_delivered', 'completed')
  AND o.created_at >= NOW() - INTERVAL '30 days';
