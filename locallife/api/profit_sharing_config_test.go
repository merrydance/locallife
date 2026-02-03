package api

import (
	"bytes"
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

func randomProfitSharingConfig() db.ProfitSharingConfig {
	return db.ProfitSharingConfig{
		ID:           1,
		Status:       "active",
		OrderSource:  "takeout",
		RegionID:     pgtype.Int8{Int64: 10, Valid: true},
		MerchantID:   pgtype.Int8{Int64: 0, Valid: false},
		PlatformRate: 2,
		OperatorRate: 3,
		RiderEnabled: true,
		Priority:     100,
		EffectiveAt:  pgtype.Timestamptz{Valid: false},
		ExpiresAt:    pgtype.Timestamptz{Valid: false},
		CreatedBy:    pgtype.Int8{Int64: 99, Valid: true},
		CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestCreateProfitSharingConfigAPI(t *testing.T) {
	admin, _ := randomUser(t)

	requestBody := map[string]any{
		"order_source":  "takeout",
		"platform_rate": 2,
		"operator_rate": 3,
		"region_id":     10,
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		body          []byte
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: bodyBytes,
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
					CreateProfitSharingConfigTx(gomock.Any(), gomock.Any()).
					Return(db.CreateProfitSharingConfigTxResult{Config: randomProfitSharingConfig()}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				var resp profitSharingConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(1), resp.ID)
				require.Equal(t, "active", resp.Status)
			},
		},
		{
			name: "InvalidPayload",
			body: []byte(`{"platform_rate":2}`),
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
			name: "UnauthorizedNonAdmin",
			body: bodyBytes,
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

			url := "/v1/platform/profit-sharing/configs"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(tc.body))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListProfitSharingConfigsAPI(t *testing.T) {
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
			query: "?page=1&limit=2",
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
					ListProfitSharingConfigs(gomock.Any(), gomock.Any()).
					Return([]db.ProfitSharingConfig{randomProfitSharingConfig()}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listProfitSharingConfigsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int32(1), resp.Page)
				require.Equal(t, int32(2), resp.Limit)
			},
		},
		{
			name:  "InvalidLimit",
			query: "?limit=0",
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

			url := "/v1/platform/profit-sharing/configs" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateProfitSharingConfigAPI(t *testing.T) {
	admin, _ := randomUser(t)

	requestBody := map[string]any{
		"status": "disabled",
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		configID      int64
		body          []byte
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			configID: 1,
			body:     bodyBytes,
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
					UpdateProfitSharingConfigTx(gomock.Any(), gomock.Any()).
					Return(db.UpdateProfitSharingConfigTxResult{Config: randomProfitSharingConfig()}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "InvalidID",
			configID: 0,
			body:     bodyBytes,
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

			url := "/v1/platform/profit-sharing/configs/" + fmt.Sprint(tc.configID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(tc.body))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDisableProfitSharingConfigAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		configID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			configID: 1,
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
					UpdateProfitSharingConfigStatusTx(gomock.Any(), gomock.Any()).
					Return(db.UpdateProfitSharingConfigStatusTxResult{Config: randomProfitSharingConfig()}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "InvalidID",
			configID: 0,
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

			url := "/v1/platform/profit-sharing/configs/" + fmt.Sprint(tc.configID) + "/disable"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
