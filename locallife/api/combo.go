package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 套餐管理 ====================

// comboDishInput 表示套餐中的菜品及其数量
type comboDishInput struct {
	DishID   int64 `json:"dish_id" binding:"required,min=1"` // 菜品ID
	Quantity int32 `json:"quantity" binding:"min=1,max=99"`  // 数量，1-99
}

type createComboSetRequest struct {
	Name          string           `json:"name" binding:"required,min=1,max=100"`             // 套餐名称，最大100字符
	Description   *string          `json:"description" binding:"omitempty,max=500"`           // 描述，最大500字符
	OriginalPrice int64            `json:"original_price" binding:"min=0"`                    // 原价（分），可选
	ComboPrice    int64            `json:"combo_price" binding:"required,gt=0,max=100000000"` // 套餐优惠价（分），最大100万元
	IsOnline      bool             `json:"is_online"`                                         // 是否上线
	DishIDs       []int64          `json:"dish_ids" binding:"max=50"`                         // 向后兼容：只传菜品ID（数量默认为1）
	Dishes        []comboDishInput `json:"dishes" binding:"max=50"`                           // 推荐：传菜品ID和数量
}

type comboSetResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	ComboPrice  int64   `json:"combo_price"`
	IsOnline    bool    `json:"is_online"`
}

// createComboSet godoc
// @Summary 创建套餐
// @Description 创建套餐并可选关联菜品
// @Tags 套餐管理
// @Accept json
// @Produce json
// @Param request body createComboSetRequest true "套餐信息"
// @Success 200 {object} comboSetResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos [post]
// @Security BearerAuth
func (server *Server) createComboSet(ctx *gin.Context) {
	var req createComboSetRequest
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建菜品列表（优先使用新的 Dishes 字段，向后兼容 DishIDs）
	var dishesWithQty []db.DishWithQuantity
	if len(req.Dishes) > 0 {
		// 使用新格式：带数量的菜品列表
		for _, d := range req.Dishes {
			qty := d.Quantity
			if qty <= 0 {
				qty = 1
			}
			dishesWithQty = append(dishesWithQty, db.DishWithQuantity{
				DishID:   d.DishID,
				Quantity: qty,
			})
		}
	} else if len(req.DishIDs) > 0 {
		// 向后兼容：只有菜品ID，数量默认为1
		for _, dishID := range req.DishIDs {
			dishesWithQty = append(dishesWithQty, db.DishWithQuantity{
				DishID:   dishID,
				Quantity: 1,
			})
		}
	}

	// 验证所有菜品属于当前商户
	for _, d := range dishesWithQty {
		dish, err := server.store.GetDish(ctx, d.DishID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("dish %d not found", d.DishID)))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if dish.MerchantID != merchant.ID {
			ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("dish %d does not belong to this merchant", d.DishID)))
			return
		}
	}

	// 构造创建参数
	originalPrice := req.OriginalPrice
	if originalPrice <= 0 {
		originalPrice = req.ComboPrice // 默认使用套餐价作为原价
	}

	var description pgtype.Text
	if req.Description != nil {
		description = pgtype.Text{String: *req.Description, Valid: true}
	}

	// 使用事务创建套餐（原子操作）
	result, err := server.store.CreateComboSetTx(ctx, db.CreateComboSetTxParams{
		MerchantID:    merchant.ID,
		Name:          req.Name,
		Description:   description,
		OriginalPrice: originalPrice,
		ComboPrice:    req.ComboPrice,
		IsOnline:      req.IsOnline,
		Dishes:        dishesWithQty,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, comboSetResponse{
		ID:          result.ComboSet.ID,
		Name:        result.ComboSet.Name,
		Description: stringPtrFromPgText(result.ComboSet.Description),
		ComboPrice:  result.ComboSet.ComboPrice,
		IsOnline:    result.ComboSet.IsOnline,
	})
}

type getComboSetRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type comboSetWithDetailsResponse struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	Description *string               `json:"description,omitempty"`
	ComboPrice  int64                 `json:"combo_price"`
	IsOnline    bool                  `json:"is_online"`
	Dishes      []dishInComboResponse `json:"dishes"`
	Tags        []tagResponse         `json:"tags"`
}

type dishInComboResponse struct {
	ID       int64  `json:"dish_id"`
	Name     string `json:"dish_name"`
	Price    int64  `json:"dish_price,omitempty"`
	ImageUrl string `json:"dish_image_url,omitempty"`
	Quantity int32  `json:"quantity,omitempty"`
}

type tagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// getComboSet godoc
// @Summary 获取套餐详情
// @Description 获取套餐详情（包含关联的菜品和标签），需要验证套餐归属权
// @Tags 套餐管理
// @Produce json
// @Param id path int true "套餐ID"
// @Success 200 {object} comboSetWithDetailsResponse "套餐详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非套餐所有者"
// @Failure 404 {object} ErrorResponse "套餐或商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id} [get]
// @Security BearerAuth
func (server *Server) getComboSet(ctx *gin.Context) {
	var req getComboSetRequest
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取套餐详情（使用优化的JSON聚合查询）
	result, err := server.store.GetComboSetWithDetails(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// P0修复: 验证套餐归属权
	if result.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("combo set does not belong to this merchant")))
		return
	}

	// 解析JSON字段
	var dishes []dishInComboResponse
	var tags []tagResponse

	if result.Dishes != nil {
		switch v := result.Dishes.(type) {
		case []byte:
			if len(v) > 2 { // 大于 "[]"
				if err := json.Unmarshal(v, &dishes); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		case string:
			if len(v) > 2 {
				if err := json.Unmarshal([]byte(v), &dishes); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		default:
			// pgx 可能直接返回解析后的 slice
			if jsonBytes, err := json.Marshal(v); err == nil {
				if err := json.Unmarshal(jsonBytes, &dishes); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		}
	}

	if result.Tags != nil {
		switch v := result.Tags.(type) {
		case []byte:
			if len(v) > 2 {
				if err := json.Unmarshal(v, &tags); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		case string:
			if len(v) > 2 {
				if err := json.Unmarshal([]byte(v), &tags); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		default:
			if jsonBytes, err := json.Marshal(v); err == nil {
				if err := json.Unmarshal(jsonBytes, &tags); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, comboSetWithDetailsResponse{
		ID:          result.ID,
		Name:        result.Name,
		Description: stringPtrFromPgText(result.Description),
		ComboPrice:  result.ComboPrice,
		IsOnline:    result.IsOnline,
		Dishes:      dishes,
		Tags:        tags,
	})
}

type listComboSetsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
	IsOnline *bool `form:"is_online"` // 可选过滤
}

type listComboSetsResponse struct {
	ComboSets []comboSetResponse `json:"combo_sets"`
}

// listComboSets godoc
// @Summary 获取套餐列表
// @Description 获取商户的套餐列表（分页）
// @Tags 套餐管理
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Param is_online query bool false "上架状态过滤"
// @Success 200 {object} listComboSetsResponse "套餐列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos [get]
// @Security BearerAuth
func (server *Server) listComboSets(ctx *gin.Context) {
	var req listComboSetsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// P0修复: 将 is_online 过滤传递到数据库层，避免应用层过滤导致分页不一致
	var isOnline pgtype.Bool
	if req.IsOnline != nil {
		isOnline = pgtype.Bool{Bool: *req.IsOnline, Valid: true}
	}

	// 查询套餐列表
	combos, err := server.store.ListComboSetsByMerchant(ctx, db.ListComboSetsByMerchantParams{
		MerchantID: merchant.ID,
		IsOnline:   isOnline,
		Limit:      req.PageSize,
		Offset:     (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]comboSetResponse, 0, len(combos))
	for _, combo := range combos {
		result = append(result, comboSetResponse{
			ID:          combo.ID,
			Name:        combo.Name,
			Description: stringPtrFromPgText(combo.Description),
			ComboPrice:  combo.ComboPrice,
			IsOnline:    combo.IsOnline,
		})
	}

	ctx.JSON(http.StatusOK, listComboSetsResponse{
		ComboSets: result,
	})
}

type updateComboSetUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type updateComboSetRequest struct {
	Name        *string          `json:"name" binding:"omitempty,min=1,max=100"`             // 套餐名称，最大100字符
	Description *string          `json:"description" binding:"omitempty,max=500"`            // 描述，最大500字符
	ComboPrice  *int64           `json:"combo_price" binding:"omitempty,gt=0,max=100000000"` // 套餐价格（分）
	IsOnline    *bool            `json:"is_online"`                                          // 是否上架
	Dishes      []comboDishInput `json:"dishes" binding:"omitempty,max=50"`                  // 可选：更新套餐菜品列表
}

// updateComboSet godoc
// @Summary 更新套餐信息
// @Description 更新套餐的基本信息
// @Tags 套餐管理
// @Accept json
// @Produce json
// @Param id path int true "套餐ID"
// @Param request body updateComboSetRequest true "更新内容"
// @Success 200 {object} comboSetResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非套餐所有者"
// @Failure 404 {object} ErrorResponse "套餐不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id} [put]
// @Security BearerAuth
func (server *Server) updateComboSet(ctx *gin.Context) {
	var uriReq updateComboSetUriRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateComboSetRequest
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证套餐是否属于该商户
	existingCombo, err := server.store.GetComboSet(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if existingCombo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to update this combo set")))
		return
	}

	// 构造更新参数
	params := db.UpdateComboSetParams{
		ID: uriReq.ID,
		Name: pgtype.Text{
			String: existingCombo.Name,
			Valid:  true,
		},
		Description:   existingCombo.Description,
		ImageUrl:      existingCombo.ImageUrl,
		OriginalPrice: pgtype.Int8{Int64: existingCombo.OriginalPrice, Valid: true},
		ComboPrice:    pgtype.Int8{Int64: existingCombo.ComboPrice, Valid: true},
		IsOnline: pgtype.Bool{
			Bool:  existingCombo.IsOnline,
			Valid: true,
		},
	}

	// 更新指定字段
	if req.Name != nil {
		params.Name = pgtype.Text{
			String: *req.Name,
			Valid:  true,
		}
	}

	if req.Description != nil {
		params.Description = pgtype.Text{
			String: *req.Description,
			Valid:  true,
		}
	}

	if req.ComboPrice != nil {
		params.ComboPrice = pgtype.Int8{
			Int64: *req.ComboPrice,
			Valid: true,
		}
	}

	if req.IsOnline != nil {
		params.IsOnline = pgtype.Bool{
			Bool:  *req.IsOnline,
			Valid: true,
		}
	}

	// 执行更新
	updated, err := server.store.UpdateComboSet(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果请求中包含菜品列表，则同步更新菜品
	if len(req.Dishes) > 0 {
		// 验证所有菜品属于当前商户
		for _, d := range req.Dishes {
			dish, err := server.store.GetDish(ctx, d.DishID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("dish %d not found", d.DishID)))
					return
				}
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			if dish.MerchantID != merchant.ID {
				ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("dish %d does not belong to this merchant", d.DishID)))
				return
			}
		}

		// 先删除所有旧菜品关联
		err = server.store.RemoveAllComboDishes(ctx, uriReq.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 添加新的菜品关联（带数量）
		for _, d := range req.Dishes {
			qty := d.Quantity
			if qty <= 0 {
				qty = 1
			}
			_, err = server.store.AddComboDish(ctx, db.AddComboDishParams{
				ComboID:  uriReq.ID,
				DishID:   d.DishID,
				Quantity: int16(qty),
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, comboSetResponse{
		ID:          updated.ID,
		Name:        updated.Name,
		Description: stringPtrFromPgText(updated.Description),
		ComboPrice:  updated.ComboPrice,
		IsOnline:    updated.IsOnline,
	})
}

type toggleComboOnlineBodyRequest struct {
	IsOnline *bool `json:"is_online" binding:"required"` // true=上架, false=下架
}

// toggleComboOnline godoc
// @Summary 上架/下架套餐
// @Description 更新套餐的上架状态
// @Tags 套餐管理
// @Accept json
// @Produce json
// @Param id path int true "套餐ID"
// @Param request body toggleComboOnlineBodyRequest true "状态更新"
// @Success 200 {object} map[string]string "message: combo set online status updated"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非套餐所有者"
// @Failure 404 {object} ErrorResponse "套餐不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id}/online [put]
// @Security BearerAuth
func (server *Server) toggleComboOnline(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var bodyReq toggleComboOnlineBodyRequest
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证套餐是否属于该商户
	combo, err := server.store.GetComboSet(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if combo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to modify this combo set")))
		return
	}

	// 更新上架状态
	err = server.store.UpdateComboSetOnlineStatus(ctx, db.UpdateComboSetOnlineStatusParams{
		ID:       uriReq.ID,
		IsOnline: *bodyReq.IsOnline,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "combo set online status updated"})
}

type deleteComboSetRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteComboSet godoc
// @Summary 删除套餐
// @Description 删除套餐（级联删除关联关系）
// @Tags 套餐管理
// @Produce json
// @Param id path int true "套餐ID"
// @Success 200 {object} map[string]string "message: combo set deleted"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非套餐所有者"
// @Failure 404 {object} ErrorResponse "套餐不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteComboSet(ctx *gin.Context) {
	var req deleteComboSetRequest
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证套餐是否属于该商户
	combo, err := server.store.GetComboSet(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if combo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to delete this combo set")))
		return
	}

	// 删除套餐（数据库会级联删除combo_dishes和combo_tags）
	err = server.store.DeleteComboSet(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "combo set deleted"})
}

// ==================== 套餐-菜品关联 ====================

type addComboDishBodyRequest struct {
	DishID int64 `json:"dish_id" binding:"required,min=1"`
}

// addComboDish godoc
// @Summary 向套餐添加菜品
// @Description 向套餐添加一个菜品
// @Tags 套餐管理
// @Accept json
// @Produce json
// @Param id path int true "套餐ID"
// @Param request body addComboDishBodyRequest true "菜品信息"
// @Success 200 {object} map[string]string "message: dish added to combo set"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非套餐所有者或菜品不属于该商户"
// @Failure 404 {object} ErrorResponse "套餐、商户或菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id}/dishes [post]
// @Security BearerAuth
func (server *Server) addComboDish(ctx *gin.Context) {
	var uriReq struct {
		ComboID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var bodyReq addComboDishBodyRequest
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证套餐是否属于该商户
	combo, err := server.store.GetComboSet(ctx, uriReq.ComboID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if combo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to modify this combo set")))
		return
	}

	// P1修复: 验证菜品是否属于该商户
	dish, err := server.store.GetDish(ctx, bodyReq.DishID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if dish.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dish does not belong to this merchant")))
		return
	}

	// 添加关联
	_, err = server.store.AddComboDish(ctx, db.AddComboDishParams{
		ComboID:  uriReq.ComboID,
		DishID:   bodyReq.DishID,
		Quantity: 1, // 默认数量为1
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "dish added to combo set"})
}

type removeComboDishRequest struct {
	ComboID int64 `uri:"id" binding:"required,min=1"`
	DishID  int64 `uri:"dish_id" binding:"required,min=1"`
}

// removeComboDish godoc
// @Summary 从套餐移除菜品
// @Description 从套餐移除一个菜品
// @Tags 套餐管理
// @Produce json
// @Param id path int true "套餐ID"
// @Param dish_id path int true "菜品ID"
// @Success 200 {object} map[string]string "message: dish removed from combo set"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非套餐所有者"
// @Failure 404 {object} ErrorResponse "套餐或商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/combos/{id}/dishes/{dish_id} [delete]
// @Security BearerAuth
func (server *Server) removeComboDish(ctx *gin.Context) {
	var req removeComboDishRequest
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证套餐是否属于该商户
	combo, err := server.store.GetComboSet(ctx, req.ComboID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo set not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if combo.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to modify this combo set")))
		return
	}

	// 移除关联
	err = server.store.RemoveComboDish(ctx, db.RemoveComboDishParams{
		ComboID: req.ComboID,
		DishID:  req.DishID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "dish removed from combo set"})
}

// ==================== 辅助函数 ====================

// stringPtrFromPgText 从pgtype.Text转换为*string
func stringPtrFromPgText(pt pgtype.Text) *string {
	if !pt.Valid {
		return nil
	}
	return &pt.String
}
