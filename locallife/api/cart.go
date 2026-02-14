package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"
)

// ==================== 购物车 API ====================
type cartItemResponse struct {
	ID             int64                  `json:"id"`
	DishID         *int64                 `json:"dish_id,omitempty"`
	ComboID        *int64                 `json:"combo_id,omitempty"`
	Quantity       int16                  `json:"quantity"`
	Customizations map[string]interface{} `json:"customizations,omitempty"`
	// 解析后的定制规格详情
	CustomizationDetails []orderCustomizationItem `json:"customization_details,omitempty"`
	// 聚合好的规格描述文字 (如 "不辣/大份")
	SpecText string `json:"spec_text,omitempty"`
	// 套餐成员图片
	ComboMemberImages []string `json:"combo_member_images,omitempty"`
	// 商品信息
	Name        string `json:"name"`
	ImageURL    string `json:"image_url,omitempty"`
	UnitPrice   int64  `json:"unit_price"`
	MemberPrice *int64 `json:"member_price,omitempty"`
	IsAvailable bool   `json:"is_available"`
	Subtotal    int64  `json:"subtotal"`
}

type cartResponse struct {
	ID            int64              `json:"id"`
	MerchantID    int64              `json:"merchant_id"`
	OrderType     string             `json:"order_type"`
	TableID       *int64             `json:"table_id,omitempty"`
	ReservationID *int64             `json:"reservation_id,omitempty"`
	Items         []cartItemResponse `json:"items"`
	TotalCount    int                `json:"total_count"`
	Subtotal      int64              `json:"subtotal"`
}

// getCart godoc
// @Summary 获取购物车
// @Description 获取指定商户的购物车内容
// @Tags 购物车
// @Accept json
// @Produce json
// @Param merchant_id query int64 true "商户ID"
// @Success 200 {object} cartResponse "购物车内容"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart [get]
// @Security BearerAuth
func (server *Server) getCart(ctx *gin.Context) {
	merchantIDStr := ctx.Query("merchant_id")
	if merchantIDStr == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant_id is required")))
		return
	}

	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid merchant_id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	orderType := ctx.DefaultQuery("order_type", "takeout")
	var tableID, reservationID pgtype.Int8

	if tidStr := ctx.Query("table_id"); tidStr != "" {
		if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil {
			tableID = pgtype.Int8{Int64: tid, Valid: true}
		}
	}
	if ridStr := ctx.Query("reservation_id"); ridStr != "" {
		if rid, err := strconv.ParseInt(ridStr, 10, 64); err == nil {
			reservationID = pgtype.Int8{Int64: rid, Valid: true}
		}
	}

	cart, err := server.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        authPayload.UserID,
		MerchantID:    merchantID,
		OrderType:     orderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if isNotFoundError(err) {
			// 购物车不存在，返回空购物车
			ctx.JSON(http.StatusOK, cartResponse{
				MerchantID: merchantID,
				OrderType:  orderType,
				Items:      []cartItemResponse{},
				TotalCount: 0,
				Subtotal:   0,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart by user and merchant: %w", err)))
		return
	}

	// 获取购物车商品列表
	items, err := server.store.ListCartItems(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list cart items: %w", err)))
		return
	}

	logicResp := logic.BuildCartResponse(cart, items, normalizeUploadURLForClient)
	resp := toCartResponse(logicResp)
	server.enrichCartItems(ctx, resp.Items)
	server.enrichComboImages(ctx, resp.Items)
	ctx.JSON(http.StatusOK, resp)
}

func toCartResponse(logicResp logic.CartResponse) cartResponse {
	resp := cartResponse{
		ID:            logicResp.ID,
		MerchantID:    logicResp.MerchantID,
		OrderType:     logicResp.OrderType,
		TableID:       logicResp.TableID,
		ReservationID: logicResp.ReservationID,
		TotalCount:    logicResp.TotalCount,
		Subtotal:      logicResp.Subtotal,
	}
	resp.Items = make([]cartItemResponse, 0, len(logicResp.Items))
	for _, item := range logicResp.Items {
		resp.Items = append(resp.Items, cartItemResponse{
			ID:             item.ID,
			DishID:         item.DishID,
			ComboID:        item.ComboID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
			Name:           item.Name,
			ImageURL:       item.ImageURL,
			UnitPrice:      item.UnitPrice,
			MemberPrice:    item.MemberPrice,
			IsAvailable:    item.IsAvailable,
			Subtotal:       item.Subtotal,
		})
	}
	return resp
}

type addCartItemRequest struct {
	// 商户ID (必填)
	MerchantID int64 `json:"merchant_id" binding:"required,min=1"`
	// 订单类型 (选填，默认为 takeout)
	OrderType string `json:"order_type"`
	// 桌台ID (堂食时必填)
	TableID *int64 `json:"table_id"`
	// 预约ID (预约时必填)
	ReservationID *int64 `json:"reservation_id"`
	// 菜品ID (dish_id和combo_id二选一)
	DishID *int64 `json:"dish_id" binding:"omitempty,min=1"`
	// 套餐ID (dish_id和combo_id二选一)
	ComboID *int64 `json:"combo_id" binding:"omitempty,min=1"`
	// 数量 (必填，范围：1-99)
	Quantity int16 `json:"quantity" binding:"required,min=1,max=99"`
	// 定制选项 (选填)
	Customizations map[string]interface{} `json:"customizations"`
}

// addCartItem godoc
// @Summary 添加商品到购物车
// @Description 添加菜品或套餐到购物车，dish_id和combo_id二选一
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body addCartItemRequest true "商品信息"
// @Success 200 {object} cartResponse "更新后的购物车"
// @Failure 400 {object} ErrorResponse "请求参数错误/商品不可售"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户/商品不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/items [post]
// @Security BearerAuth
func (server *Server) addCartItem(ctx *gin.Context) {
	var req addCartItemRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证必须提供 dish_id 或 combo_id 之一
	if req.DishID == nil && req.ComboID == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("dish_id or combo_id is required")))
		return
	}
	if req.DishID != nil && req.ComboID != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("cannot specify both dish_id and combo_id")))
		return
	}

	if req.OrderType == "" {
		req.OrderType = "takeout"
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.AddCartItem(ctx, server.store, logic.AddCartItemInput{
		UserID:         authPayload.UserID,
		MerchantID:     req.MerchantID,
		OrderType:      req.OrderType,
		TableID:        req.TableID,
		ReservationID:  req.ReservationID,
		DishID:         req.DishID,
		ComboID:        req.ComboID,
		Quantity:       req.Quantity,
		Customizations: req.Customizations,
		MaxQuantity:    CartItemMaxQuantity,
		NormalizeCustomizings: func(ctx context.Context, dishID int64, customizations map[string]interface{}) (map[string]interface{}, error) {
			ginCtx, ok := ctx.(*gin.Context)
			if !ok {
				return nil, errors.New("invalid context")
			}
			_, _, normalized, err := server.normalizeDishCustomizations(ginCtx, dishID, customizations)
			return normalized, err
		},
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.returnUpdatedCart(ctx, result.Cart)
}

func (server *Server) returnUpdatedCart(ctx *gin.Context, cart db.Cart) {
	items, err := server.store.ListCartItems(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list cart items: %w", err)))
		return
	}
	logicResp := logic.BuildCartResponse(cart, items, normalizeUploadURLForClient)
	response := toCartResponse(logicResp)
	server.enrichCartItems(ctx, response.Items)
	server.enrichComboImages(ctx, response.Items)
	ctx.JSON(http.StatusOK, response)
}

type updateCartItemRequest struct {
	// 数量 (选填，范围：1-99)
	Quantity *int16 `json:"quantity" binding:"omitempty,min=1,max=99"`
	// 定制选项 (选填)
	Customizations map[string]interface{} `json:"customizations"`
}

// updateCartItem godoc
// @Summary 更新购物车商品
// @Description 更新购物车商品的数量或定制选项
// @Tags 购物车
// @Accept json
// @Produce json
// @Param id path int64 true "购物车商品ID"
// @Param request body updateCartItemRequest true "更新内容"
// @Success 200 {object} cartResponse "更新后的购物车"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "商品不属于当前用户的购物车"
// @Failure 404 {object} ErrorResponse "商品不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/items/{id} [patch]
// @Security BearerAuth
func (server *Server) updateCartItem(ctx *gin.Context) {
	itemIDStr := ctx.Param("id")
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid item id")))
		return
	}

	var req updateCartItemRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.UpdateCartItem(ctx, server.store, logic.UpdateCartItemInput{
		UserID:         authPayload.UserID,
		ItemID:         itemID,
		Quantity:       req.Quantity,
		Customizations: req.Customizations,
		MaxQuantity:    CartItemMaxQuantity,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.returnUpdatedCart(ctx, result.Cart)
}

// deleteCartItem godoc
// @Summary 删除购物车商品
// @Description 从购物车中移除指定商品
// @Tags 购物车
// @Accept json
// @Produce json
// @Param id path int64 true "购物车商品ID"
// @Success 200 {object} cartResponse "更新后的购物车"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "商品不属于当前用户的购物车"
// @Failure 404 {object} ErrorResponse "商品不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/items/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteCartItem(ctx *gin.Context) {
	itemIDStr := ctx.Param("id")
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid item id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取购物车商品
	cartItem, err := server.store.GetCartItem(ctx, itemID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("cart item not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart item: %w", err)))
		return
	}

	// P0安全：通过cart_id获取购物车，验证所有权
	cart, err := server.store.GetCart(ctx, cartItem.CartID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("cart not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart: %w", err)))
		return
	}

	// P0安全：验证购物车属于当前用户
	if cart.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("cart item does not belong to you")))
		return
	}

	err = server.store.DeleteCartItem(ctx, itemID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("delete cart item: %w", err)))
		return
	}

	// 返回更新后的购物车
	server.returnUpdatedCart(ctx, cart)
}

type clearCartRequest struct {
	// 商户ID (必填)
	MerchantID int64 `json:"merchant_id" binding:"required,min=1"`
	// 订单类型
	OrderType string `json:"order_type"`
	// 桌台ID
	TableID *int64 `json:"table_id"`
	// 预约ID
	ReservationID *int64 `json:"reservation_id"`
}

// clearCart godoc
// @Summary 清空购物车
// @Description 清空指定商户购物车中的所有商品
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body clearCartRequest true "清空购物车请求"
// @Success 200 {object} cartResponse "清空后的购物车"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/clear [post]
// @Security BearerAuth
func (server *Server) clearCart(ctx *gin.Context) {
	var req clearCartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if req.OrderType == "" {
		req.OrderType = "takeout"
	}

	var tableID, reservationID pgtype.Int8
	if req.TableID != nil {
		tableID = pgtype.Int8{Int64: *req.TableID, Valid: true}
	}
	if req.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *req.ReservationID, Valid: true}
	}

	cart, err := server.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        authPayload.UserID,
		MerchantID:    req.MerchantID,
		OrderType:     req.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if isNotFoundError(err) {
			// 购物车不存在，返回空购物车
			ctx.JSON(http.StatusOK, cartResponse{
				MerchantID: req.MerchantID,
				OrderType:  req.OrderType,
				Items:      []cartItemResponse{},
				TotalCount: 0,
				Subtotal:   0,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart by user and merchant: %w", err)))
		return
	}

	err = server.store.ClearCart(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("clear cart: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, cartResponse{
		ID:            cart.ID,
		MerchantID:    req.MerchantID,
		OrderType:     req.OrderType,
		TableID:       req.TableID,
		ReservationID: req.ReservationID,
		Items:         []cartItemResponse{},
		TotalCount:    0,
		Subtotal:      0,
	})
}

type calculateCartRequest struct {
	// 商户ID (必填)
	MerchantID int64 `json:"merchant_id" binding:"required,min=1"`
	// 订单类型
	OrderType string `json:"order_type"`
	// 桌台ID
	TableID *int64 `json:"table_id"`
	// 预约ID
	ReservationID *int64 `json:"reservation_id"`
	// 配送地址ID (选填，用于计算配送费)
	AddressID *int64 `json:"address_id" binding:"omitempty,min=1"`
	// 用户当前位置纬度 (选填，当无地址时作为fallback)
	Latitude *float64 `json:"latitude" binding:"omitempty"`
	// 用户当前位置经度 (选填，当无地址时作为fallback)
	Longitude *float64 `json:"longitude" binding:"omitempty"`
	// 优惠券ID (选填，用于计算优惠)
	VoucherID *int64 `json:"voucher_id" binding:"omitempty,min=1"`
}

type calculateCartResponse struct {
	// 商品小计（分）
	Subtotal int64 `json:"subtotal"`
	// 配送费（分）
	DeliveryFee int64 `json:"delivery_fee"`
	// 配送费满返减免（分）
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount"`
	// 配送距离（米），仅当成功计算时返回
	DeliveryDistance int32 `json:"delivery_distance,omitempty"`
	// 预计送达总时长（分钟），包含出餐、骑手到店、配送、缓冲
	DeliveryEtaMinutes int32 `json:"delivery_eta_minutes,omitempty"`
	// 出餐时间（分钟）
	PrepareMinutes int32 `json:"prepare_minutes,omitempty"`
	// 骑手到店时间（分钟）
	RiderToStoreMinutes int32 `json:"rider_to_store_minutes,omitempty"`
	// 店到用户路网时间（分钟）
	StoreToUserMinutes int32 `json:"store_to_user_minutes,omitempty"`
	// 额外缓冲时间（分钟）
	BufferMinutes int32 `json:"buffer_minutes,omitempty"`
	// 优惠券减免金额（分）
	DiscountAmount int64 `json:"discount_amount"`
	// 实付金额（分）
	TotalAmount int64 `json:"total_amount"`
	// 优惠说明
	DiscountInfo string `json:"discount_info,omitempty"`
	// 已应用的优惠明细
	AppliedPromotions []logic.AppliedPromotion `json:"applied_promotions,omitempty"`
	// 推荐可用优惠券（仅试算，不自动使用）
	SuggestedVoucher *logic.SuggestedVoucher `json:"suggested_voucher,omitempty"`
	// 阶梯优惠试算信息
	LadderPromotions []logic.LadderPromotion `json:"ladder_promotions,omitempty"`
	// 代金券试算信息
	VoucherTrials []logic.VoucherTrial `json:"voucher_trials,omitempty"`
	// 会员余额支付能力评估
	PaymentAssessment logic.PaymentAssessment `json:"payment_assessment"`
	// 最小起送金额（分），0表示无限制
	MinOrderAmount int64 `json:"min_order_amount"`
	// 是否满足起送金额
	MeetsMinOrder bool `json:"meets_min_order"`
}

// calculateCart godoc
// @Summary 计算购物车金额
// @Description 计算购物车总金额，包括商品小计、配送费、优惠减免等。可选传入地址ID计算真实配送费，传入优惠券ID计算优惠减免
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body calculateCartRequest true "计算请求"
// @Success 200 {object} calculateCartResponse "计算结果"
// @Failure 400 {object} ErrorResponse "请求参数错误/购物车为空"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/calculate [post]
// @Security BearerAuth
func (server *Server) calculateCart(ctx *gin.Context) {
	var req calculateCartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 记录输入参数，便于排查配送费计算问题（不含用户敏感信息）
	log.Info().
		Int64("merchant_id", req.MerchantID).
		Str("order_type", req.OrderType).
		Interface("address_id", req.AddressID).
		Interface("lat", req.Latitude).
		Interface("lng", req.Longitude).
		Msg("calculateCart called")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息（用于获取region_id和min_order_amount）
	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant: %w", err)))
		return
	}
	if req.OrderType == "takeout" {
		if merchant.Status != "active" || !merchant.IsOpen {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("商户已打烊")))
			return
		}
	}

	if req.OrderType == "" {
		req.OrderType = "takeout"
	}

	var tableID, reservationID pgtype.Int8
	if req.TableID != nil {
		tableID = pgtype.Int8{Int64: *req.TableID, Valid: true}
	}
	if req.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *req.ReservationID, Valid: true}
	}

	cart, err := server.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        authPayload.UserID,
		MerchantID:    req.MerchantID,
		OrderType:     req.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("购物车为空")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart by user and merchant: %w", err)))
		return
	}

	items, err := server.store.ListCartItems(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list cart items: %w", err)))
		return
	}

	if len(items) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("购物车为空")))
		return
	}

	// 计算商品小计
	var subtotal int64
	for _, item := range items {
		var unitPrice int64
		if item.DishID.Valid {
			unitPrice = item.DishPrice.Int64
		} else if item.ComboID.Valid {
			unitPrice = item.ComboPrice.Int64
		}
		subtotal += unitPrice * int64(item.Quantity)
	}

	// 获取起送金额（从配送费配置中获取，或使用默认值0表示无限制）
	var minOrderAmount int64
	meetsMinOrder := true
	// 起送金额暂不实现，后续可从商户配置或配送费配置中获取

	// 计算配送费（如果提供了地址或坐标）
	var deliveryFee int64
	var deliveryFeeDiscount int64
	var deliveryDistance int32
	var routeDurationSec int
	var eta logic.DeliveryETAResult
	if req.AddressID != nil {
		address, err := server.store.GetUserAddress(ctx, *req.AddressID)
		if err != nil || address.UserID != authPayload.UserID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("地址无效")))
			return
		}
		if !address.Latitude.Valid || !address.Longitude.Valid || !merchant.Latitude.Valid || !merchant.Longitude.Valid {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法获取距离，请重新选择地址")))
			return
		}
		if server.mapClient == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法计算配送距离，请稍后重试")))
			return
		}
		fromLoc := maps.Location{Lat: pgNumericToFloat64(merchant.Latitude), Lng: pgNumericToFloat64(merchant.Longitude)}
		toLoc := maps.Location{Lat: pgNumericToFloat64(address.Latitude), Lng: pgNumericToFloat64(address.Longitude)}
		routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
		if err != nil || routeResult == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("距离计算失败，请重新选择位置")))
			return
		}
		distance := int32(routeResult.Distance)
		deliveryDistance = distance
		feeResult, err := server.calculateDeliveryFeeInternal(
			ctx,
			merchant.RegionID,
			merchant.ID,
			distance,
			subtotal,
		)
		if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
			deliveryFee = feeResult.FinalFee
			deliveryFeeDiscount = feeResult.PromotionDiscount
			routeDurationSec = routeResult.Duration
		}
	} else if req.Latitude != nil && req.Longitude != nil {
		if !merchant.Latitude.Valid || !merchant.Longitude.Valid {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法获取距离，请重新选择位置")))
			return
		}
		if server.mapClient == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法计算配送距离，请稍后重试")))
			return
		}
		fromLoc := maps.Location{Lat: pgNumericToFloat64(merchant.Latitude), Lng: pgNumericToFloat64(merchant.Longitude)}
		toLoc := maps.Location{Lat: *req.Latitude, Lng: *req.Longitude}
		routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
		if err != nil || routeResult == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("距离计算失败，请重新选择位置")))
			return
		}
		distance := int32(routeResult.Distance)
		deliveryDistance = distance
		feeResult, err := server.calculateDeliveryFeeInternal(
			ctx,
			merchant.RegionID,
			merchant.ID,
			distance,
			subtotal,
		)
		if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
			deliveryFee = feeResult.FinalFee
			deliveryFeeDiscount = feeResult.PromotionDiscount
			routeDurationSec = routeResult.Duration
		}
	}

	// 计算预计送达时间：出餐 + 骑手到店 + 店到用户 + 缓冲
	eta = logic.ComputeDeliveryETA(ctx, server.store, merchant.ID, deliveryDistance, routeDurationSec)

	// 使用统一优惠引擎计算最终价格
	engine := logic.NewPromotionEngine(server.store)
	calcResult, err := engine.CalculateFinalPrice(ctx, logic.OrderContext{
		MerchantID:          merchant.ID,
		UserID:              authPayload.UserID,
		OrderType:           req.OrderType,
		Subtotal:            subtotal,
		VoucherID:           req.VoucherID,
		DeliveryFee:         deliveryFee,
		DeliveryFeeDiscount: deliveryFeeDiscount,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("calculate final price: %w", err)))
		return
	}

	log.Info().
		Int64("merchant_id", req.MerchantID).
		Str("order_type", req.OrderType).
		Int32("delivery_distance", deliveryDistance).
		Int64("total_amount", calcResult.TotalAmount).
		Msg("calculateCart result from engine")

	ctx.JSON(http.StatusOK, calculateCartResponse{
		Subtotal:            calcResult.Subtotal,
		DeliveryFee:         calcResult.DeliveryFee,
		DeliveryFeeDiscount: calcResult.DeliveryFeeDiscount,
		DeliveryDistance:    deliveryDistance,
		DeliveryEtaMinutes:  eta.DeliveryEtaMinutes,
		PrepareMinutes:      eta.PrepareMinutes,
		RiderToStoreMinutes: eta.RiderToStoreMinutes,
		StoreToUserMinutes:  eta.StoreToUserMinutes,
		BufferMinutes:       eta.BufferMinutes,
		DiscountAmount:      calcResult.VoucherDiscount + calcResult.MerchantDiscount,
		TotalAmount:         calcResult.TotalAmount,
		DiscountInfo:        "", // 具体的明细已在 AppliedPromotions 中
		AppliedPromotions:   calcResult.AppliedPromotions,
		SuggestedVoucher:    calcResult.SuggestedVoucher,
		LadderPromotions:    calcResult.LadderPromotions,
		VoucherTrials:       calcResult.VoucherTrials,
		PaymentAssessment:   calcResult.PaymentAssessment,
		MinOrderAmount:      minOrderAmount,
		MeetsMinOrder:       meetsMinOrder,
	})
}

// ==================== 浏览历史 API ====================

type browseHistoryItem struct {
	// 浏览记录ID
	ID int64 `json:"id"`
	// 浏览目标类型：merchant=商户, dish=菜品
	TargetType string `json:"target_type"`
	// 目标ID
	TargetID int64 `json:"target_id"`
	// 目标名称
	Name string `json:"name,omitempty"`
	// 目标图片URL
	ImageURL string `json:"image_url,omitempty"`
	// 浏览次数
	ViewCount int32 `json:"view_count"`
	// 最后浏览时间
	LastViewedAt string `json:"last_viewed_at"`
}

type listBrowseHistoryResponse struct {
	// 浏览记录列表
	Items []browseHistoryItem `json:"items"`
	// 总数
	Total      int64 `json:"total"`
	TotalCount int64 `json:"total_count"`
	PageID     int   `json:"page_id"`
	PageSize   int   `json:"page_size"`
}

// listBrowseHistory godoc
// @Summary 获取浏览历史
// @Description 获取用户的浏览历史记录，可按类型筛选
// @Tags 浏览历史
// @Accept json
// @Produce json
// @Param type query string false "筛选类型" Enums(merchant, dish)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} listBrowseHistoryResponse "浏览历史列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/history/browse [get]
// @Security BearerAuth
func (server *Server) listBrowseHistory(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	targetType := ctx.Query("type")
	// 验证type参数
	if targetType != "" && targetType != "merchant" && targetType != "dish" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("type参数只能是merchant或dish")))
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var items []db.BrowseHistory
	var total int64
	var err error

	if targetType != "" {
		items, err = server.store.ListBrowseHistoryByType(ctx, db.ListBrowseHistoryByTypeParams{
			UserID:     authPayload.UserID,
			TargetType: targetType,
			Limit:      int32(pageSize),
			Offset:     pageOffset(int32(page), int32(pageSize)),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list browse history by type: %w", err)))
			return
		}
		total, err = server.store.CountBrowseHistoryByType(ctx, db.CountBrowseHistoryByTypeParams{
			UserID:     authPayload.UserID,
			TargetType: targetType,
		})
	} else {
		items, err = server.store.ListBrowseHistory(ctx, db.ListBrowseHistoryParams{
			UserID: authPayload.UserID,
			Limit:  int32(pageSize),
			Offset: pageOffset(int32(page), int32(pageSize)),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list browse history: %w", err)))
			return
		}
		total, err = server.store.CountBrowseHistory(ctx, authPayload.UserID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("count browse history: %w", err)))
		return
	}

	// 获取详细信息
	result := make([]browseHistoryItem, len(items))
	for i, item := range items {
		historyItem := browseHistoryItem{
			ID:           item.ID,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			ViewCount:    item.ViewCount,
			LastViewedAt: item.LastViewedAt.Format("2006-01-02 15:04:05"),
		}

		// 根据类型获取名称和图片
		switch item.TargetType {
		case "merchant":
			merchant, err := server.store.GetMerchant(ctx, item.TargetID)
			if err == nil {
				historyItem.Name = merchant.Name
				historyItem.ImageURL = normalizeUploadURLForClient(merchant.LogoUrl.String)
			}
		case "dish":
			dish, err := server.store.GetDish(ctx, item.TargetID)
			if err == nil {
				historyItem.Name = dish.Name
				historyItem.ImageURL = normalizeUploadURLForClient(dish.ImageUrl.String)
			}
		}

		result[i] = historyItem
	}

	ctx.JSON(http.StatusOK, listBrowseHistoryResponse{
		Items:      result,
		Total:      total,
		TotalCount: total,
		PageID:     page,
		PageSize:   pageSize,
	})
}

// ==================== 多商户购物车汇总 API ====================

type cartSummaryResponse struct {
	// 购物车数量（商户数）
	CartCount int `json:"cart_count"`
	// 商品总数
	TotalItems int `json:"total_items"`
	// 商品总金额（分）
	TotalAmount int64 `json:"total_amount"`
}

type merchantCartResponse struct {
	// 购物车ID
	CartID int64 `json:"cart_id"`
	// 商户ID
	MerchantID int64 `json:"merchant_id"`
	// 订单类型
	OrderType string `json:"order_type"`
	// 桌台ID
	TableID int64 `json:"table_id,omitempty"`
	// 预约ID
	ReservationID int64 `json:"reservation_id,omitempty"`
	// 商户名称
	MerchantName string `json:"merchant_name"`
	// 商户Logo URL
	MerchantLogo string `json:"merchant_logo,omitempty"`
	// 商品数量
	ItemCount int `json:"item_count"`
	// 商品小计（分）
	Subtotal int64 `json:"subtotal"`
	// 所有商品是否可购买
	AllAvailable bool `json:"all_available"`
}

type userCartsResponse struct {
	// 汇总统计
	Summary cartSummaryResponse `json:"summary"`
	// 各商户购物车列表
	Carts []merchantCartResponse `json:"carts"`
}

// getUserCarts godoc
// @Summary 获取用户所有购物车汇总
// @Description 获取用户在所有商户的购物车汇总信息，用于显示购物车角标和合单结算入口
// @Tags 购物车
// @Accept json
// @Produce json
// @Success 200 {object} userCartsResponse "购物车汇总"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/summary [get]
// @Security BearerAuth
func (server *Server) getUserCartsSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	orderType := ctx.Query("order_type")
	fmt.Printf("[DEBUG] getUserCartsSummary: user_id=%d, order_type=%s\n", authPayload.UserID, orderType)

	argSummary := db.GetUserCartsSummaryParams{
		UserID: authPayload.UserID,
		OrderType: pgtype.Text{
			String: orderType,
			Valid:  orderType != "",
		},
	}

	// 获取汇总统计
	summary, err := server.store.GetUserCartsSummary(ctx, argSummary)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user carts summary: %w", err)))
		return
	}

	argDetails := db.GetUserCartsWithDetailsParams{
		UserID: authPayload.UserID,
		OrderType: pgtype.Text{
			String: orderType,
			Valid:  orderType != "",
		},
	}

	// 获取各商户购物车详情
	carts, err := server.store.GetUserCartsWithDetails(ctx, argDetails)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user carts with details: %w", err)))
		return
	}

	// 构建响应
	cartList := make([]merchantCartResponse, len(carts))
	for i, cart := range carts {
		cartList[i] = merchantCartResponse{
			CartID:        cart.CartID,
			MerchantID:    cart.MerchantID,
			OrderType:     cart.OrderType,
			TableID:       cart.TableID.Int64,
			ReservationID: cart.ReservationID.Int64,
			MerchantName:  cart.MerchantName,
			MerchantLogo:  normalizeUploadURLForClient(cart.MerchantLogo.String),
			ItemCount:     int(cart.ItemCount),
			Subtotal:      cart.Subtotal,
			AllAvailable:  cart.AllAvailable,
		}
	}

	ctx.JSON(http.StatusOK, userCartsResponse{
		Summary: cartSummaryResponse{
			CartCount:   int(summary.CartCount),
			TotalItems:  int(summary.TotalItems),
			TotalAmount: summary.TotalAmount,
		},
		Carts: cartList,
	})
}

// ==================== 合单结算 API ====================

type combinedCheckoutRequest struct {
	// 要结算的购物车ID列表（必填，最多10个）
	CartIDs []int64 `json:"cart_ids" binding:"required,min=1,max=10"`
	// 配送地址ID（外卖时必填）
	AddressID *int64 `json:"address_id" binding:"omitempty,min=1"`
}

type combinedCheckoutItem struct {
	// 商户ID
	MerchantID int64 `json:"merchant_id"`
	// 商户名称
	MerchantName string `json:"merchant_name"`
	// 订单类型
	OrderType string `json:"order_type"`
	// 商品小计（分）
	Subtotal int64 `json:"subtotal"`
	// 配送费（分）
	DeliveryFee int64 `json:"delivery_fee"`
	// 小计+配送费（分）
	TotalAmount int64 `json:"total_amount"`
}

type combinedCheckoutResponse struct {
	Items            []combinedCheckoutItem `json:"items"`
	TotalSubtotal    int64                  `json:"total_subtotal"`
	TotalDeliveryFee int64                  `json:"total_delivery_fee"`
	TotalAmount      int64                  `json:"total_amount"`
	CanCombinePay    bool                   `json:"can_combine_pay"`
	Message          string                 `json:"message,omitempty"`
}

// previewCombinedCheckout godoc
// @Summary 合单结算预览
// @Description 预览多购物车合单结算信息
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body combinedCheckoutRequest true "合单结算预览请求"
// @Success 200 {object} combinedCheckoutResponse "合单结算预览结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/combined-checkout/preview [post]
// @Security BearerAuth
func (server *Server) previewCombinedCheckout(ctx *gin.Context) {
	var req combinedCheckoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	carts, err := server.store.GetUserCartsByCartIDs(ctx, db.GetUserCartsByCartIDsParams{
		UserID:  authPayload.UserID,
		Column2: req.CartIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if len(carts) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("购物车为空")))
		return
	}

	items := make([]combinedCheckoutItem, 0, len(carts))
	var totalSubtotal int64
	var totalDeliveryFee int64
	var totalAmount int64
	canCombinePay := true
	message := ""

	for _, cart := range carts {
		if cart.MerchantStatus != "active" || !cart.SubMchid.Valid || cart.SubMchid.String == "" {
			canCombinePay = false
			message = "部分商户暂不支持在线支付"
		}

		cartDetail, err := server.store.GetCart(ctx, cart.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		cartItems, err := server.store.ListCartItems(ctx, cart.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list cart items: %w", err)))
			return
		}

		var subtotal int64
		for _, item := range cartItems {
			var unitPrice int64
			if item.DishID.Valid {
				unitPrice = item.DishPrice.Int64
			} else if item.ComboID.Valid {
				unitPrice = item.ComboPrice.Int64
			}
			subtotal += unitPrice * int64(item.Quantity)
		}

		var deliveryFee int64
		if cartDetail.OrderType == "takeout" {
			if req.AddressID == nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("外卖合单需要选择配送地址")))
				return
			}
			address, err := server.store.GetUserAddress(ctx, *req.AddressID)
			if err != nil || address.UserID != authPayload.UserID {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("地址无效")))
				return
			}
			if !address.Latitude.Valid || !address.Longitude.Valid || !cart.MerchantLatitude.Valid || !cart.MerchantLongitude.Valid {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法获取配送距离，请重新选择地址")))
				return
			}
			if server.mapClient == nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无法计算配送距离，请稍后重试")))
				return
			}
			fromLoc := maps.Location{Lat: pgNumericToFloat64(cart.MerchantLatitude), Lng: pgNumericToFloat64(cart.MerchantLongitude)}
			toLoc := maps.Location{Lat: pgNumericToFloat64(address.Latitude), Lng: pgNumericToFloat64(address.Longitude)}
			routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
			if err != nil || routeResult == nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("距离计算失败，请重新选择位置")))
				return
			}
			distance := int32(routeResult.Distance)
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, cart.RegionID, cart.MerchantID, distance, subtotal)
			if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
				deliveryFee = feeResult.FinalFee
			}
		}

		items = append(items, combinedCheckoutItem{
			MerchantID:   cart.MerchantID,
			MerchantName: cart.MerchantName,
			OrderType:    cartDetail.OrderType,
			Subtotal:     subtotal,
			DeliveryFee:  deliveryFee,
			TotalAmount:  subtotal + deliveryFee,
		})

		totalSubtotal += subtotal
		totalDeliveryFee += deliveryFee
		totalAmount += subtotal + deliveryFee
	}

	ctx.JSON(http.StatusOK, combinedCheckoutResponse{
		Items:            items,
		TotalSubtotal:    totalSubtotal,
		TotalDeliveryFee: totalDeliveryFee,
		TotalAmount:      totalAmount,
		CanCombinePay:    canCombinePay,
		Message:          message,
	})
}

// enrichCartItems 从数据库解析定制选项的 ID 并填充显示文字
func (server *Server) enrichCartItems(ctx context.Context, items []cartItemResponse) {
	if len(items) == 0 {
		return
	}

	// 收集所有需要解析的选项 ID
	optionIDMap := make(map[int64]bool)
	for _, item := range items {
		for _, val := range item.Customizations {
			var id int64
			switch v := val.(type) {
			case string:
				id, _ = strconv.ParseInt(v, 10, 64)
			case float64:
				id = int64(v)
			case int64:
				id = v
			case int:
				id = int64(v)
			}
			if id > 0 {
				optionIDMap[id] = true
			}
		}
	}

	if len(optionIDMap) == 0 {
		return
	}

	uniqueIDs := make([]int64, 0, len(optionIDMap))
	for id := range optionIDMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	// 批量查询库中的名称和价格
	details, err := server.store.GetCustomizationDetailsByIDs(ctx, uniqueIDs)
	if err != nil {
		log.Error().Err(err).Msg("failed to get customization details for cart items")
		return
	}

	// 映射结果
	detailLookup := make(map[int64]db.GetCustomizationDetailsByIDsRow)
	for _, d := range details {
		detailLookup[d.OptionID] = d
	}

	// 填充详情
	for i := range items {
		var specNames []string
		// 为了保持顺序，我们按原 Customizations 的 key 排序（或保持原顺序）
		// 不过 Customizations 是 map，本身无序。

		// 收集并转换
		for _, val := range items[i].Customizations {
			var id int64
			switch v := val.(type) {
			case string:
				id, _ = strconv.ParseInt(v, 10, 64)
			case float64:
				id = int64(v)
			case int64:
				id = v
			}

			if d, ok := detailLookup[id]; ok {
				items[i].CustomizationDetails = append(items[i].CustomizationDetails, orderCustomizationItem{
					GroupID:    d.GroupID,
					OptionID:   d.OptionID,
					Name:       d.GroupName,
					Value:      d.TagName,
					ExtraPrice: d.ExtraPrice,
				})
				specNames = append(specNames, d.TagName)
			}
		}

		if len(specNames) > 0 {
			items[i].SpecText = strings.Join(specNames, "/")
		} else {
			items[i].SpecText = "" // 明确为空，方便前端判断
		}
	}
}

// enrichComboImages 为套餐商品填充成员图片
func (server *Server) enrichComboImages(ctx context.Context, items []cartItemResponse) {
	if len(items) == 0 {
		return
	}

	comboIDs := make([]int64, 0)
	for _, item := range items {
		if item.ComboID != nil {
			comboIDs = append(comboIDs, *item.ComboID)
		}
	}

	if len(comboIDs) == 0 {
		return
	}

	// 批量查询成员图片
	memberImages, err := server.store.GetComboMemberImagesByCombos(ctx, comboIDs)
	if err != nil {
		log.Error().Err(err).Msg("failed to get combo member images")
		return
	}

	// 按 combo_id 组织图片
	imgMap := make(map[int64][]string)
	for _, row := range memberImages {
		if row.ImageUrl.Valid {
			fullURL := normalizeUploadURLForClient(row.ImageUrl.String)
			imgMap[row.ComboID] = append(imgMap[row.ComboID], fullURL)
		}
	}

	// 回填到 items
	for i := range items {
		if items[i].ComboID != nil {
			if imgs, ok := imgMap[*items[i].ComboID]; ok {
				items[i].ComboMemberImages = imgs
			}
		}
	}
}
