package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

// ==================== 区域统计 ====================

type getRegionStatsUri struct {
	RegionID int64 `uri:"region_id" binding:"required,min=1"`
}

type getRegionStatsQuery struct {
	StartDate string `form:"start_date" binding:"required"` // 格式: 2025-11-01
	EndDate   string `form:"end_date" binding:"required"`   // 格式: 2025-11-30
}

type regionStatsResponse struct {
	RegionID        int64  `json:"region_id"`
	RegionName      string `json:"region_name"`
	MerchantCount   int32  `json:"merchant_count"`
	TotalOrders     int32  `json:"total_orders"`
	TotalGMV        int64  `json:"total_gmv"`
	TotalCommission int64  `json:"total_commission"`
}

// getRegionStats 获取区域统计
// @Summary 获取区域统计
// @Description 获取指定区域在指定日期范围内的统计数据，包括商户数量、订单总数、GMV和佣金
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param region_id path int true "区域ID"
// @Param start_date query string true "开始日期 (格式: 2025-11-01)"
// @Param end_date query string true "结束日期 (格式: 2025-11-30)"
// @Success 200 {object} regionStatsResponse "区域统计数据"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限访问该区域"
// @Failure 404 {object} ErrorResponse "区域不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/regions/{region_id}/stats [get]
func (server *Server) getRegionStats(ctx *gin.Context) {
	var uri getRegionStatsUri
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var query getRegionStatsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	startDate, err := time.Parse("2006-01-02", query.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return
	}

	endDate, err := time.Parse("2006-01-02", query.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return
	}

	// 验证日期范围 (最长365天)
	if err := validateDateRange(startDate, endDate, 365); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证运营商角色和区域权限
	if _, err := server.checkOperatorManagesRegion(ctx, uri.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 查询区域统计
	stats, err := server.store.GetRegionStats(ctx, db.GetRegionStatsParams{
		ID:          uri.RegionID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("region not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, regionStatsResponse{
		RegionID:        stats.RegionID,
		RegionName:      stats.RegionName,
		MerchantCount:   stats.MerchantCount,
		TotalOrders:     stats.TotalOrders,
		TotalGMV:        stats.TotalGmv,
		TotalCommission: stats.TotalCommission,
	})
}

// ==================== 区域商户排行 ====================

type operatorMerchantRankingRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	Page      int32  `form:"page" binding:"omitempty,min=1"`
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type operatorMerchantRankingRow struct {
	MerchantID      int64  `json:"merchant_id"`
	MerchantName    string `json:"merchant_name"`
	OrderCount      int32  `json:"order_count"`
	TotalSales      int64  `json:"total_sales"`
	TotalCommission int64  `json:"total_commission"`
	AvgOrderAmount  int64  `json:"avg_order_amount"`
}

// getOperatorMerchantRanking 获取区域商户排行
// @Summary 获取商户排行
// @Description 获取运营商管理区域内商户的销售排行榜
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-11-01)"
// @Param end_date query string true "结束日期 (格式: 2025-11-30)"
// @Param page query int false "页码 (默认: 1)"
// @Param limit query int false "每页数量 (默认: 20, 最大: 100)"
// @Success 200 {array} operatorMerchantRankingRow "商户排行列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/ranking [get]
func (server *Server) getOperatorMerchantRanking(ctx *gin.Context) {
	var req operatorMerchantRankingRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
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

	merchants, err := server.store.GetOperatorMerchantRanking(ctx, db.GetOperatorMerchantRankingParams{
		RegionID:    regionID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]operatorMerchantRankingRow, len(merchants))
	for i, merchant := range merchants {
		result[i] = operatorMerchantRankingRow{
			MerchantID:      merchant.MerchantID,
			MerchantName:    merchant.MerchantName,
			OrderCount:      merchant.OrderCount,
			TotalSales:      merchant.TotalSales,
			TotalCommission: merchant.Commission,
			AvgOrderAmount:  int64(merchant.AvgOrderAmount),
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 区域骑手排行 ====================

type operatorRiderRankingRow struct {
	RiderID                int64  `json:"rider_id"`
	RiderName              string `json:"rider_name"`
	DeliveryCount          int32  `json:"delivery_count"`
	CompletedCount         int32  `json:"completed_count"`
	AvgDeliveryTimeSeconds int32  `json:"avg_delivery_time_seconds"`
	TotalEarnings          int64  `json:"total_earnings"`
}

// getOperatorRiderRanking 获取区域骑手排行
// @Summary 获取骑手排行
// @Description 获取运营商管理区域内骑手的配送绩效排行榜
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-11-01)"
// @Param end_date query string true "结束日期 (格式: 2025-11-30)"
// @Param page query int false "页码 (默认: 1)"
// @Param limit query int false "每页数量 (默认: 20, 最大: 100)"
// @Success 200 {array} operatorRiderRankingRow "骑手排行列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/riders/ranking [get]
func (server *Server) getOperatorRiderRanking(ctx *gin.Context) {
	var req operatorMerchantRankingRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
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

	riders, err := server.store.GetOperatorRiderRanking(ctx, db.GetOperatorRiderRankingParams{
		RegionID:    regionID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       req.Limit,
		Offset:      offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]operatorRiderRankingRow, len(riders))
	for i, rider := range riders {
		result[i] = operatorRiderRankingRow{
			RiderID:                rider.RiderID,
			RiderName:              rider.RiderName,
			DeliveryCount:          rider.DeliveryCount,
			CompletedCount:         rider.CompletedCount,
			AvgDeliveryTimeSeconds: rider.AvgDeliveryTime,
			TotalEarnings:          rider.TotalEarnings,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 区域日趋势 ====================

type regionDailyTrendRow struct {
	Date            string `json:"date"`
	OrderCount      int32  `json:"order_count"`
	TotalGMV        int64  `json:"total_gmv"`
	TotalCommission int64  `json:"total_commission"`
	ActiveMerchants int32  `json:"active_merchants"`
	ActiveUsers     int32  `json:"active_users"`
}

type getOperatorStatsRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

// getRegionDailyTrend 获取区域日趋势
// @Summary 获取每日趋势
// @Description 获取运营商管理区域的每日订单、GMV、佣金等趋势数据
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-11-01)"
// @Param end_date query string true "结束日期 (格式: 2025-11-30)"
// @Success 200 {array} regionDailyTrendRow "每日趋势数据"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/trend/daily [get]
func (server *Server) getRegionDailyTrend(ctx *gin.Context) {
	var req getOperatorStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
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

	trends, err := server.store.GetRegionDailyTrend(ctx, db.GetRegionDailyTrendParams{
		RegionID:    regionID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]regionDailyTrendRow, len(trends))
	for i, trend := range trends {
		result[i] = regionDailyTrendRow{
			Date:            trend.Date.Time.Format("2006-01-02"),
			OrderCount:      trend.OrderCount,
			TotalGMV:        trend.TotalGmv,
			TotalCommission: trend.Commission,
			ActiveMerchants: trend.ActiveMerchants,
			ActiveUsers:     trend.ActiveUsers,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 运营商财务概览 ====================

type operatorFinanceOverviewResponse struct {
	// 当月统计
	CurrentMonth struct {
		TotalGMV          int64 `json:"total_gmv"`          // 区域总交易额
		TotalCommission   int64 `json:"total_commission"`   // 平台佣金（运营商可分得 60%）
		TotalOrders       int32 `json:"total_orders"`       // 订单数
		SettledCommission int64 `json:"settled_commission"` // 已完成分账佣金
		PendingCommission int64 `json:"pending_commission"` // 待分账佣金（当月订单尚未分账部分）
	} `json:"current_month"`

	// 累计统计（基于分账完成的订单）
	Total struct {
		TotalGMV          int64 `json:"total_gmv"`          // 累计交易额
		TotalCommission   int64 `json:"total_commission"`   // 累计平台佣金
		SettledCommission int64 `json:"settled_commission"` // 已结算（分账完成）
	} `json:"total"`

	// 区域信息
	RegionID   int64  `json:"region_id"`
	RegionName string `json:"region_name"`
}

// getOperatorFinanceOverview 获取运营商财务概览
// @Summary 获取财务概览
// @Description 获取运营商的财务概览信息，数据直接从分账记录（profit_sharing_orders）实时统计
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Success 200 {object} operatorFinanceOverviewResponse "财务概览"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operators/me/finance/overview [get]
func (server *Server) getOperatorFinanceOverview(ctx *gin.Context) {
	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 获取区域信息
	region, err := server.store.GetRegion(ctx, regionID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := operatorFinanceOverviewResponse{
		RegionID:   regionID,
		RegionName: region.Name,
	}

	// 当月日期范围
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)

	// 查询当月统计（从 profit_sharing_orders 实时计算，只统计分账成功的）
	monthStats, err := server.store.GetRegionStats(ctx, db.GetRegionStatsParams{
		ID:          regionID,
		CreatedAt:   monthStart,
		CreatedAt_2: monthEnd,
	})
	if err == nil {
		response.CurrentMonth.TotalGMV = monthStats.TotalGmv
		response.CurrentMonth.TotalCommission = monthStats.TotalCommission
		response.CurrentMonth.TotalOrders = monthStats.TotalOrders
		// 分账完成的佣金 = 统计的佣金（因为 GetRegionStats 只统计 status='finished' 的记录）
		response.CurrentMonth.SettledCommission = monthStats.TotalCommission
		// 微信电商分账是实时的，不存在"待分账"状态
		response.CurrentMonth.PendingCommission = 0
	}

	// 查询累计统计（全部历史分账成功的订单）
	// 使用一个很早的开始时间和很晚的结束时间来获取全部历史数据
	allTimeStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	allTimeEnd := time.Date(2099, 12, 31, 23, 59, 59, 0, time.Local)

	totalStats, err := server.store.GetRegionStats(ctx, db.GetRegionStatsParams{
		ID:          regionID,
		CreatedAt:   allTimeStart,
		CreatedAt_2: allTimeEnd,
	})
	if err == nil {
		response.Total.TotalGMV = totalStats.TotalGmv
		response.Total.TotalCommission = totalStats.TotalCommission
		response.Total.SettledCommission = totalStats.TotalCommission // 全部是已分账的
	}

	ctx.JSON(http.StatusOK, response)
}

// ==================== 运营商佣金明细 ====================

type operatorCommissionRequest struct {
	StartDate string `form:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate   string `form:"end_date" binding:"required"`   // YYYY-MM-DD
	Page      int32  `form:"page" binding:"omitempty,min=1"`
	Limit     int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type operatorCommissionItem struct {
	Date           string `json:"date"`
	OrderCount     int32  `json:"order_count"`
	TotalGMV       int64  `json:"total_gmv"`
	CommissionRate string `json:"commission_rate"` // 如 "3%"
	Commission     int64  `json:"commission"`      // 佣金金额
}

type operatorCommissionResponse struct {
	Items      []operatorCommissionItem `json:"items"`
	Total      int64                    `json:"total"`
	TotalCount int64                    `json:"total_count"`
	Page       int32                    `json:"page"`
	Limit      int32                    `json:"limit"`
	Summary    struct {
		TotalGMV        int64 `json:"total_gmv"`
		TotalCommission int64 `json:"total_commission"`
		TotalOrders     int32 `json:"total_orders"`
	} `json:"summary"`
}

// getOperatorCommission 获取运营商佣金明细
// @Summary 获取佣金明细
// @Description 获取运营商的每日佣金明细，支持分页
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-11-01)"
// @Param end_date query string true "结束日期 (格式: 2025-11-30)"
// @Param page query int false "页码 (默认: 1)"
// @Param limit query int false "每页数量 (默认: 20, 最大: 100)"
// @Success 200 {object} operatorCommissionResponse "佣金明细"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operators/me/commission [get]
func (server *Server) getOperatorCommission(ctx *gin.Context) {
	var req operatorCommissionRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
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

	// 默认分页
	page := req.Page
	if page == 0 {
		page = 1
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}

	// 获取每日佣金趋势数据
	trends, err := server.store.GetRegionDailyTrend(ctx, db.GetRegionDailyTrendParams{
		RegionID:    regionID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := operatorCommissionResponse{
		Items:      []operatorCommissionItem{},
		TotalCount: int64(len(trends)),
		Page:       page,
		Limit:      limit,
	}

	// 计算汇总
	var totalGMV, totalCommission int64
	var totalOrders int32
	for _, trend := range trends {
		totalGMV += trend.TotalGmv
		totalCommission += trend.Commission
		totalOrders += trend.OrderCount
	}
	response.Summary.TotalGMV = totalGMV
	response.Summary.TotalCommission = totalCommission
	response.Summary.TotalOrders = totalOrders
	response.Total = totalCommission

	// 分页处理
	startIdx := int((page - 1) * limit)
	endIdx := startIdx + int(limit)
	if startIdx >= len(trends) {
		ctx.JSON(http.StatusOK, response)
		return
	}
	if endIdx > len(trends) {
		endIdx = len(trends)
	}

	// 转换数据
	for _, trend := range trends[startIdx:endIdx] {
		// 计算佣金率
		rate := "3.0%"
		if trend.TotalGmv > 0 {
			actualRate := float64(trend.Commission) / float64(trend.TotalGmv) * 100
			rate = formatCommissionRate(actualRate)
		}

		response.Items = append(response.Items, operatorCommissionItem{
			Date:           trend.Date.Time.Format("2006-01-02"),
			OrderCount:     trend.OrderCount,
			TotalGMV:       trend.TotalGmv,
			CommissionRate: rate,
			Commission:     trend.Commission,
		})
	}

	ctx.JSON(http.StatusOK, response)
}

// formatCommissionRate 格式化佣金率
func formatCommissionRate(rate float64) string {
	if rate == 0 {
		return "0%"
	}
	// 使用 fmt.Sprintf 正确格式化佣金率（保留一位小数）
	return fmt.Sprintf("%.1f%%", rate)
}

// validateDateRange 验证日期范围
// 返回错误如果：startDate > endDate 或者日期范围超过 maxDays 天
func validateDateRange(startDate, endDate time.Time, maxDays int) error {
	if startDate.After(endDate) {
		return errors.New("start_date must be before or equal to end_date")
	}
	if maxDays > 0 && endDate.Sub(startDate).Hours()/24 > float64(maxDays) {
		return fmt.Errorf("date range cannot exceed %d days", maxDays)
	}
	return nil
}
