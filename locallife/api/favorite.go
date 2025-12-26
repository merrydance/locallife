package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 收藏 API ====================

type favoriteMerchantResponse struct {
	ID           int64  `json:"id"`
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	MerchantLogo string `json:"merchant_logo,omitempty"`
	Address      string `json:"address"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type favoriteDishResponse struct {
	ID           int64  `json:"id"`
	DishID       int64  `json:"dish_id"`
	DishName     string `json:"dish_name"`
	Description  string `json:"description,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	Price        int64  `json:"price"`
	MemberPrice  *int64 `json:"member_price,omitempty"`
	IsAvailable  bool   `json:"is_available"`
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	CreatedAt    string `json:"created_at"`
}

type addFavoriteMerchantRequest struct {
	MerchantID int64 `json:"merchant_id" binding:"required,min=1"`
}

// 分页请求结构体
type listFavoritesRequest struct {
	Page     int32 `form:"page" binding:"omitempty,min=1"`
	PageSize int32 `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// addFavoriteMerchant godoc
// @Summary 收藏商户
// @Description 将商户添加到用户的收藏列表
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param request body addFavoriteMerchantRequest true "商户ID"
// @Success 200 {object} map[string]interface{} "收藏成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/merchants [post]
// @Security BearerAuth
func (server *Server) addFavoriteMerchant(ctx *gin.Context) {
	var req addFavoriteMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户存在
	_, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_, err = server.store.AddFavoriteMerchant(ctx, db.AddFavoriteMerchantParams{
		UserID:     authPayload.UserID,
		MerchantID: pgtype.Int8{Int64: req.MerchantID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "merchant added to favorites",
		"merchant_id": req.MerchantID,
	})
}

// listFavoriteMerchants godoc
// @Summary 获取收藏的商户列表
// @Description 获取用户收藏的商户列表
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "收藏商户列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/merchants [get]
// @Security BearerAuth
func (server *Server) listFavoriteMerchants(ctx *gin.Context) {
	var req listFavoritesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchants, err := server.store.ListFavoriteMerchants(ctx, db.ListFavoriteMerchantsParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	count, err := server.store.CountFavoriteMerchants(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []favoriteMerchantResponse
	for _, m := range merchants {
		response = append(response, favoriteMerchantResponse{
			ID:           m.ID,
			MerchantID:   m.MerchantID,
			MerchantName: m.MerchantName,
			MerchantLogo: normalizeUploadURLForClient(m.MerchantLogo.String),
			Address:      m.MerchantAddress,
			Status:       m.MerchantStatus,
			CreatedAt:    m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if response == nil {
		response = []favoriteMerchantResponse{}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"merchants":  response,
		"total":      count,
		"page":       req.Page,
		"page_size":  req.PageSize,
		"total_page": (int(count) + int(req.PageSize) - 1) / int(req.PageSize),
	})
}

// deleteFavoriteMerchant godoc
// @Summary 取消收藏商户
// @Description 将商户从用户的收藏列表中移除
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param id path int64 true "商户ID"
// @Success 200 {object} map[string]interface{} "取消成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/merchants/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteFavoriteMerchant(ctx *gin.Context) {
	merchantIDStr := ctx.Param("id")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid merchant id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	err = server.store.RemoveFavoriteMerchant(ctx, db.RemoveFavoriteMerchantParams{
		UserID:     authPayload.UserID,
		MerchantID: pgtype.Int8{Int64: merchantID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "merchant removed from favorites",
		"merchant_id": merchantID,
	})
}

type addFavoriteDishRequest struct {
	DishID int64 `json:"dish_id" binding:"required,min=1"`
}

// addFavoriteDish godoc
// @Summary 收藏菜品
// @Description 将菜品添加到用户的收藏列表
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param request body addFavoriteDishRequest true "菜品ID"
// @Success 200 {object} map[string]interface{} "收藏成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/dishes [post]
// @Security BearerAuth
func (server *Server) addFavoriteDish(ctx *gin.Context) {
	var req addFavoriteDishRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证菜品存在
	_, err := server.store.GetDish(ctx, req.DishID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_, err = server.store.AddFavoriteDish(ctx, db.AddFavoriteDishParams{
		UserID: authPayload.UserID,
		DishID: pgtype.Int8{Int64: req.DishID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "dish added to favorites",
		"dish_id": req.DishID,
	})
}

// listFavoriteDishes godoc
// @Summary 获取收藏的菜品列表
// @Description 获取用户收藏的菜品列表
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{} "收藏菜品列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/dishes [get]
// @Security BearerAuth
func (server *Server) listFavoriteDishes(ctx *gin.Context) {
	var req listFavoritesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	dishes, err := server.store.ListFavoriteDishes(ctx, db.ListFavoriteDishesParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	count, err := server.store.CountFavoriteDishes(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []favoriteDishResponse
	for _, d := range dishes {
		item := favoriteDishResponse{
			ID:           d.ID,
			DishID:       d.DishID,
			DishName:     d.DishName,
			Description:  d.DishDescription.String,
			ImageURL:     normalizeUploadURLForClient(d.DishImageUrl.String),
			Price:        d.DishPrice,
			IsAvailable:  d.DishIsAvailable,
			MerchantID:   d.MerchantID,
			MerchantName: d.MerchantName,
			CreatedAt:    d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if d.DishMemberPrice.Valid {
			item.MemberPrice = &d.DishMemberPrice.Int64
		}
		response = append(response, item)
	}

	if response == nil {
		response = []favoriteDishResponse{}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"dishes":     response,
		"total":      count,
		"page":       req.Page,
		"page_size":  req.PageSize,
		"total_page": (int(count) + int(req.PageSize) - 1) / int(req.PageSize),
	})
}

// deleteFavoriteDish godoc
// @Summary 取消收藏菜品
// @Description 将菜品从用户的收藏列表中移除
// @Tags 收藏管理
// @Accept json
// @Produce json
// @Param id path int64 true "菜品ID"
// @Success 200 {object} map[string]interface{} "取消成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/favorites/dishes/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteFavoriteDish(ctx *gin.Context) {
	dishIDStr := ctx.Param("id")
	dishID, err := strconv.ParseInt(dishIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid dish id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	err = server.store.RemoveFavoriteDish(ctx, db.RemoveFavoriteDishParams{
		UserID: authPayload.UserID,
		DishID: pgtype.Int8{Int64: dishID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "dish removed from favorites",
		"dish_id": dishID,
	})
}
