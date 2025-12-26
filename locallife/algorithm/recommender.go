package algorithm

import "context"

// OrderRecommender 订单推荐算法接口
// 便于后期升级算法实现
type OrderRecommender interface {
	// Recommend 为骑手推荐订单
	// 返回按推荐分数排序的订单列表
	Recommend(ctx context.Context, input RecommendInput) ([]ScoredOrder, error)

	// Name 返回算法名称
	Name() string

	// Version 返回算法版本
	Version() string
}

// RouteOptimizer 路径优化器接口
type RouteOptimizer interface {
	// OptimalPath 计算最优配送路径
	// 输入：起点、取餐点列表、送餐点列表
	// 输出：最优访问顺序
	OptimalPath(start Location, pickups, deliveries []Location) []Location

	// InsertCost 计算插入新订单的额外成本
	// 用于评估是否应该接受新订单
	InsertCost(currentPath []Location, newPickup, newDelivery Location) InsertResult
}
