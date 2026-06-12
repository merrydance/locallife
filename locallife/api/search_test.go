package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type searchDistanceMatrixMapClient struct {
	result *maps.DistanceMatrixResult
	err    error
}

func (c searchDistanceMatrixMapClient) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (c searchDistanceMatrixMapClient) GetWalkingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (c searchDistanceMatrixMapClient) GetDrivingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (c searchDistanceMatrixMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []maps.Location, mode string) (*maps.DistanceMatrixResult, error) {
	return c.result, c.err
}

func (c searchDistanceMatrixMapClient) Geocode(ctx context.Context, address string) (*maps.GeocodeResult, error) {
	return nil, errors.New("not implemented")
}

func (c searchDistanceMatrixMapClient) ReverseGeocode(ctx context.Context, location maps.Location) (*maps.ReverseGeocodeResult, error) {
	return nil, errors.New("not implemented")
}

// ==================== 菜品搜索测试 ====================

func TestSearchDishesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(util.RandomInt(1, 100))

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_GlobalSearch",
			query: "?keyword=鸡&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				// 全局搜索
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchDishesGlobalRow{}, nil)

				store.EXPECT().
					CountSearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_SearchInMerchant",
			query: fmt.Sprintf("?keyword=鸡&merchant_id=%d&page_id=1&page_size=10", merchant.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Dish{}, nil)

				store.EXPECT().
					CountSearchDishesByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_MissingKeyword",
			query: "?region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchDishesGlobalRow{}, nil)

				store.EXPECT().
					CountSearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_KeywordTooLong",
			query: "?keyword=" + util.RandomString(101) + "&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidPageID",
			query: "?keyword=鸡&page_id=0&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_PageSizeTooLarge",
			query: "?keyword=鸡&page_id=1&page_size=100",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidMerchantID",
			query: "?keyword=鸡&merchant_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesByName(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError_GlobalSearch",
			query: "?keyword=鸡&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchDishesGlobalRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:  "InvalidCustomizationGroupsJSON_GlobalSearch",
			query: "?keyword=鸡&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchDishesGlobalRow{{
						ID:                  util.RandomInt(1, 1000),
						MerchantID:          merchant.ID,
						Name:                "香辣鸡腿堡",
						Price:               1999,
						IsAvailable:         true,
						IsOnline:            true,
						SortOrder:           1,
						PrepareTime:         12,
						MerchantName:        merchant.Name,
						MerchantRegionID:    merchant.RegionID,
						MerchantIsOpen:      true,
						CustomizationGroups: []byte(`not-json`),
					}}, nil)

				store.EXPECT().
					CountSearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
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

			url := "/v1/search/dishes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户搜索测试 ====================

func TestSearchMerchantsAPI(t *testing.T) {
	user, _ := randomUser(t)
	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?keyword=火锅&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchMerchantsRow{}, nil)

				store.EXPECT().
					CountSearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_MissingKeyword",
			query: "?region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchMerchantsRow{}, nil)

				store.EXPECT().
					CountSearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidPageSize",
			query: "?keyword=火锅&page_id=1&page_size=0",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?keyword=火锅&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchMerchantsRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:  "RewritesLegacyCoverImageInLocalMode",
			query: "?keyword=火锅&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				storefrontImages, err := json.Marshal([]string{"uploads/merchants/12/storefront/cover.jpg"})
				require.NoError(t, err)

				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchMerchantsRow{{
						ID:               12,
						Name:             "测试商户",
						Status:           "approved",
						RegionID:         1,
						IsOpen:           true,
						StorefrontImages: storefrontImages,
					}}, nil)

				store.EXPECT().
					CountSearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp searchMerchantListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Merchants, 1)
				require.Equal(t, "/dev/uploads/merchants/12/storefront/cover.jpg", resp.Merchants[0].CoverImage)
			},
		},
		{
			name:  "OK_TagFilterDistanceSort",
			query: "?tag_id=8&sort_by=distance&region_id=1&page_id=1&page_size=10&user_latitude=39.90&user_longitude=116.40",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchantsByTag(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.SearchMerchantsByTagParams) ([]db.SearchMerchantsByTagRow, error) {
						require.Equal(t, searchMerchantSortByDistance, arg.SortBy)
						return []db.SearchMerchantsByTagRow{}, nil
					})

				store.EXPECT().
					CountSearchMerchantsByTag(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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
			if tc.name == "RewritesLegacyCoverImageInLocalMode" {
				server.config.FileStorageProvider = "local"
			}
			recorder := httptest.NewRecorder()

			url := "/v1/search/merchants" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCalculateSearchMerchantDistancesAndFeesKeepsDisplayDistanceAlignedWithFeeDistance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		regionID   int64 = 7
		merchantID int64 = 42
	)

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.mapClient = searchDistanceMatrixMapClient{
		result: &maps.DistanceMatrixResult{
			Rows: []maps.DistanceMatrixRow{
				{Elements: []maps.DistanceMatrixElement{{Distance: 4000, Duration: 900}}},
			},
		},
	}

	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
		Return(db.DeliveryFeeConfig{
			RegionID:      regionID,
			BaseFee:       500,
			BaseDistance:  1000,
			ExtraFeePerKm: 100,
			ValueRatio:    numericFromFloat(0),
			MinFee:        0,
			IsActive:      true,
		}, nil)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), regionID).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListPeakHourConfigsByRegion(gomock.Any(), regionID).
		Return([]db.PeakHourConfig{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchantID).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	sqlDistance := 1500
	merchants := []searchMerchantResponse{{
		ID:       merchantID,
		RegionID: regionID,
		Latitude: 39.90, Longitude: 116.40,
		Distance: &sqlDistance,
	}}
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/search/merchants", nil)

	server.calculateSearchMerchantDistancesAndFees(ginCtx, merchants, 39.91, 116.41)

	require.NotNil(t, merchants[0].Distance)
	require.Equal(t, 4000, *merchants[0].Distance)
	require.NotNil(t, merchants[0].EstimatedDeliveryFee)
	require.EqualValues(t, 800, *merchants[0].EstimatedDeliveryFee)
}

func TestCalculateSearchMerchantDistancesAndFeesIgnoresZeroRouteDistance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.mapClient = searchDistanceMatrixMapClient{
		result: &maps.DistanceMatrixResult{
			Rows: []maps.DistanceMatrixRow{
				{Elements: []maps.DistanceMatrixElement{{Distance: 0, Duration: 0}}},
			},
		},
	}

	sqlDistance := 1500
	merchants := []searchMerchantResponse{{
		ID:       42,
		RegionID: 7,
		Latitude: 39.90, Longitude: 116.40,
		Distance: &sqlDistance,
	}}
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/search/merchants", nil)

	server.calculateSearchMerchantDistancesAndFees(ginCtx, merchants, 39.91, 116.41)

	require.NotNil(t, merchants[0].Distance)
	require.Equal(t, 1500, *merchants[0].Distance)
	require.Nil(t, merchants[0].EstimatedDeliveryFee)
}

// ==================== 套餐搜索测试 ====================

func TestSearchCombosAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(util.RandomInt(1, 100))

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_GlobalSearch",
			query: "?keyword=套餐&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				combos := []db.SearchCombosGlobalRow{
					{
						ID:               util.RandomInt(1, 1000),
						MerchantID:       merchant.ID,
						Name:             "超值套餐",
						OriginalPrice:    5000,
						ComboPrice:       4000,
						IsOnline:         true,
						MerchantName:     merchant.Name,
						MerchantRegionID: merchant.RegionID,
						MerchantIsOpen:   true,
						MonthlySales:     10,
						Distance:         0,
					},
				}

				store.EXPECT().
					SearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(combos, nil)

				store.EXPECT().
					CountSearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(len(combos)), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response struct {
					Combos   []searchComboResponse `json:"combos"`
					Total    int64                 `json:"total"`
					PageID   int32                 `json:"page_id"`
					PageSize int32                 `json:"page_size"`
				}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Combos, 1)
				require.EqualValues(t, 1, response.Total)
			},
		},
		{
			name:  "BadRequest_InvalidPageID",
			query: "?keyword=套餐&page_id=0&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?keyword=套餐&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchCombosGlobalRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:  "InvalidTagsJSON",
			query: "?keyword=套餐&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchCombosGlobalRow{{
						ID:               util.RandomInt(1, 1000),
						MerchantID:       merchant.ID,
						Name:             "超值套餐",
						OriginalPrice:    5000,
						ComboPrice:       4000,
						IsOnline:         true,
						MerchantName:     merchant.Name,
						MerchantRegionID: merchant.RegionID,
						MerchantIsOpen:   true,
						MonthlySales:     10,
						Distance:         0,
						Tags:             []byte(`not-json`),
					}}, nil)

				store.EXPECT().
					CountSearchCombosGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
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

			url := "/v1/search/combos" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 包间搜索测试 ====================

func TestSearchRoomsAPI(t *testing.T) {
	user, _ := randomUser(t)
	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, nil)

				store.EXPECT().
					CountSearchRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithFilters",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=1&min_capacity=4&max_capacity=10&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, nil)

				store.EXPECT().
					CountSearchRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithTagFilter",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=1&tag_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsByMerchantTag(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsByMerchantTagRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingReservationDate",
			query: "?reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingReservationTime",
			query: "?reservation_date=2025-12-15&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidDateFormat",
			query: "?reservation_date=2025/12/15&reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidTimeFormat",
			query: "?reservation_date=2025-12-15&reservation_time=25:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidMinCapacity",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&min_capacity=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_CapacityTooLarge",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&max_capacity=101&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidRegionID",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidTagID",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&tag_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsByMerchantTag(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, sql.ErrConnDone)
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

			url := "/v1/search/rooms" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
