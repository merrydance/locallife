package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
	Keyword  string `form:"keyword"`
	Page     int32  `form:"page" binding:"omitempty,min=1"`
	Limit    int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type operatorMerchantSummaryRequest struct {
	RegionID int64 `form:"region_id" binding:"omitempty,min=1"`
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

type operatorMerchantSummaryResponse struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Approved  int64 `json:"approved"`
	Rejected  int64 `json:"rejected"`
	Suspended int64 `json:"suspended"`
}

func operatorMerchantStatusFilters(status string) []string {
	if status == "" {
		return []string{}
	}
	if status == db.MerchantStatusApproved {
		return []string{db.MerchantStatusApproved, db.MerchantStatusActive}
	}
	return []string{status}
}

func (server *Server) listOperatorMerchantRows(
	ctx *gin.Context,
	regionIDs []int64,
	statuses []string,
	keyword string,
	offset int32,
	limit int32,
) ([]db.Merchant, int64, error) {
	if statuses == nil {
		statuses = []string{}
	}

	query := db.ListOperatorMerchantsParams{
		RegionIds: regionIDs,
		Statuses:  statuses,
		Keyword:   keyword,
		Offset:    offset,
		Limit:     limit,
	}
	merchants, err := server.store.ListOperatorMerchants(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	total, err := server.store.CountOperatorMerchants(ctx, db.CountOperatorMerchantsParams{
		RegionIds: regionIDs,
		Statuses:  statuses,
		Keyword:   keyword,
	})
	if err != nil {
		return nil, 0, err
	}

	return merchants, total, nil
}

// listOperatorMerchants 获取运营商管辖区域内的商户列表
// @Summary 获取区域商户列表
// @Description 运营商获取其管辖区域内的所有商户，支持按状态和名称/电话关键字筛选；不传 region_id 时聚合全部可管区域，status=approved 会包含已激活商户
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param status query string false "商户状态；approved 表示正常商户，包含 approved 与 active" Enums(pending, approved, rejected, suspended)
// @Param region_id query int false "区域ID；不传时聚合当前运营商全部可管区域"
// @Param keyword query string false "商户名称或手机号关键字，前后空白会被忽略，最长 50 字符"
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

	selection, err := resolveListOperatorMerchantRegionSelection(server, ctx, req.RegionID)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	offset := pageOffset(req.Page, req.Limit)
	statuses := operatorMerchantStatusFilters(req.Status)
	keyword := strings.TrimSpace(req.Keyword)
	if utf8.RuneCountInString(keyword) > 50 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("keyword is too long")))
		return
	}
	merchants, total, err := server.listOperatorMerchantRows(ctx, selection.RegionIDs, statuses, keyword, offset, req.Limit)
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

func resolveListOperatorMerchantRegionSelection(server *Server, ctx *gin.Context, requestedRegionID int64) (operatorRegionSelection, error) {
	if requestedRegionID > 0 {
		if _, err := server.checkOperatorManagesRegion(ctx, requestedRegionID); err != nil {
			return operatorRegionSelection{}, err
		}
		return operatorRegionSelection{RegionID: requestedRegionID, RegionIDs: []int64{requestedRegionID}}, nil
	}
	return server.resolveOperatorRegionSelection(ctx)
}

// getOperatorMerchantSummary 获取区域商户汇总
// @Summary 获取区域商户汇总
// @Description 运营商获取管辖区域内商户总数及各状态汇总，供工作台和审批入口使用
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param region_id query int false "区域ID；不传时聚合当前运营商全部可管区域"
// @Success 200 {object} operatorMerchantSummaryResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/summary [get]
func (server *Server) getOperatorMerchantSummary(ctx *gin.Context) {
	var req operatorMerchantSummaryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	countStatus := func(status string) (int64, error) {
		var total int64
		for _, regionID := range selection.RegionIDs {
			if status == "" {
				count, countErr := server.store.CountMerchantsByRegion(ctx, regionID)
				if countErr != nil {
					return 0, countErr
				}
				total += count
				continue
			}

			count, countErr := server.store.CountMerchantsByRegionWithStatus(ctx, db.CountMerchantsByRegionWithStatusParams{
				RegionID: regionID,
				Column2:  status,
			})
			if countErr != nil {
				return 0, countErr
			}
			total += count
		}
		return total, nil
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pending, err := countStatus("pending")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	approved, err := countStatus("approved")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	rejected, err := countStatus("rejected")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	suspended, err := countStatus("suspended")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, operatorMerchantSummaryResponse{
		Total:     total,
		Pending:   pending,
		Approved:  approved,
		Rejected:  rejected,
		Suspended: suspended,
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

type merchantCapabilitiesResponse struct {
	MerchantID        int64    `json:"merchant_id"`
	OpenKitchenStatus string   `json:"open_kitchen_status"`
	DineInStatus      string   `json:"dine_in_status"`
	SystemLabels      []string `json:"system_labels,omitempty"`
	Source            string   `json:"source"`
	Note              string   `json:"note,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

type updateMerchantCapabilitiesRequest struct {
	OpenKitchenStatus *string `json:"open_kitchen_status" binding:"omitempty,oneof=unknown yes no"`
	DineInStatus      *string `json:"dine_in_status" binding:"omitempty,oneof=unknown yes no"`
	Note              *string `json:"note" binding:"omitempty,max=500"`
}

func deriveMerchantSystemLabelNames(capability db.MerchantCapability) []string {
	labels := make([]string, 0, 2)

	switch capability.OpenKitchenStatus {
	case db.MerchantCapabilityStatusYes:
		labels = append(labels, db.SystemTagHasOpenKitchen)
	case db.MerchantCapabilityStatusUnknown, db.MerchantCapabilityStatusNo:
		labels = append(labels, db.SystemTagNoOpenKitchen)
	}

	if capability.DineInStatus == db.MerchantCapabilityStatusNo {
		labels = append(labels, db.SystemTagNoDineIn)
	}

	return labels
}

func merchantCapabilitiesResponseFromModel(merchantID int64, capability db.MerchantCapability, labels []db.Tag) merchantCapabilitiesResponse {
	resp := merchantCapabilitiesResponse{
		MerchantID:        merchantID,
		OpenKitchenStatus: capability.OpenKitchenStatus,
		DineInStatus:      capability.DineInStatus,
		Source:            capability.Source,
	}
	if !capability.UpdatedAt.IsZero() {
		resp.UpdatedAt = capability.UpdatedAt.Format("2006-01-02 15:04:05")
	}
	if capability.Note.Valid {
		resp.Note = capability.Note.String
	}
	if len(labels) > 0 {
		resp.SystemLabels = make([]string, len(labels))
		for i, label := range labels {
			resp.SystemLabels[i] = label.Name
		}
	} else {
		resp.SystemLabels = deriveMerchantSystemLabelNames(capability)
	}
	return resp
}

// getOperatorMerchantCapabilities 获取商户能力与系统标签
// @Summary 获取商户能力标签
// @Description 运营商获取其管辖区域内商户的能力真值及派生系统标签
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} merchantCapabilitiesResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id}/capabilities [get]
func (server *Server) getOperatorMerchantCapabilities(ctx *gin.Context) {
	var req getOperatorMerchantRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

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

	capabilityFound := true
	capability, err := server.store.GetMerchantCapabilities(ctx, merchant.ID)
	if err != nil {
		if !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		capabilityFound = false
		capability = db.MerchantCapability{
			MerchantID:        merchant.ID,
			OpenKitchenStatus: db.MerchantCapabilityStatusUnknown,
			DineInStatus:      db.MerchantCapabilityStatusUnknown,
			Source:            db.MerchantCapabilitySourceSystemDefault,
		}
	}

	var labels []db.Tag
	if capabilityFound {
		labels, err = server.store.ListMerchantSystemLabels(ctx, merchant.ID)
		if err != nil && !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	ctx.JSON(http.StatusOK, merchantCapabilitiesResponseFromModel(merchant.ID, capability, labels))
}

// updateOperatorMerchantCapabilities 更新商户能力与系统标签
// @Summary 更新商户能力标签
// @Description 运营商更新其管辖区域内商户的能力真值，并同步派生系统标签
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body updateMerchantCapabilitiesRequest true "能力更新请求"
// @Success 200 {object} merchantCapabilitiesResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id}/capabilities [patch]
func (server *Server) updateOperatorMerchantCapabilities(ctx *gin.Context) {
	var uriReq getOperatorMerchantRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var req updateMerchantCapabilitiesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.OpenKitchenStatus == nil && req.DineInStatus == nil && req.Note == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("at least one capability field is required")))
		return
	}

	merchant, err := server.store.GetMerchant(ctx, uriReq.ID)
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

	params := db.UpdateMerchantCapabilitiesTxParams{
		MerchantID:        uriReq.ID,
		Source:            db.MerchantCapabilitySourceManualReview,
		OpenKitchenStatus: pgtype.Text{Valid: false},
		DineInStatus:      pgtype.Text{Valid: false},
		Note:              pgtype.Text{Valid: false},
	}
	if req.OpenKitchenStatus != nil {
		params.OpenKitchenStatus = pgtype.Text{String: *req.OpenKitchenStatus, Valid: true}
	}
	if req.DineInStatus != nil {
		params.DineInStatus = pgtype.Text{String: *req.DineInStatus, Valid: true}
	}
	if req.Note != nil {
		params.Note = pgtype.Text{String: *req.Note, Valid: true}
	}

	result, err := server.store.UpdateMerchantCapabilitiesTx(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, merchantCapabilitiesResponseFromModel(uriReq.ID, result.Capability, result.SystemLabels))
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
	Status       string `form:"status" binding:"omitempty,oneof=approved active suspended"`
	RegionID     int64  `form:"region_id" binding:"omitempty,min=1"`
	Keyword      string `form:"keyword"`
	OnlineStatus string `form:"online_status"`
	Page         int32  `form:"page" binding:"omitempty,min=1"`
	Limit        int32  `form:"limit" binding:"omitempty,min=1,max=100"`
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

type operatorRiderSummaryResponse struct {
	Total     int64 `json:"total"`
	Approved  int64 `json:"approved"`
	Active    int64 `json:"active"`
	Suspended int64 `json:"suspended"`
	Online    int64 `json:"online"`
}

func operatorRiderStatusFilters(status string) []string {
	if status == "" {
		return []string{}
	}
	return []string{status}
}

func normalizeOperatorRiderOnlineStatus(input string) (string, error) {
	onlineStatus := strings.TrimSpace(input)
	switch onlineStatus {
	case "", "online", "offline":
		return onlineStatus, nil
	default:
		return "", errors.New("online_status must be online or offline")
	}
}

func (server *Server) listOperatorRiderRows(
	ctx *gin.Context,
	regionIDs []int64,
	statuses []string,
	keyword string,
	onlineStatus string,
	offset int32,
	limit int32,
) ([]db.Rider, int64, error) {
	if statuses == nil {
		statuses = []string{}
	}

	query := db.ListOperatorRidersParams{
		RegionIds:    regionIDs,
		Statuses:     statuses,
		Keyword:      keyword,
		OnlineStatus: onlineStatus,
		Offset:       offset,
		Limit:        limit,
	}
	riders, err := server.store.ListOperatorRiders(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	total, err := server.store.CountOperatorRiders(ctx, db.CountOperatorRidersParams{
		RegionIds:    regionIDs,
		Statuses:     statuses,
		Keyword:      keyword,
		OnlineStatus: onlineStatus,
	})
	if err != nil {
		return nil, 0, err
	}

	return riders, total, nil
}

// listOperatorRiders 获取运营商管辖区域内的骑手列表
// @Summary 获取区域骑手列表
// @Description 运营商获取其管辖区域内的所有骑手，支持按生命周期状态、姓名/电话关键字和在线状态筛选；不传 region_id 时聚合全部可管区域
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param status query string false "骑手状态" Enums(approved, active, suspended)
// @Param region_id query int false "区域ID；不传时聚合当前运营商全部可管区域"
// @Param keyword query string false "骑手姓名或手机号关键字，前后空白会被忽略，最长 50 字符"
// @Param online_status query string false "在线状态；映射 riders.is_online 当前存储值" Enums(online, offline)
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

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	keyword := strings.TrimSpace(req.Keyword)
	if utf8.RuneCountInString(keyword) > 50 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("keyword is too long")))
		return
	}
	onlineStatus, err := normalizeOperatorRiderOnlineStatus(req.OnlineStatus)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	selection, err := resolveListOperatorRiderRegionSelection(server, ctx, req.RegionID)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	offset := pageOffset(req.Page, req.Limit)
	statuses := operatorRiderStatusFilters(req.Status)
	riders, total, err := server.listOperatorRiderRows(ctx, selection.RegionIDs, statuses, keyword, onlineStatus, offset, req.Limit)
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

func resolveListOperatorRiderRegionSelection(server *Server, ctx *gin.Context, requestedRegionID int64) (operatorRegionSelection, error) {
	if requestedRegionID > 0 {
		if _, err := server.checkOperatorManagesRegion(ctx, requestedRegionID); err != nil {
			return operatorRegionSelection{}, err
		}
		return operatorRegionSelection{RegionID: requestedRegionID, RegionIDs: []int64{requestedRegionID}}, nil
	}
	return server.resolveOperatorRegionSelection(ctx)
}

// getOperatorRiderSummary 获取区域骑手汇总
// @Summary 获取区域骑手汇总
// @Description 运营商获取管辖区域内骑手总数及各状态汇总，供工作台和审批入口使用
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param region_id query int false "区域ID；不传时聚合当前运营商全部可管区域"
// @Success 200 {object} operatorRiderSummaryResponse
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders/summary [get]
func (server *Server) getOperatorRiderSummary(ctx *gin.Context) {
	var req listOperatorRidersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	countStatus := func(status string) (int64, error) {
		var total int64
		for _, regionID := range selection.RegionIDs {
			regionScope := pgtype.Int8{Int64: regionID, Valid: true}
			if status == "" {
				count, countErr := server.store.CountRidersByRegion(ctx, regionScope)
				if countErr != nil {
					return 0, countErr
				}
				total += count
				continue
			}

			count, countErr := server.store.CountRidersByRegionWithStatus(ctx, db.CountRidersByRegionWithStatusParams{
				RegionID: regionScope,
				Status:   status,
			})
			if countErr != nil {
				return 0, countErr
			}
			total += count
		}
		return total, nil
	}

	total, err := countStatus("")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	approved, err := countStatus(db.RiderStatusApproved)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	active, err := countStatus(db.RiderStatusActive)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	suspended, err := countStatus(db.RiderStatusSuspended)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	var online int64
	for _, regionID := range selection.RegionIDs {
		count, countErr := server.store.CountOnlineRidersByRegion(ctx, pgtype.Int8{Int64: regionID, Valid: true})
		if countErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, countErr))
			return
		}
		online += count
	}

	ctx.JSON(http.StatusOK, operatorRiderSummaryResponse{
		Total:     total,
		Approved:  approved,
		Active:    active,
		Suspended: suspended,
		Online:    online,
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

// getOperatorRiderStats 获取骑手代取统计
// @Summary 获取骑手代取统计
// @Description 运营商获取指定骑手在指定天数范围内的代取绩效统计
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
