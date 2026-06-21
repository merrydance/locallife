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

func TestGetCurrentMerchantReturnsResolvedLiveShopImageURLs(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	storefrontImages, err := json.Marshal([]string{"uploads/merchants/12/storefront/cover.jpg"})
	require.NoError(t, err)
	environmentImages, err := json.Marshal([]string{"uploads/merchants/12/environment/hall.jpg"})
	require.NoError(t, err)
	merchant.StorefrontImages = storefrontImages
	merchant.EnvironmentImages = environmentImages

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)
	server.config.FileStorageProvider = "local"

	request, err := http.NewRequest(http.MethodGet, "/v1/merchants/me", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.StorefrontImages)
	require.Equal(t, []string{"/dev/uploads/merchants/12/storefront/cover.jpg"}, *resp.StorefrontImages)
	require.NotNil(t, resp.EnvironmentImages)
	require.Equal(t, []string{"/dev/uploads/merchants/12/environment/hall.jpg"}, *resp.EnvironmentImages)
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
				require.Equal(t, http.StatusForbidden, recorder.Code)
				requireAPIErrorCode(t, recorder, ErrMerchantAssociationRequired)
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
	merchant.StorefrontImages = storefrontImages

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

func TestGetPublicMerchantDetailFallsBackToApprovedApplicationImagesWhenMerchantImagesAreNull(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.StorefrontImages = nil
	application := randomMerchantAppDraft(merchant.OwnerUserID)
	application.Status = "approved"
	storefrontImages, err := json.Marshal([]string{"uploads/merchants/12/storefront/application-cover.jpg"})
	require.NoError(t, err)
	application.StorefrontImages = storefrontImages

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetLatestApprovedMerchantApplicationByUser(gomock.Any(), merchant.OwnerUserID).
		Times(1).
		Return(application, nil)
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
	require.Equal(t, "/dev/uploads/merchants/12/storefront/application-cover.jpg", *resp.CoverImage)
}

func TestGetPublicMerchantDetailDoesNotFallbackToDraftApplicationImages(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.StorefrontImages = nil

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetLatestApprovedMerchantApplicationByUser(gomock.Any(), merchant.OwnerUserID).
		Times(1).
		Return(db.MerchantApplication{}, db.ErrRecordNotFound)
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
	require.Nil(t, resp.CoverImage)
}

func TestGetPublicMerchantDetailDoesNotFallbackWhenMerchantImagesAreEmpty(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	emptyStorefrontImages, err := json.Marshal([]string{})
	require.NoError(t, err)
	merchant.StorefrontImages = emptyStorefrontImages

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetLatestApprovedMerchantApplicationByUser(gomock.Any(), gomock.Any()).
		Times(0)
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
	require.Nil(t, resp.CoverImage)
}

func TestGetPublicMerchantDetail_ReturnsInternalServerErrorOnInvalidCoverImageJSON(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.StorefrontImages = []byte(`not-json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListMerchantTags(gomock.Any(), merchant.ID).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/public/merchants/"+strconv.FormatInt(merchant.ID, 10), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}

func TestGetPublicMerchantDetail_ReturnsInternalServerErrorOnInvalidApplicationDataJSON(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))
	merchant.ApplicationData = []byte(`not-json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/public/merchants/"+strconv.FormatInt(merchant.ID, 10), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
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

func TestPublicMerchantCombos_DegradesInvalidTagsJSON(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantOnlineCombos(gomock.Any(), db.GetMerchantOnlineCombosParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: false,
		}).
		Times(1).
		Return([]db.GetMerchantOnlineCombosRow{{
			ID:            81,
			Name:          "测试套餐",
			ComboPrice:    1999,
			OriginalPrice: 2599,
			Dishes:        []byte(`[]`),
			Tags:          []byte(`not-json`),
		}}, nil)
	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), db.GetComboMemberImagesByCombosParams{
			Column1:          []int64{81},
			ExcludePackaging: false,
		}).
		Times(1).
		Return([]db.GetComboMemberImagesByCombosRow{}, nil)

	server := newTestServer(t, store)

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/merchants/%d/combos", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp publicMerchantCombosResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Combos, 1)
	require.Empty(t, resp.Combos[0].Tags)
}

func TestPublicMerchantCombos_DecodesInterfaceSliceTags(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantOnlineCombos(gomock.Any(), db.GetMerchantOnlineCombosParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: false,
		}).
		Times(1).
		Return([]db.GetMerchantOnlineCombosRow{{
			ID:            81,
			Name:          "测试套餐",
			ComboPrice:    1999,
			OriginalPrice: 2599,
			Dishes:        []byte(`[]`),
			Tags:          []interface{}{"招牌", "午市"},
		}}, nil)
	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), db.GetComboMemberImagesByCombosParams{
			Column1:          []int64{81},
			ExcludePackaging: false,
		}).
		Times(1).
		Return([]db.GetComboMemberImagesByCombosRow{}, nil)

	server := newTestServer(t, store)

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/merchants/%d/combos", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp publicMerchantCombosResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Combos, 1)
	require.Equal(t, []string{"招牌", "午市"}, resp.Combos[0].Tags)
}

func TestPublicMerchantCombos_ExcludesLegacyPackagingWhenFreezeEnabled(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantOnlineCombos(gomock.Any(), db.GetMerchantOnlineCombosParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: true,
		}).
		Times(1).
		Return([]db.GetMerchantOnlineCombosRow{}, nil)

	server := newTestServer(t, store)
	server.config.PackagingLegacyDishFreezeEnabled = true
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/merchants/%d/combos", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetPublicMerchantDishes_ReturnsInternalServerErrorOnInvalidCustomizationGroupsJSON(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantDishesWithCategory(gomock.Any(), db.GetMerchantDishesWithCategoryParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: false,
		}).
		Times(1).
		Return([]db.GetMerchantDishesWithCategoryRow{{
			ID:                  71,
			Name:                "测试菜品",
			Price:               1200,
			IsAvailable:         true,
			PrepareTime:         15,
			CategoryID:          11,
			CategoryName:        "主食",
			CategorySortOrder:   1,
			Tags:                []byte(`[]`),
			CustomizationGroups: []byte(`not-json`),
		}}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/merchants/%d/dishes", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}

func TestGetPublicMerchantDishes_ExcludesLegacyPackagingWhenFreezeEnabled(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 1000))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantDishesWithCategory(gomock.Any(), db.GetMerchantDishesWithCategoryParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: true,
		}).
		Times(1).
		Return([]db.GetMerchantDishesWithCategoryRow{}, nil)

	server := newTestServer(t, store)
	server.config.PackagingLegacyDishFreezeEnabled = true
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/merchants/%d/dishes", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchant.OwnerUserID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateCurrentMerchantShopImages_ReturnsInternalServerErrorOnInvalidStoredStorefrontImages(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	merchant := randomMerchant(user.ID)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		UpdateMerchantShopImages(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Merchant{
			ID:               merchant.ID,
			StorefrontImages: []byte(`not-json`),
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(updateCurrentMerchantShopImagesRequest{
		StorefrontImages: []string{"uploads/merchants/12/storefront/new.jpg"},
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/shop-images", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}

func TestUpdateCurrentMerchantShopImagesFailsClosedWhenMerchantContextMissing(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectNoMerchantAccessResolution(store)
	store.EXPECT().
		UpdateMerchantShopImages(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(updateCurrentMerchantShopImagesRequest{
		StorefrontImages: []string{"uploads/merchants/12/storefront/new.jpg"},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request, err = http.NewRequest(http.MethodPatch, "/v1/merchants/me/shop-images", bytes.NewReader(body))
	require.NoError(t, err)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: user.ID})

	server.updateCurrentMerchantShopImages(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Error)
}

func TestUpdateCurrentMerchantShopImagesUsesResolvedMerchantForStaff(t *testing.T) {
	manager, _ := randomUser(t)
	merchant := randomMerchant(manager.ID + 1000)
	merchant.Status = "active"
	merchant.RegionID = 1
	storefrontImages := []string{"uploads/merchants/12/storefront/new.jpg"}
	storedStorefrontImages, err := json.Marshal(storefrontImages)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectStaffRole(store, manager.ID, merchant, "manager")
	store.EXPECT().
		UpdateMerchantShopImages(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpdateMerchantShopImagesParams) (db.Merchant, error) {
			require.Equal(t, merchant.ID, arg.ID)
			require.JSONEq(t, string(storedStorefrontImages), string(arg.StorefrontImages))
			return db.Merchant{
				ID:               merchant.ID,
				OwnerUserID:      merchant.OwnerUserID,
				StorefrontImages: storedStorefrontImages,
			}, nil
		})

	server := newTestServer(t, store)
	body, err := json.Marshal(updateCurrentMerchantShopImagesRequest{
		StorefrontImages: storefrontImages,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/shop-images", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}
