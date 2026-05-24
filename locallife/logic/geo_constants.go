package logic

// 地理/距离相关常量，在 logic 包内集中定义，供各模块复用。
// api 层需要时可通过 logic.MetersPerLatDegree 等直接引用导出常量。
const (
	// MetersPerLatDegree 1 度纬度对应的米数（全球近似值）
	MetersPerLatDegree = 111_000

	// MetersPerLngDegree 1 度经度对应的米数（北纬 30° 附近近似值，如长三角/成都/武汉等地区）
	// 更精确的计算应使用 MetersPerLatDegree * cos(latitude)（见 delivery_quote.go 的 Haversine 降级计算）
	MetersPerLngDegree = 96_000

	// DefaultDeliveryDistance 当无法通过地图 API 获取真实路线时使用的默认代取距离（米）
	DefaultDeliveryDistance = 3_000

	// MinDeliveryDistance 骑行路线计算结果低于此值时的下限（米），防止 0 距离异常
	MinDeliveryDistance = 500
)
