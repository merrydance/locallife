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

func TestReplaceOrderTx_ReLinksReplacementOrderToBillingGroup(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	table := createRandomTable(t, merchant.ID)
	session := createOpenDiningSession(t, merchant.ID, table.ID, user.ID, pgtype.Int8{Valid: false})

	billingGroup, err := testStore.CreateBillingGroup(context.Background(), CreateBillingGroupParams{
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
	})
	require.NoError(t, err)

	oldOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE orders
		SET status = $2, total_amount = $3, table_id = $4, updated_at = now()
		WHERE id = $1`, oldOrder.ID, OrderStatusPaid, int64(1000), pgtype.Int8{Int64: table.ID, Valid: true})
	require.NoError(t, err)

	_, err = testStore.CreateBillingGroupOrder(context.Background(), CreateBillingGroupOrderParams{
		BillingGroupID: billingGroup.ID,
		OrderID:        oldOrder.ID,
		Amount:         1000,
		Status:         "linked",
	})
	require.NoError(t, err)

	result, err := testStore.ReplaceOrderTx(context.Background(), ReplaceOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             util.RandomString(20),
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "dine_in",
			TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
			DeliveryFee:         0,
			Subtotal:            1500,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         1500,
			Status:              OrderStatusPaid,
			FulfillmentStatus:   FulfillmentStatusPendingKitchen,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "Replacement Dish",
			UnitPrice: 1500,
			Quantity:  1,
			Subtotal:  1500,
		}},
		OldOrderID:   oldOrder.ID,
		CancelReason: "replaced by new order",
	})
	require.NoError(t, err)
	require.NotZero(t, result.NewOrder.ID)
	require.Equal(t, OrderStatusCancelled, result.OldOrder.Status)
	require.True(t, result.OldOrder.ReplacedByOrderID.Valid)
	require.Equal(t, result.NewOrder.ID, result.OldOrder.ReplacedByOrderID.Int64)

	orders, err := testStore.ListBillingGroupOrdersByGroup(context.Background(), billingGroup.ID)
	require.NoError(t, err)
	require.Len(t, orders, 2)
	require.Equal(t, oldOrder.ID, orders[0].OrderID)
	require.Equal(t, result.NewOrder.ID, orders[1].OrderID)
	require.Equal(t, int64(1500), orders[1].Amount)

	amounts, err := testStore.GetBillingGroupAmounts(context.Background(), billingGroup.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1500), amounts.TotalAmount)
	require.Equal(t, int64(1500), amounts.PaidAmount)
}

func TestReplaceOrderWithRefundOrdersTxRollsBackReplacementWhenRefundGuardRejects(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	table := createRandomTable(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "checked_in")

	oldOrder, err := testStore.CreateOrder(ctx, CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           OrderTypeReservation,
		TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
		ReservationID:       pgtype.Int8{Int64: reservation.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            5000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         5000,
		Status:              OrderStatusPaid,
		FulfillmentStatus:   FulfillmentStatusPendingKitchen,
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: oldOrder.ID, Valid: true},
		ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                oldOrder.TotalAmount,
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
		PlatformCommission:  100,
		OperatorCommission:  150,
		MerchantAmount:      4735,
		OutOrderNo:          "pso_replace_started_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusProcessing,
		PaymentFee:          15,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)

	newOrderNo := util.RandomString(20)
	_, err = testStore.ReplaceOrderWithRefundOrdersTx(ctx, ReplaceOrderWithRefundOrdersTxParams{
		ReplaceOrderTxParams: ReplaceOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:             newOrderNo,
				UserID:              user.ID,
				MerchantID:          merchant.ID,
				OrderType:           OrderTypeDineIn,
				TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
				ReservationID:       pgtype.Int8{Int64: reservation.ID, Valid: true},
				DeliveryFee:         0,
				Subtotal:            3500,
				DiscountAmount:      0,
				DeliveryFeeDiscount: 0,
				TotalAmount:         3500,
				Status:              OrderStatusPaid,
				FulfillmentStatus:   FulfillmentStatusPendingKitchen,
			},
			Items: []CreateOrderItemParams{{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Replacement Dish",
				UnitPrice: 3500,
				Quantity:  1,
				Subtotal:  3500,
			}},
			OldOrderID:   oldOrder.ID,
			CancelReason: "replaced by new order",
		},
		RefundOrders: []CreateRefundOrderTxParams{{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     "profit_sharing",
			RefundAmount:   1500,
			RefundReason:   "订单改菜单退款",
			OutRefundNo:    util.RandomString(32),
		}},
	})
	require.Error(t, err)
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	unchangedOldOrder, err := testStore.GetOrder(ctx, oldOrder.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, unchangedOldOrder.Status)
	require.False(t, unchangedOldOrder.ReplacedByOrderID.Valid)

	_, err = testStore.GetOrderByOrderNo(ctx, newOrderNo)
	require.ErrorIs(t, err, ErrRecordNotFound)

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(ctx, paymentOrder.ID)
	require.NoError(t, err)
	require.Empty(t, refunds)
}

func TestReplaceOrderTxRejectsAlreadyReplacedOldOrder(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	table := createRandomTable(t, merchant.ID)

	oldOrder, err := testStore.CreateOrder(ctx, CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           OrderTypeReservation,
		TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            5000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         5000,
		Status:              OrderStatusPaid,
		FulfillmentStatus:   FulfillmentStatusPendingKitchen,
	})
	require.NoError(t, err)

	first, err := testStore.ReplaceOrderTx(ctx, ReplaceOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             util.RandomString(20),
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           OrderTypeDineIn,
			TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
			DeliveryFee:         0,
			Subtotal:            4500,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         4500,
			Status:              OrderStatusPaid,
			FulfillmentStatus:   FulfillmentStatusPendingKitchen,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "First Replacement Dish",
			UnitPrice: 4500,
			Quantity:  1,
			Subtotal:  4500,
		}},
		OldOrderID:   oldOrder.ID,
		CancelReason: "replaced by new order",
	})
	require.NoError(t, err)

	secondOrderNo := util.RandomString(20)
	_, err = testStore.ReplaceOrderTx(ctx, ReplaceOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             secondOrderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           OrderTypeDineIn,
			TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
			DeliveryFee:         0,
			Subtotal:            4000,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         4000,
			Status:              OrderStatusPaid,
			FulfillmentStatus:   FulfillmentStatusPendingKitchen,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "Second Replacement Dish",
			UnitPrice: 4000,
			Quantity:  1,
			Subtotal:  4000,
		}},
		OldOrderID:   oldOrder.ID,
		CancelReason: "replaced again",
	})
	require.Error(t, err)

	unchangedOldOrder, err := testStore.GetOrder(ctx, oldOrder.ID)
	require.NoError(t, err)
	require.Equal(t, first.NewOrder.ID, unchangedOldOrder.ReplacedByOrderID.Int64)

	_, err = testStore.GetOrderByOrderNo(ctx, secondOrderNo)
	require.ErrorIs(t, err, ErrRecordNotFound)
}
