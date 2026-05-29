package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreateReservationPositiveAdjustmentPaymentTxDoesNotReplaceEffectiveItems(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	_, err := testStore.CreateDailyInventory(ctx, CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: reservationDate, Valid: true},
		TotalQuantity: 10,
		SoldQuantity:  0,
	})
	require.NoError(t, err)

	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)
	_, err = testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price,
	})
	require.NoError(t, err)
	_, err = testStore.SyncReservationInventoryTx(ctx, SyncReservationInventoryTxParams{ReservationID: reservation.ID})
	require.NoError(t, err)

	targetItems := []CreateReservationItemParams{{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      2,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price * 2,
	}}
	result, err := testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: dish.Price,
		TargetTotal:           dish.Price * 2,
		DeltaAmount:           dish.Price,
		Items:                 targetItems,
		OutTradeNo:            "RA" + util.RandomString(20),
		ExpiresAt:             time.Now().Add(30 * time.Minute),
		Attach:                "reservation_id:1;payment_mode:full;addon:true",
	})
	require.NoError(t, err)
	require.Equal(t, ReservationAdjustmentStatusCreatingPayment, result.Adjustment.Status)
	require.Equal(t, result.PaymentOrder.ID, result.Adjustment.PaymentOrderID.Int64)
	require.Len(t, result.Items, 1)
	require.Len(t, result.Holds, 1)

	effectiveItems, err := testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, effectiveItems, 1)
	require.Equal(t, int16(1), effectiveItems[0].Quantity)

	entries, err := testStore.ListReservationInventoryByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, int32(1), entries[0].Quantity)

	inventory, err := testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), inventory.ReservedQuantity)
}

func TestCreateReservationPositiveAdjustmentPaymentTxRejectsTargetItemTotalMismatch(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)
	_, err = testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price,
	})
	require.NoError(t, err)

	_, err = testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: dish.Price,
		TargetTotal:           dish.Price * 2,
		DeltaAmount:           dish.Price,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      1,
			UnitPrice:     dish.Price,
			TotalPrice:    dish.Price,
		}},
		OutTradeNo: "RA" + util.RandomString(20),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Attach:     "reservation_id:1;payment_mode:full;addon:true",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "positive reservation adjustment items total is invalid")

	_, err = testStore.GetActiveReservationAdjustmentByReservation(ctx, reservation.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestApplyPaidReservationAdjustmentTxAppliesTargetOnceAndConvertsInventoryHold(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	_, err := testStore.CreateDailyInventory(ctx, CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: reservationDate, Valid: true},
		TotalQuantity: 10,
		SoldQuantity:  0,
	})
	require.NoError(t, err)

	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)
	_, err = testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price,
	})
	require.NoError(t, err)
	_, err = testStore.SyncReservationInventoryTx(ctx, SyncReservationInventoryTxParams{ReservationID: reservation.ID})
	require.NoError(t, err)

	createResult, err := testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: dish.Price,
		TargetTotal:           dish.Price * 2,
		DeltaAmount:           dish.Price,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      2,
			UnitPrice:     dish.Price,
			TotalPrice:    dish.Price * 2,
		}},
		OutTradeNo: "RA" + util.RandomString(20),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Attach:     "reservation_id:1;payment_mode:full;addon:true",
	})
	require.NoError(t, err)
	_, err = testStore.MarkReservationAdjustmentPendingPayment(ctx, createResult.Adjustment.ID)
	require.NoError(t, err)
	_, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            createResult.PaymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.ApplyPaidReservationAdjustmentTx(ctx, ApplyPaidReservationAdjustmentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, result.Processed)
	require.Equal(t, ReservationAdjustmentStatusApplied, result.Adjustment.Status)

	effectiveItems, err := testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, effectiveItems, 1)
	require.Equal(t, int16(2), effectiveItems[0].Quantity)

	entries, err := testStore.ListReservationInventoryByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, int32(2), entries[0].Quantity)

	inventory, err := testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), inventory.ReservedQuantity)

	secondResult, err := testStore.ApplyPaidReservationAdjustmentTx(ctx, ApplyPaidReservationAdjustmentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
	})
	require.NoError(t, err)
	require.False(t, secondResult.Processed)

	effectiveItems, err = testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, effectiveItems, 1)
	require.Equal(t, int16(2), effectiveItems[0].Quantity)
	inventory, err = testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), inventory.ReservedQuantity)
}

func TestCloseReservationAdjustmentForPaymentTxReleasesHoldAndPreservesEffectiveItems(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	_, err := testStore.CreateDailyInventory(ctx, CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: reservationDate, Valid: true},
		TotalQuantity: 10,
		SoldQuantity:  0,
	})
	require.NoError(t, err)

	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)
	_, err = testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price,
	})
	require.NoError(t, err)
	_, err = testStore.SyncReservationInventoryTx(ctx, SyncReservationInventoryTxParams{ReservationID: reservation.ID})
	require.NoError(t, err)

	createResult, err := testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: dish.Price,
		TargetTotal:           dish.Price * 2,
		DeltaAmount:           dish.Price,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      2,
			UnitPrice:     dish.Price,
			TotalPrice:    dish.Price * 2,
		}},
		OutTradeNo: "RA" + util.RandomString(20),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Attach:     "reservation_id:1;payment_mode:full;addon:true",
	})
	require.NoError(t, err)
	_, err = testStore.MarkReservationAdjustmentPendingPayment(ctx, createResult.Adjustment.ID)
	require.NoError(t, err)

	result, err := testStore.CloseReservationAdjustmentForPaymentTx(ctx, CloseReservationAdjustmentForPaymentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
		Status:         ReservationAdjustmentStatusExpired,
		Reason:         "payment timeout",
	})
	require.NoError(t, err)
	require.True(t, result.Closed)
	require.Equal(t, ReservationAdjustmentStatusExpired, result.Adjustment.Status)

	effectiveItems, err := testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, effectiveItems, 1)
	require.Equal(t, int16(1), effectiveItems[0].Quantity)

	entries, err := testStore.ListReservationInventoryByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, int32(1), entries[0].Quantity)

	holds, err := testStore.ListReservationAdjustmentInventoryHoldsForUpdate(ctx, createResult.Adjustment.ID)
	require.NoError(t, err)
	require.Len(t, holds, 1)
	require.Equal(t, ReservationAdjustmentHoldStatusReleased, holds[0].Status)

	inventory, err := testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), inventory.ReservedQuantity)
}

func TestCloseReservationAdjustmentForPaymentTxReleasesUnlimitedDishHold(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)

	createResult, err := testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: 0,
		TargetTotal:           dish.Price,
		DeltaAmount:           dish.Price,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      1,
			UnitPrice:     dish.Price,
			TotalPrice:    dish.Price,
		}},
		OutTradeNo: "RA" + util.RandomString(20),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Attach:     "reservation_id:1;payment_mode:full;addon:true",
	})
	require.NoError(t, err)
	require.Len(t, createResult.Holds, 1)

	result, err := testStore.CloseReservationAdjustmentForPaymentTx(ctx, CloseReservationAdjustmentForPaymentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
		Status:         ReservationAdjustmentStatusExpired,
		Reason:         "payment timeout",
	})
	require.NoError(t, err)
	require.True(t, result.Closed)
	require.Equal(t, ReservationAdjustmentStatusExpired, result.Adjustment.Status)

	holds, err := testStore.ListReservationAdjustmentInventoryHoldsForUpdate(ctx, createResult.Adjustment.ID)
	require.NoError(t, err)
	require.Len(t, holds, 1)
	require.Equal(t, ReservationAdjustmentHoldStatusReleased, holds[0].Status)

	_, err = testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestCloseReservationAdjustmentForPaymentTxIsIdempotentAfterTerminalStatus(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	reservationDate := time.Now().Add(24 * time.Hour)

	_, err := testStore.CreateDailyInventory(ctx, CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: reservationDate, Valid: true},
		TotalQuantity: 10,
	})
	require.NoError(t, err)
	reservation, err := testStore.CreateTableReservation(ctx, CreateTableReservationParams{
		TableID:         table.ID,
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: 18 * 3600 * 1000000, Valid: true},
		GuestCount:      4,
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "full",
		DepositAmount:   0,
		PrepaidAmount:   dish.Price,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: time.Now().Add(30 * time.Minute),
		Status:          "paid",
	})
	require.NoError(t, err)
	_, err = testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:      1,
		UnitPrice:     dish.Price,
		TotalPrice:    dish.Price,
	})
	require.NoError(t, err)
	_, err = testStore.SyncReservationInventoryTx(ctx, SyncReservationInventoryTxParams{ReservationID: reservation.ID})
	require.NoError(t, err)

	createResult, err := testStore.CreateReservationPositiveAdjustmentPaymentTx(ctx, CreateReservationPositiveAdjustmentPaymentTxParams{
		ReservationID:         reservation.ID,
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		ExpectedCurrentAmount: dish.Price,
		TargetTotal:           dish.Price * 2,
		DeltaAmount:           dish.Price,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: dish.ID, Valid: true},
			Quantity:      2,
			UnitPrice:     dish.Price,
			TotalPrice:    dish.Price * 2,
		}},
		OutTradeNo: "RA" + util.RandomString(20),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Attach:     "reservation_id:1;payment_mode:full;addon:true",
	})
	require.NoError(t, err)
	first, err := testStore.CloseReservationAdjustmentForPaymentTx(ctx, CloseReservationAdjustmentForPaymentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
		Status:         ReservationAdjustmentStatusClosed,
		Reason:         "user closed",
	})
	require.NoError(t, err)
	require.True(t, first.Closed)
	require.Equal(t, ReservationAdjustmentStatusClosed, first.Adjustment.Status)

	second, err := testStore.CloseReservationAdjustmentForPaymentTx(ctx, CloseReservationAdjustmentForPaymentTxParams{
		PaymentOrderID: createResult.PaymentOrder.ID,
		Status:         ReservationAdjustmentStatusExpired,
		Reason:         "payment timeout retry",
	})
	require.NoError(t, err)
	require.False(t, second.Closed)
	require.Equal(t, ReservationAdjustmentStatusClosed, second.Adjustment.Status)

	inventory, err := testStore.GetDailyInventory(ctx, GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: reservationDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), inventory.ReservedQuantity)
}
