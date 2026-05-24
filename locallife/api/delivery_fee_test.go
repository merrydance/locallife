package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/weather"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testWeatherCache struct {
	coefficient    *weather.CachedCoefficient
	getErr         error
	setErr         error
	deleteErr      error
	deletedRegions []int64
}

func (c *testWeatherCache) Get(ctx context.Context, regionID int64) (*weather.CachedCoefficient, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.coefficient, nil
}

func (c *testWeatherCache) Set(ctx context.Context, regionID int64, coef *weather.CachedCoefficient) error {
	if c.setErr != nil {
		return c.setErr
	}
	c.coefficient = coef
	return nil
}

func (c *testWeatherCache) Delete(ctx context.Context, regionID int64) error {
	if c.deleteErr != nil {
		return c.deleteErr
	}
	c.deletedRegions = append(c.deletedRegions, regionID)
	c.coefficient = nil
	return nil
}

// ==================== 测试数据生成 ====================

func randomDeliveryFeeConfig(regionID int64) db.DeliveryFeeConfig {
	valueRatio := pgtype.Numeric{}
	_ = valueRatio.Scan("0.01")

	return db.DeliveryFeeConfig{
		ID:            util.RandomInt(1, 1000),
		RegionID:      regionID,
		BaseFee:       500,
		BaseDistance:  3000,
		ExtraFeePerKm: 100,
		ValueRatio:    valueRatio,
		MaxFee:        pgtype.Int8{Int64: 3000, Valid: true},
		MinFee:        300,
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     pgtype.Timestamptz{},
	}
}

func randomPeakHourConfig(regionID int64) db.PeakHourConfig {
	return db.PeakHourConfig{
		ID:          util.RandomInt(1, 1000),
		RegionID:    regionID,
		StartTime:   pgtype.Time{Microseconds: 11 * 3600 * 1e6, Valid: true}, // 11:00
		EndTime:     pgtype.Time{Microseconds: 13 * 3600 * 1e6, Valid: true}, // 13:00
		Coefficient: pgtype.Numeric{Valid: true},
		DaysOfWeek:  []int16{0, 1, 2, 3, 4, 5, 6},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   pgtype.Timestamptz{},
	}
}

func randomDeliveryPromotion(merchantID int64) db.MerchantDeliveryPromotion {
	return db.MerchantDeliveryPromotion{
		ID:             util.RandomInt(1, 1000),
		MerchantID:     merchantID,
		Name:           util.RandomString(10),
		MinOrderAmount: 2000,
		DiscountAmount: 200,
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      pgtype.Timestamptz{},
	}
}

func TestNewDeliveryPromotionResponseStatus(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		promo     db.MerchantDeliveryPromotion
		wantCode  string
		wantLabel string
		wantTheme string
	}{
		{
			name: "Inactive",
			promo: db.MerchantDeliveryPromotion{
				IsActive:   false,
				ValidFrom:  now.Add(-time.Hour),
				ValidUntil: now.Add(time.Hour),
			},
			wantCode:  "inactive",
			wantLabel: "已停用",
			wantTheme: "default",
		},
		{
			name: "Expired",
			promo: db.MerchantDeliveryPromotion{
				IsActive:   true,
				ValidFrom:  now.Add(-2 * time.Hour),
				ValidUntil: now.Add(-time.Hour),
			},
			wantCode:  "expired",
			wantLabel: "已过期",
			wantTheme: "danger",
		},
		{
			name: "Scheduled",
			promo: db.MerchantDeliveryPromotion{
				IsActive:   true,
				ValidFrom:  now.Add(time.Hour),
				ValidUntil: now.Add(2 * time.Hour),
			},
			wantCode:  "scheduled",
			wantLabel: "未开始",
			wantTheme: "warning",
		},
		{
			name: "Active",
			promo: db.MerchantDeliveryPromotion{
				IsActive:   true,
				ValidFrom:  now.Add(-time.Hour),
				ValidUntil: now.Add(time.Hour),
			},
			wantCode:  "active",
			wantLabel: "生效中",
			wantTheme: "success",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusCode, statusLabel, statusTheme := buildDeliveryPromotionStatusResponse(tc.promo, now)
			require.Equal(t, tc.wantCode, statusCode)
			require.Equal(t, tc.wantLabel, statusLabel)
			require.Equal(t, tc.wantTheme, statusTheme)
		})
	}
}

func randomWeatherCoefficient(regionID int64) db.WeatherCoefficient {
	return db.WeatherCoefficient{
		ID:                 util.RandomInt(1, 1000),
		RegionID:           regionID,
		RecordedAt:         time.Now(),
		WeatherData:        []byte(`{"temp":25,"humidity":60}`),
		WarningData:        []byte(`{}`),
		WeatherType:        "sunny",
		Temperature:        pgtype.Int2{Int16: 25, Valid: true},
		FeelsLike:          pgtype.Int2{Int16: 27, Valid: true},
		Humidity:           pgtype.Int2{Int16: 60, Valid: true},
		WindSpeed:          pgtype.Int2{Int16: 10, Valid: true},
		Visibility:         pgtype.Int2{Int16: 10, Valid: true},
		HasWarning:         false,
		WeatherCoefficient: pgtype.Numeric{Valid: true},
		FinalCoefficient:   pgtype.Numeric{Valid: true},
		DeliverySuspended:  false,
		CreatedAt:          time.Now(),
	}
}

// ==================== 创建运费配置测试 ====================

// randomOperator 生成随机运营商
func randomOperator(userID int64) db.Operator {
	return db.Operator{
		ID:           util.RandomInt(1, 1000),
		UserID:       userID,
		Name:         util.RandomString(10),
		ContactName:  util.RandomString(6),
		ContactPhone: util.RandomString(11),
		RegionID:     util.RandomInt(1, 100),
		Status:       "active",
		CreatedAt:    time.Now(),
	}
}

func TestCreateDeliveryFeeConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	regionID := util.RandomInt(1, 100)
	config := randomDeliveryFeeConfig(regionID)
	operator := randomOperator(user.ID)

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
				"region_id":        regionID,
				"base_fee":         config.BaseFee,
				"base_distance":    config.BaseDistance,
				"extra_fee_per_km": config.ExtraFeePerKm,
				"value_ratio":      0.01,
				"max_fee":          config.MaxFee.Int64,
				"min_fee":          config.MinFee,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// RoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// OperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// ValidateOperatorRegionMiddleware 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					CreateDeliveryFeeConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(config, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response deliveryFeeConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, config.ID, response.ID)
				require.Equal(t, regionID, response.RegionID)
				require.Equal(t, config.BaseFee, response.BaseFee)
				require.Equal(t, config.BaseDistance, response.BaseDistance)
				require.Equal(t, config.ExtraFeePerKm, response.ExtraFeePerKm)
				require.Equal(t, config.MinFee, response.MinFee)
				require.True(t, response.IsActive)
				require.NotNil(t, response.MaxFee)
				require.Equal(t, config.MaxFee.Int64, *response.MaxFee)
				require.InDelta(t, 0.01, response.ValueRatio, 0.000001)
			},
		},
		{
			name: "Unauthorized",
			body: gin.H{
				"region_id":        regionID,
				"base_fee":         config.BaseFee,
				"base_distance":    config.BaseDistance,
				"extra_fee_per_km": config.ExtraFeePerKm,
				"min_fee":          config.MinFee,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "ForbiddenNotOperator",
			body: gin.H{
				"region_id":        regionID,
				"base_fee":         config.BaseFee,
				"base_distance":    config.BaseDistance,
				"extra_fee_per_km": config.ExtraFeePerKm,
				"min_fee":          config.MinFee,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// RoleMiddleware: 用户不是运营商
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer", Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "InvalidBody",
			body: gin.H{
				"region_id": regionID,
				"base_fee":  -1, // invalid: negative
				"min_fee":   100,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// RoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// OperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// ValidateOperatorRegionMiddleware 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidMinMaxConflict",
			body: gin.H{
				"region_id":        regionID,
				"base_fee":         config.BaseFee,
				"base_distance":    config.BaseDistance,
				"extra_fee_per_km": config.ExtraFeePerKm,
				"max_fee":          int64(100),
				"min_fee":          int64(200),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// RoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// OperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// ValidateOperatorRegionMiddleware 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DuplicateConfig",
			body: gin.H{
				"region_id":        regionID,
				"base_fee":         config.BaseFee,
				"base_distance":    config.BaseDistance,
				"extra_fee_per_km": config.ExtraFeePerKm,
				"min_fee":          config.MinFee,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// RoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// OperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// ValidateOperatorRegionMiddleware 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					CreateDeliveryFeeConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.DeliveryFeeConfig{}, db.ErrUniqueViolation)
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/delivery-fee/regions/%d/config", regionID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取运费配置测试 ====================

func TestGetDeliveryFeeConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	regionID := util.RandomInt(1, 100)
	config := randomDeliveryFeeConfig(regionID)

	testCases := []struct {
		name          string
		regionID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
					Times(1).
					Return(config, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "NotFound",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
					Times(1).
					Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "Unauthorized",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/delivery-fee/regions/%d/config", tc.regionID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 运费计算测试 ====================

func TestCalculateDeliveryFeeAPI(t *testing.T) {
	user, _ := randomUser(t)
	regionID := util.RandomInt(1, 100)
	merchantID := util.RandomInt(1, 100)
	config := randomDeliveryFeeConfig(regionID)
	weather := randomWeatherCoefficient(regionID)

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
				"region_id":    regionID,
				"merchant_id":  merchantID,
				"distance":     5000,
				"order_amount": 5000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
					Times(1).
					Return(config, nil)

				store.EXPECT().
					GetLatestWeatherCoefficient(gomock.Any(), regionID).
					Times(1).
					Return(weather, nil)

				store.EXPECT().
					ListPeakHourConfigsByRegion(gomock.Any(), regionID).
					Times(1).
					Return([]db.PeakHourConfig{}, nil)

				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantDeliveryPromotion{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response calculateDeliveryFeeResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.GreaterOrEqual(t, response.FinalFee, int64(0))
				require.False(t, response.DeliverySuspended)
			},
		},
		{
			name: "ConfigNotFoundFallsBackToPlatformDefault",
			body: gin.H{
				"region_id":    regionID,
				"merchant_id":  merchantID,
				"distance":     5000,
				"order_amount": 5000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
					Times(1).
					Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
						ConfigKey: deliveryFeeDefaultConfigKey,
						ScopeType: db.PlatformConfigScopeGlobal,
						ScopeID:   pgtype.Int8{Valid: false},
					}).
					Times(1).
					Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
						BaseFee:       700,
						BaseDistance:  3000,
						ExtraFeePerKm: 100,
						ValueRatio:    0.01,
						MinFee:        500,
					})}, nil)

				store.EXPECT().
					GetLatestWeatherCoefficient(gomock.Any(), regionID).
					Times(1).
					Return(weather, nil)

				store.EXPECT().
					ListPeakHourConfigsByRegion(gomock.Any(), regionID).
					Times(1).
					Return([]db.PeakHourConfig{}, nil)

				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantDeliveryPromotion{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response calculateDeliveryFeeResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
			},
		},
		{
			name: "ConfigDisabled",
			body: gin.H{
				"region_id":    regionID,
				"merchant_id":  merchantID,
				"distance":     5000,
				"order_amount": 5000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				disabledConfig := config
				disabledConfig.IsActive = false
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
					Times(1).
					Return(disabledConfig, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidBody",
			body: gin.H{
				"region_id":    0,
				"merchant_id":  0,
				"distance":     -1,
				"order_amount": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Unauthorized",
			body: gin.H{
				"region_id":    regionID,
				"merchant_id":  merchantID,
				"distance":     5000,
				"order_amount": 5000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/delivery-fee/calculate"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCalculateDeliveryFeeInternal_UsesPlatformDefaultWhenRegionConfigMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(18)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       800,
			BaseDistance:  3000,
			ExtraFeePerKm: 200,
			ValueRatio:    0.02,
			MinFee:        500,
		})}, nil)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), int64(18)).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListPeakHourConfigsByRegion(gomock.Any(), int64(18)).
		Return([]db.PeakHourConfig{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), int64(9)).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	result, err := server.calculateDeliveryFeeInternal(context.Background(), 18, 9, 5000, 10000)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(800), result.BaseFee)
	require.Greater(t, result.FinalFee, int64(800))
	require.False(t, result.DeliverySuspended)
}

func TestCalculateDeliveryFeeInternal_ConsumesRegionWeatherAndPeakRules(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	regionID := int64(18)
	merchantID := int64(9)

	config := db.DeliveryFeeConfig{
		ID:            88,
		RegionID:      regionID,
		BaseFee:       500,
		BaseDistance:  3000,
		ExtraFeePerKm: 100,
		ValueRatio:    numericFromFloat(0.01),
		MinFee:        300,
		IsActive:      true,
	}
	weatherCoefficient := db.WeatherCoefficient{
		RegionID:          regionID,
		FinalCoefficient:  numericFromFloat(1.2),
		DeliverySuspended: false,
	}
	now := time.Now()
	peakConfig := db.PeakHourConfig{
		RegionID:    regionID,
		StartTime:   pgtype.Time{Microseconds: 0, Valid: true},
		EndTime:     pgtype.Time{Microseconds: int64(23*time.Hour+59*time.Minute) / int64(time.Microsecond), Valid: true},
		Coefficient: numericFromFloat(1.5),
		DaysOfWeek:  []int16{int16(now.Weekday())},
		IsActive:    true,
	}

	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), regionID).
		Return(config, nil)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), regionID).
		Return(weatherCoefficient, nil)
	store.EXPECT().
		ListPeakHourConfigsByRegion(gomock.Any(), regionID).
		Return([]db.PeakHourConfig{peakConfig}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchantID).
		Return([]db.MerchantDeliveryPromotion{}, nil)

	result, err := server.calculateDeliveryFeeInternal(context.Background(), regionID, merchantID, 5000, 10000)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(500), result.BaseFee)
	require.Equal(t, int64(200), result.DistanceFee)
	require.Equal(t, int64(100), result.ValueFee)
	require.InDelta(t, 1.2, result.WeatherCoefficient, 0.0001)
	require.InDelta(t, 1.5, result.PeakHourCoefficient, 0.0001)
	require.Equal(t, int64(1440), result.FinalFee)
	require.False(t, result.DeliverySuspended)
}

// ==================== 高峰时段配置测试 ====================

func TestCreatePeakHourConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	regionID := util.RandomInt(1, 100)
	config := randomPeakHourConfig(regionID)
	operator := randomOperator(user.ID)

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
				"region_id":    regionID,
				"start_time":   "11:00",
				"end_time":     "13:00",
				"coefficient":  1.2,
				"days_of_week": []int16{0, 1, 2, 3, 4, 5, 6},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// LoadOperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// handler 中的 checkOperatorManagesRegion 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					CreatePeakHourConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(config, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "InvalidTimeFormat",
			body: gin.H{
				"region_id":    regionID,
				"start_time":   "invalid",
				"end_time":     "13:00",
				"coefficient":  1.2,
				"days_of_week": []int16{0, 1, 2, 3, 4, 5, 6},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// LoadOperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// handler 中的 checkOperatorManagesRegion 调用
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ForbiddenNotOperator",
			body: gin.H{
				"region_id":    regionID,
				"start_time":   "11:00",
				"end_time":     "13:00",
				"coefficient":  1.2,
				"days_of_week": []int16{0, 1, 2, 3, 4, 5, 6},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware: 用户不是 operator
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer", Status: "active"}}, nil)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/operator/regions/%d/peak-hours", regionID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListPeakHourConfigsAPI(t *testing.T) {
	user, _ := randomUser(t)
	regionID := util.RandomInt(1, 100)
	configs := []db.PeakHourConfig{
		randomPeakHourConfig(regionID),
		randomPeakHourConfig(regionID),
	}
	operator := randomOperator(user.ID)

	testCases := []struct {
		name          string
		regionID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// LoadOperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					ListPeakHourConfigsByRegion(gomock.Any(), regionID).
					Times(1).
					Return(configs, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []peakHourConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response, 2)
			},
		},
		{
			name:     "EmptyList",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				// LoadOperatorMiddleware 调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					ListPeakHourConfigsByRegion(gomock.Any(), regionID).
					Times(1).
					Return([]db.PeakHourConfig{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []peakHourConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response, 0)
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

			url := fmt.Sprintf("/v1/operator/regions/%d/peak-hours", tc.regionID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商家代取优惠测试 ====================

// randomMerchantForPromo 生成随机商户用于优惠测试
func randomMerchantForPromo(userID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: userID,
		RegionID:    util.RandomInt(1, 100),
		Name:        util.RandomString(10),
		Phone:       util.RandomString(11),
		Address:     util.RandomString(20),
		Status:      "active",
		CreatedAt:   time.Now(),
	}
}

func TestCreateDeliveryPromotionAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForPromo(user.ID)
	promo := randomDeliveryPromotion(merchant.ID)

	testCases := []struct {
		name          string
		merchantID    int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			body: gin.H{
				"name":             "满20减2",
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       promo.ValidFrom.Format(time.RFC3339),
				"valid_until":      promo.ValidUntil.Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					CreateDeliveryPromotion(gomock.Any(), gomock.Any()).
					Times(1).
					Return(promo, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name:       "ForbiddenNotMerchant",
			merchantID: merchant.ID,
			body: gin.H{
				"name":             "满20减2",
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       promo.ValidFrom.Format(time.RFC3339),
				"valid_until":      promo.ValidUntil.Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "ForbiddenWrongMerchant",
			merchantID: merchant.ID + 1, // 尝试操作别人的商户
			body: gin.H{
				"name":             "满20减2",
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       promo.ValidFrom.Format(time.RFC3339),
				"valid_until":      promo.ValidUntil.Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "InvalidDates",
			merchantID: merchant.ID,
			body: gin.H{
				"name":             "满20减2",
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       time.Now().Add(time.Hour).Format(time.RFC3339),
				"valid_until":      time.Now().Format(time.RFC3339), // before valid_from
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "MissingName",
			merchantID: merchant.ID,
			body: gin.H{
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       promo.ValidFrom.Format(time.RFC3339),
				"valid_until":      promo.ValidUntil.Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "Unauthorized",
			merchantID: merchant.ID,
			body: gin.H{
				"name":             "满20减2",
				"min_order_amount": promo.MinOrderAmount,
				"discount_amount":  promo.DiscountAmount,
				"valid_from":       promo.ValidFrom.Format(time.RFC3339),
				"valid_until":      promo.ValidUntil.Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/delivery-fee/merchants/%d/promotions", tc.merchantID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListDeliveryPromotionsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForPromo(user.ID)
	promos := []db.MerchantDeliveryPromotion{
		randomDeliveryPromotion(merchant.ID),
		randomDeliveryPromotion(merchant.ID),
	}

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListDeliveryPromotionsByMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(promos, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []deliveryPromotionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response, 2)
				require.Equal(t, "active", response[0].StatusCode)
				require.Equal(t, "生效中", response[0].StatusLabel)
				require.Equal(t, "success", response[0].StatusTheme)
			},
		},
		{
			name:       "ForbiddenWrongMerchant",
			merchantID: merchant.ID + 1, // 尝试查看别人的商户
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "Unauthorized",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/delivery-fee/merchants/%d/promotions", tc.merchantID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestDeleteDeliveryPromotionAPI 测试删除代取优惠
func TestDeleteDeliveryPromotionAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForPromo(user.ID)
	promo := randomDeliveryPromotion(merchant.ID)

	testCases := []struct {
		name          string
		merchantID    int64
		promoID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			promoID:    promo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDeliveryPromotion(gomock.Any(), promo.ID).
					Times(1).
					Return(promo, nil)

				store.EXPECT().
					DeleteDeliveryPromotion(gomock.Any(), promo.ID).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, recorder.Code)
			},
		},
		{
			name:       "NotFound",
			merchantID: merchant.ID,
			promoID:    999999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDeliveryPromotion(gomock.Any(), int64(999999)).
					Times(1).
					Return(db.MerchantDeliveryPromotion{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "ForbiddenPromoBelongsToDifferentMerchant",
			merchantID: merchant.ID,
			promoID:    promo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 促销属于另一个商户
				otherPromo := promo
				otherPromo.MerchantID = merchant.ID + 999
				store.EXPECT().
					GetDeliveryPromotion(gomock.Any(), promo.ID).
					Times(1).
					Return(otherPromo, nil)
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

			url := fmt.Sprintf("/v1/delivery-fee/merchants/%d/promotions/%d", tc.merchantID, tc.promoID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
