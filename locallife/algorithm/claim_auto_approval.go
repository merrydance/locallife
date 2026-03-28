package algorithm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

// NotificationDistributor 通知分发器接口。
// algorithm 层只负责分发正式 notify action；实际的通知落库、偏好检查和后续推送由下游实现负责。
// 当前生产实现会将通知提交到 worker 任务，由通知中心创建 notification 记录并按能力做实时推送。
type NotificationDistributor interface {
	// SendUserNotification 分发用户通知。
	// 当前主实现会创建站内通知记录；若下游支持，也可追加 WebSocket 或微信订阅消息等渠道。
	// notificationType: system/food_safety/order 等
	// relatedType: claim/order/merchant 等
	SendUserNotification(ctx context.Context, userID int64, notificationType, title, content string, relatedType string, relatedID int64) error
}

// ClaimPayoutDistributor 分发正式 payout action。
// algorithm 层只负责触发持久化 payout action 的执行；具体入队和重试由下游实现负责。
type ClaimPayoutDistributor interface {
	EnqueueClaimPayoutAction(ctx context.Context, actionID int64) error
}

// ClaimAutoApproval 索赔自动审核器
// 设计理念：
// 1. 所有索赔都赔付，不存在不赔的情况
// 2. 通过行为回溯标定用户是谁，而不是决定赔不赔
// 3. 问题用户照赔，但平台垫付（退还骑手/商户），然后拒绝服务
type ClaimAutoApproval struct {
	store                   db.Store
	lookbackChecker         *LookbackChecker
	notificationDistributor NotificationDistributor // 可选。用于把用户通知动作分发到通知中心链路。
	payoutDistributor       ClaimPayoutDistributor  // 可选。用于把 payout action 分发到异步执行链路。
	wsHub                   WebSocketHub            // 可选。仅用于商户/骑手的实时 WebSocket fallback。
}

type ClaimCompensationContext struct {
	RequestedAmount     int64
	OrderTotalAmount    int64
	DeliveryFee         int64
	DeliveryFeeDiscount int64
}

// ClaimEvidenceContext 索赔证据上下文（行为追溯）
type ClaimEvidenceContext struct {
	DeviceID          string
	DeviceFingerprint string
	DeviceType        string
	IPAddress         string
	UserAgent         string
	AddressID         *int64
}

type ClaimRecoveryPlan struct {
	ResponsibleParty string
	RecoveryTarget   string
	RecoveryAmount   int64
	DueAt            time.Time
	DecisionSnapshot []byte
}

type behaviorRestrictionActionPayload struct {
	Action            string `json:"action"`
	ClaimID           int64  `json:"claim_id"`
	UserID            int64  `json:"user_id"`
	DecisionMode      string `json:"decision_mode"`
	RestrictionReason string `json:"restriction_reason,omitempty"`
	Remark            string `json:"remark"`
}

type behaviorNotifyActionPayload struct {
	Action           string `json:"action"`
	ClaimID          int64  `json:"claim_id"`
	TargetEntity     string `json:"target_entity"`
	TargetID         int64  `json:"target_id,omitempty"`
	RecipientUserID  int64  `json:"recipient_user_id,omitempty"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	RelatedType      string `json:"related_type"`
	RelatedID        int64  `json:"related_id"`
	Remark           string `json:"remark"`
}

// WebSocketHub WebSocket通知接口
type WebSocketHub interface {
	SendToMerchant(merchantID int64, msg websocket.Message)
	SendToRider(riderID int64, msg websocket.Message)
}

// NewClaimAutoApproval 创建索赔自动审核器
func NewClaimAutoApproval(store db.Store, wsHub WebSocketHub) *ClaimAutoApproval {
	return &ClaimAutoApproval{
		store:                   store,
		lookbackChecker:         NewLookbackChecker(store),
		notificationDistributor: nil,
		wsHub:                   wsHub,
	}
}

// SetNotificationDistributor 设置通知分发器（可选）。
// 未设置时，用户侧 notify action 不会在 algorithm 层直接送达，但通知中心链路可由上层自行接入。
func (caa *ClaimAutoApproval) SetNotificationDistributor(distributor NotificationDistributor) {
	caa.notificationDistributor = distributor
}

// SetClaimPayoutDistributor 设置 payout action 分发器（可选）。
func (caa *ClaimAutoApproval) SetClaimPayoutDistributor(distributor ClaimPayoutDistributor) {
	caa.payoutDistributor = distributor
}

// EvaluateClaim 评估异常订单索赔申请。
// food safety 不属于本链路，误入时直接引导到独立 workflow。
func (caa *ClaimAutoApproval) EvaluateClaim(
	ctx context.Context,
	userID int64,
	orderID int64,
	compensation ClaimCompensationContext,
	claimType string,
) (*Decision, error) {
	// Step 1: 食安问题改走专门链路，不进入异常订单平台介入索赔。
	if claimType == ClaimTypeFoodSafety {
		return nil, errors.New("food safety claims must use the dedicated food safety workflow")
	}

	// Step 2: 基于订单价格口径计算可赔金额。
	netDeliveryFee := compensation.DeliveryFee - compensation.DeliveryFeeDiscount
	if netDeliveryFee < 0 {
		netDeliveryFee = 0
	}
	mealAmount := compensation.OrderTotalAmount - netDeliveryFee
	if mealAmount < 0 {
		mealAmount = 0
	}
	eligibleAmount := mealAmount + netDeliveryFee
	if eligibleAmount < 0 {
		eligibleAmount = 0
	}
	compensationAmount := compensation.RequestedAmount
	if compensationAmount > eligibleAmount {
		compensationAmount = eligibleAmount
	}
	compensationSource := CompensationSourceMerchant

	switch claimType {
	case ClaimTypeTimeout:
		// 平台介入后，超时责任默认落骑手，赔付口径按订单可赔金额执行。
		compensationSource = CompensationSourceRider
	case ClaimTypeDamage:
		// 餐损责任默认落骑手。
		compensationSource = CompensationSourceRider
	case ClaimTypeForeignObject:
		// 异物责任默认落商户。
		compensationSource = CompensationSourceMerchant
	}

	// Step 3: 检查用户行为（近3个月外卖订单和索赔比例）
	behaviorResult, err := caa.CheckUserClaimBehavior(ctx, userID)
	if err != nil {
		// 查询失败，降级为正常秒赔
		return &Decision{
			Type:               ApprovalTypeInstant,
			Approved:           true,
			Amount:             compensationAmount,
			Reason:             "行为检查失败，降级秒赔",
			BehaviorStatus:     ClaimBehaviorNormal,
			CompensationSource: compensationSource,
		}, nil
	}

	// Step 4: 根据行为状态决定处理方式
	decision := &Decision{
		Approved:           true, // 新设计：永远赔付
		Amount:             compensationAmount,
		BehaviorStatus:     behaviorResult.Status,
		CompensationSource: compensationSource,
	}

	switch behaviorResult.Status {
	case ClaimBehaviorNormal:
		// 正常用户：秒赔
		decision.Type = ApprovalTypeInstant
		decision.Reason = "正常用户秒赔"

	case ClaimBehaviorWarned:
		decision.Type = ApprovalTypeInstant
		if behaviorResult.ShouldWarn {
			// 首次触发警告（5单3索赔）：秒赔 + 警告
			decision.Reason = "首次警告，本次秒赔"
			decision.Warning = fmt.Sprintf(
				"您近3个月%d笔外卖订单中已索赔%d次，后续索赔将进入平台行为回溯审计",
				behaviorResult.TakeoutOrders, behaviorResult.ClaimCount+1)
			decision.ShouldWarn = true
			// 记录警告
			caa.recordUserWarning(ctx, userID, decision.Warning)
		} else {
			// 已被警告：仍秒赔，但记录提示
			decision.Reason = "已触发警告，仍秒赔"
			decision.Warning = "您的索赔行为已触发警告，后续索赔将进入平台行为回溯审计"
		}

	case ClaimBehaviorPlatformFallback:
		// 用户风险升高但未到限制服务阈值：本次由平台正式兜底
		decision.Type = DecisionModePlatformFallback
		decision.Reason = "用户风险较高，本次由平台兜底处理"
		decision.CompensationSource = CompensationSourcePlatform
		decision.Warning = fmt.Sprintf(
			"您的索赔行为异常（近3个月%d单索赔%d次），本次已由平台兜底处理。如继续异常行为，账号将被限制服务。",
			behaviorResult.TakeoutOrders, behaviorResult.ClaimCount+1)

	case ClaimBehaviorUserRestricted:
		// 确认高风险用户：本次进入 user_restricted 正式模式
		decision.Type = DecisionModeUserRestricted
		decision.Reason = "用户风险已确认，本次限制服务并由平台兜底"
		decision.CompensationSource = CompensationSourcePlatform
		decision.Warning = "您的账号因索赔行为异常已被限制服务，本次索赔由平台垫付。"
	}

	return decision, nil
}

// CheckUserClaimBehavior 检查用户索赔行为
// 核心逻辑：近3个月5单3索赔触发警告
func (caa *ClaimAutoApproval) CheckUserClaimBehavior(ctx context.Context, userID int64) (*ClaimBehaviorResult, error) {
	// 查询用户行为统计
	stats, err := caa.store.GetUserBehaviorStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &ClaimBehaviorResult{
		RecentMonths:  3,
		TakeoutOrders: int(stats.TakeoutOrders90d),
		ClaimCount:    int(stats.Claims90d),
		WarningCount:  int(stats.WarningCount),
	}

	// 计算索赔比例
	if result.TakeoutOrders > 0 {
		result.ClaimRatio = float64(result.ClaimCount) / float64(result.TakeoutOrders)
	}

	// 判定状态
	// 1. 平台垫付次数>=2次 → 拒绝服务
	if stats.PlatformPayCount >= 2 {
		result.Status = ClaimBehaviorUserRestricted
		result.RejectService = true
		result.Message = "多次平台兜底，限制服务"
		return result, nil
	}

	// 2. 已有平台垫付记录：持续平台垫付
	if stats.PlatformPayCount > 0 {
		result.Status = ClaimBehaviorPlatformFallback
		result.Message = "问题用户，平台兜底"
		return result, nil
	}

	// 3. 已有警告/标记：进入平台行为回溯审计
	if stats.WarningCount > 0 {
		result.Status = ClaimBehaviorWarned
		result.ShouldWarn = false
		result.Message = "已被警告，进入平台行为回溯审计"
		return result, nil
	}

	// 4. 检查是否触发首次警告（5单3索赔）
	// 注意：ClaimCount是历史索赔数，当前这次还没算进去，所以用+1来判断
	if result.TakeoutOrders <= ClaimWarningOrderCount && result.ClaimCount+1 >= ClaimWarningClaimCount {
		result.Status = ClaimBehaviorWarned
		result.ShouldWarn = true
		result.Message = fmt.Sprintf(
			"近3个月%d笔外卖订单已索赔%d次，触发警告",
			result.TakeoutOrders, result.ClaimCount+1)
		return result, nil
	}

	// 5. 订单较多但索赔比例异常高（>60%且>=3次）
	if result.ClaimRatio >= ClaimWarningRatio && result.ClaimCount+1 >= ClaimWarningClaimCount {
		result.Status = ClaimBehaviorWarned
		result.ShouldWarn = true
		result.Message = fmt.Sprintf(
			"索赔比例%.0f%%异常高，触发警告",
			result.ClaimRatio*100)
		return result, nil
	}

	// 6. 正常用户
	result.Status = ClaimBehaviorNormal
	result.Message = "正常用户"
	return result, nil
}

// recordUserWarning 记录用户警告
func (caa *ClaimAutoApproval) recordUserWarning(ctx context.Context, userID int64, reason string) {
	// 先检查是否已有警告记录
	_, err := caa.store.GetUserClaimWarningStatus(ctx, userID)
	if err != nil {
		// 不存在，创建新记录
		_, _ = caa.store.CreateUserClaimWarning(ctx, db.CreateUserClaimWarningParams{
			UserID:            userID,
			LastWarningReason: pgtype.Text{String: reason, Valid: true},
			RequiresEvidence:  false,
		})
	} else {
		// 已存在，增加警告次数
		_ = caa.store.IncrementUserClaimWarning(ctx, db.IncrementUserClaimWarningParams{
			UserID:            userID,
			LastWarningReason: pgtype.Text{String: reason, Valid: true},
			RequiresEvidence:  false,
		})
	}
}

// recordPlatformPay 记录平台垫付
func (caa *ClaimAutoApproval) recordPlatformPay(ctx context.Context, userID int64, reason string) {
	_ = caa.store.IncrementUserPlatformPayCount(ctx, db.IncrementUserPlatformPayCountParams{
		UserID:            userID,
		LastWarningReason: pgtype.Text{String: reason, Valid: true},
	})
}

// applyUserRestrictionBlock writes the formal restriction record without dispatching notifications.
func (caa *ClaimAutoApproval) applyUserRestrictionBlock(ctx context.Context, userID int64) {
	// 外卖拒绝服务：写入行为追溯黑名单
	_, err := caa.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	})
	if err == nil {
		return
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		log.Error().Err(err).Int64("user_id", userID).Msg("load active behavior blocklist failed during persisted restriction execution")
		return
	}

	blockDays := int64(14)
	if days := caa.getRejectServiceCooldownDays(ctx); days > 0 {
		blockDays = days
	}
	blockUntil := time.Now().AddDate(0, 0, int(blockDays))

	if _, err := caa.store.CreateBehaviorBlocklist(ctx, db.CreateBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
		ReasonCode: "malicious-claims",
		BlockUntil: pgtype.Timestamptz{Time: blockUntil, Valid: true},
		Status:     "active",
	}); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("create behavior blocklist failed during persisted restriction execution")
	}
}

func (caa *ClaimAutoApproval) getRejectServiceCooldownDays(ctx context.Context) int64 {
	config, err := caa.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "behavior_trace.reject_service_cooldown_days",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		return 0
	}

	var payload struct {
		Days int64 `json:"days"`
	}
	if jsonErr := json.Unmarshal(config.ConfigValue, &payload); jsonErr == nil && payload.Days > 0 {
		return payload.Days
	}

	var direct int64
	if jsonErr := json.Unmarshal(config.ConfigValue, &direct); jsonErr == nil && direct > 0 {
		return direct
	}

	return 0
}

// CheckRiderDamageHistory 检查骑手餐损历史（异步）
// 触发条件：骑手被索赔餐损时异步调用
// 功能：检查7天内餐损次数，达到阈值则记录并通知
// 注意：追偿单与结算调整在索赔链路执行，此处只处理风险记录
func (caa *ClaimAutoApproval) CheckRiderDamageHistory(
	ctx context.Context,
	riderID int64,
) error {
	// 查询骑手最近7天的餐损索赔
	startTime := time.Now().Add(-Recent7Days)
	claims, err := caa.store.ListRiderClaims(ctx, db.ListRiderClaimsParams{
		RiderID:   pgtype.Int8{Int64: riderID, Valid: true},
		CreatedAt: startTime,
	})
	if err != nil {
		return err
	}

	// 过滤餐损索赔
	damageCount := 0
	for _, claim := range claims {
		if claim.ClaimType == ClaimTypeDamage {
			damageCount++
		}
	}

	// 达到3次：记录并警告
	if damageCount >= DamageIncidentsIn7Days {
		// 1. 更新骑手profile统计
		err = caa.store.IncrementRiderDamageIncident(ctx, riderID)
		if err != nil {
			return err
		}

		// 2. 餐损高发：暂停接单
		reason := fmt.Sprintf("damage claims high: %d in 7 days", damageCount)
		_ = caa.store.SuspendRider(ctx, db.SuspendRiderParams{
			RiderID:       riderID,
			SuspendReason: pgtype.Text{String: reason, Valid: true},
			SuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		})

		// 3. 发送通知给骑手（风险警告）
		go caa.sendNotification("rider", "餐损索赔警告",
			fmt.Sprintf("您近7天内发生%d次餐损索赔，请注意配送安全", damageCount), riderID)

		// 注意：追偿单与结算调整在索赔链路执行，此处不重复处理
	}

	return nil
}

// CreateClaimWithDecision 根据决策创建索赔记录
func (caa *ClaimAutoApproval) CreateClaimWithDecision(
	ctx context.Context,
	orderID int64,
	userID int64,
	claimType string,
	description string,
	claimAmount int64,
	decision *Decision,
) (*db.Claim, error) {
	return caa.CreateClaimWithDecisionAndEvidence(ctx, orderID, userID, claimType, description, claimAmount, decision, nil, nil)
}

// CreateClaimWithDecisionAndEvidence 根据决策创建索赔记录（含行为追溯证据）
func (caa *ClaimAutoApproval) CreateClaimWithDecisionAndEvidence(
	ctx context.Context,
	orderID int64,
	userID int64,
	claimType string,
	description string,
	claimAmount int64,
	decision *Decision,
	evidenceContext *ClaimEvidenceContext,
	recoveryPlan *ClaimRecoveryPlan,
) (*db.Claim, error) {
	// 序列化回溯结果
	var lookbackJSON []byte
	if decision.LookbackData != nil {
		var err error
		lookbackJSON, err = json.Marshal(decision.LookbackData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal lookback result: %w", err)
		}
	}

	// 确定状态（新设计）
	status := ClaimStatusPending
	var approvedAmount *int64

	switch DecisionApprovalType(decision.Type) {
	case ApprovalTypeInstant, ApprovalTypeAuto:
		// 秒赔、自动通过、平台垫付都是自动批准
		status = ClaimStatusAutoApproved
		approvedAmount = &decision.Amount
	default:
		if decision.Approved {
			status = ClaimStatusAutoApproved
			approvedAmount = &decision.Amount
		}
	}

	reasonCodes := []string{decision.Type}
	if decision.BehaviorStatus != "" && decision.BehaviorStatus != decision.Type {
		reasonCodes = append(reasonCodes, decision.BehaviorStatus)
	}

	var evidenceArg ClaimEvidenceContext
	if evidenceContext != nil {
		evidenceArg = *evidenceContext
	}

	responsibleParty := "unknown"
	if decision.CompensationSource != "" {
		switch decision.CompensationSource {
		case CompensationSourceMerchant:
			responsibleParty = "merchant"
		case CompensationSourceRider:
			responsibleParty = "rider"
		case CompensationSourcePlatform:
			responsibleParty = "platform_fallback"
		}
	}
	if decision.BehaviorStatus == ClaimBehaviorUserRestricted || decision.Type == DecisionModeUserRestricted {
		responsibleParty = "user"
	}

	recoveryTarget := ""
	recoveryAmount := int64(0)
	var recoveryDueAt *time.Time
	var decisionSnapshot []byte
	if recoveryPlan != nil {
		recoveryTarget = recoveryPlan.RecoveryTarget
		recoveryAmount = recoveryPlan.RecoveryAmount
		recoveryDueAt = &recoveryPlan.DueAt
		decisionSnapshot = recoveryPlan.DecisionSnapshot
	}

	result, err := caa.store.CreateClaimWithBehaviorTx(ctx, db.CreateClaimWithBehaviorTxParams{
		OrderID:            orderID,
		UserID:             userID,
		ClaimType:          claimType,
		Description:        description,
		ClaimAmount:        claimAmount,
		Status:             status,
		ApprovalType:       DecisionApprovalType(decision.Type),
		ApprovedAmount:     approvedAmount,
		AutoApprovalReason: decision.Reason,
		LookbackResult:     lookbackJSON,
		DecisionVersion:    "v1",
		ReasonCodes:        reasonCodes,
		ResponsibleParty:   responsibleParty,
		CompensationSource: decision.CompensationSource,
		TraceSummary:       decision.Reason,
		DeviceID:           evidenceArg.DeviceID,
		DeviceFingerprint:  evidenceArg.DeviceFingerprint,
		DeviceType:         evidenceArg.DeviceType,
		IPAddress:          evidenceArg.IPAddress,
		UserAgent:          evidenceArg.UserAgent,
		AddressID:          evidenceArg.AddressID,
		CreateRecovery:     recoveryPlan != nil,
		RecoveryTarget:     recoveryTarget,
		RecoveryAmount:     recoveryAmount,
		RecoveryDueAt:      recoveryDueAt,
		DecisionSnapshot:   decisionSnapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create claim with behavior: %w", err)
	}

	claim := result.Claim
	// 这里开始只消费事务返回的正式结果。
	// claim 主链的 payout/restriction/notification/recovery 语义都应来自 persisted decision/action，
	// algorithm 不再回查 behavior tables 重新拼装副作用。
	alignDecisionWithPersistedBehaviorDecision(decision, result.BehaviorDecision, recoveryPlan)
	caa.applyPersistedDecisionSideEffects(ctx, userID, result.BehaviorDecision, decision)
	caa.executePersistedBehaviorActions(ctx, result)

	// 餐损赔付后的事后处理
	if claimType == ClaimTypeDamage && decision.NeedsReview {
		// 发现可疑模式，异步进行信用分处理
		// 使用Worker任务异步执行（如果未配置worker则降级为goroutine）
		// 接入方法：在API层通过caa.SetTaskDistributor(server.taskDistributor)设置
		// 然后通过worker.NewHandleSuspiciousPatternTask分发任务
		// 当前降级：直接用goroutine（生产环境建议使用Worker）
		go caa.handleSuspiciousPattern(ctx, userID, claim.ID, claimType, decision.LookbackData)
	}

	return &claim, nil
}

func (caa *ClaimAutoApproval) applyPersistedDecisionSideEffects(ctx context.Context, userID int64, behaviorDecision db.BehaviorDecision, decision *Decision) {
	if !behaviorDecision.DecisionMode.Valid || decision == nil {
		return
	}

	switch behaviorDecision.DecisionMode.String {
	case db.BehaviorDecisionModePlatformFallback:
		reason := decision.Warning
		if reason == "" {
			reason = decision.Reason
		}
		caa.recordPlatformPay(ctx, userID, reason)
	}
}

func (caa *ClaimAutoApproval) executePersistedBehaviorActions(ctx context.Context, result db.CreateClaimWithBehaviorTxResult) {
	if result.PayoutAction != nil {
		caa.executePayoutAction(ctx, *result.PayoutAction)
	}
	if result.RestrictionAction != nil {
		caa.executeRestrictionAction(ctx, *result.RestrictionAction)
	}
	if result.NotificationAction != nil {
		caa.executeNotificationAction(ctx, *result.NotificationAction)
	}
}

func (caa *ClaimAutoApproval) executePayoutAction(ctx context.Context, action db.BehaviorAction) {
	if action.ID == 0 {
		log.Warn().Msg("skip persisted payout action without action id")
		return
	}
	if caa.payoutDistributor == nil {
		log.Error().Int64("behavior_action_id", action.ID).Msg("claim payout distributor unavailable during persisted payout execution")
		return
	}
	if err := caa.payoutDistributor.EnqueueClaimPayoutAction(ctx, action.ID); err != nil {
		log.Error().Err(err).Int64("behavior_action_id", action.ID).Msg("dispatch persisted payout action failed")
	}
}

func (caa *ClaimAutoApproval) executeRestrictionAction(ctx context.Context, action db.BehaviorAction) {
	var detail behaviorRestrictionActionPayload
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		log.Error().Err(err).Int64("behavior_action_id", action.ID).Str("action_type", action.ActionType).Msg("decode persisted restriction action detail failed")
		return
	}
	if detail.UserID == 0 {
		log.Warn().Int64("behavior_action_id", action.ID).Msg("skip persisted restriction action without user id")
		return
	}
	caa.applyUserRestrictionBlock(ctx, detail.UserID)
}

func (caa *ClaimAutoApproval) executeNotificationAction(ctx context.Context, action db.BehaviorAction) {
	var detail behaviorNotifyActionPayload
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		log.Error().Err(err).Int64("behavior_action_id", action.ID).Str("action_type", action.ActionType).Msg("decode persisted notification action detail failed")
		return
	}

	if caa.notificationDistributor != nil && detail.RecipientUserID > 0 {
		notificationType := detail.NotificationType
		if notificationType == "" {
			notificationType = "system"
		}
		relatedType := detail.RelatedType
		if relatedType == "" {
			relatedType = "claim"
		}
		if err := caa.notificationDistributor.SendUserNotification(
			ctx,
			detail.RecipientUserID,
			notificationType,
			detail.Title,
			detail.Content,
			relatedType,
			detail.RelatedID,
		); err != nil {
			log.Error().Err(err).
				Int64("behavior_action_id", action.ID).
				Int64("recipient_user_id", detail.RecipientUserID).
				Str("target_entity", detail.TargetEntity).
				Msg("dispatch persisted notification action failed")
		}
	}

	if detail.TargetID > 0 && (detail.TargetEntity == "merchant" || detail.TargetEntity == "rider") {
		caa.sendNotification(detail.TargetEntity, detail.Title, detail.Content, detail.TargetID)
	}
}

func alignDecisionWithPersistedBehaviorDecision(decision *Decision, behaviorDecision db.BehaviorDecision, recoveryPlan *ClaimRecoveryPlan) *ClaimRecoveryPlan {
	if decision == nil || !behaviorDecision.DecisionMode.Valid {
		return recoveryPlan
	}

	switch behaviorDecision.DecisionMode.String {
	case db.BehaviorDecisionModePlatformFallback:
		decision.Type = DecisionModePlatformFallback
		decision.Approved = true
		decision.CompensationSource = CompensationSourcePlatform
		if reason := persistedBehaviorDecisionReason(behaviorDecision); reason != "" {
			decision.Reason = reason
			decision.Warning = reason
		}
		return nil
	case db.BehaviorDecisionModeUserRestricted:
		decision.Type = DecisionModeUserRestricted
		decision.Approved = true
		decision.BehaviorStatus = ClaimBehaviorUserRestricted
		decision.CompensationSource = CompensationSourcePlatform
		if reason := persistedBehaviorDecisionReason(behaviorDecision); reason != "" {
			decision.Reason = reason
			decision.Warning = reason
		}
		return nil
	case db.BehaviorDecisionModeMerchantRecovery:
		decision.Type = DecisionModeMerchantRecovery
		decision.CompensationSource = CompensationSourceMerchant
	case db.BehaviorDecisionModeRiderRecovery:
		decision.Type = DecisionModeRiderRecovery
		decision.CompensationSource = CompensationSourceRider
	}

	return recoveryPlan
}

func persistedBehaviorDecisionReason(behaviorDecision db.BehaviorDecision) string {
	if behaviorDecision.TraceSummary.Valid && behaviorDecision.TraceSummary.String != "" {
		return behaviorDecision.TraceSummary.String
	}
	if behaviorDecision.FallbackReason.Valid && behaviorDecision.FallbackReason.String != "" {
		return behaviorDecision.FallbackReason.String
	}
	if behaviorDecision.RestrictionReason.Valid && behaviorDecision.RestrictionReason.String != "" {
		return behaviorDecision.RestrictionReason.String
	}
	return ""
}

// handleSuspiciousPattern 处理可疑的餐损索赔模式
// 已经赔付，但根据信用分和模式进行事后处罚
func (caa *ClaimAutoApproval) handleSuspiciousPattern(
	ctx context.Context,
	userID int64,
	claimID int64,
	claimType string,
	lookback *LookbackResult,
) {
	_ = claimType
	// 根据索赔频率和模式发出警告
	var reason string
	var warningMessage string

	if lookback != nil && lookback.ClaimsFound >= 5 {
		// 高频索赔（5次以上）
		reason = "高频索赔处罚"
		warningMessage = fmt.Sprintf("您最近%s内已索赔%d次，被系统判定为恶意索赔风险",
			lookback.Period, lookback.ClaimsFound)
	} else if lookback != nil && lookback.ClaimsFound >= 3 {
		// 频繁索赔（3次以上）
		reason = "频繁索赔警告"
		warningMessage = fmt.Sprintf("您最近%s内5笔订单中已索赔%d次，系统判定有恶意索赔风险。继续索赔将影响您的账号使用。",
			lookback.Period, lookback.ClaimsFound)
	} else {
		// 可疑但次数不多，轻微扣分
		reason = "可疑模式提醒"
		warningMessage = "系统检测到可疑索赔模式，请注意"
	}

	// 发送用户通知。
	// 当前阶段只要求把通知写入通知中心；用户在小程序内可通过通知列表和未读计数接口获取。
	// 普通用户不要求在本链路上接 WebSocket，也不要求额外补微信订阅消息离线触达。
	if caa.notificationDistributor != nil {
		_ = caa.notificationDistributor.SendUserNotification(
			ctx,
			userID,
			"system",       // notificationType
			"索赔风险提醒",       // title
			warningMessage, // content
			"claim",        // relatedType
			claimID,        // relatedID
		)
	}

	_ = reason
}

// sendNotification 发送商户/骑手 WebSocket 实时通知。
// 该 helper 不承担普通用户通知送达；普通用户当前通过通知中心在小程序内拉取通知即可。
func (caa *ClaimAutoApproval) sendNotification(entityType, title, message string, entityID int64) {
	if isNilWebSocketHub(caa.wsHub) {
		return // 静默失败，不阻塞业务
	}

	notificationData := map[string]interface{}{
		"title":     title,
		"message":   message,
		"timestamp": time.Now().Unix(),
	}

	dataBytes, err := json.Marshal(notificationData)
	if err != nil {
		return // 序列化失败，静默跳过
	}

	msg := websocket.Message{
		Type:      "behavior_alert",
		Data:      dataBytes,
		Timestamp: time.Now(),
	}

	switch entityType {
	case "merchant":
		caa.wsHub.SendToMerchant(entityID, msg)
	case "rider":
		caa.wsHub.SendToRider(entityID, msg)
	}
}

func isNilWebSocketHub(hub WebSocketHub) bool {
	if hub == nil {
		return true
	}
	value := reflect.ValueOf(hub)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
