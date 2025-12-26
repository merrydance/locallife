package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// WeatherCacheKeyPrefix Redis 缓存 key 前缀
	WeatherCacheKeyPrefix = "weather:coefficient:"
	// WeatherCacheTTL 缓存过期时间（20分钟，略长于抓取周期）
	WeatherCacheTTL = 20 * time.Minute
)

// CachedCoefficient 缓存的天气系数数据
type CachedCoefficient struct {
	Coefficient        float64   `json:"coefficient"`
	WarningCoefficient float64   `json:"warning_coefficient"`
	SuspendDelivery    bool      `json:"suspend_delivery"`
	WeatherType        string    `json:"weather_type"`
	Temperature        int       `json:"temperature"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// WeatherCache 天气系数缓存接口
type WeatherCache interface {
	// Get 获取区域天气系数
	Get(ctx context.Context, regionID int64) (*CachedCoefficient, error)
	// Set 设置区域天气系数
	Set(ctx context.Context, regionID int64, coef *CachedCoefficient) error
	// Delete 删除区域天气系数缓存
	Delete(ctx context.Context, regionID int64) error
}

// redisWeatherCache Redis 实现的天气缓存
type redisWeatherCache struct {
	client *redis.Client
}

// NewWeatherCache 创建天气缓存
func NewWeatherCache(redisAddr string, redisPassword string) (WeatherCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: redisPassword,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &redisWeatherCache{client: client}, nil
}

// cacheKey 生成缓存 key
func cacheKey(regionID int64) string {
	return fmt.Sprintf("%s%d", WeatherCacheKeyPrefix, regionID)
}

// Get 获取天气系数缓存
func (c *redisWeatherCache) Get(ctx context.Context, regionID int64) (*CachedCoefficient, error) {
	key := cacheKey(regionID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 缓存不存在
		}
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	var coef CachedCoefficient
	if err := json.Unmarshal(data, &coef); err != nil {
		return nil, fmt.Errorf("unmarshal cached coefficient failed: %w", err)
	}

	return &coef, nil
}

// Set 设置天气系数缓存
func (c *redisWeatherCache) Set(ctx context.Context, regionID int64, coef *CachedCoefficient) error {
	key := cacheKey(regionID)

	data, err := json.Marshal(coef)
	if err != nil {
		return fmt.Errorf("marshal cached coefficient failed: %w", err)
	}

	if err := c.client.Set(ctx, key, data, WeatherCacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

// Delete 删除天气系数缓存
func (c *redisWeatherCache) Delete(ctx context.Context, regionID int64) error {
	key := cacheKey(regionID)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}

	return nil
}

// FromWeatherCoefficient 从计算结果创建缓存数据
func FromWeatherCoefficient(coef *WeatherCoefficient) *CachedCoefficient {
	return &CachedCoefficient{
		Coefficient:        coef.Coefficient,
		WarningCoefficient: coef.WarningCoefficient,
		SuspendDelivery:    coef.SuspendDelivery,
		WeatherType:        coef.WeatherType,
		Temperature:        coef.Temperature,
		UpdatedAt:          time.Now(),
	}
}
