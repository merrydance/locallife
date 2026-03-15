package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

func validateProfitSharingRates(platformRate, operatorRate int32) error {
	if platformRate < 0 || platformRate > 100 {
		return errors.New("platform_rate must be between 0 and 100")
	}
	if operatorRate < 0 || operatorRate > 100 {
		return errors.New("operator_rate must be between 0 and 100")
	}
	if platformRate+operatorRate > 100 {
		return errors.New("platform_rate + operator_rate must be less than or equal to 100")
	}
	return nil
}

type profitSharingConfigResponse struct {
	ID           int64   `json:"id"`
	Status       string  `json:"status"`
	OrderSource  string  `json:"order_source"`
	RegionID     *int64  `json:"region_id,omitempty"`
	MerchantID   *int64  `json:"merchant_id,omitempty"`
	PlatformRate int32   `json:"platform_rate"`
	OperatorRate int32   `json:"operator_rate"`
	RiderEnabled bool    `json:"rider_enabled"`
	Priority     int32   `json:"priority"`
	EffectiveAt  *string `json:"effective_at,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
	CreatedBy    *int64  `json:"created_by,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func newProfitSharingConfigResponse(config db.ProfitSharingConfig) profitSharingConfigResponse {
	var regionID *int64
	if config.RegionID.Valid {
		regionID = &config.RegionID.Int64
	}
	var merchantID *int64
	if config.MerchantID.Valid {
		merchantID = &config.MerchantID.Int64
	}
	var createdBy *int64
	if config.CreatedBy.Valid {
		createdBy = &config.CreatedBy.Int64
	}

	var effectiveAt *string
	if config.EffectiveAt.Valid {
		formatted := config.EffectiveAt.Time.Format(time.RFC3339)
		effectiveAt = &formatted
	}
	var expiresAt *string
	if config.ExpiresAt.Valid {
		formatted := config.ExpiresAt.Time.Format(time.RFC3339)
		expiresAt = &formatted
	}

	return profitSharingConfigResponse{
		ID:           config.ID,
		Status:       config.Status,
		OrderSource:  config.OrderSource,
		RegionID:     regionID,
		MerchantID:   merchantID,
		PlatformRate: config.PlatformRate,
		OperatorRate: config.OperatorRate,
		RiderEnabled: config.RiderEnabled,
		Priority:     config.Priority,
		EffectiveAt:  effectiveAt,
		ExpiresAt:    expiresAt,
		CreatedBy:    createdBy,
		CreatedAt:    config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    config.UpdatedAt.Format(time.RFC3339),
	}
}

type createProfitSharingConfigRequest struct {
	Status       string     `json:"status"`
	OrderSource  string     `json:"order_source" binding:"required"`
	RegionID     *int64     `json:"region_id"`
	MerchantID   *int64     `json:"merchant_id"`
	PlatformRate int32      `json:"platform_rate" binding:"required"`
	OperatorRate int32      `json:"operator_rate" binding:"required"`
	RiderEnabled *bool      `json:"rider_enabled"`
	Priority     *int32     `json:"priority"`
	EffectiveAt  *time.Time `json:"effective_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// createProfitSharingConfig 创建分账规则配置
// @Summary 创建分账规则配置
// @Description 平台创建分账规则配置
// @Tags Platform
// @Accept json
// @Produce json
// @Param request body createProfitSharingConfigRequest true "分账规则配置"
// @Security BearerAuth
// @Success 201 {object} profitSharingConfigResponse "创建成功"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/configs [post]
func (server *Server) createProfitSharingConfig(ctx *gin.Context) {
	var req createProfitSharingConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := validateProfitSharingRates(req.PlatformRate, req.OperatorRate); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	params := db.CreateProfitSharingConfigParams{
		Status:       req.Status,
		OrderSource:  req.OrderSource,
		PlatformRate: req.PlatformRate,
		OperatorRate: req.OperatorRate,
		RiderEnabled: true,
		Priority:     100,
		CreatedBy:    pgtype.Int8{Int64: payload.UserID, Valid: true},
	}
	if req.RegionID != nil {
		params.RegionID = pgtype.Int8{Int64: *req.RegionID, Valid: true}
	}
	if req.MerchantID != nil {
		params.MerchantID = pgtype.Int8{Int64: *req.MerchantID, Valid: true}
	}
	if req.RiderEnabled != nil {
		params.RiderEnabled = *req.RiderEnabled
	}
	if req.Priority != nil {
		params.Priority = *req.Priority
	}
	if req.EffectiveAt != nil {
		params.EffectiveAt = pgtype.Timestamptz{Time: *req.EffectiveAt, Valid: true}
	}
	if req.ExpiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	result, err := server.store.CreateProfitSharingConfigTx(ctx, db.CreateProfitSharingConfigTxParams{
		ActorID:   payload.UserID,
		ActorRole: RoleAdmin,
		Params:    params,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newProfitSharingConfigResponse(result.Config))
}

type listProfitSharingConfigsRequest struct {
	Status      string `form:"status"`
	OrderSource string `form:"order_source"`
	RegionID    int64  `form:"region_id"`
	MerchantID  int64  `form:"merchant_id"`
	Page        int32  `form:"page" binding:"omitempty,min=1"`
	Limit       int32  `form:"limit" binding:"omitempty,min=1,max=200"`
}

type listProfitSharingConfigsResponse struct {
	Items []profitSharingConfigResponse `json:"items"`
	Page  int32                         `json:"page"`
	Limit int32                         `json:"limit"`
}

// listProfitSharingConfigs 获取分账规则配置列表
// @Summary 获取分账规则配置列表
// @Description 平台获取分账规则配置列表
// @Tags Platform
// @Accept json
// @Produce json
// @Param status query string false "状态"
// @Param order_source query string false "订单来源"
// @Param region_id query int false "区域ID"
// @Param merchant_id query int false "商户ID"
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(200)
// @Security BearerAuth
// @Success 200 {object} listProfitSharingConfigsResponse "配置列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/configs [get]
func (server *Server) listProfitSharingConfigs(ctx *gin.Context) {
	var req listProfitSharingConfigsRequest
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

	configs, err := server.store.ListProfitSharingConfigs(ctx, db.ListProfitSharingConfigsParams{
		Column1: req.Status,
		Column2: req.OrderSource,
		Column3: req.RegionID,
		Column4: req.MerchantID,
		Limit:   req.Limit,
		Offset:  pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]profitSharingConfigResponse, len(configs))
	for i, config := range configs {
		items[i] = newProfitSharingConfigResponse(config)
	}

	ctx.JSON(http.StatusOK, listProfitSharingConfigsResponse{
		Items: items,
		Page:  req.Page,
		Limit: req.Limit,
	})
}

type updateProfitSharingConfigRequest struct {
	Status       *string    `json:"status"`
	OrderSource  *string    `json:"order_source"`
	RegionID     *int64     `json:"region_id"`
	MerchantID   *int64     `json:"merchant_id"`
	PlatformRate *int32     `json:"platform_rate"`
	OperatorRate *int32     `json:"operator_rate"`
	RiderEnabled *bool      `json:"rider_enabled"`
	Priority     *int32     `json:"priority"`
	EffectiveAt  *time.Time `json:"effective_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// updateProfitSharingConfig 更新分账规则配置
// @Summary 更新分账规则配置
// @Description 平台更新分账规则配置
// @Tags Platform
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param request body updateProfitSharingConfigRequest true "更新内容"
// @Security BearerAuth
// @Success 200 {object} profitSharingConfigResponse "更新成功"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "配置不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/configs/{id} [patch]
func (server *Server) updateProfitSharingConfig(ctx *gin.Context) {
	configID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateProfitSharingConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.PlatformRate != nil && req.OperatorRate != nil {
		if err := validateProfitSharingRates(*req.PlatformRate, *req.OperatorRate); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}
	if req.PlatformRate != nil && (*req.PlatformRate < 0 || *req.PlatformRate > 100) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("platform_rate must be between 0 and 100")))
		return
	}
	if req.OperatorRate != nil && (*req.OperatorRate < 0 || *req.OperatorRate > 100) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("operator_rate must be between 0 and 100")))
		return
	}

	params := db.UpdateProfitSharingConfigParams{
		ID: configID,
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.OrderSource != nil {
		params.OrderSource = pgtype.Text{String: *req.OrderSource, Valid: true}
	}
	if req.RegionID != nil {
		params.RegionID = pgtype.Int8{Int64: *req.RegionID, Valid: true}
	}
	if req.MerchantID != nil {
		params.MerchantID = pgtype.Int8{Int64: *req.MerchantID, Valid: true}
	}
	if req.PlatformRate != nil {
		params.PlatformRate = pgtype.Int4{Int32: *req.PlatformRate, Valid: true}
	}
	if req.OperatorRate != nil {
		params.OperatorRate = pgtype.Int4{Int32: *req.OperatorRate, Valid: true}
	}
	if req.RiderEnabled != nil {
		params.RiderEnabled = pgtype.Bool{Bool: *req.RiderEnabled, Valid: true}
	}
	if req.Priority != nil {
		params.Priority = pgtype.Int4{Int32: *req.Priority, Valid: true}
	}
	if req.EffectiveAt != nil {
		params.EffectiveAt = pgtype.Timestamptz{Time: *req.EffectiveAt, Valid: true}
	}
	if req.ExpiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	result, err := server.store.UpdateProfitSharingConfigTx(ctx, db.UpdateProfitSharingConfigTxParams{
		ActorID:   ctx.MustGet(authorizationPayloadKey).(*token.Payload).UserID,
		ActorRole: RoleAdmin,
		Params:    params,
	})
	if err != nil {
		if isNotFoundError(err) || err == db.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newProfitSharingConfigResponse(result.Config))
}

type disableProfitSharingConfigRequest struct {
	Reason string `json:"reason"`
}

// disableProfitSharingConfig 禁用分账规则配置
// @Summary 禁用分账规则配置
// @Description 平台禁用分账规则配置
// @Tags Platform
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param request body disableProfitSharingConfigRequest false "禁用原因"
// @Security BearerAuth
// @Success 200 {object} profitSharingConfigResponse "禁用成功"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "配置不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/configs/{id}/disable [post]
func (server *Server) disableProfitSharingConfig(ctx *gin.Context) {
	configID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req disableProfitSharingConfigRequest
	_ = ctx.ShouldBindJSON(&req)

	result, err := server.store.UpdateProfitSharingConfigStatusTx(ctx, db.UpdateProfitSharingConfigStatusTxParams{
		ActorID:   ctx.MustGet(authorizationPayloadKey).(*token.Payload).UserID,
		ActorRole: RoleAdmin,
		Note:      req.Reason,
		Params: db.UpdateProfitSharingConfigStatusParams{
			ID:     configID,
			Status: "disabled",
		},
	})
	if err != nil {
		if isNotFoundError(err) || err == db.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newProfitSharingConfigResponse(result.Config))
}
