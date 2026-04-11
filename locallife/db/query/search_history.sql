-- name: CreateSearchHistory :one
INSERT INTO search_histories (user_id, keyword, type)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: UpsertSearchHistory :one
-- 插入或更新搜索历史（同一关键词存在时更新时间戳）
INSERT INTO search_histories (user_id, keyword, type, created_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, keyword, type) DO UPDATE
  SET created_at = NOW()
RETURNING *;

-- name: ListSearchHistory :many
SELECT id, keyword, type, created_at
FROM search_histories
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: DeleteSearchHistory :exec
DELETE FROM search_histories
WHERE id = $1 AND user_id = $2;

-- name: ClearSearchHistory :exec
DELETE FROM search_histories
WHERE user_id = $1;

-- name: GetPopularKeywords :many
SELECT keyword, type, count
FROM search_popular_keywords
WHERE type = $1
ORDER BY count DESC
LIMIT $2;

-- name: IncrementPopularKeyword :exec
INSERT INTO search_popular_keywords (keyword, type, count, updated_at)
VALUES ($1, $2, 1, NOW())
ON CONFLICT (keyword, type) DO UPDATE
  SET count = search_popular_keywords.count + 1,
      updated_at = NOW();
