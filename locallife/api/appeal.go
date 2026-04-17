package api

import (
	"errors"
	"net/http"
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
// Appeal API Handlers
// 申诉功能 - 商户/骑手对索赔的申诉
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

// getOperatorFromUserID 根据用户ID获取运营商信息
func (server *Server) getOperatorFromUserID(ctx *gin.Context, userID int64) (db.Operator, error) {
	operator, err := server.store.GetOperatorByUser(ctx, userID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not an operator")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return db.Operator{}, err
	}
	return operator, nil
}

// ========================= Common Types ============================

type appealResponse struct {
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

func newAppealResponse(a db.Appeal) appealResponse {
	resp := appealResponse{
		ID:            a.ID,
		ClaimID:       a.ClaimID,
		AppellantType: a.AppellantType,
		AppellantID:   a.AppellantID,
		Reason:        a.Reason,
		Status:        a.Status,
		RegionID:      a.RegionID,
		CreatedAt:     a.CreatedAt,
	}
	if a.ReviewerID.Valid {
		resp.ReviewerID = &a.ReviewerID.Int64
	}
	if a.ReviewNotes.Valid {
		resp.ReviewNotes = &a.ReviewNotes.String
	}
	if a.ReviewedAt.Valid {
		resp.ReviewedAt = &a.ReviewedAt.Time
	}
	if a.CompensationAmount.Valid {
		resp.CompensationAmount = &a.CompensationAmount.Int64
	}
	if a.CompensatedAt.Valid {
		resp.CompensatedAt = &a.CompensatedAt.Time
	}
	return resp
}

// ========================= Merchant Claims/Appeals ============================

type listMerchantClaimsRequest struct {
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=1,max=50"`
	Bucket   string `form:"bucket" binding:"omitempty,oneof=pending_action appealed closed"`
}

type listMerchantAppealsRequest struct {
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=1,max=50"`
	Status   string `form:"status" binding:"omitempty,oneof=pending approved compensated rejected"`
}

type merchantClaimResponse struct {
	ID                int64      `json:"id"`
	OrderID           int64      `json:"order_id"`
	OrderNo           string     `json:"order_no"`
	OrderAmount       int64      `json:"order_amount"`
	UserPhone         string     `json:"user_phone"`
	UserName          string     `json:"user_name"`
	ClaimType         string     `json:"claim_type"`
	ClaimAmount       int64      `json:"claim_amount"`
	ApprovedAmount    *int64     `json:"approved_amount,omitempty"`
	Description       string     `json:"description"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	AppealID          *int64     `json:"appeal_id,omitempty"`
	AppealStatus      *string    `json:"appeal_status,omitempty"`
	RecoveryStatus    *string    `json:"recovery_status,omitempty"`
	AppealReason      *string    `json:"appeal_reason,omitempty"`
	AppealReviewNotes *string    `json:"appeal_review_notes,omitempty"`
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

type appealDetailResponse struct {
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

type appealListItem struct {
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

type operatorAppealListItem struct {
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

type merchantAppealsListResponse struct {
	Appeals  []appealListItem `json:"appeals"`
	Total    int64            `json:"total"`
	PageID   int32            `json:"page_id"`
	PageSize int32            `json:"page_size"`
	HasMore  bool             `json:"has_more"`
}

type claimSummaryResponse struct {
	Total         int64 `json:"total"`
	PendingAction int64 `json:"pending_action"`
	Appealed      int64 `json:"appealed"`
	Closed        int64 `json:"closed"`
}

type appealSummaryResponse struct {
	Total       int64 `json:"total"`
	Pending     int64 `json:"pending"`
	Approved    int64 `json:"approved"`
	Compensated int64 `json:"compensated,omitempty"`
	Rejected    int64 `json:"rejected"`
}

type operatorAppealsListResponse struct {
	Appeals []operatorAppealListItem `json:"appeals"`
	Total   int64                    `json:"total"`
	Page    int32                    `json:"page"`
	Limit   int32                    `json:"limit"`
}

type merchantClaimDecisionResult struct {
	Decision *merchantClaimDecisionResponse `json:"decision"`
}

type operatorAppealDetailResponse struct {
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
// @Description 商户查看已批准的索赔列表，包含订单信息和申诉状态
// @Tags 商户申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param bucket query string false "运营视图筛选" Enums(pending_action,appealed,closed)
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
		if c.AppealID.Valid {
			response[i].AppealID = &c.AppealID.Int64
		}
		if c.AppealStatus.Valid {
			response[i].AppealStatus = &c.AppealStatus.String
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
// @Tags 商户申诉管理
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
	appealed, err := countBucket("appealed")
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
		Appealed:      appealed,
		Closed:        closed,
	})
}

// getMerchantClaimDetail 商户查看索赔详情
// @Summary 获取索赔详情
// @Description 商户查看单个索赔的详细信息，包含申诉信息
// @Tags 商户申诉管理
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
	if claim.AppealID.Valid {
		response.AppealID = &claim.AppealID.Int64
		s := claim.AppealStatus.String
		response.AppealStatus = &s
		if claim.AppealReason.Valid {
			r := claim.AppealReason.String
			response.AppealReason = &r
		}
		if claim.AppealReviewNotes.Valid {
			n := claim.AppealReviewNotes.String
			response.AppealReviewNotes = &n
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// getMerchantClaimDecision 商户查看索赔判定依据
// @Summary 获取索赔判定依据
// @Description 商户查看该索赔对应订单的最新行为判定信息（责任方、赔付来源、原因码、判定摘要）
// @Tags 商户申诉管理
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
// @Tags 骑手申诉管理
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

type createMerchantAppealRequest struct {
	ClaimID int64  `json:"claim_id" binding:"required,min=1"`
	Reason  string `json:"reason" binding:"required,min=10,max=1000"`
}

// createMerchantAppeal 商户提交申诉
// @Summary 提交申诉
// @Description 商户对已批准的索赔提交申诉，每个索赔只能申诉一次
// @Tags 商户申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createMerchantAppealRequest true "申诉请求"
// @Success 201 {object} appealResponse "成功创建申诉"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户或索赔不属于该商户"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 409 {object} map[string]interface{} "已存在申诉"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/appeals [post]
func (server *Server) createMerchantAppeal(ctx *gin.Context) {
	var req createMerchantAppealRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	appeal, err := logic.CreateMerchantAppeal(ctx, server.store, logic.CreateMerchantAppealInput{
		MerchantID:       merchant.ID,
		ClaimID:          req.ClaimID,
		Reason:           req.Reason,
		AppealWindowDays: AppealWindowDays,
		Now:              time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newAppealResponse(appeal))
}

// listMerchantAppeals 商户查看申诉列表
// @Summary 获取申诉列表
// @Description 商户查看自己提交的申诉列表
// @Tags 商户申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param status query string false "状态筛选" Enums(pending,approved,compensated,rejected)
// @Success 200 {object} map[string]interface{} "成功返回申诉列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/appeals [get]
func (server *Server) listMerchantAppeals(ctx *gin.Context) {
	var req listMerchantAppealsRequest
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

	result, err := logic.ListMerchantAppeals(ctx, server.store, logic.ListMerchantAppealsInput{
		MerchantID: merchant.ID,
		Status:     req.Status,
		Limit:      req.PageSize,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := make([]appealListItem, len(result.Appeals))
	for i, a := range result.Appeals {
		response[i] = appealListItem{
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
	ctx.JSON(http.StatusOK, merchantAppealsListResponse{Appeals: response, Total: result.Total, PageID: req.PageID, PageSize: req.PageSize, HasMore: hasMore})
}

// listMerchantAppealsSummary 商户申诉汇总
// @Summary 获取商户申诉汇总
// @Description 返回商户申诉总数及各状态汇总，供工作台和筛选条使用
// @Tags 商户申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} appealSummaryResponse "成功返回申诉汇总"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/appeals/summary [get]
func (server *Server) listMerchantAppealsSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	countStatus := func(status string) (int64, error) {
		return server.store.CountMerchantAppealsForMerchant(ctx, db.CountMerchantAppealsForMerchantParams{
			AppellantID: merchant.ID,
			Status:      pgtype.Text{String: status, Valid: status != ""},
		})
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pending, err := countStatus("pending")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	approved, err := countStatus("approved")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	compensated, err := countStatus("compensated")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	rejected, err := countStatus("rejected")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, appealSummaryResponse{
		Total:       total,
		Pending:     pending,
		Approved:    approved,
		Compensated: compensated,
		Rejected:    rejected,
	})
}

// getMerchantAppealDetail 商户查看申诉详情
// @Summary 获取申诉详情
// @Description 商户查看自己提交的申诉详细信息
// @Tags 商户申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "申诉ID"
// @Success 200 {object} map[string]interface{} "成功返回申诉详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户"
// @Failure 404 {object} map[string]interface{} "申诉不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/appeals/{id} [get]
func (server *Server) getMerchantAppealDetail(ctx *gin.Context) {
	appealID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid appeal id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	appeal, err := logic.GetMerchantAppealDetail(ctx, server.store, logic.GetMerchantAppealDetailInput{
		AppealID:   appealID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := appealDetailResponse{
		ID:               appeal.ID,
		ClaimID:          appeal.ClaimID,
		ClaimType:        appeal.ClaimType,
		ClaimAmount:      appeal.ClaimAmount,
		ClaimDescription: appeal.ClaimDescription,
		OrderNo:          appeal.OrderNo,
		OrderAmount:      appeal.OrderAmount,
		UserPhone:        appeal.UserPhone.String,
		AppellantType:    appeal.AppellantType,
		Reason:           appeal.Reason,
		Status:           appeal.Status,
		CreatedAt:        appeal.CreatedAt,
	}
	if appeal.ClaimApprovedAmount.Valid {
		resp.ClaimApprovedAmount = &appeal.ClaimApprovedAmount.Int64
	}
	if appeal.ReviewerID.Valid {
		resp.ReviewerID = &appeal.ReviewerID.Int64
	}
	if appeal.ReviewNotes.Valid {
		s := appeal.ReviewNotes.String
		resp.ReviewNotes = &s
	}
	if appeal.ReviewedAt.Valid {
		t := appeal.ReviewedAt.Time
		resp.ReviewedAt = &t
	}
	if appeal.CompensationAmount.Valid {
		resp.CompensationAmount = &appeal.CompensationAmount.Int64
	}
	if appeal.CompensatedAt.Valid {
		t := appeal.CompensatedAt.Time
		resp.CompensatedAt = &t
	}

	ctx.JSON(http.StatusOK, resp)
}

// ========================= Rider Claims/Appeals ============================

// listRiderClaims 骑手查看收到的索赔列表
// @Summary 获取骑手收到的索赔列表
// @Description 骑手查看与自己配送订单相关的已批准索赔列表
// @Tags 骑手申诉管理
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
		if c.AppealID.Valid {
			response[i].AppealID = &c.AppealID.Int64
		}
		if c.AppealStatus.Valid {
			response[i].AppealStatus = &c.AppealStatus.String
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
// @Tags 骑手申诉管理
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
	appealed, err := countBucket("appealed")
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
		Appealed:      appealed,
		Closed:        closed,
	})
}

// getRiderClaimDetail 骑手查看索赔详情
// @Summary 获取索赔详情
// @Description 骑手查看单个索赔的详细信息
// @Tags 骑手申诉管理
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
	if claim.AppealID.Valid {
		rsp.AppealID = &claim.AppealID.Int64
		s := claim.AppealStatus.String
		rsp.AppealStatus = &s
		if claim.AppealReason.Valid {
			r := claim.AppealReason.String
			rsp.AppealReason = &r
		}
		if claim.AppealReviewNotes.Valid {
			n := claim.AppealReviewNotes.String
			rsp.AppealReviewNotes = &n
		}
	}
	if claim.RecoveryStatus != "" {
		rsp.RecoveryStatus = &claim.RecoveryStatus
	}

	ctx.JSON(http.StatusOK, rsp)
}

type createRiderAppealRequest struct {
	ClaimID int64  `json:"claim_id" binding:"required,min=1"`
	Reason  string `json:"reason" binding:"required,min=10,max=1000"`
}

// createRiderAppeal 骑手提交申诉
// @Summary 提交申诉
// @Description 骑手对与自己配送订单相关的索赔提交申诉
// @Tags 骑手申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createRiderAppealRequest true "申诉请求"
// @Success 201 {object} appealResponse "成功创建申诉"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户或索赔与骑手无关"
// @Failure 404 {object} map[string]interface{} "索赔不存在"
// @Failure 409 {object} map[string]interface{} "已存在申诉"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/appeals [post]
func (server *Server) createRiderAppeal(ctx *gin.Context) {
	var req createRiderAppealRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	result, err := logic.CreateRiderAppeal(ctx, server.store, logic.CreateRiderAppealInput{
		RiderID:          rider.ID,
		ClaimID:          req.ClaimID,
		Reason:           req.Reason,
		AppealWindowDays: AppealWindowDays,
		Now:              time.Now(),
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

	ctx.JSON(status, newAppealResponse(result.Appeal))
}

// listRiderAppeals 骑手查看申诉列表
// @Summary 获取申诉列表
// @Description 骑手查看自己提交的申诉列表
// @Tags 骑手申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} map[string]interface{} "成功返回申诉列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/appeals [get]
func (server *Server) listRiderAppeals(ctx *gin.Context) {
	var req listMerchantAppealsRequest
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

	result, err := logic.ListRiderAppeals(ctx, server.store, logic.ListRiderAppealsInput{
		RiderID: rider.ID,
		Limit:   req.PageSize,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := make([]appealListItem, len(result.Appeals))
	for i, a := range result.Appeals {
		response[i] = appealListItem{
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

	ctx.JSON(http.StatusOK, merchantAppealsListResponse{Appeals: response, Total: result.Total, PageID: req.PageID, PageSize: req.PageSize})
}

// getRiderAppealDetail 骑手查看申诉详情
// @Summary 获取申诉详情
// @Description 骑手查看自己提交的申诉详细信息
// @Tags 骑手申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "申诉ID"
// @Success 200 {object} map[string]interface{} "成功返回申诉详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户"
// @Failure 404 {object} map[string]interface{} "申诉不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/appeals/{id} [get]
func (server *Server) getRiderAppealDetail(ctx *gin.Context) {
	appealID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid appeal id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	result, err := logic.GetRiderAppealDetail(ctx, server.store, logic.GetRiderAppealDetailInput{
		AppealID: appealID,
		RiderID:  rider.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := appealDetailResponse{
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

// ========================= Operator Appeals ============================

type listOperatorAppealsRequest struct {
	RegionID *int64  `form:"region_id" binding:"omitempty,min=1"`
	Status   *string `form:"status" binding:"omitempty,oneof=pending approved rejected"`
	Page     int32   `form:"page" binding:"omitempty,min=1"`
	Limit    int32   `form:"limit" binding:"omitempty,min=1,max=100"`
}

// listOperatorAppeals 运营商查看区域内申诉列表
// @Summary 获取区域内申诉列表
// @Description 运营商查看自己管辖区域内的申诉列表，可按状态筛选
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param status query string false "状态筛选" Enums(pending, approved, rejected)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} map[string]interface{} "成功返回申诉列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非运营商用户"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/appeals [get]
func (server *Server) listOperatorAppeals(ctx *gin.Context) {
	var req listOperatorAppealsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var regionID int64
	if req.RegionID != nil && *req.RegionID > 0 {
		if _, err := server.checkOperatorManagesRegion(ctx, *req.RegionID); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		regionID = *req.RegionID
	} else {
		resolvedRegionID, err := server.getOperatorRegionID(ctx)
		if err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		regionID = resolvedRegionID
	}

	// 设置默认值
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

	appeals, err := server.store.ListOperatorAppeals(ctx, db.ListOperatorAppealsParams{
		RegionID: regionID,
		Column2:  status,
		Limit:    req.Limit,
		Offset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountOperatorAppeals(ctx, db.CountOperatorAppealsParams{
		RegionID: regionID,
		Column2:  status,
	})
	if err != nil {
		total = int64(len(appeals))
	}

	response := make([]operatorAppealListItem, len(appeals))
	for i, a := range appeals {
		response[i] = operatorAppealListItem{
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

	ctx.JSON(http.StatusOK, operatorAppealsListResponse{Appeals: response, Total: total, Page: req.Page, Limit: req.Limit})
}

// listOperatorAppealsSummary 运营商申诉汇总
// @Summary 获取区域申诉汇总
// @Description 返回运营商可管理区域内申诉总数及各状态汇总，供工作台和审批入口使用
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param region_id query int false "区域ID"
// @Success 200 {object} appealSummaryResponse "成功返回申诉汇总"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Router /v1/operator/appeals/summary [get]
func (server *Server) listOperatorAppealsSummary(ctx *gin.Context) {
	var req listOperatorAppealsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var regionID int64
	if req.RegionID != nil && *req.RegionID > 0 {
		if _, err := server.checkOperatorManagesRegion(ctx, *req.RegionID); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		regionID = *req.RegionID
	} else {
		resolvedRegionID, err := server.getOperatorRegionID(ctx)
		if err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		regionID = resolvedRegionID
	}

	countStatus := func(status string) (int64, error) {
		return server.store.CountOperatorAppeals(ctx, db.CountOperatorAppealsParams{
			RegionID: regionID,
			Column2:  status,
		})
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pending, err := countStatus("pending")
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

	ctx.JSON(http.StatusOK, appealSummaryResponse{
		Total:    total,
		Pending:  pending,
		Approved: approved,
		Rejected: rejected,
	})
}

// getOperatorAppealDetail 运营商查看申诉详情
// @Summary 获取申诉详情
// @Description 运营商查看区域内申诉的详细信息，包含索赔、订单、用户信用分等信息
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "申诉ID"
// @Success 200 {object} map[string]interface{} "成功返回申诉详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非运营商用户"
// @Failure 404 {object} map[string]interface{} "申诉不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/appeals/{id} [get]
func (server *Server) getOperatorAppealDetail(ctx *gin.Context) {
	appealID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid appeal id")))
		return
	}

	appeal, err := server.store.GetAppeal(ctx, appealID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, appeal.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	detail, err := server.store.GetOperatorAppealDetail(ctx, db.GetOperatorAppealDetailParams{
		ID:       appealID,
		RegionID: appeal.RegionID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := operatorAppealDetailResponse{
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

type reviewAppealRequest struct {
	Status             string `json:"status" binding:"required,oneof=approved rejected"`
	ReviewNotes        string `json:"review_notes" binding:"required,min=5,max=500"`
	CompensationAmount *int64 `json:"compensation_amount" binding:"omitempty,min=1,max=10000000"` // 可选补偿金额（分），最大10万元
}

// reviewAppeal 运营商审核申诉
// @Summary 审核申诉
// @Description 运营商审核申诉，可仅撤销判责与追偿，也可附带补偿金额；系统会自动更新用户信用分
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "申诉ID"
// @Param request body reviewAppealRequest true "审核请求"
// @Success 200 {object} appealResponse "审核成功"
// @Failure 400 {object} map[string]interface{} "参数错误或申诉已审核"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非运营商用户或申诉不在管辖区域"
// @Failure 404 {object} map[string]interface{} "申诉不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/appeals/{id}/review [post]
func (server *Server) reviewAppeal(ctx *gin.Context) {
	appealID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid appeal id")))
		return
	}

	var req reviewAppealRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getOperatorFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	// 验证申诉属于该运营商的区域
	appeal, err := server.store.GetAppeal(ctx, appealID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, err := server.checkOperatorManagesRegion(ctx, appeal.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("this appeal is not in your region")))
		return
	}

	if appeal.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("appeal has already been reviewed")))
		return
	}

	// 批准申诉时可选附带补偿金额；未提供则仅撤销判责与追偿。
	var compensationAmount pgtype.Int8
	if req.Status == "approved" && req.CompensationAmount != nil {
		compensationAmount = pgtype.Int8{Int64: *req.CompensationAmount, Valid: true}
	}
	if req.Status == "approved" && compensationAmount.Valid && compensationAmount.Int64 > 0 && server.transferClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrAppealCompensationUnavailable))
		return
	}

	// 审核申诉，并在同一事务内持久化申诉补偿动作。
	reviewResult, err := server.store.ReviewAppealWithCompensationTx(ctx, db.ReviewAppealWithCompensationTxParams{
		ID:                 appealID,
		Status:             req.Status,
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: req.ReviewNotes, Valid: true},
		CompensationAmount: compensationAmount,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedAppeal := reviewResult.Appeal

	if payload, ok := ctx.Get(authorizationPayloadKey); ok {
		actor := payload.(*token.Payload)
		metadata := map[string]any{
			"status":       req.Status,
			"review_notes": req.ReviewNotes,
		}
		if req.CompensationAmount != nil {
			metadata["compensation_amount"] = *req.CompensationAmount
		}
		regionID := appeal.RegionID
		server.writeAuditLog(ctx, AuditLogInput{
			ActorUserID: actor.UserID,
			ActorRole:   "operator",
			Action:      "operator_appeal_reviewed",
			TargetType:  "appeal",
			TargetID:    &appealID,
			RegionID:    &regionID,
			Metadata:    metadata,
		})
	}

	// 分发异步任务处理后续逻辑（信用分更新、通知等）
	taskPayload := &worker.ProcessAppealResultPayload{
		AppealID: appealID,
		ClaimID:  reviewResult.PostProcess.ClaimID,
		CompensationActionID: func() int64 {
			if reviewResult.CompensationAction != nil {
				return reviewResult.CompensationAction.ID
			}
			return 0
		}(),
		Status:             req.Status,
		AppellantType:      reviewResult.PostProcess.AppellantType,
		AppellantID:        reviewResult.PostProcess.AppellantID,
		ClaimantUserID:     reviewResult.PostProcess.ClaimantUserID,
		ClaimType:          reviewResult.PostProcess.ClaimType,
		ClaimAmount:        reviewResult.PostProcess.ClaimAmount,
		CompensationAmount: compensationAmount.Int64,
		OrderNo:            reviewResult.PostProcess.OrderNo,
	}

	if server.taskDistributor == nil {
		if inlineErr := server.processAppealResultInline(ctx, taskPayload); inlineErr != nil {
			if !writeLogicRequestError(ctx, inlineErr) {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, inlineErr))
			}
			return
		}
	} else {
		if err := server.taskDistributor.DistributeTaskProcessAppealResult(ctx, taskPayload); err != nil {
			if inlineErr := server.processAppealResultInline(ctx, taskPayload); inlineErr != nil {
				if !writeLogicRequestError(ctx, inlineErr) {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, inlineErr))
				}
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, newAppealResponse(updatedAppeal))
}

func (server *Server) processAppealResultInline(ctx *gin.Context, payload *worker.ProcessAppealResultPayload) error {
	switch payload.Status {
	case "approved":
		if err := server.waiveClaimRecoveryInline(ctx, payload); err != nil {
			return err
		}
		if payload.CompensationActionID > 0 {
			if err := worker.ExecuteClaimPayoutAction(ctx, server.store, server.transferClient, payload.CompensationActionID); err != nil {
				mappedErr := logic.MapClaimPayoutTransferExecutionError(err)
				log.Error().Err(logic.LoggableError(mappedErr)).Int64("appeal_id", payload.AppealID).Int64("behavior_action_id", payload.CompensationActionID).Msg("failed to execute appeal compensation action inline")
				return mappedErr
			}
		}
	case "rejected":
		if err := server.resumeClaimRecoveryInline(ctx, payload); err != nil {
			return err
		}
	}

	appellantUserID, err := server.getAppellantUserID(ctx, payload.AppellantType, payload.AppellantID)
	if err != nil {
		return err
	}

	appellantTitle, appellantContent := buildAppealNotificationContent(payload, true)
	if err := server.SendNotification(ctx, SendNotificationParams{
		UserID:      appellantUserID,
		Type:        "appeal",
		Title:       appellantTitle,
		Content:     appellantContent,
		RelatedType: "appeal",
		RelatedID:   payload.AppealID,
		ExtraData: map[string]any{
			"appeal_id":      payload.AppealID,
			"status":         payload.Status,
			"appellant_type": payload.AppellantType,
		},
	}); err != nil {
		return err
	}

	claimantTitle, claimantContent := buildAppealNotificationContent(payload, false)
	if err := server.SendNotification(ctx, SendNotificationParams{
		UserID:      payload.ClaimantUserID,
		Type:        "appeal",
		Title:       claimantTitle,
		Content:     claimantContent,
		RelatedType: "appeal",
		RelatedID:   payload.AppealID,
		ExtraData: map[string]any{
			"appeal_id": payload.AppealID,
			"status":    payload.Status,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (server *Server) waiveClaimRecoveryInline(ctx *gin.Context, payload *worker.ProcessAppealResultPayload) error {
	if payload.ClaimID == 0 {
		return nil
	}

	recovery, err := server.store.GetClaimRecoveryByClaimID(ctx, payload.ClaimID)
	if err != nil {
		return nil
	}
	if recovery.Status != "appealed" {
		return nil
	}

	if _, err := server.store.MarkClaimRecoveryWaived(ctx, recovery.ID); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "merchant" {
		order, orderErr := server.store.GetOrder(ctx, recovery.OrderID)
		if orderErr != nil {
			return orderErr
		}
		if err := server.store.UnsuspendMerchantTakeout(ctx, order.MerchantID); err != nil {
			return err
		}
	}

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "rider" {
		delivery, deliveryErr := server.store.GetDeliveryByOrderID(ctx, recovery.OrderID)
		if deliveryErr != nil {
			return deliveryErr
		}
		if delivery.RiderID.Valid {
			if err := server.store.UnsuspendRider(ctx, delivery.RiderID.Int64); err != nil {
				return err
			}
		}
	}

	return nil
}

func (server *Server) resumeClaimRecoveryInline(ctx *gin.Context, payload *worker.ProcessAppealResultPayload) error {
	if payload.ClaimID == 0 {
		return nil
	}

	recovery, err := server.store.GetClaimRecoveryByClaimID(ctx, payload.ClaimID)
	if err != nil {
		return nil
	}
	if recovery.Status != "appealed" {
		return nil
	}

	_, err = server.store.ResumeClaimRecoveryAfterAppeal(ctx, recovery.ID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return err
	}
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

func buildAppealNotificationContent(payload *worker.ProcessAppealResultPayload, isAppellant bool) (string, string) {
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
