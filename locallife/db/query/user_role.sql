-- name: CreateUserRole :one
INSERT INTO user_roles (
  user_id,
  role,
  status,
  related_entity_id
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: UpsertUserRoleActive :one
INSERT INTO user_roles (
  user_id,
  role,
  status,
  related_entity_id
) VALUES (
  $1, $2, 'active', $3
)
ON CONFLICT (user_id, role) DO UPDATE
SET status = 'active',
    related_entity_id = CASE
      WHEN user_roles.status = 'active' THEN user_roles.related_entity_id
      ELSE EXCLUDED.related_entity_id
    END
RETURNING *;

-- name: GetUserRole :one
SELECT id, user_id, role, status, related_entity_id, created_at FROM user_roles
WHERE id = $1 LIMIT 1;

-- name: ListUserRoles :many
SELECT id, user_id, role, status, related_entity_id, created_at FROM user_roles
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at;

-- name: GetUserRoleByType :one
SELECT id, user_id, role, status, related_entity_id, created_at FROM user_roles
WHERE user_id = $1 AND role = $2
LIMIT 1;

-- name: GetUserRoleByTypeForUpdate :one
SELECT id, user_id, role, status, related_entity_id, created_at FROM user_roles
WHERE user_id = $1 AND role = $2
LIMIT 1
FOR UPDATE;

-- name: UpdateUserRoleStatus :one
UPDATE user_roles
SET status = $2
WHERE id = $1
RETURNING *;

-- name: DeleteUserRole :exec
DELETE FROM user_roles
WHERE id = $1;

-- name: DeleteUserRoleByUserAndRole :exec
DELETE FROM user_roles
WHERE user_id = $1 AND role = $2;

-- name: HasRole :one
SELECT EXISTS(
  SELECT 1 FROM user_roles
  WHERE user_id = $1 AND role = $2 AND status = 'active'
);
