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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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
				"order_id":      order.ID,
				"payment_type":  "miniprogram",
				"business_type": "order",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, pgx.ErrNoRows)

				store.EXPECT().
					CreatePaymentOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(paymentOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response paymentOrderResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
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
			buildStubs: func(store *mockdb.MockStore) {
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
			buildStubs: func(store *mockdb.MockStore) {
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				// 已存在待支付订单
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(paymentOrder, nil)

				// 不应该创建新的支付订单
				store.EXPECT().
					CreatePaymentOrder(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response paymentOrderResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, paymentOrder.ID, response.ID)
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
			buildStubs: func(store *mockdb.MockStore) {},
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
			buildStubs: func(store *mockdb.MockStore) {},
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
			buildStubs: func(store *mockdb.MockStore) {},
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
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
					Return(db.PaymentOrder{}, pgx.ErrNoRows)
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

				var response []paymentOrderResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Len(t, response, 1)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize_TooSmall",
			query: "page_id=1&page_size=4",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
					Return(db.PaymentOrder{}, pgx.ErrNoRows)
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
	paymentOrder.Amount = 10000

	refundOrder := randomRefundOrder(paymentOrder.ID, 10000)

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
					CreateRefundOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(refundOrder, nil)

				store.EXPECT().
					GetRefundOrder(gomock.Any(), refundOrder.ID).
					Times(1).
					Return(refundOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response refundOrderResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
					Return(db.Merchant{}, pgx.ErrNoRows)
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
					Return(db.PaymentOrder{}, pgx.ErrNoRows)
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
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
					Return(db.RefundOrder{}, pgx.ErrNoRows)
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
					Return(db.Merchant{}, pgx.ErrNoRows)
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

				var response []refundOrderResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Len(t, response, 1)
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
					Return(db.PaymentOrder{}, pgx.ErrNoRows)
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
					Return(db.Merchant{}, pgx.ErrNoRows)
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
