-- M12: 平台端统计查询

-- name: GetPlatformOverview :one
-- 平台全局概览
SELECT 
    COUNT(DISTINCT CASE WHEN o.created_at >= $1 AND o.created_at <= $2 THEN o.id END)::int AS total_orders,
    COALESCE(SUM(CASE WHEN o.created_at >= $1 AND o.created_at <= $2 AND o.status IN ('delivered', 'completed') THEN o.final_amount ELSE 0 END), 0)::bigint AS total_gmv,
    COALESCE(SUM(CASE WHEN o.created_at >= $1 AND o.created_at <= $2 AND o.status IN ('delivered', 'completed') THEN o.platform_commission ELSE 0 END), 0)::bigint AS total_commission,
    COUNT(DISTINCT CASE WHEN o.created_at >= $1 AND o.created_at <= $2 THEN o.merchant_id END)::int AS active_merchants,
    COUNT(DISTINCT CASE WHEN o.created_at >= $1 AND o.created_at <= $2 THEN o.user_id END)::int AS active_users
FROM orders o
WHERE o.status NOT IN ('cancelled');

-- name: GetPlatformDailyStats :many
-- 平台日统计
SELECT 
    DATE(o.created_at) AS date,
    COUNT(*)::int AS order_count,
    COALESCE(SUM(CASE WHEN o.status IN ('delivered', 'completed') THEN o.final_amount ELSE 0 END), 0)::bigint AS total_gmv,
    COALESCE(SUM(CASE WHEN o.status IN ('delivered', 'completed') THEN o.platform_commission ELSE 0 END), 0)::bigint AS total_commission,
    COUNT(DISTINCT o.merchant_id)::int AS active_merchants,
    COUNT(DISTINCT o.user_id)::int AS active_users,
    COUNT(CASE WHEN o.order_type = 'takeout' THEN 1 END)::int AS takeout_orders,
    COUNT(CASE WHEN o.order_type = 'dine_in' THEN 1 END)::int AS dine_in_orders
FROM orders o
WHERE o.created_at >= $1 AND o.created_at <= $2
GROUP BY DATE(o.created_at)
ORDER BY date;

-- name: GetRegionComparison :many
-- 区域对比分析
SELECT 
    r.id AS region_id,
    r.name AS region_name,
    COUNT(DISTINCT m.id)::int AS merchant_count,
    COUNT(DISTINCT o.id)::int AS order_count,
    COALESCE(SUM(o.final_amount), 0)::bigint AS total_gmv,
    COALESCE(SUM(o.platform_commission), 0)::bigint AS total_commission,
    COALESCE(AVG(o.final_amount), 0)::bigint AS avg_order_amount,
    COUNT(DISTINCT o.user_id)::int AS active_users
FROM regions r
LEFT JOIN merchants m ON m.region_id = r.id AND m.status = 'approved'
LEFT JOIN orders o ON o.merchant_id = m.id 
    AND o.created_at >= $1 AND o.created_at <= $2
    AND o.status IN ('delivered', 'completed')
GROUP BY r.id, r.name
ORDER BY total_gmv DESC;

-- name: GetMerchantRanking :many
-- 商户销售排行
SELECT 
    m.id AS merchant_id,
    m.name AS merchant_name,
    m.region_id,
    r.name AS region_name,
    COUNT(o.id)::int AS order_count,
    COALESCE(SUM(o.final_amount), 0)::bigint AS total_sales,
    COALESCE(SUM(o.platform_commission), 0)::bigint AS total_commission,
    COALESCE(AVG(o.final_amount), 0)::bigint AS avg_order_amount
FROM merchants m
JOIN regions r ON r.id = m.region_id
LEFT JOIN orders o ON o.merchant_id = m.id 
    AND o.created_at >= $1 AND o.created_at <= $2
    AND o.status IN ('delivered', 'completed')
WHERE m.status = 'approved'
GROUP BY m.id, m.name, m.region_id, r.name
ORDER BY total_sales DESC
LIMIT $3 OFFSET $4;

-- name: GetCategoryStats :many
-- 分类销售统计
SELECT 
    t.name AS category_name,
    COUNT(DISTINCT m.id)::int AS merchant_count,
    COUNT(o.id)::int AS order_count,
    COALESCE(SUM(o.final_amount), 0)::bigint AS total_sales
FROM tags t
JOIN merchant_tags mt ON mt.tag_id = t.id
JOIN merchants m ON m.id = mt.merchant_id AND m.status = 'approved'
LEFT JOIN orders o ON o.merchant_id = m.id 
    AND o.created_at >= $1 AND o.created_at <= $2
    AND o.status IN ('delivered', 'completed')
WHERE t.type = 'merchant'
GROUP BY t.id, t.name
ORDER BY total_sales DESC;

-- name: GetUserGrowthStats :many
-- 用户增长统计
SELECT 
    DATE(created_at) AS date,
    COUNT(*)::int AS new_users
FROM users
WHERE created_at >= $1 AND created_at <= $2
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetMerchantGrowthStats :many
-- 商户增长统计
SELECT 
    DATE(created_at) AS date,
    COUNT(*)::int AS new_merchants
FROM merchants
WHERE created_at >= $1 AND created_at <= $2
  AND status = 'approved'
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetRiderPerformanceRanking :many
-- 骑手绩效排行(全平台)
SELECT 
    r.id AS rider_id,
    u.full_name AS rider_name,
    COUNT(d.id)::int AS delivery_count,
    COUNT(CASE WHEN d.status = 'completed' THEN 1 END)::int AS completed_count,
    COALESCE(AVG(EXTRACT(EPOCH FROM (d.delivered_at - d.picked_at))), 0)::int AS avg_delivery_time_seconds,
    r.total_earnings AS total_earnings
FROM riders r
JOIN users u ON u.id = r.user_id
LEFT JOIN deliveries d ON d.rider_id = r.id 
    AND d.created_at >= $1 AND d.created_at <= $2
WHERE r.status = 'active'
GROUP BY r.id, u.full_name, r.total_earnings
HAVING COUNT(d.id) > 0
ORDER BY completed_count DESC
LIMIT $3 OFFSET $4;

-- name: GetHourlyDistribution :many
-- 订单时段分布
SELECT 
    EXTRACT(HOUR FROM created_at)::int AS hour,
    COUNT(*)::int AS order_count,
    COALESCE(SUM(CASE WHEN status IN ('delivered', 'completed') THEN final_amount ELSE 0 END), 0)::bigint AS total_gmv
FROM orders
WHERE created_at >= $1 AND created_at <= $2
GROUP BY EXTRACT(HOUR FROM created_at)
ORDER BY hour;

-- name: GetRealtimeDashboard :one
-- 实时大盘数据(最近24小时)
SELECT 
    COUNT(*)::int AS orders_24h,
    COALESCE(SUM(CASE WHEN status IN ('delivered', 'completed') THEN final_amount ELSE 0 END), 0)::bigint AS gmv_24h,
    COUNT(DISTINCT merchant_id)::int AS active_merchants_24h,
    COUNT(DISTINCT user_id)::int AS active_users_24h,
    COUNT(CASE WHEN status = 'pending' THEN 1 END)::int AS pending_orders,
    COUNT(CASE WHEN status = 'preparing' THEN 1 END)::int AS preparing_orders,
    COUNT(CASE WHEN status = 'ready' THEN 1 END)::int AS ready_orders,
    COUNT(CASE WHEN status = 'delivering' THEN 1 END)::int AS delivering_orders
FROM orders
WHERE created_at >= NOW() - INTERVAL '24 hours';
