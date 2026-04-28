-- name: GetOperatorPendingDispatchSummary :one
SELECT
    r.id AS region_id,
    r.name AS region_name,
    COUNT(d.id)::bigint AS pending_total,
    COUNT(d.id) FILTER (WHERE d.created_at < sqlc.arg('timeout_before'))::bigint AS timeout_over_threshold_total,
  COALESCE(MAX((EXTRACT(EPOCH FROM (now() - d.created_at)))::bigint), 0::bigint) AS oldest_wait_seconds,
  now()::timestamptz AS latest_refresh_at
FROM regions r
LEFT JOIN merchants m ON m.region_id = r.id
LEFT JOIN orders o ON o.merchant_id = m.id
LEFT JOIN deliveries d ON d.order_id = o.id AND d.status = sqlc.arg('status')
WHERE r.id = sqlc.arg('region_id')
GROUP BY r.id, r.name;

-- name: ListOperatorPendingDispatches :many
SELECT
    d.id AS delivery_id,
    d.order_id,
    o.order_no,
    m.id AS merchant_id,
    m.name AS merchant_name,
    r.id AS region_id,
    r.name AS region_name,
    EXTRACT(EPOCH FROM (now() - d.created_at))::bigint AS wait_seconds,
    d.delivery_fee,
    d.estimated_pickup_at AS expected_pickup_at,
    (d.created_at < sqlc.arg('timeout_before')) AS is_timeout_over_threshold
FROM deliveries d
JOIN orders o ON o.id = d.order_id
JOIN merchants m ON m.id = o.merchant_id
JOIN regions r ON r.id = m.region_id
WHERE d.status = sqlc.arg('status')
  AND r.id = sqlc.arg('region_id')
ORDER BY is_timeout_over_threshold DESC, d.created_at ASC, d.id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOperatorPendingDispatches :one
SELECT COUNT(*)::bigint
FROM deliveries d
JOIN orders o ON o.id = d.order_id
JOIN merchants m ON m.id = o.merchant_id
WHERE d.status = sqlc.arg('status')
  AND m.region_id = sqlc.arg('region_id');