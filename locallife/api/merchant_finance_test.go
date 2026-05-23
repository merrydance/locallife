package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
