package api

import (
	"bytes"
	"context"
	"encoding/json"
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

func expectActiveRiderBaofuAccountForDelivery(store *mockdb.MockStore, riderID int64) {
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   riderID,
		}).
		Times(1).
		Return(db.BaofuAccountBinding{
			OwnerType:    db.BaofuAccountOwnerTypeRider,
			OwnerID:      riderID,
			AccountType:  db.BaofuAccountTypePersonal,
			OpenState:    db.BaofuAccountOpenStateActive,
			ContractNo:   pgtype.Text{String: "CP123", Valid: true},
			SharingMerID: pgtype.Text{String: "CP123", Valid: true},
		}, nil)
}

func TestGetRecommendedOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true
	rider.CurrentLongitude = numericFromFloat(116.410)
	rider.CurrentLatitude = numericFromFloat(39.920)
	rider.LocationUpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	rider.Status = "active"

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
					AnyTimes().
					Return(rider, nil)

				store.EXPECT().
					GetRiderProfile(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)

				// GetActiveRecommendConfig - may return error for default config
				store.EXPECT().
					GetActiveRecommendConfig(gomock.Any()).
					Times(1).
					Return(db.RecommendConfig{}, db.ErrRecordNotFound)

				// ListDeliveryPoolNearby - 仅按位置过滤
				store.EXPECT().
					ListDeliveryPoolNearby(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDeliveryPoolNearbyRow{}, nil)

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
					AnyTimes().
					Return(rider, nil)

				store.EXPECT().
					GetRiderProfile(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
				expectActiveRiderBaofuAccountForDelivery(store, rider.ID)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(pool, nil)

				merchant := db.Merchant{
					ID:          merchantID,
					OwnerUserID: util.RandomInt(1, 1000),
					RegionID:    util.RandomInt(1, 1000),
					Name:        util.RandomString(10),
				}
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchantID)).
					Times(2).
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
					DoAndReturn(func(_ context.Context, arg db.GrabOrderTxParams) (db.GrabOrderTxResult, error) {
						require.NotNil(t, arg.ProfitSharingRiderBill)
						return db.GrabOrderTxResult{Delivery: delivery, Order: db.Order{ID: orderID, Status: db.OrderStatusCourierAccepted}}, nil
					})

				// Mock for notification - GetOrder
				order := db.Order{
					ID:          orderID,
					UserID:      util.RandomInt(1, 1000),
					MerchantID:  merchantID,
					OrderNo:     util.RandomString(10),
					Status:      db.OrderStatusReady,
					OrderType:   db.OrderTypeTakeout,
					TotalAmount: 10000,
					DeliveryFee: 500,
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)
				paymentOrder := db.PaymentOrder{
					ID:                    util.RandomInt(1000, 2000),
					OrderID:               pgtype.Int8{Int64: orderID, Valid: true},
					BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
					PaymentChannel:        db.PaymentChannelBaofuAggregate,
					RequiresProfitSharing: true,
					Status:                "paid",
					Amount:                10000,
				}
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{OrderID: pgtype.Int8{Int64: orderID, Valid: true}, BusinessType: db.ExternalPaymentBusinessOwnerOrder}).
					Times(2).
					Return(paymentOrder, nil)
				store.EXPECT().
					GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
					Times(2).
					Return(db.ProfitSharingOrder{
						ID:                           7001,
						PaymentOrderID:               paymentOrder.ID,
						MerchantID:                   merchantID,
						OrderSource:                  db.OrderTypeTakeout,
						TotalAmount:                  paymentOrder.Amount,
						DeliveryFee:                  500,
						PlatformRate:                 200,
						OperatorRate:                 300,
						Status:                       db.ProfitSharingOrderStatusPending,
						Provider:                     db.ExternalPaymentProviderBaofu,
						Channel:                      db.PaymentChannelBaofuAggregate,
						MerchantSharingMerID:         pgtype.Text{String: "MER_SHARE", Valid: true},
						OperatorID:                   pgtype.Int8{Int64: 9001, Valid: true},
						OperatorSharingMerID:         pgtype.Text{String: "OP_SHARE", Valid: true},
						PlatformSharingMerID:         pgtype.Text{String: "PLATFORM_SHARE", Valid: true},
						ProviderPaymentFee:           30,
						ProviderPaymentFeeRateBps:    30,
						ProviderPaymentFeeBaseAmount: paymentOrder.Amount,
						ProviderPaymentFeeSource:     logic.BaofuProviderPaymentFeeSourceEstimated,
						MerchantPaymentFeeRateBps:    logic.DefaultBaofuPaymentServiceFeeRateBps,
						RiderGrossAmount:             500,
						RiderPaymentFee:              3,
						RiderAmount:                  497,
					}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
				store.EXPECT().
					UpdateDeliveryEstimatedTime(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.Delivery{}, nil)

				// Mock for final GetDelivery to return updated delivery
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Any()).
					Times(1).
					Return(delivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var body map[string]any
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &body)
				require.Equal(t, float64(500), body["rider_gross_amount"])
				require.Equal(t, float64(3), body["rider_payment_fee"])
				require.Equal(t, float64(497), body["rider_net_earnings"])
				require.Equal(t, float64(497), body["rider_earnings"])
				require.Equal(t, float64(7001), body["profit_sharing_order_id"])
				require.Equal(t, db.ProfitSharingOrderStatusPending, body["profit_sharing_status"])
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
					AnyTimes().
					Return(rider, nil)

				store.EXPECT().
					GetRiderProfile(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
				expectActiveRiderBaofuAccountForDelivery(store, rider.ID)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(db.DeliveryPool{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "MerchantNotAccepted",
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
					AnyTimes().
					Return(rider, nil)

				store.EXPECT().
					GetRiderProfile(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
				expectActiveRiderBaofuAccountForDelivery(store, rider.ID)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(pool, nil)

				merchant := db.Merchant{
					ID:          merchantID,
					OwnerUserID: util.RandomInt(1, 1000),
					RegionID:    util.RandomInt(1, 1000),
					Name:        util.RandomString(10),
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

				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(db.Order{
						ID:         orderID,
						UserID:     util.RandomInt(1, 1000),
						MerchantID: merchantID,
						OrderNo:    util.RandomString(10),
						Status:     db.OrderStatusPaid,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "商户未接单，暂不可抢单")
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

				// Mock for notification
				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
					Status:     "courier_accepted",
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
				store.EXPECT().
					UpdateDeliveryToPickedTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateDeliveryToPickedTxResult{Delivery: delivery}, nil)

				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
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

				// 代取单属于另一个骑手
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

				// 代取单状态不正确（assigned而不是picking）
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
					Return(db.Delivery{}, db.ErrRecordNotFound)
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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
	riderUser, _ := randomUser(t)
	rider := randomRider(riderUser.ID)
	orderID := util.RandomInt(1, 1000)
	deliveryID := util.RandomInt(1, 1000)

	order := db.Order{
		ID:         orderID,
		UserID:     user.ID,
		MerchantID: util.RandomInt(1, 1000),
		OrderNo:    util.RandomString(10),
	}

	delivery := randomDelivery(orderID, rider.ID)
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
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)

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
			name:    "OK_AssignedRider",
			orderID: orderID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, riderUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(delivery, nil)

				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(riderUser.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NotOrderOwnerOrAssignedRider",
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
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(delivery, nil)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
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
					Return(db.Order{}, db.ErrRecordNotFound)
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
					Return(db.Delivery{}, db.ErrRecordNotFound)
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
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
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
					Return(db.Delivery{}, db.ErrRecordNotFound)
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
					UpdateDeliveryToPickupTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateDeliveryToPickupTxResult{Delivery: pickingDelivery}, nil)

				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
					Status:     "courier_accepted",
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
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

				// 代取单属于另一个骑手
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
					Return(db.Delivery{}, db.ErrRecordNotFound)
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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
				order := db.Order{
					ID:         orderID,
					UserID:     util.RandomInt(1, 1000),
					MerchantID: util.RandomInt(1, 1000),
					OrderNo:    util.RandomString(10),
					Status:     "picked",
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
				store.EXPECT().
					UpdateDeliveryToDeliveringTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateDeliveryToDeliveringTxResult{Delivery: deliveringDelivery}, nil)

				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
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
		{
			name:       "BadRequest_InvalidStatus",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				invalidDelivery := delivery
				invalidDelivery.Status = "delivering"
				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(invalidDelivery, nil)
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
	rider.CurrentLongitude = numericFromFloat(116.410)
	rider.CurrentLatitude = numericFromFloat(39.920)
	rider.LocationUpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

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
					Status:     "delivering",
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(2).
					Return(order, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(order.MerchantID)).
					Times(1).
					Return(db.Merchant{ID: order.MerchantID, Name: util.RandomString(10)}, nil)

				store.EXPECT().
					CountOrderItems(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListOrderItemsByOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return([]db.OrderItem{}, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
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
		{
			name:       "MissingRiderLocation",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				missingLocationRider := rider
				missingLocationRider.CurrentLongitude = pgtype.Numeric{}
				missingLocationRider.CurrentLatitude = pgtype.Numeric{}

				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(missingLocationRider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "骑手定位缺失，无法确认送达，请先刷新定位")
			},
		},
		{
			name:       "StaleRiderLocation",
			deliveryID: deliveryID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staleRider := rider
				staleRider.LocationUpdatedAt = pgtype.Timestamptz{Time: time.Now().Add(-3 * time.Minute), Valid: true}

				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(staleRider, nil)

				store.EXPECT().
					GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
					Times(1).
					Return(delivery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "骑手定位已过期，无法确认送达，请刷新定位后重试")
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
	rider.DepositAmount = 100 * fenPerYuan
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
				lowDepositRider.DepositAmount = 10 * fenPerYuan
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					AnyTimes().
					Return(lowDepositRider, nil)

				store.EXPECT().
					GetRiderProfile(gomock.Any(), gomock.Eq(lowDepositRider.ID)).
					Times(1).
					Return(db.RiderProfile{RiderID: lowDepositRider.ID, IsSuspended: false}, nil)
				expectActiveRiderBaofuAccountForDelivery(store, lowDepositRider.ID)

				store.EXPECT().
					GetDeliveryPoolByOrderID(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(pool, nil)

				merchant := db.Merchant{
					ID:          merchantID,
					OwnerUserID: util.RandomInt(1, 1000),
					RegionID:    util.RandomInt(1, 1000),
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

				order := db.Order{
					ID:          orderID,
					UserID:      util.RandomInt(1, 1000),
					MerchantID:  merchantID,
					OrderNo:     util.RandomString(10),
					Status:      db.OrderStatusReady,
					TotalAmount: 5000,
				}
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(orderID)).
					Times(1).
					Return(order, nil)
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
