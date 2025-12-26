package algorithm

import (
	"context"
	"fmt"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// MerchantForeignObjectTracker 商户异物索赔追踪器
// 设计理念：异物索赔免证秒赔，追踪频次用于通知商户整改
// - 7天内达到阈值 → 通知商户注意卫生
// - 不停止业务，食品安全问题走食安熔断流程
type MerchantForeignObjectTracker struct {
	store db.Store
}

// NewMerchantForeignObjectTracker 创建商户异物追踪器
func NewMerchantForeignObjectTracker(store db.Store) *MerchantForeignObjectTracker {
	return &MerchantForeignObjectTracker{store: store}
}

// CheckMerchantForeignObjectStatus 检查商户异物索赔状态
// 返回当前状态和是否需要发送整改通知
func (t *MerchantForeignObjectTracker) CheckMerchantForeignObjectStatus(
	ctx context.Context,
	merchantID int64,
) (*MerchantForeignObjectResult, error) {
	windowStart := time.Now().AddDate(0, 0, -MerchantForeignObjectWindowDays)

	count, err := t.store.CountMerchantClaimsByType(ctx, db.CountMerchantClaimsByTypeParams{
		MerchantID: merchantID,
		ClaimType:  ClaimTypeForeignObject,
		CreatedAt:  windowStart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count foreign object claims: %w", err)
	}

	result := &MerchantForeignObjectResult{
		MerchantID:       merchantID,
		WindowDays:       MerchantForeignObjectWindowDays,
		ForeignObjectNum: int(count),
		ShouldNotify:     false,
	}

	// 达到阈值则需要通知整改
	if count >= int64(MerchantForeignObjectWarningNum) {
		result.ShouldNotify = true
		result.Message = fmt.Sprintf(
			"您的店铺在过去%d天内收到%d次异物索赔，请注意食品卫生。",
			MerchantForeignObjectWindowDays, count,
		)
	} else {
		result.Message = "正常"
	}

	return result, nil
}

// HandleForeignObjectClaim 处理异物索赔后检查商户状态
// 返回当前状态，由调用方决定后续动作（如发送通知、拒绝新订单等）
func (t *MerchantForeignObjectTracker) HandleForeignObjectClaim(
	ctx context.Context,
	merchantID int64,
	_ int64, // claimID - 预留用于日志记录
) (*MerchantForeignObjectResult, error) {
	return t.CheckMerchantForeignObjectStatus(ctx, merchantID)
}

// GetMerchantForeignObjectHistory 获取商户异物索赔历史
func (t *MerchantForeignObjectTracker) GetMerchantForeignObjectHistory(
	ctx context.Context,
	merchantID int64,
	days int,
) ([]db.Claim, error) {
	windowStart := time.Now().AddDate(0, 0, -days)

	claims, err := t.store.ListMerchantClaimsByTypeInPeriod(ctx, db.ListMerchantClaimsByTypeInPeriodParams{
		MerchantID: merchantID,
		ClaimType:  ClaimTypeForeignObject,
		CreatedAt:  windowStart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list merchant foreign object claims: %w", err)
	}

	return claims, nil
}
