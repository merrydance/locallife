-- name: CreateTag :one
INSERT INTO tags (
  name,
  type,
  sort_order,
  status
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;

-- name: GetTag :one
SELECT * FROM tags
WHERE id = $1 LIMIT 1;

-- name: ListTags :many
SELECT * FROM tags
WHERE type = $1 AND status = 'active'
ORDER BY sort_order ASC, name ASC
LIMIT $2 OFFSET $3;

-- name: ListAllTagsByType :many
SELECT * FROM tags
WHERE type = $1 AND status = 'active'
ORDER BY sort_order ASC, name ASC;

-- name: UpdateTag :one
UPDATE tags
SET
  name = COALESCE(sqlc.narg('name'), name),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
  status = COALESCE(sqlc.narg('status'), status)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags
WHERE id = $1;

-- name: SearchTags :many
SELECT * FROM tags
WHERE type = $1
  AND name ILIKE '%' || $2 || '%'
ORDER BY name
LIMIT $3 OFFSET $4;
