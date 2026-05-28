package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
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
	CategoryName   string                   `json:"category_name,omitempty"`
	Quantity       int16                    `json:"quantity"`
	Customizations []orderCustomizationItem `json:"customizations,omitempty"`
	ImageAssetID   *int64                   `json:"-"`
	ImageURL       string                   `json:"image_url,omitempty"`
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

	// 订单主状态
	OrderStatus string `json:"order_status"`

	// 履约状态
	FulfillmentStatus string `json:"fulfillment_status"`

	// 厨房阶段: paid/preparing/ready
	KitchenStatus string `json:"kitchen_status"`

	// 商户当前是否可标记出餐
	CanMarkReady bool `json:"can_mark_ready"`

	// 当前状态指引
	StatusHint string `json:"status_hint,omitempty"`

	// 桌台号 (堂食订单)
	TableNo *string `json:"table_no,omitempty"`

	// 取餐码
	PickupCode *string `json:"pickup_code,omitempty"`

	// 取餐码兼容别名
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

	// 待取餐/待代取订单列表
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	paidOrders, err := server.store.ListMerchantKitchenOrdersByStage(ctx, db.ListMerchantKitchenOrdersByStageParams{
		MerchantID: merchant.ID,
		Stage:      db.OrderStatusPaid,
		Limit:      KitchenMaxOrdersPerStatus,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	preparingOrders, err := server.store.ListMerchantKitchenOrdersByStage(ctx, db.ListMerchantKitchenOrdersByStageParams{
		MerchantID: merchant.ID,
		Stage:      db.OrderStatusPreparing,
		Limit:      KitchenMaxOrdersPerStatus,
		Offset:     0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	readyOrders, err := server.store.ListMerchantKitchenOrdersByStage(ctx, db.ListMerchantKitchenOrdersByStageParams{
		MerchantID: merchant.ID,
		Stage:      db.OrderStatusReady,
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("count merchant completed kitchen orders: %w", err)))
		return
	}

	// 转换为厨房订单响应格式
	newKitchenOrders := make([]kitchenOrderResponse, 0, len(paidOrders))
	for _, o := range paidOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("convert paid kitchen order %d: %w", o.ID, err)))
			return
		}
		newKitchenOrders = append(newKitchenOrders, ko)
	}

	preparingKitchenOrders := make([]kitchenOrderResponse, 0, len(preparingOrders))
	for _, o := range preparingOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("convert preparing kitchen order %d: %w", o.ID, err)))
			return
		}
		preparingKitchenOrders = append(preparingKitchenOrders, ko)
	}

	readyKitchenOrders := make([]kitchenOrderResponse, 0, len(readyOrders))
	for _, o := range readyOrders {
		ko, err := server.convertToKitchenOrder(ctx, o)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("convert ready kitchen order %d: %w", o.ID, err)))
			return
		}
		readyKitchenOrders = append(readyKitchenOrders, ko)
	}

	// 计算平均出餐时间（从最近N天的历史订单数据计算）
	// 逻辑：支付时间(paid_at) 到 出餐时间(status变为ready的时间) 的平均值
	calcStartTime := now.AddDate(0, 0, -AvgPrepareTimeCalcDays)
	avgPrepareTime, err := server.store.GetMerchantAvgPrepareTime(ctx, db.GetMerchantAvgPrepareTimeParams{
		MerchantID: merchant.ID,
		StartAt:    calcStartTime,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant average prepare time: %w", err)))
		return
	}
	if avgPrepareTime <= 0 {
		// 如果没有历史数据，使用默认值
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
// @Description 商户接单并将订单标记为制作中状态（仅已支付订单可操作；外卖订单会进入骑手接单池）
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（仅用于归属校验）
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if isNotFoundError(err) {
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

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.AcceptMerchantOrder(ctx, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    uri.OrderID,
		OperatorID: authPayload.UserID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

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
// @Description 将订单标记为出餐完成/待取餐状态（仅制作中订单可操作）
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（用于归属校验；统一订单服务负责后续状态流转和通知）
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if isNotFoundError(err) {
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

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.MarkMerchantOrderReady(ctx, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    uri.OrderID,
		OperatorID: authPayload.UserID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单
	order, err := server.store.GetOrder(ctx, uri.OrderID)
	if err != nil {
		if isNotFoundError(err) {
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
			if err := parseJSON(item.Customizations, &customizations); err != nil {
				return kitchenOrderResponse{}, fmt.Errorf("decode kitchen order item %d customizations: %w", item.ID, err)
			}
		}

		var imageAssetID *int64
		var prepareTime int16 = DefaultAvgPrepareTimeMinutes // 默认值
		var categoryName string

		if item.DishID.Valid {
			dish, err := server.store.GetDish(ctx, item.DishID.Int64)
			if err == nil {
				if dish.ImageMediaAssetID.Valid {
					v := dish.ImageMediaAssetID.Int64
					imageAssetID = &v
				}
				prepareTime = dish.PrepareTime

				// 获取分类名称
				if dish.CategoryID.Valid {
					category, err := server.store.GetDishCategory(ctx, dish.CategoryID.Int64)
					if err == nil {
						categoryName = category.Name
					} else if !isNotFoundError(err) {
						log.Warn().
							Err(err).
							Int64("dish_id", dish.ID).
							Int64("category_id", dish.CategoryID.Int64).
							Msg("kitchen order dish category projection skipped")
					}
				}
			} else if !isNotFoundError(err) {
				return kitchenOrderResponse{}, fmt.Errorf("get kitchen order dish %d: %w", item.DishID.Int64, err)
			}
		}

		// 更新订单最长制作时间
		if prepareTime > maxPrepareTime {
			maxPrepareTime = prepareTime
		}

		kitchenItems = append(kitchenItems, kitchenOrderItem{
			ID:             item.ID,
			Name:           item.Name,
			CategoryName:   categoryName,
			Quantity:       item.Quantity,
			Customizations: customizations,
			ImageAssetID:   imageAssetID,
			PrepareTime:    prepareTime,
		})
	}
	// 批量解析菜品图片 URL
	kitchenAssetIDs := make([]int64, 0, len(kitchenItems))
	for _, ki := range kitchenItems {
		if ki.ImageAssetID != nil {
			kitchenAssetIDs = append(kitchenAssetIDs, *ki.ImageAssetID)
		}
	}
	if len(kitchenAssetIDs) > 0 {
		imgURLs := server.batchPublicImageURLs(ctx, kitchenAssetIDs, media.VariantThumb)
		for i := range kitchenItems {
			if kitchenItems[i].ImageAssetID != nil {
				kitchenItems[i].ImageURL = imgURLs[*kitchenItems[i].ImageAssetID]
			}
		}
	}
	// 获取桌台号（堂食订单）
	var tableNo *string
	if order.TableID.Valid {
		table, err := server.store.GetTable(ctx, order.TableID.Int64)
		if err == nil {
			tableNo = &table.TableNo
		} else if !isNotFoundError(err) {
			return kitchenOrderResponse{}, fmt.Errorf("get kitchen order table %d: %w", order.TableID.Int64, err)
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
	urgeCount, err := server.store.CountOrderUrges(ctx, order.ID)
	if err != nil {
		return kitchenOrderResponse{}, fmt.Errorf("count kitchen order urges %d: %w", order.ID, err)
	}
	if urgeCount > 0 {
		isUrged = true
	}

	// 获取备注
	var notes *string
	if order.Notes.Valid && order.Notes.String != "" {
		notes = &order.Notes.String
	}
	var pickupCode *string
	if order.PickupCode.Valid && order.PickupCode.String != "" {
		code := formatPickupCode(order.PickupCode.String)
		pickupCode = &code
	}

	// 支付时间
	paidAt := order.CreatedAt
	if order.PaidAt.Valid {
		paidAt = order.PaidAt.Time
	}

	kitchenStatus, canMarkReady, statusHint := kitchenOrderStageState(order)

	return kitchenOrderResponse{
		ID:                order.ID,
		OrderNo:           order.OrderNo,
		OrderType:         order.OrderType,
		Status:            kitchenStatus,
		OrderStatus:       order.Status,
		FulfillmentStatus: order.FulfillmentStatus,
		KitchenStatus:     kitchenStatus,
		CanMarkReady:      canMarkReady,
		StatusHint:        statusHint,
		TableNo:           tableNo,
		PickupCode:        pickupCode,
		PickupNumber:      pickupCode,
		Notes:             notes,
		Items:             kitchenItems,
		EstimatedReadyAt:  estimatedReadyAt,
		WaitingMinutes:    waitingMinutes,
		IsUrged:           isUrged,
		CreatedAt:         order.CreatedAt,
		PaidAt:            paidAt,
	}, nil
}

func kitchenOrderStageState(order db.Order) (string, bool, string) {
	switch {
	case order.Status == db.OrderStatusPaid:
		return db.OrderStatusPaid, false, "顾客已支付，请接单后开始制作"
	case order.Status == db.OrderStatusPreparing:
		return db.OrderStatusPreparing, true, "餐品制作中，出餐后请标记完成"
	case order.Status == db.OrderStatusReady:
		return db.OrderStatusReady, false, "餐品已出餐，等待取餐"
	case order.OrderType == db.OrderTypeTakeout && order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus == db.FulfillmentStatusPreparing:
		return db.OrderStatusPreparing, true, "骑手已接单，餐品仍在制作，请出餐后标记完成"
	case order.OrderType == db.OrderTypeTakeout && order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus == db.FulfillmentStatusReady:
		return db.OrderStatusReady, false, "骑手已接单，等待骑手取餐"
	default:
		if order.StatusHint.Valid && order.StatusHint.String != "" {
			return order.Status, false, order.StatusHint.String
		}
		return order.Status, false, ""
	}
}
