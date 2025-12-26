package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomDelivery(orderID, riderID int64) db.Delivery {
	return db.Delivery{
		ID:                util.RandomInt(1, 1000),
		OrderID:           orderID,
		RiderID:           pgtype.Int8{Int64: riderID, Valid: true},
		PickupAddress:     "北京市朝阳区某商家",
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		DeliveryAddress:   "北京市朝阳区某小区",
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		Distance:          2500,
		DeliveryFee:       500,
		RiderEarnings:     400,
		Status:            "assigned",
		CreatedAt:         time.Now(),
	}
}

func randomDeliveryPool(orderID, merchantID int64) db.DeliveryPool {
	return db.DeliveryPool{
		ID:                util.RandomInt(1, 1000),
		OrderID:           orderID,
		MerchantID:        merchantID,
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		Distance:          2500,
		DeliveryFee:       500,
		ExpectedPickupAt:  time.Now().Add(30 * time.Minute),
		ExpiresAt:         time.Now().Add(10 * time.Minute),
		Priority:          1,
		CreatedAt:         time.Now(),
	}
}

func TestGetRecommendedOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true
	rider.Status = "active"
	rider.RegionID = pgtype.Int8{Int64: 1, Valid: true} // 设置骑手区域

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "longitude=116.404&latitude=39.915",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// GetActiveRecommendConfig - may return error for default config
				store.EXPECT().
					GetActiveRecommendConfig(gomock.Any()).
					Times(1).
					Return(db.RecommendConfig{}, pgx.ErrNoRows)

				// ListDeliveryPoolNearbyByRegion - 按区域过滤
				store.EXPECT().
					ListDeliveryPoolNearbyByRegion(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDeliveryPoolNearbyByRegionRow{}, nil)

				// ListRiderActiveDeliveries
				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotOnline",
			query: "longitude=116.404&latitude=39.915",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				offlineRider := rider
				offlineRider.IsOnline = false
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(offlineRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MissingLocation",
			query: "longitude=116.404", // missing latitude
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
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

			url := "/v1/delivery/recommend?" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGrabOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true
	rider.Status = "active"
	rider.RegionID = pgtype.Int8{Int64: 1, Valid: true} // 设置骑手区域

	orderID := util.RandomInt(1, 1000)
	merchantID := util.RandomInt(1, 1000)
	pool := randomDeliveryPool(orderID, merchantID)

	testCases := []struct {
		name          string
		orderID       int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(pool, nil)

				// 高值单资格积分检查
				store.EXPECT().
					GetRiderPremiumScore(gomock.Any(), rider.ID).
					Times(1).
					Return(int16(0), nil)

				// 区域检查：获取商家确认区域匹配
				merchant := db.Merchant{
					ID:          merchantID,
					OwnerUserID: util.RandomInt(1, 1000),
					RegionID:    rider.RegionID.Int64, // 匹配骑手区域
				}
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchantID)).
					Times(1).
					Return(merchant, nil)

				existingDelivery := randomDelivery(orderID, 0)
				existingDelivery.RiderID = pgtype.Int8{Valid: false}
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(existingDelivery, nil)

				delivery := randomDelivery(orderID, rider.ID)
				store.EXPECT().
					GrabOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GrabOrderTxResult{Delivery: delivery}, nil)

				// Mock for notification - GetOrder and GetMerchant (second call)
				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: merchantID,
					OrderNo:    util.RandomString(10),
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchantID)).
					Times(1).
					Return(merchant, nil)

				// Mock for final GetDelivery to return updated delivery
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Any()).
					Times(1).
					Return(delivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(db.DeliveryPool{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotOnline",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				offlineRider := rider
				offlineRider.IsOnline = false
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(offlineRider, nil)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/delivery/grab/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestConfirmPickupAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true

	deliveryID := util.RandomInt(1, 1000)
	orderID := util.RandomInt(1, 1000)
	delivery := randomDelivery(orderID, rider.ID)
	delivery.ID = deliveryID
	delivery.Status = "picking"

	testCases := []struct {
		name          string
		deliveryID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 新增: GetDelivery 用于状态和归属检查
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				store.EXPECT().
					UpdateDeliveryToPicked(gomock.Any(), gomock.Any()).
					Times(1).
					Return(delivery, nil)

				// Mock for notification
				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "NotOwner",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 配送单属于另一个骑手
				otherRiderDelivery := delivery
				otherRiderDelivery.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(otherRiderDelivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "WrongStatus",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 配送单状态不正确（assigned而不是picking）
				wrongStatusDelivery := delivery
				wrongStatusDelivery.Status = "assigned"
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(wrongStatusDelivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "DeliveryNotFound",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(db.Delivery{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", tc.deliveryID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMyActiveDeliveriesAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)

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
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotARider",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
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

			url := "/v1/delivery/active"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
func TestGetDeliveryByOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	orderID := util.RandomInt(1, 1000)
	deliveryID := util.RandomInt(1, 1000)

	order := db.Order{
		ID:         orderID,
		UserID:     user.ID,
		MerchantID: util.RandomInt(1, 1000),
		OrderNo:    util.RandomString(10),
	}

	delivery := randomDelivery(orderID, 0)
	delivery.ID = deliveryID

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: orderID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(delivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NotOrderOwner",
			orderID: orderID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 订单属于另一个用户
				otherUserOrder := order
				otherUserOrder.UserID = user.ID + 1
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(otherUserOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: orderID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "DeliveryNotFound",
			orderID: orderID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(db.Delivery{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/delivery/order/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetDeliveryTrackAPI(t *testing.T) {
	user, _ := randomUser(t)
	orderID := util.RandomInt(1, 1000)
	deliveryID := util.RandomInt(1, 1000)
	riderID := util.RandomInt(1, 1000)

	order := db.Order{
		ID:         orderID,
		UserID:     user.ID,
		MerchantID: util.RandomInt(1, 1000),
		OrderNo:    util.RandomString(10),
	}

	delivery := randomDelivery(orderID, riderID)
	delivery.ID = deliveryID

	testCases := []struct {
		name          string
		deliveryID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK_OrderOwner",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					ListDeliveryLocations(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.RiderLocation{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "Forbidden_NotOwnerNotRider",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				// 订单属于另一个用户
				otherUserOrder := order
				otherUserOrder.UserID = user.ID + 1
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(otherUserOrder, nil)

				// 当前用户也不是骑手
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "DeliveryNotFound",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(db.Delivery{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/delivery/%d/track", tc.deliveryID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestStartPickupAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true

	deliveryID := util.RandomInt(1, 1000)
	orderID := util.RandomInt(1, 1000)
	delivery := randomDelivery(orderID, rider.ID)
	delivery.ID = deliveryID
	delivery.Status = "assigned"

	testCases := []struct {
		name          string
		deliveryID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				pickingDelivery := delivery
				pickingDelivery.Status = "picking"
				store.EXPECT().
					UpdateDeliveryToPickup(gomock.Any(), gomock.Any()).
					Times(1).
					Return(pickingDelivery, nil)

				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "WrongStatus_AlreadyPicking",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				wrongStatusDelivery := delivery
				wrongStatusDelivery.Status = "picking"
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(wrongStatusDelivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NotOwner",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 配送单属于另一个骑手
				otherRiderDelivery := delivery
				otherRiderDelivery.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(otherRiderDelivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "DeliveryNotFound",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(db.Delivery{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "NotARider",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/delivery/%d/start-pickup", tc.deliveryID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestStartDeliveryAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true

	deliveryID := util.RandomInt(1, 1000)
	orderID := util.RandomInt(1, 1000)
	delivery := randomDelivery(orderID, rider.ID)
	delivery.ID = deliveryID
	delivery.Status = "picked"

	testCases := []struct {
		name          string
		deliveryID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				deliveringDelivery := delivery
				deliveringDelivery.Status = "delivering"
				store.EXPECT().
					UpdateDeliveryToDelivering(gomock.Any(), gomock.Any()).
					Times(1).
					Return(deliveringDelivery, nil)

				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "WrongStatus_NotPicked",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				wrongStatusDelivery := delivery
				wrongStatusDelivery.Status = "picking"
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(wrongStatusDelivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NotOwner",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				otherRiderDelivery := delivery
				otherRiderDelivery.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(otherRiderDelivery, nil)
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

			url := fmt.Sprintf("/v1/delivery/%d/start-delivery", tc.deliveryID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestConfirmDeliveryAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true

	deliveryID := util.RandomInt(1, 1000)
	orderID := util.RandomInt(1, 1000)
	delivery := randomDelivery(orderID, rider.ID)
	delivery.ID = deliveryID
	delivery.Status = "delivering"

	testCases := []struct {
		name          string
		deliveryID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)

				deliveredDelivery := delivery
				deliveredDelivery.Status = "delivered"
				store.EXPECT().
					CompleteDeliveryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteDeliveryTxResult{Delivery: deliveredDelivery}, nil)

				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "NotOwner",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				otherRiderDelivery := delivery
				otherRiderDelivery.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(otherRiderDelivery, nil)
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

			url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", tc.deliveryID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGrabOrderAPI_EdgeCases(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true
	rider.Status = "active"
	rider.RegionID = pgtype.Int8{Int64: 1, Valid: true}
	rider.DepositAmount = 10000 // 100元
	rider.FrozenDeposit = 0

	orderID := util.RandomInt(1, 1000)
	merchantID := util.RandomInt(1, 1000)
	pool := randomDeliveryPool(orderID, merchantID)

	testCases := []struct {
		name          string
		orderID       int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "InsufficientDeposit",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				lowDepositRider := rider
				lowDepositRider.DepositAmount = 1000 // 只有10元
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(lowDepositRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "RegionMismatch",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(pool, nil)

				// 高值单资格积分检查
				store.EXPECT().
					GetRiderPremiumScore(gomock.Any(), rider.ID).
					Times(1).
					Return(int16(0), nil)

				// 商户在不同区域
				merchant := db.Merchant{
					ID:          merchantID,
					OwnerUserID: util.RandomInt(1, 1000),
					RegionID:    rider.RegionID.Int64 + 1, // 不同区域
				}
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchantID)).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NoRegionAssigned",
			orderID: orderID,
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				noRegionRider := rider
				noRegionRider.RegionID = pgtype.Int8{Valid: false}
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(noRegionRider, nil)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/delivery/grab/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}