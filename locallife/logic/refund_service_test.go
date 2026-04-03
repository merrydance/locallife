package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type refundServiceTaskSchedulerStub struct {
	profitSharingReturnInputs []ProfitSharingReturnResultTaskInput
}

func (s *refundServiceTaskSchedulerStub) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error {
	return nil
}

func (s *refundServiceTaskSchedulerStub) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error {
	return nil
}

func (s *refundServiceTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error {
	return nil
}

func (s *refundServiceTaskSchedulerStub) ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error {
	return nil
}

func (s *refundServiceTaskSchedulerStub) ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error {
	return nil
}

func (s *refundServiceTaskSchedulerStub) ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error {
	s.profitSharingReturnInputs = append(s.profitSharingReturnInputs, input)
	return nil
}

func (s *refundServiceTaskSchedulerStub) ScheduleOrderPrint(ctx context.Context, input OrderPrintTaskInput) error {
	return nil
}

type refundServiceIDGeneratorStub struct {
	outRefundNo string
}

func (s refundServiceIDGeneratorStub) OrderNo(now time.Time) (string, error) {
	return "", nil
}

func (s refundServiceIDGeneratorStub) PickupCode(now time.Time) (string, error) {
	return "", nil
}

func (s refundServiceIDGeneratorStub) OutTradeNo(prefix string, now time.Time) (string, error) {
	return "", nil
}

func (s refundServiceIDGeneratorStub) OutRefundNo(now time.Time) (string, error) {
	return s.outRefundNo, nil
}

func TestRefundServiceApplyAbnormalRefundProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(store, NewDefaultPaymentFacade(store, nil, ecommerceClient), nil, nil, nil)

	refundOrder := db.RefundOrder{
		ID:             11,
		PaymentOrderID: 22,
		OutRefundNo:    "R20250601001",
		RefundID:       pgtype.Text{String: "wx_refund_processing_001", Valid: true},
		RefundAmount:   1200,
		Status:         "failed",
	}
	paymentOrder := db.PaymentOrder{
		ID:          22,
		PaymentType: paymentTypeProfitSharing,
		Amount:      5000,
		OrderID:     pgtype.Int8{Int64: 33, Valid: true},
	}
	order := db.Order{ID: 33, MerchantID: 44}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: 44, SubMchID: "1900000109", Status: "active"}
	updatedRefund := refundOrder
	updatedRefund.Status = "processing"

	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Times(1).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Times(1).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), order.MerchantID).Times(1).Return(paymentConfig, nil)
	ecommerceClient.EXPECT().
		ApplyEcommerceAbnormalRefund(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.EcommerceAbnormalRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Equal(t, refundOrder.RefundID.String, req.RefundID)
			require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
			require.Equal(t, paymentConfig.SubMchID, req.SubMchID)
			require.Equal(t, wechat.EcommerceAbnormalRefundTypeUserBankCard, req.Type)
			require.Equal(t, "工商银行", req.BankType)
			require.Equal(t, "6222000000000000", req.BankAccount)
			require.Equal(t, "张三", req.RealName)
			return &wechat.EcommerceRefundResponse{
				RefundID: refundOrder.RefundID.String,
				Status:   wechat.RefundStatusProcessing,
			}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundOrder.RefundID.String, Valid: true},
		}).
		Times(1).
		Return(updatedRefund, nil)

	result, err := service.ApplyAbnormalRefund(context.Background(), ApplyAbnormalRefundInput{
		RefundID:    refundOrder.ID,
		Type:        wechat.EcommerceAbnormalRefundTypeUserBankCard,
		BankType:    "工商银行",
		BankAccount: "6222000000000000",
		RealName:    "张三",
	})

	require.NoError(t, err)
	require.Equal(t, "processing", result.RefundOrder.Status)
	require.Equal(t, wechat.RefundStatusProcessing, result.WechatRefund.Status)
}

func TestRefundServiceApplyAbnormalRefundRejectsNonFailedRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(store, NewDefaultPaymentFacade(store, nil, ecommerceClient), nil, nil, nil)
	refundOrder := db.RefundOrder{
		ID:     99,
		Status: "processing",
	}

	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Times(1).Return(refundOrder, nil)

	_, err := service.ApplyAbnormalRefund(context.Background(), ApplyAbnormalRefundInput{RefundID: refundOrder.ID})
	require.Error(t, err)

	var requestErr *RequestError
	require.True(t, errors.As(err, &requestErr))
	require.Equal(t, 400, requestErr.Status)
}

func TestCreateRefundOrder_ProfitSharingReturnAmbiguousErrorFallsBackToPolling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskScheduler := &refundServiceTaskSchedulerStub{}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		taskScheduler,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-001"},
	)

	merchant := db.Merchant{ID: 44, OwnerUserID: 7}
	paymentOrder := db.PaymentOrder{
		ID:          22,
		Amount:      1000,
		Status:      "paid",
		PaymentType: paymentTypeProfitSharing,
		OrderID:     pgtype.Int8{Int64: 33, Valid: true},
	}
	order := db.Order{ID: 33, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 55, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-001", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 66,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-logic-001",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-001", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   77,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-001",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR55PL",
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-001"}, nil)
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
		func(_ context.Context, req *wechat.ProfitSharingReturnRequest) (*wechat.ProfitSharingReturnResponse, error) {
			require.Equal(t, "sub-mchid-001", req.SubMchID)
			require.Equal(t, profitSharingOrder.OutOrderNo, req.OutOrderNo)
			require.Equal(t, returnRecord.OutReturnNo, req.OutReturnNo)
			require.Equal(t, wechat.ReceiverTypeMerchant, req.ReturnAccountType)
			require.Equal(t, "service-mchid-001", req.ReturnAccount)
			return nil, &wechat.WechatPayError{Code: "PAYER_ACCOUNT_ABNORMAL", Message: "分账方账户异常", StatusCode: 400}
		},
	)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{},
	}).Return(returnRecord, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.NoError(t, err)
	require.Equal(t, refundOrder.ID, result.RefundOrder.ID)
	require.Len(t, taskScheduler.profitSharingReturnInputs, 1)
	require.Equal(t, returnRecord.ID, taskScheduler.profitSharingReturnInputs[0].ProfitSharingReturnID)
	require.Equal(t, returnRecord.OutReturnNo, taskScheduler.profitSharingReturnInputs[0].OutReturnNo)
	require.Equal(t, returnRecord.OutOrderNo, taskScheduler.profitSharingReturnInputs[0].OutOrderNo)
	require.Equal(t, returnRecord.SubMchid, taskScheduler.profitSharingReturnInputs[0].SubMchID)
	require.Equal(t, refundOrder.ID, taskScheduler.profitSharingReturnInputs[0].RefundOrderID)
	require.Equal(t, 30*time.Second, taskScheduler.profitSharingReturnInputs[0].Delay)
}
