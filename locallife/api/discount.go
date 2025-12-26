package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 满减规则管理 ====================

// createDiscountRuleRequest 创建满减规则请求
type createDiscountRuleRequest struct {
	Name                   string    `json:"name" binding:"required,min=1,max=100"`
	Description            string    `json:"description"`
	MinOrderAmount         int64     `json:"min_order_amount" binding:"required,min=1"`
	DiscountAmount         int64     `json:"discount_amount" binding:"required,min=1"`
	CanStackWithVoucher    bool      `json:"can_stack_with_voucher"`
	CanStackWithMembership bool      `json:"can_stack_with_membership"`
	ValidFrom              time.Time `json:"valid_from" binding:"required"`
	ValidUntil             time.Time `json:"valid_until" binding:"required"`
}

// discountRuleResponse 满减规则响应
type discountRuleResponse struct {
	ID                     int64     `json:"id"`
	MerchantID             int64     `json:"merchant_id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description,omitempty"`
	MinOrderAmount         int64     `json:"min_order_amount"`
	DiscountAmount         int64     `json:"discount_amount"`
	CanStackWithVoucher    bool      `json:"can_stack_with_voucher"`
	CanStackWithMembership bool      `json:"can_stack_with_membership"`
	ValidFrom              time.Time `json:"valid_from"`
	ValidUntil             time.Time `json:"valid_until"`
	IsActive               bool      `json:"is_active"`
	CreatedAt              time.Time `json:"created_at"`
}

// createDiscountRule 创建满减规则
func (server *Server) createDiscountRule(ctx *gin.Context) {
	var req createDiscountRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证时间范围
	if req.ValidUntil.Before(req.ValidFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	// 验证折扣金额小于最低消费
	if req.DiscountAmount >= req.MinOrderAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("discount_amount must be less than min_order_amount")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}

	rule, err := server.store.CreateDiscountRule(ctx, db.CreateDiscountRuleParams{
		MerchantID:             merchantID,
		Name:                   req.Name,
		Description:            pgtype.Text{String: req.Description, Valid: req.Description != ""},
		MinOrderAmount:         req.MinOrderAmount,
		DiscountAmount:         req.DiscountAmount,
		CanStackWithVoucher:    req.CanStackWithVoucher,
		CanStackWithMembership: req.CanStackWithMembership,
		ValidFrom:              req.ValidFrom,
		ValidUntil:             req.ValidUntil,
		IsActive:               true,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(rule)
	ctx.JSON(http.StatusOK, rsp)
}

// getDiscountRuleRequest 获取满减规则请求
type getDiscountRuleRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getDiscountRule 获取满减规则详情
func (server *Server) getDiscountRule(ctx *gin.Context) {
	var req getDiscountRuleRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rule, err := server.store.GetDiscountRule(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("discount rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(rule)
	ctx.JSON(http.StatusOK, rsp)
}

// listMerchantDiscountRulesRequest 获取商户满减规则列表请求
type listMerchantDiscountRulesRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	PageID     int32 `form:"page_id" binding:"required,min=1"`
	PageSize   int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listMerchantDiscountRules 获取商户满减规则列表
func (server *Server) listMerchantDiscountRules(ctx *gin.Context) {
	var req listMerchantDiscountRulesRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules, err := server.store.ListMerchantDiscountRules(ctx, db.ListMerchantDiscountRulesParams{
		MerchantID: req.MerchantID,
		Limit:      req.PageSize,
		Offset:     (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]discountRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertDiscountRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// listActiveDiscountRulesRequest 获取有效满减规则请求
type listActiveDiscountRulesRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// listActiveDiscountRules 获取商户当前有效的满减规则
func (server *Server) listActiveDiscountRules(ctx *gin.Context) {
	var req listActiveDiscountRulesRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules, err := server.store.ListActiveDiscountRules(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]discountRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertDiscountRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// getApplicableDiscountRules 获取订单可使用的满减规则
func (server *Server) getApplicableDiscountRules(ctx *gin.Context) {
	var uriReq struct {
		MerchantID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var queryReq struct {
		OrderAmount int64 `form:"order_amount" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules, err := server.store.GetApplicableDiscountRules(ctx, db.GetApplicableDiscountRulesParams{
		MerchantID:     uriReq.MerchantID,
		MinOrderAmount: queryReq.OrderAmount,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]discountRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertDiscountRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// getBestDiscountRule 获取订单最优满减规则（折扣最大）
func (server *Server) getBestDiscountRule(ctx *gin.Context) {
	var uriReq struct {
		MerchantID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var queryReq struct {
		OrderAmount int64 `form:"order_amount" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rule, err := server.store.GetBestDiscountRule(ctx, db.GetBestDiscountRuleParams{
		MerchantID:     uriReq.MerchantID,
		MinOrderAmount: queryReq.OrderAmount,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("no applicable discount rule found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(rule)
	ctx.JSON(http.StatusOK, rsp)
}

// updateDiscountRuleRequest 更新满减规则请求
type updateDiscountRuleRequest struct {
	ID                     int64      `json:"id" binding:"required,min=1"`
	Name                   *string    `json:"name"`
	Description            *string    `json:"description"`
	MinOrderAmount         *int64     `json:"min_order_amount"`
	DiscountAmount         *int64     `json:"discount_amount"`
	CanStackWithVoucher    *bool      `json:"can_stack_with_voucher"`
	CanStackWithMembership *bool      `json:"can_stack_with_membership"`
	ValidFrom              *time.Time `json:"valid_from"`
	ValidUntil             *time.Time `json:"valid_until"`
	IsActive               *bool      `json:"is_active"`
}

// updateDiscountRule 更新满减规则
func (server *Server) updateDiscountRule(ctx *gin.Context) {
	var req updateDiscountRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取原规则验证权限
	rule, err := server.store.GetDiscountRule(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("discount rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != rule.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	// 构造更新参数
	arg := db.UpdateDiscountRuleParams{
		ID: req.ID,
	}

	if req.Name != nil {
		arg.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		arg.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.MinOrderAmount != nil {
		arg.MinOrderAmount = pgtype.Int8{Int64: *req.MinOrderAmount, Valid: true}
	}
	if req.DiscountAmount != nil {
		arg.DiscountAmount = pgtype.Int8{Int64: *req.DiscountAmount, Valid: true}
	}
	if req.CanStackWithVoucher != nil {
		arg.CanStackWithVoucher = pgtype.Bool{Bool: *req.CanStackWithVoucher, Valid: true}
	}
	if req.CanStackWithMembership != nil {
		arg.CanStackWithMembership = pgtype.Bool{Bool: *req.CanStackWithMembership, Valid: true}
	}
	if req.ValidFrom != nil {
		arg.ValidFrom = pgtype.Timestamptz{Time: *req.ValidFrom, Valid: true}
	}
	if req.ValidUntil != nil {
		arg.ValidUntil = pgtype.Timestamptz{Time: *req.ValidUntil, Valid: true}
	}
	if req.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	updatedRule, err := server.store.UpdateDiscountRule(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(updatedRule)
	ctx.JSON(http.StatusOK, rsp)
}

// deleteDiscountRuleRequest 删除满减规则请求
type deleteDiscountRuleRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteDiscountRule 删除满减规则
func (server *Server) deleteDiscountRule(ctx *gin.Context) {
	var req deleteDiscountRuleRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取规则验证权限
	rule, err := server.store.GetDiscountRule(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("discount rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != rule.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	err = server.store.DeleteDiscountRule(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "discount rule deleted successfully"})
}

// ==================== 辅助函数 ====================

// convertDiscountRuleResponse 转换满减规则为响应格式
func convertDiscountRuleResponse(rule db.DiscountRule) discountRuleResponse {
	rsp := discountRuleResponse{
		ID:                     rule.ID,
		MerchantID:             rule.MerchantID,
		Name:                   rule.Name,
		MinOrderAmount:         rule.MinOrderAmount,
		DiscountAmount:         rule.DiscountAmount,
		CanStackWithVoucher:    rule.CanStackWithVoucher,
		CanStackWithMembership: rule.CanStackWithMembership,
		ValidFrom:              rule.ValidFrom,
		ValidUntil:             rule.ValidUntil,
		IsActive:               rule.IsActive,
		CreatedAt:              rule.CreatedAt,
	}

	if rule.Description.Valid {
		rsp.Description = rule.Description.String
	}

	return rsp
}
