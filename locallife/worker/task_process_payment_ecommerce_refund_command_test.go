package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_EcommerceRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             812,
		OutTradeNo:     "PAY_WORKER_REFUND_ACCEPTED",
		Amount:         2000,
		Status:         "paid",
		BusinessType:   "order",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 912, Valid: true},
	}
	order := db.Order{ID: 912, MerchantID: 712}
	refundOrder := db.RefundOrder{ID: 1012, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF812_912"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             1112,
		MerchantID:     order.MerchantID,
		OutOrderNo:     "PS_WORKER_REFUND_ACCEPTED",
		SharingOrderID: pgtype.Text{String: "wx-ps-worker-refund", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   500,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-worker-refund"}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
		require.Equal(t, "sub-mchid-worker-refund", req.SubMchID)
		require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
		require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
		require.Equal(t, int64(500), req.RefundAmount)
		return &wechat.EcommerceRefundResponse{RefundID: "erefund_worker_accepted"}, nil
	})
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "erefund_worker_accepted", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerEcommerceRefundAcceptedCommand(t, store, refundOrder.ID, refundOrder.OutRefundNo, "erefund_worker_accepted", db.ExternalPaymentBusinessOwnerOrder, 9601)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   500,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_OrdinaryServiceProviderRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &refundRecoveryOrdinaryClient{createRefundResponse: &ospcontracts.RefundResponse{RefundID: "orefund_worker_accepted"}}

	paymentOrder := db.PaymentOrder{
		ID:             1812,
		OutTradeNo:     "PAY_WORKER_ORDINARY_REFUND_ACCEPTED",
		Amount:         2000,
		Status:         "paid",
		BusinessType:   "order",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		OrderID:        pgtype.Int8{Int64: 1912, Valid: true},
	}
	order := db.Order{ID: 1912, MerchantID: 1712}
	refundOrder := db.RefundOrder{ID: 2012, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF1812_1912"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             2112,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		OutOrderNo:     "PS_WORKER_ORDINARY_REFUND_ACCEPTED",
		SharingOrderID: pgtype.Text{String: "wx-ordinary-ps-worker-refund", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   500,
		RefundReason:   "用户取消",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-worker-ordinary-refund"}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "orefund_worker_accepted", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerExternalRefundAcceptedCommand(t, store, db.PaymentChannelOrdinaryServiceProvider, db.ExternalPaymentCapabilityPartnerRefund, refundOrder.ID, refundOrder.OutRefundNo, "orefund_worker_accepted", db.ExternalPaymentBusinessOwnerOrder, 9701)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetOrdinaryServiceProviderClient(ordinaryClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   500,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.NotNil(t, ordinaryClient.createRefundRequest)
	require.Equal(t, "sub-mchid-worker-ordinary-refund", ordinaryClient.createRefundRequest.SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createRefundRequest.OutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, ordinaryClient.createRefundRequest.OutRefundNo)
	require.Equal(t, "用户取消", ordinaryClient.createRefundRequest.Reason)
	require.Equal(t, ordinaryClient.RefundNotifyURL(), ordinaryClient.createRefundRequest.NotifyURL)
	require.Equal(t, int64(500), ordinaryClient.createRefundRequest.Amount.Refund)
	require.Equal(t, paymentOrder.Amount, ordinaryClient.createRefundRequest.Amount.Total)
	require.Equal(t, ospcontracts.CurrencyCNY, ordinaryClient.createRefundRequest.Amount.Currency)
}

func TestProcessTaskInitiateRefund_EcommerceRefundAPIFailureSkipsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             813,
		OutTradeNo:     "PAY_WORKER_REFUND_RETRYABLE",
		Amount:         2000,
		Status:         "paid",
		BusinessType:   "order",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 913, Valid: true},
	}
	order := db.Order{ID: 913, MerchantID: 713}
	refundOrder := db.RefundOrder{ID: 1013, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF813_913"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             1113,
		MerchantID:     order.MerchantID,
		OutOrderNo:     "PS_WORKER_REFUND_RETRYABLE",
		SharingOrderID: pgtype.Text{String: "wx-ps-worker-retryable", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-worker-retryable"}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: 503, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   500,
		Reason:         "用户取消",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "call wechat ecommerce refund API")
}

func TestProcessTaskAnomalyRefund_EcommerceRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:             814,
		OutTradeNo:     "PAY_WORKER_ANOMALY_REFUND",
		Amount:         2000,
		Status:         "closed",
		BusinessType:   "order",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 914, Valid: true},
	}
	order := db.Order{ID: 914, MerchantID: 714}
	refundOrder := db.RefundOrder{ID: 1014, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "CRF814"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().CreateAnomalyRefundRecord(gomock.Any(), db.CreateAnomalyRefundRecordParams{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   2000,
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(refundOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-worker-anomaly"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
		require.Equal(t, "sub-mchid-worker-anomaly", req.SubMchID)
		require.Equal(t, "wx_tx_worker_anomaly", req.TransactionID)
		require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
		return &wechat.EcommerceRefundResponse{RefundID: "erefund_worker_anomaly"}, nil
	})
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "erefund_worker_anomaly", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerEcommerceRefundAcceptedCommand(t, store, refundOrder.ID, refundOrder.OutRefundNo, "erefund_worker_anomaly", db.ExternalPaymentBusinessOwnerOrder, 9603)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessAnomalyRefund{
		PaymentOrderID: paymentOrder.ID,
		TransactionID:  "wx_tx_worker_anomaly",
		RefundAmount:   2000,
		OutRefundNo:    refundOrder.OutRefundNo,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskAnomalyRefund(context.Background(), asynq.NewTask(worker.TaskProcessAnomalyRefund, payloadBytes))
	require.NoError(t, err)
}

func expectWorkerEcommerceRefundAcceptedCommand(t *testing.T, store *mockdb.MockStore, refundOrderID int64, outRefundNo string, refundID string, businessOwner string, commandID int64) {
	t.Helper()
	expectWorkerExternalRefundAcceptedCommand(t, store, db.PaymentChannelEcommerce, db.ExternalPaymentCapabilityEcommerceRefund, refundOrderID, outRefundNo, refundID, businessOwner, commandID)
}

func expectWorkerExternalRefundAcceptedCommand(t *testing.T, store *mockdb.MockStore, channel string, capability string, refundOrderID int64, outRefundNo string, refundID string, businessOwner string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, channel, arg.Channel)
		require.Equal(t, capability, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
		require.Equal(t, businessOwner, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "refund_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, refundOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
		require.Equal(t, outRefundNo, arg.ExternalObjectKey)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outRefundNo)
		require.Contains(t, string(arg.ResponseSnapshot), refundID)
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}
