package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRefundableRiderDepositCredit(t *testing.T, rider Rider, amount int64) PaymentOrder {
	t.Helper()

	paymentOrder := createPaidRiderDepositPaymentOrder(t, rider, amount)
	result, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, result.Processed)

	return paymentOrder
}

func TestPrepareRiderDepositRefundTx_ReservesCreditAndFreezesBalance(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	result, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  12000,
		Remark:  "骑手押金提现",
	})
	require.NoError(t, err)
	require.Equal(t, int64(12000), result.FrozenAmount)
	require.Equal(t, int64(30000), result.Rider.DepositAmount)
	require.Equal(t, int64(12000), result.Rider.FrozenDeposit)
	require.Len(t, result.RefundPlans, 1)

	plan := result.RefundPlans[0]
	require.Equal(t, paymentOrder.ID, plan.SourcePaymentOrder.ID)
	require.Equal(t, int64(12000), plan.RefundOrder.RefundAmount)
	require.Equal(t, riderDepositRefundType, plan.RefundOrder.RefundType)
	require.Equal(t, "pending", plan.RefundOrder.Status)
	require.Equal(t, "freeze", plan.FreezeDepositRecord.Type)
	require.Equal(t, int64(18000), plan.FreezeDepositRecord.BalanceAfter)
	require.Equal(t, riderDepositCreditStatusPartial, plan.ReservedCredit.Status)
	require.Equal(t, int64(18000), plan.ReservedCredit.RefundableAmount)
	require.Equal(t, int64(12000), plan.ReservedCredit.RefundedAmount)

	persistedCredit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, int64(18000), persistedCredit.RefundableAmount)
	require.Equal(t, int64(12000), persistedCredit.RefundedAmount)
	require.Equal(t, riderDepositCreditStatusPartial, persistedCredit.Status)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), updatedRider.DepositAmount)
	require.Equal(t, int64(12000), updatedRider.FrozenDeposit)

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Len(t, refunds, 1)
	require.Equal(t, plan.RefundOrder.ID, refunds[0].ID)
}

func TestPrepareRiderDepositRefundTx_SplitsAcrossCredits(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	firstPaymentOrder := createRefundableRiderDepositCredit(t, rider, 10000)
	secondPaymentOrder := createRefundableRiderDepositCredit(t, rider, 20000)

	result, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  25000,
		Remark:  "跨押金凭证提现",
	})
	require.NoError(t, err)
	require.Equal(t, int64(25000), result.FrozenAmount)
	require.Len(t, result.RefundPlans, 2)

	creditsByPaymentOrderID := make(map[int64]RiderDepositCredit, len(result.RefundPlans))
	for _, plan := range result.RefundPlans {
		creditsByPaymentOrderID[plan.SourcePaymentOrder.ID] = plan.ReservedCredit
	}

	firstCredit := creditsByPaymentOrderID[firstPaymentOrder.ID]
	require.Equal(t, int64(0), firstCredit.RefundableAmount)
	require.Equal(t, int64(10000), firstCredit.RefundedAmount)
	require.Equal(t, "fully_refunded", firstCredit.Status)

	secondCredit := creditsByPaymentOrderID[secondPaymentOrder.ID]
	require.Equal(t, int64(5000), secondCredit.RefundableAmount)
	require.Equal(t, int64(15000), secondCredit.RefundedAmount)
	require.Equal(t, riderDepositCreditStatusPartial, secondCredit.Status)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), updatedRider.DepositAmount)
	require.Equal(t, int64(25000), updatedRider.FrozenDeposit)
}

func TestPrepareRiderDepositRefundTx_SubtractsPendingRefundWhenFrozenDrifted(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	_, err := testStore.CreateRefundOrder(context.Background(), CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     riderDepositRefundType,
		RefundAmount:   25000,
		RefundReason:   pgtype.Text{String: "legacy pending withdraw without frozen balance", Valid: true},
		OutRefundNo:    "ORNP" + util.RandomString(28),
		Status:         "pending",
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  10000,
		Remark:  "押金提现",
	})
	require.ErrorIs(t, err, ErrInsufficientDeposit)
}

func TestListRiderDepositLedgerAnomaliesDetectsDrift(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	baseline, err := testStore.ListRiderDepositLedgerAnomalies(context.Background(), ListRiderDepositLedgerAnomaliesParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Empty(t, baseline)

	_, err = testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 40000,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	_, err = testStore.CreateRefundOrder(context.Background(), CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     riderDepositRefundType,
		RefundAmount:   5000,
		RefundReason:   pgtype.Text{String: "success without local settlement", Valid: true},
		OutRefundNo:    "ORNS" + util.RandomString(28),
		Status:         "success",
	})
	require.NoError(t, err)

	unprocessedPaymentOrder := createPaidRiderDepositPaymentOrder(t, rider, 7000)
	_, err = testStore.CreateRiderDeposit(context.Background(), CreateRiderDepositParams{
		RiderID:        rider.ID,
		Amount:         unprocessedPaymentOrder.Amount,
		Type:           "deposit",
		PaymentOrderID: pgtype.Int8{Int64: unprocessedPaymentOrder.ID, Valid: true},
		BalanceAfter:   40000,
		Remark:         pgtype.Text{String: "half processed paid payment fixture", Valid: true},
	})
	require.NoError(t, err)

	anomalies, err := testStore.ListRiderDepositLedgerAnomalies(context.Background(), ListRiderDepositLedgerAnomaliesParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Limit:   20,
	})
	require.NoError(t, err)

	byType := make(map[string]ListRiderDepositLedgerAnomaliesRow, len(anomalies))
	for _, anomaly := range anomalies {
		byType[anomaly.AnomalyType] = anomaly
	}

	balanceAnomaly, ok := byType["deposit_amount_gt_refundable_credit"]
	require.True(t, ok)
	require.Equal(t, int64(10000), balanceAnomaly.AnomalyAmount)

	settlementAnomaly, ok := byType["success_refund_not_settled"]
	require.True(t, ok)
	require.True(t, settlementAnomaly.PaymentOrderID.Valid)
	require.Equal(t, paymentOrder.ID, settlementAnomaly.PaymentOrderID.Int64)
	require.Equal(t, int64(5000), settlementAnomaly.AnomalyAmount)

	unprocessedAnomaly, ok := byType["paid_unprocessed_has_artifacts"]
	require.True(t, ok)
	require.True(t, unprocessedAnomaly.PaymentOrderID.Valid)
	require.Equal(t, unprocessedPaymentOrder.ID, unprocessedAnomaly.PaymentOrderID.Int64)
	require.Equal(t, int64(7000), unprocessedAnomaly.LedgerAmount)
	require.Equal(t, int64(1), unprocessedAnomaly.AnomalyAmount)
}

func TestListRiderDepositWithdrawalRefundOrdersByIDsFiltersUser(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)
	prepareResult, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  12000,
		Remark:  "押金提现",
	})
	require.NoError(t, err)
	require.Len(t, prepareResult.RefundPlans, 1)
	refundOrder := prepareResult.RefundPlans[0].RefundOrder

	otherRider := createRandomRider(t)
	otherPaymentOrder := createRefundableRiderDepositCredit(t, otherRider, 20000)
	otherPrepareResult, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: otherRider.ID,
		Amount:  5000,
		Remark:  "其他骑手押金提现",
	})
	require.NoError(t, err)
	require.Len(t, otherPrepareResult.RefundPlans, 1)
	otherRefundOrder := otherPrepareResult.RefundPlans[0].RefundOrder

	rows, err := testStore.ListRiderDepositWithdrawalRefundOrdersByIDs(context.Background(), ListRiderDepositWithdrawalRefundOrdersByIDsParams{
		UserID:         rider.UserID,
		RefundOrderIds: []int64{refundOrder.ID, otherRefundOrder.ID},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, refundOrder.ID, rows[0].RefundOrderID)
	require.Equal(t, paymentOrder.ID, rows[0].PaymentOrderID)
	require.Equal(t, int64(12000), rows[0].RefundAmount)
	require.Equal(t, "pending", rows[0].Status)
	require.Equal(t, paymentOrder.OutTradeNo, rows[0].OutTradeNo)
	require.Equal(t, paymentOrder.Amount, rows[0].SourcePaymentAmount)

	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "wx_refund_tracking", Valid: true},
	})
	require.NoError(t, err)

	rows, err = testStore.ListRiderDepositWithdrawalRefundOrdersByIDs(context.Background(), ListRiderDepositWithdrawalRefundOrdersByIDsParams{
		UserID:         rider.UserID,
		RefundOrderIds: []int64{refundOrder.ID},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "processing", rows[0].Status)
	require.True(t, rows[0].RefundID.Valid)
	require.Equal(t, "wx_refund_tracking", rows[0].RefundID.String)

	otherRows, err := testStore.ListRiderDepositWithdrawalRefundOrdersByIDs(context.Background(), ListRiderDepositWithdrawalRefundOrdersByIDsParams{
		UserID:         otherRider.UserID,
		RefundOrderIds: []int64{refundOrder.ID},
	})
	require.NoError(t, err)
	require.Empty(t, otherRows)
	require.NotEqual(t, paymentOrder.ID, otherPaymentOrder.ID)
}

func TestResolveRiderDepositRefundTx_SuccessSettlesFrozenBalance(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	prepareResult, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  12000,
		Remark:  "押金提现成功",
	})
	require.NoError(t, err)
	require.Len(t, prepareResult.RefundPlans, 1)

	refundOrder := prepareResult.RefundPlans[0].RefundOrder
	result, err := testStore.(*SQLStore).ResolveRiderDepositRefundTx(context.Background(), ResolveRiderDepositRefundTxParams{
		RefundOrderID: refundOrder.ID,
		RefundStatus:  riderDepositRefundSucceeded,
		RefundID:      "REFUND_SUCCESS_001",
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, "success", result.RefundOrder.Status)
	require.Equal(t, "withdraw", result.DepositLog.Type)
	require.Equal(t, int64(18000), result.DepositLog.BalanceAfter)
	require.Equal(t, int64(18000), result.Rider.DepositAmount)
	require.Equal(t, int64(0), result.Rider.FrozenDeposit)
	require.Equal(t, RiderStatusApproved, result.Rider.Status)
	require.Equal(t, riderDepositCreditStatusPartial, result.Credit.Status)
	require.Equal(t, int64(18000), result.Credit.RefundableAmount)
	require.Equal(t, int64(12000), result.Credit.RefundedAmount)

	persistedRefund, err := testStore.GetRefundOrder(context.Background(), refundOrder.ID)
	require.NoError(t, err)
	require.Equal(t, "success", persistedRefund.Status)
	require.Equal(t, "REFUND_SUCCESS_001", persistedRefund.RefundID.String)
	require.True(t, persistedRefund.RefundedAt.Valid)

	persistedCredit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, int64(18000), persistedCredit.RefundableAmount)
	require.Equal(t, int64(12000), persistedCredit.RefundedAmount)
	require.Equal(t, riderDepositCreditStatusPartial, persistedCredit.Status)
}

func TestResolveRiderDepositRefundTx_ClosedRestoresCreditAndUnfreezesBalance(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	prepareResult, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  12000,
		Remark:  "押金提现关闭",
	})
	require.NoError(t, err)
	require.Len(t, prepareResult.RefundPlans, 1)

	refundOrder := prepareResult.RefundPlans[0].RefundOrder
	result, err := testStore.(*SQLStore).ResolveRiderDepositRefundTx(context.Background(), ResolveRiderDepositRefundTxParams{
		RefundOrderID: refundOrder.ID,
		RefundStatus:  riderDepositRefundClosed,
		RefundID:      "REFUND_CLOSED_001",
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, "closed", result.RefundOrder.Status)
	require.Equal(t, "unfreeze", result.DepositLog.Type)
	require.Equal(t, int64(30000), result.DepositLog.BalanceAfter)
	require.Equal(t, int64(30000), result.Rider.DepositAmount)
	require.Equal(t, int64(0), result.Rider.FrozenDeposit)
	require.Equal(t, riderDepositCreditStatusActive, result.Credit.Status)
	require.Equal(t, int64(30000), result.Credit.RefundableAmount)
	require.Equal(t, int64(0), result.Credit.RefundedAmount)

	persistedRefund, err := testStore.GetRefundOrder(context.Background(), refundOrder.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", persistedRefund.Status)

	persistedCredit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), persistedCredit.RefundableAmount)
	require.Equal(t, int64(0), persistedCredit.RefundedAmount)
	require.Equal(t, riderDepositCreditStatusActive, persistedCredit.Status)
}

func TestResolveRiderDepositRefundTx_SuccessWithDrainRemainingCreditReconcilesStaleBalance(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createRefundableRiderDepositCredit(t, rider, 30000)

	prepareResult, err := testStore.(*SQLStore).PrepareRiderDepositRefundTx(context.Background(), PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  10000,
		Remark:  "押金提现对账",
	})
	require.NoError(t, err)
	require.Len(t, prepareResult.RefundPlans, 1)

	refundOrder := prepareResult.RefundPlans[0].RefundOrder
	result, err := testStore.(*SQLStore).ResolveRiderDepositRefundTx(context.Background(), ResolveRiderDepositRefundTxParams{
		RefundOrderID:        refundOrder.ID,
		RefundStatus:         riderDepositRefundSucceeded,
		RefundID:             "",
		DrainRemainingCredit: true,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, int64(20000), result.ReconciledAmount)
	require.Equal(t, "success", result.RefundOrder.Status)
	require.Equal(t, "withdraw", result.DepositLog.Type)
	require.Equal(t, int64(20000), result.DepositLog.BalanceAfter)
	require.Equal(t, int64(0), result.Rider.DepositAmount)
	require.Equal(t, int64(0), result.Rider.FrozenDeposit)
	require.Equal(t, RiderStatusApproved, result.Rider.Status)
	require.Equal(t, "fully_refunded", result.Credit.Status)
	require.Equal(t, int64(0), result.Credit.RefundableAmount)
	require.Equal(t, int64(30000), result.Credit.RefundedAmount)

	persistedCredit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), persistedCredit.RefundableAmount)
	require.Equal(t, int64(30000), persistedCredit.RefundedAmount)
	require.Equal(t, "fully_refunded", persistedCredit.Status)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), updatedRider.DepositAmount)
	require.Equal(t, int64(0), updatedRider.FrozenDeposit)
	require.Equal(t, RiderStatusApproved, updatedRider.Status)

	logs, err := testStore.ListRiderDeposits(context.Background(), ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   20,
		Offset:  0,
	})
	require.NoError(t, err)

	var foundWithdraw bool
	var foundDeduct bool
	for _, log := range logs {
		if !log.PaymentOrderID.Valid || log.PaymentOrderID.Int64 != paymentOrder.ID {
			continue
		}
		if log.Type == "withdraw" {
			foundWithdraw = true
			require.Equal(t, int64(10000), log.Amount)
			require.Equal(t, int64(20000), log.BalanceAfter)
		}
		if log.Type == "deduct" {
			foundDeduct = true
			require.Equal(t, int64(20000), log.Amount)
			require.Equal(t, int64(0), log.BalanceAfter)
		}
	}
	require.True(t, foundWithdraw)
	require.True(t, foundDeduct)
}
