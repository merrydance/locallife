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
	"github.com/merrydance/locallife/util"
)

// ============================================================================
// 运营商管理商户测试
// ============================================================================

func TestListOperatorMerchantsAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	merchants := []db.Merchant{
		randomMerchantInRegionForOp(operator.RegionID),
		randomMerchantInRegionForOp(operator.RegionID),
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)

				store.EXPECT().
					ListMerchantsByRegion(gomock.Any(), db.ListMerchantsByRegionParams{
						RegionID: operator.RegionID,
						Limit:    20,
						Offset:   0,
					}).
					Return(merchants, nil)

				store.EXPECT().
					CountMerchantsByRegion(gomock.Any(), operator.RegionID).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorMerchantsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Merchants, 2)
				require.Equal(t, int64(2), resp.Total)
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "?status=approved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)

				store.EXPECT().
					ListMerchantsByRegionWithStatus(gomock.Any(), db.ListMerchantsByRegionWithStatusParams{
						RegionID: operator.RegionID,
						Column2:  "approved",
						Limit:    20,
						Offset:   0,
					}).
					Return(merchants, nil)

				store.EXPECT().
					CountMerchantsByRegionWithStatus(gomock.Any(), db.CountMerchantsByRegionWithStatusParams{
						RegionID: operator.RegionID,
						Column2:  "approved",
					}).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NoAuthorization",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth header
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "?status=invalid_status",
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
			name:  "InvalidPage",
			query: "?page=-1",
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
			name:  "InvalidLimit",
			query: "?limit=200",
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

			url := "/v1/operator/merchants" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetOperatorMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	merchant := randomMerchantInRegionForOp(operator.RegionID)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Return(merchant, nil)

				expectOperatorManagesRegion(store, operator, merchant.RegionID, true)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantDetailResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, merchant.ID, resp.ID)
				require.Equal(t, merchant.Name, resp.Name)
			},
		},
		{
			name:       "MerchantNotFound",
			merchantID: 9999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetMerchant(gomock.Any(), int64(9999)).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "MerchantNotInRegion",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				differentRegionMerchant := merchant
				differentRegionMerchant.RegionID = operator.RegionID + 1
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Return(differentRegionMerchant, nil)

				expectOperatorManagesRegion(store, operator, differentRegionMerchant.RegionID, false)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
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

			url := fmt.Sprintf("/v1/operator/merchants/%d", tc.merchantID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 运营商管理骑手测试
// ============================================================================

func TestListOperatorRidersAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	riders := []db.Rider{
		randomRiderInRegionForOp(operator.RegionID),
		randomRiderInRegionForOp(operator.RegionID),
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
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					ListRidersByRegion(gomock.Any(), db.ListRidersByRegionParams{
						RegionID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
						Limit:    20,
						Offset:   0,
					}).
					Return(riders, nil)

				store.EXPECT().
					CountRidersByRegion(gomock.Any(), pgtype.Int8{Int64: operator.RegionID, Valid: true}).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listOperatorRidersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Riders, 2)
				require.Equal(t, int64(2), resp.Total)
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "?status=active",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					ListRidersByRegionWithStatus(gomock.Any(), db.ListRidersByRegionWithStatusParams{
						RegionID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
						Status:   "active",
						Limit:    20,
						Offset:   0,
					}).
					Return(riders, nil)

				store.EXPECT().
					CountRidersByRegionWithStatus(gomock.Any(), db.CountRidersByRegionWithStatusParams{
						RegionID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
						Status:   "active",
					}).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

			url := "/v1/operator/riders" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetOperatorRiderAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	rider := randomRiderInRegionForOp(operator.RegionID)

	testCases := []struct {
		name          string
		riderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			riderID: rider.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetRider(gomock.Any(), rider.ID).
					Return(rider, nil)

				expectOperatorManagesRegion(store, operator, rider.RegionID.Int64, true)

			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderDetailResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, rider.ID, resp.ID)
				require.Equal(t, rider.RealName, resp.RealName)
			},
		},
		{
			name:    "RiderNotFound",
			riderID: 9999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetRider(gomock.Any(), int64(9999)).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "RiderNotInRegion",
			riderID: rider.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				differentRegionRider := rider
				differentRegionRider.RegionID = pgtype.Int8{Int64: operator.RegionID + 1, Valid: true}

				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetRider(gomock.Any(), rider.ID).
					Return(differentRegionRider, nil)

				expectOperatorManagesRegion(store, operator, differentRegionRider.RegionID.Int64, false)

			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
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

			url := fmt.Sprintf("/v1/operator/riders/%d", tc.riderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ============================================================================
// 辅助函数 - 运营商商户/骑手管理测试专用
// ============================================================================

func randomMerchantInRegionForOp(regionID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: util.RandomInt(1, 1000),
		Name:        util.RandomString(10),
		Phone:       "13800138000",
		Address:     util.RandomString(20),
		Status:      "approved",
		IsOpen:      true,
		RegionID:    regionID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func randomRiderInRegionForOp(regionID int64) db.Rider {
	return db.Rider{
		ID:            util.RandomInt(1, 1000),
		UserID:        util.RandomInt(1, 1000),
		RealName:      util.RandomString(6),
		Phone:         "13900139000",
		IDCardNo:      "110101199001011234",
		Status:        "active",
		IsOnline:      false,
		DepositAmount: 50000,
		TotalOrders:   100,
		TotalEarnings: 100000,
		CreditScore:   80,
		RegionID:      pgtype.Int8{Int64: regionID, Valid: true},
		CreatedAt:     time.Now(),
	}
}
