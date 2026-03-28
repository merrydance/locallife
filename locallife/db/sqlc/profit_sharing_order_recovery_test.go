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
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:       user.ID,
		PaymentType:  "profit_sharing",
		BusinessType: "order",
		Amount:       order.TotalAmount,
		OutTradeNo:   util.RandomString(24),
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
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

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           "takeaway",
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
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:       user.ID,
		PaymentType:  "profit_sharing",
		BusinessType: "order",
		Amount:       order.TotalAmount,
		OutTradeNo:   util.RandomString(24),
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
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
