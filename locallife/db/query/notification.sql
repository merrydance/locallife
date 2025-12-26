-- name: CreateNotification :one
INSERT INTO notifications (
  user_id,
  type,
  title,
  content,
  related_type,
  related_id,
  extra_data,
  expires_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetNotification :one
SELECT * FROM notifications
WHERE id = $1;

-- name: ListUserNotifications :many
SELECT * FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('is_read')::boolean IS NULL OR is_read = sqlc.narg('is_read'))
  AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUserNotifications :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('is_read')::boolean IS NULL OR is_read = sqlc.narg('is_read'))
  AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type'));

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1
  AND is_read = false;

-- name: MarkNotificationAsRead :one
UPDATE notifications
SET 
  is_read = true,
  read_at = now()
WHERE id = $1
  AND user_id = $2
  AND is_read = false
RETURNING *;

-- name: MarkAllNotificationsAsRead :exec
UPDATE notifications
SET 
  is_read = true,
  read_at = now()
WHERE user_id = $1
  AND is_read = false;

-- name: MarkNotificationAsPushed :exec
UPDATE notifications
SET 
  is_pushed = true,
  pushed_at = now()
WHERE id = $1;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE id = $1
  AND user_id = $2;

-- name: DeleteReadNotifications :exec
DELETE FROM notifications
WHERE user_id = $1
  AND is_read = true;

-- name: DeleteExpiredNotifications :exec
DELETE FROM notifications
WHERE expires_at < now();

-- name: GetNotificationsByRelated :many
SELECT * FROM notifications
WHERE related_type = $1
  AND related_id = $2
ORDER BY created_at DESC;

-- ==================== 用户通知偏好设置 ====================

-- name: GetUserNotificationPreferences :one
SELECT * FROM user_notification_preferences
WHERE user_id = $1;

-- name: CreateUserNotificationPreferences :one
INSERT INTO user_notification_preferences (
  user_id,
  enable_order_notifications,
  enable_payment_notifications,
  enable_delivery_notifications,
  enable_system_notifications,
  enable_food_safety_notifications,
  do_not_disturb_start,
  do_not_disturb_end
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: UpdateUserNotificationPreferences :one
UPDATE user_notification_preferences
SET 
  enable_order_notifications = COALESCE(sqlc.narg('enable_order_notifications'), enable_order_notifications),
  enable_payment_notifications = COALESCE(sqlc.narg('enable_payment_notifications'), enable_payment_notifications),
  enable_delivery_notifications = COALESCE(sqlc.narg('enable_delivery_notifications'), enable_delivery_notifications),
  enable_system_notifications = COALESCE(sqlc.narg('enable_system_notifications'), enable_system_notifications),
  enable_food_safety_notifications = COALESCE(sqlc.narg('enable_food_safety_notifications'), enable_food_safety_notifications),
  do_not_disturb_start = COALESCE(sqlc.narg('do_not_disturb_start'), do_not_disturb_start),
  do_not_disturb_end = COALESCE(sqlc.narg('do_not_disturb_end'), do_not_disturb_end),
  updated_at = now()
WHERE user_id = $1
RETURNING *;

-- name: GetOrCreateUserNotificationPreferences :one
INSERT INTO user_notification_preferences (
  user_id,
  enable_order_notifications,
  enable_payment_notifications,
  enable_delivery_notifications,
  enable_system_notifications,
  enable_food_safety_notifications
) VALUES (
  $1, true, true, true, true, true
)
ON CONFLICT (user_id) DO UPDATE
SET user_id = EXCLUDED.user_id
RETURNING *;
