-- name: FindActiveTakeoutMerchantByNormalizedName :one
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, m.storefront_images, m.environment_images, m.manual_open_status_until
FROM merchants m
LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
WHERE m.region_id = $1
  AND m.status = 'active'
  AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND lower(regexp_replace(m.name, '[[:space:]]+', '', 'g')) = sqlc.arg('normalized_name')::text
ORDER BY m.id ASC
LIMIT 1;

-- name: FindActiveWantedMerchantByNormalizedName :one
SELECT id, region_id, normalized_name, display_name, address, latitude, longitude, source, status, want_count, created_by_user_id, matched_merchant_id, last_voted_at, matched_at, created_at, updated_at
FROM wanted_merchants
WHERE region_id = $1
  AND normalized_name = $2
  AND status = 'active'
LIMIT 1;

-- name: GetActiveWantedMerchantByID :one
SELECT id, region_id, normalized_name, display_name, address, latitude, longitude, source, status, want_count, created_by_user_id, matched_merchant_id, last_voted_at, matched_at, created_at, updated_at
FROM wanted_merchants
WHERE id = $1
  AND region_id = $2
  AND status = 'active'
LIMIT 1;

-- name: GetActiveWantedMerchantByIDForUpdate :one
SELECT id, region_id, normalized_name, display_name, address, latitude, longitude, source, status, want_count, created_by_user_id, matched_merchant_id, last_voted_at, matched_at, created_at, updated_at
FROM wanted_merchants
WHERE id = $1
  AND region_id = $2
  AND status = 'active'
LIMIT 1
FOR UPDATE;

-- name: CreateOrGetActiveWantedMerchant :one
INSERT INTO wanted_merchants (
  region_id,
  normalized_name,
  display_name,
  address,
  latitude,
  longitude,
  source,
  created_by_user_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (region_id, normalized_name) WHERE status = 'active'
DO UPDATE SET updated_at = wanted_merchants.updated_at
RETURNING id, region_id, normalized_name, display_name, address, latitude, longitude, source, status, want_count, created_by_user_id, matched_merchant_id, last_voted_at, matched_at, created_at, updated_at;

-- name: CreateWantedMerchantVote :one
INSERT INTO wanted_merchant_votes (
  wanted_merchant_id,
  region_id,
  user_id
) VALUES (
  $1, $2, $3
)
ON CONFLICT (region_id, user_id, wanted_merchant_id) DO NOTHING
RETURNING id, wanted_merchant_id, region_id, user_id, created_at;

-- name: IncrementWantedMerchantWantCount :one
UPDATE wanted_merchants
SET
  want_count = want_count + 1,
  last_voted_at = now(),
  updated_at = now()
WHERE id = $1
  AND region_id = $2
  AND status = 'active'
RETURNING id, region_id, normalized_name, display_name, address, latitude, longitude, source, status, want_count, created_by_user_id, matched_merchant_id, last_voted_at, matched_at, created_at, updated_at;

-- name: GetWantedMerchantRank :one
SELECT (COUNT(*) + 1)::bigint AS rank
FROM wanted_merchants w
JOIN wanted_merchants target
  ON target.id = $2
  AND target.region_id = $1
  AND target.status = 'active'
WHERE w.region_id = target.region_id
  AND w.status = 'active'
  AND (
    w.want_count > target.want_count
    OR (
      w.want_count = target.want_count
      AND COALESCE(w.last_voted_at, 'epoch'::timestamptz) > COALESCE(target.last_voted_at, 'epoch'::timestamptz)
    )
    OR (
      w.want_count = target.want_count
      AND COALESCE(w.last_voted_at, 'epoch'::timestamptz) = COALESCE(target.last_voted_at, 'epoch'::timestamptz)
      AND w.id < target.id
    )
  );

-- name: ListWantedMerchantLeaderboard :many
SELECT
  w.id,
  w.region_id,
  w.normalized_name,
  w.display_name,
  w.address,
  w.latitude,
  w.longitude,
  w.source,
  w.status,
  w.want_count,
  w.last_voted_at,
  w.created_at,
  w.updated_at,
  (ROW_NUMBER() OVER (
    ORDER BY w.want_count DESC, w.last_voted_at DESC NULLS LAST, w.id ASC
  ))::bigint AS rank,
  EXISTS (
    SELECT 1
    FROM wanted_merchant_votes v
    WHERE v.region_id = w.region_id
      AND v.wanted_merchant_id = w.id
      AND v.user_id = $2
  ) AS has_voted
FROM wanted_merchants w
WHERE w.region_id = $1
  AND w.status = 'active'
ORDER BY w.want_count DESC, w.last_voted_at DESC NULLS LAST, w.id ASC
LIMIT $3 OFFSET $4;

-- name: CountActiveWantedMerchantsByRegion :one
SELECT COUNT(*)
FROM wanted_merchants
WHERE region_id = $1
  AND status = 'active';

-- name: MarkActiveWantedMerchantMatchedByMerchant :exec
UPDATE wanted_merchants
SET
  status = 'matched',
  matched_merchant_id = $3,
  matched_at = now(),
  updated_at = now()
WHERE region_id = $1
  AND normalized_name = $2
  AND status = 'active';
