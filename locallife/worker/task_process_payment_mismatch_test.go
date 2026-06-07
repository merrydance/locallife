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
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_RiderDepositMismatchRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:           3,
		OutTradeNo:   "RIDER_PAY_3",
		Amount:       10000,
		Status:       "paid",
		BusinessType: "rider_deposit",
		PaymentType:  "miniprogram",
	}
	refundOrder := db.RefundOrder{ID: 33, PaymentOrderID: 3, Status: "pending", OutRefundNo: "RFM3_D"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(3)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), "RFM3_D").
		Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(3), arg.PaymentOrderID)
			require.Equal(t, int64(12000), arg.RefundAmount)
			require.Equal(t, "RFM3_D", arg.OutRefundNo)
			return refundOrder, nil
		})
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
			require.Equal(t, "RIDER_PAY_3", req.OutTradeNo)
			require.Equal(t, "RFM3_D", req.OutRefundNo)
			require.Equal(t, int64(12000), req.RefundAmount)
			require.Equal(t, int64(12000), req.TotalAmount)
			return &wechat.RefundResponse{RefundID: "refund_rider_3", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().
		GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(12000), nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).
		Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil, paymentClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   12000,
		Reason:         "金额异常，系统自动退款",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_ClaimRecoveryDirectRefundUsesDirectPaymentClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:           5,
		OutTradeNo:   "CLAIM_PAY_5",
		Amount:       8800,
		Status:       "paid",
		BusinessType: "claim_recovery",
		PaymentType:  "miniprogram",
		OrderID:      toPgInt8(15),
	}
	order := db.Order{ID: 15, MerchantID: 25}
	refundOrder := db.RefundOrder{ID: 55, PaymentOrderID: 5, Status: "pending", OutRefundNo: "RF5_15"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(5)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(15)).
		Return(order, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), "RF5_15").
		Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
			require.Equal(t, int64(5), arg.PaymentOrderID)
			require.Equal(t, int64(1200), arg.RefundAmount)
			require.Equal(t, "RF5_15", arg.OutRefundNo)
			return db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil
		})
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
			require.Equal(t, "CLAIM_PAY_5", req.OutTradeNo)
			require.Equal(t, "RF5_15", req.OutRefundNo)
			require.Equal(t, int64(1200), req.RefundAmount)
			require.Equal(t, int64(8800), req.TotalAmount)
			return &wechat.RefundResponse{RefundID: "refund_claim_5", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().
		GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(1200), nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil, paymentClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   1200,
		Reason:         "追偿金额异常，系统自动退款",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_OrderRefundUsesProvidedOutRefundNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{refundResult: &aggregatecontracts.RefundResult{
		OriginTradeNo:    "BF_PAY_9",
		OutTradeNo:       "BFRFD_MERCHANT_REJECT_009",
		TradeNo:          "BFREFUND_009",
		RefundAmountFen:  3600,
		TotalAmountFen:   3600,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		RefundState:      aggregatecontracts.RefundStateAccepted,
		SuccessAmountFen: 3600,
	}}

	paymentOrder := db.PaymentOrder{
		ID:                    9,
		OutTradeNo:            "ORDER_PAY_9",
		TransactionID:         pgtype.Text{String: "BF_PAY_9", Valid: true},
		Amount:                3600,
		Status:                "paid",
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:           "profit_sharing",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		OrderID:               toPgInt8(19),
	}
	order := db.Order{ID: 19, MerchantID: 29}
	refundOrder := db.RefundOrder{
		ID:             99,
		PaymentOrderID: paymentOrder.ID,
		Status:         "pending",
		OutRefundNo:    "BFRFD_MERCHANT_REJECT_009",
		RefundAmount:   3600,
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).
		Return(refundOrder, nil)
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: "BFREFUND_009", Valid: true},
		}).
		Return(db.RefundOrder{
			ID:             refundOrder.ID,
			PaymentOrderID: refundOrder.PaymentOrderID,
			Status:         "processing",
			OutRefundNo:    refundOrder.OutRefundNo,
			RefundAmount:   refundOrder.RefundAmount,
		}, nil)
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND_009", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9709)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
		RefundAmount:   refundOrder.RefundAmount,
		Reason:         "商户拒单：临时缺货",
		OutRefundNo:    refundOrder.OutRefundNo,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
	require.True(t, baofuClient.called)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRefundRequest.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastRefundRequest.TerminalID)
	require.Equal(t, "BF_PAY_9", baofuClient.lastRefundRequest.OriginTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
	require.Equal(t, int64(3600), baofuClient.lastRefundRequest.RefundAmountFen)
	require.Equal(t, int64(3600), baofuClient.lastRefundRequest.TotalAmountFen)
}

func TestProcessTaskInitiateRefund_OrderRefundBaofuOrderExistMarksProcessingForQueryRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{
		err: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "ORDER_EXIST",
			UpstreamMessage: "订单已存在，请查询原单",
			Frontend:        baofu.ClassifyBaofuError("ORDER_EXIST", "订单已存在，请查询原单").FrontendGuidance(),
		},
	}

	paymentOrder := db.PaymentOrder{
		ID:                    11,
		OutTradeNo:            "ORDER_PAY_11",
		TransactionID:         pgtype.Text{String: "BF_PAY_11", Valid: true},
		Amount:                3900,
		Status:                "paid",
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:           "profit_sharing",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		OrderID:               toPgInt8(21),
	}
	order := db.Order{ID: 21, MerchantID: 31}
	refundOrder := db.RefundOrder{
		ID:             101,
		PaymentOrderID: paymentOrder.ID,
		Status:         "pending",
		OutRefundNo:    "BFRFD_MERCHANT_REJECT_011",
		RefundAmount:   3900,
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).
		Return(refundOrder, nil)
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{},
		}).
		Return(db.RefundOrder{
			ID:             refundOrder.ID,
			PaymentOrderID: refundOrder.PaymentOrderID,
			Status:         "processing",
			OutRefundNo:    refundOrder.OutRefundNo,
			RefundAmount:   refundOrder.RefundAmount,
		}, nil)
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusUnknown, "ORDER_EXIST", 9711)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
		RefundAmount:   refundOrder.RefundAmount,
		Reason:         "商户拒单：临时缺货",
		OutRefundNo:    refundOrder.OutRefundNo,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
	require.True(t, baofuClient.called)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
}

func TestProcessTaskInitiateRefund_OrderRefundRejectsProvidedOutRefundNoPaymentMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{}

	paymentOrder := db.PaymentOrder{
		ID:                    10,
		OutTradeNo:            "ORDER_PAY_10",
		TransactionID:         pgtype.Text{String: "BF_PAY_10", Valid: true},
		Amount:                4200,
		Status:                "paid",
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		PaymentType:           "profit_sharing",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		OrderID:               toPgInt8(20),
	}
	order := db.Order{ID: 20, MerchantID: 30}
	refundOrder := db.RefundOrder{
		ID:             100,
		PaymentOrderID: 999,
		Status:         "pending",
		OutRefundNo:    "BFRFD_OTHER_PAYMENT",
		RefundAmount:   4200,
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).
		Return(refundOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
		RefundAmount:   refundOrder.RefundAmount,
		Reason:         "商户拒单：测试错配",
		OutRefundNo:    refundOrder.OutRefundNo,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment order mismatch")
	require.False(t, baofuClient.called)
}

func TestProcessTaskAnomalyRefund_ClaimRecoveryDirectRefundUsesDirectPaymentClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:           7,
		OutTradeNo:   "CLAIM_PAY_7",
		Amount:       6600,
		Status:       "closed",
		BusinessType: "claim_recovery",
		PaymentType:  "miniprogram",
		OrderID:      toPgInt8(17),
	}
	order := db.Order{ID: 17, MerchantID: 27}
	refundOrder := db.RefundOrder{ID: 77, PaymentOrderID: 7, Status: "pending", OutRefundNo: "CRF7"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(7)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(17)).
		Return(order, nil)
	store.EXPECT().
		CreateAnomalyRefundRecord(gomock.Any(), db.CreateAnomalyRefundRecordParams{
			PaymentOrderID: paymentOrder.ID,
			RefundAmount:   6600,
			OutRefundNo:    "CRF7",
		}).
		Return(refundOrder, nil)
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
			require.Equal(t, "CLAIM_PAY_7", req.OutTradeNo)
			require.Equal(t, "CRF7", req.OutRefundNo)
			require.Equal(t, int64(6600), req.RefundAmount)
			require.Equal(t, int64(6600), req.TotalAmount)
			return &wechat.RefundResponse{RefundID: "refund_claim_7", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().
		GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(6600), nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).
		Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil, paymentClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessAnomalyRefund{
		PaymentOrderID: paymentOrder.ID,
		TransactionID:  "wx_tx_claim_7",
		RefundAmount:   6600,
		OutRefundNo:    "CRF7",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessAnomalyRefund, payloadBytes)
	err = processor.ProcessTaskAnomalyRefund(context.Background(), task)
	require.NoError(t, err)
}

func toPgInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: true}
}
