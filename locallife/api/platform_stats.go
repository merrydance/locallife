package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
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
// @Router /v1/platform/stats/overview [get]
func (server *Server) getPlatformOverview(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetPlatformOverview(ctx, db.GetPlatformOverviewParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_overview_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/daily [get]
func (server *Server) getPlatformDailyStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetPlatformDailyStats(ctx, db.GetPlatformDailyStatsParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_daily_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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

// ==================== 分账对账汇总 ====================

type platformProfitSharingReconciliationRow struct {
	Status                  string `json:"status"`
	TotalOrders             int64  `json:"total_orders"`
	TotalAmount             int64  `json:"total_amount"`
	TotalPlatformCommission int64  `json:"total_platform_commission"`
	TotalOperatorCommission int64  `json:"total_operator_commission"`
}

// getPlatformProfitSharingReconciliation 获取分账对账汇总
// @Summary 获取分账对账汇总
// @Description 获取指定时间范围内分账订单的对账汇总数据，按状态汇总订单数与金额
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} platformProfitSharingReconciliationRow "分账对账汇总"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/stats/profit-sharing/reconciliation [get]
func (server *Server) getPlatformProfitSharingReconciliation(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	rows, err := server.store.GetProfitSharingReconciliationSummary(ctx, db.GetProfitSharingReconciliationSummaryParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_profit_sharing_reconciliation_viewed",
		TargetType:  "profit_sharing_orders",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

	result := make([]platformProfitSharingReconciliationRow, len(rows))
	for i, row := range rows {
		result[i] = platformProfitSharingReconciliationRow{
			Status:                  row.Status,
			TotalOrders:             row.TotalOrders,
			TotalAmount:             row.TotalAmount,
			TotalPlatformCommission: row.TotalPlatformCommission,
			TotalOperatorCommission: row.TotalOperatorCommission,
		}
	}

	ctx.JSON(http.StatusOK, result)
}

// ==================== 分账 SLA 汇总 ====================

type platformProfitSharingSlaSummaryResponse struct {
	TotalOrders      int64 `json:"total_orders"`
	FinishedOrders   int64 `json:"finished_orders"`
	FailedOrders     int64 `json:"failed_orders"`
	PendingOrders    int64 `json:"pending_orders"`
	AvgFinishSeconds int64 `json:"avg_finish_seconds"`
	P95FinishSeconds int64 `json:"p95_finish_seconds"`
}

// getPlatformProfitSharingSlaSummary 获取分账 SLA 汇总
// @Summary 获取分账 SLA 汇总
// @Description 获取指定时间范围内分账处理 SLA 统计（完成/失败/待处理与处理耗时）
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {object} platformProfitSharingSlaSummaryResponse "分账 SLA 汇总"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/stats/profit-sharing/sla [get]
func (server *Server) getPlatformProfitSharingSlaSummary(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	stats, err := server.store.GetProfitSharingSlaSummary(ctx, db.GetProfitSharingSlaSummaryParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_profit_sharing_sla_viewed",
		TargetType:  "profit_sharing_orders",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

	ctx.JSON(http.StatusOK, platformProfitSharingSlaSummaryResponse{
		TotalOrders:      stats.TotalOrders,
		FinishedOrders:   stats.FinishedOrders,
		FailedOrders:     stats.FailedOrders,
		PendingOrders:    stats.PendingOrders,
		AvgFinishSeconds: stats.AvgFinishSeconds,
		P95FinishSeconds: stats.P95FinishSeconds,
	})
}

// ==================== 分账规则审计 ====================

type listProfitSharingConfigAuditsRequest struct {
	ConfigID int64 `form:"config_id"`
	Page     int32 `form:"page" binding:"omitempty,min=1"`
	Limit    int32 `form:"limit" binding:"omitempty,min=1,max=200"`
}

type profitSharingConfigAuditItem struct {
	ID        int64           `json:"id"`
	ConfigID  int64           `json:"config_id"`
	Action    string          `json:"action"`
	ActorID   *int64          `json:"actor_id,omitempty"`
	ActorRole *string         `json:"actor_role,omitempty"`
	Detail    json.RawMessage `json:"detail"`
	CreatedAt string          `json:"created_at"`
}

type listProfitSharingConfigAuditsResponse struct {
	Items []profitSharingConfigAuditItem `json:"items"`
	Page  int32                          `json:"page"`
	Limit int32                          `json:"limit"`
}

// getPlatformProfitSharingConfigAudits 获取分账规则审计记录
// @Summary 获取分账规则审计记录
// @Description 获取分账规则配置的审计记录，支持按配置ID过滤
// @Tags Platform
// @Accept json
// @Produce json
// @Param config_id query int false "配置ID"
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(200)
// @Security BearerAuth
// @Success 200 {object} listProfitSharingConfigAuditsResponse "审计记录列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/stats/profit-sharing/config-audits [get]
func (server *Server) getPlatformProfitSharingConfigAudits(ctx *gin.Context) {
	var req listProfitSharingConfigAuditsRequest
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

	items, err := server.store.ListProfitSharingConfigAudits(ctx, db.ListProfitSharingConfigAuditsParams{
		Column1: req.ConfigID,
		Limit:   req.Limit,
		Offset:  pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_profit_sharing_config_audits_viewed",
		TargetType:  "profit_sharing_configs",
		RegionID:    nil,
		Metadata: map[string]any{
			"config_id": req.ConfigID,
			"page":      req.Page,
			"limit":     req.Limit,
		},
	})

	responseItems := make([]profitSharingConfigAuditItem, len(items))
	for i, item := range items {
		var actorID *int64
		if item.ActorID.Valid {
			actorID = &item.ActorID.Int64
		}
		var actorRole *string
		if item.ActorRole.Valid {
			role := item.ActorRole.String
			actorRole = &role
		}
		responseItems[i] = profitSharingConfigAuditItem{
			ID:        item.ID,
			ConfigID:  item.ConfigID,
			Action:    item.Action,
			ActorID:   actorID,
			ActorRole: actorRole,
			Detail:    json.RawMessage(item.Detail),
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		}
	}

	ctx.JSON(http.StatusOK, listProfitSharingConfigAuditsResponse{
		Items: responseItems,
		Page:  req.Page,
		Limit: req.Limit,
	})
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
// @Router /v1/platform/stats/regions/compare [get]
func (server *Server) getRegionComparison(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regions, err := server.store.GetRegionComparison(ctx, db.GetRegionComparisonParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_regions_compared",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/merchants/ranking [get]
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
	offset := pageOffset(req.Page, req.Limit)

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchants, err := server.store.GetMerchantRanking(ctx, db.GetMerchantRankingParams{
		StartAt: startDate,
		EndAt:   endDate,
		Limit:   req.Limit,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_merchant_ranking_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
			"page":       req.Page,
			"limit":      req.Limit,
		},
	})

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
// @Router /v1/platform/stats/categories [get]
func (server *Server) getCategoryStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	categories, err := server.store.GetCategoryStats(ctx, db.GetCategoryStatsParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_category_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/growth/users [get]
func (server *Server) getUserGrowthStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetUserGrowthStats(ctx, db.GetUserGrowthStatsParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_user_growth_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/growth/merchants [get]
func (server *Server) getMerchantGrowthStats(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	stats, err := server.store.GetMerchantGrowthStats(ctx, db.GetMerchantGrowthStatsParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_merchant_growth_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/riders/ranking [get]
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
	offset := pageOffset(req.Page, req.Limit)

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	riders, err := server.store.GetRiderPerformanceRanking(ctx, db.GetRiderPerformanceRankingParams{
		StartAt: startDate,
		EndAt:   endDate,
		Limit:   req.Limit,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_rider_ranking_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
			"page":       req.Page,
			"limit":      req.Limit,
		},
	})

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
// @Router /v1/platform/stats/hourly [get]
func (server *Server) getHourlyDistribution(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	hours, err := server.store.GetHourlyDistribution(ctx, db.GetHourlyDistributionParams{
		StartAt: startDate,
		EndAt:   endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_hourly_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

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
// @Router /v1/platform/stats/realtime [get]
func (server *Server) getRealtimeDashboard(ctx *gin.Context) {
	dashboard, err := server.store.GetRealtimeDashboard(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_stats_realtime_viewed",
		TargetType:  "platform_stats",
		RegionID:    nil,
		Metadata:    map[string]any{},
	})

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

// ==================== 每日账单对账报告 ====================

type listBillReconciliationReportsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=1,max=100"`
}

type billReconciliationReportResponse struct {
	ID             int64           `json:"id"`
	BillDate       string          `json:"bill_date"` // "2006-01-02"
	BillType       string          `json:"bill_type"` // trade | ecommerce_trade | refund
	Status         string          `json:"status"`    // pending | running | completed | failed
	WxpayCount     int32           `json:"wxpay_count"`
	LocalCount     int32           `json:"local_count"`
	MismatchCount  int32           `json:"mismatch_count"`
	MissingLocal   json.RawMessage `json:"missing_local"`
	MissingWxpay   json.RawMessage `json:"missing_wxpay"`
	AmountMismatch json.RawMessage `json:"amount_mismatch"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

// getBillReconciliationReports 获取每日账单对账报告列表
// @Summary 获取每日账单对账报告列表
// @Description 列出微信支付账单与本地数据库的对账报告，按账单日期倒序排列
// @Tags Platform
// @Accept json
// @Produce json
// @Param page_id query int true "页码 (从1开始)"
// @Param page_size query int true "每页条数 (1-100)"
// @Security BearerAuth
// @Success 200 {array} billReconciliationReportResponse "对账报告列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/stats/bill-reconciliation [get]
func (server *Server) getBillReconciliationReports(ctx *gin.Context) {
	var req listBillReconciliationReportsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	reports, err := server.store.ListReconciliationReports(ctx, db.ListReconciliationReportsParams{
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]billReconciliationReportResponse, len(reports))
	for i, r := range reports {
		resp := billReconciliationReportResponse{
			ID:             r.ID,
			BillDate:       r.BillDate.Time.Format("2006-01-02"),
			BillType:       r.BillType,
			Status:         r.Status,
			WxpayCount:     r.WxpayCount,
			LocalCount:     r.LocalCount,
			MismatchCount:  r.MismatchCount,
			MissingLocal:   json.RawMessage(r.MissingLocal),
			MissingWxpay:   json.RawMessage(r.MissingWxpay),
			AmountMismatch: json.RawMessage(r.AmountMismatch),
			CreatedAt:      r.CreatedAt.Format(timeLayout),
			UpdatedAt:      r.UpdatedAt.Time.Format(timeLayout),
		}
		if r.ErrorMessage.Valid {
			resp.ErrorMessage = &r.ErrorMessage.String
		}
		result[i] = resp
	}

	ctx.JSON(http.StatusOK, result)
}
