package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== 辅助函数 ====================

// getTestRegionID 获取一个测试用的 region ID
// 使用 offset 来避免每次都使用同一个 region
func getTestRegionID(t *testing.T, offset int) int64 {
	regions, err := testStore.ListRegions(context.Background(), ListRegionsParams{
		Limit:  10,
		Offset: int32(offset),
	})
	require.NoError(t, err)
	require.NotEmpty(t, regions)
	return regions[0].ID
}

// 使用计数器避免 region 冲突
var regionOffset = 0

// getCleanRegionID 获取一个干净的 region ID（先删除已有配置）
func getCleanRegionID(t *testing.T) int64 {
	regionOffset++
	regionID := getTestRegionID(t, regionOffset%100)

	// 尝试获取并删除已存在的配置
	existingConfig, err := testStore.GetDeliveryFeeConfigByRegion(context.Background(), regionID)
	if err == nil && existingConfig.ID > 0 {
		_ = testStore.DeleteDeliveryFeeConfig(context.Background(), existingConfig.ID)
	}

	return regionID
}

// getTestMerchant 创建一个测试用的商户
func getTestMerchant(t *testing.T) Merchant {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	arg := CreateMerchantParams{
		OwnerUserID:     user.ID,
		Name:            util.RandomString(8),
		Phone:           "1380000" + util.RandomString(4),
		Address:         util.RandomString(20),
		Status:          "approved",
		ApplicationData: []byte(`{}`),
		RegionID:        region.ID,
	}
	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)
	return merchant
}

// ==================== 运费配置测试 ====================

func createRandomDeliveryFeeConfig(t *testing.T) DeliveryFeeConfig {
	regionID := getCleanRegionID(t)

	valueRatio := pgtype.Numeric{}
	_ = valueRatio.Scan("0.01")

	arg := CreateDeliveryFeeConfigParams{
		RegionID:      regionID,
		BaseFee:       util.RandomInt(300, 800),
		BaseDistance:  int32(util.RandomInt(2000, 5000)),
		ExtraFeePerKm: util.RandomInt(50, 200),
		ValueRatio:    valueRatio,
		MaxFee:        pgtype.Int8{Int64: 3000, Valid: true},
		MinFee:        util.RandomInt(200, 400),
		IsActive:      true,
	}

	config, err := testStore.CreateDeliveryFeeConfig(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	require.Equal(t, regionID, config.RegionID)
	require.Equal(t, arg.BaseFee, config.BaseFee)
	require.Equal(t, arg.BaseDistance, config.BaseDistance)
	require.Equal(t, arg.ExtraFeePerKm, config.ExtraFeePerKm)
	require.Equal(t, arg.MinFee, config.MinFee)
	require.True(t, config.IsActive)
	require.NotZero(t, config.ID)
	require.NotZero(t, config.CreatedAt)

	return config
}

func TestCreateDeliveryFeeConfig(t *testing.T) {
	createRandomDeliveryFeeConfig(t)
}

func TestGetDeliveryFeeConfig(t *testing.T) {
	config1 := createRandomDeliveryFeeConfig(t)

	config2, err := testStore.GetDeliveryFeeConfig(context.Background(), config1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, config2)

	require.Equal(t, config1.ID, config2.ID)
	require.Equal(t, config1.RegionID, config2.RegionID)
	require.Equal(t, config1.BaseFee, config2.BaseFee)
}

func TestGetDeliveryFeeConfigByRegion(t *testing.T) {
	config1 := createRandomDeliveryFeeConfig(t)

	config2, err := testStore.GetDeliveryFeeConfigByRegion(context.Background(), config1.RegionID)
	require.NoError(t, err)
	require.NotEmpty(t, config2)

	// 可能返回不同的config（如果之前已存在），只验证region匹配
	require.Equal(t, config1.RegionID, config2.RegionID)
}

func TestUpdateDeliveryFeeConfig(t *testing.T) {
	config1 := createRandomDeliveryFeeConfig(t)

	newBaseFee := int64(999)
	arg := UpdateDeliveryFeeConfigParams{
		ID:      config1.ID,
		BaseFee: pgtype.Int8{Int64: newBaseFee, Valid: true},
	}

	config2, err := testStore.UpdateDeliveryFeeConfig(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, config2)

	require.Equal(t, config1.ID, config2.ID)
	require.Equal(t, newBaseFee, config2.BaseFee)
}

func TestListActiveDeliveryFeeConfigs(t *testing.T) {
	// 先创建一个配置（会成功因为使用唯一的region）
	config := createRandomDeliveryFeeConfig(t)
	require.True(t, config.IsActive)

	configs, err := testStore.ListActiveDeliveryFeeConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, configs)

	for _, c := range configs {
		require.True(t, c.IsActive)
	}
}

// ==================== 高峰时段配置测试 ====================

func createRandomPeakHourConfig(t *testing.T) PeakHourConfig {
	regionID := getCleanRegionID(t)

	startTime := pgtype.Time{Microseconds: 11 * 3600 * 1e6, Valid: true} // 11:00
	endTime := pgtype.Time{Microseconds: 13 * 3600 * 1e6, Valid: true}   // 13:00
	coefficient := pgtype.Numeric{}
	_ = coefficient.Scan("1.2")

	arg := CreatePeakHourConfigParams{
		RegionID:    regionID,
		Name:        "午餐高峰" + util.RandomString(4),
		StartTime:   startTime,
		EndTime:     endTime,
		Coefficient: coefficient,
		DaysOfWeek:  []int16{0, 1, 2, 3, 4, 5, 6},
		IsActive:    true,
	}

	config, err := testStore.CreatePeakHourConfig(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	require.Equal(t, regionID, config.RegionID)
	require.True(t, config.IsActive)
	require.NotZero(t, config.ID)
	require.NotZero(t, config.CreatedAt)

	return config
}

func TestCreatePeakHourConfig(t *testing.T) {
	createRandomPeakHourConfig(t)
}

func TestGetPeakHourConfig(t *testing.T) {
	config1 := createRandomPeakHourConfig(t)

	config2, err := testStore.GetPeakHourConfig(context.Background(), config1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, config2)

	require.Equal(t, config1.ID, config2.ID)
	require.Equal(t, config1.RegionID, config2.RegionID)
}

func TestListPeakHourConfigsByRegion(t *testing.T) {
	config1 := createRandomPeakHourConfig(t)

	configs, err := testStore.ListPeakHourConfigsByRegion(context.Background(), config1.RegionID)
	require.NoError(t, err)
	require.NotEmpty(t, configs)

	found := false
	for _, config := range configs {
		if config.ID == config1.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestDeletePeakHourConfig(t *testing.T) {
	config := createRandomPeakHourConfig(t)

	err := testStore.DeletePeakHourConfig(context.Background(), config.ID)
	require.NoError(t, err)

	_, err = testStore.GetPeakHourConfig(context.Background(), config.ID)
	require.Error(t, err)
}

// ==================== 商家配送优惠测试 ====================

func createRandomDeliveryPromotion(t *testing.T) MerchantDeliveryPromotion {
	merchant := getTestMerchant(t)

	arg := CreateDeliveryPromotionParams{
		MerchantID:     merchant.ID,
		MinOrderAmount: util.RandomInt(1000, 5000),
		DiscountAmount: util.RandomInt(100, 500),
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
		IsActive:       true,
	}

	promo, err := testStore.CreateDeliveryPromotion(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, promo)

	require.Equal(t, merchant.ID, promo.MerchantID)
	require.Equal(t, arg.MinOrderAmount, promo.MinOrderAmount)
	require.Equal(t, arg.DiscountAmount, promo.DiscountAmount)
	require.True(t, promo.IsActive)
	require.NotZero(t, promo.ID)
	require.NotZero(t, promo.CreatedAt)

	return promo
}

func TestCreateDeliveryPromotion(t *testing.T) {
	createRandomDeliveryPromotion(t)
}

func TestGetDeliveryPromotion(t *testing.T) {
	promo1 := createRandomDeliveryPromotion(t)

	promo2, err := testStore.GetDeliveryPromotion(context.Background(), promo1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, promo2)

	require.Equal(t, promo1.ID, promo2.ID)
	require.Equal(t, promo1.MerchantID, promo2.MerchantID)
}

func TestListDeliveryPromotionsByMerchant(t *testing.T) {
	promo1 := createRandomDeliveryPromotion(t)

	promos, err := testStore.ListDeliveryPromotionsByMerchant(context.Background(), promo1.MerchantID)
	require.NoError(t, err)
	require.NotEmpty(t, promos)

	found := false
	for _, promo := range promos {
		if promo.ID == promo1.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestListActiveDeliveryPromotionsByMerchant(t *testing.T) {
	promo := createRandomDeliveryPromotion(t)

	promos, err := testStore.ListActiveDeliveryPromotionsByMerchant(context.Background(), promo.MerchantID)
	require.NoError(t, err)
	require.NotEmpty(t, promos)

	for _, p := range promos {
		require.True(t, p.IsActive)
	}
}

func TestDeleteDeliveryPromotion(t *testing.T) {
	promo := createRandomDeliveryPromotion(t)

	err := testStore.DeleteDeliveryPromotion(context.Background(), promo.ID)
	require.NoError(t, err)

	_, err = testStore.GetDeliveryPromotion(context.Background(), promo.ID)
	require.Error(t, err)
}

// ==================== 天气系数测试 ====================

func createRandomWeatherCoefficient(t *testing.T) WeatherCoefficient {
	regionID := getCleanRegionID(t)

	weatherCoeff := pgtype.Numeric{}
	_ = weatherCoeff.Scan("1.0")
	warningCoeff := pgtype.Numeric{}
	_ = warningCoeff.Scan("1.0")
	finalCoeff := pgtype.Numeric{}
	_ = finalCoeff.Scan("1.0")

	arg := CreateWeatherCoefficientParams{
		RegionID:           regionID,
		RecordedAt:         time.Now(),
		WeatherData:        []byte(`{"temp":25}`),
		WarningData:        []byte(`{}`),
		WeatherType:        "sunny",
		Temperature:        pgtype.Int2{Int16: 25, Valid: true},
		FeelsLike:          pgtype.Int2{Int16: 27, Valid: true},
		Humidity:           pgtype.Int2{Int16: 60, Valid: true},
		WindSpeed:          pgtype.Int2{Int16: 10, Valid: true},
		HasWarning:         false,
		WeatherCoefficient: weatherCoeff,
		WarningCoefficient: warningCoeff,
		FinalCoefficient:   finalCoeff,
		DeliverySuspended:  false,
	}

	coeff, err := testStore.CreateWeatherCoefficient(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, coeff)

	require.Equal(t, regionID, coeff.RegionID)
	require.Equal(t, "sunny", coeff.WeatherType)
	require.False(t, coeff.HasWarning)
	require.False(t, coeff.DeliverySuspended)
	require.NotZero(t, coeff.ID)
	require.NotZero(t, coeff.CreatedAt)

	return coeff
}

func TestCreateWeatherCoefficient(t *testing.T) {
	createRandomWeatherCoefficient(t)
}

func TestGetLatestWeatherCoefficient(t *testing.T) {
	coeff1 := createRandomWeatherCoefficient(t)

	coeff2, err := testStore.GetLatestWeatherCoefficient(context.Background(), coeff1.RegionID)
	require.NoError(t, err)
	require.NotEmpty(t, coeff2)

	// 验证 region 匹配
	require.Equal(t, coeff1.RegionID, coeff2.RegionID)
}

func TestListWeatherCoefficients(t *testing.T) {
	coeff := createRandomWeatherCoefficient(t)

	arg := ListWeatherCoefficientsParams{
		RegionID: coeff.RegionID,
		Limit:    10,
		Offset:   0,
	}

	coeffs, err := testStore.ListWeatherCoefficients(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, coeffs)

	found := false
	for _, c := range coeffs {
		if c.ID == coeff.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestWeatherCoefficientWithWarning(t *testing.T) {
	regionID := getCleanRegionID(t)

	weatherCoeff := pgtype.Numeric{}
	_ = weatherCoeff.Scan("1.2")
	warningCoeff := pgtype.Numeric{}
	_ = warningCoeff.Scan("1.5")
	finalCoeff := pgtype.Numeric{}
	_ = finalCoeff.Scan("1.5") // max(1.2, 1.5)

	arg := CreateWeatherCoefficientParams{
		RegionID:           regionID,
		RecordedAt:         time.Now(),
		WeatherData:        []byte(`{"temp":15}`),
		WarningData:        []byte(`{"type":"rain"}`),
		WeatherType:        "heavy_rain",
		Temperature:        pgtype.Int2{Int16: 15, Valid: true},
		FeelsLike:          pgtype.Int2{Int16: 12, Valid: true},
		Humidity:           pgtype.Int2{Int16: 90, Valid: true},
		WindSpeed:          pgtype.Int2{Int16: 30, Valid: true},
		HasWarning:         true,
		WarningType:        pgtype.Text{String: "1003", Valid: true},
		WarningLevel:       pgtype.Text{String: "orange", Valid: true},
		WarningSeverity:    pgtype.Text{String: "severe", Valid: true},
		WarningText:        pgtype.Text{String: "暴雨橙色预警", Valid: true},
		WeatherCoefficient: weatherCoeff,
		WarningCoefficient: warningCoeff,
		FinalCoefficient:   finalCoeff,
		DeliverySuspended:  false,
	}

	coeff, err := testStore.CreateWeatherCoefficient(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, coeff)

	require.True(t, coeff.HasWarning)
	require.Equal(t, "orange", coeff.WarningLevel.String)
	require.False(t, coeff.DeliverySuspended)
}

func TestWeatherCoefficientSuspendDelivery(t *testing.T) {
	regionID := getCleanRegionID(t)

	// 红色预警，暂停配送
	weatherCoeff := pgtype.Numeric{}
	_ = weatherCoeff.Scan("1.0")
	warningCoeff := pgtype.Numeric{}
	_ = warningCoeff.Scan("2.0")
	finalCoeff := pgtype.Numeric{}
	_ = finalCoeff.Scan("2.0")

	arg := CreateWeatherCoefficientParams{
		RegionID:           regionID,
		RecordedAt:         time.Now(),
		WeatherData:        []byte(`{"temp":0}`),
		WarningData:        []byte(`{"type":"blizzard"}`),
		WeatherType:        "extreme",
		Temperature:        pgtype.Int2{Int16: -5, Valid: true},
		FeelsLike:          pgtype.Int2{Int16: -15, Valid: true},
		Humidity:           pgtype.Int2{Int16: 95, Valid: true},
		WindSpeed:          pgtype.Int2{Int16: 80, Valid: true},
		HasWarning:         true,
		WarningType:        pgtype.Text{String: "1101", Valid: true},
		WarningLevel:       pgtype.Text{String: "red", Valid: true},
		WarningSeverity:    pgtype.Text{String: "extreme", Valid: true},
		WarningText:        pgtype.Text{String: "暴雪红色预警", Valid: true},
		WeatherCoefficient: weatherCoeff,
		WarningCoefficient: warningCoeff,
		FinalCoefficient:   finalCoeff,
		DeliverySuspended:  true,
		SuspendReason:      pgtype.Text{String: "极端天气预警，暂停配送", Valid: true},
	}

	coeff, err := testStore.CreateWeatherCoefficient(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, coeff)

	require.True(t, coeff.HasWarning)
	require.Equal(t, "red", coeff.WarningLevel.String)
	require.True(t, coeff.DeliverySuspended)
	require.Equal(t, "极端天气预警，暂停配送", coeff.SuspendReason.String)
}
