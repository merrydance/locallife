WITH active_legacy_packaging_merchants AS (
    SELECT DISTINCT d.merchant_id
    FROM dishes d
    WHERE d.is_packaging = true
      AND d.is_online = true
      AND d.is_available = true
      AND d.deleted_at IS NULL
)
INSERT INTO merchant_packaging_settings (
    merchant_id,
    enabled,
    required,
    applicable_order_types,
    updated_at
)
SELECT
    merchant_id,
    true,
    true,
    ARRAY['takeout','takeaway']::TEXT[],
    now()
FROM active_legacy_packaging_merchants
ON CONFLICT (merchant_id) DO NOTHING;

WITH legacy_packaging_dishes AS (
    SELECT
        d.id,
        d.merchant_id,
        LEFT(COALESCE(NULLIF(TRIM(d.name), ''), '包装-' || d.id::TEXT), 50) AS base_name,
        d.description,
        d.price,
        (d.is_online = true AND d.is_available = true AND d.deleted_at IS NULL) AS is_enabled,
        d.sort_order
    FROM dishes d
    WHERE d.is_packaging = true
),
named_legacy_packaging_dishes AS (
    SELECT
        l.*,
        COUNT(*) OVER (
            PARTITION BY l.merchant_id, LOWER(l.base_name)
        ) AS duplicate_name_count,
        ROW_NUMBER() OVER (
            PARTITION BY l.merchant_id, LOWER(l.base_name)
            ORDER BY l.id
        ) AS duplicate_name_rank
    FROM legacy_packaging_dishes l
),
legacy_packaging_options AS (
    SELECT
        d.id,
        d.merchant_id,
        candidate.name,
        d.description,
        d.price,
        d.is_enabled,
        d.sort_order
    FROM named_legacy_packaging_dishes
    d
    CROSS JOIN LATERAL (
        SELECT candidate_name AS name
        FROM (
            SELECT
                candidate_rank,
                CASE
                    WHEN candidate_rank = 0 THEN d.base_name
                    WHEN candidate_rank = 1 THEN
                        LEFT(
                            d.base_name,
                            GREATEST(1, 50 - CHAR_LENGTH('-legacy-' || d.id::TEXT))
                        ) || '-legacy-' || d.id::TEXT
                    ELSE
                        LEFT(
                            d.base_name,
                            GREATEST(1, 50 - CHAR_LENGTH('-legacy-' || d.id::TEXT || '-' || candidate_rank::TEXT))
                        ) || '-legacy-' || d.id::TEXT || '-' || candidate_rank::TEXT
                END AS candidate_name
            FROM generate_series(0, 10000) AS candidate_rank
        ) candidates
        WHERE (
                candidate_rank > 0
                OR d.duplicate_name_rank = 1
            )
          AND NOT EXISTS (
              SELECT 1
              FROM merchant_packaging_options existing_option
              WHERE existing_option.merchant_id = d.merchant_id
                AND existing_option.deleted_at IS NULL
                AND LOWER(existing_option.name) = LOWER(candidates.candidate_name)
                AND existing_option.legacy_dish_id IS DISTINCT FROM d.id
          )
        ORDER BY candidate_rank
        LIMIT 1
    ) candidate
)
INSERT INTO merchant_packaging_options (
    merchant_id,
    legacy_dish_id,
    name,
    description,
    price,
    is_enabled,
    sort_order,
    updated_at
)
SELECT
    merchant_id,
    id,
    name,
    description,
    price,
    is_enabled,
    sort_order,
    now()
FROM legacy_packaging_options
ON CONFLICT (legacy_dish_id) WHERE legacy_dish_id IS NOT NULL DO UPDATE
SET
    merchant_id = EXCLUDED.merchant_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price = EXCLUDED.price,
    is_enabled = EXCLUDED.is_enabled,
    sort_order = EXCLUDED.sort_order,
    deleted_at = NULL,
    updated_at = CASE
        WHEN merchant_packaging_options.merchant_id IS DISTINCT FROM EXCLUDED.merchant_id
          OR merchant_packaging_options.name IS DISTINCT FROM EXCLUDED.name
          OR merchant_packaging_options.description IS DISTINCT FROM EXCLUDED.description
          OR merchant_packaging_options.price IS DISTINCT FROM EXCLUDED.price
          OR merchant_packaging_options.is_enabled IS DISTINCT FROM EXCLUDED.is_enabled
          OR merchant_packaging_options.sort_order IS DISTINCT FROM EXCLUDED.sort_order
          OR merchant_packaging_options.deleted_at IS NOT NULL
            THEN now()
        ELSE merchant_packaging_options.updated_at
    END;

DO $$
DECLARE
    missing_count BIGINT;
BEGIN
    SELECT COUNT(*)
    INTO missing_count
    FROM dishes d
    WHERE d.is_packaging = true
      AND NOT EXISTS (
          SELECT 1
          FROM merchant_packaging_options mpo
          WHERE mpo.legacy_dish_id = d.id
            AND mpo.merchant_id = d.merchant_id
            AND mpo.deleted_at IS NULL
      );

    IF missing_count > 0 THEN
        RAISE EXCEPTION 'legacy packaging dish migration missed % dishes', missing_count;
    END IF;
END $$;
