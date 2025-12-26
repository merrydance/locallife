-- 浏览历史查询

-- name: RecordBrowseHistory :one
INSERT INTO browse_history (user_id, target_type, target_id, last_viewed_at, view_count)
VALUES ($1, $2, $3, NOW(), 1)
ON CONFLICT (user_id, target_type, target_id) DO UPDATE SET
    last_viewed_at = NOW(),
    view_count = browse_history.view_count + 1
RETURNING *;

-- name: ListBrowseHistory :many
SELECT * FROM browse_history
WHERE user_id = $1
ORDER BY last_viewed_at DESC
LIMIT $2 OFFSET $3;

-- name: ListBrowseHistoryByType :many
SELECT * FROM browse_history
WHERE user_id = $1 AND target_type = $2
ORDER BY last_viewed_at DESC
LIMIT $3 OFFSET $4;

-- name: CountBrowseHistory :one
SELECT COUNT(*) FROM browse_history
WHERE user_id = $1;

-- name: CountBrowseHistoryByType :one
SELECT COUNT(*) FROM browse_history
WHERE user_id = $1 AND target_type = $2;

-- name: DeleteBrowseHistory :exec
DELETE FROM browse_history
WHERE user_id = $1 AND target_type = $2 AND target_id = $3;

-- name: ClearBrowseHistory :exec
DELETE FROM browse_history
WHERE user_id = $1;
