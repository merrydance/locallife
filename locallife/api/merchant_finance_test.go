package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantFinanceOverviewAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
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
				startAt, err := time.Parse("2006-01-02", "2025-11-01")
				require.NoError(t, err)
				endAt, err := time.Parse("2006-01-02", "2025-11-30")
				require.NoError(t, err)
				endAt = endAt.Add(24*time.Hour - time.Nanosecond)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().
					GetMerchantFinanceOverview(gomock.Any(), gomock.Eq(db.GetMerchantFinanceOverviewParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(db.GetMerchantFinanceOverviewRow{
						CompletedOrders:                 100,
						PendingOrders:                   5,
						TotalGmv:                        1000000,
						TotalMerchantReceivableAmount:   950000,
						TotalPlatformServiceFeeAmount:   50000,
						TotalPaymentChannelFeeAmount:    3000,
						PendingMerchantReceivableAmount: 50000,
					}, nil)
				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Eq(db.GetMerchantPromotionExpensesParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(db.GetMerchantPromotionExpensesRow{
						PromoOrderCount: 10,
						TotalDiscount:   10000,
					}, nil)
				store.EXPECT().
					SumMerchantSettlementAdjustments(gomock.Any(), gomock.Eq(db.SumMerchantSettlementAdjustmentsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, float64(100), resp["completed_orders"])
				require.Equal(t, float64(1000000), resp["total_gmv"])
				require.Equal(t, float64(950000), resp["total_merchant_receivable_amount"])
				require.Equal(t, float64(50000), resp["total_platform_service_fee_amount"])
				require.Equal(t, float64(3000), resp["total_payment_channel_fee_amount"])
				require.Equal(t, float64(53000), resp["total_deduction_fee_amount"])
				require.Equal(t, float64(940000), resp["net_income"])
				require.NotContains(t, resp, "total_platform_fee")
				require.NotContains(t, resp, "total_operator_fee")
				require.NotContains(t, resp, "total_payment_fee")
				require.NotContains(t, resp, "total_service_fee")
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=2025/11/01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
			request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/overview"+tc.query, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMerchantFinanceRoutesHonorSelectedStaffMerchant(t *testing.T) {
	manager, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          4101,
		RegionID:    1,
		OwnerUserID: manager.ID + 100,
		Name:        "多店财务门店",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	otherMerchant := db.Merchant{
		ID:          4102,
		RegionID:    1,
		OwnerUserID: manager.ID + 200,
		Name:        "未选中门店",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	startAt, err := time.Parse("2006-01-02", "2026-06-01")
	require.NoError(t, err)
	endAt, err := time.Parse("2006-01-02", "2026-06-10")
	require.NoError(t, err)
	endAt = endAt.Add(24*time.Hour - time.Nanosecond)

	testCases := []struct {
		name       string
		path       string
		buildStubs func(store *mockdb.MockStore)
	}{
		{
			name: "overview",
			path: "/v1/merchant/finance/overview?start_date=2026-06-01&end_date=2026-06-10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantFinanceOverview(gomock.Any(), gomock.Eq(db.GetMerchantFinanceOverviewParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(db.GetMerchantFinanceOverviewRow{
						CompletedOrders:               3,
						TotalGmv:                      9000,
						TotalMerchantReceivableAmount: 7200,
						TotalPlatformServiceFeeAmount: 600,
						TotalPaymentChannelFeeAmount:  200,
					}, nil)
				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Eq(db.GetMerchantPromotionExpensesParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(db.GetMerchantPromotionExpensesRow{}, nil)
				store.EXPECT().
					SumMerchantSettlementAdjustments(gomock.Any(), gomock.Eq(db.SumMerchantSettlementAdjustmentsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(-500), nil)
			},
		},
		{
			name: "orders",
			path: "/v1/merchant/finance/orders?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.ListMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantFinanceOrdersRow{}, nil)
				store.EXPECT().
					CountMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.CountMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), nil)
			},
		},
		{
			name: "service-fees",
			path: "/v1/merchant/finance/service-fees?start_date=2026-06-01&end_date=2026-06-10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantServiceFeeDetail(gomock.Any(), gomock.Eq(db.GetMerchantServiceFeeDetailParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return([]db.GetMerchantServiceFeeDetailRow{}, nil)
			},
		},
		{
			name: "promotions",
			path: "/v1/merchant/finance/promotions?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantPromotionOrders(gomock.Any(), gomock.Eq(db.ListMerchantPromotionOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantPromotionOrdersRow{}, nil)
				store.EXPECT().
					CountMerchantPromotionOrders(gomock.Any(), gomock.Eq(db.CountMerchantPromotionOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), nil)
				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Eq(db.GetMerchantPromotionExpensesParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(db.GetMerchantPromotionExpensesRow{}, nil)
			},
		},
		{
			name: "daily",
			path: "/v1/merchant/finance/daily?start_date=2026-06-01&end_date=2026-06-10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantDailyFinance(gomock.Any(), gomock.Eq(db.GetMerchantDailyFinanceParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return([]db.GetMerchantDailyFinanceRow{}, nil)
				store.EXPECT().
					ListMerchantDailySettlementAdjustments(gomock.Any(), gomock.Eq(db.ListMerchantDailySettlementAdjustmentsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return([]db.ListMerchantDailySettlementAdjustmentsRow{}, nil)
			},
		},
		{
			name: "settlements",
			path: "/v1/merchant/finance/settlements?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantSettlements(gomock.Any(), gomock.Eq(db.ListMerchantSettlementsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantSettlementsRow{}, nil)
				store.EXPECT().
					CountMerchantSettlements(gomock.Any(), gomock.Eq(db.CountMerchantSettlementsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), nil)
				store.EXPECT().
					GetMerchantProfitSharingStats(gomock.Any(), gomock.Eq(db.GetMerchantProfitSharingStatsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(db.GetMerchantProfitSharingStatsRow{}, nil)
			},
		},
		{
			name: "settlement-timeline",
			path: "/v1/merchant/finance/settlement-timeline?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantSettlementTimeline(gomock.Any(), gomock.Eq(db.ListMerchantSettlementTimelineParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantSettlementTimelineRow{}, nil)
				store.EXPECT().
					CountMerchantSettlementTimeline(gomock.Any(), gomock.Eq(db.CountMerchantSettlementTimelineParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListMerchantsByOwner(gomock.Any(), gomock.Eq(manager.ID)).
				Return([]db.Merchant{}, nil)
			store.EXPECT().
				ListMerchantsByStaff(gomock.Any(), gomock.Eq(manager.ID)).
				Return([]db.Merchant{merchant, otherMerchant}, nil)
			store.EXPECT().
				GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
					MerchantID: merchant.ID,
					UserID:     manager.ID,
				})).
				Return(db.MerchantStaffRoleManager, nil)
			store.EXPECT().
				GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
					MerchantID: otherMerchant.ID,
					UserID:     manager.ID,
				})).
				AnyTimes().
				Return(db.MerchantStaffRoleManager, nil)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, tc.path, nil)
			require.NoError(t, err)
			request.Header.Set(merchantSelectionHeader, "4101")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		})
	}
}

func TestListMerchantSettlementsReturnsInternalErrorWhenCountFails(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	startAt, err := time.Parse("2006-01-02", "2026-06-01")
	require.NoError(t, err)
	endAt, err := time.Parse("2006-01-02", "2026-06-10")
	require.NoError(t, err)
	endAt = endAt.Add(24*time.Hour - time.Nanosecond)

	testCases := []struct {
		name       string
		path       string
		buildStubs func(store *mockdb.MockStore)
	}{
		{
			name: "without status filter",
			path: "/v1/merchant/finance/settlements?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantSettlements(gomock.Any(), gomock.Eq(db.ListMerchantSettlementsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantSettlementsRow{}, nil)
				store.EXPECT().
					CountMerchantSettlements(gomock.Any(), gomock.Eq(db.CountMerchantSettlementsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), errors.New("count settlements failed"))
			},
		},
		{
			name: "with status filter",
			path: "/v1/merchant/finance/settlements?start_date=2026-06-01&end_date=2026-06-10&page=1&limit=20&status=finished",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantSettlementsByStatus(gomock.Any(), gomock.Eq(db.ListMerchantSettlementsByStatusParams{
						MerchantID: merchant.ID,
						Status:     "finished",
						StartAt:    startAt,
						EndAt:      endAt,
						Limit:      20,
						Offset:     0,
					})).
					Return([]db.ListMerchantSettlementsByStatusRow{}, nil)
				store.EXPECT().
					CountMerchantSettlementsByStatus(gomock.Any(), gomock.Eq(db.CountMerchantSettlementsByStatusParams{
						MerchantID: merchant.ID,
						Status:     "finished",
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Return(int64(0), errors.New("count settlements by status failed"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
			tc.buildStubs(store)
			store.EXPECT().
				GetMerchantProfitSharingStats(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfitSharingStatsRow{}, nil)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, tc.path, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
		})
	}
}

func TestMerchantSettlementTimelineIncludesAdjustments(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	createdAt := time.Date(2026, time.June, 6, 15, 30, 0, 0, time.UTC)
	startAt, err := time.Parse("2006-01-02", "2026-06-01")
	require.NoError(t, err)
	endAt, err := time.Parse("2006-01-02", "2026-06-10")
	require.NoError(t, err)
	endAt = endAt.Add(24*time.Hour - time.Nanosecond)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		ListMerchantSettlementTimeline(gomock.Any(), gomock.Eq(db.ListMerchantSettlementTimelineParams{
			MerchantID: merchant.ID,
			StartAt:    startAt,
			EndAt:      endAt,
			Limit:      20,
			Offset:     0,
		})).
		Return([]db.ListMerchantSettlementTimelineRow{
			{
				RecordType:               "adjustment",
				ID:                       991,
				PaymentOrderID:           0,
				OrderSource:              "settlement_adjustment",
				TotalAmount:              -2300,
				PlatformServiceFeeAmount: 0,
				PaymentChannelFeeAmount:  0,
				MerchantReceivableAmount: -2300,
				OutOrderNo:               "ADJ-991",
				Status:                   "finished",
				CreatedAt:                createdAt,
				AdjustmentType:           pgtype.Text{String: "refund_return", Valid: true},
				RelatedType:              pgtype.Text{String: "refund_order", Valid: true},
				RelatedID:                pgtype.Int8{Int64: 7201, Valid: true},
			},
		}, nil)
	store.EXPECT().
		CountMerchantSettlementTimeline(gomock.Any(), gomock.Eq(db.CountMerchantSettlementTimelineParams{
			MerchantID: merchant.ID,
			StartAt:    startAt,
			EndAt:      endAt,
		})).
		Return(int64(1), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/settlement-timeline?start_date=2026-06-01&end_date=2026-06-10", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var resp merchantSettlementTimelineResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Timeline, 1)
	require.Equal(t, "adjustment", resp.Timeline[0].RecordType)
	require.Equal(t, int64(-2300), resp.Timeline[0].MerchantReceivableAmount)
	require.Equal(t, "refund_return", resp.Timeline[0].AdjustmentType)
	require.Equal(t, "refund_order", resp.Timeline[0].RelatedType)
	require.Equal(t, int64(7201), resp.Timeline[0].RelatedID)
}
