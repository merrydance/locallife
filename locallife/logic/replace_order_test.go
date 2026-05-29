package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessReplaceOrderRefundWithBaofuBuildsBaofooRefundWithOriginalPaymentReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{
		baofuRefund: &aggregatecontracts.RefundResult{
			OutTradeNo:      "R202605230001",
			TradeNo:         "BFRFD202605230001",
			RefundAmountFen: 3500,
			TotalAmountFen:  3500,
			ResultCode:      aggregatecontracts.BusinessResultCodeSuccess,
			RefundState:     aggregatecontracts.RefundStateAccepted,
		},
	}
	paymentOrder := db.PaymentOrder{
		ID:             5101,
		OutTradeNo:     "BF202605230001",
		TransactionID:  pgtype.Text{String: "BFPAY202605230001", Valid: true},
		Amount:         10000,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	refundOrder := db.RefundOrder{ID: 6101, PaymentOrderID: paymentOrder.ID, RefundAmount: 3500, OutRefundNo: "R202605230001", Status: "pending"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	status, refundID, err := processReplaceOrderRefundWithBaofu(context.Background(), store, facade, 9001, paymentOrder, refundOrder.OutRefundNo, "订单改菜单退款", refundOrder.RefundAmount)

	require.NoError(t, err)
	require.Equal(t, "PROCESSING", status)
	require.Equal(t, "BFRFD202605230001", refundID)
	require.True(t, facade.baofuRefundCalled)
	req := facade.lastBaofuRefund
	require.Equal(t, refundOrder.OutRefundNo, req.OutTradeNo)
	require.Equal(t, paymentOrder.TransactionID.String, req.OriginTradeNo)
	require.Empty(t, req.OriginOutTradeNo)
	require.Equal(t, refundOrder.RefundAmount, req.RefundAmountFen)
	require.Equal(t, refundOrder.RefundAmount, req.TotalAmountFen)
	require.Equal(t, "订单改菜单退款", req.RefundReason)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/refund", req.NotifyURL)
	require.NotEmpty(t, req.TransactionTime)
	_, parseErr := time.Parse("20060102150405", req.TransactionTime)
	require.NoError(t, parseErr)
}

func TestProcessReplaceOrderRefundWithBaofuFallsBackToOriginOutTradeNoWhenTransactionIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{
		baofuRefund: &aggregatecontracts.RefundResult{
			OutTradeNo:      "R202605230002",
			TradeNo:         "BFRFD202605230002",
			RefundAmountFen: 1200,
			TotalAmountFen:  1200,
			ResultCode:      aggregatecontracts.BusinessResultCodeSuccess,
			RefundState:     aggregatecontracts.RefundStateAccepted,
		},
	}
	paymentOrder := db.PaymentOrder{
		ID:             5102,
		OutTradeNo:     "BF202605230002",
		Amount:         5000,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	refundOrder := db.RefundOrder{ID: 6102, PaymentOrderID: paymentOrder.ID, RefundAmount: 1200, OutRefundNo: "R202605230002", Status: "pending"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	_, _, err := processReplaceOrderRefundWithBaofu(context.Background(), store, facade, 9001, paymentOrder, refundOrder.OutRefundNo, "订单改菜单退款", refundOrder.RefundAmount)

	require.NoError(t, err)
	require.True(t, facade.baofuRefundCalled)
	require.Empty(t, facade.lastBaofuRefund.OriginTradeNo)
	require.Equal(t, paymentOrder.OutTradeNo, facade.lastBaofuRefund.OriginOutTradeNo)
}

func TestProcessReplaceOrderRefundWithBaofuRejectsBaofooFailResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{
		baofuRefund: &aggregatecontracts.RefundResult{
			OutTradeNo:      "R202605230004",
			TradeNo:         "BFRFD202605230004",
			RefundAmountFen: 900,
			TotalAmountFen:  900,
			ResultCode:      aggregatecontracts.BusinessResultCodeFail,
			ErrorCode:       "REFUND_AMT_EXCEEDS",
			ErrorMessage:    "raw upstream refund amount detail",
			RefundState:     aggregatecontracts.RefundStateError,
		},
	}
	paymentOrder := db.PaymentOrder{
		ID:             5104,
		OutTradeNo:     "BF202605230004",
		TransactionID:  pgtype.Text{String: "BFPAY202605230004", Valid: true},
		Amount:         2600,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	refundOrder := db.RefundOrder{ID: 6104, PaymentOrderID: paymentOrder.ID, RefundAmount: 900, OutRefundNo: "R202605230004", Status: "pending"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	status, refundID, err := processReplaceOrderRefundWithBaofu(context.Background(), store, facade, 9001, paymentOrder, refundOrder.OutRefundNo, "订单改菜单退款", refundOrder.RefundAmount)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.Equal(t, "退款提交失败，请稍后重试或联系平台处理", reqErr.Err.Error())
	require.Empty(t, status)
	require.Empty(t, refundID)
	require.True(t, facade.baofuRefundCalled)
	errorCode, errorMessage := refundCommandErrorFields(err)
	require.NotNil(t, errorCode)
	require.Equal(t, "REFUND_AMT_EXCEEDS", *errorCode)
	require.NotNil(t, errorMessage)
	require.Equal(t, "资料信息不完整，请核对后重新提交，check_and_resubmit", *errorMessage)
	require.NotContains(t, *errorMessage, "raw upstream")
}

func TestCreateReplaceOrderBaofuPaymentRequiresClientIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{}
	order := db.Order{ID: 7001}

	_, err := createReplaceOrderBaofuPayment(context.Background(), store, facade, 1001, order, 2600, "")

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Equal(t, "支付环境信息缺失，请刷新页面后重试", reqErr.Err.Error())
	require.False(t, facade.createPaymentCalled)
}

func TestCreateReplaceOrderBaofuPaymentPassesClientIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{
		paymentResult: CreatePaymentOrderResult{PaymentOrder: db.PaymentOrder{ID: 8801}},
	}
	order := db.Order{ID: 7002}

	store.EXPECT().GetUser(gomock.Any(), int64(1002)).Return(db.User{ID: 1002, WechatOpenid: "openid-replace"}, nil)

	paymentOrder, err := createReplaceOrderBaofuPayment(context.Background(), store, facade, 1002, order, 2600, "198.51.100.8")

	require.NoError(t, err)
	require.Equal(t, int64(8801), paymentOrder.ID)
	require.True(t, facade.createPaymentCalled)
	require.Equal(t, "198.51.100.8", facade.lastCreatePayment.ClientIP)
	require.Equal(t, order.ID, facade.lastCreatePayment.OrderID)
	require.Equal(t, businessTypeOrder, facade.lastCreatePayment.BusinessType)
	require.Equal(t, int64(2600), facade.lastCreatePayment.Amount)
}

func TestReplaceReservationOrderWithBaofuCreatesRefundOrderThroughGuardedTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{
		baofuRefund: &aggregatecontracts.RefundResult{
			OutTradeNo:      "RF_REPLACE_001",
			TradeNo:         "BFRFD_REPLACE_001",
			RefundAmountFen: 1500,
			TotalAmountFen:  1500,
			ResultCode:      aggregatecontracts.BusinessResultCodeSuccess,
			RefundState:     aggregatecontracts.RefundStateAccepted,
		},
	}
	userID := int64(1001)
	merchantID := int64(2001)
	reservationID := int64(3001)
	oldOrderID := int64(4001)
	newOrderID := int64(4002)
	paymentOrderID := int64(5001)
	dishID := int64(6001)

	oldOrder := db.Order{
		ID:            oldOrderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   5000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     88,
		Status:      "paid",
		PaymentMode: "full",
	}
	paymentOrder := db.PaymentOrder{
		ID:             paymentOrderID,
		OrderID:        pgtype.Int8{Int64: oldOrderID, Valid: true},
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		UserID:         userID,
		PaymentType:    paymentTypeProfitSharing,
		BusinessType:   businessTypeReservation,
		Amount:         5000,
		OutTradeNo:     "BF_REPLACE_PAY_001",
		TransactionID:  pgtype.Text{String: "BF_UP_REPLACE_PAY_001", Valid: true},
		Status:         paymentStatusPaid,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	newOrder := db.Order{
		ID:            newOrderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "dine_in",
		Status:        "paid",
		TotalAmount:   3500,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	refundOrder := db.RefundOrder{
		ID:             7001,
		PaymentOrderID: paymentOrderID,
		RefundAmount:   1500,
		OutRefundNo:    "RF_REPLACE_001",
		Status:         "pending",
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), oldOrderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(db.DiningSession{ID: 700, UserID: userID}, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "审计套餐", Price: 3500, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrderID).Return(int64(0), nil)
	store.EXPECT().ReplaceOrderWithRefundOrdersTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReplaceOrderWithRefundOrdersTxParams) (db.ReplaceOrderWithRefundOrdersTxResult, error) {
		require.Equal(t, oldOrderID, arg.OldOrderID)
		require.Equal(t, int64(3500), arg.CreateOrderParams.TotalAmount)
		require.Equal(t, "paid", arg.CreateOrderParams.Status)
		require.Len(t, arg.RefundOrders, 1)
		require.Equal(t, paymentOrderID, arg.RefundOrders[0].PaymentOrderID)
		require.Equal(t, paymentTypeProfitSharing, arg.RefundOrders[0].RefundType)
		require.Equal(t, int64(1500), arg.RefundOrders[0].RefundAmount)
		require.Equal(t, "订单改菜单退款", arg.RefundOrders[0].RefundReason)
		require.NotEmpty(t, arg.RefundOrders[0].OutRefundNo)
		refundOrder.OutRefundNo = arg.RefundOrders[0].OutRefundNo
		return db.ReplaceOrderWithRefundOrdersTxResult{
			ReplaceOrderTxResult: db.ReplaceOrderTxResult{NewOrder: newOrder, OldOrder: oldOrder},
			RefundOrders:         []db.RefundOrder{refundOrder},
		}, nil
	})
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), gomock.Any()).Return(refundOrder, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), gomock.Any()).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{}, nil)

	result, err := ReplaceReservationOrderWithBaofu(
		context.Background(),
		store,
		facade,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: oldOrderID,
			Items: []OrderItemInput{{
				DishID:   &dishID,
				Quantity: 1,
			}},
			ClientIP: "198.51.100.9",
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return nil, 0, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, newOrderID, result.NewOrder.ID)
	require.Equal(t, int64(-1500), result.Delta)
	require.True(t, result.RefundInitiated)
	require.True(t, facade.baofuRefundCalled)
	require.Equal(t, refundOrder.OutRefundNo, facade.lastBaofuRefund.OutTradeNo)
}

func TestReplaceReservationOrderWithBaofuReturnsSuccessWhenPostCommitRefundSubmissionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	facade := &replaceOrderPaymentFacade{baofuRefundErr: errors.New("temporary baofu outage")}
	userID := int64(1101)
	merchantID := int64(2101)
	reservationID := int64(3101)
	oldOrderID := int64(4101)
	newOrderID := int64(4102)
	paymentOrderID := int64(5101)
	dishID := int64(6101)

	oldOrder := db.Order{
		ID:            oldOrderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   5000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     88,
		Status:      "paid",
		PaymentMode: "full",
	}
	paymentOrder := db.PaymentOrder{
		ID:             paymentOrderID,
		OrderID:        pgtype.Int8{Int64: oldOrderID, Valid: true},
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		UserID:         userID,
		PaymentType:    paymentTypeProfitSharing,
		BusinessType:   businessTypeReservation,
		Amount:         5000,
		OutTradeNo:     "BF_REPLACE_PAY_002",
		TransactionID:  pgtype.Text{String: "BF_UP_REPLACE_PAY_002", Valid: true},
		Status:         paymentStatusPaid,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	newOrder := db.Order{
		ID:            newOrderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "dine_in",
		Status:        "paid",
		TotalAmount:   3500,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	refundOrder := db.RefundOrder{
		ID:             7101,
		PaymentOrderID: paymentOrderID,
		RefundAmount:   1500,
		OutRefundNo:    "RF_REPLACE_002",
		Status:         "pending",
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), oldOrderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(db.DiningSession{ID: 701, UserID: userID}, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "审计套餐", Price: 3500, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrderID).Return(int64(0), nil)
	store.EXPECT().ReplaceOrderWithRefundOrdersTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReplaceOrderWithRefundOrdersTxParams) (db.ReplaceOrderWithRefundOrdersTxResult, error) {
		require.Len(t, arg.RefundOrders, 1)
		refundOrder.OutRefundNo = arg.RefundOrders[0].OutRefundNo
		return db.ReplaceOrderWithRefundOrdersTxResult{
			ReplaceOrderTxResult: db.ReplaceOrderTxResult{NewOrder: newOrder, OldOrder: oldOrder},
			RefundOrders:         []db.RefundOrder{refundOrder},
		}, nil
	})
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), gomock.Any()).Return(refundOrder, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{}, nil)

	result, err := ReplaceReservationOrderWithBaofu(
		context.Background(),
		store,
		facade,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: oldOrderID,
			Items: []OrderItemInput{{
				DishID:   &dishID,
				Quantity: 1,
			}},
			ClientIP: "198.51.100.10",
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return nil, 0, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, newOrderID, result.NewOrder.ID)
	require.Equal(t, int64(-1500), result.Delta)
	require.True(t, result.RefundInitiated)
	require.True(t, facade.baofuRefundCalled)
	require.Equal(t, refundOrder.OutRefundNo, facade.lastBaofuRefund.OutTradeNo)
}

func TestReplaceReservationRefundCommandInputUsesBaofuProvider(t *testing.T) {
	refundOrderID := int64(6103)
	input := dbReplaceReservationRefundCommandInput(
		db.PaymentOrder{ID: 5103, PaymentChannel: db.PaymentChannelBaofuAggregate},
		db.RefundOrder{ID: refundOrderID},
		"R202605230003",
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty("BFRFD202605230003"),
		nil,
		nil,
		[]byte(`{}`),
	)

	require.Equal(t, db.ExternalPaymentProviderBaofu, input.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, input.Channel)
	require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, input.Capability)
	require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, input.BusinessOwner)
	require.NotNil(t, input.BusinessObjectID)
	require.Equal(t, refundOrderID, *input.BusinessObjectID)
}

type replaceOrderPaymentFacade struct {
	createPaymentCalled bool
	lastCreatePayment   CreatePaymentOrderInput
	paymentResult       CreatePaymentOrderResult
	paymentErr          error

	baofuRefundCalled bool
	lastBaofuRefund   aggregatecontracts.RefundBeforeShareRequest
	baofuRefund       *aggregatecontracts.RefundResult
	baofuRefundErr    error
}

func (f *replaceOrderPaymentFacade) CreatePaymentOrder(_ context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	f.createPaymentCalled = true
	f.lastCreatePayment = input
	return f.paymentResult, f.paymentErr
}

func (f *replaceOrderPaymentFacade) CreateReservationAdjustmentPaymentOrder(context.Context, CreateReservationAdjustmentPaymentInput) (CreatePaymentOrderResult, error) {
	return CreatePaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) CreateCombinedPaymentOrder(context.Context, CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	return CreateCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) GetCombinedPaymentOrder(context.Context, GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error) {
	return GetCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) QueryCombinedPaymentOrder(context.Context, QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	return QueryCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) CloseCombinedPaymentOrder(context.Context, CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	return CloseCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) GetPaymentOrder(context.Context, GetPaymentOrderInput) (GetPaymentOrderResult, error) {
	return GetPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) QueryPaymentOrder(context.Context, QueryPaymentOrderInput) (QueryPaymentOrderResult, error) {
	return QueryPaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) ListPaymentOrders(context.Context, ListPaymentOrdersInput) (ListPaymentOrdersResult, error) {
	return ListPaymentOrdersResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) ListPaymentLedger(context.Context, ListPaymentLedgerInput) (ListPaymentLedgerResult, error) {
	return ListPaymentLedgerResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) ClosePaymentOrder(context.Context, ClosePaymentOrderInput) (ClosePaymentOrderResult, error) {
	return ClosePaymentOrderResult{}, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) CreateRefund(context.Context, *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *replaceOrderPaymentFacade) CreateBaofuRefund(_ context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	f.baofuRefundCalled = true
	f.lastBaofuRefund = req
	return f.baofuRefund, f.baofuRefundErr
}

func (f *replaceOrderPaymentFacade) BaofuRefundNotifyURL() string {
	return "https://api.example.com/v1/webhooks/baofu/refund"
}
