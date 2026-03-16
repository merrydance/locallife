package api

import (
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"

	"github.com/gin-gonic/gin"
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
	StackingGroup          *string   `json:"stacking_group" binding:"omitempty,max=50"`
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
	StackingGroup          *string   `json:"stacking_group,omitempty"`
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	rule, err := logic.CreateDiscountRule(ctx, server.store, logic.CreateDiscountRuleInput{
		MerchantID:             merchant.ID,
		Name:                   req.Name,
		Description:            req.Description,
		MinOrderAmount:         req.MinOrderAmount,
		DiscountAmount:         req.DiscountAmount,
		CanStackWithVoucher:    req.CanStackWithVoucher,
		CanStackWithMembership: req.CanStackWithMembership,
		StackingGroup:          req.StackingGroup,
		ValidFrom:              req.ValidFrom,
		ValidUntil:             req.ValidUntil,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(rule)
	ctx.JSON(http.StatusCreated, rsp)
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	rule, err := logic.GetDiscountRuleForMerchant(ctx, server.store, logic.DiscountRuleAccessInput{
		MerchantID: merchant.ID,
		RuleID:     req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertDiscountRuleResponse(rule)
	ctx.JSON(http.StatusOK, rsp)
}

// listMerchantDiscountRulesURIRequest 获取商户满减规则列表URI请求
type listMerchantDiscountRulesURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// listMerchantDiscountRulesQueryRequest 获取商户满减规则列表Query请求
type listMerchantDiscountRulesQueryRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

type listMerchantDiscountRulesResponse struct {
	Rules      []discountRuleResponse `json:"rules"`
	Total      int64                  `json:"total"`
	PageID     int32                  `json:"page_id"`
	PageSize   int32                  `json:"page_size"`
}

// listMerchantDiscountRules 获取商户满减规则列表
func (server *Server) listMerchantDiscountRules(ctx *gin.Context) {
	var uriReq listMerchantDiscountRulesURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var queryReq listMerchantDiscountRulesQueryRequest
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	rules, err := logic.ListMerchantDiscountRules(ctx, server.store, logic.ListMerchantDiscountRulesInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: uriReq.MerchantID,
		Limit:            queryReq.PageSize,
		Offset:           pageOffset(queryReq.PageID, queryReq.PageSize),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]discountRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertDiscountRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, listMerchantDiscountRulesResponse{
		Rules:      rsp,
		Total:      int64(len(rsp)),
		PageID:     queryReq.PageID,
		PageSize:   queryReq.PageSize,
	})
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	rules, err := logic.ListActiveDiscountRules(ctx, server.store, logic.ListActiveDiscountRulesInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: req.MerchantID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	var queryReq struct {
		OrderAmount int64 `form:"order_amount" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules, err := logic.GetApplicableDiscountRules(ctx, server.store, logic.ApplicableDiscountRulesInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: uriReq.MerchantID,
		OrderAmount:      queryReq.OrderAmount,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	var queryReq struct {
		OrderAmount int64 `form:"order_amount" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rule, err := logic.GetBestDiscountRule(ctx, server.store, logic.BestDiscountRuleInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: uriReq.MerchantID,
		OrderAmount:      queryReq.OrderAmount,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
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
	StackingGroup          *string    `json:"stacking_group"`
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	updatedRule, err := logic.UpdateDiscountRuleForMerchant(ctx, server.store, logic.UpdateDiscountRuleInput{
		MerchantID:             merchant.ID,
		RuleID:                 req.ID,
		Name:                   req.Name,
		Description:            req.Description,
		MinOrderAmount:         req.MinOrderAmount,
		DiscountAmount:         req.DiscountAmount,
		CanStackWithVoucher:    req.CanStackWithVoucher,
		CanStackWithMembership: req.CanStackWithMembership,
		StackingGroup:          req.StackingGroup,
		ValidFrom:              req.ValidFrom,
		ValidUntil:             req.ValidUntil,
		IsActive:               req.IsActive,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	merchantVal, exists := ctx.Get(merchantKey)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}
	merchant := merchantVal.(db.Merchant)

	err := logic.DeleteDiscountRuleForMerchant(ctx, server.store, logic.DeleteDiscountRuleInput{
		MerchantID: merchant.ID,
		RuleID:     req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("discount rule deleted successfully"))
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
	if rule.StackingGroup.Valid {
		value := rule.StackingGroup.String
		rsp.StackingGroup = &value
	}

	return rsp
}
