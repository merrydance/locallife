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

func TestGetRiderWorkbenchSummaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = db.RiderStatusActive
	rider.IsOnline = true
	rider.DepositAmount = 30000
	rider.FrozenDeposit = 1000
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}
	activeDelivery := db.Delivery{ID: 11, OrderID: 22, Status: "delivering", DeliveryFee: 800, RiderEarnings: 720, PickupAddress: "取餐地址", DeliveryAddress: "送达地址", CreatedAt: time.Now().UTC()}
	paymentOrder := db.PaymentOrder{
		ID:                    31,
		OrderID:               pgtype.Int8{Int64: activeDelivery.OrderID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:               41,
		PaymentOrderID:   paymentOrder.ID,
		Status:           db.ProfitSharingOrderStatusPending,
		RiderGrossAmount: 800,
		RiderPaymentFee:  5,
		RiderAmount:      795,
	}

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
				store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Return(rider, nil)
				store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), riderID).Return([]db.Delivery{activeDelivery}, nil)
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: activeDelivery.OrderID, Valid: true},
						BusinessType: db.ExternalPaymentBusinessOwnerOrder,
					}).
					Return(paymentOrder, nil)
				store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
				store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), user.ID).Return(int64(0), nil)
				expectRiderThresholdFromRegionRule(store, rider, 200*fenPerYuan)
				store.EXPECT().CountDeliveryPool(gomock.Any()).Return(int64(3), nil)
				store.EXPECT().CountRiderCompletedDeliveriesInRange(gomock.Any(), gomock.Any()).Return(int64(2), nil)
				store.EXPECT().GetRiderProfitSharingStats(gomock.Any(), gomock.Any()).Return(db.GetRiderProfitSharingStatsRow{TotalDeliveries: 2, TotalRiderIncome: 1800, TotalDeliveryFee: 2000}, nil)
				store.EXPECT().GetRiderProfitSharingStatusSummary(gomock.Any(), gomock.Any()).Return([]db.GetRiderProfitSharingStatusSummaryRow{{Status: db.ProfitSharingOrderStatusPending, OrderCount: 1, RiderAmount: 600}}, nil)
				store.EXPECT().CountRiderClaimsForRider(gomock.Any(), db.CountRiderClaimsForRiderParams{RiderID: riderID, Bucket: pgtype.Text{String: "pending_action", Valid: true}}).Return(int64(4), nil)
				store.EXPECT().CountUnreadNotifications(gomock.Any(), user.ID).Return(int64(5), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response riderWorkbenchSummaryResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "delivering", response.RiderStatus.OnlineStatus)
				require.True(t, response.RiderStatus.CanGoOnline)
				require.Equal(t, 1, response.CurrentDeliveries.ActiveCount)
				require.Equal(t, int64(3), response.OrderPool.AvailableCount)
				require.Equal(t, int64(1800), response.Income.TotalRiderIncome)
				require.Equal(t, int64(4), response.Claims.PendingActionCount)
				require.Equal(t, int64(5), response.Notifications.UnreadCount)
				require.True(t, apiSectionAvailable(response.Sections, "income"))

				var raw map[string]any
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &raw)
				currentDeliveries, ok := raw["current_deliveries"].(map[string]any)
				require.True(t, ok)
				items, ok := currentDeliveries["items"].([]any)
				require.True(t, ok)
				require.Len(t, items, 1)
				item, ok := items[0].(map[string]any)
				require.True(t, ok)
				require.Equal(t, float64(800), item["rider_gross_amount"])
				require.Equal(t, float64(5), item["rider_payment_fee"])
				require.Equal(t, float64(795), item["rider_net_earnings"])
				require.Equal(t, float64(profitSharingOrder.ID), item["profit_sharing_order_id"])
				require.Equal(t, profitSharingOrder.Status, item["profit_sharing_status"])
			},
		},
		{
			name: "PartialDegradeStillReturnsOK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Return(rider, nil)
				store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), riderID).Return(nil, errors.New("delivery unavailable"))
				store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), user.ID).Return(int64(0), nil)
				expectRiderThresholdFromRegionRule(store, rider, 200*fenPerYuan)
				store.EXPECT().CountDeliveryPool(gomock.Any()).Return(int64(3), nil)
				store.EXPECT().CountRiderCompletedDeliveriesInRange(gomock.Any(), gomock.Any()).Return(int64(2), nil)
				store.EXPECT().GetRiderProfitSharingStats(gomock.Any(), gomock.Any()).Return(db.GetRiderProfitSharingStatsRow{}, nil)
				store.EXPECT().GetRiderProfitSharingStatusSummary(gomock.Any(), gomock.Any()).Return(nil, nil)
				store.EXPECT().CountRiderClaimsForRider(gomock.Any(), gomock.Any()).Return(int64(0), nil)
				store.EXPECT().CountUnreadNotifications(gomock.Any(), user.ID).Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response riderWorkbenchSummaryResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.False(t, apiSectionAvailable(response.Sections, "current_deliveries"))
				require.True(t, apiSectionAvailable(response.Sections, "order_pool"))
				require.Equal(t, int64(3), response.OrderPool.AvailableCount)
			},
		},
		{
			name: "RiderNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoCurrentRegion",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				noRegionRider := rider
				noRegionRider.RegionID = pgtype.Int8{Valid: false}
				store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Return(noRegionRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			testCase.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodGet, "/v1/rider/workbench/summary", nil)
			require.NoError(t, err)

			testCase.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			testCase.checkResponse(t, recorder)
		})
	}
}

func apiSectionAvailable(sections []riderWorkbenchSectionStatusResponse, section string) bool {
	for _, item := range sections {
		if item.Section == section {
			return item.Available
		}
	}
	return false
}
