package db

import (
	"context"
	"testing"

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