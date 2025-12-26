-- name: UpsertWechatAccessToken :one
INSERT INTO wechat_access_tokens (
  app_type,
  access_token,
  expires_at
) VALUES (
  $1, $2, $3
)
ON CONFLICT (app_type)
DO UPDATE SET
  access_token = EXCLUDED.access_token,
  expires_at = EXCLUDED.expires_at,
  created_at = NOW()
RETURNING *;

-- name: GetWechatAccessToken :one
SELECT * FROM wechat_access_tokens
WHERE app_type = $1 LIMIT 1;
