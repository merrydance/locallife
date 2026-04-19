package db

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreateRefundOrderTx_CountsPendingAndProcessingRefunds(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)

	payment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          "order",
		Amount:                1000,
		OutTradeNo:            util.RandomString(32),
	})
	require.NoError(t, err)

	_, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	pendingRefund := createRandomRefundOrder(t, payment.ID, 300)
	processingRefund := createRandomRefundOrder(t, payment.ID, 400)

	_, err = testStore.UpdateRefundOrderToProcessing(ctx, UpdateRefundOrderToProcessingParams{
		ID:       processingRefund.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.CreateRefundOrderTx(ctx, CreateRefundOrderTxParams{
		PaymentOrderID: payment.ID,
		RefundType:     "miniprogram",
		RefundAmount:   301,
		RefundReason:   "exceeds occupied amount",
		OutRefundNo:    util.RandomString(32),
	})
	require.Error(t, err)

	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)
	require.Contains(t, err.Error(), "exceeds payment amount")

	latestPendingRefund, err := testStore.GetRefundOrder(ctx, pendingRefund.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", latestPendingRefund.Status)
}
