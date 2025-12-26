// Package algorithm 提供订单推荐、路径优化等算法
// 该包独立于业务逻辑，便于测试和升级
package algorithm

import "time"

// Location 地理位置
type Location struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

// PoolOrder 订单池中的订单
type PoolOrder struct {
	OrderID          int64     `json:"order_id"`
	MerchantID       int64     `json:"merchant_id"`
	PickupLocation   Location  `json:"pickup_location"`
	DeliveryLocation Location  `json:"delivery_location"`
	Distance         int       `json:"distance"`     // 配送距离（米）
	DeliveryFee      int64     `json:"delivery_fee"` // 配送费（分）
	ExpectedPickupAt time.Time `json:"expected_pickup_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	Priority         int       `json:"priority"`
	CreatedAt        time.Time `json:"created_at"`
}

// ActiveDelivery 骑手当前正在配送的订单
type ActiveDelivery struct {
	DeliveryID       int64     `json:"delivery_id"`
	OrderID          int64     `json:"order_id"`
	PickupLocation   Location  `json:"pickup_location"`
	DeliveryLocation Location  `json:"delivery_location"`
	Status           string    `json:"status"` // picking/picked/delivering
	PickedAt         time.Time `json:"picked_at,omitempty"`
}

// ScoredOrder 带推荐分数的订单
type ScoredOrder struct {
	OrderID       int64 `json:"order_id"`
	TotalScore    int   `json:"total_score"`    // 总分 0-100
	DistanceScore int   `json:"distance_score"` // 距离分
	RouteScore    int   `json:"route_score"`    // 顺路分
	UrgencyScore  int   `json:"urgency_score"`  // 时效分
	ProfitScore   int   `json:"profit_score"`   // 收益分

	// 计算结果
	DistanceToPickup int `json:"distance_to_pickup"` // 到取餐点距离（米）
	ExtraDistance    int `json:"extra_distance"`     // 额外绕路距离（米）
	EstimatedMinutes int `json:"estimated_minutes"`  // 预计配送时间（分钟）

	// 原始订单信息
	PoolOrder PoolOrder `json:"pool_order"`
}

// RecommendConfig 推荐算法配置
type RecommendConfig struct {
	DistanceWeight float64 `json:"distance_weight"` // 距离权重
	RouteWeight    float64 `json:"route_weight"`    // 顺路权重
	UrgencyWeight  float64 `json:"urgency_weight"`  // 时效权重
	ProfitWeight   float64 `json:"profit_weight"`   // 收益权重
	MaxDistance    int     `json:"max_distance"`    // 最大推荐距离（米）
	MaxResults     int     `json:"max_results"`     // 最大返回数量
}

// DefaultConfig 默认配置
func DefaultConfig() RecommendConfig {
	return RecommendConfig{
		DistanceWeight: 0.40,
		RouteWeight:    0.30,
		UrgencyWeight:  0.20,
		ProfitWeight:   0.10,
		MaxDistance:    5000,
		MaxResults:     20,
	}
}

// RecommendInput 推荐算法输入
type RecommendInput struct {
	RiderID       int64            `json:"rider_id"`
	RiderLocation Location         `json:"rider_location"`
	ActiveOrders  []ActiveDelivery `json:"active_orders"`  // 骑手当前已接订单
	AvailablePool []PoolOrder      `json:"available_pool"` // 可接订单池
	Config        RecommendConfig  `json:"config"`
}

// InsertResult 插入订单的成本计算结果
type InsertResult struct {
	ExtraDistance   int `json:"extra_distance"`    // 额外距离（米）
	ExtraTime       int `json:"extra_time"`        // 额外时间（分钟）
	BestInsertIndex int `json:"best_insert_index"` // 最佳插入位置
}
