package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListKitchenOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetMerchantByOwner
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				// Mock ListMerchantOrdersByStatus for paid orders
				store.EXPECT().
					ListMerchantOrdersByStatus(gomock.Any(), gomock.Any()).
					Times(3). // paid, preparing, ready
					Return([]db.Order{}, nil)

				// Mock CountMerchantOrdersByStatusAfterTime
				store.EXPECT().
					CountMerchantOrdersByStatusAfterTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(5), nil)

				// Mock GetMerchantAvgPrepareTime - 返回历史平均出餐时间
				store.EXPECT().
					GetMerchantAvgPrepareTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(12), nil) // 平均12分钟
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrdersResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, 5, response.Stats.CompletedTodayCount)
				require.Equal(t, 12, response.Stats.AvgPrepareTime) // 验证平均出餐时间
			},
		},
		{
			name: "OK_NoHistoryData_UseDefault",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListMerchantOrdersByStatus(gomock.Any(), gomock.Any()).
					Times(3).
					Return([]db.Order{}, nil)

				store.EXPECT().
					CountMerchantOrdersByStatusAfterTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)

				// Mock GetMerchantAvgPrepareTime - 返回0表示没有历史数据
				store.EXPECT().
					GetMerchantAvgPrepareTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrdersResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, 0, response.Stats.CompletedTodayCount)
				// 没有历史数据时，使用默认值15分钟
				require.Equal(t, 15, response.Stats.AvgPrepareTime)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NotAMerchant",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/kitchen/orders"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestStartPreparingAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomKitchenOrder(merchant.ID, user.ID)
	order.Status = "paid" // 已支付状态

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				updatedOrder := order
				updatedOrder.Status = "preparing"
				store.EXPECT().
					UpdateOrderStatus(gomock.Any(), db.UpdateOrderStatusParams{
						ID:     order.ID,
						Status: "preparing",
					}).
					Times(1).
					Return(updatedOrder, nil)

				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "preparing", response.Status)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.Order{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				otherMerchant := merchant
				otherMerchant.ID = merchant.ID + 999

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(otherMerchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotInPaidStatus",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				// 返回一个已经在 preparing 状态的订单
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "InvalidOrderID",
			orderID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Should not reach store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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

			url := fmt.Sprintf("/v1/kitchen/orders/%d/preparing", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMarkKitchenOrderReadyAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomKitchenOrder(merchant.ID, user.ID)
	order.Status = "preparing" // 制作中状态

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK_FromPreparing",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				updatedOrder := order
				updatedOrder.Status = "ready"
				store.EXPECT().
					UpdateOrderStatus(gomock.Any(), db.UpdateOrderStatusParams{
						ID:     order.ID,
						Status: "ready",
					}).
					Times(1).
					Return(updatedOrder, nil)

				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "ready", response.Status)
			},
		},
		{
			name:    "OK_FromPaid",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(paidOrder, nil)

				updatedOrder := order
				updatedOrder.Status = "ready"
				store.EXPECT().
					UpdateOrderStatus(gomock.Any(), db.UpdateOrderStatusParams{
						ID:     order.ID,
						Status: "ready",
					}).
					Times(1).
					Return(updatedOrder, nil)

				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "InvalidStatus_Completed",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(completedOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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

			url := fmt.Sprintf("/v1/kitchen/orders/%d/ready", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetKitchenOrderDetailsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomKitchenOrder(merchant.ID, user.ID)
	dish := db.Dish{
		ID:          1,
		MerchantID:  merchant.ID,
		Name:        "测试菜品",
		Price:       1000,
		PrepareTime: 20, // 20分钟制作时间
		ImageUrl:    pgtype.Text{String: "https://example.com/dish.jpg", Valid: true},
	}
	orderItem := db.OrderItem{
		ID:        1,
		OrderID:   order.ID,
		DishID:    pgtype.Int8{Int64: dish.ID, Valid: true},
		Name:      dish.Name,
		Quantity:  2,
		UnitPrice: dish.Price,
		Subtotal:  dish.Price * 2,
	}

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{orderItem}, nil)

				// Mock GetDish for prepare_time
				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(1), nil) // Has urge
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, order.ID, response.ID)
				require.True(t, response.IsUrged)
				// 验证订单商品包含制作时间
				require.Len(t, response.Items, 1)
				require.Equal(t, int16(20), response.Items[0].PrepareTime)
				// 验证预估出餐时间已设置
				require.NotNil(t, response.EstimatedReadyAt)
			},
		},
		{
			name:    "OK_NoItems",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				// Mock for convertToKitchenOrder - 无订单商品
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, order.ID, response.ID)
				require.False(t, response.IsUrged)
				require.Empty(t, response.Items)
				// 无商品时，预估出餐时间为nil
				require.Nil(t, response.EstimatedReadyAt)
			},
		},
		{
			name:    "NotFound",
			orderID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.Order{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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

			url := fmt.Sprintf("/v1/kitchen/orders/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper function
func randomKitchenOrder(merchantID, userID int64) db.Order {
	return db.Order{
		ID:          1,
		OrderNo:     "ORD20251210001",
		MerchantID:  merchantID,
		UserID:      userID,
		OrderType:   "dine_in",
		Status:      "paid",
		TotalAmount: 10000,
		CreatedAt:   time.Now(),
		PaidAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}
