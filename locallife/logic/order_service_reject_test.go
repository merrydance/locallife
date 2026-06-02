package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOrderServiceRejectMerchantOrder_ReturnsRefundSubmissionStateWhenRefundNeedsRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 41, UserID: 9, MerchantID: 77, Status: db.OrderStatusPaid}
	cancelled := order
	cancelled.Status = db.OrderStatusCancelled
	paymentOrder := db.PaymentOrder{
		ID:             501,
		OrderID:        pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:   businessTypeOrder,
		Status:         "paid",
		Amount:         1200,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OutTradeNo:     "BF_ORDER_41",
	}
	refundOrder := db.RefundOrder{
		ID:             601,
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   paymentOrder.Amount,
		OutRefundNo:    "RF_ORDER_41",
		Status:         "pending",
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).Return(db.CancelOrderTxResult{Order: cancelled}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)

	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := service.RejectMerchantOrder(context.Background(), MerchantOrderUpdateInput{
		MerchantID: order.MerchantID,
		OrderID:    order.ID,
		OperatorID: 3,
		Reason:     "临时缺货",
	})

	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCancelled, result.Order.Status)
	require.NotNil(t, result.RefundSubmission)
	require.Equal(t, MerchantRefundSubmissionStatusPendingRecovery, result.RefundSubmission.Status)
	require.Equal(t, refundOrder.ID, result.RefundSubmission.RefundOrder.ID)
	require.Contains(t, result.RefundSubmission.Message, "退款")
}

func TestOrderServiceRejectMerchantOrder_ReturnsManualRequiredWhenRefundSubmissionRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 42, UserID: 9, MerchantID: 77, Status: db.OrderStatusPaid}
	cancelled := order
	cancelled.Status = db.OrderStatusCancelled
	paymentOrder := db.PaymentOrder{
		ID:             502,
		OrderID:        pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:   businessTypeOrder,
		Status:         "paid",
		Amount:         1900,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OutTradeNo:     "BF_ORDER_42",
	}
	refundOrder := db.RefundOrder{
		ID:             602,
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   paymentOrder.Amount,
		OutRefundNo:    "RF_ORDER_42",
		Status:         "pending",
	}
	failedRefundOrder := refundOrder
	failedRefundOrder.Status = "failed"

	store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).Return(db.CancelOrderTxResult{Order: cancelled}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(failedRefundOrder, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
			require.Equal(t, refundOrder.ID, arg.BusinessObjectID.Int64)
			return db.ExternalPaymentCommand{ID: 1}, nil
		})
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(failedRefundOrder, nil)

	facade := &merchantRejectRefundPaymentFacade{
		baofuRefundErr: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "RISK_REFUSED",
			UpstreamMessage: "raw upstream detail",
		},
	}
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, facade, nil, nil, nil)

	result, err := service.RejectMerchantOrder(context.Background(), MerchantOrderUpdateInput{
		MerchantID: order.MerchantID,
		OrderID:    order.ID,
		OperatorID: 3,
		Reason:     "临时缺货",
	})

	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCancelled, result.Order.Status)
	require.True(t, facade.baofuRefundCalled)
	require.NotNil(t, result.RefundSubmission)
	require.Equal(t, MerchantRefundSubmissionStatusManualRequired, result.RefundSubmission.Status)
	require.Equal(t, failedRefundOrder.ID, result.RefundSubmission.RefundOrder.ID)
	require.Contains(t, result.RefundSubmission.Message, "请联系平台处理")
}

func TestOrderServiceRejectMerchantOrder_RetryableBaofuRefundErrorNeedsRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 43, UserID: 9, MerchantID: 77, Status: db.OrderStatusPaid}
	cancelled := order
	cancelled.Status = db.OrderStatusCancelled
	paymentOrder := db.PaymentOrder{
		ID:             503,
		OrderID:        pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:   businessTypeOrder,
		Status:         "paid",
		Amount:         2100,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OutTradeNo:     "BF_ORDER_43",
	}
	refundOrder := db.RefundOrder{
		ID:             603,
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   paymentOrder.Amount,
		OutRefundNo:    "RF_ORDER_43",
		Status:         "pending",
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).Return(db.CancelOrderTxResult{Order: cancelled}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
			require.Equal(t, refundOrder.ID, arg.BusinessObjectID.Int64)
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, "SYSTEM_BUSY", arg.LastErrorCode.String)
			return db.ExternalPaymentCommand{ID: 2}, nil
		})
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)

	facade := &merchantRejectRefundPaymentFacade{
		baofuRefundErr: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "SYSTEM_BUSY",
			UpstreamMessage: "raw upstream retryable detail",
		},
	}
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, facade, nil, nil, nil)

	result, err := service.RejectMerchantOrder(context.Background(), MerchantOrderUpdateInput{
		MerchantID: order.MerchantID,
		OrderID:    order.ID,
		OperatorID: 3,
		Reason:     "临时缺货",
	})

	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCancelled, result.Order.Status)
	require.True(t, facade.baofuRefundCalled)
	require.NotNil(t, result.RefundSubmission)
	require.Equal(t, MerchantRefundSubmissionStatusPendingRecovery, result.RefundSubmission.Status)
	require.Equal(t, refundOrder.ID, result.RefundSubmission.RefundOrder.ID)
	require.Contains(t, result.RefundSubmission.Message, "系统会稍后自动重试")
}

func TestOrderServiceRejectMerchantOrder_BaofuOrderExistReturnsAcceptedForQueryRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 44, UserID: 9, MerchantID: 77, Status: db.OrderStatusPaid}
	cancelled := order
	cancelled.Status = db.OrderStatusCancelled
	paymentOrder := db.PaymentOrder{
		ID:             504,
		OrderID:        pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:   businessTypeOrder,
		Status:         "paid",
		Amount:         2300,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OutTradeNo:     "BF_ORDER_44",
	}
	refundOrder := db.RefundOrder{
		ID:             604,
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   paymentOrder.Amount,
		OutRefundNo:    "RF_ORDER_44",
		Status:         "pending",
	}
	processingRefundOrder := refundOrder
	processingRefundOrder.Status = "processing"

	store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).Return(db.CancelOrderTxResult{Order: cancelled}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{},
	}).Return(processingRefundOrder, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
			require.Equal(t, refundOrder.ID, arg.BusinessObjectID.Int64)
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, "ORDER_EXIST", arg.LastErrorCode.String)
			return db.ExternalPaymentCommand{ID: 3}, nil
		})
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(processingRefundOrder, nil)

	facade := &merchantRejectRefundPaymentFacade{
		baofuRefundErr: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "ORDER_EXIST",
			UpstreamMessage: "raw upstream duplicate detail",
		},
	}
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, facade, nil, nil, nil)

	result, err := service.RejectMerchantOrder(context.Background(), MerchantOrderUpdateInput{
		MerchantID: order.MerchantID,
		OrderID:    order.ID,
		OperatorID: 3,
		Reason:     "临时缺货",
	})

	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCancelled, result.Order.Status)
	require.True(t, facade.baofuRefundCalled)
	require.NotNil(t, result.RefundSubmission)
	require.Equal(t, MerchantRefundSubmissionStatusAccepted, result.RefundSubmission.Status)
	require.Equal(t, refundOrder.ID, result.RefundSubmission.RefundOrder.ID)
}

type merchantRejectRefundPaymentFacade struct {
	baofuRefundCalled bool
	baofuRefundErr    error
}

func (f *merchantRejectRefundPaymentFacade) CreatePaymentOrder(context.Context, CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	return CreatePaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) CreateReservationAdjustmentPaymentOrder(context.Context, CreateReservationAdjustmentPaymentInput) (CreatePaymentOrderResult, error) {
	return CreatePaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) CreateCombinedPaymentOrder(context.Context, CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	return CreateCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) GetCombinedPaymentOrder(context.Context, GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error) {
	return GetCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) QueryCombinedPaymentOrder(context.Context, QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	return QueryCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) CloseCombinedPaymentOrder(context.Context, CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	return CloseCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) GetPaymentOrder(context.Context, GetPaymentOrderInput) (GetPaymentOrderResult, error) {
	return GetPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) QueryPaymentOrder(context.Context, QueryPaymentOrderInput) (QueryPaymentOrderResult, error) {
	return QueryPaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) ListPaymentOrders(context.Context, ListPaymentOrdersInput) (ListPaymentOrdersResult, error) {
	return ListPaymentOrdersResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) ListPaymentLedger(context.Context, ListPaymentLedgerInput) (ListPaymentLedgerResult, error) {
	return ListPaymentLedgerResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) ClosePaymentOrder(context.Context, ClosePaymentOrderInput) (ClosePaymentOrderResult, error) {
	return ClosePaymentOrderResult{}, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) CreateRefund(context.Context, *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *merchantRejectRefundPaymentFacade) CreateBaofuRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	f.baofuRefundCalled = true
	return nil, f.baofuRefundErr
}

func (f *merchantRejectRefundPaymentFacade) BaofuRefundNotifyURL() string {
	return "https://api.example.com/v1/webhooks/baofu/refund"
}
