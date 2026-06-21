package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const merchantPackagingOptionNameUniqueConstraint = "uq_merchant_packaging_options_name_active"

var errMerchantPackagingDefaultOptionUnavailable = errors.New("默认包装方式不可用")

type merchantPackagingSettingsResponse struct {
	MerchantID           int64    `json:"merchant_id"`
	Enabled              bool     `json:"enabled"`
	Required             bool     `json:"required"`
	ApplicableOrderTypes []string `json:"applicable_order_types"`
	DefaultOptionID      *int64   `json:"default_option_id,omitempty"`
}

type upsertMerchantPackagingSettingsRequest struct {
	Enabled              *bool    `json:"enabled" binding:"required"`
	Required             *bool    `json:"required" binding:"required"`
	ApplicableOrderTypes []string `json:"applicable_order_types" binding:"omitempty,dive,oneof=takeout takeaway"`
	DefaultOptionID      *int64   `json:"default_option_id" binding:"omitempty,min=1"`
}

type merchantPackagingOptionResponse struct {
	ID          int64  `json:"id"`
	MerchantID  int64  `json:"merchant_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	IsEnabled   bool   `json:"is_enabled"`
	SortOrder   int16  `json:"sort_order"`
}

type merchantPackagingOptionListResponse struct {
	Options    []merchantPackagingOptionResponse `json:"options"`
	Total      int                               `json:"total"`
	Page       int                               `json:"page"`
	Limit      int                               `json:"limit"`
	TotalPages int                               `json:"total_pages"`
}

type upsertMerchantPackagingOptionRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"omitempty"`
	Price       *int64 `json:"price" binding:"required,min=0,max=9999900"`
	IsEnabled   *bool  `json:"is_enabled" binding:"required"`
	SortOrder   *int16 `json:"sort_order" binding:"required,min=0,max=999"`
}

// getMerchantPackagingSettings godoc
// @Summary 获取商户包装设置
// @Description 获取当前商户包装启用状态、适用订单类型和默认包装选项
// @Tags 商户包装管理
// @Produce json
// @Success 200 {object} merchantPackagingSettingsResponse "包装设置"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-settings [get]
// @Security BearerAuth
func (server *Server) getMerchantPackagingSettings(ctx *gin.Context) {
	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	settings, err := server.store.GetMerchantPackagingSettings(ctx, merchant.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusOK, merchantPackagingSettingsResponse{
				MerchantID:           merchant.ID,
				Enabled:              false,
				Required:             true,
				ApplicableOrderTypes: defaultMerchantPackagingOrderTypes(),
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantPackagingSettingsResponse(settings))
}

// upsertMerchantPackagingSettings godoc
// @Summary 更新商户包装设置
// @Description 幂等更新当前商户包装启用状态、必选规则、适用订单类型和默认包装选项
// @Tags 商户包装管理
// @Accept json
// @Produce json
// @Param request body upsertMerchantPackagingSettingsRequest true "包装设置"
// @Success 200 {object} merchantPackagingSettingsResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-settings [put]
// @Security BearerAuth
func (server *Server) upsertMerchantPackagingSettings(ctx *gin.Context) {
	var req upsertMerchantPackagingSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	applicableOrderTypes := req.ApplicableOrderTypes
	if len(applicableOrderTypes) == 0 {
		applicableOrderTypes = defaultMerchantPackagingOrderTypes()
	}

	defaultOptionID := pgtype.Int8{}
	if req.DefaultOptionID != nil {
		defaultOptionID = pgtype.Int8{Int64: *req.DefaultOptionID, Valid: true}
	}

	settings, err := server.store.UpsertMerchantPackagingSettingsTx(ctx, db.UpsertMerchantPackagingSettingsParams{
		MerchantID:           merchant.ID,
		Enabled:              *req.Enabled,
		Required:             *req.Required,
		ApplicableOrderTypes: applicableOrderTypes,
		DefaultOptionID:      defaultOptionID,
	})
	if err != nil {
		if errors.Is(err, db.ErrMerchantPackagingDefaultOptionUnavailable) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errMerchantPackagingDefaultOptionUnavailable))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantPackagingSettingsResponse(settings))
}

// listMerchantPackagingOptions godoc
// @Summary 获取商户包装选项
// @Description 获取当前商户未删除的包装选项列表
// @Tags 商户包装管理
// @Produce json
// @Success 200 {object} merchantPackagingOptionListResponse "包装选项列表"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-options [get]
// @Security BearerAuth
func (server *Server) listMerchantPackagingOptions(ctx *gin.Context) {
	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	options, err := server.store.ListMerchantPackagingOptions(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total := len(options)
	totalPages := 0
	if total > 0 {
		totalPages = 1
	}
	response := merchantPackagingOptionListResponse{
		Options:    make([]merchantPackagingOptionResponse, 0, len(options)),
		Total:      total,
		Page:       1,
		Limit:      total,
		TotalPages: totalPages,
	}
	for _, option := range options {
		response.Options = append(response.Options, newMerchantPackagingOptionResponse(option))
	}
	ctx.JSON(http.StatusOK, response)
}

// createMerchantPackagingOption godoc
// @Summary 创建商户包装选项
// @Description 为当前商户创建一个包装选项，活动选项名称在商户内唯一
// @Tags 商户包装管理
// @Accept json
// @Produce json
// @Param request body upsertMerchantPackagingOptionRequest true "包装选项"
// @Success 201 {object} merchantPackagingOptionResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 409 {object} ErrorResponse "包装选项名称重复"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-options [post]
// @Security BearerAuth
func (server *Server) createMerchantPackagingOption(ctx *gin.Context) {
	req, ok := bindMerchantPackagingOptionRequest(ctx)
	if !ok {
		return
	}
	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	option, err := server.store.CreateMerchantPackagingOption(ctx, db.CreateMerchantPackagingOptionParams{
		MerchantID:  merchant.ID,
		Name:        req.name,
		Description: req.description,
		Price:       req.price,
		IsEnabled:   req.isEnabled,
		SortOrder:   req.sortOrder,
	})
	if err != nil {
		if isMerchantPackagingNameConflict(err) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("包装方式名称已存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newMerchantPackagingOptionResponse(option))
}

// updateMerchantPackagingOption godoc
// @Summary 更新商户包装选项
// @Description 幂等更新当前商户拥有的包装选项
// @Tags 商户包装管理
// @Accept json
// @Produce json
// @Param id path int true "包装选项ID"
// @Param request body upsertMerchantPackagingOptionRequest true "包装选项"
// @Success 200 {object} merchantPackagingOptionResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "包装选项不存在"
// @Failure 409 {object} ErrorResponse "包装选项名称重复"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-options/{id} [put]
// @Security BearerAuth
func (server *Server) updateMerchantPackagingOption(ctx *gin.Context) {
	optionID, ok := parseMerchantPackagingOptionID(ctx)
	if !ok {
		return
	}
	req, ok := bindMerchantPackagingOptionRequest(ctx)
	if !ok {
		return
	}
	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	option, err := server.store.UpdateMerchantPackagingOptionTx(ctx, db.UpdateMerchantPackagingOptionParams{
		ID:          optionID,
		MerchantID:  merchant.ID,
		Name:        req.name,
		Description: req.description,
		Price:       req.price,
		IsEnabled:   req.isEnabled,
		SortOrder:   req.sortOrder,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("包装方式不存在")))
			return
		}
		if isMerchantPackagingNameConflict(err) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("包装方式名称已存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantPackagingOptionResponse(option))
}

// deleteMerchantPackagingOption godoc
// @Summary 删除商户包装选项
// @Description 软删除当前商户拥有的包装选项；重复删除已删除的自有选项保持幂等成功
// @Tags 商户包装管理
// @Produce json
// @Param id path int true "包装选项ID"
// @Success 200 {object} merchantPackagingOptionResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "包装选项不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/packaging-options/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteMerchantPackagingOption(ctx *gin.Context) {
	optionID, ok := parseMerchantPackagingOptionID(ctx)
	if !ok {
		return
	}
	merchant, ok := merchantFromRequestContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	option, err := server.store.SoftDeleteMerchantPackagingOptionTx(ctx, db.SoftDeleteMerchantPackagingOptionParams{
		ID:         optionID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("包装方式不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantPackagingOptionResponse(option))
}

func merchantFromRequestContext(ctx *gin.Context) (db.Merchant, bool) {
	return GetMerchantFromContext(ctx)
}

func newMerchantPackagingSettingsResponse(settings db.MerchantPackagingSetting) merchantPackagingSettingsResponse {
	return merchantPackagingSettingsResponse{
		MerchantID:           settings.MerchantID,
		Enabled:              settings.Enabled,
		Required:             settings.Required,
		ApplicableOrderTypes: settings.ApplicableOrderTypes,
		DefaultOptionID:      nullableInt64(settings.DefaultOptionID),
	}
}

func newMerchantPackagingOptionResponse(option db.MerchantPackagingOption) merchantPackagingOptionResponse {
	description := ""
	if option.Description.Valid {
		description = option.Description.String
	}
	return merchantPackagingOptionResponse{
		ID:          option.ID,
		MerchantID:  option.MerchantID,
		Name:        option.Name,
		Description: description,
		Price:       option.Price,
		IsEnabled:   option.IsEnabled,
		SortOrder:   option.SortOrder,
	}
}

func defaultMerchantPackagingOrderTypes() []string {
	return []string{db.OrderTypeTakeout, db.OrderTypeTakeaway}
}

type normalizedMerchantPackagingOptionRequest struct {
	name        string
	description pgtype.Text
	price       int64
	isEnabled   bool
	sortOrder   int16
}

func bindMerchantPackagingOptionRequest(ctx *gin.Context) (normalizedMerchantPackagingOptionRequest, bool) {
	var req upsertMerchantPackagingOptionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return normalizedMerchantPackagingOptionRequest{}, false
	}

	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 50 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("包装方式名称长度需为1到50个字符")))
		return normalizedMerchantPackagingOptionRequest{}, false
	}

	descriptionText := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(descriptionText) > 200 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("包装方式描述不能超过200个字符")))
		return normalizedMerchantPackagingOptionRequest{}, false
	}
	description := pgtype.Text{}
	if descriptionText != "" {
		description = pgtype.Text{String: descriptionText, Valid: true}
	}

	return normalizedMerchantPackagingOptionRequest{
		name:        name,
		description: description,
		price:       *req.Price,
		isEnabled:   *req.IsEnabled,
		sortOrder:   *req.SortOrder,
	}, true
}

func parseMerchantPackagingOptionID(ctx *gin.Context) (int64, bool) {
	optionID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || optionID <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("包装方式ID无效")))
		return 0, false
	}
	return optionID, true
}

func isMerchantPackagingNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == merchantPackagingOptionNameUniqueConstraint
}
