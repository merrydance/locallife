package algorithm

import (
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// Claim types
const (
	ClaimTypeForeignObject = "foreign-object"
	ClaimTypeDamage        = "damage"
	ClaimTypeTimeout       = "timeout"
	ClaimTypeFoodSafety    = "food-safety"
)

// Approval types
const (
	ApprovalTypeInstant = "instant"
	ApprovalTypeAuto    = "auto"
	ApprovalTypeManual  = "manual"
)

// Compensation sources
const (
	CompensationSourceMerchant = "merchant"
	CompensationSourceRider    = "rider"
	CompensationSourcePlatform = "platform"
)

// Claim behavior statuses
const (
	ClaimBehaviorNormal        = "normal"
	ClaimBehaviorWarned        = "warned"
	ClaimBehaviorPlatformPay   = "platform-pay"
	ClaimBehaviorRejectService = "reject-service"
)

// Claim statuses
const (
	ClaimStatusPending      = "pending"
	ClaimStatusAutoApproved = "auto-approved"
	ClaimStatusManualReview = "manual-review"
)

// Behavior thresholds
const (
	ClaimWarningOrderCount          = 5
	ClaimWarningClaimCount          = 3
	ClaimWarningRatio               = 0.6
	DamageIncidentsIn7Days          = 3
	FoodSafetyReportsIn1Hour        = 3
	MerchantForeignObjectWindowDays = 7
	MerchantForeignObjectWarningNum = 3
	HighMatchCount                  = 2
)

// Time windows
const (
	Lookback30Days = 30 * 24 * time.Hour
	Lookback90Days = 90 * 24 * time.Hour
	Lookback1Year  = 365 * 24 * time.Hour
	Recent7Days    = 7 * 24 * time.Hour
)

// Decision 审批决策
type Decision struct {
	Type               string          `json:"type"`
	Approved           bool            `json:"approved"`
	Amount             int64           `json:"amount"`
	Reason             string          `json:"reason"`
	BehaviorStatus     string          `json:"behavior_status,omitempty"`
	CompensationSource string          `json:"compensation_source,omitempty"`
	NeedsReview        bool            `json:"needs_review,omitempty"`
	ReviewMessage      string          `json:"review_message,omitempty"`
	Warning            string          `json:"warning,omitempty"`
	ShouldWarn         bool            `json:"should_warn,omitempty"`
	LookbackData       *LookbackResult `json:"lookback_data,omitempty"`
}

// ClaimBehaviorResult 用户索赔行为评估结果
type ClaimBehaviorResult struct {
	RecentMonths  int     `json:"recent_months"`
	TakeoutOrders int     `json:"takeout_orders"`
	ClaimCount    int     `json:"claim_count"`
	WarningCount  int     `json:"warning_count"`
	ClaimRatio    float64 `json:"claim_ratio"`
	Status        string  `json:"status"`
	ShouldWarn    bool    `json:"should_warn,omitempty"`
	RejectService bool    `json:"reject_service,omitempty"`
	Message       string  `json:"message,omitempty"`
}

// LookbackResult 回溯检查结果
type LookbackResult struct {
	Period        string     `json:"period,omitempty"`
	OrdersChecked int        `json:"orders_checked"`
	Orders        []int64    `json:"orders"`
	ClaimsFound   int        `json:"claims_found"`
	Claims        []db.Claim `json:"claims"`
	Merchants     []int64    `json:"merchants"`
	Riders        []int64    `json:"riders"`
}

// CorrelationResult 索赔相关性检查结果
type CorrelationResult struct {
	IsSuspicious     bool   `json:"is_suspicious"`
	Pattern          string `json:"pattern"`
	Details          string `json:"details,omitempty"`
	TimeConcentrated bool   `json:"time_concentrated,omitempty"`
	SameMerchant     bool   `json:"same_merchant,omitempty"`
	SameRider        bool   `json:"same_rider,omitempty"`
	HighFrequency    bool   `json:"high_frequency,omitempty"`
}

// FoodSafetyCheckResult 食安检查结果
type FoodSafetyCheckResult struct {
	ShouldCircuitBreak bool   `json:"should_circuit_break"`
	IsMalicious        bool   `json:"is_malicious"`
	ReasonCode         string `json:"reason_code"`
	Message            string `json:"message"`
	DurationHours      int    `json:"duration_hours"`
}

// MerchantForeignObjectResult 商户异物索赔结果
type MerchantForeignObjectResult struct {
	MerchantID       int64  `json:"merchant_id"`
	WindowDays       int    `json:"window_days"`
	ForeignObjectNum int    `json:"foreign_object_num"`
	ShouldNotify     bool   `json:"should_notify"`
	Message          string `json:"message"`
}

// UniqueInt64 去重
func UniqueInt64(values []int64) []int64 {
	if len(values) == 0 {
		return values
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}
