package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomMerchantPaymentConfig(t *testing.T, merchant Merchant) MerchantPaymentConfig {
	arg := CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   util.RandomString(10),
		Status:     "active",
	}
	config, err := testStore.CreateMerchantPaymentConfig(context.Background(), arg)
	require.NoError(t, err)
	return config
}

func TestCreateCombinedPaymentTx(t *testing.T) {
	store := testStore

	// 1. Create a user
	user := createRandomUser(t)

	// 2. Create 2 merchants
	merchantFull := createRandomMerchantForTest(t)
	merchantFullConfig := createRandomMerchantPaymentConfig(t, merchantFull)

	merchantPartial := createRandomMerchantForTest(t)
	merchantPartialConfig := createRandomMerchantPaymentConfig(t, merchantPartial)

	// 3. Create 2 takeout orders for this user
	order1, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchantFull.ID,
		OrderType:   "takeout",
		Subtotal:    1000,
		TotalAmount: 1000,
		Status:      "pending",
	})
	require.NoError(t, err)
	order2, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchantPartial.ID,
		OrderType:   "takeout",
		Subtotal:    2000,
		TotalAmount: 2000,
		Status:      "pending",
	})
	require.NoError(t, err)

	// 4. Input arguments
	arg := CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order1.ID, order2.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	}

	// 5. Execute transaction
	result, err := store.CreateCombinedPaymentTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 6. Verify CombinedPaymentOrder
	require.NotEmpty(t, result.CombinedPaymentOrder)
	require.Equal(t, user.ID, result.CombinedPaymentOrder.UserID)
	require.Equal(t, arg.CombineOutTradeNo, result.CombinedPaymentOrder.CombineOutTradeNo)
	require.Equal(t, order1.TotalAmount+order2.TotalAmount, result.CombinedPaymentOrder.TotalAmount)
	require.Equal(t, "pending", result.CombinedPaymentOrder.Status)

	// 7. Verify PaymentOrders
	require.Len(t, result.PaymentOrders, 2)
	for _, po := range result.PaymentOrders {
		require.NotEmpty(t, po)
		require.Equal(t, user.ID, po.UserID)
		require.Equal(t, "miniprogram", po.PaymentType)
		require.Equal(t, PaymentChannelEcommerce, po.PaymentChannel)
		require.True(t, po.RequiresProfitSharing)
		require.Equal(t, "order", po.BusinessType)
		require.Equal(t, result.CombinedPaymentOrder.ID, po.CombinedPaymentID.Int64)
		require.True(t, po.Attach.Valid)
	}

	// 8. Verify OrderInfos and SubMchIDs
	require.Len(t, result.OrderInfos, 2)
	for _, info := range result.OrderInfos {
		require.NotEmpty(t, info.PaymentConfig)
		switch info.Order.ID {
		case order1.ID:
			require.Equal(t, merchantFullConfig.SubMchID, info.PaymentConfig.SubMchID)
		case order2.ID:
			require.Equal(t, merchantPartialConfig.SubMchID, info.PaymentConfig.SubMchID)
		}
	}
}

func TestCreateCombinedPaymentTx_OrderMismatch(t *testing.T) {
	store := testStore
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	merchant := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant)

	// Order belongs to user2
	order := createRandomOrderWithUserAndMerchant(t, user2.ID, merchant.ID)

	arg := CreateCombinedPaymentTxParams{
		UserID:            user1.ID, // Mismatch
		OrderIDs:          []int64{order.ID},
		CombineOutTradeNo: "CPFAIL" + util.RandomString(5),
		ExpiresAt:         time.Now(),
	}

	_, err := store.CreateCombinedPaymentTx(context.Background(), arg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not belong to user")
}

func TestCreateCombinedPaymentTx_UsesRemainingPayableAmount(t *testing.T) {
	store := testStore
	user := createRandomUser(t)

	merchant1 := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant1)
	merchant2 := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant2)

	order1, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant1.ID,
		OrderType:   "takeout",
		Subtotal:    1000,
		TotalAmount: 1000,
		BalancePaid: 200,
		Status:      "pending",
	})
	require.NoError(t, err)
	order2, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant2.ID,
		OrderType:   "takeout",
		Subtotal:    2000,
		TotalAmount: 2000,
		Status:      "pending",
	})
	require.NoError(t, err)

	result, err := store.CreateCombinedPaymentTx(context.Background(), CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order1.ID, order2.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2800), result.CombinedPaymentOrder.TotalAmount)

	amountByOrderID := make(map[int64]int64, len(result.OrderInfos))
	for _, info := range result.OrderInfos {
		amountByOrderID[info.Order.ID] = info.PaymentOrder.Amount
	}
	require.Equal(t, int64(800), amountByOrderID[order1.ID])
	require.Equal(t, int64(2000), amountByOrderID[order2.ID])
}

func TestCreateCombinedPaymentTx_RejectsTakeawayOrders(t *testing.T) {
	store := testStore
	user := createRandomUser(t)
	merchant := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant)

	order, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   "takeaway",
		Subtotal:    1200,
		TotalAmount: 1200,
		Status:      "pending",
	})
	require.NoError(t, err)

	_, err = store.CreateCombinedPaymentTx(context.Background(), CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCombinedPaymentUnsupportedOrderType)
	require.Contains(t, err.Error(), "takeaway")
}

func TestCreateCombinedPaymentTx_CopiesReservationIDFromOrder(t *testing.T) {
	store := testStore
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "confirmed")

	order, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:       util.RandomString(20),
		UserID:        user.ID,
		MerchantID:    merchant.ID,
		OrderType:     "dine_in",
		TableID:       pgtype.Int8{Int64: table.ID, Valid: true},
		ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
		Subtotal:      1200,
		TotalAmount:   1200,
		Status:        "pending",
	})
	require.NoError(t, err)

	result, err := store.CreateCombinedPaymentTx(context.Background(), CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, result.PaymentOrders, 1)
	require.True(t, result.PaymentOrders[0].ReservationID.Valid)
	require.Equal(t, reservation.ID, result.PaymentOrders[0].ReservationID.Int64)
}

func TestCreateCombinedPaymentTx_ReturnsPendingPaymentConflict(t *testing.T) {
	store := testStore
	user := createRandomUser(t)
	merchant := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	order, err := store.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   "takeout",
		Subtotal:    1800,
		TotalAmount: 1800,
		Status:      "pending",
	})
	require.NoError(t, err)

	combinedPayment, err := store.CreateCombinedPaymentOrder(context.Background(), CreateCombinedPaymentOrderParams{
		UserID:            user.ID,
		CombineOutTradeNo: "CP" + util.RandomString(10),
		TotalAmount:       order.TotalAmount,
		Status:            "pending",
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	require.NoError(t, err)

	existingPayment, err := store.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		ReservationID:         pgtype.Int8{Valid: false},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelEcommerce,
		RequiresProfitSharing: false,
		BusinessType:          "order",
		Amount:                order.TotalAmount,
		OutTradeNo:            "CP" + util.RandomString(20),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		Attach:                pgtype.Text{String: "合单测试", Valid: true},
	})
	require.NoError(t, err)

	_, err = store.SetPaymentOrderCombinedID(context.Background(), SetPaymentOrderCombinedIDParams{
		ID:                existingPayment.ID,
		CombinedPaymentID: pgtype.Int8{Int64: combinedPayment.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = store.CreateCombinedPaymentTx(context.Background(), CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrOrderPendingPaymentConflict))

	latestPayment, getErr := store.GetPaymentOrder(context.Background(), existingPayment.ID)
	require.NoError(t, getErr)
	require.Equal(t, "pending", latestPayment.Status)
}
