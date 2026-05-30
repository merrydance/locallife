-- Phase3: abnormal stats aggregation queries

-- name: GetAbnormalStatsSummary :one
SELECT
    COALESCE(SUM(total_orders), 0)::INT AS total_orders,
    COALESCE(SUM(abnormal_claims), 0)::INT AS abnormal_claims
FROM abnormal_stats_daily
WHERE entity_type = $1
  AND entity_id = $2
  AND stat_date >= $3
  AND stat_date <= $4;

-- name: ListAbnormalStatsDaily :many
SELECT id, stat_date, entity_type, entity_id, total_orders, abnormal_claims, created_at, updated_at FROM abnormal_stats_daily
WHERE entity_type = $1
  AND entity_id = $2
  AND stat_date >= $3
  AND stat_date <= $4
ORDER BY stat_date ASC;

-- name: ListAbnormalStatsAlerts :many
SELECT
    entity_id,
    SUM(total_orders)::INT AS total_orders,
    SUM(abnormal_claims)::INT AS abnormal_claims,
    CASE WHEN SUM(total_orders) = 0 THEN 0 ELSE SUM(abnormal_claims)::FLOAT / SUM(total_orders) END AS abnormal_rate
FROM abnormal_stats_daily
WHERE entity_type = sqlc.arg('entity_type')
  AND stat_date >= sqlc.arg('start_date')
  AND stat_date <= sqlc.arg('end_date')
GROUP BY entity_id
HAVING SUM(total_orders) > 0
  AND SUM(abnormal_claims) >= sqlc.arg('min_claims')
  AND (SUM(abnormal_claims)::FLOAT / SUM(total_orders)) >= sqlc.arg('min_rate')::double precision
ORDER BY abnormal_rate DESC
LIMIT sqlc.arg('limit');

-- name: ClearAbnormalStatsDailyForBackfill :exec
DELETE FROM abnormal_stats_daily
WHERE stat_date >= sqlc.arg('start_at')::date
  AND stat_date < sqlc.arg('end_at')::date;

-- name: InsertBackfillAbnormalStatsDaily :exec
INSERT INTO abnormal_stats_daily (
    stat_date,
    entity_type,
    entity_id,
    total_orders,
    abnormal_claims
)
SELECT
    stat_date,
    entity_type,
    entity_id,
    SUM(total_orders)::INT AS total_orders,
    SUM(abnormal_claims)::INT AS abnormal_claims
FROM (
    -- user total orders
    SELECT DATE(COALESCE(o.completed_at, o.created_at)) AS stat_date,
           'user' AS entity_type,
           o.user_id AS entity_id,
           COUNT(*) AS total_orders,
           0 AS abnormal_claims
    FROM orders o
    WHERE o.status = 'completed'
      AND COALESCE(o.completed_at, o.created_at) >= sqlc.arg('start_at')
      AND COALESCE(o.completed_at, o.created_at) < sqlc.arg('end_at')
    GROUP BY 1,2,3

    UNION ALL

    -- merchant total orders
    SELECT DATE(COALESCE(o.completed_at, o.created_at)) AS stat_date,
           'merchant' AS entity_type,
           o.merchant_id AS entity_id,
           COUNT(*) AS total_orders,
           0 AS abnormal_claims
    FROM orders o
    WHERE o.status = 'completed'
      AND COALESCE(o.completed_at, o.created_at) >= sqlc.arg('start_at')
      AND COALESCE(o.completed_at, o.created_at) < sqlc.arg('end_at')
    GROUP BY 1,2,3

    UNION ALL

    -- rider total orders
    SELECT DATE(COALESCE(d.completed_at, d.delivered_at, d.created_at)) AS stat_date,
           'rider' AS entity_type,
           d.rider_id AS entity_id,
           COUNT(*) AS total_orders,
           0 AS abnormal_claims
    FROM deliveries d
    WHERE d.status = 'completed'
      AND d.rider_id IS NOT NULL
      AND COALESCE(d.completed_at, d.delivered_at, d.created_at) >= sqlc.arg('start_at')
      AND COALESCE(d.completed_at, d.delivered_at, d.created_at) < sqlc.arg('end_at')
    GROUP BY 1,2,3

    UNION ALL

    -- user abnormal claims
    SELECT DATE(c.created_at) AS stat_date,
           'user' AS entity_type,
           c.user_id AS entity_id,
           0 AS total_orders,
           COUNT(*) AS abnormal_claims
    FROM claims c
    WHERE c.status IN ('auto-approved', 'approved')
      AND c.created_at >= sqlc.arg('start_at')
      AND c.created_at < sqlc.arg('end_at')
    GROUP BY 1,2,3

    UNION ALL

    -- merchant abnormal claims
    SELECT DATE(c.created_at) AS stat_date,
           'merchant' AS entity_type,
           o.merchant_id AS entity_id,
           0 AS total_orders,
           COUNT(*) AS abnormal_claims
    FROM claims c
    JOIN orders o ON o.id = c.order_id
    WHERE c.status IN ('auto-approved', 'approved')
      AND c.created_at >= sqlc.arg('start_at')
      AND c.created_at < sqlc.arg('end_at')
    GROUP BY 1,2,3

    UNION ALL

    -- rider abnormal claims
    SELECT DATE(c.created_at) AS stat_date,
           'rider' AS entity_type,
           d.rider_id AS entity_id,
           0 AS total_orders,
           COUNT(*) AS abnormal_claims
    FROM claims c
    JOIN orders o ON o.id = c.order_id
    JOIN deliveries d ON d.order_id = o.id
    WHERE c.status IN ('auto-approved', 'approved')
      AND d.rider_id IS NOT NULL
      AND c.created_at >= sqlc.arg('start_at')
      AND c.created_at < sqlc.arg('end_at')
    GROUP BY 1,2,3
) AS summary
GROUP BY stat_date, entity_type, entity_id
ON CONFLICT (stat_date, entity_type, entity_id)
DO UPDATE SET
    total_orders = EXCLUDED.total_orders,
    abnormal_claims = EXCLUDED.abnormal_claims,
    updated_at = NOW();
