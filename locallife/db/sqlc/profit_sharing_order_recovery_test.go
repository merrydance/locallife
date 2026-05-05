package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListCompletedOrdersMissingProfitSharing_ExcludesTakeout(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           OrderTypeTakeout,
		AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
		DeliveryFee:         500,
		DeliveryDistance:    pgtype.Int4{Int32: 1800, Valid: true},
		Subtotal:            4800,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         5300,
		Status:              OrderStatusCompleted,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         OrderStatusCompleted,
		ExpectedStatus: OrderStatusCompleted,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelOrdinaryServiceProvider,
		RequiresProfitSharing: true,
		BusinessType:          "order",
		Amount:                order.TotalAmount,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(24), Valid: true},
	})
	require.NoError(t, err)

	rows, err := testStore.ListCompletedOrdersMissingProfitSharing(context.Background(), 200)
	require.NoError(t, err)

	for _, row := range rows {
		require.NotEqual(t, paymentOrder.ID, row.PaymentOrderID)
	}
}

func TestListCompletedOrdersMissingProfitSharing_IncludesNonTakeout(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "confirmed")

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           "dine_in",
		ReservationID:       pgtype.Int8{Int64: reservation.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            3600,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         3600,
		Status:              OrderStatusCompleted,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         OrderStatusCompleted,
		ExpectedStatus: OrderStatusCompleted,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelOrdinaryServiceProvider,
		RequiresProfitSharing: true,
		BusinessType:          "order",
		Amount:                order.TotalAmount,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(24), Valid: true},
	})
	require.NoError(t, err)

	rows, err := testStore.ListCompletedOrdersMissingProfitSharing(context.Background(), 200)
	require.NoError(t, err)

	matched := false
	for _, row := range rows {
		if row.PaymentOrderID == paymentOrder.ID {
			require.True(t, row.OrderID.Valid)
			require.Equal(t, order.ID, row.OrderID.Int64)
			matched = true
			break
		}
	}
	require.True(t, matched)
}

func TestListBaofuOrdersReadyForProfitSharing_GatesCompletedPaidAndRefundClosed(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           OrderTypeTakeout,
		AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
		DeliveryFee:         500,
		DeliveryDistance:    pgtype.Int4{Int32: 1800, Valid: true},
		Subtotal:            4800,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         5300,
		Status:              OrderStatusCompleted,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         OrderStatusCompleted,
		ExpectedStatus: OrderStatusCompleted,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(24), Valid: true},
	})
	require.NoError(t, err)

	rows, err := testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true},
		Limit:              200,
	})
	require.NoError(t, err)

	matched := false
	for _, row := range rows {
		if row.PaymentOrderID == paymentOrder.ID {
			require.True(t, row.OrderID.Valid)
			require.Equal(t, order.ID, row.OrderID.Int64)
			matched = true
			break
		}
	}
	require.True(t, matched)

	_, err = testStore.CreateRefundOrder(context.Background(), CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "miniprogram",
		RefundAmount:   100,
		RefundReason:   pgtype.Text{String: "test refund guard", Valid: true},
		OutRefundNo:    util.RandomString(24),
		PlatformRefund: pgtype.Int8{Int64: 0, Valid: true},
		OperatorRefund: pgtype.Int8{Int64: 0, Valid: true},
		MerchantRefund: pgtype.Int8{Int64: 100, Valid: true},
		Status:         "pending",
	})
	require.NoError(t, err)

	rows, err = testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true},
		Limit:              200,
	})
	require.NoError(t, err)

	for _, row := range rows {
		require.NotEqual(t, paymentOrder.ID, row.PaymentOrderID)
	}
}

func TestListBaofuOrdersReadyForProfitSharing_IncludesPaidReservations(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "paid")

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(24), Valid: true},
	})
	require.NoError(t, err)

	rows, err := testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true},
		Limit:              200,
	})
	require.NoError(t, err)

	matched := false
	for _, row := range rows {
		if row.PaymentOrderID == paymentOrder.ID {
			require.False(t, row.OrderID.Valid)
			require.True(t, row.ReservationID.Valid)
			require.Equal(t, reservation.ID, row.ReservationID.Int64)
			require.Equal(t, ExternalPaymentBusinessOwnerReservation, row.BusinessType)
			matched = true
			break
		}
	}
	require.True(t, matched)
}
