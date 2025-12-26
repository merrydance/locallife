package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
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
		TotalOrders:   0,
		TotalEarnings: 0,
		CreatedAt:     time.Now(),
	}
}

func TestApplyRiderAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "13812345678",
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// First check if already applied - should return not found
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)

				rider := randomRider(user.ID)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "13812345678",
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MissingField",
			body: map[string]interface{}{
				"real_name": "张三",
				"phone":     "13812345678",
				// missing id_card_no
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyApplied",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "13812345678",
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 已经申请过骑手
				existingRider := randomRider(user.ID)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(existingRider, nil)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "InvalidPhone_TooShort",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "1381234567", // 只有10位
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidPhone_WrongPrefix",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "12812345678", // 以12开头
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidIDCard_WrongLength",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "13812345678",
				"id_card_no": "1101011990010112", // 只有16位
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidIDCard_InvalidCharacter",
			body: map[string]interface{}{
				"real_name":  "张三",
				"phone":      "13812345678",
				"id_card_no": "11010119900101123A", // A不是有效字符
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RealNameTooShort",
			body: map[string]interface{}{
				"real_name":  "李", // 只有1个字
				"phone":      "13812345678",
				"id_card_no": "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/apply"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
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
		buildStubs    func(store *mockdb.MockStore)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

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
			name: "InsufficientDeposit",
			body: map[string]interface{}{
				"longitude": 116.404,
				"latitude":  39.915,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				insufficientRider := rider
				insufficientRider.DepositAmount = 0
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(insufficientRider, nil)
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
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
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
		buildStubs    func(store *mockdb.MockStore)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 创建支付订单
				store.EXPECT().
					CreatePaymentOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						UserID:       user.ID,
						PaymentType:  "mini_program",
						BusinessType: "rider_deposit",
						Amount:       10000,
						Status:       "pending",
						OutTradeNo:   "test_order_123",
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				// 验证返回支付订单信息
				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Contains(t, resp, "payment_order_id")
				require.Contains(t, resp, "out_trade_no")
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
			buildStubs: func(store *mockdb.MockStore) {
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
				"amount": 10000,
				"remark": "充值押金",
			},
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
		{
			name: "RiderNotActive",
			body: map[string]interface{}{
				"amount": 10000,
				"remark": "充值押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
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
	rider.DepositAmount = 50000 // 500元
	rider.FrozenDeposit = 5000  // 冻结50元

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
				var resp depositBalanceResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int64(50000), resp.TotalDeposit)
				require.Equal(t, int64(5000), resp.FrozenDeposit)
				require.Equal(t, int64(45000), resp.AvailableDeposit)
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
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Unauthorized",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Any()).
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
	rider.DepositAmount = 50000 // 500元
	rider.FrozenDeposit = 5000  // 冻结50元

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"amount": 10000, // 100元
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(rider, nil)

				// 检查活跃配送
				store.EXPECT().
					ListRiderActiveDeliveries(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Delivery{}, nil)

				// 执行提现事务
				store.EXPECT().
					WithdrawDepositTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.WithdrawDepositTxResult{
						Rider: db.Rider{
							ID:            rider.ID,
							DepositAmount: 40000,
							FrozenDeposit: 5000,
						},
						DepositLog: db.RiderDeposit{
							ID:           1,
							RiderID:      rider.ID,
							Amount:       10000,
							Type:         "withdraw",
							BalanceAfter: 40000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp depositResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int64(10000), resp.Amount)
				require.Equal(t, "withdraw", resp.Type)
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
			buildStubs: func(store *mockdb.MockStore) {
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
			buildStubs: func(store *mockdb.MockStore) {
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
				"amount": 10000,
				"remark": "提现押金",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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
				"amount": 100000, // 超过可用余额 45000
				"remark": "提现押金",
			},
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
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "HasActiveDeliveries",
			body: map[string]interface{}{
				"amount": 10000,
				"remark": "提现押金",
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
					Return([]db.Delivery{{ID: 1}}, nil) // 有活跃配送
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
		{ID: 1, RiderID: rider.ID, Amount: 10000, Type: "deposit", BalanceAfter: 10000},
		{ID: 2, RiderID: rider.ID, Amount: 5000, Type: "withdraw", BalanceAfter: 5000},
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp []depositResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 2)
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp []depositResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp) // 确保返回空数组而非 null
				require.Len(t, resp, 0)
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderStatusResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
					Return(db.Rider{}, pgx.ErrNoRows)
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
					Return(db.Rider{}, pgx.ErrNoRows)
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

// ==================== 高值单资格积分测试 ====================

func TestGetRiderPremiumScoreAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_CanAcceptPremiumOrder",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderPremiumScoreWithProfile(gomock.Any(), rider.ID).
					Times(1).
					Return(db.GetRiderPremiumScoreWithProfileRow{
						RiderID:               rider.ID,
						RealName:              rider.RealName,
						PremiumScore:          5,
						CanAcceptPremiumOrder: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderPremiumScoreResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, rider.ID, resp.RiderID)
				require.Equal(t, int16(5), resp.PremiumScore)
				require.True(t, resp.CanAcceptPremiumOrder)
			},
		},
		{
			name: "OK_CannotAcceptPremiumOrder",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderPremiumScoreWithProfile(gomock.Any(), rider.ID).
					Times(1).
					Return(db.GetRiderPremiumScoreWithProfileRow{
						RiderID:               rider.ID,
						RealName:              rider.RealName,
						PremiumScore:          -2,
						CanAcceptPremiumOrder: false,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderPremiumScoreResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int16(-2), resp.PremiumScore)
				require.False(t, resp.CanAcceptPremiumOrder)
			},
		},
		{
			name: "NotARider",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "ProfileNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderPremiumScoreWithProfile(gomock.Any(), rider.ID).
					Times(1).
					Return(db.GetRiderPremiumScoreWithProfileRow{}, pgx.ErrNoRows)
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

			url := "/v1/rider/score"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListRiderPremiumScoreHistoryAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderPremiumScore(gomock.Any(), rider.ID).
					Times(1).
					Return(int16(3), nil)

				store.EXPECT().
					CountRiderPremiumScoreLogs(gomock.Any(), rider.ID).
					Times(1).
					Return(int64(2), nil)

				store.EXPECT().
					ListRiderPremiumScoreLogs(gomock.Any(), db.ListRiderPremiumScoreLogsParams{
						RiderID: rider.ID,
						Limit:   10,
						Offset:  0,
					}).
					Times(1).
					Return([]db.RiderPremiumScoreLog{
						{
							ID:           1,
							RiderID:      rider.ID,
							ChangeAmount: 1,
							OldScore:     2,
							NewScore:     3,
							ChangeType:   "normal_order",
							CreatedAt:    time.Now(),
						},
						{
							ID:           2,
							RiderID:      rider.ID,
							ChangeAmount: -3,
							OldScore:     5,
							NewScore:     2,
							ChangeType:   "premium_order",
							CreatedAt:    time.Now().Add(-time.Hour),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listRiderPremiumScoreHistoryResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int16(3), resp.CurrentScore)
				require.Equal(t, int64(2), resp.Total)
				require.Len(t, resp.Logs, 2)
				require.Equal(t, "完成普通单", resp.Logs[0].ChangeTypeName)
				require.Equal(t, "完成高值单", resp.Logs[1].ChangeTypeName)
			},
		},
		{
			name:  "OK_EmptyHistory",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderPremiumScore(gomock.Any(), rider.ID).
					Times(1).
					Return(int16(0), nil)

				store.EXPECT().
					CountRiderPremiumScoreLogs(gomock.Any(), rider.ID).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ListRiderPremiumScoreLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.RiderPremiumScoreLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listRiderPremiumScoreHistoryResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int16(0), resp.CurrentScore)
				require.Equal(t, int64(0), resp.Total)
				require.Len(t, resp.Logs, 0)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=10",
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
			name:  "InvalidPageSize",
			query: "?page_id=1&page_size=100",
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
			name:  "NotARider",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?page_id=1&page_size=10",
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

			url := "/v1/rider/score/history" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
