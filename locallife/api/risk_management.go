package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

type behaviorWindowConfig struct {
	Window7d  int `json:"window_7d"`
	Window30d int `json:"window_30d"`
}

type behaviorThresholdConfig struct {
	UserClaimRate7d      float64 `json:"user_claim_rate_7d"`
	UserClaimRate30d     float64 `json:"user_claim_rate_30d"`
	UserClaims7d         int32   `json:"user_claims_7d"`
	UserClaims30d        int32   `json:"user_claims_30d"`
	MerchantAbnormalRate float64 `json:"merchant_abnormal_rate_30d"`
	RiderAbnormalRate    float64 `json:"rider_abnormal_rate_30d"`
}

func getBehaviorWindowDays(ctx *gin.Context, store db.Store) (int, int) {
	window7d := 7
	window30d := 30

	config, err := store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "behavior_trace.window_days",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		return window7d, window30d
	}
	if len(config.ConfigValue) == 0 {
		return window7d, window30d
	}
	var payload behaviorWindowConfig
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return window7d, window30d
	}
	if payload.Window7d > 0 {
		window7d = payload.Window7d
	}
	if payload.Window30d > 0 {
		window30d = payload.Window30d
	}
	return window7d, window30d
}

func getBehaviorThresholds(ctx *gin.Context, store db.Store) behaviorThresholdConfig {
	thresholds := behaviorThresholdConfig{
		UserClaimRate7d:      0.3,
		UserClaimRate30d:     0.2,
		UserClaims7d:         3,
		UserClaims30d:        5,
		MerchantAbnormalRate: 0.08,
		RiderAbnormalRate:    0.06,
	}

	config, err := store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "behavior_trace.abnormal_thresholds",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil || len(config.ConfigValue) == 0 {
		return thresholds
	}

	var payload behaviorThresholdConfig
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return thresholds
	}

	if payload.UserClaimRate7d > 0 {
		thresholds.UserClaimRate7d = payload.UserClaimRate7d
	}
	if payload.UserClaimRate30d > 0 {
		thresholds.UserClaimRate30d = payload.UserClaimRate30d
	}
	if payload.UserClaims7d > 0 {
		thresholds.UserClaims7d = payload.UserClaims7d
	}
	if payload.UserClaims30d > 0 {
		thresholds.UserClaims30d = payload.UserClaims30d
	}
	if payload.MerchantAbnormalRate > 0 {
		thresholds.MerchantAbnormalRate = payload.MerchantAbnormalRate
	}
	if payload.RiderAbnormalRate > 0 {
		thresholds.RiderAbnormalRate = payload.RiderAbnormalRate
	}
	return thresholds
}

func normalizeClaimRuleAction(action string) string {
	return strings.ReplaceAll(strings.ToLower(action), "_", "-")
}

func applyClaimRuleDecisionOverride(decision *algorithm.Decision, ruleDecision rules.Decision) string {
	if decision == nil {
		return ""
	}

	// 规则引擎在 API 层只允许覆盖正式 decision mode 与对外文案。
	// 真正的正式副作用仍以下游事务返回的 persisted behavior decision/action 为准，
	// 这里不要重新派生 payout/recovery/block/notify 等执行动作。
	normalizedAction := normalizeClaimRuleAction(ruleDecision.Action)
	switch normalizedAction {
	case "platform-fallback":
		decision.Type = algorithm.DecisionModePlatformFallback
		decision.Approved = true
		decision.CompensationSource = algorithm.CompensationSourcePlatform
		decision.NeedsReview = false
	case "merchant-recovery":
		decision.Type = algorithm.DecisionModeMerchantRecovery
		decision.Approved = true
		decision.CompensationSource = algorithm.CompensationSourceMerchant
	case "rider-recovery":
		decision.Type = algorithm.DecisionModeRiderRecovery
		decision.Approved = true
		decision.CompensationSource = algorithm.CompensationSourceRider
	case "user-restricted":
		decision.Type = algorithm.DecisionModeUserRestricted
		decision.Approved = true
		decision.BehaviorStatus = algorithm.ClaimBehaviorUserRestricted
		decision.CompensationSource = algorithm.CompensationSourcePlatform
	case "instant", "auto":
		decision.Type = normalizedAction
	}

	if ruleDecision.Reason != "" && normalizedAction != "" {
		decision.Reason = ruleDecision.Reason
		if normalizedAction == "platform-fallback" || normalizedAction == "user-restricted" {
			decision.Warning = ruleDecision.Reason
		}
	}

	return normalizedAction
}

// SubmitClaimRequest 提交索赔请求
type SubmitClaimRequest struct {
	OrderID           int64  `json:"order_id" binding:"required,min=1"`
	ClaimType         string `json:"claim_type" binding:"required,oneof=foreign-object damage timeout"`
	ClaimAmount       int64  `json:"claim_amount" binding:"required,min=1,max=100000000"` // 最高100万分(1万元)
	ClaimReason       string `json:"claim_reason" binding:"required,min=5,max=1000"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty" binding:"omitempty,max=256"`
}

// SubmitClaimResponse 索赔响应
type SubmitClaimResponse struct {
	ClaimID            int64   `json:"claim_id"`
	Status             string  `json:"status"`                    // accepted
	DecisionStatus     string  `json:"decision_status,omitempty"` // auto-adjudicated
	PayoutStatus       string  `json:"payout_status,omitempty"`   // processing, paid
	ApprovedAmount     *int64  `json:"approved_amount,omitempty"`
	CompensationSource string  `json:"compensation_source,omitempty"` // merchant, rider, platform
	Reason             string  `json:"reason"`
	PayoutETA          *string `json:"payout_eta,omitempty"` // 预计赔付时间
	Warning            *string `json:"warning,omitempty"`    // 警告信息
}

const (
	submitClaimStatusAccepted                = "accepted"
	submitClaimStatusRejected                = "rejected"
	submitClaimDecisionStatusAutoAdjudicated = "auto-adjudicated"
	submitClaimDecisionStatusRejected        = "rejected"
	submitClaimPayoutStatusProcessing        = "processing"
	submitClaimPayoutStatusPaid              = "paid"
)

func submitClaimPayoutETA(decisionType string) *string {
	eta := "1-3个工作日"
	if decisionType == algorithm.ApprovalTypeInstant {
		eta = "即时到账"
	}
	return &eta
}

func claimApprovalTypeValue(claim db.Claim) string {
	if claim.ApprovalType.Valid {
		return claim.ApprovalType.String
	}
	return ""
}

func userClaimReason(claim db.Claim) string {
	if claim.Status == "rejected" {
		if claim.RejectionReason.Valid && claim.RejectionReason.String != "" {
			return claim.RejectionReason.String
		}
		if claim.ReviewNotes.Valid && claim.ReviewNotes.String != "" {
			return claim.ReviewNotes.String
		}
	}
	if claim.DecisionReason.Valid && claim.DecisionReason.String != "" {
		return claim.DecisionReason.String
	}
	if claim.AutoApprovalReason.Valid && claim.AutoApprovalReason.String != "" {
		return claim.AutoApprovalReason.String
	}
	if claim.ReviewNotes.Valid && claim.ReviewNotes.String != "" {
		return claim.ReviewNotes.String
	}
	return ""
}

func userClaimProcessedAt(claim db.Claim) *time.Time {
	if claim.PaidAt.Valid {
		return &claim.PaidAt.Time
	}
	if claim.ReviewedAt.Valid {
		return &claim.ReviewedAt.Time
	}
	return nil
}

func userClaimLifecycleFromClaim(claim db.Claim) (string, string, string, *string) {
	if claim.Status == "rejected" {
		return submitClaimStatusRejected, submitClaimDecisionStatusRejected, "", nil
	}

	status := submitClaimStatusAccepted
	decisionStatus := ""
	payoutStatus := ""
	var payoutETA *string

	if claim.ApprovedAmount.Valid && claim.ApprovedAmount.Int64 > 0 {
		decisionStatus = submitClaimDecisionStatusAutoAdjudicated
		if claim.PaidAt.Valid {
			payoutStatus = submitClaimPayoutStatusPaid
		} else {
			payoutStatus = submitClaimPayoutStatusProcessing
			payoutETA = submitClaimPayoutETA(claimApprovalTypeValue(claim))
		}
	}

	return status, decisionStatus, payoutStatus, payoutETA
}

// SubmitClaim 提交索赔
// @Summary 提交索赔
// @Description 用户为已完成的订单提交索赔申请。系统基于行为追溯规则进行评估，决定秒赔或平台垫付。
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param request body SubmitClaimRequest true "索赔信息"
// @Success 200 {object} SubmitClaimResponse "索赔提交成功"
// @Failure 400 {object} ErrorResponse "参数错误或订单状态不允许索赔"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 409 {object} ErrorResponse "该订单已有索赔记录"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims [post]
// @Security BearerAuth
func (server *Server) SubmitClaim(ctx *gin.Context) {
	var req SubmitClaimRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 1. 验证订单存在
	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOrderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get order: %w", err)))
		return
	}

	// 2. 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrOrderNotOwned))
		return
	}

	// 3. 验证订单已完成（只有完成的订单才能索赔）
	if order.Status != OrderStatusCompleted {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrOrderNotEligibleForClaim))
		return
	}

	// 3.1 行为黑名单拦截（拒绝服务用户）
	if _, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   authPayload.UserID,
	}); err == nil {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrAccountBehaviorRestricted))
		return
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 4. 检查是否已存在该订单的索赔（幂等性检查）
	existingClaims, err := server.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
		UserID:    authPayload.UserID,
		CreatedAt: order.CreatedAt, // 从订单创建时间开始查
	})
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list user claims in period: %w", err)))
		return
	}
	for _, c := range existingClaims {
		if c.OrderID == req.OrderID {
			ctx.JSON(http.StatusConflict, errorResponse(ErrOrderAlreadyHasClaim))
			return
		}
	}

	// 5. 索赔金额不能超过订单总额
	if req.ClaimAmount > order.TotalAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrClaimAmountExceedsOrder))
		return
	}

	// 6.1 规则引擎判定（索赔/异常规则）
	ruleDecision := rules.Decision{Action: "allow"}
	if server.rulesEngine != nil && server.config.RulesEngineEnabled {
		regionID := int64(0)
		if merchant, err := server.store.GetMerchant(ctx, order.MerchantID); err == nil {
			regionID = merchant.RegionID
		}
		metadata := map[string]interface{}{
			"claim_type":          req.ClaimType,
			"claim_amount":        req.ClaimAmount,
			"order_amount":        order.TotalAmount,
			"claim_reason_length": len(req.ClaimReason),
			"device_fingerprint":  req.DeviceFingerprint,
		}

		window7d, window30d := getBehaviorWindowDays(ctx, server.store)
		thresholds := getBehaviorThresholds(ctx, server.store)
		windowEnd := time.Now()
		windowStart7d := windowEnd.AddDate(0, 0, -window7d)
		windowStart30d := windowEnd.AddDate(0, 0, -window30d)

		start7d := pgtype.Date{Time: windowStart7d, Valid: true}
		endDate := pgtype.Date{Time: windowEnd, Valid: true}
		start30d := pgtype.Date{Time: windowStart30d, Valid: true}

		if summary7d, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
			EntityType: "user",
			EntityID:   authPayload.UserID,
			StatDate:   start7d,
			StatDate_2: endDate,
		}); err == nil {
			metadata["takeout_orders_7d"] = summary7d.TotalOrders
			metadata["claims_7d"] = summary7d.AbnormalClaims
			if summary7d.TotalOrders > 0 {
				metadata["claim_rate_7d"] = float64(summary7d.AbnormalClaims) / float64(summary7d.TotalOrders)
			}
			metadata["user_claims_7d_exceeded"] = summary7d.AbnormalClaims >= thresholds.UserClaims7d
			metadata["user_claim_rate_7d_exceeded"] = summary7d.TotalOrders > 0 && float64(summary7d.AbnormalClaims)/float64(summary7d.TotalOrders) >= thresholds.UserClaimRate7d
		}

		if summary30d, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
			EntityType: "user",
			EntityID:   authPayload.UserID,
			StatDate:   start30d,
			StatDate_2: endDate,
		}); err == nil {
			metadata["takeout_orders_30d"] = summary30d.TotalOrders
			metadata["claims_30d"] = summary30d.AbnormalClaims
			if summary30d.TotalOrders > 0 {
				metadata["claim_rate_30d"] = float64(summary30d.AbnormalClaims) / float64(summary30d.TotalOrders)
			}
			metadata["user_claims_30d_exceeded"] = summary30d.AbnormalClaims >= thresholds.UserClaims30d
			metadata["user_claim_rate_30d_exceeded"] = summary30d.TotalOrders > 0 && float64(summary30d.AbnormalClaims)/float64(summary30d.TotalOrders) >= thresholds.UserClaimRate30d
		}

		startDateParam := pgtype.Date{Time: windowStart30d, Valid: true}
		endDateParam := endDate

		if summary, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
			EntityType: "merchant",
			EntityID:   order.MerchantID,
			StatDate:   startDateParam,
			StatDate_2: endDateParam,
		}); err == nil {
			metadata["merchant_total_orders_30d"] = summary.TotalOrders
			metadata["merchant_abnormal_claims_30d"] = summary.AbnormalClaims
			if summary.TotalOrders > 0 {
				metadata["merchant_abnormal_rate_30d"] = float64(summary.AbnormalClaims) / float64(summary.TotalOrders)
			}
			metadata["merchant_abnormal_rate_30d_exceeded"] = summary.TotalOrders > 0 && float64(summary.AbnormalClaims)/float64(summary.TotalOrders) >= thresholds.MerchantAbnormalRate
		}
		if summary7d, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
			EntityType: "merchant",
			EntityID:   order.MerchantID,
			StatDate:   start7d,
			StatDate_2: endDate,
		}); err == nil {
			metadata["merchant_total_orders_7d"] = summary7d.TotalOrders
			metadata["merchant_abnormal_claims_7d"] = summary7d.AbnormalClaims
			if summary7d.TotalOrders > 0 {
				metadata["merchant_abnormal_rate_7d"] = float64(summary7d.AbnormalClaims) / float64(summary7d.TotalOrders)
			}
			metadata["merchant_abnormal_rate_7d_exceeded"] = summary7d.TotalOrders > 0 && float64(summary7d.AbnormalClaims)/float64(summary7d.TotalOrders) >= thresholds.MerchantAbnormalRate
		}

		if delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID); err == nil && delivery.RiderID.Valid {
			riderID := delivery.RiderID.Int64
			metadata["rider_id"] = riderID
			if summary, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
				EntityType: "rider",
				EntityID:   riderID,
				StatDate:   startDateParam,
				StatDate_2: endDateParam,
			}); err == nil {
				metadata["rider_total_orders_30d"] = summary.TotalOrders
				metadata["rider_abnormal_claims_30d"] = summary.AbnormalClaims
				if summary.TotalOrders > 0 {
					metadata["rider_abnormal_rate_30d"] = float64(summary.AbnormalClaims) / float64(summary.TotalOrders)
				}
				metadata["rider_abnormal_rate_30d_exceeded"] = summary.TotalOrders > 0 && float64(summary.AbnormalClaims)/float64(summary.TotalOrders) >= thresholds.RiderAbnormalRate
			}
			if summary7d, err := server.store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
				EntityType: "rider",
				EntityID:   riderID,
				StatDate:   start7d,
				StatDate_2: endDate,
			}); err == nil {
				metadata["rider_total_orders_7d"] = summary7d.TotalOrders
				metadata["rider_abnormal_claims_7d"] = summary7d.AbnormalClaims
				if summary7d.TotalOrders > 0 {
					metadata["rider_abnormal_rate_7d"] = float64(summary7d.AbnormalClaims) / float64(summary7d.TotalOrders)
				}
				metadata["rider_abnormal_rate_7d_exceeded"] = summary7d.TotalOrders > 0 && float64(summary7d.AbnormalClaims)/float64(summary7d.TotalOrders) >= thresholds.RiderAbnormalRate
			}
		}

		ruleInput := rules.Context{
			Domain:     rules.DomainClaim,
			RegionID:   regionID,
			MerchantID: order.MerchantID,
			UserID:     authPayload.UserID,
			OrderType:  order.OrderType,
			Amount:     req.ClaimAmount,
			Metadata:   metadata,
		}
		decision, err := server.rulesEngine.Evaluate(ctx, ruleInput)
		if err != nil {
			// 规则引擎故障时仍继续自动裁定，不再转人工审核。
			log.Error().Err(err).
				Int64("order_id", req.OrderID).
				Int64("user_id", authPayload.UserID).
				Msg("Rules engine evaluation failed, falling back to deterministic platform_fallback adjudication")

			ruleDecision = rules.Decision{
				Action: "platform-fallback",
				Reason: "系统风控服务暂时不可用，已按平台自动裁定继续处理",
			}
		} else {
			server.recordRuleHit(ctx, ruleInput, decision, RoleCustomer)
			ruleDecision = decision
			if !decision.Allow {
				reason := decision.Reason
				if reason == "" {
					reason = "claim blocked by rule"
				}
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New(reason)))
				return
			}
		}
	}

	// SubmitClaim 的正式主链边界：
	// 1. API 负责请求校验、规则覆盖标准化、证据采集与依赖注入；
	// 2. algorithm 负责生成 claim 创建参数并消费事务返回的 persisted decision/action；
	// 3. API 不再通过二次查询 behavior decision/action 自行拼接 payout 等副作用。
	// 后续任何读路径（如申诉、对账、调度）如需读取 decision，只能作为只读消费者。
	// 创建自动审核器
	approver := algorithm.NewClaimAutoApproval(server.store, server.wsHub)
	if server.taskDistributor != nil {
		approver.SetNotificationDistributor(worker.NewNotificationAdapter(server.taskDistributor))
		approver.SetClaimPayoutDistributor(worker.NewClaimPayoutAdapter(server.taskDistributor))
	}

	// 评估索赔（新设计）
	decision, err := approver.EvaluateClaim(
		ctx,
		authPayload.UserID,
		req.OrderID,
		algorithm.ClaimCompensationContext{
			RequestedAmount:     req.ClaimAmount,
			OrderTotalAmount:    order.TotalAmount,
			DeliveryFee:         order.DeliveryFee,
			DeliveryFeeDiscount: order.DeliveryFeeDiscount,
		},
		req.ClaimType,
	)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 规则引擎结果覆盖（如需）
	if ruleDecision.Action != "" && ruleDecision.Action != "allow" && ruleDecision.Action != "alert" {
		applyClaimRuleDecisionOverride(decision, ruleDecision)
	}
	if ruleDecision.Meta != nil {
		if v, ok := ruleDecision.Meta["decision_reason"].(string); ok && v != "" && decision.Reason == "" {
			decision.Reason = v
		}
	}

	// 采集证据信息（事务内落库）
	deviceID := ""
	deviceType := ""
	if devices, err := server.store.GetDevicesByUserID(ctx, authPayload.UserID); err == nil && len(devices) > 0 {
		deviceID = devices[0].DeviceID
		deviceType = devices[0].DeviceType
	}
	var addressID *int64
	if order.AddressID.Valid {
		addr := order.AddressID.Int64
		addressID = &addr
	}

	evidenceContext := &algorithm.ClaimEvidenceContext{
		DeviceID:          deviceID,
		DeviceFingerprint: req.DeviceFingerprint,
		DeviceType:        deviceType,
		IPAddress:         ctx.ClientIP(),
		UserAgent:         ctx.Request.UserAgent(),
		AddressID:         addressID,
	}

	// 生成追偿单（责任方为商户/骑手且需要追偿）
	responsibleParty := "unknown"
	recoveryTarget := ""
	recoveryRequired := false
	recoveryAmount := decision.Amount
	if ruleDecision.Meta != nil {
		if v, ok := ruleDecision.Meta["responsible_party"].(string); ok && v != "" {
			responsibleParty = v
		}
		if v, ok := ruleDecision.Meta["recovery_required"].(bool); ok {
			recoveryRequired = v
		}
		if v, ok := ruleDecision.Meta["recovery_target"].(string); ok && v != "" {
			recoveryTarget = v
		}
		if v, ok := ruleDecision.Meta["recovery_amount"].(float64); ok {
			recoveryAmount = int64(v)
		}
		if v, ok := ruleDecision.Meta["recovery_amount"].(int64); ok {
			recoveryAmount = v
		}
	}
	if responsibleParty == "platform_fallback" {
		decision.CompensationSource = algorithm.CompensationSourcePlatform
		decision.Type = algorithm.DecisionModePlatformFallback
		recoveryRequired = false
		recoveryTarget = ""
	}
	if responsibleParty == "unknown" && decision.CompensationSource != "" {
		switch decision.CompensationSource {
		case algorithm.CompensationSourceMerchant:
			responsibleParty = "merchant"
		case algorithm.CompensationSourceRider:
			responsibleParty = "rider"
		case algorithm.CompensationSourcePlatform:
			responsibleParty = "platform_fallback"
		}
	}
	if responsibleParty == "platform_fallback" {
		recoveryRequired = false
		recoveryTarget = ""
	}
	if !recoveryRequired {
		recoveryRequired = responsibleParty == "merchant" || responsibleParty == "rider"
	}
	if recoveryTarget == "" && (responsibleParty == "merchant" || responsibleParty == "rider") {
		recoveryTarget = responsibleParty
	}
	if recoveryAmount <= 0 {
		recoveryAmount = decision.Amount
	}
	var recoveryPlan *algorithm.ClaimRecoveryPlan
	if decision.Approved && recoveryRequired && (recoveryTarget == "merchant" || recoveryTarget == "rider") {
		decisionSnapshot := map[string]any{
			"decision_type":       decision.Type,
			"decision_reason":     decision.Reason,
			"behavior_status":     decision.BehaviorStatus,
			"compensation_source": decision.CompensationSource,
			"rule_action":         ruleDecision.Action,
			"rule_reason":         ruleDecision.Reason,
			"rule_meta":           ruleDecision.Meta,
			"responsible_party":   responsibleParty,
			"recovery_target":     recoveryTarget,
			"recovery_amount":     recoveryAmount,
		}
		decisionSnapshotJSON, _ := json.Marshal(decisionSnapshot)
		dueAt := time.Now().Add(24 * time.Hour)
		recoveryPlan = &algorithm.ClaimRecoveryPlan{
			ResponsibleParty: responsibleParty,
			RecoveryTarget:   recoveryTarget,
			RecoveryAmount:   recoveryAmount,
			DueAt:            dueAt,
			DecisionSnapshot: decisionSnapshotJSON,
		}
	}
	if decision.Approved && decision.Amount > 0 && server.paymentClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrClaimPayoutServiceUnavailable))
		return
	}

	// 创建索赔记录（必要时在同一事务中创建追偿单）
	claim, err := approver.CreateClaimWithDecisionAndEvidence(
		ctx,
		req.OrderID,
		authPayload.UserID,
		req.ClaimType,
		req.ClaimReason,
		req.ClaimAmount,
		decision,
		evidenceContext,
		recoveryPlan,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create claim with decision: %w", err)))
		return
	}

	// 构造响应
	resp := SubmitClaimResponse{
		ClaimID:            claim.ID,
		Status:             submitClaimStatusAccepted,
		CompensationSource: decision.CompensationSource,
		Reason:             decision.Reason,
	}

	if decision.Approved {
		resp.ApprovedAmount = &decision.Amount
		resp.DecisionStatus = submitClaimDecisionStatusAutoAdjudicated
		resp.PayoutStatus = submitClaimPayoutStatusProcessing
		resp.PayoutETA = submitClaimPayoutETA(algorithm.DecisionApprovalType(decision.Type))
	}

	// 如果有警告信息，添加到响应
	if decision.Warning != "" {
		resp.Warning = &decision.Warning
	}

	metadata := map[string]any{
		"order_id":            req.OrderID,
		"claim_type":          req.ClaimType,
		"status":              resp.Status,
		"decision_status":     resp.DecisionStatus,
		"payout_status":       resp.PayoutStatus,
		"compensation_source": resp.CompensationSource,
		"requested_amount":    req.ClaimAmount,
		"approved_amount":     decision.Amount,
		"auto_adjudicated":    decision.Approved,
	}
	if resp.PayoutETA != nil {
		metadata["payout_eta"] = *resp.PayoutETA
	}
	if resp.Warning != nil {
		metadata["warning"] = *resp.Warning
	}
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "customer",
		Action:      "user_claim_submitted",
		TargetType:  "claim",
		TargetID:    &claim.ID,
		Metadata:    metadata,
	})

	// 📢 异步执行商户/骑手索赔历史检查（避免阻塞API响应）
	if server.taskDistributor != nil {
		// 异物索赔：检查商户历史
		if req.ClaimType == "foreign-object" {
			_ = server.taskDistributor.DistributeTaskCheckMerchantForeignObject(
				ctx,
				order.MerchantID,
				asynq.Queue(worker.QueueDefault),
				asynq.MaxRetry(3),
			)
		}
		// 餐损/超时索赔：如果是外卖订单，检查骑手历史
		if (req.ClaimType == "damage" || req.ClaimType == "timeout") && order.OrderType == "takeout" {
			// 获取骑手ID
			delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
			if err == nil && delivery.RiderID.Valid {
				_ = server.taskDistributor.DistributeTaskCheckRiderDamage(
					ctx,
					delivery.RiderID.Int64,
					asynq.Queue(worker.QueueDefault),
					asynq.MaxRetry(3),
				)
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// ReportFoodSafetyRequest 上报食安请求
type ReportFoodSafetyRequest struct {
	ReporterID    int64  `json:"reporter_id" binding:"required,min=1"`
	MerchantID    int64  `json:"merchant_id" binding:"required,min=1"`
	OrderID       int64  `json:"order_id" binding:"required,min=1"`
	IncidentType  string `json:"incident_type" binding:"required,oneof=foreign-object contamination expired"`
	Description   string `json:"description" binding:"required,min=10,max=1000"`
	SeverityLevel int16  `json:"severity_level" binding:"required,min=1,max=5"`
}

// ReportFoodSafetyResponse 食安上报响应
type ReportFoodSafetyResponse struct {
	IncidentID        int64  `json:"incident_id"`
	MerchantSuspended bool   `json:"merchant_suspended"`
	SuspendDuration   *int   `json:"suspend_duration,omitempty"` // 小时
	Message           string `json:"message"`
}

// ReportFoodSafety 上报食安问题
// @Summary 上报食品安全问题
// @Description 用户上报商户食品安全问题，系统将根据举报频率与协同模式决定是否熔断商户
// @Tags 食品安全
// @Accept json
// @Produce json
// @Param request body ReportFoodSafetyRequest true "食安上报信息"
// @Success 200 {object} ReportFoodSafetyResponse "上报成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/food-safety/report [post]
// @Security BearerAuth
func (server *Server) ReportFoodSafety(ctx *gin.Context) {
	var req ReportFoodSafetyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证严重程度
	if req.SeverityLevel < 1 || req.SeverityLevel > 5 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("severity_level must be between 1 and 5")))
		return
	}

	// 创建食安处理器
	handler := algorithm.NewFoodSafetyHandler(server.store, server.wsHub)

	// 评估食安举报（无证据输入）
	result, err := handler.EvaluateFoodSafetyReport(
		ctx,
		req.ReporterID,
		req.MerchantID,
		nil,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("evaluate food safety report for order %d: %w", req.OrderID, err)))
		return
	}

	// 创建食安事件记录
	incident, err := server.store.CreateFoodSafetyIncident(ctx, db.CreateFoodSafetyIncidentParams{
		UserID:           req.ReporterID,
		MerchantID:       req.MerchantID,
		OrderID:          req.OrderID,
		IncidentType:     req.IncidentType,
		Description:      req.Description,
		OrderSnapshot:    []byte{},
		MerchantSnapshot: []byte{},
		RiderSnapshot:    []byte{},
		Status:           "pending",
		CreatedAt:        time.Now(),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create food safety incident for order %d: %w", req.OrderID, err)))
		return
	}

	// 执行熔断
	if result.ShouldCircuitBreak {
		err = handler.CircuitBreakMerchant(
			ctx,
			req.MerchantID,
			fmt.Sprintf("食安举报确认（事件ID: %d）", incident.ID),
			result.DurationHours,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("circuit break merchant %d: %w", req.MerchantID, err)))
			return
		}
	}

	resp := ReportFoodSafetyResponse{
		IncidentID:        incident.ID,
		MerchantSuspended: result.ShouldCircuitBreak,
		Message:           result.Message,
	}

	if result.ShouldCircuitBreak {
		resp.SuspendDuration = &result.DurationHours
	}

	ctx.JSON(http.StatusOK, resp)
}

// TriggerFraudDetectionRequest 触发欺诈检测请求
type TriggerFraudDetectionRequest struct {
	ClaimID           *int64  `json:"claim_id,omitempty"`
	DeviceFingerprint *string `json:"device_fingerprint,omitempty"`
	AddressID         *int64  `json:"address_id,omitempty"`
}

// TriggerFraudDetection 触发欺诈检测
// @Summary 触发欺诈检测
// @Description 管理员手动触发欺诈检测，支持三种检测模式：协同索赔检测、设备复用检测、地址聚类检测
// @Tags 欺诈检测
// @Accept json
// @Produce json
// @Param request body TriggerFraudDetectionRequest true "检测请求（三选一）"
// @Success 200 {object} algorithm.FraudDetectionResult "检测结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/fraud/detect [post]
// @Security BearerAuth
func (server *Server) TriggerFraudDetection(ctx *gin.Context) {
	var req TriggerFraudDetectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	detector := algorithm.NewFraudDetector(server.store, server.wsHub)

	// 协同索赔检测
	if req.ClaimID != nil {
		result, err := detector.DetectCoordinatedClaims(ctx, *req.ClaimID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("detect coordinated claims for claim %d: %w", *req.ClaimID, err)))
			return
		}
		ctx.JSON(http.StatusOK, result)
		return
	}

	// 设备复用检测
	if req.DeviceFingerprint != nil {
		result, err := detector.DetectDeviceReuse(ctx, *req.DeviceFingerprint)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("detect device reuse for fingerprint %s: %w", *req.DeviceFingerprint, err)))
			return
		}
		ctx.JSON(http.StatusOK, result)
		return
	}

	// 地址聚类检测
	if req.AddressID != nil {
		result, err := detector.DetectAddressCluster(ctx, *req.AddressID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("detect address cluster for address %d: %w", *req.AddressID, err)))
			return
		}
		ctx.JSON(http.StatusOK, result)
		return
	}

	ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("must provide claim_id, device_fingerprint, or address_id")))
}

// SuspendMerchantRequest 熔断商户请求（管理员使用）
type SuspendMerchantRequest struct {
	MerchantID    int64  `json:"merchant_id" binding:"required,min=1"`
	Reason        string `json:"reason" binding:"required,min=5,max=500"`
	DurationHours int    `json:"duration_hours" binding:"required,min=1,max=720"` // 最长30天
	AdminID       int64  `json:"admin_id" binding:"required,min=1"`
}

// SuspendMerchant 熔断商户
// @Summary 熔断商户
// @Description 管理员手动熔断（停业）商户，指定停业时长和原因
// @Tags 商户管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body SuspendMerchantRequest true "熔断信息"
// @Success 200 {object} MessageResponse "熔断成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/food-safety/merchants/{id}/suspend [patch]
// @Security BearerAuth
func (server *Server) SuspendMerchant(ctx *gin.Context) {
	merchantIDStr := ctx.Param("id")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req SuspendMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if merchantID != req.MerchantID {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant_id mismatch")))
		return
	}

	handler := algorithm.NewFoodSafetyHandler(server.store, server.wsHub)
	err = handler.CircuitBreakMerchant(ctx, merchantID, req.Reason, req.DurationHours)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("circuit break merchant %d: %w", merchantID, err)))
		return
	}

	ctx.JSON(http.StatusOK, successMessage(fmt.Sprintf("商户 %d 已熔断 %d 小时", merchantID, req.DurationHours)))
}

// ResumeMerchantRequest 恢复商户请求
type ResumeMerchantRequest struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

// ResumeMerchant 恢复商户上线（运营商）
func (server *Server) ResumeMerchant(ctx *gin.Context) {
	merchantIDStr := ctx.Param("id")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req ResumeMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户信息以验证区域
	merchant, err := server.store.GetMerchant(ctx, merchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant %d: %w", merchantID, err)))
		return
	}

	// 验证 operator 是否管理该商户的区域
	if _, err := server.checkOperatorManagesRegion(ctx, merchant.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	if err = server.store.UnsuspendMerchant(ctx, merchantID); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("unsuspend merchant %d: %w", merchantID, err)))
		return
	}

	if err = server.store.UnsuspendMerchantTakeout(ctx, merchantID); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("unsuspend merchant takeout %d: %w", merchantID, err)))
		return
	}

	// 更新商户状态为正常
	_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     merchantID,
		Status: "active",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("resume merchant %d: %w", merchantID, err)))
		return
	}

	ctx.JSON(http.StatusOK, successMessage(fmt.Sprintf("商户 %d 已恢复上线", merchantID)))
}

// SuspendRiderRequest 暂停骑手请求
type SuspendRiderRequest struct {
	Reason        string `json:"reason" binding:"required,min=5,max=500"`
	DurationHours int    `json:"duration_hours" binding:"required,min=1,max=720"` // 最长30天
}

// SuspendRider 暂停骑手上线（运营商）
func (server *Server) SuspendRider(ctx *gin.Context) {
	riderIDStr := ctx.Param("id")
	riderID, err := strconv.ParseInt(riderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req SuspendRiderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取骑手信息以验证区域
	rider, err := server.store.GetRider(ctx, riderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rider not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider %d: %w", riderID, err)))
		return
	}

	// 验证骑手有区域且 operator 管理该区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rider has no assigned region")))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 更新骑手状态为暂停
	updatedRider, err := server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: "suspended",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("suspend rider %d: %w", riderID, err)))
		return
	}
	if _, err = db.ReconcileRiderOperationalStatus(ctx, server.store, updatedRider); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reconcile suspended rider %d: %w", riderID, err)))
		return
	}

	ctx.JSON(http.StatusOK, riderSuspendResponse{Message: fmt.Sprintf("骑手 %d 已暂停 %d 小时", riderID, req.DurationHours), Reason: req.Reason, DurationHours: req.DurationHours})
}

// ResumeRiderRequest 恢复骑手请求
type ResumeRiderRequest struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

// ResumeRider 恢复骑手上线（运营商）
func (server *Server) ResumeRider(ctx *gin.Context) {
	riderIDStr := ctx.Param("id")
	riderID, err := strconv.ParseInt(riderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req ResumeRiderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取骑手信息以验证区域
	rider, err := server.store.GetRider(ctx, riderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rider not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider %d: %w", riderID, err)))
		return
	}

	// 验证骑手有区域且 operator 管理该区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rider has no assigned region")))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 恢复后先回到 approved，再按押金阈值统一收敛为 approved/active。
	restoredRider, err := server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: db.RiderStatusApproved,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("resume rider %d: %w", riderID, err)))
		return
	}
	if _, err = db.ReconcileRiderOperationalStatus(ctx, server.store, restoredRider); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reconcile resumed rider %d: %w", riderID, err)))
		return
	}

	ctx.JSON(http.StatusOK, successMessage(fmt.Sprintf("骑手 %d 已恢复上线", riderID)))
}

// ==================== 用户索赔查询 API ====================

type userClaimResponse struct {
	ID             int64      `json:"id"`
	OrderID        int64      `json:"order_id"`
	ClaimType      string     `json:"claim_type"`
	Description    string     `json:"description"`
	ClaimAmount    int64      `json:"claim_amount"`
	ApprovedAmount *int64     `json:"approved_amount,omitempty"`
	Status         string     `json:"status"`                    // accepted, rejected
	DecisionStatus string     `json:"decision_status,omitempty"` // auto-adjudicated, rejected
	PayoutStatus   string     `json:"payout_status,omitempty"`   // processing, paid
	Reason         string     `json:"reason,omitempty"`
	PayoutETA      *string    `json:"payout_eta,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
}

type riderSuspendResponse struct {
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"duration_hours"`
}

type userClaimsListResponse struct {
	Claims   []userClaimResponse `json:"claims"`
	Total    int64               `json:"total"`
	PageSize int                 `json:"page_size"`
	Page     int                 `json:"page"`
}

func newUserClaimResponse(claim db.Claim) userClaimResponse {
	status, decisionStatus, payoutStatus, payoutETA := userClaimLifecycleFromClaim(claim)
	resp := userClaimResponse{
		ID:             claim.ID,
		OrderID:        claim.OrderID,
		ClaimType:      claim.ClaimType,
		Description:    claim.Description,
		ClaimAmount:    claim.ClaimAmount,
		Status:         status,
		DecisionStatus: decisionStatus,
		PayoutStatus:   payoutStatus,
		Reason:         userClaimReason(claim),
		PayoutETA:      payoutETA,
		CreatedAt:      claim.CreatedAt,
		ProcessedAt:    userClaimProcessedAt(claim),
	}

	if claim.ApprovedAmount.Valid {
		resp.ApprovedAmount = &claim.ApprovedAmount.Int64
	}

	return resp
}

// ListUserClaims 获取用户的索赔列表
// @Summary 获取我的索赔列表
// @Description 获取当前用户提交的所有索赔记录
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} userClaimsListResponse "索赔列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims [get]
// @Security BearerAuth
func (server *Server) ListUserClaims(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := pageOffset(int32(page), int32(pageSize))

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	claims, err := server.store.ListUserClaims(ctx, db.ListUserClaimsParams{
		UserID: authPayload.UserID,
		Limit:  int32(pageSize),
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list claims for user %d: %w", authPayload.UserID, err)))
		return
	}

	totalCount, err := server.store.CountUserClaimsInPeriod(ctx, db.CountUserClaimsInPeriodParams{
		UserID:    authPayload.UserID,
		CreatedAt: time.Unix(0, 0),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("count claims for user %d: %w", authPayload.UserID, err)))
		return
	}

	var response []userClaimResponse
	for _, c := range claims {
		response = append(response, newUserClaimResponse(c))
	}

	if response == nil {
		response = []userClaimResponse{}
	}

	ctx.JSON(http.StatusOK, userClaimsListResponse{Claims: response, Total: totalCount, PageSize: pageSize, Page: page})
}

// GetClaimDetail 获取索赔详情
// @Summary 获取索赔详情
// @Description 获取指定索赔的详细信息，只能查看自己提交的索赔
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Success 200 {object} userClaimResponse "索赔详情"
// @Failure 400 {object} ErrorResponse "无效的索赔ID"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "该索赔不属于当前用户"
// @Failure 404 {object} ErrorResponse "索赔不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims/{id} [get]
// @Security BearerAuth
func (server *Server) GetClaimDetail(ctx *gin.Context) {
	claimIDStr := ctx.Param("id")
	claimID, err := strconv.ParseInt(claimIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidClaimID))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	claim, err := server.store.GetClaim(ctx, claimID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrClaimNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get claim %d: %w", claimID, err)))
		return
	}

	// 验证是当前用户的索赔
	if claim.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrClaimNotOwned))
		return
	}

	ctx.JSON(http.StatusOK, newUserClaimResponse(claim))
}
