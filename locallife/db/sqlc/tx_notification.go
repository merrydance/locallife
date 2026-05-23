package db

import (
	"context"
)

// TryClaimWechatNotification 原子性地尝试认领一个微信通知占位。
func (store *SQLStore) TryClaimWechatNotification(ctx context.Context, arg CreateWechatNotificationParams) (bool, error) {
	result, err := store.connPool.Exec(ctx,
		`INSERT INTO wechat_notifications (id, event_type, resource_type, summary, out_trade_no, transaction_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		arg.ID, arg.EventType, arg.ResourceType, arg.Summary, arg.OutTradeNo, arg.TransactionID,
	)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

// ReleaseWechatNotificationClaim 删除一个已认领但未成功处理的通知占位记录。
func (store *SQLStore) ReleaseWechatNotificationClaim(ctx context.Context, id string) error {
	_, err := store.connPool.Exec(ctx, `DELETE FROM wechat_notifications WHERE id = $1`, id)
	return err
}

// MarkWechatNotificationProcessed 标记通知处理完成。
func (store *SQLStore) MarkWechatNotificationProcessed(ctx context.Context, id, outTradeNo, transactionID string) error {
	_, err := store.connPool.Exec(ctx,
		`UPDATE wechat_notifications
		 SET processed_at = NOW(),
		     out_trade_no = COALESCE(NULLIF($2, ''), out_trade_no),
		     transaction_id = COALESCE(NULLIF($3, ''), transaction_id)
		 WHERE id = $1`,
		id, outTradeNo, transactionID,
	)
	return err
}
