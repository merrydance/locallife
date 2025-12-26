package api

import (
	"database/sql"
	"encoding/json"
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

func TestGetMerchantFinanceOverviewAPI(t *testing.T) {
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
					GetMerchantFinanceOverview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantFinanceOverviewRow{
						CompletedOrders:  100,
						PendingOrders:    5,
						TotalGmv:         1000000,
						TotalIncome:      950000,
						TotalPlatformFee: 20000,
						TotalOperatorFee: 30000,
						PendingIncome:    50000,
					}, nil)

				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantPromotionExpensesRow{
						PromoOrderCount: 10,
						TotalDiscount:   10000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp financeOverviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int64(100), resp.CompletedOrders)
				require.Equal(t, int64(1000000), resp.TotalGMV)
				require.Equal(t, int64(50000), resp.TotalServiceFee)
				require.Equal(t, int64(940000), resp.NetIncome) // 950000 - 10000
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
			name:  "InvalidDateFormat",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/overview" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantFinanceOrdersAPI(t *testing.T) {
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
					ListMerchantFinanceOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantFinanceOrdersRow{
						{
							ID:                 1,
							PaymentOrderID:     100,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							Status:             "finished",
							CreatedAt:          time.Now(),
							OrderID:            pgtype.Int8{Int64: 1000, Valid: true},
						},
					}, nil)

				store.EXPECT().
					CountMerchantFinanceOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["orders"])
				require.Equal(t, float64(1), resp["total"])
			},
		},
		{
			name:  "WithPagination",
			query: "?start_date=2025-11-01&end_date=2025-11-30&page=2&limit=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListMerchantFinanceOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantFinanceOrdersRow{}, nil)

				store.EXPECT().
					CountMerchantFinanceOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(15), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, float64(2), resp["page"])
				require.Equal(t, float64(10), resp["limit"])
				require.Equal(t, float64(2), resp["total_pages"])
			},
		},
		{
			name:  "InvalidPageNumberNegative",
			query: "?start_date=2025-11-01&end_date=2025-11-30&page=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLimitExceedsMax",
			query: "?start_date=2025-11-01&end_date=2025-11-30&limit=101",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/orders" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantServiceFeesAPI(t *testing.T) {
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
					GetMerchantServiceFeeDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantServiceFeeDetailRow{
						{
							Date:        pgtype.Date{Time: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderSource: "takeout",
							OrderCount:  50,
							TotalAmount: 500000,
							PlatformFee: 10000,
							OperatorFee: 15000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["details"])
				require.Equal(t, float64(10000), resp["total_platform_fee"])
				require.Equal(t, float64(15000), resp["total_operator_fee"])
				require.Equal(t, float64(25000), resp["total_service_fee"])
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/service-fees" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantPromotionExpensesAPI(t *testing.T) {
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
					ListMerchantPromotionOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantPromotionOrdersRow{
						{
							ID:                  1,
							OrderNo:             "ORD20251115001",
							OrderType:           "takeout",
							Subtotal:            10000,
							DeliveryFee:         500,
							DeliveryFeeDiscount: 500,
							TotalAmount:         10000,
							CreatedAt:           time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantPromotionOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantPromotionExpensesRow{
						PromoOrderCount: 1,
						TotalDiscount:   500,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["orders"])
				require.Equal(t, float64(1), resp["total_promo_orders"])
				require.Equal(t, float64(500), resp["total_promo_amount"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/promotions" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantDailyFinanceAPI(t *testing.T) {
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
					GetMerchantDailyFinance(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantDailyFinanceRow{
						{
							Date:           pgtype.Date{Time: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:     100,
							TotalGmv:       1000000,
							MerchantIncome: 950000,
							TotalFee:       50000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["daily_stats"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/daily" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantSettlementsAPI(t *testing.T) {
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
			name:  "OK_NoStatusFilter",
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
					ListMerchantSettlements(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ProfitSharingOrder{
						{
							ID:                 1,
							PaymentOrderID:     100,
							MerchantID:         merchant.ID,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							OutOrderNo:         "PSO20251115001",
							Status:             "finished",
							CreatedAt:          time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantSettlements(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantProfitSharingStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantProfitSharingStatsRow{
						TotalOrders:             1,
						TotalAmount:             10000,
						TotalMerchantAmount:     9500,
						TotalPlatformCommission: 200,
						TotalOperatorCommission: 300,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["settlements"])
				require.Equal(t, float64(1), resp["total"])
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "?start_date=2025-11-01&end_date=2025-11-30&status=finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListMerchantSettlementsByStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ProfitSharingOrder{
						{
							ID:                 1,
							PaymentOrderID:     100,
							MerchantID:         merchant.ID,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							OutOrderNo:         "PSO20251115001",
							Status:             "finished",
							CreatedAt:          time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantSettlementsByStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantProfitSharingStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantProfitSharingStatsRow{
						TotalOrders:             1,
						TotalAmount:             10000,
						TotalMerchantAmount:     9500,
						TotalPlatformCommission: 200,
						TotalOperatorCommission: 300,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp["settlements"])
			},
		},
		{
			name:  "InvalidStatus",
			query: "?start_date=2025-11-01&end_date=2025-11-30&status=invalid",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/settlements" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
