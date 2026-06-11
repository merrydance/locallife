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

func TestGetBaofuDailyReconciliationCountsOnlyEvidenceBackedFailedOrderRefunds(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Date(2099, 1, 1, 10, 0, 0, 0, time.UTC).
		AddDate(0, 0, int(time.Now().UnixNano()%20000))
	expectedDate := time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), 0, 0, 0, 0, time.UTC)

	retryableRefund := createFailedBaofuOrderRefundForReconciliation(t, baseTime, 1200)
	createBaofuRefundCommandForReconciliation(t, retryableRefund, ExternalPaymentCommandStatusRejected, "SYSTEM_BUSY", baseTime.Add(time.Minute))

	queryableRefund := createFailedBaofuOrderRefundForReconciliation(t, baseTime, 3400)
	createBaofuRefundCommandForReconciliation(t, queryableRefund, ExternalPaymentCommandStatusRejected, "ORDER_EXIST", baseTime.Add(2*time.Minute))

	noCommandRefund := createFailedBaofuOrderRefundForReconciliation(t, baseTime, 5600)
	_ = noCommandRefund

	terminalBusinessRefund := createFailedBaofuOrderRefundForReconciliation(t, baseTime, 7800)
	createBaofuRefundCommandForReconciliation(t, terminalBusinessRefund, ExternalPaymentCommandStatusRejected, "REFUND_AMT_EXCEEDS", baseTime.Add(3*time.Minute))

	httpStatusRefund := createFailedBaofuOrderRefundForReconciliation(t, baseTime, 8900)
	createBaofuRefundCommandForReconciliation(t, httpStatusRefund, ExternalPaymentCommandStatusRejected, "HTTP_STATUS", baseTime.Add(4*time.Minute))

	processingPaymentOrder := createBaofuOrderPaymentForRefundReconciliation(t, baseTime, 9100)
	processingRefund := createRandomRefundOrder(t, processingPaymentOrder.ID, processingPaymentOrder.Amount)
	_, err := testStore.UpdateRefundOrderToProcessing(ctx, UpdateRefundOrderToProcessingParams{
		ID:       processingRefund.ID,
		RefundID: pgtype.Text{},
	})
	require.NoError(t, err)
	createBaofuRefundCommandForReconciliation(t, processingRefund, ExternalPaymentCommandStatusRejected, "SYSTEM_BUSY", baseTime.Add(5*time.Minute))

	reservationUser := createRandomUser(t)
	reservationPayment, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                reservationUser.ID,
		PaymentType:           "profit_sharing",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerReservation,
		Amount:                2300,
		OutTradeNo:            "BFPAYRES" + util.RandomString(16),
		ExpiresAt:             pgtype.Timestamptz{Time: baseTime.Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	reservationRefund := createRandomRefundOrder(t, reservationPayment.ID, reservationPayment.Amount)
	_, err = testStore.UpdateRefundOrderToFailed(ctx, reservationRefund.ID)
	require.NoError(t, err)
	createBaofuRefundCommandForReconciliation(t, reservationRefund, ExternalPaymentCommandStatusRejected, "SYSTEM_BUSY", baseTime.Add(6*time.Minute))

	rows, err := testStore.GetBaofuDailyReconciliation(ctx, GetBaofuDailyReconciliationParams{
		StartAt: pgtype.Timestamptz{Time: baseTime.Add(-time.Hour), Valid: true},
		EndAt:   pgtype.Timestamptz{Time: baseTime.Add(time.Hour), Valid: true},
	})
	require.NoError(t, err)

	var matched *GetBaofuDailyReconciliationRow
	for i := range rows {
		if rows[i].Date.Time.Equal(expectedDate) &&
			rows[i].Provider == ExternalPaymentProviderBaofu &&
			rows[i].Channel == PaymentChannelBaofuAggregate {
			matched = &rows[i]
			break
		}
	}
	require.NotNil(t, matched)
	require.Equal(t, int64(1), matched.HistoricalRetryableFailedRefundCount)
	require.Equal(t, int64(1200), matched.HistoricalRetryableFailedRefundAmount)
	require.Equal(t, int64(1), matched.HistoricalQueryableFailedRefundCount)
	require.Equal(t, int64(3400), matched.HistoricalQueryableFailedRefundAmount)
}

func createFailedBaofuOrderRefundForReconciliation(t *testing.T, createdAt time.Time, amount int64) RefundOrder {
	paymentOrder := createBaofuOrderPaymentForRefundReconciliation(t, createdAt, amount)
	refundOrder := createRandomRefundOrder(t, paymentOrder.ID, amount)
	_, err := testStore.UpdateRefundOrderToFailed(context.Background(), refundOrder.ID)
	require.NoError(t, err)
	return refundOrder
}

func createBaofuOrderPaymentForRefundReconciliation(t *testing.T, createdAt time.Time, amount int64) PaymentOrder {
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
		Amount:                amount,
		OutTradeNo:            "BFPAY" + util.RandomString(18),
		ExpiresAt:             pgtype.Timestamptz{Time: createdAt.Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "BFPAYUP" + util.RandomString(18), Valid: true},
	})
	require.NoError(t, err)
	return paymentOrder
}

func createBaofuRefundCommandForReconciliation(t *testing.T, refundOrder RefundOrder, status string, errorCode string, submittedAt time.Time) ExternalPaymentCommand {
	command, err := testStore.CreateExternalPaymentCommand(context.Background(), CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuRefund,
		CommandType:        ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:      ExternalPaymentBusinessOwnerOrder,
		BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: refundOrder.ID, Valid: true},
		ExternalObjectType: ExternalPaymentObjectRefund,
		ExternalObjectKey:  refundOrder.OutRefundNo,
		CommandStatus:      status,
		SubmittedAt:        submittedAt,
		RejectedAt:         pgtype.Timestamptz{Time: submittedAt, Valid: status == ExternalPaymentCommandStatusRejected},
		LastErrorCode:      pgtype.Text{String: errorCode, Valid: errorCode != ""},
		LastErrorMessage:   pgtype.Text{String: "sanitized provider guidance", Valid: true},
		ResponseSnapshot:   []byte(`{"provider":"baofu","operation":"order_refund"}`),
	})
	require.NoError(t, err)
	return command
}
