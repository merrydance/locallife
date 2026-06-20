-- 浏览历史查询

-- name: RecordBrowseHistory :one
INSERT INTO browse_history (user_id, target_type, target_id, last_viewed_at, view_count)
VALUES ($1, $2, $3, NOW(), 1)
ON CONFLICT (user_id, target_type, target_id) DO UPDATE SET
    last_viewed_at = NOW(),
    view_count = browse_history.view_count + 1
RETURNING *;

-- name: ListBrowseHistory :many
SELECT id, user_id, target_type, target_id, last_viewed_at, view_count FROM browse_history
WHERE user_id = $1
ORDER BY last_viewed_at DESC
LIMIT $2 OFFSET $3;

-- name: ListBrowseHistoryByType :many
SELECT id, user_id, target_type, target_id, last_viewed_at, view_count FROM browse_history
WHERE user_id = $1 AND target_type = $2
ORDER BY last_viewed_at DESC
LIMIT $3 OFFSET $4;

-- name: ListBrowseHistoryFiltered :many
SELECT bh.id, bh.user_id, bh.target_type, bh.target_id, bh.last_viewed_at, bh.view_count
FROM browse_history bh
WHERE bh.user_id = sqlc.arg('user_id')
  AND (
    NOT sqlc.arg('exclude_packaging')::boolean
    OR bh.target_type <> 'dish'
    OR NOT EXISTS (
      SELECT 1
      FROM dishes d
      WHERE d.id = bh.target_id
        AND d.is_packaging = true
    )
  )
ORDER BY bh.last_viewed_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListBrowseHistoryByTypeFiltered :many
SELECT bh.id, bh.user_id, bh.target_type, bh.target_id, bh.last_viewed_at, bh.view_count
FROM browse_history bh
WHERE bh.user_id = sqlc.arg('user_id')
  AND bh.target_type = sqlc.arg('target_type')
  AND (
    NOT sqlc.arg('exclude_packaging')::boolean
    OR bh.target_type <> 'dish'
    OR NOT EXISTS (
      SELECT 1
      FROM dishes d
      WHERE d.id = bh.target_id
        AND d.is_packaging = true
    )
  )
ORDER BY bh.last_viewed_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountBrowseHistory :one
SELECT COUNT(*) FROM browse_history
WHERE user_id = $1;

-- name: CountBrowseHistoryByType :one
SELECT COUNT(*) FROM browse_history
WHERE user_id = $1 AND target_type = $2;

-- name: CountBrowseHistoryFiltered :one
SELECT COUNT(*) FROM browse_history bh
WHERE bh.user_id = sqlc.arg('user_id')
  AND (
    NOT sqlc.arg('exclude_packaging')::boolean
    OR bh.target_type <> 'dish'
    OR NOT EXISTS (
      SELECT 1
      FROM dishes d
      WHERE d.id = bh.target_id
        AND d.is_packaging = true
    )
  );

-- name: CountBrowseHistoryByTypeFiltered :one
SELECT COUNT(*) FROM browse_history bh
WHERE bh.user_id = sqlc.arg('user_id')
  AND bh.target_type = sqlc.arg('target_type')
  AND (
    NOT sqlc.arg('exclude_packaging')::boolean
    OR bh.target_type <> 'dish'
    OR NOT EXISTS (
      SELECT 1
      FROM dishes d
      WHERE d.id = bh.target_id
        AND d.is_packaging = true
    )
  );

-- name: DeleteBrowseHistory :exec
DELETE FROM browse_history
WHERE user_id = $1 AND target_type = $2 AND target_id = $3;

-- name: ClearBrowseHistory :exec
DELETE FROM browse_history
WHERE user_id = $1;
