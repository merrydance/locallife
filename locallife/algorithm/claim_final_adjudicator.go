package algorithm

import (
	"errors"
)

type ClaimFinalAdjudicator struct {
	config ClaimFinalAdjudicatorConfig
}

func NewClaimFinalAdjudicator(config ClaimFinalAdjudicatorConfig) *ClaimFinalAdjudicator {
	return &ClaimFinalAdjudicator{config: normalizeClaimFinalConfig(config)}
}

func (a *ClaimFinalAdjudicator) Adjudicate(input ClaimFinalAdjudicationInput) (ClaimFinalAdjudicationResult, error) {
	baseParty, baseCode, baseReason, err := baseClaimResponsibleParty(input.ClaimType)
	if err != nil {
		return ClaimFinalAdjudicationResult{}, err
	}

	breakdown := ClaimFinalScoreBreakdown{
		Version:              "claim_final_adjudicator_v1",
		RegionID:             input.RegionID,
		ClaimType:            input.ClaimType,
		BaseResponsibleParty: baseParty,
		Thresholds: ClaimFinalThresholds{
			MinUserOrders30d:        a.config.MinUserOrders30d,
			MinUserClaims30d:        a.config.MinUserClaims30d,
			UserClaimRate30d:        a.config.UserClaimRate30d,
			UserClaims7d:            a.config.UserClaims7d,
			UserClaimRate7d:         a.config.UserClaimRate7d,
			MinRiderOrders30d:       a.config.MinRiderOrders30d,
			RiderAbnormalRate30d:    a.config.RiderAbnormalRate30d,
			MinMerchantOrders30d:    a.config.MinMerchantOrders30d,
			MerchantAbnormalRate30d: a.config.MerchantAbnormalRate30d,
		},
	}

	reasonCodes := []string{}
	addSignal := func(detail *ClaimFinalScoreDetail, code string, weight int32, message string) {
		detail.Score += weight
		detail.Signals = append(detail.Signals, ClaimFinalScoreSignal{
			Code:    code,
			Weight:  weight,
			Message: message,
		})
		reasonCodes = appendUniqueString(reasonCodes, code)
	}

	switch baseParty {
	case "rider":
		addSignal(&breakdown.Scores.RiderLiability, baseCode, baseWeight(input.ClaimType), baseReason)
	case "merchant":
		addSignal(&breakdown.Scores.MerchantLiability, baseCode, baseWeight(input.ClaimType), baseReason)
	}
	addSignal(&breakdown.Scores.Confidence, "base_responsibility_matched", 40, "裁定符合索赔类型基线责任")

	if input.User.MaliciousConfirmedClaims > 0 {
		addSignal(&breakdown.Scores.UserRisk, "historical_malicious_confirmed", 100, "用户存在历史恶意索赔确认记录")
	}
	if input.User.TotalOrders30d >= a.config.MinUserOrders30d &&
		input.User.AbnormalClaims30d >= a.config.MinUserClaims30d &&
		rate(input.User.AbnormalClaims30d, input.User.TotalOrders30d) >= a.config.UserClaimRate30d {
		addSignal(&breakdown.Scores.UserRisk, "user_claim_rate_30d_exceeded", 70, "用户30天索赔频率超过平台阈值")
	}
	if input.User.AbnormalClaims7d >= a.config.UserClaims7d &&
		rate(input.User.AbnormalClaims7d, input.User.TotalOrders7d) >= a.config.UserClaimRate7d {
		addSignal(&breakdown.Scores.UserRisk, "user_claim_rate_7d_exceeded", 50, "用户7天索赔频率超过平台阈值")
	}
	if (input.User.SharedDeviceOtherUsers > 0 || input.User.SharedAddressOtherUsers > 0) &&
		input.User.NetAbnormalClaims30d >= 3 {
		addSignal(&breakdown.Scores.UserRisk, "shared_graph_repeat_claim_pattern", 80, "用户存在共享设备或地址下的重复异常索赔特征")
	}

	if baseParty == "rider" && input.Rider != nil &&
		input.Rider.TotalOrders30d >= a.config.MinRiderOrders30d &&
		rate(input.Rider.AbnormalClaims30d, input.Rider.TotalOrders30d) >= a.config.RiderAbnormalRate30d {
		addSignal(&breakdown.Scores.RiderLiability, "rider_abnormal_rate_30d_exceeded", 30, "骑手30天异常索赔率超过平台阈值")
		if input.User.AbnormalClaims30d == 0 {
			addSignal(&breakdown.Scores.Confidence, "clean_user_rider_abnormal_history", 20, "用户历史正常且骑手历史异常")
		}
	}
	if baseParty == "rider" && input.Rider != nil && input.Rider.TotalOrders7d >= 3 && input.Rider.AbnormalClaims7d >= 2 {
		addSignal(&breakdown.Scores.RiderLiability, "rider_recent_abnormal_claims_7d", 20, "骑手7天内存在多次异常索赔")
	}

	if baseParty == "merchant" && input.Merchant.TotalOrders30d >= a.config.MinMerchantOrders30d &&
		rate(input.Merchant.AbnormalClaims30d, input.Merchant.TotalOrders30d) >= a.config.MerchantAbnormalRate30d {
		addSignal(&breakdown.Scores.MerchantLiability, "merchant_abnormal_rate_30d_exceeded", 30, "商户30天异常索赔率超过平台阈值")
		if input.User.AbnormalClaims30d == 0 {
			addSignal(&breakdown.Scores.Confidence, "clean_user_merchant_abnormal_history", 20, "用户历史正常且商户历史异常")
		}
	}
	if baseParty == "merchant" && input.Merchant.TotalOrders7d >= 3 && input.Merchant.AbnormalClaims7d >= 2 {
		addSignal(&breakdown.Scores.MerchantLiability, "merchant_recent_abnormal_claims_7d", 20, "商户7天内存在多次异常索赔")
	}

	result := ClaimFinalAdjudicationResult{
		BaseResponsibleParty: baseParty,
		Reason:               baseReason,
		ReasonCodes:          reasonCodes,
		BehaviorStatus:       ClaimBehaviorNormal,
	}
	if breakdown.Scores.UserRisk.Score >= 70 {
		result.DecisionMode = FinalDecisionUserRestricted
		result.ResponsibleParty = "user"
		result.CompensationSource = CompensationSourcePlatform
		result.BehaviorStatus = ClaimBehaviorUserRestricted
		result.Reason = "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。"
	} else if baseParty == "merchant" {
		result.DecisionMode = FinalDecisionMerchantRecovery
		result.ResponsibleParty = "merchant"
		result.CompensationSource = CompensationSourceMerchant
	} else {
		result.DecisionMode = FinalDecisionRiderRecovery
		result.ResponsibleParty = "rider"
		result.CompensationSource = CompensationSourceRider
	}

	breakdown.FinalDecisionMode = result.DecisionMode
	breakdown.Scores.UserRisk.Level = scoreLevel(breakdown.Scores.UserRisk.Score)
	breakdown.Scores.RiderLiability.Level = scoreLevel(breakdown.Scores.RiderLiability.Score)
	breakdown.Scores.MerchantLiability.Level = scoreLevel(breakdown.Scores.MerchantLiability.Score)
	breakdown.Scores.Confidence.Level = scoreLevel(breakdown.Scores.Confidence.Score)
	result.ScoreBreakdown = breakdown

	return result, nil
}

func normalizeClaimFinalConfig(config ClaimFinalAdjudicatorConfig) ClaimFinalAdjudicatorConfig {
	defaults := DefaultClaimFinalAdjudicatorConfig()
	if config.MinUserOrders30d <= 0 {
		config.MinUserOrders30d = defaults.MinUserOrders30d
	}
	if config.MinUserClaims30d <= 0 {
		config.MinUserClaims30d = defaults.MinUserClaims30d
	}
	if config.UserClaimRate30d <= 0 {
		config.UserClaimRate30d = defaults.UserClaimRate30d
	}
	if config.UserClaims7d <= 0 {
		config.UserClaims7d = defaults.UserClaims7d
	}
	if config.UserClaimRate7d <= 0 {
		config.UserClaimRate7d = defaults.UserClaimRate7d
	}
	if config.MinRiderOrders30d <= 0 {
		config.MinRiderOrders30d = defaults.MinRiderOrders30d
	}
	if config.RiderAbnormalRate30d <= 0 {
		config.RiderAbnormalRate30d = defaults.RiderAbnormalRate30d
	}
	if config.MinMerchantOrders30d <= 0 {
		config.MinMerchantOrders30d = defaults.MinMerchantOrders30d
	}
	if config.MerchantAbnormalRate30d <= 0 {
		config.MerchantAbnormalRate30d = defaults.MerchantAbnormalRate30d
	}
	return config
}

func baseClaimResponsibleParty(claimType string) (party string, code string, reason string, err error) {
	switch claimType {
	case ClaimTypeTimeout:
		return "rider", "base_type_timeout_rider", "服务侧异常索赔默认由骑手承担责任", nil
	case ClaimTypeDamage:
		return "rider", "base_type_damage_rider", "餐损由取餐骑手承担基线责任", nil
	case ClaimTypeForeignObject:
		return "merchant", "base_type_foreign_object_merchant", "销售侧异常索赔默认由商户承担责任", nil
	case ClaimTypeFoodSafety:
		return "", "", "", errors.New("food safety claims must use the dedicated food safety workflow")
	default:
		return "", "", "", errors.New("unsupported claim type for final adjudication")
	}
}

func baseWeight(claimType string) int32 {
	if claimType == ClaimTypeTimeout {
		return 60
	}
	return 70
}

func rate(count, total int32) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total)
}

func scoreLevel(score int32) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 30:
		return "medium"
	default:
		return "low"
	}
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
