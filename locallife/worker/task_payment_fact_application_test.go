package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskPaymentFactApplication_SkipsUnclaimableApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(901)).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: 901})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentFactApplication_RejectsMissingApplicationID(t *testing.T) {
	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "application id is required")
}

func TestProcessTaskPaymentFactApplication_OrdinaryProfitSharingReturnContinuesOrdinaryRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	application := db.ExternalPaymentFactApplication{
		ID:                 9301,
		FactID:             9201,
		Consumer:           "profit_sharing_domain",
		BusinessObjectType: "profit_sharing_return",
		BusinessObjectID:   9101,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}
	fact := db.ExternalPaymentFact{
		ID:                   application.FactID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    "PR-WORKER-ORDINARY",
		ExternalSecondaryKey: pgtype.Text{String: "WX-RETURN-WORKER", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "profit_sharing_return", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
	returnRecord := db.ProfitSharingReturn{
		ID:             application.BusinessObjectID,
		RefundOrderID:  9001,
		PaymentOrderID: 8901,
		OutReturnNo:    "PR-WORKER-ORDINARY",
		Status:         "processing",
	}
	updatedReturn := returnRecord
	updatedReturn.Status = "success"
	refundOrder := db.RefundOrder{
		ID:             returnRecord.RefundOrderID,
		PaymentOrderID: returnRecord.PaymentOrderID,
		Status:         "pending",
		OutRefundNo:    "RF-WORKER-ORDINARY",
		RefundAmount:   330,
		RefundReason:   pgtype.Text{String: "用户取消订单", Valid: true},
	}
	paymentOrder := db.PaymentOrder{
		ID:             refundOrder.PaymentOrderID,
		OrderID:        pgtype.Int8{Int64: 8801, Valid: true},
		OutTradeNo:     "PO-WORKER-ORDINARY",
		Amount:         1100,
		Status:         "paid",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
	}
	order := db.Order{ID: paymentOrder.OrderID.Int64, MerchantID: 8701}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingReturn(gomock.Any(), application.BusinessObjectID).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "WX-RETURN-WORKER", Valid: true},
	}).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToSuccess(gomock.Any(), returnRecord.ID).Return(updatedReturn, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrder(gomock.Any(), refundOrder.ID).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "success"}).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "failed"}).Return(int32(0), nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-worker"}, nil)
	ordinaryClient.EXPECT().RefundNotifyURL().Return("https://pay.example.com/ordinary/refund-notify")
	ordinaryClient.EXPECT().CreateRefund(gomock.Any(), gomock.AssignableToTypeOf(ospcontracts.RefundCreateRequest{})).DoAndReturn(func(_ context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error) {
		require.Equal(t, "sub-mchid-worker", req.SubMchID)
		require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
		require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
		require.Equal(t, refundOrder.RefundReason.String, req.Reason)
		require.Equal(t, "https://pay.example.com/ordinary/refund-notify", req.NotifyURL)
		require.Equal(t, refundOrder.RefundAmount, req.Amount.Refund)
		require.Equal(t, paymentOrder.Amount, req.Amount.Total)
		return &ospcontracts.RefundResponse{RefundID: "refund-worker-ordinary"}, nil
	})
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "refund-worker-ordinary", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).DoAndReturn(func(_ context.Context, arg db.UpdateExternalPaymentFactProcessingStatusParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, fact.ID, arg.ID)
		require.Equal(t, db.ExternalPaymentFactProcessingStatusTerminalized, arg.ProcessingStatus)
		require.True(t, arg.ProcessedAt.Valid)
		return db.ExternalPaymentFact{ID: fact.ID, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil
	})
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).DoAndReturn(func(_ context.Context, arg db.MarkExternalPaymentFactApplicationAppliedParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, application.ID, arg.ID)
		require.True(t, arg.AppliedAt.Valid)
		applied := application
		applied.Status = db.ExternalPaymentFactApplicationStatusApplied
		return applied, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetOrdinaryServiceProviderClient(ordinaryClient)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: application.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.NoError(t, err)
}
