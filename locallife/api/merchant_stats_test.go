package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 商户日报测试 ====================

func TestGetMerchantDailyStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantDailyStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantDailyStatsRow{
						{
							Date:          pgtype.Date{Time: time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:    50,
							TotalSales:    500000,
							Commission:    25000,
							TakeoutOrders: 30,
							DineInOrders:  20,
						},
						{
							Date:          pgtype.Date{Time: time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:    45,
							TotalSales:    450000,
							Commission:    22500,
							TakeoutOrders: 25,
							DineInOrders:  20,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []dailyStatRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 2)
				require.Equal(t, int32(50), resp[0].OrderCount)
				require.Equal(t, int64(500000), resp[0].TotalSales)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MissingStartDate",
			query: "?end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MissingEndDate",
			query: "?start_date=2025-11-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidStartDateFormat",
			query: "?start_date=2025/11/01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidEndDateFormat",
			query: "?start_date=2025-11-01&end_date=11-30-2025",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "StartDateAfterEndDate",
			query: "?start_date=2025-12-01&end_date=2025-11-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "DateRangeExceeds365Days",
			query: "?start_date=2024-01-01&end_date=2025-12-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
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
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantDailyStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			url := "/v1/merchant/stats/daily" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户概览测试 ====================

func TestGetMerchantOverviewAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantOverview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantOverviewRow{
						TotalDays:       30,
						TotalOrders:     1000,
						TotalSales:      10000000,
						TotalCommission: 500000,
						AvgDailySales:   333333,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantOverviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int32(30), resp.TotalDays)
				require.Equal(t, int32(1000), resp.TotalOrders)
				require.Equal(t, int64(10000000), resp.TotalSales)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
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

			url := "/v1/merchant/stats/overview" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 菜品销量排行测试 ====================

func TestGetTopSellingDishesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTopSellingDishes(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetTopSellingDishesRow{
						{
							DishID:       pgtype.Int8{Int64: 1, Valid: true},
							DishName:     "宫保鸡丁",
							DishPrice:    3800,
							TotalSold:    500,
							TotalRevenue: 1900000,
						},
						{
							DishID:       pgtype.Int8{Int64: 2, Valid: true},
							DishName:     "麻婆豆腐",
							DishPrice:    2500,
							TotalSold:    400,
							TotalRevenue: 1000000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []topSellingDishRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 2)
				require.Equal(t, "宫保鸡丁", resp[0].DishName)
				require.Equal(t, int32(500), resp[0].TotalSold)
			},
		},
		{
			name:  "OK_WithLimit",
			query: "?start_date=2025-11-01&end_date=2025-11-30&limit=5",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTopSellingDishes(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetTopSellingDishesRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?start_date=2025-11-01&end_date=2025-11-30&limit=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// limit=0时会被设为默认值10，所以仍然成功
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTopSellingDishes(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetTopSellingDishesRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "LimitExceedsMax",
			query: "?start_date=2025-11-01&end_date=2025-11-30&limit=101",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
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

			url := "/v1/merchant/stats/dishes/top" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 顾客列表测试 ====================

func TestListMerchantCustomersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantCustomerStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantCustomerStatsRow{
						{
							UserID:         100,
							FullName:       "张三",
							Phone:          pgtype.Text{String: "13800138000", Valid: true},
							AvatarUrl:      pgtype.Text{String: "https://example.com/avatar.jpg", Valid: true},
							TotalOrders:    10,
							TotalAmount:    100000,
							AvgOrderAmount: 10000,
							FirstOrderAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
							LastOrderAt:    time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC),
						},
					}, nil)

				store.EXPECT().
					CountMerchantCustomers(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(int32(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, float64(1), resp["total_count"])
				require.Len(t, resp["data"], 1)
			},
		},
		{
			name:  "OK_WithPagination",
			query: "?page=2&limit=10&order_by=total_orders",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantCustomerStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantCustomerStatsRow{}, nil)

				store.EXPECT().
					CountMerchantCustomers(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(int32(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidOrderBy",
			query: "?order_by=invalid_field",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPage_Negative",
			query: "?page=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?limit=101",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "NoAuthorization",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "",
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

			url := "/v1/merchant/stats/customers" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 顾客详情测试 ====================

func TestGetCustomerDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	customerID := int64(100)

	testCases := []struct {
		name          string
		userID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			userID: customerID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCustomerMerchantDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetCustomerMerchantDetailRow{
						UserID:         customerID,
						FullName:       "张三",
						Phone:          pgtype.Text{String: "13800138000", Valid: true},
						AvatarUrl:      pgtype.Text{String: "https://example.com/avatar.jpg", Valid: true},
						TotalOrders:    10,
						TotalAmount:    100000,
						AvgOrderAmount: 10000,
						FirstOrderAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						LastOrderAt:    time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC),
					}, nil)

				store.EXPECT().
					GetCustomerFavoriteDishes(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetCustomerFavoriteDishesRow{
						{
							DishID:        pgtype.Int8{Int64: 1, Valid: true},
							DishName:      "宫保鸡丁",
							OrderCount:    5,
							TotalQuantity: 8,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp customerDetailResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, customerID, resp.UserID)
				require.Equal(t, "张三", resp.FullName)
				require.Len(t, resp.FavoriteDishes, 1)
			},
		},
		{
			name:   "InvalidUserID",
			userID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:   "CustomerNotFound",
			userID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCustomerMerchantDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetCustomerMerchantDetailRow{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "NoAuthorization",
			userID: customerID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
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

			url := fmt.Sprintf("/v1/merchant/stats/customers/%d", tc.userID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 时段分析测试 ====================

func TestGetMerchantHourlyStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantHourlyStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantHourlyStatsRow{
						{Hour: 11, OrderCount: 50, TotalSales: 500000, AvgOrderAmount: 10000},
						{Hour: 12, OrderCount: 80, TotalSales: 800000, AvgOrderAmount: 10000},
						{Hour: 18, OrderCount: 60, TotalSales: 600000, AvgOrderAmount: 10000},
						{Hour: 19, OrderCount: 70, TotalSales: 700000, AvgOrderAmount: 10000},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []merchantHourlyStatsRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 4)
				// 12点是高峰
				require.Equal(t, int32(12), resp[1].Hour)
				require.Equal(t, int32(80), resp[1].OrderCount)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MissingStartDate",
			query: "?end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/merchant/stats/hourly" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 订单来源分析测试 ====================

func TestGetMerchantOrderSourceStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantOrderSourceStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantOrderSourceStatsRow{
						{OrderType: "takeout", OrderCount: 600, TotalSales: 6000000, AvgOrderAmount: 10000},
						{OrderType: "dine_in", OrderCount: 400, TotalSales: 4000000, AvgOrderAmount: 10000},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []merchantOrderSourceStatsRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 2)
				require.Equal(t, "takeout", resp[0].OrderType)
				require.Equal(t, int32(600), resp[0].OrderCount)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
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

			url := "/v1/merchant/stats/sources" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 复购率分析测试 ====================

func TestGetMerchantRepurchaseRateAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantRepurchaseRate(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantRepurchaseRateRow{
						TotalCustomers:            100,
						RepeatCustomers:           30,
						TotalOrders:               150,
						RepurchaseRateBasisPoints: 3050, // 30.50% -> 万分比 3050
						AvgOrdersPerUserCents:     150,  // 1.50 -> 百分数 150
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantRepurchaseRateResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int32(100), resp.TotalUsers)
				require.Equal(t, int32(30), resp.RepeatUsers)
				require.Equal(t, float64(30.50), resp.RepurchaseRate)
				require.Equal(t, float64(1.50), resp.AvgOrdersPerUser)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
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

			url := "/v1/merchant/stats/repurchase" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 菜品分类统计测试 ====================

func TestGetMerchantDishCategoryStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
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
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDishCategoryStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetDishCategoryStatsRow{
						{CategoryID: 1, CategoryName: "热菜", DishCount: 10, TotalQuantity: 500, TotalRevenue: 5000000},
						{CategoryID: 2, CategoryName: "凉菜", DishCount: 5, TotalQuantity: 200, TotalRevenue: 1000000},
						{CategoryID: 3, CategoryName: "主食", DishCount: 3, TotalQuantity: 300, TotalRevenue: 600000},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []dishCategoryStatsRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 3)
				require.Equal(t, "热菜", resp[0].CategoryName)
				require.Equal(t, int64(5000000), resp[0].TotalSales)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
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
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=invalid&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)
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

			url := "/v1/merchant/stats/categories" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
