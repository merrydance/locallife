package algorithm

import "math"

const (
	// 地球半径（米）
	earthRadius = 6371000

	// 平均骑行速度（米/秒）- 约 20km/h
	avgRidingSpeed = 5.5
)

// HaversineDistance 计算两点间的球面距离（米）
// 使用 Haversine 公式
func HaversineDistance(loc1, loc2 Location) int {
	lat1 := toRadians(loc1.Latitude)
	lat2 := toRadians(loc2.Latitude)
	deltaLat := toRadians(loc2.Latitude - loc1.Latitude)
	deltaLng := toRadians(loc2.Longitude - loc1.Longitude)

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return int(earthRadius * c)
}

// EstimateTime 估算骑行时间（分钟）
func EstimateTime(distanceMeters int) int {
	if distanceMeters <= 0 {
		return 0
	}
	seconds := float64(distanceMeters) / avgRidingSpeed
	return int(math.Ceil(seconds / 60))
}

// toRadians 角度转弧度
func toRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// Bearing 计算从 loc1 到 loc2 的方位角（0-360度，北为0）
func Bearing(loc1, loc2 Location) float64 {
	lat1 := toRadians(loc1.Latitude)
	lat2 := toRadians(loc2.Latitude)
	deltaLng := toRadians(loc2.Longitude - loc1.Longitude)

	y := math.Sin(deltaLng) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(deltaLng)
	bearing := math.Atan2(y, x)

	// 转换为 0-360 度
	return math.Mod(toDegrees(bearing)+360, 360)
}

// toDegrees 弧度转角度
func toDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

// IsInSameDirection 判断两个方向是否大致相同（夹角小于阈值）
func IsInSameDirection(bearing1, bearing2 float64, thresholdDegrees float64) bool {
	diff := math.Abs(bearing1 - bearing2)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff <= thresholdDegrees
}

// CenterPoint 计算多个点的中心点
func CenterPoint(locations []Location) Location {
	if len(locations) == 0 {
		return Location{}
	}
	if len(locations) == 1 {
		return locations[0]
	}

	var sumLat, sumLng float64
	for _, loc := range locations {
		sumLat += loc.Latitude
		sumLng += loc.Longitude
	}

	n := float64(len(locations))
	return Location{
		Longitude: sumLng / n,
		Latitude:  sumLat / n,
	}
}

// BoundingBox 计算包围盒
type BoundingBox struct {
	MinLng float64
	MaxLng float64
	MinLat float64
	MaxLat float64
}

// GetBoundingBox 获取一组位置的包围盒
func GetBoundingBox(locations []Location) BoundingBox {
	if len(locations) == 0 {
		return BoundingBox{}
	}

	bb := BoundingBox{
		MinLng: locations[0].Longitude,
		MaxLng: locations[0].Longitude,
		MinLat: locations[0].Latitude,
		MaxLat: locations[0].Latitude,
	}

	for _, loc := range locations[1:] {
		if loc.Longitude < bb.MinLng {
			bb.MinLng = loc.Longitude
		}
		if loc.Longitude > bb.MaxLng {
			bb.MaxLng = loc.Longitude
		}
		if loc.Latitude < bb.MinLat {
			bb.MinLat = loc.Latitude
		}
		if loc.Latitude > bb.MaxLat {
			bb.MaxLat = loc.Latitude
		}
	}

	return bb
}

// ExpandBoundingBox 扩展包围盒（米）
func ExpandBoundingBox(bb BoundingBox, meters int) BoundingBox {
	// 纬度方向：1度约111km
	latDelta := float64(meters) / 111000.0

	// 经度方向：1度约111km * cos(lat)
	avgLat := (bb.MinLat + bb.MaxLat) / 2
	lngDelta := float64(meters) / (111000.0 * math.Cos(toRadians(avgLat)))

	return BoundingBox{
		MinLng: bb.MinLng - lngDelta,
		MaxLng: bb.MaxLng + lngDelta,
		MinLat: bb.MinLat - latDelta,
		MaxLat: bb.MaxLat + latDelta,
	}
}
