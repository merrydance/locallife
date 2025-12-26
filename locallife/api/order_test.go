package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	// 设置商户状态为 active（order.go 验证需要）
	merchant.Status = "active"
	dish := randomDish(merchant.ID, nil)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
				"notes": "少辣",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				// 满减规则查询
				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateOrderTxResult{
						Order: db.Order{
							ID:          1,
							OrderNo:     "20240101120000123456",
							UserID:      user.ID,
							MerchantID:  merchant.ID,
							OrderType:   "takeaway",
							Subtotal:    dish.Price * 2,
							TotalAmount: dish.Price * 2,
							Status:      "pending",
							CreatedAt:   time.Now(),
						},
						Items: []db.OrderItem{
							{
								ID:        1,
								OrderID:   1,
								DishID:    pgtype.Int8{Int64: dish.ID, Valid: true},
								Name:      dish.Name,
								UnitPrice: dish.Price,
								Quantity:  2,
								Subtotal:  dish.Price * 2,
							},
						},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidOrderType",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "invalid",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TakeoutRequiresAddress",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DineInRequiresTable",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TakeoutAddressNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  int64(99999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// calculateOrderItems 会先调用 GetDish
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetUserAddress(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.UserAddress{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "TakeoutAddressBelongsToOtherUser",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  address.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// calculateOrderItems 会先调用 GetDish
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				// 返回属于其他用户的地址
				otherUserAddress := address
				otherUserAddress.UserID = user.ID + 1
				store.EXPECT().
					GetUserAddress(gomock.Any(), address.ID).
					Times(1).
					Return(otherUserAddress, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "DineInTableNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    int64(99999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTable(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Table{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "DineInTableBelongsToOtherMerchant",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// 返回属于其他商户的桌台
				otherMerchantTable := table
				otherMerchantTable.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(otherMerchantTable, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuth",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotActive",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				inactiveMerchant := merchant
				inactiveMerchant.Status = "inactive"
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(inactiveMerchant, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DishOffline",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				offlineDish := dish
				offlineDish.IsOnline = false
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(offlineDish, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ItemsExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items":       generateManyItems(dish.ID, 51), // max=50
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ItemsAtMax_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items":       generateManyItems(dish.ID, 50), // max=50, should pass validation
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 50个商品的GetDish调用
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(50).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateOrderTxResult{
						Order: db.Order{
							ID:          1,
							OrderNo:     "20240101120000123456",
							UserID:      user.ID,
							MerchantID:  merchant.ID,
							OrderType:   "takeaway",
							Subtotal:    dish.Price * 50,
							TotalAmount: dish.Price * 50,
							Status:      "pending",
							CreatedAt:   time.Now(),
						},
						Items: []db.OrderItem{},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "CustomizationsExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":        dish.ID,
						"quantity":       1,
						"customizations": generateManyCustomizations(11), // max=10
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationNameTooLong",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        generateLongString(51), // max=50
								"value":       "medium",
								"extra_price": 0,
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationValueTooLong",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       generateLongString(51), // max=50
								"extra_price": 0,
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationExtraPriceNegative",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       "medium",
								"extra_price": -100, // min=0
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationExtraPriceExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       "medium",
								"extra_price": 10001, // max=10000
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).
					Times(1).
					Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NotFound",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "Forbidden",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/orders/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCancelOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			body:    gin.H{"reason": "不想要了"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					CancelOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelOrderTxResult{
						Order: db.Order{
							ID:           order.ID,
							OrderNo:      order.OrderNo,
							UserID:       order.UserID,
							MerchantID:   order.MerchantID,
							Status:       "cancelled",
							CancelledAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
							CancelReason: pgtype.Text{String: "不想要了", Valid: true},
						},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "AlreadyPaid_WithRefund",
			orderID: order.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
				store.EXPECT().
					CancelOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelOrderTxResult{Order: paidOrder}, nil)
				// 注意：因为 taskDistributor 为 nil，不会调用 GetPaymentOrdersByOrder
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "AlreadyPreparing",
			orderID: order.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/orders/%d/cancel", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// Helper function for order tests
func randomOrder(userID, merchantID int64) db.Order {
	return db.Order{
		ID:          util.RandomInt(1, 1000),
		OrderNo:     fmt.Sprintf("%d%d", time.Now().Unix(), util.RandomInt(100000, 999999)),
		UserID:      userID,
		MerchantID:  merchantID,
		OrderType:   "takeaway",
		Subtotal:    util.RandomInt(1000, 10000),
		TotalAmount: util.RandomInt(1000, 10000),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
}

// ==================== 商户端订单管理测试 ====================

func TestListMerchantOrdersAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherUser, _ := randomUser(t)

	orders := []db.Order{
		randomOrder(otherUser.ID, merchant.ID),
		randomOrder(otherUser.ID, merchant.ID),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListOrdersByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(orders, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotAMerchant",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherUser.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "FilterByStatus",
			query: "page_id=1&page_size=10&status=paid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListOrdersByMerchantAndStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(orders, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestAcceptOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	customer, _ := randomUser(t)

	paidOrder := randomOrder(customer.ID, merchant.ID)
	paidOrder.Status = "paid"

	pendingOrder := randomOrder(customer.ID, merchant.ID)
	pendingOrder.Status = "pending"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: paidOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)

				acceptedOrder := paidOrder
				acceptedOrder.Status = "preparing"
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateOrderStatusTxResult{Order: acceptedOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: paidOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherMerchantOwner.ID).
					Times(1).
					Return(otherMerchant, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotPaid",
			orderID: pendingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), pendingOrder.ID).
					Times(1).
					Return(pendingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/accept", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestRejectOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	customer, _ := randomUser(t)

	paidOrder := randomOrder(customer.ID, merchant.ID)
	paidOrder.Status = "paid"

	testCases := []struct {
		name          string
		orderID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: paidOrder.ID,
			body:    gin.H{"reason": "材料售罄"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)

				rejectedOrder := paidOrder
				rejectedOrder.Status = "cancelled"
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateOrderStatusTxResult{Order: rejectedOrder}, nil)

				// 退款相关调用
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, pgx.ErrNoRows) // 模拟无支付订单
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "MissingReason",
			orderID: paidOrder.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ReasonTooShort",
			orderID: paidOrder.ID,
			body:    gin.H{"reason": "x"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/merchant/orders/%d/reject", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOrderStatsAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherUser, _ := randomUser(t)

	stats := db.GetOrderStatsRow{
		PendingCount:    2,
		PaidCount:       5,
		PreparingCount:  3,
		ReadyCount:      1,
		DeliveringCount: 2,
		CompletedCount:  10,
		CancelledCount:  1,
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(stats, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotAMerchant",
			query: "start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherUser.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "start_date=2025/01/01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "DateRangeExceeds90Days",
			query: "start_date=2025-01-01&end_date=2025-06-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "StartDateAfterEndDate",
			query: "start_date=2025-02-01&end_date=2025-01-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders/stats?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// generateManyItems 生成指定数量的订单项（使用同一个dish_id）
func generateManyItems(dishID int64, count int) []gin.H {
	items := make([]gin.H, count)
	for i := 0; i < count; i++ {
		items[i] = gin.H{
			"dish_id":  dishID,
			"quantity": 1,
		}
	}
	return items
}

// generateCustomizations 生成指定数量的定制项
func generateCustomizations(count int) []gin.H {
	customizations := make([]gin.H, count)
	for i := 0; i < count; i++ {
		customizations[i] = gin.H{
			"name":  fmt.Sprintf("Option%d", i+1),
			"value": fmt.Sprintf("Value%d", i+1),
		}
	}
	return customizations
}

// generateManyCustomizations 生成指定数量的定制项
func generateManyCustomizations(count int) []gin.H {
	return generateCustomizations(count)
}

// generateLongString 生成指定长度的字符串
func generateLongString(length int) string {
	return strings.Repeat("a", length)
}

// ==================== 用户端订单操作测试 ====================

func TestUrgeOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK_Paid",
			orderID: order.ID,
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
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Preparing",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(preparingOrder, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Delivering",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "delivering"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), order.ID).
					Times(1).
					Return(db.Delivery{}, pgx.ErrNoRows) // 无骑手信息
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToUser",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidStatus_Pending",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				pendingOrder := order
				pendingOrder.Status = "pending"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(pendingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "InvalidStatus_Completed",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(completedOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/orders/%d/urge", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestConfirmOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.OrderType = "takeout"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "delivering"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)

				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					UpdateOrderToCompleted(gomock.Any(), order.ID).
					Times(1).
					Return(completedOrder, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToUser",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "delivering"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NotTakeoutOrder",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				dineInOrder := order
				dineInOrder.OrderType = "dine_in"
				dineInOrder.Status = "delivering"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(dineInOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "NotDeliveringStatus",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/orders/%d/confirm", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ==================== 商户端订单操作测试 ====================

func TestGetMerchantOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	customer, _ := randomUser(t)

	order := randomOrder(customer.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).
					Times(1).
					Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherMerchantOwner.ID).
					Times(1).
					Return(otherMerchant, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NotAMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, customer.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), customer.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestMarkOrderReadyAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	customer, _ := randomUser(t)

	preparingOrder := randomOrder(customer.ID, merchant.ID)
	preparingOrder.Status = "preparing"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(preparingOrder, nil)

				readyOrder := preparingOrder
				readyOrder.Status = "ready"
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateOrderStatusTxResult{Order: readyOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotPreparing",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				paidOrder := preparingOrder
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherMerchantOwner.ID).
					Times(1).
					Return(otherMerchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/ready", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCompleteOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	customer, _ := randomUser(t)

	readyOrder := randomOrder(customer.ID, merchant.ID)
	readyOrder.Status = "ready"
	readyOrder.OrderType = "dine_in" // 堂食订单

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK_DineIn",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(readyOrder, nil)

				completedOrder := readyOrder
				completedOrder.Status = "completed"
				store.EXPECT().
					CompleteOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteOrderTxResult{Order: completedOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Takeaway",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				takeawayOrder := readyOrder
				takeawayOrder.OrderType = "takeaway"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(takeawayOrder, nil)

				completedOrder := takeawayOrder
				completedOrder.Status = "completed"
				store.EXPECT().
					CompleteOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteOrderTxResult{Order: completedOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TakeoutOrderCannotComplete",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				takeoutOrder := readyOrder
				takeoutOrder.OrderType = "takeout"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(takeoutOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotReady",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				preparingOrder := readyOrder
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), otherMerchantOwner.ID).
					Times(1).
					Return(otherMerchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(readyOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), merchantOwner.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/complete", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCancelOrderAPI_ReasonTooLong(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	// 超过500字符的取消原因
	longReason := generateLongString(501)
	data, err := json.Marshal(gin.H{"reason": longReason})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/orders/%d/cancel", order.ID)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	// 应该返回400，因为reason超过了最大长度
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestListOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	orders := []db.ListOrdersByUserRow{
		{
			ID:           1,
			OrderNo:      "20251210000001",
			UserID:       user.ID,
			MerchantID:   merchant.ID,
			MerchantName: "测试商户",
			OrderType:    "takeaway",
			Subtotal:     1000,
			TotalAmount:  1000,
			Status:       "pending",
			CreatedAt:    time.Now(),
		},
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListOrdersByUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(orders, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "page_id=1&page_size=10&status=paid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListOrdersByUserAndStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListOrdersByUserAndStatusRow{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			query:     "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/orders?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ==================== 优惠券和余额支付测试 ====================

func TestCreateOrderWithVoucherAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	dish := randomDish(merchant.ID, nil)
	dish.Price = 5000 // 设置为50元，5个就是250元，满足100元最低消费

	// 创建用户优惠券（默认允许所有订单类型）
	userVoucher := db.GetUserVoucherRow{
		ID:                1,
		VoucherID:         1,
		UserID:            user.ID,
		Status:            "unused",
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		MerchantID:        merchant.ID,
		Code:              "VOUCHER001",
		Name:              "满100减10",
		Amount:            1000,                                                      // 10元
		MinOrderAmount:    10000,                                                     // 最低100元
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"}, // 默认允许所有
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "WithVoucher_OK",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5, // 确保金额>=100
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 获取用户优惠券
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(userVoucher, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证优惠券参数传递正确
						require.NotNil(t, arg.UserVoucherID)
						require.Equal(t, userVoucher.ID, *arg.UserVoucherID)
						require.Equal(t, userVoucher.Amount, arg.VoucherAmount)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "takeaway",
								Subtotal:      dish.Price * 5,
								VoucherAmount: userVoucher.Amount,
								TotalAmount:   dish.Price*5 - userVoucher.Amount,
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "VoucherNotFound",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": int64(9999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetUserVoucher(gomock.Any(), int64(9999)).
					Times(1).
					Return(db.GetUserVoucherRow{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "VoucherNotOwnedByUser",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回其他用户的优惠券
				otherUserVoucher := userVoucher
				otherUserVoucher.UserID = user.ID + 999
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(otherUserVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "VoucherAlreadyUsed",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				usedVoucher := userVoucher
				usedVoucher.Status = "used"
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(usedVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherExpired",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				expiredVoucher := userVoucher
				expiredVoucher.ExpiresAt = time.Now().Add(-1 * time.Hour) // 已过期
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(expiredVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherWrongMerchant",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				wrongMerchantVoucher := userVoucher
				wrongMerchantVoucher.MerchantID = merchant.ID + 999 // 其他商户
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(wrongMerchantVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherMinOrderNotMet",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1, // 金额不足100元
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 设置菜品价格为50元
				lowPriceDish := dish
				lowPriceDish.Price = 5000 // 50元
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(lowPriceDish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(userVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherAmountExceedsOrder_TotalBecomesZero",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 菜品价格很低，5个只有100元
				lowPriceDish := dish
				lowPriceDish.Price = 2000 // 20元
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(lowPriceDish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 优惠券金额大于订单金额（满100减200）
				largeVoucher := userVoucher
				largeVoucher.Amount = 20000         // 200元优惠
				largeVoucher.MinOrderAmount = 10000 // 最低100元
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(largeVoucher, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证订单总额为0（不能为负）
						require.Equal(t, int64(0), arg.CreateOrderParams.TotalAmount)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "takeaway",
								Subtotal:      2000 * 5,
								VoucherAmount: 20000,
								TotalAmount:   0, // 优惠后为0
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "VoucherOrderTypeNotAllowed_TakeoutVoucherOnDineIn",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "dine_in", // 堂食订单
				"table_id":        int64(1),
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 堂食订单需要验证桌台
				store.EXPECT().
					GetTable(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Table{
						ID:         1,
						MerchantID: merchant.ID,
						Status:     "available",
					}, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回仅限外卖的优惠券
				takeoutOnlyVoucher := userVoucher
				takeoutOnlyVoucher.AllowedOrderTypes = []string{"takeout"} // 仅限外卖
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(takeoutOnlyVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				// 验证错误信息
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response["error"], "不适用于此订单类型")
			},
		},
		{
			name: "VoucherOrderTypeAllowed_DineInVoucherOnDineIn",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "dine_in", // 堂食订单
				"table_id":        int64(1),
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回允许堂食和外带的优惠券（店内券）
				dineInVoucher := userVoucher
				dineInVoucher.AllowedOrderTypes = []string{"dine_in", "takeaway"} // 店内券
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(dineInVoucher, nil)

				store.EXPECT().
					GetTable(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Table{
						ID:         1,
						MerchantID: merchant.ID,
						Status:     "available",
					}, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						require.NotNil(t, arg.UserVoucherID)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "dine_in",
								Subtotal:      dish.Price * 5,
								VoucherAmount: dineInVoucher.Amount,
								TotalAmount:   dish.Price*5 - dineInVoucher.Amount,
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCreateOrderWithBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	dish := randomDish(merchant.ID, nil)
	table := randomTable(merchant.ID)

	// 创建会员卡
	membership := db.MerchantMembership{
		ID:             1,
		MerchantID:     merchant.ID,
		UserID:         user.ID,
		Balance:        50000, // 500元余额
		TotalRecharged: 50000,
		TotalConsumed:  0,
	}

	// 会员设置
	membershipSettings := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "WithBalance_DineIn_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 获取会员卡
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					}).
					Times(1).
					Return(membership, nil)

				// 获取会员设置
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证余额支付参数
						require.NotNil(t, arg.MembershipID)
						require.Equal(t, membership.ID, *arg.MembershipID)
						require.True(t, arg.BalancePaid > 0)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:          1,
								OrderNo:     "20240101120000123456",
								UserID:      user.ID,
								MerchantID:  merchant.ID,
								OrderType:   "dine_in",
								Subtotal:    dish.Price * 2,
								BalancePaid: dish.Price * 2,
								TotalAmount: dish.Price * 2,
								Status:      "pending",
								CreatedAt:   time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "WithBalance_Takeout_Rejected",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  int64(1),
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				region := randomRegion()
				address := randomUserAddress(user.ID, region.ID)
				store.EXPECT().
					GetUserAddress(gomock.Any(), int64(1)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 配送费计算相关mock
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), region.ID).
					Times(1).
					Return(db.DeliveryFeeConfig{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				// 验证错误信息
				var response map[string]string
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response["error"], "外卖和预定订单暂不支持余额支付")
			},
		},
		{
			name: "WithBalance_NotMember",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 用户不是会员
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantMembership{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WithBalance_InsufficientBalance",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 余额为0
				zeroBalanceMembership := membership
				zeroBalanceMembership.Balance = 0
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(zeroBalanceMembership, nil)

				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WithBalance_SceneNotAllowed",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(membership, nil)

				// 设置不允许堂食使用余额
				restrictedSettings := membershipSettings
				restrictedSettings.BalanceUsableScenes = []string{"takeaway"} // 只允许自提
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(restrictedSettings, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WithBalance_PartialPayment_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				// 设置固定价格的菜品：200元/个，2个=400元
				fixedPriceDish := dish
				fixedPriceDish.Price = 20000 // 200元
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(fixedPriceDish, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 余额只有100元，不够支付400元的订单
				partialBalanceMembership := membership
				partialBalanceMembership.Balance = 10000 // 100元
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(partialBalanceMembership, nil)

				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证部分余额支付：订单400元，余额100元，只用100元
						require.NotNil(t, arg.MembershipID)
						require.Equal(t, int64(10000), arg.BalancePaid) // 只使用100元余额
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:          1,
								OrderNo:     "20240101120000123456",
								UserID:      user.ID,
								MerchantID:  merchant.ID,
								OrderType:   "dine_in",
								Subtotal:    40000, // 200*2
								BalancePaid: 10000,
								TotalAmount: 40000,
								Status:      "pending",
								CreatedAt:   time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCreateOrderWithVoucherAndBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	dish := randomDish(merchant.ID, nil)
	dish.Price = 20000 // 200元
	table := randomTable(merchant.ID)

	// 创建用户优惠券（允许堂食）
	userVoucher := db.GetUserVoucherRow{
		ID:                1,
		VoucherID:         1,
		UserID:            user.ID,
		Status:            "unused",
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		MerchantID:        merchant.ID,
		Code:              "VOUCHER001",
		Name:              "满100减10",
		Amount:            1000,                                                      // 10元
		MinOrderAmount:    10000,                                                     // 最低100元
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"}, // 允许所有类型
	}

	// 创建会员卡
	membership := db.MerchantMembership{
		ID:             1,
		MerchantID:     merchant.ID,
		UserID:         user.ID,
		Balance:        50000, // 500元余额
		TotalRecharged: 50000,
		TotalConsumed:  0,
	}

	membershipSettings := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	t.Run("VoucherAndBalance_Combined", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)

		store.EXPECT().
			GetMerchant(gomock.Any(), merchant.ID).
			Times(1).
			Return(merchant, nil)

		store.EXPECT().
			GetTable(gomock.Any(), table.ID).
			Times(1).
			Return(table, nil)

		store.EXPECT().
			GetDish(gomock.Any(), dish.ID).
			Times(1).
			Return(dish, nil)

		store.EXPECT().
			ListActiveDiscountRules(gomock.Any(), merchant.ID).
			Times(1).
			Return([]db.DiscountRule{}, nil)

		store.EXPECT().
			GetUserVoucher(gomock.Any(), userVoucher.ID).
			Times(1).
			Return(userVoucher, nil)

		store.EXPECT().
			GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
			Times(1).
			Return(membership, nil)

		store.EXPECT().
			GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
			Times(1).
			Return(membershipSettings, nil)

		store.EXPECT().
			CreateOrderTx(gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
				// 验证同时使用优惠券和余额
				require.NotNil(t, arg.UserVoucherID)
				require.Equal(t, userVoucher.ID, *arg.UserVoucherID)
				require.Equal(t, userVoucher.Amount, arg.VoucherAmount)
				require.NotNil(t, arg.MembershipID)
				require.Equal(t, membership.ID, *arg.MembershipID)
				// 订单金额200元 - 优惠券10元 = 190元，余额500元足够
				require.Equal(t, int64(19000), arg.BalancePaid)
				return db.CreateOrderTxResult{
					Order: db.Order{
						ID:            1,
						OrderNo:       "20240101120000123456",
						UserID:        user.ID,
						MerchantID:    merchant.ID,
						OrderType:     "dine_in",
						Subtotal:      dish.Price,
						VoucherAmount: userVoucher.Amount,
						BalancePaid:   19000,
						TotalAmount:   dish.Price - userVoucher.Amount,
						Status:        "pending",
						CreatedAt:     time.Now(),
					},
				}, nil
			})

		server := newTestServer(t, store)
		recorder := httptest.NewRecorder()

		body := gin.H{
			"merchant_id":     merchant.ID,
			"order_type":      "dine_in",
			"table_id":        table.ID,
			"user_voucher_id": userVoucher.ID,
			"use_balance":     true,
			"items": []gin.H{
				{
					"dish_id":  dish.ID,
					"quantity": 1,
				},
			},
		}
		data, err := json.Marshal(body)
		require.NoError(t, err)

		url := "/v1/orders"
		request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		require.NoError(t, err)

		addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
		server.router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}
