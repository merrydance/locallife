package db

import (
	"context"
	"testing"

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
