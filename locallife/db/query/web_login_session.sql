-- name: CreateWebLoginSession :one
INSERT INTO web_login_sessions (
  code,
  status,
  expires_at,
  web_user_agent,
  web_client_ip,
  poll_token
) VALUES (
  $1, 'pending', $2, $3, $4, $5
) RETURNING *;

-- name: GetWebLoginSessionByCode :one
SELECT id, code, status, user_id, expires_at, confirmed_at, consumed_at, web_user_agent, web_client_ip, confirm_client_ip, created_at, updated_at, poll_token FROM web_login_sessions
WHERE code = $1
LIMIT 1;

-- name: GetWebLoginSessionByPollToken :one
SELECT id, code, status, user_id, expires_at, confirmed_at, consumed_at, web_user_agent, web_client_ip, confirm_client_ip, created_at, updated_at, poll_token FROM web_login_sessions
WHERE poll_token = $1
LIMIT 1;

-- name: ConfirmWebLoginSession :one
UPDATE web_login_sessions
SET status = 'confirmed',
    user_id = $2,
    confirmed_at = NOW(),
    confirm_client_ip = $3,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: ConsumeWebLoginSession :one
UPDATE web_login_sessions
SET status = 'consumed',
    consumed_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND status = 'confirmed'
RETURNING *;

-- name: ExpireWebLoginSession :one
UPDATE web_login_sessions
SET status = 'expired',
    updated_at = NOW()
WHERE id = $1 AND status <> 'consumed'
RETURNING *;

-- name: ExpireWebLoginSessionsBefore :execrows
-- 批量过期超时的 pending 会话
UPDATE web_login_sessions
SET status = 'expired',
    updated_at = NOW()
WHERE status = sqlc.arg('status')
  AND expires_at < sqlc.arg('expires_at');
