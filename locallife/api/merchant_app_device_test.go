package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRegisterMerchantAppDeviceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		body          map[string]any
		setupAuth     func(t *testing.T, request *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]any{
				"device_id":    "device-1",
				"push_token":   "token-1",
				"platform":     "android",
				"provider":     "xiaomi",
				"device_model": "Redmi K70",
				"os_version":   "Android 15",
				"app_version":  "1.0.0",
			},
			setupAuth: func(t *testing.T, request *http.Request, server *Server) {
				addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().
					RegisterMerchantAppDeviceTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.RegisterMerchantAppDeviceParams) (db.MerchantAppDevice, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.Equal(t, user.ID, arg.UserID)
						require.Equal(t, "device-1", arg.DeviceID)
						require.Equal(t, "token-1", arg.PushToken)
						require.Equal(t, db.MerchantAppDeviceProviderXiaomi, arg.Provider)
						return db.MerchantAppDevice{MerchantID: arg.MerchantID, UserID: arg.UserID, DeviceID: arg.DeviceID, Provider: arg.Provider}, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response registerMerchantAppDeviceResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "device-1", response.DeviceID)
				require.Equal(t, db.MerchantAppDeviceProviderXiaomi, response.Provider)
				require.True(t, response.Registered)
				require.Equal(t, merchant.ID, response.MerchantID)
				require.Equal(t, merchant.Name, response.MerchantName)
			},
		},
		{
			name: "UnsupportedProvider",
			body: map[string]any{
				"device_id":  "device-1",
				"push_token": "token-1",
				"platform":   "android",
				"provider":   "jpush",
			},
			setupAuth: func(t *testing.T, request *http.Request, server *Server) {
				addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, "unsupported provider", response.Message)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]any{
				"device_id":  "device-1",
				"push_token": "token-1",
				"platform":   "android",
			},
			setupAuth: func(t *testing.T, request *http.Request, server *Server) {},
			buildStubs: func(store *mockdb.MockStore) {
				expectNoMerchantAccessResolution(store)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			server := newTestServer(t, store)
			tc.buildStubs(store)

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/device/register", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestHeartbeatMerchantAppDeviceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		UpdateMerchantAppDeviceHeartbeatTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantAppDeviceHeartbeatParams) (db.MerchantAppDevice, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, "device-1", arg.DeviceID)
			require.Equal(t, pgtype.Text{String: db.MerchantAppDeviceProviderVivo, Valid: true}, arg.Provider)
			return db.MerchantAppDevice{MerchantID: arg.MerchantID, DeviceID: arg.DeviceID, Provider: arg.Provider.String}, nil
		})

	body, err := json.Marshal(map[string]any{
		"device_id":   "device-1",
		"provider":    "vivo",
		"app_version": "1.0.1",
	})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, "/v1/merchant/device/heartbeat", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response heartbeatMerchantAppDeviceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "device-1", response.DeviceID)
	require.True(t, response.Heartbeat)
}

func TestUnregisterMerchantAppDeviceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		UnregisterMerchantAppDevice(gomock.Any(), db.UnregisterMerchantAppDeviceParams{MerchantID: merchant.ID, DeviceID: "device-1"}).
		Times(1).
		Return(int64(1), nil)

	request, err := http.NewRequest(http.MethodDelete, "/v1/merchant/device/device-1", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response unregisterMerchantAppDeviceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "device-1", response.DeviceID)
	require.True(t, response.Unregistered)
}
