package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomRefundOrder(t *testing.T, paymentOrderID int64, refundAmount int64) RefundOrder {
	outRefundNo := util.RandomString(32)

	arg := CreateRefundOrderParams{
		PaymentOrderID: paymentOrderID,
		RefundType:     "miniprogram", // 有效值: 'miniprogram' 或 'profit_sharing'
		RefundAmount:   refundAmount,
		RefundReason:   pgtype.Text{String: "Test refund", Valid: true},
		OutRefundNo:    outRefundNo,
		PlatformRefund: pgtype.Int8{Int64: 0, Valid: true},
		OperatorRefund: pgtype.Int8{Int64: 0, Valid: true},
		MerchantRefund: pgtype.Int8{Int64: refundAmount, Valid: true},
		Status:         "pending",
	}

	refund, err := testStore.CreateRefundOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, refund)

	require.Equal(t, arg.PaymentOrderID, refund.PaymentOrderID)
	require.Equal(t, arg.OutRefundNo, refund.OutRefundNo)
	require.Equal(t, arg.RefundAmount, refund.RefundAmount)
	require.Equal(t, arg.Status, refund.Status)
	require.NotZero(t, refund.ID)
	require.NotZero(t, refund.CreatedAt)

	return refund
}

// ==================== Refund Order Tests ====================

func TestCreateRefundOrder(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	createRandomRefundOrder(t, payment.ID, payment.Amount/2)
}

func TestGetRefundOrder(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund1 := createRandomRefundOrder(t, payment.ID, payment.Amount)

	refund2, err := testStore.GetRefundOrder(context.Background(), refund1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refund2)

	require.Equal(t, refund1.ID, refund2.ID)
	require.Equal(t, refund1.OutRefundNo, refund2.OutRefundNo)
	require.Equal(t, refund1.PaymentOrderID, refund2.PaymentOrderID)
	require.Equal(t, refund1.RefundAmount, refund2.RefundAmount)
}

func TestGetRefundOrderByOutRefundNo(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund1 := createRandomRefundOrder(t, payment.ID, payment.Amount)

	refund2, err := testStore.GetRefundOrderByOutRefundNo(context.Background(), refund1.OutRefundNo)
	require.NoError(t, err)
	require.NotEmpty(t, refund2)

	require.Equal(t, refund1.ID, refund2.ID)
	require.Equal(t, refund1.OutRefundNo, refund2.OutRefundNo)
}

func TestListRefundOrdersByPaymentOrder(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	// 创建多个退款订单
	for i := 0; i < 2; i++ {
		createRandomRefundOrder(t, payment.ID, 100)
	}

	refunds, err := testStore.ListRefundOrdersByPaymentOrder(context.Background(), payment.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(refunds), 2)

	for _, refund := range refunds {
		require.Equal(t, payment.ID, refund.PaymentOrderID)
	}
}

func TestGetTotalRefundedByPaymentOrder(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	// 创建成功的退款订单
	refundAmount1 := int64(100)
	refundAmount2 := int64(200)

	refund1 := createRandomRefundOrder(t, payment.ID, refundAmount1)
	refund2 := createRandomRefundOrder(t, payment.ID, refundAmount2)

	// 设置退款状态为 processing，然后 success
	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), UpdateRefundOrderToProcessingParams{
		ID:       refund1.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRefundOrderToSuccess(context.Background(), refund1.ID)
	require.NoError(t, err)

	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), UpdateRefundOrderToProcessingParams{
		ID:       refund2.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRefundOrderToSuccess(context.Background(), refund2.ID)
	require.NoError(t, err)

	totalRefunded, err := testStore.GetTotalRefundedByPaymentOrder(context.Background(), payment.ID)
	require.NoError(t, err)
	require.Equal(t, refundAmount1+refundAmount2, totalRefunded)
}

func TestUpdateRefundOrderToProcessing(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund := createRandomRefundOrder(t, payment.ID, payment.Amount)
	require.Equal(t, "pending", refund.Status)

	arg := UpdateRefundOrderToProcessingParams{
		ID:       refund.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	}

	updatedRefund, err := testStore.UpdateRefundOrderToProcessing(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedRefund)

	require.Equal(t, refund.ID, updatedRefund.ID)
	require.Equal(t, "processing", updatedRefund.Status)
}

func TestUpdateRefundOrderToSuccess(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund := createRandomRefundOrder(t, payment.ID, payment.Amount)

	// 先设为 processing
	arg := UpdateRefundOrderToProcessingParams{
		ID:       refund.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	}
	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), arg)
	require.NoError(t, err)

	// 再设为 success
	updatedRefund, err := testStore.UpdateRefundOrderToSuccess(context.Background(), refund.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedRefund)

	require.Equal(t, refund.ID, updatedRefund.ID)
	require.Equal(t, "success", updatedRefund.Status)
	require.True(t, updatedRefund.RefundedAt.Valid)
}

func TestUpdateRefundOrderToFailed(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund := createRandomRefundOrder(t, payment.ID, payment.Amount)

	updatedRefund, err := testStore.UpdateRefundOrderToFailed(context.Background(), refund.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedRefund)

	require.Equal(t, refund.ID, updatedRefund.ID)
	require.Equal(t, "failed", updatedRefund.Status)
}

func TestUpdateRefundOrderToClosed(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund := createRandomRefundOrder(t, payment.ID, payment.Amount)

	updatedRefund, err := testStore.UpdateRefundOrderToClosed(context.Background(), refund.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedRefund)

	require.Equal(t, refund.ID, updatedRefund.ID)
	require.Equal(t, "closed", updatedRefund.Status)
}

func TestListRefundOrdersByStatus(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	// 创建多个 pending 状态的退款
	for i := 0; i < 2; i++ {
		createRandomRefundOrder(t, payment.ID, 100)
	}

	arg := ListRefundOrdersByStatusParams{
		Status: "pending",
		Limit:  100,
		Offset: 0,
	}

	refunds, err := testStore.ListRefundOrdersByStatus(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(refunds), 2)

	for _, refund := range refunds {
		require.Equal(t, "pending", refund.Status)
	}
}

func TestGetRefundOrderForUpdate(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)

	// 先将支付单设为已支付
	_, err := testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	refund1 := createRandomRefundOrder(t, payment.ID, payment.Amount)

	refund2, err := testStore.GetRefundOrderForUpdate(context.Background(), refund1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refund2)

	require.Equal(t, refund1.ID, refund2.ID)
	require.Equal(t, refund1.OutRefundNo, refund2.OutRefundNo)
}
