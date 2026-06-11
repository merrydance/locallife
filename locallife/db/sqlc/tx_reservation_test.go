package db

import (
	"context"
	"encoding/json"
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

func createActiveReservationAdjustmentForTxTest(t *testing.T, reservation TableReservation) ReservationAdjustment {
	adjustment, err := testStore.CreateReservationAdjustment(context.Background(), CreateReservationAdjustmentParams{
		ReservationID: reservation.ID,
		UserID:        reservation.UserID,
		MerchantID:    reservation.MerchantID,
		Direction:     ReservationAdjustmentDirectionPositive,
		Status:        ReservationAdjustmentStatusCreatingPayment,
		CurrentTotal:  1000,
		TargetTotal:   2000,
		DeltaAmount:   1000,
	})
	require.NoError(t, err)
	return adjustment
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

func TestCreateReservationTxRejectsDisabledTableAfterLock(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	_, err := testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:     room.ID,
		Status: TableStatusDisabled,
	})
	require.NoError(t, err)

	tomorrow := time.Now().Add(24 * time.Hour)
	_, err = testStore.CreateReservationTx(context.Background(), CreateReservationTxParams{
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
	})
	require.ErrorIs(t, err, ErrTableDisabledForReservation)

	reservations, listErr := testStore.ListReservationsByTable(context.Background(), ListReservationsByTableParams{
		TableID: room.ID,
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, listErr)
	require.Empty(t, reservations)
}

func TestCreateMerchantReservationTxRejectsDisabledTableAfterLock(t *testing.T) {
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	_, err := testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:     room.ID,
		Status: TableStatusDisabled,
	})
	require.NoError(t, err)

	tomorrow := time.Now().Add(24 * time.Hour)
	_, err = testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         room.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 19 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "李四",
			ContactPhone:    "13900139000",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Source:          pgtype.Text{String: "merchant", Valid: true},
		},
	})
	require.ErrorIs(t, err, ErrTableDisabledForReservation)

	reservations, listErr := testStore.ListReservationsByTable(context.Background(), ListReservationsByTableParams{
		TableID: room.ID,
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, listErr)
	require.Empty(t, reservations)
}

func TestUpdateReservationTxRejectsDisabledTargetTableAfterLock(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	originalRoom := createRandomRoom(t, merchant.ID)
	targetRoom := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, originalRoom.ID, "confirmed")

	_, err := testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:     targetRoom.ID,
		Status: TableStatusDisabled,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:      reservation.ID,
			TableID: pgtype.Int8{Int64: targetRoom.ID, Valid: true},
		},
	})
	require.ErrorIs(t, err, ErrTableDisabledForReservation)

	after, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Equal(t, originalRoom.ID, after.TableID)
}

func TestUpdateReservationTxRejectsGuestCountExceedsLockedCapacity(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	room, err := testStore.UpdateTable(context.Background(), UpdateTableParams{
		ID:       room.ID,
		Capacity: pgtype.Int2{Int16: 4, Valid: true},
	})
	require.NoError(t, err)
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "confirmed")

	_, err = testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:         reservation.ID,
			GuestCount: pgtype.Int2{Int16: room.Capacity + 1, Valid: true},
		},
	})
	require.ErrorIs(t, err, ErrReservationGuestCountExceedsCapacity)

	after, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Equal(t, reservation.GuestCount, after.GuestCount)
}

func TestUpdateReservationTxRejectsTimeConflictAfterLock(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	room, err := testStore.UpdateTable(context.Background(), UpdateTableParams{
		ID:           room.ID,
		MinimumSpend: pgtype.Int8{Int64: 0, Valid: true},
	})
	require.NoError(t, err)
	tomorrow := time.Now().Add(24 * time.Hour)

	existing := createRandomReservation(t, user.ID, merchant.ID, room.ID, ReservationStatusConfirmed)
	existing, err = testStore.UpdateReservation(context.Background(), UpdateReservationParams{
		ID:              existing.ID,
		ReservationDate: pgtype.Date{Time: tomorrow, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
	})
	require.NoError(t, err)

	moving := createRandomReservation(t, user.ID, merchant.ID, room.ID, ReservationStatusConfirmed)
	moving, err = testStore.UpdateReservation(context.Background(), UpdateReservationParams{
		ID:              moving.ID,
		ReservationDate: pgtype.Date{Time: tomorrow.Add(24 * time.Hour), Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 12 * 3600 * 1000000, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:              moving.ID,
			ReservationDate: existing.ReservationDate,
			ReservationTime: existing.ReservationTime,
		},
	})
	require.ErrorIs(t, err, ErrReservationTimeConflict)

	after, getErr := testStore.GetTableReservation(context.Background(), moving.ID)
	require.NoError(t, getErr)
	require.Equal(t, moving.ReservationDate, after.ReservationDate)
	require.Equal(t, moving.ReservationTime, after.ReservationTime)
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
			PrepaidAmount:   100800, // 1008元，满足包间最低消费
			RefundDeadline:  tomorrow.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Status:          "pending",
		},
		Items: []ReservationItemInput{
			{DishID: &dishID1, Quantity: 2, UnitPrice: 44000}, // 440元 x 2
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
	require.Equal(t, int64(100800), result.Reservation.PrepaidAmount)

	// 验证菜品明细创建成功
	require.Len(t, result.Items, 2)

	// 验证第一个菜品
	require.Equal(t, result.Reservation.ID, result.Items[0].ReservationID)
	require.True(t, result.Items[0].DishID.Valid)
	require.Equal(t, dish1.ID, result.Items[0].DishID.Int64)
	require.Equal(t, int16(2), result.Items[0].Quantity)
	require.Equal(t, int64(44000), result.Items[0].UnitPrice)
	require.Equal(t, int64(88000), result.Items[0].TotalPrice) // 44000 * 2

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

	// 验证确认预定不会提前占用桌台
	require.Equal(t, room.ID, result.Table.ID)
	require.Equal(t, "available", result.Table.Status)
	require.False(t, result.Table.CurrentReservationID.Valid)

	// 从数据库验证
	dbReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, "confirmed", dbReservation.Status)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
	require.False(t, dbTable.CurrentReservationID.Valid)
}

func TestConfirmReservationTx_NearReservationDoesNotOccupyTable(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	nearTime := time.Now().Add(20 * time.Minute)
	reservation, err := testStore.CreateTableReservation(context.Background(), CreateTableReservationParams{
		TableID:         room.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: nearTime, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: int64(nearTime.Hour()*3600+nearTime.Minute()*60) * 1000000, Valid: true},
		GuestCount:      2,
		ContactName:     "近时段确认",
		ContactPhone:    "13800138000",
		PaymentMode:     "deposit",
		DepositAmount:   10000,
		PrepaidAmount:   0,
		RefundDeadline:  nearTime.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(10 * time.Minute),
		Status:          "pending",
	})
	require.NoError(t, err)

	_, err = testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)

	result, err := testStore.ConfirmReservationTx(context.Background(), ConfirmReservationTxParams{
		ReservationID: reservation.ID,
		TableID:       room.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "confirmed", result.Reservation.Status)
	require.Equal(t, "available", result.Table.Status)
	require.False(t, result.Table.CurrentReservationID.Valid)

	dbTable, err := testStore.GetTable(context.Background(), room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", dbTable.Status)
	require.False(t, dbTable.CurrentReservationID.Valid)
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

	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "occupied",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

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

func TestCompleteReservationTxRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)
	_, err = testStore.UpdateReservationToConfirmed(context.Background(), reservation.ID)
	require.NoError(t, err)
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err = testStore.CompleteReservationTx(context.Background(), CompleteReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, 409, statusCode)

	current, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Equal(t, "confirmed", current.Status)
	require.False(t, current.CompletedAt.Valid)
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
	require.Equal(t, "available", confirmResult.Table.Status)

	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "reserved",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

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

func TestCancelReservationTxRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err := testStore.CancelReservationTx(context.Background(), CancelReservationTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CancelReason:         "cancel while adjustment pending",
		CurrentReservationID: pgtype.Int8{Valid: false},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, 409, statusCode)

	current, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Equal(t, reservation.Status, current.Status)
	require.False(t, current.CancelledAt.Valid)
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
	require.Equal(t, "available", confirmResult.Table.Status)

	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "reserved",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

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

	sqlStore := testStore.(*SQLStore)
	var decisionUserID pgtype.Int8
	var factSnapshot []byte
	err = sqlStore.connPool.QueryRow(context.Background(), `
		SELECT user_id, fact_snapshot
		FROM behavior_decisions
		WHERE reservation_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, reservation.ID).Scan(&decisionUserID, &factSnapshot)
	require.NoError(t, err)
	require.True(t, decisionUserID.Valid)
	require.Equal(t, user.ID, decisionUserID.Int64)
	require.Empty(t, factSnapshot)
}

func TestMarkNoShowTxRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err = testStore.MarkNoShowTx(context.Background(), MarkNoShowTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, 409, statusCode)

	current, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Equal(t, "paid", current.Status)
}

func TestMarkNoShowTxDoesNotAttributeMerchantCreatedReservationToOperator(t *testing.T) {
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	reservation, err := testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         room.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "线下顾客",
			ContactPhone:    "13800138000",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Notes:           pgtype.Text{String: "电话预约", Valid: true},
			Source:          pgtype.Text{String: "phone", Valid: true},
		},
	})
	require.NoError(t, err)
	require.Equal(t, operator.ID, reservation.UserID)

	_, err = testStore.MarkNoShowTx(context.Background(), MarkNoShowTxParams{
		ReservationID:        reservation.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	sqlStore := testStore.(*SQLStore)
	var decisionUserID pgtype.Int8
	var factSnapshot []byte
	err = sqlStore.connPool.QueryRow(context.Background(), `
		SELECT user_id, fact_snapshot
		FROM behavior_decisions
		WHERE reservation_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, reservation.ID).Scan(&decisionUserID, &factSnapshot)
	require.NoError(t, err)
	require.False(t, decisionUserID.Valid, "offline/phone no-show must not punish the operator account")

	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(factSnapshot, &snapshot))
	require.Equal(t, "offline_customer", snapshot["customer_identity_type"])
	require.NotEmpty(t, snapshot["offline_customer_id"])
	require.Equal(t, float64(operator.ID), snapshot["created_by_user_id"])
	require.Equal(t, "phone", snapshot["reservation_source"])
}

func TestCountReservationsByUserAndStatusExcludesOfflineOperatorReservations(t *testing.T) {
	user := createRandomUser(t)
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	onlineReservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, ReservationStatusConfirmed)
	legacyBlankSourceReservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, ReservationStatusConfirmed)
	legacyWhitespaceOnlineReservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, ReservationStatusConfirmed)
	sqlStore := testStore.(*SQLStore)
	_, err := sqlStore.connPool.Exec(context.Background(), `
		UPDATE table_reservations
		SET source = $2
		WHERE id = $1
	`, legacyBlankSourceReservation.ID, "   ")
	require.NoError(t, err)
	_, err = sqlStore.connPool.Exec(context.Background(), `
		UPDATE table_reservations
		SET source = $2
		WHERE id = $1
	`, legacyWhitespaceOnlineReservation.ID, " online ")
	require.NoError(t, err)

	_, err = testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         room.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 20 * 3600 * 1000000, Valid: true},
			GuestCount:      2,
			ContactName:     "线下顾客",
			ContactPhone:    "13900139001",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Source:          pgtype.Text{String: ReservationSourcePhone, Valid: true},
		},
	})
	require.NoError(t, err)

	userCount, err := testStore.CountReservationsByUserAndStatus(context.Background(), CountReservationsByUserAndStatusParams{
		UserID: user.ID,
		Status: ReservationStatusConfirmed,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), userCount)

	operatorCount, err := testStore.CountReservationsByUserAndStatus(context.Background(), CountReservationsByUserAndStatusParams{
		UserID: operator.ID,
		Status: ReservationStatusConfirmed,
	})
	require.NoError(t, err)
	require.Zero(t, operatorCount)

	listed, err := testStore.ListReservationsByUserWithStatus(context.Background(), ListReservationsByUserWithStatusParams{
		UserID: user.ID,
		Status: pgtype.Text{String: ReservationStatusConfirmed, Valid: true},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, listed, 3)
	listedIDs := []int64{listed[0].ID, listed[1].ID, listed[2].ID}
	require.Contains(t, listedIDs, onlineReservation.ID)
	require.Contains(t, listedIDs, legacyBlankSourceReservation.ID)
	require.Contains(t, listedIDs, legacyWhitespaceOnlineReservation.ID)
}

func TestUpdateReservationTxRepointsOfflineCustomerWhenContactPhoneChanges(t *testing.T) {
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	reservation, err := testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         room.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "旧客人",
			ContactPhone:    "13800138000",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Notes:           pgtype.Text{String: "电话预约", Valid: true},
			Source:          pgtype.Text{String: "phone", Valid: true},
		},
	})
	require.NoError(t, err)
	require.True(t, reservation.OfflineCustomerID.Valid)
	originalOfflineCustomerID := reservation.OfflineCustomerID.Int64

	edited, err := testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:           reservation.ID,
			ContactName:  pgtype.Text{String: "新客人", Valid: true},
			ContactPhone: pgtype.Text{String: "13900139000", Valid: true},
		},
	})
	require.NoError(t, err)
	require.True(t, edited.OfflineCustomerID.Valid)
	require.NotEqual(t, originalOfflineCustomerID, edited.OfflineCustomerID.Int64)

	offlineCustomer, err := testStore.GetMerchantOfflineCustomer(context.Background(), GetMerchantOfflineCustomerParams{
		ID:         edited.OfflineCustomerID.Int64,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, merchant.ID, offlineCustomer.MerchantID)
	require.Equal(t, "新客人", offlineCustomer.ContactName)
	require.Equal(t, "13900139000", offlineCustomer.ContactPhone)

	_, err = testStore.MarkNoShowTx(context.Background(), MarkNoShowTxParams{
		ReservationID:        edited.ID,
		TableID:              room.ID,
		CurrentReservationID: pgtype.Int8{},
	})
	require.NoError(t, err)

	sqlStore := testStore.(*SQLStore)
	var factSnapshot []byte
	err = sqlStore.connPool.QueryRow(context.Background(), `
		SELECT fact_snapshot
		FROM behavior_decisions
		WHERE reservation_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, edited.ID).Scan(&factSnapshot)
	require.NoError(t, err)

	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(factSnapshot, &snapshot))
	require.Equal(t, float64(edited.OfflineCustomerID.Int64), snapshot["offline_customer_id"])
}

func TestUpdateReservationTxNormalizesLegacyOfflineSourceWhenContactChanges(t *testing.T) {
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	reservation, err := testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         room.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "旧客人",
			ContactPhone:    "13800138009",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Source:          pgtype.Text{String: ReservationSourcePhone, Valid: true},
		},
	})
	require.NoError(t, err)

	sqlStore := testStore.(*SQLStore)
	_, err = sqlStore.connPool.Exec(context.Background(), `
		UPDATE table_reservations
		SET source = $2
		WHERE id = $1
	`, reservation.ID, " phone ")
	require.NoError(t, err)

	edited, err := testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:           reservation.ID,
			ContactName:  pgtype.Text{String: "新客人", Valid: true},
			ContactPhone: pgtype.Text{String: "13900139009", Valid: true},
		},
	})
	require.NoError(t, err)
	require.True(t, edited.OfflineCustomerID.Valid)

	offlineCustomer, err := testStore.GetMerchantOfflineCustomer(context.Background(), GetMerchantOfflineCustomerParams{
		ID:         edited.OfflineCustomerID.Int64,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, ReservationSourcePhone, offlineCustomer.Source)
	require.Equal(t, "13900139009", offlineCustomer.ContactPhone)

	current, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, " phone ", current.Source.String)
}

func TestUpdateReservationCookingStartedRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "pending")
	_, err := testStore.UpdateReservationToPaid(context.Background(), reservation.ID)
	require.NoError(t, err)
	_, err = testStore.UpdateReservationToConfirmed(context.Background(), reservation.ID)
	require.NoError(t, err)
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err = testStore.UpdateReservationCookingStarted(context.Background(), reservation.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	current, getErr := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.False(t, current.CookingStartedAt.Valid)
}

func TestReplaceReservationItemsTxRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "paid")
	_, err := testStore.CreateReservationItem(context.Background(), CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     1000,
		TotalPrice:    1000,
	})
	require.NoError(t, err)
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err = testStore.ReplaceReservationItemsTx(context.Background(), ReplaceReservationItemsTxParams{
		ReservationID:         reservation.ID,
		ExpectedCurrentAmount: 1000,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      2,
			UnitPrice:     1000,
			TotalPrice:    2000,
		}},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, 409, statusCode)

	items, getErr := testStore.GetReservationItemsByReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Len(t, items, 1)
	require.Equal(t, int16(1), items[0].Quantity)
}

func TestReplaceReservationItemsWithRefundOrdersTxRejectsActiveAdjustment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "paid")
	_, err := testStore.CreateReservationItem(context.Background(), CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      2,
		UnitPrice:     1000,
		TotalPrice:    2000,
	})
	require.NoError(t, err)
	createActiveReservationAdjustmentForTxTest(t, reservation)

	_, err = testStore.ReplaceReservationItemsWithRefundOrdersTx(context.Background(), ReplaceReservationItemsWithRefundOrdersTxParams{
		ReservationID:         reservation.ID,
		ExpectedCurrentAmount: 2000,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      1,
			UnitPrice:     1000,
			TotalPrice:    1000,
		}},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, 409, statusCode)

	items, getErr := testStore.GetReservationItemsByReservation(context.Background(), reservation.ID)
	require.NoError(t, getErr)
	require.Len(t, items, 1)
	require.Equal(t, int16(2), items[0].Quantity)
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
			PrepaidAmount:   120000,
			RefundDeadline:  tomorrow.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Status:          "pending",
		},
		Items: []ReservationItemInput{
			{DishID: &dishID, Quantity: 3, UnitPrice: 40000},
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
	require.Equal(t, "available", confirmResult.Table.Status)
	require.False(t, confirmResult.Table.CurrentReservationID.Valid)

	// Step 3.5: 到店开台后才真正占桌
	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "occupied",
		CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	})
	require.NoError(t, err)

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

func TestMerchantReservationEditOpenAndCloseDiningSessionFlow(t *testing.T) {
	operator := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	originalRoom := createRandomRoom(t, merchant.ID)
	targetRoom := createRandomRoom(t, merchant.ID)
	targetRoom, err := testStore.UpdateTable(context.Background(), UpdateTableParams{
		ID:           targetRoom.ID,
		MinimumSpend: pgtype.Int8{Int64: 0, Valid: true},
	})
	require.NoError(t, err)

	reservationDate := time.Now().Add(24 * time.Hour)
	reservation, err := testStore.CreateMerchantReservationTx(context.Background(), CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: CreateTableReservationByMerchantParams{
			TableID:         originalRoom.ID,
			UserID:          operator.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
			GuestCount:      4,
			ContactName:     "到店客人",
			ContactPhone:    "13800138000",
			PaymentMode:     "deposit",
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  time.Now(),
			PaymentDeadline: time.Now().Add(365 * 24 * time.Hour),
			Notes:           pgtype.Text{String: "电话预约", Valid: true},
			Source:          pgtype.Text{String: "merchant", Valid: true},
		},
	})
	require.NoError(t, err)
	require.Equal(t, ReservationStatusConfirmed, reservation.Status)
	require.True(t, reservation.ConfirmedAt.Valid)

	edited, err := testStore.UpdateReservationTx(context.Background(), UpdateReservationTxParams{
		MerchantID: merchant.ID,
		Reservation: UpdateReservationParams{
			ID:           reservation.ID,
			TableID:      pgtype.Int8{Int64: targetRoom.ID, Valid: true},
			GuestCount:   pgtype.Int2{Int16: 5, Valid: true},
			ContactName:  pgtype.Text{String: "更新后的客人", Valid: true},
			ContactPhone: pgtype.Text{String: "13900139000", Valid: true},
			Notes:        pgtype.Text{String: "改到目标包间", Valid: true},
		},
	})
	require.NoError(t, err)
	require.Equal(t, targetRoom.ID, edited.TableID)
	require.Equal(t, int16(5), edited.GuestCount)
	require.Equal(t, ReservationStatusConfirmed, edited.Status)

	originalRoomAfterEdit, err := testStore.GetTable(context.Background(), originalRoom.ID)
	require.NoError(t, err)
	require.Equal(t, TableStatusAvailable, originalRoomAfterEdit.Status)
	require.False(t, originalRoomAfterEdit.CurrentReservationID.Valid)

	openResult, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                edited.TableID,
		MerchantID:             merchant.ID,
		UserID:                 operator.ID,
		ReservationID:          pgtype.Int8{Int64: edited.ID, Valid: true},
		ImportReservationItems: false,
	})
	require.NoError(t, err)
	require.NotZero(t, openResult.Session.ID)
	require.Equal(t, targetRoom.ID, openResult.Session.TableID)
	require.Equal(t, edited.ID, openResult.Session.ReservationID.Int64)
	require.NotZero(t, openResult.BillingGroup.ID)
	require.Equal(t, openResult.Session.ID, openResult.BillingGroup.DiningSessionID)
	require.True(t, openResult.BillingGroup.IsDefault)

	checkedIn, err := testStore.GetTableReservation(context.Background(), edited.ID)
	require.NoError(t, err)
	require.Equal(t, ReservationStatusCheckedIn, checkedIn.Status)
	require.True(t, checkedIn.CheckedInAt.Valid)

	occupiedRoom, err := testStore.GetTable(context.Background(), targetRoom.ID)
	require.NoError(t, err)
	require.Equal(t, TableStatusOccupied, occupiedRoom.Status)
	require.True(t, occupiedRoom.CurrentReservationID.Valid)
	require.Equal(t, edited.ID, occupiedRoom.CurrentReservationID.Int64)

	closeResult, err := testStore.CloseDiningSessionTx(context.Background(), CloseDiningSessionTxParams{
		ID:         openResult.Session.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "closed", closeResult.Session.Status)

	completed, err := testStore.GetTableReservation(context.Background(), edited.ID)
	require.NoError(t, err)
	require.Equal(t, ReservationStatusCompleted, completed.Status)
	require.True(t, completed.CompletedAt.Valid)

	availableRoom, err := testStore.GetTable(context.Background(), targetRoom.ID)
	require.NoError(t, err)
	require.Equal(t, TableStatusAvailable, availableRoom.Status)
	require.False(t, availableRoom.CurrentReservationID.Valid)

	closedGroup, err := testStore.GetDefaultBillingGroupBySession(context.Background(), openResult.Session.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", closedGroup.Status)
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
	require.Equal(t, "available", confirmResult.Table.Status)

	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "reserved",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

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

	// 验证一致性：预定confirmed时，桌台不应被提前占用
	require.Equal(t, "confirmed", confirmResult.Reservation.Status)
	require.Equal(t, "available", confirmResult.Table.Status)
	require.False(t, confirmResult.Table.CurrentReservationID.Valid)

	_, err = testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   room.ID,
		Status:               "occupied",
		CurrentReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
	})
	require.NoError(t, err)

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
