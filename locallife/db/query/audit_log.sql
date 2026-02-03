-- name: CreateAuditLog :one
INSERT INTO audit_logs (
  actor_user_id,
  actor_role,
  action,
  target_type,
  target_id,
  region_id,
  request_id,
  trace_id,
  client_ip,
  user_agent,
  metadata
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10, $11
)
RETURNING *;
