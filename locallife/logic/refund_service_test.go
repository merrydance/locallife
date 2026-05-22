package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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

func TestRefundServiceBaofuRefundBusinessOwnerUsesReservationForAddonPayments(t *testing.T) {
	paymentOrder := db.PaymentOrder{
		BusinessType:  reservationAddonBusiness,
		ReservationID: pgtype.Int8{Int64: 7101, Valid: true},
		OrderID:       pgtype.Int8{Int64: 8101, Valid: true},
	}

	require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, refundServiceBaofuRefundBusinessOwner(paymentOrder))
}

func TestCreateRefundOrder_BaofuShareStartedReturnsSettlementBusinessError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewRefundService(
		store,
		nil,
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-share-started"},
	)

	merchant := db.Merchant{ID: 644, OwnerUserID: 67}
	paymentOrder := db.PaymentOrder{
		ID:                    622,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 633, Valid: true},
	}
	order := db.Order{ID: 633, MerchantID: merchant.ID}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: true,
		}, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})
	require.Error(t, err)

	var requestErr *RequestError
	require.True(t, errors.As(err, &requestErr))
	require.Equal(t, http.StatusBadRequest, requestErr.Status)
	require.EqualError(t, requestErr.Err, "订单已进入结算分账流程，不支持退款")
}

func TestCreateRefundOrder_BaofuPreShareRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuRefundAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:  "RF-baofu-pre-share",
			TradeNo:     "BFREFUND202605040001",
			RefundState: aggregatecontracts.RefundStateAccepted,
			ResultCode:  "SUCCESS",
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithBaofuAggregate(store, nil, baofuClient, BaofuAggregateFacadeConfig{
			CollectMerchantID: "102004465",
			CollectTerminalID: "200005200",
			RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
		}),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-pre-share"},
	)

	merchant := db.Merchant{ID: 645, OwnerUserID: 68}
	paymentOrder := db.PaymentOrder{
		ID:                    623,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 634, Valid: true},
		OutTradeNo:            "BF202605040001",
		TransactionID:         pgtype.Text{String: "BFPAY_UP_202605040001", Valid: true},
	}
	order := db.Order{ID: 634, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 656, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-baofu-pre-share", Status: "pending"}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: false,
		}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "BFREFUND202605040001", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND202605040001", db.ExternalPaymentCommandStatusAccepted, "", 9901, db.ExternalPaymentProviderBaofu)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})

	require.NoError(t, err)
	require.Equal(t, refundOrder.ID, result.RefundOrder.ID)
	require.Equal(t, "102004465", baofuClient.lastRefundRequest.MerchantID)
	require.Equal(t, "200005200", baofuClient.lastRefundRequest.TerminalID)
	require.Equal(t, "BFPAY_UP_202605040001", baofuClient.lastRefundRequest.OriginTradeNo)
	require.Empty(t, baofuClient.lastRefundRequest.OriginOutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.RefundAmountFen)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.TotalAmountFen)
	require.Empty(t, baofuClient.lastRefundRequest.SharingRefundInfo)
}

func TestCreateRefundOrder_BaofuPreShareRefundRejectedRecordsGuidanceFromRefundResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuRefundAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OriginTradeNo:    "BFPAY_UP_202605040004",
			OutTradeNo:       "BFREFUND202605040004",
			TradeNo:          "BFREFUND_UP_202605040004",
			RefundAmountFen:  300,
			TotalAmountFen:   300,
			ResultCode:       aggregatecontracts.BusinessResultCodeFail,
			ErrorCode:        "REFUND_AMT_EXCEEDS",
			ErrorMessage:     "raw upstream refund amount detail",
			RefundState:      aggregatecontracts.RefundStateError,
			SuccessAmountFen: 0,
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithBaofuAggregate(store, nil, baofuClient, BaofuAggregateFacadeConfig{
			CollectMerchantID: "102004465",
			CollectTerminalID: "200005200",
			RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
		}),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-rejected-response"},
	)

	merchant := db.Merchant{ID: 647, OwnerUserID: 70}
	paymentOrder := db.PaymentOrder{
		ID:             625,
		Amount:         1000,
		Status:         "paid",
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   businessTypeOrder,
		OrderID:        pgtype.Int8{Int64: 636, Valid: true},
		OutTradeNo:     "BF202605040004",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_202605040004", Valid: true},
	}
	order := db.Order{ID: 636, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 659, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-baofu-rejected-response", Status: "pending"}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: false,
		}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
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
		require.Contains(t, string(arg.ResponseSnapshot), `"error_present":true`)
		require.Contains(t, string(arg.ResponseSnapshot), refundOrder.OutRefundNo)
		require.Contains(t, string(arg.ResponseSnapshot), "REFUND_AMT_EXCEEDS")
		require.NotContains(t, string(arg.ResponseSnapshot), "raw upstream")
		return db.ExternalPaymentCommand{ID: 9905}, nil
	})

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})

	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, requestErr.Status)
}

func TestCreateRefundOrder_BaofuPreShareRefundFallsBackToOriginOutTradeNoWhenTransactionIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuRefundAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:  "RF-baofu-origin-out-trade",
			TradeNo:     "BFREFUND202605040011",
			RefundState: aggregatecontracts.RefundStateAccepted,
			ResultCode:  "SUCCESS",
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithBaofuAggregate(store, nil, baofuClient, BaofuAggregateFacadeConfig{
			CollectMerchantID: "102004465",
			CollectTerminalID: "200005200",
			RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
		}),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-origin-out-trade"},
	)

	merchant := db.Merchant{ID: 655, OwnerUserID: 78}
	paymentOrder := db.PaymentOrder{
		ID:                    633,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 644, Valid: true},
		OutTradeNo:            "BF202605040011",
	}
	order := db.Order{ID: 644, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 666, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-baofu-origin-out-trade", Status: "pending"}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: false,
		}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "BFREFUND202605040011", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND202605040011", db.ExternalPaymentCommandStatusAccepted, "", 9903, db.ExternalPaymentProviderBaofu)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})

	require.NoError(t, err)
	require.Empty(t, baofuClient.lastRefundRequest.OriginTradeNo)
	require.Equal(t, paymentOrder.OutTradeNo, baofuClient.lastRefundRequest.OriginOutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.TotalAmountFen)
}

func TestCreateRefundOrder_BaofuPreShareRefundSyncSuccessStillWaitsForQueryOrCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuRefundAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:  "RF-baofu-sync-success",
			TradeNo:     "BFREFUND202605040002",
			RefundState: aggregatecontracts.RefundStateSuccess,
			ResultCode:  "SUCCESS",
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithBaofuAggregate(store, nil, baofuClient, BaofuAggregateFacadeConfig{
			CollectMerchantID: "102004465",
			CollectTerminalID: "200005200",
			RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
		}),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-sync-success"},
	)

	merchant := db.Merchant{ID: 646, OwnerUserID: 69}
	paymentOrder := db.PaymentOrder{
		ID:                    624,
		Amount:                1000,
		Status:                "paid",
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		OrderID:               pgtype.Int8{Int64: 635, Valid: true},
		OutTradeNo:            "BF202605040002",
	}
	order := db.Order{ID: 635, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 657, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-baofu-sync-success", Status: "pending"}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: false,
		}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
		OutRefundNo:    refundOrder.OutRefundNo,
	}).Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: "BFREFUND202605040002", Valid: true},
	}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	expectRefundServiceExternalRefundCommand(t, store, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND202605040002", db.ExternalPaymentCommandStatusAccepted, "", 9902, db.ExternalPaymentProviderBaofu)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)

	result, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})

	require.NoError(t, err)
	require.Equal(t, refundOrder.ID, result.RefundOrder.ID)
	require.Equal(t, "processing", result.RefundOrder.Status)
	require.Empty(t, baofuClient.lastRefundRequest.OriginTradeNo)
	require.Equal(t, paymentOrder.OutTradeNo, baofuClient.lastRefundRequest.OriginOutTradeNo)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.TotalAmountFen)
}

func TestCreateRefundOrder_BaofuPreShareRefundProviderErrorRecordsCommandRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuRefundAggregateClient{
		refundErr: &baofu.ProviderError{
			Operation:       "order_refund",
			UpstreamCode:    "REFUND_AMT_EXCEEDS",
			UpstreamMessage: "raw upstream refund amount detail",
			Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "raw upstream refund amount detail").FrontendGuidance(),
		},
	}
	service := NewRefundService(
		store,
		NewDefaultPaymentFacadeWithBaofuAggregate(store, nil, baofuClient, BaofuAggregateFacadeConfig{
			CollectMerchantID: "102004465",
			CollectTerminalID: "200005200",
			RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
		}),
		nil,
		nil,
		refundServiceIDGeneratorStub{outRefundNo: "RF-baofu-provider-error"},
	)

	merchant := db.Merchant{ID: 647, OwnerUserID: 70}
	paymentOrder := db.PaymentOrder{
		ID:             625,
		Amount:         1000,
		Status:         "paid",
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   businessTypeOrder,
		OrderID:        pgtype.Int8{Int64: 636, Valid: true},
		OutTradeNo:     "BF202605040003",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_202605040003", Valid: true},
	}
	order := db.Order{ID: 636, MerchantID: merchant.ID}
	refundOrder := db.RefundOrder{ID: 658, PaymentOrderID: paymentOrder.ID, OutRefundNo: "RF-baofu-provider-error", Status: "pending"}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), merchant.OwnerUserID).Return(merchant, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		GetBaofuPaymentOrderRefundGuardForUpdate(gomock.Any(), paymentOrder.ID).
		Return(db.GetBaofuPaymentOrderRefundGuardForUpdateRow{
			ID:                      paymentOrder.ID,
			Status:                  paymentOrder.Status,
			PaymentChannel:          db.PaymentChannelBaofuAggregate,
			HasStartedProfitSharing: false,
		}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
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
		return db.ExternalPaymentCommand{ID: 9904}, nil
	})

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   300,
		RefundReason:   "用户申请退款",
	})

	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, requestErr.Status)
}

func TestCreateRefundOrder_RejectsMainBusinessNonBaofuPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewRefundService(
		store,
		NewDefaultPaymentFacade(store, nil),
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
	store.EXPECT().GetOrder(gomock.Any(), paymentOrder.OrderID.Int64).Return(db.Order{ID: paymentOrder.OrderID.Int64, MerchantID: merchant.ID}, nil)

	_, err := service.CreateRefundOrder(context.Background(), CreateRefundOrderInput{
		ActorUserID:    merchant.OwnerUserID,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "merchant_cancel",
		RefundAmount:   300,
		RefundReason:   "商品售罄",
	})
	requestErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, requestErr.Status)
	require.Equal(t, "当前主营业务支付单仅支持宝付链路，无法发起退款，请联系平台处理", requestErr.Err.Error())
}

func expectRefundServiceExternalRefundCommand(t *testing.T, store *mockdb.MockStore, channel string, capability string, refundOrderID int64, outRefundNo string, secondaryKey string, status string, errorCode string, commandID int64, providers ...string) {
	t.Helper()
	provider := db.ExternalPaymentProviderBaofu
	if len(providers) > 0 && providers[0] != "" {
		provider = providers[0]
	}

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, provider, arg.Provider)
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

type fakeBaofuRefundAggregateClient struct {
	lastRefundRequest aggregatecontracts.RefundBeforeShareRequest
	refundResult      *aggregatecontracts.RefundResult
	refundErr         error
}

func (c *fakeBaofuRefundAggregateClient) CreateUnifiedOrder(ctx context.Context, req aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in refund tests")
}

func (c *fakeBaofuRefundAggregateClient) QueryPayment(ctx context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in refund tests")
}

func (c *fakeBaofuRefundAggregateClient) CreateProfitSharing(ctx context.Context, req aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in refund tests")
}

func (c *fakeBaofuRefundAggregateClient) QueryProfitSharing(ctx context.Context, req aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in refund tests")
}

func (c *fakeBaofuRefundAggregateClient) CreateRefund(ctx context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	c.lastRefundRequest = req
	if c.refundErr != nil {
		return nil, c.refundErr
	}
	return c.refundResult, nil
}

func (c *fakeBaofuRefundAggregateClient) QueryRefund(ctx context.Context, req aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in refund tests")
}

func (c *fakeBaofuRefundAggregateClient) CloseOrder(ctx context.Context, req aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, errors.New("not implemented in refund tests")
}
