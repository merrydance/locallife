package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestUploadMerchantImageAPI_Gone verifies the old upload endpoint returns 410 Gone.
// The endpoint has been replaced by the media upload flow (POST /v1/media/upload-sessions).
func TestUploadMerchantImageAPI_Gone(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)
	server := newTestServerWithWechat(t, store, wechatClient)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchants/images/upload", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusGone, recorder.Code)
}

func TestGetCurrentMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

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
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					MinTimes(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, merchant.ID, response.ID)
				require.Equal(t, merchant.Name, response.Name)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					MinTimes(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					ListMerchantsByStaff(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return([]db.Merchant{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := "/v1/merchants/me"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateCurrentMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"name":        "Updated Name",
				"description": "Updated Description",
				"version":     1, // ✅ 添加version字段
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					MinTimes(1).
					Return(merchant, nil)

				store.EXPECT().
					UpdateMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{
						ID:          merchant.ID,
						OwnerUserID: merchant.OwnerUserID,
						Name:        "Updated Name",
						Description: pgtype.Text{String: "Updated Description", Valid: true},
						Phone:       merchant.Phone,
						Address:     merchant.Address,
						Status:      merchant.Status,
						Version:     2, // 版本号递增
						CreatedAt:   merchant.CreatedAt,
						UpdatedAt:   time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "Updated Name", response.Name)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"name":    "Updated Name",
				"version": 1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					MinTimes(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					ListMerchantsByStaff(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return([]db.Merchant{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "VersionConflict",
			body: gin.H{
				"name":    "Updated Name",
				"version": 1, // 旧版本号
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				merchantWithNewVersion := merchant
				merchantWithNewVersion.Version = 2 // 数据库中已是版本2

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchantWithNewVersion, nil)

				// UpdateMerchant不会被调用，因为version检查在之前
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchants/me"
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper functions

func randomMerchant(ownerID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: ownerID,
		RegionID:    util.RandomInt(1, 1000),
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		Phone:       "13800138000",
		Address:     util.RandomString(30),
		Status:      "approved",
		Version:     1, // 初始版本号
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// TestGetCurrentMerchantAPI_WithLogoURL 回归测试（Phase 5.4）：
// 当商户设置了 logo_media_asset_id 时，GET /v1/merchants/me 响应中应包含 CDN logo_url。
func TestGetCurrentMerchantAPI_WithLogoURL(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.LogoMediaAssetID = pgtype.Int8{Int64: 42, Valid: true}

	const assetID int64 = 42
	logoAsset := db.MediaAsset{
		ID:         assetID,
		ObjectKey:  "merchant/logo/1/logo_card.jpg",
		Visibility: "public",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
		MinTimes(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), assetID).
		Times(1).
		Return(logoAsset, nil)

	server, _ := newTestServerForMedia(t, store)

	request, err := http.NewRequest(http.MethodGet, "/v1/merchants/me", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Contains(t, resp.LogoURL, "https://cdn.test.example.com", "logo_url 应指向 CDN 域名")
	require.Contains(t, resp.LogoURL, logoAsset.ObjectKey, "logo_url 应包含 object key")
}
