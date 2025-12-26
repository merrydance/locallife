package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

// ==================== 平台全局概览 ====================

type getPlatformOverviewRequest struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
}

type platformOverviewResponse struct {
	TotalOrders     int32 `json:"total_orders"`     // 总订单数
	TotalGMV        int64 `json:"total_gmv"`        // 总GMV(分)
	TotalCommission int64 `json:"total_commission"` // 平台总佣金(分)
	ActiveMerchants int32 `json:"active_merchants"` // 活跃商户数
	ActiveUsers     int32 `json:"active_users"`     // 活跃用户数
}

// getPlatformOverview 获取平台全局概览
// @Summary 获取平台全局概览
// @Description 获取指定时间范围内的平台全局统计数据，包括订单数、GMV、佣金、活跃商户和用户数
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {object} platformOverviewResponse "平台概览数据"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/overview [get]
func (server *Server) getPlatformOverview(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	stats, err := server.store.GetPlatformOverview(ctx, db.GetPlatformOverviewParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, platformOverviewResponse{
		TotalOrders:     stats.TotalOrders,
		TotalGMV:        stats.TotalGmv,
		TotalCommission: stats.TotalCommission,
		ActiveMerchants: stats.ActiveMerchants,
		ActiveUsers:     stats.ActiveUsers,
	})
}

// ==================== 平台日趋势 ====================

type platformDailyStatRow struct {
	Date            string `json:"date"`             // 日期
	OrderCount      int32  `json:"order_count"`      // 订单数
	TotalGMV        int64  `json:"total_gmv"`        // 总GMV(分)
	TotalCommission int64  `json:"total_commission"` // 平台佣金(分)
	ActiveMerchants int32  `json:"active_merchants"` // 活跃商户数
	ActiveUsers     int32  `json:"active_users"`     // 活跃用户数
	TakeoutOrders   int32  `json:"takeout_orders"`   // 外卖订单数
	DineInOrders    int32  `json:"dine_in_orders"`   // 堂食订单数
}

// getPlatformDailyStats 获取平台日趋势
// @Summary 获取平台日趋势统计
// @Description 获取指定时间范围内每日的平台统计数据，包括订单数、GMV、佣金及订单类型分布
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} platformDailyStatRow "每日统计数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/daily [get]
func (server *Server) getPlatformDailyStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	stats, err := server.store.GetPlatformDailyStats(ctx, db.GetPlatformDailyStatsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]platformDailyStatRow, len(stats))
	for i, stat := range stats {
		result[i] = platformDailyStatRow{
			Date:            stat.Date.Time.Format("2006-01-02"),
			OrderCount:      stat.OrderCount,
			TotalGMV:        stat.TotalGmv,
			TotalCommission: stat.TotalCommission,
			ActiveMerchants: stat.ActiveMerchants,
			ActiveUsers:     stat.ActiveUsers,
			TakeoutOrders:   stat.TakeoutOrders,
			DineInOrders:    stat.DineInOrders,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 区域对比分析 ====================

type regionComparisonRow struct {
	RegionID        int64  `json:"region_id"`        // 区域ID
	RegionName      string `json:"region_name"`      // 区域名称
	MerchantCount   int32  `json:"merchant_count"`   // 商户数
	OrderCount      int32  `json:"order_count"`      // 订单数
	TotalGMV        int64  `json:"total_gmv"`        // 总GMV(分)
	TotalCommission int64  `json:"total_commission"` // 总佣金(分)
	AvgOrderAmount  int64  `json:"avg_order_amount"` // 平均订单金额(分)
	ActiveUsers     int32  `json:"active_users"`     // 活跃用户数
}

// getRegionComparison 获取区域对比分析
// @Summary 获取区域对比分析
// @Description 获取各区域的销售数据对比，包括商户数、订单数、GMV、佣金等
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} regionComparisonRow "区域对比数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/regions/compare [get]
func (server *Server) getRegionComparison(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	regions, err := server.store.GetRegionComparison(ctx, db.GetRegionComparisonParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]regionComparisonRow, len(regions))
	for i, region := range regions {
		result[i] = regionComparisonRow{
			RegionID:        region.RegionID,
			RegionName:      region.RegionName,
			MerchantCount:   region.MerchantCount,
			OrderCount:      region.OrderCount,
			TotalGMV:        region.TotalGmv,
			TotalCommission: region.TotalCommission,
			AvgOrderAmount:  region.AvgOrderAmount,
			ActiveUsers:     region.ActiveUsers,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 商户销售排行 ====================

type getMerchantRankingRequest struct {
	StartDate string `form:"start_date" binding:"required"`           // 开始日期
	EndDate   string `form:"end_date" binding:"required"`             // 结束日期
	Page      int32  `form:"page" binding:"omitempty,min=1"`          // 页码
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"` // 每页数量
}

type merchantRankingRow struct {
	MerchantID      int64  `json:"merchant_id"`      // 商户ID
	MerchantName    string `json:"merchant_name"`    // 商户名称
	RegionID        int64  `json:"region_id"`        // 区域ID
	RegionName      string `json:"region_name"`      // 区域名称
	OrderCount      int32  `json:"order_count"`      // 订单数
	TotalSales      int64  `json:"total_sales"`      // 总销售额(分)
	TotalCommission int64  `json:"total_commission"` // 总佣金(分)
	AvgOrderAmount  int64  `json:"avg_order_amount"` // 平均订单金额(分)
}

// getMerchantRanking 获取商户销售排行
// @Summary 获取商户销售排行榜
// @Description 获取商户按销售额排序的排行榜，支持分页
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Param page query int false "页码 (默认: 1)" minimum(1)
// @Param limit query int false "每页数量 (默认: 20, 最大: 100)" minimum(1) maximum(100)
// @Security BearerAuth
// @Success 200 {array} merchantRankingRow "商户排行列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/merchants/ranking [get]
func (server *Server) getMerchantRanking(ctx *gin.Context) {
	var req getMerchantRankingRequest
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
	offset := (req.Page - 1) * req.Limit

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

	merchants, err := server.store.GetMerchantRanking(ctx, db.GetMerchantRankingParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]merchantRankingRow, len(merchants))
	for i, merchant := range merchants {
		result[i] = merchantRankingRow{
			MerchantID:      merchant.MerchantID,
			MerchantName:    merchant.MerchantName,
			RegionID:        merchant.RegionID,
			RegionName:      merchant.RegionName,
			OrderCount:      merchant.OrderCount,
			TotalSales:      merchant.TotalSales,
			TotalCommission: merchant.TotalCommission,
			AvgOrderAmount:  merchant.AvgOrderAmount,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 分类销售统计 ====================

type categoryStatRow struct {
	CategoryName  string `json:"category_name"`  // 分类名称
	MerchantCount int32  `json:"merchant_count"` // 商户数
	OrderCount    int32  `json:"order_count"`    // 订单数
	TotalSales    int64  `json:"total_sales"`    // 总销售额(分)
}

// getCategoryStats 获取分类销售统计
// @Summary 获取分类销售统计
// @Description 获取各商户分类的销售统计数据
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} categoryStatRow "分类统计数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/categories [get]
func (server *Server) getCategoryStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	categories, err := server.store.GetCategoryStats(ctx, db.GetCategoryStatsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]categoryStatRow, len(categories))
	for i, category := range categories {
		result[i] = categoryStatRow{
			CategoryName:  category.CategoryName,
			MerchantCount: category.MerchantCount,
			OrderCount:    category.OrderCount,
			TotalSales:    category.TotalSales,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 增长统计 ====================

type growthStatRow struct {
	Date  string `json:"date"`  // 日期
	Count int32  `json:"count"` // 数量
}

// getUserGrowthStats 获取用户增长统计
// @Summary 获取用户增长统计
// @Description 获取每日新注册用户数量统计
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} growthStatRow "用户增长数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/growth/users [get]
func (server *Server) getUserGrowthStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	stats, err := server.store.GetUserGrowthStats(ctx, db.GetUserGrowthStatsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]growthStatRow, len(stats))
	for i, stat := range stats {
		result[i] = growthStatRow{
			Date:  stat.Date.Time.Format("2006-01-02"),
			Count: stat.NewUsers,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// getMerchantGrowthStats 获取商户增长统计
// @Summary 获取商户增长统计
// @Description 获取每日新增审核通过的商户数量统计
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} growthStatRow "商户增长数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/growth/merchants [get]
func (server *Server) getMerchantGrowthStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	stats, err := server.store.GetMerchantGrowthStats(ctx, db.GetMerchantGrowthStatsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]growthStatRow, len(stats))
	for i, stat := range stats {
		result[i] = growthStatRow{
			Date:  stat.Date.Time.Format("2006-01-02"),
			Count: stat.NewMerchants,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 骑手绩效排行 ====================

type riderRankingRow struct {
	RiderID                int64  `json:"rider_id"`                  // 骑手ID
	RiderName              string `json:"rider_name"`                // 骑手姓名
	DeliveryCount          int32  `json:"delivery_count"`            // 配送次数
	CompletedCount         int32  `json:"completed_count"`           // 完成次数
	AvgDeliveryTimeSeconds int32  `json:"avg_delivery_time_seconds"` // 平均配送时长(秒)
	TotalEarnings          int64  `json:"total_earnings"`            // 总收入(分)
}

// getRiderRanking 获取骑手绩效排行
// @Summary 获取骑手绩效排行榜
// @Description 获取骑手按完成配送数排序的绩效排行榜，支持分页
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Param page query int false "页码 (默认: 1)" minimum(1)
// @Param limit query int false "每页数量 (默认: 20, 最大: 100)" minimum(1) maximum(100)
// @Security BearerAuth
// @Success 200 {array} riderRankingRow "骑手绩效排行列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/riders/ranking [get]
func (server *Server) getRiderRanking(ctx *gin.Context) {
	var req getMerchantRankingRequest
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
	offset := (req.Page - 1) * req.Limit

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

	riders, err := server.store.GetRiderPerformanceRanking(ctx, db.GetRiderPerformanceRankingParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]riderRankingRow, len(riders))
	for i, rider := range riders {
		result[i] = riderRankingRow{
			RiderID:                rider.RiderID,
			RiderName:              rider.RiderName,
			DeliveryCount:          rider.DeliveryCount,
			CompletedCount:         rider.CompletedCount,
			AvgDeliveryTimeSeconds: rider.AvgDeliveryTimeSeconds,
			TotalEarnings:          rider.TotalEarnings,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 订单时段分布 ====================

type hourlyDistributionRow struct {
	Hour       int32 `json:"hour"`        // 小时(0-23)
	OrderCount int32 `json:"order_count"` // 订单数
	TotalGMV   int64 `json:"total_gmv"`   // 总GMV(分)
}

// getHourlyDistribution 获取订单时段分布
// @Summary 获取订单时段分布
// @Description 获取每小时的订单数量和GMV分布，用于分析订单高峰时段
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} hourlyDistributionRow "时段分布数据列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/hourly [get]
func (server *Server) getHourlyDistribution(ctx *gin.Context) {
	var req getPlatformOverviewRequest
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

	hours, err := server.store.GetHourlyDistribution(ctx, db.GetHourlyDistributionParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]hourlyDistributionRow, len(hours))
	for i, hour := range hours {
		result[i] = hourlyDistributionRow{
			Hour:       hour.Hour,
			OrderCount: hour.OrderCount,
			TotalGMV:   hour.TotalGmv,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 实时大盘 ====================

type realtimeDashboardResponse struct {
	Orders24h          int32 `json:"orders_24h"`           // 24小时订单数
	GMV24h             int64 `json:"gmv_24h"`              // 24小时GMV(分)
	ActiveMerchants24h int32 `json:"active_merchants_24h"` // 24小时活跃商户数
	ActiveUsers24h     int32 `json:"active_users_24h"`     // 24小时活跃用户数
	PendingOrders      int32 `json:"pending_orders"`       // 待接单订单数
	PreparingOrders    int32 `json:"preparing_orders"`     // 制作中订单数
	ReadyOrders        int32 `json:"ready_orders"`         // 待取餐订单数
	DeliveringOrders   int32 `json:"delivering_orders"`    // 配送中订单数
}

// getRealtimeDashboard 获取实时大盘数据
// @Summary 获取实时大盘数据
// @Description 获取最近24小时的实时统计数据，包括订单数、GMV及各状态订单分布
// @Tags Platform
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} realtimeDashboardResponse "实时大盘数据"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /platform/stats/realtime [get]
func (server *Server) getRealtimeDashboard(ctx *gin.Context) {
	dashboard, err := server.store.GetRealtimeDashboard(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, realtimeDashboardResponse{
		Orders24h:          dashboard.Orders24h,
		GMV24h:             dashboard.Gmv24h,
		ActiveMerchants24h: dashboard.ActiveMerchants24h,
		ActiveUsers24h:     dashboard.ActiveUsers24h,
		PendingOrders:      dashboard.PendingOrders,
		PreparingOrders:    dashboard.PreparingOrders,
		ReadyOrders:        dashboard.ReadyOrders,
		DeliveringOrders:   dashboard.DeliveringOrders,
	})
}
