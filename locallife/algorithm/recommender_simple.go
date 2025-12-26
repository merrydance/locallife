package algorithm

import (
	"context"
	"sort"
	"time"
)

// SimpleRecommender V1 简单推荐算法
// 基于距离、顺路、时效、收益四个维度计算推荐分数
type SimpleRecommender struct{}

// NewSimpleRecommender 创建简单推荐算法实例
func NewSimpleRecommender() *SimpleRecommender {
	return &SimpleRecommender{}
}

func (r *SimpleRecommender) Name() string {
	return "SimpleRecommender"
}

func (r *SimpleRecommender) Version() string {
	return "1.0.0"
}

// Recommend 为骑手推荐订单
func (r *SimpleRecommender) Recommend(ctx context.Context, input RecommendInput) ([]ScoredOrder, error) {
	if len(input.AvailablePool) == 0 {
		return []ScoredOrder{}, nil
	}

	config := input.Config
	if config.MaxResults <= 0 {
		config.MaxResults = 20
	}
	if config.MaxDistance <= 0 {
		config.MaxDistance = 5000
	}

	var scored []ScoredOrder
	now := time.Now()

	for _, order := range input.AvailablePool {
		// 检查是否过期
		if order.ExpiresAt.Before(now) {
			continue
		}

		// 计算到取餐点的距离
		distanceToPickup := HaversineDistance(input.RiderLocation, order.PickupLocation)

		// 超过最大距离则跳过
		if distanceToPickup > config.MaxDistance {
			continue
		}

		// 计算各维度分数
		distanceScore := r.calculateDistanceScore(distanceToPickup, config.MaxDistance)
		routeScore := r.calculateRouteScore(order, input.ActiveOrders, input.RiderLocation)
		urgencyScore := r.calculateUrgencyScore(order, now)
		profitScore := r.calculateProfitScore(order)

		// 加权计算总分
		totalScore := int(
			float64(distanceScore)*config.DistanceWeight +
				float64(routeScore)*config.RouteWeight +
				float64(urgencyScore)*config.UrgencyWeight +
				float64(profitScore)*config.ProfitWeight,
		)

		// 预计配送时间
		estimatedMinutes := EstimateTime(distanceToPickup + order.Distance)

		scored = append(scored, ScoredOrder{
			OrderID:          order.OrderID,
			TotalScore:       totalScore,
			DistanceScore:    distanceScore,
			RouteScore:       routeScore,
			UrgencyScore:     urgencyScore,
			ProfitScore:      profitScore,
			DistanceToPickup: distanceToPickup,
			ExtraDistance:    r.calculateExtraDistance(order, input.ActiveOrders, input.RiderLocation),
			EstimatedMinutes: estimatedMinutes,
			PoolOrder:        order,
		})
	}

	// 按总分降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].TotalScore > scored[j].TotalScore
	})

	// 返回 Top N
	if len(scored) > config.MaxResults {
		scored = scored[:config.MaxResults]
	}

	return scored, nil
}

// calculateDistanceScore 计算距离分 (0-100)
// 距离越近分数越高
func (r *SimpleRecommender) calculateDistanceScore(distance, maxDistance int) int {
	if distance <= 0 {
		return 100
	}
	if distance >= maxDistance {
		return 0
	}
	// 线性衰减
	return 100 - int(float64(distance)/float64(maxDistance)*100)
}

// calculateRouteScore 计算顺路分 (0-100)
// 考虑新订单是否与已有订单路线重合
func (r *SimpleRecommender) calculateRouteScore(order PoolOrder, activeOrders []ActiveDelivery, riderLocation Location) int {
	if len(activeOrders) == 0 {
		// 没有已接订单，任何订单都算顺路
		return 100
	}

	// 计算当前配送路径的方向（只看第一个活跃订单）
	var targetLocation Location
	if len(activeOrders) > 0 {
		active := activeOrders[0]
		switch active.Status {
		case "picking":
			targetLocation = active.PickupLocation
		case "picked", "delivering":
			targetLocation = active.DeliveryLocation
		}
	}

	if targetLocation.Longitude == 0 && targetLocation.Latitude == 0 {
		return 100
	}

	// 计算当前方向
	currentBearing := Bearing(riderLocation, targetLocation)
	// 计算新订单取餐点方向
	newBearing := Bearing(riderLocation, order.PickupLocation)

	// 方向夹角越小，顺路分越高
	diff := currentBearing - newBearing
	if diff < 0 {
		diff = -diff
	}
	if diff > 180 {
		diff = 360 - diff
	}

	// 0度=100分，90度=50分，180度=0分
	score := 100 - int(diff/180*100)
	if score < 0 {
		score = 0
	}
	return score
}

// calculateUrgencyScore 计算时效分 (0-100)
// 越接近过期时间分数越高
func (r *SimpleRecommender) calculateUrgencyScore(order PoolOrder, now time.Time) int {
	remaining := order.ExpiresAt.Sub(now)

	// 已过期
	if remaining <= 0 {
		return 0
	}

	// 剩余时间越少，紧急度越高
	// 5分钟内 = 100分
	// 30分钟 = 50分
	// 60分钟 = 0分
	minutes := remaining.Minutes()
	if minutes <= 5 {
		return 100
	}
	if minutes >= 60 {
		return 0
	}

	return int(100 - (minutes-5)/55*100)
}

// calculateProfitScore 计算收益分 (0-100)
// 配送费/距离 的性价比
func (r *SimpleRecommender) calculateProfitScore(order PoolOrder) int {
	if order.Distance <= 0 {
		return 100
	}

	// 计算每公里配送费（分/公里）
	feePerKm := float64(order.DeliveryFee) / (float64(order.Distance) / 1000)

	// 假设 5元/公里 = 100分，0元/公里 = 0分
	// 500分/公里 = 100分
	score := int(feePerKm / 500 * 100)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// calculateExtraDistance 计算额外绕路距离
func (r *SimpleRecommender) calculateExtraDistance(order PoolOrder, activeOrders []ActiveDelivery, riderLocation Location) int {
	if len(activeOrders) == 0 {
		return 0
	}

	// 简化计算：额外距离 = 到新订单取餐点的距离
	// 更精确的计算需要考虑路径规划
	return HaversineDistance(riderLocation, order.PickupLocation)
}

// 确保实现了接口
var _ OrderRecommender = (*SimpleRecommender)(nil)
