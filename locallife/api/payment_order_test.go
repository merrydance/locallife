package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
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
	request.RemoteAddr = "203.0.113.9:12345"
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
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchantID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		}).
		Times(1).
		Return(db.BaofuMerchantReport{
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchantID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
			SubMchID:        pgtype.Text{String: subMchID, Valid: true},
		}, nil)
}

type fakeAPIBaofuAggregateClient struct {
	calledUnified bool
	lastUnified   aggregatecontracts.UnifiedOrderRequest
	queryErr      error
	queryResult   *aggregatecontracts.UnifiedOrderResult
	lastQuery     aggregatecontracts.PaymentQueryRequest
	closeErr      error
	closeResult   *aggregatecontracts.OrderCloseResult
}

func (c *fakeAPIBaofuAggregateClient) CreateUnifiedOrder(_ context.Context, req aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.calledUnified = true
	c.lastUnified = req
	return &aggregatecontracts.UnifiedOrderResult{
		TradeNo: "BFPAY_API_001",
		ChannelReturn: aggregatecontracts.ChannelReturn{
			WechatPayData: json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"nonce-api-baofu","package":"prepay_id=baofu-api","signType":"RSA","paySign":"pay-sign-api-baofu"}`),
		},
	}, nil
}

func (c *fakeAPIBaofuAggregateClient) QueryPayment(_ context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastQuery = req
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.queryResult != nil {
		return c.queryResult, nil
	}
	return nil, errors.New("not implemented in api baofu payment test")
}

func (c *fakeAPIBaofuAggregateClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in api baofu payment test")
}

func (c *fakeAPIBaofuAggregateClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in api baofu payment test")
}

func (c *fakeAPIBaofuAggregateClient) CreateRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in api baofu payment test")
}

func (c *fakeAPIBaofuAggregateClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in api baofu payment test")
}

func (c *fakeAPIBaofuAggregateClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	if c.closeErr != nil {
		return nil, c.closeErr
	}
	if c.closeResult != nil {
		return c.closeResult, nil
	}
	return nil, errors.New("not implemented in api baofu payment test")
}

// ==================== CreatePaymentOrder Tests ====================

func TestCreatePaymentOrderAPIUsesBaofuWhenMainBusinessConfigured(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	order.TotalAmount = 1200
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Amount = order.TotalAmount
	paymentOrder.PaymentType = PaymentTypeMiniProgram
	paymentOrder.PaymentChannel = db.PaymentChannelBaofuAggregate
	paymentOrder.OutTradeNo = "BFAPI202605040001"
	paymentOrder.Attach = pgtype.Text{String: "order_id:8001", Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeAPIBaofuAggregateClient{}

	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	expectActiveMerchantBaofuAccountForPaymentAPI(store, merchant.ID, "sub-api-baofu")
	store.EXPECT().
		GetUser(gomock.Any(), user.ID).
		Times(1).
		Return(user, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePartnerPaymentTxParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.PaymentChannel)
			require.True(t, arg.RequiresProfitSharing)
			return db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "sub-api-baofu"}, nil
		})
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
			return db.ExternalPaymentCommand{ID: 8801, CommandStatus: arg.CommandStatus}, nil
		})
	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(baofuClient, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_API",
		CollectTerminalID: "TER_API",
		MiniProgramAppID:  "wx-mini-api",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	server.SetTaskDistributorForTest(nil)

	body, err := json.Marshal(gin.H{"order_id": order.ID, "business_type": "order"})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/payments", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.9:12345"
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.True(t, baofuClient.calledUnified)
	require.Equal(t, "COLLECT_API", baofuClient.lastUnified.MerchantID)
	require.Equal(t, "sub-api-baofu", baofuClient.lastUnified.SubMchID)
	require.Equal(t, user.WechatOpenid, baofuClient.lastUnified.PayExtend.SubOpenID)
	var response paymentOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, paymentOrder.ID, response.ID)
	require.NotNil(t, response.PayParams)
	require.Equal(t, "prepay_id=baofu-api", response.PayParams.Package)
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

func TestClosePaymentOrderAPI_BaofuProviderErrorReturnsStableChineseMessage(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)
	paymentOrder.ID = 77101
	paymentOrder.PaymentChannel = db.PaymentChannelBaofuAggregate
	paymentOrder.Status = PaymentStatusPending
	paymentOrder.OutTradeNo = "BF_API_CLOSE_ERR_1"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeAPIBaofuAggregateClient{
		closeErr: baofu.NewProviderBusinessError("order_close", "ORDER_NOT_EXIST", "provider-secret-detail"),
	}
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 77102}, nil)

	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(baofuClient, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/payments/%d/close", paymentOrder.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "宝付支付状态暂不可确认，请稍后刷新支付状态；如持续失败请联系平台处理", resp.Message)
	require.NotContains(t, recorder.Body.String(), "ORDER_NOT_EXIST")
	require.NotContains(t, recorder.Body.String(), "provider-secret-detail")
}

func TestClosePaymentOrderAPI_BaofuServiceNotConfiguredReturnsStableChineseMessage(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)
	paymentOrder.ID = 77111
	paymentOrder.PaymentChannel = db.PaymentChannelBaofuAggregate
	paymentOrder.Status = PaymentStatusPending
	paymentOrder.OutTradeNo = "BF_API_CLOSE_CFG_1"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)

	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(&fakeAPIBaofuAggregateClient{}, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/payments/%d/close", paymentOrder.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "宝付支付服务暂不可用，请联系平台处理", resp.Message)
	require.NotContains(t, recorder.Body.String(), logic.ErrBaofuPaymentServiceNotConfigured.Error())
}

func TestQueryPaymentOrderAPI_BaofuProviderErrorReturnsStableChineseMessage(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)
	paymentOrder.ID = 77121
	paymentOrder.PaymentChannel = db.PaymentChannelBaofuAggregate
	paymentOrder.Status = PaymentStatusPending
	paymentOrder.OutTradeNo = "BF_API_QUERY_ERR_1"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeAPIBaofuAggregateClient{
		queryErr: baofu.NewProviderBusinessError("order_query", "SYSTEM_BUSY", "provider-secret-detail"),
	}
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)

	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(baofuClient, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/payments/%d/query", paymentOrder.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "宝付支付状态暂不可确认，请稍后刷新支付状态；如持续失败请联系平台处理", resp.Message)
	require.NotContains(t, recorder.Body.String(), "SYSTEM_BUSY")
	require.NotContains(t, recorder.Body.String(), "provider-secret-detail")
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: paymentOrder.OutTradeNo,
	}, baofuClient.lastQuery)
}

func TestCreatePaymentOrderAPI_ServiceUnavailableWhenBaofuPaymentUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(11)).
		Return(db.Order{ID: 11, UserID: 1001, MerchantID: 22, Status: "pending", TotalAmount: 5000}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(22)).
		Return(db.Merchant{ID: 22, Name: "测试商户"}, nil)
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
	require.Equal(t, "宝付支付通道未配置，请联系平台处理", resp.Message)
}

func TestQueryPaymentOrderAPI_DirectPaymentReturnsClearError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
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
					require.Equal(t, "refund-api-1", input.IdempotencyKey)
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
			name: "BaofuShareStarted",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "partial",
				"refund_amount":    1000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(orchestrator *stubRefundOrchestrator) {
				orchestrator.createRefundOrderFunc = func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
					return logic.CreateRefundOrderResult{}, logic.NewRequestError(http.StatusBadRequest, errors.New("订单已进入结算分账流程，不支持退款"))
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "订单已进入结算分账流程，不支持退款")
				require.NotContains(t, recorder.Body.String(), "sharing_mer_id")
				require.NotContains(t, recorder.Body.String(), "contractNo")
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
			if tc.name != "NoAuthorization" && tc.name != "InvalidRefundType" && tc.name != "InvalidRefundAmount_Zero" {
				request.Header.Set("Idempotency-Key", "refund-api-1")
			}
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreateRefundOrderAPI_RequiresIdempotencyKey(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	orchestrator := &stubRefundOrchestrator{
		createRefundOrderFunc: func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
			t.Fatal("refund orchestrator should not be called without Idempotency-Key")
			return logic.CreateRefundOrderResult{}, nil
		},
	}

	server := newTestServer(t, store)
	server.refundOrchestrator = orchestrator
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(gin.H{
		"payment_order_id": paymentOrder.ID,
		"refund_type":      "full",
		"refund_amount":    int64(10000),
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/refunds", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Idempotency-Key header is required")
}

func TestCreateRefundOrderAPI_RejectsTooLongIdempotencyKey(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	orchestrator := &stubRefundOrchestrator{
		createRefundOrderFunc: func(context.Context, logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
			t.Fatal("refund orchestrator should not be called with an oversized Idempotency-Key")
			return logic.CreateRefundOrderResult{}, nil
		},
	}

	server := newTestServer(t, store)
	server.refundOrchestrator = orchestrator
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(gin.H{
		"payment_order_id": paymentOrder.ID,
		"refund_type":      "full",
		"refund_amount":    int64(10000),
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/refunds", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", strings.Repeat("a", refundCreateMaxIdempotencyKeyLength+1))
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Idempotency-Key header is too long")
}

func TestCreateRefundOrderAPI_PassesTrimmedIdempotencyKeyToLogic(t *testing.T) {
	user, _ := randomUser(t)
	paymentOrder := randomPaymentOrder(user.ID, nil)
	refundOrder := randomRefundOrder(paymentOrder.ID, 100*fenPerYuan)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	orchestrator := &stubRefundOrchestrator{
		createRefundOrderFunc: func(_ context.Context, input logic.CreateRefundOrderInput) (logic.CreateRefundOrderResult, error) {
			require.Equal(t, "refund-key-1", input.IdempotencyKey)
			return logic.CreateRefundOrderResult{RefundOrder: refundOrder}, nil
		},
	}

	server := newTestServer(t, store)
	server.refundOrchestrator = orchestrator
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(gin.H{
		"payment_order_id": paymentOrder.ID,
		"refund_type":      "full",
		"refund_amount":    int64(10000),
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/refunds", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", " refund-key-1 ")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
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

func TestCreateCombinedPaymentOrderAPI_BaofuMainBusinessFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(&fakeAPIBaofuAggregateClient{}, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_API",
		CollectTerminalID: "TER_API",
		MiniProgramAppID:  "wx-mini-api",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})

	body, err := json.Marshal(gin.H{"order_ids": []int64{11, 22}})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/payments/combined", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "宝付合单支付暂未开通，请分开支付", resp.Message)
}

func TestGetPaymentCapabilitiesAPI_BaofuMainBusinessRequiresSplitCheckout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregateClientForTest(&fakeAPIBaofuAggregateClient{}, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "COLLECT_API",
		CollectTerminalID: "TER_API",
		MiniProgramAppID:  "wx-mini-api",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})

	request, err := http.NewRequest(http.MethodGet, "/v1/payments/capabilities", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, 1001, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int                         `json:"code"`
		Data paymentCapabilitiesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeOK, resp.Code)
	require.Equal(t, db.PaymentChannelBaofuAggregate, resp.Data.MainBusinessPaymentChannel)
	require.False(t, resp.Data.CombinedPaymentSupported)
	require.True(t, resp.Data.SplitCheckoutRequired)
	require.Equal(t, "宝付暂不支持合单支付，请按商户分别下单支付", resp.Data.CombinedPaymentUnavailableMessage)
}
