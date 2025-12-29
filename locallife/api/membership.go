package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
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
	ID             int64     `json:"id"`
	MerchantID     int64     `json:"merchant_id"`
	MerchantName   string    `json:"merchant_name,omitempty"`
	UserID         int64     `json:"user_id"`
	Balance        int64     `json:"balance"`
	TotalRecharged int64     `json:"total_recharged"`
	TotalConsumed  int64     `json:"total_consumed"`
	CreatedAt      time.Time `json:"created_at"`
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

	// 验证商户是否存在
	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 使用事务加入会员
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

	ctx.JSON(http.StatusOK, rsp)
}

// listUserMembershipsRequest 获取用户会员列表请求
type listUserMembershipsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listUserMemberships godoc
// @Summary 获取用户会员列表
// @Description 获取当前用户的所有会员卡
// @Tags 会员管理
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} membershipResponse "会员列表"
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
		Offset: (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, memberships)
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

	membership, err := server.store.GetMerchantMembership(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("membership not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：只能查看自己的会员卡
	if membership.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
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
	ID             int64     `json:"id"`
	MerchantID     int64     `json:"merchant_id"`
	RechargeAmount int64     `json:"recharge_amount"`
	BonusAmount    int64     `json:"bonus_amount"`
	IsActive       bool      `json:"is_active"`
	ValidFrom      time.Time `json:"valid_from"`
	ValidUntil     time.Time `json:"valid_until"`
	CreatedAt      time.Time `json:"created_at"`
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}

	// 验证URL中的商户ID与用户的商户ID一致
	if merchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	// 验证时间范围
	if req.ValidUntil.Before(req.ValidFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	rule, err := server.store.CreateRechargeRule(ctx, db.CreateRechargeRuleParams{
		MerchantID:     merchantID,
		RechargeAmount: req.RechargeAmount,
		BonusAmount:    req.BonusAmount,
		IsActive:       true,
		ValidFrom:      req.ValidFrom,
		ValidUntil:     req.ValidUntil,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := rechargeRuleResponse{
		ID:             rule.ID,
		MerchantID:     rule.MerchantID,
		RechargeAmount: rule.RechargeAmount,
		BonusAmount:    rule.BonusAmount,
		IsActive:       rule.IsActive,
		ValidFrom:      rule.ValidFrom,
		ValidUntil:     rule.ValidUntil,
		CreatedAt:      rule.CreatedAt,
	}

	ctx.JSON(http.StatusOK, rsp)
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

	rules, err := server.store.ListMerchantRechargeRules(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, rules)
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

	rules, err := server.store.ListActiveRechargeRules(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, rules)
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取原规则验证权限
	rule, err := server.store.GetRechargeRule(ctx, uriReq.RuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证规则属于当前商户
	if rule.MerchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != rule.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	// 构造更新参数
	arg := db.UpdateRechargeRuleParams{
		ID: uriReq.RuleID,
	}

	if req.RechargeAmount != nil {
		arg.RechargeAmount = pgtype.Int8{Int64: *req.RechargeAmount, Valid: true}
	}
	if req.BonusAmount != nil {
		arg.BonusAmount = pgtype.Int8{Int64: *req.BonusAmount, Valid: true}
	}
	if req.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}
	if req.ValidFrom != nil {
		arg.ValidFrom = pgtype.Timestamptz{Time: *req.ValidFrom, Valid: true}
	}
	if req.ValidUntil != nil {
		arg.ValidUntil = pgtype.Timestamptz{Time: *req.ValidUntil, Valid: true}
	}

	// 验证时间范围（如果两个时间都提供或部分提供）
	effectiveValidFrom := rule.ValidFrom
	effectiveValidUntil := rule.ValidUntil
	if req.ValidFrom != nil {
		effectiveValidFrom = *req.ValidFrom
	}
	if req.ValidUntil != nil {
		effectiveValidUntil = *req.ValidUntil
	}
	if effectiveValidUntil.Before(effectiveValidFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	updatedRule, err := server.store.UpdateRechargeRule(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, updatedRule)
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取规则验证权限
	rule, err := server.store.GetRechargeRule(ctx, req.RuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证规则属于当前商户
	if rule.MerchantID != req.MerchantID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != rule.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	err = server.store.DeleteRechargeRule(ctx, req.RuleID)
	if err != nil {
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
	PaymentMethod  string `json:"payment_method" binding:"required,oneof=wechat alipay"`
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

	// 验证会员卡所有权
	membership, err := server.store.GetMerchantMembership(ctx, req.MembershipID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("membership not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if membership.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	// 查找匹配的充值规则
	matchingRule, err := server.store.GetMatchingRechargeRule(ctx, db.GetMatchingRechargeRuleParams{
		MerchantID:     membership.MerchantID,
		RechargeAmount: req.RechargeAmount,
	})

	var bonusAmount int64
	var ruleID *int64
	if err == nil {
		// 找到匹配的规则，应用赠送
		bonusAmount = matchingRule.BonusAmount
		ruleID = &matchingRule.ID
	} else if errors.Is(err, sql.ErrNoRows) {
		// 没有匹配规则，不赠送
		bonusAmount = 0
		ruleID = nil
	} else {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建支付订单
	outTradeNo := fmt.Sprintf("MBR%d_%d", membership.ID, time.Now().Unix())
	expireTime := time.Now().Add(5 * time.Minute) // 5分钟过期

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
	var ruleIDValue interface{} = nil
	if ruleID != nil {
		ruleIDValue = *ruleID
	}
	attachStr := fmt.Sprintf(`{"membership_id":%d,"bonus_amount":%d,"recharge_rule_id":%v}`,
		req.MembershipID, bonusAmount, ruleIDValue)

	// 创建支付订单记录
	paymentOrder, err := server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		UserID:       authPayload.UserID,
		PaymentType:  "jsapi",
		BusinessType: "membership_recharge",
		Amount:       req.RechargeAmount,
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: expireTime, Valid: true},
		Attach:       pgtype.Text{String: attachStr, Valid: true},
	})
	if err != nil {
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

// listMembershipTransactions godoc
// @Summary 获取会员交易流水
// @Description 获取会员卡的交易历史记录（充值、消费等）
// @Tags 会员管理
// @Produce json
// @Param id path int true "会员ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} transactionResponse "交易流水列表"
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

	// 验证会员卡所有权
	membership, err := server.store.GetMerchantMembership(ctx, req.MembershipID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("membership not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if membership.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	transactions, err := server.store.ListMembershipTransactions(ctx, db.ListMembershipTransactionsParams{
		MembershipID: req.MembershipID,
		Limit:        req.PageSize,
		Offset:       (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	rsp := make([]transactionResponse, len(transactions))
	for i, tx := range transactions {
		rsp[i] = convertTransaction(tx)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// ==================== 辅助函数 ====================

// getMerchantIDByUser 根据用户ID获取商户ID
func (server *Server) getMerchantIDByUser(ctx *gin.Context, userID int64) (int64, error) {
	// 查询用户的商户角色
	role, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
		UserID: userID,
		Role:   "merchant",
	})
	if err != nil {
		return 0, err
	}

	if !role.RelatedEntityID.Valid {
		return 0, errors.New("merchant entity not found")
	}

	return role.RelatedEntityID.Int64, nil
}

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

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取会员设置
	settings, err := server.store.GetMerchantMembershipSettings(ctx, merchant.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 返回默认设置
			ctx.JSON(http.StatusOK, membershipSettingsResponse{
				MerchantID:          merchant.ID,
				BalanceUsableScenes: []string{"dine_in", "takeout", "reservation"},
				BonusUsableScenes:   []string{"dine_in"},
				AllowWithVoucher:    true,
				AllowWithDiscount:   true,
				MaxDeductionPercent: 100,
			})
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

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 设置默认值
	balanceScenes := []string{"dine_in", "takeout", "reservation"}
	bonusScenes := []string{"dine_in"}
	allowVoucher := true
	allowDiscount := true
	maxPercent := int32(100)

	if req.BalanceUsableScenes != nil {
		balanceScenes = req.BalanceUsableScenes
	}
	if req.BonusUsableScenes != nil {
		bonusScenes = req.BonusUsableScenes
	}
	if req.AllowWithVoucher != nil {
		allowVoucher = *req.AllowWithVoucher
	}
	if req.AllowWithDiscount != nil {
		allowDiscount = *req.AllowWithDiscount
	}
	if req.MaxDeductionPercent != nil {
		maxPercent = *req.MaxDeductionPercent
	}

	// Upsert 设置
	settings, err := server.store.UpsertMerchantMembershipSettings(ctx, db.UpsertMerchantMembershipSettingsParams{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: balanceScenes,
		BonusUsableScenes:   bonusScenes,
		AllowWithVoucher:    allowVoucher,
		AllowWithDiscount:   allowDiscount,
		MaxDeductionPercent: maxPercent,
	})
	if err != nil {
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
// @Success 200 {array} merchantMemberResponse "会员列表"
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}
	if merchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	members, err := server.store.ListMerchantMembers(ctx, db.ListMerchantMembersParams{
		MerchantID: merchantID,
		Limit:      queryReq.PageSize,
		Offset:     (queryReq.PageID - 1) * queryReq.PageSize,
	})
	if err != nil {
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
			avatarURL = m.AvatarUrl.String
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

	ctx.JSON(http.StatusOK, rsp)
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}
	if merchantID != req.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	// 获取会员信息
	membership, err := server.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: merchantID,
		UserID:     req.UserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("membership not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取用户信息
	user, err := server.store.GetUser(ctx, req.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取交易记录（最近20条）
	transactions, err := server.store.ListMembershipTransactions(ctx, db.ListMembershipTransactionsParams{
		MembershipID: membership.ID,
		Limit:        20,
		Offset:       0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

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
		avatarURL = user.AvatarUrl.String
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

	if bodyReq.Amount == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("amount cannot be zero")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}
	if merchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	// 获取会员信息
	membership, err := server.store.GetMembershipByMerchantAndUserForUpdate(ctx, db.GetMembershipByMerchantAndUserForUpdateParams{
		MerchantID: merchantID,
		UserID:     uriReq.UserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("membership not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var updatedMembership db.MerchantMembership
	var txType string

	if bodyReq.Amount > 0 {
		// 增加余额（退款/充值调整）
		updatedMembership, err = server.store.IncrementMembershipBalance(ctx, db.IncrementMembershipBalanceParams{
			ID:      membership.ID,
			Balance: bodyReq.Amount,
		})
		txType = "adjustment_credit"
	} else {
		// 扣减余额
		if membership.Balance < -bodyReq.Amount {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("insufficient balance")))
			return
		}
		updatedMembership, err = server.store.DecrementMembershipBalance(ctx, db.DecrementMembershipBalanceParams{
			ID:      membership.ID,
			Balance: -bodyReq.Amount,
		})
		txType = "adjustment_debit"
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 记录交易流水
	_, err = server.store.CreateMembershipTransaction(ctx, db.CreateMembershipTransactionParams{
		MembershipID:   membership.ID,
		Type:           txType,
		Amount:         bodyReq.Amount,
		BalanceAfter:   updatedMembership.Balance,
		RelatedOrderID: pgtype.Int8{},
		RechargeRuleID: pgtype.Int8{},
		Notes:          pgtype.Text{String: bodyReq.Notes, Valid: true},
	})
	if err != nil {
		// 流水记录失败不影响主流程
		fmt.Printf("failed to create transaction log: %v\n", err)
	}

	// 获取用户信息用于响应
	user, _ := server.store.GetUser(ctx, uriReq.UserID)
	phone := ""
	if user.Phone.Valid {
		phone = user.Phone.String
	}
	avatarURL := ""
	if user.AvatarUrl.Valid {
		avatarURL = user.AvatarUrl.String
	}

	ctx.JSON(http.StatusOK, merchantMemberResponse{
		UserID:         user.ID,
		FullName:       user.FullName,
		Phone:          phone,
		AvatarURL:      avatarURL,
		MembershipID:   updatedMembership.ID,
		Balance:        updatedMembership.Balance,
		TotalRecharged: updatedMembership.TotalRecharged,
		TotalConsumed:  updatedMembership.TotalConsumed,
		CreatedAt:      updatedMembership.CreatedAt,
	})
}
