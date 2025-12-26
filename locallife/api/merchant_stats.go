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

// ==================== 商户日报 ====================

type getMerchantDailyStatsRequest struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
}

type dailyStatRow struct {
	Date          string `json:"date"`
	OrderCount    int32  `json:"order_count"`
	TotalSales    int64  `json:"total_sales"`
	Commission    int64  `json:"commission"`
	TakeoutOrders int32  `json:"takeout_orders"`
	DineInOrders  int32  `json:"dine_in_orders"`
}

// getMerchantDailyStats 获取商户日报
// @Summary 获取商户日报统计
// @Description 商户获取指定日期范围内的每日订单、销售额、佣金等统计数据
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {array} dailyStatRow "日报统计列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/daily [get]
func (server *Server) getMerchantDailyStats(ctx *gin.Context) {
	var req getMerchantDailyStatsRequest
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
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

	// 查询日报统计
	stats, err := server.store.GetMerchantDailyStats(ctx, db.GetMerchantDailyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应格式
	result := make([]dailyStatRow, len(stats))
	for i, stat := range stats {
		result[i] = dailyStatRow{
			Date:          stat.Date.Time.Format("2006-01-02"),
			OrderCount:    stat.OrderCount,
			TotalSales:    stat.TotalSales,
			Commission:    stat.Commission,
			TakeoutOrders: stat.TakeoutOrders,
			DineInOrders:  stat.DineInOrders,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 商户概览 ====================

type getMerchantOverviewRequest struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
}

type merchantOverviewResponse struct {
	TotalDays       int32 `json:"total_days"`
	TotalOrders     int32 `json:"total_orders"`
	TotalSales      int64 `json:"total_sales"`
	TotalCommission int64 `json:"total_commission"`
	AvgDailySales   int64 `json:"avg_daily_sales"`
}

// getMerchantOverview 获取商户概览
// @Summary 获取商户概览统计
// @Description 商户获取指定日期范围内的总体统计数据，包括总天数、总订单数、总销售额、总佣金、日均销售额
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {object} merchantOverviewResponse "概览统计"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/overview [get]
func (server *Server) getMerchantOverview(ctx *gin.Context) {
	var req getMerchantOverviewRequest
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
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

	// 查询概览统计
	overview, err := server.store.GetMerchantOverview(ctx, db.GetMerchantOverviewParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, merchantOverviewResponse{
		TotalDays:       overview.TotalDays,
		TotalOrders:     overview.TotalOrders,
		TotalSales:      overview.TotalSales,
		TotalCommission: overview.TotalCommission,
		AvgDailySales:   int64(overview.AvgDailySales),
	})
}

// ==================== 菜品销量排行 ====================

type getTopSellingDishesRequest struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type topSellingDishRow struct {
	DishID       int64  `json:"dish_id"`
	DishName     string `json:"dish_name"`
	DishPrice    int64  `json:"dish_price"`
	TotalSold    int32  `json:"total_sold"`
	TotalRevenue int64  `json:"total_revenue"`
}

// getTopSellingDishes 获取菜品销量排行
// @Summary 获取菜品销量排行
// @Description 商户获取指定日期范围内销量最高的菜品列表
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Param limit query int false "返回数量限制 (默认10, 最大100)"
// @Success 200 {array} topSellingDishRow "菜品销量排行列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/dishes/top [get]
func (server *Server) getTopSellingDishes(ctx *gin.Context) {
	var req getTopSellingDishesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 默认Limit为10
	if req.Limit == 0 {
		req.Limit = 10
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
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

	// 查询销量排行
	dishes, err := server.store.GetTopSellingDishes(ctx, db.GetTopSellingDishesParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应格式
	result := make([]topSellingDishRow, len(dishes))
	for i, dish := range dishes {
		result[i] = topSellingDishRow{
			DishID:       dish.DishID.Int64,
			DishName:     dish.DishName,
			DishPrice:    dish.DishPrice,
			TotalSold:    dish.TotalSold,
			TotalRevenue: dish.TotalRevenue,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 顾客列表 ====================

type listMerchantCustomersRequest struct {
	OrderBy string `form:"order_by" binding:"omitempty,oneof=total_orders total_amount last_order_at"`
	Page    int32  `form:"page" binding:"omitempty,min=1"`
	Limit   int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type customerStatRow struct {
	UserID         int64  `json:"user_id"`
	FullName       string `json:"full_name"`
	Phone          string `json:"phone"`
	AvatarURL      string `json:"avatar_url"`
	TotalOrders    int32  `json:"total_orders"`
	TotalAmount    int64  `json:"total_amount"`
	AvgOrderAmount int64  `json:"avg_order_amount"`
	FirstOrderAt   string `json:"first_order_at"`
	LastOrderAt    string `json:"last_order_at"`
}

// listMerchantCustomers 获取商户的顾客列表
// @Summary 获取商户顾客列表
// @Description 商户获取所有曾在本店消费的顾客统计信息，支持按总订单数、总金额、最后下单时间排序
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param order_by query string false "排序字段 (total_orders/total_amount/last_order_at, 默认last_order_at)"
// @Param page query int false "页码 (默认1)"
// @Param limit query int false "每页数量 (默认20, 最大100)"
// @Success 200 {object} map[string]interface{} "顾客列表及分页信息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/customers [get]
func (server *Server) listMerchantCustomers(ctx *gin.Context) {
	var req listMerchantCustomersRequest
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
	if req.OrderBy == "" {
		req.OrderBy = "last_order_at"
	}

	offset := (req.Page - 1) * req.Limit

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

	// 查询顾客列表
	customers, err := server.store.GetMerchantCustomerStats(ctx, db.GetMerchantCustomerStatsParams{
		MerchantID: merchant.ID,
		OrderBy:    req.OrderBy,
		Limit:      req.Limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数
	totalCount, err := server.store.CountMerchantCustomers(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应格式
	result := make([]customerStatRow, len(customers))
	for i, customer := range customers {
		// 处理时间类型
		firstOrderAt := ""
		if t, ok := customer.FirstOrderAt.(time.Time); ok {
			firstOrderAt = t.Format(time.RFC3339)
		}
		lastOrderAt := ""
		if t, ok := customer.LastOrderAt.(time.Time); ok {
			lastOrderAt = t.Format(time.RFC3339)
		}

		result[i] = customerStatRow{
			UserID:         customer.UserID,
			FullName:       customer.FullName,
			Phone:          customer.Phone.String,
			AvatarURL:      normalizeUploadURLForClient(customer.AvatarUrl.String),
			TotalOrders:    customer.TotalOrders,
			TotalAmount:    customer.TotalAmount,
			AvgOrderAmount: int64(customer.AvgOrderAmount),
			FirstOrderAt:   firstOrderAt,
			LastOrderAt:    lastOrderAt,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":        result,
		"total_count": totalCount,
		"page":        req.Page,
		"limit":       req.Limit,
	})
}

// ==================== 顾客详情 ====================

type getCustomerDetailRequest struct {
	UserID int64 `uri:"user_id" binding:"required,min=1"`
}

type customerDetailResponse struct {
	UserID         int64             `json:"user_id"`
	FullName       string            `json:"full_name"`
	Phone          string            `json:"phone"`
	AvatarURL      string            `json:"avatar_url"`
	TotalOrders    int32             `json:"total_orders"`
	TotalAmount    int64             `json:"total_amount"`
	AvgOrderAmount int64             `json:"avg_order_amount"`
	FirstOrderAt   string            `json:"first_order_at"`
	LastOrderAt    string            `json:"last_order_at"`
	FavoriteDishes []favoriteDishRow `json:"favorite_dishes"`
}

type favoriteDishRow struct {
	DishID        int64  `json:"dish_id"`
	DishName      string `json:"dish_name"`
	OrderCount    int32  `json:"order_count"`
	TotalQuantity int32  `json:"total_quantity"`
}

// getCustomerDetail 获取顾客详情
// @Summary 获取顾客详情
// @Description 商户获取指定顾客在本店的消费详情，包括统计数据和喜好菜品
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param user_id path int true "用户ID"
// @Success 200 {object} customerDetailResponse "顾客详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户或顾客不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/customers/{user_id} [get]
func (server *Server) getCustomerDetail(ctx *gin.Context) {
	var req getCustomerDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
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

	// 查询顾客统计
	customer, err := server.store.GetCustomerMerchantDetail(ctx, db.GetCustomerMerchantDetailParams{
		MerchantID: merchant.ID,
		UserID:     req.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("customer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询喜好菜品
	favoriteDishes, err := server.store.GetCustomerFavoriteDishes(ctx, db.GetCustomerFavoriteDishesParams{
		MerchantID: merchant.ID,
		UserID:     req.UserID,
		Limit:      10,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换喜好菜品
	dishes := make([]favoriteDishRow, len(favoriteDishes))
	for i, dish := range favoriteDishes {
		dishes[i] = favoriteDishRow{
			DishID:        dish.DishID.Int64,
			DishName:      dish.DishName,
			OrderCount:    dish.OrderCount,
			TotalQuantity: dish.TotalQuantity,
		}
	}

	// 处理时间类型
	firstOrderAt := ""
	if t, ok := customer.FirstOrderAt.(time.Time); ok {
		firstOrderAt = t.Format(time.RFC3339)
	}
	lastOrderAt := ""
	if t, ok := customer.LastOrderAt.(time.Time); ok {
		lastOrderAt = t.Format(time.RFC3339)
	}

	ctx.JSON(http.StatusOK, customerDetailResponse{
		UserID:         customer.UserID,
		FullName:       customer.FullName,
		Phone:          customer.Phone.String,
		AvatarURL:      normalizeUploadURLForClient(customer.AvatarUrl.String),
		TotalOrders:    customer.TotalOrders,
		TotalAmount:    customer.TotalAmount,
		AvgOrderAmount: int64(customer.AvgOrderAmount),
		FirstOrderAt:   firstOrderAt,
		LastOrderAt:    lastOrderAt,
		FavoriteDishes: dishes,
	})
}

// ==================== 商户时段分析 ====================

type merchantHourlyStatsRow struct {
	Hour           int32 `json:"hour"`
	OrderCount     int32 `json:"order_count"`
	AvgOrderAmount int64 `json:"avg_order_amount"`
}

type getMerchantStatsRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

// getMerchantHourlyStats 获取商户订单时段分析
// @Summary 获取商户时段分析
// @Description 商户获取指定日期范围内按小时统计的订单分布，分析高峰时段
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {array} merchantHourlyStatsRow "时段统计列表 (0-23小时)"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/hourly [get]
func (server *Server) getMerchantHourlyStats(ctx *gin.Context) {
	var req getMerchantStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取授权信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetMerchantHourlyStats(ctx, db.GetMerchantHourlyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]merchantHourlyStatsRow, len(stats))
	for i, stat := range stats {
		result[i] = merchantHourlyStatsRow{
			Hour:           stat.Hour,
			OrderCount:     stat.OrderCount,
			AvgOrderAmount: stat.AvgOrderAmount,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 订单来源分析 ====================

type merchantOrderSourceStatsRow struct {
	OrderType  string `json:"order_type"`
	OrderCount int32  `json:"order_count"`
	TotalSales int64  `json:"total_sales"`
}

// getMerchantOrderSourceStats 获取商户订单来源分析
// @Summary 获取订单来源分析
// @Description 商户获取指定日期范围内按订单类型（外卖/堂食等）的统计分析
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {array} merchantOrderSourceStatsRow "订单来源统计列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/sources [get]
func (server *Server) getMerchantOrderSourceStats(ctx *gin.Context) {
	var req getMerchantStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取授权信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetMerchantOrderSourceStats(ctx, db.GetMerchantOrderSourceStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]merchantOrderSourceStatsRow, len(stats))
	for i, stat := range stats {
		result[i] = merchantOrderSourceStatsRow{
			OrderType:  stat.OrderType,
			OrderCount: stat.OrderCount,
			TotalSales: stat.TotalSales,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 复购率分析 ====================

type merchantRepurchaseRateResponse struct {
	TotalUsers       int32   `json:"total_users"`
	RepeatUsers      int32   `json:"repeat_users"`
	RepurchaseRate   float64 `json:"repurchase_rate"` // 百分比
	AvgOrdersPerUser float64 `json:"avg_orders_per_user"`
}

// getMerchantRepurchaseRate 获取商户复购率
// @Summary 获取商户复购率
// @Description 商户获取指定日期范围内的顾客复购率分析，包括总顾客数、复购顾客数、复购率、平均订单数
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {object} merchantRepurchaseRateResponse "复购率统计"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/repurchase [get]
func (server *Server) getMerchantRepurchaseRate(ctx *gin.Context) {
	var req getMerchantStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取授权信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetMerchantRepurchaseRate(ctx, db.GetMerchantRepurchaseRateParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, merchantRepurchaseRateResponse{
		TotalUsers:       stats.TotalCustomers,
		RepeatUsers:      stats.RepeatCustomers,
		RepurchaseRate:   float64(stats.RepurchaseRateBasisPoints) / 100.0, // 万分比转百分比 (7550 -> 75.50)
		AvgOrdersPerUser: float64(stats.AvgOrdersPerUserCents) / 100.0,     // 百分数转小数 (235 -> 2.35)
	})
}

// ==================== 菜品分类分析 ====================

type dishCategoryStatsRow struct {
	CategoryName  string `json:"category_name"`
	OrderCount    int32  `json:"order_count"`
	TotalSales    int64  `json:"total_sales"`
	TotalQuantity int32  `json:"total_quantity"`
}

// getMerchantDishCategoryStats 获取商户菜品分类统计
// @Summary 获取菜品分类统计
// @Description 商户获取指定日期范围内按菜品分类的销售统计分析
// @Tags 商户统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (格式: YYYY-MM-DD)"
// @Success 200 {array} dishCategoryStatsRow "菜品分类统计列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/merchant/stats/categories [get]
func (server *Server) getMerchantDishCategoryStats(ctx *gin.Context) {
	var req getMerchantStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取授权信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetDishCategoryStats(ctx, db.GetDishCategoryStatsParams{
		MerchantID: merchant.ID,
		StartDate:  startDate,
		EndDate:    endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]dishCategoryStatsRow, len(stats))
	for i, stat := range stats {
		result[i] = dishCategoryStatsRow{
			CategoryName:  stat.CategoryName,
			OrderCount:    stat.DishCount,
			TotalSales:    stat.TotalRevenue,
			TotalQuantity: stat.TotalQuantity,
		}
	}

	ctx.JSON(http.StatusOK, result)
}
