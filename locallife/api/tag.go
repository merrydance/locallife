package api

import (
	"errors"
	"fmt"
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

type deleteTagRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
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

	ctx.JSON(http.StatusCreated, tagDetailResponse{
		ID:        tag.ID,
		Name:      tag.Name,
		Type:      tag.Type,
		SortOrder: tag.SortOrder,
	})
}

// deleteTag godoc
// @Summary 删除标签（管理员）
// @Description 删除标签并级联删除关联关系
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "标签ID"
// @Success 201 {object} map[string]any "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "标签不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tags/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteTag(ctx *gin.Context) {
	var req deleteTagRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, err := server.store.GetTag(ctx, req.ID); err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("tag not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if err := server.store.DeleteTag(ctx, req.ID); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, deleteTagResponse{Deleted: true})
}

// ==================== 商户自助经营类目 ====================

type merchantTagsResponse struct {
	Tags []tagDetailResponse `json:"tags"`
}

type deleteTagResponse struct {
	Deleted bool `json:"deleted"`
}

type setMerchantTagsRequest struct {
	TagIDs []int64 `json:"tag_ids" binding:"required,max=5"` // 最多选5个类目
}

// getMerchantTags godoc
// @Summary 获取当前商户的经营类目标签
// @Description 获取当前登录商户已选择的经营类目标签（type=merchant）
// @Tags 商户
// @Produce json
// @Success 200 {object} merchantTagsResponse "类目标签列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/tags [get]
// @Security BearerAuth
func (server *Server) getMerchantTags(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	tags, err := server.store.ListMerchantTags(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]tagDetailResponse, len(tags))
	for i, t := range tags {
		result[i] = tagDetailResponse{
			ID:        t.ID,
			Name:      t.Name,
			Type:      t.Type,
			SortOrder: t.SortOrder,
		}
	}
	ctx.JSON(http.StatusOK, merchantTagsResponse{Tags: result})
}

// setMerchantTags godoc
// @Summary 设置当前商户的经营类目标签
// @Description 替换商户所有经营类目标签（type=merchant）。类目决定店铺在首页分类筛选中的展示位置，强烈建议完整填写。最多选 5 个。
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body setMerchantTagsRequest true "标签ID列表"
// @Success 200 {object} merchantTagsResponse "更新后的类目标签"
// @Failure 400 {object} ErrorResponse "参数错误（如超出5个或包含非merchant类型标签）"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/tags [put]
// @Security BearerAuth
func (server *Server) setMerchantTags(ctx *gin.Context) {
	var req setMerchantTagsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	seenTagIDs := make(map[int64]bool, len(req.TagIDs))
	for _, tagID := range req.TagIDs {
		if seenTagIDs[tagID] {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("tag %d is duplicated", tagID)))
			return
		}
		seenTagIDs[tagID] = true

		tag, err := server.store.GetTag(ctx, tagID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("tag %d not found", tagID)))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if tag.Type != "merchant" {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("tag %d is not a merchant category tag", tagID)))
			return
		}
	}

	result, err := server.store.SetMerchantTagsTx(ctx, db.SetMerchantTagsTxParams{
		MerchantID: merchant.ID,
		TagIDs:     req.TagIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	tags := make([]tagDetailResponse, len(result.Tags))
	for i, t := range result.Tags {
		tags[i] = tagDetailResponse{
			ID:        t.ID,
			Name:      t.Name,
			Type:      t.Type,
			SortOrder: t.SortOrder,
		}
	}
	ctx.JSON(http.StatusOK, merchantTagsResponse{Tags: tags})
}
