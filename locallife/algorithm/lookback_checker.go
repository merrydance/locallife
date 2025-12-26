package algorithm

import (
	"context"
	"fmt"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// LookbackChecker 回溯检查器
type LookbackChecker struct {
	store db.Store
}

// NewLookbackChecker 创建回溯检查器
func NewLookbackChecker(store db.Store) *LookbackChecker {
	return &LookbackChecker{
		store: store,
	}
}

// PerformLookback 执行回溯检查
// 逻辑：
// 1. 尝试查询最近30天的5笔订单
// 2. 如果不足5笔，扩展到90天
// 3. 如果仍不足5笔，扩展到1年
// 4. 返回实际检查的订单和索赔记录
func (lc *LookbackChecker) PerformLookback(
	ctx context.Context,
	userID int64,
	targetOrders int,
) (*LookbackResult, error) {
	// Step 1: 尝试30天
	result, err := lc.lookbackInPeriod(ctx, userID, Lookback30Days, targetOrders)
	if err != nil {
		return nil, err
	}

	if result.OrdersChecked >= targetOrders {
		result.Period = "30d"
		return result, nil
	}

	// Step 2: 扩展到90天
	result, err = lc.lookbackInPeriod(ctx, userID, Lookback90Days, targetOrders)
	if err != nil {
		return nil, err
	}

	if result.OrdersChecked >= targetOrders {
		result.Period = "90d"
		return result, nil
	}

	// Step 3: 扩展到1年
	result, err = lc.lookbackInPeriod(ctx, userID, Lookback1Year, targetOrders)
	if err != nil {
		return nil, err
	}

	result.Period = "1y"
	return result, nil
}

// lookbackInPeriod 在指定时间段内回溯检查
func (lc *LookbackChecker) lookbackInPeriod(
	ctx context.Context,
	userID int64,
	duration time.Duration,
	limit int,
) (*LookbackResult, error) {
	startTime := time.Now().Add(-duration)

	// 查询用户在时间段内的索赔记录（按创建时间倒序，最多取limit条）
	claims, err := lc.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
		UserID:    userID,
		CreatedAt: startTime,
	})
	if err != nil {
		return nil, err
	}

	// 提取订单ID
	orderIDs := make([]int64, 0, len(claims))
	orderMap := make(map[int64]bool)
	for _, claim := range claims {
		if !orderMap[claim.OrderID] {
			orderIDs = append(orderIDs, claim.OrderID)
			orderMap[claim.OrderID] = true
		}
	}

	// 限制订单数量
	if len(orderIDs) > limit {
		orderIDs = orderIDs[:limit]
	}

	// 提取商户ID和骑手ID（批量查询orders和deliveries）
	merchants := []int64{}
	riders := []int64{}

	if len(orderIDs) > 0 {
		orderInfo, err := lc.store.GetOrdersMerchantAndRider(ctx, orderIDs)
		if err != nil {
			// 查询失败不影响主流程，记录日志即可
			fmt.Printf("Failed to get order merchant/rider info: %v\n", err)
		} else {
			for _, info := range orderInfo {
				merchants = append(merchants, info.MerchantID)
				if info.RiderID.Valid {
					riders = append(riders, info.RiderID.Int64)
				}
			}
		}
	}

	return &LookbackResult{
		OrdersChecked: len(orderIDs),
		Orders:        orderIDs,
		ClaimsFound:   len(claims),
		Claims:        claims,
		Merchants:     UniqueInt64(merchants),
		Riders:        UniqueInt64(riders),
	}, nil
}

// CheckClaimCorrelation 检查索赔相关性
// 判断是否为可疑模式：
// 1. 时间集中：3次索赔在72小时内
// 2. 同一商户：80%以上索赔针对同一商户
// 3. 同一骑手：80%以上索赔针对同一骑手
// 4. 高频索赔：7天内3次以上
func (lc *LookbackChecker) CheckClaimCorrelation(
	ctx context.Context,
	userID int64,
	claims []db.Claim,
) *CorrelationResult {
	if len(claims) == 0 {
		return &CorrelationResult{
			IsSuspicious: false,
			Pattern:      "无索赔记录",
		}
	}

	result := &CorrelationResult{}
	patterns := []string{}

	// 检查1: 时间集中（连续3天内发生3次）
	if len(claims) >= 3 {
		timeDiff := claims[0].CreatedAt.Sub(claims[len(claims)-1].CreatedAt)
		if timeDiff <= 72*time.Hour {
			result.TimeConcentrated = true
			patterns = append(patterns, fmt.Sprintf("时间集中（72小时内%d次）", len(claims)))
		}
	}

	// 检查2: 同一商户（80%以上索赔针对同一商户）
	orderIDs := make([]int64, 0, len(claims))
	for _, claim := range claims {
		orderIDs = append(orderIDs, claim.OrderID)
	}

	if len(orderIDs) > 0 {
		orderInfo, err := lc.store.GetOrdersMerchantAndRider(ctx, orderIDs)
		if err == nil && len(orderInfo) > 0 {
			// 统计商户出现次数
			merchantCounts := make(map[int64]int)
			riderCounts := make(map[int64]int)

			for _, info := range orderInfo {
				merchantCounts[info.MerchantID]++
				if info.RiderID.Valid {
					riderCounts[info.RiderID.Int64]++
				}
			}

			// 检查是否有商户占比超过80%
			totalOrders := len(orderInfo)
			for merchantID, count := range merchantCounts {
				if float64(count)/float64(totalOrders) >= 0.8 {
					result.SameMerchant = true
					patterns = append(patterns, fmt.Sprintf("同一商户集中(商户%d占%d%%)", merchantID, count*100/totalOrders))
					break
				}
			}

			// 检查是否有骑手占比超过80%
			if len(riderCounts) > 0 {
				for riderID, count := range riderCounts {
					if float64(count)/float64(totalOrders) >= 0.8 {
						result.SameRider = true
						patterns = append(patterns, fmt.Sprintf("同一骑手集中(骑手%d占%d%%)", riderID, count*100/totalOrders))
						break
					}
				}
			}
		}
	}

	// 检查4: 高频索赔（7天内3次+）
	now := time.Now()
	recent7dCount := 0
	for _, claim := range claims {
		if now.Sub(claim.CreatedAt) <= Recent7Days {
			recent7dCount++
		}
	}

	if recent7dCount >= ClaimWarningClaimCount { // 使用新常量：3次
		result.HighFrequency = true
		patterns = append(patterns, fmt.Sprintf("高频索赔（7天内%d次）", recent7dCount))
	}

	// 综合判断
	result.IsSuspicious = result.TimeConcentrated ||
		result.SameMerchant ||
		result.SameRider ||
		result.HighFrequency

	if result.IsSuspicious {
		if len(patterns) > 0 {
			result.Pattern = "可疑模式"
			result.Details = joinStrings(patterns, " + ")
		}
	} else {
		result.Pattern = "正常分散模式"
		result.Details = "索赔分散在不同时间、不同商户"
	}

	return result
}

// GetUserRecentClaimCount 获取用户最近的索赔次数
func (lc *LookbackChecker) GetUserRecentClaimCount(
	ctx context.Context,
	userID int64,
	duration time.Duration,
) (int, error) {
	startTime := time.Now().Add(-duration)
	count, err := lc.store.CountUserClaimsInPeriod(ctx, db.CountUserClaimsInPeriodParams{
		UserID:    userID,
		CreatedAt: startTime,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Helper: joinStrings 连接字符串
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
