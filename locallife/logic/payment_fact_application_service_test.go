package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingSuccessFinishesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(701, 601, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(601, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.False(t, result.Skipped)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryProfitSharingSuccessFinishesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 2, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(1701, 1601, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(1601, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Channel = db.PaymentChannelOrdinaryServiceProvider

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuProfitSharingSuccessFinishesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 3, 12, 10, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(2701, 2601, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(2601, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Provider = db.ExternalPaymentProviderBaofu
	fact.Channel = db.PaymentChannelBaofuAggregate
	fact.Capability = db.ExternalPaymentCapabilityBaofuProfitSharing
	fact.ExternalObjectType = db.ExternalPaymentObjectProfitSharing
	fact.ExternalObjectKey = "BFSHARE202605030001"
	fact.ExternalSecondaryKey = pgtype.Text{String: "BFSHAREUP202605030001", Valid: true}
	fact.UpstreamState = "SUCCESS"
	fact.RawResource = []byte(`{"txnState":"SUCCESS","succAmt":10000}`)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingSuccessRejectsPendingOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 5, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(702, 602, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(602, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(db.ProfitSharingOrder{ID: application.BusinessObjectID, Status: "pending"}, nil)
	expectApplicationFailed(t, store, application, now, "cannot apply success fact")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot apply success fact")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingFailedAlreadyFailedIsApplied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 10, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(703, 603, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(603, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.RawResource = []byte(`{"fail_reason":"NO_RELATION"}`)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFailed), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "FAILED", "NO_RELATION")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingReturnSuccessContinuesRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	now := time.Date(2026, 4, 26, 11, 10, 30, 0, time.UTC)
	application := buildProfitSharingReturnFactApplication(704, 604, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingReturnFact(604, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, "SUCCESS", "WX_RETURN_3001")
	returnRecord := db.ProfitSharingReturn{ID: application.BusinessObjectID, RefundOrderID: 4101, PaymentOrderID: 5101, OutReturnNo: "PR3001", Status: "processing"}
	updatedReturn := returnRecord
	updatedReturn.Status = "success"
	refundOrder := db.RefundOrder{ID: returnRecord.RefundOrderID, PaymentOrderID: returnRecord.PaymentOrderID, Status: "pending", OutRefundNo: "RF3001", RefundAmount: 300, RefundReason: pgtype.Text{String: "用户取消", Valid: true}}
	paymentOrder := db.PaymentOrder{ID: refundOrder.PaymentOrderID, OrderID: pgtype.Int8{Int64: 6101, Valid: true}, OutTradeNo: "TRADE3001", Amount: 1000, Status: "paid", PaymentChannel: db.PaymentChannelEcommerce}
	order := db.Order{ID: 6101, MerchantID: 7101}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingReturn(gomock.Any(), application.BusinessObjectID).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "WX_RETURN_3001", Valid: true},
	}).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToSuccess(gomock.Any(), returnRecord.ID).Return(updatedReturn, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrder(gomock.Any(), refundOrder.ID).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "success"}).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "failed"}).Return(int32(0), nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-3001"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
		require.Equal(t, "sub-mchid-3001", req.SubMchID)
		require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
		require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
		require.Equal(t, refundOrder.RefundReason.String, req.Reason)
		require.Equal(t, refundOrder.RefundAmount, req.RefundAmount)
		require.Equal(t, paymentOrder.Amount, req.TotalAmount)
		return &wechat.EcommerceRefundResponse{RefundID: "WX_REFUND_3001"}, nil
	})
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "WX_REFUND_3001", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithEcommerceClient(ecommerceClient)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryProfitSharingReturnSuccessContinuesOrdinaryRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	now := time.Date(2026, 4, 26, 11, 10, 35, 0, time.UTC)
	application := buildProfitSharingReturnFactApplication(714, 614, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingReturnFact(614, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, "SUCCESS", "WX_RETURN_3011")
	returnRecord := db.ProfitSharingReturn{ID: application.BusinessObjectID, RefundOrderID: 4111, PaymentOrderID: 5111, OutReturnNo: "PR3011", Status: "processing"}
	updatedReturn := returnRecord
	updatedReturn.Status = "success"
	refundOrder := db.RefundOrder{ID: returnRecord.RefundOrderID, PaymentOrderID: returnRecord.PaymentOrderID, Status: "pending", OutRefundNo: "RF3011", RefundAmount: 330, RefundReason: pgtype.Text{String: "用户取消", Valid: true}}
	paymentOrder := db.PaymentOrder{ID: refundOrder.PaymentOrderID, OrderID: pgtype.Int8{Int64: 6111, Valid: true}, OutTradeNo: "TRADE3011", Amount: 1100, Status: "paid", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	order := db.Order{ID: 6111, MerchantID: 7111}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingReturn(gomock.Any(), application.BusinessObjectID).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "WX_RETURN_3011", Valid: true},
	}).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToSuccess(gomock.Any(), returnRecord.ID).Return(updatedReturn, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrder(gomock.Any(), refundOrder.ID).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "success"}).Return(int32(1), nil)
	store.EXPECT().CountProfitSharingReturnsByRefundOrderStatus(gomock.Any(), db.CountProfitSharingReturnsByRefundOrderStatusParams{RefundOrderID: refundOrder.ID, Status: "failed"}).Return(int32(0), nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Return(db.MerchantPaymentConfig{MerchantID: order.MerchantID, SubMchID: "sub-mchid-3011"}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "refund-ordinary", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithRefundCreator(NewDefaultPaymentFacadeWithOrdinaryServiceProvider(store, nil, nil, ordinaryClient))
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, ordinaryClient.createRefundRequest)
	require.Equal(t, "sub-mchid-3011", ordinaryClient.createRefundRequest.SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createRefundRequest.OutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, ordinaryClient.createRefundRequest.OutRefundNo)
	require.Equal(t, refundOrder.RefundReason.String, ordinaryClient.createRefundRequest.Reason)
	require.Equal(t, ordinaryClient.RefundNotifyURL(), ordinaryClient.createRefundRequest.NotifyURL)
	require.Equal(t, refundOrder.RefundAmount, ordinaryClient.createRefundRequest.Amount.Refund)
	require.Equal(t, paymentOrder.Amount, ordinaryClient.createRefundRequest.Amount.Total)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingReturnFailedFailsRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 10, 45, 0, time.UTC)
	application := buildProfitSharingReturnFactApplication(705, 605, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingReturnFact(605, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed, "FAILED", "")
	fact.RawResource = []byte(`{"fail_reason":"PAYER_ACCOUNT_ABNORMAL"}`)
	returnRecord := db.ProfitSharingReturn{ID: application.BusinessObjectID, RefundOrderID: 4102, PaymentOrderID: 5102, OutReturnNo: "PR3002", Status: "processing"}
	failedReturn := returnRecord
	failedReturn.Status = "failed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingReturn(gomock.Any(), application.BusinessObjectID).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToFailed(gomock.Any(), db.UpdateProfitSharingReturnToFailedParams{
		ID:         returnRecord.ID,
		FailReason: pgtype.Text{String: "PAYER_ACCOUNT_ABNORMAL", Valid: true},
	}).Return(failedReturn, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), returnRecord.RefundOrderID).Return(db.RefundOrder{ID: returnRecord.RefundOrderID, Status: "failed"}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositPaymentSuccessDoesNotWriteReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 0, 0, time.UTC)
	application := buildRiderDepositPaymentFactApplication(801, 701, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositPaymentFact(701, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, UserID: 77, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithEcommerceClient(ecommerceClient)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositPaymentRetryDoesNotWriteReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 45, 0, time.UTC)
	application := buildRiderDepositPaymentFactApplication(8021, 7021, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositPaymentFact(7021, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{
		ID:           application.BusinessObjectID,
		UserID:       79,
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
		ProcessedAt:  pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
	}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    false,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithEcommerceClient(ecommerceClient)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ClaimRecoveryPaymentSuccessMarksRecoveryPaidWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 803,
		FactID:             703,
		Consumer:           paymentFactConsumerClaimRecoveryDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   903,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 703,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelDirect,
		Capability:         db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		ExternalObjectType: db.ExternalPaymentObjectPayment,
		ExternalObjectKey:  "CR_PAY_903",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerClaimRecovery, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
	}
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, UserID: 88, BusinessType: db.ExternalPaymentBusinessOwnerClaimRecovery}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.ClaimRecoveryPayment)
	require.Equal(t, paymentOrder.ID, result.ClaimRecoveryPayment.PaymentOrder.ID)
	require.True(t, result.ClaimRecoveryPayment.Processed)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeSuccessContinuesOpening(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1803, 1703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "paid",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1001;purpose:initial_open", Valid: true},
	}
	processedPaymentOrder := paymentOrder
	processedPaymentOrder.ProcessedAt = pgtype.Timestamptz{Time: now, Valid: true}
	continuation := &testBaofuVerifyFeeContinuation{}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderProcessedAt(gomock.Any(), application.BusinessObjectID).Return(processedPaymentOrder, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, processedPaymentOrder.ID, result.BaofuVerifyFeePayment.PaymentOrder.ID)
	require.True(t, continuation.called)
	require.Equal(t, processedPaymentOrder.ID, continuation.paymentOrder.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeRejectsAmountMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 32, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1807, 1707, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1707, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Amount = pgtype.Int8{Int64: 999, Valid: true}
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "paid",
		Amount:         200,
	}
	continuation := &testBaofuVerifyFeeContinuation{}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	expectApplicationFailed(t, store, application, now, "amount")

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "amount")
	require.False(t, result.Applied)
	require.Nil(t, result.BaofuVerifyFeePayment)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeClosedReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 35, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1804, 1704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1001;purpose:initial_open", Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7101,
		OwnerType:               db.BaofuAccountOwnerTypeRider,
		OwnerID:                 1001,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7001, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}
	pendingFlow := flow
	pendingFlow.State = db.BaofuAccountOpeningStateVerifyFeePending

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.Equal(t, flow.ProfileID, arg.ProfileID)
			require.Equal(t, flow.VerifyFeeAmount, arg.VerifyFeeAmount)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"state":"verify_fee_pending"`)
			require.Contains(t, string(arg.RawSnapshot), `"payment_status":"closed"`)
			return pendingFlow, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "closed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, result.BaofuVerifyFeePayment.Processed)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeFailedReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 36, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1805, 1705, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1705, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.UpstreamState = "PAYERROR"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:operator;owner_id:1002;purpose:initial_open", Valid: true},
	}
	failedPaymentOrder := paymentOrder
	failedPaymentOrder.Status = "failed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7102,
		OwnerType:               db.BaofuAccountOwnerTypeOperator,
		OwnerID:                 1002,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7002, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), paymentOrder.ID).Return(failedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"payment_status":"failed"`)
			return db.BaofuAccountOpeningFlow{ID: flow.ID, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, State: db.BaofuAccountOpeningStateVerifyFeePending}, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "failed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeExpiredReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 37, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1806, 1706, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1706, application.BusinessObjectID, db.ExternalPaymentTerminalStatusExpired)
	fact.UpstreamState = "EXPIRED"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1003;purpose:initial_open", Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7103,
		OwnerType:               db.BaofuAccountOwnerTypeRider,
		OwnerID:                 1003,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7003, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"payment_terminal_status":"expired"`)
			require.Contains(t, string(arg.RawSnapshot), "支付未完成，请重新支付开户核验费。")
			return db.BaofuAccountOpeningFlow{ID: flow.ID, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, State: db.BaofuAccountOpeningStateVerifyFeePending}, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "closed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, result.BaofuVerifyFeePayment.Processed)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinishWithoutAuthorizationActivatesMerchant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 0, 0, time.UTC)
	application := buildApplymentFactApplication(850, 750, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildApplymentFact(750, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, "sub_mch_850")
	fact.Channel = db.PaymentChannelOrdinaryServiceProvider
	applymentBefore := db.EcommerceApplyment{ID: application.BusinessObjectID, SubjectType: "merchant", SubjectID: 910, OutRequestNo: "APPLY_M_850", Status: "submitted"}
	applymentAfter := applymentBefore
	applymentAfter.Status = "finish"
	applymentAfter.SubMchID = pgtype.Text{String: "sub_mch_850", Valid: true}
	applymentAfter.AccountAuthorizeState = pgtype.Text{String: "AUTHORIZE_STATE_UNAUTHORIZED", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentBefore, nil)
	store.EXPECT().ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
		ApplymentID:       application.BusinessObjectID,
		WechatApplymentID: pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		SubjectType:       "merchant",
		SubjectID:         applymentBefore.SubjectID,
		SubMchID:          "sub_mch_850",
	}).Return(nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentAfter, nil)
	expectApplymentActivatedOutbox(t, store, application, fact, applymentAfter)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinishWithStoredAuthorizationActivatesMerchant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 30, 0, time.UTC)
	application := buildApplymentFactApplication(858, 758, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildApplymentFact(758, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, "sub_mch_858")
	fact.Channel = db.PaymentChannelOrdinaryServiceProvider
	applymentBefore := db.EcommerceApplyment{
		ID:                    application.BusinessObjectID,
		SubjectType:           "merchant",
		SubjectID:             918,
		OutRequestNo:          "APPLY_M_858",
		Status:                "submitted",
		AccountAuthorizeState: pgtype.Text{String: "AUTHORIZE_STATE_AUTHORIZED", Valid: true},
	}
	applymentAfter := applymentBefore
	applymentAfter.Status = "finish"
	applymentAfter.SubMchID = pgtype.Text{String: "sub_mch_858", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentBefore, nil)
	store.EXPECT().ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
		ApplymentID:           application.BusinessObjectID,
		WechatApplymentID:     pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		SubjectType:           "merchant",
		SubjectID:             applymentBefore.SubjectID,
		SubMchID:              "sub_mch_858",
		AccountAuthorizeState: "AUTHORIZE_STATE_AUTHORIZED",
	}).Return(nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentAfter, nil)
	expectApplymentActivatedOutbox(t, store, application, fact, applymentAfter)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFactAuthorizationActivatesMerchant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 45, 0, time.UTC)
	application := buildApplymentFactApplication(859, 759, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildApplymentFact(759, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, "sub_mch_859")
	fact.Channel = db.PaymentChannelOrdinaryServiceProvider
	raw := map[string]any{
		"applyment_id":            application.BusinessObjectID,
		"sub_mch_id":              "sub_mch_859",
		"account_authorize_state": db.AccountAuthorizeStateAuthorized,
	}
	fact.RawResource, _ = json.Marshal(raw)
	applymentBefore := db.EcommerceApplyment{
		ID:                    application.BusinessObjectID,
		SubjectType:           "merchant",
		SubjectID:             919,
		OutRequestNo:          "APPLY_M_859",
		Status:                "finish",
		SubMchID:              pgtype.Text{String: "sub_mch_859", Valid: true},
		AccountAuthorizeState: pgtype.Text{String: db.AccountAuthorizeStateUnauthorized, Valid: true},
	}
	applymentAfter := applymentBefore
	applymentAfter.AccountAuthorizeState = pgtype.Text{String: db.AccountAuthorizeStateAuthorized, Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentBefore, nil)
	store.EXPECT().ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
		ApplymentID:           application.BusinessObjectID,
		WechatApplymentID:     pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		SubjectType:           "merchant",
		SubjectID:             applymentBefore.SubjectID,
		SubMchID:              "sub_mch_859",
		AccountAuthorizeState: db.AccountAuthorizeStateAuthorized,
	}).Return(nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentAfter, nil)
	expectApplymentActivatedOutbox(t, store, application, fact, applymentAfter)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ApplymentRejectedUpdatesStatusAndCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 13, 0, 0, time.UTC)
	application := buildApplymentFactApplication(851, 751, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildApplymentTerminalFact(751, application.BusinessObjectID, "REJECTED", db.ExternalPaymentTerminalStatusFailed, "资料驳回", "")
	applymentBefore := db.EcommerceApplyment{ID: application.BusinessObjectID, SubjectType: "merchant", SubjectID: 911, OutRequestNo: "APPLY_M_851", Status: "auditing"}
	applymentAfter := applymentBefore
	applymentAfter.Status = "rejected"
	applymentAfter.RejectReason = pgtype.Text{String: "资料驳回", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentBefore, nil)
	store.EXPECT().UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
		ID:                 application.BusinessObjectID,
		ApplymentID:        pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		Status:             "rejected",
		RejectReason:       pgtype.Text{String: "资料驳回", Valid: true},
		SignUrl:            applymentBefore.SignUrl,
		SignState:          applymentBefore.SignState,
		LegalValidationUrl: applymentBefore.LegalValidationUrl,
		AccountValidation:  applymentBefore.AccountValidation,
		SubMchID:           pgtype.Text{},
	}).Return(applymentAfter, nil)
	expectApplymentTerminalOutbox(t, store, application, fact, applymentAfter)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventApplymentTerminalStateReady, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ApplymentPendingUpdatesStatusAndCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 14, 0, 0, time.UTC)
	application := buildApplymentFactApplication(852, 752, db.ExternalPaymentFactApplicationStatusProcessing)
	accountValidation := wechat.MarshalEcommerceApplymentAccountValidation(&wechatcontracts.EcommerceApplymentAccountValidation{
		AccountName: "测试商户有限公司",
		Remark:      "请汇款 0.01 元完成验证",
	})
	fact := buildApplymentPendingFact(752, application.BusinessObjectID, "ACCOUNT_NEED_VERIFY", "https://wx.example.com/legal-check", accountValidation)
	applymentBefore := db.EcommerceApplyment{ID: application.BusinessObjectID, SubjectType: "merchant", SubjectID: 912, OutRequestNo: "APPLY_M_852", Status: "auditing"}
	applymentAfter := applymentBefore
	applymentAfter.Status = "account_need_verify"
	applymentAfter.SignUrl = pgtype.Text{String: fmt.Sprintf("https://pay.weixin.qq.com/sign/%d", application.BusinessObjectID), Valid: true}
	applymentAfter.SignState = pgtype.Text{String: "UNSIGNED", Valid: true}
	applymentAfter.LegalValidationUrl = pgtype.Text{String: "https://wx.example.com/legal-check", Valid: true}
	applymentAfter.AccountValidation = accountValidation

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applymentBefore, nil)
	store.EXPECT().UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
		ID:                 application.BusinessObjectID,
		ApplymentID:        pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		Status:             "account_need_verify",
		RejectReason:       pgtype.Text{},
		SignUrl:            applymentAfter.SignUrl,
		SignState:          applymentAfter.SignState,
		LegalValidationUrl: applymentAfter.LegalValidationUrl,
		AccountValidation:  accountValidation,
		SubMchID:           pgtype.Text{},
	}).Return(applymentAfter, nil)
	expectApplymentPendingOutbox(t, store, application, fact, applymentAfter)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventApplymentPendingStateReady, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderPaymentSuccessProcessesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 40, 0, time.UTC)
	application := buildOrderPaymentFactApplication(803, 703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildOrderPaymentFact(703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelEcommerce}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6201, MerchantID: 7101, OrderNo: "ORD6201"}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8201, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, result.Outbox.EventType)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.OrderPayment.OrderResult)
	require.Equal(t, orderResult.Order.ID, result.OrderPayment.OrderResult.Order.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderOrdinaryPaymentSuccessProcessesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 1, 10, 30, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(813, 713, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildOrderPaymentFact(713, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Channel = db.PaymentChannelOrdinaryServiceProvider
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6301, MerchantID: 7201, OrderNo: "ORD6301"}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8301, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentSuccessMarksPaidAndProcessesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(823, 723, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuOrderPaymentFact(723, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6401, MerchantID: 7301, OrderNo: "ORD6401"}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6401", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8401, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderPaymentOutboxRetryAfterProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 50, 0, time.UTC)
	application := buildOrderPaymentFactApplication(805, 705, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildOrderPaymentFact(705, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	processedAt := pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelEcommerce, OrderID: pgtype.Int8{Int64: 6202, Valid: true}, ProcessedAt: processedAt}
	order := db.Order{ID: paymentOrder.OrderID.Int64, MerchantID: 7102, OrderNo: "ORD6202"}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{Processed: false, PaymentOrder: paymentOrder}, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8203, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.OrderPayment.OrderResult)
	require.Equal(t, order.ID, result.OrderPayment.OrderResult.Order.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationPaymentSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(804, 704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8202, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, result.Outbox.EventType)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, int64(6201), result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationPaymentOutboxRetryAfterProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 0, 30, 0, time.UTC)
	application := buildReservationPaymentFactApplication(806, 706, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(706, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6203, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce, ProcessedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{Processed: false, PaymentOrder: paymentOrder}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8204, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, paymentOrder.ReservationID.Int64, result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundSuccessUpdatesPrepaidAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 45, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 811,
		FactID:             711,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4101,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 711,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityEcommerceRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		ExternalObjectKey:  "RFD4101",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: 4101, Valid: true},
		UpstreamState:      riderDepositRefundStatusSuccess,
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
	}
	refundOrder := db.RefundOrder{ID: 4101, PaymentOrderID: 5101, RefundAmount: 400, OutRefundNo: "RFD4101", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5101, ReservationID: pgtype.Int8{Int64: 6101, Valid: true}, Amount: 400, BusinessType: reservationAddonBusiness}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(400), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().AddReservationPrepaidAmount(gomock.Any(), db.AddReservationPrepaidAmountParams{ID: 6101, PrepaidAmount: -400}).Return(db.TableReservation{ID: 6101}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundClosedUpdatesRefundOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 15, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 812,
		FactID:             712,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4102,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 712,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityEcommerceRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: 4102, Valid: true},
		UpstreamState:      riderDepositRefundStatusClosed,
		TerminalStatus:     db.ExternalPaymentTerminalStatusClosed,
		IsTerminal:         true,
	}
	refundOrder := db.RefundOrder{ID: 4102, PaymentOrderID: 5102, RefundAmount: 280, OutRefundNo: "RFD4102", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5102, ReservationID: pgtype.Int8{Int64: 6102, Valid: true}, Amount: 400, BusinessType: reservationAddonBusiness}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToClosed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusClosed}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 30, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 813,
		FactID:             713,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4103,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   713,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityEcommerceRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "WX_REFUND_4103", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4103, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4103, PaymentOrderID: 5103, RefundAmount: 280, OutRefundNo: "RFD4103", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5103, ReservationID: pgtype.Int8{Int64: 6103, Valid: true}, Amount: 400, BusinessType: db.ExternalPaymentBusinessOwnerReservation}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusFailed}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8102, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 45, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 814,
		FactID:             714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   714,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityEcommerceRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "WX_REFUND_4201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4201, Valid: true},
		UpstreamState:        riderDepositRefundStatusSuccess,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4201, PaymentOrderID: 5201, RefundAmount: 500, OutRefundNo: "RFD4201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5201, OrderID: pgtype.Int8{Int64: 6201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderOrdinaryRefundSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 50, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 1814,
		FactID:             1714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   14201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   1714,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityPartnerRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "WX_ORDINARY_REFUND_4201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 14201, Valid: true},
		UpstreamState:        riderDepositRefundStatusSuccess,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 14201, PaymentOrderID: 15201, RefundAmount: 500, OutRefundNo: "RFD14201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 15201, OrderID: pgtype.Int8{Int64: 16201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 177}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 18103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderBaofuRefundSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2814,
		FactID:             2714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2714,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24201, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24201, PaymentOrderID: 25201, RefundAmount: 500, OutRefundNo: "BFRFD24201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 25201, OrderID: pgtype.Int8{Int64: 26201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 28103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 13, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 815,
		FactID:             715,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4202,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   715,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityEcommerceRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "WX_REFUND_4202", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4202, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4202, PaymentOrderID: 5202, RefundAmount: 500, OutRefundNo: "RFD4202", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5202, OrderID: pgtype.Int8{Int64: 6202, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 78}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusFailed}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8104, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositClosedResolvesRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 0, 0, time.UTC)
	application := buildRiderDepositRefundFactApplication(803, 703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositRefundFact(703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed, riderDepositRefundStatusClosed)
	refundOrder := db.RefundOrder{ID: application.BusinessObjectID, PaymentOrderID: 5101, OutRefundNo: "RFD3001"}
	paymentOrder := db.PaymentOrder{ID: 5101, UserID: 811, Amount: 30000, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}
	fact.ExternalObjectKey = refundOrder.OutRefundNo
	fact.ExternalSecondaryKey = pgtype.Text{String: "WX_REFUND_RFD3001", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
		RefundOrderID: application.BusinessObjectID,
		RefundStatus:  riderDepositRefundStatusClosed,
		RefundID:      "WX_REFUND_RFD3001",
	}).Return(db.ResolveRiderDepositRefundTxResult{}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	now := time.Date(2026, 4, 26, 11, 13, 0, 0, time.UTC)
	application := buildRiderDepositRefundFactApplication(804, 704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositRefundFact(704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed, riderDepositRefundStatusAbnormal)
	refundOrder := db.RefundOrder{ID: application.BusinessObjectID, PaymentOrderID: 5102, OutRefundNo: "RFD3002"}
	paymentOrder := db.PaymentOrder{ID: 5102, UserID: 812, Amount: 30000, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}
	fact.ExternalObjectKey = refundOrder.OutRefundNo
	fact.ExternalSecondaryKey = pgtype.Text{String: "WX_REFUND_RFD3002", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
		RefundOrderID: application.BusinessObjectID,
		RefundStatus:  riderDepositRefundStatusAbnormal,
		RefundID:      "WX_REFUND_RFD3002",
	}).Return(db.ResolveRiderDepositRefundTxResult{}, nil)
	expectRiderDepositRefundAbnormalOutbox(t, store, application, fact, refundOrder, paymentOrder)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithEcommerceClient(ecommerceClient)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_MarkAppliedFailureSchedulesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 15, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(705, 605, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(605, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)
	expectApplicationFailed(t, store, application, now, "mark external payment fact application applied")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mark external payment fact application applied")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OutboxFailureSchedulesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 17, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(707, 607, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(607, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).Return(db.PaymentDomainOutbox{}, db.ErrRecordNotFound)
	expectApplicationFailed(t, store, application, now, "create profit sharing result outbox")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create profit sharing result outbox")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RejectsNonTerminalFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 20, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(706, 606, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(606, application.BusinessObjectID, db.ExternalPaymentTerminalStatusProcessing)
	fact.IsTerminal = false

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	expectApplicationFailed(t, store, application, now, "is not terminal")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not terminal")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_SkipsUnclaimableApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(704)).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)

	svc := NewPaymentFactService(store)
	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), 704)
	require.NoError(t, err)
	require.True(t, result.Skipped)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_SettlementVerificationThirdCheckPromotesToSuccessWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	application := buildSettlementVerificationFactApplication(920, 820, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildSettlementVerificationFact(820, application.BusinessObjectID, db.ExternalPaymentTerminalStatusProcessing, "VERIFYING", "", now.Add(-24*time.Hour), now, 3)
	applyment := db.EcommerceApplyment{
		ID:                            application.BusinessObjectID,
		SubjectType:                   "merchant",
		SubjectID:                     91,
		SubMchID:                      pgtype.Text{String: "1900000091", Valid: true},
		SettlementVerifyStatus:        pgtype.Text{String: "verifying", Valid: true},
		SettlementVerifyCheckCount:    2,
		SettlementVerifyLastCheckedAt: pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
	}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), application.BusinessObjectID).Return(applyment, nil)
	store.EXPECT().UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
		require.Equal(t, application.BusinessObjectID, arg.ID)
		require.True(t, arg.SettlementVerifyFirstTradeAt.Valid)
		require.Equal(t, now.Add(-24*time.Hour), arg.SettlementVerifyFirstTradeAt.Time)
		require.True(t, arg.SettlementVerifyLastCheckedAt.Valid)
		require.Equal(t, now, arg.SettlementVerifyLastCheckedAt.Time)
		require.True(t, arg.SettlementVerifyCheckCount.Valid)
		require.Equal(t, int32(3), arg.SettlementVerifyCheckCount.Int32)
		require.True(t, arg.SettlementVerifyStatus.Valid)
		require.Equal(t, "success", arg.SettlementVerifyStatus.String)
		require.True(t, arg.SettlementVerifyFailReason.Valid)
		require.Equal(t, "", arg.SettlementVerifyFailReason.String)
		applyment.SettlementVerifyStatus = pgtype.Text{String: "success", Valid: true}
		applyment.SettlementVerifyFailReason = pgtype.Text{String: "", Valid: true}
		applyment.SettlementVerifyCheckCount = 3
		applyment.SettlementVerifyLastCheckedAt = pgtype.Timestamptz{Time: now, Valid: true}
		return applyment, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.SettlementVerification)
	require.Equal(t, "success", result.SettlementVerification.Status)
	require.Equal(t, application.BusinessObjectID, result.SettlementVerification.Applyment.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_SettlementApplicationTrackingWritesLatestApplicationNoWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 20, 0, 0, time.UTC)
	application := buildSettlementApplicationFactApplication(921, 821, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildSettlementApplicationFact(821, application.BusinessObjectID, db.ExternalPaymentTerminalStatusProcessing, "AUDITING", "APP_AUDITING")
	paymentConfig := db.MerchantPaymentConfig{
		ID:                            application.BusinessObjectID,
		MerchantID:                    92,
		SubMchID:                      "1900000092",
		LatestSettlementApplicationNo: pgtype.Text{String: "APP_OLD", Valid: true},
	}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetMerchantPaymentConfigBySubMchID(gomock.Any(), paymentConfig.SubMchID).Return(paymentConfig, nil)
	store.EXPECT().UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), db.UpdateMerchantPaymentConfigSettlementApplicationParams{
		MerchantID:                             paymentConfig.MerchantID,
		LatestSettlementApplicationNo:          pgtype.Text{String: "APP_AUDITING", Valid: true},
		LatestSettlementApplicationSubmittedAt: pgtype.Timestamptz{},
	}).Return(paymentConfig, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.SettlementApplicationTracking)
	require.Equal(t, "APP_AUDITING", result.SettlementApplicationTracking.ApplicationNo)
	require.Equal(t, "AUDITING", result.SettlementApplicationTracking.VerifyResult)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_MerchantWithdrawSuccessUpdatesWithdrawalWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 25, 0, 0, time.UTC)
	application := buildMerchantWithdrawFactApplication(922, 822, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildMerchantWithdrawFact(822, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess, wechatcontracts.FundManagementWithdrawStatusSuccess, "wd_merchant_001", "")
	withdrawalRecord := db.WithdrawalRecord{
		ID:      application.BusinessObjectID,
		UserID:  77,
		Amount:  1200,
		Status:  "pending",
		Channel: "wechat_ecommerce_fund",
		Reason:  pgtype.Text{String: "old timeout", Valid: true},
	}
	updatedRecord := withdrawalRecord
	updatedRecord.Status = "success"
	updatedRecord.Reason = pgtype.Text{}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetWithdrawalRecord(gomock.Any(), application.BusinessObjectID).Return(withdrawalRecord, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), db.UpdateWithdrawalStatusParams{
		ID:          withdrawalRecord.ID,
		Status:      "success",
		Reason:      pgtype.Text{},
		ClearReason: true,
	}).Return(updatedRecord, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.MerchantWithdraw)
	require.Equal(t, "success", result.MerchantWithdraw.WithdrawalRecord.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_MerchantCancelWithdrawUpdatesApplicationWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 40, 0, 0, time.UTC)
	application := buildMerchantCancelWithdrawFactApplication(923, 823, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildMerchantCancelWithdrawFact(
		823,
		application.BusinessObjectID,
		db.ExternalPaymentTerminalStatusSuccess,
		db.MerchantCancelStateFinish,
		"WX-CANCEL-3402",
	)
	current := db.MerchantCancelWithdrawApplication{
		ID:             application.BusinessObjectID,
		MerchantID:     88,
		SubMchID:       "1900000109",
		OutRequestNo:   "MCW3402",
		LocalSyncState: db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
		CancelState:    pgtype.Text{String: db.MerchantCancelStateReviewing, Valid: true},
	}
	updated := current
	updated.ApplymentID = pgtype.Text{String: "WX-CANCEL-3402", Valid: true}
	updated.LocalSyncState = db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded
	updated.CancelState = pgtype.Text{String: db.MerchantCancelStateFinish, Valid: true}
	updated.CancelStateDescription = pgtype.Text{String: "完成", Valid: true}
	updated.WithdrawState = pgtype.Text{String: db.MerchantCancelWithdrawStateSucceed, Valid: true}
	updated.WithdrawStateDescription = pgtype.Text{String: "提现成功", Valid: true}
	updated.LastQueryAt = pgtype.Timestamptz{Time: now, Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), application.BusinessObjectID).Return(current, nil)
	store.EXPECT().UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantCancelWithdrawApplicationSyncParams{})).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
		require.Equal(t, current.ID, arg.ID)
		require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
		require.Equal(t, "WX-CANCEL-3402", arg.ApplymentID.String)
		require.Equal(t, db.MerchantCancelStateFinish, arg.CancelState.String)
		require.Equal(t, db.MerchantCancelWithdrawStateSucceed, arg.WithdrawState.String)
		require.True(t, arg.ClearLastError)
		require.True(t, arg.LastQueryAt.Valid)
		return updated, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.MerchantCancelWithdraw)
	require.Equal(t, db.MerchantCancelStateFinish, result.MerchantCancelWithdraw.Application.CancelState.String)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_MerchantCancelWithdrawBuildFailureSchedulesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 45, 0, 0, time.UTC)
	application := buildMerchantCancelWithdrawFactApplication(924, 824, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildMerchantCancelWithdrawFact(
		824,
		application.BusinessObjectID,
		db.ExternalPaymentTerminalStatusSuccess,
		db.MerchantCancelStateFinish,
		"WX-CANCEL-3403",
	)
	fact.RawResource = []byte(`{"applyment_id":"WX-CANCEL-3403","out_request_no":"MCW3403","cancel_state":"FINISH","modify_time":"not-rfc3339"}`)
	current := db.MerchantCancelWithdrawApplication{ID: application.BusinessObjectID, MerchantID: 89, SubMchID: "1900000110", OutRequestNo: "MCW3403"}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), application.BusinessObjectID).Return(current, nil)
	expectApplicationFailed(t, store, application, now, "build merchant cancel withdraw sync params")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build merchant cancel withdraw sync params")
	require.False(t, result.Applied)
}

func buildProfitSharingFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   3001,
		Status:             status,
	}
}

func buildProfitSharingReturnFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingReturn,
		BusinessObjectID:   3002,
		Status:             status,
	}
}

func buildRiderDepositRefundFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4001,
		Status:             status,
	}
}

func buildRiderDepositPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   5001,
		Status:             status,
	}
}

func buildBaofuVerifyFeePaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerBaofuAccountVerifyFeeDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   5101,
		Status:             status,
	}
}

func buildOrderPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   6001,
		Status:             status,
	}
}

func buildReservationPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   6201,
		Status:             status,
	}
}

func buildApplymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerApplymentDomain,
		BusinessObjectType: paymentFactBusinessObjectApplyment,
		BusinessObjectID:   8501,
		Status:             status,
	}
}

func buildSettlementVerificationFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerSettlementDomain,
		BusinessObjectType: paymentFactBusinessObjectApplyment,
		BusinessObjectID:   3201,
		Status:             status,
	}
}

func buildSettlementApplicationFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerSettlementDomain,
		BusinessObjectType: paymentFactBusinessObjectMerchantPaymentConfig,
		BusinessObjectID:   3301,
		Status:             status,
	}
}

func buildMerchantWithdrawFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerMerchantFundsDomain,
		BusinessObjectType: paymentFactBusinessObjectWithdrawalRecord,
		BusinessObjectID:   3401,
		Status:             status,
	}
}

func buildMerchantCancelWithdrawFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerMerchantFundsDomain,
		BusinessObjectType: paymentFactBusinessObjectMerchantCancelWithdraw,
		BusinessObjectID:   3402,
		Status:             status,
	}
}

func buildProfitSharingFact(factID, profitSharingOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityProfitSharing,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  "PS3001",
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectProfitSharingOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: profitSharingOrderID, Valid: true},
		TerminalStatus:     terminalStatus,
		IsTerminal:         true,
		RawResource:        []byte(`{}`),
	}
}

func buildProfitSharingReturnFact(factID, profitSharingReturnID int64, terminalStatus string, upstreamState string, returnID string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    "PR3001",
		ExternalSecondaryKey: pgtype.Text{String: returnID, Valid: returnID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectProfitSharingReturn, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: profitSharingReturnID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildRiderDepositRefundFact(factID, refundOrderID int64, terminalStatus, upstreamState string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    "RFD3001",
		ExternalSecondaryKey: pgtype.Text{String: "WX_REFUND_RFD3001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: refundOrderID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildRiderDepositPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "RD5001",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_5001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildBaofuVerifyFeePaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "BFVF5101",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_5101", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerBaofuVerifyFee, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildOrderPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "PO6001",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_6001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildBaofuOrderPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    "PO6401",
		ExternalSecondaryKey: pgtype.Text{String: "BFPAY_6401", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{"tradeNo":"BFPAY_6401","txnState":"SUCCESS"}`),
	}
}

func buildReservationPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "RES6201",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_6201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildApplymentFact(factID, applymentID int64, terminalStatus, subMchID string) db.ExternalPaymentFact {
	resource := map[string]any{
		"applyment_id": applymentID,
		"sub_mch_id":   subMchID,
	}
	raw, _ := json.Marshal(resource)
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityApplyment,
		ExternalObjectType: db.ExternalPaymentObjectApplyment,
		ExternalObjectKey:  "APPLY_M_850",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectApplyment, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: applymentID, Valid: true},
		UpstreamState:      "FINISH",
		TerminalStatus:     terminalStatus,
		IsTerminal:         true,
		RawResource:        raw,
	}
}

func buildApplymentTerminalFact(factID, applymentID int64, upstreamState, terminalStatus, rejectReason, subMchID string) db.ExternalPaymentFact {
	resource := map[string]any{
		"applyment_id":         applymentID,
		"applyment_state":      upstreamState,
		"applyment_state_desc": rejectReason,
		"reject_reason":        rejectReason,
	}
	if subMchID != "" {
		resource["sub_mch_id"] = subMchID
	}
	raw, _ := json.Marshal(resource)
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityApplyment,
		ExternalObjectType: db.ExternalPaymentObjectApplyment,
		ExternalObjectKey:  "APPLY_M_851",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectApplyment, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: applymentID, Valid: true},
		UpstreamState:      upstreamState,
		TerminalStatus:     terminalStatus,
		IsTerminal:         true,
		RawResource:        raw,
	}
}

func buildApplymentPendingFact(factID, applymentID int64, upstreamState, legalValidationURL string, accountValidation []byte) db.ExternalPaymentFact {
	resource := map[string]any{
		"applyment_id":         applymentID,
		"applyment_state":      upstreamState,
		"sign_url":             fmt.Sprintf("https://pay.weixin.qq.com/sign/%d", applymentID),
		"sign_state":           "UNSIGNED",
		"legal_validation_url": legalValidationURL,
	}
	if len(accountValidation) > 0 {
		resource["account_validation"] = json.RawMessage(accountValidation)
	}
	raw, _ := json.Marshal(resource)
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityApplyment,
		ExternalObjectType: db.ExternalPaymentObjectApplyment,
		ExternalObjectKey:  "APPLY_M_852",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectApplyment, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: applymentID, Valid: true},
		UpstreamState:      upstreamState,
		TerminalStatus:     db.ExternalPaymentTerminalStatusProcessing,
		IsTerminal:         false,
		RawResource:        raw,
	}
}

func buildSettlementVerificationFact(factID, applymentID int64, terminalStatus, upstreamState, failReason string, firstTradeAt, checkedAt time.Time, checkCount int32) db.ExternalPaymentFact {
	raw, _ := json.Marshal(map[string]any{
		"applyment_id":                      applymentID,
		"sub_mch_id":                        "1900000091",
		"verify_result":                     upstreamState,
		"verify_fail_reason":                failReason,
		"settlement_verify_first_trade_at":  firstTradeAt.Format(time.RFC3339Nano),
		"settlement_verify_last_checked_at": checkedAt.Format(time.RFC3339Nano),
		"settlement_verify_check_count":     checkCount,
	})
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelOrdinaryServiceProvider,
		Capability:         db.ExternalPaymentCapabilitySettlement,
		FactSource:         db.ExternalPaymentFactSourceQuery,
		ExternalObjectType: db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:  "1900000091",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: applymentID, Valid: true},
		UpstreamState:      upstreamState,
		TerminalStatus:     terminalStatus,
		IsTerminal:         terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:        raw,
	}
}

func buildSettlementApplicationFact(factID, paymentConfigID int64, terminalStatus, upstreamState, applicationNo string) db.ExternalPaymentFact {
	raw, _ := json.Marshal(map[string]any{
		"application_no": applicationNo,
		"sub_mch_id":     "1900000092",
		"verify_result":  upstreamState,
	})
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilitySettlement,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:    "1900000092",
		ExternalSecondaryKey: pgtype.Text{String: applicationNo, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectMerchantPaymentConfig, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentConfigID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          raw,
	}
}

func buildMerchantWithdrawFact(factID, withdrawalRecordID int64, terminalStatus, upstreamState, withdrawID, reason string) db.ExternalPaymentFact {
	raw, _ := json.Marshal(map[string]any{
		"withdrawal_record_id": withdrawalRecordID,
		"out_request_no":       "MW3401",
		"withdraw_id":          withdrawID,
		"wechat_status":        upstreamState,
		"reason":               reason,
		"amount":               1200,
	})
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    "MW3401",
		ExternalSecondaryKey: pgtype.Text{String: withdrawID, Valid: withdrawID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectWithdrawalRecord, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: withdrawalRecordID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          raw,
	}
}

func buildMerchantCancelWithdrawFact(factID, applicationID int64, terminalStatus, cancelState, applymentID string) db.ExternalPaymentFact {
	raw, _ := json.Marshal(map[string]any{
		"application_id":             applicationID,
		"merchant_id":                88,
		"sub_mch_id":                 "1900000109",
		"out_request_no":             "MCW3402",
		"applyment_id":               applymentID,
		"cancel_state":               cancelState,
		"cancel_state_description":   "完成",
		"withdraw":                   db.MerchantCancelWithdrawModeNoWithdraw,
		"withdraw_state":             db.MerchantCancelWithdrawStateSucceed,
		"withdraw_state_description": "提现成功",
		"modify_time":                "2026-04-26T20:40:00+08:00",
		"account_info":               []map[string]any{},
		"account_withdraw_result":    []map[string]any{},
	})
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    "MCW3402",
		ExternalSecondaryKey: pgtype.Text{String: applymentID, Valid: applymentID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectMerchantCancelWithdraw, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: applicationID, Valid: true},
		UpstreamState:        cancelState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          raw,
	}
}

func buildProfitSharingOrderForApplication(application db.ExternalPaymentFactApplication, status string) db.ProfitSharingOrder {
	return db.ProfitSharingOrder{
		ID:                 application.BusinessObjectID,
		PaymentOrderID:     9001,
		MerchantID:         801,
		OutOrderNo:         "PS3001",
		Status:             status,
		MerchantAmount:     1200,
		PlatformCommission: 100,
		OperatorCommission: 60,
	}
}

func expectProfitSharingResultOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, result string, failReason string) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateProfitSharingOrder, arg.AggregateType)
		require.Equal(t, application.BusinessObjectID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(application.BusinessObjectID), payload["profit_sharing_order_id"])
		require.Equal(t, "PS3001", payload["out_order_no"])
		require.Equal(t, result, payload["result"])
		if failReason == "" {
			require.NotContains(t, payload, "fail_reason")
		} else {
			require.Equal(t, failReason, payload["fail_reason"])
		}
		require.Equal(t, float64(801), payload["merchant_id"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{
			ID:            8001,
			EventType:     arg.EventType,
			AggregateType: arg.AggregateType,
			AggregateID:   arg.AggregateID,
			Payload:       arg.Payload,
			Status:        arg.Status,
		}, nil
	})
}

func expectApplymentActivatedOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, applyment db.EcommerceApplyment) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventApplymentActivated, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateEcommerceApplyment, arg.AggregateType)
		require.Equal(t, applyment.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(applyment.ID), payload["applyment_id"])
		require.Equal(t, float64(applyment.SubjectID), payload["merchant_id"])
		require.Equal(t, applyment.OutRequestNo, payload["out_request_no"])
		require.Equal(t, applyment.SubMchID.String, payload["sub_mch_id"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{
			ID:            85001,
			EventType:     arg.EventType,
			AggregateType: arg.AggregateType,
			AggregateID:   arg.AggregateID,
			Payload:       arg.Payload,
			Status:        arg.Status,
		}, nil
	})
}

func expectApplymentTerminalOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, applyment db.EcommerceApplyment) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventApplymentTerminalStateReady, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateEcommerceApplyment, arg.AggregateType)
		require.Equal(t, applyment.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(applyment.ID), payload["applyment_id"])
		require.Equal(t, float64(applyment.SubjectID), payload["merchant_id"])
		require.Equal(t, applyment.OutRequestNo, payload["out_request_no"])
		require.Equal(t, applyment.Status, payload["applyment_status"])
		require.Equal(t, applyment.RejectReason.String, payload["reject_reason"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{ID: 85002, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
}

func expectApplymentPendingOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, applyment db.EcommerceApplyment) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventApplymentPendingStateReady, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateEcommerceApplyment, arg.AggregateType)
		require.Equal(t, applyment.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(applyment.ID), payload["applyment_id"])
		require.Equal(t, float64(applyment.SubjectID), payload["merchant_id"])
		require.Equal(t, applyment.OutRequestNo, payload["out_request_no"])
		require.Equal(t, applyment.Status, payload["applyment_status"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{ID: 85003, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
}

func expectRiderDepositRefundAbnormalOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(refundOrder.ID), payload["refund_order_id"])
		require.Equal(t, float64(paymentOrder.ID), payload["payment_order_id"])
		require.Equal(t, refundOrder.OutRefundNo, payload["out_refund_no"])
		require.Equal(t, riderDepositRefundStatusAbnormal, payload["refund_status"])
		require.Equal(t, "WX_REFUND_RFD3002", payload["refund_id"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{
			ID:            8101,
			EventType:     arg.EventType,
			AggregateType: arg.AggregateType,
			AggregateID:   arg.AggregateID,
			Payload:       arg.Payload,
			Status:        arg.Status,
		}, nil
	})
}

func expectFactTerminalized(t *testing.T, store *mockdb.MockStore, factID int64, now time.Time) {
	t.Helper()
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), db.UpdateExternalPaymentFactProcessingStatusParams{
		ID:               factID,
		ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized,
		ProcessedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFact{ID: factID, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
}

func expectApplicationApplied(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, now time.Time) {
	t.Helper()
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 application.ID,
		FactID:             application.FactID,
		Consumer:           application.Consumer,
		BusinessObjectType: application.BusinessObjectType,
		BusinessObjectID:   application.BusinessObjectID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
	}, nil)
}

func expectApplicationFailed(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, now time.Time, errorSubstring string) {
	t.Helper()
	store.EXPECT().MarkExternalPaymentFactApplicationFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkExternalPaymentFactApplicationFailedParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, application.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.True(t, strings.Contains(arg.LastError.String, errorSubstring), "last_error %q does not contain %q", arg.LastError.String, errorSubstring)
		require.True(t, arg.NextRetryAt.Valid)
		require.Equal(t, now.Add(paymentFactApplicationRetryDelay), arg.NextRetryAt.Time)
		application.Status = db.ExternalPaymentFactApplicationStatusFailed
		application.LastError = arg.LastError
		application.NextRetryAt = arg.NextRetryAt
		return application, nil
	})
}

type testBaofuVerifyFeeContinuation struct {
	called       bool
	paymentOrder db.PaymentOrder
}

func (c *testBaofuVerifyFeeContinuation) ContinueAfterVerifyFeePaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	c.called = true
	c.paymentOrder = paymentOrder
	return nil
}
