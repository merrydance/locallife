package algorithm

import (
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// TrustScore信任分系统 - 核心类型定义

// Decision 索赔决策结果
type Decision struct {
	Type              string          // instant, auto, manual, evidence-required, platform-pay
	Approved          bool            // 是否赔付（新设计下总是true，除非需要证据）
	Amount            int64           // 赔付金额
	Reason            string          // 决策原因
	LookbackData      *LookbackResult // 回溯检查数据
	BehaviorStatus    string          // 用户行为状态（normal, warned, evidence-required, platform-pay）
	CompensationSource string         // 赔付来源（merchant, rider, platform）
	NeedsEvidence     bool            // 是否需要证据
	NeedsReview       bool            // 是否需要人工审核（仅食安）
	ReviewMessage     string          // 审核提示信息
	Warning           string          // 给用户的警告信息
	ShouldWarn        bool            // 是否应该发出警告（用于触发警告记录）
}

// ClaimBehaviorResult 用户索赔行为检查结果
type ClaimBehaviorResult struct {
	Status           string  // normal, warned, evidence-required, platform-pay, reject-service
	RecentMonths     int     // 检查的月数
	TakeoutOrders    int     // 外卖订单数
	ClaimCount       int     // 索赔次数
	ClaimRatio       float64 // 索赔比例
	WarningCount     int     // 已被警告次数
	Message          string  // 提示信息
	ShouldWarn       bool    // 是否应该警告用户
	NeedsEvidence    bool    // 是否需要证据
	PlatformPay      bool    // 是否平台垫付
	RejectService    bool    // 是否拒绝服务
}

// LookbackResult 回溯检查结果
type LookbackResult struct {
	Period        string     `json:"period"`         // "30d", "90d", "1y"
	OrdersChecked int        `json:"orders_checked"` // 实际检查的订单数
	Orders        []int64    `json:"orders"`         // 订单ID列表
	ClaimsFound   int        `json:"claims_found"`   // 发现的索赔数
	Claims        []db.Claim `json:"claims"`         // 索赔列表
	Merchants     []int64    `json:"merchants"`      // 涉及的商户ID
	Riders        []int64    `json:"riders"`         // 涉及的骑手ID（可选）
}

// CorrelationResult 索赔相关性检查结果
type CorrelationResult struct {
	IsSuspicious     bool   // 是否可疑
	TimeConcentrated bool   // 时间集中（连续3天内3次）
	SameMerchant     bool   // 同一商户（80%以上）
	SameRider        bool   // 同一骑手（80%以上）
	HighFrequency    bool   // 高频（7天内3次+）
	Pattern          string // 模式描述
	Details          string // 详细说明
}

// FraudDetectionResult 欺诈检测结果
type FraudDetectionResult struct {
	IsFraud           bool
	PatternType       string // device-reuse, address-cluster, coordinated-claims
	Confidence        int    // 匹配规则数量
	RelatedUserIDs    []int64
	RelatedClaimIDs   []int64
	Description       string
	ShouldBlock       bool  // 是否应该立即拉黑
	MerchantSuspect   bool  // 是否商户可疑（多个独立用户投诉同一商户）
	SuspectMerchantID int64 // 可疑商户ID
}

// FoodSafetyCheckResult 食安检查结果
type FoodSafetyCheckResult struct {
	ShouldCircuitBreak bool   // 是否应该熔断
	IsMalicious        bool   // 是否恶作剧
	ReasonCode         string // 熔断原因代码
	Message            string // 消息
	DurationHours      int    // 熔断时长（小时）
}

// MerchantForeignObjectResult 商户异物追踪结果
type MerchantForeignObjectResult struct {
	MerchantID       int64  // 商户ID
	WindowDays       int    // 追踪窗口（天）
	ForeignObjectNum int    // 异物索赔次数
	ShouldNotify     bool   // 是否需要通知整改
	Message          string // 提示信息
}

// RiderHighValueQualification 骑手高值单资格检查结果
type RiderHighValueQualification struct {
	RiderID           int64    // 骑手ID
	WindowDays        int      // 考核窗口（天）
	IsQualified       bool     // 是否有资格接高值单
	BasicOrdersCount  int      // 基本单完成数量
	TimeoutCount      int      // 超时次数
	DamageCount       int      // 餐损次数
	CompletionRate    float64  // 完成率
	DisqualifyReasons []string // 不合格原因列表
	Message           string   // 提示信息
}

// TrustScore常量定义
// 设计理念："你的行为决定你是谁" + "惹不起躲得起"
// - 所有索赔都赔付，不存在不赔的情况
// - 通过行为回溯标定用户是谁，而不是决定赔不赔
// - 问题用户照赔，但平台垫付（退还骑手/商户），然后拒绝服务
const (
	// 信任分阈值（新体系：100分制，只降不升）
	TrustScoreMax           = 100 // 最高分/初始分
	TrustScoreInitial       = 100 // 初始分（人人平等）
	TrustScoreMin           = 0   // 最低分
	TrustScoreRejectService = 70  // 拒绝服务阈值（<70分）
	TrustScoreWarning       = 85  // 警告阈值（需要证据）

	// 旧体系常量（兼容过渡期，后续删除）
	TrustScoreHighTrust        = 750 // @deprecated 高信任阈值
	TrustScoreMediumTrust      = 600 // @deprecated 中信任阈值
	TrustScoreBlacklistUser    = 300 // @deprecated 顾客拉黑阈值
	TrustScoreSuspendMerchant  = 400 // @deprecated 商户停业阈值
	TrustScoreSuspendRider     = 350 // @deprecated 骑手暂停阈值
	TrustScoreFoodSafetyReport = 800 // @deprecated 食安快速熔断阈值

	// 信任分恢复机制
	RecoveryPeriodMonths       = 6  // 正常用户恢复周期（月）
	SevereRecoveryPeriodMonths = 12 // 严重违规恢复周期（月）

	// 时间窗口
	Lookback30Days  = 30 * 24 * time.Hour
	Lookback90Days  = 90 * 24 * time.Hour
	Lookback3Months = 90 * 24 * time.Hour // 行为回溯窗口（3个月）
	Lookback1Year   = 365 * 24 * time.Hour
	Recent7Days     = 7 * 24 * time.Hour
	Recent1Hour     = 1 * time.Hour

	// 行为回溯阈值（新设计核心）
	ClaimWarningOrderCount = 5 // 订单数阈值
	ClaimWarningClaimCount = 3 // 索赔数阈值（5单3索赔触发警告）
	ClaimWarningRatio      = 0.6 // 索赔比例阈值（60%）

	// 超时判定
	SevereTimeoutMinutes = 30 // 严重超时阈值（分钟）

	// ========================================
	// 商户监管规则
	// ========================================
	// 异物索赔追踪（免证秒赔，达到阈值通知整改）
	MerchantForeignObjectWindowDays = 7 // 异物追踪时间窗口（天）
	MerchantForeignObjectWarningNum = 3 // 通知整改阈值：7天内3单

	// 食安规则（需要人工审核，熔断后需人工恢复）
	FoodSafetyReportsIn1Hour = 3 // 1小时内3次举报触发检查

	// ========================================
	// 骑手高值单资格积分规则（长期积累机制）
	// ========================================
	// 积分变更规则
	PremiumScoreNormalOrder  = 1   // 完成普通单 +1
	PremiumScorePremiumOrder = -3  // 完成高值单 -3
	PremiumScoreTimeout      = -5  // 超时 -5
	PremiumScoreDamage       = -10 // 餐损 -10
	
	// 高值单运费阈值（分）
	HighValueOrderThreshold = 1000 // 运费≥10元为高值单

	// ========================================
	// 旧常量（兼容过渡期，后续删除）
	// ========================================
	ForeignObjectsIn7Days = 3 // @deprecated 使用 MerchantForeignObjectWarningNum

	// 骑手阈值（旧）
	DamageIncidentsIn7Days  = 3 // @deprecated
	TimeoutIncidentsIn7Days = 3 // @deprecated

	// 团伙欺诈阈值
	MinUsersForFraud  = 3 // 最少3个账号构成团伙
	MinClaimsForFraud = 3 // 最少3次索赔构成欺诈
	HighMatchCount    = 2 // 匹配2个以上规则直接确认
)

// 信用分扣分规则（新体系：100分制，只降不升）
const (
	// 顾客扣分（基于行为回溯）
	ScoreClaimWarning     = -5  // 5单3索赔首次警告
	ScoreEvidenceRequired = -10 // 需要提交证据（已被警告过）
	ScorePlatformPay      = -15 // 平台垫付（问题用户）
	ScoreAppealLost       = -10 // 申诉失败（被证明索赔有问题）
	ScoreFraudDetected    = -30 // 团伙欺诈检测命中

	// 商户扣分
	ScoreForeignObject3Times = -15 // 一周内3次异物
	ScoreFoodSafetyIncident  = -25 // 食安事件
	ScoreTimeout3Times       = -10 // 一周内3次超时
	ScoreRefuseOrder3Times   = -10 // 频繁拒单

	// 骑手扣分
	ScoreDamage3Times   = -15 // 一周内3次餐损
	ScoreCancelDelivery = -20 // 私自取消订单
	ScoreSevereTimeout  = -10 // 严重超时（≥30分钟）
	ScoreTimeoutPerTime = -5  // 每次超时

	// 旧体系常量（兼容过渡期，后续删除）
	ScoreMaliciousClaim       = -100 // @deprecated 恶意索赔（已确认）
	ScoreFalseFoodSafety      = -100 // @deprecated 虚假食安举报
	ScoreFrequentCancel       = -10  // @deprecated 频繁取消订单
	ScoreFirstMaliciousClaim  = -30  // @deprecated 首次恶意索赔
	ScoreSecondMaliciousClaim = -40  // @deprecated 第二次
	ScoreThirdMaliciousClaim  = -50  // @deprecated 第三次
	ScoreFifthMaliciousClaim  = -200 // @deprecated 第五次
)

// 救济机制
const (
	MaxRecoveryAttempts = 1 // 最多恢复次数（第二次再犯永久封禁）
)

// 审核类型
const (
	ApprovalTypeInstant = "instant" // 秒赔（高信用+小额）
	ApprovalTypeAuto    = "auto"    // 回溯检查自动通过
	ApprovalTypeManual  = "manual"  // 人工审核
)

// 索赔状态
const (
	ClaimStatusPending      = "pending"       // 待审核
	ClaimStatusAutoApproved = "auto-approved" // 回溯检查通过
	ClaimStatusManualReview = "manual-review" // 人工审核中
	ClaimStatusApproved     = "approved"      // 已通过
	ClaimStatusRejected     = "rejected"      // 已拒绝
)

// 欺诈模式类型
const (
	FraudPatternDeviceReuse       = "device-reuse"       // 设备复用
	FraudPatternAddressCluster    = "address-cluster"    // 地址聚类
	FraudPatternCoordinatedClaims = "coordinated-claims" // 协同索赔
	FraudPatternPaymentLink       = "payment-link"       // 支付关联
	FraudPatternTimeAnomaly       = "time-anomaly"       // 时间异常
)

// 食安事件状态
const (
	FoodSafetyStatusReported          = "reported"           // 已上报
	FoodSafetyStatusInvestigating     = "investigating"      // 调查中
	FoodSafetyStatusMerchantSuspended = "merchant-suspended" // 商户已熔断
	FoodSafetyStatusResolved          = "resolved"           // 已解决
)

// 角色类型
const (
	EntityTypeCustomer = "customer"
	EntityTypeMerchant = "merchant"
	EntityTypeRider    = "rider"
)

// 索赔类型
const (
	ClaimTypeForeignObject = "foreign-object" // 异物 → 对商户 → 全额（商户退款）
	ClaimTypeFoodSafety    = "food-safety"    // 食安 → 对商户 → 人工审核
	ClaimTypeDamage        = "damage"         // 餐损 → 对骑手 → 全额（骑手押金）
	ClaimTypeTimeout       = "timeout"        // 超时 → 对骑手 → 仅运费（骑手押金）
)

// 索赔赔付来源
const (
	CompensationSourceMerchant = "merchant" // 商户（异物、食安）
	CompensationSourceRider    = "rider"    // 骑手押金（餐损、超时）
	CompensationSourcePlatform = "platform" // 平台垫付（问题用户）
)

// 用户索赔状态（行为回溯结果）
const (
	ClaimBehaviorNormal           = "normal"            // 正常：秒赔
	ClaimBehaviorWarned           = "warned"            // 已警告：下次需证据
	ClaimBehaviorEvidenceRequired = "evidence-required" // 需证据：不秒赔
	ClaimBehaviorPlatformPay      = "platform-pay"      // 平台垫付：照赔但平台承担
	ClaimBehaviorRejectService    = "reject-service"    // 拒绝服务
)

// Helper functions

// UniqueInt64 去重int64切片
func UniqueInt64(slice []int64) []int64 {
	seen := make(map[int64]bool)
	result := []int64{}
	for _, val := range slice {
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	return result
}

// ContainsInt64 检查切片是否包含元素
func ContainsInt64(slice []int64, item int64) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// MaxInt 返回较大值
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinInt 返回较小值
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ClampInt16 限制int16范围
func ClampInt16(val, min, max int16) int16 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
