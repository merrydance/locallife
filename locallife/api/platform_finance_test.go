package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	mockosp "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetPlatformAccountBalanceAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "RealtimeOK",
			query: "?account_type=operation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
			},
		},
		{
			name:  "DayEndOK",
			query: "?date=2026-04-05&account_type=fees",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
			},
		},
		{
			name:  "InvalidAccountType",
			query: "?account_type=deposit",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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
			request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance"+tc.query, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetPlatformAccountBalanceAPINoAuthReturnsExplicitMessage(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func TestGetPlatformAccountBalanceAPIEcommerceClientUnavailable(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func TestGetPlatformAccountBalanceAPIOrdinaryServiceProviderGatesUnsupportedFundManagement(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl))

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
}

func TestGetPlatformBaofuSettlementStatusAPI_IncludesSanitizedReadiness(t *testing.T) {
	admin, _ := randomUser(t)
	binding := db.BaofuAccountBinding{
		ID:           5301,
		OwnerType:    db.BaofuAccountOwnerTypePlatform,
		OwnerID:      platformBaofuAccountOwnerID,
		AccountType:  db.BaofuAccountTypePlatform,
		ContractNo:   pgtype.Text{String: "PF5301ABC", Valid: true},
		SharingMerID: pgtype.Text{String: "PS5301XYZ", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypePlatform,
			OwnerID:   platformBaofuAccountOwnerID,
		}).
		Times(1).
		Return(binding, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/settlement-account/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "PF5301ABC")
	require.NotContains(t, recorder.Body.String(), "PS5301XYZ")
	require.NotContains(t, recorder.Body.String(), "contractNo")
	require.NotContains(t, recorder.Body.String(), "sharingMerId")

	var resp platformBaofuSettlementStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.SettlementAccount)
	require.Equal(t, "ready", resp.SettlementAccount.State)
	require.Equal(t, "结算账户可用", resp.SettlementAccount.Label)
	require.True(t, resp.SettlementAccount.PaymentReady)
	require.Equal(t, "PF5****ABC", resp.MaskedContractNo)
	require.Equal(t, "PS5****XYZ", resp.MaskedSharingMerID)
}
