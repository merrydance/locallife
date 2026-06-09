package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListKitchenOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantKitchenOrdersByStage(gomock.Any(), gomock.Any()).
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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, 5, response.Stats.CompletedTodayCount)
				require.Equal(t, 12, response.Stats.AvgPrepareTime) // 验证平均出餐时间
			},
		},
		{
			name: "OK_IncludesCourierAcceptedPreparingAsPreparing",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				order := randomKitchenOrder(merchant.ID, user.ID)
				order.ID = 601
				order.OrderType = db.OrderTypeTakeout
				order.Status = db.OrderStatusCourierAccepted
				order.FulfillmentStatus = db.FulfillmentStatusPreparing

				store.EXPECT().
					ListMerchantKitchenOrdersByStage(gomock.Any(), gomock.Any()).
					Times(3).
					DoAndReturn(func(_ any, arg db.ListMerchantKitchenOrdersByStageParams) ([]db.Order, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						switch arg.Stage {
						case "paid":
							return []db.Order{}, nil
						case "preparing":
							return []db.Order{order}, nil
						case "ready":
							return []db.Order{}, nil
						default:
							t.Fatalf("unexpected kitchen stage %q", arg.Stage)
							return nil, nil
						}
					})

				store.EXPECT().
					CountMerchantOrdersByStatusAfterTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					GetMerchantAvgPrepareTime(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(15), nil)
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
				var response kitchenOrdersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.PreparingOrders, 1)
				got := response.PreparingOrders[0]
				require.Equal(t, db.OrderStatusCourierAccepted, got.OrderStatus)
				require.Equal(t, db.FulfillmentStatusPreparing, got.FulfillmentStatus)
				require.Equal(t, "preparing", got.KitchenStatus)
				require.True(t, got.CanMarkReady)
				require.Contains(t, got.StatusHint, "骑手已接单")
			},
		},
		{
			name: "OK_NoHistoryData_UseDefault",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantKitchenOrdersByStage(gomock.Any(), gomock.Any()).
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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Forbidden_PendingStaffRole",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				pendingStaffMerchant := merchant
				pendingStaffMerchant.OwnerUserID = user.ID + 1000
				pendingStaffMerchant.Status = "active"
				pendingStaffMerchant.RegionID = 1

				expectResolveSingleStaffMerchant(store, user.ID, pendingStaffMerchant)
				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: pendingStaffMerchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return(db.MerchantStaffRolePending, nil)
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
	merchant.IsOpen = true
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				updatedOrder := order
				updatedOrder.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdateOrderStatusTxParams) (db.UpdateOrderStatusTxResult, error) {
						require.Equal(t, order.ID, arg.OrderID)
						require.Equal(t, db.OrderStatusPreparing, arg.NewStatus)
						require.Equal(t, db.OrderStatusPaid, arg.OldStatus)
						return db.UpdateOrderStatusTxResult{Order: updatedOrder}, nil
					})

				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
				expectReadyPrintConfigFallback(store, merchant.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "preparing", response.Status)
			},
		},
		{
			name:    "OK_TakeoutCreatesDeliveryPoolEntry",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				takeoutOrder := order
				takeoutOrder.OrderType = db.OrderTypeTakeout
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(takeoutOrder, nil)

				updatedOrder := takeoutOrder
				updatedOrder.Status = db.OrderStatusPreparing
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(takeoutOrder, nil)
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)
				store.EXPECT().
					AcceptTakeoutOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.AcceptTakeoutOrderTxParams) (db.AcceptTakeoutOrderTxResult, error) {
						require.Equal(t, takeoutOrder.ID, arg.OrderID)
						require.Equal(t, db.OrderStatusPaid, arg.OldStatus)
						require.Equal(t, user.ID, arg.OperatorID)
						require.Equal(t, "merchant", arg.OperatorType)
						return db.AcceptTakeoutOrderTxResult{
							Order:    updatedOrder,
							PoolItem: db.DeliveryPool{OrderID: takeoutOrder.ID, MerchantID: merchant.ID},
						}, nil
					})

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
				expectReadyPrintConfigFallback(store, merchant.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, db.OrderStatusPreparing, response.Status)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
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

				expectResolveSingleOwnedMerchant(store, user.ID, otherMerchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 返回一个已经在 preparing 状态的订单
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(preparingOrder, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(preparingOrder, nil)
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
	merchant.IsOpen = true
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				updatedOrder := order
				updatedOrder.Status = "ready"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdateOrderStatusTxParams) (db.UpdateOrderStatusTxResult, error) {
						require.Equal(t, order.ID, arg.OrderID)
						require.Equal(t, db.OrderStatusReady, arg.NewStatus)
						require.Equal(t, db.OrderStatusPreparing, arg.OldStatus)
						return db.UpdateOrderStatusTxResult{Order: updatedOrder}, nil
					})
				// Mock for convertToKitchenOrder
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
				expectReadyPrintConfigFallback(store, merchant.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(paidOrder, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OK_TakeoutMarksReadyWithoutCreatingDeliveryPoolEntry",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				takeoutOrder := order
				takeoutOrder.OrderType = db.OrderTypeTakeout
				takeoutOrder.Status = db.OrderStatusPreparing
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(takeoutOrder, nil)

				updatedOrder := takeoutOrder
				updatedOrder.Status = db.OrderStatusReady
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(takeoutOrder, nil)
				store.EXPECT().
					MarkTakeoutOrderReadyTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.MarkTakeoutOrderReadyTxParams) (db.MarkTakeoutOrderReadyTxResult, error) {
						require.Equal(t, takeoutOrder.ID, arg.OrderID)
						require.Equal(t, db.OrderStatusPreparing, arg.OldStatus)
						require.Equal(t, user.ID, arg.OperatorID)
						require.Equal(t, "merchant", arg.OperatorType)
						return db.MarkTakeoutOrderReadyTxResult{
							Order: updatedOrder,
						}, nil
					})

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

				store.EXPECT().
					CountOrderUrges(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(int64(0), nil)
				expectReadyPrintConfigFallback(store, merchant.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response kitchenOrderResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, db.OrderStatusReady, response.Status)
			},
		},
		{
			name:    "InvalidStatus_Completed",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(completedOrder, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
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

func expectReadyPrintConfigFallback(store *mockdb.MockStore, merchantID int64) {
	store.EXPECT().
		GetOrderDisplayConfigByMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.OrderDisplayConfig{}, db.ErrRecordNotFound)
}

func TestMarkKitchenOrderReadyAPILogicErrorMapping(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	order := randomKitchenOrder(merchant.ID, user.ID)
	order.Status = db.OrderStatusPreparing
	order.OrderType = db.OrderTypeTakeout

	testCases := []struct {
		name       string
		serviceErr error
		wantCode   int
	}{
		{
			name:       "FoodSafetySuspended",
			serviceErr: logic.NewRequestError(http.StatusForbidden, errors.New("食安暂停期间不可继续处理外卖订单")),
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "InfraFailure",
			serviceErr: errors.New("database unavailable"),
			wantCode:   http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			store.EXPECT().
				GetOrder(gomock.Any(), gomock.Eq(order.ID)).
				Times(1).
				Return(order, nil)
			store.EXPECT().
				GetOrderForUpdate(gomock.Any(), gomock.Eq(order.ID)).
				Times(1).
				Return(order, nil)
			store.EXPECT().
				MarkTakeoutOrderReadyTx(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.MarkTakeoutOrderReadyTxResult{}, tc.serviceErr)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/kitchen/orders/%d/ready", order.ID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			server.router.ServeHTTP(recorder, request)
			require.Equal(t, tc.wantCode, recorder.Code)
		})
	}
}

func TestGetKitchenOrderDetailsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	order := randomKitchenOrder(merchant.ID, user.ID)
	dish := db.Dish{
		ID:                1,
		MerchantID:        merchant.ID,
		Name:              "测试菜品",
		Price:             10 * fenPerYuan,
		PrepareTime:       20, // 20分钟制作时间
		ImageMediaAssetID: pgtype.Int8{},
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, order.ID, response.ID)
				require.NotNil(t, response.PickupCode)
				require.Equal(t, "0001", *response.PickupCode)
				require.NotNil(t, response.PickupNumber)
				require.Equal(t, "0001", *response.PickupNumber)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, order.ID, response.ID)
				require.False(t, response.IsUrged)
				require.Empty(t, response.Items)
				// 无商品时，预估出餐时间为nil
				require.Nil(t, response.EstimatedReadyAt)
			},
		},
		{
			name:    "InvalidItemCustomizations",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				brokenItem := orderItem
				brokenItem.Customizations = []byte("not-json")
				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return([]db.OrderItem{brokenItem}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
			},
		},
		{
			name:    "NotFound",
			orderID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
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
		PickupCode:  pgtype.Text{String: "0001", Valid: true},
		CreatedAt:   time.Now(),
		PaidAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}
