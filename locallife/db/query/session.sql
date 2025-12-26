-- name: CreateSession :one
INSERT INTO sessions (
  user_id,
  access_token,
  refresh_token,
  access_token_expires_at,
  refresh_token_expires_at,
  user_agent,
  client_ip
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions
WHERE id = $1 LIMIT 1;

-- name: GetSessionByAccessToken :one
SELECT * FROM sessions
WHERE access_token = $1 LIMIT 1;

-- name: GetSessionByRefreshToken :one
SELECT * FROM sessions
WHERE refresh_token = $1 LIMIT 1;

-- name: RevokeSession :one
UPDATE sessions
SET is_revoked = true
WHERE id = $1
RETURNING *;

-- name: RevokeUserSessions :exec
UPDATE sessions
SET is_revoked = true
WHERE user_id = $1 AND is_revoked = false;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE refresh_token_expires_at < NOW();

-- name: ListUserActiveSessions :many
SELECT * FROM sessions
WHERE user_id = $1 AND is_revoked = false
ORDER BY created_at DESC;
