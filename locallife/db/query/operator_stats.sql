-- M12: 运营商统计查询
-- 
-- 说明: 运营商结算通过微信电商分账系统实时处理，每笔订单支付时自动分账
-- 运营商佣金数据从 profit_sharing_orders 表 (status='finished') 实时统计
-- operator_settlements 表已废弃删除，因为分账是实时的，不需要手动创建月度结算

-- name: GetRegionStats :one
-- 区域统计：基于实际完成分账的数据（status='finished'）
-- 这样确保统计的佣金是真实分账成功的，而不是预估值
SELECT 
    r.id AS region_id,
    r.name AS region_name,
    COUNT(DISTINCT m.id)::int AS merchant_count,
    COUNT(DISTINCT ps.id)::int AS total_orders,
    COALESCE(SUM(ps.total_amount), 0)::bigint AS total_gmv,
    COALESCE(SUM(ps.platform_commission), 0)::bigint AS total_commission
FROM regions r
LEFT JOIN merchants m ON m.region_id = r.id AND m.status = 'active'
LEFT JOIN profit_sharing_orders ps ON ps.merchant_id = m.id 
    AND ps.created_at >= $2
    AND ps.created_at <= $3
    AND ps.status = 'finished'  -- 只统计分账成功的订单
WHERE r.id = $1
GROUP BY r.id, r.name;

-- name: GetOperatorMerchantRanking :many
-- 运营商区域内商户排行（基于实际分账数据）
SELECT 
    m.id AS merchant_id,
    m.name AS merchant_name,
    COUNT(ps.id)::int AS order_count,
    COALESCE(SUM(ps.total_amount), 0)::bigint AS total_sales,
    COALESCE(SUM(ps.platform_commission), 0)::bigint AS commission,
    CASE 
        WHEN COUNT(ps.id) > 0 
        THEN (COALESCE(SUM(ps.total_amount), 0) / COUNT(ps.id))::bigint
        ELSE 0
    END AS avg_order_amount
FROM merchants m
LEFT JOIN profit_sharing_orders ps ON ps.merchant_id = m.id 
    AND ps.created_at >= $2
    AND ps.created_at <= $3
    AND ps.status = 'finished'  -- 只统计分账成功的订单
WHERE m.region_id = $1
  AND m.status = 'active'
GROUP BY m.id, m.name
ORDER BY total_sales DESC
LIMIT $4 OFFSET $5;

-- name: GetOperatorRiderRanking :many
-- 运营商区域内骑手绩效排行(通过配送订单关联区域)
SELECT 
    r.id AS rider_id,
    u.full_name AS rider_name,
    COUNT(d.id)::int AS delivery_count,
    COUNT(CASE WHEN d.status = 'completed' THEN 1 END)::int AS completed_count,
    COALESCE(AVG(EXTRACT(EPOCH FROM (d.delivered_at - d.picked_at))), 0)::int AS avg_delivery_time,
    r.total_earnings
FROM riders r
JOIN users u ON u.id = r.user_id
LEFT JOIN deliveries d ON d.rider_id = r.id 
    AND d.created_at >= $2
    AND d.created_at <= $3
LEFT JOIN orders o ON o.id = d.order_id
LEFT JOIN merchants m ON m.id = o.merchant_id
WHERE m.region_id = $1
  AND r.status = 'active'
GROUP BY r.id, u.full_name, r.total_earnings
HAVING COUNT(d.id) > 0
ORDER BY completed_count DESC
LIMIT $4 OFFSET $5;

-- name: GetRegionDailyTrend :many
-- 区域日趋势（基于实际分账数据）
SELECT 
    DATE(ps.created_at) AS date,
    COUNT(ps.id)::int AS order_count,
    COALESCE(SUM(ps.total_amount), 0)::bigint AS total_gmv,
    COALESCE(SUM(ps.platform_commission), 0)::bigint AS commission,
    COUNT(DISTINCT po.user_id)::int AS active_users,
    COUNT(DISTINCT ps.merchant_id)::int AS active_merchants
FROM profit_sharing_orders ps
JOIN merchants m ON m.id = ps.merchant_id
JOIN payment_orders po ON po.id = ps.payment_order_id
WHERE m.region_id = $1
  AND ps.created_at >= $2
  AND ps.created_at <= $3
  AND ps.status = 'finished'  -- 只统计分账成功的订单
GROUP BY DATE(ps.created_at)
ORDER BY date;
