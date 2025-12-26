package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ============================================================================
// Helper: 权限验证
// ============================================================================

// checkOperatorManagesRegion 检查当前 operator 是否管理指定区域
// 返回 operator 记录和错误
func (server *Server) checkOperatorManagesRegion(ctx *gin.Context, regionID int64) (*db.Operator, error) {
	var operator db.Operator

	// 如果中间件已经加载了 operator，直接使用
	if op, ok := GetOperatorFromContext(ctx); ok {
		operator = op
	} else {
		// 向后兼容：从数据库查询
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
		var err error
		operator, err = server.store.GetOperatorByUser(ctx, authPayload.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("operator record not found")
			}
			return nil, err
		}
	}

	// 检查 operator 是否管理该区域
	manages, err := server.store.CheckOperatorManagesRegion(ctx, db.CheckOperatorManagesRegionParams{
		OperatorID: operator.ID,
		RegionID:   regionID,
	})
	if err != nil {
		return nil, err
	}

	if !manages {
		return nil, errors.New("you do not have permission to manage this region")
	}

	return &operator, nil
}

// getOperatorRegionID 获取运营商管理的区域ID
func (server *Server) getOperatorRegionID(ctx *gin.Context) (int64, error) {
	// 如果中间件已经设置了 operator，直接使用
	if op, ok := GetOperatorFromContext(ctx); ok {
		if op.RegionID == 0 {
			return 0, errors.New("operator has no assigned region")
		}
		return op.RegionID, nil
	}

	// 向后兼容：从数据库查询
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 查询operator角色记录
	operatorRole, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
		UserID: authPayload.UserID,
		Role:   "operator",
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New("operator role not found")
		}
		return 0, err
	}

	// 检查状态
	if operatorRole.Status != "active" {
		return 0, errors.New("operator role is not active")
	}

	// related_entity_id存储region_id
	if !operatorRole.RelatedEntityID.Valid {
		return 0, errors.New("operator has no assigned region")
	}

	return operatorRole.RelatedEntityID.Int64, nil
}

// ============================================================================
// 运费配置 API
// ============================================================================

type createDeliveryFeeConfigRequest struct {
	RegionID      int64   `json:"region_id" binding:"required,gt=0"`
	BaseFee       int64   `json:"base_fee" binding:"required,gte=0"`
	BaseDistance  int32   `json:"base_distance" binding:"required,gt=0"`
	ExtraFeePerKm int64   `json:"extra_fee_per_km" binding:"required,gte=0"`
	ValueRatio    float64 `json:"value_ratio" binding:"gte=0,lte=1"`
	MaxFee        *int64  `json:"max_fee"`
	MinFee        int64   `json:"min_fee" binding:"gte=0"`
}

type deliveryFeeConfigResponse struct {
	ID            int64      `json:"id"`
	RegionID      int64      `json:"region_id"`
	BaseFee       int64      `json:"base_fee"`
	BaseDistance  int32      `json:"base_distance"`
	ExtraFeePerKm int64      `json:"extra_fee_per_km"`
	ValueRatio    float64    `json:"value_ratio"`
	MaxFee        *int64     `json:"max_fee,omitempty"`
	MinFee        int64      `json:"min_fee"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

func newDeliveryFeeConfigResponse(config db.DeliveryFeeConfig) deliveryFeeConfigResponse {
	rsp := deliveryFeeConfigResponse{
		ID:            config.ID,
		RegionID:      config.RegionID,
		BaseFee:       config.BaseFee,
		BaseDistance:  config.BaseDistance,
		ExtraFeePerKm: config.ExtraFeePerKm,
		MinFee:        config.MinFee,
		IsActive:      config.IsActive,
		CreatedAt:     config.CreatedAt,
	}

	if config.ValueRatio.Valid {
		f, _ := config.ValueRatio.Float64Value()
		rsp.ValueRatio = f.Float64
	}

	if config.MaxFee.Valid {
		rsp.MaxFee = &config.MaxFee.Int64
	}

	if config.UpdatedAt.Valid {
		rsp.UpdatedAt = &config.UpdatedAt.Time
	}

	return rsp
}

// createDeliveryFeeConfig godoc
// @Summary Create delivery fee config (Operator)
// @Description Create delivery fee configuration for a region (operator only). Each region can only have one active configuration.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param region_id path int true "Region ID"
// @Param request body createDeliveryFeeConfigRequest true "Fee config details"
// @Success 201 {object} deliveryFeeConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Operator role required or not authorized for this region"
// @Failure 409 {object} ErrorResponse "Config already exists for this region"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/regions/{region_id}/config [post]
// @Security BearerAuth
func (server *Server) createDeliveryFeeConfig(ctx *gin.Context) {
	var req createDeliveryFeeConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 权限验证由中间件处理（RoleMiddleware + OperatorMiddleware + OperatorRegionMiddleware）

	arg := db.CreateDeliveryFeeConfigParams{
		RegionID:      req.RegionID,
		BaseFee:       req.BaseFee,
		BaseDistance:  req.BaseDistance,
		ExtraFeePerKm: req.ExtraFeePerKm,
		MinFee:        req.MinFee,
		IsActive:      true,
	}

	// ValueRatio: 默认1%
	if req.ValueRatio > 0 {
		arg.ValueRatio.Scan(req.ValueRatio)
	} else {
		arg.ValueRatio.Scan(0.01)
	}

	if req.MaxFee != nil {
		arg.MaxFee = pgtype.Int8{Int64: *req.MaxFee, Valid: true}
	}

	config, err := server.store.CreateDeliveryFeeConfig(ctx, arg)
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("delivery fee config already exists for this region")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newDeliveryFeeConfigResponse(config))
}

type getDeliveryFeeConfigURI struct {
	RegionID int64 `uri:"region_id" binding:"required,gt=0"`
}

// getDeliveryFeeConfig godoc
// @Summary Get delivery fee config
// @Description Get delivery fee configuration for a region
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param region_id path int true "Region ID"
// @Success 200 {object} deliveryFeeConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Config not found"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/config/{region_id} [get]
// @Security BearerAuth
func (server *Server) getDeliveryFeeConfig(ctx *gin.Context) {
	var uri getDeliveryFeeConfigURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	config, err := server.store.GetDeliveryFeeConfigByRegion(ctx, uri.RegionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("delivery fee config not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newDeliveryFeeConfigResponse(config))
}

type updateDeliveryFeeConfigURI struct {
	RegionID int64 `uri:"region_id" binding:"required,gt=0"`
}

type updateDeliveryFeeConfigRequest struct {
	BaseFee       *int64   `json:"base_fee"`
	BaseDistance  *int32   `json:"base_distance"`
	ExtraFeePerKm *int64   `json:"extra_fee_per_km"`
	ValueRatio    *float64 `json:"value_ratio"`
	MaxFee        *int64   `json:"max_fee"`
	MinFee        *int64   `json:"min_fee"`
	IsActive      *bool    `json:"is_active"`
}

// updateDeliveryFeeConfig godoc
// @Summary Update delivery fee config (Operator)
// @Description Update delivery fee configuration for a region (operator only). Supports partial updates.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param region_id path int true "Region ID"
// @Param request body updateDeliveryFeeConfigRequest true "Update fields"
// @Success 200 {object} deliveryFeeConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Operator role required or not authorized for this region"
// @Failure 404 {object} ErrorResponse "Config not found"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/regions/{region_id}/config [patch]
// @Security BearerAuth
func (server *Server) updateDeliveryFeeConfig(ctx *gin.Context) {
	var uri updateDeliveryFeeConfigURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateDeliveryFeeConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 权限验证由中间件处理（RoleMiddleware + OperatorMiddleware + OperatorRegionMiddleware）

	// 获取现有配置
	existingConfig, err := server.store.GetDeliveryFeeConfigByRegion(ctx, uri.RegionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("delivery fee config not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	arg := db.UpdateDeliveryFeeConfigParams{
		ID: existingConfig.ID,
	}

	if req.BaseFee != nil {
		arg.BaseFee = pgtype.Int8{Int64: *req.BaseFee, Valid: true}
	}
	if req.BaseDistance != nil {
		arg.BaseDistance = pgtype.Int4{Int32: *req.BaseDistance, Valid: true}
	}
	if req.ExtraFeePerKm != nil {
		arg.ExtraFeePerKm = pgtype.Int8{Int64: *req.ExtraFeePerKm, Valid: true}
	}
	if req.ValueRatio != nil {
		arg.ValueRatio = pgtype.Numeric{}
		arg.ValueRatio.Scan(*req.ValueRatio)
	}
	if req.MaxFee != nil {
		arg.MaxFee = pgtype.Int8{Int64: *req.MaxFee, Valid: true}
	}
	if req.MinFee != nil {
		arg.MinFee = pgtype.Int8{Int64: *req.MinFee, Valid: true}
	}
	if req.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	config, err := server.store.UpdateDeliveryFeeConfig(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newDeliveryFeeConfigResponse(config))
}

// ============================================================================
// 高峰时段配置 API
// ============================================================================

type createPeakHourConfigRequest struct {
	RegionID    int64   `json:"region_id" binding:"required,gt=0"`
	StartTime   string  `json:"start_time" binding:"required"`
	EndTime     string  `json:"end_time" binding:"required"`
	Coefficient float64 `json:"coefficient" binding:"required,gt=1"`
	DaysOfWeek  []int16 `json:"days_of_week" binding:"required,min=1,max=7,dive,min=0,max=6"`
}

type peakHourConfigResponse struct {
	ID          int64      `json:"id"`
	RegionID    int64      `json:"region_id"`
	StartTime   string     `json:"start_time"`
	EndTime     string     `json:"end_time"`
	Coefficient float64    `json:"coefficient"`
	DaysOfWeek  []int16    `json:"days_of_week"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

func newPeakHourConfigResponse(config db.PeakHourConfig) peakHourConfigResponse {
	rsp := peakHourConfigResponse{
		ID:         config.ID,
		RegionID:   config.RegionID,
		DaysOfWeek: config.DaysOfWeek,
		IsActive:   config.IsActive,
		CreatedAt:  config.CreatedAt,
	}

	if config.Coefficient.Valid {
		f, _ := config.Coefficient.Float64Value()
		rsp.Coefficient = f.Float64
	}

	if config.UpdatedAt.Valid {
		rsp.UpdatedAt = &config.UpdatedAt.Time
	}

	// 格式化时间
	rsp.StartTime = formatPgTime(config.StartTime)
	rsp.EndTime = formatPgTime(config.EndTime)

	return rsp
}

// formatPgTime 将 pgtype.Time 转换为 "HH:MM" 格式
func formatPgTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	// Microseconds 表示从午夜开始的微秒数
	totalSeconds := t.Microseconds / 1e6
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
}

// parsePgTime 将 "HH:MM" 格式的字符串转换为 pgtype.Time
func parsePgTime(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	microseconds := int64(t.Hour()*3600+t.Minute()*60) * 1e6
	return pgtype.Time{Microseconds: microseconds, Valid: true}, nil
}

// createPeakHourConfig godoc
// @Summary Create peak hour config (Operator)
// @Description Create peak hour delivery fee configuration for a region (operator only). Time format is HH:MM.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param region_id path int true "Region ID"
// @Param request body createPeakHourConfigRequest true "Peak hour config details"
// @Success 201 {object} peakHourConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Operator role required or not authorized for this region"
// @Failure 500 {object} ErrorResponse
// @Router /operator/regions/{region_id}/peak-hours [post]
// @Security BearerAuth
func (server *Server) createPeakHourConfig(ctx *gin.Context) {
	var req createPeakHourConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证 operator 角色和区域权限
	if _, err := server.checkOperatorManagesRegion(ctx, req.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	startTime, err := parsePgTime(req.StartTime)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_time format, expected HH:MM")))
		return
	}

	endTime, err := parsePgTime(req.EndTime)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_time format, expected HH:MM")))
		return
	}

	arg := db.CreatePeakHourConfigParams{
		RegionID:   req.RegionID,
		StartTime:  startTime,
		EndTime:    endTime,
		DaysOfWeek: req.DaysOfWeek,
		IsActive:   true,
	}
	arg.Coefficient.Scan(req.Coefficient)

	config, err := server.store.CreatePeakHourConfig(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newPeakHourConfigResponse(config))
}

type listPeakHourConfigsURI struct {
	RegionID int64 `uri:"region_id" binding:"required,gt=0"`
}

// listPeakHourConfigs godoc
// @Summary List peak hour configs (Operator)
// @Description Get all peak hour configurations for a region. Only operator can access.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param region_id path int true "Region ID"
// @Success 200 {array} peakHourConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Operator role required"
// @Failure 500 {object} ErrorResponse
// @Router /operator/regions/{region_id}/peak-hours [get]
// @Security BearerAuth
func (server *Server) listPeakHourConfigs(ctx *gin.Context) {
	var uri listPeakHourConfigsURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	configs, err := server.store.ListPeakHourConfigsByRegion(ctx, uri.RegionID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]peakHourConfigResponse, len(configs))
	for i, config := range configs {
		rsp[i] = newPeakHourConfigResponse(config)
	}

	ctx.JSON(http.StatusOK, rsp)
}

type deletePeakHourConfigURI struct {
	ID int64 `uri:"id" binding:"required,gt=0"`
}

// deletePeakHourConfig godoc
// @Summary Delete peak hour config (Operator)
// @Description Delete a peak hour configuration (operator only). Verifies operator has permission for the region.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param id path int true "Peak hour config ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Operator role required or not authorized for this region"
// @Failure 404 {object} ErrorResponse "Config not found"
// @Failure 500 {object} ErrorResponse
// @Router /operator/peak-hours/{id} [delete]
// @Security BearerAuth
func (server *Server) deletePeakHourConfig(ctx *gin.Context) {
	var uri deletePeakHourConfigURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 先获取配置以验证区域权限
	config, err := server.store.GetPeakHourConfig(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("peak hour config not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证 operator 角色和区域权限
	if _, err := server.checkOperatorManagesRegion(ctx, config.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	err = server.store.DeletePeakHourConfig(ctx, uri.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// ============================================================================
// 商家配送优惠 API
// ============================================================================

type createDeliveryPromotionRequest struct {
	Name           string `json:"name" binding:"required,min=1,max=50"`
	MinOrderAmount int64  `json:"min_order_amount" binding:"required,gte=0,lte=100000000"`
	DiscountAmount int64  `json:"discount_amount" binding:"required,gt=0,lte=10000000"`
	ValidFrom      string `json:"valid_from" binding:"required"`
	ValidUntil     string `json:"valid_until" binding:"required"`
}

type deliveryPromotionResponse struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchant_id"`
	Name           string     `json:"name"`
	MinOrderAmount int64      `json:"min_order_amount"`
	DiscountAmount int64      `json:"discount_amount"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidUntil     time.Time  `json:"valid_until"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

func newDeliveryPromotionResponse(promo db.MerchantDeliveryPromotion) deliveryPromotionResponse {
	rsp := deliveryPromotionResponse{
		ID:             promo.ID,
		MerchantID:     promo.MerchantID,
		Name:           promo.Name,
		MinOrderAmount: promo.MinOrderAmount,
		DiscountAmount: promo.DiscountAmount,
		ValidFrom:      promo.ValidFrom,
		ValidUntil:     promo.ValidUntil,
		IsActive:       promo.IsActive,
		CreatedAt:      promo.CreatedAt,
	}

	if promo.UpdatedAt.Valid {
		rsp.UpdatedAt = &promo.UpdatedAt.Time
	}

	return rsp
}

// createDeliveryPromotion godoc
// @Summary Create delivery promotion (Merchant)
// @Description Create delivery fee promotion for merchant. Only the merchant owner can create promotions for their own merchant.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param id path int true "Merchant ID"
// @Param request body createDeliveryPromotionRequest true "Promotion details"
// @Success 201 {object} deliveryPromotionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Merchant role required or not authorized for this merchant"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/merchants/{id}/promotions [post]
// @Security BearerAuth
func (server *Server) createDeliveryPromotion(ctx *gin.Context) {
	// 获取 URI 中的 merchant_id
	var uri listDeliveryPromotionsURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限：当前用户必须是该商户的所有者
	merchant, exists := GetMerchantFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant information not found")))
		return
	}

	if merchant.ID != uri.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only manage your own merchant's promotions")))
		return
	}

	var req createDeliveryPromotionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	validFrom, err := time.Parse(time.RFC3339, req.ValidFrom)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid valid_from format, expected RFC3339")))
		return
	}

	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid valid_until format, expected RFC3339")))
		return
	}

	if validUntil.Before(validFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	// 业务规则：折扣金额不能超过最低订单金额
	if req.DiscountAmount > req.MinOrderAmount && req.MinOrderAmount > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("discount_amount cannot exceed min_order_amount")))
		return
	}

	arg := db.CreateDeliveryPromotionParams{
		MerchantID:     uri.MerchantID,
		Name:           req.Name,
		MinOrderAmount: req.MinOrderAmount,
		DiscountAmount: req.DiscountAmount,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		IsActive:       true,
	}

	promo, err := server.store.CreateDeliveryPromotion(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newDeliveryPromotionResponse(promo))
}

type listDeliveryPromotionsURI struct {
	MerchantID int64 `uri:"merchant_id" binding:"required,gt=0"`
}

// listDeliveryPromotions godoc
// @Summary List delivery promotions (Merchant)
// @Description Get all delivery promotions for a merchant. Only the merchant owner can view their own promotions.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param merchant_id path int true "Merchant ID"
// @Success 200 {array} deliveryPromotionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Merchant role required or not authorized for this merchant"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/merchants/{merchant_id}/promotions [get]
// @Security BearerAuth
func (server *Server) listDeliveryPromotions(ctx *gin.Context) {
	var uri listDeliveryPromotionsURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限：当前用户必须是该商户的所有者
	merchant, exists := GetMerchantFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant information not found")))
		return
	}

	if merchant.ID != uri.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only view your own merchant's promotions")))
		return
	}

	promos, err := server.store.ListDeliveryPromotionsByMerchant(ctx, uri.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]deliveryPromotionResponse, len(promos))
	for i, promo := range promos {
		rsp[i] = newDeliveryPromotionResponse(promo)
	}

	ctx.JSON(http.StatusOK, rsp)
}

type deleteDeliveryPromotionURI struct {
	MerchantID int64 `uri:"merchant_id" binding:"required,gt=0"`
	ID         int64 `uri:"id" binding:"required,gt=0"`
}

// deleteDeliveryPromotion godoc
// @Summary Delete delivery promotion (Merchant)
// @Description Delete a delivery fee promotion. Only the merchant owner can delete their own promotions.
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param merchant_id path int true "Merchant ID"
// @Param id path int true "Promotion ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Merchant role required or not authorized for this merchant"
// @Failure 404 {object} ErrorResponse "Promotion not found"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/merchants/{merchant_id}/promotions/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteDeliveryPromotion(ctx *gin.Context) {
	var uri deleteDeliveryPromotionURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限：当前用户必须是该商户的所有者
	merchant, exists := GetMerchantFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant information not found")))
		return
	}

	if merchant.ID != uri.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only delete your own merchant's promotions")))
		return
	}

	// 先获取促销活动，验证归属
	promo, err := server.store.GetDeliveryPromotion(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("delivery promotion not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证促销属于该商户
	if promo.MerchantID != uri.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("promotion does not belong to this merchant")))
		return
	}

	err = server.store.DeleteDeliveryPromotion(ctx, uri.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// ============================================================================
// 运费计算 API (核心业务逻辑)
// ============================================================================

// 默认运费配置常量（当区域没有配置时使用）
const (
	DefaultBaseFee             int64   = 500 // 默认基础运费 5 元
	DefaultWeatherCoefficient  float64 = 1.0
	DefaultPeakHourCoefficient float64 = 1.0
)

type calculateDeliveryFeeRequest struct {
	RegionID    int64 `json:"region_id" binding:"required,gt=0"`
	MerchantID  int64 `json:"merchant_id" binding:"required,gt=0"`
	Distance    int32 `json:"distance" binding:"required,gt=0"` // 米
	OrderAmount int64 `json:"order_amount" binding:"required,gt=0"`
}

type calculateDeliveryFeeResponse struct {
	BaseFee             int64   `json:"base_fee"`
	DistanceFee         int64   `json:"distance_fee"`
	ValueFee            int64   `json:"value_fee"`
	WeatherCoefficient  float64 `json:"weather_coefficient"`
	PeakHourCoefficient float64 `json:"peak_hour_coefficient"`
	SubtotalFee         int64   `json:"subtotal_fee"`
	PromotionDiscount   int64   `json:"promotion_discount"`
	FinalFee            int64   `json:"final_fee"`
	DeliverySuspended   bool    `json:"delivery_suspended"`
	SuspendReason       string  `json:"suspend_reason,omitempty"`
}

// calculateDeliveryFee godoc
// @Summary Calculate delivery fee
// @Description Calculate delivery fee based on region, distance, and promotions
// @Tags delivery-fee
// @Accept json
// @Produce json
// @Param request body calculateDeliveryFeeRequest true "Calculation parameters"
// @Success 200 {object} DeliveryFeeResult
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Config not found"
// @Failure 500 {object} ErrorResponse
// @Router /delivery-fee/calculate [post]
// @Security BearerAuth
func (server *Server) calculateDeliveryFee(ctx *gin.Context) {
	var req calculateDeliveryFeeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// API 层严格检查：配置必须存在且激活
	config, err := server.store.GetDeliveryFeeConfigByRegion(ctx, req.RegionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrDeliveryFeeConfigNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !config.IsActive {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrDeliveryServiceDisabled))
		return
	}

	// 使用预取配置计算运费（避免重复查询数据库）
	result, err := server.calculateDeliveryFeeWithConfig(ctx, &config, req.MerchantID, req.Distance, req.OrderAmount)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, calculateDeliveryFeeResponse{
		BaseFee:             result.BaseFee,
		DistanceFee:         result.DistanceFee,
		ValueFee:            result.ValueFee,
		WeatherCoefficient:  result.WeatherCoefficient,
		PeakHourCoefficient: result.PeakHourCoefficient,
		SubtotalFee:         result.SubtotalFee,
		PromotionDiscount:   result.PromotionDiscount,
		FinalFee:            result.FinalFee,
		DeliverySuspended:   result.DeliverySuspended,
		SuspendReason:       result.SuspendReason,
	})
}

// DeliveryFeeResult 运费计算结果
type DeliveryFeeResult struct {
	BaseFee             int64
	DistanceFee         int64
	ValueFee            int64
	WeatherCoefficient  float64
	PeakHourCoefficient float64
	SubtotalFee         int64
	PromotionDiscount   int64
	FinalFee            int64
	DeliverySuspended   bool
	SuspendReason       string
}

// 运费计算相关错误
var (
	ErrDeliveryFeeConfigNotFound = errors.New("delivery fee config not found for this region")
	ErrDeliveryServiceDisabled   = errors.New("delivery service is disabled for this region")
)

// calculateDeliveryFeeInternal 内部运费计算方法，供其他模块调用
// 此方法会自动获取配置，如果配置不存在则使用默认值
func (server *Server) calculateDeliveryFeeInternal(ctx *gin.Context, regionID, merchantID int64, distance int32, orderAmount int64) (*DeliveryFeeResult, error) {
	// 获取基础运费配置
	config, err := server.store.GetDeliveryFeeConfigByRegion(ctx, regionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有配置，返回默认运费（内部调用降级处理）
			return &DeliveryFeeResult{
				BaseFee:             DefaultBaseFee,
				FinalFee:            DefaultBaseFee,
				WeatherCoefficient:  DefaultWeatherCoefficient,
				PeakHourCoefficient: DefaultPeakHourCoefficient,
			}, nil
		}
		return nil, err
	}

	if !config.IsActive {
		return &DeliveryFeeResult{
			DeliverySuspended: true,
			SuspendReason:     ErrDeliveryServiceDisabled.Error(),
		}, nil
	}

	return server.calculateDeliveryFeeWithConfig(ctx, &config, merchantID, distance, orderAmount)
}

// calculateDeliveryFeeWithConfig 使用预取配置计算运费的核心方法
func (server *Server) calculateDeliveryFeeWithConfig(ctx *gin.Context, config *db.DeliveryFeeConfig, merchantID int64, distance int32, orderAmount int64) (*DeliveryFeeResult, error) {
	regionID := config.RegionID

	// 1. 计算基础运费
	baseFee := config.BaseFee

	// 2. 计算距离附加费
	var distanceFee int64 = 0
	if distance > config.BaseDistance {
		extraMeters := distance - config.BaseDistance
		extraKm := float64(extraMeters) / 1000.0
		distanceFee = int64(extraKm * float64(config.ExtraFeePerKm))
	}

	// 3. 计算货值附加费
	var valueFee int64 = 0
	if config.ValueRatio.Valid {
		ratio, _ := config.ValueRatio.Float64Value()
		valueFee = int64(float64(orderAmount) * ratio.Float64)
	}

	// 4. 获取天气系数（优先 Redis 缓存，回退数据库）
	weatherCoeff := DefaultWeatherCoefficient
	weatherFromCache := false
	deliverySuspended := false
	suspendReason := ""

	// 先尝试从 Redis 缓存获取
	if server.weatherCache != nil {
		cachedCoef, err := server.weatherCache.Get(ctx, regionID)
		if err == nil && cachedCoef != nil {
			weatherCoeff = cachedCoef.Coefficient * cachedCoef.WarningCoefficient
			weatherFromCache = true
			if cachedCoef.SuspendDelivery {
				deliverySuspended = true
				suspendReason = "extreme weather warning"
			}
		}
	}

	// 缓存未命中时回退到数据库（使用标志位而非系数值判断）
	if !weatherFromCache {
		weatherData, err := server.store.GetLatestWeatherCoefficient(ctx, regionID)
		if err == nil {
			if weatherData.DeliverySuspended {
				deliverySuspended = true
				suspendReason = "extreme weather warning"
			}
			if weatherData.FinalCoefficient.Valid {
				f, _ := weatherData.FinalCoefficient.Float64Value()
				weatherCoeff = f.Float64
			}
		}
		// 数据库查询失败时使用默认系数1.0，不返回错误
	}

	// 5. 获取高峰时段系数
	peakCoeff := DefaultPeakHourCoefficient
	now := time.Now()
	peakConfigs, err := server.store.ListPeakHourConfigsByRegion(ctx, regionID)
	if err == nil {
		currentDayOfWeek := int16(now.Weekday())
		currentTime := now.Format("15:04")

		for _, pc := range peakConfigs {
			if !pc.IsActive {
				continue
			}
			dayMatch := false
			for _, d := range pc.DaysOfWeek {
				if d == currentDayOfWeek {
					dayMatch = true
					break
				}
			}
			if !dayMatch {
				continue
			}

			startStr := formatPgTime(pc.StartTime)
			endStr := formatPgTime(pc.EndTime)

			// 处理跨日情况 (如 22:00 - 06:00)
			if endStr < startStr {
				if currentTime >= startStr || currentTime < endStr {
					if pc.Coefficient.Valid {
						f, _ := pc.Coefficient.Float64Value()
						if f.Float64 > peakCoeff {
							peakCoeff = f.Float64
						}
					}
				}
			} else {
				if currentTime >= startStr && currentTime < endStr {
					if pc.Coefficient.Valid {
						f, _ := pc.Coefficient.Float64Value()
						if f.Float64 > peakCoeff {
							peakCoeff = f.Float64
						}
					}
				}
			}
		}
	}

	// 6. 应用系数计算小计
	subtotal := int64(float64(baseFee+distanceFee+valueFee) * weatherCoeff * peakCoeff)

	// 7. 应用封顶和保底
	if config.MaxFee.Valid && subtotal > config.MaxFee.Int64 {
		subtotal = config.MaxFee.Int64
	}
	if subtotal < config.MinFee {
		subtotal = config.MinFee
	}

	// 8. 获取商家优惠（阶梯式，取最优档）
	var promotionDiscount int64 = 0
	promos, err := server.store.ListActiveDeliveryPromotionsByMerchant(ctx, merchantID)
	if err == nil && len(promos) > 0 {
		for _, promo := range promos {
			if orderAmount >= promo.MinOrderAmount {
				if promo.DiscountAmount > promotionDiscount {
					promotionDiscount = promo.DiscountAmount
				}
			}
		}
	}

	// 9. 计算最终运费（不能为负）
	finalFee := subtotal - promotionDiscount
	if finalFee < 0 {
		finalFee = 0
	}

	return &DeliveryFeeResult{
		BaseFee:             baseFee,
		DistanceFee:         distanceFee,
		ValueFee:            valueFee,
		WeatherCoefficient:  weatherCoeff,
		PeakHourCoefficient: peakCoeff,
		SubtotalFee:         subtotal,
		PromotionDiscount:   promotionDiscount,
		FinalFee:            finalFee,
		DeliverySuspended:   deliverySuspended,
		SuspendReason:       suspendReason,
	}, nil
}
