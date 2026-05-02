package logic

import (
	"bytes"
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
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeOrdinaryPaymentClient struct {
	createPaymentRequest         *ospcontracts.PaymentPrepayRequest
	createPaymentResponse        *ospcontracts.PaymentPrepayResponse
	createPaymentErr             error
	queryPaymentRequest          *ospcontracts.PaymentQueryRequest
	queryPaymentResponse         *ospcontracts.PaymentQueryResponse
	closePaymentRequest          *ospcontracts.PaymentCloseRequest
	createCombinePaymentRequest  *ospcontracts.CombinePrepayRequest
	createCombinePaymentResponse *ospcontracts.CombinePrepayResponse
	createCombinePaymentErr      error
	queryCombinePaymentRequest   *ospcontracts.CombineQueryRequest
	queryCombinePaymentResponse  *ospcontracts.CombineQueryResponse
	closeCombinePaymentRequest   *ospcontracts.CombineCloseRequest
	closeCombinePaymentErr       error
	createRefundRequest          *ospcontracts.RefundCreateRequest
	createRefundResponse         *ospcontracts.RefundResponse
	createRefundErr              error
	closePaymentErr              error
	createReturnRequest          *ospcontracts.ProfitSharingReturnRequest
	createReturnResponse         *ospcontracts.ProfitSharingReturnResponse
	createReturnErr              error
}

func (c *fakeOrdinaryPaymentClient) ServiceProviderAppID() string { return "wxsp_app" }
func (c *fakeOrdinaryPaymentClient) ServiceProviderMchID() string { return "1900000109" }
func (c *fakeOrdinaryPaymentClient) PaymentNotifyURL() string {
	return "https://api.example.com/v1/webhooks/wechat-ordinary/payment-notify"
}
func (c *fakeOrdinaryPaymentClient) CombineNotifyURL() string {
	return "https://api.example.com/v1/webhooks/wechat-ordinary/combine-notify"
}
func (c *fakeOrdinaryPaymentClient) RefundNotifyURL() string {
	return "https://api.example.com/v1/webhooks/wechat-ordinary/refund-notify"
}

func (c *fakeOrdinaryPaymentClient) CreatePayment(_ context.Context, req ospcontracts.PaymentPrepayRequest) (*ospcontracts.PaymentPrepayResponse, error) {
	c.createPaymentRequest = &req
	if c.createPaymentErr != nil {
		return nil, c.createPaymentErr
	}
	if c.createPaymentResponse != nil {
		return c.createPaymentResponse, nil
	}
	return &ospcontracts.PaymentPrepayResponse{PrepayID: "prepay-ordinary"}, nil
}

func (c *fakeOrdinaryPaymentClient) QueryPayment(_ context.Context, req ospcontracts.PaymentQueryRequest) (*ospcontracts.PaymentQueryResponse, error) {
	c.queryPaymentRequest = &req
	return c.queryPaymentResponse, nil
}

func (c *fakeOrdinaryPaymentClient) ClosePayment(_ context.Context, req ospcontracts.PaymentCloseRequest) error {
	c.closePaymentRequest = &req
	return c.closePaymentErr
}

func (c *fakeOrdinaryPaymentClient) CreateCombinePayment(_ context.Context, req ospcontracts.CombinePrepayRequest) (*ospcontracts.CombinePrepayResponse, error) {
	c.createCombinePaymentRequest = &req
	if c.createCombinePaymentErr != nil {
		return nil, c.createCombinePaymentErr
	}
	if c.createCombinePaymentResponse != nil {
		return c.createCombinePaymentResponse, nil
	}
	return &ospcontracts.CombinePrepayResponse{PrepayID: "prepay-combine-ordinary"}, nil
}

func (c *fakeOrdinaryPaymentClient) QueryCombinePayment(_ context.Context, req ospcontracts.CombineQueryRequest) (*ospcontracts.CombineQueryResponse, error) {
	c.queryCombinePaymentRequest = &req
	return c.queryCombinePaymentResponse, nil
}

func (c *fakeOrdinaryPaymentClient) CloseCombinePayment(_ context.Context, req ospcontracts.CombineCloseRequest) error {
	c.closeCombinePaymentRequest = &req
	return c.closeCombinePaymentErr
}

func (c *fakeOrdinaryPaymentClient) CreateRefund(_ context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error) {
	c.createRefundRequest = &req
	if c.createRefundErr != nil {
		return nil, c.createRefundErr
	}
	if c.createRefundResponse != nil {
		return c.createRefundResponse, nil
	}
	return &ospcontracts.RefundResponse{RefundID: "refund-ordinary", OutRefundNo: req.OutRefundNo}, nil
}

func (c *fakeOrdinaryPaymentClient) CreateProfitSharingReturn(_ context.Context, req ospcontracts.ProfitSharingReturnRequest) (*ospcontracts.ProfitSharingReturnResponse, error) {
	c.createReturnRequest = &req
	if c.createReturnErr != nil {
		return nil, c.createReturnErr
	}
	if c.createReturnResponse != nil {
		return c.createReturnResponse, nil
	}
	return &ospcontracts.ProfitSharingReturnResponse{
		SubMchID:    req.SubMchID,
		OrderID:     req.OrderID,
		OutOrderNo:  req.OutOrderNo,
		OutReturnNo: req.OutReturnNo,
		ReturnID:    "return-ordinary",
		ReturnMchID: req.ReturnMchID,
		Amount:      req.Amount,
		State:       ospcontracts.ProfitSharingReturnStateProcessing,
	}, nil
}

func (c *fakeOrdinaryPaymentClient) GenerateJSAPIPayParams(prepayID string) (*ospcontracts.JSAPIPayParams, error) {
	return &ospcontracts.JSAPIPayParams{Package: "prepay_id=" + prepayID, NonceStr: "nonce-ordinary"}, nil
}

func TestPaymentOrderServiceCreatePaymentOrder_UsesOrdinaryServiceProviderForMainBusiness(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		Amount:         1000,
		OutTradeNo:     "ordinary-out-trade-no",
		Attach:         pgtype.Text{String: "order_id:2001;sub_mchid:sub-ordinary", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
		require.Equal(t, "order_id:2001", arg.Attach)
		return db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-ordinary"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-ordinary", Valid: true},
	}).Return(db.PaymentOrder{ID: txPayment.ID, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, PrepayID: pgtype.Text{String: "prepay-ordinary", Valid: true}}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
		require.Equal(t, txPayment.OutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, "prepay-ordinary", arg.ExternalSecondaryKey.String)
		return db.ExternalPaymentCommand{ID: 9700}, nil
	})

	svc := NewPaymentOrderServiceWithOrdinaryServiceProvider(store, nil, ordinaryClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, result.PaymentOrder.PaymentChannel)
	require.NotNil(t, result.PayParams)
	require.NotNil(t, ordinaryClient.createPaymentRequest)
	require.Equal(t, "wxsp_app", ordinaryClient.createPaymentRequest.SpAppID)
	require.Equal(t, "1900000109", ordinaryClient.createPaymentRequest.SpMchID)
	require.Equal(t, "sub-ordinary", ordinaryClient.createPaymentRequest.SubMchID)
	require.Equal(t, "Merchant A - Order Payment", ordinaryClient.createPaymentRequest.Description)
	require.Equal(t, "ordinary-out-trade-no", ordinaryClient.createPaymentRequest.OutTradeNo)
	require.Equal(t, int64(1000), ordinaryClient.createPaymentRequest.Amount.Total)
	require.Equal(t, "openid", ordinaryClient.createPaymentRequest.Payer.SpOpenID)
	require.Equal(t, "127.0.0.1", ordinaryClient.createPaymentRequest.SceneInfo.PayerClientIP)
	require.Equal(t, "https://api.example.com/v1/webhooks/wechat-ordinary/payment-notify", ordinaryClient.createPaymentRequest.NotifyURL)
	require.False(t, ordinaryClient.createPaymentRequest.SettleInfo.ProfitSharing)
}

func TestPaymentOrderServiceCreatePaymentOrder_LogsOrdinaryMarkFailedCleanupError(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4012,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		Amount:         1000,
		OutTradeNo:     "ordinary-cleanup-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{
		PaymentOrder: txPayment,
		SubMchID:     "sub-ordinary",
	}, nil)
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-ordinary", Valid: true},
	}).Return(db.PaymentOrder{}, errors.New("prepay write failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), txPayment.ID).Return(db.PaymentOrder{}, errors.New("mark failed failed"))

	svc := NewPaymentOrderServiceWithOrdinaryServiceProvider(store, nil, ordinaryClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "update prepay id")
	require.Contains(t, logs.String(), "failed to mark ordinary service provider payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark failed failed")
	require.NotNil(t, ordinaryClient.closePaymentRequest)
}

func TestPaymentOrderServiceClosePaymentOrder_OrdinaryServiceProviderCallsRemoteClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	svc := NewPaymentOrderServiceWithOrdinaryServiceProvider(store, nil, ordinaryClient)
	paymentOrder := db.PaymentOrder{
		ID:             4101,
		UserID:         1001,
		Status:         paymentStatusPending,
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		OutTradeNo:     "ordinary-out-trade-no",
		Attach:         pgtype.Text{String: "order_id:2001;sub_mchid:sub-ordinary", Valid: true},
		PrepayID:       pgtype.Text{String: "prepay-ordinary", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "closed"}, nil)

	result, err := svc.ClosePaymentOrder(context.Background(), ClosePaymentOrderInput{UserID: 1001, PaymentOrderID: paymentOrder.ID})

	require.NoError(t, err)
	require.Equal(t, "closed", result.PaymentOrder.Status)
	require.NotNil(t, ordinaryClient.closePaymentRequest)
	require.Equal(t, "1900000109", ordinaryClient.closePaymentRequest.SpMchID)
	require.Equal(t, "sub-ordinary", ordinaryClient.closePaymentRequest.SubMchID)
	require.Equal(t, "ordinary-out-trade-no", ordinaryClient.closePaymentRequest.OutTradeNo)
}

func TestPaymentOrderServiceQueryPaymentOrder_OrdinaryServiceProviderUsesRemoteQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{
		queryPaymentResponse: &ospcontracts.PaymentQueryResponse{
			SpAppID:        "wxsp_app",
			SpMchID:        "1900000109",
			SubMchID:       "sub-ordinary",
			OutTradeNo:     "ordinary-out-trade-no",
			TradeState:     ospcontracts.PaymentTradeStateNotPay,
			TradeStateDesc: "NOTPAY",
			Amount:         &ospcontracts.PaymentAmount{Total: 1000, Currency: ospcontracts.CurrencyCNY},
		},
	}
	svc := NewPaymentOrderServiceWithOrdinaryServiceProvider(store, nil, ordinaryClient)
	svc.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }
	paymentOrder := db.PaymentOrder{
		ID:             4102,
		UserID:         1001,
		Status:         paymentStatusPending,
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		OutTradeNo:     "ordinary-out-trade-no",
		Attach:         pgtype.Text{String: "order_id:2001;sub_mchid:sub-ordinary", Valid: true},
		PrepayID:       pgtype.Text{String: "prepay-ordinary", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC), Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)

	result, err := svc.QueryPaymentOrder(context.Background(), QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: paymentOrder.ID})

	require.NoError(t, err)
	require.NotNil(t, ordinaryClient.queryPaymentRequest)
	require.Equal(t, "sub-ordinary", ordinaryClient.queryPaymentRequest.SubMchID)
	require.Equal(t, "ordinary-out-trade-no", ordinaryClient.queryPaymentRequest.OutTradeNo)
	require.Equal(t, "ordinary-out-trade-no", result.WechatOrder.OutTradeNo)
	require.Equal(t, "NOTPAY", result.WechatOrder.TradeState)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay-ordinary", result.PayParams.Package)
}

func TestPaymentOrderServiceCreatePaymentOrder_RecreatesPendingOrderWhenAmountChanged(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
		BalancePaid: 300,
	}
	existingPayment := db.PaymentOrder{
		ID:                4001,
		UserID:            input.UserID,
		Status:            paymentStatusPending,
		PaymentType:       "profit_sharing",
		PaymentChannel:    db.PaymentChannelEcommerce,
		Amount:            1000,
		OutTradeNo:        "old-out-trade-no",
		CombinedPaymentID: pgtype.Int8{Int64: 5001, Valid: true},
	}
	newPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         700,
		OutTradeNo:     "new-out-trade-no",
		Attach:         pgtype.Text{String: "order_id:2001;sub_mchid:sub-new", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(1).
		Return(existingPayment, nil)
	store.EXPECT().
		UpdatePaymentOrderToClosed(gomock.Any(), int64(4001)).
		Times(1).
		Return(db.PaymentOrder{ID: 4001, UserID: input.UserID, Status: "closed"}, nil)
	store.EXPECT().
		UpdateCombinedPaymentOrderToClosed(gomock.Any(), int64(5001)).
		Times(1).
		Return(db.CombinedPaymentOrder{ID: 5001, Status: "closed"}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Times(1).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Times(1).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
			require.Equal(t, input.UserID, arg.UserID)
			require.Equal(t, order.MerchantID, arg.MerchantID)
			require.Equal(t, input.OrderID, arg.OrderID)
			require.Equal(t, businessTypeOrder, arg.BusinessType)
			require.Equal(t, int64(700), arg.Amount)
			require.Equal(t, "order_id:2001", arg.Attach)
			return db.CreatePartnerPaymentTxResult{
				PaymentOrder: newPayment,
				SubMchID:     "sub-new",
			}, nil
		})
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.PartnerJSAPIOrderRequest) (*wechatcontracts.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, "sub-new", req.SubMchID)
			require.Equal(t, int64(700), req.TotalAmount)
			require.Equal(t, "Merchant A - Order Payment", req.Description)
			require.Equal(t, "order_id:2001;sub_mchid:sub-new", req.Attach)
			return &wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
			ID:       newPayment.ID,
			PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
		}).
		Times(1).
		Return(db.PaymentOrder{ID: newPayment.ID, Amount: newPayment.Amount, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, newPayment.ID, newPayment.OutTradeNo, "prepay-new", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9701)

	svc := NewPaymentOrderService(store, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(700), result.PaymentOrder.Amount)
	require.True(t, result.PaymentOrder.PrepayID.Valid)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_OrderAlwaysUsesEcommerceWhenBothClientsConfigured(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{
			PaymentOrder: txPayment,
			SubMchID:     "sub-new",
		}, nil)
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Return(&wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil)
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
			ID:       txPayment.ID,
			PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
		}).
		Return(db.PaymentOrder{ID: txPayment.ID, Amount: txPayment.Amount, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, txPayment.ID, txPayment.OutTradeNo, "prepay-new", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9702)

	svc := NewPaymentOrderService(store, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.True(t, result.PaymentOrder.PrepayID.Valid)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_PartnerPrepayUpdateFailureClosesByOutTradeNoFirst(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{
			PaymentOrder: txPayment,
			SubMchID:     "sub-new",
		}, nil)
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Return(&wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil)
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
			ID:       txPayment.ID,
			PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
		}).
		Return(db.PaymentOrder{}, errors.New("write failed"))
	store.EXPECT().
		UpdatePaymentOrderToFailed(gomock.Any(), txPayment.ID).
		Return(db.PaymentOrder{ID: txPayment.ID, Status: "failed"}, nil)
	ecommerceClient.EXPECT().
		ClosePartnerOrder(gomock.Any(), txPayment.OutTradeNo, "sub-new").
		Return(nil)

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update prepay id")
}

func TestPaymentOrderServiceCreatePaymentOrder_WechatOrderClosedReturnsConflict(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{
		PaymentOrder: txPayment,
		SubMchID:     "sub-new",
	}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).Return(nil, nil, &wechat.WechatPayError{StatusCode: 400, Code: "ORDER_CLOSED", Message: "订单已关闭"})
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), txPayment.ID).Return(db.PaymentOrder{ID: txPayment.ID, Status: "closed"}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, txPayment.ID, txPayment.OutTradeNo, "", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusRejected, "ORDER_CLOSED", 9703)

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, "支付订单已过期或已关闭，请重新发起支付", reqErr.Err.Error())
}

func TestPaymentOrderServiceCreatePaymentOrder_WechatOrderClosedSkipsCommandWhenCloseFails(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{
		PaymentOrder: txPayment,
		SubMchID:     "sub-new",
	}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).Return(nil, nil, &wechat.WechatPayError{StatusCode: 400, Code: "ORDER_CLOSED", Message: "订单已关闭"})
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), txPayment.ID).Return(db.PaymentOrder{}, errors.New("close failed"))

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
}

func TestMapReservationEcommerceError_ChangedTargetReturnsConflict(t *testing.T) {
	err := mapReservationEcommerceError(errors.New("reservation 42 payable amount changed"))
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "支付金额或支付模式已变化，请返回订单页重新发起支付", reqErr.Err.Error())
}

func TestPaymentOrderServiceCreatePaymentOrder_ReservationPendingModeMismatchSupersedesExisting(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeReservation,
		ClientIP:     "127.0.0.1",
	}
	reservation := db.TableReservation{
		ID:            input.OrderID,
		UserID:        input.UserID,
		MerchantID:    3001,
		Status:        "pending",
		PaymentMode:   paymentModeDeposit,
		DepositAmount: 1000,
	}
	existingPayment := db.PaymentOrder{
		ID:             4001,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "old-out-trade-no",
		Attach:         pgtype.Text{String: "reservation_id:2001", Valid: true},
	}
	newPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
		Attach:         pgtype.Text{String: buildReservationPaymentAttach(input.OrderID, paymentModeDeposit) + ";sub_mchid:sub-new", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetTableReservation(gomock.Any(), input.OrderID).Return(reservation, nil)
	store.EXPECT().GetLatestPaymentOrderByReservation(gomock.Any(), db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType:  businessTypeReservation,
	}).Return(existingPayment, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), existingPayment.ID).Return(db.PaymentOrder{ID: existingPayment.ID, Status: "closed"}, nil)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), reservation.MerchantID).Return(db.Merchant{ID: reservation.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
		require.Equal(t, buildReservationPaymentAttach(input.OrderID, paymentModeDeposit), arg.Attach)
		require.Equal(t, paymentModeDeposit, arg.PaymentMode)
		return db.CreatePartnerPaymentTxResult{PaymentOrder: newPayment, SubMchID: "sub-new"}, nil
	})
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.PartnerJSAPIOrderRequest) (*wechatcontracts.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.Equal(t, buildReservationPaymentAttach(input.OrderID, paymentModeDeposit)+";sub_mchid:sub-new", req.Attach)
		require.True(t, req.ProfitSharing)
		return &wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       newPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: newPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, newPayment.ID, newPayment.OutTradeNo, "prepay-new", db.ExternalPaymentBusinessOwnerReservation, db.ExternalPaymentCommandStatusAccepted, "", 9704)

	svc := NewPaymentOrderService(store, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(4002), result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_TakeawayUsesPartnerSingleWithoutProfitSharing(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-new"}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.PartnerJSAPIOrderRequest) (*wechatcontracts.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.False(t, req.ProfitSharing)
		require.Equal(t, "Merchant A - Order Payment", req.Description)
		return &wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: txPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, txPayment.ID, txPayment.OutTradeNo, "prepay-new", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9705)

	svc := NewPaymentOrderService(store, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_ReservationLinkedDineInUsesProfitSharingSinglePay(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:            input.OrderID,
		UserID:        input.UserID,
		MerchantID:    3001,
		OrderType:     orderTypeDineIn,
		ReservationID: pgtype.Int8{Int64: 9001, Valid: true},
		Status:        "pending",
		TotalAmount:   1000,
	}
	txPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
		ReservationID:  pgtype.Int8{Int64: 9001, Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-new"}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.PartnerJSAPIOrderRequest) (*wechatcontracts.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.True(t, req.ProfitSharing)
		require.Equal(t, "Merchant A - Order Payment", req.Description)
		return &wechatcontracts.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: txPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)
	expectPartnerJSAPIPaymentCommand(t, store, txPayment.ID, txPayment.OutTradeNo, "prepay-new", db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentCommandStatusAccepted, "", 9706)

	svc := NewPaymentOrderService(store, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_ConcurrentPendingOrderSigningFailureReturnsError(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	pendingPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
		PrepayID:       pgtype.Text{String: "prepay-pending", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(1).
		Return(pendingPayment, nil)
	ecommerceClient.EXPECT().
		GenerateJSAPIPayParams("prepay-pending").
		Times(1).
		Return(nil, errors.New("signing unavailable"))

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sign concurrent pending payment order")
	require.Contains(t, err.Error(), "signing unavailable")
}

func TestPaymentOrderServiceCreatePaymentOrder_ReusedPendingOrderSigningFailureReturnsError(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	pendingPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "existing-out-trade-no",
		PrepayID:       pgtype.Text{String: "prepay-existing", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(pendingPayment, nil)
	ecommerceClient.EXPECT().
		GenerateJSAPIPayParams("prepay-existing").
		Return(nil, errors.New("signing unavailable"))

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sign existing payment order")
	require.Contains(t, err.Error(), "signing unavailable")
}

func TestPaymentOrderServiceCreatePaymentOrder_ConcurrentPendingOrderWithoutPrepayReturnsConflict(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	pendingPayment := db.PaymentOrder{
		ID:             4002,
		UserID:         input.UserID,
		Status:         paymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         1000,
		OutTradeNo:     "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(outTradeNoMaxRetry).
		Return(pendingPayment, nil)

	svc := NewPaymentOrderService(store, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "payment order is still preparing, please retry", reqErr.Err.Error())
}

func TestPaymentOrderServiceGetPaymentOrder(t *testing.T) {
	input := GetPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result GetPaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1}, nil)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, result GetPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, input.PaymentOrderID, result.PaymentOrder.ID)
				require.Equal(t, input.UserID, result.PaymentOrder.UserID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			svc := NewPaymentOrderService(store, nil)
			result, err := svc.GetPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceQueryPaymentOrder(t *testing.T) {
	input := QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}

	testCases := []struct {
		name            string
		useDirectClient bool
		useEcomClient   bool
		buildStubs      func(store *mockdb.MockStore, directClient *mockwechat.MockDirectPaymentClientInterface, ecomClient *mockwechat.MockEcommerceClientInterface)
		check           func(t *testing.T, result QueryPaymentOrderResult, err error)
	}{
		{
			name: "ClientNotConfigured",
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, _ *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, PaymentType: paymentTypeProfitSharing, PaymentChannel: db.PaymentChannelEcommerce}, nil)
			},
			check: func(t *testing.T, _ QueryPaymentOrderResult, err error) {
				require.EqualError(t, err, "ecommerce client: not configured")
			},
		},
		{
			name:          "CombinedPaymentRejected",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, _ *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, PaymentType: paymentTypeProfitSharing, CombinedPaymentID: pgtype.Int8{Int64: 9, Valid: true}}, nil)
			},
			check: func(t *testing.T, _ QueryPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusBadRequest, reqErr.Status)
				require.Equal(t, "合单支付订单请使用合单查询接口", reqErr.Err.Error())
			},
		},
		{
			name: "DirectPaymentClientNotConfigured",
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, _ *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, PaymentType: paymentTypeMiniProgram, PaymentChannel: db.PaymentChannelDirect, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}, nil)
			},
			check: func(t *testing.T, _ QueryPaymentOrderResult, err error) {
				require.EqualError(t, err, "direct payment client: not configured")
			},
		},
		{
			name:            "DirectPaymentRemotePendingExposesPayParams",
			useDirectClient: true,
			buildStubs: func(store *mockdb.MockStore, directClient *mockwechat.MockDirectPaymentClientInterface, _ *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						PaymentType:    paymentTypeMiniProgram,
						PaymentChannel: db.PaymentChannelDirect,
						BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
						Status:         paymentStatusPending,
						OutTradeNo:     "DP20260415000001",
						PrepayID:       pgtype.Text{String: "prepay-direct-123", Valid: true},
						ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
					}, nil)
				directClient.EXPECT().
					QueryOrderByOutTradeNo(gomock.Any(), "DP20260415000001").
					Return(&wechatcontracts.DirectOrderQueryResponse{OutTradeNo: "DP20260415000001", TradeState: "NOTPAY", TradeStateDesc: "待支付", Amount: wechatcontracts.DirectOrderQueryAmount{Total: 1000}}, nil)
				directClient.EXPECT().
					GenerateJSAPIPayParams("prepay-direct-123").
					Return(&wechat.JSAPIPayParams{NonceStr: "direct-nonce"}, nil)
			},
			check: func(t *testing.T, result QueryPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "NOTPAY", result.WechatOrder.TradeState)
				require.NotNil(t, result.PayParams)
			},
		},
		{
			name:            "DirectPaymentRemoteSuccessAppliesFact",
			useDirectClient: true,
			buildStubs: func(store *mockdb.MockStore, directClient *mockwechat.MockDirectPaymentClientInterface, _ *mockwechat.MockEcommerceClientInterface) {
				paymentOrder := db.PaymentOrder{
					ID:             input.PaymentOrderID,
					UserID:         input.UserID,
					PaymentType:    paymentTypeMiniProgram,
					PaymentChannel: db.PaymentChannelDirect,
					BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
					Status:         paymentStatusPending,
					OutTradeNo:     "DP20260415000002",
				}
				paidOrder := paymentOrder
				paidOrder.Status = paymentStatusPaid
				paidOrder.TransactionID = pgtype.Text{String: "wx-direct-transaction-001", Valid: true}
				fact := db.ExternalPaymentFact{
					ID:                 9101,
					Provider:           db.ExternalPaymentProviderWechat,
					Channel:            db.PaymentChannelDirect,
					Capability:         db.ExternalPaymentCapabilityDirectJSAPIPayment,
					FactSource:         db.ExternalPaymentFactSourceQuery,
					ExternalObjectType: db.ExternalPaymentObjectPayment,
					ExternalObjectKey:  paymentOrder.OutTradeNo,
					BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
					BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
					BusinessObjectID:   pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
					UpstreamState:      "SUCCESS",
					TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
					IsTerminal:         true,
					Amount:             pgtype.Int8{Int64: 1000, Valid: true},
					Currency:           "CNY",
					RawResource:        []byte(`{"trade_state":"SUCCESS"}`),
					DedupeKey:          "wechat:query:direct_payment:DP20260415000002:SUCCESS",
				}
				application := db.ExternalPaymentFactApplication{
					ID:                 9201,
					FactID:             fact.ID,
					Consumer:           paymentFactConsumerRiderDepositDomain,
					BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
					BusinessObjectID:   paymentOrder.ID,
					Status:             db.ExternalPaymentFactApplicationStatusPending,
				}

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(paymentOrder, nil)
				directClient.EXPECT().
					QueryOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).
					Return(&wechatcontracts.DirectOrderQueryResponse{
						OutTradeNo:     paymentOrder.OutTradeNo,
						TransactionID:  "wx-direct-transaction-001",
						TradeState:     "SUCCESS",
						TradeStateDesc: "支付成功",
						SuccessTime:    "2026-04-27T12:00:00+08:00",
						Amount:         wechatcontracts.DirectOrderQueryAmount{Total: 1000},
					}, nil)
				store.EXPECT().
					CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
					DoAndReturn(func(_ context.Context, params db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
						require.True(t, params.IsTerminal)
						require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, params.TerminalStatus)
						return fact, nil
					})
				store.EXPECT().
					CreateExternalPaymentFactApplication(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactApplicationParams{})).
					Return(application, nil)
				store.EXPECT().
					ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).
					Return(application, nil)
				store.EXPECT().
					GetExternalPaymentFact(gomock.Any(), fact.ID).
					Return(fact, nil)
				store.EXPECT().
					ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: paymentOrder.ID}).
					Return(db.ProcessPaymentSuccessTxResult{PaymentOrder: paidOrder, Processed: true}, nil)
				store.EXPECT().
					UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).
					Return(fact, nil)
				store.EXPECT().
					MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).
					Return(application, nil)
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Return(paidOrder, nil)
			},
			check: func(t *testing.T, result QueryPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, paymentStatusPaid, result.PaymentOrder.Status)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "SUCCESS", result.WechatOrder.TradeState)
				require.Nil(t, result.PayParams)
			},
		},
		{
			name:          "Success_RemotePendingExposesPayParams",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						PaymentType:    paymentTypeProfitSharing,
						PaymentChannel: db.PaymentChannelEcommerce,
						Status:         paymentStatusPending,
						OutTradeNo:     "OC20260415000001",
						OrderID:        pgtype.Int8{Int64: 301, Valid: true},
						PrepayID:       pgtype.Text{String: "prepay-123", Valid: true},
						ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
					}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(301)).
					Return(db.Order{ID: 301, MerchantID: 77}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(77)).
					Return(db.MerchantPaymentConfig{MerchantID: 77, SubMchID: "1900000109"}, nil)
				client.EXPECT().
					QueryPartnerOrderByOutTradeNo(gomock.Any(), "OC20260415000001", "1900000109").
					Return(&wechatcontracts.PartnerOrderQueryResponse{OutTradeNo: "OC20260415000001", SubMchID: "1900000109", TradeState: "NOTPAY", TradeStateDesc: "待支付"}, nil)
				client.EXPECT().
					GenerateJSAPIPayParams("prepay-123").
					Return(&wechat.JSAPIPayParams{NonceStr: "nonce"}, nil)
			},
			check: func(t *testing.T, result QueryPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "NOTPAY", result.WechatOrder.TradeState)
				require.NotNil(t, result.PayParams)
			},
		},
		{
			name:          "PaidOrderUsesTransactionIDQuery",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						PaymentType:    paymentTypeProfitSharing,
						PaymentChannel: db.PaymentChannelEcommerce,
						Status:         paymentStatusPaid,
						OutTradeNo:     "OC20260415000011",
						TransactionID:  pgtype.Text{String: "wx-transaction-001", Valid: true},
						OrderID:        pgtype.Int8{Int64: 311, Valid: true},
					}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(311)).
					Return(db.Order{ID: 311, MerchantID: 79}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(79)).
					Return(db.MerchantPaymentConfig{MerchantID: 79, SubMchID: "1900000111"}, nil)
				client.EXPECT().
					QueryPartnerOrderByTransactionID(gomock.Any(), "wx-transaction-001", "1900000111").
					Return(&wechatcontracts.PartnerOrderQueryResponse{TransactionID: "wx-transaction-001", OutTradeNo: "OC20260415000011", SubMchID: "1900000111", TradeState: "SUCCESS", TradeStateDesc: "支付成功"}, nil)
			},
			check: func(t *testing.T, result QueryPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "wx-transaction-001", result.WechatOrder.TransactionID)
				require.Equal(t, "SUCCESS", result.WechatOrder.TradeState)
				require.Nil(t, result.PayParams)
			},
		},
		{
			name:          "RemoteQueryMapsWechatError",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, PaymentType: paymentTypeProfitSharing, PaymentChannel: db.PaymentChannelEcommerce, Status: paymentStatusPending, OutTradeNo: "OC20260415000002", OrderID: pgtype.Int8{Int64: 302, Valid: true}}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(302)).
					Return(db.Order{ID: 302, MerchantID: 78}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(78)).
					Return(db.MerchantPaymentConfig{MerchantID: 78, SubMchID: "1900000110"}, nil)
				client.EXPECT().
					QueryPartnerOrderByOutTradeNo(gomock.Any(), "OC20260415000002", "1900000110").
					Return(nil, &wechat.WechatPayError{StatusCode: 404, Code: "ORDER_NOT_EXIST", Message: "订单不存在"})
			},
			check: func(t *testing.T, _ QueryPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusBadGateway, reqErr.Status)
				require.Equal(t, "微信侧暂未确认该支付单，请保留当前订单并稍后刷新结果", reqErr.Err.Error())
			},
		},
		{
			name:          "RemoteQueryContractDriftReturnsClearError",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockDirectPaymentClientInterface, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, PaymentType: paymentTypeProfitSharing, PaymentChannel: db.PaymentChannelEcommerce, Status: paymentStatusPending, OutTradeNo: "OC20260415000003", OrderID: pgtype.Int8{Int64: 303, Valid: true}}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(303)).
					Return(db.Order{ID: 303, MerchantID: 79}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(79)).
					Return(db.MerchantPaymentConfig{MerchantID: 79, SubMchID: "1900000112"}, nil)
				client.EXPECT().
					QueryPartnerOrderByOutTradeNo(gomock.Any(), "OC20260415000003", "1900000112").
					Return(nil, &wechat.PartnerOrderQueryContractError{Message: "query partner order by out_trade_no: wechat response missing trade_state"})
			},
			check: func(t *testing.T, _ QueryPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusBadGateway, reqErr.Status)
				require.Equal(t, "微信支付状态返回异常，请不要重复支付，返回订单页后重新查询", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store, directClient, ecommerceClient)
			}

			var directInterface wechat.DirectPaymentClientInterface
			if tc.useDirectClient {
				directInterface = directClient
			}
			var ecommerceInterface wechat.EcommerceClientInterface
			if tc.useEcomClient {
				ecommerceInterface = ecommerceClient
			}

			svc := NewPaymentOrderServiceWithClients(store, directInterface, ecommerceInterface)
			result, err := svc.QueryPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceListPaymentOrders(t *testing.T) {
	baseInput := ListPaymentOrdersInput{UserID: 1002, PageID: 1, PageSize: 10}

	testCases := []struct {
		name       string
		input      ListPaymentOrdersInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ListPaymentOrdersResult, err error)
	}{
		{
			name: "ByOrderID_NotFound_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3001)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3001, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_OtherUser_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3002)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3002, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4001, UserID: baseInput.UserID + 10}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_Success",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3003)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3003, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4002, UserID: baseInput.UserID}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 1)
				require.Equal(t, int64(4002), result.PaymentOrders[0].ID)
				require.Equal(t, int64(1), result.TotalCount)
			},
		},
		{
			name:  "Paged_Success",
			input: ListPaymentOrdersInput{UserID: baseInput.UserID, PageID: 2, PageSize: 5},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: baseInput.UserID,
						Limit:  5,
						Offset: 5,
					}).
					Times(1).
					Return([]db.PaymentOrder{{ID: 5001}, {ID: 5002}}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 2)
				require.Equal(t, int64(2), result.TotalCount)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			svc := NewPaymentOrderService(store, nil)
			result, err := svc.ListPaymentOrders(context.Background(), tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceClosePaymentOrder(t *testing.T) {
	input := ClosePaymentOrderInput{UserID: 1003, PaymentOrderID: 2003}

	testCases := []struct {
		name             string
		buildStubs       func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface)
		buildEcomStubs   func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface)
		usePaymentClient bool
		useEcomClient    bool
		check            func(t *testing.T, result ClosePaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "paid"}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only pending payment orders can be closed", reqErr.Err.Error())
			},
		},
		{
			name: "Success_WithoutClient",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, OutTradeNo: "P202001010000000001", PrepayID: pgtype.Text{Valid: true, String: "prepay"}}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name: "Success_StaleNonEcommercePayment_ClosesLocally",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, OutTradeNo: "P202001010000000002", PrepayID: pgtype.Text{Valid: true, String: "prepay"}}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "Success_CombinedPayment_CloseCombineOrderCalled",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:                input.PaymentOrderID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						PaymentType:       "profit_sharing",
						PaymentChannel:    db.PaymentChannelEcommerce,
						OutTradeNo:        "CP202001010000000003",
						CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrder(gomock.Any(), int64(9001)).
					Times(1).
					Return(db.CombinedPaymentOrder{ID: 9001, CombineOutTradeNo: "OC123", Status: paymentStatusPending}, nil)
				store.EXPECT().
					ListCombinedPaymentSubOrders(gomock.Any(), int64(9001)).
					Times(1).
					Return([]db.CombinedPaymentSubOrder{{SubMchid: "1900000109", OutTradeNo: "CP202001010000000003"}}, nil)
				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "OC123", []wechatcontracts.SubOrderClose{{SubMchID: "1900000109", OutTradeNo: "CP202001010000000003"}}).
					Times(1).
					Return(nil)
				store.EXPECT().
					CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
						CombinedPaymentOrderID: 9001,
						SubOrderOutTradeNos:    []string{"CP202001010000000003"},
					}).
					Times(1).
					Return(db.CloseCombinedPaymentOrderTxResult{}, nil)
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "Success_PartnerSingle_ClosePartnerOrderCalledWithOutTradeNoFirst",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						Status:         paymentStatusPending,
						PaymentType:    "profit_sharing",
						PaymentChannel: db.PaymentChannelEcommerce,
						OutTradeNo:     "RS202001010000000003",
						OrderID:        pgtype.Int8{Int64: 7001, Valid: true},
						PrepayID:       pgtype.Text{String: "prepay", Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7001)).
					Times(1).
					Return(db.Order{ID: 7001, MerchantID: 8801}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(8801)).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 8801, SubMchID: "1900000109"}, nil)
				client.EXPECT().
					ClosePartnerOrder(gomock.Any(), "RS202001010000000003", "1900000109").
					Times(1).
					Return(nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "PartnerSingle_CloseWithoutPrepayIDStillCallsPartnerClose",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						Status:         paymentStatusPending,
						PaymentType:    "profit_sharing",
						PaymentChannel: db.PaymentChannelEcommerce,
						OutTradeNo:     "RS202001010000000031",
						OrderID:        pgtype.Int8{Int64: 7031, Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7031)).
					Times(1).
					Return(db.Order{ID: 7031, MerchantID: 8831}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(8831)).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 8831, SubMchID: "1900000131"}, nil)
				client.EXPECT().
					ClosePartnerOrder(gomock.Any(), "RS202001010000000031", "1900000131").
					Times(1).
					Return(nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "PartnerSingle_ResolveSubMchFailureReturnsError",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:             input.PaymentOrderID,
						UserID:         input.UserID,
						Status:         paymentStatusPending,
						PaymentType:    "profit_sharing",
						PaymentChannel: db.PaymentChannelEcommerce,
						OutTradeNo:     "RS202001010000000004",
						OrderID:        pgtype.Int8{Int64: 7002, Valid: true},
						PrepayID:       pgtype.Text{String: "prepay", Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7002)).
					Times(1).
					Return(db.Order{}, errors.New("resolve failed"))
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "resolve payment order sub_mchid")
			},
		},
		{
			name:          "PartnerSingle_RemoteCloseFailureReturnsMappedError",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, OutTradeNo: "RS202001010000000005", OrderID: pgtype.Int8{Int64: 7003, Valid: true}, PrepayID: pgtype.Text{String: "prepay", Valid: true}}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7003)).
					Return(db.Order{ID: 7003, MerchantID: 8803}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(8803)).
					Return(db.MerchantPaymentConfig{MerchantID: 8803, SubMchID: "1900000111"}, nil)
				client.EXPECT().
					ClosePartnerOrder(gomock.Any(), "RS202001010000000005", "1900000111").
					Return(&wechat.WechatPayError{StatusCode: 202, Code: "USERPAYING", Message: "用户支付中"})
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusConflict, reqErr.Status)
				require.Equal(t, "支付处理中，请先刷新支付结果确认后再决定是否关闭", reqErr.Err.Error())
			},
		},
		{
			name:          "PartnerSingle_RemoteAlreadyClosedStillClosesLocalState",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, OutTradeNo: "RS202001010000000006", OrderID: pgtype.Int8{Int64: 7004, Valid: true}, PrepayID: pgtype.Text{String: "prepay", Valid: true}}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7004)).
					Return(db.Order{ID: 7004, MerchantID: 8804}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(8804)).
					Return(db.MerchantPaymentConfig{MerchantID: 8804, SubMchID: "1900000112"}, nil)
				client.EXPECT().
					ClosePartnerOrder(gomock.Any(), "RS202001010000000006", "1900000112").
					Return(&wechat.WechatPayError{StatusCode: 400, Code: "ORDER_CLOSED", Message: "订单已关闭"})
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)
			if tc.buildEcomStubs != nil {
				tc.buildEcomStubs(store, ecommerceClient)
			}

			var ecommerceInterface wechat.EcommerceClientInterface
			if tc.useEcomClient {
				ecommerceInterface = ecommerceClient
			}

			svc := NewPaymentOrderService(store, ecommerceInterface)
			result, err := svc.ClosePaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func expectPartnerJSAPIPaymentCommand(t *testing.T, store *mockdb.MockStore, paymentOrderID int64, outTradeNo string, secondaryKey string, businessOwner string, status string, errorCode string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
		require.Equal(t, businessOwner, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, paymentOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
		require.Equal(t, outTradeNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outTradeNo)
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
