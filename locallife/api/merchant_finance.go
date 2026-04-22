package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	merchantWithdrawMinAmount                    = int64(100) // 1元
	merchantWithdrawMaxAmount                    = int64(500000000)
	merchantWithdrawChannel                      = "wechat_ecommerce_fund"
	merchantWithdrawSyncStatePendingConfirmation = "pending_confirmation"
	merchantWithdrawSyncStateStale               = "stale"
)

// ==================== 财务概览 ====================

type getMerchantFinanceOverviewRequest struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
}

type financeOverviewResponse struct {
	// 订单统计
	CompletedOrders int64 `json:"completed_orders"`
	PendingOrders   int64 `json:"pending_orders"`

	// 金额统计（分）
	TotalGMV         int64 `json:"total_gmv"`          // 总交易额
	TotalIncome      int64 `json:"total_income"`       // 商户净收入
	TotalPlatformFee int64 `json:"total_platform_fee"` // 平台服务费
	TotalOperatorFee int64 `json:"total_operator_fee"` // 运营商服务费
	TotalServiceFee  int64 `json:"total_service_fee"`  // 总服务费（平台+运营商）
	PendingIncome    int64 `json:"pending_income"`     // 待结算收入

	// 满返支出统计
	PromotionOrders   int64 `json:"promotion_orders"`    // 满返订单数
	TotalPromotionExp int64 `json:"total_promotion_exp"` // 满返支出总额

	// 汇总
	NetIncome int64 `json:"net_income"` // 净收入 = 商户收入 - 满返支出
}

// getMerchantFinanceOverview 获取商户财务概览
// @Summary 获取财务概览
// @Description 商户查看指定时间范围内的财务汇总数据，包含订单统计、收入统计、满返支出等
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Success 200 {object} financeOverviewResponse "成功返回财务概览"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/overview [get]
func (server *Server) getMerchantFinanceOverview(ctx *gin.Context) {
	var req getMerchantFinanceOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 90)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 设置为当天结束
	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询分账统计
	financeStats, err := server.store.GetMerchantFinanceOverview(ctx, db.GetMerchantFinanceOverviewParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询满返支出统计
	promoStats, err := server.store.GetMerchantPromotionExpenses(ctx, db.GetMerchantPromotionExpensesParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	adjustmentTotal, err := server.store.SumMerchantSettlementAdjustments(ctx, db.SumMerchantSettlementAdjustmentsParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalServiceFee := financeStats.TotalPlatformFee + financeStats.TotalOperatorFee
	totalIncome := financeStats.TotalIncome + adjustmentTotal
	netIncome := totalIncome - promoStats.TotalDiscount

	resp := financeOverviewResponse{
		CompletedOrders:   financeStats.CompletedOrders,
		PendingOrders:     financeStats.PendingOrders,
		TotalGMV:          financeStats.TotalGmv,
		TotalIncome:       totalIncome,
		TotalPlatformFee:  financeStats.TotalPlatformFee,
		TotalOperatorFee:  financeStats.TotalOperatorFee,
		TotalServiceFee:   totalServiceFee,
		PendingIncome:     financeStats.PendingIncome,
		PromotionOrders:   promoStats.PromoOrderCount,
		TotalPromotionExp: promoStats.TotalDiscount,
		NetIncome:         netIncome,
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 订单收入明细 ====================

type listFinanceOrdersRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	Page      int32  `form:"page" binding:"omitempty,min=1"`
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type financeOrderItem struct {
	ID                 int64  `json:"id"`
	PaymentOrderID     int64  `json:"payment_order_id"`
	OrderID            int64  `json:"order_id,omitempty"`
	OrderSource        string `json:"order_source"`
	TotalAmount        int64  `json:"total_amount"`
	PlatformCommission int64  `json:"platform_commission"`
	OperatorCommission int64  `json:"operator_commission"`
	MerchantAmount     int64  `json:"merchant_amount"`
	Status             string `json:"status"`
	CreatedAt          string `json:"created_at"`
	FinishedAt         string `json:"finished_at,omitempty"`
}

type financeOrdersResponse struct {
	Orders     []financeOrderItem `json:"orders"`
	Total      int64              `json:"total"`
	Page       int32              `json:"page"`
	Limit      int32              `json:"limit"`
	TotalPages int64              `json:"total_pages"`
}

// listMerchantFinanceOrders 获取商户订单收入明细
// @Summary 获取订单收入明细
// @Description 商户查看指定时间范围内的订单收入明细，按分账订单维度展示
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "成功返回订单收入列表"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/orders [get]
func (server *Server) listMerchantFinanceOrders(ctx *gin.Context) {
	var req listFinanceOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 90)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := pageOffset(req.Page, req.Limit)

	// 查询订单列表
	orders, err := server.store.ListMerchantFinanceOrders(ctx, db.ListMerchantFinanceOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
		Limit:      req.Limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数
	totalCount, err := server.store.CountMerchantFinanceOrders(ctx, db.CountMerchantFinanceOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]financeOrderItem, len(orders))
	for i, order := range orders {
		finishedAt := ""
		if order.FinishedAt.Valid {
			finishedAt = order.FinishedAt.Time.Format(time.RFC3339)
		}

		var orderID int64
		if order.OrderID.Valid {
			orderID = order.OrderID.Int64
		}

		result[i] = financeOrderItem{
			ID:                 order.ID,
			PaymentOrderID:     order.PaymentOrderID,
			OrderID:            orderID,
			OrderSource:        order.OrderSource,
			TotalAmount:        order.TotalAmount,
			PlatformCommission: order.PlatformCommission,
			OperatorCommission: order.OperatorCommission,
			MerchantAmount:     order.MerchantAmount,
			Status:             order.Status,
			CreatedAt:          order.CreatedAt.Format(time.RFC3339),
			FinishedAt:         finishedAt,
		}
	}

	ctx.JSON(http.StatusOK, financeOrdersResponse{
		Orders:     result,
		Total:      totalCount,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
	})
}

// ==================== 服务费明细 ====================

type listServiceFeesRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

type serviceFeeItem struct {
	Date        string `json:"date"`
	OrderSource string `json:"order_source"`
	OrderCount  int64  `json:"order_count"`
	TotalAmount int64  `json:"total_amount"`
	PlatformFee int64  `json:"platform_fee"`
	OperatorFee int64  `json:"operator_fee"`
	TotalFee    int64  `json:"total_fee"`
}

type serviceFeeSummaryResponse struct {
	Details          []serviceFeeItem `json:"details"`
	TotalPlatformFee int64            `json:"total_platform_fee"`
	TotalOperatorFee int64            `json:"total_operator_fee"`
	TotalServiceFee  int64            `json:"total_service_fee"`
}

// listMerchantServiceFees 获取商户服务费明细
// @Summary 获取服务费明细
// @Description 商户查看指定时间范围内的服务费明细，按日期和订单来源分组
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Success 200 {object} map[string]interface{} "成功返回服务费明细"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/service-fees [get]
func (server *Server) listMerchantServiceFees(ctx *gin.Context) {
	var req listServiceFeesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 90)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询服务费明细
	fees, err := server.store.GetMerchantServiceFeeDetail(ctx, db.GetMerchantServiceFeeDetailParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 计算汇总
	var totalPlatformFee, totalOperatorFee int64
	result := make([]serviceFeeItem, len(fees))
	for i, fee := range fees {
		totalFee := fee.PlatformFee + fee.OperatorFee
		totalPlatformFee += fee.PlatformFee
		totalOperatorFee += fee.OperatorFee

		result[i] = serviceFeeItem{
			Date:        fee.Date.Time.Format("2006-01-02"),
			OrderSource: fee.OrderSource,
			OrderCount:  fee.OrderCount,
			TotalAmount: fee.TotalAmount,
			PlatformFee: fee.PlatformFee,
			OperatorFee: fee.OperatorFee,
			TotalFee:    totalFee,
		}
	}

	ctx.JSON(http.StatusOK, serviceFeeSummaryResponse{
		Details:          result,
		TotalPlatformFee: totalPlatformFee,
		TotalOperatorFee: totalOperatorFee,
		TotalServiceFee:  totalPlatformFee + totalOperatorFee,
	})
}

// ==================== 满返支出明细 ====================

type listPromotionExpensesRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	Page      int32  `form:"page" binding:"omitempty,min=1"`
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type promotionExpenseItem struct {
	ID                  int64  `json:"id"`
	OrderNo             string `json:"order_no"`
	OrderType           string `json:"order_type"`
	Subtotal            int64  `json:"subtotal"`
	DeliveryFee         int64  `json:"delivery_fee"`
	DeliveryFeeDiscount int64  `json:"delivery_fee_discount"`
	TotalAmount         int64  `json:"total_amount"`
	CreatedAt           string `json:"created_at"`
	CompletedAt         string `json:"completed_at,omitempty"`
}

type promotionExpensesResponse struct {
	Orders           []promotionExpenseItem `json:"orders"`
	Total            int64                  `json:"total"`
	Page             int32                  `json:"page"`
	Limit            int32                  `json:"limit"`
	TotalPages       int64                  `json:"total_pages"`
	TotalPromoOrders int64                  `json:"total_promo_orders"`
	TotalPromoAmount int64                  `json:"total_promo_amount"`
}

// listMerchantPromotionExpenses 获取商户满返支出明细
// @Summary 获取满返支出明细
// @Description 商户查看指定时间范围内的满返优惠支出明细
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "成功返回满返支出列表"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/promotions [get]
func (server *Server) listMerchantPromotionExpenses(ctx *gin.Context) {
	var req listPromotionExpensesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 90)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := pageOffset(req.Page, req.Limit)

	// 查询满返支出订单列表
	orders, err := server.store.ListMerchantPromotionOrders(ctx, db.ListMerchantPromotionOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
		Limit:      req.Limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数和汇总
	totalCount, err := server.store.CountMerchantPromotionOrders(ctx, db.CountMerchantPromotionOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询汇总统计
	stats, err := server.store.GetMerchantPromotionExpenses(ctx, db.GetMerchantPromotionExpensesParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]promotionExpenseItem, len(orders))
	for i, order := range orders {
		completedAt := ""
		if order.CompletedAt.Valid {
			completedAt = order.CompletedAt.Time.Format(time.RFC3339)
		}

		result[i] = promotionExpenseItem{
			ID:                  order.ID,
			OrderNo:             order.OrderNo,
			OrderType:           order.OrderType,
			Subtotal:            order.Subtotal,
			DeliveryFee:         order.DeliveryFee,
			DeliveryFeeDiscount: order.DeliveryFeeDiscount,
			TotalAmount:         order.TotalAmount,
			CreatedAt:           order.CreatedAt.Format(time.RFC3339),
			CompletedAt:         completedAt,
		}
	}

	ctx.JSON(http.StatusOK, promotionExpensesResponse{
		Orders:           result,
		Total:            totalCount,
		Page:             req.Page,
		Limit:            req.Limit,
		TotalPages:       (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		TotalPromoOrders: stats.PromoOrderCount,
		TotalPromoAmount: stats.TotalDiscount,
	})
}

// ==================== 每日财务汇总 ====================

type listDailyFinanceRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

type dailyFinanceItem struct {
	Date           string `json:"date"`
	OrderCount     int64  `json:"order_count"`
	TotalGMV       int64  `json:"total_gmv"`
	MerchantIncome int64  `json:"merchant_income"`
	TotalFee       int64  `json:"total_fee"`
}

type dailyFinanceSummaryResponse struct {
	DailyStats []dailyFinanceItem `json:"daily_stats"`
}

// listMerchantDailyFinance 获取商户每日财务汇总
// @Summary 获取每日财务汇总
// @Description 商户查看指定时间范围内的每日财务汇总数据
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Success 200 {object} map[string]interface{} "成功返回每日财务汇总"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/daily [get]
func (server *Server) listMerchantDailyFinance(ctx *gin.Context) {
	var req listDailyFinanceRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 90)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询每日财务汇总
	dailyStats, err := server.store.GetMerchantDailyFinance(ctx, db.GetMerchantDailyFinanceParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	adjustments, err := server.store.ListMerchantDailySettlementAdjustments(ctx, db.ListMerchantDailySettlementAdjustmentsParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resultMap := make(map[string]*dailyFinanceItem)
	for _, stat := range dailyStats {
		dateKey := stat.Date.Time.Format("2006-01-02")
		resultMap[dateKey] = &dailyFinanceItem{
			Date:           dateKey,
			OrderCount:     stat.OrderCount,
			TotalGMV:       stat.TotalGmv,
			MerchantIncome: stat.MerchantIncome,
			TotalFee:       stat.TotalFee,
		}
	}

	for _, adj := range adjustments {
		dateKey := adj.Date.Time.Format("2006-01-02")
		item, ok := resultMap[dateKey]
		if !ok {
			item = &dailyFinanceItem{Date: dateKey}
			resultMap[dateKey] = item
		}
		item.MerchantIncome += adj.TotalAdjustment
	}

	result := make([]dailyFinanceItem, 0, len(resultMap))
	for _, item := range resultMap {
		result = append(result, *item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})

	ctx.JSON(http.StatusOK, dailyFinanceSummaryResponse{
		DailyStats: result,
	})
}

// ==================== 结算记录（基于分账记录） ====================

type listMerchantSettlementsRequest struct {
	StartDate string  `form:"start_date" binding:"required"`
	EndDate   string  `form:"end_date" binding:"required"`
	Status    *string `form:"status" binding:"omitempty,oneof=pending processing finished failed"`
	Page      int32   `form:"page" binding:"omitempty,min=1"`
	Limit     int32   `form:"limit" binding:"omitempty,min=1,max=100"`
}

type merchantSettlementItem struct {
	ID                 int64  `json:"id"`
	PaymentOrderID     int64  `json:"payment_order_id"`
	OrderSource        string `json:"order_source"`
	TotalAmount        int64  `json:"total_amount"`
	PlatformCommission int64  `json:"platform_commission"`
	OperatorCommission int64  `json:"operator_commission"`
	MerchantAmount     int64  `json:"merchant_amount"`
	OutOrderNo         string `json:"out_order_no"`
	SharingOrderID     string `json:"sharing_order_id,omitempty"`
	Status             string `json:"status"`
	CreatedAt          string `json:"created_at"`
	FinishedAt         string `json:"finished_at,omitempty"`
}

type listMerchantSettlementTimelineRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	Page      int32  `form:"page" binding:"omitempty,min=1"`
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type merchantSettlementTimelineItem struct {
	RecordType         string `json:"record_type"`
	ID                 int64  `json:"id"`
	PaymentOrderID     int64  `json:"payment_order_id"`
	OrderSource        string `json:"order_source"`
	TotalAmount        int64  `json:"total_amount"`
	PlatformCommission int64  `json:"platform_commission"`
	OperatorCommission int64  `json:"operator_commission"`
	MerchantAmount     int64  `json:"merchant_amount"`
	OutOrderNo         string `json:"out_order_no"`
	SharingOrderID     string `json:"sharing_order_id,omitempty"`
	Status             string `json:"status"`
	CreatedAt          string `json:"created_at"`
	FinishedAt         string `json:"finished_at,omitempty"`
	AdjustmentType     string `json:"adjustment_type,omitempty"`
	RelatedType        string `json:"related_type,omitempty"`
	RelatedID          int64  `json:"related_id,omitempty"`
}

type merchantSettlementsResponse struct {
	Settlements         []merchantSettlementItem `json:"settlements"`
	Total               int64                    `json:"total"`
	Page                int32                    `json:"page"`
	Limit               int32                    `json:"limit"`
	TotalPages          int64                    `json:"total_pages"`
	TotalAmount         int64                    `json:"total_amount"`
	TotalMerchantAmount int64                    `json:"total_merchant_amount"`
	TotalPlatformFee    int64                    `json:"total_platform_fee"`
	TotalOperatorFee    int64                    `json:"total_operator_fee"`
}

type merchantSettlementTimelineResponse struct {
	Timeline   []merchantSettlementTimelineItem `json:"timeline"`
	Total      int64                            `json:"total"`
	Page       int32                            `json:"page"`
	Limit      int32                            `json:"limit"`
	TotalPages int64                            `json:"total_pages"`
}

// listMerchantSettlements 获取商户结算记录（分账订单列表）
// @Summary 获取结算记录
// @Description 商户查看分账订单列表，即结算记录
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Param status query string false "状态筛选" Enums(pending, processing, finished, failed)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "成功返回结算记录"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/settlements [get]
func (server *Server) listMerchantSettlements(ctx *gin.Context) {
	var req listMerchantSettlementsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := pageOffset(req.Page, req.Limit)

	// 根据是否有状态筛选选择不同的查询
	var orders []db.ProfitSharingOrder
	var totalCount int64
	if req.Status != nil {
		// 带状态筛选的查询
		orders, err = server.store.ListMerchantSettlementsByStatus(ctx, db.ListMerchantSettlementsByStatusParams{
			MerchantID: merchant.ID,
			Status:     *req.Status,
			StartAt:    startDate,
			EndAt:      endDate,
			Limit:      req.Limit,
			Offset:     offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		totalCount, err = server.store.CountMerchantSettlementsByStatus(ctx, db.CountMerchantSettlementsByStatusParams{
			MerchantID: merchant.ID,
			Status:     *req.Status,
			StartAt:    startDate,
			EndAt:      endDate,
		})
	} else {
		// 不带状态筛选的查询
		orders, err = server.store.ListMerchantSettlements(ctx, db.ListMerchantSettlementsParams{
			MerchantID: merchant.ID,
			StartAt:    startDate,
			EndAt:      endDate,
			Limit:      req.Limit,
			Offset:     offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		totalCount, err = server.store.CountMerchantSettlements(ctx, db.CountMerchantSettlementsParams{
			MerchantID: merchant.ID,
			StartAt:    startDate,
			EndAt:      endDate,
		})
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询统计（只统计已完成的）
	stats, err := server.store.GetMerchantProfitSharingStats(ctx, db.GetMerchantProfitSharingStatsParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]merchantSettlementItem, len(orders))
	for i, order := range orders {
		finishedAt := ""
		if order.FinishedAt.Valid {
			finishedAt = order.FinishedAt.Time.Format(time.RFC3339)
		}

		sharingOrderID := ""
		if order.SharingOrderID.Valid {
			sharingOrderID = order.SharingOrderID.String
		}

		result[i] = merchantSettlementItem{
			ID:                 order.ID,
			PaymentOrderID:     order.PaymentOrderID,
			PlatformCommission: order.PlatformCommission,
			OperatorCommission: order.OperatorCommission,
			MerchantAmount:     order.MerchantAmount,
			OutOrderNo:         order.OutOrderNo,
			SharingOrderID:     sharingOrderID,
			Status:             order.Status,
			CreatedAt:          order.CreatedAt.Format(time.RFC3339),
			FinishedAt:         finishedAt,
		}
	}
	ctx.JSON(http.StatusOK, merchantSettlementsResponse{
		Settlements:         result,
		Total:               totalCount,
		Page:                req.Page,
		Limit:               req.Limit,
		TotalPages:          (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		TotalAmount:         stats.TotalAmount,
		TotalMerchantAmount: stats.TotalMerchantAmount,
		TotalPlatformFee:    stats.TotalPlatformCommission,
		TotalOperatorFee:    stats.TotalOperatorCommission,
	})
}

// listMerchantSettlementTimeline 获取商户结算流水（分账记录 + 调整项）
// @Summary 获取结算流水
// @Description 商户查看结算流水，包含分账记录与结算调整
// @Tags 商户财务管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "开始日期" example(2025-11-01)
// @Param end_date query string true "结束日期" example(2025-11-30)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "成功返回结算流水"
// @Failure 400 {object} map[string]interface{} "参数错误或日期格式错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/finance/settlement-timeline [get]
func (server *Server) listMerchantSettlementTimeline(ctx *gin.Context) {
	var req listMerchantSettlementTimelineRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := pageOffset(req.Page, req.Limit)

	rows, err := server.store.ListMerchantSettlementTimeline(ctx, db.ListMerchantSettlementTimelineParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
		Limit:      req.Limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountMerchantSettlementTimeline(ctx, db.CountMerchantSettlementTimelineParams{
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]merchantSettlementTimelineItem, len(rows))
	for i, row := range rows {
		finishedAt := ""
		if row.FinishedAt.Valid {
			finishedAt = row.FinishedAt.Time.Format(time.RFC3339)
		}

		sharingOrderID := ""
		if row.SharingOrderID.Valid {
			sharingOrderID = row.SharingOrderID.String
		}

		adjustmentType := ""
		if row.AdjustmentType.Valid {
			adjustmentType = row.AdjustmentType.String
		}

		relatedType := ""
		if row.RelatedType.Valid {
			relatedType = row.RelatedType.String
		}

		relatedID := int64(0)
		if row.RelatedID.Valid {
			relatedID = row.RelatedID.Int64
		}

		item := merchantSettlementTimelineItem{
			RecordType:         row.RecordType,
			ID:                 row.ID,
			PaymentOrderID:     row.PaymentOrderID,
			OrderSource:        row.OrderSource,
			TotalAmount:        row.TotalAmount,
			PlatformCommission: row.PlatformCommission,
			OperatorCommission: row.OperatorCommission,
			MerchantAmount:     row.MerchantAmount,
			OutOrderNo:         row.OutOrderNo,
			SharingOrderID:     sharingOrderID,
			Status:             row.Status,
			CreatedAt:          row.CreatedAt.Format(time.RFC3339),
			FinishedAt:         finishedAt,
			AdjustmentType:     adjustmentType,
			RelatedType:        relatedType,
			RelatedID:          relatedID,
		}

		result[i] = item
	}

	ctx.JSON(http.StatusOK, merchantSettlementTimelineResponse{
		Timeline:   result,
		Total:      totalCount,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
	})
}

type merchantAccountBalanceResponse struct {
	SubMchID           string `json:"sub_mch_id"`
	AvailableAmount    int64  `json:"available_amount"`
	PendingAmount      int64  `json:"pending_amount"`
	WithdrawableAmount int64  `json:"withdrawable_amount"`
	AccountType        string `json:"account_type,omitempty"`
	BalanceDate        string `json:"balance_date,omitempty"`
	AccountStatus      string `json:"account_status"`
	StatusDesc         string `json:"status_desc"`
}

type createMerchantWithdrawRequest struct {
	Amount       int64  `json:"amount" binding:"required,min=100"`
	Remark       string `json:"remark" binding:"required,max=128"`
	OutRequestNo string `json:"out_request_no" binding:"omitempty,max=64"`
}

type merchantWithdrawItem struct {
	ID           int64  `json:"id"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	Channel      string `json:"channel"`
	OutRequestNo string `json:"out_request_no,omitempty"`
	WithdrawID   string `json:"withdraw_id,omitempty"`
	SubMchID     string `json:"sub_mch_id,omitempty"`
	Reason       string `json:"reason,omitempty"`
	SyncState    string `json:"sync_state,omitempty"`
	SyncMessage  string `json:"sync_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type listMerchantWithdrawalsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type merchantWithdrawalsResponse struct {
	Withdrawals   []merchantWithdrawItem `json:"withdrawals"`
	Total         int64                  `json:"total"`
	Page          int32                  `json:"page"`
	Limit         int32                  `json:"limit"`
	TotalPages    int64                  `json:"total_pages"`
	AccountStatus string                 `json:"account_status"`
	StatusDesc    string                 `json:"status_desc"`
}

type merchantWithdrawAccountInfo struct {
	MerchantID   int64  `json:"merchant_id"`
	SubMchID     string `json:"sub_mch_id"`
	OutRequestNo string `json:"out_request_no"`
	WithdrawID   string `json:"withdraw_id,omitempty"`
	Remark       string `json:"remark,omitempty"`
}

type merchantWithdrawCreateResponse struct {
	Withdrawal merchantWithdrawItem `json:"withdrawal"`
	Wechat     interface{}          `json:"wechat"`
}

func deriveMerchantWithdrawableAmount(balance *wechat.EcommerceFundBalanceResponse) int64 {
	if balance == nil {
		return 0
	}
	if balance.AvailableAmount <= 0 {
		return 0
	}
	return balance.AvailableAmount
}

func mapWechatWithdrawStatus(status string) string {
	switch strings.ToUpper(status) {
	case wechatcontracts.FundManagementWithdrawStatusSuccess:
		return "success"
	case wechatcontracts.FundManagementWithdrawStatusFail, wechatcontracts.FundManagementWithdrawStatusRefund, wechatcontracts.FundManagementWithdrawStatusClose:
		return "failed"
	default:
		return "pending"
	}
}

func (server *Server) updateWithdrawalRecordStatus(ctx *gin.Context, record db.WithdrawalRecord, status, reason string) db.WithdrawalRecord {
	updated, err := server.persistWithdrawalRecordStatus(ctx, record, status, reason)
	if err == nil {
		return updated
	}
	return record
}

func (server *Server) persistWithdrawalRecordStatus(ctx *gin.Context, record db.WithdrawalRecord, status, reason string) (db.WithdrawalRecord, error) {
	if status == "" {
		status = record.Status
	}
	reasonArg := pgtype.Text{}
	if reason != "" {
		reasonArg = pgtype.Text{String: reason, Valid: true}
	}
	clearReason := reason == "" && record.Reason.Valid
	if status == record.Status && !reasonArg.Valid && !clearReason {
		return record, nil
	}
	updated, err := server.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
		ID:          record.ID,
		Status:      status,
		Reason:      reasonArg,
		ClearReason: clearReason,
	})
	if err != nil {
		return record, err
	}
	return updated, nil
}

func (server *Server) updateWithdrawalRecordAccountInfo(ctx *gin.Context, record db.WithdrawalRecord, accountInfo []byte) db.WithdrawalRecord {
	updated, err := server.persistWithdrawalRecordAccountInfo(ctx, record, accountInfo)
	if err == nil {
		return updated
	}
	record.AccountInfo = accountInfo
	server.sendAlert(websocket.AlertData{
		AlertType:   websocket.AlertTypeSystemError,
		Level:       websocket.AlertLevelCritical,
		Title:       "提现账户信息更新失败",
		Message:     fmt.Sprintf("提现记录 %d 已拿到微信提现单号，但 account_info 持久化失败，请尽快核查该笔提现的外部流水信息。", record.ID),
		RelatedID:   record.ID,
		RelatedType: "withdrawal_record",
		Extra: map[string]interface{}{
			"withdrawal_record_id": record.ID,
			"channel":              record.Channel,
			"error":                err.Error(),
		},
	})
	return record
}

func (server *Server) persistWithdrawalRecordAccountInfo(ctx *gin.Context, record db.WithdrawalRecord, accountInfo []byte) (db.WithdrawalRecord, error) {
	if len(accountInfo) == 0 || bytes.Equal(accountInfo, record.AccountInfo) {
		return record, nil
	}
	updated, err := server.store.UpdateWithdrawalAccountInfo(ctx, db.UpdateWithdrawalAccountInfoParams{
		ID:          record.ID,
		AccountInfo: accountInfo,
	})
	if err != nil {
		return record, err
	}
	return updated, nil
}

func (server *Server) enqueueWithdrawalResultPolling(ctx *gin.Context, record db.WithdrawalRecord) {
	if record.Status != "pending" || server.taskDistributor == nil {
		return
	}
	if err := server.taskDistributor.DistributeTaskProcessMerchantWithdrawResult(
		ctx,
		&worker.MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: 0},
		asynq.ProcessIn(30*time.Second),
		asynq.Queue(worker.QueueDefault),
	); err != nil {
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeTaskEnqueueFailure,
			Level:       websocket.AlertLevelCritical,
			Title:       "提现状态轮询任务入队失败",
			Message:     fmt.Sprintf("提现记录 %d 已创建，但状态轮询任务入队失败，需依赖恢复调度兜底，请关注 Redis/任务队列状态。", record.ID),
			RelatedID:   record.ID,
			RelatedType: "withdrawal_record",
			Extra: map[string]interface{}{
				"withdrawal_record_id": record.ID,
				"channel":              record.Channel,
				"error":                err.Error(),
			},
		})
	}
}

func parseMerchantWithdrawAccountInfo(raw []byte) merchantWithdrawAccountInfo {
	if len(raw) == 0 {
		return merchantWithdrawAccountInfo{}
	}
	var info merchantWithdrawAccountInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return merchantWithdrawAccountInfo{}
	}
	return info
}

func toMerchantWithdrawItem(record db.WithdrawalRecord) merchantWithdrawItem {
	info := parseMerchantWithdrawAccountInfo(record.AccountInfo)
	reason := ""
	if record.Reason.Valid {
		reason = record.Reason.String
	}

	return merchantWithdrawItem{
		ID:           record.ID,
		Amount:       record.Amount,
		Status:       record.Status,
		Channel:      record.Channel,
		OutRequestNo: info.OutRequestNo,
		WithdrawID:   info.WithdrawID,
		SubMchID:     info.SubMchID,
		Reason:       reason,
		CreatedAt:    record.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    record.UpdatedAt.Format(time.RFC3339),
	}
}

func withMerchantWithdrawSyncState(item merchantWithdrawItem, state, message string) merchantWithdrawItem {
	item.SyncState = state
	item.SyncMessage = message
	return item
}

func (server *Server) getOwnerMerchantWithActivePaymentConfig(ctx *gin.Context, userID int64) (db.Merchant, db.MerchantPaymentConfig, error) {
	merchant, err := server.requireOwnedMerchantForUser(ctx, userID)
	if err != nil {
		return db.Merchant{}, db.MerchantPaymentConfig{}, err
	}

	paymentConfig, err := server.store.GetMerchantPaymentConfig(ctx, merchant.ID)
	if err != nil {
		return db.Merchant{}, db.MerchantPaymentConfig{}, err
	}

	if paymentConfig.SubMchID == "" || paymentConfig.Status != "active" {
		return db.Merchant{}, db.MerchantPaymentConfig{}, errors.New("merchant payment config is not active")
	}

	return merchant, paymentConfig, nil
}

func getMerchantFinanceAccountStatus(paymentConfig *db.MerchantPaymentConfig) (string, string) {
	if paymentConfig == nil {
		return "not_configured", "尚未开通收付通账户"
	}
	if paymentConfig.SubMchID == "" || paymentConfig.Status != "active" {
		return "inactive", "收付通账户未激活，完成进件签约后可查看余额并提现"
	}
	return "active", "收付通账户已激活"
}

func (server *Server) getFinanceViewerPaymentConfigState(ctx *gin.Context, userID int64) (db.Merchant, *db.MerchantPaymentConfig, string, string, error) {
	merchant, err := server.resolveMerchantForUser(ctx, userID)
	if err != nil {
		return db.Merchant{}, nil, "", "", err
	}

	paymentConfig, err := server.store.GetMerchantPaymentConfig(ctx, merchant.ID)
	if err != nil {
		if isNotFoundError(err) {
			status, desc := getMerchantFinanceAccountStatus(nil)
			return merchant, nil, status, desc, nil
		}
		return db.Merchant{}, nil, "", "", err
	}

	status, desc := getMerchantFinanceAccountStatus(&paymentConfig)
	return merchant, &paymentConfig, status, desc, nil
}

// getMerchantAccountBalance 查询商户收付通账户余额
func (server *Server) getMerchantAccountBalance(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "merchant account balance ecommerce client not configured"))
		return
	}

	query, ok := bindSubMerchantFundBalanceQuery(ctx)
	if !ok {
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusOK, merchantAccountBalanceResponse{
			SubMchID:           "",
			AvailableAmount:    0,
			PendingAmount:      0,
			WithdrawableAmount: 0,
			AccountType:        query.AccountType,
			BalanceDate:        query.Date,
			AccountStatus:      accountStatus,
			StatusDesc:         statusDesc,
		})
		return
	}

	balance, err := loadSubMerchantFundBalance(ctx, server.ecommerceClient, paymentConfig.SubMchID, query)
	if err != nil {
		respondFundBalanceQueryError(ctx, "query ecommerce fund balance", err)
		return
	}

	ctx.JSON(http.StatusOK, merchantAccountBalanceResponse{
		SubMchID:           paymentConfig.SubMchID,
		AvailableAmount:    balance.AvailableAmount,
		PendingAmount:      balance.PendingAmount,
		WithdrawableAmount: deriveMerchantWithdrawableAmount(balance),
		AccountType:        query.AccountType,
		BalanceDate:        query.Date,
		AccountStatus:      "active",
		StatusDesc:         "收付通账户已激活",
	})
}

// createMerchantAccountWithdraw 发起商户收付通提现
func (server *Server) createMerchantAccountWithdraw(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "merchant account withdraw ecommerce client not configured"))
		return
	}

	var req createMerchantWithdrawRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Amount < merchantWithdrawMinAmount || req.Amount > merchantWithdrawMaxAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("withdraw amount out of range")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, err := server.getOwnerMerchantWithActivePaymentConfig(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant or payment config not found")))
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if err.Error() == "merchant payment config is not active" {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	outRequestNo := req.OutRequestNo
	if outRequestNo == "" {
		b := make([]byte, 12)
		if _, randErr := rand.Read(b); randErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("generate out_request_no: %w", randErr)))
			return
		}
		outRequestNo = fmt.Sprintf("MW%d%s", merchant.ID, hex.EncodeToString(b))
	}

	// 检查 out_request_no 是否已存在，防止重复提现
	_, lookupErr := server.store.GetWithdrawalRecordByOutRequestNo(ctx, pgtype.Text{String: outRequestNo, Valid: true})
	if lookupErr == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: withdrawal already exists")))
		return
	}
	if !isNotFoundError(lookupErr) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, lookupErr))
		return
	}

	accountInfoBytes, _ := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: outRequestNo,
		Remark:       req.Remark,
	})

	record, err := server.store.CreateWithdrawalRecord(ctx, db.CreateWithdrawalRecordParams{
		UserID:       authPayload.UserID,
		Amount:       req.Amount,
		Status:       "pending",
		Channel:      merchantWithdrawChannel,
		AccountInfo:  accountInfoBytes,
		OutRequestNo: pgtype.Text{String: outRequestNo, Valid: true},
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: withdrawal already exists")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	withdrawCreateResp, err := server.ecommerceClient.CreateEcommerceWithdraw(ctx, &wechat.EcommerceWithdrawRequest{
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: outRequestNo,
		Amount:       req.Amount,
		Remark:       req.Remark,
	})
	var (
		wechatPayload interface{}
		withdrawID    string
		status        = "pending"
		reason        string
	)
	if err != nil {
		queryResp, queryErr := server.ecommerceClient.QueryEcommerceWithdrawByOutRequestNo(ctx, paymentConfig.SubMchID, outRequestNo)
		if queryErr != nil {
			_ = ctx.Error(err)
			_ = ctx.Error(queryErr)
			log.Warn().
				Err(err).
				Str("request_id", GetRequestID(ctx)).
				Int64("merchant_id", merchant.ID).
				Int64("withdrawal_record_id", record.ID).
				Str("sub_mchid", paymentConfig.SubMchID).
				Str("out_request_no", outRequestNo).
				Str("query_error", queryErr.Error()).
				Msg("merchant withdraw submitted but wechat confirmation is pending")
			record = server.updateWithdrawalRecordStatus(ctx, record, "pending", "withdraw request submitted, awaiting wechat confirmation")
			server.enqueueWithdrawalResultPolling(ctx, record)
			ctx.JSON(http.StatusAccepted, merchantWithdrawCreateResponse{
				Withdrawal: withMerchantWithdrawSyncState(
					toMerchantWithdrawItem(record),
					merchantWithdrawSyncStatePendingConfirmation,
					"微信提现已提交，但微信侧结果暂未确认，系统将继续同步状态。",
				),
				Wechat: nil,
			})
			return
		}
		withdrawID = queryResp.WithdrawID
		status = mapWechatWithdrawStatus(queryResp.Status)
		reason = queryResp.Reason
		wechatPayload = queryResp
	} else {
		withdrawID = withdrawCreateResp.WithdrawID
		wechatPayload = withdrawCreateResp
	}

	accountInfoBytes, _ = json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: outRequestNo,
		WithdrawID:   withdrawID,
		Remark:       req.Remark,
	})
	record = server.updateWithdrawalRecordAccountInfo(ctx, record, accountInfoBytes)

	record = server.updateWithdrawalRecordStatus(ctx, record, status, reason)
	server.enqueueWithdrawalResultPolling(ctx, record)

	ctx.JSON(http.StatusCreated, merchantWithdrawCreateResponse{
		Withdrawal: toMerchantWithdrawItem(record),
		Wechat:     wechatPayload,
	})
}

// listMerchantAccountWithdrawals 查询商户提现记录
func (server *Server) listMerchantAccountWithdrawals(ctx *gin.Context) {
	var req listMerchantWithdrawalsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, _, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if accountStatus != "active" {
		ctx.JSON(http.StatusOK, merchantWithdrawalsResponse{
			Withdrawals:   []merchantWithdrawItem{},
			Total:         0,
			Page:          req.Page,
			Limit:         req.Limit,
			TotalPages:    0,
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	offset := pageOffset(req.Page, req.Limit)
	rows, err := server.store.ListWithdrawalRecords(ctx, db.ListWithdrawalRecordsParams{
		UserID:  authPayload.UserID,
		Channel: merchantWithdrawChannel,
		Limit:   req.Limit,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountWithdrawalRecords(ctx, db.CountWithdrawalRecordsParams{
		UserID:  authPayload.UserID,
		Channel: merchantWithdrawChannel,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]merchantWithdrawItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toMerchantWithdrawItem(row))
	}

	ctx.JSON(http.StatusOK, merchantWithdrawalsResponse{
		Withdrawals:   items,
		Total:         totalCount,
		Page:          req.Page,
		Limit:         req.Limit,
		TotalPages:    (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		AccountStatus: "active",
		StatusDesc:    "收付通账户已激活",
	})
}

type getMerchantWithdrawalRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getMerchantAccountWithdrawal 查询单笔提现并同步微信状态
func (server *Server) getMerchantAccountWithdrawal(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "merchant account withdrawal detail ecommerce client not configured"))
		return
	}

	var req getMerchantWithdrawalRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, _, err := server.getOwnerMerchantWithActivePaymentConfig(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant or payment config not found")))
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if err.Error() == "merchant payment config is not active" {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	record, err := server.store.GetWithdrawalRecord(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("withdrawal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if record.UserID != authPayload.UserID || record.Channel != merchantWithdrawChannel {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("no permission to access this withdrawal")))
		return
	}

	info := parseMerchantWithdrawAccountInfo(record.AccountInfo)
	response := toMerchantWithdrawItem(record)
	if info.SubMchID != "" && info.OutRequestNo != "" {
		wxResp, queryErr := server.ecommerceClient.QueryEcommerceWithdrawByOutRequestNo(ctx, info.SubMchID, info.OutRequestNo)
		if queryErr == nil {
			newStatus := mapWechatWithdrawStatus(wxResp.Status)
			reasonText := ""
			if wxResp.Reason != "" {
				reasonText = wxResp.Reason
			}

			record = server.updateWithdrawalRecordStatus(ctx, record, newStatus, reasonText)

			if wxResp.WithdrawID != "" && wxResp.WithdrawID != info.WithdrawID {
				info.WithdrawID = wxResp.WithdrawID
				if raw, marshalErr := json.Marshal(info); marshalErr == nil {
					record = server.updateWithdrawalRecordAccountInfo(ctx, record, raw)
				}
			}
			response = toMerchantWithdrawItem(record)
		} else {
			_ = ctx.Error(queryErr)
			log.Warn().
				Err(queryErr).
				Str("request_id", GetRequestID(ctx)).
				Int64("withdrawal_record_id", record.ID).
				Str("sub_mchid", info.SubMchID).
				Str("out_request_no", info.OutRequestNo).
				Msg("query merchant withdraw status failed; returning cached record")
			response = withMerchantWithdrawSyncState(
				response,
				merchantWithdrawSyncStateStale,
				"微信提现状态同步失败，当前展示的是本地缓存结果，请稍后刷新。",
			)
		}
	}

	ctx.JSON(http.StatusOK, response)
}
