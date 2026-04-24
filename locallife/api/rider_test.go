package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomRider(userID int64) db.Rider {
	return db.Rider{
		ID:            util.RandomInt(1, 1000),
		UserID:        userID,
		RealName:      util.RandomString(6),
		Phone:         "138" + util.RandomString(8),
		IDCardNo:      util.RandomString(18),
		Status:        "active",
		DepositAmount: 30000, // 300元
		FrozenDeposit: 0,
		IsOnline:      false,
		RegionID:      pgtype.Int8{Int64: 1, Valid: true},
		TotalOrders:   0,
		TotalEarnings: 0,
		CreatedAt:     time.Now(),
	}
}

func expectRiderThresholdFromOperator(store *mockdb.MockStore, rider db.Rider, threshold int64) {
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), rider.RegionID.Int64).
		Times(1).
		Return(db.Operator{ID: 1, RiderDeposit: threshold}, nil)
	if threshold == db.DefaultRiderDepositThresholdFen {
		store.EXPECT().
			GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
				ConfigKey: db.PlatformConfigKeyRiderDepositFen,
				ScopeType: db.PlatformConfigScopeOperator,
				ScopeID:   pgtype.Int8{Int64: 1, Valid: true},
			}).
			Times(1).
			Return(db.PlatformConfig{}, db.ErrRecordNotFound)
		store.EXPECT().
			GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
				ConfigKey: db.PlatformConfigKeyRiderDepositFen,
				ScopeType: db.PlatformConfigScopeGlobal,
				ScopeID:   pgtype.Int8{Valid: false},
			}).
			Times(1).
			Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	}
}

func TestGetRiderMeAPI(t *testing.T) {
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotFound",
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

			url := "/v1/rider/me"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGoOnlineAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"
	rider.DepositAmount = 30000 // 押金足够

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
				expectRiderThresholdFromOperator(store, rider, db.DefaultRiderDepositThresholdFen)

				updatedRider := rider
				updatedRider.IsOnline = true
				store.EXPECT().
					UpdateRiderOnlineStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_ApprovedRider",
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				approvedRider := rider
				approvedRider.Status = db.RiderStatusApproved
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(approvedRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InsufficientDeposit",
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				insufficientRider := rider
				insufficientRider.DepositAmount = 0
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(insufficientRider, nil)
				expectRiderThresholdFromOperator(store, insufficientRider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotApproved",
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				pendingRider := rider
				pendingRider.Status = "pending"
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(pendingRider, nil)
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
			paymentClient := wechatmock.NewMockDirectPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			server := newTestServerWithPayment(t, store, paymentClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/online"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGoOnlineAPI_UsesConfiguredDepositThreshold(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = db.RiderStatusActive
	rider.DepositAmount = 25000
	rider.RegionID = pgtype.Int8{Int64: 66, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	thresholdConfig, err := json.Marshal(depositConfigValue{AmountFen: 26000})
	require.NoError(t, err)

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), int64(66)).
		Times(1).
		Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PlatformConfig{ConfigValue: thresholdConfig}, nil)
	store.EXPECT().
		UpdateRiderOnlineStatus(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPost, "/v1/rider/online", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGoOfflineAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.IsOnline = true

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

				// Check no active deliveries
				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				offlineRider := rider
				offlineRider.IsOnline = false
				store.EXPECT().
					UpdateRiderOnlineStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(offlineRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "HasActiveDeliveries",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// Has active delivery
				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{{ID: 1}}, nil)
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

			url := "/v1/rider/offline"
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDepositRiderAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"amount": 10000, // 100元
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 幂等检查：无已有 pending 支付单
				store.EXPECT().
					GetPendingPaymentOrderByUserAndBusinessType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				// 创建支付订单
				store.EXPECT().
					CreatePaymentOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						UserID:       user.ID,
						PaymentType:  "miniprogram",
						BusinessType: "rider_deposit",
						Amount:       100 * fenPerYuan,
						Status:       "pending",
						OutTradeNo:   "test_order_123",
					}, nil)

				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{ID: user.ID, WechatOpenid: "openid_rider_001"}, nil)

				paymentClient.EXPECT().
					CreateJSAPIOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay_rider_deposit_001"}, &wechat.JSAPIPayParams{
						TimeStamp: "1234567890",
						NonceStr:  "nonce-001",
						Package:   "prepay_id=prepay_rider_deposit_001",
						SignType:  "RSA",
						PaySign:   "sign-001",
					}, nil)

				store.EXPECT().
					UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				// 验证返回支付订单信息
				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Contains(t, resp, "payment_order_id")
				require.Contains(t, resp, "out_trade_no")
				require.Contains(t, resp, "pay_params")
			},
		},
		{
			name: "InvalidAmount",
			body: map[string]interface{}{
				"amount": -100, // negative
				"remark": "错误金额",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RiderNotFound",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "RiderNotActive",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				pendingRider := rider
				pendingRider.Status = "pending"
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(pendingRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "PaymentClientNotConfigured",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, CodeServiceUnavail, resp.Code)
				require.Equal(t, "支付服务暂不可用，请稍后重试", resp.Message)
			},
		},
		{
			name: "ClosePendingPaymentOrderFailureReturnsInternalServerError",
			body: map[string]interface{}{
				"amount": 10000,
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
				store.EXPECT().
					GetPendingPaymentOrderByUserAndBusinessType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
				store.EXPECT().
					CreatePaymentOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{
						ID:           2,
						UserID:       user.ID,
						PaymentType:  "miniprogram",
						BusinessType: "rider_deposit",
						Amount:       10000,
						Status:       "pending",
						OutTradeNo:   "test_order_456",
					}, nil)
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{ID: user.ID, WechatOpenid: "openid_rider_001"}, nil)
				paymentClient.EXPECT().
					CreateJSAPIOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil, errors.New("wechat unavailable"))
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), int64(2)).
					Times(1).
					Return(db.PaymentOrder{}, errors.New("close failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, CodeInternalError, resp.Code)
				require.Equal(t, "internal server error", resp.Message)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := wechatmock.NewMockDirectPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			server := newTestServerWithPayment(t, store, paymentClient)
			if tc.name == "PaymentClientNotConfigured" {
				server.directPaymentClient = nil
			}
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/deposit"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetRiderDepositBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.DepositAmount = 500 * fenPerYuan
	rider.FrozenDeposit = 50 * fenPerYuan

	testCases := []struct {
		name          string
		rider         db.Rider
		pendingAmount int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			rider:         rider,
			pendingAmount: int64(20 * fenPerYuan),
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
				store.EXPECT().
					GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(int64(20*fenPerYuan), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp depositBalanceResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(500*fenPerYuan), resp.TotalDeposit)
				require.Equal(t, int64(50*fenPerYuan), resp.FrozenDeposit)
				require.Equal(t, int64(30*fenPerYuan), resp.DeliveryFrozenDeposit)
				require.Equal(t, int64(20*fenPerYuan), resp.WithdrawalProcessingAmount)
				require.Equal(t, int64(450*fenPerYuan), resp.AvailableDeposit)
			},
		},
		{
			name: "PendingWithdrawalStillReducesAvailableWhenFrozenNotUpdated",
			rider: func() db.Rider {
				legacyRider := rider
				legacyRider.FrozenDeposit = 0
				return legacyRider
			}(),
			pendingAmount: int64(100 * fenPerYuan),
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				legacyRider := rider
				legacyRider.FrozenDeposit = 0
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(legacyRider, nil)
				store.EXPECT().
					GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(int64(100*fenPerYuan), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp depositBalanceResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(500*fenPerYuan), resp.TotalDeposit)
				require.Equal(t, int64(0), resp.FrozenDeposit)
				require.Equal(t, int64(0), resp.DeliveryFrozenDeposit)
				require.Equal(t, int64(100*fenPerYuan), resp.WithdrawalProcessingAmount)
				require.Equal(t, int64(400*fenPerYuan), resp.AvailableDeposit)
			},
		},
		{
			name:          "PendingRefundAmountError",
			rider:         rider,
			pendingAmount: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
				store.EXPECT().
					GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(int64(0), errors.New("query failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:          "RiderNotFound",
			rider:         rider,
			pendingAmount: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:          "Unauthorized",
			rider:         rider,
			pendingAmount: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), gomock.Any()).
					Times(0)
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/rider/deposit"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestWithdrawRiderAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"
	rider.DepositAmount = 500 * fenPerYuan
	rider.FrozenDeposit = 0

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 检查活跃配送
				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				store.EXPECT().
					PrepareRiderDepositRefundTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PrepareRiderDepositRefundTxResult{
						Rider: db.Rider{
							ID:            rider.ID,
							DepositAmount: 500 * fenPerYuan,
							FrozenDeposit: 150 * fenPerYuan,
						},
						RefundPlans: []db.RiderDepositRefundPlan{{
							RefundOrder:        db.RefundOrder{ID: 1, PaymentOrderID: 91, RefundAmount: 100 * fenPerYuan, OutRefundNo: "RTEST123"},
							SourcePaymentOrder: db.PaymentOrder{ID: 91, OutTradeNo: "PTEST123", Amount: 100 * fenPerYuan},
						}},
					}, nil)

				paymentClient.EXPECT().
					CreateRefund(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechat.RefundResponse{RefundID: "wx_refund_1", Status: wechat.RefundStatusProcessing}, nil)

				store.EXPECT().
					UpdateRefundOrderToProcessing(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.RefundOrder{ID: 1, RefundAmount: 100 * fenPerYuan, OutRefundNo: "RTEST123", Status: "processing"}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusAccepted, recorder.Code)
				var resp riderWithdrawResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(100*fenPerYuan), resp.RequestedAmount)
				require.Equal(t, int64(100*fenPerYuan), resp.AcceptedAmount)
				require.Equal(t, riderWithdrawProcessingStatus, resp.Status)
				require.Len(t, resp.Refunds, 1)
				require.Equal(t, int64(1), resp.Refunds[0].RefundOrderID)
				require.Equal(t, int64(91), resp.Refunds[0].PaymentOrderID)
				require.Equal(t, "RTEST123", resp.Refunds[0].OutRefundNo)
				require.Equal(t, int64(100*fenPerYuan), resp.Refunds[0].Amount)
				require.Equal(t, riderWithdrawProcessingStatus, resp.Refunds[0].Status)
			},
		},
		{
			name: "AmountTooSmall",
			body: map[string]interface{}{
				"amount": 50, // 小于最小提现金额 100分
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				// 不应该调用任何 store 方法
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AmountTooLarge",
			body: map[string]interface{}{
				"amount": 6000000, // 超过最大提现金额 5000000分
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RiderNotActive",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				pendingRider := rider
				pendingRider.Status = "pending"
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(pendingRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InsufficientBalance",
			body: map[string]interface{}{
				"amount": 1000 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "FrozenDepositBlocked",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				frozenRider := rider
				frozenRider.FrozenDeposit = 50 * fenPerYuan
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(frozenRider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					PrepareRiderDepositRefundTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "HasActiveDeliveries",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{{ID: 1}}, nil) // 有活跃配送
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RefundChannelNotEnough",
			body: map[string]interface{}{
				"amount": 100 * fenPerYuan,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *wechatmock.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				store.EXPECT().
					PrepareRiderDepositRefundTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PrepareRiderDepositRefundTxResult{
						Rider: db.Rider{
							ID:            rider.ID,
							DepositAmount: 500 * fenPerYuan,
							FrozenDeposit: 100 * fenPerYuan,
						},
						RefundPlans: []db.RiderDepositRefundPlan{{
							RefundOrder:        db.RefundOrder{ID: 2, PaymentOrderID: 92, RefundAmount: 100 * fenPerYuan, OutRefundNo: "RTEST124"},
							SourcePaymentOrder: db.PaymentOrder{ID: 92, OutTradeNo: "PTEST124", Amount: 100 * fenPerYuan},
						}},
					}, nil)

				paymentClient.EXPECT().
					CreateRefund(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: wechaterrorcodes.DirectPaymentCodeNotEnough, Message: "基本账户余额不足，请充值后重新发起"})

				store.EXPECT().
					ResolveRiderDepositRefundTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ResolveRiderDepositRefundTxResult{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, CodeConflict, resp.Code)
				require.Equal(t, "商户退款余额不足，暂时无法原路退款，请联系平台处理", resp.Message)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := wechatmock.NewMockDirectPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			server := newTestServerWithPayment(t, store, paymentClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/withdraw"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListRiderDepositsAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)

	// 创建测试押金流水
	deposits := []db.RiderDeposit{
		{ID: 1, RiderID: rider.ID, Amount: 100 * fenPerYuan, Type: "deposit", BalanceAfter: 100 * fenPerYuan},
		{ID: 2, RiderID: rider.ID, Amount: 50 * fenPerYuan, Type: "withdraw", BalanceAfter: 50 * fenPerYuan},
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page=1&limit=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderDeposits(gomock.Any(), db.ListRiderDepositsParams{
						RiderID: rider.ID,
						Limit:   20,
						Offset:  0,
					}).
					Times(1).
					Return(deposits, nil)

				store.EXPECT().
					CountRiderDeposits(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(int64(23), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listRiderDepositsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Deposits, 2)
				require.Equal(t, int64(23), resp.Total)
			},
		},
		{
			name:  "DefaultPagination",
			query: "", // 不传分页参数，使用默认值
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderDeposits(gomock.Any(), db.ListRiderDepositsParams{
						RiderID: rider.ID,
						Limit:   20, // 默认值
						Offset:  0,
					}).
					Times(1).
					Return(deposits, nil)

				store.EXPECT().
					CountRiderDeposits(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(int64(23), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "EmptyList",
			query: "?page=1&limit=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderDeposits(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.RiderDeposit{}, nil) // 空列表

				store.EXPECT().
					CountRiderDeposits(gomock.Any(), gomock.Eq(rider.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listRiderDepositsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp.Deposits) // 确保返回空数组而非 null
				require.Len(t, resp.Deposits, 0)
				require.Equal(t, int64(0), resp.Total)
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

			url := "/v1/rider/deposits" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 骑手状态测试 ====================

func TestGetRiderStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"
	rider.IsOnline = true
	rider.DepositAmount = 30000

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Online_NoDelivery",
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
				expectRiderThresholdFromOperator(store, rider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "active", resp.Status)
				require.True(t, resp.IsOnline)
				require.Equal(t, "online", resp.OnlineStatus)
				require.Equal(t, 0, resp.ActiveDeliveries)
				require.True(t, resp.CanGoOnline)
				require.True(t, resp.CanGoOffline)
			},
		},
		{
			name: "OK_Online_Delivering",
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
					Return([]db.Delivery{{ID: 1}}, nil)
				expectRiderThresholdFromOperator(store, rider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "delivering", resp.OnlineStatus)
				require.Equal(t, 1, resp.ActiveDeliveries)
				require.False(t, resp.CanGoOffline) // 有配送中订单不能下线
			},
		},
		{
			name: "OK_Offline",
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

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)
				expectRiderThresholdFromOperator(store, offlineRider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "offline", resp.OnlineStatus)
				require.False(t, resp.CanGoOffline) // 已经离线，不能再下线
			},
		},
		{
			name: "OK_InsufficientDeposit",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				insufficientRider := rider
				insufficientRider.DepositAmount = 0
				insufficientRider.IsOnline = false
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(insufficientRider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)
				expectRiderThresholdFromOperator(store, insufficientRider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.False(t, resp.CanGoOnline)
				require.Contains(t, resp.OnlineBlockReason, "押金不足")
			},
		},
		{
			name: "OK_ApprovedRiderCannotGoOnline",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				approvedRider := rider
				approvedRider.Status = db.RiderStatusApproved
				approvedRider.IsOnline = false
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(approvedRider, nil)

				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)
				expectRiderThresholdFromOperator(store, approvedRider, db.DefaultRiderDepositThresholdFen)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
				require.False(t, resp.CanGoOnline)
				require.Contains(t, resp.OnlineBlockReason, "押金不足")
			},
		},
		{
			name: "RiderNotFound",
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
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/rider/status"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 位置上报测试 ====================

func TestUpdateRiderLocationAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"
	rider.IsOnline = true

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_SingleLocation",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"accuracy":    10.5,
						"speed":       5.0,
						"heading":     90.0,
						"recorded_at": time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
					},
				},
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
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				store.EXPECT().
					BatchCreateRiderLocations(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					UpdateRiderLocation(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "位置更新成功", resp["message"])
				require.Equal(t, float64(1), resp["count"])
			},
		},
		{
			name: "OK_BatchLocations",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
					},
					{
						"longitude":   116.405,
						"latitude":    39.916,
						"recorded_at": time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
					},
					{
						"longitude":   116.406,
						"latitude":    39.917,
						"recorded_at": time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
					},
				},
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
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				store.EXPECT().
					BatchCreateRiderLocations(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(3), nil)

				store.EXPECT().
					UpdateRiderLocation(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, float64(3), resp["count"])
			},
		},
		{
			name: "NotOnline",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Format(time.RFC3339),
					},
				},
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
		{
			name: "FutureTime",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Add(10 * time.Minute).Format(time.RFC3339), // 未来时间
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TooOldTime",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339), // 超过1小时前
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidLongitude",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   200.0, // 超出范围
						"latitude":    39.915,
						"recorded_at": time.Now().Format(time.RFC3339),
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidLatitude",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    100.0, // 超出范围
						"recorded_at": time.Now().Format(time.RFC3339),
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "EmptyLocations",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RiderNotFound",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Format(time.RFC3339),
					},
				},
			},
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
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"locations": []map[string]interface{}{
					{
						"longitude":   116.404,
						"latitude":    39.915,
						"recorded_at": time.Now().Format(time.RFC3339),
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/location"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateRiderLocationGeofenceEvents(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "active"
	rider.IsOnline = true

	now := time.Now()
	deliveryID := int64(101)
	orderID := int64(202)

	delivery := db.Delivery{
		ID:              deliveryID,
		OrderID:         orderID,
		RiderID:         pgtype.Int8{Int64: rider.ID, Valid: true},
		PickupLongitude: numericFromFloat(116.404),
		PickupLatitude:  numericFromFloat(39.915),
		Status:          "assigned",
	}

	body := map[string]interface{}{
		"locations": []map[string]interface{}{
			{
				"delivery_id": deliveryID,
				"longitude":   116.404,
				"latitude":    39.915,
				"accuracy":    10.0,
				"recorded_at": now.Format(time.RFC3339),
				"source":      "gps",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(rider, nil)

	store.EXPECT().
		ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.Delivery{delivery}, nil)

	store.EXPECT().
		BatchCreateRiderLocations(gomock.Any(), gomock.Any()).
		Times(1).
		Return(int64(1), nil)

	store.EXPECT().
		UpdateRiderLocation(gomock.Any(), gomock.Any()).
		Times(1).
		Return(rider, nil)

	store.EXPECT().
		GetDelivery(gomock.Any(), gomock.Eq(deliveryID)).
		Times(1).
		Return(delivery, nil)

	store.EXPECT().
		CreateDeliveryLocationEvent(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.DeliveryLocationEvent{}, nil)

	store.EXPECT().
		ListDeliveryLocationsSince(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.RiderLocation{}, nil)

	server := newTestServer(t, store)
	server.config.GeofenceRadiusMeters = 80
	server.config.GeofenceDwellMinSeconds = 60
	server.config.GeofenceDwellMinSamples = 3
	server.config.GeofenceMinAccuracyMeters = 80
	server.config.GeofenceAutoAdvanceEnabled = false

	recorder := httptest.NewRecorder()
	url := "/v1/rider/location"
	requestBody, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(requestBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}
