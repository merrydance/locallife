package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// 创建随机微信通知记录（测试辅助函数）
func createRandomWechatNotification(t *testing.T) WechatNotification {
	arg := CreateWechatNotificationParams{
		ID:            util.RandomString(32),
		EventType:     "TRANSACTION.SUCCESS",
		ResourceType:  pgtype.Text{String: "encrypt-resource", Valid: true},
		Summary:       pgtype.Text{String: "支付成功", Valid: true},
		OutTradeNo:    pgtype.Text{String: util.RandomString(32), Valid: true},
		TransactionID: pgtype.Text{String: util.RandomString(28), Valid: true},
	}

	notification, err := testStore.CreateWechatNotification(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, notification)

	require.Equal(t, arg.ID, notification.ID)
	require.Equal(t, arg.EventType, notification.EventType)
	require.Equal(t, arg.ResourceType.String, notification.ResourceType.String)
	require.Equal(t, arg.Summary.String, notification.Summary.String)
	require.Equal(t, arg.OutTradeNo.String, notification.OutTradeNo.String)
	require.Equal(t, arg.TransactionID.String, notification.TransactionID.String)
	require.NotZero(t, notification.CreatedAt)

	return notification
}

// 测试创建微信通知记录
func TestCreateWechatNotification(t *testing.T) {
	createRandomWechatNotification(t)
}

// 测试幂等性检查：已存在的通知
func TestCheckNotificationExists_True(t *testing.T) {
	notification := createRandomWechatNotification(t)

	exists, err := testStore.CheckNotificationExists(context.Background(), notification.ID)
	require.NoError(t, err)
	require.True(t, exists, "已插入的通知应该返回exists=true")
}

// 测试幂等性检查：不存在的通知
func TestCheckNotificationExists_False(t *testing.T) {
	nonExistentID := util.RandomString(32)

	exists, err := testStore.CheckNotificationExists(context.Background(), nonExistentID)
	require.NoError(t, err)
	require.False(t, exists, "不存在的通知应该返回exists=false")
}

// 测试获取单条通知记录
func TestGetWechatNotification(t *testing.T) {
	notification1 := createRandomWechatNotification(t)

	notification2, err := testStore.GetWechatNotification(context.Background(), notification1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notification2)

	require.Equal(t, notification1.ID, notification2.ID)
	require.Equal(t, notification1.EventType, notification2.EventType)
	require.Equal(t, notification1.OutTradeNo.String, notification2.OutTradeNo.String)
	require.Equal(t, notification1.TransactionID.String, notification2.TransactionID.String)
}

// 测试按out_trade_no查询列表
func TestListWechatNotificationsByOutTradeNo(t *testing.T) {
	// 创建同一个out_trade_no的3条通知（模拟重试场景）
	targetOutTradeNo := "TEST_" + util.RandomString(28)

	for i := 0; i < 3; i++ {
		arg := CreateWechatNotificationParams{
			ID:            util.RandomString(32),
			EventType:     "TRANSACTION.SUCCESS",
			ResourceType:  pgtype.Text{String: "encrypt-resource", Valid: true},
			Summary:       pgtype.Text{String: "测试订单", Valid: true},
			OutTradeNo:    pgtype.Text{String: targetOutTradeNo, Valid: true},
			TransactionID: pgtype.Text{String: util.RandomString(28), Valid: true},
		}
		_, err := testStore.CreateWechatNotification(context.Background(), arg)
		require.NoError(t, err)
	}

	// 查询该out_trade_no的所有通知
	notifications, err := testStore.ListWechatNotificationsByOutTradeNo(
		context.Background(),
		pgtype.Text{String: targetOutTradeNo, Valid: true},
	)
	require.NoError(t, err)
	require.Len(t, notifications, 3, "应该返回该订单的3条通知")

	for _, notification := range notifications {
		require.NotEmpty(t, notification.ID)
		require.Equal(t, "TRANSACTION.SUCCESS", notification.EventType)
		require.Equal(t, targetOutTradeNo, notification.OutTradeNo.String)
	}
}

// 测试删除旧通知（清理机制）
func TestDeleteOldWechatNotifications(t *testing.T) {
	// 创建一条新通知
	notification := createRandomWechatNotification(t)

	// 验证通知存在
	exists, err := testStore.CheckNotificationExists(context.Background(), notification.ID)
	require.NoError(t, err)
	require.True(t, exists)

	// 删除30天前的通知（刚创建的不会被删除）
	err = testStore.DeleteOldWechatNotifications(context.Background())
	require.NoError(t, err)

	// 验证通知仍然存在
	exists, err = testStore.CheckNotificationExists(context.Background(), notification.ID)
	require.NoError(t, err)
	require.True(t, exists, "刚创建的通知不应该被删除")
}

// 测试重复插入相同notification_id（应该失败）
func TestCreateWechatNotification_DuplicateID(t *testing.T) {
	notification := createRandomWechatNotification(t)

	// 尝试插入相同的ID
	arg := CreateWechatNotificationParams{
		ID:            notification.ID, // 重复的ID
		EventType:     "REFUND.SUCCESS",
		ResourceType:  pgtype.Text{String: "encrypt-resource", Valid: true},
		Summary:       pgtype.Text{String: "退款成功", Valid: true},
		OutTradeNo:    pgtype.Text{String: util.RandomString(32), Valid: true},
		TransactionID: pgtype.Text{String: util.RandomString(28), Valid: true},
	}

	_, err := testStore.CreateWechatNotification(context.Background(), arg)
	require.Error(t, err, "重复的notification_id应该返回错误")
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

// 测试不同事件类型的通知
func TestCreateWechatNotification_DifferentEventTypes(t *testing.T) {
	eventTypes := []string{
		"TRANSACTION.SUCCESS",
		"REFUND.SUCCESS",
		"REFUND.ABNORMAL",
		"REFUND.CLOSED",
	}

	for _, eventType := range eventTypes {
		arg := CreateWechatNotificationParams{
			ID:            util.RandomString(32),
			EventType:     eventType,
			ResourceType:  pgtype.Text{String: "encrypt-resource", Valid: true},
			Summary:       pgtype.Text{String: eventType + " 测试", Valid: true},
			OutTradeNo:    pgtype.Text{String: util.RandomString(32), Valid: true},
			TransactionID: pgtype.Text{String: util.RandomString(28), Valid: true},
		}

		notification, err := testStore.CreateWechatNotification(context.Background(), arg)
		require.NoError(t, err)
		require.Equal(t, eventType, notification.EventType)
	}
}

// 测试可选字段（TransactionID为NULL的情况，如合单支付）
func TestCreateWechatNotification_NullableFields(t *testing.T) {
	arg := CreateWechatNotificationParams{
		ID:        util.RandomString(32),
		EventType: "TRANSACTION.SUCCESS",
		// ResourceType, Summary, OutTradeNo, TransactionID 都是可选的
		ResourceType:  pgtype.Text{Valid: false}, // NULL
		Summary:       pgtype.Text{Valid: false}, // NULL
		OutTradeNo:    pgtype.Text{String: util.RandomString(32), Valid: true},
		TransactionID: pgtype.Text{Valid: false}, // NULL（合单支付场景）
	}

	notification, err := testStore.CreateWechatNotification(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, notification)

	require.Equal(t, arg.ID, notification.ID)
	require.False(t, notification.ResourceType.Valid)
	require.False(t, notification.Summary.Valid)
	require.False(t, notification.TransactionID.Valid)
	require.True(t, notification.OutTradeNo.Valid)
}

// 测试查询不存在的out_trade_no
func TestListWechatNotificationsByOutTradeNo_NotFound(t *testing.T) {
	nonExistentOutTradeNo := "NONEXISTENT_" + util.RandomString(28)

	notifications, err := testStore.ListWechatNotificationsByOutTradeNo(
		context.Background(),
		pgtype.Text{String: nonExistentOutTradeNo, Valid: true},
	)
	require.NoError(t, err)
	require.Empty(t, notifications, "不存在的out_trade_no应该返回空列表")
}

// 性能测试：批量插入和幂等性检查
func TestWechatNotification_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	ctx := context.Background()
	notificationIDs := make([]string, 100)

	// 批量创建100条通知
	startInsert := time.Now()
	for i := 0; i < 100; i++ {
		arg := CreateWechatNotificationParams{
			ID:            util.RandomString(32),
			EventType:     "TRANSACTION.SUCCESS",
			ResourceType:  pgtype.Text{String: "encrypt-resource", Valid: true},
			Summary:       pgtype.Text{String: "性能测试", Valid: true},
			OutTradeNo:    pgtype.Text{String: util.RandomString(32), Valid: true},
			TransactionID: pgtype.Text{String: util.RandomString(28), Valid: true},
		}

		notification, err := testStore.CreateWechatNotification(ctx, arg)
		require.NoError(t, err)
		notificationIDs[i] = notification.ID
	}
	insertDuration := time.Since(startInsert)
	t.Logf("插入100条通知耗时: %v (平均 %v/条)", insertDuration, insertDuration/100)

	// 批量检查幂等性
	startCheck := time.Now()
	for _, id := range notificationIDs {
		exists, err := testStore.CheckNotificationExists(ctx, id)
		require.NoError(t, err)
		require.True(t, exists)
	}
	checkDuration := time.Since(startCheck)
	t.Logf("检查100条通知幂等性耗时: %v (平均 %v/条)", checkDuration, checkDuration/100)

	// 性能断言：单次检查应该 < 10ms
	avgCheckTime := checkDuration / 100
	require.Less(t, avgCheckTime, 10*time.Millisecond, "单次幂等性检查应该小于10ms")
}
