package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Recovery Dispute API Handlers
// 追偿争议功能 - 商户/骑手对平台追偿提出争议
// =============================================================================

// ========================= Helper Functions ============================

// getMerchantFromUserID 根据用户ID获取商户信息
func (server *Server) getMerchantFromUserID(ctx *gin.Context, userID int64) (db.Merchant, error) {
	merchant, err := server.resolveMerchantForUser(ctx, userID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return db.Merchant{}, err
	}
	return merchant, nil
}

// getRiderFromUserID 根据用户ID获取骑手信息
func (server *Server) getRiderFromUserID(ctx *gin.Context, userID int64) (db.Rider, error) {
	rider, err := server.store.GetRiderByUserID(ctx, userID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a rider")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return db.Rider{}, err
	}
	return rider, nil
}

// ========================= Common Types ============================

type recoveryDisputeResponse struct {
	ID                 int64      `json:"id"`
	ClaimID            int64      `json:"claim_id"`
	AppellantType      string     `json:"appellant_type"`
	AppellantID        int64      `json:"appellant_id"`
	Reason             string     `json:"reason"`
	Status             string     `json:"status"`
	ReviewerID         *int64     `json:"reviewer_id,omitempty"`
	ReviewNotes        *string    `json:"review_notes,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	CompensationAmount *int64     `json:"compensation_amount,omitempty"`
	CompensatedAt      *time.Time `json:"compensated_at,omitempty"`
	RegionID           int64      `json:"region_id"`
	CreatedAt          time.Time  `json:"created_at"`
}

func newRecoveryDisputeResponse(recoveryDispute db.RecoveryDispute) recoveryDisputeResponse {
	resp := recoveryDisputeResponse{
		ID:            recoveryDispute.ID,
		ClaimID:       recoveryDispute.ClaimID,
		AppellantType: recoveryDispute.AppellantType,
		AppellantID:   recoveryDispute.AppellantID,
		Reason:        recoveryDispute.Reason,
		Status:        recoveryDispute.Status,
		RegionID:      recoveryDispute.RegionID,
		CreatedAt:     recoveryDispute.CreatedAt,
	}
	if recoveryDispute.ReviewerID.Valid {
		resp.ReviewerID = &recoveryDispute.ReviewerID.Int64
	}
	if recoveryDispute.ReviewNotes.Valid {
		resp.ReviewNotes = &recoveryDispute.ReviewNotes.String
	}
	if recoveryDispute.ReviewedAt.Valid {
		resp.ReviewedAt = &recoveryDispute.ReviewedAt.Time
	}
	if recoveryDispute.CompensationAmount.Valid {
		resp.CompensationAmount = &recoveryDispute.CompensationAmount.Int64
	}
	if recoveryDispute.CompensatedAt.Valid {
		resp.CompensatedAt = &recoveryDispute.CompensatedAt.Time
	}
	return resp
}

// ========================= Merchant Claims/Recovery Disputes ============================

type listMerchantClaimsRequest struct {
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=1,max=50"`
	Bucket   string `form:"bucket" binding:"omitempty,oneof=pending_action disputed closed"`
}

type listRecoveryDisputesRequest struct {
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=1,max=50"`
	Status   string `form:"status" binding:"omitempty,oneof=submitted approved rejected"`
}

type merchantClaimResponse struct {
	ID                         int64      `json:"id"`
	OrderID                    int64      `json:"order_id"`
	OrderNo                    string     `json:"order_no"`
	OrderAmount                int64      `json:"order_amount"`
	UserPhone                  string     `json:"user_phone"`
	UserName                   string     `json:"user_name"`
	ClaimType                  string     `json:"claim_type"`
	ClaimAmount                int64      `json:"claim_amount"`
	ApprovedAmount             *int64     `json:"approved_amount,omitempty"`
	Description                string     `json:"description"`
	Status                     string     `json:"status"`
	CreatedAt                  time.Time  `json:"created_at"`
	ReviewedAt                 *time.Time `json:"reviewed_at,omitempty"`
	RecoveryDisputeID          *int64     `json:"recovery_dispute_id,omitempty"`
	RecoveryDisputeStatus      *string    `json:"recovery_dispute_status,omitempty"`
	RecoveryID                 *int64     `json:"recovery_id,omitempty"`
	RecoveryStatus             *string    `json:"recovery_status,omitempty"`
	RecoveryDisputeReason      *string    `json:"recovery_dispute_reason,omitempty"`
	RecoveryDisputeReviewNotes *string    `json:"recovery_dispute_review_notes,omitempty"`
}

type merchantClaimDecisionResponse struct {
	DecisionID         int64    `json:"decision_id"`
	ResponsibleParty   string   `json:"responsible_party"`
	CompensationSource string   `json:"compensation_source"`
	DecisionStatus     string   `json:"decision_status"`
	ReasonCodes        []string `json:"reason_codes"`
	TraceSummary       *string  `json:"trace_summary,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

type recoveryDisputeDetailResponse struct {
	ID                  int64      `json:"id"`
	ClaimID             int64      `json:"claim_id"`
	ClaimType           string     `json:"claim_type"`
	ClaimAmount         int64      `json:"claim_amount"`
	ClaimDescription    string     `json:"claim_description"`
	OrderNo             string     `json:"order_no"`
	OrderAmount         int64      `json:"order_amount"`
	UserPhone           string     `json:"user_phone"`
	AppellantType       string     `json:"appellant_type"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	ClaimApprovedAmount *int64     `json:"claim_approved_amount,omitempty"`
	ReviewerID          *int64     `json:"reviewer_id,omitempty"`
	ReviewNotes         *string    `json:"review_notes,omitempty"`
	ReviewedAt          *time.Time `json:"reviewed_at,omitempty"`
	CompensationAmount  *int64     `json:"compensation_amount,omitempty"`
	CompensatedAt       *time.Time `json:"compensated_at,omitempty"`
}

type recoveryDisputeListItem struct {
	ID                 int64      `json:"id"`
	ClaimID            int64      `json:"claim_id"`
	ClaimType          string     `json:"claim_type"`
	ClaimAmount        int64      `json:"claim_amount"`
	ClaimDescription   string     `json:"claim_description"`
	OrderNo            string     `json:"order_no"`
	Reason             string     `json:"reason"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	ReviewerID         *int64     `json:"reviewer_id,omitempty"`
	ReviewNotes        *string    `json:"review_notes,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	CompensationAmount *int64     `json:"compensation_amount,omitempty"`
}

type operatorRecoveryDisputeListItem struct {
	ID               int64       `json:"id"`
	ClaimID          int64       `json:"claim_id"`
	ClaimType        string      `json:"claim_type"`
	ClaimAmount      int64       `json:"claim_amount"`
	ClaimDescription string      `json:"claim_description"`
	OrderNo          string      `json:"order_no"`
	MerchantID       int64       `json:"merchant_id"`
	MerchantName     string      `json:"merchant_name"`
	AppellantType    string      `json:"appellant_type"`
	AppellantID      int64       `json:"appellant_id"`
	AppellantName    interface{} `json:"appellant_name"`
	Reason           string      `json:"reason"`
	Status           string      `json:"status"`
	CreatedAt        time.Time   `json:"created_at"`
	ReviewedAt       *time.Time  `json:"reviewed_at,omitempty"`
}

type merchantClaimsListResponse struct {
	Claims   []merchantClaimResponse `json:"claims"`
	Total    int64                   `json:"total"`
	PageID   int32                   `json:"page_id"`
	PageSize int32                   `json:"page_size"`
	HasMore  bool                    `json:"has_more"`
}

type recoveryDisputesListResponse struct {
	Disputes []recoveryDisputeListItem `json:"disputes"`
	Total    int64                     `json:"total"`
	PageID   int32                     `json:"page_id"`
	PageSize int32                     `json:"page_size"`
	HasMore  bool                      `json:"has_more"`
}

type claimSummaryResponse struct {
	Total         int64 `json:"total"`
	PendingAction int64 `json:"pending_action"`
	Disputed      int64 `json:"disputed"`
	Closed        int64 `json:"closed"`
}

type recoveryDisputeSummaryResponse struct {
	Total     int64 `json:"total"`
	Submitted int64 `json:"submitted"`
	Approved  int64 `json:"approved"`
	Rejected  int64 `json:"rejected"`
}

type operatorRecoveryDisputesListResponse struct {
	Disputes []operatorRecoveryDisputeListItem `json:"disputes"`
	Total    int64                             `json:"total"`
	Page     int32                             `json:"page"`
	Limit    int32                             `json:"limit"`
}

type merchantClaimDecisionResult struct {
	Decision *merchantClaimDecisionResponse `json:"decision"`
}

type operatorRecoveryDisputeDetailResponse struct {
	ID                  int64      `json:"id"`
	ClaimID             int64      `json:"claim_id"`
	ClaimType           string     `json:"claim_type"`
	ClaimAmount         int64      `json:"claim_amount"`
	ClaimDescription    string     `json:"claim_description"`
	ClaimStatus         string     `json:"claim_status"`
	ClaimCreatedAt      time.Time  `json:"claim_created_at"`
	OrderNo             string     `json:"order_no"`
	OrderAmount         int64      `json:"order_amount"`
	OrderStatus         string     `json:"order_status"`
	OrderCreatedAt      time.Time  `json:"order_created_at"`
	MerchantID          int64      `json:"merchant_id"`
	MerchantName        string     `json:"merchant_name"`
	MerchantPhone       string     `json:"merchant_phone"`
	UserPhone           string     `json:"user_phone"`
	UserName            string     `json:"user_name"`
	AppellantType       string     `json:"appellant_type"`
	AppellantID         int64      `json:"appellant_id"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	RegionID            int64      `json:"region_id"`
	CreatedAt           time.Time  `json:"created_at"`
	ClaimApprovedAmount *int64     `json:"claim_approved_amount,omitempty"`
	LookbackResult      *string    `json:"lookback_result,omitempty"`
	RiderID             *int64     `json:"rider_id,omitempty"`
	ReviewerID          *int64     `json:"reviewer_id,omitempty"`
	ReviewNotes         *string    `json:"review_notes,omitempty"`
	ReviewedAt          *time.Time `json:"reviewed_at,omitempty"`
	CompensationAmount  *int64     `json:"compensation_amount,omitempty"`
	CompensatedAt       *time.Time `json:"compensated_at,omitempty"`
}

// listMerchantClaims 商户查看收到的索赔列表
// @Summary 获取商户收到的索赔列表
// @Description 商户查看已批准的索赔列表，包含订单信息和追偿争议状态
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param bucket query string false "运营视图筛选" Enums(pending_action,disputed,closed)
// @Success 200 {object} map[string]interface{} "成功返回索赔列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims [get]
func (server *Server) listMerchantClaims(ctx *gin.Context) {
	var req listMerchantClaimsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	claims, err := server.store.ListMerchantClaimsForMerchant(ctx, db.ListMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket: pgtype.Text{
			String: req.Bucket,
			Valid:  req.Bucket != "",
		},
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountMerchantClaimsForMerchant(ctx, db.CountMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket: pgtype.Text{
			String: req.Bucket,
			Valid:  req.Bucket != "",
		},
	})
	if err != nil {
		total = int64(len(claims))
	}

	response := make([]merchantClaimResponse, len(claims))
	for i, c := range claims {
		response[i] = merchantClaimResponse{
			ID:          c.ID,
			OrderID:     c.OrderID,
			OrderNo:     c.OrderNo,
			OrderAmount: c.OrderAmount,
			UserPhone:   c.UserPhone.String,
			UserName:    c.UserName,
			ClaimType:   c.ClaimType,
			ClaimAmount: c.ClaimAmount,
			Description: c.Description,
			Status:      c.Status,
			CreatedAt:   c.CreatedAt,
		}
		if c.ApprovedAmount.Valid {
			response[i].ApprovedAmount = &c.ApprovedAmount.Int64
		}
		if c.ReviewedAt.Valid {
			response[i].ReviewedAt = &c.ReviewedAt.Time
		}
		if c.RecoveryDisputeID.Valid {
			response[i].RecoveryDisputeID = &c.RecoveryDisputeID.Int64
		}
		if c.RecoveryDisputeStatus.Valid {
			response[i].RecoveryDisputeStatus = &c.RecoveryDisputeStatus.String
		}
		if c.RecoveryID > 0 {
			recoveryID := c.RecoveryID
			response[i].RecoveryID = &recoveryID
		}
		if c.RecoveryStatus != "" {
			response[i].RecoveryStatus = &c.RecoveryStatus
		}
	}

	hasMore := int64(offset)+int64(len(response)) < total
	ctx.JSON(http.StatusOK, merchantClaimsListResponse{Claims: response, Total: total, PageID: req.PageID, PageSize: req.PageSize, HasMore: hasMore})
}

// listMerchantClaimsSummary 商户索赔汇总
// @Summary 获取商户索赔汇总
// @Description 返回商户索赔总数及各 bucket 汇总，供工作台和筛选条使用
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} claimSummaryResponse "成功返回索赔汇总"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims/summary [get]
func (server *Server) listMerchantClaimsSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	countBucket := func(bucket string) (int64, error) {
		return server.store.CountMerchantClaimsForMerchant(ctx, db.CountMerchantClaimsForMerchantParams{
			MerchantID: merchant.ID,
			Bucket:     pgtype.Text{String: bucket, Valid: bucket != ""},
		})
	}

	total, err := countBucket("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pendingAction, err := countBucket("pending_action")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	disputed, err := countBucket("disputed")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	closed, err := countBucket("closed")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, claimSummaryResponse{
		Total:         total,
		PendingAction: pendingAction,
		Disputed:      disputed,
		Closed:        closed,
	})
}

// getMerchantClaimDetail 商户查看索赔详情
// @Summary 获取索赔详情
// @Description 商户查看单个索赔的详细信息，包含追偿争议信息
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} map[string]interface{} "成功返回索赔详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims/{id} [get]
func (server *Server) getMerchantClaimDetail(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid claim id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	claim, err := server.store.GetMerchantClaimDetailForMerchant(ctx, db.GetMerchantClaimDetailForMerchantParams{
		ID:         claimID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := merchantClaimResponse{
		ID:          claim.ID,
		OrderID:     claim.OrderID,
		OrderNo:     claim.OrderNo,
		OrderAmount: claim.OrderAmount,
		UserPhone:   claim.UserPhone.String,
		UserName:    claim.UserName,
		ClaimType:   claim.ClaimType,
		ClaimAmount: claim.ClaimAmount,
		Description: claim.Description,
		Status:      claim.Status,
		CreatedAt:   claim.CreatedAt,
	}
	if claim.ApprovedAmount.Valid {
		response.ApprovedAmount = &claim.ApprovedAmount.Int64
	}
	if claim.ReviewedAt.Valid {
		v := claim.ReviewedAt.Time
		response.ReviewedAt = &v
	}
	if claim.RecoveryDisputeID.Valid {
		response.RecoveryDisputeID = &claim.RecoveryDisputeID.Int64
		s := claim.RecoveryDisputeStatus.String
		response.RecoveryDisputeStatus = &s
		if claim.RecoveryDisputeReason.Valid {
			r := claim.RecoveryDisputeReason.String
			response.RecoveryDisputeReason = &r
		}
		if claim.RecoveryDisputeReviewNotes.Valid {
			n := claim.RecoveryDisputeReviewNotes.String
			response.RecoveryDisputeReviewNotes = &n
		}
	}
	if claim.RecoveryID > 0 {
		recoveryID := claim.RecoveryID
		response.RecoveryID = &recoveryID
	}
	if claim.RecoveryStatus != "" {
		recoveryStatus := claim.RecoveryStatus
		response.RecoveryStatus = &recoveryStatus
	}

	ctx.JSON(http.StatusOK, response)
}

// getMerchantClaimDecision 商户查看索赔判定依据
// @Summary 获取索赔判定依据
// @Description 商户查看该索赔对应订单的最新行为判定信息（责任方、赔付来源、原因码、判定摘要）
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} map[string]interface{} "成功返回判定依据"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims/{id}/decision [get]
func (server *Server) getMerchantClaimDecision(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid claim id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	claim, err := server.store.GetMerchantClaimDetailForMerchant(ctx, db.GetMerchantClaimDetailForMerchantParams{
		ID:         claimID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 这里是只读查询路径。
	// 申诉侧只读取已持久化的 behavior decision 作为展示依据，
	// 不在查看接口里追加 payout/recovery/block/notify 等执行副作用。
	decisions, err := server.store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: claim.OrderID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if len(decisions) == 0 {
		ctx.JSON(http.StatusOK, merchantClaimDecisionResult{Decision: nil})
		return
	}

	latest := decisions[0]
	var traceSummary *string
	if latest.TraceSummary.Valid {
		traceSummary = &latest.TraceSummary.String
	}

	decisionValue := merchantClaimDecisionResponse{
		DecisionID:         latest.ID,
		ResponsibleParty:   latest.ResponsibleParty,
		CompensationSource: latest.CompensationSource,
		DecisionStatus:     latest.DecisionStatus,
		ReasonCodes:        latest.ReasonCodes,
		TraceSummary:       traceSummary,
		CreatedAt:          latest.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          latest.UpdatedAt.Format(time.RFC3339),
	}
	ctx.JSON(http.StatusOK, merchantClaimDecisionResult{Decision: &decisionValue})
}

// getRiderClaimDecision 骑手查看索赔判定依据
// @Summary 获取索赔判定依据
// @Description 骑手查看该索赔对应订单的最新行为判定信息（责任方、赔付来源、原因码、判定摘要）
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} merchantClaimDecisionResult "成功返回判定依据"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims/{id}/decision [get]
func (server *Server) getRiderClaimDecision(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid claim id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	claim, err := server.store.GetRiderClaimDetailForRider(ctx, db.GetRiderClaimDetailForRiderParams{
		ID:      claimID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 这里是只读查询路径。
	// 骑手查看判定依据时只消费 persisted behavior decision，
	// 不应在读取接口里重跑主判或派生新的行为动作。
	decisions, err := server.store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: claim.OrderID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if len(decisions) == 0 {
		ctx.JSON(http.StatusOK, merchantClaimDecisionResult{Decision: nil})
		return
	}

	latest := decisions[0]
	var traceSummary *string
	if latest.TraceSummary.Valid {
		traceSummary = &latest.TraceSummary.String
	}

	decisionValue := merchantClaimDecisionResponse{
		DecisionID:         latest.ID,
		ResponsibleParty:   latest.ResponsibleParty,
		CompensationSource: latest.CompensationSource,
		DecisionStatus:     latest.DecisionStatus,
		ReasonCodes:        latest.ReasonCodes,
		TraceSummary:       traceSummary,
		CreatedAt:          latest.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          latest.UpdatedAt.Format(time.RFC3339),
	}
	ctx.JSON(http.StatusOK, merchantClaimDecisionResult{Decision: &decisionValue})
}

type createMerchantRecoveryDisputeRequest struct {
	ClaimID int64  `json:"claim_id" binding:"required,min=1"`
	Reason  string `json:"reason" binding:"required,min=10,max=1000"`
}

// createMerchantRecoveryDispute 商户提交追偿争议
// @Summary 提交追偿争议
// @Description 商户对平台向自身发起的追偿提交争议，提交后由系统自动复核并写回最终结果
// @Tags 商户追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createMerchantRecoveryDisputeRequest true "追偿争议请求"
// @Success 201 {object} recoveryDisputeResponse "成功创建追偿争议"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户或追偿不属于该商户"
// @Failure 404 {object} map[string]interface{} "追偿不存在"
// @Failure 409 {object} map[string]interface{} "已存在追偿争议"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/recovery-disputes [post]
func (server *Server) createMerchantRecoveryDispute(ctx *gin.Context) {
	var req createMerchantRecoveryDisputeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	recoveryDispute, err := logic.CreateMerchantRecoveryDispute(ctx, server.store, logic.CreateMerchantRecoveryDisputeInput{
		MerchantID:        merchant.ID,
		ClaimID:           req.ClaimID,
		Reason:            req.Reason,
		DisputeWindowDays: RecoveryDisputeWindowDays,
		Now:               time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if recoveryDispute.Status == "submitted" {
		recoveryDispute = server.autoResolveRecoveryDisputeBestEffort(ctx, recoveryDispute)
	}

	ctx.JSON(http.StatusCreated, newRecoveryDisputeResponse(recoveryDispute))
}

// listMerchantRecoveryDisputes 商户查看追偿争议列表
// @Summary 获取追偿争议列表
// @Description 商户查看自己提交的追偿争议列表
// @Tags 商户追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param status query string false "状态筛选" Enums(submitted,approved,rejected)
// @Success 200 {object} map[string]interface{} "成功返回追偿争议列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/recovery-disputes [get]
func (server *Server) listMerchantRecoveryDisputes(ctx *gin.Context) {
	var req listRecoveryDisputesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	result, err := logic.ListMerchantRecoveryDisputes(ctx, server.store, logic.ListMerchantRecoveryDisputesInput{
		MerchantID: merchant.ID,
		Status:     req.Status,
		Limit:      req.PageSize,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := make([]recoveryDisputeListItem, len(result.Disputes))
	for i, a := range result.Disputes {
		response[i] = recoveryDisputeListItem{
			ID:               a.ID,
			ClaimID:          a.ClaimID,
			ClaimType:        a.ClaimType,
			ClaimAmount:      a.ClaimAmount,
			ClaimDescription: a.ClaimDescription,
			OrderNo:          a.OrderNo,
			Reason:           a.Reason,
			Status:           a.Status,
			CreatedAt:        a.CreatedAt,
		}
		if a.ReviewerID.Valid {
			response[i].ReviewerID = &a.ReviewerID.Int64
		}
		if a.ReviewNotes.Valid {
			s := a.ReviewNotes.String
			response[i].ReviewNotes = &s
		}
		if a.ReviewedAt.Valid {
			t := a.ReviewedAt.Time
			response[i].ReviewedAt = &t
		}
		if a.CompensationAmount.Valid {
			response[i].CompensationAmount = &a.CompensationAmount.Int64
		}
	}

	hasMore := int64(offset)+int64(len(response)) < result.Total
	ctx.JSON(http.StatusOK, recoveryDisputesListResponse{Disputes: response, Total: result.Total, PageID: req.PageID, PageSize: req.PageSize, HasMore: hasMore})
}

// listMerchantRecoveryDisputesSummary 商户追偿争议汇总
// @Summary 获取商户追偿争议汇总
// @Description 返回商户追偿争议总数及各状态汇总，供工作台和筛选条使用
// @Tags 商户追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} recoveryDisputeSummaryResponse "成功返回追偿争议汇总"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/recovery-disputes/summary [get]
func (server *Server) listMerchantRecoveryDisputesSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	countStatus := func(status string) (int64, error) {
		return server.store.CountMerchantRecoveryDisputesForMerchant(ctx, db.CountMerchantRecoveryDisputesForMerchantParams{
			AppellantID: merchant.ID,
			Status:      pgtype.Text{String: status, Valid: status != ""},
		})
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	submitted, err := countStatus("submitted")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	approved, err := countStatus("approved")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	rejected, err := countStatus("rejected")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, recoveryDisputeSummaryResponse{
		Total:     total,
		Submitted: submitted,
		Approved:  approved,
		Rejected:  rejected,
	})
}

// getMerchantRecoveryDisputeDetail 商户查看追偿争议详情
// @Summary 获取追偿争议详情
// @Description 商户查看自己提交的追偿争议详细信息
// @Tags 商户追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "追偿争议ID"
// @Success 200 {object} map[string]interface{} "成功返回追偿争议详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 404 {object} map[string]interface{} "追偿争议不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/recovery-disputes/{id} [get]
func (server *Server) getMerchantRecoveryDisputeDetail(ctx *gin.Context) {
	recoveryDisputeID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid recovery dispute id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	recoveryDispute, err := logic.GetMerchantRecoveryDisputeDetail(ctx, server.store, logic.GetMerchantRecoveryDisputeDetailInput{
		DisputeID:  recoveryDisputeID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := recoveryDisputeDetailResponse{
		ID:               recoveryDispute.ID,
		ClaimID:          recoveryDispute.ClaimID,
		ClaimType:        recoveryDispute.ClaimType,
		ClaimAmount:      recoveryDispute.ClaimAmount,
		ClaimDescription: recoveryDispute.ClaimDescription,
		OrderNo:          recoveryDispute.OrderNo,
		OrderAmount:      recoveryDispute.OrderAmount,
		UserPhone:        recoveryDispute.UserPhone.String,
		AppellantType:    recoveryDispute.AppellantType,
		Reason:           recoveryDispute.Reason,
		Status:           recoveryDispute.Status,
		CreatedAt:        recoveryDispute.CreatedAt,
	}
	if recoveryDispute.ClaimApprovedAmount.Valid {
		resp.ClaimApprovedAmount = &recoveryDispute.ClaimApprovedAmount.Int64
	}
	if recoveryDispute.ReviewerID.Valid {
		resp.ReviewerID = &recoveryDispute.ReviewerID.Int64
	}
	if recoveryDispute.ReviewNotes.Valid {
		s := recoveryDispute.ReviewNotes.String
		resp.ReviewNotes = &s
	}
	if recoveryDispute.ReviewedAt.Valid {
		t := recoveryDispute.ReviewedAt.Time
		resp.ReviewedAt = &t
	}
	if recoveryDispute.CompensationAmount.Valid {
		resp.CompensationAmount = &recoveryDispute.CompensationAmount.Int64
	}
	if recoveryDispute.CompensatedAt.Valid {
		t := recoveryDispute.CompensatedAt.Time
		resp.CompensatedAt = &t
	}

	ctx.JSON(http.StatusOK, resp)
}

// ========================= Rider Claims/Recovery Disputes ============================

// listRiderClaims 骑手查看收到的索赔列表
// @Summary 获取骑手收到的索赔列表
// @Description 骑手查看与自己代取订单相关的已批准索赔列表
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} map[string]interface{} "成功返回索赔列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims [get]
func (server *Server) listRiderClaims(ctx *gin.Context) {
	var req listMerchantClaimsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	claims, err := server.store.ListRiderClaimsForRider(ctx, db.ListRiderClaimsForRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Bucket: pgtype.Text{
			String: req.Bucket,
			Valid:  req.Bucket != "",
		},
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountRiderClaimsForRider(ctx, db.CountRiderClaimsForRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Bucket: pgtype.Text{
			String: req.Bucket,
			Valid:  req.Bucket != "",
		},
	})
	if err != nil {
		total = int64(len(claims))
	}

	response := make([]merchantClaimResponse, len(claims))
	for i, c := range claims {
		response[i] = merchantClaimResponse{
			ID:          c.ID,
			OrderID:     c.OrderID,
			OrderNo:     c.OrderNo,
			OrderAmount: c.OrderAmount,
			UserPhone:   c.UserPhone.String,
			UserName:    c.UserName,
			ClaimType:   c.ClaimType,
			ClaimAmount: c.ClaimAmount,
			Description: c.Description,
			Status:      c.Status,
			CreatedAt:   c.CreatedAt,
		}
		if c.ApprovedAmount.Valid {
			response[i].ApprovedAmount = &c.ApprovedAmount.Int64
		}
		if c.ReviewedAt.Valid {
			response[i].ReviewedAt = &c.ReviewedAt.Time
		}
		if c.RecoveryDisputeID.Valid {
			response[i].RecoveryDisputeID = &c.RecoveryDisputeID.Int64
		}
		if c.RecoveryDisputeStatus.Valid {
			response[i].RecoveryDisputeStatus = &c.RecoveryDisputeStatus.String
		}
		if c.RecoveryID > 0 {
			recoveryID := c.RecoveryID
			response[i].RecoveryID = &recoveryID
		}
		if c.RecoveryStatus != "" {
			response[i].RecoveryStatus = &c.RecoveryStatus
		}
	}

	ctx.JSON(http.StatusOK, merchantClaimsListResponse{Claims: response, Total: total, PageID: req.PageID, PageSize: req.PageSize})
}

// listRiderClaimsSummary 骑手索赔汇总
// @Summary 获取骑手索赔汇总
// @Description 返回骑手索赔总数及各 bucket 汇总，供工作台和筛选条使用
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} claimSummaryResponse "成功返回索赔汇总"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims/summary [get]
func (server *Server) listRiderClaimsSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	countBucket := func(bucket string) (int64, error) {
		return server.store.CountRiderClaimsForRider(ctx, db.CountRiderClaimsForRiderParams{
			RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
			Bucket:  pgtype.Text{String: bucket, Valid: bucket != ""},
		})
	}

	total, err := countBucket("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pendingAction, err := countBucket("pending_action")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	disputed, err := countBucket("disputed")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	closed, err := countBucket("closed")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, claimSummaryResponse{
		Total:         total,
		PendingAction: pendingAction,
		Disputed:      disputed,
		Closed:        closed,
	})
}

// getRiderClaimDetail 骑手查看索赔详情
// @Summary 获取索赔详情
// @Description 骑手查看单个索赔的详细信息
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} map[string]interface{} "成功返回索赔详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims/{id} [get]
func (server *Server) getRiderClaimDetail(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid claim id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	claim, err := server.store.GetRiderClaimDetailForRider(ctx, db.GetRiderClaimDetailForRiderParams{
		ID:      claimID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := merchantClaimResponse{
		ID:          claim.ID,
		OrderID:     claim.OrderID,
		OrderNo:     claim.OrderNo,
		OrderAmount: claim.OrderAmount,
		UserPhone:   claim.UserPhone.String,
		UserName:    claim.UserName,
		ClaimType:   claim.ClaimType,
		ClaimAmount: claim.ClaimAmount,
		Description: claim.Description,
		Status:      claim.Status,
		CreatedAt:   claim.CreatedAt,
	}
	if claim.ApprovedAmount.Valid {
		rsp.ApprovedAmount = &claim.ApprovedAmount.Int64
	}
	if claim.ReviewedAt.Valid {
		v := claim.ReviewedAt.Time
		rsp.ReviewedAt = &v
	}
	if claim.RecoveryDisputeID.Valid {
		rsp.RecoveryDisputeID = &claim.RecoveryDisputeID.Int64
		s := claim.RecoveryDisputeStatus.String
		rsp.RecoveryDisputeStatus = &s
		if claim.RecoveryDisputeReason.Valid {
			r := claim.RecoveryDisputeReason.String
			rsp.RecoveryDisputeReason = &r
		}
		if claim.RecoveryDisputeReviewNotes.Valid {
			n := claim.RecoveryDisputeReviewNotes.String
			rsp.RecoveryDisputeReviewNotes = &n
		}
	}
	if claim.RecoveryID > 0 {
		recoveryID := claim.RecoveryID
		rsp.RecoveryID = &recoveryID
	}
	if claim.RecoveryStatus != "" {
		rsp.RecoveryStatus = &claim.RecoveryStatus
	}

	ctx.JSON(http.StatusOK, rsp)
}

type createRiderRecoveryDisputeRequest struct {
	ClaimID int64  `json:"claim_id" binding:"required,min=1"`
	Reason  string `json:"reason" binding:"required,min=10,max=1000"`
}

// createRiderRecoveryDispute 骑手提交追偿争议
// @Summary 提交追偿争议
// @Description 骑手对平台向自身发起的追偿提交争议，提交后由系统自动复核并写回最终结果
// @Tags 骑手追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createRiderRecoveryDisputeRequest true "追偿争议请求"
// @Success 201 {object} recoveryDisputeResponse "成功创建追偿争议"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户或追偿与骑手无关"
// @Failure 404 {object} map[string]interface{} "追偿不存在"
// @Failure 409 {object} map[string]interface{} "已存在追偿争议"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/recovery-disputes [post]
func (server *Server) createRiderRecoveryDispute(ctx *gin.Context) {
	var req createRiderRecoveryDisputeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	result, err := logic.CreateRiderRecoveryDispute(ctx, server.store, logic.CreateRiderRecoveryDisputeInput{
		RiderID:           rider.ID,
		ClaimID:           req.ClaimID,
		Reason:            req.Reason,
		DisputeWindowDays: RecoveryDisputeWindowDays,
		Now:               time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	status := http.StatusCreated
	if result.AlreadyExists {
		status = http.StatusOK
	}

	recoveryDispute := result.RecoveryDispute
	if recoveryDispute.Status == "submitted" {
		recoveryDispute = server.autoResolveRecoveryDisputeBestEffort(ctx, recoveryDispute)
	}

	ctx.JSON(status, newRecoveryDisputeResponse(recoveryDispute))
}

// listRiderRecoveryDisputes 骑手查看追偿争议列表
// @Summary 获取追偿争议列表
// @Description 骑手查看自己提交的追偿争议列表
// @Tags 骑手追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param status query string false "状态筛选" Enums(submitted,approved,rejected)
// @Success 200 {object} map[string]interface{} "成功返回追偿争议列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/recovery-disputes [get]
func (server *Server) listRiderRecoveryDisputes(ctx *gin.Context) {
	var req listRecoveryDisputesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	result, err := logic.ListRiderRecoveryDisputes(ctx, server.store, logic.ListRiderRecoveryDisputesInput{
		RiderID: rider.ID,
		Status:  req.Status,
		Limit:   req.PageSize,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := make([]recoveryDisputeListItem, len(result.Disputes))
	for i, a := range result.Disputes {
		response[i] = recoveryDisputeListItem{
			ID:               a.ID,
			ClaimID:          a.ClaimID,
			ClaimType:        a.ClaimType,
			ClaimAmount:      a.ClaimAmount,
			ClaimDescription: a.ClaimDescription,
			OrderNo:          a.OrderNo,
			Reason:           a.Reason,
			Status:           a.Status,
			CreatedAt:        a.CreatedAt,
		}
		if a.ReviewerID.Valid {
			response[i].ReviewerID = &a.ReviewerID.Int64
		}
		if a.ReviewNotes.Valid {
			s := a.ReviewNotes.String
			response[i].ReviewNotes = &s
		}
		if a.ReviewedAt.Valid {
			t := a.ReviewedAt.Time
			response[i].ReviewedAt = &t
		}
		if a.CompensationAmount.Valid {
			response[i].CompensationAmount = &a.CompensationAmount.Int64
		}
	}

	hasMore := int64(offset)+int64(len(response)) < result.Total
	ctx.JSON(http.StatusOK, recoveryDisputesListResponse{Disputes: response, Total: result.Total, PageID: req.PageID, PageSize: req.PageSize, HasMore: hasMore})
}

// getRiderRecoveryDisputeDetail 骑手查看追偿争议详情
// @Summary 获取追偿争议详情
// @Description 骑手查看自己提交的追偿争议详细信息
// @Tags 骑手追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "追偿争议ID"
// @Success 200 {object} map[string]interface{} "成功返回追偿争议详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 404 {object} map[string]interface{} "追偿争议不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/recovery-disputes/{id} [get]
func (server *Server) getRiderRecoveryDisputeDetail(ctx *gin.Context) {
	recoveryDisputeID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid recovery dispute id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	result, err := logic.GetRiderRecoveryDisputeDetail(ctx, server.store, logic.GetRiderRecoveryDisputeDetailInput{
		DisputeID: recoveryDisputeID,
		RiderID:   rider.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := recoveryDisputeDetailResponse{
		ID:               result.ID,
		ClaimID:          result.ClaimID,
		ClaimType:        result.ClaimType,
		ClaimAmount:      result.ClaimAmount,
		ClaimDescription: result.ClaimDescription,
		OrderNo:          result.OrderNo,
		OrderAmount:      result.OrderAmount,
		UserPhone:        result.UserPhone.String,
		AppellantType:    result.AppellantType,
		Reason:           result.Reason,
		Status:           result.Status,
		CreatedAt:        result.CreatedAt,
	}
	if result.ClaimApprovedAmount.Valid {
		resp.ClaimApprovedAmount = &result.ClaimApprovedAmount.Int64
	}
	if result.ReviewerID.Valid {
		resp.ReviewerID = &result.ReviewerID.Int64
	}
	if result.ReviewNotes.Valid {
		s := result.ReviewNotes.String
		resp.ReviewNotes = &s
	}
	if result.ReviewedAt.Valid {
		t := result.ReviewedAt.Time
		resp.ReviewedAt = &t
	}
	if result.CompensationAmount.Valid {
		resp.CompensationAmount = &result.CompensationAmount.Int64
	}
	if result.CompensatedAt.Valid {
		t := result.CompensatedAt.Time
		resp.CompensatedAt = &t
	}

	ctx.JSON(http.StatusOK, resp)
}

// ========================= Operator Recovery Disputes ============================

type listOperatorRecoveryDisputesRequest struct {
	RegionID *int64  `form:"region_id" binding:"omitempty,min=1"`
	Status   *string `form:"status" binding:"omitempty,oneof=submitted approved rejected"`
	Page     int32   `form:"page" binding:"omitempty,min=1"`
	Limit    int32   `form:"limit" binding:"omitempty,min=1,max=100"`
}

// listOperatorRecoveryDisputes 运营商查看区域内追偿争议列表
// @Summary 获取区域内追偿争议列表
// @Description 运营商查看自己管辖区域内的追偿争议列表，可按状态筛选
// @Tags 运营商追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param status query string false "状态筛选" Enums(submitted, approved, rejected)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} map[string]interface{} "成功返回追偿争议列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非运营商用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/recovery-disputes [get]
func (server *Server) listOperatorRecoveryDisputes(ctx *gin.Context) {
	var req listOperatorRecoveryDisputesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	offset := pageOffset(req.Page, req.Limit)

	var status string
	if req.Status != nil {
		status = *req.Status
	}

	recoveryDisputes := make([]db.ListOperatorRecoveryDisputesRow, 0)
	var total int64
	if selection.IsAllRegions {
		fetchLimit := req.Page * req.Limit
		for _, regionID := range selection.RegionIDs {
			regionDisputes, queryErr := server.store.ListOperatorRecoveryDisputes(ctx, db.ListOperatorRecoveryDisputesParams{
				RegionID: regionID,
				Column2:  status,
				Limit:    fetchLimit,
				Offset:   0,
			})
			if queryErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, queryErr))
				return
			}
			recoveryDisputes = append(recoveryDisputes, regionDisputes...)

			regionTotal, countErr := server.store.CountOperatorRecoveryDisputes(ctx, db.CountOperatorRecoveryDisputesParams{
				RegionID: regionID,
				Column2:  status,
			})
			if countErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, countErr))
				return
			}
			total += regionTotal
		}

		sort.Slice(recoveryDisputes, func(i, j int) bool {
			if !recoveryDisputes[i].CreatedAt.Equal(recoveryDisputes[j].CreatedAt) {
				return recoveryDisputes[i].CreatedAt.After(recoveryDisputes[j].CreatedAt)
			}
			return recoveryDisputes[i].ID > recoveryDisputes[j].ID
		})

		start := int(offset)
		if start >= len(recoveryDisputes) {
			recoveryDisputes = []db.ListOperatorRecoveryDisputesRow{}
		} else {
			end := start + int(req.Limit)
			if end > len(recoveryDisputes) {
				end = len(recoveryDisputes)
			}
			recoveryDisputes = recoveryDisputes[start:end]
		}
	} else {
		recoveryDisputes, err = server.store.ListOperatorRecoveryDisputes(ctx, db.ListOperatorRecoveryDisputesParams{
			RegionID: selection.RegionID,
			Column2:  status,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountOperatorRecoveryDisputes(ctx, db.CountOperatorRecoveryDisputesParams{
			RegionID: selection.RegionID,
			Column2:  status,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	response := make([]operatorRecoveryDisputeListItem, len(recoveryDisputes))
	for i, a := range recoveryDisputes {
		response[i] = operatorRecoveryDisputeListItem{
			ID:               a.ID,
			ClaimID:          a.ClaimID,
			ClaimType:        a.ClaimType,
			ClaimAmount:      a.ClaimAmount,
			ClaimDescription: a.ClaimDescription,
			OrderNo:          a.OrderNo,
			MerchantID:       a.MerchantID,
			MerchantName:     a.MerchantName,
			AppellantType:    a.AppellantType,
			AppellantID:      a.AppellantID,
			AppellantName:    a.AppellantName,
			Reason:           a.Reason,
			Status:           a.Status,
			CreatedAt:        a.CreatedAt,
		}
		if a.ReviewedAt.Valid {
			t := a.ReviewedAt.Time
			response[i].ReviewedAt = &t
		}
	}

	ctx.JSON(http.StatusOK, operatorRecoveryDisputesListResponse{Disputes: response, Total: total, Page: req.Page, Limit: req.Limit})
}

// listOperatorRecoveryDisputesSummary 运营商追偿争议汇总
// @Summary 获取区域追偿争议汇总
// @Description 返回运营商可管理区域内追偿争议总数及各状态汇总，供工作台和审批入口使用
// @Tags 运营商追偿争议管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param region_id query int false "区域ID"
// @Success 200 {object} recoveryDisputeSummaryResponse "成功返回追偿争议汇总"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Router /v1/operator/recovery-disputes/summary [get]
func (server *Server) listOperatorRecoveryDisputesSummary(ctx *gin.Context) {
	var req listOperatorRecoveryDisputesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	countStatus := func(status string) (int64, error) {
		var total int64
		for _, regionID := range selection.RegionIDs {
			count, err := server.store.CountOperatorRecoveryDisputes(ctx, db.CountOperatorRecoveryDisputesParams{
				RegionID: regionID,
				Column2:  status,
			})
			if err != nil {
				return 0, err
			}
			total += count
		}
		return total, nil
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	submitted, err := countStatus("submitted")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	approved, err := countStatus("approved")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	rejected, err := countStatus("rejected")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, recoveryDisputeSummaryResponse{
		Total:     total,
		Submitted: submitted,
		Approved:  approved,
		Rejected:  rejected,
	})
}

// getOperatorRecoveryDisputeDetail 运营商查看追偿争议详情
// @Summary 获取追偿争议详情
// @Description 运营商查看区域内追偿争议的详细信息，包含索赔、订单、用户信用分等信息
// @Tags 运营商追偿争议管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "追偿争议ID"
// @Success 200 {object} map[string]interface{} "成功返回追偿争议详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非运营商用户"
// @Failure 404 {object} map[string]interface{} "追偿争议不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/recovery-disputes/{id} [get]
func (server *Server) getOperatorRecoveryDisputeDetail(ctx *gin.Context) {
	recoveryDisputeID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid recovery dispute id")))
		return
	}

	recoveryDispute, err := server.store.GetRecoveryDispute(ctx, recoveryDisputeID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("recovery dispute not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, recoveryDispute.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	detail, err := server.store.GetOperatorRecoveryDisputeDetail(ctx, db.GetOperatorRecoveryDisputeDetailParams{
		ID:       recoveryDisputeID,
		RegionID: recoveryDispute.RegionID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("recovery dispute not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := operatorRecoveryDisputeDetailResponse{
		ID:               detail.ID,
		ClaimID:          detail.ClaimID,
		ClaimType:        detail.ClaimType,
		ClaimAmount:      detail.ClaimAmount,
		ClaimDescription: detail.ClaimDescription,
		ClaimStatus:      detail.ClaimStatus,
		ClaimCreatedAt:   detail.ClaimCreatedAt,
		OrderNo:          detail.OrderNo,
		OrderAmount:      detail.OrderAmount,
		OrderStatus:      detail.OrderStatus,
		OrderCreatedAt:   detail.OrderCreatedAt,
		MerchantID:       detail.MerchantID,
		MerchantName:     detail.MerchantName,
		MerchantPhone:    detail.MerchantPhone,
		UserPhone:        detail.UserPhone.String,
		UserName:         detail.UserName,
		AppellantType:    detail.AppellantType,
		AppellantID:      detail.AppellantID,
		Reason:           detail.Reason,
		Status:           detail.Status,
		RegionID:         detail.RegionID,
		CreatedAt:        detail.CreatedAt,
	}
	if detail.ClaimApprovedAmount.Valid {
		resp.ClaimApprovedAmount = &detail.ClaimApprovedAmount.Int64
	}
	if detail.LookbackResult != nil {
		s := string(detail.LookbackResult)
		resp.LookbackResult = &s
	}
	if detail.RiderID.Valid {
		resp.RiderID = &detail.RiderID.Int64
	}
	if detail.ReviewerID.Valid {
		resp.ReviewerID = &detail.ReviewerID.Int64
	}
	if detail.ReviewNotes.Valid {
		s := detail.ReviewNotes.String
		resp.ReviewNotes = &s
	}
	if detail.ReviewedAt.Valid {
		t := detail.ReviewedAt.Time
		resp.ReviewedAt = &t
	}
	if detail.CompensationAmount.Valid {
		resp.CompensationAmount = &detail.CompensationAmount.Int64
	}
	if detail.CompensatedAt.Valid {
		t := detail.CompensatedAt.Time
		resp.CompensatedAt = &t
	}

	ctx.JSON(http.StatusOK, resp)
}

type automaticRecoveryDisputeResolution struct {
	status      string
	reviewNotes string
	decisionID  pgtype.Int8
}

func (server *Server) autoResolveRecoveryDispute(ctx *gin.Context, recoveryDispute db.RecoveryDispute) (db.RecoveryDispute, error) {
	resolutionResult, err := logic.ResolveRecoveryDisputeAutomatically(ctx, server.store, recoveryDispute)
	if err != nil {
		return db.RecoveryDispute{}, err
	}
	resolution := automaticRecoveryDisputeResolution{
		status:      resolutionResult.Resolution.Status,
		reviewNotes: resolutionResult.Resolution.ReviewNotes,
		decisionID:  resolutionResult.Resolution.DecisionID,
	}
	reviewResult := resolutionResult.ReviewResult

	metadata := map[string]any{
		"status":              resolution.status,
		"review_notes":        resolution.reviewNotes,
		"resolution_source":   "system",
		"compensation_amount": int64(0),
	}
	if resolution.decisionID.Valid {
		metadata["decision_id"] = resolution.decisionID.Int64
	}
	regionID := recoveryDispute.RegionID
	recoveryDisputeID := recoveryDispute.ID
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: 0,
		ActorRole:   "system",
		Action:      "system_recovery_dispute_resolved",
		TargetType:  "recovery_dispute",
		TargetID:    &recoveryDisputeID,
		RegionID:    &regionID,
		Metadata:    metadata,
	})

	taskPayload := &worker.ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID:    recoveryDispute.ID,
		ClaimID:              reviewResult.PostProcess.ClaimID,
		RecoveryTarget:       reviewResult.PostProcess.AppellantType,
		CompensationActionID: 0,
		ReleaseActionID: func() int64 {
			if reviewResult.ReleaseAction != nil {
				return reviewResult.ReleaseAction.ID
			}
			return 0
		}(),
		Status:             resolution.status,
		AppellantType:      reviewResult.PostProcess.AppellantType,
		AppellantID:        reviewResult.PostProcess.AppellantID,
		ClaimantUserID:     reviewResult.PostProcess.ClaimantUserID,
		ClaimType:          reviewResult.PostProcess.ClaimType,
		ClaimAmount:        reviewResult.PostProcess.ClaimAmount,
		CompensationAmount: 0,
		OrderNo:            reviewResult.PostProcess.OrderNo,
	}
	if err := server.dispatchRecoveryDisputeResult(ctx, taskPayload); err != nil {
		return db.RecoveryDispute{}, err
	}

	return reviewResult.RecoveryDispute, nil
}

func (server *Server) autoResolveRecoveryDisputeBestEffort(ctx *gin.Context, recoveryDispute db.RecoveryDispute) db.RecoveryDispute {
	resolvedRecoveryDispute, err := server.autoResolveRecoveryDispute(ctx, recoveryDispute)
	if err == nil {
		return resolvedRecoveryDispute
	}

	log.Error().Err(logic.LoggableError(err)).
		Int64("recovery_dispute_id", recoveryDispute.ID).
		Int64("claim_id", recoveryDispute.ClaimID).
		Str("appellant_type", recoveryDispute.AppellantType).
		Int64("appellant_id", recoveryDispute.AppellantID).
		Msg("recovery dispute was created but automatic resolution did not fully complete")

	if server.taskDistributor == nil {
		log.Warn().
			Int64("recovery_dispute_id", recoveryDispute.ID).
			Int64("claim_id", recoveryDispute.ClaimID).
			Msg("skip automatic recovery dispute resolution retry enqueue because task distributor is unavailable")
	} else if retryErr := server.taskDistributor.DistributeTaskAutomaticRecoveryDisputeResolution(ctx, &worker.AutomaticRecoveryDisputeResolutionPayload{RecoveryDisputeID: recoveryDispute.ID}); retryErr != nil {
		log.Error().Err(retryErr).
			Int64("recovery_dispute_id", recoveryDispute.ID).
			Int64("claim_id", recoveryDispute.ClaimID).
			Msg("failed to enqueue automatic recovery dispute resolution retry task")
	}

	persistedRecoveryDispute, persistedErr := server.store.GetRecoveryDispute(ctx, recoveryDispute.ID)
	if persistedErr == nil {
		return persistedRecoveryDispute
	}

	log.Error().Err(persistedErr).
		Int64("recovery_dispute_id", recoveryDispute.ID).
		Int64("claim_id", recoveryDispute.ClaimID).
		Msg("failed to reload persisted recovery dispute after automatic resolution error")

	return recoveryDispute
}

func (server *Server) dispatchRecoveryDisputeResult(ctx *gin.Context, payload *worker.ProcessRecoveryDisputeResultPayload) error {
	if server.taskDistributor == nil {
		return server.processRecoveryDisputeResultInline(ctx, payload)
	}

	if err := server.taskDistributor.DistributeTaskProcessRecoveryDisputeResult(ctx, payload); err != nil {
		return server.processRecoveryDisputeResultInline(ctx, payload)
	}

	return nil
}

func (server *Server) processRecoveryDisputeResultInline(ctx *gin.Context, payload *worker.ProcessRecoveryDisputeResultPayload) error {
	if err := worker.ExecuteRecoveryDisputeResultEffects(ctx, server.store, server.taskDistributor, server.transferClient, *payload); err != nil {
		return err
	}

	appellantUserID, err := server.getAppellantUserID(ctx, payload.AppellantType, payload.AppellantID)
	if err != nil {
		log.Error().Err(logic.LoggableError(err)).Int64("recovery_dispute_id", payload.RecoveryDisputeID).Str("appellant_type", payload.AppellantType).Int64("appellant_id", payload.AppellantID).Msg("failed to resolve appellant user id for recovery dispute notification")
	} else {
		appellantTitle, appellantContent := buildRecoveryDisputeNotificationContent(payload, true)
		server.SendNotificationSync(ctx, SendNotificationParams{
			UserID:      appellantUserID,
			Type:        "recovery_dispute",
			Title:       appellantTitle,
			Content:     appellantContent,
			RelatedType: "recovery_dispute",
			RelatedID:   payload.RecoveryDisputeID,
			ExtraData: map[string]any{
				"recovery_dispute_id": payload.RecoveryDisputeID,
				"status":              payload.Status,
				"appellant_type":      payload.AppellantType,
			},
		})
	}

	claimantTitle, claimantContent := buildRecoveryDisputeNotificationContent(payload, false)
	server.SendNotificationSync(ctx, SendNotificationParams{
		UserID:      payload.ClaimantUserID,
		Type:        "recovery_dispute",
		Title:       claimantTitle,
		Content:     claimantContent,
		RelatedType: "recovery_dispute",
		RelatedID:   payload.RecoveryDisputeID,
		ExtraData: map[string]any{
			"recovery_dispute_id": payload.RecoveryDisputeID,
			"status":              payload.Status,
		},
	})

	return nil
}

func (server *Server) getAppellantUserID(ctx *gin.Context, appellantType string, appellantID int64) (int64, error) {
	if appellantType == "merchant" {
		merchant, err := server.store.GetMerchant(ctx, appellantID)
		if err != nil {
			return 0, err
		}
		return merchant.OwnerUserID, nil
	}

	rider, err := server.store.GetRider(ctx, appellantID)
	if err != nil {
		return 0, err
	}
	return rider.UserID, nil
}

func buildRecoveryDisputeNotificationContent(payload *worker.ProcessRecoveryDisputeResultPayload, isAppellant bool) (string, string) {
	if isAppellant {
		if payload.Status == "approved" {
			content := "您针对订单" + payload.OrderNo + "的申诉已通过审核，平台已撤销本次判责与追偿。"
			if payload.CompensationAmount > 0 {
				content += " 已核定补偿金额" + formatMoney(payload.CompensationAmount) + "。"
			}
			return "申诉成功通知", content
		}
		return "申诉结果通知", "您针对订单" + payload.OrderNo + "的申诉未通过审核，原判责与追偿继续有效。"
	}

	if payload.Status == "approved" {
		return "索赔申诉结果通知", "您在订单" + payload.OrderNo + "中的" + getClaimTypeLabel(payload.ClaimType) + "索赔经审核认定为不当，平台已撤销对责任方的追责与追偿安排，已发放赔付不再向您追回。请合理使用售后服务。"
	}
	return "索赔申诉结果通知", "商家/骑手针对您订单" + payload.OrderNo + "的申诉未通过审核，原索赔与平台判责继续有效。"
}

func formatMoney(amount int64) string {
	return fenToYuanString(amount, 2) + "元"
}

func getClaimTypeLabel(claimType string) string {
	switch claimType {
	case "foreign-object":
		return "异物"
	case "damage":
		return "餐损"
	case "delay":
		return "延迟"
	case "quality":
		return "质量问题"
	case "missing-item":
		return "缺漏"
	default:
		return "其他"
	}
}
