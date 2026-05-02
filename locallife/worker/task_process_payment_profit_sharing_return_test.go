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
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type profitSharingReturnWorkerFlowRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
	resultPayloads []*worker.ProfitSharingReturnResultPayload
	callKinds      []string
}

func (r *profitSharingReturnWorkerFlowRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	r.callKinds = append(r.callKinds, "application")
	return nil
}

func (r *profitSharingReturnWorkerFlowRecorder) DistributeTaskProcessProfitSharingReturnResult(_ context.Context, payload *worker.ProfitSharingReturnResultPayload, _ ...asynq.Option) error {
	r.resultPayloads = append(r.resultPayloads, payload)
	r.callKinds = append(r.callKinds, "result")
	return nil
}

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
	expectWorkerProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusUnknown, "NOT_ENOUGH", 9701)
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

func TestProcessTaskInitiateRefund_ProfitSharingReturnProcessingRecordsAcceptedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             12,
		OutTradeNo:     "PAY_12",
		Amount:         1000,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 23, Valid: true},
	}
	order := db.Order{ID: 23, MerchantID: 34}
	refundOrder := db.RefundOrder{ID: 45, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF12_23"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 56,
		MerchantID:         order.MerchantID,
		OutOrderNo:         "PS12",
		SharingOrderID:     pgtype.Text{String: "wx-ps-012", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   67,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-012",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR45PL",
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-012"}, nil)
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
		SubMchid:             "sub-mchid-012",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-012",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-012")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
			require.Equal(t, "sub-mchid-012", req.SubMchID)
			require.Equal(t, profitSharingOrder.OutOrderNo, req.OutOrderNo)
			require.Equal(t, returnRecord.OutReturnNo, req.OutReturnNo)
			require.Equal(t, "service-mchid-012", req.ReturnMchID)
			return &wechatcontracts.ProfitSharingReturnResponse{ReturnID: "wx-return-012", Result: "PROCESSING"}, nil
		},
	)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-012", Valid: true},
	}).Return(returnRecord, nil)
	expectWorkerProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "wx-return-012", db.ExternalPaymentCommandStatusAccepted, "", 9702)
	distributor.EXPECT().DistributeTaskProcessProfitSharingReturnResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingReturnResultPayload{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ProfitSharingReturnResultPayload, _ ...asynq.Option) error {
			require.Equal(t, returnRecord.ID, payload.ProfitSharingReturnID)
			require.Equal(t, returnRecord.OutReturnNo, payload.OutReturnNo)
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

func TestProcessTaskInitiateRefund_OrdinaryProfitSharingReturnUsesOrdinaryServiceProviderMchID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ordinaryClient := &refundRecoveryOrdinaryClient{
		createProfitSharingReturnResponse: &ospcontracts.ProfitSharingReturnResponse{
			SubMchID:    "sub-mchid-ordinary-012",
			OrderID:     "wx-ps-ordinary-012",
			OutOrderNo:  "PSO12",
			OutReturnNo: "PR145PL",
			ReturnID:    "wx-return-ordinary-012",
			ReturnMchID: "1900000109",
			Amount:      300,
			State:       ospcontracts.ProfitSharingReturnStateProcessing,
		},
	}

	paymentOrder := db.PaymentOrder{
		ID:             111,
		OutTradeNo:     "PAY_ORDINARY_111",
		Amount:         1000,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		OrderID:        pgtype.Int8{Int64: 122, Valid: true},
	}
	order := db.Order{ID: 122, MerchantID: 133}
	refundOrder := db.RefundOrder{ID: 145, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF111_122"}
	profitSharingOrder := db.ProfitSharingOrder{ID: 155, MerchantID: order.MerchantID, OutOrderNo: "PSO12", SharingOrderID: pgtype.Text{String: "wx-ps-ordinary-012", Valid: true}, PlatformCommission: 300}
	returnRecord := db.ProfitSharingReturn{ID: 166, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-ordinary-012", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR145PL"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-ordinary-012"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{PaymentOrderID: paymentOrder.ID, RefundType: "user_cancel", RefundAmount: 300, RefundReason: "用户取消", OutRefundNo: refundOrder.OutRefundNo}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-ordinary-012", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, ReturnMchid: ordinaryClient.ServiceProviderMchID(), Amount: profitSharingOrder.PlatformCommission, Status: "pending"}).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{ID: returnRecord.ID, ReturnID: pgtype.Text{String: "wx-return-ordinary-012", Valid: true}}).Return(returnRecord, nil)
	expectWorkerProfitSharingReturnCommandForChannel(t, store, db.PaymentChannelOrdinaryServiceProvider, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "wx-return-ordinary-012", db.ExternalPaymentCommandStatusAccepted, "", 9712)
	distributor.EXPECT().DistributeTaskProcessProfitSharingReturnResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingReturnResultPayload{}), gomock.Any()).Return(nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	processor.SetOrdinaryServiceProviderClient(ordinaryClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Reason: "用户取消"})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.NotNil(t, ordinaryClient.createProfitSharingReturnRequest)
	require.Equal(t, ordinaryClient.ServiceProviderMchID(), ordinaryClient.createProfitSharingReturnRequest.ReturnMchID)
	require.Equal(t, returnRecord.SubMchid, ordinaryClient.createProfitSharingReturnRequest.SubMchID)
}

func TestProcessTaskInitiateRefund_ProfitSharingReturnSuccessSchedulesQueryAndSkipsDirectRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingReturnWorkerFlowRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             13,
		OutTradeNo:     "PAY_13",
		Amount:         1000,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		TransactionID:  pgtype.Text{String: "wx-txn-013", Valid: true},
		OrderID:        pgtype.Int8{Int64: 24, Valid: true},
	}
	order := db.Order{ID: 24, MerchantID: 35}
	refundOrder := db.RefundOrder{ID: 46, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF13_24"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 57,
		MerchantID:         order.MerchantID,
		OutOrderNo:         "PS13",
		SharingOrderID:     pgtype.Text{String: "wx-ps-013", Valid: true},
		PlatformCommission: 300,
	}
	platformReturnRecord := db.ProfitSharingReturn{
		ID:                   68,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-013",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR46PL",
		Amount:               profitSharingOrder.PlatformCommission,
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-013"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   500,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), platformReturnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-013",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          platformReturnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-013",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(platformReturnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-013")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
			require.Equal(t, platformReturnRecord.OutReturnNo, req.OutReturnNo)
			return &wechatcontracts.ProfitSharingReturnResponse{
				SubMchID:    platformReturnRecord.SubMchid,
				OutOrderNo:  platformReturnRecord.OutOrderNo,
				OutReturnNo: platformReturnRecord.OutReturnNo,
				ReturnID:    "wx-return-013-pl",
				Amount:      platformReturnRecord.Amount,
				Result:      "SUCCESS",
			}, nil
		},
	)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       platformReturnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-013-pl", Valid: true},
	}).Return(platformReturnRecord, nil)
	expectWorkerProfitSharingReturnCommand(t, store, platformReturnRecord.ID, platformReturnRecord.OutReturnNo, platformReturnRecord.OutOrderNo, "wx-return-013-pl", db.ExternalPaymentCommandStatusAccepted, "", 9703)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, platformReturnRecord.OutReturnNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 9001, DedupeKey: arg.DedupeKey, IsTerminal: false}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   500,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.Equal(t, []string{"result"}, distributor.callKinds)
	require.Empty(t, distributor.applicationIDs)
	require.Len(t, distributor.resultPayloads, 1)
	require.Equal(t, platformReturnRecord.ID, distributor.resultPayloads[0].ProfitSharingReturnID)
}

func TestProcessTaskInitiateRefund_ProfitSharingReturnRejectedErrorRecordsCommandFactAndFailsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             14,
		OutTradeNo:     "PAY_14",
		Amount:         1000,
		Status:         "paid",
		BusinessType:   "takeout",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 25, Valid: true},
	}
	order := db.Order{ID: 25, MerchantID: 36}
	refundOrder := db.RefundOrder{ID: 47, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF14_25"}
	profitSharingOrder := db.ProfitSharingOrder{ID: 58, MerchantID: order.MerchantID, OutOrderNo: "PS14", SharingOrderID: pgtype.Text{String: "wx-ps-014", Valid: true}, PlatformCommission: 300}
	returnRecord := db.ProfitSharingReturn{ID: 70, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-014", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR47PL", Amount: profitSharingOrder.PlatformCommission}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-014"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{PaymentOrderID: paymentOrder.ID, RefundType: "user_cancel", RefundAmount: 300, RefundReason: "用户取消", OutRefundNo: refundOrder.OutRefundNo}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-014", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, ReturnMchid: "service-mchid-014", Amount: profitSharingOrder.PlatformCommission, Status: "pending"}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-014")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: 400, Code: "PAYER_ACCOUNT_ABNORMAL", Message: "分账方账户异常"})
	expectWorkerProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusRejected, "PAYER_ACCOUNT_ABNORMAL", 9705)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 9002, DedupeKey: arg.DedupeKey, IsTerminal: false}, nil
	})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Reason: "用户取消"})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.Error(t, err)
	require.Empty(t, distributor.applicationIDs)
}

func TestProcessTaskInitiateRefund_ProfitSharingReturnFailedResultSchedulesQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 15, OutTradeNo: "PAY_15", Amount: 1000, Status: "paid", BusinessType: "takeout", PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, OrderID: pgtype.Int8{Int64: 26, Valid: true}}
	order := db.Order{ID: 26, MerchantID: 37}
	refundOrder := db.RefundOrder{ID: 48, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF15_26"}
	profitSharingOrder := db.ProfitSharingOrder{ID: 59, MerchantID: order.MerchantID, OutOrderNo: "PS15", SharingOrderID: pgtype.Text{String: "wx-ps-015", Valid: true}, PlatformCommission: 300}
	returnRecord := db.ProfitSharingReturn{ID: 71, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-015", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR48PL", Amount: profitSharingOrder.PlatformCommission}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-015"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{PaymentOrderID: paymentOrder.ID, RefundType: "user_cancel", RefundAmount: 300, RefundReason: "用户取消", OutRefundNo: refundOrder.OutRefundNo}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-015", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, ReturnMchid: "service-mchid-015", Amount: profitSharingOrder.PlatformCommission, Status: "pending"}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-015")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(&wechatcontracts.ProfitSharingReturnResponse{SubMchID: returnRecord.SubMchid, OutOrderNo: returnRecord.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, Amount: returnRecord.Amount, Result: "FAILED", FailReason: "PAYER_ACCOUNT_ABNORMAL"}, nil)
	expectWorkerProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusRejected, "", 9706)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{},
	}).Return(returnRecord, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 9003, DedupeKey: arg.DedupeKey, IsTerminal: false}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Reason: "用户取消"})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.Empty(t, distributor.applicationIDs)
}

func TestProcessTaskInitiateRefund_ProfitSharingReturnUnknownResultFallsBackToPollingAndRecordsFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingReturnWorkerFlowRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 16, OutTradeNo: "PAY_16", Amount: 1000, Status: "paid", BusinessType: "takeout", PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, OrderID: pgtype.Int8{Int64: 27, Valid: true}}
	order := db.Order{ID: 27, MerchantID: 38}
	refundOrder := db.RefundOrder{ID: 49, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF16_27"}
	profitSharingOrder := db.ProfitSharingOrder{ID: 60, MerchantID: order.MerchantID, OutOrderNo: "PS16", SharingOrderID: pgtype.Text{String: "wx-ps-016", Valid: true}, PlatformCommission: 300}
	returnRecord := db.ProfitSharingReturn{ID: 72, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-016", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR49PL", Amount: profitSharingOrder.PlatformCommission}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-016"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{PaymentOrderID: paymentOrder.ID, RefundType: "user_cancel", RefundAmount: 300, RefundReason: "用户取消", OutRefundNo: refundOrder.OutRefundNo}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-016", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, ReturnMchid: "service-mchid-016", Amount: profitSharingOrder.PlatformCommission, Status: "pending"}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-016")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(&wechatcontracts.ProfitSharingReturnResponse{SubMchID: returnRecord.SubMchid, OutOrderNo: returnRecord.OutOrderNo, OutReturnNo: returnRecord.OutReturnNo, ReturnID: "wx-return-016", Amount: returnRecord.Amount, Result: "NEW_STATE"}, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{ID: returnRecord.ID, ReturnID: pgtype.Text{String: "wx-return-016", Valid: true}}).Return(returnRecord, nil)
	expectWorkerProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusUnknown, "", 9707)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 9004, DedupeKey: arg.DedupeKey, IsTerminal: false}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Reason: "用户取消"})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.Empty(t, distributor.applicationIDs)
	require.Len(t, distributor.resultPayloads, 1)
	require.Equal(t, returnRecord.ID, distributor.resultPayloads[0].ProfitSharingReturnID)
}

func TestProcessTaskProfitSharingReturnResult_SuccessRecordsQueryFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:             168,
		RefundOrderID:  268,
		PaymentOrderID: 368,
		SubMchid:       "sub-mchid-068",
		OutOrderNo:     "PS68",
		OutReturnNo:    "PR68PL",
		Amount:         300,
	}

	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(returnRecord, nil)
	ecommerceClient.EXPECT().QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    returnRecord.SubMchid,
		OutOrderNo:  returnRecord.OutOrderNo,
		OutReturnNo: returnRecord.OutReturnNo,
		ReturnID:    "wx-return-068",
		Amount:      returnRecord.Amount,
		Result:      "SUCCESS",
	}, nil)
	expectProfitSharingReturnQueryFact(t, store, returnRecord, "wx-return-068", "SUCCESS", db.ExternalPaymentTerminalStatusSuccess, "")

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingReturnResultPayload{
		ProfitSharingReturnID: returnRecord.ID,
		OutReturnNo:           returnRecord.OutReturnNo,
		OutOrderNo:            returnRecord.OutOrderNo,
		SubMchID:              returnRecord.SubMchid,
		RefundOrderID:         returnRecord.RefundOrderID,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReturnResult(context.Background(), asynq.NewTask(worker.TaskProcessProfitSharingReturnResult, payloadBytes))
	require.NoError(t, err)
	require.Len(t, distributor.applicationIDs, 1)
	require.Equal(t, int64(10168), distributor.applicationIDs[0])
}

func TestProcessTaskProfitSharingReturnResult_FailedRecordsQueryFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:             169,
		RefundOrderID:  269,
		PaymentOrderID: 369,
		SubMchid:       "sub-mchid-069",
		OutOrderNo:     "PS69",
		OutReturnNo:    "PR69PL",
		Amount:         260,
	}

	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(returnRecord, nil)
	ecommerceClient.EXPECT().QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    returnRecord.SubMchid,
		OutOrderNo:  returnRecord.OutOrderNo,
		OutReturnNo: returnRecord.OutReturnNo,
		Amount:      returnRecord.Amount,
		Result:      "FAILED",
		FailReason:  "PAYER_ACCOUNT_ABNORMAL",
	}, nil)
	expectProfitSharingReturnQueryFact(t, store, returnRecord, "", "FAILED", db.ExternalPaymentTerminalStatusFailed, "PAYER_ACCOUNT_ABNORMAL")

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingReturnResultPayload{
		ProfitSharingReturnID: returnRecord.ID,
		OutReturnNo:           returnRecord.OutReturnNo,
		OutOrderNo:            returnRecord.OutOrderNo,
		SubMchID:              returnRecord.SubMchid,
		RefundOrderID:         returnRecord.RefundOrderID,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReturnResult(context.Background(), asynq.NewTask(worker.TaskProcessProfitSharingReturnResult, payloadBytes))
	require.NoError(t, err)
	require.Len(t, distributor.applicationIDs, 1)
	require.Equal(t, int64(10169), distributor.applicationIDs[0])
}

func TestProcessTaskProfitSharingReturnResult_ProcessingMaxRetriesKeepsProcessingForRecoveryScheduler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:             170,
		RefundOrderID:  270,
		PaymentOrderID: 370,
		SubMchid:       "sub-mchid-070",
		OutOrderNo:     "PS70",
		OutReturnNo:    "PR70PL",
		Amount:         280,
	}

	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(returnRecord, nil)
	ecommerceClient.EXPECT().QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    returnRecord.SubMchid,
		OutOrderNo:  returnRecord.OutOrderNo,
		OutReturnNo: returnRecord.OutReturnNo,
		ReturnID:    "wx-return-070",
		Amount:      returnRecord.Amount,
		Result:      "PROCESSING",
	}, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-070", Valid: true},
	}).Return(returnRecord, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingReturnResultPayload{
		ProfitSharingReturnID: returnRecord.ID,
		OutReturnNo:           returnRecord.OutReturnNo,
		OutOrderNo:            returnRecord.OutOrderNo,
		SubMchID:              returnRecord.SubMchid,
		RefundOrderID:         returnRecord.RefundOrderID,
		RetryCount:            6,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReturnResult(context.Background(), asynq.NewTask(worker.TaskProcessProfitSharingReturnResult, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskProfitSharingReturnResult_UnknownResultSkipsRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:             171,
		RefundOrderID:  271,
		PaymentOrderID: 371,
		SubMchid:       "sub-mchid-071",
		OutOrderNo:     "PS71",
		OutReturnNo:    "PR71PL",
		Amount:         280,
	}

	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(returnRecord, nil)
	ecommerceClient.EXPECT().QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    returnRecord.SubMchid,
		OutOrderNo:  returnRecord.OutOrderNo,
		OutReturnNo: returnRecord.OutReturnNo,
		ReturnID:    "wx-return-071",
		Amount:      returnRecord.Amount,
		Result:      "UNSUPPORTED_STATUS",
	}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingReturnResultPayload{
		ProfitSharingReturnID: returnRecord.ID,
		OutReturnNo:           returnRecord.OutReturnNo,
		OutOrderNo:            returnRecord.OutOrderNo,
		SubMchID:              returnRecord.SubMchid,
		RefundOrderID:         returnRecord.RefundOrderID,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReturnResult(context.Background(), asynq.NewTask(worker.TaskProcessProfitSharingReturnResult, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown profit sharing return result")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}

func expectWorkerProfitSharingReturnCommand(t *testing.T, store *mockdb.MockStore, returnID int64, outReturnNo string, outOrderNo string, secondaryKey string, status string, errorCode string, commandID int64) {
	expectWorkerProfitSharingReturnCommandForChannel(t, store, db.PaymentChannelEcommerce, returnID, outReturnNo, outOrderNo, secondaryKey, status, errorCode, commandID)
}

func expectWorkerProfitSharingReturnCommandForChannel(t *testing.T, store *mockdb.MockStore, channel string, returnID int64, outReturnNo string, outOrderNo string, secondaryKey string, status string, errorCode string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, channel, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateProfitSharingReturn, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "profit_sharing_return", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, returnID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectProfitSharingReturn, arg.ExternalObjectType)
		require.Equal(t, outReturnNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		snapshot := string(arg.ResponseSnapshot)
		require.Contains(t, snapshot, outReturnNo)
		require.Contains(t, snapshot, outOrderNo)
		if secondaryKey != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
			require.Contains(t, snapshot, secondaryKey)
		} else {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		}
		if errorCode != "" {
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, snapshot, errorCode)
		}
		require.NotContains(t, snapshot, "ReturnMchID")
		require.NotContains(t, snapshot, "return_mchid")
		require.NotContains(t, snapshot, "receiver")
		require.NotContains(t, snapshot, "encrypted")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}

func expectProfitSharingReturnQueryFact(t *testing.T, store *mockdb.MockStore, returnRecord db.ProfitSharingReturn, secondaryKey string, upstreamState string, terminalStatus string, failReason string) {
	expectProfitSharingReturnQueryFactForChannel(t, store, db.PaymentChannelEcommerce, returnRecord, secondaryKey, upstreamState, terminalStatus, failReason)
}

func expectProfitSharingReturnQueryFactForChannel(t *testing.T, store *mockdb.MockStore, channel string, returnRecord db.ProfitSharingReturn, secondaryKey string, upstreamState string, terminalStatus string, failReason string) {
	t.Helper()

	factID := int64(9000) + returnRecord.ID
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, channel, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectProfitSharingReturn, arg.ExternalObjectType)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		if secondaryKey == "" {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		} else {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
		}
		require.True(t, arg.BusinessOwner.Valid)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner.String)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "profit_sharing_return", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, returnRecord.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, upstreamState, arg.UpstreamState)
		require.Equal(t, terminalStatus, arg.TerminalStatus)
		require.True(t, arg.Amount.Valid)
		require.Equal(t, returnRecord.Amount, arg.Amount.Int64)
		require.Equal(t, "wechat:query:"+channel+":profit_sharing_return:"+returnRecord.OutReturnNo+":"+terminalStatus, arg.DedupeKey)
		raw := string(arg.RawResource)
		require.Contains(t, raw, returnRecord.OutReturnNo)
		require.Contains(t, raw, upstreamState)
		if secondaryKey != "" {
			require.Contains(t, raw, secondaryKey)
		}
		if failReason != "" {
			require.Contains(t, raw, failReason)
		}
		return db.ExternalPaymentFact{ID: factID, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})

	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             factID,
		Consumer:           "profit_sharing_domain",
		BusinessObjectType: "profit_sharing_return",
		BusinessObjectID:   returnRecord.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 factID + 1000,
		FactID:             factID,
		Consumer:           "profit_sharing_domain",
		BusinessObjectType: "profit_sharing_return",
		BusinessObjectID:   returnRecord.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
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
