package worker

import (
	"context"
)

// NotificationAdapter 将 TaskDistributor 适配为 algorithm.NotificationDistributor
// 用于在 algorithm 包中发送用户通知，避免循环依赖
type NotificationAdapter struct {
	distributor TaskDistributor
}

// NewNotificationAdapter 创建通知适配器
func NewNotificationAdapter(distributor TaskDistributor) *NotificationAdapter {
	return &NotificationAdapter{
		distributor: distributor,
	}
}

// SendUserNotification 实现 algorithm.NotificationDistributor 接口
// 通过 TaskDistributor 异步发送用户通知
func (a *NotificationAdapter) SendUserNotification(
	ctx context.Context,
	userID int64,
	notificationType, title, content string,
	relatedType string,
	relatedID int64,
) error {
	return a.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:      userID,
		Type:        notificationType,
		Title:       title,
		Content:     content,
		RelatedType: relatedType,
		RelatedID:   relatedID,
	})
}
