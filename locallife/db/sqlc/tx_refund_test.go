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

func TestCreateRefundOrderTx_BaofuRejectsRefundAfterProfitSharingStarts(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	payment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          "order",
		Amount:                1000,
		OutTradeNo:            util.RandomString(32),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	payment, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:      payment.ID,
		MerchantID:          merchant.ID,
		OrderSource:         "takeout",
		TotalAmount:         payment.Amount,
		DeliveryFee:         0,
		RiderAmount:         0,
		DistributableAmount: payment.Amount,
		PlatformRate:        200,
		OperatorRate:        300,
		PlatformCommission:  20,
		OperatorCommission:  30,
		MerchantAmount:      947,
		OutOrderNo:          "baofu_refund_guard_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusPending,
		PaymentFee:          3,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)

	_, err = testStore.CreateRefundOrderTx(ctx, CreateRefundOrderTxParams{
		PaymentOrderID: payment.ID,
		RefundType:     "profit_sharing",
		RefundAmount:   100,
		RefundReason:   "商品售罄",
		OutRefundNo:    util.RandomString(32),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已进入结算分账流程，不支持退款")

	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Empty(t, refunds)
}

func TestCreateRefundOrderTx_BaofuAllowsRefundWhenOnlyFailedProfitSharingExists(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	payment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          "order",
		Amount:                1000,
		OutTradeNo:            util.RandomString(32),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	payment, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:      payment.ID,
		MerchantID:          merchant.ID,
		OrderSource:         "takeout",
		TotalAmount:         payment.Amount,
		DeliveryFee:         0,
		RiderAmount:         0,
		DistributableAmount: payment.Amount,
		PlatformRate:        200,
		OperatorRate:        300,
		PlatformCommission:  20,
		OperatorCommission:  30,
		MerchantAmount:      947,
		OutOrderNo:          "baofu_refund_failed_" + util.RandomString(16),
		Status:              "failed",
		PaymentFee:          3,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)

	result, err := testStore.CreateRefundOrderTx(ctx, CreateRefundOrderTxParams{
		PaymentOrderID: payment.ID,
		RefundType:     "profit_sharing",
		RefundAmount:   100,
		RefundReason:   "商品售罄",
		OutRefundNo:    util.RandomString(32),
	})
	require.NoError(t, err)
	require.Equal(t, payment.ID, result.RefundOrder.PaymentOrderID)
	require.Equal(t, "pending", result.RefundOrder.Status)
}
