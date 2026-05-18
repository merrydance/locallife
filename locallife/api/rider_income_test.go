package api

import (
	"fmt"
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

func TestGetRiderIncomeSummaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	startAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 4, 2, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "start_date=2026-04-01&end_date=2026-04-02",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Return(rider, nil)
				store.EXPECT().
					GetRiderProfitSharingStats(gomock.Any(), db.GetRiderProfitSharingStatsParams{
						RiderID: riderID,
						StartAt: startAt,
						EndAt:   endAt,
					}).
					Return(db.GetRiderProfitSharingStatsRow{
						TotalDeliveries:       2,
						TotalRiderIncome:      1800,
						TotalDeliveryFee:      2000,
						TotalRiderGrossAmount: 2000,
						TotalRiderPaymentFee:  12,
					}, nil)
				store.EXPECT().
					GetRiderProfitSharingStatusSummary(gomock.Any(), db.GetRiderProfitSharingStatusSummaryParams{
						RiderID: riderID,
						StartAt: startAt,
						EndAt:   endAt,
					}).
					Return([]db.GetRiderProfitSharingStatusSummaryRow{
						{Status: db.ProfitSharingOrderStatusFinished, OrderCount: 2, RiderAmount: 1800, DeliveryFee: 2000, RiderGrossAmount: 2000, RiderPaymentFee: 12},
						{Status: db.ProfitSharingOrderStatusPending, OrderCount: 1, RiderAmount: 600, DeliveryFee: 700, RiderGrossAmount: 700, RiderPaymentFee: 5},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response riderIncomeSummaryResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(2), response.TotalDeliveries)
				require.Equal(t, int64(1800), response.TotalRiderIncome)
				require.Equal(t, int64(2000), response.TotalRiderGrossAmount)
				require.Equal(t, int64(12), response.TotalRiderPaymentFee)
				require.Len(t, response.StatusSummary, 4)
				require.Equal(t, db.ProfitSharingOrderStatusPending, response.StatusSummary[0].Status)
				require.Equal(t, int64(1), response.StatusSummary[0].OrderCount)
				require.Equal(t, int64(5), response.StatusSummary[0].RiderPaymentFee)
			},
		},
		{
			name:  "RiderNotFound",
			query: "start_date=2026-04-01&end_date=2026-04-02",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:  "InvalidDateRange",
			query: "start_date=2026-04-03&end_date=2026-04-02",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := fmt.Sprintf("/v1/rider/income/summary?%s", testCase.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			testCase.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			testCase.checkResponse(t, recorder)
		})
	}
}

func TestListRiderIncomeLedgerAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	createdAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	finishedAt := pgtype.Timestamptz{Time: createdAt.Add(time.Hour), Valid: true}
	startAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 4, 2, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}
	status := pgtype.Text{String: db.ProfitSharingOrderStatusFinished, Valid: true}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "start_date=2026-04-01&end_date=2026-04-02&status=finished&page_id=2&page_size=1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Return(rider, nil)
				store.EXPECT().
					CountRiderProfitSharingOrders(gomock.Any(), db.CountRiderProfitSharingOrdersParams{
						RiderID: riderID,
						Status:  status,
						StartAt: startAt,
						EndAt:   endAt,
					}).
					Return(int64(3), nil)
				store.EXPECT().
					ListRiderProfitSharingOrders(gomock.Any(), db.ListRiderProfitSharingOrdersParams{
						RiderID: riderID,
						Status:  status,
						StartAt: startAt,
						EndAt:   endAt,
						Offset:  1,
						Limit:   1,
					}).
					Return([]db.ListRiderProfitSharingOrdersRow{
						{
							ID:                  101,
							PaymentOrderID:      201,
							MerchantID:          301,
							OrderID:             pgtype.Int8{Int64: 401, Valid: true},
							OrderNo:             "ORDER202604020001",
							MerchantName:        "测试商户",
							Status:              db.ProfitSharingOrderStatusFinished,
							TotalAmount:         5000,
							DeliveryFee:         800,
							RiderGrossAmount:    800,
							RiderPaymentFee:     5,
							RiderAmount:         720,
							DistributableAmount: 4200,
							OutOrderNo:          "PS202604020001",
							SharingOrderID:      pgtype.Text{String: "WXPS001", Valid: true},
							FinishedAt:          finishedAt,
							CreatedAt:           createdAt,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response riderIncomeLedgerResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(3), response.Total)
				require.Equal(t, int32(2), response.PageID)
				require.Equal(t, int32(1), response.PageSize)
				require.True(t, response.HasMore)
				require.Len(t, response.Items, 1)
				require.Equal(t, int64(800), response.Items[0].RiderGrossAmount)
				require.Equal(t, int64(5), response.Items[0].RiderPaymentFee)
				require.Equal(t, int64(720), response.Items[0].RiderAmount)
				require.Equal(t, "WXPS001", response.Items[0].SharingOrderID)
				require.NotNil(t, response.Items[0].FinishedAt)
			},
		},
		{
			name:  "InvalidStatus",
			query: "start_date=2026-04-01&end_date=2026-04-02&status=paid&page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			query:      "start_date=2026-04-01&end_date=2026-04-02&page_id=1&page_size=10",
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/rider/income/ledger?%s", testCase.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			testCase.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			testCase.checkResponse(t, recorder)
		})
	}
}

func TestGetRiderIncomeDailyAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	startAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 4, 2, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), user.ID).
		Return(rider, nil)
	store.EXPECT().
		GetRiderDailyIncome(gomock.Any(), db.GetRiderDailyIncomeParams{
			RiderID: riderID,
			StartAt: startAt,
			EndAt:   endAt,
		}).
		Return([]db.GetRiderDailyIncomeRow{
			{
				Date:             pgtype.Date{Time: startAt, Valid: true},
				DeliveryCount:    2,
				DailyIncome:      1600,
				RiderGrossAmount: 1700,
				RiderPaymentFee:  10,
			},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/rider/income/daily?start_date=2026-04-01&end_date=2026-04-02", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response riderIncomeDailyResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Items, 1)
	require.Equal(t, "2026-04-01", response.Items[0].Date)
	require.Equal(t, int64(1600), response.Items[0].DailyIncome)
	require.Equal(t, int64(1700), response.Items[0].RiderGrossAmount)
	require.Equal(t, int64(10), response.Items[0].RiderPaymentFee)
}
