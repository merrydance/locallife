package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
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

// ==================== CreatePaymentOrder Tests ====================

func TestCreatePaymentOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Amount = order.TotalAmount
	paymentOrder.PaymentType = PaymentTypeProfitShare
	paymentOrder.CombinedPaymentID = pgtype.Int8{Int64: util.RandomInt(1, 1000), Valid: true}
	combinedPaymentOrder := db.CombinedPaymentOrder{
		ID:                paymentOrder.CombinedPaymentID.Int64,
		UserID:            user.ID,
		CombineOutTradeNo: "OC" + util.RandomString(21),
		TotalAmount:       order.TotalAmount,
		Status:            PaymentStatusPending,
		ExpiresAt:         paymentOrder.ExpiresAt,
	}
	payParams := &wechat.JSAPIPayParams{
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
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
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
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUser(gomock.Any(), user.ID).
					Times(1).
					Return(user, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{
						CombinedPaymentOrder: combinedPaymentOrder,
						PaymentOrders:        []db.PaymentOrder{paymentOrder},
						OrderInfos: []db.CombinedPaymentOrderInfo{{
							Order:        order,
							PaymentOrder: paymentOrder,
							PaymentConfig: db.MerchantPaymentConfig{
								MerchantID: merchant.ID,
								SubMchID:   "1900000109",
								Status:     "active",
							},
							Merchant: merchant,
						}},
					}, nil)

				ecommerceClient.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechat.CombineOrderResponse{PrepayID: "wx123"}, payParams, nil)

				store.EXPECT().
					UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
						updated := paymentOrder
						updated.PrepayID = arg.PrepayID
						return updated, nil
					})

				store.EXPECT().
					UpdateCombinedPaymentOrderPrepay(gomock.Any(), gomock.Any()).
					Times(1).
					Return(combinedPaymentOrder, nil)
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
			name: "OrderNotFound",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {
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
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {
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
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {
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
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
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

				ecommerceClient.EXPECT().
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
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
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
						GetUser(gomock.Any(), user.ID).
						Times(1).
						Return(user, nil),
					store.EXPECT().
						CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
						Times(1).
						Return(db.CreateCombinedPaymentTxResult{}, db.ErrOrderPendingPaymentConflict),
					store.EXPECT().
						GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
						Times(1).
						Return(existingPendingPayment, nil),
					ecommerceClient.EXPECT().
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
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "LegacyNativePaymentType_StillAccepted",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "native",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUser(gomock.Any(), user.ID).
					Times(1).
					Return(user, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{
						CombinedPaymentOrder: combinedPaymentOrder,
						PaymentOrders:        []db.PaymentOrder{paymentOrder},
						OrderInfos: []db.CombinedPaymentOrderInfo{{
							Order:        order,
							PaymentOrder: paymentOrder,
							PaymentConfig: db.MerchantPaymentConfig{
								MerchantID: merchant.ID,
								SubMchID:   "1900000109",
								Status:     "active",
							},
							Merchant: merchant,
						}},
					}, nil)

				ecommerceClient.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechat.CombineOrderResponse{PrepayID: "wx123"}, payParams, nil)

				store.EXPECT().
					UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
						updated := paymentOrder
						updated.PrepayID = arg.PrepayID
						return updated, nil
					})

				store.EXPECT().
					UpdateCombinedPaymentOrderPrepay(gomock.Any(), gomock.Any()).
					Times(1).
					Return(combinedPaymentOrder, nil)
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
			name: "InvalidBusinessType",
			body: gin.H{
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "invalid",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {},
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
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {},
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
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore, _ *mockwechat.MockEcommerceClientInterface) {},
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
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServer(t, store)
			server.SetEcommerceClientForTest(ecommerceClient)
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

// ==================== CreateRefundOrder Tests ====================

func TestCreateRefundOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomPaymentTestOrder(user.ID, merchant.ID)
	order.MerchantID = merchant.ID

	paymentOrder := randomPaymentOrder(user.ID, &order.ID)
	paymentOrder.Status = "paid"
	paymentOrder.Amount = 100 * fenPerYuan

	refundOrder := randomRefundOrder(paymentOrder.ID, 100*fenPerYuan)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					CreateRefundOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateRefundOrderTxResult{RefundOrder: refundOrder}, nil)

				store.EXPECT().
					GetRefundOrder(gomock.Any(), refundOrder.ID).
					Times(1).
					Return(refundOrder, nil)
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
			buildStubs: func(store *mockdb.MockStore) {
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
			name: "PaymentNotFound",
			body: gin.H{
				"payment_order_id": 99999,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
			name: "PaymentNotPaid",
			body: gin.H{
				"payment_order_id": paymentOrder.ID,
				"refund_type":      "full",
				"refund_amount":    10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				pendingPayment := paymentOrder
				pendingPayment.Status = "pending"
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(pendingPayment, nil)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(1).
					Return(paymentOrder, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
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
			buildStubs: func(store *mockdb.MockStore) {},
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
			buildStubs: func(store *mockdb.MockStore) {},
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
