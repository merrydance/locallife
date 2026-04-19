package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveNoAccessibleMerchants(store, user.ID)
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

				expectResolveSingleOwnedMerchant(store, user.ID, merchantWithNewVersion)

				// UpdateMerchant不会被调用，因为version检查在之前
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "ClearLogo",
			body: gin.H{
				"clear_logo": true,
				"version":    1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				merchantWithLogo := merchant
				merchantWithLogo.LogoMediaAssetID = pgtype.Int8{Int64: 88, Valid: true}

				expectResolveSingleOwnedMerchant(store, user.ID, merchantWithLogo)

				store.EXPECT().
					ClearMerchantLogo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{
						ID:               merchant.ID,
						OwnerUserID:      merchant.OwnerUserID,
						Name:             merchant.Name,
						Description:      merchant.Description,
						Phone:            merchant.Phone,
						Address:          merchant.Address,
						Status:           merchant.Status,
						Version:          2,
						LogoMediaAssetID: pgtype.Int8{Valid: false},
						CreatedAt:        merchant.CreatedAt,
						UpdatedAt:        time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Nil(t, response.LogoAssetID)
				require.Empty(t, response.LogoURL)
				require.Equal(t, int32(2), response.Version)
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
		ID:               assetID,
		ObjectKey:        "merchant/logo/1/logo_card.jpg",
		Visibility:       "public",
		ModerationStatus: "approved",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

func TestGetPublicMerchantDetail_RewritesCoverImageInLocalMode(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	storefrontImages, err := json.Marshal([]string{"uploads/merchants/12/storefront/cover.jpg"})
	require.NoError(t, err)
	application := randomMerchantAppDraft(merchant.OwnerUserID)
	application.StorefrontImages = storefrontImages

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), merchant.OwnerUserID).
		Times(1).
		Return(application, nil)
	store.EXPECT().
		ListMerchantTags(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{}, nil)
	store.EXPECT().
		ListMerchantSystemLabels(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{{Name: db.SystemTagNoOpenKitchen}}, nil)
	store.EXPECT().
		GetMerchantProfile(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantProfileRow{}, nil)
	store.EXPECT().
		GetMerchantAvgPrepMinutes(gomock.Any(), merchant.ID).
		Times(1).
		Return(int32(0), nil)
	store.EXPECT().
		ListMerchantBusinessHours(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantBusinessHour{}, nil)
	store.EXPECT().
		ListMerchantActiveDiscountRules(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListMerchantActiveVouchers(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Voucher{}, nil)
	store.EXPECT().
		ListMerchantActiveDeliveryPromotions(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	server := newTestServer(t, store)
	server.config.FileStorageProvider = "local"

	request, err := http.NewRequest(http.MethodGet, "/v1/public/merchants/"+strconv.FormatInt(merchant.ID, 10), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp publicMerchantDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.CoverImage)
	require.Equal(t, "/dev/uploads/merchants/12/storefront/cover.jpg", *resp.CoverImage)
	require.Equal(t, []string{db.SystemTagNoOpenKitchen}, resp.SystemLabels)
}

func TestGetPublicMerchantDetail_ReturnsOrderingSuspendedFlag(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListMerchantTags(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{}, nil)
	store.EXPECT().
		ListMerchantSystemLabels(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{}, nil)
	store.EXPECT().
		GetMerchantProfile(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantProfileRow{IsTakeoutSuspended: true}, nil)
	store.EXPECT().
		GetMerchantAvgPrepMinutes(gomock.Any(), merchant.ID).
		Times(1).
		Return(int32(0), nil)
	store.EXPECT().
		ListMerchantActiveDiscountRules(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListMerchantActiveVouchers(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Voucher{}, nil)
	store.EXPECT().
		ListMerchantActiveDeliveryPromotions(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	server := newTestServer(t, store)

	request, err := http.NewRequest(http.MethodGet, "/v1/public/merchants/"+strconv.FormatInt(merchant.ID, 10)+"?lite=true", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp publicMerchantDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.IsOrderingSuspended)
}

func TestGetPublicMerchantDetail_AllowsBindbankSubmittedStorefrontPreview(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.Status = "bindbank_submitted"
	merchant.IsOpen = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListMerchantTags(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{}, nil)
	store.EXPECT().
		ListMerchantSystemLabels(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Tag{}, nil)
	store.EXPECT().
		GetMerchantProfile(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantProfileRow{}, nil)
	store.EXPECT().
		GetMerchantAvgPrepMinutes(gomock.Any(), merchant.ID).
		Times(1).
		Return(int32(0), nil)
	store.EXPECT().
		ListMerchantActiveDiscountRules(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListMerchantActiveVouchers(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.Voucher{}, nil)
	store.EXPECT().
		ListMerchantActiveDeliveryPromotions(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	server := newTestServer(t, store)

	request, err := http.NewRequest(http.MethodGet, "/v1/public/merchants/"+strconv.FormatInt(merchant.ID, 10)+"?lite=true", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp publicMerchantDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.False(t, resp.IsOpen, "进件审核中的店铺页应可浏览，但不可下单")
}

func TestPublicMerchantStorefrontSubresources_RejectUnavailableMerchant(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.Status = "suspended"

	testCases := []struct {
		name string
		path string
	}{
		{name: "Dishes", path: "/v1/public/merchants/%d/dishes"},
		{name: "Combos", path: "/v1/public/merchants/%d/combos"},
		{name: "Rooms", path: "/v1/public/merchants/%d/rooms"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				GetMerchant(gomock.Any(), merchant.ID).
				Times(1).
				Return(merchant, nil)

			server := newTestServer(t, store)

			request, err := http.NewRequest(http.MethodGet, fmt.Sprintf(tc.path, merchant.ID), nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}
