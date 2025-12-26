-- name: CreateUserRole :one
INSERT INTO user_roles (
  user_id,
  role,
  status,
  related_entity_id
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: GetUserRole :one
SELECT * FROM user_roles
WHERE id = $1 LIMIT 1;

-- name: ListUserRoles :many
SELECT * FROM user_roles
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at;

-- name: GetUserRoleByType :one
SELECT * FROM user_roles
WHERE user_id = $1 AND role = $2
LIMIT 1;

-- name: UpdateUserRoleStatus :one
UPDATE user_roles
SET status = $2
WHERE id = $1
RETURNING *;

-- name: DeleteUserRole :exec
DELETE FROM user_roles
WHERE id = $1;

-- name: HasRole :one
SELECT EXISTS(
  SELECT 1 FROM user_roles
  WHERE user_id = $1 AND role = $2 AND status = 'active'
);
