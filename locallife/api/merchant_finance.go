package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 设置为当天结束
	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	// 查询分账统计
	financeStats, err := server.store.GetMerchantFinanceOverview(ctx, db.GetMerchantFinanceOverviewParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询满返支出统计
	promoStats, err := server.store.GetMerchantPromotionExpenses(ctx, db.GetMerchantPromotionExpensesParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalServiceFee := financeStats.TotalPlatformFee + financeStats.TotalOperatorFee
	netIncome := financeStats.TotalIncome - promoStats.TotalDiscount

	resp := financeOverviewResponse{
		CompletedOrders:   financeStats.CompletedOrders,
		PendingOrders:     financeStats.PendingOrders,
		TotalGMV:          financeStats.TotalGmv,
		TotalIncome:       financeStats.TotalIncome,
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	offset := (req.Page - 1) * req.Limit

	// 查询订单列表
	orders, err := server.store.ListMerchantFinanceOrders(ctx, db.ListMerchantFinanceOrdersParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数
	totalCount, err := server.store.CountMerchantFinanceOrders(ctx, db.CountMerchantFinanceOrdersParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
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

	ctx.JSON(http.StatusOK, gin.H{
		"orders":      result,
		"total":       totalCount,
		"page":        req.Page,
		"limit":       req.Limit,
		"total_pages": (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	// 查询服务费明细
	fees, err := server.store.GetMerchantServiceFeeDetail(ctx, db.GetMerchantServiceFeeDetailParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
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

	ctx.JSON(http.StatusOK, gin.H{
		"details":            result,
		"total_platform_fee": totalPlatformFee,
		"total_operator_fee": totalOperatorFee,
		"total_service_fee":  totalPlatformFee + totalOperatorFee,
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	offset := (req.Page - 1) * req.Limit

	// 查询满返支出订单列表
	orders, err := server.store.ListMerchantPromotionOrders(ctx, db.ListMerchantPromotionOrdersParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数和汇总
	totalCount, err := server.store.CountMerchantPromotionOrders(ctx, db.CountMerchantPromotionOrdersParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询汇总统计
	stats, err := server.store.GetMerchantPromotionExpenses(ctx, db.GetMerchantPromotionExpensesParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
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

	ctx.JSON(http.StatusOK, gin.H{
		"orders":             result,
		"total":              totalCount,
		"page":               req.Page,
		"limit":              req.Limit,
		"total_pages":        (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		"total_promo_orders": stats.PromoOrderCount,
		"total_promo_amount": stats.TotalDiscount,
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	// 查询每日财务汇总
	dailyStats, err := server.store.GetMerchantDailyFinance(ctx, db.GetMerchantDailyFinanceParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]dailyFinanceItem, len(dailyStats))
	for i, stat := range dailyStats {
		result[i] = dailyFinanceItem{
			Date:           stat.Date.Time.Format("2006-01-02"),
			OrderCount:     stat.OrderCount,
			TotalGMV:       stat.TotalGmv,
			MerchantIncome: stat.MerchantIncome,
			TotalFee:       stat.TotalFee,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"daily_stats": result,
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

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format")))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format")))
		return
	}

	// 验证日期范围
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

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

	offset := (req.Page - 1) * req.Limit

	// 根据是否有状态筛选选择不同的查询
	var orders []db.ProfitSharingOrder
	var totalCount int64
	if req.Status != nil {
		// 带状态筛选的查询
		orders, err = server.store.ListMerchantSettlementsByStatus(ctx, db.ListMerchantSettlementsByStatusParams{
			MerchantID:  merchant.ID,
			Status:      *req.Status,
			CreatedAt:   startDate,
			CreatedAt_2: endDate,
			Limit:       req.Limit,
			Offset:      offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		totalCount, err = server.store.CountMerchantSettlementsByStatus(ctx, db.CountMerchantSettlementsByStatusParams{
			MerchantID:  merchant.ID,
			Status:      *req.Status,
			CreatedAt:   startDate,
			CreatedAt_2: endDate,
		})
	} else {
		// 不带状态筛选的查询
		orders, err = server.store.ListMerchantSettlements(ctx, db.ListMerchantSettlementsParams{
			MerchantID:  merchant.ID,
			CreatedAt:   startDate,
			CreatedAt_2: endDate,
			Limit:       req.Limit,
			Offset:      offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		totalCount, err = server.store.CountMerchantSettlements(ctx, db.CountMerchantSettlementsParams{
			MerchantID:  merchant.ID,
			CreatedAt:   startDate,
			CreatedAt_2: endDate,
		})
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询统计（只统计已完成的）
	stats, err := server.store.GetMerchantProfitSharingStats(ctx, db.GetMerchantProfitSharingStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
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
			OrderSource:        order.OrderSource,
			TotalAmount:        order.TotalAmount,
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

	ctx.JSON(http.StatusOK, gin.H{
		"settlements":           result,
		"total":                 totalCount,
		"page":                  req.Page,
		"limit":                 req.Limit,
		"total_pages":           (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		"total_amount":          stats.TotalAmount,
		"total_merchant_amount": stats.TotalMerchantAmount,
		"total_platform_fee":    stats.TotalPlatformCommission,
		"total_operator_fee":    stats.TotalOperatorCommission,
	})
}
