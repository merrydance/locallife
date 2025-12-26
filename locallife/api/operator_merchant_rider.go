package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
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
	Status string `form:"status" binding:"omitempty,oneof=pending approved rejected suspended"`
	Page   int32  `form:"page" binding:"omitempty,min=1"`
	Limit  int32  `form:"limit" binding:"omitempty,min=1,max=100"`
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
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	// 获取运营商管理的区域
	if operator.RegionID == 0 {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator has no assigned region")))
		return
	}

	offset := (req.Page - 1) * req.Limit

	// 查询商户列表
	var merchants []db.Merchant
	var total int64
	var err error

	if req.Status == "" {
		merchants, err = server.store.ListMerchantsByRegion(ctx, db.ListMerchantsByRegionParams{
			RegionID: operator.RegionID,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountMerchantsByRegion(ctx, operator.RegionID)
	} else {
		merchants, err = server.store.ListMerchantsByRegionWithStatus(ctx, db.ListMerchantsByRegionWithStatusParams{
			RegionID: operator.RegionID,
			Column2:  req.Status,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err = server.store.CountMerchantsByRegionWithStatus(ctx, db.CountMerchantsByRegionWithStatusParams{
			RegionID: operator.RegionID,
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
	LogoURL     string  `json:"logo_url"`
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
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
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

	ctx.JSON(http.StatusOK, merchantDetailResponse{
		ID:          merchant.ID,
		Name:        merchant.Name,
		Description: merchant.Description.String,
		LogoURL:     normalizeUploadURLForClient(merchant.LogoUrl.String),
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
	})
}

// ==================== 暂停/恢复商户 ====================

type suspendOperatorMerchantRequest struct {
	Reason        string `json:"reason" binding:"required,min=5,max=500"`
	DurationHours int    `json:"duration_hours" binding:"required,min=1,max=720"`
}

// suspendOperatorMerchant 运营商暂停商户
// @Summary 暂停商户
// @Description 运营商暂停其管辖区域内的商户，最长可暂停30天（720小时）
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body suspendOperatorMerchantRequest true "暂停原因和时长"
// @Success 200 {object} MessageResponse "暂停成功"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id}/suspend [post]
func (server *Server) suspendOperatorMerchant(ctx *gin.Context) {
	merchantIDStr := ctx.Param("id")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的商户ID")))
		return
	}

	var req suspendOperatorMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户信息
	merchant, err := server.store.GetMerchant(ctx, merchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
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

	// 检查商户当前状态
	if merchant.Status == "suspended" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("商户已处于暂停状态")))
		return
	}

	// 更新商户状态为暂停
	_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     merchantID,
		Status: "suspended",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 记录暂停信息到商户画像（用于审计追溯，忽略错误不影响主流程）
	suspendUntil := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
	_ = server.store.SuspendMerchant(ctx, db.SuspendMerchantParams{
		MerchantID:    merchantID,
		SuspendReason: pgtype.Text{String: req.Reason, Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: suspendUntil, Valid: true},
	})

	ctx.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("商户 %d 已暂停 %d 小时", merchantID, req.DurationHours),
		"reason":         req.Reason,
		"duration_hours": req.DurationHours,
	})
}

type resumeOperatorMerchantRequest struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

// resumeOperatorMerchant 运营商恢复商户
// @Summary 恢复商户
// @Description 运营商恢复其管辖区域内被暂停的商户
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body resumeOperatorMerchantRequest true "恢复原因"
// @Success 200 {object} MessageResponse "恢复成功"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "商户不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/merchants/{id}/resume [post]
func (server *Server) resumeOperatorMerchant(ctx *gin.Context) {
	merchantIDStr := ctx.Param("id")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的商户ID")))
		return
	}

	var req resumeOperatorMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户信息
	merchant, err := server.store.GetMerchant(ctx, merchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
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

	// 检查商户当前状态
	if merchant.Status != "suspended" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("商户未处于暂停状态")))
		return
	}

	// 更新商户状态为正常
	_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     merchantID,
		Status: "approved",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 清除商户画像中的暂停信息（忽略错误不影响主流程）
	_ = server.store.UnsuspendMerchant(ctx, merchantID)

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("商户 %d 已恢复上线", merchantID),
		"reason":  req.Reason,
	})
}

// ==================== 骑手列表查询 ====================

type listOperatorRidersRequest struct {
	Status string `form:"status" binding:"omitempty,oneof=pending active suspended deactivated"`
	Page   int32  `form:"page" binding:"omitempty,min=1"`
	Limit  int32  `form:"limit" binding:"omitempty,min=1,max=100"`
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
	Riders []riderListItem `json:"riders"`
	Total  int64           `json:"total"`
	Page   int32           `json:"page"`
	Limit  int32           `json:"limit"`
}

// listOperatorRiders 获取运营商管辖区域内的骑手列表
// @Summary 获取区域骑手列表
// @Description 运营商获取其管辖区域内的所有骑手，支持按状态筛选
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param status query string false "骑手状态" Enums(pending, active, suspended, deactivated)
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

	// 从中间件获取运营商信息
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	// 获取运营商管理的区域
	if operator.RegionID == 0 {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator has no assigned region")))
		return
	}

	offset := (req.Page - 1) * req.Limit
	regionID := pgtype.Int8{Int64: operator.RegionID, Valid: true}

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
		Riders: result,
		Total:  total,
		Page:   req.Page,
		Limit:  req.Limit,
	})
}

// ==================== 骑手详情查询 ====================

type getOperatorRiderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type riderDetailResponse struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"user_id"`
	RealName           string  `json:"real_name"`
	Phone              string  `json:"phone"`
	IDCardNo           string  `json:"id_card_no"`
	Status             string  `json:"status"`
	IsOnline           bool    `json:"is_online"`
	RegionID           int64   `json:"region_id"`
	DepositAmount      int64   `json:"deposit_amount"`
	FrozenDeposit      int64   `json:"frozen_deposit"`
	TotalOrders        int32   `json:"total_orders"`
	TotalEarnings      int64   `json:"total_earnings"`
	CurrentLatitude    float64 `json:"current_latitude"`
	CurrentLongitude   float64 `json:"current_longitude"`
	LocationUpdatedAt  string  `json:"location_updated_at,omitempty"`
	CreditScore        int16   `json:"credit_score"`
	HighValueQualified bool    `json:"high_value_qualified"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
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
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证骑手有区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("骑手未分配区域")))
		return
	}

	// 验证运营商是否管理该骑手的区域
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 获取高值单资格积分
	premiumScore, _ := server.store.GetRiderPremiumScore(ctx, rider.ID)
	highValueQualified := premiumScore >= 0

	updatedAt := ""
	if rider.UpdatedAt.Valid {
		updatedAt = rider.UpdatedAt.Time.Format("2006-01-02 15:04:05")
	}

	resp := riderDetailResponse{
		ID:                 rider.ID,
		UserID:             rider.UserID,
		RealName:           rider.RealName,
		Phone:              rider.Phone,
		IDCardNo:           maskIDCard(rider.IDCardNo),
		Status:             rider.Status,
		IsOnline:           rider.IsOnline,
		RegionID:           rider.RegionID.Int64,
		DepositAmount:      rider.DepositAmount,
		FrozenDeposit:      rider.FrozenDeposit,
		TotalOrders:        rider.TotalOrders,
		TotalEarnings:      rider.TotalEarnings,
		CurrentLatitude:    pgNumericToFloat64(rider.CurrentLatitude),
		CurrentLongitude:   pgNumericToFloat64(rider.CurrentLongitude),
		CreditScore:        rider.CreditScore,
		HighValueQualified: highValueQualified,
		CreatedAt:          rider.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:          updatedAt,
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

// ==================== 暂停/恢复骑手 ====================

type suspendOperatorRiderRequest struct {
	Reason        string `json:"reason" binding:"required,min=5,max=500"`
	DurationHours int    `json:"duration_hours" binding:"required,min=1,max=720"`
}

// suspendOperatorRider 运营商暂停骑手
// @Summary 暂停骑手
// @Description 运营商暂停其管辖区域内的骑手，最长可暂停30天（720小时）
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "骑手ID"
// @Param request body suspendOperatorRiderRequest true "暂停原因和时长"
// @Success 200 {object} MessageResponse "暂停成功"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "骑手不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders/{id}/suspend [post]
func (server *Server) suspendOperatorRider(ctx *gin.Context) {
	riderIDStr := ctx.Param("id")
	riderID, err := strconv.ParseInt(riderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的骑手ID")))
		return
	}

	var req suspendOperatorRiderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取骑手信息
	rider, err := server.store.GetRider(ctx, riderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证骑手有区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("骑手未分配区域")))
		return
	}

	// 验证运营商是否管理该骑手的区域
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 检查骑手当前状态
	if rider.Status == "suspended" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("骑手已处于暂停状态")))
		return
	}

	// 更新骑手状态为暂停
	_, err = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: "suspended",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 同时下线骑手
	if rider.IsOnline {
		_, _ = server.store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{
			ID:       riderID,
			IsOnline: false,
		})
	}

	// 记录暂停信息到骑手画像（用于审计追溯，忽略错误不影响主流程）
	suspendUntil := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
	_ = server.store.SuspendRider(ctx, db.SuspendRiderParams{
		RiderID:       riderID,
		SuspendReason: pgtype.Text{String: req.Reason, Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: suspendUntil, Valid: true},
	})

	ctx.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("骑手 %d 已暂停 %d 小时", riderID, req.DurationHours),
		"reason":         req.Reason,
		"duration_hours": req.DurationHours,
	})
}

type resumeOperatorRiderRequest struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

// resumeOperatorRider 运营商恢复骑手
// @Summary 恢复骑手
// @Description 运营商恢复其管辖区域内被暂停的骑手
// @Tags 运营商-商户骑手管理
// @Accept json
// @Produce json
// @Param id path int true "骑手ID"
// @Param request body resumeOperatorRiderRequest true "恢复原因"
// @Success 200 {object} MessageResponse "恢复成功"
// @Failure 400 {object} errorMessage "请求参数错误"
// @Failure 401 {object} errorMessage "未授权"
// @Failure 403 {object} errorMessage "无权限"
// @Failure 404 {object} errorMessage "骑手不存在"
// @Failure 500 {object} errorMessage "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/riders/{id}/resume [post]
func (server *Server) resumeOperatorRider(ctx *gin.Context) {
	riderIDStr := ctx.Param("id")
	riderID, err := strconv.ParseInt(riderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的骑手ID")))
		return
	}

	var req resumeOperatorRiderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取骑手信息
	rider, err := server.store.GetRider(ctx, riderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证骑手有区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("骑手未分配区域")))
		return
	}

	// 验证运营商是否管理该骑手的区域
	if _, err := server.checkOperatorManagesRegion(ctx, rider.RegionID.Int64); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 检查骑手当前状态
	if rider.Status != "suspended" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("骑手未处于暂停状态")))
		return
	}

	// 更新骑手状态为正常
	_, err = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     riderID,
		Status: "active",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 清除骑手画像中的暂停信息（忽略错误不影响主流程）
	_ = server.store.UnsuspendRider(ctx, riderID)

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("骑手 %d 已恢复上线资格", riderID),
		"reason":  req.Reason,
	})
}
