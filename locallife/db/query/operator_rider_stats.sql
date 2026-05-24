-- M12: 运营商骑手统计查询（按指定时间段）

-- name: GetOperatorRiderStats :one
-- 运营商视角：单个骑手在指定时间段内的代取统计
SELECT
    COUNT(d.id)::int                                                              AS total_deliveries,
    COUNT(CASE WHEN d.status = 'completed' THEN 1 END)::int                       AS completed_deliveries,
    COALESCE(
        CASE WHEN COUNT(d.id) > 0
             THEN COUNT(CASE WHEN d.status = 'completed' THEN 1 END) * 10000 / COUNT(d.id)
             ELSE 0 END,
        0
    )::int                                                                        AS completion_rate_basis_points,
    COALESCE(
        AVG(
            CASE WHEN d.status = 'completed' AND d.picked_at IS NOT NULL AND d.delivered_at IS NOT NULL
                 THEN EXTRACT(EPOCH FROM (d.delivered_at - d.picked_at))
            END
        ),
        0
    )::int                                                                        AS avg_delivery_seconds,
    COALESCE(SUM(d.rider_earnings), 0)::bigint                                    AS period_earnings,
    COUNT(CASE WHEN d.is_delayed THEN 1 END)::int                                 AS delayed_count
FROM deliveries d
WHERE d.rider_id = sqlc.arg('rider_id')
  AND d.created_at >= sqlc.arg('start_at')
  AND d.created_at <= sqlc.arg('end_at');
