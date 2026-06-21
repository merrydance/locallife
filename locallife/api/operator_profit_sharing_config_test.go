package api

import (
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

func TestListOperatorProfitSharingConfigsAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	config := randomProfitSharingConfig()
	config.RegionID = pgtype.Int8{Int64: operator.RegionID, Valid: true}
	secondRegionID := operator.RegionID + 1

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page=1&limit=2",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID)

				store.EXPECT().
					ListProfitSharingConfigsForRegions(gomock.Any(), gomock.Any()).
					Return([]db.ProfitSharingConfig{config}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorProfitSharingConfigsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int32(1), resp.Page)
				require.Equal(t, int32(2), resp.Limit)
			},
		},
		{
			name:  "DefaultAggregatesManagedRegions",
			query: "?page=1&limit=2",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				firstRegionConfig := config
				firstRegionConfig.RegionID = pgtype.Int8{Int64: operator.RegionID, Valid: true}
				secondRegionConfig := config
				secondRegionConfig.ID = config.ID + 1
				secondRegionConfig.RegionID = pgtype.Int8{Int64: secondRegionID, Valid: true}

				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, operator.RegionID, secondRegionID)

				store.EXPECT().
					ListProfitSharingConfigsForRegions(gomock.Any(), db.ListProfitSharingConfigsForRegionsParams{
						Status:      "",
						OrderSource: "",
						RegionIds:   []int64{operator.RegionID, secondRegionID},
						MerchantID:  int64(0),
						Limit:       int32(2),
						Offset:      int32(0),
					}).
					Return([]db.ProfitSharingConfig{firstRegionConfig, secondRegionConfig}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorProfitSharingConfigsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 2)
			},
		},
		{
			name:  "ExplicitRegionUsesAuthorizedSingleRegion",
			query: "?region_id=25&status=active&order_source=takeout&merchant_id=88&page=2&limit=5",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				explicitRegionID := int64(25)
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, explicitRegionID, true)

				store.EXPECT().
					ListProfitSharingConfigsForRegions(gomock.Any(), db.ListProfitSharingConfigsForRegionsParams{
						Status:      "active",
						OrderSource: "takeout",
						RegionIds:   []int64{explicitRegionID},
						MerchantID:  int64(88),
						Limit:       int32(5),
						Offset:      int32(5),
					}).
					Return([]db.ProfitSharingConfig{config}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorProfitSharingConfigsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int32(2), resp.Page)
				require.Equal(t, int32(5), resp.Limit)
			},
		},
		{
			name:  "DefaultAggregationExcludesSuspendedLegacyPrimaryRegion",
			query: "?page=1&limit=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				activeRegionConfig := config
				activeRegionConfig.RegionID = pgtype.Int8{Int64: secondRegionID, Valid: true}

				expectActiveOperatorAuth(store, user.ID, operator)
				store.EXPECT().
					ListOperatorRegions(gomock.Any(), operator.ID).
					Return([]db.ListOperatorRegionsRow{{
						OperatorID: operator.ID,
						RegionID:   secondRegionID,
						Status:     db.OperatorRegionStatusActive,
					}}, nil)
				store.EXPECT().
					GetOperatorRegion(gomock.Any(), db.GetOperatorRegionParams{
						OperatorID: operator.ID,
						RegionID:   operator.RegionID,
					}).
					Return(db.OperatorRegion{
						OperatorID: operator.ID,
						RegionID:   operator.RegionID,
						Status:     db.OperatorRegionStatusSuspended,
					}, nil)
				store.EXPECT().
					ListProfitSharingConfigsForRegions(gomock.Any(), db.ListProfitSharingConfigsForRegionsParams{
						Status:      "",
						OrderSource: "",
						RegionIds:   []int64{secondRegionID},
						MerchantID:  int64(0),
						Limit:       int32(20),
						Offset:      int32(0),
					}).
					Return([]db.ProfitSharingConfig{activeRegionConfig}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorProfitSharingConfigsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?limit=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)
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

			url := "/v1/operators/me/profit-sharing/configs" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
