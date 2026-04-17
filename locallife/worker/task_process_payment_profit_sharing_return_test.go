package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_ProfitSharingReturnAmbiguousErrorFallsBackToPolling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             11,
		OutTradeNo:     "PAY_11",
		Amount:         1000,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 22, Valid: true},
	}
	order := db.Order{ID: 22, MerchantID: 33}
	refundOrder := db.RefundOrder{ID: 44, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF11_22"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 55,
		MerchantID:         order.MerchantID,
		OutOrderNo:         "PS11",
		SharingOrderID:     pgtype.Text{String: "wx-ps-001", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   66,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-001",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR44PL",
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-001"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   300,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-001",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-001",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-001")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
			require.Equal(t, "sub-mchid-001", req.SubMchID)
			require.Equal(t, profitSharingOrder.OutOrderNo, req.OutOrderNo)
			require.Equal(t, returnRecord.OutReturnNo, req.OutReturnNo)
			require.Equal(t, "service-mchid-001", req.ReturnMchID)
			return nil, &wechat.WechatPayError{Code: "NOT_ENOUGH", Message: "余额不足", StatusCode: 400}
		},
	)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{},
	}).Return(returnRecord, nil)
	distributor.EXPECT().DistributeTaskProcessProfitSharingReturnResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingReturnResultPayload{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ProfitSharingReturnResultPayload, _ ...asynq.Option) error {
			require.Equal(t, returnRecord.ID, payload.ProfitSharingReturnID)
			require.Equal(t, returnRecord.OutReturnNo, payload.OutReturnNo)
			require.Equal(t, returnRecord.OutOrderNo, payload.OutOrderNo)
			require.Equal(t, returnRecord.SubMchid, payload.SubMchID)
			require.Equal(t, refundOrder.ID, payload.RefundOrderID)
			return nil
		},
	)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   300,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_BlocksPersonalProfitSharingReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	paymentOrder := db.PaymentOrder{
		ID:             21,
		OutTradeNo:     "PAY_21",
		Amount:         3200,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 31, Valid: true},
	}
	order := db.Order{ID: 31, MerchantID: 41}
	refundOrder := db.RefundOrder{ID: 51, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF21_31"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             61,
		MerchantID:     order.MerchantID,
		OutOrderNo:     "PS21",
		SharingOrderID: pgtype.Text{String: "wx-ps-021", Valid: true},
		RiderAmount:    800,
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-021"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   800,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   800,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单包含个人分账，当前不支持自动退款")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestProcessTaskInitiateRefund_BlocksPersonalOperatorProfitSharingReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	paymentOrder := db.PaymentOrder{
		ID:             22,
		OutTradeNo:     "PAY_22",
		Amount:         4200,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 32, Valid: true},
	}
	order := db.Order{ID: 32, MerchantID: 42}
	refundOrder := db.RefundOrder{ID: 52, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF22_32"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 62,
		MerchantID:         order.MerchantID,
		OutOrderNo:         "PS22",
		SharingOrderID:     pgtype.Text{String: "wx-ps-022", Valid: true},
		OperatorID:         pgtype.Int8{Int64: 72, Valid: true},
		OperatorCommission: 900,
	}
	operator := db.Operator{ID: 72, UserID: 502, Name: "个人运营商"}
	operatorUser := db.User{ID: operator.UserID, WechatOpenid: "operator_openid_72"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-022"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   900,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil)
	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(operatorUser, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   900,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单包含个人分账，当前不支持自动退款")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}
