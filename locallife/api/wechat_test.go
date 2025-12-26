package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestWechatLoginAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		buildStubs    func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "ExistingUser",
			body: map[string]interface{}{
				"code":        "valid_code",
				"device_id":   "test_device_id",
				"device_type": "ios",
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					Code2Session(gomock.Any(), gomock.Eq("valid_code")).
					Times(1).
					Return(&wechat.Code2SessionResponse{
						OpenID:     user.WechatOpenid,
						SessionKey: "test_session_key",
					}, nil)

				store.EXPECT().
					GetUserByWechatOpenID(gomock.Any(), gomock.Eq(user.WechatOpenid)).
					Times(1).
					Return(user, nil)

				store.EXPECT().
					UpsertUserDevice(gomock.Any(), gomock.Any()).
					Times(1)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:                    util.RandomInt(1, 1000),
						UserID:                user.ID,
						AccessToken:           "access_token",
						RefreshToken:          "refresh_token",
						RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
					}, nil)

				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NewUser",
			body: map[string]interface{}{
				"code":        "valid_code_new_user",
				"device_id":   "test_device_id",
				"device_type": "android",
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					Code2Session(gomock.Any(), gomock.Eq("valid_code_new_user")).
					Times(1).
					Return(&wechat.Code2SessionResponse{
						OpenID:     "new_open_id",
						SessionKey: "test_session_key",
					}, nil)

				store.EXPECT().
					GetUserByWechatOpenID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)

				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateUserTxResult{
						User:     user,
						UserRole: db.UserRole{UserID: user.ID, Role: "customer", Status: "active"},
					}, nil)

				store.EXPECT().
					UpsertUserDevice(gomock.Any(), gomock.Any()).
					Times(1)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:                    util.RandomInt(1, 1000),
						UserID:                user.ID,
						AccessToken:           "access_token",
						RefreshToken:          "refresh_token",
						RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
					}, nil)

				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "MissingCode",
			body: map[string]interface{}{},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					Code2Session(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					GetUserByWechatOpenID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WechatInvalidCode",
			body: map[string]interface{}{
				"code":        "error_code",
				"device_id":   "test_device_id",
				"device_type": "ios",
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					Code2Session(gomock.Any(), gomock.Eq("error_code")).
					Times(1).
					Return(nil, &wechat.APIError{Code: 40029, Msg: "invalid code"})

				store.EXPECT().
					GetUserByWechatOpenID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WechatNetworkError",
			body: map[string]interface{}{
				"code":        "network_error_code",
				"device_id":   "test_device_id",
				"device_type": "ios",
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					Code2Session(gomock.Any(), gomock.Eq("network_error_code")).
					Times(1).
					Return(nil, sql.ErrConnDone)

				store.EXPECT().
					GetUserByWechatOpenID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			tc.buildStubs(store, wechatClient)

			server := newTestServerWithWechat(t, store, wechatClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/auth/wechat-login"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestBindPhoneAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"phone": "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// GetUserByPhone returns no rows (phone available)
				store.EXPECT().
					GetUserByPhone(gomock.Any(), pgtype.Text{String: "13800138000", Valid: true}).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)

				updatedUser := user
				updatedUser.Phone = pgtype.Text{String: "13800138000", Valid: true}

				store.EXPECT().
					UpdateUser(gomock.Any(), db.UpdateUserParams{
						ID:    user.ID,
						Phone: pgtype.Text{String: "13800138000", Valid: true},
					}).
					Times(1).
					Return(updatedUser, nil)

				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"phone": "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InvalidPhone",
			body: map[string]interface{}{
				"phone": "invalid",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Handler will check if phone is available
				store.EXPECT().
					GetUserByPhone(gomock.Any(), pgtype.Text{String: "invalid", Valid: true}).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)

				// UpdateUser will be called but may fail due to DB constraint
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "MissingPhone",
			body: map[string]interface{}{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/auth/bind-phone"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
