package db

import (
	"context"
	"testing"
	"time"

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

	refundPendingAmount := int64(100)
	refundProcessingAmount := int64(200)
	refundSuccessAmount := int64(300)

	refundPending := createRandomRefundOrder(t, payment.ID, refundPendingAmount)
	refundProcessing := createRandomRefundOrder(t, payment.ID, refundProcessingAmount)
	refundSuccess := createRandomRefundOrder(t, payment.ID, refundSuccessAmount)

	require.Equal(t, "pending", refundPending.Status)

	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), UpdateRefundOrderToProcessingParams{
		ID:       refundProcessing.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRefundOrderToProcessing(context.Background(), UpdateRefundOrderToProcessingParams{
		ID:       refundSuccess.ID,
		RefundID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRefundOrderToSuccess(context.Background(), refundSuccess.ID)
	require.NoError(t, err)

	totalRefunded, err := testStore.GetTotalRefundedByPaymentOrder(context.Background(), payment.ID)
	require.NoError(t, err)
	require.Equal(t, refundPendingAmount+refundProcessingAmount+refundSuccessAmount, totalRefunded)
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
	var pendingCountBefore int64
	err := testStore.(*SQLStore).connPool.QueryRow(context.Background(), `SELECT count(*) FROM refund_orders WHERE status = 'pending'`).Scan(&pendingCountBefore)
	require.NoError(t, err)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	// 先将支付单设为已支付
	_, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	// 创建多个 pending 状态的退款
	var refundIDs []int64
	for i := 0; i < 2; i++ {
		refund := createRandomRefundOrder(t, payment.ID, 100)
		refundIDs = append(refundIDs, refund.ID)
	}

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE refund_orders SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		refundIDs,
	)
	require.NoError(t, err)

	arg := ListRefundOrdersByStatusParams{
		Status: "pending",
		Limit:  2,
		Offset: int32(pendingCountBefore),
	}

	refunds, err := testStore.ListRefundOrdersByStatus(context.Background(), arg)
	require.NoError(t, err)
	require.Len(t, refunds, 2)

	for _, refund := range refunds {
		require.Equal(t, "pending", refund.Status)
	}
	require.Equal(t, refundIDs[0], refunds[0].ID)
	require.Equal(t, refundIDs[1], refunds[1].ID)
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
