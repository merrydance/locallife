package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomReservation 创建一个随机的预定
func createRandomReservation(t *testing.T, userID, merchantID, tableID int64, status string) TableReservation {
	tomorrow := time.Now().Add(24 * time.Hour)

	arg := CreateTableReservationParams{
		TableID:         tableID,
		UserID:          userID,
		MerchantID:      merchantID,
		ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true}, // 18:00
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "deposit",
		DepositAmount:   10000, // 100元
		PrepaidAmount:   0,
		RefundDeadline:  tomorrow.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          status,
	}

	reservation, err := testStore.CreateTableReservation(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, reservation.ID)
	require.Equal(t, status, reservation.Status)

	return reservation
}

// ==================== CreateReservationTx Tests ====================

func TestCreateReservationTx_DepositMode(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	tomorrow := time.Now().Add(24 * time.Hour)

	// 创建押金模式预定（无菜品）
	arg := CreateReservationTxParams{
		CreateTableReservationParams: CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          user.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "张三",
			ContactPhone:    "13800138000",
			PaymentMode:     "deposit",
			DepositAmount:   10000,
			PrepaidAmount:   0,
			RefundDeadline:  tomorrow.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Status:          "pending",
		},
		Items: nil, // 押金模式无菜品
	}

	// 执行事务
	result, err := testStore.CreateReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定创建成功
	require.NotZero(t, result.Reservation.ID)
	require.Equal(t, room.ID, result.Reservation.TableID)
	require.Equal(t, user.ID, result.Reservation.UserID)
	require.Equal(t, merchant.ID, result.Reservation.MerchantID)
	require.Equal(t, "deposit", result.Reservation.PaymentMode)
	require.Equal(t, "pending", result.Reservation.Status)

	// 验证无菜品明细
	require.Empty(t, result.Items)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), result.Reservation.ID)
	require.NoError(t, err)
	require.Equal(t, result.Reservation.ID, dbReservation.ID)
}

func TestCreateReservationTx_FullPaymentWithItems(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	tomorrow := time.Now().Add(24 * time.Hour)

	// 创建全款模式预定（带菜品）
	dishID1 := dish1.ID
	dishID2 := dish2.ID
	arg := CreateReservationTxParams{
		CreateTableReservationParams: CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          user.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 19 * 3600 * 1000000, Valid: true}, // 19:00
			GuestCount:      6,
			ContactName:     "李四",
			ContactPhone:    "13900139000",
			PaymentMode:     "full",
			DepositAmount:   0,
			PrepaidAmount:   50000, // 500元
			RefundDeadline:  tomorrow.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Status:          "pending",
		},
		Items: []ReservationItemInput{
			{DishID: &dishID1, Quantity: 2, UnitPrice: 8800},  // 88元 x 2
			{DishID: &dishID2, Quantity: 1, UnitPrice: 12800}, // 128元 x 1
		},
	}

	// 执行事务
	result, err := testStore.CreateReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定创建成功
	require.NotZero(t, result.Reservation.ID)
	require.Equal(t, "full", result.Reservation.PaymentMode)
	require.Equal(t, int64(50000), result.Reservation.PrepaidAmount)

	// 验证菜品明细创建成功
	require.Len(t, result.Items, 2)

	// 验证第一个菜品
	require.Equal(t, result.Reservation.ID, result.Items[0].ReservationID)
	require.True(t, result.Items[0].DishID.Valid)
	require.Equal(t, dish1.ID, result.Items[0].DishID.Int64)
	require.Equal(t, int16(2), result.Items[0].Quantity)
	require.Equal(t, int64(8800), result.Items[0].UnitPrice)
	require.Equal(t, int64(17600), result.Items[0].TotalPrice) // 8800 * 2

	// 验证第二个菜品
	require.Equal(t, dish2.ID, result.Items[1].DishID.Int64)
	require.Equal(t, int16(1), result.Items[1].Quantity)
	require.Equal(t, int64(12800), result.Items[1].UnitPrice)
	require.Equal(t, int64(12800), result.Items[1].TotalPrice) // 12800 * 1

	// 从数据库验证菜品
	dbItems, err := testStore.GetReservationItemsByReservation(context.Background(), result.Reservation.ID)
	require.NoError(t, err)
	require.Len(t, dbItems, 2)
}

// ==================== ConfirmReservationTx Tests ====================

func TestConfirmReservationTx_Success(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建已支付的预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	// 更新为已支付状态
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	// 执行确认事务
	arg := ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	}

	result, err := testStore.ConfirmReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定状态已更新为confirmed
	require.Equal(t, reservation.ID, result.Reservation.ID)
	require.Equal(t, "confirmed", result.Reservation.Status)
	require.True(t, result.Reservation.ConfirmedAt.Valid)

	// 验证桌台状态已更新为reserved，并关联当前预定
	require.Equal(t, room.ID, result.Table.ID)
	require.Equal(t, "reserved", result.Table.Status)
	require.True(t, result.Table.CurrentReservationID.Valid)
	require.Equal(t, reservation.ID, result.Table.CurrentReservationID.Int64)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, "confirmed", dbReservation.Status)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "reserved", dbTable.Status)
	require.Equal(t, reservation.ID, dbTable.CurrentReservationID.Int64)
}

// ==================== CompleteReservationTx Tests ====================

func TestCompleteReservationTx_Success(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建并确认预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	// 确认预定
	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "confirmed", confirmResult.Reservation.Status)

	// 执行完成事务
	arg := CompleteReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	}

	result, err := testStore.CompleteReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定状态已更新为completed
	require.Equal(t, reservation.ID, result.Reservation.ID)
	require.Equal(t, "completed", result.Reservation.Status)
	require.True(t, result.Reservation.CompletedAt.Valid)

	// 验证桌台已释放
	require.True(t, result.TableUpdated)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", dbReservation.Status)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
	require.False(t, dbTable.CurrentReservationID.Valid) // 已清空
}

func TestCompleteReservationTx_TableNotReleased(t *testing.T) {
	// 场景：桌台当前预定不是要完成的预定，不应释放桌台
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建并确认预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)
	_, err = testStore.UpdateReservationToConfirmed(context.Background(), reservation.ID)
	require.NoError(t, err)

	// 执行完成事务，但传递不匹配的CurrentReservationID
	arg := CompleteReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: 99999, Valid: true}, // 不匹配
	}

	result, err := testStore.CompleteReservationTx(context.Background(), arg)
	require.NoError(t, err)

	// 预定状态应该更新
	require.Equal(t, "completed", result.Reservation.Status)

	// 但桌台不应该被释放
	require.False(t, result.TableUpdated)
}

// ==================== CancelReservationTx Tests ====================

func TestCancelReservationTx_Success(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")

	// 执行取消事务
	arg := CancelReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CancelReason:         "用户临时有事",
		CurrentReservationID: pgtype.Int8{Valid: false},
	}

	result, err := testStore.CancelReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定状态已更新为cancelled
	require.Equal(t, reservation.ID, result.Reservation.ID)
	require.Equal(t, "cancelled", result.Reservation.Status)
	require.True(t, result.Reservation.CancelledAt.Valid)
	require.True(t, result.Reservation.CancelReason.Valid)
	require.Equal(t, "用户临时有事", result.Reservation.CancelReason.String)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", dbReservation.Status)
}

func TestCancelReservationTx_WithTableRelease(t *testing.T) {
	// 场景：取消已确认的预定，需要释放桌台
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建并确认预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	// 确认预定（会锁定桌台）
	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "reserved", confirmResult.Table.Status)

	// 执行取消事务
	arg := CancelReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CancelReason:         "商户取消",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	}

	result, err := testStore.CancelReservationTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定已取消
	require.Equal(t, "cancelled", result.Reservation.Status)

	// 验证桌台已释放
	require.True(t, result.TableUpdated)

	// 从数据库验证
	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
	require.False(t, dbTable.CurrentReservationID.Valid)
}

// ==================== MarkNoShowTx Tests ====================

func TestMarkNoShowTx_Success(t *testing.T) {
	// 准备测试数据
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建并确认预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	// 确认预定
	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "reserved", confirmResult.Table.Status)

	// 执行标记未到店事务
	arg := MarkNoShowTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	}

	result, err := testStore.MarkNoShowTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证预定状态已更新为no_show
	require.Equal(t, reservation.ID, result.Reservation.ID)
	require.Equal(t, "no_show", result.Reservation.Status)

	// 验证桌台已释放
	require.True(t, result.TableUpdated)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, "no_show", dbReservation.Status)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
}

// ==================== Complete Business Flow Test ====================

func TestReservationCompleteFlow(t *testing.T) {
	// 完整业务流程测试：创建 → 支付 → 确认 → 完成
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	tomorrow := time.Now().Add(24 * time.Hour)
	dishID := dish.ID

	// Step 1: 创建预定（全款模式带菜品）
	createArg := CreateReservationTxParams{
		CreateTableReservationParams: CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          user.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "完整流程测试",
			ContactPhone:    "13800000000",
			PaymentMode:     "full",
			DepositAmount:   0,
			PrepaidAmount:   30000,
			RefundDeadline:  tomorrow.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Status:          "pending",
		},
		Items: []ReservationItemInput{
			{DishID: &dishID, Quantity: 3, UnitPrice: 10000},
		},
	}

	createResult, err := testStore.CreateReservationTx(context.Background(), createArg)
	require.NoError(t, err)
	require.Equal(t, "pending", createResult.Reservation.Status)
	require.Len(t, createResult.Items, 1)
	reservationID := createResult.Reservation.ID

	// 验证初始状态
	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status) // 未确认前桌台仍可用

	// Step 2: 支付
	paidReservation, err := testStore.UpdateReservationToPaid(context.Background(), reservationID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidReservation.Status)

	// Step 3: 商户确认
	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservationID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "confirmed", confirmResult.Reservation.Status)
	require.Equal(t, "reserved", confirmResult.Table.Status)
	require.Equal(t, reservationID, confirmResult.Table.CurrentReservationID.Int64)

	// Step 4: 完成消费
	completeResult, err := testStore.CompleteReservationTx(context.Background(), CompleteReservationTxParams{
		ReservationID:        reservationID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", completeResult.Reservation.Status)
	require.True(t, completeResult.TableUpdated)

	// 最终验证：桌台已释放
	dbTableFinal, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTableFinal.Status)
	require.False(t, dbTableFinal.CurrentReservationID.Valid)

	// 最终验证：预定记录完整
	dbReservationFinal, err := testStore.GetTableReservation(context.Background(), reservationID)
	require.NoError(t, err)
	require.Equal(t, "completed", dbReservationFinal.Status)
	require.True(t, dbReservationFinal.PaidAt.Valid)
	require.True(t, dbReservationFinal.ConfirmedAt.Valid)
	require.True(t, dbReservationFinal.CompletedAt.Valid)

	// 验证菜品明细仍存在
	items, err := testStore.GetReservationItemsByReservation(context.Background(), reservationID)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestReservationCancelFlow(t *testing.T) {
	// 取消流程测试：创建 → 支付 → 确认 → 取消
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// Step 1: 创建预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")

	// Step 2: 支付
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	// Step 3: 确认
	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "reserved", confirmResult.Table.Status)

	// Step 4: 取消
	cancelResult, err := testStore.CancelReservationTx(context.Background(), CancelReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CancelReason:         "用户取消",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelResult.Reservation.Status)
	require.True(t, cancelResult.TableUpdated)

	// 验证桌台已恢复可用
	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
}

// ==================== Cross-Table Consistency Test ====================

func TestReservationTableConsistency(t *testing.T) {
	// 跨表一致性测试：预定状态与桌台状态必须保持一致
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 创建并确认预定
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	confirmResult, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)

	// 验证一致性：预定confirmed时，桌台必须是reserved且关联此预定
	require.Equal(t, "confirmed", confirmResult.Reservation.Status)
	require.Equal(t, "reserved", confirmResult.Table.Status)
	require.Equal(t, reservation.ID, confirmResult.Table.CurrentReservationID.Int64)

	// 完成预定
	completeResult, err := testStore.CompleteReservationTx(context.Background(), CompleteReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

	// 验证一致性：预定completed时，桌台必须释放
	require.Equal(t, "completed", completeResult.Reservation.Status)
	require.True(t, completeResult.TableUpdated)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
	require.False(t, dbTable.CurrentReservationID.Valid)
}
