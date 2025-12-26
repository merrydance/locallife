package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 库存管理 ====================

// calculateAvailable 计算可用库存，处理无限库存情况
// total_quantity = -1 表示无限库存，此时返回 -1
func calculateAvailable(totalQuantity, soldQuantity int32) int32 {
	if totalQuantity == -1 {
		return -1 // 无限库存
	}
	return totalQuantity - soldQuantity
}

type createDailyInventoryRequest struct {
	DishID        int64  `json:"dish_id" binding:"required,min=1"`
	Date          string `json:"date" binding:"required"`                  // 日期 YYYY-MM-DD
	TotalQuantity int32  `json:"total_quantity" binding:"required,gte=-1"` // -1表示无限库存
}

type dailyInventoryResponse struct {
	ID            int64  `json:"id"`
	MerchantID    int64  `json:"merchant_id"`
	DishID        int64  `json:"dish_id"`
	Date          string `json:"date"`
	TotalQuantity int32  `json:"total_quantity"`
	SoldQuantity  int32  `json:"sold_quantity"`
	Available     int32  `json:"available"` // 计算字段: total - sold
}

// createDailyInventory godoc
// @Summary 创建每日库存
// @Description 为指定菜品创建某日的库存记录，-1表示无限库存
// @Tags 库存管理
// @Accept json
// @Produce json
// @Param request body createDailyInventoryRequest true "库存信息"
// @Success 200 {object} dailyInventoryResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory [post]
// @Security BearerAuth
func (server *Server) createDailyInventory(ctx *gin.Context) {
	var req createDailyInventoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建库存记录
	inventory, err := server.store.CreateDailyInventory(ctx, db.CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        req.DishID,
		Date:          pgtype.Date{Time: date, Valid: true},
		TotalQuantity: req.TotalQuantity,
		SoldQuantity:  0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, dailyInventoryResponse{
		ID:            inventory.ID,
		MerchantID:    inventory.MerchantID,
		DishID:        inventory.DishID,
		Date:          inventory.Date.Time.Format("2006-01-02"),
		TotalQuantity: inventory.TotalQuantity,
		SoldQuantity:  inventory.SoldQuantity,
		Available:     calculateAvailable(inventory.TotalQuantity, inventory.SoldQuantity),
	})
}

type listDailyInventoryRequest struct {
	Date string `form:"date" binding:"required"` // 按日期查询 (YYYY-MM-DD)
}

type listDailyInventoryResponse struct {
	Inventories []dailyInventoryWithDishResponse `json:"inventories"`
}

type dailyInventoryWithDishResponse struct {
	ID            int64  `json:"id"`
	MerchantID    int64  `json:"merchant_id"`
	DishID        int64  `json:"dish_id"`
	DishName      string `json:"dish_name"`
	DishPrice     int64  `json:"dish_price"`
	Date          string `json:"date"`
	TotalQuantity int32  `json:"total_quantity"`
	SoldQuantity  int32  `json:"sold_quantity"`
	Available     int32  `json:"available"`
}

// listDailyInventory godoc
// @Summary 查询每日库存
// @Description 列出商户某日的所有菜品库存
// @Tags 库存管理
// @Produce json
// @Param date query string true "日期(YYYY-MM-DD)"
// @Success 200 {object} listDailyInventoryResponse "库存列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory [get]
// @Security BearerAuth
func (server *Server) listDailyInventory(ctx *gin.Context) {
	var req listDailyInventoryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format")))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 按商户和日期查询
	inventories, err := server.store.ListDailyInventoryByMerchant(ctx, db.ListDailyInventoryByMerchantParams{
		MerchantID: merchant.ID,
		Date:       pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	result := make([]dailyInventoryWithDishResponse, len(inventories))
	for i, inv := range inventories {
		result[i] = dailyInventoryWithDishResponse{
			ID:            inv.ID,
			MerchantID:    inv.MerchantID,
			DishID:        inv.DishID,
			DishName:      inv.DishName,
			DishPrice:     inv.DishPrice,
			Date:          inv.Date.Time.Format("2006-01-02"),
			TotalQuantity: inv.TotalQuantity,
			SoldQuantity:  inv.SoldQuantity,
			Available:     calculateAvailable(inv.TotalQuantity, inv.SoldQuantity),
		}
	}

	ctx.JSON(http.StatusOK, listDailyInventoryResponse{
		Inventories: result,
	})
}

type updateDailyInventoryRequest struct {
	DishID        int64  `json:"dish_id" binding:"required,min=1"`
	Date          string `json:"date" binding:"required"`
	TotalQuantity *int32 `json:"total_quantity" binding:"omitempty,gte=-1"`
	SoldQuantity  *int32 `json:"sold_quantity" binding:"omitempty,gte=0"`
}

// updateDailyInventory godoc
// @Summary 更新库存
// @Description 更新指定菜品某日的库存数量
// @Tags 库存管理
// @Accept json
// @Produce json
// @Param request body updateDailyInventoryRequest true "库存更新信息"
// @Success 200 {object} dailyInventoryResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "库存不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory [put]
// @Security BearerAuth
func (server *Server) updateDailyInventory(ctx *gin.Context) {
	var req updateDailyInventoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format")))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证库存是否存在且属于该商户
	existing, err := server.store.GetDailyInventory(ctx, db.GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     req.DishID,
		Date:       pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("inventory not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构造更新参数
	params := db.UpdateDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     req.DishID,
		Date:       pgtype.Date{Time: date, Valid: true},
		TotalQuantity: pgtype.Int4{
			Int32: existing.TotalQuantity,
			Valid: true,
		},
		SoldQuantity: pgtype.Int4{
			Int32: existing.SoldQuantity,
			Valid: true,
		},
	}

	if req.TotalQuantity != nil {
		params.TotalQuantity = pgtype.Int4{
			Int32: *req.TotalQuantity,
			Valid: true,
		}
	}

	if req.SoldQuantity != nil {
		params.SoldQuantity = pgtype.Int4{
			Int32: *req.SoldQuantity,
			Valid: true,
		}
	}

	// 执行更新
	updated, err := server.store.UpdateDailyInventory(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, dailyInventoryResponse{
		ID:            updated.ID,
		MerchantID:    updated.MerchantID,
		DishID:        updated.DishID,
		Date:          updated.Date.Time.Format("2006-01-02"),
		TotalQuantity: updated.TotalQuantity,
		SoldQuantity:  updated.SoldQuantity,
		Available:     calculateAvailable(updated.TotalQuantity, updated.SoldQuantity),
	})
}

type checkInventoryRequest struct {
	DishID   int64  `json:"dish_id" binding:"required,min=1"`
	Date     string `json:"date" binding:"required"`
	Quantity int32  `json:"quantity" binding:"required,gt=0"`
}

type checkInventoryResponse struct {
	Available    bool   `json:"available"`
	CurrentStock int32  `json:"current_stock"`
	Message      string `json:"message,omitempty"`
}

// checkAndDecrementInventory godoc
// @Summary 检查并扣减库存
// @Description 检查库存是否充足并原子扣减（用于下单）
// @Tags 库存管理
// @Accept json
// @Produce json
// @Param request body checkInventoryRequest true "扣减请求"
// @Success 200 {object} checkInventoryResponse "检查结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory/check [post]
// @Security BearerAuth
func (server *Server) checkAndDecrementInventory(ctx *gin.Context) {
	var req checkInventoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 使用CheckAndDecrementInventory原子操作
	inventory, err := server.store.CheckAndDecrementInventory(ctx, db.CheckAndDecrementInventoryParams{
		MerchantID:   merchant.ID,
		DishID:       req.DishID,
		Date:         pgtype.Date{Time: date, Valid: true},
		SoldQuantity: req.Quantity,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有库存记录或库存不足
			existing, getErr := server.store.GetDailyInventory(ctx, db.GetDailyInventoryParams{
				MerchantID: merchant.ID,
				DishID:     req.DishID,
				Date:       pgtype.Date{Time: date, Valid: true},
			})
			if getErr != nil {
				if errors.Is(getErr, sql.ErrNoRows) {
					ctx.JSON(http.StatusOK, checkInventoryResponse{
						Available:    true,
						CurrentStock: -1,
						Message:      "unlimited inventory",
					})
					return
				}
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, getErr))
				return
			}
			ctx.JSON(http.StatusOK, checkInventoryResponse{
				Available:    false,
				CurrentStock: calculateAvailable(existing.TotalQuantity, existing.SoldQuantity),
				Message:      "insufficient inventory",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, checkInventoryResponse{
		Available:    true,
		CurrentStock: calculateAvailable(inventory.TotalQuantity, inventory.SoldQuantity),
		Message:      "success",
	})
}

type inventoryStatsResponse struct {
	TotalDishes     int64 `json:"total_dishes"`     // 总菜品数
	UnlimitedDishes int64 `json:"unlimited_dishes"` // 无限库存菜品数
	SoldOutDishes   int64 `json:"sold_out_dishes"`  // 已售罄菜品数
	AvailableDishes int64 `json:"available_dishes"` // 有库存菜品数
}

type getInventoryStatsRequest struct {
	Date string `form:"date" binding:"required"` // 日期 YYYY-MM-DD
}

// getInventoryStats godoc
// @Summary 获取库存统计
// @Description 获取商户某日的库存汇总数据
// @Tags 库存管理
// @Produce json
// @Param date query string true "日期(YYYY-MM-DD)"
// @Success 200 {object} inventoryStatsResponse "统计数据"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory/stats [get]
// @Security BearerAuth
func (server *Server) getInventoryStats(ctx *gin.Context) {
	var req getInventoryStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	stats, err := server.store.GetInventoryStats(ctx, db.GetInventoryStatsParams{
		MerchantID: merchant.ID,
		Date:       pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusOK, inventoryStatsResponse{})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, inventoryStatsResponse{
		TotalDishes:     stats.TotalDishes,
		UnlimitedDishes: stats.UnlimitedDishes,
		SoldOutDishes:   stats.SoldOutDishes,
		AvailableDishes: stats.AvailableDishes,
	})
}

type updateSingleInventoryUri struct {
	DishID int64 `uri:"dish_id" binding:"required,min=1"`
}

type updateSingleInventoryRequest struct {
	Date          string `json:"date" binding:"required"`
	TotalQuantity *int32 `json:"total_quantity" binding:"omitempty,gte=-1"`
	SoldQuantity  *int32 `json:"sold_quantity" binding:"omitempty,gte=0"`
}

// updateSingleInventory godoc
// @Summary 更新单品库存
// @Description 通过菜品ID更新指定菜品的库存
// @Tags 库存管理
// @Accept json
// @Produce json
// @Param dish_id path int true "菜品ID"
// @Param request body updateSingleInventoryRequest true "库存更新信息"
// @Success 200 {object} dailyInventoryResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "库存不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/inventory/{dish_id} [patch]
// @Security BearerAuth
func (server *Server) updateSingleInventory(ctx *gin.Context) {
	var uri updateSingleInventoryUri
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateSingleInventoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证库存是否存在且属于该商户
	existing, err := server.store.GetDailyInventory(ctx, db.GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     uri.DishID,
		Date:       pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("inventory not found for this dish")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构造更新参数
	params := db.UpdateDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     uri.DishID,
		Date:       pgtype.Date{Time: date, Valid: true},
		TotalQuantity: pgtype.Int4{
			Int32: existing.TotalQuantity,
			Valid: true,
		},
		SoldQuantity: pgtype.Int4{
			Int32: existing.SoldQuantity,
			Valid: true,
		},
	}

	if req.TotalQuantity != nil {
		params.TotalQuantity = pgtype.Int4{Int32: *req.TotalQuantity, Valid: true}
	}
	if req.SoldQuantity != nil {
		params.SoldQuantity = pgtype.Int4{Int32: *req.SoldQuantity, Valid: true}
	}

	// 执行更新
	updated, err := server.store.UpdateDailyInventory(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, dailyInventoryResponse{
		ID:            updated.ID,
		MerchantID:    updated.MerchantID,
		DishID:        updated.DishID,
		Date:          updated.Date.Time.Format("2006-01-02"),
		TotalQuantity: updated.TotalQuantity,
		SoldQuantity:  updated.SoldQuantity,
		Available:     calculateAvailable(updated.TotalQuantity, updated.SoldQuantity),
	})
}
