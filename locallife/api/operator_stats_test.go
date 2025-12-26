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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ============================================================================
// 区域统计测试
// ============================================================================

func TestGetRegionStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	testCases := []struct {
		name          string
		regionID      int64
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			regionID: operator.RegionID,
			query:    "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				// LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				// checkOperatorManagesRegion
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   operator.RegionID,
					}).
					Return(true, nil)

				// GetRegionStats
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{
						RegionID:        operator.RegionID,
						RegionName:      "测试区域",
						MerchantCount:   10,
						TotalOrders:     100,
						TotalGmv:        100000,
						TotalCommission: 3000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp regionStatsResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, operator.RegionID, resp.RegionID)
				require.Equal(t, "测试区域", resp.RegionName)
				require.Equal(t, int32(10), resp.MerchantCount)
				require.Equal(t, int32(100), resp.TotalOrders)
			},
		},
		{
			name:     "InvalidDateFormat",
			regionID: operator.RegionID,
			query:    "?start_date=invalid&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "DateRangeTooLong",
			regionID: operator.RegionID,
			query:    "?start_date=2024-01-01&end_date=2025-12-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "RegionNotManaged",
			regionID: operator.RegionID + 999,
			query:    "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   operator.RegionID + 999,
					}).
					Return(false, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "RegionNotFound",
			regionID: operator.RegionID,
			query:    "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   operator.RegionID,
					}).
					Return(true, nil)

				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "NoAuthorization",
			regionID: operator.RegionID,
			query:    "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth header
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

			url := fmt.Sprintf("/v1/operator/regions/%d/stats%s", tc.regionID, tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 商户排行测试
// ============================================================================
// 商户排行测试
// ============================================================================

func TestGetOperatorMerchantRankingAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	merchants := []db.GetOperatorMerchantRankingRow{
		{
			MerchantID:     1,
			MerchantName:   "商户1",
			OrderCount:     100,
			TotalSales:     50000,
			Commission:     1500,
			AvgOrderAmount: 500,
		},
		{
			MerchantID:     2,
			MerchantName:   "商户2",
			OrderCount:     80,
			TotalSales:     40000,
			Commission:     1200,
			AvgOrderAmount: 500,
		},
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
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					GetOperatorMerchantRanking(gomock.Any(), gomock.Any()).
					Return(merchants, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorMerchantRankingRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 2)
				require.Equal(t, "商户1", resp[0].MerchantName)
			},
		},
		{
			name:  "MissingStartDate",
			query: "?end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)
			},
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

			url := "/v1/operator/merchants/ranking" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 骑手排行测试
// ============================================================================

func TestGetOperatorRiderRankingAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	riders := []db.GetOperatorRiderRankingRow{
		{
			RiderID:         1,
			RiderName:       "骑手1",
			DeliveryCount:   100,
			CompletedCount:  95,
			AvgDeliveryTime: 1800,
			TotalEarnings:   10000,
		},
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
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					GetOperatorRiderRanking(gomock.Any(), gomock.Any()).
					Return(riders, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorRiderRankingRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 1)
				require.Equal(t, "骑手1", resp[0].RiderName)
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

			url := "/v1/operator/riders/ranking" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 每日趋势测试
// ============================================================================

func TestGetRegionDailyTrendAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	trends := []db.GetRegionDailyTrendRow{
		{
			Date:            pgtype.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			OrderCount:      50,
			TotalGmv:        25000,
			Commission:      750,
			ActiveUsers:     30,
			ActiveMerchants: 5,
		},
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
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					GetRegionDailyTrend(gomock.Any(), gomock.Any()).
					Return(trends, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []regionDailyTrendRow
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp, 1)
				require.Equal(t, "2025-01-01", resp[0].Date)
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

			url := "/v1/operator/trend/daily" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 财务概览测试
// ============================================================================

func TestGetOperatorFinanceOverviewAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

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
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					GetRegion(gomock.Any(), operator.RegionID).
					Return(db.Region{ID: operator.RegionID, Name: "测试区域"}, nil)

				// 当月统计
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{
						RegionID:        operator.RegionID,
						RegionName:      "测试区域",
						TotalOrders:     100,
						TotalGmv:        100000,
						TotalCommission: 3000,
					}, nil)

				// 累计统计
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{
						RegionID:        operator.RegionID,
						RegionName:      "测试区域",
						TotalOrders:     500,
						TotalGmv:        500000,
						TotalCommission: 15000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp operatorFinanceOverviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, operator.RegionID, resp.RegionID)
				require.Equal(t, "测试区域", resp.RegionName)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/overview", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 佣金明细测试
// ============================================================================

func TestGetOperatorCommissionAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	trends := []db.GetRegionDailyTrendRow{
		{
			Date:            pgtype.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			OrderCount:      50,
			TotalGmv:        25000,
			Commission:      750,
			ActiveUsers:     30,
			ActiveMerchants: 5,
		},
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
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					GetRegionDailyTrend(gomock.Any(), gomock.Any()).
					Return(trends, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp operatorCommissionResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int64(750), resp.Summary.TotalCommission)
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

			url := "/v1/operators/me/commission" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// randomOperator is already defined in delivery_fee_test.go
