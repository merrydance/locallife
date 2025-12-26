package api

import (
	"database/sql"
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
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
)

// UserProfileDTO 用户信用画像
type UserProfileDTO struct {
	ID              int64  `json:"id"`
	UserID          int64  `json:"user_id"`
	Role            string `json:"role"`
	TrustScore      int16  `json:"trust_score"`
	TotalOrders     int32  `json:"total_orders"`
	CompletedOrders int32  `json:"completed_orders"`
	TotalClaims     int32  `json:"total_claims"`
	ViolationCount  int32  `json:"violation_count"`
}

// MerchantProfileDTO 商户信用画像
type MerchantProfileDTO struct {
	ID                  int64 `json:"id"`
	MerchantID          int64 `json:"merchant_id"`
	TrustScore          int16 `json:"trust_score"`
	TotalOrders         int32 `json:"total_orders"`
	TotalSales          int64 `json:"total_sales"`
	CompletedOrders     int32 `json:"completed_orders"`
	TotalClaims         int32 `json:"total_claims"`
	ForeignObjectClaims int32 `json:"foreign_object_claims"`
	FoodSafetyIncidents int32 `json:"food_safety_incidents"`
}

// RiderProfileDTO 骑手信用画像
type RiderProfileDTO struct {
	ID                   int64 `json:"id"`
	RiderID              int64 `json:"rider_id"`
	TrustScore           int16 `json:"trust_score"`
	TotalDeliveries      int32 `json:"total_deliveries"`
	CompletedDeliveries  int32 `json:"completed_deliveries"`
	OnTimeDeliveries     int32 `json:"on_time_deliveries"`
	TotalDamageIncidents int32 `json:"total_damage_incidents"`
}

// TrustScoreProfileResponse 信用分画像响应（包含三种角色）
type TrustScoreProfileResponse struct {
	Customer *UserProfileDTO    `json:"customer,omitempty"`
	Merchant *MerchantProfileDTO `json:"merchant,omitempty"`
	Rider    *RiderProfileDTO    `json:"rider,omitempty"`
}

// TrustScoreChangeResponse 信用分变更记录响应
type TrustScoreChangeResponse struct {
	ID                int64     `json:"id"`
	EntityType        string    `json:"entity_type"`
	EntityID          int64     `json:"entity_id"`
	OldScore          int16     `json:"old_score"`
	NewScore          int16     `json:"new_score"`
	ScoreChange       int16     `json:"score_change"`
	ReasonType        string    `json:"reason_type"`
	ReasonDescription string    `json:"reason_description"`
	IsAuto            bool      `json:"is_auto"`
	CreatedAt         time.Time `json:"created_at"`
}

// GetTrustScoreProfile 查询信用分画像
// @Summary 查询信用分画像
// @Description 根据角色（customer/merchant/rider）和ID查询信用分画像信息，包含信用分、历史订单统计、违规记录等
// @Tags 信用分管理
// @Accept json
// @Produce json
// @Param role path string true "角色类型" Enums(customer, merchant, rider)
// @Param id path int true "实体ID（用户ID/商户ID/骑手ID）"
// @Success 200 {object} TrustScoreProfileResponse "信用分画像（根据角色返回对应字段）"
// @Failure 400 {object} ErrorResponse "无效的角色类型或ID"
// @Failure 404 {object} ErrorResponse "画像不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/trust-score/profiles/{role}/{id} [get]
// @Security BearerAuth
func (server *Server) GetTrustScoreProfile(ctx *gin.Context) {
	role := ctx.Param("role")
	idStr := ctx.Param("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证角色
	if role != algorithm.EntityTypeCustomer &&
		role != algorithm.EntityTypeMerchant &&
		role != algorithm.EntityTypeRider {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	var resp TrustScoreProfileResponse

	switch role {
	case algorithm.EntityTypeCustomer:
		p, err := server.store.GetUserProfile(ctx, db.GetUserProfileParams{
			UserID: id,
			Role:   role,
		})
		if err != nil {
			if err == sql.ErrNoRows {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user profile: %w", err)))
			}
			return
		}
		resp.Customer = &UserProfileDTO{
			ID:              p.ID,
			UserID:          p.UserID,
			Role:            p.Role,
			TrustScore:      p.TrustScore,
			TotalOrders:     p.TotalOrders,
			CompletedOrders: p.CompletedOrders,
			TotalClaims:     p.TotalClaims,
			ViolationCount:  p.VerifiedViolations,
		}

	case algorithm.EntityTypeMerchant:
		p, err := server.store.GetMerchantProfile(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant profile: %w", err)))
			}
			return
		}
		resp.Merchant = &MerchantProfileDTO{
			ID:                  p.ID,
			MerchantID:          p.MerchantID,
			TrustScore:          p.TrustScore,
			TotalOrders:         p.TotalOrders,
			TotalSales:          p.TotalSales,
			CompletedOrders:     p.CompletedOrders,
			TotalClaims:         p.TotalClaims,
			ForeignObjectClaims: p.ForeignObjectClaims,
			FoodSafetyIncidents: p.FoodSafetyIncidents,
		}

	case algorithm.EntityTypeRider:
		p, err := server.store.GetRiderProfile(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider profile: %w", err)))
			}
			return
		}
		resp.Rider = &RiderProfileDTO{
			ID:                   p.ID,
			RiderID:              p.RiderID,
			TrustScore:           p.TrustScore,
			TotalDeliveries:      p.TotalDeliveries,
			CompletedDeliveries:  p.CompletedDeliveries,
			OnTimeDeliveries:     p.OnTimeDeliveries,
			TotalDamageIncidents: p.TotalDamageIncidents,
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// SubmitClaimRequest 提交索赔请求
type SubmitClaimRequest struct {
	OrderID        int64    `json:"order_id" binding:"required,min=1"`
	ClaimType      string   `json:"claim_type" binding:"required,oneof=foreign-object damage timeout food-safety"`
	ClaimAmount    int64    `json:"claim_amount" binding:"required,min=1,max=100000000"` // 最高100万分(1万元)
	ClaimReason    string   `json:"claim_reason" binding:"required,min=5,max=1000"`
	EvidencePhotos []string `json:"evidence_photos,omitempty" binding:"omitempty,max=10,dive,url,max=500"`
}

// SubmitClaimResponse 索赔响应
type SubmitClaimResponse struct {
	ClaimID            int64   `json:"claim_id"`
	Status             string  `json:"status"` // instant, auto, manual, evidence-required, platform-pay
	ApprovedAmount     *int64  `json:"approved_amount,omitempty"`
	CompensationSource string  `json:"compensation_source,omitempty"` // merchant, rider, platform
	Reason             string  `json:"reason"`
	RefundETA          *string `json:"refund_eta,omitempty"`     // 秒赔/自动通过时提供预计到账时间
	Warning            *string `json:"warning,omitempty"`        // 警告信息
	NeedsEvidence      bool    `json:"needs_evidence,omitempty"` // 是否需要证据
}

// SubmitClaim 提交索赔
// @Summary 提交索赔
// @Description 用户为已完成的订单提交索赔申请。系统会根据用户信用分自动评估，高信用用户可获得秒赔或自动审核，低信用用户需人工审核。
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
// @Router /v1/trust-score/claims [post]
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
		if errors.Is(err, sql.ErrNoRows) {
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

	// 4. 检查是否已存在该订单的索赔（幂等性检查）
	existingClaims, err := server.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
		UserID:    authPayload.UserID,
		CreatedAt: order.CreatedAt, // 从订单创建时间开始查
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

	// 6. 获取配送费（超时索赔用）
	var deliveryFee int64
	if req.ClaimType == algorithm.ClaimTypeTimeout || req.ClaimType == algorithm.ClaimTypeDamage {
		delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil {
			deliveryFee = delivery.DeliveryFee
		}
	}

	// 7. 检查是否提交了证据
	hasEvidence := len(req.EvidencePhotos) > 0

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
		hasEvidence,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("evaluate claim: %w", err)))
		return
	}

	// 如果需要证据但未提交，返回提示
	if decision.NeedsEvidence && !hasEvidence {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":          "需要提交证据",
			"needs_evidence": true,
			"message":        "您已被警告，请提交证据照片后重新提交索赔",
		})
		return
	}

	// 创建索赔记录
	claim, err := approver.CreateClaimWithDecision(
		ctx,
		req.OrderID,
		authPayload.UserID,
		req.ClaimType,
		req.ClaimReason,
		req.EvidencePhotos,
		decision.Amount, // 使用决策后的金额（超时可能只赔运费）
		decision,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create claim with decision: %w", err)))
		return
	}

	// 构造响应
	resp := SubmitClaimResponse{
		ClaimID:            claim.ID,
		Status:             decision.Type,
		CompensationSource: decision.CompensationSource,
		Reason:             decision.Reason,
		NeedsEvidence:      decision.NeedsEvidence,
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
// @Router /v1/trust-score/claims/{id}/review [patch]
// @Security BearerAuth
func (server *Server) ReviewClaim(ctx *gin.Context) {
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
		if err == sql.ErrNoRows {
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

	// 如果拒绝且判定为恶意索赔，扣除信用分
	if !*req.Approved {
		calculator := algorithm.NewTrustScoreCalculator(server.store, server.wsHub)
		err = calculator.DecrementMaliciousClaim(ctx, claim.UserID, claimID)
		if err != nil {
			// 记录错误但不影响返回
			fmt.Printf("Failed to decrement trust score: %v\n", err)
		}
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
	ReporterID     int64  `json:"reporter_id" binding:"required,min=1"`
	MerchantID     int64  `json:"merchant_id" binding:"required,min=1"`
	OrderID        int64  `json:"order_id" binding:"required,min=1"`
	IncidentType   string `json:"incident_type" binding:"required,oneof=foreign-object contamination expired"`
	Description    string `json:"description" binding:"required,min=10,max=1000"`
	EvidencePhotos string `json:"evidence_photos" binding:"required,url,max=500"` // 食安必须有证据
	SeverityLevel  int16  `json:"severity_level" binding:"required,min=1,max=5"`
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
// @Description 用户上报商户食品安全问题，系统将根据举报频率和用户信用分决定是否熔断商户
// @Tags 食品安全
// @Accept json
// @Produce json
// @Param request body ReportFoodSafetyRequest true "食安上报信息"
// @Success 200 {object} ReportFoodSafetyResponse "上报成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/trust-score/food-safety/report [post]
// @Security BearerAuth
func (server *Server) ReportFoodSafety(ctx *gin.Context) {
	var req ReportFoodSafetyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证严重程度
	if req.SeverityLevel < 1 || req.SeverityLevel > 5 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "severity_level must be between 1 and 5"})
		return
	}

	// 创建食安处理器
	handler := algorithm.NewFoodSafetyHandler(server.store, server.wsHub)

	// 评估食安举报
	evidencePhotos := []string{req.EvidencePhotos}
	result, err := handler.EvaluateFoodSafetyReport(
		ctx,
		req.ReporterID,
		req.MerchantID,
		evidencePhotos,
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
		EvidenceUrls:     evidencePhotos,
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

// GetTrustScoreHistory 查询信用分变更历史
// @Summary 查询信用分变更历史
// @Description 查询指定角色实体的信用分变更历史记录，包括扣分原因、扣分金额、变更时间等
// @Tags 信用分管理
// @Accept json
// @Produce json
// @Param role path string true "角色类型" Enums(customer, merchant, rider)
// @Param id path int true "实体ID"
// @Success 200 {array} TrustScoreChangeResponse "变更历史列表"
// @Failure 400 {object} ErrorResponse "无效的角色类型或ID"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/trust-score/history/{role}/{id} [get]
// @Security BearerAuth
func (server *Server) GetTrustScoreHistory(ctx *gin.Context) {
	role := ctx.Param("role")
	idStr := ctx.Param("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证角色
	if role != algorithm.EntityTypeCustomer &&
		role != algorithm.EntityTypeMerchant &&
		role != algorithm.EntityTypeRider {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	// 查询变更历史（20条记录）
	changes, err := server.store.ListEntityTrustScoreChanges(ctx, db.ListEntityTrustScoreChangesParams{
		EntityType: role,
		EntityID:   id,
		Limit:      20,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list trust score changes for %s %d: %w", role, id, err)))
		return
	}

	resp := make([]TrustScoreChangeResponse, len(changes))
	for i, c := range changes {
		resp[i] = TrustScoreChangeResponse{
			ID:                c.ID,
			EntityType:        c.EntityType,
			EntityID:          c.EntityID,
			OldScore:          c.OldScore,
			NewScore:          c.NewScore,
			ScoreChange:       c.ScoreChange,
			ReasonType:        c.ReasonType,
			ReasonDescription: c.ReasonDescription,
			IsAuto:            c.IsAuto,
			CreatedAt:         c.CreatedAt,
		}
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
// @Router /v1/trust-score/fraud/detect [post]
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

	ctx.JSON(http.StatusBadRequest, gin.H{"error": "must provide claim_id, device_fingerprint, or address_id"})
}

// SubmitRecoveryRequest 提交恢复申请请求
type SubmitRecoveryRequest struct {
	EntityType        string `json:"entity_type" binding:"required,oneof=merchant rider"` // merchant/rider
	EntityID          int64  `json:"entity_id" binding:"required,min=1"`
	CommitmentMessage string `json:"commitment_message" binding:"required,min=10,max=500"` // 改善承诺
}

// SubmitRecoveryRequest 提交恢复申请（商户/骑手）
// @Summary 提交恢复申请
// @Description 商户或骑手因信用分过低被封禁后，可提交恢复申请。系统自动给一次机会，第二次再犯永久封禁
// @Tags 信用分管理
// @Accept json
// @Produce json
// @Param request body SubmitRecoveryRequest true "恢复申请信息"
// @Success 200 {object} MessageResponse "恢复成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "已超过最大恢复次数"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/trust-score/recovery [post]
// @Security BearerAuth
func (server *Server) SubmitRecoveryRequest(ctx *gin.Context) {
	var req SubmitRecoveryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证实体类型（只支持商户和骑手）
	if req.EntityType != algorithm.EntityTypeMerchant &&
		req.EntityType != algorithm.EntityTypeRider {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "只支持商户和骑手申请恢复"})
		return
	}

	calculator := algorithm.NewTrustScoreCalculator(server.store, server.wsHub)
	err := calculator.ProcessRecoveryRequest(
		ctx,
		req.EntityType,
		req.EntityID,
		req.CommitmentMessage,
	)

	if err != nil {
		// 检查是否超过最大恢复次数
		if err.Error() == fmt.Sprintf("已超过最大恢复次数（%d次），永久封禁", algorithm.MaxRecoveryAttempts) {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error":   "已超过最大恢复次数",
				"message": "您已用完所有恢复机会，账号永久封禁",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("process recovery request for %s %d: %w", req.EntityType, req.EntityID, err)))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "恢复申请已批准，信用分已恢复，这是您的机会，请遵守规则",
		"status":  "approved",
	})
}

// SubmitAppealRequest 提交申诉请求
type SubmitAppealRequest struct {
	EntityType   string `json:"entity_type" binding:"required,oneof=customer merchant rider"`
	EntityID     int64  `json:"entity_id" binding:"required,min=1"`
	AppealReason string `json:"appeal_reason" binding:"required,min=10,max=1000"`
	Evidence     string `json:"evidence,omitempty" binding:"omitempty,url,max=500"`
}

// SubmitAppeal 提交申诉
// @Summary 提交申诉
// @Description 提交信用分申诉，系统采用信用驱动的自动化处理，不设人工申诉通道。信用分将根据后续正常行为自动恢复
// @Tags 信用分管理
// @Accept json
// @Produce json
// @Param request body SubmitAppealRequest true "申诉信息"
// @Success 200 {object} MessageResponse "申诉已记录"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Router /v1/trust-score/appeals [post]
// @Security BearerAuth
func (server *Server) SubmitAppeal(ctx *gin.Context) {
	var req SubmitAppealRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证实体类型
	if req.EntityType != algorithm.EntityTypeCustomer &&
		req.EntityType != algorithm.EntityTypeMerchant &&
		req.EntityType != algorithm.EntityTypeRider {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_type"})
		return
	}

	// 本系统采用算法驱动的信用体系，不设人工申诉通道
	// 信用分会根据后续正常行为自动恢复
	ctx.JSON(http.StatusOK, gin.H{
		"message": "感谢您的反馈。本平台采用智能信用系统，信用分将根据您后续的正常经营/消费行为自动恢复。建议：1）保持良好的订单完成率；2）减少索赔纠纷；3）获取正面评价。",
		"status":  "noted",
		"tips": []string{
			"连续7天无负面行为可恢复10分",
			"每完成10笔正常订单可恢复5分",
			"获得5星好评可恢复2分",
		},
	})
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
// @Router /v1/trust-score/merchants/{id}/suspend [patch]
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "merchant_id mismatch"})
		return
	}

	handler := algorithm.NewFoodSafetyHandler(server.store, server.wsHub)
	err = handler.CircuitBreakMerchant(ctx, merchantID, req.Reason, req.DurationHours)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("circuit break merchant %d: %w", merchantID, err)))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("商户 %d 已熔断 %d 小时", merchantID, req.DurationHours),
	})
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
		if errors.Is(err, sql.ErrNoRows) {
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

	// 更新商户状态为正常
	_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     merchantID,
		Status: "active",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("resume merchant %d: %w", merchantID, err)))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("商户 %d 已恢复上线", merchantID),
	})
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
		if errors.Is(err, sql.ErrNoRows) {
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

	ctx.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("骑手 %d 已暂停 %d 小时", riderID, req.DurationHours),
		"reason":         req.Reason,
		"duration_hours": req.DurationHours,
	})
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
		if errors.Is(err, sql.ErrNoRows) {
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

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("骑手 %d 已恢复上线", riderID),
	})
}

// ==================== 用户索赔查询 API ====================

type claimResponse struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	OrderID        int64      `json:"order_id"`
	ClaimType      string     `json:"claim_type"`
	Description    string     `json:"description"`
	EvidenceURLs   []string   `json:"evidence_urls,omitempty"`
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

	// 解析证据URL数组
	if len(claim.EvidenceUrls) > 0 {
		resp.EvidenceURLs = claim.EvidenceUrls
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

	offset := int32((page - 1) * pageSize)

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

	var response []claimResponse
	for _, c := range claims {
		response = append(response, newClaimResponse(c))
	}

	if response == nil {
		response = []claimResponse{}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"claims":    response,
		"page":      page,
		"page_size": pageSize,
	})
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
		if err == sql.ErrNoRows {
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
