package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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
		ID:                    22,
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		Amount:                5000,
		OrderID:               pgtype.Int8{Int64: 33, Valid: true},
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

func TestCreateRefundOrder_ProfitSharingReturnNotEnoughFallsBackToPolling(t *testing.T) {
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
		ID:                    22,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		OrderID:               pgtype.Int8{Int64: 33, Valid: true},
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
	expectProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusUnknown, "NOT_ENOUGH", 9601)
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

func TestCreateRefundOrder_ProfitSharingReturnSuccessAppliesFactAndInitiatesRefund(t *testing.T) {
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
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-success-fact"},
	)

	merchant := db.Merchant{ID: 244, OwnerUserID: 27}
	paymentOrder := db.PaymentOrder{
		ID:                    222,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 233, Valid: true},
		OutTradeNo:            "order_paid_profit_sharing_return_success",
	}
	order := db.Order{ID: 233, MerchantID: merchant.ID}
	refreshedRefundOrder := db.RefundOrder{ID: 255, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-success-fact", Status: "pending", RefundAmount: 300, RefundReason: pgtype.Text{String: "商品售罄", Valid: true}}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 266,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-logic-success-fact",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-success-fact", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   277,
		RefundOrderID:        refreshedRefundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-success-fact",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR255PL",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}
	commandFact := db.ExternalPaymentFact{
		ID:                   377,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: pgtype.Text{String: "wx-return-success-fact", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "profit_sharing_return", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: returnRecord.ID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		IsTerminal:           false,
		DedupeKey:            "wechat:command_response:ecommerce:profit_sharing_return:" + returnRecord.OutReturnNo + ":SUCCESS",
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
		OutRefundNo:    refreshedRefundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refreshedRefundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: returnRecord.SubMchid}, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refreshedRefundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             returnRecord.SubMchid,
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-success-fact",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-success-fact")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
			require.Equal(t, "service-mchid-success-fact", req.ReturnMchID)
			return &wechatcontracts.ProfitSharingReturnResponse{
				SubMchID:    returnRecord.SubMchid,
				OutOrderNo:  returnRecord.OutOrderNo,
				OutReturnNo: returnRecord.OutReturnNo,
				ReturnID:    "wx-return-success-fact",
				Amount:      returnRecord.Amount,
				Result:      "SUCCESS",
			}, nil
		},
	)
	expectProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "wx-return-success-fact", db.ExternalPaymentCommandStatusAccepted, "", 9801)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-success-fact", Valid: true},
	}).Return(db.ProfitSharingReturn{ID: returnRecord.ID, RefundOrderID: returnRecord.RefundOrderID, PaymentOrderID: returnRecord.PaymentOrderID, OutReturnNo: returnRecord.OutReturnNo, Status: "processing"}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return commandFact, nil
	})
	store.EXPECT().GetRefundOrder(gomock.Any(), refreshedRefundOrder.ID).Return(db.RefundOrder{ID: refreshedRefundOrder.ID, OutRefundNo: refreshedRefundOrder.OutRefundNo, Status: "processing"}, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.NoError(t, err)
	require.Equal(t, refreshedRefundOrder.ID, result.RefundOrder.ID)
	require.Len(t, taskScheduler.profitSharingReturnInputs, 1)
	require.Equal(t, returnRecord.ID, taskScheduler.profitSharingReturnInputs[0].ProfitSharingReturnID)
}

func TestCreateRefundOrder_ProfitSharingReturnSuccessThenProcessingDoesNotStartRefundEarly(t *testing.T) {
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
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-success-processing"},
	)

	merchant := db.Merchant{ID: 344, OwnerUserID: 37}
	paymentOrder := db.PaymentOrder{
		ID:                    322,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 333, Valid: true},
		OutTradeNo:            "order_paid_profit_sharing_return_mixed",
	}
	order := db.Order{ID: 333, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 355, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-success-processing", Status: "pending", RefundAmount: 300, RefundReason: pgtype.Text{String: "商品售罄", Valid: true}}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 366,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-logic-success-processing",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-success-processing", Valid: true},
		PlatformCommission: 300,
		OperatorCommission: 200,
		OperatorID:         pgtype.Int8{Int64: 388, Valid: true},
	}
	operator := db.Operator{ID: 388, UserID: 389, WechatMchID: pgtype.Text{String: "operator-mchid-388", Valid: true}}
	platformReturn := db.ProfitSharingReturn{ID: 377, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-success-processing", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR355PL", Amount: 300, Status: "pending"}
	operatorReturn := db.ProfitSharingReturn{ID: 378, RefundOrderID: refundOrder.ID, ProfitSharingOrderID: profitSharingOrder.ID, PaymentOrderID: paymentOrder.ID, SubMchid: "sub-mchid-success-processing", OutOrderNo: profitSharingOrder.OutOrderNo, OutReturnNo: "PR355OP", Amount: 200, Status: "pending"}
	platformFact := db.ExternalPaymentFact{
		ID:                   477,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    platformReturn.OutReturnNo,
		ExternalSecondaryKey: pgtype.Text{String: "wx-return-platform-355", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "profit_sharing_return", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: platformReturn.ID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		IsTerminal:           false,
		DedupeKey:            "wechat:command_response:ecommerce:profit_sharing_return:" + platformReturn.OutReturnNo + ":SUCCESS",
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: platformReturn.SubMchid}, nil)
	store.EXPECT().GetOperator(gomock.Any(), profitSharingOrder.OperatorID.Int64).Return(operator, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), platformReturn.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             platformReturn.SubMchid,
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          platformReturn.OutReturnNo,
		ReturnMchid:          "service-mchid-processing",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(platformReturn, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-processing")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    platformReturn.SubMchid,
		OutOrderNo:  platformReturn.OutOrderNo,
		OutReturnNo: platformReturn.OutReturnNo,
		ReturnID:    "wx-return-platform-355",
		Amount:      platformReturn.Amount,
		Result:      "SUCCESS",
	}, nil)
	expectProfitSharingReturnCommand(t, store, platformReturn.ID, platformReturn.OutReturnNo, platformReturn.OutOrderNo, "wx-return-platform-355", db.ExternalPaymentCommandStatusAccepted, "", 9802)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       platformReturn.ID,
		ReturnID: pgtype.Text{String: "wx-return-platform-355", Valid: true},
	}).Return(db.ProfitSharingReturn{ID: platformReturn.ID, RefundOrderID: platformReturn.RefundOrderID, PaymentOrderID: platformReturn.PaymentOrderID, OutReturnNo: platformReturn.OutReturnNo, Status: "processing"}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, platformReturn.OutReturnNo, arg.ExternalObjectKey)
		return platformFact, nil
	})
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), operatorReturn.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             operatorReturn.SubMchid,
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          operatorReturn.OutReturnNo,
		ReturnMchid:          operator.WechatMchID.String,
		Amount:               profitSharingOrder.OperatorCommission,
		Status:               "pending",
	}).Return(operatorReturn, nil)
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    operatorReturn.SubMchid,
		OutOrderNo:  operatorReturn.OutOrderNo,
		OutReturnNo: operatorReturn.OutReturnNo,
		ReturnID:    "wx-return-operator-355",
		Amount:      operatorReturn.Amount,
		Result:      "PROCESSING",
	}, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       operatorReturn.ID,
		ReturnID: pgtype.Text{String: "wx-return-operator-355", Valid: true},
	}).Return(db.ProfitSharingReturn{ID: operatorReturn.ID, RefundOrderID: operatorReturn.RefundOrderID, PaymentOrderID: operatorReturn.PaymentOrderID, OutReturnNo: operatorReturn.OutReturnNo, Status: "processing"}, nil)
	expectProfitSharingReturnCommand(t, store, operatorReturn.ID, operatorReturn.OutReturnNo, operatorReturn.OutOrderNo, "wx-return-operator-355", db.ExternalPaymentCommandStatusAccepted, "", 9803)
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
	require.Len(t, taskScheduler.profitSharingReturnInputs, 2)
	require.Equal(t, platformReturn.ID, taskScheduler.profitSharingReturnInputs[0].ProfitSharingReturnID)
	require.Equal(t, operatorReturn.ID, taskScheduler.profitSharingReturnInputs[1].ProfitSharingReturnID)
}

func TestCreateRefundOrder_ProfitSharingReturnRejectedErrorAppliesFailedFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-return-rejected-fact"},
	)

	merchant := db.Merchant{ID: 444, OwnerUserID: 47}
	paymentOrder := db.PaymentOrder{
		ID:                    422,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 433, Valid: true},
		OutTradeNo:            "order_paid_profit_sharing_return_rejected",
	}
	order := db.Order{ID: 433, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 455, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-return-rejected-fact", Status: "pending", RefundAmount: 300, RefundReason: pgtype.Text{String: "商品售罄", Valid: true}}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 466,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-logic-return-rejected-fact",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-return-rejected-fact", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   477,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-return-rejected-fact",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR455PL",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}
	commandFact := db.ExternalPaymentFact{
		ID:                 577,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityProfitSharing,
		FactSource:         db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:  returnRecord.OutReturnNo,
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType: pgtype.Text{String: "profit_sharing_return", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: returnRecord.ID, Valid: true},
		UpstreamState:      db.ExternalPaymentTerminalStatusFailed,
		TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
		IsTerminal:         false,
		RawResource:        []byte(`{"fail_reason":"分账方账户异常"}`),
		DedupeKey:          "wechat:command_response:ecommerce:profit_sharing_return:" + returnRecord.OutReturnNo + ":rejected",
		ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: returnRecord.SubMchid}, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             returnRecord.SubMchid,
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-return-rejected-fact",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-return-rejected-fact")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PAYER_ACCOUNT_ABNORMAL", Message: "分账方账户异常"})
	expectProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusRejected, "PAYER_ACCOUNT_ABNORMAL", 9804)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return commandFact, nil
	})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "platform profit sharing return")
	require.Contains(t, err.Error(), "PAYER_ACCOUNT_ABNORMAL")
}

func TestCreateRefundOrder_ProfitSharingReturnUnknownResultFallsBackToPolling(t *testing.T) {
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
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-return-unknown"},
	)

	merchant := db.Merchant{ID: 544, OwnerUserID: 57}
	paymentOrder := db.PaymentOrder{
		ID:                    522,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 533, Valid: true},
		OutTradeNo:            "order_paid_profit_sharing_return_unknown",
	}
	order := db.Order{ID: 533, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 555, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-return-unknown", Status: "pending", RefundAmount: 300, RefundReason: pgtype.Text{String: "商品售罄", Valid: true}}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 566,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-logic-return-unknown",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-return-unknown", Valid: true},
		PlatformCommission: 300,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   577,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-return-unknown",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR555PL",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}
	unknownFact := db.ExternalPaymentFact{
		ID:                   677,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: pgtype.Text{String: "wx-return-unknown-555", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "profit_sharing_return", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: returnRecord.ID, Valid: true},
		UpstreamState:        "SOMETHING_NEW",
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		IsTerminal:           false,
		DedupeKey:            "wechat:command_response:ecommerce:profit_sharing_return:" + returnRecord.OutReturnNo + ":unknown",
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: returnRecord.SubMchid}, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             returnRecord.SubMchid,
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          "service-mchid-return-unknown",
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("service-mchid-return-unknown")
	ecommerceClient.EXPECT().CreateProfitSharingReturn(gomock.Any(), gomock.Any()).Return(&wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:    returnRecord.SubMchid,
		OutOrderNo:  returnRecord.OutOrderNo,
		OutReturnNo: returnRecord.OutReturnNo,
		ReturnID:    "wx-return-unknown-555",
		Amount:      returnRecord.Amount,
		Result:      "SOMETHING_NEW",
	}, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-unknown-555", Valid: true},
	}).Return(db.ProfitSharingReturn{ID: returnRecord.ID, RefundOrderID: returnRecord.RefundOrderID, PaymentOrderID: returnRecord.PaymentOrderID, OutReturnNo: returnRecord.OutReturnNo, Status: "processing"}, nil)
	expectProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "", db.ExternalPaymentCommandStatusUnknown, "", 9805)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
		require.Equal(t, returnRecord.OutReturnNo, arg.ExternalObjectKey)
		return unknownFact, nil
	})
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
	require.Equal(t, 30*time.Second, taskScheduler.profitSharingReturnInputs[0].Delay)
}

func TestCreateRefundOrder_EcommerceRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-accepted"},
	)

	merchant := db.Merchant{ID: 144, OwnerUserID: 17}
	paymentOrder := db.PaymentOrder{
		ID:                    122,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 133, Valid: true},
		OutTradeNo:            "order_paid_accepted",
	}
	order := db.Order{ID: 133, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 155, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-accepted", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             166,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-logic-accepted",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-accepted", Valid: true},
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-accepted"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Equal(t, "sub-mchid-accepted", req.SubMchID)
			require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
			require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
			require.Equal(t, int64(300), req.RefundAmount)
			return &wechat.EcommerceRefundResponse{RefundID: "erefund_service_accepted"}, nil
		},
	)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "erefund_service_accepted", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectRefundServiceEcommerceRefundCommand(t, store, refundOrder.ID, refundOrder.OutRefundNo, "erefund_service_accepted", db.ExternalPaymentCommandStatusAccepted, "", 9501)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.NoError(t, err)
	require.Equal(t, refundOrder.ID, result.RefundOrder.ID)
}

func TestCreateRefundOrder_OrdinaryServiceProviderRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createRefundResponse: &ospcontracts.RefundResponse{RefundID: "orefund_service_accepted"}}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithOrdinaryServiceProvider(store, nil, nil, ordinaryClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-ordinary-accepted"},
	)

	merchant := db.Merchant{ID: 244, OwnerUserID: 27}
	paymentOrder := db.PaymentOrder{
		ID:                    222,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelOrdinaryServiceProvider,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 233, Valid: true},
		OutTradeNo:            "ordinary_paid_accepted",
	}
	order := db.Order{ID: 233, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 255, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-ordinary-accepted", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             266,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-ordinary-accepted",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-ordinary-accepted", Valid: true},
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-ordinary"}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "orefund_service_accepted", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelOrdinaryServiceProvider, db.ExternalPaymentCapabilityPartnerRefund, refundOrder.ID, refundOrder.OutRefundNo, "orefund_service_accepted", db.ExternalPaymentCommandStatusAccepted, "", 9601)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.NoError(t, err)
	require.Equal(t, refundOrder.ID, result.RefundOrder.ID)
	require.NotNil(t, ordinaryClient.createRefundRequest)
	require.Equal(t, "sub-mchid-ordinary", ordinaryClient.createRefundRequest.SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createRefundRequest.OutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, ordinaryClient.createRefundRequest.OutRefundNo)
	require.Equal(t, "商品售罄", ordinaryClient.createRefundRequest.Reason)
	require.Equal(t, ordinaryClient.RefundNotifyURL(), ordinaryClient.createRefundRequest.NotifyURL)
	require.Equal(t, int64(300), ordinaryClient.createRefundRequest.Amount.Refund)
	require.Equal(t, paymentOrder.Amount, ordinaryClient.createRefundRequest.Amount.Total)
	require.Equal(t, ospcontracts.CurrencyCNY, ordinaryClient.createRefundRequest.Amount.Currency)
}

func TestCreateRefundOrder_OrdinaryServiceProviderProfitSharingReturnUsesOrdinaryClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{
		createReturnResponse: &ospcontracts.ProfitSharingReturnResponse{
			SubMchID:    "sub-mchid-ordinary-return",
			OrderID:     "wx-profitsharing-ordinary-return",
			OutOrderNo:  "PS-ordinary-return",
			OutReturnNo: "PR257PL",
			ReturnID:    "wx-return-ordinary-257",
			ReturnMchID: "1900000109",
			Amount:      200,
			State:       ospcontracts.ProfitSharingReturnStateProcessing,
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithOrdinaryServiceProvider(store, nil, nil, ordinaryClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-ordinary-return"},
	)

	merchant := db.Merchant{ID: 246, OwnerUserID: 29}
	paymentOrder := db.PaymentOrder{
		ID:                    224,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelOrdinaryServiceProvider,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 235, Valid: true},
		OutTradeNo:            "ordinary_paid_return",
		TransactionID:         pgtype.Text{String: "wx-tx-ordinary-return", Valid: true},
	}
	order := db.Order{ID: 235, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 257, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-ordinary-return", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 268,
		MerchantID:         merchant.ID,
		OutOrderNo:         "PS-ordinary-return",
		SharingOrderID:     pgtype.Text{String: "wx-profitsharing-ordinary-return", Valid: true},
		PlatformCommission: 200,
	}
	returnRecord := db.ProfitSharingReturn{
		ID:                   269,
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             "sub-mchid-ordinary-return",
		OutOrderNo:           profitSharingOrder.OutOrderNo,
		OutReturnNo:          "PR257PL",
		ReturnMchid:          ordinaryClient.ServiceProviderMchID(),
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-ordinary-return"}, nil)
	store.EXPECT().GetProfitSharingReturnByOutReturnNo(gomock.Any(), returnRecord.OutReturnNo).Return(db.ProfitSharingReturn{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingReturn(gomock.Any(), db.CreateProfitSharingReturnParams{
		RefundOrderID:        refundOrder.ID,
		ProfitSharingOrderID: profitSharingOrder.ID,
		PaymentOrderID:       paymentOrder.ID,
		SubMchid:             returnRecord.SubMchid,
		OutOrderNo:           returnRecord.OutOrderNo,
		OutReturnNo:          returnRecord.OutReturnNo,
		ReturnMchid:          ordinaryClient.ServiceProviderMchID(),
		Amount:               profitSharingOrder.PlatformCommission,
		Status:               "pending",
	}).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
		ID:       returnRecord.ID,
		ReturnID: pgtype.Text{String: "wx-return-ordinary-257", Valid: true},
	}).Return(returnRecord, nil)
	expectProfitSharingReturnCommand(t, store, returnRecord.ID, returnRecord.OutReturnNo, returnRecord.OutOrderNo, "wx-return-ordinary-257", db.ExternalPaymentCommandStatusAccepted, "", 9701, db.PaymentChannelOrdinaryServiceProvider)
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
	require.Nil(t, ordinaryClient.createRefundRequest)
	require.NotNil(t, ordinaryClient.createReturnRequest)
	require.Equal(t, returnRecord.SubMchid, ordinaryClient.createReturnRequest.SubMchID)
	require.Equal(t, profitSharingOrder.SharingOrderID.String, ordinaryClient.createReturnRequest.OrderID)
	require.Equal(t, returnRecord.OutOrderNo, ordinaryClient.createReturnRequest.OutOrderNo)
	require.Equal(t, returnRecord.OutReturnNo, ordinaryClient.createReturnRequest.OutReturnNo)
	require.Equal(t, ordinaryClient.ServiceProviderMchID(), ordinaryClient.createReturnRequest.ReturnMchID)
	require.Equal(t, profitSharingOrder.PlatformCommission, ordinaryClient.createReturnRequest.Amount)
}

func TestCreateRefundOrder_EcommerceRefundRejectedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-rejected"},
	)

	merchant := db.Merchant{ID: 145, OwnerUserID: 18}
	paymentOrder := db.PaymentOrder{
		ID:                    123,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 134, Valid: true},
		OutTradeNo:            "order_paid_rejected",
	}
	order := db.Order{ID: 134, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 156, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-rejected", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             167,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-logic-rejected",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-rejected", Valid: true},
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-rejected"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)
	expectRefundServiceEcommerceRefundCommand(t, store, refundOrder.ID, refundOrder.OutRefundNo, "", db.ExternalPaymentCommandStatusRejected, "SYSTEM_ERROR", 9502)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, requestErr.Status)
}

func TestCreateRefundOrder_OrdinaryServiceProviderRefundRejectedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createRefundErr: &ordinaryserviceprovider.ProviderError{
		Operation:    "refund.create",
		StatusCode:   http.StatusForbidden,
		ProviderCode: "NOT_ENOUGH",
		Category:     ordinaryserviceprovider.ErrorCategoryBusinessConflict,
		Frontend: ordinaryserviceprovider.FrontendGuidance{
			Code:    "WECHAT_BUSINESS_CONFLICT",
			Message: "微信支付返回业务状态冲突",
			Action:  "请刷新微信侧退款状态后再处理",
		},
	}}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithOrdinaryServiceProvider(store, nil, nil, ordinaryClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-ordinary-rejected"},
	)

	merchant := db.Merchant{ID: 245, OwnerUserID: 28}
	paymentOrder := db.PaymentOrder{
		ID:                    223,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelOrdinaryServiceProvider,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 234, Valid: true},
		OutTradeNo:            "ordinary_paid_rejected",
	}
	order := db.Order{ID: 234, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 256, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-ordinary-rejected", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             267,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-ordinary-rejected",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-ordinary-rejected", Valid: true},
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-ordinary-rejected"}, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelOrdinaryServiceProvider, db.ExternalPaymentCapabilityPartnerRefund, refundOrder.ID, refundOrder.OutRefundNo, "", db.ExternalPaymentCommandStatusRejected, "NOT_ENOUGH", 9602)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, requestErr.Status)
	require.Contains(t, requestErr.Error(), "微信支付返回业务状态冲突")
	require.NotNil(t, ordinaryClient.createRefundRequest)
}

func TestCreateRefundOrder_EcommerceRefundRejectedSkipsCommandWhenFailedUpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-rejected-db-fail"},
	)

	merchant := db.Merchant{ID: 146, OwnerUserID: 19}
	paymentOrder := db.PaymentOrder{
		ID:                    124,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 135, Valid: true},
		OutTradeNo:            "order_paid_rejected_db_fail",
	}
	order := db.Order{ID: 135, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 157, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-rejected-db-fail", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             168,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-logic-rejected-db-fail",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-rejected-db-fail", Valid: true},
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-rejected-db-fail"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{}, errors.New("failed update unavailable"))

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, requestErr.Status)
}

func TestCreateRefundOrder_BlocksPersonalProfitSharingReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, ecommerceClient),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-002"},
	)

	merchant := db.Merchant{ID: 44, OwnerUserID: 7}
	paymentOrder := db.PaymentOrder{
		ID:                    23,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           paymentTypeProfitSharing,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		TransactionID:         pgtype.Text{String: "wx_txn_23", Valid: true},
		OrderID:               pgtype.Int8{Int64: 33, Valid: true},
	}
	order := db.Order{ID: 33, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 56, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-logic-002", Status: "pending"}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             67,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS-logic-002",
		SharingOrderID: pgtype.Text{String: "wx-profitsharing-002", Valid: true},
		RiderAmount:    300,
		RiderID:        pgtype.Int8{Int64: 88, Valid: true},
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
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub-mchid-002"}, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	require.Error(t, err)

	var requestErr *RequestError
	require.True(t, errors.As(err, &requestErr))
	require.Equal(t, 400, requestErr.Status)
	require.Contains(t, err.Error(), "订单包含个人分账，当前不支持自动退款")
}

func expectRefundServiceEcommerceRefundCommand(t *testing.T, store *mockdb.MockStore, refundOrderID int64, outRefundNo string, secondaryKey string, status string, errorCode string, commandID int64) {
	t.Helper()
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelEcommerce, db.ExternalPaymentCapabilityEcommerceRefund, refundOrderID, outRefundNo, secondaryKey, status, errorCode, commandID)
}

func expectRefundServiceExternalRefundCommand(t *testing.T, store *mockdb.MockStore, channel string, capability string, refundOrderID int64, outRefundNo string, secondaryKey string, status string, errorCode string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, channel, arg.Channel)
		require.Equal(t, capability, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "refund_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, refundOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
		require.Equal(t, outRefundNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outRefundNo)
		if secondaryKey != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
			require.Contains(t, string(arg.ResponseSnapshot), secondaryKey)
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

func expectProfitSharingReturnCommand(t *testing.T, store *mockdb.MockStore, returnID int64, outReturnNo string, outOrderNo string, secondaryKey string, status string, errorCode string, commandID int64, channels ...string) {
	t.Helper()
	channel := db.PaymentChannelEcommerce
	if len(channels) > 0 && channels[0] != "" {
		channel = channels[0]
	}

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

func TestCreateRefundOrder_RejectsMainBusinessNonWechatServiceProviderPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil, nil),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-logic-003"},
	)

	merchant := db.Merchant{ID: 45, OwnerUserID: 8}
	paymentOrder := db.PaymentOrder{
		ID:          24,
		Amount:      1000,
		Status:      "paid",
		PaymentType: paymentTypeMiniProgram,
		OrderID:     pgtype.Int8{Int64: 34, Valid: true},
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	requestErr := assertRequestError(t, err)
	require.Equal(t, 409, requestErr.Status)
	require.Equal(t, "当前主营业务支付单不属于微信服务商链路，无法发起退款，请联系平台处理", requestErr.Err.Error())
}
