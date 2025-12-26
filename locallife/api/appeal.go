package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
)

// =============================================================================
// Appeal API Handlers
// 申诉功能 - 商户/骑手对索赔的申诉
// =============================================================================

// ========================= Helper Functions ============================

// getMerchantFromUserID 根据用户ID获取商户信息
func (server *Server) getMerchantFromUserID(ctx *gin.Context, userID int64) (db.Merchant, error) {
	merchant, err := server.store.GetMerchantByOwner(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	EvidenceURLs       []string   `json:"evidence_urls,omitempty"`
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
	if a.EvidenceUrls != nil {
		resp.EvidenceURLs = a.EvidenceUrls
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
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=1,max=50"`
}

type merchantClaimResponse struct {
	ID             int64      `json:"id"`
	OrderID        int64      `json:"order_id"`
	OrderNo        string     `json:"order_no"`
	OrderAmount    int64      `json:"order_amount"`
	UserPhone      string     `json:"user_phone"`
	UserName       string     `json:"user_name"`
	ClaimType      string     `json:"claim_type"`
	ClaimAmount    int64      `json:"claim_amount"`
	ApprovedAmount *int64     `json:"approved_amount,omitempty"`
	Description    string     `json:"description"`
	EvidenceURLs   []string   `json:"evidence_urls,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	AppealID       *int64     `json:"appeal_id,omitempty"`
	AppealStatus   *string    `json:"appeal_status,omitempty"`
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

	offset := (req.PageID - 1) * req.PageSize

	claims, err := server.store.ListMerchantClaimsForMerchant(ctx, db.ListMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Limit:      req.PageSize,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountMerchantClaimsForMerchant(ctx, merchant.ID)
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
		if c.EvidenceUrls != nil {
			response[i].EvidenceURLs = c.EvidenceUrls
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
	}

	ctx.JSON(http.StatusOK, gin.H{
		"claims":    response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := gin.H{
		"id":            claim.ID,
		"order_id":      claim.OrderID,
		"order_no":      claim.OrderNo,
		"order_amount":  claim.OrderAmount,
		"user_phone":    claim.UserPhone.String,
		"user_name":     claim.UserName,
		"claim_type":    claim.ClaimType,
		"claim_amount":  claim.ClaimAmount,
		"description":   claim.Description,
		"evidence_urls": claim.EvidenceUrls,
		"status":        claim.Status,
		"created_at":    claim.CreatedAt,
	}
	if claim.ApprovedAmount.Valid {
		response["approved_amount"] = claim.ApprovedAmount.Int64
	}
	if claim.ReviewedAt.Valid {
		response["reviewed_at"] = claim.ReviewedAt.Time
	}
	if claim.AppealID.Valid {
		response["appeal_id"] = claim.AppealID.Int64
		response["appeal_status"] = claim.AppealStatus.String
		if claim.AppealReason.Valid {
			response["appeal_reason"] = claim.AppealReason.String
		}
		if claim.AppealReviewNotes.Valid {
			response["appeal_review_notes"] = claim.AppealReviewNotes.String
		}
	}

	ctx.JSON(http.StatusOK, response)
}

type createMerchantAppealRequest struct {
	ClaimID      int64    `json:"claim_id" binding:"required,min=1"`
	Reason       string   `json:"reason" binding:"required,min=10,max=1000"`
	EvidenceURLs []string `json:"evidence_urls" binding:"omitempty,max=10,dive,url,max=500"`
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

	// 检查索赔是否存在且属于该商户
	claimInfo, err := server.store.GetClaimForAppeal(ctx, req.ClaimID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found or not eligible for appeal")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if claimInfo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("this claim does not belong to your merchant")))
		return
	}

	// 检查是否已有申诉
	exists, err := server.store.CheckAppealExists(ctx, req.ClaimID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if exists {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("appeal already exists for this claim")))
		return
	}

	// 规范化证据图片URL
	for i, url := range req.EvidenceURLs {
		req.EvidenceURLs[i] = normalizeImageURLForStorage(url)
	}

	// 创建申诉
	appeal, err := server.store.CreateAppeal(ctx, db.CreateAppealParams{
		ClaimID:       req.ClaimID,
		AppellantType: "merchant",
		AppellantID:   merchant.ID,
		Reason:        req.Reason,
		EvidenceUrls:  req.EvidenceURLs,
		RegionID:      claimInfo.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newAppealResponse(appeal))
}

type listMerchantAppealsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=1,max=50"`
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

	offset := (req.PageID - 1) * req.PageSize

	appeals, err := server.store.ListMerchantAppealsForMerchant(ctx, db.ListMerchantAppealsForMerchantParams{
		AppellantID: merchant.ID,
		Limit:       req.PageSize,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountMerchantAppealsForMerchant(ctx, merchant.ID)
	if err != nil {
		total = int64(len(appeals))
	}

	response := make([]gin.H, len(appeals))
	for i, a := range appeals {
		response[i] = gin.H{
			"id":                a.ID,
			"claim_id":          a.ClaimID,
			"claim_type":        a.ClaimType,
			"claim_amount":      a.ClaimAmount,
			"claim_description": a.ClaimDescription,
			"order_no":          a.OrderNo,
			"reason":            a.Reason,
			"status":            a.Status,
			"created_at":        a.CreatedAt,
		}
		if a.EvidenceUrls != nil {
			response[i]["evidence_urls"] = a.EvidenceUrls
		}
		if a.ReviewerID.Valid {
			response[i]["reviewer_id"] = a.ReviewerID.Int64
		}
		if a.ReviewNotes.Valid {
			response[i]["review_notes"] = a.ReviewNotes.String
		}
		if a.ReviewedAt.Valid {
			response[i]["reviewed_at"] = a.ReviewedAt.Time
		}
		if a.CompensationAmount.Valid {
			response[i]["compensation_amount"] = a.CompensationAmount.Int64
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"appeals":   response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
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

	appeal, err := server.store.GetMerchantAppealDetail(ctx, db.GetMerchantAppealDetailParams{
		ID:          appealID,
		AppellantID: merchant.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := gin.H{
		"id":                  appeal.ID,
		"claim_id":            appeal.ClaimID,
		"claim_type":          appeal.ClaimType,
		"claim_amount":        appeal.ClaimAmount,
		"claim_description":   appeal.ClaimDescription,
		"claim_evidence_urls": appeal.ClaimEvidenceUrls,
		"order_no":            appeal.OrderNo,
		"order_amount":        appeal.OrderAmount,
		"user_phone":          appeal.UserPhone.String,
		"appellant_type":      appeal.AppellantType,
		"reason":              appeal.Reason,
		"evidence_urls":       appeal.EvidenceUrls,
		"status":              appeal.Status,
		"created_at":          appeal.CreatedAt,
	}
	if appeal.ClaimApprovedAmount.Valid {
		response["claim_approved_amount"] = appeal.ClaimApprovedAmount.Int64
	}
	if appeal.ReviewerID.Valid {
		response["reviewer_id"] = appeal.ReviewerID.Int64
	}
	if appeal.ReviewNotes.Valid {
		response["review_notes"] = appeal.ReviewNotes.String
	}
	if appeal.ReviewedAt.Valid {
		response["reviewed_at"] = appeal.ReviewedAt.Time
	}
	if appeal.CompensationAmount.Valid {
		response["compensation_amount"] = appeal.CompensationAmount.Int64
	}
	if appeal.CompensatedAt.Valid {
		response["compensated_at"] = appeal.CompensatedAt.Time
	}

	ctx.JSON(http.StatusOK, response)
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

	offset := (req.PageID - 1) * req.PageSize

	claims, err := server.store.ListRiderClaimsForRider(ctx, db.ListRiderClaimsForRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Limit:   req.PageSize,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountRiderClaimsForRider(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
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
		if c.EvidenceUrls != nil {
			response[i].EvidenceURLs = c.EvidenceUrls
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
	}

	ctx.JSON(http.StatusOK, gin.H{
		"claims":    response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := gin.H{
		"id":            claim.ID,
		"order_id":      claim.OrderID,
		"order_no":      claim.OrderNo,
		"order_amount":  claim.OrderAmount,
		"user_phone":    claim.UserPhone.String,
		"user_name":     claim.UserName,
		"claim_type":    claim.ClaimType,
		"claim_amount":  claim.ClaimAmount,
		"description":   claim.Description,
		"evidence_urls": claim.EvidenceUrls,
		"status":        claim.Status,
		"created_at":    claim.CreatedAt,
	}
	if claim.ApprovedAmount.Valid {
		response["approved_amount"] = claim.ApprovedAmount.Int64
	}
	if claim.ReviewedAt.Valid {
		response["reviewed_at"] = claim.ReviewedAt.Time
	}
	if claim.AppealID.Valid {
		response["appeal_id"] = claim.AppealID.Int64
		response["appeal_status"] = claim.AppealStatus.String
		if claim.AppealReason.Valid {
			response["appeal_reason"] = claim.AppealReason.String
		}
		if claim.AppealReviewNotes.Valid {
			response["appeal_review_notes"] = claim.AppealReviewNotes.String
		}
	}

	ctx.JSON(http.StatusOK, response)
}

type createRiderAppealRequest struct {
	ClaimID      int64    `json:"claim_id" binding:"required,min=1"`
	Reason       string   `json:"reason" binding:"required,min=10,max=1000"`
	EvidenceURLs []string `json:"evidence_urls" binding:"omitempty,max=10,dive,url,max=500"`
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

	// 检查索赔是否存在且关联到该骑手的配送单
	claimInfo, err := server.store.GetClaimForAppeal(ctx, req.ClaimID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("claim not found or not eligible for appeal")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证骑手是否是该订单的配送员
	if !claimInfo.RiderID.Valid || claimInfo.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("this claim is not related to your deliveries")))
		return
	}

	// 检查是否已有申诉
	exists, err := server.store.CheckAppealExists(ctx, req.ClaimID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if exists {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("appeal already exists for this claim")))
		return
	}

	// 规范化证据图片URL
	for i, url := range req.EvidenceURLs {
		req.EvidenceURLs[i] = normalizeImageURLForStorage(url)
	}

	// 创建申诉
	appeal, err := server.store.CreateAppeal(ctx, db.CreateAppealParams{
		ClaimID:       req.ClaimID,
		AppellantType: "rider",
		AppellantID:   rider.ID,
		Reason:        req.Reason,
		EvidenceUrls:  req.EvidenceURLs,
		RegionID:      claimInfo.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newAppealResponse(appeal))
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

	offset := (req.PageID - 1) * req.PageSize

	appeals, err := server.store.ListRiderAppeals(ctx, db.ListRiderAppealsParams{
		AppellantID: rider.ID,
		Limit:       req.PageSize,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountRiderAppeals(ctx, rider.ID)
	if err != nil {
		total = int64(len(appeals))
	}

	response := make([]gin.H, len(appeals))
	for i, a := range appeals {
		response[i] = gin.H{
			"id":                a.ID,
			"claim_id":          a.ClaimID,
			"claim_type":        a.ClaimType,
			"claim_amount":      a.ClaimAmount,
			"claim_description": a.ClaimDescription,
			"order_no":          a.OrderNo,
			"reason":            a.Reason,
			"status":            a.Status,
			"created_at":        a.CreatedAt,
		}
		if a.EvidenceUrls != nil {
			response[i]["evidence_urls"] = a.EvidenceUrls
		}
		if a.ReviewerID.Valid {
			response[i]["reviewer_id"] = a.ReviewerID.Int64
		}
		if a.ReviewNotes.Valid {
			response[i]["review_notes"] = a.ReviewNotes.String
		}
		if a.ReviewedAt.Valid {
			response[i]["reviewed_at"] = a.ReviewedAt.Time
		}
		if a.CompensationAmount.Valid {
			response[i]["compensation_amount"] = a.CompensationAmount.Int64
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"appeals":   response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
	})
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

	appeal, err := server.store.GetRiderAppealDetail(ctx, db.GetRiderAppealDetailParams{
		ID:          appealID,
		AppellantID: rider.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := gin.H{
		"id":                  appeal.ID,
		"claim_id":            appeal.ClaimID,
		"claim_type":          appeal.ClaimType,
		"claim_amount":        appeal.ClaimAmount,
		"claim_description":   appeal.ClaimDescription,
		"claim_evidence_urls": appeal.ClaimEvidenceUrls,
		"order_no":            appeal.OrderNo,
		"order_amount":        appeal.OrderAmount,
		"user_phone":          appeal.UserPhone.String,
		"appellant_type":      appeal.AppellantType,
		"reason":              appeal.Reason,
		"evidence_urls":       appeal.EvidenceUrls,
		"status":              appeal.Status,
		"created_at":          appeal.CreatedAt,
	}
	if appeal.ClaimApprovedAmount.Valid {
		response["claim_approved_amount"] = appeal.ClaimApprovedAmount.Int64
	}
	if appeal.ReviewerID.Valid {
		response["reviewer_id"] = appeal.ReviewerID.Int64
	}
	if appeal.ReviewNotes.Valid {
		response["review_notes"] = appeal.ReviewNotes.String
	}
	if appeal.ReviewedAt.Valid {
		response["reviewed_at"] = appeal.ReviewedAt.Time
	}
	if appeal.CompensationAmount.Valid {
		response["compensation_amount"] = appeal.CompensationAmount.Int64
	}
	if appeal.CompensatedAt.Valid {
		response["compensated_at"] = appeal.CompensatedAt.Time
	}

	ctx.JSON(http.StatusOK, response)
}

// ========================= Operator Appeals ============================

type listOperatorAppealsRequest struct {
	Status   *string `form:"status" binding:"omitempty,oneof=pending approved rejected"`
	PageID   int32   `form:"page_id" binding:"required,min=1"`
	PageSize int32   `form:"page_size" binding:"required,min=1,max=50"`
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getOperatorFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	var status string
	if req.Status != nil {
		status = *req.Status
	}

	appeals, err := server.store.ListOperatorAppeals(ctx, db.ListOperatorAppealsParams{
		RegionID: operator.RegionID,
		Column2:  status,
		Limit:    req.PageSize,
		Offset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountOperatorAppeals(ctx, db.CountOperatorAppealsParams{
		RegionID: operator.RegionID,
		Column2:  status,
	})
	if err != nil {
		total = int64(len(appeals))
	}

	response := make([]gin.H, len(appeals))
	for i, a := range appeals {
		response[i] = gin.H{
			"id":                a.ID,
			"claim_id":          a.ClaimID,
			"claim_type":        a.ClaimType,
			"claim_amount":      a.ClaimAmount,
			"claim_description": a.ClaimDescription,
			"order_no":          a.OrderNo,
			"merchant_id":       a.MerchantID,
			"merchant_name":     a.MerchantName,
			"appellant_type":    a.AppellantType,
			"appellant_id":      a.AppellantID,
			"appellant_name":    a.AppellantName,
			"reason":            a.Reason,
			"status":            a.Status,
			"created_at":        a.CreatedAt,
		}
		if a.EvidenceUrls != nil {
			response[i]["evidence_urls"] = a.EvidenceUrls
		}
		if a.ReviewedAt.Valid {
			response[i]["reviewed_at"] = a.ReviewedAt.Time
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"appeals":   response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getOperatorFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	appeal, err := server.store.GetOperatorAppealDetail(ctx, db.GetOperatorAppealDetailParams{
		ID:       appealID,
		RegionID: operator.RegionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := gin.H{
		"id":                  appeal.ID,
		"claim_id":            appeal.ClaimID,
		"claim_type":          appeal.ClaimType,
		"claim_amount":        appeal.ClaimAmount,
		"claim_description":   appeal.ClaimDescription,
		"claim_evidence_urls": appeal.ClaimEvidenceUrls,
		"claim_status":        appeal.ClaimStatus,
		"user_trust_score":    appeal.UserTrustScore,
		"claim_created_at":    appeal.ClaimCreatedAt,
		"order_no":            appeal.OrderNo,
		"order_amount":        appeal.OrderAmount,
		"order_status":        appeal.OrderStatus,
		"order_created_at":    appeal.OrderCreatedAt,
		"merchant_id":         appeal.MerchantID,
		"merchant_name":       appeal.MerchantName,
		"merchant_phone":      appeal.MerchantPhone,
		"user_phone":          appeal.UserPhone.String,
		"user_name":           appeal.UserName,
		"appellant_type":      appeal.AppellantType,
		"appellant_id":        appeal.AppellantID,
		"reason":              appeal.Reason,
		"evidence_urls":       appeal.EvidenceUrls,
		"status":              appeal.Status,
		"region_id":           appeal.RegionID,
		"created_at":          appeal.CreatedAt,
	}
	if appeal.ClaimApprovedAmount.Valid {
		response["claim_approved_amount"] = appeal.ClaimApprovedAmount.Int64
	}
	if appeal.LookbackResult != nil {
		response["lookback_result"] = string(appeal.LookbackResult)
	}
	if appeal.RiderID.Valid {
		response["rider_id"] = appeal.RiderID.Int64
	}
	if appeal.ReviewerID.Valid {
		response["reviewer_id"] = appeal.ReviewerID.Int64
	}
	if appeal.ReviewNotes.Valid {
		response["review_notes"] = appeal.ReviewNotes.String
	}
	if appeal.ReviewedAt.Valid {
		response["reviewed_at"] = appeal.ReviewedAt.Time
	}
	if appeal.CompensationAmount.Valid {
		response["compensation_amount"] = appeal.CompensationAmount.Int64
	}
	if appeal.CompensatedAt.Valid {
		response["compensated_at"] = appeal.CompensatedAt.Time
	}

	ctx.JSON(http.StatusOK, response)
}

type reviewAppealRequest struct {
	Status             string `json:"status" binding:"required,oneof=approved rejected"`
	ReviewNotes        string `json:"review_notes" binding:"required,min=5,max=500"`
	CompensationAmount *int64 `json:"compensation_amount" binding:"omitempty,min=1,max=10000000"` // 申诉成功时的补偿金额（分），最大10万元
}

// reviewAppeal 运营商审核申诉
// @Summary 审核申诉
// @Description 运营商审核申诉，批准时需提供补偿金额，系统会自动更新用户信用分
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("appeal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if appeal.RegionID != operator.RegionID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("this appeal is not in your region")))
		return
	}

	if appeal.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("appeal has already been reviewed")))
		return
	}

	// 如果批准申诉，需要提供补偿金额
	var compensationAmount pgtype.Int8
	if req.Status == "approved" {
		if req.CompensationAmount == nil || *req.CompensationAmount <= 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("compensation_amount is required for approved appeals")))
			return
		}
		compensationAmount = pgtype.Int8{Int64: *req.CompensationAmount, Valid: true}
	}

	// 审核申诉
	updatedAppeal, err := server.store.ReviewAppeal(ctx, db.ReviewAppealParams{
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

	// 获取申诉后处理所需的信息
	appealInfo, err := server.store.GetAppealForPostProcess(ctx, appealID)
	if err != nil {
		// 不阻塞响应，但记录错误
		// 申诉已成功审核，异步任务可能不会执行
		ctx.JSON(http.StatusOK, newAppealResponse(updatedAppeal))
		return
	}

	// 分发异步任务处理后续逻辑（信用分更新、通知等）
	taskPayload := &worker.ProcessAppealResultPayload{
		AppealID:           appealID,
		Status:             req.Status,
		AppellantType:      appealInfo.AppellantType,
		AppellantID:        appealInfo.AppellantID,
		ClaimantUserID:     appealInfo.ClaimantUserID,
		ClaimType:          appealInfo.ClaimType,
		ClaimAmount:        appealInfo.ClaimAmount,
		CompensationAmount: compensationAmount.Int64,
		OrderNo:            appealInfo.OrderNo,
	}

	if err := server.taskDistributor.DistributeTaskProcessAppealResult(ctx, taskPayload); err != nil {
		// 记录错误但不阻塞响应
		// 申诉已成功审核，异步任务可以手动重试
	}

	ctx.JSON(http.StatusOK, newAppealResponse(updatedAppeal))
}
