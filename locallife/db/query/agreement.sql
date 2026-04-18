-- name: GetActiveAgreementByType :one
SELECT id, type, title, content, version, published_on, is_active, created_at, updated_at FROM agreements
WHERE type = $1 AND is_active = true
ORDER BY published_on DESC, created_at DESC
LIMIT 1;

-- name: ListActiveAgreements :many
SELECT type, title, version, published_on 
FROM agreements
WHERE is_active = true
ORDER BY type ASC;
