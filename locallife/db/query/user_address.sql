-- name: CreateUserAddress :one
INSERT INTO user_addresses (
  user_id,
  region_id,
  detail_address,
  contact_name,
  contact_phone,
  longitude,
  latitude,
  is_default
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetUserAddress :one
SELECT * FROM user_addresses
WHERE id = $1 LIMIT 1;

-- name: ListUserAddresses :many
SELECT * FROM user_addresses
WHERE user_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: GetUserDefaultAddress :one
SELECT * FROM user_addresses
WHERE user_id = $1 AND is_default = true
LIMIT 1;

-- name: UpdateUserAddress :one
UPDATE user_addresses
SET
  region_id = COALESCE(sqlc.narg(region_id), region_id),
  detail_address = COALESCE(sqlc.narg(detail_address), detail_address),
  contact_name = COALESCE(sqlc.narg(contact_name), contact_name),
  contact_phone = COALESCE(sqlc.narg(contact_phone), contact_phone),
  longitude = COALESCE(sqlc.narg(longitude), longitude),
  latitude = COALESCE(sqlc.narg(latitude), latitude),
  is_default = COALESCE(sqlc.narg(is_default), is_default)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: SetDefaultAddress :exec
-- 先将用户的所有地址设为非默认
UPDATE user_addresses
SET is_default = false
WHERE user_id = $1;

-- name: SetAddressAsDefault :one
-- 设置指定地址为默认
UPDATE user_addresses
SET is_default = true
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteUserAddress :exec
DELETE FROM user_addresses
WHERE id = $1 AND user_id = $2;

-- name: CountUserAddresses :one
SELECT COUNT(*) FROM user_addresses
WHERE user_id = $1;
