-- name: CreateUser :one
INSERT INTO users (
  wechat_openid,
  wechat_unionid,
  full_name,
  phone,
  avatar_url
) VALUES (
  $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByWechatOpenID :one
SELECT * FROM users
WHERE wechat_openid = $1 LIMIT 1;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET
  full_name = COALESCE(sqlc.narg(full_name), full_name),
  phone = COALESCE(sqlc.narg(phone), phone),
  avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
  wechat_unionid = COALESCE(sqlc.narg(wechat_unionid), wechat_unionid)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY id
LIMIT $1
OFFSET $2;
