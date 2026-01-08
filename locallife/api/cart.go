package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 购物车 API ====================

type cartItemResponse struct {
	ID             int64                  `json:"id"`
	DishID         *int64                 `json:"dish_id,omitempty"`
	ComboID        *int64                 `json:"combo_id,omitempty"`
	Quantity       int16                  `json:"quantity"`
	Customizations map[string]interface{} `json:"customizations,omitempty"`
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
		if errors.Is(err, pgx.ErrNoRows) {
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

	response := buildCartResponse(cart, items)
	ctx.JSON(http.StatusOK, response)
}

func buildCartResponse(cart db.Cart, items []db.ListCartItemsRow) cartResponse {
	var cartItems []cartItemResponse
	var subtotal int64
	var totalCount int

	for _, item := range items {
		var unitPrice int64
		var name string
		var imageURL string
		var memberPrice *int64
		var isAvailable bool

		if item.DishID.Valid {
			name = item.DishName.String
			imageURL = normalizeUploadURLForClient(item.DishImageUrl.String)
			unitPrice = item.DishPrice.Int64
			if item.DishMemberPrice.Valid {
				memberPrice = &item.DishMemberPrice.Int64
			}
			isAvailable = item.DishIsAvailable.Bool
		} else if item.ComboID.Valid {
			name = item.ComboName.String
			imageURL = normalizeUploadURLForClient(item.ComboImageUrl.String)
			unitPrice = item.ComboPrice.Int64
			isAvailable = item.ComboIsAvailable.Bool
		}

		itemSubtotal := unitPrice * int64(item.Quantity)
		subtotal += itemSubtotal
		totalCount += int(item.Quantity)

		cartItem := cartItemResponse{
			ID:          item.ID,
			Quantity:    item.Quantity,
			Name:        name,
			ImageURL:    imageURL,
			UnitPrice:   unitPrice,
			MemberPrice: memberPrice,
			IsAvailable: isAvailable,
			Subtotal:    itemSubtotal,
		}

		if item.DishID.Valid {
			dishID := item.DishID.Int64
			cartItem.DishID = &dishID
		}
		if item.ComboID.Valid {
			comboID := item.ComboID.Int64
			cartItem.ComboID = &comboID
		}

		// 解析定制选项
		if len(item.Customizations) > 0 {
			var customizations map[string]interface{}
			if err := json.Unmarshal(item.Customizations, &customizations); err == nil {
				cartItem.Customizations = customizations
			}
		}

		cartItems = append(cartItems, cartItem)
	}

	return cartResponse{
		ID:            cart.ID,
		MerchantID:    cart.MerchantID,
		OrderType:     cart.OrderType,
		TableID:       nullableInt64(cart.TableID),
		ReservationID: nullableInt64(cart.ReservationID),
		Items:         cartItems,
		TotalCount:    totalCount,
		Subtotal:      subtotal,
	}
}

func nullableInt64(v pgtype.Int8) *int64 {
	if v.Valid {
		return &v.Int64
	}
	return nil
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户存在
	_, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant: %w", err)))
		return
	}

	// 验证菜品/套餐存在且属于该商户
	if req.DishID != nil {
		dish, err := server.store.GetDish(ctx, *req.DishID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dish not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get dish: %w", err)))
			return
		}
		if dish.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("dish does not belong to this merchant")))
			return
		}
		if !dish.IsOnline || !dish.IsAvailable {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("dish is not available")))
			return
		}
	}

	if req.ComboID != nil {
		combo, err := server.store.GetComboSet(ctx, *req.ComboID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("combo not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get combo: %w", err)))
			return
		}
		if combo.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("combo does not belong to this merchant")))
			return
		}
		if !combo.IsOnline {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("combo is not available")))
			return
		}
	}

	// 获取或创建购物车
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

	// 先尝试获取现有购物车
	cart, err := server.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        authPayload.UserID,
		MerchantID:    req.MerchantID,
		OrderType:     req.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 不存在，则创建新购物车
			cart, err = server.store.CreateCart(ctx, db.CreateCartParams{
				UserID:        authPayload.UserID,
				MerchantID:    req.MerchantID,
				OrderType:     req.OrderType,
				TableID:       tableID,
				ReservationID: reservationID,
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create cart: %w", err)))
				return
			}
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get existing cart: %w", err)))
			return
		}
	}

	// 处理定制选项
	var customizations []byte
	if req.Customizations != nil {
		customizations, _ = json.Marshal(req.Customizations)
	}

	// 检查是否已有相同商品（同菜品+同定制选项）
	if req.DishID != nil {
		existingItem, err := server.store.GetCartItemByDishAndCustomizations(ctx, db.GetCartItemByDishAndCustomizationsParams{
			CartID:         cart.ID,
			DishID:         pgtype.Int8{Int64: *req.DishID, Valid: true},
			Customizations: customizations,
		})
		if err == nil {
			// 已存在相同商品，更新数量
			newQuantity := existingItem.Quantity + req.Quantity
			_, err = server.store.UpdateCartItem(ctx, db.UpdateCartItemParams{
				ID:       existingItem.ID,
				Quantity: pgtype.Int2{Int16: newQuantity, Valid: true},
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update cart item quantity: %w", err)))
				return
			}
			server.returnUpdatedCart(ctx, cart)
			return
		}
	}

	if req.ComboID != nil {
		existingItem, err := server.store.GetCartItemByCombo(ctx, db.GetCartItemByComboParams{
			CartID:  cart.ID,
			ComboID: pgtype.Int8{Int64: *req.ComboID, Valid: true},
		})
		if err == nil {
			// 已存在相同套餐，更新数量
			newQuantity := existingItem.Quantity + req.Quantity
			_, err = server.store.UpdateCartItem(ctx, db.UpdateCartItemParams{
				ID:       existingItem.ID,
				Quantity: pgtype.Int2{Int16: newQuantity, Valid: true},
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update cart item quantity: %w", err)))
				return
			}
			server.returnUpdatedCart(ctx, cart)
			return
		}
	}

	// 添加新商品到购物车
	var dishID, comboID pgtype.Int8
	if req.DishID != nil {
		dishID = pgtype.Int8{Int64: *req.DishID, Valid: true}
	}
	if req.ComboID != nil {
		comboID = pgtype.Int8{Int64: *req.ComboID, Valid: true}
	}

	_, err = server.store.AddCartItem(ctx, db.AddCartItemParams{
		CartID:         cart.ID,
		DishID:         dishID,
		ComboID:        comboID,
		Quantity:       req.Quantity,
		Customizations: customizations,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("add cart item: %w", err)))
		return
	}

	server.returnUpdatedCart(ctx, cart)
}

func (server *Server) returnUpdatedCart(ctx *gin.Context, cart db.Cart) {
	items, err := server.store.ListCartItems(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list cart items: %w", err)))
		return
	}
	response := buildCartResponse(cart, items)
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

	// 获取购物车商品信息
	cartItem, err := server.store.GetCartItem(ctx, itemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("cart item not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart item: %w", err)))
		return
	}

	// P0安全：通过cart_id获取购物车，验证所有权
	cart, err := server.store.GetCart(ctx, cartItem.CartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	// 构建更新参数
	updateParams := db.UpdateCartItemParams{
		ID: itemID,
	}

	if req.Quantity != nil {
		updateParams.Quantity = pgtype.Int2{Int16: *req.Quantity, Valid: true}
	}

	if req.Customizations != nil {
		customizations, _ := json.Marshal(req.Customizations)
		updateParams.Customizations = customizations
	}

	_, err = server.store.UpdateCartItem(ctx, updateParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update cart item: %w", err)))
		return
	}

	server.returnUpdatedCart(ctx, cart)
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("cart item not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get cart item: %w", err)))
		return
	}

	// P0安全：通过cart_id获取购物车，验证所有权
	cart, err := server.store.GetCart(ctx, cartItem.CartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	// 优惠券减免金额（分）
	DiscountAmount int64 `json:"discount_amount"`
	// 实付金额（分）
	TotalAmount int64 `json:"total_amount"`
	// 优惠说明
	DiscountInfo string `json:"discount_info"`
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息（用于获取region_id和min_order_amount）
	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant: %w", err)))
		return
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	if req.AddressID != nil {
		address, err := server.store.GetUserAddress(ctx, *req.AddressID)
		if err == nil && address.UserID == authPayload.UserID {
			// 计算距离：优先调用地图API，失败则使用默认值
			distance := int32(3000) // 默认3公里
			if address.Latitude.Valid && address.Longitude.Valid &&
				merchant.Latitude.Valid && merchant.Longitude.Valid {
				// 调用腾讯地图API计算骑行距离
				fromLoc := maps.Location{
					Lat: numericToFloat64(merchant.Latitude),
					Lng: numericToFloat64(merchant.Longitude),
				}
				toLoc := maps.Location{
					Lat: numericToFloat64(address.Latitude),
					Lng: numericToFloat64(address.Longitude),
				}
				if server.mapClient != nil {
					routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
					if err == nil && routeResult != nil {
						distance = int32(routeResult.Distance)
					}
				}
			}

			// 调用配送费计算
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
			}
		}
	} else if req.Latitude != nil && req.Longitude != nil {
		// 用户未选择地址但传入了当前位置坐标，使用坐标计算配送费
		distance := int32(3000) // 默认3公里
		if merchant.Latitude.Valid && merchant.Longitude.Valid {
			fromLoc := maps.Location{
				Lat: numericToFloat64(merchant.Latitude),
				Lng: numericToFloat64(merchant.Longitude),
			}
			toLoc := maps.Location{
				Lat: *req.Latitude,
				Lng: *req.Longitude,
			}
			if server.mapClient != nil {
				routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
				if err == nil && routeResult != nil {
					distance = int32(routeResult.Distance)
				}
			}
		}

		// 调用配送费计算
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
		}
	}

	// 计算优惠券减免（如果提供了优惠券）
	var discountAmount int64
	var discountInfo string
	if req.VoucherID != nil {
		voucher, err := server.store.GetUserVoucher(ctx, *req.VoucherID)
		if err == nil && voucher.UserID == authPayload.UserID &&
			voucher.Status == "unused" {
			// 检查优惠券是否适用于该商户和订单金额
			if voucher.MerchantID == req.MerchantID && subtotal >= voucher.MinOrderAmount {
				discountAmount = voucher.Amount
				discountInfo = voucher.Name
			} else if subtotal < voucher.MinOrderAmount {
				discountInfo = "未达到优惠券使用门槛"
			} else {
				discountInfo = "优惠券不适用于该商户"
			}
		} else {
			discountInfo = "优惠券不可用"
		}
	}

	totalAmount := subtotal + deliveryFee - deliveryFeeDiscount - discountAmount
	if totalAmount < 0 {
		totalAmount = 0
	}

	ctx.JSON(http.StatusOK, calculateCartResponse{
		Subtotal:            subtotal,
		DeliveryFee:         deliveryFee,
		DeliveryFeeDiscount: deliveryFeeDiscount,
		DiscountAmount:      discountAmount,
		TotalAmount:         totalAmount,
		DiscountInfo:        discountInfo,
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
	Total int64 `json:"total"`
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
			Offset:     int32((page - 1) * pageSize),
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
			Offset: int32((page - 1) * pageSize),
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
		Items: result,
		Total: total,
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
	// 各商户订单
	Items []combinedCheckoutItem `json:"items"`
	// 商品合计（分）
	TotalSubtotal int64 `json:"total_subtotal"`
	// 配送费合计（分）
	TotalDeliveryFee int64 `json:"total_delivery_fee"`
	// 支付总额（分）
	TotalAmount int64 `json:"total_amount"`
	// 是否可以合单支付
	CanCombinePay bool `json:"can_combine_pay"`
	// 提示信息
	Message string `json:"message,omitempty"`
}

// previewCombinedCheckout godoc
// @Summary 预览合单结算
// @Description 预览多商户合单结算金额，返回各商户子单和合计金额
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body combinedCheckoutRequest true "结算请求"
// @Success 200 {object} combinedCheckoutResponse "结算预览"
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

	// 获取用户指定的购物车（使用正确的查询：根据cart_id而非merchant_id）
	carts, err := server.store.GetUserCartsByCartIDs(ctx, db.GetUserCartsByCartIDsParams{
		UserID:  authPayload.UserID,
		Column2: req.CartIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user carts by cart ids: %w", err)))
		return
	}

	if len(carts) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("未找到有效的购物车")))
		return
	}

	// 检查所有商户是否支持合单支付（需要有微信子商户号）
	var canCombinePay = true
	var message string
	var items []combinedCheckoutItem
	var totalSubtotal, totalDeliveryFee, totalAmount int64

	for _, cart := range carts {
		// 检查商户状态
		if cart.MerchantStatus != "active" {
			canCombinePay = false
			message = "部分商户暂停营业"
			continue
		}

		// 检查子商户号
		if !cart.SubMchid.Valid || cart.SubMchid.String == "" {
			canCombinePay = false
			message = "部分商户暂不支持在线支付"
		}

		// 获取购物车商品计算金额
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

		// 计算配送费（调用真实的配送费计算逻辑）
		var deliveryFee int64 = 0
		if req.AddressID != nil {
			// 获取地址信息用于距离计算
			address, err := server.store.GetUserAddress(ctx, *req.AddressID)
			if err == nil && address.UserID == authPayload.UserID {
				// 计算距离：优先调用地图API，失败则使用默认值
				distance := int32(3000) // 默认3公里
				if address.Latitude.Valid && address.Longitude.Valid &&
					cart.MerchantLatitude.Valid && cart.MerchantLongitude.Valid {
					// 调用腾讯地图API计算骑行距离
					fromLoc := maps.Location{
						Lat: numericToFloat64(cart.MerchantLatitude),
						Lng: numericToFloat64(cart.MerchantLongitude),
					}
					toLoc := maps.Location{
						Lat: numericToFloat64(address.Latitude),
						Lng: numericToFloat64(address.Longitude),
					}
					if server.mapClient != nil {
						routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
						if err == nil && routeResult != nil {
							distance = int32(routeResult.Distance)
						}
					}
				}

				// 调用配送费计算
				feeResult, err := server.calculateDeliveryFeeInternal(
					ctx,
					cart.RegionID,
					cart.MerchantID,
					distance,
					subtotal,
				)
				if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
					deliveryFee = feeResult.FinalFee
				}
			}
		}

		items = append(items, combinedCheckoutItem{
			MerchantID:   cart.MerchantID,
			MerchantName: cart.MerchantName,
			OrderType:    "takeout",
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
