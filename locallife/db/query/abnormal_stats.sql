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
SELECT * FROM abnormal_stats_daily
WHERE entity_type = $1
  AND entity_id = $2
  AND stat_date >= $3
  AND stat_date <= $4
ORDER BY stat_date ASC;
