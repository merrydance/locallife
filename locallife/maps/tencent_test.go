package maps

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func skipIfQuotaExceeded(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	// Tencent LBS daily quota exceeded: status=121
	if strings.Contains(err.Error(), "status=121") {
		t.Skip("腾讯地图 key 每日调用量已达上限（status=121），跳过联网测试")
	}
}

func getTestClient(t *testing.T) *TencentMapClient {
	key := os.Getenv("TENCENT_MAP_KEY")
	if key == "" {
		t.Skip("TENCENT_MAP_KEY not set, skipping integration test")
	}
	return NewTencentMapClient(key)
}

func TestGetBicyclingRoute(t *testing.T) {
	client := getTestClient(t)

	// 北京：天安门 -> 故宫
	from := Location{Lat: 39.908722, Lng: 116.397499}
	to := Location{Lat: 39.916345, Lng: 116.397155}

	result, err := client.GetBicyclingRoute(context.Background(), from, to)
	skipIfQuotaExceeded(t, err)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.Distance, 0, "距离应该大于0")
	require.Greater(t, result.Duration, 0, "时间应该大于0")

	t.Logf("骑行距离: %d 米, 预计时间: %d 秒 (%.1f 分钟)",
		result.Distance, result.Duration, float64(result.Duration)/60)
}

func TestGetWalkingRoute(t *testing.T) {
	client := getTestClient(t)

	from := Location{Lat: 39.908722, Lng: 116.397499}
	to := Location{Lat: 39.916345, Lng: 116.397155}

	result, err := client.GetWalkingRoute(context.Background(), from, to)
	skipIfQuotaExceeded(t, err)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.Distance, 0)
	require.Greater(t, result.Duration, 0)

	t.Logf("步行距离: %d 米, 预计时间: %d 秒 (%.1f 分钟)",
		result.Distance, result.Duration, float64(result.Duration)/60)
}

func TestGetDistanceMatrix(t *testing.T) {
	client := getTestClient(t)

	// 1个起点, 3个终点（模拟一个订单对多个骑手）
	froms := []Location{
		{Lat: 39.908722, Lng: 116.397499}, // 订单取货点
	}
	tos := []Location{
		{Lat: 39.916345, Lng: 116.397155}, // 骑手1位置
		{Lat: 39.920000, Lng: 116.400000}, // 骑手2位置
		{Lat: 39.905000, Lng: 116.390000}, // 骑手3位置
	}

	result, err := client.GetDistanceMatrix(context.Background(), froms, tos, "bicycling")
	skipIfQuotaExceeded(t, err)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Rows, 1, "应该有1行（1个起点）")
	require.Len(t, result.Rows[0].Elements, 3, "应该有3列（3个终点）")

	for i, elem := range result.Rows[0].Elements {
		t.Logf("骑手%d: 距离=%d米, 时间=%d秒", i+1, elem.Distance, elem.Duration)
	}
}

func TestGeocode(t *testing.T) {
	client := getTestClient(t)

	result, err := client.Geocode(context.Background(), "北京市海淀区上地十街10号")
	skipIfQuotaExceeded(t, err)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotZero(t, result.Location.Lat)
	require.NotZero(t, result.Location.Lng)
	t.Logf("地址: %s, 坐标: (%f, %f)", result.Address, result.Location.Lat, result.Location.Lng)
}

func TestReverseGeocode(t *testing.T) {
	client := getTestClient(t)

	// 天安门坐标
	location := Location{Lat: 39.908722, Lng: 116.397499}

	result, err := client.ReverseGeocode(context.Background(), location)
	skipIfQuotaExceeded(t, err)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Address)

	t.Logf("坐标: (%f, %f)", location.Lat, location.Lng)
	t.Logf("地址: %s", result.Address)
	t.Logf("省市区: %s %s %s", result.Province, result.City, result.District)
}
