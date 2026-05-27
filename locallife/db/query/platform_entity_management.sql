-- name: ListPlatformRiderCards :many
SELECT
    r.id,
    r.real_name,
    r.region_id,
    regions.name AS region_name,
    r.status,
    EXISTS (
        SELECT 1
        FROM deliveries d
        WHERE d.rider_id = r.id
          AND d.status <> 'pending'
          AND COALESCE(d.assigned_at, d.created_at) >= now() - interval '3 days'
    ) AS accepted_in_3d,
    (
        SELECT COUNT(DISTINCT c.id)::bigint
        FROM deliveries d
        JOIN claims c ON c.order_id = d.order_id
        WHERE d.rider_id = r.id
    ) AS complaint_count
FROM riders r
LEFT JOIN regions ON regions.id = r.region_id
ORDER BY r.created_at DESC, r.id DESC
LIMIT $1 OFFSET $2;

-- name: CountPlatformRiders :one
SELECT COUNT(*) FROM riders;

-- name: GetPlatformRiderDetail :one
SELECT
    r.id,
    r.real_name,
    r.region_id,
    regions.name AS region_name,
    r.status,
    EXISTS (
        SELECT 1
        FROM deliveries d
        WHERE d.rider_id = r.id
          AND d.status <> 'pending'
          AND COALESCE(d.assigned_at, d.created_at) >= now() - interval '3 days'
    ) AS accepted_in_3d,
    r.total_orders,
    r.total_earnings,
    (
        SELECT COUNT(*)::int
        FROM deliveries d
        WHERE d.rider_id = r.id
          AND d.status IN ('delivered', 'completed')
          AND COALESCE(d.completed_at, d.delivered_at, d.rider_delivered_at, d.created_at) >= now() - interval '1 month'
    ) AS month_orders,
    (
        SELECT COALESCE(SUM(d.rider_earnings), 0)::bigint
        FROM deliveries d
        WHERE d.rider_id = r.id
          AND d.status IN ('delivered', 'completed')
          AND COALESCE(d.completed_at, d.delivered_at, d.rider_delivered_at, d.created_at) >= now() - interval '1 month'
    ) AS month_income,
    (
        SELECT COUNT(DISTINCT c.id)::bigint
        FROM deliveries d
        JOIN claims c ON c.order_id = d.order_id
        WHERE d.rider_id = r.id
    ) AS complaint_count,
    ra.id_card_ocr,
    r.created_at,
    r.location_updated_at
FROM riders r
LEFT JOIN regions ON regions.id = r.region_id
LEFT JOIN rider_applications ra ON ra.id = r.application_id
WHERE r.id = $1
LIMIT 1;

-- name: ListPlatformRiderComplaintCategories :many
SELECT
    c.claim_type,
    COUNT(DISTINCT c.id)::bigint AS count
FROM deliveries d
JOIN claims c ON c.order_id = d.order_id
WHERE d.rider_id = sqlc.arg('rider_id')::bigint
GROUP BY c.claim_type
ORDER BY count DESC, c.claim_type ASC;

-- name: ListPlatformOperatorCards :many
WITH operator_region_scope AS (
    SELECT operator_id, region_id, status
    FROM operator_regions
    UNION
    SELECT id AS operator_id, region_id, 'active'::text AS status
    FROM operators
    WHERE region_id IS NOT NULL
),
region_counts AS (
    SELECT operator_id, COUNT(DISTINCT region_id)::bigint AS region_count
    FROM operator_region_scope
    WHERE status = 'active'
    GROUP BY operator_id
),
merchant_counts AS (
    SELECT ors.operator_id, COUNT(DISTINCT m.id)::bigint AS merchant_count
    FROM operator_region_scope ors
    JOIN merchants m ON m.region_id = ors.region_id
    WHERE ors.status = 'active'
      AND m.deleted_at IS NULL
    GROUP BY ors.operator_id
),
complaint_counts AS (
    SELECT ors.operator_id, COUNT(DISTINCT c.id)::bigint AS complaint_count
    FROM operator_region_scope ors
    JOIN merchants m ON m.region_id = ors.region_id
    JOIN orders ord ON ord.merchant_id = m.id
    JOIN claims c ON c.order_id = ord.id
    WHERE ors.status = 'active'
      AND m.deleted_at IS NULL
    GROUP BY ors.operator_id
)
SELECT
    op.id,
    op.name,
    op.status,
    COALESCE(region_counts.region_count, 0)::bigint AS region_count,
    COALESCE(merchant_counts.merchant_count, 0)::bigint AS merchant_count,
    COALESCE(complaint_counts.complaint_count, 0)::bigint AS complaint_count
FROM operators op
LEFT JOIN region_counts ON region_counts.operator_id = op.id
LEFT JOIN merchant_counts ON merchant_counts.operator_id = op.id
LEFT JOIN complaint_counts ON complaint_counts.operator_id = op.id
ORDER BY op.created_at DESC, op.id DESC
LIMIT $1 OFFSET $2;

-- name: CountPlatformOperators :one
SELECT COUNT(*) FROM operators;

-- name: GetPlatformOperatorDetail :one
WITH operator_region_scope AS (
    SELECT operator_id, region_id, status
    FROM operator_regions
    WHERE operator_id = $1
    UNION
    SELECT id AS operator_id, region_id, 'active'::text AS status
    FROM operators
    WHERE id = $1 AND region_id IS NOT NULL
)
SELECT
    op.id,
    op.name,
    op.contact_name,
    op.contact_phone,
    op.status,
    op.region_id,
    regions.name AS region_name,
    (
        SELECT COUNT(DISTINCT region_id)::bigint
        FROM operator_region_scope
        WHERE status = 'active'
    ) AS region_count,
    (
        SELECT COUNT(DISTINCT m.id)::bigint
        FROM operator_region_scope ors
        JOIN merchants m ON m.region_id = ors.region_id
        WHERE ors.status = 'active'
          AND m.deleted_at IS NULL
    ) AS merchant_count,
    (
        SELECT COUNT(DISTINCT ord.id)::int
        FROM operator_region_scope ors
        JOIN merchants m ON m.region_id = ors.region_id
        JOIN orders ord ON ord.merchant_id = m.id
        WHERE ors.status = 'active'
          AND m.deleted_at IS NULL
          AND ord.status = 'completed'
          AND COALESCE(ord.completed_at, ord.created_at) >= now() - interval '1 month'
    ) AS month_orders,
    (
        SELECT COALESCE(SUM(ord.final_amount), 0)::bigint
        FROM operator_region_scope ors
        JOIN merchants m ON m.region_id = ors.region_id
        JOIN orders ord ON ord.merchant_id = m.id
        WHERE ors.status = 'active'
          AND m.deleted_at IS NULL
          AND ord.status = 'completed'
          AND COALESCE(ord.completed_at, ord.created_at) >= now() - interval '1 month'
    ) AS month_revenue,
    (
        SELECT COUNT(DISTINCT c.id)::bigint
        FROM operator_region_scope ors
        JOIN merchants m ON m.region_id = ors.region_id
        JOIN orders ord ON ord.merchant_id = m.id
        JOIN claims c ON c.order_id = ord.id
        WHERE ors.status = 'active'
          AND m.deleted_at IS NULL
    ) AS complaint_count,
    op.created_at
FROM operators op
LEFT JOIN regions ON regions.id = op.region_id
WHERE op.id = $1
LIMIT 1;

-- name: ListPlatformOperatorRegions :many
WITH operator_region_scope AS (
    SELECT operator_id, region_id, status
    FROM operator_regions
    WHERE operator_id = $1
    UNION
    SELECT id AS operator_id, region_id, 'active'::text AS status
    FROM operators
    WHERE id = $1 AND region_id IS NOT NULL
)
SELECT
    ors.region_id,
    regions.name AS region_name,
    ors.status
FROM operator_region_scope ors
JOIN regions ON regions.id = ors.region_id
ORDER BY regions.name ASC, ors.region_id ASC;

-- name: ListPlatformOperatorComplaintCategories :many
WITH operator_region_scope AS (
    SELECT operator_id, region_id, status
    FROM operator_regions
    WHERE operator_id = $1
    UNION
    SELECT id AS operator_id, region_id, 'active'::text AS status
    FROM operators
    WHERE id = $1 AND region_id IS NOT NULL
)
SELECT
    c.claim_type,
    COUNT(DISTINCT c.id)::bigint AS count
FROM operator_region_scope ors
JOIN merchants m ON m.region_id = ors.region_id
JOIN orders ord ON ord.merchant_id = m.id
JOIN claims c ON c.order_id = ord.id
WHERE ors.status = 'active'
  AND m.deleted_at IS NULL
GROUP BY c.claim_type
ORDER BY count DESC, c.claim_type ASC;

-- name: ListPlatformMerchantCards :many
SELECT
    m.id,
    m.name,
    m.region_id,
    regions.name AS region_name,
    m.status,
    m.is_open,
    (
        SELECT COUNT(*)::int
        FROM orders ord
        WHERE ord.merchant_id = m.id
          AND ord.status = 'completed'
          AND COALESCE(ord.completed_at, ord.created_at) >= now() - interval '1 month'
    ) AS month_orders,
    (
        SELECT COUNT(DISTINCT c.id)::bigint
        FROM orders ord
        JOIN claims c ON c.order_id = ord.id
        WHERE ord.merchant_id = m.id
    ) AS complaint_count
FROM merchants m
LEFT JOIN regions ON regions.id = m.region_id
WHERE m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT $1 OFFSET $2;

-- name: CountPlatformMerchants :one
SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL;

-- name: GetPlatformMerchantDetail :one
SELECT
    m.id,
    m.name,
    m.phone,
    m.address,
    m.region_id,
    regions.name AS region_name,
    m.status,
    m.is_open,
    (
        SELECT COUNT(*)::int
        FROM orders ord
        WHERE ord.merchant_id = m.id
          AND ord.status = 'completed'
    ) AS total_orders,
    (
        SELECT COALESCE(SUM(ord.final_amount), 0)::bigint
        FROM orders ord
        WHERE ord.merchant_id = m.id
          AND ord.status = 'completed'
    ) AS total_income,
    (
        SELECT COUNT(*)::int
        FROM orders ord
        WHERE ord.merchant_id = m.id
          AND ord.status = 'completed'
          AND COALESCE(ord.completed_at, ord.created_at) >= now() - interval '1 month'
    ) AS month_orders,
    (
        SELECT COALESCE(SUM(ord.final_amount), 0)::bigint
        FROM orders ord
        WHERE ord.merchant_id = m.id
          AND ord.status = 'completed'
          AND COALESCE(ord.completed_at, ord.created_at) >= now() - interval '1 month'
    ) AS month_income,
    (
        SELECT COUNT(DISTINCT c.id)::bigint
        FROM orders ord
        JOIN claims c ON c.order_id = ord.id
        WHERE ord.merchant_id = m.id
    ) AS complaint_count,
    m.created_at
FROM merchants m
LEFT JOIN regions ON regions.id = m.region_id
WHERE m.id = $1
  AND m.deleted_at IS NULL
LIMIT 1;

-- name: ListPlatformMerchantComplaintCategories :many
SELECT
    c.claim_type,
    COUNT(DISTINCT c.id)::bigint AS count
FROM orders ord
JOIN claims c ON c.order_id = ord.id
WHERE ord.merchant_id = $1
GROUP BY c.claim_type
ORDER BY count DESC, c.claim_type ASC;
