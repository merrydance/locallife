package algorithm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHaversineDistance(t *testing.T) {
	// 测试：北京天安门到王府井（约1.3km）
	tiananmen := Location{Longitude: 116.397128, Latitude: 39.916527}
	wangfujing := Location{Longitude: 116.417199, Latitude: 39.917718}

	distance := HaversineDistance(tiananmen, wangfujing)
	// 允许 100 米误差
	require.InDelta(t, 1700, distance, 200)

	// 测试：同一点距离为 0
	d0 := HaversineDistance(tiananmen, tiananmen)
	require.Equal(t, 0, d0)
}

func TestEstimateTime(t *testing.T) {
	// 1km 约 3分钟
	time1km := EstimateTime(1000)
	require.InDelta(t, 3, time1km, 1)

	// 5km 约 15分钟
	time5km := EstimateTime(5000)
	require.InDelta(t, 15, time5km, 2)

	// 0距离
	time0 := EstimateTime(0)
	require.Equal(t, 0, time0)
}

func TestBearing(t *testing.T) {
	loc1 := Location{Longitude: 116.0, Latitude: 39.0}

	// 正北
	north := Location{Longitude: 116.0, Latitude: 40.0}
	bearingN := Bearing(loc1, north)
	require.InDelta(t, 0, bearingN, 1)

	// 正东
	east := Location{Longitude: 117.0, Latitude: 39.0}
	bearingE := Bearing(loc1, east)
	require.InDelta(t, 90, bearingE, 1)

	// 正南
	south := Location{Longitude: 116.0, Latitude: 38.0}
	bearingS := Bearing(loc1, south)
	require.InDelta(t, 180, bearingS, 1)
}

func TestIsInSameDirection(t *testing.T) {
	require.True(t, IsInSameDirection(10, 20, 30))
	require.True(t, IsInSameDirection(350, 10, 30)) // 跨越 0 度
	require.False(t, IsInSameDirection(0, 90, 30))
	require.False(t, IsInSameDirection(0, 180, 90))
}

func TestCenterPoint(t *testing.T) {
	locations := []Location{
		{Longitude: 116.0, Latitude: 39.0},
		{Longitude: 117.0, Latitude: 40.0},
	}
	center := CenterPoint(locations)
	require.InDelta(t, 116.5, center.Longitude, 0.01)
	require.InDelta(t, 39.5, center.Latitude, 0.01)

	// 空列表
	empty := CenterPoint([]Location{})
	require.Equal(t, Location{}, empty)

	// 单点
	single := CenterPoint(locations[:1])
	require.Equal(t, locations[0], single)
}

func TestSimpleRecommender_Recommend(t *testing.T) {
	recommender := NewSimpleRecommender()
	require.Equal(t, "SimpleRecommender", recommender.Name())
	require.Equal(t, "1.0.0", recommender.Version())

	ctx := context.Background()
	now := time.Now()

	// 骑手位置：假设在某点
	riderLocation := Location{Longitude: 116.4, Latitude: 39.9}

	// 创建测试订单池
	availablePool := []PoolOrder{
		{
			OrderID:          1,
			MerchantID:       100,
			PickupLocation:   Location{Longitude: 116.41, Latitude: 39.91}, // 近
			DeliveryLocation: Location{Longitude: 116.42, Latitude: 39.92},
			Distance:         1000,
			DeliveryFee:      500, // 5元
			ExpectedPickupAt: now.Add(10 * time.Minute),
			ExpiresAt:        now.Add(30 * time.Minute),
			Priority:         0,
		},
		{
			OrderID:          2,
			MerchantID:       101,
			PickupLocation:   Location{Longitude: 116.45, Latitude: 39.95}, // 远
			DeliveryLocation: Location{Longitude: 116.46, Latitude: 39.96},
			Distance:         2000,
			DeliveryFee:      800,
			ExpectedPickupAt: now.Add(5 * time.Minute),
			ExpiresAt:        now.Add(20 * time.Minute),
			Priority:         0,
		},
		{
			OrderID:          3,
			MerchantID:       102,
			PickupLocation:   Location{Longitude: 116.5, Latitude: 40.0}, // 超出范围
			DeliveryLocation: Location{Longitude: 116.6, Latitude: 40.1},
			Distance:         5000,
			DeliveryFee:      1500,
			ExpectedPickupAt: now.Add(15 * time.Minute),
			ExpiresAt:        now.Add(60 * time.Minute),
			Priority:         0,
		},
	}

	input := RecommendInput{
		RiderID:       1,
		RiderLocation: riderLocation,
		ActiveOrders:  []ActiveDelivery{},
		AvailablePool: availablePool,
		Config:        DefaultConfig(),
	}

	result, err := recommender.Recommend(ctx, input)
	require.NoError(t, err)

	// 应该只返回距离内的订单
	require.LessOrEqual(t, len(result), 2)

	// 第一个应该是分数最高的
	if len(result) > 0 {
		require.Greater(t, result[0].TotalScore, 0)
		require.Greater(t, result[0].DistanceToPickup, 0)
	}

	// 分数应该是递减的
	for i := 1; i < len(result); i++ {
		require.GreaterOrEqual(t, result[i-1].TotalScore, result[i].TotalScore)
	}
}

func TestSimpleRecommender_EmptyPool(t *testing.T) {
	recommender := NewSimpleRecommender()
	ctx := context.Background()

	input := RecommendInput{
		RiderID:       1,
		RiderLocation: Location{Longitude: 116.4, Latitude: 39.9},
		ActiveOrders:  []ActiveDelivery{},
		AvailablePool: []PoolOrder{},
		Config:        DefaultConfig(),
	}

	result, err := recommender.Recommend(ctx, input)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestSimpleRecommender_WithActiveOrders(t *testing.T) {
	recommender := NewSimpleRecommender()
	ctx := context.Background()
	now := time.Now()

	riderLocation := Location{Longitude: 116.4, Latitude: 39.9}

	// 骑手已有一个正在配送的订单
	activeOrders := []ActiveDelivery{
		{
			DeliveryID:       1,
			OrderID:          100,
			PickupLocation:   Location{Longitude: 116.41, Latitude: 39.91},
			DeliveryLocation: Location{Longitude: 116.42, Latitude: 39.92},
			Status:           "picking",
		},
	}

	// 新订单：一个顺路，一个不顺路（但都在距离范围内）
	availablePool := []PoolOrder{
		{
			OrderID:          1,
			MerchantID:       101,
			PickupLocation:   Location{Longitude: 116.415, Latitude: 39.915}, // 顺路
			DeliveryLocation: Location{Longitude: 116.425, Latitude: 39.925},
			Distance:         1000,
			DeliveryFee:      500,
			ExpectedPickupAt: now.Add(10 * time.Minute),
			ExpiresAt:        now.Add(30 * time.Minute),
		},
		{
			OrderID:          2,
			MerchantID:       102,
			PickupLocation:   Location{Longitude: 116.39, Latitude: 39.89}, // 反方向但在范围内
			DeliveryLocation: Location{Longitude: 116.38, Latitude: 39.88},
			Distance:         2000,
			DeliveryFee:      800,
			ExpectedPickupAt: now.Add(10 * time.Minute),
			ExpiresAt:        now.Add(30 * time.Minute),
		},
	}

	input := RecommendInput{
		RiderID:       1,
		RiderLocation: riderLocation,
		ActiveOrders:  activeOrders,
		AvailablePool: availablePool,
		Config:        DefaultConfig(),
	}

	result, err := recommender.Recommend(ctx, input)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// 顺路的订单应该排在前面（RouteScore 更高）
	require.Greater(t, result[0].RouteScore, result[1].RouteScore)
}

func TestSimpleRecommender_ExpiredOrders(t *testing.T) {
	recommender := NewSimpleRecommender()
	ctx := context.Background()
	now := time.Now()

	riderLocation := Location{Longitude: 116.4, Latitude: 39.9}

	// 一个过期订单，一个未过期
	availablePool := []PoolOrder{
		{
			OrderID:          1,
			MerchantID:       101,
			PickupLocation:   Location{Longitude: 116.41, Latitude: 39.91},
			DeliveryLocation: Location{Longitude: 116.42, Latitude: 39.92},
			Distance:         1000,
			DeliveryFee:      500,
			ExpectedPickupAt: now.Add(-20 * time.Minute), // 已过期
			ExpiresAt:        now.Add(-10 * time.Minute), // 已过期
		},
		{
			OrderID:          2,
			MerchantID:       102,
			PickupLocation:   Location{Longitude: 116.41, Latitude: 39.91},
			DeliveryLocation: Location{Longitude: 116.42, Latitude: 39.92},
			Distance:         1000,
			DeliveryFee:      500,
			ExpectedPickupAt: now.Add(10 * time.Minute),
			ExpiresAt:        now.Add(30 * time.Minute),
		},
	}

	input := RecommendInput{
		RiderID:       1,
		RiderLocation: riderLocation,
		ActiveOrders:  []ActiveDelivery{},
		AvailablePool: availablePool,
		Config:        DefaultConfig(),
	}

	result, err := recommender.Recommend(ctx, input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(2), result[0].OrderID)
}
