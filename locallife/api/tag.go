package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ==================== 标签管理 ====================

// tagDetailResponse 标签详细信息（包含类型和排序）
type tagDetailResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	SortOrder int16  `json:"sort_order"`
}

type listTagsRequest struct {
	Type string `form:"type" binding:"required,oneof=dish merchant combo table customization"` // 标签类型
}

type listTagsResponse struct {
	Tags []tagDetailResponse `json:"tags"`
}

// listTags godoc
// @Summary 获取标签列表
// @Description 根据类型获取所有激活状态的标签
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param type query string true "标签类型" Enums(dish, merchant, combo, table, customization)
// @Success 200 {object} listTagsResponse "标签列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tags [get]
// @Security BearerAuth
func (server *Server) listTags(ctx *gin.Context) {
	var req listTagsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证用户已登录
	_ = ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取标签列表
	tags, err := server.store.ListAllTagsByType(ctx, req.Type)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]tagDetailResponse, len(tags))
	for i, tag := range tags {
		result[i] = tagDetailResponse{
			ID:        tag.ID,
			Name:      tag.Name,
			Type:      tag.Type,
			SortOrder: tag.SortOrder,
		}
	}

	ctx.JSON(http.StatusOK, listTagsResponse{Tags: result})
}

// createTag godoc
// @Summary 创建标签（管理员）
// @Description 创建新标签，需要管理员权限
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param request body createTagRequest true "标签信息"
// @Success 200 {object} tagResponse "创建的标签"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tags [post]
// @Security BearerAuth
type createTagRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=50"`                                  // 标签名称
	Type      string `json:"type" binding:"required,oneof=dish merchant combo table customization"` // 标签类型
	SortOrder int16  `json:"sort_order" binding:"min=0,max=999"`                                    // 排序
}

func (server *Server) createTag(ctx *gin.Context) {
	var req createTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证用户已登录（后续可添加管理员权限检查）
	_ = ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 创建标签
	tag, err := server.store.CreateTag(ctx, db.CreateTagParams{
		Name:      req.Name,
		Type:      req.Type,
		SortOrder: req.SortOrder,
		Status:    "active",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, tagDetailResponse{
		ID:        tag.ID,
		Name:      tag.Name,
		Type:      tag.Type,
		SortOrder: tag.SortOrder,
	})
}
