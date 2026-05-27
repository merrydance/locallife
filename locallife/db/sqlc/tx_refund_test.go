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

func TestCreateRefundOrderTx_AllowsProductionRefundTypes(t *testing.T) {
	ctx := context.Background()
	refundTypes := []string{
		"miniprogram",
		"profit_sharing",
		"rider_deposit",
		"user_cancel",
		"full",
		"partial",
		"merchant_cancel",
		"amount_mismatch",
		"closed_order_anomaly",
	}

	for _, refundType := range refundTypes {
		t.Run(refundType, func(t *testing.T) {
			user := createRandomUser(t)
			payment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
				UserID:                user.ID,
				PaymentType:           "profit_sharing",
				PaymentChannel:        PaymentChannelBaofuAggregate,
				RequiresProfitSharing: true,
				BusinessType:          "order",
				Amount:                1459,
				OutTradeNo:            util.RandomString(32),
			})
			require.NoError(t, err)

			payment, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
				ID:            payment.ID,
				TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
			})
			require.NoError(t, err)

			result, err := testStore.CreateRefundOrderTx(ctx, CreateRefundOrderTxParams{
				PaymentOrderID: payment.ID,
				RefundType:     refundType,
				RefundAmount:   payment.Amount,
				RefundReason:   "代取时间太长",
				OutRefundNo:    util.RandomString(32),
			})
			require.NoError(t, err)
			require.Equal(t, refundType, result.RefundOrder.RefundType)
			require.Equal(t, payment.Amount, result.RefundOrder.RefundAmount)
			require.Equal(t, "pending", result.RefundOrder.Status)
		})
	}
}

func TestCreateRefundOrderTx_BaofuRejectsRefundAfterProfitSharingCommandStarted(t *testing.T) {
	for _, status := range []string{ProfitSharingOrderStatusProcessing, ProfitSharingOrderStatusFinished} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			user := createRandomUser(t)
			merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
			payment, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, status, "baofu_refund_guard_")

			_, err := testStore.CreateRefundOrderTx(ctx, CreateRefundOrderTxParams{
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

			currentProfitSharingOrder, err := testStore.GetProfitSharingOrder(ctx, profitSharingOrder.ID)
			require.NoError(t, err)
			require.Equal(t, status, currentProfitSharingOrder.Status)
		})
	}
}

func TestCreateRefundOrderTx_BaofuAllowsRefundWhenOnlyPendingProfitSharingBillExists(t *testing.T) {
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
		OutOrderNo:          "baofu_refund_pending_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusPending,
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

func createPaidBaofuPaymentWithProfitSharingOrder(t *testing.T, ctx context.Context, userID int64, merchantID int64, profitSharingStatus string, outOrderPrefix string) (PaymentOrder, ProfitSharingOrder) {
	payment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                userID,
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

	profitSharingOrder, err := testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:      payment.ID,
		MerchantID:          merchantID,
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
		OutOrderNo:          outOrderPrefix + util.RandomString(16),
		Status:              profitSharingStatus,
		PaymentFee:          3,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)
	return payment, profitSharingOrder
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
