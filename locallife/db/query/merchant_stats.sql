-- M12: 商户统计查询 (实时计算)

-- name: GetMerchantDailyStats :many
-- 商户日报: 按天聚合订单数据
SELECT 
    DATE(created_at) AS date,
    COUNT(*)::int AS order_count,
    COALESCE(SUM(final_amount), 0)::bigint AS total_sales,
    COALESCE(SUM(platform_commission), 0)::bigint AS commission,
    COUNT(*) FILTER (WHERE order_type = 'takeout')::int AS takeout_orders,
    COUNT(*) FILTER (WHERE order_type = 'dine_in')::int AS dine_in_orders
FROM orders
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND status IN ('delivered', 'completed')
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- name: GetMerchantOverview :one
-- 商户概览: 指定日期范围的汇总统计
SELECT 
    COUNT(DISTINCT DATE(created_at))::int AS total_days,
    COUNT(*)::int AS total_orders,
    COALESCE(SUM(final_amount), 0)::bigint AS total_sales,
    COALESCE(SUM(platform_commission), 0)::bigint AS total_commission,
    CASE 
        WHEN COUNT(DISTINCT DATE(created_at)) > 0 
        THEN (COALESCE(SUM(final_amount), 0) / COUNT(DISTINCT DATE(created_at)))::bigint
        ELSE 0
    END AS avg_daily_sales
FROM orders
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND status IN ('delivered', 'completed');

-- name: GetTopSellingDishes :many
-- 菜品销量排行: 从order_items实时聚合
SELECT 
    oi.dish_id,
    d.name AS dish_name,
    d.price AS dish_price,
    SUM(oi.quantity)::int AS total_sold,
    COALESCE(SUM(oi.subtotal), 0)::bigint AS total_revenue
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
JOIN orders o ON o.id = oi.order_id
WHERE o.merchant_id = $1
  AND o.created_at >= $2
  AND o.created_at <= $3
  AND o.status IN ('delivered', 'completed')
GROUP BY oi.dish_id, d.name, d.price
ORDER BY total_sold DESC
LIMIT $4;

-- name: GetMerchantCustomerStats :many
-- 顾客消费分析: 实时计算每个顾客的消费统计
SELECT 
    o.user_id,
    u.full_name,
    u.phone,
    u.avatar_url,
    COUNT(*)::int AS total_orders,
    COALESCE(SUM(o.final_amount), 0)::bigint AS total_amount,
    CASE 
        WHEN COUNT(*) > 0 
        THEN (COALESCE(SUM(o.final_amount), 0) / COUNT(*))::bigint
        ELSE 0
    END AS avg_order_amount,
    MIN(o.created_at) AS first_order_at,
    MAX(o.created_at) AS last_order_at
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.merchant_id = $1
  AND o.status IN ('delivered', 'completed')
GROUP BY o.user_id, u.full_name, u.phone, u.avatar_url
ORDER BY 
    CASE 
        WHEN sqlc.arg(order_by)::text = 'total_orders' THEN COUNT(*)
        WHEN sqlc.arg(order_by)::text = 'total_amount' THEN SUM(o.final_amount)
        ELSE EXTRACT(EPOCH FROM MAX(o.created_at))
    END DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantCustomers :one
-- 统计商户的顾客总数
SELECT COUNT(DISTINCT user_id)::int
FROM orders
WHERE merchant_id = $1
  AND status IN ('delivered', 'completed');

-- name: GetCustomerMerchantDetail :one
-- 单个顾客在某商户的消费详情
SELECT 
    o.user_id,
    u.full_name,
    u.phone,
    u.avatar_url,
    COUNT(*)::int AS total_orders,
    COALESCE(SUM(o.final_amount), 0)::bigint AS total_amount,
    CASE 
        WHEN COUNT(*) > 0 
        THEN (COALESCE(SUM(o.final_amount), 0) / COUNT(*))::bigint
        ELSE 0
    END AS avg_order_amount,
    MIN(o.created_at) AS first_order_at,
    MAX(o.created_at) AS last_order_at
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.merchant_id = $1
  AND o.user_id = $2
  AND o.status IN ('delivered', 'completed')
GROUP BY o.user_id, u.full_name, u.phone, u.avatar_url;

-- name: GetCustomerFavoriteDishes :many
-- 查询顾客最喜欢的菜品
SELECT 
    oi.dish_id,
    d.name AS dish_name,
    COUNT(*)::int AS order_count,
    SUM(oi.quantity)::int AS total_quantity
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
JOIN orders o ON o.id = oi.order_id
WHERE o.merchant_id = $1
  AND o.user_id = $2
  AND o.status IN ('delivered', 'completed')
GROUP BY oi.dish_id, d.name
ORDER BY order_count DESC, total_quantity DESC
LIMIT $3;

-- name: GetMerchantHourlyStats :many
-- 商户时段分析: 按小时统计订单分布
SELECT 
    EXTRACT(HOUR FROM created_at)::int AS hour,
    COUNT(*)::int AS order_count,
    COALESCE(SUM(final_amount), 0)::bigint AS total_sales,
    COALESCE(AVG(final_amount), 0)::bigint AS avg_order_amount
FROM orders
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND status IN ('delivered', 'completed')
GROUP BY EXTRACT(HOUR FROM created_at)
ORDER BY hour;

-- name: GetMerchantOrderSourceStats :many
-- 订单来源分析
SELECT 
    order_type,
    COUNT(*)::int AS order_count,
    COALESCE(SUM(final_amount), 0)::bigint AS total_sales,
    COALESCE(AVG(final_amount), 0)::bigint AS avg_order_amount
FROM orders
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND status IN ('delivered', 'completed')
GROUP BY order_type
ORDER BY order_count DESC;

-- name: GetMerchantRepurchaseRate :one
-- 复购率分析
-- 注意: repurchase_rate_percent 返回万分比(如 7550 表示 75.50%)，API层需除以100
-- 注意: avg_orders_per_user 返回百分比形式(如 235 表示 2.35次)，API层需除以100
WITH customer_order_counts AS (
    SELECT 
        user_id,
        COUNT(*) AS order_count
    FROM orders
    WHERE merchant_id = $1
      AND created_at >= $2
      AND created_at <= $3
      AND status IN ('delivered', 'completed')
    GROUP BY user_id
)
SELECT 
    COUNT(*)::int AS total_customers,
    COUNT(*) FILTER (WHERE order_count > 1)::int AS repeat_customers,
    COALESCE(SUM(order_count), 0)::int AS total_orders,
    CASE 
        WHEN COUNT(*) > 0 
        THEN (COUNT(*) FILTER (WHERE order_count > 1)::numeric / COUNT(*)::numeric * 10000)::int
        ELSE 0
    END AS repurchase_rate_basis_points,
    CASE 
        WHEN COUNT(*) > 0 
        THEN (COALESCE(SUM(order_count), 0)::numeric / COUNT(*)::numeric * 100)::int
        ELSE 0
    END AS avg_orders_per_user_cents
FROM customer_order_counts;

-- name: GetDishCategoryStats :many
-- 菜品分类销售统计
SELECT 
    dc.id AS category_id,
    dc.name AS category_name,
    COUNT(DISTINCT oi.dish_id)::int AS dish_count,
    SUM(oi.quantity)::int AS total_quantity,
    COALESCE(SUM(oi.subtotal), 0)::bigint AS total_revenue
FROM dish_categories dc
JOIN merchant_dish_categories mdc ON dc.id = mdc.category_id
LEFT JOIN dishes d ON d.category_id = dc.id AND d.merchant_id = mdc.merchant_id
LEFT JOIN order_items oi ON oi.dish_id = d.id
LEFT JOIN orders o ON o.id = oi.order_id 
    AND o.created_at >= @start_date 
    AND o.created_at <= @end_date
    AND o.status IN ('delivered', 'completed')
WHERE mdc.merchant_id = $1
GROUP BY dc.id, dc.name, mdc.sort_order
ORDER BY mdc.sort_order ASC, total_revenue DESC;

-- name: GetMerchantAvgPrepMinutes :one
-- 获取商户平均出餐时间（分钟）
-- 从 order_status_logs 计算 paid → ready 的时间差
-- 取最近30天完成订单的平均值
SELECT COALESCE(
  AVG(EXTRACT(EPOCH FROM (ready_log.created_at - paid_log.created_at)) / 60),
  0
)::INTEGER as avg_prep_minutes
FROM orders o
JOIN order_status_logs paid_log ON paid_log.order_id = o.id AND paid_log.to_status = 'paid'
JOIN order_status_logs ready_log ON ready_log.order_id = o.id AND ready_log.to_status = 'ready'
WHERE o.merchant_id = $1
  AND o.status IN ('ready', 'delivering', 'completed')
  AND o.created_at > NOW() - INTERVAL '30 days';

-- name: GetMerchantDishesWithCategory :many
-- 获取商户所有在线菜品（含分类信息）- 消费者端使用
SELECT 
  d.id,
  d.name,
  d.description,
  d.price,
  d.member_price,
  d.image_url,
  d.is_available,
  d.sort_order,
  d.prepare_time,
  COALESCE(dc.id, 0) as category_id,
  COALESCE(dc.name, '未分类') as category_name,
  COALESCE(mdc.sort_order, 999) as category_sort_order,
  COALESCE(
    (SELECT json_agg(t.name) FROM dish_tags dt JOIN tags t ON t.id = dt.tag_id WHERE dt.dish_id = d.id),
    '[]'::json
  ) as tags,
  COALESCE(
    (SELECT SUM(oi.quantity)::int FROM order_items oi JOIN orders o ON o.id = oi.order_id 
     WHERE oi.dish_id = d.id AND o.status IN ('completed', 'delivered') 
     AND o.created_at > NOW() - INTERVAL '30 days'),
    0
  ) as monthly_sales
FROM dishes d
LEFT JOIN dish_categories dc ON dc.id = d.category_id
LEFT JOIN merchant_dish_categories mdc ON mdc.category_id = dc.id AND mdc.merchant_id = d.merchant_id
WHERE d.merchant_id = $1
  AND d.is_online = true
  AND d.is_available = true
  AND d.deleted_at IS NULL
ORDER BY COALESCE(mdc.sort_order, 999), d.sort_order, d.id;

-- name: GetMerchantOnlineCombos :many
-- 获取商户所有在线套餐 - 消费者端使用
SELECT 
  cs.id,
  cs.name,
  cs.description,
  cs.image_url,
  cs.combo_price,
  -- 实时计算实际原价（单品价之和）
  COALESCE(
    (SELECT SUM(d.price * cd.quantity) FROM combo_dishes cd JOIN dishes d ON d.id = cd.dish_id WHERE cd.combo_id = cs.id),
    cs.original_price
  )::bigint as original_price,
  cs.is_online,
  (
    SELECT json_agg(json_build_object(
      'dish_id', cd.dish_id,
      'dish_name', d.name,
      'quantity', cd.quantity
    ))
    FROM combo_dishes cd
    JOIN dishes d ON d.id = cd.dish_id
    WHERE cd.combo_id = cs.id
  ) as dishes
FROM combo_sets cs
WHERE cs.merchant_id = $1
  AND cs.is_online = true
  AND cs.deleted_at IS NULL
ORDER BY cs.id;

-- name: ListMerchantActiveDiscountRules :many
-- 获取商户当前有效的满减规则
SELECT * FROM discount_rules
WHERE merchant_id = $1
  AND is_active = true
  AND valid_from <= NOW()
  AND valid_until >= NOW()
  AND deleted_at IS NULL
ORDER BY min_order_amount ASC;

-- name: ListMerchantActiveVouchers :many
-- 获取商户当前有效的代金券
SELECT * FROM vouchers
WHERE merchant_id = $1
  AND is_active = true
  AND valid_from <= NOW()
  AND valid_until >= NOW()
  AND claimed_quantity < total_quantity
  AND deleted_at IS NULL
ORDER BY amount DESC;

-- name: ListMerchantActiveDeliveryPromotions :many
-- 获取商户当前有效的配送费优惠
SELECT * FROM merchant_delivery_promotions
WHERE merchant_id = $1
  AND is_active = true
  AND valid_from <= NOW()
  AND valid_until >= NOW()
ORDER BY min_order_amount ASC;


