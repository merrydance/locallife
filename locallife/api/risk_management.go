package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

var manualReviewEnabled = false

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

// SubmitClaimRequest 提交索赔请求
type SubmitClaimRequest struct {
	OrderID           int64  `json:"order_id" binding:"required,min=1"`
	ClaimType         string `json:"claim_type" binding:"required,oneof=foreign-object damage timeout food-safety"`
	ClaimAmount       int64  `json:"claim_amount" binding:"required,min=1,max=100000000"` // 最高100万分(1万元)
	ClaimReason       string `json:"claim_reason" binding:"required,min=5,max=1000"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty" binding:"omitempty,max=256"`
}

// SubmitClaimResponse 索赔响应
type SubmitClaimResponse struct {
	ClaimID            int64   `json:"claim_id"`
	Status             string  `json:"status"` // instant, auto, manual, platform-pay
	ApprovedAmount     *int64  `json:"approved_amount,omitempty"`
	CompensationSource string  `json:"compensation_source,omitempty"` // merchant, rider, platform
	Reason             string  `json:"reason"`
	RefundETA          *string `json:"refund_eta,omitempty"` // 秒赔/自动通过时提供预计到账时间
	Warning            *string `json:"warning,omitempty"`    // 警告信息
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
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get order: %w", err)))
		return
	}

	// 2. 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("订单不属于当前用户")))
		return
	}

	// 3. 验证订单已完成（只有完成的订单才能索赔）
	if order.Status != OrderStatusCompleted {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只有已完成的订单才能申请索赔")))
		return
	}

	// 3.1 行为黑名单拦截（拒绝服务用户）
	if _, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   authPayload.UserID,
	}); err == nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("账户存在异常行为限制，无法提交索赔")))
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
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("该订单已存在索赔记录")))
			return
		}
	}

	// 5. 索赔金额不能超过订单总额
	if req.ClaimAmount > order.TotalAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("索赔金额不能超过订单总额")))
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
			// P1-004 Fix: 规则引擎故障时降级为人工审核，而不是直接报错
			log.Error().Err(err).
				Int64("order_id", req.OrderID).
				Int64("user_id", authPayload.UserID).
				Msg("Rules engine evaluation failed, falling back to manual review")

			ruleDecision = rules.Decision{
				Action: "manual",
				Reason: "系统风控服务暂时不可用，转入人工审核",
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

	// 7. 获取配送费（超时索赔用）
	var deliveryFee int64
	if req.ClaimType == algorithm.ClaimTypeTimeout || req.ClaimType == algorithm.ClaimTypeDamage {
		delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil {
			deliveryFee = delivery.DeliveryFee
		}
	}

	// 创建自动审核器
	approver := algorithm.NewClaimAutoApproval(server.store, server.wsHub)

	// 评估索赔（新设计）
	decision, err := approver.EvaluateClaim(
		ctx,
		authPayload.UserID,
		req.OrderID,
		req.ClaimAmount,
		deliveryFee,
		req.ClaimType,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("evaluate claim: %w", err)))
		return
	}

	// 规则引擎结果覆盖（如需）
	if ruleDecision.Action != "" && ruleDecision.Action != "allow" && ruleDecision.Action != "alert" {
		switch ruleDecision.Action {
		case "manual":
			decision.Type = algorithm.ApprovalTypeManual
			decision.Approved = false
			decision.Amount = 0
			decision.Reason = ruleDecision.Reason
			decision.NeedsReview = true
		case "platform-pay":
			decision.Type = "platform-pay"
			decision.CompensationSource = algorithm.CompensationSourcePlatform
			if ruleDecision.Reason != "" {
				decision.Reason = ruleDecision.Reason
			}
		case "instant", "auto":
			decision.Type = ruleDecision.Action
			if ruleDecision.Reason != "" {
				decision.Reason = ruleDecision.Reason
			}
		}
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

	// 创建索赔记录
	claim, err := approver.CreateClaimWithDecisionAndEvidence(
		ctx,
		req.OrderID,
		authPayload.UserID,
		req.ClaimType,
		req.ClaimReason,
		decision.Amount, // 使用决策后的金额（超时可能只赔运费）
		decision,
		evidenceContext,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create claim with decision: %w", err)))
		return
	}

	// 平台先行赔付（异步，不阻塞响应）
	if decision.Approved && decision.Amount > 0 {
		if server.taskDistributor != nil {
			// P2-001 修复: 使用 TaskQueue 保证退款可靠执行
			err := server.taskDistributor.DistributeTaskClaimRefund(ctx, &worker.ClaimRefundPayload{
				ClaimID:    claim.ID,
				UserID:     authPayload.UserID,
				Amount:     decision.Amount,
				SourceType: "platform",
				SourceID:   0,
				Remark:     "platform payout",
			}, asynq.Queue(worker.QueueCritical))
			if err != nil {
				fmt.Printf("failed to enqueue claim refund task: %v\n", err)
			}
		} else {
			// Fallback (测试环境或无Redis): 使用原有的 goroutine
			go func() {
				_, refundErr := server.store.ClaimRefundTx(context.Background(), db.ClaimRefundTxParams{
					ClaimID:    claim.ID,
					UserID:     authPayload.UserID,
					Amount:     decision.Amount,
					SourceType: "platform",
					SourceID:   0,
					Remark:     "platform payout",
				})
				if refundErr != nil {
					fmt.Printf("claim refund failed: claim_id=%d, user_id=%d, err=%v\n", claim.ID, authPayload.UserID, refundErr)
				}
			}()
		}
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
		userSafe := !boolMeta(ruleDecision.Meta, "user_claims_7d_exceeded") &&
			!boolMeta(ruleDecision.Meta, "user_claims_30d_exceeded") &&
			!boolMeta(ruleDecision.Meta, "user_claim_rate_7d_exceeded") &&
			!boolMeta(ruleDecision.Meta, "user_claim_rate_30d_exceeded")
		merchantSafe := !boolMeta(ruleDecision.Meta, "merchant_abnormal_rate_7d_exceeded") &&
			!boolMeta(ruleDecision.Meta, "merchant_abnormal_rate_30d_exceeded")
		riderSafe := !boolMeta(ruleDecision.Meta, "rider_abnormal_rate_7d_exceeded") &&
			!boolMeta(ruleDecision.Meta, "rider_abnormal_rate_30d_exceeded")
		if userSafe && merchantSafe && riderSafe {
			decision.CompensationSource = algorithm.CompensationSourcePlatform
			decision.Type = "platform-pay"
			recoveryRequired = false
			recoveryTarget = ""
		} else {
			responsibleParty = "unknown"
		}
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
		_, _ = server.store.CreateClaimRecovery(ctx, db.CreateClaimRecoveryParams{
			ClaimID:          claim.ID,
			OrderID:          req.OrderID,
			ResponsibleParty: responsibleParty,
			RecoveryTarget:   pgtype.Text{String: recoveryTarget, Valid: recoveryTarget != ""},
			RecoveryAmount:   recoveryAmount,
			Status:           "pending",
			DueAt:            dueAt,
			DecisionSnapshot: decisionSnapshotJSON,
		})
	}

	// 构造响应
	resp := SubmitClaimResponse{
		ClaimID:            claim.ID,
		Status:             decision.Type,
		CompensationSource: decision.CompensationSource,
		Reason:             decision.Reason,
	}

	if decision.Approved {
		resp.ApprovedAmount = &decision.Amount

		// 秒赔和自动通过提供预计到账时间
		if decision.Type == "instant" || decision.Type == "auto" || decision.Type == "platform-pay" {
			eta := "1-3个工作日"
			if decision.Type == "instant" {
				eta = "即时到账"
			}
			resp.RefundETA = &eta
		}
	}

	// 如果有警告信息，添加到响应
	if decision.Warning != "" {
		resp.Warning = &decision.Warning
	}

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

// ReviewClaimRequest 审核索赔请求
type ReviewClaimRequest struct {
	Approved       *bool  `json:"approved" binding:"required"`                         // 是否通过
	ApprovedAmount *int64 `json:"approved_amount,omitempty" binding:"omitempty,min=1"` // 审核通过金额（分）
	ReviewNote     string `json:"review_note" binding:"required,min=5,max=500"`        // 审核备注
}

// ReviewClaim 人工审核索赔
// @Summary 审核索赔
// @Description 运营商/客服人工审核索赔申请。仅限低信用用户提交的需要人工审核的索赔。
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Param request body ReviewClaimRequest true "审核信息"
// @Success 200 {object} claimResponse "审核成功"
// @Failure 400 {object} ErrorResponse "参数错误或索赔状态不允许审核"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权限审核此索赔"
// @Failure 404 {object} ErrorResponse "索赔不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims/{id}/review [patch]
// @Security BearerAuth
func (server *Server) ReviewClaim(ctx *gin.Context) {
	if !manualReviewEnabled {
		ctx.JSON(http.StatusGone, errorResponse(errors.New("manual review disabled; claims are auto-adjudicated")))
		return
	}

	claimIDStr := ctx.Param("id")
	claimID, err := strconv.ParseInt(claimIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的索赔ID")))
		return
	}

	var req ReviewClaimRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前审核员信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取索赔记录
	claim, err := server.store.GetClaim(ctx, claimID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("索赔记录不存在")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get claim %d: %w", claimID, err)))
		}
		return
	}

	// 检查状态 - 只允许审核pending状态的索赔
	if claim.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该索赔已审核完成")))
		return
	}

	// 检查是否是需要人工审核的索赔
	if claim.ApprovalType.Valid && claim.ApprovalType.String != "manual" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该索赔无需人工审核")))
		return
	}

	// 通过时必须提供审核金额
	if *req.Approved && req.ApprovedAmount == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("通过审核时必须提供审核金额")))
		return
	}

	// 审核金额不能超过索赔金额
	if req.ApprovedAmount != nil && *req.ApprovedAmount > claim.ClaimAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("审核金额不能超过索赔金额")))
		return
	}

	// 更新索赔状态
	status := "rejected"
	if *req.Approved {
		status = "approved"
	}

	params := db.UpdateClaimStatusParams{
		ID:     claimID,
		Status: status,
		ReviewerID: pgtype.Int8{
			Int64: authPayload.UserID, // 使用token中的用户ID
			Valid: true,
		},
		ReviewedAt: pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: true,
		},
		ReviewNotes: pgtype.Text{
			String: req.ReviewNote,
			Valid:  true,
		},
	}

	if req.ApprovedAmount != nil {
		params.ApprovedAmount = pgtype.Int8{
			Int64: *req.ApprovedAmount,
			Valid: true,
		}
	}

	err = server.store.UpdateClaimStatus(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update claim %d status: %w", claimID, err)))
		return
	}

	// 重新获取更新后的索赔记录
	updatedClaim, err := server.store.GetClaim(ctx, claimID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get updated claim %d: %w", claimID, err)))
		return
	}

	ctx.JSON(http.StatusOK, newClaimResponse(updatedClaim))
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
	_, err = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: "suspended",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("suspend rider %d: %w", riderID, err)))
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

	// 更新骑手状态为正常
	_, err = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: "active",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("resume rider %d: %w", riderID, err)))
		return
	}

	ctx.JSON(http.StatusOK, successMessage(fmt.Sprintf("骑手 %d 已恢复上线", riderID)))
}

// ==================== 用户索赔查询 API ====================

type claimResponse struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	OrderID        int64      `json:"order_id"`
	ClaimType      string     `json:"claim_type"`
	Description    string     `json:"description"`
	ClaimAmount    int64      `json:"claim_amount"`
	ApprovedAmount *int64     `json:"approved_amount,omitempty"`
	Status         string     `json:"status"`
	ApprovalType   string     `json:"approval_type,omitempty"`
	ReviewerID     *int64     `json:"reviewer_id,omitempty"`
	ReviewNotes    string     `json:"review_notes,omitempty"`
	IsMalicious    bool       `json:"is_malicious"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
}

type riderSuspendResponse struct {
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"duration_hours"`
}

type claimsListResponse struct {
	Claims   []claimResponse `json:"claims"`
	Total    int64           `json:"total"`
	PageSize int             `json:"page_size"`
	Page     int             `json:"page"`
}

func newClaimResponse(claim db.Claim) claimResponse {
	resp := claimResponse{
		ID:          claim.ID,
		UserID:      claim.UserID,
		OrderID:     claim.OrderID,
		ClaimType:   claim.ClaimType,
		Description: claim.Description,
		ClaimAmount: claim.ClaimAmount,
		Status:      claim.Status,
		IsMalicious: claim.IsMalicious,
		CreatedAt:   claim.CreatedAt,
	}

	// 处理可空字段
	if claim.ApprovedAmount.Valid {
		resp.ApprovedAmount = &claim.ApprovedAmount.Int64
	}
	if claim.ApprovalType.Valid {
		resp.ApprovalType = claim.ApprovalType.String
	}
	if claim.ReviewerID.Valid {
		resp.ReviewerID = &claim.ReviewerID.Int64
	}
	if claim.ReviewNotes.Valid {
		resp.ReviewNotes = claim.ReviewNotes.String
	}
	if claim.ReviewedAt.Valid {
		resp.ReviewedAt = &claim.ReviewedAt.Time
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
// @Success 200 {object} map[string]interface{} "索赔列表"
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

	var response []claimResponse
	for _, c := range claims {
		response = append(response, newClaimResponse(c))
	}

	if response == nil {
		response = []claimResponse{}
	}

	ctx.JSON(http.StatusOK, claimsListResponse{Claims: response, Total: totalCount, PageSize: pageSize, Page: page})
}

// GetClaimDetail 获取索赔详情
// @Summary 获取索赔详情
// @Description 获取指定索赔的详细信息，只能查看自己提交的索赔
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Success 200 {object} claimResponse "索赔详情"
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
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的索赔ID")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	claim, err := server.store.GetClaim(ctx, claimID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("索赔不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get claim %d: %w", claimID, err)))
		return
	}

	// 验证是当前用户的索赔
	if claim.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("该索赔不属于当前用户")))
		return
	}

	ctx.JSON(http.StatusOK, newClaimResponse(claim))
}
