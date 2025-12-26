package weather

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler 天气数据定时抓取调度器
type Scheduler struct {
	cron          *cron.Cron
	store         db.Store
	weatherClient QweatherClient
	cache         WeatherCache
}

// NewScheduler 创建调度器
func NewScheduler(store db.Store, weatherClient QweatherClient, cache WeatherCache) *Scheduler {
	return &Scheduler{
		cron:          cron.New(),
		store:         store,
		weatherClient: weatherClient,
		cache:         cache,
	}
}

// Start 启动调度器（每15分钟执行一次）
func (s *Scheduler) Start() error {
	// 每15分钟执行一次
	_, err := s.cron.AddFunc("*/15 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := s.FetchAllWeather(ctx); err != nil {
			log.Error().Err(err).Msg("failed to fetch weather data")
		}
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("weather scheduler started (every 15 minutes)")

	// 启动时立即执行一次
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := s.FetchAllWeather(ctx); err != nil {
			log.Error().Err(err).Msg("failed to fetch initial weather data")
		}
	}()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("weather scheduler stopped")
}

// FetchAllWeather 抓取所有已开通区域的天气数据
func (s *Scheduler) FetchAllWeather(ctx context.Context) error {
	// 获取所有已开通运费配置的区县
	regions, err := s.store.GetRegionsWithDeliveryFeeConfig(ctx)
	if err != nil {
		return err
	}

	if len(regions) == 0 {
		log.Info().Msg("no regions with active delivery fee config, skipping weather fetch")
		return nil
	}

	log.Info().Int("count", len(regions)).Msg("fetching weather for regions")

	for _, region := range regions {
		if err := s.fetchRegionWeather(ctx, region); err != nil {
			log.Error().
				Err(err).
				Int64("region_id", region.ID).
				Str("region_name", region.Name).
				Msg("failed to fetch weather for region")
			// 继续处理其他区域
			continue
		}
	}

	return nil
}

// fetchRegionWeather 抓取单个区域的天气数据
func (s *Scheduler) fetchRegionWeather(ctx context.Context, region db.GetRegionsWithDeliveryFeeConfigRow) error {
	// 1. 获取或查询 LocationID
	locationID, lat, lon, err := s.getLocationID(ctx, region)
	if err != nil {
		return err
	}

	// 2. 获取实时天气
	weatherResp, err := s.weatherClient.GetWeatherNow(ctx, locationID)
	if err != nil {
		return err
	}

	// 3. 获取天气预警（使用经纬度）
	var warnings []WarningAlert
	if lat != 0 && lon != 0 {
		warningResp, err := s.weatherClient.GetWeatherWarning(ctx, lat, lon)
		if err != nil {
			log.Warn().Err(err).Msg("failed to get weather warning, continuing without warning data")
		} else if !warningResp.Metadata.ZeroResult {
			warnings = warningResp.Alerts
		}
	}

	// 4. 计算天气系数
	coef := CalculateCoefficient(&weatherResp.Now, warnings)

	// 5. 保存到数据库
	if err := s.saveToDatabase(ctx, region.ID, weatherResp, coef); err != nil {
		log.Error().Err(err).Msg("failed to save weather to database")
	}

	// 6. 更新 Redis 缓存
	if err := s.cache.Set(ctx, region.ID, FromWeatherCoefficient(coef)); err != nil {
		log.Error().Err(err).Msg("failed to update weather cache")
	}

	log.Info().
		Int64("region_id", region.ID).
		Str("region_name", region.Name).
		Str("weather", weatherResp.Now.Text).
		Float64("coefficient", coef.FinalCoefficient()).
		Bool("suspend", coef.SuspendDelivery).
		Msg("weather data updated")

	return nil
}

// getLocationID 获取区域的和风天气 LocationID
func (s *Scheduler) getLocationID(ctx context.Context, region db.GetRegionsWithDeliveryFeeConfigRow) (locationID string, lat, lon float64, err error) {
	// 如果已经有缓存的 LocationID，直接使用
	if region.QweatherLocationID.Valid && region.QweatherLocationID.String != "" {
		locationID = region.QweatherLocationID.String
		// 需要再次查询获取经纬度（用于预警API）
		cityResp, err := s.weatherClient.LookupCity(ctx, region.Name, region.CityName.String)
		if err == nil && len(cityResp.Location) > 0 {
			lat, _ = strconv.ParseFloat(cityResp.Location[0].Lat, 64)
			lon, _ = strconv.ParseFloat(cityResp.Location[0].Lon, 64)
		}
		return locationID, lat, lon, nil
	}

	// 查询 GeoAPI 获取 LocationID
	cityName := ""
	if region.CityName.Valid {
		cityName = region.CityName.String
	}

	cityResp, err := s.weatherClient.LookupCity(ctx, region.Name, cityName)
	if err != nil {
		return "", 0, 0, err
	}

	if len(cityResp.Location) == 0 {
		return "", 0, 0, nil
	}

	loc := cityResp.Location[0]
	locationID = loc.ID
	lat, _ = strconv.ParseFloat(loc.Lat, 64)
	lon, _ = strconv.ParseFloat(loc.Lon, 64)

	// 缓存 LocationID 到数据库
	if err := s.store.UpdateRegionQweatherLocationID(ctx, db.UpdateRegionQweatherLocationIDParams{
		ID:                 region.ID,
		QweatherLocationID: pgtype.Text{String: locationID, Valid: true},
	}); err != nil {
		log.Warn().Err(err).Msg("failed to cache LocationID")
	}

	return locationID, lat, lon, nil
}

// saveToDatabase 保存天气数据到数据库
func (s *Scheduler) saveToDatabase(ctx context.Context, regionID int64, weather *WeatherNowResponse, coef *WeatherCoefficient) error {
	// 解析数据
	temp, _ := strconv.Atoi(weather.Now.Temp)
	feelsLike, _ := strconv.Atoi(weather.Now.FeelsLike)
	humidity, _ := strconv.Atoi(weather.Now.Humidity)
	windSpeed, _ := strconv.Atoi(weather.Now.WindSpeed)
	precip, _ := strconv.ParseFloat(weather.Now.Precip, 64)
	vis, _ := strconv.Atoi(weather.Now.Vis)

	// 准备 coefficient
	weatherCoefficient := pgtype.Numeric{}
	_ = weatherCoefficient.Scan(strconv.FormatFloat(coef.Coefficient, 'f', 2, 64))

	warningCoefficient := pgtype.Numeric{}
	_ = warningCoefficient.Scan(strconv.FormatFloat(coef.WarningCoefficient, 'f', 2, 64))

	finalCoefficient := pgtype.Numeric{}
	_ = finalCoefficient.Scan(strconv.FormatFloat(coef.FinalCoefficient(), 'f', 2, 64))

	precipNumeric := pgtype.Numeric{}
	_ = precipNumeric.Scan(strconv.FormatFloat(precip, 'f', 2, 64))

	// 创建天气系数记录
	_, err := s.store.CreateWeatherCoefficient(ctx, db.CreateWeatherCoefficientParams{
		RegionID:           regionID,
		RecordedAt:         time.Now(),
		WeatherData:        []byte("{}"),
		WarningData:        []byte("{}"),
		WeatherType:        coef.WeatherType,
		WeatherCode:        pgtype.Text{String: weather.Now.Icon, Valid: true},
		Temperature:        pgtype.Int2{Int16: int16(temp), Valid: true},
		FeelsLike:          pgtype.Int2{Int16: int16(feelsLike), Valid: true},
		Humidity:           pgtype.Int2{Int16: int16(humidity), Valid: true},
		WindSpeed:          pgtype.Int2{Int16: int16(windSpeed), Valid: true},
		WindScale:          pgtype.Text{String: weather.Now.WindScale, Valid: true},
		Precip:             precipNumeric,
		Visibility:         pgtype.Int2{Int16: int16(vis), Valid: true},
		HasWarning:         coef.WarningType != "",
		WarningType:        pgtype.Text{String: coef.WarningType, Valid: coef.WarningType != ""},
		WarningLevel:       pgtype.Text{String: coef.WarningLevel, Valid: coef.WarningLevel != ""},
		WarningSeverity:    pgtype.Text{String: coef.WarningLevel, Valid: coef.WarningLevel != ""},
		WarningText:        pgtype.Text{String: coef.WarningText, Valid: coef.WarningText != ""},
		WeatherCoefficient: weatherCoefficient,
		WarningCoefficient: warningCoefficient,
		FinalCoefficient:   finalCoefficient,
		DeliverySuspended:  coef.SuspendDelivery,
		SuspendReason:      pgtype.Text{String: "", Valid: false},
	})

	return err
}
