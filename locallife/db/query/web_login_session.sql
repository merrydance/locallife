-- name: CreateWebLoginSession :one
INSERT INTO web_login_sessions (
  code,
  status,
  expires_at,
  web_user_agent,
  web_client_ip
) VALUES (
  $1, 'pending', $2, $3, $4
) RETURNING *;

-- name: GetWebLoginSessionByCode :one
SELECT * FROM web_login_sessions
WHERE code = $1
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
