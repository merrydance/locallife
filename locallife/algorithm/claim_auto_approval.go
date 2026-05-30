package algorithm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
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

// ClaimAutoApproval 索赔自动审核器
// 设计理念：
// 1. 所有索赔都赔付，不存在不赔的情况
// 2. 通过行为回溯标定用户是谁，而不是决定赔不赔
// 3. 问题用户照赔，但平台垫付（退还骑手/商户），然后拒绝服务
type ClaimAutoApproval struct {
	store                   db.Store
	lookbackChecker         *LookbackChecker
	notificationDistributor NotificationDistributor // 可选。用于把用户通知动作分发到通知中心链路。
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
	if compensationAmount > 0 && compensationAmount < ClaimPayoutMinimumAmountFen {
		return nil, ErrClaimPayoutBelowMinimum
	}
	compensationSource := CompensationSourceMerchant
	reason := "销售侧异常索赔默认由商户承担责任"

	switch claimType {
	case ClaimTypeTimeout:
		// 平台介入后，超时责任默认落骑手，赔付口径按订单可赔金额执行。
		compensationSource = CompensationSourceRider
		reason = "服务侧异常索赔默认由骑手承担责任"
	case ClaimTypeDamage:
		// 餐损责任默认落骑手。
		compensationSource = CompensationSourceRider
		reason = "服务侧异常索赔默认由骑手承担责任"
	case ClaimTypeForeignObject:
		// 异物责任默认落商户。
		compensationSource = CompensationSourceMerchant
		reason = "销售侧异常索赔默认由商户承担责任"
	}

	return &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             compensationAmount,
		Reason:             reason,
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: compensationSource,
	}, nil
}

// CheckRiderDamageHistory 是历史异步风险任务入口；当前索赔判责与后续动作由行为追溯主链处理。
func (caa *ClaimAutoApproval) CheckRiderDamageHistory(
	ctx context.Context,
	riderID int64,
) error {
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
		// 已完成自动裁定但尚未进入补偿执行的索赔，先进入等待用户确认继续状态。
		status = ClaimStatusWaitingCustomerConfirmation
		approvedAmount = &decision.Amount
	default:
		if decision.Approved {
			status = ClaimStatusWaitingCustomerConfirmation
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
		ScoreBreakdown:     decision.ScoreBreakdown,
		FactSnapshot:       decision.FactSnapshot,
		SkipActionCreation: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create claim with behavior: %w", err)
	}

	claim := result.Claim
	alignDecisionWithPersistedBehaviorDecision(decision, result.BehaviorDecision, recoveryPlan)

	return &claim, nil
}

func alignDecisionWithPersistedBehaviorDecision(decision *Decision, behaviorDecision db.BehaviorDecision, recoveryPlan *ClaimRecoveryPlan) *ClaimRecoveryPlan {
	if decision == nil || !behaviorDecision.DecisionMode.Valid {
		return recoveryPlan
	}

	switch behaviorDecision.DecisionMode.String {
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
