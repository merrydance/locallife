package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func backdateBaofuProfitSharingFixtures(t *testing.T, paymentOrderID int64, orderID int64, reservationID int64, hasOrder bool, hasReservation bool, at time.Time) {
	t.Helper()

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	if hasOrder {
		_, err := store.connPool.Exec(context.Background(), `
			UPDATE orders
			SET created_at = $1,
			    updated_at = $1,
			    completed_at = $1
			WHERE id = $2
		`, at, orderID)
		require.NoError(t, err)
	}

	if hasReservation {
		_, err := store.connPool.Exec(context.Background(), `
			UPDATE table_reservations
			SET created_at = $1,
			    updated_at = $1,
			    paid_at = $1
			WHERE id = $2
		`, at, reservationID)
		require.NoError(t, err)
	}

	_, err := store.connPool.Exec(context.Background(), `
		UPDATE payment_orders
		SET created_at = $1,
		    paid_at = $1
		WHERE id = $2
	`, at, paymentOrderID)
	require.NoError(t, err)
}

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
		PaymentChannel:        PaymentChannelBaofuAggregate,
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
		PaymentChannel:        PaymentChannelBaofuAggregate,
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

	profitSharingAnchor := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	backdateBaofuProfitSharingFixtures(t, paymentOrder.ID, order.ID, 0, true, false, profitSharingAnchor)

	rows, err := testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: profitSharingAnchor.Add(time.Minute), Valid: true},
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

	profitSharingAnchor := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	backdateBaofuProfitSharingFixtures(t, paymentOrder.ID, 0, reservation.ID, false, true, profitSharingAnchor)

	rows, err := testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: profitSharingAnchor.Add(time.Minute), Valid: true},
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

func TestListBaofuProcessingProfitSharingOrdersForRecoveryUsesCommandStartedAt(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, ProfitSharingOrderStatusPending, "pso_processing_recovery_")

	createdAt := time.Now().UTC().Add(-10 * time.Minute)
	startedAt := time.Now().UTC()
	_, err := testStore.(*SQLStore).connPool.Exec(ctx, `
		UPDATE profit_sharing_orders
		SET created_at = $1,
		    command_started_at = $2,
		    status = 'processing'
		WHERE id = $3
	`, createdAt, startedAt, profitSharingOrder.ID)
	require.NoError(t, err)

	rows, err := testStore.ListBaofuProcessingProfitSharingOrdersForRecovery(ctx, ListBaofuProcessingProfitSharingOrdersForRecoveryParams{
		CreatedBefore: pgtype.Timestamptz{Time: startedAt.Add(-time.Minute), Valid: true},
		Limit:         200,
	})
	require.NoError(t, err)
	for _, row := range rows {
		require.NotEqual(t, profitSharingOrder.ID, row.ID)
	}

	rows, err = testStore.ListBaofuProcessingProfitSharingOrdersForRecovery(ctx, ListBaofuProcessingProfitSharingOrdersForRecoveryParams{
		CreatedBefore: pgtype.Timestamptz{Time: startedAt.Add(time.Minute), Valid: true},
		Limit:         200,
	})
	require.NoError(t, err)
	matched := false
	for _, row := range rows {
		if row.ID == profitSharingOrder.ID {
			require.Equal(t, paymentOrder.ID, row.PaymentOrderID)
			require.True(t, row.CommandStartedAt.Valid)
			matched = true
			break
		}
	}
	require.True(t, matched)
}
