package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ptr64 returns a pointer to the given int64 value.
func ptr64(v int64) *int64 { return &v }

// ==================== batchPublicImageURLs ====================

func TestBatchPublicImageURLs(t *testing.T) {
	t.Run("empty input returns empty map without DB call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		// gomock strict: any unexpected DB call fails the test
		server, _ := newTestServerForMedia(t, store)

		result := server.batchPublicImageURLs(context.Background(), nil, media.VariantCard)
		require.Empty(t, result)
	})

	t.Run("single asset ID returns CDN URL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{42})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 42, ObjectKey: "merchant/dish/1/20260318/img.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		result := server.batchPublicImageURLs(context.Background(), []int64{42}, media.VariantCard)
		require.Len(t, result, 1)
		require.Contains(t, result[42], "cdn.test.example.com")
		require.Contains(t, result[42], "merchant/dish/1/20260318/img.jpg")
	})

	t.Run("duplicate IDs are deduplicated before DB call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		// [5, 5, 5] deduped → DB receives [5] exactly once
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{5})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 5, ObjectKey: "merchant/logo/1/20260318/logo.png"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		result := server.batchPublicImageURLs(context.Background(), []int64{5, 5, 5}, media.VariantCard)
		require.Len(t, result, 1)
		require.Contains(t, result[5], "cdn.test.example.com")
	})

	t.Run("multiple distinct IDs returns all CDN URLs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{1, 2, 3})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 1, ObjectKey: "merchant/dish/1/a.jpg"},
				{ID: 2, ObjectKey: "merchant/dish/1/b.jpg"},
				{ID: 3, ObjectKey: "merchant/dish/1/c.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		result := server.batchPublicImageURLs(context.Background(), []int64{1, 2, 3}, media.VariantCard)
		require.Len(t, result, 3)
		for _, id := range []int64{1, 2, 3} {
			require.NotEmpty(t, result[id], "expected CDN URL for asset ID %d", id)
			require.Contains(t, result[id], "cdn.test.example.com")
		}
	})

	t.Run("DB error returns empty map silently", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
			Times(1).
			Return(nil, db.ErrRecordNotFound)

		server, _ := newTestServerForMedia(t, store)

		result := server.batchPublicImageURLs(context.Background(), []int64{99}, media.VariantCard)
		require.Empty(t, result)
	})
}

// ==================== publicImageURL ====================

func TestPublicImageURL(t *testing.T) {
	t.Run("nil assetID returns empty string without DB call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		server, _ := newTestServerForMedia(t, store)

		url := server.publicImageURL(context.Background(), nil, media.VariantCard)
		require.Empty(t, url)
	})

	t.Run("valid assetID returns CDN URL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			GetMediaAssetByID(gomock.Any(), int64(7)).
			Times(1).
			Return(db.MediaAsset{ID: 7, ObjectKey: "merchant/dish/1/20260318/d.jpg"}, nil)

		server, _ := newTestServerForMedia(t, store)

		url := server.publicImageURL(context.Background(), ptr64(7), media.VariantCard)
		require.Contains(t, url, "cdn.test.example.com")
		require.Contains(t, url, "merchant/dish/1/20260318/d.jpg")
	})

	t.Run("DB error returns empty string silently", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			GetMediaAssetByID(gomock.Any(), int64(999)).
			Times(1).
			Return(db.MediaAsset{}, db.ErrRecordNotFound)

		server, _ := newTestServerForMedia(t, store)

		url := server.publicImageURL(context.Background(), ptr64(999), media.VariantCard)
		require.Empty(t, url)
	})
}

// ==================== enrichCartImageURLs ====================

func TestEnrichCartImageURLs(t *testing.T) {
	t.Run("items without asset IDs keep empty ImageURL, no DB call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		server, _ := newTestServerForMedia(t, store)

		items := []cartItemResponse{
			{ID: 1, Name: "菜A"},
			{ID: 2, Name: "菜B"},
		}
		server.enrichCartImageURLs(context.Background(), items)

		require.Empty(t, items[0].ImageURL)
		require.Empty(t, items[1].ImageURL)
	})

	t.Run("items with asset IDs get CDN URLs, nil items untouched", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{10, 20})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 10, ObjectKey: "merchant/dish/1/20260318/a.jpg"},
				{ID: 20, ObjectKey: "merchant/dish/1/20260318/b.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		items := []cartItemResponse{
			{ID: 1, Name: "菜A", ImageAssetID: ptr64(10)},
			{ID: 2, Name: "菜B", ImageAssetID: ptr64(20)},
			{ID: 3, Name: "菜C"}, // no asset
		}
		server.enrichCartImageURLs(context.Background(), items)

		require.Contains(t, items[0].ImageURL, "cdn.test.example.com")
		require.Contains(t, items[0].ImageURL, "20260318/a.jpg")
		require.Contains(t, items[1].ImageURL, "cdn.test.example.com")
		require.Contains(t, items[1].ImageURL, "20260318/b.jpg")
		require.Empty(t, items[2].ImageURL)
	})
}

// ==================== enrichSearchDishURLs ====================

func TestEnrichSearchDishURLs(t *testing.T) {
	t.Run("nil asset IDs produce no DB call, URLs stay empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		server, _ := newTestServerForMedia(t, store)

		dishes := []searchDishResponse{
			{ImageAssetID: nil, MerchantLogoAssetID: nil},
		}
		server.enrichSearchDishURLs(context.Background(), dishes)

		require.Empty(t, dishes[0].ImageURL)
		require.Empty(t, dishes[0].MerchantLogoURL)
	})

	t.Run("fills dish image and merchant logo URLs in one batch call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		// enrichSearchDishURLs appends ImageAssetID then MerchantLogoAssetID → [1, 2]
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{1, 2})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 1, ObjectKey: "merchant/dish/1/20260318/img.jpg"},
				{ID: 2, ObjectKey: "merchant/logo/1/20260318/logo.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		dishes := []searchDishResponse{
			{ImageAssetID: ptr64(1), MerchantLogoAssetID: ptr64(2)},
		}
		server.enrichSearchDishURLs(context.Background(), dishes)

		require.Contains(t, dishes[0].ImageURL, "20260318/img.jpg")
		require.Contains(t, dishes[0].MerchantLogoURL, "20260318/logo.jpg")
	})

	t.Run("multiple dishes each get correct URLs from single batch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		// dish0: img=10, no logo; dish1: img=20, logo=30 → [10, 20, 30]
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{10, 20, 30})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 10, ObjectKey: "d/10.jpg"},
				{ID: 20, ObjectKey: "d/20.jpg"},
				{ID: 30, ObjectKey: "d/30.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		dishes := []searchDishResponse{
			{ImageAssetID: ptr64(10), MerchantLogoAssetID: nil},
			{ImageAssetID: ptr64(20), MerchantLogoAssetID: ptr64(30)},
		}
		server.enrichSearchDishURLs(context.Background(), dishes)

		require.Contains(t, dishes[0].ImageURL, "d/10.jpg")
		require.Empty(t, dishes[0].MerchantLogoURL)
		require.Contains(t, dishes[1].ImageURL, "d/20.jpg")
		require.Contains(t, dishes[1].MerchantLogoURL, "d/30.jpg")
	})
}

// ==================== enrichSearchMerchantURLs ====================

func TestEnrichSearchMerchantURLs(t *testing.T) {
	t.Run("merchants without logo keep empty LogoURL, no DB call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		server, _ := newTestServerForMedia(t, store)

		merchants := []searchMerchantResponse{
			{LogoAssetID: nil},
		}
		server.enrichSearchMerchantURLs(context.Background(), merchants)
		require.Empty(t, merchants[0].LogoURL)
	})

	t.Run("merchants with logo asset ID get CDN URL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{55})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 55, ObjectKey: "merchant/logo/1/20260318/logo.png"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		merchants := []searchMerchantResponse{
			{LogoAssetID: ptr64(55)},
		}
		server.enrichSearchMerchantURLs(context.Background(), merchants)
		require.Contains(t, merchants[0].LogoURL, "cdn.test.example.com")
		require.Contains(t, merchants[0].LogoURL, "20260318/logo.png")
	})
}

// ==================== enrichSearchComboURLs ====================

func TestEnrichSearchComboURLs(t *testing.T) {
	t.Run("fills combo image and merchant logo from one batch call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		// ImageAssetID appended first, then MerchantLogoAssetID → [100, 200]
		store.EXPECT().
			ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{100, 200})).
			Times(1).
			Return([]db.MediaAsset{
				{ID: 100, ObjectKey: "combo/100.jpg"},
				{ID: 200, ObjectKey: "merchant/200.jpg"},
			}, nil)

		server, _ := newTestServerForMedia(t, store)

		combos := []searchComboResponse{
			{ImageAssetID: ptr64(100), MerchantLogoAssetID: ptr64(200)},
		}
		server.enrichSearchComboURLs(context.Background(), combos)
		require.Contains(t, combos[0].ImageURL, "combo/100.jpg")
		require.Contains(t, combos[0].MerchantLogoURL, "merchant/200.jpg")
	})
}

// ==================== listDishesByMerchant HTTP handler ====================
// Regression: CDN URLs appear in JSON when dishes have a valid image asset ID.

func TestListDishesByMerchantWithImageURLs(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	assetID := int64(55)

	dish := db.Dish{
		ID:                1,
		MerchantID:        merchant.ID,
		Name:              "测试菜品",
		ImageMediaAssetID: pgtype.Int8{Int64: assetID, Valid: true},
		IsAvailable:       true,
		IsOnline:          true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).AnyTimes().Return(merchant, nil)
	store.EXPECT().ListDishesByMerchant(gomock.Any(), gomock.Any()).Times(1).Return([]db.Dish{dish}, nil)
	store.EXPECT().CountDishesByMerchant(gomock.Any(), gomock.Any()).Times(1).Return(int64(1), nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{assetID})).
		Times(1).
		Return([]db.MediaAsset{{ID: assetID, ObjectKey: "merchant/dish/1/20260318/t.jpg"}}, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/v1/dishes?page_id=1&page_size=10", bytes.NewReader(nil))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp listDishesResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Dishes, 1)
	require.Contains(t, resp.Dishes[0].ImageURL, "cdn.test.example.com")
	require.Contains(t, resp.Dishes[0].ImageURL, "merchant/dish/1/20260318/t.jpg")
}

// ==================== getDish HTTP handler ====================
// Regression: publicImageURL is called and CDN URL appears in JSON response.

func TestGetDishWithImageURL(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	assetID := int64(88)
	dishID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).AnyTimes().Return(merchant, nil)
	store.EXPECT().GetDishComplete(gomock.Any(), gomock.Eq(dishID)).Times(1).Return(
		db.GetDishCompleteRow{
			ID:                dishID,
			MerchantID:        merchant.ID,
			Name:              "查看菜品",
			ImageMediaAssetID: pgtype.Int8{Int64: assetID, Valid: true},
			IsAvailable:       true,
			IsOnline:          true,
		}, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), gomock.Eq(assetID)).
		Times(1).
		Return(db.MediaAsset{ID: assetID, ObjectKey: "merchant/dish/1/20260318/get.jpg"}, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/v1/dishes/10", bytes.NewReader(nil))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp dishResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Contains(t, resp.ImageURL, "cdn.test.example.com")
	require.Contains(t, resp.ImageURL, "merchant/dish/1/20260318/get.jpg")
}
