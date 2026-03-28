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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetCurrentUserAPI(t *testing.T) {
	user, _ := randomUser(t)

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
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				store.EXPECT().
					ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return([]db.UserRole{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "UserNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InternalError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/users/me"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateCurrentUserAPI(t *testing.T) {
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
				"full_name":             "New Name",
				"avatar_media_asset_id": int64(101),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				updatedUser := user
				updatedUser.FullName = "Updated Name"
				updatedUser.AvatarMediaAssetID = pgtype.Int8{Int64: 101, Valid: true}

				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedUser, nil)
				store.EXPECT().
					ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return([]db.UserRole{}, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), gomock.Eq(int64(101))).
					Times(1).
					Return(db.MediaAsset{ID: 101, ObjectKey: "user/avatar/101/profile.jpg", Visibility: "public", ModerationStatus: "approved"}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "PartialUpdate",
			body: map[string]interface{}{
				"full_name": "New Name Only",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				updatedUser := user
				updatedUser.FullName = "New Name Only"

				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedUser, nil)
				store.EXPECT().
					ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return([]db.UserRole{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"full_name": "New Name",
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
			name: "InvalidBody",
			body: map[string]interface{}{
				"full_name": 12345, // 类型错误：应该是string
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					ListUserRoles(gomock.Any(), gomock.Any()).
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

			url := "/v1/users/me"
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func requireBodyMatchUser(t *testing.T, body *bytes.Buffer, user db.User) {
	data := body.Bytes()

	var gotUser userResponse
	requireUnmarshalAPIResponseData(t, data, &gotUser)
	require.Equal(t, user.ID, gotUser.ID)
	require.Equal(t, user.WechatOpenid, gotUser.WechatOpenID)
}

// TestGetCurrentUserAPI_WithAvatarURL — Phase 5.5
// 当用户 avatar_media_asset_id 有值时，GET /v1/users/me 应返回包含 CDN 地址的 avatar_url
func TestGetCurrentUserAPI_WithAvatarURL(t *testing.T) {
	user, _ := randomUser(t)
	const avatarAssetID int64 = 55
	user.AvatarMediaAssetID = pgtype.Int8{Int64: avatarAssetID, Valid: true}

	avatarAsset := db.MediaAsset{
		ID:               avatarAssetID,
		ObjectKey:        "user/avatar/55/avatar.jpg",
		Visibility:       "public",
		ModerationStatus: "approved",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetUser(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(user, nil)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return([]db.UserRole{}, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), gomock.Eq(avatarAssetID)).
		Times(1).
		Return(avatarAsset, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.AvatarURL)
	require.Contains(t, *resp.AvatarURL, "https://cdn.test.example.com")
	require.Contains(t, *resp.AvatarURL, "user/avatar/55/avatar.jpg")
}

// TestUpdateCurrentUserAPI_WithAvatarAssetID — Phase 5.5
// PATCH /v1/users/me 提交 avatar_media_asset_id 后，响应中 avatar_url 应指向 CDN 地址
func TestUpdateCurrentUserAPI_WithAvatarAssetID(t *testing.T) {
	user, _ := randomUser(t)
	const avatarAssetID int64 = 88

	updatedUser := user
	updatedUser.AvatarMediaAssetID = pgtype.Int8{Int64: avatarAssetID, Valid: true}

	avatarAsset := db.MediaAsset{
		ID:               avatarAssetID,
		ObjectKey:        "user/avatar/88/profile.jpg",
		Visibility:       "public",
		ModerationStatus: "approved",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		UpdateUser(gomock.Any(), gomock.Any()).
		Times(1).
		Return(updatedUser, nil)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return([]db.UserRole{}, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), gomock.Eq(avatarAssetID)).
		Times(1).
		Return(avatarAsset, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	body := map[string]interface{}{"avatar_media_asset_id": avatarAssetID}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPatch, "/v1/users/me", bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.AvatarURL)
	require.Contains(t, *resp.AvatarURL, "https://cdn.test.example.com")
	require.Contains(t, *resp.AvatarURL, "user/avatar/88/profile.jpg")
}

func TestGetCurrentUserAPI_WithPendingAvatarVisibleToOwner(t *testing.T) {
	user, _ := randomUser(t)
	const avatarAssetID int64 = 109
	user.AvatarMediaAssetID = pgtype.Int8{Int64: avatarAssetID, Valid: true}

	avatarAsset := db.MediaAsset{
		ID:               avatarAssetID,
		ObjectKey:        "user/avatar/109/profile.jpg",
		Visibility:       "public",
		MediaCategory:    "avatar",
		ModerationStatus: "pending",
		UploadedBy:       user.ID,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetUser(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(user, nil)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return([]db.UserRole{}, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), gomock.Eq(avatarAssetID)).
		Times(1).
		Return(avatarAsset, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.AvatarURL)
	require.Contains(t, *resp.AvatarURL, "https://cdn.test.example.com")
	require.Contains(t, *resp.AvatarURL, "user/avatar/109/profile.jpg")
}
