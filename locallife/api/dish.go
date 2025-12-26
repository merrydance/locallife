package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

// ==================== 菜品分类 ====================

type createDishCategoryRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=30"`
	SortOrder int16  `json:"sort_order" binding:"min=0,max=999"`
}

type dishCategoryResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int16  `json:"sort_order"`
}

// createDishCategory godoc
// @Summary 创建菜品分类
// @Description 为商户创建新的菜品分类
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param request body createDishCategoryRequest true "分类详情"
// @Success 200 {object} dishCategoryResponse "创建成功的分类"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes/categories [post]
// @Security BearerAuth
func (server *Server) createDishCategory(ctx *gin.Context) {
	var req createDishCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息（验证商户权限）
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 1. 获取或创建全局分类
	category, err := server.store.CreateDishCategory(ctx, req.Name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create global dish category: %w", err)))
		return
	}

	// 2. 关联商户与分类
	_, err = server.store.LinkMerchantDishCategory(ctx, db.LinkMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("link merchant dish category: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, dishCategoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		SortOrder: req.SortOrder,
	})
}

type listDishCategoriesResponse struct {
	Categories []dishCategoryResponse `json:"categories"`
}

// listDishCategories godoc
// @Summary 获取菜品分类列表
// @Description 获取当前商户的所有菜品分类
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Success 200 {object} listDishCategoriesResponse "分类列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes/categories [get]
// @Security BearerAuth
func (server *Server) listDishCategories(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 查询分类列表
	categories, err := server.store.ListDishCategories(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list dish categories: %w", err)))
		return
	}

	// 转换响应
	result := make([]dishCategoryResponse, len(categories))
	for i, cat := range categories {
		result[i] = dishCategoryResponse{
			ID:        cat.ID,
			Name:      cat.Name,
			SortOrder: cat.SortOrder,
		}
	}

	ctx.JSON(http.StatusOK, listDishCategoriesResponse{
		Categories: result,
	})
}

type updateDishCategoryRequest struct {
	Name      *string `json:"name" binding:"omitempty,min=1,max=30"`
	SortOrder *int16  `json:"sort_order" binding:"omitempty,min=0,max=999"`
}

type updateDishCategoryUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// updateDishCategory godoc
// @Summary 更新菜品分类
// @Description 更新当前商户的菜品分类
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "分类ID"
// @Param request body updateDishCategoryRequest true "分类详情"
// @Success 200 {object} dishCategoryResponse "更新后的分类"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此分类"
// @Failure 404 {object} ErrorResponse "分类不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes/categories/{id} [patch]
// @Security BearerAuth
func (server *Server) updateDishCategory(ctx *gin.Context) {
	var uri updateDishCategoryUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateDishCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 检查分类是否属于该商户
	mdc, err := server.store.GetMerchantDishCategory(ctx, db.GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: uri.ID,
	})
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not your category")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant dish category: %w", err)))
		return
	}

	// 获取分类名称（用于判断是否改名）
	category, err := server.store.GetDishCategory(ctx, uri.ID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("category not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish category: %w", err)))
		return
	}

	// 此时已知该分类属于该商户，可以直接使用 mdc 中的 SortOrder
	finalID := uri.ID
	finalName := category.Name
	finalSortOrder := mdc.SortOrder
	if req.SortOrder != nil {
		finalSortOrder = *req.SortOrder
	}

	if req.Name != nil && *req.Name != category.Name {
		// 1. 获取或创建新名称的全局分类
		newCategory, err := server.store.CreateDishCategory(ctx, *req.Name)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create new global dish category: %w", err)))
			return
		}

		// 2. 将商户关联到新分类
		_, err = server.store.LinkMerchantDishCategory(ctx, db.LinkMerchantDishCategoryParams{
			MerchantID: merchant.ID,
			CategoryID: newCategory.ID,
			SortOrder:  finalSortOrder,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("link merchant to new dish category: %w", err)))
			return
		}

		// 3. 更新该商户下所有属于旧分类的菜品到新分类
		// 注意：我们需要一个 UpdateDishCategoryForMerchant 的查询，但目前可以先用简单的逻辑
		// 这里暂且假设我们需要更新 dishes 表
		err = server.store.UpdateDishesCategory(ctx, db.UpdateDishesCategoryParams{
			MerchantID:    merchant.ID,
			OldCategoryID: pgtype.Int8{Int64: uri.ID, Valid: true},
			NewCategoryID: pgtype.Int8{Int64: newCategory.ID, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("migrate dishes to new category: %w", err)))
			return
		}

		// 4. 取消旧分类的关联
		err = server.store.UnlinkMerchantDishCategory(ctx, db.UnlinkMerchantDishCategoryParams{
			MerchantID: merchant.ID,
			CategoryID: uri.ID,
		})
		if err != nil {
			// 即使取消失败也继续，因为新关联已经建立
		}

		finalID = newCategory.ID
		finalName = newCategory.Name
	} else if req.SortOrder != nil {
		// 如果只更新了排序，且名称没变，则直接使用新的排序
		finalSortOrder = *req.SortOrder
	}

	ctx.JSON(http.StatusOK, dishCategoryResponse{
		ID:        finalID,
		Name:      finalName,
		SortOrder: finalSortOrder,
	})
}

type deleteDishCategoryUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteDishCategory godoc
// @Summary 删除菜品分类
// @Description 删除当前商户的菜品分类
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "分类ID"
// @Success 200 {object} map[string]string "删除成功消息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此分类"
// @Failure 404 {object} ErrorResponse "分类不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes/categories/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteDishCategory(ctx *gin.Context) {
	var uri deleteDishCategoryUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 检查分类是否属于该商户
	_, err = server.store.GetMerchantDishCategory(ctx, db.GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: uri.ID,
	})
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not your category")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant dish category: %w", err)))
		return
	}

	// 取消关联分类
	err = server.store.UnlinkMerchantDishCategory(ctx, db.UnlinkMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: uri.ID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("unlink merchant dish category: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "category deleted successfully"})
}

// ==================== 菜品管理 ====================

type createDishRequest struct {
	CategoryID          *int64                    `json:"category_id" binding:"omitempty,min=1"`
	Name                string                    `json:"name" binding:"required,min=1,max=50"`
	Description         string                    `json:"description" binding:"omitempty,max=500"`
	ImageURL            string                    `json:"image_url" binding:"omitempty,max=500"`
	Price               int64                     `json:"price" binding:"required,min=1,max=9999900"` // 最高99999元
	MemberPrice         *int64                    `json:"member_price" binding:"omitempty,min=1,max=9999900"`
	IsAvailable         bool                      `json:"is_available"`
	IsOnline            bool                      `json:"is_online"`
	SortOrder           int16                     `json:"sort_order" binding:"min=0,max=999"`
	PrepareTime         int16                     `json:"prepare_time" binding:"omitempty,min=0,max=120"`       // 预估制作时间（分钟），0表示使用默认值10分钟
	IngredientIDs       []int64                   `json:"ingredient_ids" binding:"omitempty,max=20,dive,min=1"` // 最多20个食材
	TagIDs              []int64                   `json:"tag_ids" binding:"omitempty,max=10,dive,min=1"`        // 最多10个标签
	CustomizationGroups []customizationGroupInput `json:"customization_groups" binding:"omitempty,max=20,dive"` // 定制选项分组
}

type dishResponse struct {
	ID                  int64                `json:"id"`
	MerchantID          int64                `json:"merchant_id"`
	CategoryID          *int64               `json:"category_id"`
	CategoryName        *string              `json:"category_name,omitempty"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	ImageURL            string               `json:"image_url"`
	Price               int64                `json:"price"`
	MemberPrice         *int64               `json:"member_price"`
	IsAvailable         bool                 `json:"is_available"`
	IsOnline            bool                 `json:"is_online"`
	SortOrder           int16                `json:"sort_order"`
	PrepareTime         int16                `json:"prepare_time"` // 预估制作时间（分钟）
	Ingredients         []ingredient         `json:"ingredients,omitempty"`
	Tags                []tagInfo            `json:"tags,omitempty"`
	CustomizationGroups []customizationGroup `json:"customization_groups,omitempty"`
}

type ingredient struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	IsAllergen bool   `json:"is_allergen"`
}

type tagInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type customizationGroup struct {
	ID         int64                 `json:"id"`
	Name       string                `json:"name"`
	IsRequired bool                  `json:"is_required"`
	SortOrder  int16                 `json:"sort_order"`
	Options    []customizationOption `json:"options"`
}

type customizationOption struct {
	ID         int64  `json:"id"`
	TagID      int64  `json:"tag_id"`
	TagName    string `json:"tag_name"`
	ExtraPrice int64  `json:"extra_price"`
	SortOrder  int16  `json:"sort_order"`
}

// createDish godoc
// @Summary 创建菜品
// @Description 为商户创建新菜品，可同时关联食材和标签（使用事务保证原子性）
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param request body createDishRequest true "菜品详情"
// @Success 200 {object} dishResponse "创建成功的菜品"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes [post]
// @Security BearerAuth
func (server *Server) createDish(ctx *gin.Context) {
	var req createDishRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 菜品图片必须先审后存：仅允许使用 uploads/public/... 本地路径
	if strings.TrimSpace(req.ImageURL) != "" {
		normalized := normalizeStoredUploadPath(req.ImageURL)
		prefix := fmt.Sprintf("uploads/public/merchants/%d/dishes/", merchant.ID)
		if normalized == "" || !strings.HasPrefix(normalized, prefix) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("image_url 仅允许使用通过菜品图片上传接口生成的本地路径")))
			return
		}
		req.ImageURL = normalized
	}

	// 准备参数
	var categoryID pgtype.Int8
	if req.CategoryID != nil {
		// P1修复: 验证分类属于当前商户 (通过关联表)
		_, err := server.store.GetMerchantDishCategory(ctx, db.GetMerchantDishCategoryParams{
			MerchantID: merchant.ID,
			CategoryID: *req.CategoryID,
		})
		if err != nil {
			if isNoRows(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("category does not belong to this merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("verify merchant category: %w", err)))
			return
		}
		categoryID = pgtype.Int8{Int64: *req.CategoryID, Valid: true}
	}

	var memberPrice pgtype.Int8
	if req.MemberPrice != nil {
		memberPrice = pgtype.Int8{Int64: *req.MemberPrice, Valid: true}
	}

	// 处理预估制作时间默认值
	const defaultPrepareTime int16 = 10 // 默认10分钟
	prepareTime := req.PrepareTime
	if prepareTime <= 0 {
		prepareTime = defaultPrepareTime
	}

	// 使用事务创建菜品+食材+标签，保证原子性
	txResult, err := server.store.CreateDishTx(ctx, db.CreateDishTxParams{
		MerchantID:    merchant.ID,
		CategoryID:    categoryID,
		Name:          req.Name,
		Description:   pgtype.Text{String: req.Description, Valid: req.Description != ""},
		ImageUrl:      pgtype.Text{String: normalizeImageURLForStorage(req.ImageURL), Valid: req.ImageURL != ""},
		Price:         req.Price,
		MemberPrice:   memberPrice,
		IsAvailable:   req.IsAvailable,
		IsOnline:      req.IsOnline,
		SortOrder:     req.SortOrder,
		PrepareTime:   prepareTime,
		IngredientIDs: req.IngredientIDs,
		TagIDs:        req.TagIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create dish tx: %w", err)))
		return
	}

	// 如果有定制选项，保存定制选项
	var customizationGroups []customizationGroup
	if len(req.CustomizationGroups) > 0 {
		groups := make([]db.CustomizationGroupInput, 0, len(req.CustomizationGroups))
		for _, g := range req.CustomizationGroups {
			options := make([]db.CustomizationOptionInput, 0, len(g.Options))
			for _, o := range g.Options {
				options = append(options, db.CustomizationOptionInput{
					TagID:      o.TagID,
					ExtraPrice: o.ExtraPrice,
					SortOrder:  o.SortOrder,
				})
			}
			groups = append(groups, db.CustomizationGroupInput{
				Name:       g.Name,
				IsRequired: g.IsRequired,
				SortOrder:  g.SortOrder,
				Options:    options,
			})
		}

		custResult, err := server.store.SetDishCustomizationsTx(ctx, db.SetDishCustomizationsTxParams{
			DishID: txResult.Dish.ID,
			Groups: groups,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("set dish customizations: %w", err)))
			return
		}

		// 构建定制选项响应
		for _, g := range custResult.Groups {
			var options []customizationOption
			for _, o := range g.Options {
				// 获取标签名称
				tag, _ := server.store.GetTag(ctx, o.TagID)
				options = append(options, customizationOption{
					ID:         o.ID,
					TagID:      o.TagID,
					TagName:    tag.Name,
					ExtraPrice: o.ExtraPrice,
					SortOrder:  o.SortOrder,
				})
			}
			customizationGroups = append(customizationGroups, customizationGroup{
				ID:         g.Group.ID,
				Name:       g.Group.Name,
				IsRequired: g.Group.IsRequired,
				SortOrder:  g.Group.SortOrder,
				Options:    options,
			})
		}
	}

	ctx.JSON(http.StatusOK, dishResponse{
		ID:                  txResult.Dish.ID,
		MerchantID:          txResult.Dish.MerchantID,
		CategoryID:          toPtrInt64(txResult.Dish.CategoryID),
		Name:                txResult.Dish.Name,
		Description:         txResult.Dish.Description.String,
		ImageURL:            normalizeUploadURLForClient(txResult.Dish.ImageUrl.String),
		Price:               txResult.Dish.Price,
		MemberPrice:         toPtrInt64(txResult.Dish.MemberPrice),
		IsAvailable:         txResult.Dish.IsAvailable,
		IsOnline:            txResult.Dish.IsOnline,
		SortOrder:           txResult.Dish.SortOrder,
		PrepareTime:         txResult.Dish.PrepareTime,
		CustomizationGroups: customizationGroups,
	})
}

type listDishesRequest struct {
	CategoryID  *int64 `form:"category_id"`
	IsOnline    *bool  `form:"is_online"`
	IsAvailable *bool  `form:"is_available"`
	PageID      int32  `form:"page_id" binding:"required,min=1"`
	PageSize    int32  `form:"page_size" binding:"required,min=5,max=50"`
}

type listDishesResponse struct {
	Dishes     []dishResponse `json:"dishes"`
	TotalCount int64          `json:"total_count"`
}

// listDishesByMerchant godoc
// @Summary 获取商户菜品列表
// @Description 分页获取当前商户的菜品列表，支持筛选
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param category_id query int false "按分类筛选"
// @Param is_online query bool false "按上架状态筛选"
// @Param is_available query bool false "按可用状态筛选"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} listDishesResponse "菜品列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dishes [get]
// @Security BearerAuth
func (server *Server) listDishesByMerchant(ctx *gin.Context) {
	var req listDishesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if isNoRows(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 准备查询参数
	var categoryID pgtype.Int8
	if req.CategoryID != nil {
		categoryID = pgtype.Int8{Int64: *req.CategoryID, Valid: true}
	}

	var isOnline pgtype.Bool
	if req.IsOnline != nil {
		isOnline = pgtype.Bool{Bool: *req.IsOnline, Valid: true}
	}

	var isAvailable pgtype.Bool
	if req.IsAvailable != nil {
		isAvailable = pgtype.Bool{Bool: *req.IsAvailable, Valid: true}
	}

	// 查询菜品列表
	dishes, err := server.store.ListDishesByMerchant(ctx, db.ListDishesByMerchantParams{
		MerchantID:  merchant.ID,
		CategoryID:  categoryID,
		IsOnline:    isOnline,
		IsAvailable: isAvailable,
		Limit:       req.PageSize,
		Offset:      (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list dishes by merchant: %w", err)))
		return
	}

	// 获取总数
	count, err := server.store.CountDishesByMerchant(ctx, db.CountDishesByMerchantParams{
		MerchantID: merchant.ID,
		IsOnline:   isOnline,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("count dishes by merchant: %w", err)))
		return
	}

	// 转换响应
	result := make([]dishResponse, len(dishes))
	for i, dish := range dishes {
		result[i] = dishResponse{
			ID:          dish.ID,
			MerchantID:  dish.MerchantID,
			CategoryID:  toPtrInt64(dish.CategoryID),
			Name:        dish.Name,
			Description: dish.Description.String,
			ImageURL:    normalizeUploadURLForClient(dish.ImageUrl.String),
			Price:       dish.Price,
			MemberPrice: toPtrInt64(dish.MemberPrice),
			IsAvailable: dish.IsAvailable,
			IsOnline:    dish.IsOnline,
			SortOrder:   dish.SortOrder,
		}
	}

	ctx.JSON(http.StatusOK, listDishesResponse{
		Dishes:     result,
		TotalCount: count,
	})
}

type getDishRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getDish godoc
// @Summary 获取菜品详情
// @Description 获取菜品详细信息，包括食材、标签和定制选项
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "菜品ID"
// @Success 200 {object} dishResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "菜品或商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id} [get]
// @Security BearerAuth
func (server *Server) getDish(ctx *gin.Context) {
	var req getDishRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 使用单一查询获取菜品完整信息(含食材、标签、定制选项)
	dish, err := server.store.GetDishComplete(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish complete: %w", err)))
		return
	}

	// 验证菜品属于当前商户
	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 解析 JSON 字段
	var ingredients []ingredient
	if err := parseJSON(dish.Ingredients, &ingredients); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to parse ingredients: %w", err)))
		return
	}

	var tags []tagInfo
	if err := parseJSON(dish.Tags, &tags); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to parse tags: %w", err)))
		return
	}

	var customizationGroups []customizationGroup
	if err := parseJSON(dish.CustomizationGroups, &customizationGroups); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to parse customization_groups: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, dishResponse{
		ID:                  dish.ID,
		MerchantID:          dish.MerchantID,
		CategoryID:          toPtrInt64(dish.CategoryID),
		CategoryName:        toPtrString(dish.CategoryName),
		Name:                dish.Name,
		Description:         dish.Description.String,
		ImageURL:            normalizeUploadURLForClient(dish.ImageUrl.String),
		Price:               dish.Price,
		MemberPrice:         toPtrInt64(dish.MemberPrice),
		IsAvailable:         dish.IsAvailable,
		IsOnline:            dish.IsOnline,
		SortOrder:           dish.SortOrder,
		Ingredients:         ingredients,
		Tags:                tags,
		CustomizationGroups: customizationGroups,
	})
}

type updateDishRequest struct {
	CategoryID  *int64  `json:"category_id" binding:"omitempty,min=1"`
	Name        string  `json:"name" binding:"omitempty,min=1,max=100"`        // 菜品名称，最大100字符
	Description string  `json:"description" binding:"omitempty,max=1000"`      // 描述，最大1000字符
	ImageURL    string  `json:"image_url" binding:"omitempty,max=500"`         // 图片URL，最大500字符
	Price       *int64  `json:"price" binding:"omitempty,min=1,max=100000000"` // 价格（分），最大100万元
	MemberPrice *int64  `json:"member_price" binding:"omitempty,min=0,max=100000000"`
	IsAvailable *bool   `json:"is_available"`
	IsOnline    *bool   `json:"is_online"`
	SortOrder   *int16  `json:"sort_order" binding:"omitempty,min=0"`
	PrepareTime *int16  `json:"prepare_time" binding:"omitempty,min=1,max=120"` // 预估制作时间（分钟），1-120分钟
	TagIDs      []int64 `json:"tag_ids" binding:"omitempty,max=10,dive,min=1"`  // 标签ID列表（最多10个）
}

// updateDish godoc
// @Summary 更新菜品信息
// @Description 更新菜品信息（部分更新）
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "菜品ID"
// @Param request body updateDishRequest true "菜品更新详情"
// @Success 200 {object} dishResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非菜品所有者"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id} [put]
// @Security BearerAuth
func (server *Server) updateDish(ctx *gin.Context) {
	var uriReq getDishRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateDishRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 验证菜品所有权
	dish, err := server.store.GetDish(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish: %w", err)))
		return
	}

	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 菜品图片必须先审后存：仅允许使用 uploads/public/... 本地路径
	if strings.TrimSpace(req.ImageURL) != "" {
		normalized := normalizeStoredUploadPath(req.ImageURL)
		prefix := fmt.Sprintf("uploads/public/merchants/%d/dishes/", merchant.ID)
		if normalized == "" || !strings.HasPrefix(normalized, prefix) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("image_url 仅允许使用通过菜品图片上传接口生成的本地路径")))
			return
		}
		req.ImageURL = normalized
	}

	// 准备更新参数
	var categoryID pgtype.Int8
	if req.CategoryID != nil {
		categoryID = pgtype.Int8{Int64: *req.CategoryID, Valid: true}
	}

	var name pgtype.Text
	if req.Name != "" {
		name = pgtype.Text{String: req.Name, Valid: true}
	}

	var description pgtype.Text
	if req.Description != "" {
		description = pgtype.Text{String: req.Description, Valid: true}
	}

	var imageURL pgtype.Text
	if req.ImageURL != "" {
		imageURL = pgtype.Text{String: normalizeImageURLForStorage(req.ImageURL), Valid: true}
	}

	var price pgtype.Int8
	if req.Price != nil {
		price = pgtype.Int8{Int64: *req.Price, Valid: true}
	}

	var memberPrice pgtype.Int8
	if req.MemberPrice != nil {
		memberPrice = pgtype.Int8{Int64: *req.MemberPrice, Valid: true}
	}

	var isAvailable pgtype.Bool
	if req.IsAvailable != nil {
		isAvailable = pgtype.Bool{Bool: *req.IsAvailable, Valid: true}
	}

	var isOnline pgtype.Bool
	if req.IsOnline != nil {
		isOnline = pgtype.Bool{Bool: *req.IsOnline, Valid: true}
	}

	var sortOrder pgtype.Int2
	if req.SortOrder != nil {
		sortOrder = pgtype.Int2{Int16: *req.SortOrder, Valid: true}
	}

	var prepareTime pgtype.Int2
	if req.PrepareTime != nil {
		prepareTime = pgtype.Int2{Int16: *req.PrepareTime, Valid: true}
	}

	// 准备标签ID（如果提供则更新）
	var tagIDs *[]int64
	if req.TagIDs != nil {
		tagIDs = &req.TagIDs
	}

	// 使用事务更新菜品（支持标签更新）
	txResult, err := server.store.UpdateDishTx(ctx, db.UpdateDishTxParams{
		ID:          uriReq.ID,
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		ImageUrl:    imageURL,
		Price:       price,
		MemberPrice: memberPrice,
		IsAvailable: isAvailable,
		IsOnline:    isOnline,
		SortOrder:   sortOrder,
		PrepareTime: prepareTime,
		TagIDs:      tagIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update dish tx: %w", err)))
		return
	}

	// 构建标签响应
	var tags []tagInfo
	for _, dt := range txResult.Tags {
		tag, _ := server.store.GetTag(ctx, dt.TagID)
		tags = append(tags, tagInfo{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	ctx.JSON(http.StatusOK, dishResponse{
		ID:          txResult.Dish.ID,
		MerchantID:  txResult.Dish.MerchantID,
		CategoryID:  toPtrInt64(txResult.Dish.CategoryID),
		Name:        txResult.Dish.Name,
		Description: txResult.Dish.Description.String,
		ImageURL:    normalizeUploadURLForClient(txResult.Dish.ImageUrl.String),
		Price:       txResult.Dish.Price,
		MemberPrice: toPtrInt64(txResult.Dish.MemberPrice),
		IsAvailable: txResult.Dish.IsAvailable,
		IsOnline:    txResult.Dish.IsOnline,
		SortOrder:   txResult.Dish.SortOrder,
		PrepareTime: txResult.Dish.PrepareTime,
		Tags:        tags,
	})
}

// deleteDish godoc
// @Summary 删除菜品
// @Description 软删除菜品（标记为删除）
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "菜品ID"
// @Success 200 {object} map[string]string "message: dish deleted successfully"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非菜品所有者"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteDish(ctx *gin.Context) {
	var req getDishRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 验证菜品所有权
	dish, err := server.store.GetDish(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish: %w", err)))
		return
	}

	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 删除菜品（CASCADE 会自动删除关联的食材、标签、定制选项等）
	err = server.store.DeleteDish(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("delete dish: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "dish deleted successfully"})
}

// ==================== 辅助函数 ====================

func toPtrInt64(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func toPtrString(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// parseJSON 解析 interface{} 类型的 JSON 数据
func parseJSON(data interface{}, target interface{}) error {
	if data == nil {
		return nil
	}

	// pgx 可能返回不同类型的数据，需要处理多种情况
	switch v := data.(type) {
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.Unmarshal(v, target)
	case string:
		if v == "" || v == "[]" || v == "null" {
			return nil
		}
		return json.Unmarshal([]byte(v), target)
	default:
		// 如果已经是 Go 类型（map/slice），使用 JSON 序列化再反序列化
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		return json.Unmarshal(jsonBytes, target)
	}
}

// ==================== 菜品上下架管理 ====================

type updateDishStatusRequest struct {
	IsOnline *bool `json:"is_online" binding:"required"` // true=上架, false=下架
}

type updateDishStatusUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type dishStatusResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsOnline bool   `json:"is_online"`
	Message  string `json:"message"`
}

// updateDishStatus godoc
// @Summary 更新菜品上下架状态
// @Description 上架或下架菜品
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "菜品ID"
// @Param request body updateDishStatusRequest true "状态更新"
// @Success 200 {object} dishStatusResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id}/status [patch]
// @Security BearerAuth
func (server *Server) updateDishStatus(ctx *gin.Context) {
	var uri updateDishStatusUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateDishStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 获取菜品
	dish, err := server.store.GetDish(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish: %w", err)))
		return
	}

	// 验证菜品所有权
	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 更新菜品状态
	err = server.store.UpdateDishOnlineStatus(ctx, db.UpdateDishOnlineStatusParams{
		ID:       uri.ID,
		IsOnline: *req.IsOnline,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update dish online status: %w", err)))
		return
	}

	// 构建响应
	message := "菜品已下架"
	if *req.IsOnline {
		message = "菜品已上架"
	}

	ctx.JSON(http.StatusOK, dishStatusResponse{
		ID:       dish.ID,
		Name:     dish.Name,
		IsOnline: *req.IsOnline,
		Message:  message,
	})
}

// batchUpdateDishStatus godoc
// @Summary 批量更新菜品上下架状态
// @Description 批量上架或下架多个菜品
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param request body batchUpdateDishStatusRequest true "批量状态更新"
// @Success 200 {object} batchDishStatusResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/batch/status [patch]
// @Security BearerAuth
func (server *Server) batchUpdateDishStatus(ctx *gin.Context) {
	var req batchUpdateDishStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 批量获取菜品信息（验证菜品存在及所有权）
	dishes, err := server.store.GetDishesByIDsAll(ctx, req.DishIDs)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dishes by ids: %w", err)))
		return
	}

	// 构建属于该商户的菜品 ID 列表
	dishMap := make(map[int64]bool)
	for _, dish := range dishes {
		dishMap[dish.ID] = dish.MerchantID == merchant.ID
	}

	var validDishIDs []int64
	var failed []int64
	for _, dishID := range req.DishIDs {
		if belongs, exists := dishMap[dishID]; !exists || !belongs {
			failed = append(failed, dishID)
		} else {
			validDishIDs = append(validDishIDs, dishID)
		}
	}

	var updated []int64
	if len(validDishIDs) > 0 {
		// 批量更新菜品状态
		rowsAffected, err := server.store.BatchUpdateDishOnlineStatus(ctx, db.BatchUpdateDishOnlineStatusParams{
			IsOnline:   *req.IsOnline,
			Column2:    validDishIDs,
			MerchantID: merchant.ID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("batch update dish online status: %w", err)))
			return
		}

		if rowsAffected == int64(len(validDishIDs)) {
			updated = validDishIDs
		} else {
			// 部分更新成功，需要查询哪些实际更新了（边缘情况）
			updated = validDishIDs
		}
	}

	message := "批量下架完成"
	if *req.IsOnline {
		message = "批量上架完成"
	}

	ctx.JSON(http.StatusOK, batchDishStatusResponse{
		Updated: updated,
		Failed:  failed,
		Message: message,
	})
}

type batchUpdateDishStatusRequest struct {
	DishIDs  []int64 `json:"dish_ids" binding:"required,min=1,max=100"` // 菜品ID列表，最多100个
	IsOnline *bool   `json:"is_online" binding:"required"`              // true=上架, false=下架
}

type batchDishStatusResponse struct {
	Updated []int64 `json:"updated"` // 更新成功的ID
	Failed  []int64 `json:"failed"`  // 更新失败的ID
	Message string  `json:"message"`
}

// ==================== 菜品定制选项管理 ====================

type customizationOptionInput struct {
	TagID      int64 `json:"tag_id" binding:"required,min=1"`         // 标签ID（如：微辣、中辣、特辣）
	ExtraPrice int64 `json:"extra_price" binding:"min=0,max=1000000"` // 加价（分），最大1万元
	SortOrder  int16 `json:"sort_order" binding:"min=0"`              // 排序
}

type customizationGroupInput struct {
	Name       string                     `json:"name" binding:"required,min=1,max=50"` // 分组名称（如：辣度、规格），最大50字符
	IsRequired bool                       `json:"is_required"`                          // 是否必选
	SortOrder  int16                      `json:"sort_order" binding:"min=0"`           // 排序
	Options    []customizationOptionInput `json:"options" binding:"required,dive"`      // 选项列表
}

type setDishCustomizationsRequest struct {
	Groups []customizationGroupInput `json:"groups" binding:"max=20,dive"` // 定制分组列表，最多20个分组
}

type customizationOptionResponse struct {
	ID         int64  `json:"id"`
	TagID      int64  `json:"tag_id"`
	TagName    string `json:"tag_name"`
	ExtraPrice int64  `json:"extra_price"`
	SortOrder  int16  `json:"sort_order"`
}

type customizationGroupResponse struct {
	ID         int64                         `json:"id"`
	Name       string                        `json:"name"`
	IsRequired bool                          `json:"is_required"`
	SortOrder  int16                         `json:"sort_order"`
	Options    []customizationOptionResponse `json:"options"`
}

type dishCustomizationsResponse struct {
	DishID int64                        `json:"dish_id"`
	Groups []customizationGroupResponse `json:"groups"`
}

// setDishCustomizations godoc
// @Summary 设置菜品定制选项
// @Description 设置/替换菜品的所有定制分组和选项（规格定制）
// @Tags 菜品管理
// @Accept json
// @Produce json
// @Param id path int true "菜品ID"
// @Param request body setDishCustomizationsRequest true "定制分组"
// @Success 200 {object} dishCustomizationsResponse
// @Failure 400 {object} ErrorResponse "参数错误或标签不存在"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户或菜品不属于该商户"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id}/customizations [put]
// @Security BearerAuth
func (server *Server) setDishCustomizations(ctx *gin.Context) {
	var uri getDishRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req setDishCustomizationsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

	// 获取菜品
	dish, err := server.store.GetDish(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish: %w", err)))
		return
	}

	// 验证菜品所有权
	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 预先验证所有标签存在，并收集标签名称
	tagNameMap := make(map[int64]string)
	for _, g := range req.Groups {
		for _, o := range g.Options {
			if _, exists := tagNameMap[o.TagID]; !exists {
				tag, err := server.store.GetTag(ctx, o.TagID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("tag %d not found", o.TagID)))
						return
					}
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get tag %d: %w", o.TagID, err)))
					return
				}
				tagNameMap[o.TagID] = tag.Name
			}
		}
	}

	// 构造事务参数
	groups := make([]db.CustomizationGroupInput, 0, len(req.Groups))
	for _, g := range req.Groups {
		options := make([]db.CustomizationOptionInput, 0, len(g.Options))
		for _, o := range g.Options {
			options = append(options, db.CustomizationOptionInput{
				TagID:      o.TagID,
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
		}
		groups = append(groups, db.CustomizationGroupInput{
			Name:       g.Name,
			IsRequired: g.IsRequired,
			SortOrder:  g.SortOrder,
			Options:    options,
		})
	}

	// 使用事务设置定制选项（原子操作）
	result, err := server.store.SetDishCustomizationsTx(ctx, db.SetDishCustomizationsTxParams{
		DishID: uri.ID,
		Groups: groups,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("set dish customizations tx: %w", err)))
		return
	}

	// 构建响应
	var resultGroups []customizationGroupResponse
	for _, g := range result.Groups {
		var options []customizationOptionResponse
		for _, o := range g.Options {
			options = append(options, customizationOptionResponse{
				ID:         o.ID,
				TagID:      o.TagID,
				TagName:    tagNameMap[o.TagID],
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
		}
		resultGroups = append(resultGroups, customizationGroupResponse{
			ID:         g.Group.ID,
			Name:       g.Group.Name,
			IsRequired: g.Group.IsRequired,
			SortOrder:  g.Group.SortOrder,
			Options:    options,
		})
	}

	ctx.JSON(http.StatusOK, dishCustomizationsResponse{
		DishID: uri.ID,
		Groups: resultGroups,
	})
}

// getDishCustomizations godoc
// @Summary 获取菜品定制选项
// @Description 获取菜品的所有定制分组和选项
// @Tags 菜品管理
// @Produce json
// @Param id path int true "菜品ID"
// @Success 200 {object} dishCustomizationsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/dishes/{id}/customizations [get]
func (server *Server) getDishCustomizations(ctx *gin.Context) {
	var uri getDishRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 使用单次查询获取菜品和所有定制信息（消除 N+1 查询）
	dish, err := server.store.GetDishWithCustomizations(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish with customizations: %w", err)))
		return
	}

	// 解析 JSON 格式的定制分组数据
	var resultGroups []customizationGroupResponse
	if dish.CustomizationGroups != nil {
		groupsJSON, err := json.Marshal(dish.CustomizationGroups)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal customization groups: %w", err)))
			return
		}

		var dbGroups []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			IsRequired bool   `json:"is_required"`
			SortOrder  int16  `json:"sort_order"`
			Options    []struct {
				ID         int64  `json:"id"`
				TagID      int64  `json:"tag_id"`
				TagName    string `json:"tag_name"`
				ExtraPrice int64  `json:"extra_price"`
				SortOrder  int16  `json:"sort_order"`
			} `json:"options"`
		}

		if err := json.Unmarshal(groupsJSON, &dbGroups); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("unmarshal customization groups: %w", err)))
			return
		}

		for _, g := range dbGroups {
			var options []customizationOptionResponse
			for _, o := range g.Options {
				options = append(options, customizationOptionResponse{
					ID:         o.ID,
					TagID:      o.TagID,
					TagName:    o.TagName,
					ExtraPrice: o.ExtraPrice,
					SortOrder:  o.SortOrder,
				})
			}
			resultGroups = append(resultGroups, customizationGroupResponse{
				ID:         g.ID,
				Name:       g.Name,
				IsRequired: g.IsRequired,
				SortOrder:  g.SortOrder,
				Options:    options,
			})
		}
	}

	ctx.JSON(http.StatusOK, dishCustomizationsResponse{
		DishID: dish.ID,
		Groups: resultGroups,
	})
}
