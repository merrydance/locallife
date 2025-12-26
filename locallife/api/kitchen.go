package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
)

// ==================== 厨房显示系统 (KDS) ====================
// Kitchen Display System - 用于厨房管理订单出餐流程

// 厨房订单状态
const (
	KitchenStatusNew       = "new"       // 新订单（待确认）
	KitchenStatusPreparing = "preparing" // 制作中
	KitchenStatusReady     = "ready"     // 出餐完成
)

// 厨房系统配置常量
const (
	KitchenMaxOrdersPerStatus    = 100 // 每种状态最多显示订单数
	DefaultAvgPrepareTimeMinutes = 15  // 默认平均出餐时间（分钟）
	AvgPrepareTimeCalcDays       = 7   // 计算平均出餐时间的天数范围
)

// ==================== 请求/响应结构体 ====================

type kitchenOrderItem struct {
	ID             int64                    `json:"id"`
	Name           string                   `json:"name"`
	Quantity       int16                    `json:"quantity"`
	Customizations []orderCustomizationItem `json:"customizations,omitempty"`
	ImageURL       *string                  `json:"image_url,omitempty"`
	PrepareTime    int16                    `json:"prepare_time"` // 预估制作时间（分钟）
}

type kitchenOrderResponse struct {
	// 订单ID
	ID int64 `json:"id"`

	// 订单编号
	OrderNo string `json:"order_no"`

	// 订单类型: takeout-外卖, dine_in-堂食, takeaway-打包自取
	OrderType string `json:"order_type"`

	// 订单状态: paid-新订单, preparing-制作中, ready-出餐完成
	Status string `json:"status"`

	// 桌台号 (堂食订单)
	TableNo *string `json:"table_no,omitempty"`

	// 取餐号 (打包自取)
	PickupNumber *string `json:"pickup_number,omitempty"`

	// 订单备注
	Notes *string `json:"notes,omitempty"`

	// 订单商品列表
	Items []kitchenOrderItem `json:"items"`

	// 预计出餐时间
	EstimatedReadyAt *time.Time `json:"estimated_ready_at,omitempty"`

	// 等待时间（分钟）
	WaitingMinutes int `json:"waiting_minutes"`

	// 是否催单
	IsUrged bool `json:"is_urged"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`

	// 支付时间（订单开始计时的时间）
	PaidAt time.Time `json:"paid_at"`
}

type kitchenOrdersResponse struct {
	// 新订单列表
	NewOrders []kitchenOrderResponse `json:"new_orders"`

	// 制作中订单列表
	PreparingOrders []kitchenOrderResponse `json:"preparing_orders"`

	// 待取餐/待配送订单列表
	ReadyOrders []kitchenOrderResponse `json:"ready_orders"`

	// 统计信息
	Stats kitchenStats `json:"stats"`
}

type kitchenStats struct {
	// 新订单数
	NewCount int `json:"new_count"`

	// 制作中订单数
	PreparingCount int `json:"preparing_count"`

	// 待取餐数
	ReadyCount int `json:"ready_count"`

	// 今日完成订单数
	CompletedTodayCount int `json:"completed_today_count"`

	// 平均出餐时间（分钟）
	AvgPrepareTime int `json:"avg_prepare_time"`
}

// kitchenOrderURIRequest 厨房订单路径参数请求
type kitchenOrderURIRequest struct {
	OrderID int64 `uri:"id" binding:"required,min=1"`
}

// ==================== API 处理函数 ====================

// listKitchenOrders godoc
// @Summary 获取厨房订单列表
// @Description 获取厨房显示系统(KDS)的订单列表，包含新订单、制作中、待取餐三种状态
// @Tags 厨房管理(KDS)
// @Produce json
// @Success 200 {object} kitchenOrdersResponse "订单列表和统计信息"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/kitchen/orders [get]
// @Security BearerAuth
func (server *Server) listKitchenOrders(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询需要在厨房显示的订单（paid, preparing, ready 状态）
	// 使用 ListMerchantOrdersByStatuses 或类似查询
	paidOrders, err := server.store.ListMerchantOrdersByStatus(ctx, db.ListMerchantOrdersByStatusParams{
		MerchantID: merchant.ID,
		Status:     "paid",
		Limit:      KitchenMaxOrdersPerStatus,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	preparingOrders, err := server.store.ListMerchantOrdersByStatus(ctx, db.ListMerchantOrdersByStatusParams{
		MerchantID: merchant.ID,
		Status:     "preparing",
		Limit:      KitchenMaxOrdersPerStatus,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	readyOrders, err := server.store.ListMerchantOrdersByStatus(ctx, db.ListMerchantOrdersByStatusParams{
		MerchantID: merchant.ID,
		Status:     "ready",
		Limit:      KitchenMaxOrdersPerStatus,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取今日完成的订单数 - 使用本地时区的当日零点
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	completedCount, err := server.store.CountMerchantOrdersByStatusAfterTime(ctx, db.CountMerchantOrdersByStatusAfterTimeParams{
		MerchantID: merchant.ID,
		Status:     "completed",
		CreatedAt:  todayStart,
	})
	if err != nil {
		completedCount = 0 // 忽略统计错误
	}

	// 转换为厨房订单响应格式
	newKitchenOrders := make([]kitchenOrderResponse, 0, len(paidOrders))
	for _, o := range paidOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err == nil {
			newKitchenOrders = append(newKitchenOrders, ko)
		}
	}

	preparingKitchenOrders := make([]kitchenOrderResponse, 0, len(preparingOrders))
	for _, o := range preparingOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err == nil {
			preparingKitchenOrders = append(preparingKitchenOrders, ko)
		}
	}

	readyKitchenOrders := make([]kitchenOrderResponse, 0, len(readyOrders))
	for _, o := range readyOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err == nil {
			readyKitchenOrders = append(readyKitchenOrders, ko)
		}
	}

	// 计算平均出餐时间（从最近N天的历史订单数据计算）
	// 逻辑：支付时间(paid_at) 到 出餐时间(status变为ready的时间) 的平均值
	calcStartTime := now.AddDate(0, 0, -AvgPrepareTimeCalcDays)
	avgPrepareTime, err := server.store.GetMerchantAvgPrepareTime(ctx, db.GetMerchantAvgPrepareTimeParams{
		MerchantID: merchant.ID,
		CreatedAt:  calcStartTime,
	})
	if err != nil || avgPrepareTime <= 0 {
		// 如果没有历史数据或查询失败，使用默认值
		avgPrepareTime = DefaultAvgPrepareTimeMinutes
	}

	ctx.JSON(http.StatusOK, kitchenOrdersResponse{
		NewOrders:       newKitchenOrders,
		PreparingOrders: preparingKitchenOrders,
		ReadyOrders:     readyKitchenOrders,
		Stats: kitchenStats{
			NewCount:            len(newKitchenOrders),
			PreparingCount:      len(preparingKitchenOrders),
			ReadyCount:          len(readyKitchenOrders),
			CompletedTodayCount: int(completedCount),
			AvgPrepareTime:      int(avgPrepareTime),
		},
	})
}

// startPreparing godoc
// @Summary 开始制作订单
// @Description 将订单标记为制作中状态（仅已支付订单可操作）
// @Tags 厨房管理(KDS)
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} kitchenOrderResponse "更新后的订单信息"
// @Failure 400 {object} ErrorResponse "参数错误或订单状态不允许"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户或订单不属于该商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/kitchen/orders/{id}/preparing [post]
// @Security BearerAuth
func (server *Server) startPreparing(ctx *gin.Context) {
	var uri kitchenOrderURIRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单归属
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to this merchant")))
		return
	}

	// 验证订单状态
	if order.Status != "paid" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order is not in paid status")))
		return
	}

	// 更新订单状态为 preparing
	updatedOrder, err := server.store.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:     uri.OrderID,
		Status: "preparing",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为厨房订单响应
	ko, err := server.convertToKitchenOrder(ctx, updatedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ko)
}

// markKitchenOrderReady godoc
// @Summary 标记订单出餐完成
// @Description 将订单标记为出餐完成/待取餐状态（仅制作中或已支付订单可操作）
// @Tags 厨房管理(KDS)
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} kitchenOrderResponse "更新后的订单信息"
// @Failure 400 {object} ErrorResponse "参数错误或订单状态不允许"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户或订单不属于该商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/kitchen/orders/{id}/ready [post]
// @Security BearerAuth
func (server *Server) markKitchenOrderReady(ctx *gin.Context) {
	var uri kitchenOrderURIRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单归属
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to this merchant")))
		return
	}

	// 验证订单状态
	if order.Status != "preparing" && order.Status != "paid" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order is not in preparing or paid status")))
		return
	}

	// 更新订单状态为 ready
	updatedOrder, err := server.store.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:     uri.OrderID,
		Status: "ready",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为厨房订单响应
	ko, err := server.convertToKitchenOrder(ctx, updatedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ko)
}

// getKitchenOrderDetails godoc
// @Summary 获取厨房订单详情
// @Description 获取单个订单在厨房视图中的详细信息（商品、定制、备注等）
// @Tags 厨房管理(KDS)
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} kitchenOrderResponse "订单详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户或订单不属于该商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/kitchen/orders/{id} [get]
// @Security BearerAuth
func (server *Server) getKitchenOrderDetails(ctx *gin.Context) {
	var uri kitchenOrderURIRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单归属
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to this merchant")))
		return
	}

	// 转换为厨房订单响应
	ko, err := server.convertToKitchenOrder(ctx, order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ko)
}

// ==================== 辅助函数 ====================

// convertToKitchenOrder 将订单转换为厨房显示格式
func (server *Server) convertToKitchenOrder(ctx *gin.Context, order db.Order) (kitchenOrderResponse, error) {
	// 获取订单明细
	items, err := server.store.ListOrderItemsByOrder(ctx, order.ID)
	if err != nil {
		return kitchenOrderResponse{}, err
	}

	// 转换订单商品，并计算订单预估出餐时间（取各菜品制作时间的最大值）
	kitchenItems := make([]kitchenOrderItem, 0, len(items))
	var maxPrepareTime int16 = 0

	for _, item := range items {
		var customizations []orderCustomizationItem
		if item.Customizations != nil {
			_ = parseJSON(item.Customizations, &customizations)
		}

		var imageURL *string
		var prepareTime int16 = DefaultAvgPrepareTimeMinutes // 默认值

		if item.DishID.Valid {
			dish, err := server.store.GetDish(ctx, item.DishID.Int64)
			if err == nil {
				if dish.ImageUrl.Valid {
					img := normalizeUploadURLForClient(dish.ImageUrl.String)
					imageURL = &img
				}
				prepareTime = dish.PrepareTime
			}
		}

		// 更新订单最长制作时间
		if prepareTime > maxPrepareTime {
			maxPrepareTime = prepareTime
		}

		kitchenItems = append(kitchenItems, kitchenOrderItem{
			ID:             item.ID,
			Name:           item.Name,
			Quantity:       item.Quantity,
			Customizations: customizations,
			ImageURL:       imageURL,
			PrepareTime:    prepareTime,
		})
	}

	// 获取桌台号（堂食订单）
	var tableNo *string
	if order.TableID.Valid {
		table, err := server.store.GetTable(ctx, order.TableID.Int64)
		if err == nil {
			tableNo = &table.TableNo
		}
	}

	// 计算等待时间
	var waitingMinutes int
	startTime := order.CreatedAt
	if order.PaidAt.Valid {
		startTime = order.PaidAt.Time
	}
	waitingMinutes = int(time.Since(startTime).Minutes())

	// 计算预估出餐时间：支付时间 + 最大菜品制作时间
	var estimatedReadyAt *time.Time
	if order.PaidAt.Valid && maxPrepareTime > 0 {
		estimated := order.PaidAt.Time.Add(time.Duration(maxPrepareTime) * time.Minute)
		estimatedReadyAt = &estimated
	}

	// 检查是否被催单（简化实现：检查是否有催单记录）
	isUrged := false
	urgeCount, _ := server.store.CountOrderUrges(ctx, order.ID)
	if urgeCount > 0 {
		isUrged = true
	}

	// 获取备注
	var notes *string
	if order.Notes.Valid && order.Notes.String != "" {
		notes = &order.Notes.String
	}

	// 支付时间
	paidAt := order.CreatedAt
	if order.PaidAt.Valid {
		paidAt = order.PaidAt.Time
	}

	return kitchenOrderResponse{
		ID:               order.ID,
		OrderNo:          order.OrderNo,
		OrderType:        order.OrderType,
		Status:           order.Status,
		TableNo:          tableNo,
		Notes:            notes,
		Items:            kitchenItems,
		EstimatedReadyAt: estimatedReadyAt,
		WaitingMinutes:   waitingMinutes,
		IsUrged:          isUrged,
		CreatedAt:        order.CreatedAt,
		PaidAt:           paidAt,
	}, nil
}
