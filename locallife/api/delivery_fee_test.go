package api

import (
	"bytes"
	"database/sql"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomDeliveryFeeConfig(regionID int64) db.DeliveryFeeConfig {
	return db.DeliveryFeeConfig{
		ID:            util.RandomInt(1, 1000),
		RegionID:      regionID,
		BaseFee:       500,
		BaseDistance:  3000,
		ExtraFeePerKm: 100,
		ValueRatio:    pgtype.Numeric{Valid: true},
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
		MinOrderAmount: 2000,
		DiscountAmount: 200,
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      pgtype.Timestamptz{},
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
		ID:             util.RandomInt(1, 1000),
		UserID:         userID,
		Name:           util.RandomString(10),
		ContactName:    util.RandomString(6),
		ContactPhone:   util.RandomString(11),
		RegionID:       util.RandomInt(1, 100),
		CommissionRate: pgtype.Numeric{},
		Status:         "active",
		CreatedAt:      time.Now(),
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

				// OperatorRegionMiddleware 调用
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

				// OperatorRegionMiddleware 调用
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

				// OperatorRegionMiddleware 调用
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
					Return(db.DeliveryFeeConfig{}, sql.ErrNoRows)
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
			name: "ConfigNotFound",
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
					Return(db.DeliveryFeeConfig{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
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

// ==================== 商家配送优惠测试 ====================

// randomMerchantForPromo 生成随机商户用于优惠测试
func randomMerchantForPromo(userID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: userID,
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

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
				// CasbinRoleMiddleware: 用户不是商户
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListDeliveryPromotionsByMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(promos, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []deliveryPromotionResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Len(t, response, 2)
			},
		},
		{
			name:       "ForbiddenWrongMerchant",
			merchantID: merchant.ID + 1, // 尝试查看别人的商户
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
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

// TestDeleteDeliveryPromotionAPI 测试删除配送优惠
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDeliveryPromotion(gomock.Any(), int64(999999)).
					Times(1).
					Return(db.MerchantDeliveryPromotion{}, sql.ErrNoRows)
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
				// CasbinRoleMiddleware 调用
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}}, nil)

				// LoadMerchantMiddleware 调用
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "merchant_owner",
					}).
					Times(1).
					Return(db.UserRole{
						UserID:          user.ID,
						Role:            "merchant_owner",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

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
