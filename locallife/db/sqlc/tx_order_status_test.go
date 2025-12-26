package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// ==================== UpdateOrderStatusTx Transaction Tests ====================

func TestUpdateOrderStatusTx(t *testing.T) {
	// 准备测试数据
	order := createRandomOrder(t)
	require.Equal(t, "pending", order.Status)

	arg := UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "paid",
		OldStatus:    "pending",
		OperatorID:   order.UserID,
		OperatorType: "system",
		Notes:        "用户完成支付",
	}

	// 执行事务
	result, err := testStore.UpdateOrderStatusTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单状态已更新
	require.Equal(t, order.ID, result.Order.ID)
	require.Equal(t, "paid", result.Order.Status)

	// 验证状态日志已创建
	require.NotZero(t, result.StatusLog.ID)
	require.Equal(t, order.ID, result.StatusLog.OrderID)
	require.True(t, result.StatusLog.FromStatus.Valid)
	require.Equal(t, "pending", result.StatusLog.FromStatus.String)
	require.Equal(t, "paid", result.StatusLog.ToStatus)
	require.True(t, result.StatusLog.OperatorID.Valid)
	require.Equal(t, order.UserID, result.StatusLog.OperatorID.Int64)
	require.True(t, result.StatusLog.OperatorType.Valid)
	require.Equal(t, "system", result.StatusLog.OperatorType.String)
	require.True(t, result.StatusLog.Notes.Valid)
	require.Equal(t, "用户完成支付", result.StatusLog.Notes.String)

	// 验证数据库中的订单
	dbOrder, err := testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", dbOrder.Status)
}

func TestUpdateOrderStatusTx_MerchantAccept(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "paid")

	arg := UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "preparing",
		OldStatus:    "paid",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
		Notes:        "商户接单",
	}

	result, err := testStore.UpdateOrderStatusTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "preparing", result.Order.Status)
	require.Equal(t, "merchant", result.StatusLog.OperatorType.String)
}

func TestUpdateOrderStatusTx_MarkReady(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "preparing")

	arg := UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "ready",
		OldStatus:    "preparing",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
		Notes:        "出餐完成",
	}

	result, err := testStore.UpdateOrderStatusTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "ready", result.Order.Status)
}

func TestUpdateOrderStatusTx_StartDelivery(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "ready")

	// 系统自动派单开始配送（数据库约束 operator_type 只允许 user/merchant/system）
	arg := UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "delivering",
		OldStatus:    "ready",
		OperatorID:   user.ID, // 用于记录关联用户
		OperatorType: "system",
		Notes:        "系统派单，开始配送",
	}

	result, err := testStore.UpdateOrderStatusTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "delivering", result.Order.Status)
	require.Equal(t, "system", result.StatusLog.OperatorType.String)
}

func TestUpdateOrderStatusTx_WithoutNotes(t *testing.T) {
	order := createRandomOrder(t)

	arg := UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "paid",
		OldStatus:    "pending",
		OperatorID:   order.UserID,
		OperatorType: "system",
		Notes:        "", // 无备注
	}

	result, err := testStore.UpdateOrderStatusTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "paid", result.Order.Status)
	require.False(t, result.StatusLog.Notes.Valid) // Notes 应该为空
}

// ==================== CompleteOrderTx Transaction Tests ====================

func TestCompleteOrderTx(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建一个 ready 状态的订单（堂食/打包自取）
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "ready")

	arg := CompleteOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "ready",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	}

	result, err := testStore.CompleteOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单已完成
	require.Equal(t, order.ID, result.Order.ID)
	require.Equal(t, "completed", result.Order.Status)
	require.True(t, result.Order.CompletedAt.Valid)

	// 验证状态日志
	require.NotZero(t, result.StatusLog.ID)
	require.Equal(t, order.ID, result.StatusLog.OrderID)
	require.Equal(t, "ready", result.StatusLog.FromStatus.String)
	require.Equal(t, "completed", result.StatusLog.ToStatus)
	require.Equal(t, merchantOwner.ID, result.StatusLog.OperatorID.Int64)
	require.Equal(t, "merchant", result.StatusLog.OperatorType.String)

	// 验证数据库
	dbOrder, err := testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", dbOrder.Status)
	require.True(t, dbOrder.CompletedAt.Valid)
}

func TestCompleteOrderTx_ByUser(t *testing.T) {
	// 用户确认收货（外卖订单）
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "delivering")

	// 先将状态改为 delivering（模拟配送中）
	_, err := testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:     order.ID,
		Status: "delivering",
	})
	require.NoError(t, err)

	// 获取更新后的订单
	order, err = testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)

	arg := CompleteOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "delivering",
		OperatorID:   user.ID,
		OperatorType: "user",
	}

	result, err := testStore.CompleteOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "completed", result.Order.Status)
	require.Equal(t, "user", result.StatusLog.OperatorType.String)
}

// ==================== CancelOrderTx Transaction Tests ====================

func TestCancelOrderTx(t *testing.T) {
	order := createRandomOrder(t)
	require.Equal(t, "pending", order.Status)

	arg := CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "pending",
		CancelReason: "用户主动取消",
		OperatorID:   order.UserID,
		OperatorType: "user",
	}

	result, err := testStore.CancelOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单已取消
	require.Equal(t, order.ID, result.Order.ID)
	require.Equal(t, "cancelled", result.Order.Status)
	require.True(t, result.Order.CancelledAt.Valid)
	require.True(t, result.Order.CancelReason.Valid)
	require.Equal(t, "用户主动取消", result.Order.CancelReason.String)

	// 验证状态日志
	require.NotZero(t, result.StatusLog.ID)
	require.Equal(t, order.ID, result.StatusLog.OrderID)
	require.Equal(t, "pending", result.StatusLog.FromStatus.String)
	require.Equal(t, "cancelled", result.StatusLog.ToStatus)
	require.Equal(t, order.UserID, result.StatusLog.OperatorID.Int64)
	require.Equal(t, "user", result.StatusLog.OperatorType.String)
	require.True(t, result.StatusLog.Notes.Valid)
	require.Equal(t, "用户主动取消", result.StatusLog.Notes.String)

	// 验证数据库
	dbOrder, err := testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", dbOrder.Status)
	require.True(t, dbOrder.CancelledAt.Valid)
}

func TestCancelOrderTx_PaidOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "paid")

	arg := CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "paid",
		CancelReason: "商户未接单，用户取消",
		OperatorID:   user.ID,
		OperatorType: "user",
	}

	result, err := testStore.CancelOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "cancelled", result.Order.Status)
	require.Equal(t, "商户未接单，用户取消", result.Order.CancelReason.String)
}

func TestCancelOrderTx_ByMerchant(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "paid")

	arg := CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "paid",
		CancelReason: "商户拒单：菜品售罄",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	}

	result, err := testStore.CancelOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "cancelled", result.Order.Status)
	require.Equal(t, "商户拒单：菜品售罄", result.Order.CancelReason.String)
	require.Equal(t, "merchant", result.StatusLog.OperatorType.String)
}

func TestCancelOrderTx_WithoutReason(t *testing.T) {
	order := createRandomOrder(t)

	arg := CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "pending",
		CancelReason: "", // 无取消原因
		OperatorID:   order.UserID,
		OperatorType: "user",
	}

	result, err := testStore.CancelOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "cancelled", result.Order.Status)
	require.False(t, result.Order.CancelReason.Valid) // CancelReason 应该为空
}

func TestCancelOrderTx_BySystem(t *testing.T) {
	// 系统自动取消（如超时未支付）
	order := createRandomOrder(t)

	// 注意：operator_id 有外键约束，必须是有效的用户ID
	// 系统操作时使用订单的用户ID作为关联
	arg := CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "pending",
		CancelReason: "订单超时未支付，系统自动取消",
		OperatorID:   order.UserID, // 关联订单用户
		OperatorType: "system",
	}

	result, err := testStore.CancelOrderTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Equal(t, "cancelled", result.Order.Status)
	require.Equal(t, "system", result.StatusLog.OperatorType.String)
}

// ==================== Order Lifecycle Integration Tests ====================

func TestOrderLifecycle_Takeaway(t *testing.T) {
	// 测试打包自取订单的完整生命周期: pending -> paid -> preparing -> ready -> completed
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 1. 创建订单 (pending)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")
	require.Equal(t, "pending", order.Status)

	// 2. 用户支付 (pending -> paid)
	result1, err := testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "paid",
		OldStatus:    "pending",
		OperatorID:   user.ID,
		OperatorType: "system",
		Notes:        "微信支付成功",
	})
	require.NoError(t, err)
	require.Equal(t, "paid", result1.Order.Status)

	// 3. 商户接单 (paid -> preparing)
	result2, err := testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "preparing",
		OldStatus:    "paid",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
		Notes:        "商户已接单",
	})
	require.NoError(t, err)
	require.Equal(t, "preparing", result2.Order.Status)

	// 4. 出餐完成 (preparing -> ready)
	result3, err := testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "ready",
		OldStatus:    "preparing",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
		Notes:        "出餐完成，请取餐",
	})
	require.NoError(t, err)
	require.Equal(t, "ready", result3.Order.Status)

	// 5. 完成订单 (ready -> completed)
	result4, err := testStore.CompleteOrderTx(context.Background(), CompleteOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "ready",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, "completed", result4.Order.Status)
	require.True(t, result4.Order.CompletedAt.Valid)

	// 验证状态日志数量
	logs, err := testStore.ListOrderStatusLogs(context.Background(), order.ID)
	require.NoError(t, err)
	require.Len(t, logs, 4) // 4 次状态变更
}

func TestOrderLifecycle_Takeout(t *testing.T) {
	// 测试外卖订单的完整生命周期: pending -> paid -> preparing -> ready -> delivering -> completed
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 1. 创建订单 (pending)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")

	// 2. 用户支付 (pending -> paid)
	_, err := testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "paid",
		OldStatus:    "pending",
		OperatorID:   user.ID,
		OperatorType: "system",
	})
	require.NoError(t, err)

	// 3. 商户接单 (paid -> preparing)
	_, err = testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "preparing",
		OldStatus:    "paid",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	})
	require.NoError(t, err)

	// 4. 出餐完成 (preparing -> ready)
	_, err = testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "ready",
		OldStatus:    "preparing",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	})
	require.NoError(t, err)

	// 5. 系统派单开始配送 (ready -> delivering)
	// 数据库约束 operator_type 只允许 user/merchant/system
	_, err = testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "delivering",
		OldStatus:    "ready",
		OperatorID:   user.ID,
		OperatorType: "system",
		Notes:        "系统派单，开始配送",
	})
	require.NoError(t, err)

	// 6. 用户确认收货 (delivering -> completed)
	result, err := testStore.CompleteOrderTx(context.Background(), CompleteOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "delivering",
		OperatorID:   user.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)
	require.Equal(t, "completed", result.Order.Status)

	// 验证状态日志数量
	logs, err := testStore.ListOrderStatusLogs(context.Background(), order.ID)
	require.NoError(t, err)
	require.Len(t, logs, 5) // 5 次状态变更
}

func TestOrderLifecycle_CancelBeforePayment(t *testing.T) {
	// 测试支付前取消: pending -> cancelled
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")

	result, err := testStore.CancelOrderTx(context.Background(), CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "pending",
		CancelReason: "用户取消",
		OperatorID:   user.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Order.Status)

	// 验证状态日志
	logs, err := testStore.ListOrderStatusLogs(context.Background(), order.ID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
}

func TestOrderLifecycle_CancelAfterPayment(t *testing.T) {
	// 测试支付后取消: pending -> paid -> cancelled
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")

	// 支付
	_, err := testStore.UpdateOrderStatusTx(context.Background(), UpdateOrderStatusTxParams{
		OrderID:      order.ID,
		NewStatus:    "paid",
		OldStatus:    "pending",
		OperatorID:   user.ID,
		OperatorType: "system",
	})
	require.NoError(t, err)

	// 取消（此时应触发退款，但这里只测试事务层）
	result, err := testStore.CancelOrderTx(context.Background(), CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "paid",
		CancelReason: "商户未接单，用户申请取消",
		OperatorID:   user.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Order.Status)

	// 验证状态日志
	logs, err := testStore.ListOrderStatusLogs(context.Background(), order.ID)
	require.NoError(t, err)
	require.Len(t, logs, 2) // paid + cancelled
}

func TestOrderLifecycle_MerchantReject(t *testing.T) {
	// 测试商户拒单: pending -> paid -> cancelled
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, "paid")

	result, err := testStore.CancelOrderTx(context.Background(), CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    "paid",
		CancelReason: "商户拒单：今日已打烊",
		OperatorID:   merchantOwner.ID,
		OperatorType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Order.Status)
	require.Equal(t, "商户拒单：今日已打烊", result.Order.CancelReason.String)
}
