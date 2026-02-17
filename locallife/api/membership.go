package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 会员管理 ====================

// joinMembershipRequest 加入会员请求
type joinMembershipRequest struct {
	MerchantID int64 `json:"merchant_id" binding:"required,min=1"`
}

// membershipResponse 会员信息响应
type membershipResponse struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchant_id"`
	MerchantName   string     `json:"merchant_name,omitempty"`
	LogoURL        string     `json:"logo_url,omitempty"`
	UserID         int64      `json:"user_id"`
	Balance        int64      `json:"balance"`
	TotalRecharged int64      `json:"total_recharged"`
	TotalConsumed  int64      `json:"total_consumed"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

// joinMembership godoc
// @Summary 加入会员
// @Description 加入商户的会员计划
// @Tags 会员管理
// @Accept json
// @Produce json
// @Param request body joinMembershipRequest true "商户ID"
// @Success 200 {object} membershipResponse "加入成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/memberships [post]
// @Security BearerAuth
func (server *Server) joinMembership(ctx *gin.Context) {
	var req joinMembershipRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result, err := server.store.JoinMembershipTx(ctx, db.JoinMembershipTxParams{
		MerchantID: req.MerchantID,
		UserID:     authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := membershipResponse{
		ID:             result.Membership.ID,
		MerchantID:     result.Membership.MerchantID,
		MerchantName:   merchant.Name,
		UserID:         result.Membership.UserID,
		Balance:        result.Membership.Balance,
		TotalRecharged: result.Membership.TotalRecharged,
		TotalConsumed:  result.Membership.TotalConsumed,
		CreatedAt:      result.Membership.CreatedAt,
	}
	if merchant.LogoUrl.Valid {
		rsp.LogoURL = normalizeUploadURLForClient(merchant.LogoUrl.String)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// listUserMembershipsRequest 获取用户会员列表请求
type listUserMembershipsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

type listUserMembershipsResponse struct {
	Memberships []membershipResponse `json:"memberships"`
	TotalCount  int64                `json:"total_count"`
	Total       int64                `json:"total"`
	PageID      int32                `json:"page_id"`
	PageSize    int32                `json:"page_size"`
}

// listUserMemberships godoc
// @Summary 获取用户会员列表
// @Description 获取当前用户的所有会员卡
// @Tags 会员管理
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} listUserMembershipsResponse "会员列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/memberships [get]
// @Security BearerAuth
func (server *Server) listUserMemberships(ctx *gin.Context) {
	var req listUserMembershipsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	memberships, err := server.store.ListUserMemberships(ctx, db.ListUserMembershipsParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]membershipResponse, len(memberships))
	for i, m := range memberships {
		rsp[i] = convertUserMembershipResponse(m)
	}

	ctx.JSON(http.StatusOK, listUserMembershipsResponse{
		Memberships: rsp,
		TotalCount:  int64(len(rsp)),
		Total:       int64(len(rsp)),
		PageID:      req.PageID,
		PageSize:    req.PageSize,
	})
}

// getMembershipRequest 获取会员详情请求
type getMembershipRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getMembership godoc
// @Summary 获取会员详情
// @Description 获取会员卡的详细信息
// @Tags 会员管理
// @Produce json
// @Param id path int true "会员ID"
// @Success 200 {object} membershipResponse "会员详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非会员所有者"
// @Failure 404 {object} ErrorResponse "会员不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/memberships/{id} [get]
// @Security BearerAuth
func (server *Server) getMembership(ctx *gin.Context) {
	var req getMembershipRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	membership, err := logic.GetMembershipForUser(ctx, server.store, logic.MembershipAccessInput{
		UserID:       authPayload.UserID,
		MembershipID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := membershipResponse{
		ID:             membership.ID,
		MerchantID:     membership.MerchantID,
		UserID:         membership.UserID,
		Balance:        membership.Balance,
		TotalRecharged: membership.TotalRecharged,
		TotalConsumed:  membership.TotalConsumed,
		CreatedAt:      membership.CreatedAt,
	}

	ctx.JSON(http.StatusOK, rsp)
}

// ==================== 充值规则管理 ====================

// createRechargeRuleURIRequest 创建充值规则URI参数
type createRechargeRuleURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// createRechargeRuleRequest 创建充值规则请求
type createRechargeRuleRequest struct {
	RechargeAmount int64     `json:"recharge_amount" binding:"required,min=1,max=100000000"` // 充值金额(分),最大100万元
	BonusAmount    int64     `json:"bonus_amount" binding:"required,min=0,max=100000000"`    // 赠送金额(分),最大100万元
	ValidFrom      time.Time `json:"valid_from" binding:"required"`
	ValidUntil     time.Time `json:"valid_until" binding:"required"`
}

// rechargeRuleResponse 充值规则响应
type rechargeRuleResponse struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchant_id"`
	RechargeAmount int64      `json:"recharge_amount"`
	BonusAmount    int64      `json:"bonus_amount"`
	IsActive       bool       `json:"is_active"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidUntil     time.Time  `json:"valid_until"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

// createRechargeRule godoc
// @Summary 创建充值规则
// @Description 商户创建充值规则（含赠送）
// @Tags 会员管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body createRechargeRuleRequest true "充值规则信息"
// @Success 200 {object} rechargeRuleResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/recharge-rules [post]
// @Security BearerAuth
func (server *Server) createRechargeRule(ctx *gin.Context) {
	var uriReq createRechargeRuleURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req createRechargeRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context required")))
		return
	}
	if merchant.ID != uriReq.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	rule, err := logic.CreateRechargeRule(ctx, server.store, logic.CreateRechargeRuleInput{
		MerchantID:     merchant.ID,
		RechargeAmount: req.RechargeAmount,
		BonusAmount:    req.BonusAmount,
		ValidFrom:      req.ValidFrom,
		ValidUntil:     req.ValidUntil,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, convertRechargeRuleResponse(rule))
}

// listRechargeRulesRequest 获取充值规则列表请求
type listRechargeRulesRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// listRechargeRules godoc
// @Summary 获取充值规则列表
// @Description 获取商户的所有充值规则
// @Tags 会员管理
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {array} rechargeRuleResponse "规则列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/recharge-rules [get]
// @Security BearerAuth
func (server *Server) listRechargeRules(ctx *gin.Context) {
	var req listRechargeRulesRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context required")))
		return
	}

	rules, err := logic.ListMerchantRechargeRules(ctx, server.store, logic.MerchantRechargeRulesInput{
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

	rsp := make([]rechargeRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertRechargeRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// listActiveRechargeRulesRequest 获取有效充值规则请求
type listActiveRechargeRulesRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// listActiveRechargeRules godoc
// @Summary 获取生效中的充值规则
// @Description 获取商户当前生效中的充值规则（已激活且在有效期内）
// @Tags 会员管理
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {array} rechargeRuleResponse "规则列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/recharge-rules/active [get]
// @Security BearerAuth
func (server *Server) listActiveRechargeRules(ctx *gin.Context) {
	var req listActiveRechargeRulesRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context required")))
		return
	}

	rules, err := logic.ListActiveRechargeRules(ctx, server.store, logic.MerchantRechargeRulesInput{
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

	rsp := make([]rechargeRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertRechargeRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// getPublicRechargeRulesRequest 获取公开充值规则请求（C端）
type getPublicRechargeRulesRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// getPublicRechargeRules godoc
// @Summary 获取商户的生效中充值规则（C端公开）
// @Description 获取商户当前生效中的充值规则，供C端用户查看充值活动
// @Tags 会员管理
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {array} rechargeRuleResponse "规则列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/public/merchants/{id}/recharge-rules [get]
// @Security BearerAuth
func (server *Server) getPublicRechargeRules(ctx *gin.Context) {
	var req getPublicRechargeRulesRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules, err := logic.GetPublicRechargeRules(ctx, server.store, req.MerchantID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]rechargeRuleResponse, len(rules))
	for i, rule := range rules {
		rsp[i] = convertRechargeRuleResponse(rule)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// updateRechargeRuleURIRequest 更新充值规则路径参数
type updateRechargeRuleURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	RuleID     int64 `uri:"rule_id" binding:"required,min=1"`
}

// updateRechargeRuleRequest 更新充值规则请求
type updateRechargeRuleRequest struct {
	RechargeAmount *int64     `json:"recharge_amount" binding:"omitempty,min=1,max=100000000"`
	BonusAmount    *int64     `json:"bonus_amount" binding:"omitempty,min=0,max=100000000"`
	IsActive       *bool      `json:"is_active"`
	ValidFrom      *time.Time `json:"valid_from"`
	ValidUntil     *time.Time `json:"valid_until"`
}

// updateRechargeRule godoc
// @Summary 更新充值规则
// @Description 商户更新充值规则（仅规则所有者可操作）
// @Tags 会员管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param rule_id path int true "充值规则ID"
// @Param request body updateRechargeRuleRequest true "更新字段"
// @Success 200 {object} rechargeRuleResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非规则所有者"
// @Failure 404 {object} ErrorResponse "规则不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/recharge-rules/{rule_id} [patch]
// @Security BearerAuth
func (server *Server) updateRechargeRule(ctx *gin.Context) {
	var uriReq updateRechargeRuleURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateRechargeRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context required")))
		return
	}

	updatedRule, err := logic.UpdateRechargeRuleForMerchant(ctx, server.store, logic.UpdateRechargeRuleInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: uriReq.MerchantID,
		RuleID:           uriReq.RuleID,
		RechargeAmount:   req.RechargeAmount,
		BonusAmount:      req.BonusAmount,
		IsActive:         req.IsActive,
		ValidFrom:        req.ValidFrom,
		ValidUntil:       req.ValidUntil,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, convertRechargeRuleResponse(updatedRule))
}

func convertUserMembershipResponse(m db.ListUserMembershipsRow) membershipResponse {
	rsp := membershipResponse{
		ID:             m.ID,
		MerchantID:     m.MerchantID,
		MerchantName:   m.MerchantName,
		UserID:         m.UserID,
		Balance:        m.Balance,
		TotalRecharged: m.TotalRecharged,
		TotalConsumed:  m.TotalConsumed,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      pgTimeToPtr(m.UpdatedAt),
	}
	if m.LogoUrl.Valid {
		rsp.LogoURL = normalizeUploadURLForClient(m.LogoUrl.String)
	}
	return rsp
}

func convertRechargeRuleResponse(rule db.RechargeRule) rechargeRuleResponse {
	return rechargeRuleResponse{
		ID:             rule.ID,
		MerchantID:     rule.MerchantID,
		RechargeAmount: rule.RechargeAmount,
		BonusAmount:    rule.BonusAmount,
		IsActive:       rule.IsActive,
		ValidFrom:      rule.ValidFrom,
		ValidUntil:     rule.ValidUntil,
		CreatedAt:      rule.CreatedAt,
		UpdatedAt:      pgTimeToPtr(rule.UpdatedAt),
	}
}

// deleteRechargeRuleURIRequest 删除充值规则路径参数
type deleteRechargeRuleURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	RuleID     int64 `uri:"rule_id" binding:"required,min=1"`
}

// deleteRechargeRule godoc
// @Summary 删除充值规则
// @Description 商户删除充值规则（仅规则所有者可操作）
// @Tags 会员管理-商户
// @Produce json
// @Param id path int true "商户ID"
// @Param rule_id path int true "充值规则ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非规则所有者"
// @Failure 404 {object} ErrorResponse "规则不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/recharge-rules/{rule_id} [delete]
// @Security BearerAuth
func (server *Server) deleteRechargeRule(ctx *gin.Context) {
	var req deleteRechargeRuleURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context required")))
		return
	}

	err := logic.DeleteRechargeRuleForMerchant(ctx, server.store, logic.DeleteRechargeRuleInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: req.MerchantID,
		RuleID:           req.RuleID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "rule deleted successfully"})
}

// ==================== 会员充值 ====================

// rechargeRequest 充值请求
type rechargeRequest struct {
	MembershipID   int64  `json:"membership_id" binding:"required,min=1"`
	RechargeAmount int64  `json:"recharge_amount" binding:"required,min=1,max=100000000"` // 最大100万元
	PaymentMethod  string `json:"payment_method" binding:"required,oneof=wechat"`
}

// transactionResponse 交易流水响应
type transactionResponse struct {
	ID             int64     `json:"id"`
	MembershipID   int64     `json:"membership_id"`
	Type           string    `json:"type"`
	Amount         int64     `json:"amount"`
	BalanceAfter   int64     `json:"balance_after"`
	RelatedOrderID *int64    `json:"related_order_id,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// rechargeMembership godoc
// @Summary 会员卡充值
// @Description 用户为会员卡充值，支持微信/支付宝支付，自动匹配赠送规则
// @Tags 会员管理
// @Accept json
// @Produce json
// @Param request body rechargeRequest true "充值信息"
// @Success 200 {object} object "支付参数和交易信息"
// @Failure 400 {object} ErrorResponse "参数错误或用户未绑定微信"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非会员卡所有者"
// @Failure 404 {object} ErrorResponse "会员卡不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/memberships/recharge [post]
// @Security BearerAuth
func (server *Server) rechargeMembership(ctx *gin.Context) {
	var req rechargeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rechargeCtx, err := logic.PrepareMembershipRecharge(ctx, server.store, logic.MembershipRechargeInput{
		UserID:         authPayload.UserID,
		MembershipID:   req.MembershipID,
		RechargeAmount: req.RechargeAmount,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	membership := rechargeCtx.Membership
	bonusAmount := rechargeCtx.BonusAmount
	ruleID := rechargeCtx.RechargeRuleID

	// 创建支付订单
	outTradeNo := generateOutTradeNoWithPrefix("MBR")
	expireTime := time.Now().Add(5 * time.Minute) // 5分钟过期

	if req.PaymentMethod != "wechat" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("unsupported payment method")))
		return
	}
	if server.paymentClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("payment service not available")))
		return
	}

	// 获取用户OpenID（从users表）
	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to get user: %w", err)))
		return
	}

	if user.WechatOpenid == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("user wechat openid not found")))
		return
	}

	// 准备attach数据
	attachPayload := struct {
		MembershipID   int64  `json:"membership_id"`
		BonusAmount    int64  `json:"bonus_amount"`
		RechargeRuleID *int64 `json:"recharge_rule_id"`
	}{
		MembershipID:   req.MembershipID,
		BonusAmount:    bonusAmount,
		RechargeRuleID: ruleID,
	}
	attachBytes, err := json.Marshal(attachPayload)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to marshal attach payload: %w", err)))
		return
	}
	attachStr := string(attachBytes)

	// 创建支付订单记录（out_trade_no 碰撞重试）
	var paymentOrder db.PaymentOrder
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		outTradeNo = generateOutTradeNoWithPrefix("MBR")
		paymentOrder, err = server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
			UserID:       authPayload.UserID,
			PaymentType:  PaymentTypeMiniProgram,
			BusinessType: BusinessTypeMembershipRecharge,
			Amount:       req.RechargeAmount,
			OutTradeNo:   outTradeNo,
			ExpiresAt:    pgtype.Timestamptz{Time: expireTime, Valid: true},
			Attach:       pgtype.Text{String: attachStr, Valid: true},
		})
		if err == nil {
			break
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx.Request.Context(), outTradeNoRetryBaseBack*time.Duration(attempt)) {
				ctx.JSON(http.StatusRequestTimeout, errorResponse(errors.New("request canceled")))
				return
			}
			continue
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to create payment order: %w", err)))
		return
	}

	// 调用微信支付API创建JSAPI订单
	description := fmt.Sprintf("会员充值 - 商户ID:%d", membership.MerchantID)
	jsapiReq := &wechat.JSAPIOrderRequest{
		OutTradeNo:    outTradeNo,
		Description:   description,
		TotalAmount:   req.RechargeAmount,
		OpenID:        user.WechatOpenid,
		ExpireTime:    expireTime,
		Attach:        attachStr,
		PayerClientIP: ctx.ClientIP(),
	}

	_, payParams, err := server.paymentClient.CreateJSAPIOrder(ctx, jsapiReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to create wechat order: %w", err)))
		return
	}

	// 返回支付参数给前端，前端调用wx.requestPayment进行支付
	ctx.JSON(http.StatusOK, gin.H{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     outTradeNo,
		"pay_params":       payParams,
	})
}

// ==================== 交易流水 ====================

// listTransactionsRequest 获取交易流水请求
type listTransactionsRequest struct {
	MembershipID int64 `uri:"id" binding:"required,min=1"`
	PageID       int32 `form:"page_id" binding:"required,min=1"`
	PageSize     int32 `form:"page_size" binding:"required,min=5,max=50"`
}

type listMembershipTransactionsResponse struct {
	Transactions []transactionResponse `json:"transactions"`
	TotalCount   int64                 `json:"total_count"`
	Total        int64                 `json:"total"`
	PageID       int32                 `json:"page_id"`
	PageSize     int32                 `json:"page_size"`
}

// listMembershipTransactions godoc
// @Summary 获取会员交易流水
// @Description 获取会员卡的交易历史记录（充值、消费等）
// @Tags 会员管理
// @Produce json
// @Param id path int true "会员ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} listMembershipTransactionsResponse "交易流水列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非会员卡所有者"
// @Failure 404 {object} ErrorResponse "会员卡不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/memberships/{id}/transactions [get]
// @Security BearerAuth
func (server *Server) listMembershipTransactions(ctx *gin.Context) {
	var req listTransactionsRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	transactions, err := logic.ListMembershipTransactionsForUser(ctx, server.store, logic.MembershipTransactionsInput{
		UserID:       authPayload.UserID,
		MembershipID: req.MembershipID,
		Limit:        req.PageSize,
		Offset:       pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	rsp := make([]transactionResponse, len(transactions))
	for i, tx := range transactions {
		rsp[i] = convertTransaction(tx)
	}

	ctx.JSON(http.StatusOK, listMembershipTransactionsResponse{
		Transactions: rsp,
		TotalCount:   int64(len(rsp)),
		Total:        int64(len(rsp)),
		PageID:       req.PageID,
		PageSize:     req.PageSize,
	})
}

// ==================== 辅助函数 ====================

// convertTransaction 转换交易记录为响应格式
func convertTransaction(tx db.MembershipTransaction) transactionResponse {
	rsp := transactionResponse{
		ID:           tx.ID,
		MembershipID: tx.MembershipID,
		Type:         tx.Type,
		Amount:       tx.Amount,
		BalanceAfter: tx.BalanceAfter,
		CreatedAt:    tx.CreatedAt,
	}

	if tx.RelatedOrderID.Valid {
		rsp.RelatedOrderID = &tx.RelatedOrderID.Int64
	}

	if tx.Notes.Valid {
		rsp.Notes = tx.Notes.String
	}

	return rsp
}

// ==================== 商户会员设置 ====================

type membershipSettingsResponse struct {
	MerchantID          int64    `json:"merchant_id"`
	BalanceUsableScenes []string `json:"balance_usable_scenes"`
	BonusUsableScenes   []string `json:"bonus_usable_scenes"`
	AllowWithVoucher    bool     `json:"allow_with_voucher"`
	AllowWithDiscount   bool     `json:"allow_with_discount"`
	MaxDeductionPercent int32    `json:"max_deduction_percent"`
}

// getMerchantMembershipSettings godoc
// @Summary 获取商户会员设置
// @Description 获取当前商户的会员使用设置（余额使用场景、赠送金使用场景、叠加优惠等）
// @Tags 会员管理-商户
// @Produce json
// @Success 200 {object} membershipSettingsResponse "会员设置"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/me/membership-settings [get]
// @Security BearerAuth
func (server *Server) getMerchantMembershipSettings(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	settings, err := logic.GetMembershipSettingsForOwner(ctx, server.store, authPayload.UserID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, membershipSettingsResponse{
		MerchantID:          settings.MerchantID,
		BalanceUsableScenes: settings.BalanceUsableScenes,
		BonusUsableScenes:   settings.BonusUsableScenes,
		AllowWithVoucher:    settings.AllowWithVoucher,
		AllowWithDiscount:   settings.AllowWithDiscount,
		MaxDeductionPercent: settings.MaxDeductionPercent,
	})
}

type updateMembershipSettingsRequest struct {
	BalanceUsableScenes []string `json:"balance_usable_scenes" binding:"omitempty,dive,oneof=dine_in takeout reservation"`
	BonusUsableScenes   []string `json:"bonus_usable_scenes" binding:"omitempty,dive,oneof=dine_in takeout reservation"`
	AllowWithVoucher    *bool    `json:"allow_with_voucher"`
	AllowWithDiscount   *bool    `json:"allow_with_discount"`
	MaxDeductionPercent *int32   `json:"max_deduction_percent" binding:"omitempty,min=1,max=100"`
}

// updateMerchantMembershipSettings godoc
// @Summary 更新商户会员设置
// @Description 更新当前商户的会员使用设置（余额使用场景、赠送金使用场景、叠加优惠等）
// @Tags 会员管理-商户
// @Accept json
// @Produce json
// @Param request body updateMembershipSettingsRequest true "会员设置"
// @Success 200 {object} membershipSettingsResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/me/membership-settings [put]
// @Security BearerAuth
func (server *Server) updateMerchantMembershipSettings(ctx *gin.Context) {
	var req updateMembershipSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	settings, err := logic.UpdateMembershipSettingsForOwner(ctx, server.store, logic.UpdateMembershipSettingsInput{
		OwnerUserID:         authPayload.UserID,
		BalanceUsableScenes: req.BalanceUsableScenes,
		BonusUsableScenes:   req.BonusUsableScenes,
		AllowWithVoucher:    req.AllowWithVoucher,
		AllowWithDiscount:   req.AllowWithDiscount,
		MaxDeductionPercent: req.MaxDeductionPercent,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, membershipSettingsResponse{
		MerchantID:          settings.MerchantID,
		BalanceUsableScenes: settings.BalanceUsableScenes,
		BonusUsableScenes:   settings.BonusUsableScenes,
		AllowWithVoucher:    settings.AllowWithVoucher,
		AllowWithDiscount:   settings.AllowWithDiscount,
		MaxDeductionPercent: settings.MaxDeductionPercent,
	})
}

// ==================== 商户会员管理 ====================

// listMerchantMembersUriRequest 获取商户会员列表 URI 参数
type listMerchantMembersUriRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// listMerchantMembersQueryRequest 获取商户会员列表 Query 参数
type listMerchantMembersQueryRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

type listMerchantMembersResponse struct {
	Members    []merchantMemberResponse `json:"members"`
	TotalCount int64                    `json:"total_count"`
	Total      int64                    `json:"total"`
	PageID     int32                    `json:"page_id"`
	PageSize   int32                    `json:"page_size"`
}

// merchantMemberResponse 商户会员响应
type merchantMemberResponse struct {
	UserID         int64     `json:"user_id"`
	FullName       string    `json:"full_name"`
	Phone          string    `json:"phone"`
	AvatarURL      string    `json:"avatar_url"`
	MembershipID   int64     `json:"membership_id"`
	Balance        int64     `json:"balance"`
	TotalRecharged int64     `json:"total_recharged"`
	TotalConsumed  int64     `json:"total_consumed"`
	CreatedAt      time.Time `json:"created_at"`
}

// listMerchantMembers godoc
// @Summary 获取商户会员列表
// @Description 商户获取本店所有会员的列表（含余额、消费统计）
// @Tags 会员管理-商户
// @Produce json
// @Param id path int true "商户ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} listMerchantMembersResponse "会员列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/members [get]
// @Security BearerAuth
func (server *Server) listMerchantMembers(ctx *gin.Context) {
	var uriReq listMerchantMembersUriRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var queryReq listMerchantMembersQueryRequest
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}

	members, err := logic.ListMerchantMembers(ctx, server.store, logic.MerchantMembersInput{
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

	rsp := make([]merchantMemberResponse, len(members))
	for i, m := range members {
		phone := ""
		if m.Phone.Valid {
			phone = m.Phone.String
		}
		avatarURL := ""
		if m.AvatarUrl.Valid {
			avatarURL = normalizeUploadURLForClient(m.AvatarUrl.String)
		}
		rsp[i] = merchantMemberResponse{
			UserID:         m.UserID,
			FullName:       m.FullName,
			Phone:          phone,
			AvatarURL:      avatarURL,
			MembershipID:   m.ID,
			Balance:        m.Balance,
			TotalRecharged: m.TotalRecharged,
			TotalConsumed:  m.TotalConsumed,
			CreatedAt:      m.CreatedAt,
		}
	}

	ctx.JSON(http.StatusOK, listMerchantMembersResponse{
		Members:    rsp,
		TotalCount: int64(len(rsp)),
		Total:      int64(len(rsp)),
		PageID:     queryReq.PageID,
		PageSize:   queryReq.PageSize,
	})
}

// getMerchantMemberDetailRequest 获取商户会员详情请求
type getMerchantMemberDetailRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	UserID     int64 `uri:"user_id" binding:"required,min=1"`
}

// merchantMemberDetailResponse 商户会员详情响应
type merchantMemberDetailResponse struct {
	UserID         int64                 `json:"user_id"`
	FullName       string                `json:"full_name"`
	Phone          string                `json:"phone"`
	AvatarURL      string                `json:"avatar_url"`
	MembershipID   int64                 `json:"membership_id"`
	Balance        int64                 `json:"balance"`
	TotalRecharged int64                 `json:"total_recharged"`
	TotalConsumed  int64                 `json:"total_consumed"`
	CreatedAt      time.Time             `json:"created_at"`
	Transactions   []transactionResponse `json:"transactions"`
}

// getMerchantMemberDetail godoc
// @Summary 获取商户会员详情
// @Description 商户获取指定会员的详细信息和交易记录
// @Tags 会员管理-商户
// @Produce json
// @Param id path int true "商户ID"
// @Param user_id path int true "用户ID"
// @Success 200 {object} merchantMemberDetailResponse "会员详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 404 {object} ErrorResponse "会员不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/members/{user_id} [get]
// @Security BearerAuth
func (server *Server) getMerchantMemberDetail(ctx *gin.Context) {
	var req getMerchantMemberDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}

	detail, err := logic.GetMerchantMemberDetail(ctx, server.store, logic.MerchantMemberDetailInput{
		MerchantID:        merchant.ID,
		TargetMerchantID:  req.MerchantID,
		UserID:            req.UserID,
		TransactionsLimit: 20,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	membership := detail.Membership
	user := detail.User
	transactions := detail.Transactions

	txRsp := make([]transactionResponse, len(transactions))
	for i, tx := range transactions {
		txRsp[i] = convertTransaction(tx)
	}

	phone := ""
	if user.Phone.Valid {
		phone = user.Phone.String
	}
	avatarURL := ""
	if user.AvatarUrl.Valid {
		avatarURL = normalizeUploadURLForClient(user.AvatarUrl.String)
	}

	ctx.JSON(http.StatusOK, merchantMemberDetailResponse{
		UserID:         user.ID,
		FullName:       user.FullName,
		Phone:          phone,
		AvatarURL:      avatarURL,
		MembershipID:   membership.ID,
		Balance:        membership.Balance,
		TotalRecharged: membership.TotalRecharged,
		TotalConsumed:  membership.TotalConsumed,
		CreatedAt:      membership.CreatedAt,
		Transactions:   txRsp,
	})
}

// adjustMemberBalanceRequest 调整会员余额请求
type adjustMemberBalanceRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	UserID     int64 `uri:"user_id" binding:"required,min=1"`
}

// adjustMemberBalanceBody 调整会员余额请求体
type adjustMemberBalanceBody struct {
	Amount int64  `json:"amount" binding:"required"` // 正数增加，负数扣减
	Notes  string `json:"notes" binding:"required,min=1,max=200"`
}

// adjustMemberBalance godoc
// @Summary 调整会员余额
// @Description 商户调整会员余额（正数为增加/退款，负数为扣减）
// @Tags 会员管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param user_id path int true "用户ID"
// @Param request body adjustMemberBalanceBody true "调整信息"
// @Success 200 {object} merchantMemberResponse "更新后的会员信息"
// @Failure 400 {object} ErrorResponse "参数错误或余额不足"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 404 {object} ErrorResponse "会员不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/members/{user_id}/balance [post]
// @Security BearerAuth
func (server *Server) adjustMemberBalance(ctx *gin.Context) {
	var uriReq adjustMemberBalanceRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var bodyReq adjustMemberBalanceBody
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant context not found")))
		return
	}

	result, err := logic.AdjustMemberBalance(ctx, server.store, logic.AdjustMemberBalanceInput{
		MerchantID:       merchant.ID,
		TargetMerchantID: uriReq.MerchantID,
		UserID:           uriReq.UserID,
		Amount:           bodyReq.Amount,
		Notes:            bodyReq.Notes,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	user := result.User
	phone := ""
	if user.Phone.Valid {
		phone = user.Phone.String
	}
	avatarURL := ""
	if user.AvatarUrl.Valid {
		avatarURL = normalizeUploadURLForClient(user.AvatarUrl.String)
	}

	ctx.JSON(http.StatusOK, merchantMemberResponse{
		UserID:         user.ID,
		FullName:       user.FullName,
		Phone:          phone,
		AvatarURL:      avatarURL,
		MembershipID:   result.Membership.ID,
		Balance:        result.Membership.Balance,
		TotalRecharged: result.Membership.TotalRecharged,
		TotalConsumed:  result.Membership.TotalConsumed,
		CreatedAt:      result.Membership.CreatedAt,
	})
}
