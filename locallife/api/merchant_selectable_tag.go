package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type listMerchantSelectableTagsRequest struct {
	Type string `form:"type" binding:"required,oneof=dish table combo customization"`
}

type createMerchantSelectableTagRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=50"`
	Type      string `json:"type" binding:"required,oneof=dish table combo customization"`
	SortOrder int16  `json:"sort_order" binding:"min=0,max=999"`
}

func buildMerchantSelectableTagResponse(tag db.ListMerchantSelectableTagsRow) tagDetailResponse {
	resp := tagDetailResponse{
		ID:        tag.ID,
		Name:      tag.Name,
		Type:      tag.Type,
		SortOrder: tag.SortOrder,
	}
	if tag.Icon.Valid {
		resp.Icon = tag.Icon.String
	}
	return resp
}

// listMerchantSelectableTags godoc
// @Summary 获取当前商户可选业务标签
// @Description 获取当前商户自行维护的菜品、桌台、套餐或定制选项标签
// @Tags 商户
// @Produce json
// @Param type query string true "标签类型" Enums(dish, table, combo, customization)
// @Success 200 {object} listTagsResponse "标签列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户或无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/tags [get]
// @Security BearerAuth
func (server *Server) listMerchantSelectableTags(ctx *gin.Context) {
	var req listMerchantSelectableTagsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}

	rows, err := server.store.ListMerchantSelectableTags(ctx, db.ListMerchantSelectableTagsParams{
		MerchantID: merchant.ID,
		Type:       req.Type,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]tagDetailResponse, len(rows))
	for i, tag := range rows {
		result[i] = buildMerchantSelectableTagResponse(tag)
	}
	ctx.JSON(http.StatusOK, listTagsResponse{Tags: result})
}

// createMerchantSelectableTag godoc
// @Summary 创建当前商户可选业务标签
// @Description 创建或复用全局唯一标签，并幂等关联到当前商户；不授予平台 /v1/tags 管理权限
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body createMerchantSelectableTagRequest true "标签信息"
// @Success 201 {object} tagDetailResponse "创建或关联后的标签"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户或无权限"
// @Failure 409 {object} ErrorResponse "同名停用标签冲突"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/tags [post]
// @Security BearerAuth
func (server *Server) createMerchantSelectableTag(ctx *gin.Context) {
	var req createMerchantSelectableTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(db.ErrInvalidTagName))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	tag, err := server.store.CreateMerchantSelectableTagTx(ctx, db.CreateMerchantSelectableTagTxParams{
		MerchantID:      merchant.ID,
		Name:            name,
		Type:            req.Type,
		SortOrder:       req.SortOrder,
		CreatedByUserID: pgtype.Int8{Int64: authPayload.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, db.ErrInvalidTagName) || errors.Is(err, db.ErrTagTypeNotSelectable) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		if errors.Is(err, db.ErrTagNameReservedInactive) {
			ctx.JSON(http.StatusConflict, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, buildTagDetailResponse(tag))
}
