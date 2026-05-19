package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListPaidUnrefundedPaymentOrdersSkipsFailedRefundOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithStatus(t, user.ID, merchant.ID, OrderStatusCancelled)

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "profit_sharing",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerOrder,
		Amount:                1459,
		OutTradeNo:            util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "BFPAY_UP_" + util.RandomString(12), Valid: true},
	})
	require.NoError(t, err)

	refundOrder := createRandomRefundOrder(t, paymentOrder.ID, paymentOrder.Amount)
	_, err = testStore.UpdateRefundOrderToFailed(context.Background(), refundOrder.ID)
	require.NoError(t, err)

	candidates, err := testStore.ListPaidUnrefundedPaymentOrders(context.Background(), 50)
	require.NoError(t, err)
	for _, candidate := range candidates {
		require.NotEqual(t, paymentOrder.ID, candidate.ID)
	}
}
