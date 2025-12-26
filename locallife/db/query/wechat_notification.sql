-- name: CreateWechatNotification :one
INSERT INTO wechat_notifications (
    id,
    event_type,
    resource_type,
    summary,
    out_trade_no,
    transaction_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetWechatNotification :one
SELECT * FROM wechat_notifications
WHERE id = $1 LIMIT 1;

-- name: CheckNotificationExists :one
SELECT EXISTS(
    SELECT 1 FROM wechat_notifications
    WHERE id = $1
) AS exists;

-- name: ListWechatNotificationsByOutTradeNo :many
SELECT * FROM wechat_notifications
WHERE out_trade_no = $1
ORDER BY created_at DESC;

-- name: DeleteOldWechatNotifications :exec
-- 删除30天前的通知记录（数据清理）
DELETE FROM wechat_notifications
WHERE created_at < NOW() - INTERVAL '30 days';
