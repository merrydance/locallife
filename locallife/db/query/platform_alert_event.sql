-- name: CreatePlatformAlertEvent :one
INSERT INTO platform_alert_events (
  alert_type,
  level,
  title,
  message,
  related_id,
  related_type,
  extra,
  emitted_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: CountPlatformAlertEvents :one
SELECT COUNT(*) FROM platform_alert_events;

-- name: ListPlatformAlertEvents :many
SELECT id, alert_type, level, title, message, related_id, related_type, extra, emitted_at FROM platform_alert_events
ORDER BY emitted_at DESC, id DESC
LIMIT $1 OFFSET $2;
