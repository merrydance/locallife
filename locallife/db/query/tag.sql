-- name: CreateTag :one
INSERT INTO tags (
  name,
  type,
  sort_order,
  status,
  icon
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpsertActiveTagByNameAndType :one
INSERT INTO tags (
  name,
  type,
  sort_order,
  status
) VALUES (
  $1, $2, $3, 'active'
)
ON CONFLICT (name, type) DO UPDATE
SET sort_order = tags.sort_order
WHERE tags.status = 'active'
RETURNING id, name, type, sort_order, status, created_at, icon;

-- name: LinkMerchantSelectableTag :one
INSERT INTO merchant_selectable_tags (
  merchant_id,
  tag_id,
  sort_order,
  created_by_user_id
) VALUES (
  $1, $2, $3, $4
)
ON CONFLICT (merchant_id, tag_id) DO UPDATE
SET sort_order = EXCLUDED.sort_order
RETURNING merchant_id, tag_id, sort_order, created_by_user_id, created_at;

-- name: ListMerchantSelectableTags :many
SELECT
  t.id,
  t.name,
  t.type,
  mst.sort_order,
  t.status,
  t.created_at,
  t.icon
FROM tags t
INNER JOIN merchant_selectable_tags mst ON t.id = mst.tag_id
WHERE mst.merchant_id = $1
  AND t.type = $2
  AND t.status = 'active'
ORDER BY mst.sort_order ASC, t.name ASC, t.id ASC;

-- name: GetMerchantSelectableTag :one
SELECT
  t.id,
  t.name,
  t.type,
  mst.sort_order,
  t.status,
  t.created_at,
  t.icon
FROM tags t
INNER JOIN merchant_selectable_tags mst ON t.id = mst.tag_id
WHERE mst.merchant_id = $1
  AND mst.tag_id = $2
  AND t.type = $3
  AND t.status = 'active'
LIMIT 1;

-- name: GetTag :one
SELECT id, name, type, sort_order, status, created_at, icon FROM tags
WHERE id = $1 LIMIT 1;

-- name: ListTags :many
SELECT id, name, type, sort_order, status, created_at, icon FROM tags
WHERE type = $1 AND status = 'active'
ORDER BY sort_order ASC, name ASC
LIMIT $2 OFFSET $3;

-- name: ListAllTagsByType :many
SELECT id, name, type, sort_order, status, created_at, icon FROM tags
WHERE type = $1 AND status = 'active'
ORDER BY sort_order ASC, name ASC;

-- name: UpdateTag :one
UPDATE tags
SET
  name = COALESCE(sqlc.narg('name'), name),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
  status = COALESCE(sqlc.narg('status'), status),
  icon = COALESCE(sqlc.narg('icon'), icon)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags
WHERE id = $1;

-- name: SearchTags :many
SELECT id, name, type, sort_order, status, created_at, icon FROM tags
WHERE type = $1
  AND name ILIKE '%' || $2 || '%'
ORDER BY name
LIMIT $3 OFFSET $4;

-- name: GetActiveCategoriesByRegion :many
-- 返回指定区域内有商户覆盖的品类标签，按商户数量降序
SELECT t.id, t.name, t.sort_order, t.icon, COUNT(DISTINCT mt.merchant_id)::int AS merchant_count
FROM tags t
INNER JOIN merchant_tags mt ON t.id = mt.tag_id
INNER JOIN merchants m ON mt.merchant_id = m.id
WHERE m.region_id = sqlc.arg('region_id')
  AND m.status = 'active'
  AND m.deleted_at IS NULL
  AND t.type = 'merchant'
  AND t.status = 'active'
GROUP BY t.id, t.name, t.sort_order, t.icon
HAVING COUNT(DISTINCT mt.merchant_id) > 0
ORDER BY COUNT(DISTINCT mt.merchant_id) DESC, t.sort_order ASC;
