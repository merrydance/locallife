package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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
	appliedAction := ""
	switch normalizedAction {
	case "merchant-recovery":
		appliedAction = normalizedAction
		decision.Type = algorithm.DecisionModeMerchantRecovery
		decision.Approved = true
		decision.CompensationSource = algorithm.CompensationSourceMerchant
	case "rider-recovery":
		appliedAction = normalizedAction
		decision.Type = algorithm.DecisionModeRiderRecovery
		decision.Approved = true
		decision.CompensationSource = algorithm.CompensationSourceRider
	case "user-restricted":
		appliedAction = normalizedAction
		decision.Type = algorithm.DecisionModeUserRestricted
		decision.Approved = true
		decision.BehaviorStatus = algorithm.ClaimBehaviorUserRestricted
		decision.CompensationSource = algorithm.CompensationSourcePlatform
	case "instant", "auto":
		appliedAction = normalizedAction
		decision.Type = normalizedAction
	}

	if ruleDecision.Reason != "" && appliedAction != "" {
		decision.Reason = ruleDecision.Reason
		if appliedAction == "user-restricted" {
			decision.Warning = ruleDecision.Reason
		}
	}

	return appliedAction
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
	ClaimID                int64   `json:"claim_id"`
	Status                 string  `json:"status"`
	DecisionStatus         string  `json:"decision_status,omitempty"` // auto-adjudicated
	CompensationStatus     string  `json:"compensation_status,omitempty"`
	PayoutStatus           string  `json:"payout_status,omitempty"` // processing, paid
	CustomerActionRequired bool    `json:"customer_action_required"`
	CustomerAction         string  `json:"customer_action,omitempty"`
	ApprovedAmount         *int64  `json:"approved_amount,omitempty"`
	CompensationSource     string  `json:"compensation_source,omitempty"` // merchant, rider, platform
	Reason                 string  `json:"reason"`
	Warning                *string `json:"warning,omitempty"` // 警告信息
}

const (
	submitClaimStatusAccepted                            = "accepted"
	submitClaimStatusRejected                            = "rejected"
	submitClaimStatusClosed                              = "closed"
	submitClaimStatusWaitingCustomerConfirm              = "warned_waiting_customer_confirmation"
	submitClaimDecisionStatusAutoAdjudicated             = "auto-adjudicated"
	submitClaimDecisionStatusRejected                    = "rejected"
	submitClaimCompensationStatusAwaiting                = "awaiting_compensation"
	submitClaimCompensationStatusCompensating            = "compensating"
	submitClaimCompensationStatusCompensated             = "compensated"
	submitClaimPayoutStatusProcessing                    = "processing"
	submitClaimPayoutStatusPaid                          = "paid"
	claimCustomerActionConfirmContinue                   = "confirm_continue"
	claimPayoutConfirmationActionRequestMerchantTransfer = "request_merchant_transfer"
	claimReviewNoteCustomerWithdrawn                     = "claim withdrawn by customer before compensation execution"
	claimReasonCustomerWithdrawn                         = "用户已主动撤回索赔，未进入补偿执行"
)

func claimWasWithdrawnByCustomer(claim db.Claim) bool {
	return claim.Status == db.ClaimStatusWithdrawn
}

func claimAwaitingCustomerConfirmation(claim db.Claim) bool {
	if !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 || claim.PaidAt.Valid {
		return false
	}

	return claim.Status == db.ClaimStatusWaitingCustomerConfirmation
}

func isDuplicateOrderClaimError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != db.UniqueViolation {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, "claims_order_id_unique")
}

// claimLegacyCompensating keeps pre-confirmation rollout rows readable as processing.
// New compensation writes must not treat auto-approved as an executable state.
func claimLegacyCompensating(claim db.Claim) bool {
	if !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 || claim.PaidAt.Valid {
		return false
	}

	return claim.Status == db.ClaimStatusAutoApproved
}

func userClaimReason(claim db.Claim) string {
	if claimWasWithdrawnByCustomer(claim) {
		return claimReasonCustomerWithdrawn
	}
	if claim.Status == db.ClaimStatusRejected {
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
	if claimWasWithdrawnByCustomer(claim) && claim.ReviewedAt.Valid {
		return &claim.ReviewedAt.Time
	}
	if claim.PaidAt.Valid {
		return &claim.PaidAt.Time
	}
	if claim.ReviewedAt.Valid {
		return &claim.ReviewedAt.Time
	}
	return nil
}

func userClaimLifecycleFromClaim(claim db.Claim) (string, string, string, string, bool, string) {
	if claimWasWithdrawnByCustomer(claim) {
		decisionStatus := ""
		if claim.ApprovedAmount.Valid && claim.ApprovedAmount.Int64 > 0 {
			decisionStatus = submitClaimDecisionStatusAutoAdjudicated
		}
		return submitClaimStatusClosed, decisionStatus, "", "", false, ""
	}

	if claim.Status == db.ClaimStatusRejected {
		return submitClaimStatusRejected, submitClaimDecisionStatusRejected, "", "", false, ""
	}

	status := submitClaimStatusAccepted
	decisionStatus := ""
	compensationStatus := ""
	payoutStatus := ""
	customerActionRequired := false
	customerAction := ""

	if claim.ApprovedAmount.Valid && claim.ApprovedAmount.Int64 > 0 {
		decisionStatus = submitClaimDecisionStatusAutoAdjudicated
		if claim.PaidAt.Valid {
			compensationStatus = submitClaimCompensationStatusCompensated
			payoutStatus = submitClaimPayoutStatusPaid
		} else if claim.Status == db.ClaimStatusApproved || claimLegacyCompensating(claim) {
			compensationStatus = submitClaimCompensationStatusCompensating
			payoutStatus = submitClaimPayoutStatusProcessing
		} else if claimAwaitingCustomerConfirmation(claim) {
			status = submitClaimStatusWaitingCustomerConfirm
			compensationStatus = submitClaimCompensationStatusAwaiting
			customerActionRequired = true
			customerAction = claimCustomerActionConfirmContinue
		}
	}

	return status, decisionStatus, compensationStatus, payoutStatus, customerActionRequired, customerAction
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
			// 规则引擎故障时仍继续 deterministic claims adjudication，不允许回落到平台兜底模式。
			log.Error().Err(err).
				Int64("order_id", req.OrderID).
				Int64("user_id", authPayload.UserID).
				Msg("Rules engine evaluation failed, continuing deterministic claims adjudication")
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

	claimFinalAdjudicatorEnabled := false
	claimFinalAdjudicatorConfig := algorithm.DefaultClaimFinalAdjudicatorConfig()
	claimFinalRegionID := int64(0)
	if server.config.ClaimFinalAdjudicatorEnabled {
		if merchant, err := server.store.GetMerchant(ctx, order.MerchantID); err == nil {
			claimFinalRegionID = merchant.RegionID
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant for claim final adjudicator: %w", err)))
			return
		}
		var enabled bool
		claimFinalAdjudicatorConfig, enabled, err = loadClaimFinalAdjudicatorConfig(ctx, server.store, claimFinalRegionID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		claimFinalAdjudicatorEnabled = enabled
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

	if claimFinalAdjudicatorEnabled {
		windowEnd := time.Now()
		start7d := pgtype.Date{Time: windowEnd.AddDate(0, 0, -7), Valid: true}
		start30d := pgtype.Date{Time: windowEnd.AddDate(0, 0, -30), Valid: true}
		endDate := pgtype.Date{Time: windowEnd, Valid: true}
		userStats, err := loadClaimFinalPartyStats(ctx, server.store, "user", authPayload.UserID, start7d, start30d, endDate)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		addressID := resolveClaimAddressID(order)
		if err := enrichClaimFinalUserBehaviorStats(ctx, server.store, &userStats, req.DeviceFingerprint, addressID, windowEnd); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchantStats, err := loadClaimFinalPartyStats(ctx, server.store, "merchant", order.MerchantID, start7d, start30d, endDate)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := enrichClaimFinalLiabilityStats(ctx, server.store, &merchantStats, windowEnd); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		var riderStats *algorithm.PartyWindowStats
		if delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID); err == nil && delivery.RiderID.Valid {
			loaded, err := loadClaimFinalPartyStats(ctx, server.store, "rider", delivery.RiderID.Int64, start7d, start30d, endDate)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			if err := enrichClaimFinalLiabilityStats(ctx, server.store, &loaded, windowEnd); err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			riderStats = &loaded
		}

		adjudication, err := algorithm.NewClaimFinalAdjudicator(claimFinalAdjudicatorConfig).Adjudicate(algorithm.ClaimFinalAdjudicationInput{
			RegionID:  claimFinalRegionID,
			ClaimType: req.ClaimType,
			User:      userStats,
			Rider:     riderStats,
			Merchant:  merchantStats,
		})
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		scoreBreakdown, err := json.Marshal(adjudication.ScoreBreakdown)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal claim final score breakdown: %w", err)))
			return
		}
		factSnapshot, err := json.Marshal(claimFinalAdjudicatorFactSnapshot{
			OrderID:              order.ID,
			ClaimType:            req.ClaimType,
			ClaimAmount:          req.ClaimAmount,
			BaseResponsibleParty: adjudication.BaseResponsibleParty,
			ResponsibleParty:     adjudication.ResponsibleParty,
			DecisionMode:         adjudication.DecisionMode,
			CompensationSource:   adjudication.CompensationSource,
			User:                 userStats,
			Rider:                riderStats,
			Merchant:             merchantStats,
			ReasonCodes:          adjudication.ReasonCodes,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal claim final fact snapshot: %w", err)))
			return
		}
		applyClaimFinalAdjudication(decision, adjudication, scoreBreakdown, factSnapshot)
	}

	// 规则引擎结果覆盖（如需）
	if ruleDecision.Action != "" && ruleDecision.Action != "allow" && ruleDecision.Action != "alert" {
		if applied := applyClaimRuleDecisionOverride(decision, ruleDecision); applied != "" {
			decision.ScoreBreakdown = nil
			decision.FactSnapshot = nil
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

	// 生成追偿单（责任方为商户/骑手且需要追偿）
	responsibleParty := "unknown"
	recoveryTarget := ""
	recoveryRequired := false
	recoveryAmount := decision.Amount
	if ruleDecision.Meta != nil {
		if v, ok := ruleDecision.Meta["responsible_party"].(string); ok && v != "" {
			switch v {
			case "merchant", "rider", "user":
				responsibleParty = v
			}
		}
		if v, ok := ruleDecision.Meta["recovery_required"].(bool); ok {
			recoveryRequired = v
		}
		if v, ok := ruleDecision.Meta["recovery_target"].(string); ok && v != "" {
			if v == "merchant" || v == "rider" {
				recoveryTarget = v
			}
		}
		if v, ok := ruleDecision.Meta["recovery_amount"].(float64); ok {
			recoveryAmount = int64(v)
		}
		if v, ok := ruleDecision.Meta["recovery_amount"].(int64); ok {
			recoveryAmount = v
		}
	}
	if responsibleParty == "unknown" && decision.CompensationSource != "" {
		switch decision.CompensationSource {
		case algorithm.CompensationSourceMerchant:
			responsibleParty = "merchant"
		case algorithm.CompensationSourceRider:
			responsibleParty = "rider"
		case algorithm.CompensationSourcePlatform:
			if decision.Type == algorithm.DecisionModeUserRestricted || decision.BehaviorStatus == algorithm.ClaimBehaviorUserRestricted {
				responsibleParty = "user"
			}
		}
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
		if isDuplicateOrderClaimError(err) {
			ctx.JSON(http.StatusConflict, errorResponse(ErrOrderAlreadyHasClaim))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create claim with decision: %w", err)))
		return
	}

	// 构造响应
	status, decisionStatus, compensationStatus, payoutStatus, customerActionRequired, customerAction := userClaimLifecycleFromClaim(*claim)
	resp := SubmitClaimResponse{
		ClaimID:                claim.ID,
		Status:                 status,
		DecisionStatus:         decisionStatus,
		CompensationStatus:     compensationStatus,
		PayoutStatus:           payoutStatus,
		CustomerActionRequired: customerActionRequired,
		CustomerAction:         customerAction,
		CompensationSource:     decision.CompensationSource,
		Reason:                 decision.Reason,
	}

	if decision.Approved {
		resp.ApprovedAmount = &decision.Amount
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
		"compensation_status": resp.CompensationStatus,
		"compensation_source": resp.CompensationSource,
		"requested_amount":    req.ClaimAmount,
		"approved_amount":     decision.Amount,
		"auto_adjudicated":    decision.Approved,
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

	ctx.JSON(http.StatusOK, resp)
}

// ReportFoodSafetyRequest 上报食安请求
type ReportFoodSafetyRequest struct {
	MerchantID    int64  `json:"merchant_id" binding:"required,min=1"`
	OrderID       int64  `json:"order_id" binding:"required,min=1"`
	IncidentType  string `json:"incident_type" binding:"required,oneof=foreign-object contamination expired"`
	Description   string `json:"description" binding:"required,min=10,max=1000"`
	SeverityLevel int16  `json:"severity_level" binding:"required,min=1,max=5"`
}

// ReportFoodSafetyResponse 食安上报响应
type ReportFoodSafetyResponse struct {
	IncidentID        int64  `json:"incident_id"`
	CaseID            *int64 `json:"case_id,omitempty"`
	MerchantSuspended bool   `json:"merchant_suspended"`
	SuspendDuration   *int   `json:"suspend_duration,omitempty"` // 小时
	Message           string `json:"message"`
}

func canReportFoodSafetyForOrder(order db.Order) bool {
	switch order.OrderType {
	case OrderTypeTakeout:
		return order.Status == OrderStatusRiderDelivered ||
			order.Status == OrderStatusUserDelivered ||
			order.Status == OrderStatusCompleted
	case OrderTypeTakeaway, OrderTypeDineIn, OrderTypeReservation:
		return order.Status == OrderStatusCompleted
	default:
		return order.Status == OrderStatusCompleted
	}
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证严重程度
	if req.SeverityLevel < 1 || req.SeverityLevel > 5 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("severity_level must be between 1 and 5")))
		return
	}

	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOrderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get order: %w", err)))
		return
	}

	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrOrderNotOwned))
		return
	}

	if order.MerchantID != req.MerchantID {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order does not belong to the provided merchant")))
		return
	}

	if !canReportFoodSafetyForOrder(order) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("food safety reports require a fulfilled order")))
		return
	}

	orderItems, err := server.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list order items for food safety report %d: %w", order.ID, err)))
		return
	}

	orderSnapshot, merchantSnapshot, riderSnapshot, err := buildFoodSafetyIncidentSnapshots(order, orderItems, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("build food safety incident snapshots for order %d: %w", order.ID, err)))
		return
	}
	productKey, productLabel := buildFoodSafetyPrimaryProduct(orderItems)

	// 创建食安处理器
	handler := algorithm.NewFoodSafetyHandler(server.store, server.wsHub)

	// 评估食安举报
	result, err := handler.EvaluateFoodSafetyReport(
		ctx,
		algorithm.FoodSafetyReportInput{
			ReporterUserID: authPayload.UserID,
			MerchantID:     req.MerchantID,
			Order:          order,
		},
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("evaluate food safety report for order %d: %w", req.OrderID, err)))
		return
	}

	txResult, err := server.store.ReportFoodSafetyIncidentTx(ctx, db.ReportFoodSafetyIncidentTxParams{
		CreateFoodSafetyIncidentParams: db.CreateFoodSafetyIncidentParams{
			UserID:           authPayload.UserID,
			MerchantID:       order.MerchantID,
			OrderID:          order.ID,
			IncidentType:     req.IncidentType,
			Description:      req.Description,
			OrderSnapshot:    orderSnapshot,
			MerchantSnapshot: merchantSnapshot,
			RiderSnapshot:    riderSnapshot,
			Status:           foodSafetyIncidentStatusFromResult(result),
			CreatedAt:        time.Now(),
		},
		ProductKey:                productKey,
		ProductLabel:              productLabel,
		ShouldCircuitBreak:        result.ShouldCircuitBreak,
		CircuitBreakReason:        fmt.Sprintf("同商户1小时内多名顾客食安举报触发熔断（订单ID: %d）", order.ID),
		CircuitBreakDurationHours: result.DurationHours,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("report food safety incident for order %d: %w", req.OrderID, err)))
		return
	}

	if result.ShouldCircuitBreak && txResult.OpenedNewCase {
		if server.wsHub != nil {
			handler.NotifyMerchantCircuitBreak(order.MerchantID)
		}
		server.dispatchFoodSafetySuspensionFollowUps(ctx, txResult)
	}

	merchantSuspended := txResult.Case != nil && txResult.Case.Status != "resolved"
	responseMessage := result.Message
	if txResult.ReusedExistingIncident {
		responseMessage = "当前订单已有有效食安上报，已复用现有记录"
	}

	resp := ReportFoodSafetyResponse{
		IncidentID:        txResult.Incident.ID,
		MerchantSuspended: merchantSuspended,
		Message:           responseMessage,
	}
	if txResult.Case != nil {
		resp.CaseID = &txResult.Case.ID
	}

	if merchantSuspended && txResult.OpenedNewCase {
		resp.SuspendDuration = &result.DurationHours
	}

	ctx.JSON(http.StatusOK, resp)
}

func (server *Server) dispatchFoodSafetySuspensionFollowUps(ctx *gin.Context, txResult db.ReportFoodSafetyIncidentTxResult) {
	if server.taskDistributor == nil || txResult.Case == nil {
		return
	}

	merchant, err := server.store.GetMerchant(ctx, txResult.Case.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", txResult.Case.MerchantID).Msg("get merchant for food safety follow-up failed")
		return
	}

	server.enqueueFoodSafetyNotification(ctx, &worker.SendNotificationPayload{
		UserID:            merchant.OwnerUserID,
		Type:              "food_safety",
		Title:             "食安停业告警",
		Content:           "您的店铺已因顾客食安上报触发停业，请立即停止外卖履约并联系平台处理。",
		RelatedType:       "merchant",
		RelatedID:         merchant.ID,
		IgnorePreferences: true,
		ExtraData: map[string]any{
			"case_id":        txResult.Case.ID,
			"merchant_id":    merchant.ID,
			"scene":          "food_safety_suspension",
			"trigger_reason": txResult.Case.TriggerReason,
		},
	})

	for _, reservation := range txResult.AffectedReservations {
		server.scheduleFoodSafetyReservationAlert(ctx, reservation)
	}

	for _, pausedOrder := range txResult.AffectedTakeoutOrders {
		server.enqueueFoodSafetyNotification(ctx, &worker.SendNotificationPayload{
			UserID:            pausedOrder.UserID,
			Type:              "food_safety",
			Title:             "订单暂停提醒",
			Content:           "商户因食安事件暂停营业，您的外卖订单已暂停履约。请等待商家或平台联系；退款需由您或商家主动发起。",
			RelatedType:       "order",
			RelatedID:         pausedOrder.ID,
			IgnorePreferences: true,
			ExtraData: map[string]any{
				"order_id":    pausedOrder.ID,
				"merchant_id": pausedOrder.MerchantID,
				"scene":       "food_safety_order_paused",
			},
		})

		delivery, err := server.store.GetDeliveryByOrderID(ctx, pausedOrder.ID)
		if err != nil || !delivery.RiderID.Valid {
			continue
		}

		rider, err := server.store.GetRider(ctx, delivery.RiderID.Int64)
		if err != nil {
			log.Error().Err(err).Int64("order_id", pausedOrder.ID).Int64("rider_id", delivery.RiderID.Int64).Msg("get rider for food safety pause notification failed")
			continue
		}

		server.enqueueFoodSafetyNotification(ctx, &worker.SendNotificationPayload{
			UserID:            rider.UserID,
			Type:              "food_safety",
			Title:             "订单暂停提醒",
			Content:           "关联商户因食安事件暂停营业，请立即停止继续履约并等待平台或商家进一步处理。",
			RelatedType:       "delivery",
			RelatedID:         delivery.ID,
			IgnorePreferences: true,
			ExtraData: map[string]any{
				"order_id":    pausedOrder.ID,
				"delivery_id": delivery.ID,
				"merchant_id": pausedOrder.MerchantID,
				"scene":       "food_safety_order_paused",
			},
		})
	}
}

func (server *Server) scheduleFoodSafetyReservationAlert(ctx *gin.Context, reservation db.TableReservation) {
	if server.taskDistributor == nil {
		return
	}

	reservationTime := time.Date(
		reservation.ReservationDate.Time.Year(), reservation.ReservationDate.Time.Month(), reservation.ReservationDate.Time.Day(),
		int(reservation.ReservationTime.Microseconds/1000000/3600), int((reservation.ReservationTime.Microseconds/1000000%3600)/60), 0, 0, time.Local,
	)
	alertTime := reservationTime.Add(-3 * time.Hour)
	options := []asynq.Option{
		asynq.Queue(worker.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Unique(45 * 24 * time.Hour),
	}
	if alertTime.After(time.Now()) {
		options = append(options, asynq.ProcessAt(alertTime))
	} else {
		options = append(options, asynq.ProcessIn(0))
	}

	if err := server.taskDistributor.DistributeTaskReservationFoodSafetyAlert(ctx, &worker.PayloadReservationFoodSafetyAlert{
		ReservationID: reservation.ID,
	}, options...); err != nil {
		log.Error().Err(err).Int64("reservation_id", reservation.ID).Msg("enqueue reservation food safety alert failed")
	}
}

func (server *Server) enqueueFoodSafetyNotification(ctx *gin.Context, payload *worker.SendNotificationPayload) {
	if server.taskDistributor == nil {
		return
	}
	if err := server.taskDistributor.DistributeTaskSendNotification(
		ctx,
		payload,
		asynq.Queue(worker.QueueDefault),
		asynq.MaxRetry(3),
	); err != nil {
		log.Error().Err(err).Int64("user_id", payload.UserID).Str("title", payload.Title).Msg("enqueue food safety notification failed")
	}
}

func buildFoodSafetyPrimaryProduct(items []db.ListOrderItemsWithDishByOrderRow) (string, string) {
	keys := make([]string, 0, len(items))
	labels := make([]string, 0, len(items))

	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		switch {
		case item.DishID.Valid:
			keys = appendUniqueFoodSafetyStrings(keys, fmt.Sprintf("dish:%d", item.DishID.Int64))
		case item.ComboID.Valid:
			keys = appendUniqueFoodSafetyStrings(keys, fmt.Sprintf("combo:%d", item.ComboID.Int64))
		case name != "":
			keys = appendUniqueFoodSafetyStrings(keys, "name:"+strings.ToLower(name))
		}
		if name != "" {
			labels = appendUniqueFoodSafetyStrings(labels, name)
		}
	}

	if len(keys) == 1 {
		label := ""
		if len(labels) > 0 {
			label = labels[0]
		}
		return keys[0], label
	}

	if len(labels) == 0 {
		return "", ""
	}

	return "", strings.Join(labels, "、")
}

func appendUniqueFoodSafetyStrings(values []string, candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return values
	}
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func foodSafetyIncidentStatusFromResult(result *algorithm.FoodSafetyCheckResult) string {
	if result != nil && result.IsMalicious {
		return "rejected"
	}
	return "reported"
}

func buildFoodSafetyIncidentSnapshots(order db.Order, items []db.ListOrderItemsWithDishByOrderRow, reporterUserID int64) ([]byte, []byte, []byte, error) {
	var addressID *int64
	if order.AddressID.Valid {
		addressID = &order.AddressID.Int64
	}

	itemPayload := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		payload := map[string]interface{}{
			"name":       item.Name,
			"quantity":   item.Quantity,
			"subtotal":   item.Subtotal,
			"dish_id":    nil,
			"combo_id":   nil,
			"created_at": item.CreatedAt,
		}
		if item.DishID.Valid {
			payload["dish_id"] = item.DishID.Int64
		}
		if item.ComboID.Valid {
			payload["combo_id"] = item.ComboID.Int64
		}
		itemPayload = append(itemPayload, payload)
	}

	orderSnapshot, err := json.Marshal(map[string]interface{}{
		"order_id":         order.ID,
		"order_no":         order.OrderNo,
		"user_id":          order.UserID,
		"reporter_user_id": reporterUserID,
		"merchant_id":      order.MerchantID,
		"order_type":       order.OrderType,
		"address_id":       addressID,
		"status":           order.Status,
		"total_amount":     order.TotalAmount,
		"delivery_fee":     order.DeliveryFee,
		"created_at":       order.CreatedAt,
		"completed_at":     order.CompletedAt,
		"items":            itemPayload,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	merchantSnapshot, err := json.Marshal(map[string]interface{}{
		"merchant_id": order.MerchantID,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	riderSnapshot, err := json.Marshal(map[string]interface{}{})
	if err != nil {
		return nil, nil, nil, err
	}

	return orderSnapshot, merchantSnapshot, riderSnapshot, nil
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

// ==================== 用户索赔查询 API ====================

type userClaimResponse struct {
	ID                         int64      `json:"id"`
	OrderID                    int64      `json:"order_id"`
	ClaimType                  string     `json:"claim_type"`
	Description                string     `json:"description"`
	ClaimAmount                int64      `json:"claim_amount"`
	ApprovedAmount             *int64     `json:"approved_amount,omitempty"`
	Status                     string     `json:"status"`
	DecisionStatus             string     `json:"decision_status,omitempty"` // auto-adjudicated, rejected
	CompensationStatus         string     `json:"compensation_status,omitempty"`
	PayoutStatus               string     `json:"payout_status,omitempty"` // processing, paid
	PayoutConfirmationRequired bool       `json:"payout_confirmation_required,omitempty"`
	PayoutConfirmationAction   string     `json:"payout_confirmation_action,omitempty"` // request_merchant_transfer
	CustomerActionRequired     bool       `json:"customer_action_required"`
	CustomerAction             string     `json:"customer_action,omitempty"`
	Reason                     string     `json:"reason,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	ProcessedAt                *time.Time `json:"processed_at,omitempty"`
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
	status, decisionStatus, compensationStatus, payoutStatus, customerActionRequired, customerAction := userClaimLifecycleFromClaim(claim)
	resp := userClaimResponse{
		ID:                     claim.ID,
		OrderID:                claim.OrderID,
		ClaimType:              claim.ClaimType,
		Description:            claim.Description,
		ClaimAmount:            claim.ClaimAmount,
		Status:                 status,
		DecisionStatus:         decisionStatus,
		CompensationStatus:     compensationStatus,
		PayoutStatus:           payoutStatus,
		CustomerActionRequired: customerActionRequired,
		CustomerAction:         customerAction,
		Reason:                 userClaimReason(claim),
		CreatedAt:              claim.CreatedAt,
		ProcessedAt:            userClaimProcessedAt(claim),
	}

	if claim.ApprovedAmount.Valid {
		resp.ApprovedAmount = &claim.ApprovedAmount.Int64
	}

	return resp
}

type claimPayoutConfirmationResponse struct {
	MchID   string `json:"mch_id"`
	AppID   string `json:"app_id"`
	Package string `json:"package"`
}

type claimPayoutConfirmationActionDetail struct {
	ClaimID           int64  `json:"claim_id"`
	UserID            int64  `json:"user_id"`
	RecoveryDisputeID int64  `json:"recovery_dispute_id"`
	TransferState     string `json:"transfer_state"`
	PackageInfo       string `json:"package_info"`
}

type claimPayoutConfirmationState struct {
	Required    bool
	PackageInfo string
}

func (server *Server) claimPayoutConfirmationState(ctx context.Context, claim db.Claim) (claimPayoutConfirmationState, error) {
	if claim.PaidAt.Valid || claim.Status != db.ClaimStatusApproved || !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 {
		return claimPayoutConfirmationState{}, nil
	}

	decision, err := server.store.GetLatestBehaviorDecisionByClaimID(ctx, pgtype.Int8{Int64: claim.ID, Valid: true})
	if err != nil {
		if isNotFoundError(err) {
			return claimPayoutConfirmationState{}, nil
		}
		return claimPayoutConfirmationState{}, fmt.Errorf("get latest behavior decision for claim %d: %w", claim.ID, err)
	}

	actions, err := server.store.ListBehaviorActionsByDecision(ctx, decision.ID)
	if err != nil {
		return claimPayoutConfirmationState{}, fmt.Errorf("list behavior actions for decision %d: %w", decision.ID, err)
	}

	for _, action := range actions {
		if action.ActionType != "payout" || action.TargetEntity != "user" || action.Status != "running" {
			continue
		}

		var detail claimPayoutConfirmationActionDetail
		if err := json.Unmarshal(action.Detail, &detail); err != nil {
			return claimPayoutConfirmationState{}, fmt.Errorf("decode claim payout action %d detail: %w", action.ID, err)
		}
		if detail.ClaimID != claim.ID || detail.UserID != claim.UserID || detail.RecoveryDisputeID > 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(detail.TransferState), wechatcontracts.DirectMerchantTransferStateWaitUserConfirm) && strings.TrimSpace(detail.PackageInfo) != "" {
			return claimPayoutConfirmationState{Required: true, PackageInfo: strings.TrimSpace(detail.PackageInfo)}, nil
		}
	}

	return claimPayoutConfirmationState{}, nil
}

func (server *Server) enqueueClaimCompensationActions(ctx context.Context, result db.CreateClaimCompensationTxResult) error {
	if server.taskDistributor == nil {
		return fmt.Errorf("task distributor unavailable")
	}

	if result.RestrictionAction != nil {
		if err := server.taskDistributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&worker.ClaimBehaviorActionPayload{ActionID: result.RestrictionAction.ID},
			asynq.Queue(worker.QueueCritical),
			asynq.MaxRetry(10),
		); err != nil {
			return fmt.Errorf("enqueue claim restriction action %d: %w", result.RestrictionAction.ID, err)
		}
	}

	if result.RecoveryAction != nil {
		if err := server.taskDistributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&worker.ClaimBehaviorActionPayload{ActionID: result.RecoveryAction.ID},
			asynq.Queue(worker.QueueCritical),
			asynq.MaxRetry(10),
		); err != nil {
			return fmt.Errorf("enqueue claim recovery action %d: %w", result.RecoveryAction.ID, err)
		}
	}

	if result.NotificationAction != nil {
		if err := server.taskDistributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&worker.ClaimBehaviorActionPayload{ActionID: result.NotificationAction.ID},
			asynq.Queue(worker.QueueDefault),
			asynq.MaxRetry(5),
		); err != nil {
			return fmt.Errorf("enqueue claim notification action %d: %w", result.NotificationAction.ID, err)
		}
	}

	if result.PayoutAction != nil {
		if err := server.taskDistributor.DistributeTaskClaimPayout(
			ctx,
			&worker.ClaimPayoutPayload{ActionID: result.PayoutAction.ID},
			asynq.Queue(worker.QueueCritical),
			asynq.MaxRetry(10),
		); err != nil {
			return fmt.Errorf("enqueue claim payout action %d: %w", result.PayoutAction.ID, err)
		}
	}

	return nil
}

// WithdrawClaim 撤回尚未进入补偿执行的索赔
// @Summary 撤回索赔
// @Description 用户撤回已完成判责但尚未进入补偿执行的索赔
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Success 200 {object} userClaimResponse "索赔状态"
// @Failure 400 {object} ErrorResponse "无效的索赔ID"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "该索赔不属于当前用户"
// @Failure 404 {object} ErrorResponse "索赔不存在"
// @Failure 409 {object} ErrorResponse "当前索赔状态不允许撤回"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims/{id}/withdraw [post]
// @Security BearerAuth
func (server *Server) WithdrawClaim(ctx *gin.Context) {
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

	if claim.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrClaimNotOwned))
		return
	}

	if claimWasWithdrawnByCustomer(claim) {
		ctx.JSON(http.StatusOK, newUserClaimResponse(claim))
		return
	}

	if claim.Status == db.ClaimStatusApproved || claim.PaidAt.Valid || claim.Status == db.ClaimStatusRejected || !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 || !claimAwaitingCustomerConfirmation(claim) {
		ctx.JSON(http.StatusConflict, errorResponse(ErrClaimCannotBeWithdrawn))
		return
	}

	withdrewAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	updatedClaim, err := server.store.UpdateClaimStatusIfCurrent(ctx, db.UpdateClaimStatusIfCurrentParams{
		ID:            claim.ID,
		CurrentStatus: claim.Status,
		Status:        db.ClaimStatusWithdrawn,
		ReviewNotes:   pgtype.Text{String: claimReviewNoteCustomerWithdrawn, Valid: true},
		ReviewedAt:    withdrewAt,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusConflict, errorResponse(ErrClaimCannotBeWithdrawn))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("withdraw claim %d: %w", claim.ID, err)))
		return
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "customer",
		Action:      "user_claim_withdrawn",
		TargetType:  "claim",
		TargetID:    &claim.ID,
		Metadata: map[string]any{
			"claim_id": claim.ID,
			"order_id": claim.OrderID,
			"status":   submitClaimStatusClosed,
		},
	})

	ctx.JSON(http.StatusOK, newUserClaimResponse(updatedClaim))
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

	resp := newUserClaimResponse(claim)
	confirmationState, err := server.claimPayoutConfirmationState(ctx, claim)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("load claim payout confirmation state for claim %d: %w", claim.ID, err)))
		return
	}
	if confirmationState.Required {
		resp.PayoutConfirmationRequired = true
		resp.PayoutConfirmationAction = claimPayoutConfirmationActionRequestMerchantTransfer
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetClaimPayoutConfirmation 获取微信确认收款参数
// @Summary 获取索赔赔付确认收款参数
// @Description 仅当当前用户自己的索赔赔付处于微信 WAIT_USER_CONFIRM 状态时，返回小程序 wx.requestMerchantTransfer 所需参数
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Success 200 {object} claimPayoutConfirmationResponse "确认收款参数"
// @Failure 400 {object} ErrorResponse "无效的索赔ID"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "该索赔不属于当前用户"
// @Failure 404 {object} ErrorResponse "索赔不存在"
// @Failure 409 {object} ErrorResponse "当前赔付不需要微信确认收款"
// @Failure 503 {object} ErrorResponse "赔付服务不可用"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims/{id}/payout-confirmation [get]
// @Security BearerAuth
func (server *Server) GetClaimPayoutConfirmation(ctx *gin.Context) {
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

	if claim.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrClaimNotOwned))
		return
	}

	confirmationState, err := server.claimPayoutConfirmationState(ctx, claim)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("load claim payout confirmation state for claim %d: %w", claim.ID, err)))
		return
	}
	if !confirmationState.Required {
		ctx.JSON(http.StatusConflict, errorResponse(ErrClaimPayoutConfirmationUnavailable))
		return
	}
	if server.transferClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrClaimPayoutServiceUnavailable))
		return
	}

	resp := claimPayoutConfirmationResponse{
		MchID:   strings.TrimSpace(server.transferClient.GetMchID()),
		AppID:   strings.TrimSpace(server.transferClient.GetAppID()),
		Package: confirmationState.PackageInfo,
	}
	if err := wechatcontracts.ValidateRequestMerchantTransferParams(&wechatcontracts.RequestMerchantTransferParams{
		MchID:   resp.MchID,
		AppID:   resp.AppID,
		Package: resp.Package,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("build claim payout requestMerchantTransfer params for claim %d: %w", claim.ID, err)))
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// ConfirmContinueClaim 确认继续索赔并进入补偿阶段
// @Summary 确认继续索赔
// @Description 用户确认继续已完成判责的索赔，系统进入补偿执行阶段
// @Tags 索赔管理
// @Accept json
// @Produce json
// @Param id path int true "索赔ID"
// @Success 200 {object} userClaimResponse "索赔状态"
// @Failure 400 {object} ErrorResponse "无效的索赔ID"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "该索赔不属于当前用户"
// @Failure 404 {object} ErrorResponse "索赔不存在"
// @Failure 409 {object} ErrorResponse "当前索赔状态不允许继续"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/claims/{id}/confirm-continue [post]
// @Security BearerAuth
func (server *Server) ConfirmContinueClaim(ctx *gin.Context) {
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

	if claim.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrClaimNotOwned))
		return
	}

	if claimWasWithdrawnByCustomer(claim) || claim.Status == db.ClaimStatusRejected || !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 {
		ctx.JSON(http.StatusConflict, errorResponse(ErrClaimCannotContinue))
		return
	}

	if claim.PaidAt.Valid {
		ctx.JSON(http.StatusOK, newUserClaimResponse(claim))
		return
	}

	if claim.Status != db.ClaimStatusApproved && !claimAwaitingCustomerConfirmation(claim) {
		ctx.JSON(http.StatusConflict, errorResponse(ErrClaimCannotContinue))
		return
	}

	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user for claim payout real name: %w", err)))
		return
	}
	if !logic.ClaimPayoutRealNameReady(user.FullName) {
		ctx.JSON(http.StatusConflict, errorResponse(ErrClaimPayoutRealNameRequired))
		return
	}

	result, err := server.store.CreateClaimCompensationTx(ctx, db.CreateClaimCompensationTxParams{ClaimID: claim.ID})
	if err != nil {
		if db.IsClaimCompensationNotEligible(err) {
			ctx.JSON(http.StatusConflict, errorResponse(ErrClaimCannotContinue))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create claim compensation tx: %w", err)))
		return
	}

	enqueueDeferred := false
	if err := server.enqueueClaimCompensationActions(ctx, result); err != nil {
		enqueueDeferred = true
		log.Warn().Err(err).
			Int64("claim_id", claim.ID).
			Bool("payout_action_created", result.PayoutAction != nil).
			Bool("recovery_action_created", result.RecoveryAction != nil).
			Bool("restriction_action_created", result.RestrictionAction != nil).
			Bool("notification_action_created", result.NotificationAction != nil).
			Msg("failed to enqueue claim compensation actions after compensation state was persisted")
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "customer",
		Action:      "user_claim_continue_confirmed",
		TargetType:  "claim",
		TargetID:    &claim.ID,
		Metadata: map[string]any{
			"claim_id":               claim.ID,
			"compensation_triggered": result.PayoutAction != nil,
			"recovery_created":       result.RecoveryAction != nil,
			"restriction_created":    result.RestrictionAction != nil,
			"dispatch_deferred":      enqueueDeferred,
		},
	})

	ctx.JSON(http.StatusOK, newUserClaimResponse(result.Claim))
}
