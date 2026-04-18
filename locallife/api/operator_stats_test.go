package api

import (
	"context"
	"errors"
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)

				// GetRegionStats
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{
						RegionID:        operator.RegionID,
						RegionName:      "测试区域",
						MerchantCount:   10,
						TotalOrders:     100,
						TotalCommission: 3000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp regionStatsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
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
				expectActiveOperatorAuth(store, user.ID, operator)
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
				expectActiveOperatorAuth(store, user.ID, operator)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID+999, false)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)

				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{}, db.ErrRecordNotFound)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					GetOperatorMerchantRanking(gomock.Any(), gomock.Any()).
					Return(merchants, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorMerchantRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
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
				expectActiveOperatorAuth(store, user.ID, operator)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MultiRegionAggregatesWhenRegionMissing",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				regionA := operator.RegionID
				regionB := operator.RegionID + 1
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, regionA, regionB)

				store.EXPECT().
					GetOperatorMerchantRanking(gomock.Any(), gomock.Any()).
					Times(2).
					DoAndReturn(func(_ context.Context, arg db.GetOperatorMerchantRankingParams) ([]db.GetOperatorMerchantRankingRow, error) {
						require.Equal(t, int32(20), arg.Limit)
						require.Equal(t, int32(0), arg.Offset)
						switch arg.RegionID {
						case regionA:
							return []db.GetOperatorMerchantRankingRow{{MerchantID: 10, MerchantName: "一区商户", OrderCount: 3, TotalSales: 3000, Commission: 120, AvgOrderAmount: 1000}}, nil
						case regionB:
							return []db.GetOperatorMerchantRankingRow{{MerchantID: 20, MerchantName: "二区商户", OrderCount: 5, TotalSales: 5000, Commission: 200, AvgOrderAmount: 1000}}, nil
						default:
							t.Fatalf("unexpected region id %d", arg.RegionID)
							return nil, nil
						}
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorMerchantRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 2)
				require.Equal(t, int64(20), resp[0].MerchantID)
				require.Equal(t, int64(10), resp[1].MerchantID)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					GetOperatorRiderRanking(gomock.Any(), gomock.Any()).
					Return(riders, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorRiderRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 1)
				require.Equal(t, "骑手1", resp[0].RiderName)
			},
		},
		{
			name:  "AllManagedRegionsAggregateTotalEarnings",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				secondRegion := operator.RegionID + 1
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID, secondRegion)

				store.EXPECT().
					GetOperatorRiderRanking(gomock.Any(), db.GetOperatorRiderRankingParams{
						RegionID: operator.RegionID,
						StartAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:    time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
						Limit:    20,
						Offset:   0,
					}).
					Times(1).
					Return([]db.GetOperatorRiderRankingRow{{
						RiderID:         1,
						RiderName:       "骑手1",
						DeliveryCount:   10,
						CompletedCount:  9,
						AvgDeliveryTime: 1200,
						TotalEarnings:   1000,
					}}, nil)

				store.EXPECT().
					GetOperatorRiderRanking(gomock.Any(), db.GetOperatorRiderRankingParams{
						RegionID: secondRegion,
						StartAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:    time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
						Limit:    20,
						Offset:   0,
					}).
					Times(1).
					Return([]db.GetOperatorRiderRankingRow{{
						RiderID:         1,
						RiderName:       "骑手1",
						DeliveryCount:   20,
						CompletedCount:  19,
						AvgDeliveryTime: 1800,
						TotalEarnings:   2500,
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []operatorRiderRankingRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 1)
				require.Equal(t, int64(3500), resp[0].TotalEarnings)
				require.Equal(t, int32(30), resp[0].DeliveryCount)
				require.Equal(t, int32(28), resp[0].CompletedCount)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					GetRegionDailyTrend(gomock.Any(), gomock.Any()).
					Return(trends, nil)

				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{
						TotalOrders:             5,
						TotalAmount:             25000,
						TotalOperatorCommission: 450,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp []regionDailyTrendRow
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp, 1)
				require.Equal(t, "2025-01-01", resp[0].Date)
				require.Equal(t, int64(450), resp[0].OperatorIncome)
			},
		},
		{
			name:  "ProfitSharingStatsFailureReturns500",
			query: "?start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					GetRegionDailyTrend(gomock.Any(), gomock.Any()).
					Return(trends, nil)

				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{}, errors.New("profit sharing stats failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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
		path          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			path: "/v1/operators/me/finance/overview",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

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

				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{
						TotalOrders:             100,
						TotalAmount:             100000,
						TotalOperatorCommission: 1800,
					}, nil)

				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{
						TotalOrders:             500,
						TotalAmount:             500000,
						TotalOperatorCommission: 9000,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp operatorFinanceOverviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, operator.RegionID, resp.RegionID)
				require.Equal(t, "测试区域", resp.RegionName)
				require.Equal(t, int64(1800), resp.CurrentMonth.OperatorIncome)
			},
		},
		{
			name: "InvalidRegionIDReturns400",
			path: "/v1/operators/me/finance/overview?region_id=bad",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "MultiRegionAggregatesWhenRegionMissing",
			path: "/v1/operators/me/finance/overview",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				regionA := operator.RegionID
				regionB := operator.RegionID + 1
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, regionA, regionB)

				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{TotalOrders: 10, TotalGmv: 1000, TotalCommission: 100}, nil)
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{TotalOrders: 20, TotalGmv: 2000, TotalCommission: 200}, nil)
				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{TotalOperatorCommission: 60}, nil)
				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{TotalOperatorCommission: 90}, nil)
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{TotalOrders: 100, TotalGmv: 10000, TotalCommission: 1000}, nil)
				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{TotalOrders: 200, TotalGmv: 20000, TotalCommission: 2000}, nil)
				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{TotalOperatorCommission: 600}, nil)
				store.EXPECT().
					GetOperatorProfitSharingStatsByRegion(gomock.Any(), gomock.Any()).
					Return(db.GetOperatorProfitSharingStatsByRegionRow{TotalOperatorCommission: 900}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp operatorFinanceOverviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(0), resp.RegionID)
				require.Equal(t, "全部区域", resp.RegionName)
				require.Equal(t, int32(30), resp.CurrentMonth.TotalOrders)
				require.Equal(t, int64(300), resp.CurrentMonth.TotalCommission)
				require.Equal(t, int64(150), resp.CurrentMonth.OperatorIncome)
				require.Equal(t, int64(30000), resp.Total.TotalGMV)
				require.Equal(t, int64(1500), resp.Total.OperatorIncome)
			},
		},
		{
			name: "MonthStatsFailureReturns500",
			path: "/v1/operators/me/finance/overview",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					GetRegion(gomock.Any(), operator.RegionID).
					Return(db.Region{ID: operator.RegionID, Name: "测试区域"}, nil)

				store.EXPECT().
					GetRegionStats(gomock.Any(), gomock.Any()).
					Return(db.GetRegionStatsRow{}, errors.New("month stats failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			request, err := http.NewRequest(http.MethodGet, tc.path, nil)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)

				store.EXPECT().
					GetRegionDailyTrend(gomock.Any(), gomock.Any()).
					Return(trends, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp operatorCommissionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
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
