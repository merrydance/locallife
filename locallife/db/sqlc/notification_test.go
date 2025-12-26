package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomNotification(t *testing.T, userID int64) Notification {
	arg := CreateNotificationParams{
		UserID:  userID,
		Type:    "order",
		Title:   "订单状态更新",
		Content: "您的订单已支付",
		RelatedType: pgtype.Text{
			String: "order",
			Valid:  true,
		},
		RelatedID: pgtype.Int8{
			Int64: util.RandomInt(1, 1000),
			Valid: true,
		},
		ExtraData: []byte(`{"order_id": 123, "status": "paid"}`),
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(7 * 24 * time.Hour),
			Valid: true,
		},
	}

	notification, err := testStore.CreateNotification(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, notification)

	require.Equal(t, arg.UserID, notification.UserID)
	require.Equal(t, arg.Type, notification.Type)
	require.Equal(t, arg.Title, notification.Title)
	require.Equal(t, arg.Content, notification.Content)
	require.False(t, notification.IsRead)
	require.False(t, notification.IsPushed)
	require.NotZero(t, notification.ID)
	require.NotZero(t, notification.CreatedAt)

	return notification
}

func TestCreateNotification(t *testing.T) {
	user := createRandomUser(t)
	createRandomNotification(t, user.ID)
}

func TestGetNotification(t *testing.T) {
	user := createRandomUser(t)
	notification1 := createRandomNotification(t, user.ID)

	notification2, err := testStore.GetNotification(context.Background(), notification1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notification2)

	require.Equal(t, notification1.ID, notification2.ID)
	require.Equal(t, notification1.UserID, notification2.UserID)
	require.Equal(t, notification1.Type, notification2.Type)
	require.Equal(t, notification1.Title, notification2.Title)
	require.Equal(t, notification1.Content, notification2.Content)
}

func TestGetNotification_NotFound(t *testing.T) {
	_, err := testStore.GetNotification(context.Background(), 999999)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestListUserNotifications(t *testing.T) {
	user := createRandomUser(t)

	// 创建5个通知
	for i := 0; i < 5; i++ {
		createRandomNotification(t, user.ID)
	}

	// 查询所有通知
	notifications, err := testStore.ListUserNotifications(context.Background(), ListUserNotificationsParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, notifications, 5)

	// 验证所有通知都属于该用户
	for _, n := range notifications {
		require.Equal(t, user.ID, n.UserID)
	}
}

func TestListUserNotifications_WithFilters(t *testing.T) {
	user := createRandomUser(t)

	// 创建不同类型的通知
	createRandomNotification(t, user.ID) // order类型

	paymentNotif := CreateNotificationParams{
		UserID:  user.ID,
		Type:    "payment",
		Title:   "支付成功",
		Content: "您的支付已完成",
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(7 * 24 * time.Hour),
			Valid: true,
		},
	}
	testStore.CreateNotification(context.Background(), paymentNotif)

	// 过滤payment类型
	notifications, err := testStore.ListUserNotifications(context.Background(), ListUserNotificationsParams{
		UserID: user.ID,
		Type: pgtype.Text{
			String: "payment",
			Valid:  true,
		},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	require.Equal(t, "payment", notifications[0].Type)
}

func TestCountUserNotifications(t *testing.T) {
	user := createRandomUser(t)

	// 创建3个通知
	for i := 0; i < 3; i++ {
		createRandomNotification(t, user.ID)
	}

	count, err := testStore.CountUserNotifications(context.Background(), CountUserNotificationsParams{
		UserID: user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountUnreadNotifications(t *testing.T) {
	user := createRandomUser(t)

	// 创建3个未读通知
	for i := 0; i < 3; i++ {
		createRandomNotification(t, user.ID)
	}

	count, err := testStore.CountUnreadNotifications(context.Background(), user.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

func TestMarkNotificationAsRead(t *testing.T) {
	user := createRandomUser(t)
	notification := createRandomNotification(t, user.ID)

	require.False(t, notification.IsRead)
	require.False(t, notification.ReadAt.Valid)

	// 标记为已读
	updated, err := testStore.MarkNotificationAsRead(context.Background(), MarkNotificationAsReadParams{
		ID:     notification.ID,
		UserID: user.ID,
	})
	require.NoError(t, err)
	require.True(t, updated.IsRead)
	require.True(t, updated.ReadAt.Valid)

	// 再次标记（应该返回0行）
	_, err = testStore.MarkNotificationAsRead(context.Background(), MarkNotificationAsReadParams{
		ID:     notification.ID,
		UserID: user.ID,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestMarkNotificationAsRead_WrongUser(t *testing.T) {
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	notification := createRandomNotification(t, user1.ID)

	// 尝试用错误的用户ID标记
	_, err := testStore.MarkNotificationAsRead(context.Background(), MarkNotificationAsReadParams{
		ID:     notification.ID,
		UserID: user2.ID,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestMarkAllNotificationsAsRead(t *testing.T) {
	user := createRandomUser(t)

	// 创建5个未读通知
	for i := 0; i < 5; i++ {
		createRandomNotification(t, user.ID)
	}

	// 标记所有为已读
	err := testStore.MarkAllNotificationsAsRead(context.Background(), user.ID)
	require.NoError(t, err)

	// 验证未读数量为0
	count, err := testStore.CountUnreadNotifications(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestMarkNotificationAsPushed(t *testing.T) {
	user := createRandomUser(t)
	notification := createRandomNotification(t, user.ID)

	require.False(t, notification.IsPushed)

	// 标记为已推送
	err := testStore.MarkNotificationAsPushed(context.Background(), notification.ID)
	require.NoError(t, err)

	// 验证
	updated, err := testStore.GetNotification(context.Background(), notification.ID)
	require.NoError(t, err)
	require.True(t, updated.IsPushed)
	require.True(t, updated.PushedAt.Valid)
}

func TestDeleteNotification(t *testing.T) {
	user := createRandomUser(t)
	notification := createRandomNotification(t, user.ID)

	// 删除通知
	err := testStore.DeleteNotification(context.Background(), DeleteNotificationParams{
		ID:     notification.ID,
		UserID: user.ID,
	})
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetNotification(context.Background(), notification.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestDeleteNotification_WrongUser(t *testing.T) {
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	notification := createRandomNotification(t, user1.ID)

	// 尝试用错误的用户ID删除
	err := testStore.DeleteNotification(context.Background(), DeleteNotificationParams{
		ID:     notification.ID,
		UserID: user2.ID,
	})
	require.NoError(t, err) // DELETE不存在的记录不报错

	// 验证通知仍然存在
	_, err = testStore.GetNotification(context.Background(), notification.ID)
	require.NoError(t, err)
}

func TestDeleteReadNotifications(t *testing.T) {
	user := createRandomUser(t)

	// 创建5个通知，标记3个为已读
	for i := 0; i < 5; i++ {
		n := createRandomNotification(t, user.ID)
		if i < 3 {
			testStore.MarkNotificationAsRead(context.Background(), MarkNotificationAsReadParams{
				ID:     n.ID,
				UserID: user.ID,
			})
		}
	}

	// 删除已读通知
	err := testStore.DeleteReadNotifications(context.Background(), user.ID)
	require.NoError(t, err)

	// 验证还剩2个未读通知
	count, err := testStore.CountUnreadNotifications(context.Background(), user.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(2))
}

func TestDeleteExpiredNotifications(t *testing.T) {
	user := createRandomUser(t)

	// 创建过期通知
	expiredNotif := CreateNotificationParams{
		UserID:  user.ID,
		Type:    "system",
		Title:   "过期通知",
		Content: "这是一个过期的通知",
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-1 * time.Hour), // 1小时前过期
			Valid: true,
		},
	}
	expired, err := testStore.CreateNotification(context.Background(), expiredNotif)
	require.NoError(t, err)

	// 创建未过期通知
	createRandomNotification(t, user.ID)

	// 删除过期通知
	err = testStore.DeleteExpiredNotifications(context.Background())
	require.NoError(t, err)

	// 验证过期通知已删除
	_, err = testStore.GetNotification(context.Background(), expired.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestGetNotificationsByRelated(t *testing.T) {
	user := createRandomUser(t)
	orderID := util.RandomInt(1, 1000)

	// 创建3个关联到同一订单的通知
	for i := 0; i < 3; i++ {
		arg := CreateNotificationParams{
			UserID:  user.ID,
			Type:    "order",
			Title:   "订单更新",
			Content: "订单状态已变更",
			RelatedType: pgtype.Text{
				String: "order",
				Valid:  true,
			},
			RelatedID: pgtype.Int8{
				Int64: orderID,
				Valid: true,
			},
			ExpiresAt: pgtype.Timestamptz{
				Time:  time.Now().Add(7 * 24 * time.Hour),
				Valid: true,
			},
		}
		testStore.CreateNotification(context.Background(), arg)
	}

	// 查询关联通知
	notifications, err := testStore.GetNotificationsByRelated(context.Background(), GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "order", Valid: true},
		RelatedID:   pgtype.Int8{Int64: orderID, Valid: true},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(notifications), 3)

	// 验证关联信息
	for _, n := range notifications {
		require.Equal(t, "order", n.RelatedType.String)
		require.Equal(t, orderID, n.RelatedID.Int64)
	}
}

func TestGetNotificationsByRelated_Empty(t *testing.T) {
	notifications, err := testStore.GetNotificationsByRelated(context.Background(), GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "order", Valid: true},
		RelatedID:   pgtype.Int8{Int64: 999999, Valid: true},
	})
	require.NoError(t, err)
	require.Empty(t, notifications)
}

// ==================== 用户通知偏好设置测试 ====================

func TestGetOrCreateUserNotificationPreferences(t *testing.T) {
	user := createRandomUser(t)

	// 首次调用，应该创建
	prefs, err := testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, prefs)
	require.Equal(t, user.ID, prefs.UserID)
	require.True(t, prefs.EnableOrderNotifications)
	require.True(t, prefs.EnablePaymentNotifications)

	// 再次调用，应该返回已存在的
	prefs2, err := testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, prefs.ID, prefs2.ID)
}

func TestUpdateUserNotificationPreferences(t *testing.T) {
	user := createRandomUser(t)

	// 先创建
	testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)

	// 更新设置
	updated, err := testStore.UpdateUserNotificationPreferences(context.Background(), UpdateUserNotificationPreferencesParams{
		UserID: user.ID,
		EnableOrderNotifications: pgtype.Bool{
			Bool:  false,
			Valid: true,
		},
		EnableSystemNotifications: pgtype.Bool{
			Bool:  false,
			Valid: true,
		},
		DoNotDisturbStart: pgtype.Time{
			Microseconds: int64(22 * time.Hour / time.Microsecond), // 22:00
			Valid:        true,
		},
		DoNotDisturbEnd: pgtype.Time{
			Microseconds: int64(8 * time.Hour / time.Microsecond), // 08:00
			Valid:        true,
		},
	})
	require.NoError(t, err)
	require.False(t, updated.EnableOrderNotifications)
	require.False(t, updated.EnableSystemNotifications)
	require.True(t, updated.DoNotDisturbStart.Valid)
	require.True(t, updated.DoNotDisturbEnd.Valid)
}

func TestNotificationExtraData_JSON(t *testing.T) {
	user := createRandomUser(t)

	// 创建包含复杂JSON数据的通知
	extraData := map[string]interface{}{
		"order_id":     123,
		"order_status": "paid",
		"amount":       9999,
		"items": []map[string]interface{}{
			{"name": "商品1", "quantity": 2},
			{"name": "商品2", "quantity": 1},
		},
	}
	jsonData, err := json.Marshal(extraData)
	require.NoError(t, err)

	arg := CreateNotificationParams{
		UserID:    user.ID,
		Type:      "order",
		Title:     "订单详情",
		Content:   "您的订单已支付",
		ExtraData: jsonData,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(7 * 24 * time.Hour),
			Valid: true,
		},
	}

	notification, err := testStore.CreateNotification(context.Background(), arg)
	require.NoError(t, err)

	// 解析JSON
	var parsedData map[string]interface{}
	err = json.Unmarshal(notification.ExtraData, &parsedData)
	require.NoError(t, err)
	require.Equal(t, float64(123), parsedData["order_id"])
	require.Equal(t, "paid", parsedData["order_status"])
}

func TestGetUserNotificationPreferences(t *testing.T) {
	user := createRandomUser(t)

	// 首次查询，应该返回 ErrNoRows
	_, err := testStore.GetUserNotificationPreferences(context.Background(), user.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	// 创建偏好设置
	_, err = testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)

	// 再次查询，应该成功
	prefs, err := testStore.GetUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, user.ID, prefs.UserID)
	require.True(t, prefs.EnableOrderNotifications)
	require.True(t, prefs.EnablePaymentNotifications)
	require.True(t, prefs.EnableDeliveryNotifications)
	require.True(t, prefs.EnableSystemNotifications)
	require.True(t, prefs.EnableFoodSafetyNotifications)
}

func TestGetUserNotificationPreferences_NotFound(t *testing.T) {
	// 查询不存在的用户ID
	_, err := testStore.GetUserNotificationPreferences(context.Background(), 999999)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestUpdateUserNotificationPreferences_DisableTypes(t *testing.T) {
	user := createRandomUser(t)

	// 先创建
	_, err := testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)

	// 禁用订单和系统通知
	updated, err := testStore.UpdateUserNotificationPreferences(context.Background(), UpdateUserNotificationPreferencesParams{
		UserID: user.ID,
		EnableOrderNotifications: pgtype.Bool{
			Bool:  false,
			Valid: true,
		},
		EnableSystemNotifications: pgtype.Bool{
			Bool:  false,
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.False(t, updated.EnableOrderNotifications)
	require.False(t, updated.EnableSystemNotifications)
	// 其他类型应该保持不变
	require.True(t, updated.EnablePaymentNotifications)
	require.True(t, updated.EnableDeliveryNotifications)
	require.True(t, updated.EnableFoodSafetyNotifications)

	// 验证持久化
	prefs, err := testStore.GetUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.False(t, prefs.EnableOrderNotifications)
	require.False(t, prefs.EnableSystemNotifications)
}

func TestUpdateUserNotificationPreferences_DoNotDisturb(t *testing.T) {
	user := createRandomUser(t)

	// 先创建
	_, err := testStore.GetOrCreateUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)

	// 设置免打扰时段 22:00 - 08:00
	startMicroseconds := int64(22 * 3600 * 1000000) // 22:00
	endMicroseconds := int64(8 * 3600 * 1000000)    // 08:00

	updated, err := testStore.UpdateUserNotificationPreferences(context.Background(), UpdateUserNotificationPreferencesParams{
		UserID: user.ID,
		DoNotDisturbStart: pgtype.Time{
			Microseconds: startMicroseconds,
			Valid:        true,
		},
		DoNotDisturbEnd: pgtype.Time{
			Microseconds: endMicroseconds,
			Valid:        true,
		},
	})
	require.NoError(t, err)
	require.True(t, updated.DoNotDisturbStart.Valid)
	require.True(t, updated.DoNotDisturbEnd.Valid)
	require.Equal(t, startMicroseconds, updated.DoNotDisturbStart.Microseconds)
	require.Equal(t, endMicroseconds, updated.DoNotDisturbEnd.Microseconds)

	// 验证持久化
	prefs, err := testStore.GetUserNotificationPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, startMicroseconds, prefs.DoNotDisturbStart.Microseconds)
	require.Equal(t, endMicroseconds, prefs.DoNotDisturbEnd.Microseconds)
}

func TestNotificationTypeFiltering(t *testing.T) {
	user := createRandomUser(t)

	// 创建不同类型的通知
	types := []string{"order", "payment", "delivery", "system", "food_safety"}
	for _, nType := range types {
		arg := CreateNotificationParams{
			UserID:  user.ID,
			Type:    nType,
			Title:   "测试通知-" + nType,
			Content: "测试内容",
		}
		_, err := testStore.CreateNotification(context.Background(), arg)
		require.NoError(t, err)
	}

	// 验证每种类型都能筛选出来
	for _, nType := range types {
		notifications, err := testStore.ListUserNotifications(context.Background(), ListUserNotificationsParams{
			UserID: user.ID,
			Type: pgtype.Text{
				String: nType,
				Valid:  true,
			},
			Limit:  10,
			Offset: 0,
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(notifications), 1)
		require.Equal(t, nType, notifications[0].Type)
	}
}

func TestListUserNotifications_IsReadFilter(t *testing.T) {
	user := createRandomUser(t)

	// 创建3个已读和2个未读通知
	for i := 0; i < 5; i++ {
		n := createRandomNotification(t, user.ID)
		if i < 3 {
			_, err := testStore.MarkNotificationAsRead(context.Background(), MarkNotificationAsReadParams{
				ID:     n.ID,
				UserID: user.ID,
			})
			require.NoError(t, err)
		}
	}

	// 查询已读通知
	readNotifs, err := testStore.ListUserNotifications(context.Background(), ListUserNotificationsParams{
		UserID: user.ID,
		IsRead: pgtype.Bool{Bool: true, Valid: true},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(readNotifs), 3)
	for _, n := range readNotifs {
		require.True(t, n.IsRead)
	}

	// 查询未读通知
	unreadNotifs, err := testStore.ListUserNotifications(context.Background(), ListUserNotificationsParams{
		UserID: user.ID,
		IsRead: pgtype.Bool{Bool: false, Valid: true},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(unreadNotifs), 2)
	for _, n := range unreadNotifs {
		require.False(t, n.IsRead)
	}
}
