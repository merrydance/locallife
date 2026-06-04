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

func TestListBaofuOrdersReadyForProfitSharing_SkipsOrderAfterSuccessfulRefund(t *testing.T) {
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

	refund := createRandomRefundOrder(t, paymentOrder.ID, 300)
	_, err = testStore.UpdateRefundOrderToSuccess(context.Background(), refund.ID)
	require.NoError(t, err)

	profitSharingAnchor := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	backdateBaofuProfitSharingFixtures(t, paymentOrder.ID, order.ID, 0, true, false, profitSharingAnchor)

	rows, err := testStore.ListBaofuOrdersReadyForProfitSharing(context.Background(), ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: profitSharingAnchor.Add(time.Minute), Valid: true},
		Limit:              200,
	})
	require.NoError(t, err)

	for _, row := range rows {
		require.NotEqual(t, paymentOrder.ID, row.PaymentOrderID)
	}
}

func TestListBaofuOrdersReadyForProfitSharing_AllowsReservationSuccessfulRefundWithNetAmount(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "completed")

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                5300,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(24), Valid: true},
	})
	require.NoError(t, err)

	refund := createRandomRefundOrder(t, paymentOrder.ID, 300)
	_, err = testStore.UpdateRefundOrderToSuccess(context.Background(), refund.ID)
	require.NoError(t, err)

	profitSharingAnchor := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	backdateBaofuProfitSharingFixtures(t, paymentOrder.ID, 0, reservation.ID, false, true, profitSharingAnchor)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE table_reservations
		SET completed_at = $1
		WHERE id = $2
	`, profitSharingAnchor, reservation.ID)
	require.NoError(t, err)

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
			require.Equal(t, int64(5000), row.NetAmount)
			matched = true
			break
		}
	}
	require.True(t, matched)
}

func TestListBaofuOrdersReadyForProfitSharing_RequiresCompletedReservationWithCompletedAt(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)

	type reservationCase struct {
		name           string
		status         string
		completedAt    pgtype.Timestamptz
		expectIncluded bool
	}

	cases := []reservationCase{
		{name: "paid", status: "paid"},
		{name: "confirmed", status: "confirmed"},
		{name: "checked_in", status: "checked_in"},
		{name: "completed_without_completed_at", status: "completed"},
		{name: "completed_with_completed_at", status: "completed", completedAt: pgtype.Timestamptz{Time: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC), Valid: true}, expectIncluded: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, tc.status)

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
			if tc.completedAt.Valid {
				_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
					UPDATE table_reservations
					SET completed_at = $1
					WHERE id = $2
				`, tc.completedAt.Time, reservation.ID)
				require.NoError(t, err)
			}

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
			require.Equal(t, tc.expectIncluded, matched)
		})
	}
}

func TestListBaofuProfitSharingOrdersReadyForCommand_RequiresCompletedReservationWithCompletedAt(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "checked_in")

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

	shareOrder, err := testStore.CreateProfitSharingOrder(context.Background(), CreateProfitSharingOrderParams{
		PaymentOrderID:       paymentOrder.ID,
		MerchantID:           merchant.ID,
		OrderSource:          OrderTypeReservation,
		TotalAmount:          paymentOrder.Amount,
		DeliveryFee:          0,
		RiderAmount:          0,
		DistributableAmount:  paymentOrder.Amount,
		PlatformRate:         200,
		OperatorRate:         300,
		PlatformCommission:   200,
		OperatorCommission:   300,
		MerchantAmount:       9500,
		OutOrderNo:           util.RandomString(24),
		Status:               ProfitSharingOrderStatusPending,
		PaymentFee:           30,
		PaymentFeeRateBps:    30,
		Provider:             pgtype.Text{String: ExternalPaymentProviderBaofu, Valid: true},
		Channel:              pgtype.Text{String: PaymentChannelBaofuAggregate, Valid: true},
		MerchantSharingMerID: pgtype.Text{String: "MER_SHARE", Valid: true},
		PlatformSharingMerID: pgtype.Text{String: "PLATFORM_SHARE", Valid: true},
		SharingDetailSnapshot: []byte(`{
			"shareable_amount": 9500,
			"receivers": [
				{"sharing_mer_id": "MER_SHARE", "amount": 9500}
			]
		}`),
	})
	require.NoError(t, err)

	profitSharingAnchor := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	backdateBaofuProfitSharingFixtures(t, paymentOrder.ID, 0, reservation.ID, false, true, profitSharingAnchor)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE profit_sharing_orders
		SET created_at = $1
		WHERE id = $2
	`, profitSharingAnchor, shareOrder.ID)
	require.NoError(t, err)

	rows, err := testStore.ListBaofuProfitSharingOrdersReadyForCommand(context.Background(), ListBaofuProfitSharingOrdersReadyForCommandParams{
		CreatedBefore: profitSharingAnchor.Add(time.Minute),
		Limit:         200,
	})
	require.NoError(t, err)

	for _, row := range rows {
		require.NotEqual(t, shareOrder.ID, row.ID)
	}

	completed, err := testStore.UpdateReservationToCompleted(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.True(t, completed.CompletedAt.Valid)

	rows, err = testStore.ListBaofuProfitSharingOrdersReadyForCommand(context.Background(), ListBaofuProfitSharingOrdersReadyForCommandParams{
		CreatedBefore: profitSharingAnchor.Add(time.Minute),
		Limit:         200,
	})
	require.NoError(t, err)

	matched := false
	for _, row := range rows {
		if row.ID == shareOrder.ID {
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
		Limit:         countBaofuProcessingProfitSharingOrdersForRecovery(t, ctx, startedAt.Add(time.Minute)),
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

func countBaofuProcessingProfitSharingOrdersForRecovery(t *testing.T, ctx context.Context, createdBefore time.Time) int32 {
	t.Helper()

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	var count int64
	err := store.connPool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM profit_sharing_orders
		WHERE provider = 'baofu'
		  AND channel = 'baofu_aggregate'
		  AND status = 'processing'
		  AND command_started_at IS NOT NULL
		  AND command_started_at <= $1
	`, createdBefore).Scan(&count)
	require.NoError(t, err)
	require.Positive(t, count)
	require.Less(t, count, int64(1<<31))

	return int32(count)
}
