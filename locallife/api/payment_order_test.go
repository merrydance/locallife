package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== Helper Functions ====================

func randomPaymentOrder(userID int64, orderID *int64) db.PaymentOrder {
	p := db.PaymentOrder{
		ID:           util.RandomInt(1, 1000),
		UserID:       userID,
		PaymentType:  "miniprogram",
		BusinessType: "order",
		Amount:       util.RandomMoney(),
		OutTradeNo:   "P" + util.RandomString(20),
		Status:       "pending",
		CreatedAt:    time.Now(),
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	}
	if orderID != nil {
		p.OrderID = pgtype.Int8{Int64: *orderID, Valid: true}
	}
	return p
}

func randomRefundOrder(paymentOrderID int64, amount int64) db.RefundOrder {
	return db.RefundOrder{
		ID:             util.RandomInt(1, 1000),
		PaymentOrderID: paymentOrderID,
		RefundType:     "full",
		RefundAmount:   amount,
		RefundReason:   pgtype.Text{String: "用户申请退款", Valid: true},
		OutRefundNo:    "R" + util.RandomString(20),
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
}

func TestWriteLogicRequestErrorSanitizesLegacyEnglishMessages(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/payments", nil)

	handled := writeLogicRequestError(ctx, logic.NewRequestError(http.StatusNotFound, errors.New("payment order not found")))

	require.True(t, handled)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "未找到支付单，请刷新页面后重试", resp.Error)
	require.NotContains(t, resp.Error, "payment")
	require.NotContains(t, resp.Error, "not found")
}

func TestWriteLogicRequestErrorKeepsChineseActionMessages(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/payments", nil)

	handled := writeLogicRequestError(ctx, logic.NewRequestError(http.StatusConflict, errors.New("支付单仍在准备中，请稍后重试")))

	require.True(t, handled)
	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "支付单仍在准备中，请稍后重试", resp.Error)
}

func TestNewPaymentOrderWechatQueryResultSupportsDirectPayment(t *testing.T) {
	query := &logic.QueryPaymentOrderWechatOrder{
		AppID:          "wx-app",
		MchID:          "direct-mch",
		OutTradeNo:     "DP202604270001",
		TransactionID:  "wx-direct-transaction",
		TradeType:      "JSAPI",
		TradeState:     "SUCCESS",
		TradeStateDesc: "支付成功",
		Payer:          logic.QueryPaymentOrderWechatPayer{OpenID: "direct-openid"},
		Amount:         logic.QueryPaymentOrderWechatAmount{Total: 1000, PayerTotal: 1000, Currency: "CNY", PayerCurrency: "CNY"},
	}

	resp := newPaymentOrderWechatQueryResult(query)

	require.NotNil(t, resp)
	require.Equal(t, "wx-app", resp.AppID)
	require.Equal(t, "direct-mch", resp.MchID)
	require.Equal(t, "SUCCESS", resp.TradeState)
	require.NotNil(t, resp.Payer)
	require.Equal(t, "direct-openid", resp.Payer.OpenID)
	require.NotNil(t, resp.Amount)
	require.Equal(t, int64(1000), resp.Amount.Total)
}

func TestCreatePaymentOrderAPI_InvalidPayloadReturnsChineseGuidance(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/payments", bytes.NewReader([]byte(`{"order_id":0,"payment_type":"native","business_type":"invalid"}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "支付请求参数格式无效，请选择订单并确认支付类型后重试", resp.Message)
	require.NotContains(t, resp.Message, "PaymentType")
	require.NotContains(t, resp.Message, "binding")
	require.NotContains(t, resp.Message, "provider")
}

// randomPaymentTestOrder creates a random order for payment testing
// Named differently to avoid conflict with randomOrder in order_test.go
func randomPaymentTestOrder(userID, merchantID int64) db.Order {
	return db.Order{
		ID:          util.RandomInt(1, 1000),
		OrderNo:     util.RandomString(20),
		UserID:      userID,
		MerchantID:  merchantID,
		OrderType:   "takeaway",
		Subtotal:    util.RandomMoney(),
		TotalAmount: util.RandomMoney(),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
}

func newTestServerWithOrdinaryServiceProvider(t *testing.T, store db.Store, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) *Server {
	t.Helper()
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)
	return server
}

type stubRefundOrchestrator struct {
	createRefundOrderFunc func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error)
}

func (s stubRefundOrchestrator) CreateRefundOrder(ctx context.Context, input logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
	if s.createRefundOrderFunc != nil {
		return s.createRefundOrderFunc(ctx, input)
	}
	return logic.CreateRefundOrderResult{}, nil
}

func (s stubRefundOrchestrator) GetRefundOrder(context.Context, logic.GetRefundOrderInput) (logic.GetRefundOrderResult, error) {
	return logic.GetRefundOrderResult{}, nil
}

func (s stubRefundOrchestrator) ListRefundOrdersByPayment(context.Context, logic.ListRefundOrdersByPaymentInput) (logic.ListRefundOrdersByPaymentResult, error) {
	return logic.ListRefundOrdersByPaymentResult{}, nil
}

func (s stubRefundOrchestrator) ListProfitSharingReturnsByRefund(context.Context, logic.ListProfitSharingReturnsByRefundInput) (logic.ListProfitSharingReturnsByRefundResult, error) {
	return logic.ListProfitSharingReturnsByRefundResult{}, nil
}

func (s stubRefundOrchestrator) ApplyAbnormalRefund(context.Context, logic.ApplyAbnormalRefundInput) (logic.ApplyAbnormalRefundResult, error) {
	return logic.ApplyAbnormalRefundResult{}, nil
}

func expectOrderPaymentCommandAccepted(t *testing.T, store *mockdb.MockStore, paymentOrder db.PaymentOrder, prepayID string) {
	t.Helper()

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, paymentOrder.PaymentChannel, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
			require.True(t, arg.BusinessObjectType.Valid)
			require.Equal(t, "payment_order", arg.BusinessObjectType.String)
			require.True(t, arg.BusinessObjectID.Valid)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, prepayID, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.Contains(t, string(arg.ResponseSnapshot), paymentOrder.OutTradeNo)
			require.Contains(t, string(arg.ResponseSnapshot), prepayID)
			require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
			return db.ExternalPaymentCommand{ID: util.RandomInt(1, 1000), CommandStatus: arg.CommandStatus}, nil
		})
}

func expectActiveMerchantBaofuAccountForPaymentAPI(store *mockdb.MockStore, merchantID int64, subMchID string) {
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchantID,
		}).
		Times(1).
		Return(db.BaofuAccountBinding{
			OwnerType:      db.BaofuAccountOwnerTypeMerchant,
			OwnerID:        merchantID,
			AccountType:    db.BaofuAccountTypeBusiness,
			OpenState:      db.BaofuAccountOpenStateActive,
			ContractNo:     pgtype.Text{String: "CMAPI", Valid: true},
			SharingMerID:   pgtype.Text{String: "CMAPI", Valid: true},
			WechatSubMchID: pgtype.Text{String: subMchID, Valid: true},
		}, nil)
}

// ==================== CreatePaymentOrder Tests ====================

func TestCreatePaymentOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Amount = order.TotalAmount
	paymentOrder.PaymentType = PaymentTypeProfitShare
	paymentOrder.PaymentChannel = db.PaymentChannelOrdinaryServiceProvider
	paymentOrder.CombinedPaymentID = pgtype.Int8{Int64: util.RandomInt(1, 1000), Valid: true}
	payParams := &ospcontracts.JSAPIPayParams{
		TimeStamp: "1234567890",
		NonceStr:  "nonce",
		Package:   "prepay_id=wx123",
		SignType:  "RSA",
		PaySign:   "signature",
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_MissingPaymentType_UsesCompatibleDefault",
			body: gin.H{
				"order_id":      order.ID,
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
				expectActiveMerchantBaofuAccountForPaymentAPI(store, merchant.ID, "1900000109")

				store.EXPECT().
					GetUser(gomock.Any(), user.ID).
					Times(1).
					Return(user, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreatePartnerPaymentTxResult{
						PaymentOrder: paymentOrder,
						SubMchID:     "1900000109",
					}, nil)

				ordinaryClient.EXPECT().
					ServiceProviderAppID().
					AnyTimes().
					Return("wx-service-app")
				ordinaryClient.EXPECT().
					ServiceProviderMchID().
					AnyTimes().
					Return("1900000001")
				ordinaryClient.EXPECT().
					PaymentNotifyURL().
					AnyTimes().
					Return("https://api.example.com/v1/webhooks/wechat-ordinary/payment-notify")
				ordinaryClient.EXPECT().
					CreatePayment(gomock.Any(), gomock.AssignableToTypeOf(ospcontracts.PaymentPrepayRequest{})).
					Times(1).
					DoAndReturn(func(_ context.Context, req ospcontracts.PaymentPrepayRequest) (*ospcontracts.PaymentPrepayResponse, error) {
						require.Equal(t, "wx-service-app", req.SpAppID)
						require.Equal(t, "1900000001", req.SpMchID)
						require.Equal(t, "1900000109", req.SubMchID)
						require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
						require.Equal(t, paymentOrder.Amount, req.Amount.Total)
						require.NotNil(t, req.SettleInfo)
						require.False(t, req.SettleInfo.ProfitSharing)
						return &ospcontracts.PaymentPrepayResponse{PrepayID: "wx123"}, nil
					})
				ordinaryClient.EXPECT().
					GenerateJSAPIPayParams("wx123").
					Times(1).
					Return(payParams, nil)

				store.EXPECT().
					UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
						updated := paymentOrder
						updated.PrepayID = arg.PrepayID
						return updated, nil
					})

				expectOrderPaymentCommandAccepted(t, store, paymentOrder, "wx123")
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response paymentOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, paymentOrder.ID, response.ID)
				require.Equal(t, "pending", response.Status)
			},
		},
		{
			name: "MerchantBaofuAccountMissing",
			body: gin.H{
				"order_id":      order.ID,
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
						OwnerType: db.BaofuAccountOwnerTypeMerchant,
						OwnerID:   merchant.ID,
					}).
					Times(1).
					Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
				store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Times(0)
				ordinaryClient.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "商户结算账户未开通，暂不能创建支付订单", resp.Message)
				require.NotContains(t, resp.Message, "contract")
				require.NotContains(t, resp.Message, "sharing")
				require.NotContains(t, resp.Message, "provider")
			},
		},
		{
			name: "OrderNotFound",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "OrderNotBelongToUser",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "OrderNotPending",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ExistingPendingPayment_Idempotent",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				existingPendingPayment := paymentOrder
				existingPendingPayment.PrepayID = pgtype.Text{String: "wx123", Valid: true}

				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				// 已存在待支付订单
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingPendingPayment, nil)

				ordinaryClient.EXPECT().
					GenerateJSAPIPayParams(gomock.Any()).
					Times(1).
					Return(payParams, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response paymentOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, paymentOrder.ID, response.ID)
			},
		},
		{
			name: "ConcurrentPendingPaymentConflict_ReusesExistingPayment",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				existingPendingPayment := paymentOrder
				existingPendingPayment.PrepayID = pgtype.Text{String: "wx123", Valid: true}

				gomock.InOrder(
					store.EXPECT().
						GetOrder(gomock.Any(), order.ID).
						Times(1).
						Return(order, nil),
					store.EXPECT().
						GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
						Times(1).
						Return(db.PaymentOrder{}, db.ErrRecordNotFound),
					store.EXPECT().
						GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
							OwnerType: db.BaofuAccountOwnerTypeMerchant,
							OwnerID:   merchant.ID,
						}).
						Times(1).
						Return(db.BaofuAccountBinding{
							OwnerType:      db.BaofuAccountOwnerTypeMerchant,
							OwnerID:        merchant.ID,
							AccountType:    db.BaofuAccountTypeBusiness,
							OpenState:      db.BaofuAccountOpenStateActive,
							ContractNo:     pgtype.Text{String: "CMAPI", Valid: true},
							SharingMerID:   pgtype.Text{String: "CMAPI", Valid: true},
							WechatSubMchID: pgtype.Text{String: "1900000109", Valid: true},
						}, nil),
					store.EXPECT().
						GetMerchant(gomock.Any(), merchant.ID).
						Times(1).
						Return(merchant, nil),
					store.EXPECT().
						GetUser(gomock.Any(), user.ID).
						Times(1).
						Return(user, nil),
					store.EXPECT().
						CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
						Times(1).
						Return(db.CreatePartnerPaymentTxResult{}, db.ErrOrderPendingPaymentConflict),
					store.EXPECT().
						GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
						Times(1).
						Return(existingPendingPayment, nil),
					ordinaryClient.EXPECT().
						GenerateJSAPIPayParams("wx123").
						Times(1).
						Return(payParams, nil),
				)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response paymentOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, paymentOrder.ID, response.ID)
				require.Equal(t, "pending", response.Status)
				require.NotNil(t, response.PayParams)
			},
		},
		{
			name: "InvalidPaymentType",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "invalid",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "LegacyNativePaymentType_Rejected",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "native",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidBusinessType",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "invalid",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidOrderID_Zero",
			body: gin.H{
				"order_id":      0,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore, _ *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
			tc.buildStubs(store, ordinaryClient)

			server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
			server.SetTaskDistributorForTest(nil)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/payments"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== GetPaymentOrder Tests ====================

func TestGetPaymentOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)

	testCases := []struct {
		name          string
		paymentID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response paymentOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, paymentOrder.ID, response.ID)
			},
		},
		{
			name:      "NotFound",
			paymentID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "Forbidden_NotOwner",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			paymentID:  paymentOrder.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/payments/%d", tc.paymentID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ListPaymentOrders Tests ====================

func TestListPaymentOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: user.ID,
						Limit:  10,
						Offset: 0,
					}).
					Times(1).
					Return([]db.PaymentOrder{paymentOrder}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listPaymentOrdersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.PaymentOrders, 1)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: user.ID,
						Limit:  10,
						Offset: 0,
					}).
					Times(1).
					Return([]db.PaymentOrder{paymentOrder}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listPaymentOrdersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.PaymentOrders, 1)
			},
		},
		{
			name:  "InvalidPageSize_TooSmall",
			query: "page_id=1&page_size=4",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: user.ID,
						Limit:  4,
						Offset: 0,
					}).
					Times(1).
					Return([]db.PaymentOrder{paymentOrder}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listPaymentOrdersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.PaymentOrders, 1)
			},
		},
		{
			name:  "InvalidPageSize_TooLarge",
			query: "page_id=1&page_size=21",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			query:      "page_id=1&page_size=10",
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/payments?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ClosePaymentOrder Tests ====================

func TestClosePaymentOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)

	testCases := []struct {
		name          string
		paymentID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				closedPayment := paymentOrder
				closedPayment.Status = "closed"
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(closedPayment, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response paymentOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "closed", response.Status)
			},
		},
		{
			name:      "NotFound",
			paymentID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "Forbidden_NotOwner",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "AlreadyPaid_CannotClose",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidPayment := paymentOrder
				paidPayment.Status = "paid"
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paidPayment, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			paymentID:  paymentOrder.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/payments/%d/close", tc.paymentID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreatePaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(11)).
		Return(db.Order{ID: 11, UserID: 1001, MerchantID: 22, Status: "pending", TotalAmount: 5000}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(gin.H{"order_id": int64(11), "business_type": "order"})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/payments", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "商户支付能力未完成配置，请联系平台处理", resp.Message)
}

func TestQueryPaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:             123,
			UserID:         1001,
			PaymentType:    PaymentTypeProfitShare,
			PaymentChannel: db.PaymentChannelEcommerce,
			BusinessType:   BusinessTypeOrder,
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "商户支付能力未完成配置，当前无法确认支付状态，请联系平台处理", resp.Message)
}

func TestQueryPaymentOrderAPI_RemotePendingReturnsWechatQueryAndPayParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:             123,
			UserID:         1001,
			PaymentType:    PaymentTypeProfitShare,
			PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
			BusinessType:   BusinessTypeOrder,
			Amount:         5000,
			OutTradeNo:     "OC20260415000001",
			Status:         PaymentStatusPending,
			PrepayID:       pgtype.Text{String: "prepay-123", Valid: true},
			OrderID:        pgtype.Int8{Int64: 77, Valid: true},
			ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
		}, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(77)).
		Return(db.Order{ID: 77, MerchantID: 88}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(88)).
		Return(db.MerchantPaymentConfig{MerchantID: 88, SubMchID: "1900000109"}, nil)
	ordinaryClient.EXPECT().
		QueryPayment(gomock.Any(), ospcontracts.PaymentQueryRequest{SubMchID: "1900000109", OutTradeNo: "OC20260415000001"}).
		Return(&ospcontracts.PaymentQueryResponse{
			SpAppID:        "wx-service-app",
			SpMchID:        "1900000001",
			SubMchID:       "1900000109",
			OutTradeNo:     "OC20260415000001",
			TradeState:     ospcontracts.PaymentTradeStateNotPay,
			TradeStateDesc: "待支付",
			Amount:         &ospcontracts.PaymentAmount{Total: 5000, Currency: ospcontracts.CurrencyCNY},
		}, nil)
	ordinaryClient.EXPECT().
		GenerateJSAPIPayParams("prepay-123").
		Return(&ospcontracts.JSAPIPayParams{TimeStamp: "1", NonceStr: "nonce", Package: "prepay_id=prepay-123", SignType: "RSA", PaySign: "sign"}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response paymentOrderQueryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.WechatQuery)
	require.Equal(t, "NOTPAY", response.WechatQuery.TradeState)
	require.NotNil(t, response.PayParams)
	require.Equal(t, "wx-service-app", response.WechatQuery.SpAppID)
}

func TestQueryPaymentOrderAPI_UsesTransactionIDQueryWhenAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:             123,
			UserID:         1001,
			PaymentType:    PaymentTypeProfitShare,
			PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
			BusinessType:   BusinessTypeOrder,
			Amount:         5000,
			OutTradeNo:     "OC20260415000011",
			TransactionID:  pgtype.Text{String: "wx-transaction-001", Valid: true},
			Status:         PaymentStatusPaid,
			OrderID:        pgtype.Int8{Int64: 77, Valid: true},
		}, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(77)).
		Return(db.Order{ID: 77, MerchantID: 88}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(88)).
		Return(db.MerchantPaymentConfig{MerchantID: 88, SubMchID: "1900000109"}, nil)
	ordinaryClient.EXPECT().
		QueryPayment(gomock.Any(), ospcontracts.PaymentQueryRequest{SubMchID: "1900000109", TransactionID: "wx-transaction-001", OutTradeNo: "OC20260415000011"}).
		Return(&ospcontracts.PaymentQueryResponse{
			SpAppID:        "wx-service-app",
			SpMchID:        "1900000001",
			SubMchID:       "1900000109",
			OutTradeNo:     "OC20260415000011",
			TransactionID:  "wx-transaction-001",
			TradeType:      "JSAPI",
			TradeState:     ospcontracts.PaymentTradeStateSuccess,
			TradeStateDesc: "支付成功",
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response paymentOrderQueryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.WechatQuery)
	require.Equal(t, "wx-transaction-001", response.WechatQuery.TransactionID)
	require.Equal(t, "SUCCESS", response.WechatQuery.TradeState)
	require.Nil(t, response.PayParams)
}

func TestQueryPaymentOrderAPI_OutTradeNoQueryPreservesWechatFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:             123,
			UserID:         1001,
			PaymentType:    PaymentTypeProfitShare,
			PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
			BusinessType:   BusinessTypeOrder,
			Amount:         5000,
			OutTradeNo:     "OC20260415000021",
			Status:         PaymentStatusPending,
			OrderID:        pgtype.Int8{Int64: 77, Valid: true},
		}, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(77)).
		Return(db.Order{ID: 77, MerchantID: 88}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(88)).
		Return(db.MerchantPaymentConfig{MerchantID: 88, SubMchID: "1900000109"}, nil)
	ordinaryClient.EXPECT().
		QueryPayment(gomock.Any(), ospcontracts.PaymentQueryRequest{SubMchID: "1900000109", OutTradeNo: "OC20260415000021"}).
		Return(&ospcontracts.PaymentQueryResponse{
			SpAppID:        "wx-service-app",
			SpMchID:        "1900000001",
			SubAppID:       "wx-sub-app",
			SubMchID:       "1900000109",
			OutTradeNo:     "OC20260415000021",
			TransactionID:  "wx-transaction-021",
			TradeType:      "JSAPI",
			TradeState:     ospcontracts.PaymentTradeStateSuccess,
			TradeStateDesc: "支付成功",
			BankType:       "OTHERS",
			Attach:         "order=77",
			SuccessTime:    "2026-04-15T12:00:00+08:00",
			Payer:          ospcontracts.PaymentPayer{SpOpenID: "openid-1"},
			Amount: &ospcontracts.PaymentAmount{
				Total:         5000,
				PayerTotal:    4800,
				Currency:      ospcontracts.CurrencyCNY,
				PayerCurrency: ospcontracts.CurrencyCNY,
			},
			SceneInfo: &ospcontracts.PaymentSceneInfo{DeviceID: "device-77"},
			PromotionDetail: []ospcontracts.PaymentPromotionDetail{{
				CouponID:           "coupon-1",
				Name:               "满减券",
				Scope:              "GLOBAL",
				Type:               "CASH",
				Amount:             200,
				StockID:            "stock-1",
				MerchantContribute: 200,
				Currency:           "CNY",
				GoodsDetail:        []ospcontracts.PaymentPromotionGoodsDetail{{GoodsID: "dish-1", Quantity: 1, UnitPrice: 5000, DiscountAmount: 200, GoodsRemark: "招牌菜"}},
			}},
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response paymentOrderQueryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.WechatQuery)
	require.Equal(t, "OC20260415000021", response.WechatQuery.OutTradeNo)
	require.Equal(t, "wx-transaction-021", response.WechatQuery.TransactionID)
	require.Equal(t, "SUCCESS", response.WechatQuery.TradeState)
	require.NotNil(t, response.WechatQuery.SceneInfo)
	require.Equal(t, "device-77", response.WechatQuery.SceneInfo.DeviceID)
	require.Len(t, response.WechatQuery.PromotionDetail, 1)
	require.Equal(t, "coupon-1", response.WechatQuery.PromotionDetail[0].CouponID)
	require.Len(t, response.WechatQuery.PromotionDetail[0].GoodsDetail, 1)
	require.Equal(t, "dish-1", response.WechatQuery.PromotionDetail[0].GoodsDetail[0].GoodsID)
	require.Nil(t, response.PayParams)
}

func TestQueryPaymentOrderAPI_ContractDriftReturnsClearError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:             123,
			UserID:         1001,
			PaymentType:    PaymentTypeProfitShare,
			PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
			BusinessType:   BusinessTypeOrder,
			Amount:         5000,
			OutTradeNo:     "OC20260415000031",
			Status:         PaymentStatusPending,
			OrderID:        pgtype.Int8{Int64: 77, Valid: true},
		}, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(77)).
		Return(db.Order{ID: 77, MerchantID: 88}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(88)).
		Return(db.MerchantPaymentConfig{MerchantID: 88, SubMchID: "1900000109"}, nil)
	ordinaryClient.EXPECT().
		QueryPayment(gomock.Any(), ospcontracts.PaymentQueryRequest{SubMchID: "1900000109", OutTradeNo: "OC20260415000031"}).
		Return(nil, &ordinaryserviceprovider.ProviderError{
			Operation:    "query_payment",
			Category:     ordinaryserviceprovider.ErrorCategoryProvider,
			Frontend:     ordinaryserviceprovider.FrontendGuidance{Message: "微信支付状态返回异常", Action: "请不要重复支付，返回订单页后重新查询"},
			ProviderCode: "RESPONSE_SCHEMA_DRIFT",
		})

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "微信支付状态返回异常，请不要重复支付，返回订单页后重新查询", resp.Message)
}

func TestQueryPaymentOrderAPI_CombinedPaymentReturnsClearError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:                123,
			UserID:            1001,
			PaymentType:       PaymentTypeProfitShare,
			PaymentChannel:    db.PaymentChannelEcommerce,
			CombinedPaymentID: pgtype.Int8{Int64: 9, Valid: true},
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadRequest, resp.Code)
	require.Equal(t, "合单支付订单请使用合单查询接口", resp.Message)
}

func TestQueryPaymentOrderAPI_DirectPaymentReturnsClearError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(123)).
		Return(db.PaymentOrder{
			ID:          123,
			UserID:      1001,
			PaymentType: PaymentTypeMiniProgram,
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadRequest, resp.Code)
	require.Equal(t, "当前支付通道不支持微信远端查询", resp.Message)
}

func TestCreateCombinedPaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(gin.H{"order_ids": []int64{11, 22}})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/payments/combined", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "合单支付能力未完成配置，请联系平台处理", resp.Message)
}

func TestCloseCombinedPaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPost, "/v1/payments/combined/123/close", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "合单支付能力未完成配置，当前无法关闭支付单，请联系平台处理", resp.Message)
}

func TestQueryCombinedPaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/combined/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "合单支付能力未完成配置，当前无法确认支付状态，请联系平台处理", resp.Message)
}

func TestQueryCombinedPaymentOrderAPI_ContractDriftReturnsClearError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	subOrders, err := json.Marshal([]map[string]any{{
		"order_id":         int64(11),
		"payment_order_id": int64(22),
		"merchant_id":      int64(33),
		"sub_mch_id":       "1900001111",
		"amount":           int64(5000),
		"out_trade_no":     "P202001010000000001",
		"description":      "test-sub-order",
	}})
	require.NoError(t, err)

	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(123)).
		Times(1).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:                123,
			UserID:            1001,
			CombineOutTradeNo: "CP20260406000001",
			TotalAmount:       5000,
			PrepayID:          pgtype.Text{String: "combine-prepay-123", Valid: true},
			Status:            "pending",
			ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
			SubOrders:         subOrders,
		}, nil)
	ordinaryClient.EXPECT().
		ServiceProviderMchID().
		Return("1900000001")
	ordinaryClient.EXPECT().
		QueryCombinePayment(gomock.Any(), ospcontracts.CombineQueryRequest{CombineMchID: "1900000001", CombineOutTradeNo: "CP20260406000001"}).
		Times(1).
		Return(nil, &ordinaryserviceprovider.ProviderError{
			Operation:    "query_combine_payment",
			Category:     ordinaryserviceprovider.ErrorCategoryProvider,
			Frontend:     ordinaryserviceprovider.FrontendGuidance{Message: "微信支付状态返回异常", Action: "请不要重复支付，返回订单页后重新查询"},
			ProviderCode: "RESPONSE_SCHEMA_DRIFT",
		})

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/combined/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "微信支付状态返回异常，请不要重复支付，返回订单页后重新查询", resp.Message)
}

func TestQueryCombinedPaymentOrderAPI_RemotePaidOmitsPayParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServerWithOrdinaryServiceProvider(t, store, ordinaryClient)
	recorder := httptest.NewRecorder()

	subOrders, err := json.Marshal([]map[string]any{{
		"order_id":         int64(11),
		"payment_order_id": int64(22),
		"merchant_id":      int64(33),
		"sub_mch_id":       "1900001111",
		"amount":           int64(5000),
		"out_trade_no":     "P202001010000000001",
		"description":      "test-sub-order",
	}})
	require.NoError(t, err)

	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(123)).
		Times(1).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:                123,
			UserID:            1001,
			CombineOutTradeNo: "CP20260406000001",
			TotalAmount:       5000,
			PrepayID:          pgtype.Text{String: "combine-prepay-123", Valid: true},
			Status:            "pending",
			ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
			SubOrders:         subOrders,
		}, nil)
	ordinaryClient.EXPECT().
		ServiceProviderMchID().
		Return("1900000001")
	ordinaryClient.EXPECT().
		QueryCombinePayment(gomock.Any(), ospcontracts.CombineQueryRequest{CombineMchID: "1900000001", CombineOutTradeNo: "CP20260406000001"}).
		Times(1).
		Return(&ospcontracts.CombineQueryResponse{
			CombineMchID:      "1900000001",
			CombineOutTradeNo: "CP20260406000001",
			SubOrders: []ospcontracts.CombineOrderState{{
				MchID:         "service-mchid-001",
				SubMchID:      "1900001111",
				OutTradeNo:    "P202001010000000001",
				TransactionID: "wx-txn-123",
				TradeType:     "JSAPI",
				TradeState:    ospcontracts.PaymentTradeStateSuccess,
				PromotionDetail: []ospcontracts.PaymentPromotionDetail{{
					CouponID: "coupon-1",
					Amount:   300,
					Currency: "CNY",
				}},
				Amount: ospcontracts.CombineAmount{
					TotalAmount:   5000,
					PayerAmount:   5000,
					Currency:      ospcontracts.CurrencyCNY,
					PayerCurrency: ospcontracts.CurrencyCNY,
				},
			}},
		}, nil)

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/combined/123/query", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response combinedPaymentOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Nil(t, response.PayParams)
	require.NotNil(t, response.WechatQuery)
	require.Equal(t, "paid", response.WechatQuery.AggregateTradeState)
	require.Equal(t, "CP20260406000001", response.WechatQuery.CombineOutTradeNo)
	require.Len(t, response.WechatQuery.SubOrders, 1)
	require.Len(t, response.WechatQuery.SubOrders[0].PromotionDetail, 1)
	require.Equal(t, "coupon-1", response.WechatQuery.SubOrders[0].PromotionDetail[0].CouponID)
	require.Len(t, response.SubOrders, 1)
}

// ==================== CreateRefundOrder Tests ====================

func TestCreateRefundOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)

	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Status = "paid"
	paymentOrder.Amount = 100 * fenPerYuan
	paymentOrder.PaymentType = PaymentTypeProfitShare

	refundOrder := randomRefundOrder(paymentOrder.ID, 100*fenPerYuan)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(orchestrator *stubRefundOrchestrator)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    10000,
				"refund_reason":    "用户申请退款",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(_ context.Context, input logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					require.Equal(t, user.ID, input.ActorUserID)
					require.Equal(t, paymentOrder.ID, input.PaymentOrderID)
					require.Equal(t, "full", input.RefundType)
					require.Equal(t, int64(10000), input.RefundAmount)
					require.Equal(t, "用户申请退款", input.RefundReason)
					return logic.CreateRefundOrderResult{RefundOrder: refundOrder}, nil
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response refundOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, refundOrder.ID, response.ID)
			},
		},
		{
			name: "NotAMerchant",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					return logic.CreateRefundOrderResult{}, logic.NewRequestError(http.StatusForbidden, errors.New("you are not a merchant"))
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "PaymentNotFound",
			body: gin.H{
				"payment_order_id": 99999,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					return logic.CreateRefundOrderResult{}, logic.NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "PaymentNotPaid",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					return logic.CreateRefundOrderResult{}, logic.NewRequestError(http.StatusBadRequest, errors.New("payment order is not paid"))
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RefundAmountExceedsPayment",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "partial",
				"refund_amount":    20000, // 超过支付金额
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					return logic.CreateRefundOrderResult{}, logic.NewRequestError(http.StatusBadRequest, errors.New("refund amount exceeds payment amount"))
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidRefundType",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "invalid",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidRefundAmount_Zero",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			orchestrator := &stubRefundOrchestrator{}
			tc.buildStubs(orchestrator)

			server := newTestServer(t, store)
			server.refundOrchestrator = orchestrator
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/refunds"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== GetRefundOrder Tests ====================

func TestGetRefundOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	order.MerchantID = merchant.ID

	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Status = "paid"

	refundOrder := randomRefundOrder(paymentOrder.ID, paymentOrder.Amount)

	testCases := []struct {
		name          string
		refundID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK_AsUser",
			refundID: refundOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRefundOrder(gomock.Any(), refundOrder.ID).
					Times(1).
					Return(refundOrder, nil)

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response refundOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, refundOrder.ID, response.ID)
			},
		},
		{
			name:     "OK_AsMerchant",
			refundID: refundOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 商户以自己的身份访问
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRefundOrder(gomock.Any(), refundOrder.ID).
					Times(1).
					Return(refundOrder, nil)

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "NotFound",
			refundID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRefundOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.RefundOrder{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "Forbidden_NotRelated",
			refundID: refundOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRefundOrder(gomock.Any(), refundOrder.ID).
					Times(1).
					Return(refundOrder, nil)

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				// 不是用户，检查是否是商户
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherUser.ID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			refundID:   refundOrder.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/refunds/%d", tc.refundID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ListRefundOrdersByPayment Tests ====================

func TestListRefundOrdersByPaymentAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	otherUser.ID = user.ID + 100000
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	order.MerchantID = merchant.ID

	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Status = "paid"

	refundOrder := randomRefundOrder(paymentOrder.ID, paymentOrder.Amount)

	testCases := []struct {
		name          string
		paymentID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				store.EXPECT().
					ListRefundOrdersByPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return([]db.RefundOrder{refundOrder}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listRefundOrdersByPaymentResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.RefundOrders, 1)
			},
		},
		{
			name:      "PaymentNotFound",
			paymentID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "Forbidden_NotOwner",
			paymentID: paymentOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				// 不是用户，检查是否是商户
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherUser.ID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			paymentID:  paymentOrder.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/payments/%d/refunds", tc.paymentID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestApplyPlatformAbnormalRefundAPI(t *testing.T) {
	admin, _ := randomUser(t)
	user, _ := randomUser(t)
	order := randomPaymentTestOrder(user.ID, util.RandomInt(1, 1000))
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	refundOrder := randomRefundOrder(paymentOrder.ID, paymentOrder.Amount)
	refundOrder.Status = "failed"
	refundOrder.RefundID = pgtype.Text{String: "wx_refund_001", Valid: true}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Admin",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Times(1).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
			},
		},
		{
			name: "Forbidden_NotAdmin",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: RoleCustomer, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServer(t, store)
			server.ecommerceClient = ecommerceClient
			server.refundOrchestrator = nil

			recorder := httptest.NewRecorder()
			body := gin.H{"type": wechat.EcommerceAbnormalRefundTypeMerchantBankCard}
			data, err := json.Marshal(body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/platform/refunds/%d/apply-abnormal-refund", refundOrder.ID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
