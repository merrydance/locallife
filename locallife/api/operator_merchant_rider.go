package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
)

// ============================================================================
// 运营商管理商户/骑手 API
// ============================================================================

// pgNumericToFloat64 将 pgtype.Numeric 转换为 float64
func pgNumericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

// ==================== 商户列表查询 ====================

type listOperatorMerchantsRequest struct {
	Status   string `form:"status" binding:"omitempty,oneof=pending approved rejected suspended"`
	RegionID int64  `form:"region_id" binding:"omitempty,min=1"`
	Page     int32  `form:"page" binding:"omitempty,min=1"`
	Limit    int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type merchantListItem struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Phone       string  `json:"phone"`
	Address     string  `json:"address"`
	Status      string  `json:"status"`
	IsOpen      bool    `json:"is_open"`
	OwnerUserID int64   `json:"owner_user_id"`
	RegionID    int64   `json:"region_id"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CreatedAt   string  `json:"created_at"`
}

type listOperatorMerchantsResponse struct {
	Merchants []merchantListItem `json:"merchants"`
	Total     int64              `json:"total"`
	PageID    int32              `json:"page_id"`
	PageSize  int32              `json:"page_size"`
	Page      int32              `json:"page"`
	Limit     int32              `json:"limit"`
}

// listOperatorMerchants 获取运营商管辖区域内的商户列表
// @Summary 获取区域商户列表
// @Description 运营商获取其管辖区域内的所有商户，支持按状态筛选
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param status query string false "商户状态" Enums(pending, approved, rejected, suspended)
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页数量" default(20) maximum(100)
// @Success 200 {object} listOperatorMerchantsResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants [get]
func (server *Server) listOperatorMerchants(ctx *gin.Context) {
	var req listOperatorMerchantsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	// 从中间件获取运营商信息
	if _, ok := GetOperatorFromContext(ctx); !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	// 确定目标区域 ID
	targetRegionID := req.RegionID
	if targetRegionID == 0 {
		resolvedRegionID, err := server.getOperatorRegionID(ctx)
		if err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		targetRegionID = resolvedRegionID
	} else {
		// 验证是否有权管理该特定区域
		if _, err := server.checkOperatorManagesRegion(ctx, targetRegionID); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
	}

	offset := pageOffset(req.Page, req.Limit)

	// 查询商户列表
	var merchants []db.Merchant
	var total int64
	var err error

	if req.Status == "" {
		merchants, err = server.store.ListMerchantsByRegion(ctx, db.ListMerchantsByRegionParams{
			RegionID: targetRegionID,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountMerchantsByRegion(ctx, targetRegionID)
	} else {
		merchants, err = server.store.ListMerchantsByRegionWithStatus(ctx, db.ListMerchantsByRegionWithStatusParams{
			RegionID: targetRegionID,
			Column2:  req.Status,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountMerchantsByRegionWithStatus(ctx, db.CountMerchantsByRegionWithStatusParams{
			RegionID: targetRegionID,
			Column2:  req.Status,
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	result := make([]merchantListItem, len(merchants))
	for i, m := range merchants {
		result[i] = merchantListItem{
			ID:          m.ID,
			Name:        m.Name,
			Phone:       m.Phone,
			Address:     m.Address,
			Status:      m.Status,
			IsOpen:      m.IsOpen,
			OwnerUserID: m.OwnerUserID,
			RegionID:    m.RegionID,
			Latitude:    pgNumericToFloat64(m.Latitude),
			Longitude:   pgNumericToFloat64(m.Longitude),
			CreatedAt:   m.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	ctx.JSON(http.StatusOK, listOperatorMerchantsResponse{
		Merchants: result,
		Total:     total,
		PageID:    req.Page,
		PageSize:  req.Limit,
		Page:      req.Page,
		Limit:     req.Limit,
	})
}

// ==================== 商户详情查询 ====================

type getOperatorMerchantRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type merchantDetailResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	LogoAssetID int64   `json:"-"`
	LogoURL     string  `json:"logo_url,omitempty"`
	Phone       string  `json:"phone"`
	Address     string  `json:"address"`
	Status      string  `json:"status"`
	IsOpen      bool    `json:"is_open"`
	OwnerUserID int64   `json:"owner_user_id"`
	RegionID    int64   `json:"region_id"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Version     int32   `json:"version"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// getOperatorMerchant 获取商户详情
// @Summary 获取商户详情
// @Description 运营商获取其管辖区域内指定商户的详细信息
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} merchantDetailResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id} [get]
func (server *Server) getOperatorMerchant(ctx *gin.Context) {
	var req getOperatorMerchantRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户信息
	merchant, err := server.store.GetMerchant(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证运营商是否管理该商户的区域
	if _, err := server.checkOperatorManagesRegion(ctx, merchant.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	resp := merchantDetailResponse{
		ID:          merchant.ID,
		Name:        merchant.Name,
		Description: merchant.Description.String,
		LogoAssetID: merchant.LogoMediaAssetID.Int64,
		Phone:       merchant.Phone,
		Address:     merchant.Address,
		Status:      merchant.Status,
		IsOpen:      merchant.IsOpen,
		OwnerUserID: merchant.OwnerUserID,
		RegionID:    merchant.RegionID,
		Latitude:    pgNumericToFloat64(merchant.Latitude),
		Longitude:   pgNumericToFloat64(merchant.Longitude),
		Version:     merchant.Version,
		CreatedAt:   merchant.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   merchant.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if resp.LogoAssetID != 0 {
		resp.LogoURL = server.publicImageURL(ctx, &resp.LogoAssetID, media.VariantCard)
	}
	ctx.JSON(http.StatusOK, resp)
}

// ==================== 商户经营统计 ====================

type getOperatorMerchantStatsRequest struct {
	ID   int64 `uri:"id" binding:"required,min=1"`
	Days int   `form:"days" binding:"omitempty,min=1,max=365"`
}

type merchantStatsDish struct {
	DishName     string `json:"dish_name"`
	TotalSold    int32  `json:"total_sold"`
	TotalRevenue int64  `json:"total_revenue"`
}

type merchantStatsResponse struct {
	Days                      int                 `json:"days"`
	TotalOrders               int32               `json:"total_orders"`
	TotalSales                int64               `json:"total_sales"`
	TotalCommission           int64               `json:"total_commission"`
	AvgDailySales             int32               `json:"avg_daily_sales"`
	TotalCustomers            int32               `json:"total_customers"`
	RepeatCustomers           int32               `json:"repeat_customers"`
	RepurchaseRateBasisPoints int32               `json:"repurchase_rate_basis_points"`
	AvgOrdersPerUserCents     int32               `json:"avg_orders_per_user_cents"`
	TopDishes                 []merchantStatsDish `json:"top_dishes"`
}

// getOperatorMerchantStats 获取商户经营统计
// @Summary 获取商户经营统计
// @Description 运营商获取指定商户在指定天数范围内的经营统计数据
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param days query int false "统计天数（1~365，默认30）"
// @Success 200 {object} merchantStatsResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id}/stats [get]
func (server *Server) getOperatorMerchantStats(ctx *gin.Context) {
	var req getOperatorMerchantStatsRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Days == 0 {
		req.Days = 30
	}

	// 验证商户存在并属于运营商管辖区域
	merchant, err := server.store.GetMerchant(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, merchant.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	endAt := time.Now()
	startAt := endAt.AddDate(0, 0, -req.Days)

	// 概览统计
	overview, err := server.store.GetMerchantOverview(ctx, db.GetMerchantOverviewParams{
		MerchantID: req.ID,
		StartAt:    startAt,
		EndAt:      endAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 复购率
	repurchase, err := server.store.GetMerchantRepurchaseRate(ctx, db.GetMerchantRepurchaseRateParams{
		MerchantID: req.ID,
		StartAt:    startAt,
		EndAt:      endAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 热销菜品 Top 5
	topDisheRows, err := server.store.GetTopSellingDishes(ctx, db.GetTopSellingDishesParams{
		MerchantID: req.ID,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      5,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	topDishes := make([]merchantStatsDish, len(topDisheRows))
	for i, d := range topDisheRows {
		topDishes[i] = merchantStatsDish{
			DishName:     d.DishName,
			TotalSold:    d.TotalSold,
			TotalRevenue: d.TotalRevenue,
		}
	}

	ctx.JSON(http.StatusOK, merchantStatsResponse{
		Days:                      req.Days,
		TotalOrders:               overview.TotalOrders,
		TotalSales:                overview.TotalSales,
		TotalCommission:           overview.TotalCommission,
		AvgDailySales:             overview.AvgDailySales,
		TotalCustomers:            repurchase.TotalCustomers,
		RepeatCustomers:           repurchase.RepeatCustomers,
		RepurchaseRateBasisPoints: repurchase.RepurchaseRateBasisPoints,
		AvgOrdersPerUserCents:     repurchase.AvgOrdersPerUserCents,
		TopDishes:                 topDishes,
	})
}

// ==================== 骑手列表查询 ====================

type listOperatorRidersRequest struct {
	Status   string `form:"status" binding:"omitempty,oneof=approved active suspended"`
	RegionID int64  `form:"region_id" binding:"omitempty,min=1"`
	Page     int32  `form:"page" binding:"omitempty,min=1"`
	Limit    int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type riderListItem struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	RealName      string `json:"real_name"`
	Phone         string `json:"phone"`
	Status        string `json:"status"`
	IsOnline      bool   `json:"is_online"`
	RegionID      int64  `json:"region_id"`
	DepositAmount int64  `json:"deposit_amount"`
	TotalOrders   int32  `json:"total_orders"`
	TotalEarnings int64  `json:"total_earnings"`
	CreatedAt     string `json:"created_at"`
}

type listOperatorRidersResponse struct {
	Riders   []riderListItem `json:"riders"`
	Total    int64           `json:"total"`
	PageID   int32           `json:"page_id"`
	PageSize int32           `json:"page_size"`
	Page     int32           `json:"page"`
	Limit    int32           `json:"limit"`
}

// listOperatorRiders 获取运营商管辖区域内的骑手列表
// @Summary 获取区域骑手列表
// @Description 运营商获取其管辖区域内的所有骑手，支持按状态筛选
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param status query string false "骑手状态" Enums(approved, active, suspended)
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页数量" default(20) maximum(100)
// @Success 200 {object} listOperatorRidersResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders [get]
func (server *Server) listOperatorRiders(ctx *gin.Context) {
	var req listOperatorRidersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 从中间件获取运营商信息
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	// 确定目标区域 ID
	targetRegionID := req.RegionID
	if targetRegionID == 0 {
		// 默认使用主区域
		if operator.RegionID == 0 {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator has no assigned region")))
			return
		}
		targetRegionID = operator.RegionID
	} else {
		// 验证是否有权管理该特定区域
		if _, err := server.checkOperatorManagesRegion(ctx, targetRegionID); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
	}

	offset := pageOffset(req.Page, req.Limit)
	regionID := pgtype.Int8{Int64: targetRegionID, Valid: true}

	// 查询骑手列表
	var riders []db.Rider
	var total int64
	var err error

	if req.Status == "" {
		riders, err = server.store.ListRidersByRegion(ctx, db.ListRidersByRegionParams{
			RegionID: regionID,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountRidersByRegion(ctx, regionID)
	} else {
		riders, err = server.store.ListRidersByRegionWithStatus(ctx, db.ListRidersByRegionWithStatusParams{
			RegionID: regionID,
			Status:   req.Status,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountRidersByRegionWithStatus(ctx, db.CountRidersByRegionWithStatusParams{
			RegionID: regionID,
			Status:   req.Status,
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	result := make([]riderListItem, len(riders))
	for i, r := range riders {
		rRegionID := int64(0)
		if r.RegionID.Valid {
			rRegionID = r.RegionID.Int64
		}

		result[i] = riderListItem{
			ID:            r.ID,
			UserID:        r.UserID,
			RealName:      r.RealName,
			Phone:         r.Phone,
			Status:        r.Status,
			IsOnline:      r.IsOnline,
			RegionID:      rRegionID,
			DepositAmount: r.DepositAmount,
			TotalOrders:   r.TotalOrders,
			TotalEarnings: r.TotalEarnings,
			CreatedAt:     r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	ctx.JSON(http.StatusOK, listOperatorRidersResponse{
		Riders:   result,
		Total:    total,
		PageID:   req.Page,
		PageSize: req.Limit,
		Page:     req.Page,
		Limit:    req.Limit,
	})
}

// ==================== 骑手详情查询 ====================

type getOperatorRiderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type riderDetailResponse struct {
	ID                int64   `json:"id"`
	UserID            int64   `json:"user_id"`
	RealName          string  `json:"real_name"`
	Phone             string  `json:"phone"`
	IDCardNo          string  `json:"id_card_no"`
	Status            string  `json:"status"`
	IsOnline          bool    `json:"is_online"`
	RegionID          int64   `json:"region_id"`
	DepositAmount     int64   `json:"deposit_amount"`
	FrozenDeposit     int64   `json:"frozen_deposit"`
	TotalOrders       int32   `json:"total_orders"`
	TotalEarnings     int64   `json:"total_earnings"`
	CurrentLatitude   float64 `json:"current_latitude"`
	CurrentLongitude  float64 `json:"current_longitude"`
	LocationUpdatedAt string  `json:"location_updated_at,omitempty"`
	CreditScore       int16   `json:"credit_score"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// getOperatorRider 获取骑手详情
// @Summary 获取骑手详情
// @Description 运营商获取其管辖区域内指定骑手的详细信息
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "骑手ID"
// @Success 200 {object} riderDetailResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "骑手不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders/{id} [get]
func (server *Server) getOperatorRider(ctx *gin.Context) {
	var req getOperatorRiderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取骑手信息
	rider, err := server.store.GetRider(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证骑手有区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNoRegionAssigned))
		return
	}

	// 验证运营商是否管理该骑手的区域
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	updatedAt := ""
	if rider.UpdatedAt.Valid {
		updatedAt = rider.UpdatedAt.Time.Format("2006-01-02 15:04:05")
	}

	resp := riderDetailResponse{
		ID:               rider.ID,
		UserID:           rider.UserID,
		RealName:         rider.RealName,
		Phone:            rider.Phone,
		IDCardNo:         maskIDCard(rider.IDCardNo),
		Status:           rider.Status,
		IsOnline:         rider.IsOnline,
		RegionID:         rider.RegionID.Int64,
		DepositAmount:    rider.DepositAmount,
		FrozenDeposit:    rider.FrozenDeposit,
		TotalOrders:      rider.TotalOrders,
		TotalEarnings:    rider.TotalEarnings,
		CurrentLatitude:  pgNumericToFloat64(rider.CurrentLatitude),
		CurrentLongitude: pgNumericToFloat64(rider.CurrentLongitude),
		CreditScore:      rider.CreditScore,
		CreatedAt:        rider.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        updatedAt,
	}

	if rider.LocationUpdatedAt.Valid {
		resp.LocationUpdatedAt = rider.LocationUpdatedAt.Time.Format("2006-01-02 15:04:05")
	}

	ctx.JSON(http.StatusOK, resp)
}

// maskIDCard 脱敏身份证号（只显示前6位和后4位）
func maskIDCard(idCard string) string {
	if len(idCard) < 10 {
		return idCard
	}
	return idCard[:6] + "********" + idCard[len(idCard)-4:]
}

// ==================== 骑手经营统计 ====================

type getOperatorRiderStatsRequest struct {
	ID   int64 `uri:"id" binding:"required,min=1"`
	Days int   `form:"days" binding:"omitempty,min=1,max=365"`
}

type riderStatsResponse struct {
	Days                      int   `json:"days"`
	TotalDeliveries           int32 `json:"total_deliveries"`
	CompletedDeliveries       int32 `json:"completed_deliveries"`
	CompletionRateBasisPoints int32 `json:"completion_rate_basis_points"`
	AvgDeliverySeconds        int32 `json:"avg_delivery_seconds"`
	PeriodEarnings            int64 `json:"period_earnings"`
	DelayedCount              int32 `json:"delayed_count"`
}

// getOperatorRiderStats 获取骑手配送统计
// @Summary 获取骑手配送统计
// @Description 运营商获取指定骑手在指定天数范围内的配送绩效统计
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "骑手ID"
// @Param days query int false "统计天数（1~365，默认30）"
// @Success 200 {object} riderStatsResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "骑手不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders/{id}/stats [get]
func (server *Server) getOperatorRiderStats(ctx *gin.Context) {
	var req getOperatorRiderStatsRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Days == 0 {
		req.Days = 30
	}

	// 验证骑手存在并属于运营商区域
	rider, err := server.store.GetRider(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNoRegionAssigned))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	endAt := time.Now()
	startAt := endAt.AddDate(0, 0, -req.Days)

	stats, err := server.store.GetOperatorRiderStats(ctx, db.GetOperatorRiderStatsParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, riderStatsResponse{
		Days:                      req.Days,
		TotalDeliveries:           stats.TotalDeliveries,
		CompletedDeliveries:       stats.CompletedDeliveries,
		CompletionRateBasisPoints: stats.CompletionRateBasisPoints,
		AvgDeliverySeconds:        stats.AvgDeliverySeconds,
		PeriodEarnings:            stats.PeriodEarnings,
		DelayedCount:              stats.DelayedCount,
	})
}
