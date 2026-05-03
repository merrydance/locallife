package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ============================================================================
// 平台全局概览测试
// ============================================================================

func TestGetPlatformOverviewAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware - Admin role
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				// GetPlatformOverview
				store.EXPECT().
					GetPlatformOverview(gomock.Any(), gomock.Any()).
					Return(db.GetPlatformOverviewRow{
						TotalOrders:     1000,
						TotalGmv:        5000000,
						TotalCommission: 150000,
						ActiveMerchants: 50,
						ActiveUsers:     2000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp platformOverviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int32(1000), resp.TotalOrders)
				require.Equal(t, int64(5000000), resp.TotalGMV)
				require.Equal(t, int64(150000), resp.TotalCommission)
				require.Equal(t, int32(50), resp.ActiveMerchants)
				require.Equal(t, int32(2000), resp.ActiveUsers)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=invalid&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "DateRangeExceeded",
			query: "?start_date=2024-01-01&end_date=2025-12-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "StartDateAfterEndDate",
			query: "?start_date=2025-02-01&end_date=2025-01-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MissingStartDate",
			query: "?end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "UnauthorizedNonAdmin",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Return non-admin role
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "customer",
						Status: "active",
					}}, nil)
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

			url := "/v1/platform/stats/overview" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 平台日趋势测试
// ============================================================================

func TestGetPlatformDailyStatsAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-03",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetPlatformDailyStats(gomock.Any(), gomock.Any()).
					Return([]db.GetPlatformDailyStatsRow{
						{
							Date:            pgtype.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:      100,
							TotalGmv:        500000,
							TotalCommission: 15000,
							ActiveMerchants: 30,
							ActiveUsers:     200,
							TakeoutOrders:   70,
							DineInOrders:    30,
						},
						{
							Date:            pgtype.Date{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:      120,
							TotalGmv:        600000,
							TotalCommission: 18000,
							ActiveMerchants: 35,
							ActiveUsers:     250,
							TakeoutOrders:   80,
							DineInOrders:    40,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []platformDailyStatRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "2025-01-01", resp[0].Date)
				require.Equal(t, int32(100), resp[0].OrderCount)
				require.Equal(t, int32(70), resp[0].TakeoutOrders)
				require.Equal(t, int32(30), resp[0].DineInOrders)
			},
		},
		{
			name:  "EmptyResult",
			query: "?start_date=2025-06-01&end_date=2025-06-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetPlatformDailyStats(gomock.Any(), gomock.Any()).
					Return([]db.GetPlatformDailyStatsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []platformDailyStatRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
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

			url := "/v1/platform/stats/daily" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 平台分账对账汇总测试
// ============================================================================

func TestGetPlatformProfitSharingReconciliationAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetProfitSharingReconciliationSummary(gomock.Any(), gomock.Any()).
					Return([]db.GetProfitSharingReconciliationSummaryRow{
						{
							Status:                  "finished",
							TotalOrders:             12,
							TotalAmount:             120000,
							TotalPlatformCommission: 3600,
							TotalOperatorCommission: 2400,
						},
						{
							Status:                  "failed",
							TotalOrders:             2,
							TotalAmount:             20000,
							TotalPlatformCommission: 0,
							TotalOperatorCommission: 0,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []platformProfitSharingReconciliationRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "finished", resp[0].Status)
				require.Equal(t, int64(12), resp[0].TotalOrders)
				require.Equal(t, int64(120000), resp[0].TotalAmount)
				require.Equal(t, int64(3600), resp[0].TotalPlatformCommission)
				require.Equal(t, int64(2400), resp[0].TotalOperatorCommission)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=bad&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "UnauthorizedNonAdmin",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "customer",
						Status: "active",
					}}, nil)
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

			url := "/v1/platform/stats/profit-sharing/reconciliation" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetPlatformBaofuDailyReconciliationAPI(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{
			UserID: admin.ID,
			Role:   "admin",
			Status: "active",
		}}, nil)
	store.EXPECT().
		GetBaofuDailyReconciliation(gomock.Any(), gomock.Any()).
		Return([]db.GetBaofuDailyReconciliationRow{{
			Date:                     pgtype.Date{Time: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), Valid: true},
			Provider:                 db.ExternalPaymentProviderBaofu,
			Channel:                  db.PaymentChannelBaofuAggregate,
			PaidAmount:               10000,
			PaymentFee:               30,
			MerchantAmount:           8970,
			RiderAmount:              500,
			PlatformCommission:       200,
			OperatorCommission:       300,
			WithdrawSucceededAmount:  6000,
			WithdrawProcessingAmount: 1500,
			UnappliedFactCount:       2,
			UnknownCommandCount:      1,
			FeeLedgerMismatchCount:   1,
		}}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/stats/baofu/reconciliation/daily?start_date=2026-05-03&end_date=2026-05-03", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "sharing_mer_id")
	require.NotContains(t, recorder.Body.String(), "contract_no")
	require.NotContains(t, recorder.Body.String(), "contractNo")

	var resp []platformBaofuDailyReconciliationRow
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp, 1)
	require.Equal(t, "2026-05-03", resp[0].Date)
	require.Equal(t, db.ExternalPaymentProviderBaofu, resp[0].Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, resp[0].Channel)
	require.Equal(t, int64(10000), resp[0].PaidAmount)
	require.Equal(t, int64(30), resp[0].PaymentFee)
	require.Equal(t, int64(8970), resp[0].MerchantAmount)
	require.Equal(t, int64(500), resp[0].RiderAmount)
	require.Equal(t, int64(200), resp[0].PlatformCommission)
	require.Equal(t, int64(300), resp[0].OperatorCommission)
	require.Equal(t, int64(6000), resp[0].WithdrawSucceededAmount)
	require.Equal(t, int64(1500), resp[0].WithdrawProcessingAmount)
	require.Equal(t, int64(2), resp[0].UnappliedFactCount)
	require.Equal(t, int64(1), resp[0].UnknownCommandCount)
	require.Equal(t, int64(1), resp[0].FeeLedgerMismatchCount)
}

// ============================================================================
// 平台分账 SLA 汇总测试
// ============================================================================

func TestGetPlatformProfitSharingSlaSummaryAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetProfitSharingSlaSummary(gomock.Any(), gomock.Any()).
					Return(db.GetProfitSharingSlaSummaryRow{
						TotalOrders:      20,
						FinishedOrders:   18,
						FailedOrders:     1,
						PendingOrders:    1,
						AvgFinishSeconds: 42,
						P95FinishSeconds: 120,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp platformProfitSharingSlaSummaryResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(20), resp.TotalOrders)
				require.Equal(t, int64(18), resp.FinishedOrders)
				require.Equal(t, int64(1), resp.FailedOrders)
				require.Equal(t, int64(1), resp.PendingOrders)
				require.Equal(t, int64(42), resp.AvgFinishSeconds)
				require.Equal(t, int64(120), resp.P95FinishSeconds)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=bad&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "UnauthorizedNonAdmin",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "customer",
						Status: "active",
					}}, nil)
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

			url := "/v1/platform/stats/profit-sharing/sla" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 平台分账规则审计记录测试
// ============================================================================

func TestGetPlatformProfitSharingConfigAuditsAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?config_id=12&page=1&limit=2",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					ListProfitSharingConfigAudits(gomock.Any(), gomock.Any()).
					Return([]db.ProfitSharingConfigAudit{
						{
							ID:        1,
							ConfigID:  12,
							Action:    "update",
							ActorID:   pgtype.Int8{Int64: 99, Valid: true},
							ActorRole: pgtype.Text{String: "admin", Valid: true},
							Detail:    []byte(`{"before":{},"after":{"status":"active"}}`),
							CreatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listProfitSharingConfigAuditsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int32(1), resp.Page)
				require.Equal(t, int32(2), resp.Limit)
				require.Equal(t, int64(12), resp.Items[0].ConfigID)
				require.Equal(t, "update", resp.Items[0].Action)
				require.NotNil(t, resp.Items[0].ActorID)
				require.NotNil(t, resp.Items[0].ActorRole)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?limit=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "UnauthorizedNonAdmin",
			query: "?page=1&limit=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "customer",
						Status: "active",
					}}, nil)
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

			url := "/v1/platform/stats/profit-sharing/config-audits" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 区域对比分析测试
// ============================================================================

func TestGetRegionComparisonAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetRegionComparison(gomock.Any(), gomock.Any()).
					Return([]db.GetRegionComparisonRow{
						{
							RegionID:        1,
							RegionName:      "北京",
							MerchantCount:   100,
							OrderCount:      5000,
							TotalGmv:        25000000,
							TotalCommission: 750000,
							AvgOrderAmount:  5000,
							ActiveUsers:     3000,
						},
						{
							RegionID:        2,
							RegionName:      "上海",
							MerchantCount:   80,
							OrderCount:      4000,
							TotalGmv:        20000000,
							TotalCommission: 600000,
							AvgOrderAmount:  5000,
							ActiveUsers:     2500,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []regionComparisonRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "北京", resp[0].RegionName)
				require.Equal(t, int32(100), resp[0].MerchantCount)
				require.Equal(t, int64(25000000), resp[0].TotalGMV)
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

			url := "/v1/platform/stats/regions/compare" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 商户销售排行测试
// ============================================================================

func TestGetMerchantRankingAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetMerchantRanking(gomock.Any(), db.GetMerchantRankingParams{
						StartAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
						Limit:   20,
						Offset:  0,
					}).
					Return([]db.GetMerchantRankingRow{
						{
							MerchantID:      1,
							MerchantName:    "测试商户1",
							RegionID:        1,
							RegionName:      "北京",
							OrderCount:      500,
							TotalSales:      2500000,
							TotalCommission: 75000,
							AvgOrderAmount:  5000,
						},
						{
							MerchantID:      2,
							MerchantName:    "测试商户2",
							RegionID:        2,
							RegionName:      "上海",
							OrderCount:      400,
							TotalSales:      2000000,
							TotalCommission: 60000,
							AvgOrderAmount:  5000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []merchantRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "测试商户1", resp[0].MerchantName)
				require.Equal(t, int32(500), resp[0].OrderCount)
			},
		},
		{
			name:  "WithPagination",
			query: "?start_date=2025-01-01&end_date=2025-01-31&page=2&limit=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetMerchantRanking(gomock.Any(), db.GetMerchantRankingParams{
						StartAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
						Limit:   10,
						Offset:  10, // (page-1) * limit = (2-1) * 10 = 10
					}).
					Return([]db.GetMerchantRankingRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?start_date=2025-01-01&end_date=2025-01-31&limit=200",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)
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

			url := "/v1/platform/stats/merchants/ranking" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 分类销售统计测试
// ============================================================================

func TestGetCategoryStatsAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetCategoryStats(gomock.Any(), gomock.Any()).
					Return([]db.GetCategoryStatsRow{
						{
							CategoryName:  "快餐",
							MerchantCount: 50,
							OrderCount:    2000,
							TotalSales:    10000000,
						},
						{
							CategoryName:  "火锅",
							MerchantCount: 30,
							OrderCount:    1000,
							TotalSales:    8000000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []categoryStatRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "快餐", resp[0].CategoryName)
				require.Equal(t, int32(50), resp[0].MerchantCount)
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

			url := "/v1/platform/stats/categories" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 用户增长统计测试
// ============================================================================

func TestGetUserGrowthStatsAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-03",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetUserGrowthStats(gomock.Any(), gomock.Any()).
					Return([]db.GetUserGrowthStatsRow{
						{
							Date:     pgtype.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
							NewUsers: 50,
						},
						{
							Date:     pgtype.Date{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true},
							NewUsers: 60,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []growthStatRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "2025-01-01", resp[0].Date)
				require.Equal(t, int32(50), resp[0].Count)
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

			url := "/v1/platform/stats/growth/users" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 商户增长统计测试
// ============================================================================

func TestGetMerchantGrowthStatsAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-03",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetMerchantGrowthStats(gomock.Any(), gomock.Any()).
					Return([]db.GetMerchantGrowthStatsRow{
						{
							Date:         pgtype.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
							NewMerchants: 5,
						},
						{
							Date:         pgtype.Date{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true},
							NewMerchants: 8,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []growthStatRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "2025-01-01", resp[0].Date)
				require.Equal(t, int32(5), resp[0].Count)
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

			url := "/v1/platform/stats/growth/merchants" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 骑手绩效排行测试
// ============================================================================

func TestGetRiderRankingAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetRiderPerformanceRanking(gomock.Any(), db.GetRiderPerformanceRankingParams{
						StartAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
						Limit:   20,
						Offset:  0,
					}).
					Return([]db.GetRiderPerformanceRankingRow{
						{
							RiderID:                1,
							RiderName:              "骑手A",
							DeliveryCount:          200,
							CompletedCount:         190,
							AvgDeliveryTimeSeconds: 1800, // 30分钟
							TotalEarnings:          95000,
						},
						{
							RiderID:                2,
							RiderName:              "骑手B",
							DeliveryCount:          180,
							CompletedCount:         175,
							AvgDeliveryTimeSeconds: 2100, // 35分钟
							TotalEarnings:          87500,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []riderRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, "骑手A", resp[0].RiderName)
				require.Equal(t, int32(200), resp[0].DeliveryCount)
				require.Equal(t, int32(190), resp[0].CompletedCount)
				require.Equal(t, int32(1800), resp[0].AvgDeliveryTimeSeconds)
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

			url := "/v1/platform/stats/riders/ranking" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 订单时段分布测试
// ============================================================================

func TestGetHourlyDistributionAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetHourlyDistribution(gomock.Any(), gomock.Any()).
					Return([]db.GetHourlyDistributionRow{
						{Hour: 11, OrderCount: 300, TotalGmv: 1500000},
						{Hour: 12, OrderCount: 500, TotalGmv: 2500000},
						{Hour: 18, OrderCount: 400, TotalGmv: 2000000},
						{Hour: 19, OrderCount: 450, TotalGmv: 2250000},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []hourlyDistributionRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 4)
				// 验证午餐高峰时段数据
				require.Equal(t, int32(12), resp[1].Hour)
				require.Equal(t, int32(500), resp[1].OrderCount)
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

			url := "/v1/platform/stats/hourly" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 实时大盘测试
// ============================================================================

func TestGetRealtimeDashboardAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetRealtimeDashboard(gomock.Any()).
					Return(db.GetRealtimeDashboardRow{
						Orders24h:          500,
						Gmv24h:             2500000,
						ActiveMerchants24h: 80,
						ActiveUsers24h:     300,
						PendingOrders:      10,
						PreparingOrders:    15,
						ReadyOrders:        5,
						DeliveringOrders:   20,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp realtimeDashboardResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int32(500), resp.Orders24h)
				require.Equal(t, int64(2500000), resp.GMV24h)
				require.Equal(t, int32(80), resp.ActiveMerchants24h)
				require.Equal(t, int32(300), resp.ActiveUsers24h)
				require.Equal(t, int32(10), resp.PendingOrders)
				require.Equal(t, int32(15), resp.PreparingOrders)
				require.Equal(t, int32(5), resp.ReadyOrders)
				require.Equal(t, int32(20), resp.DeliveringOrders)
			},
		},
		{
			name: "NoAuth",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No stubs needed
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "DBError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   "admin",
						Status: "active",
					}}, nil)

				store.EXPECT().
					GetRealtimeDashboard(gomock.Any()).
					Return(db.GetRealtimeDashboardRow{}, fmt.Errorf("database error"))
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

			url := "/v1/platform/stats/realtime"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
