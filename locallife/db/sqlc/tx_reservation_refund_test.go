package db

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestReplaceReservationItemsWithRefundOrdersTxRollsBackItemsWhenRefundGuardRejects(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	category := createRandomDishCategory(t)
	oldDish := createRandomDish(t, merchant.ID, category.ID)
	newDish := createRandomDish(t, merchant.ID, category.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "checked_in")

	_, err := testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: oldDish.ID, Valid: true},
		Quantity:      2,
		UnitPrice:     3000,
		TotalPrice:    6000,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                6000,
		OutTradeNo:            util.RandomString(32),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:      paymentOrder.ID,
		MerchantID:          merchant.ID,
		OrderSource:         OrderTypeReservation,
		TotalAmount:         paymentOrder.Amount,
		DeliveryFee:         0,
		RiderAmount:         0,
		DistributableAmount: paymentOrder.Amount,
		PlatformRate:        200,
		OperatorRate:        300,
		PlatformCommission:  120,
		OperatorCommission:  180,
		MerchantAmount:      5700,
		OutOrderNo:          "pso_started_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusProcessing,
		PaymentFee:          18,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)

	_, err = testStore.ReplaceReservationItemsWithRefundOrdersTx(ctx, ReplaceReservationItemsWithRefundOrdersTxParams{
		ReservationID:         reservation.ID,
		ExpectedCurrentAmount: 6000,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: newDish.ID, Valid: true},
			Quantity:      1,
			UnitPrice:     1000,
			TotalPrice:    1000,
		}},
		RefundOrders: []CreateRefundOrderTxParams{{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     "profit_sharing",
			RefundAmount:   5000,
			RefundReason:   "reservation dish change refund",
			OutRefundNo:    util.RandomString(32),
		}},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	items, err := testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, oldDish.ID, items[0].DishID.Int64)
	require.Equal(t, int64(6000), items[0].TotalPrice)

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(ctx, paymentOrder.ID)
	require.NoError(t, err)
	require.Empty(t, refunds)
}

func TestReplaceReservationItemsWithRefundOrdersTxRejectsStaleCurrentTotal(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	table := createRandomTable(t, merchant.ID)
	oldDish := createRandomDish(t, merchant.ID, createRandomDishCategory(t).ID)
	newDish := createRandomDish(t, merchant.ID, createRandomDishCategory(t).ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "checked_in")

	_, err := testStore.CreateReservationItem(ctx, CreateReservationItemParams{
		ReservationID: reservation.ID,
		DishID:        pgtype.Int8{Int64: oldDish.ID, Valid: true},
		Quantity:      2,
		UnitPrice:     3000,
		TotalPrice:    6000,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                6000,
		OutTradeNo:            util.RandomString(32),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.ReplaceReservationItemsWithRefundOrdersTx(ctx, ReplaceReservationItemsWithRefundOrdersTxParams{
		ReservationID:         reservation.ID,
		ExpectedCurrentAmount: 5000,
		Items: []CreateReservationItemParams{{
			ReservationID: reservation.ID,
			DishID:        pgtype.Int8{Int64: newDish.ID, Valid: true},
			Quantity:      1,
			UnitPrice:     1000,
			TotalPrice:    1000,
		}},
		RefundOrders: []CreateRefundOrderTxParams{{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     "profit_sharing",
			RefundAmount:   4000,
			RefundReason:   "reservation dish change refund",
			OutRefundNo:    util.RandomString(32),
		}},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, statusCode)

	items, err := testStore.GetReservationItemsByReservation(ctx, reservation.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, oldDish.ID, items[0].DishID.Int64)
	require.Equal(t, int64(6000), items[0].TotalPrice)

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(ctx, paymentOrder.ID)
	require.NoError(t, err)
	require.Empty(t, refunds)
}
