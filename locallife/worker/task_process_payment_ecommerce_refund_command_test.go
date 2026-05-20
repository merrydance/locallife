package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_BaofuAggregateRefundAcceptedSkipsProfitSharingLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{refundResult: &aggregatecontracts.RefundResult{
		OriginTradeNo:    "BFPAY_UP_2819",
		OutTradeNo:       "RF2819_3719",
		TradeNo:          "BFREFUND_UP_2819",
		RefundAmountFen:  1459,
		TotalAmountFen:   1459,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		RefundState:      aggregatecontracts.RefundStateAccepted,
		SuccessAmountFen: 1459,
	}}

	paymentOrder := db.PaymentOrder{
		ID:             2819,
		OutTradeNo:     "BFPAY_2819",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_2819", Valid: true},
		Amount:         1459,
		Status:         "paid",
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OrderID:        pgtype.Int8{Int64: 3719, Valid: true},
	}
	order := db.Order{ID: 3719, MerchantID: 2719}
	refundOrder := db.RefundOrder{ID: 3819, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF2819_3719"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   1459,
		RefundReason:   "配送时间太长",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "BFREFUND_UP_2819", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND_UP_2819", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9801)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   1459,
		Reason:         "配送时间太长",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.True(t, baofuClient.called)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRefundRequest.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastRefundRequest.TerminalID)
	require.Equal(t, "BFPAY_UP_2819", baofuClient.lastRefundRequest.OriginTradeNo)
	require.Empty(t, baofuClient.lastRefundRequest.OriginOutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/refund", baofuClient.lastRefundRequest.NotifyURL)
	require.Equal(t, int64(1459), baofuClient.lastRefundRequest.RefundAmountFen)
	require.Equal(t, int64(1459), baofuClient.lastRefundRequest.TotalAmountFen)
	require.Equal(t, "配送时间太长", baofuClient.lastRefundRequest.RefundReason)
	require.Empty(t, baofuClient.lastRefundRequest.SharingRefundInfo)
	require.Zero(t, baofuClient.lastRefundRequest.AdvanceAmountFen)
}

func TestProcessTaskInitiateRefund_BaofuAggregatePartialRefundUsesRefundAmountAsTotal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{refundResult: &aggregatecontracts.RefundResult{
		OriginTradeNo:    "BFPAY_UP_2820",
		OutTradeNo:       "RF2820_3720",
		TradeNo:          "BFREFUND_UP_2820",
		RefundAmountFen:  500,
		TotalAmountFen:   500,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		RefundState:      aggregatecontracts.RefundStateAccepted,
		SuccessAmountFen: 500,
	}}

	paymentOrder := db.PaymentOrder{
		ID:             2820,
		OutTradeNo:     "BFPAY_2820",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_2820", Valid: true},
		Amount:         1459,
		Status:         "paid",
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OrderID:        pgtype.Int8{Int64: 3720, Valid: true},
	}
	order := db.Order{ID: 3720, MerchantID: 2720}
	refundOrder := db.RefundOrder{ID: 3820, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF2820_3720"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   500,
		RefundReason:   "商品售罄",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "BFREFUND_UP_2820", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND_UP_2820", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9802)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   500,
		Reason:         "商品售罄",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.NoError(t, err)
	require.True(t, baofuClient.called)
	require.Equal(t, int64(500), baofuClient.lastRefundRequest.RefundAmountFen)
	require.Equal(t, int64(500), baofuClient.lastRefundRequest.TotalAmountFen)
	require.Equal(t, "商品售罄", baofuClient.lastRefundRequest.RefundReason)
}

func TestProcessTaskInitiateRefund_BaofuAggregateProviderErrorRecordsGuidanceNotRawText(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{
		err: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "REFUND_AMT_EXCEEDS",
			UpstreamMessage: "raw upstream refund amount detail",
			Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "raw upstream refund amount detail").FrontendGuidance(),
		},
	}

	paymentOrder := db.PaymentOrder{
		ID:             2821,
		OutTradeNo:     "BFPAY_2821",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_2821", Valid: true},
		Amount:         1459,
		Status:         "paid",
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		OrderID:        pgtype.Int8{Int64: 3721, Valid: true},
	}
	order := db.Order{ID: 3721, MerchantID: 2721}
	refundOrder := db.RefundOrder{ID: 3821, PaymentOrderID: paymentOrder.ID, Status: "pending", OutRefundNo: "RF2821_3721"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   1459,
		RefundReason:   "商品售罄",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
		require.True(t, arg.RejectedAt.Valid)
		require.True(t, arg.LastErrorCode.Valid)
		require.Equal(t, "REFUND_AMT_EXCEEDS", arg.LastErrorCode.String)
		require.True(t, arg.LastErrorMessage.Valid)
		require.Equal(t, "资料信息不完整，请核对后重新提交，check_and_resubmit", arg.LastErrorMessage.String)
		require.NotContains(t, arg.LastErrorMessage.String, "raw upstream")
		require.Contains(t, string(arg.ResponseSnapshot), refundOrder.OutRefundNo)
		require.Contains(t, string(arg.ResponseSnapshot), "REFUND_AMT_EXCEEDS")
		require.NotContains(t, string(arg.ResponseSnapshot), "raw upstream")
		return db.ExternalPaymentCommand{ID: 9803}, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   1459,
		Reason:         "商品售罄",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskInitiateRefund(context.Background(), asynq.NewTask(worker.TaskProcessRefund, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "call baofu refund API")
}

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
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderWechat, channel, capability, refundOrderID, outRefundNo, refundID, businessOwner, db.ExternalPaymentCommandStatusAccepted, "", commandID)
}

func expectWorkerExternalRefundCommand(t *testing.T, store *mockdb.MockStore, provider string, channel string, capability string, refundOrderID int64, outRefundNo string, refundID string, businessOwner string, status string, errorCode string, commandID int64) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, provider, arg.Provider)
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
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outRefundNo)
		if refundID != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
			require.Contains(t, string(arg.ResponseSnapshot), refundID)
		}
		if errorCode != "" {
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), errorCode)
		}
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}

type fakeWorkerBaofuRefundClient struct {
	called            bool
	lastRefundRequest aggregatecontracts.RefundBeforeShareRequest
	refundResult      *aggregatecontracts.RefundResult
	err               error
}

func (c *fakeWorkerBaofuRefundClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryPayment(context.Context, aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CreateRefund(_ context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	c.called = true
	c.lastRefundRequest = req
	if c.err != nil {
		return nil, c.err
	}
	return c.refundResult, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, nil
}
