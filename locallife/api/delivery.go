package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

// sendDeliveryStatusNotification 异步发送配送状态通知
func (server *Server) sendDeliveryStatusNotification(
	ctx context.Context,
	userID int64,
	orderID int64,
	deliveryID int64,
	status string,
	title string,
	message string,
) {
	expiresAt := time.Now().Add(1 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
		UserID:      userID,
		Type:        "delivery",
		Title:       title,
		Content:     message,
		RelatedType: "delivery",
		RelatedID:   deliveryID,
		ExtraData: map[string]any{
			"order_id": orderID,
			"status":   status,
		},
		ExpiresAt: &expiresAt,
	})
}

// uploadShippingInfoAsync 骑手取货后将发货信息上报任务投入 asynq 队列。
// 满足「发货信息管理」合规要求，触发后续结算事件推送。
// 仅对已支付的 profit_sharing 订单上报（堂食/自取无需上报）。
// 任务由 worker/task_upload_shipping_info.go 处理，支持指数退避重试。
func (server *Server) uploadShippingInfoAsync(_ context.Context, orderID int64, userID int64) {
	if server.wechatClient == nil || server.taskDistributor == nil {
		return
	}
	err := server.taskDistributor.DistributeTaskUploadShippingInfo(
		context.Background(),
		&worker.UploadShippingInfoPayload{
			OrderID: orderID,
			UserID:  userID,
		},
		asynq.Queue(worker.QueueDefault),
		asynq.MaxRetry(5),
	)
	if err != nil {
		log.Error().Err(err).Int64("order_id", orderID).Msg("failed to enqueue upload_shipping_info task")
	}
}

// ==================== 推荐订单 ====================

type getRecommendedOrdersRequest struct {
	Longitude float64 `form:"longitude" binding:"required,gte=-180,lte=180"`
	Latitude  float64 `form:"latitude" binding:"required,gte=-90,lte=90"`
}

type recommendedOrderResponse struct {
	OrderID            int64      `json:"order_id"`
	MerchantID         int64      `json:"merchant_id"`
	MerchantName       string     `json:"merchant_name,omitempty"`
	MerchantAddress    string     `json:"merchant_address,omitempty"`
	CustomerAddress    string     `json:"customer_address,omitempty"`
	ItemCount          int        `json:"item_count,omitempty"`
	TotalScore         int        `json:"total_score"`
	DistanceScore      int        `json:"distance_score"`
	RouteScore         int        `json:"route_score"`
	UrgencyScore       int        `json:"urgency_score"`
	ProfitScore        int        `json:"profit_score"`
	DistanceToPickup   int        `json:"distance_to_pickup"`      // 直线距离（米）
	RealDistance       int        `json:"real_distance,omitempty"` // 真实骑行距离（米）
	EstimatedMinutes   int        `json:"estimated_minutes"`       // 预估时间（分钟）
	RealDuration       int        `json:"real_duration,omitempty"` // 真实骑行时间（秒）
	DeliveryFee        int64      `json:"delivery_fee"`
	Distance           int        `json:"distance"` // 商家到顾客距离
	PickupLongitude    float64    `json:"pickup_longitude"`
	PickupLatitude     float64    `json:"pickup_latitude"`
	DeliveryLongitude  float64    `json:"delivery_longitude"`
	DeliveryLatitude   float64    `json:"delivery_latitude"`
	ExpiresAt          time.Time  `json:"expires_at"`
	ExpectedDeliveryAt *time.Time `json:"expected_delivery_at,omitempty"` // 预计送达时间
}

// getRecommendedOrders godoc
// @Summary 获取推荐订单
// @Description 根据骑手当前位置获取推荐的可接订单列表，按综合评分排序
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param longitude query number true "骑手当前经度" minimum(-180) maximum(180)
// @Param latitude query number true "骑手当前纬度" minimum(-90) maximum(90)
// @Success 200 {array} recommendedOrderResponse "推荐订单列表"
// @Failure 400 {object} ErrorResponse "参数校验失败或骑手未上线"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "非骑手用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/recommend [get]
// @Security BearerAuth
func (server *Server) getRecommendedOrders(ctx *gin.Context) {
	var req getRecommendedOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	recommendationResult, err := logic.RecommendDeliveryOrdersForUser(ctx, server.store, server.routeService, logic.RecommendDeliveryForUserInput{
		UserID:   authPayload.UserID,
		RiderLat: req.Latitude,
		RiderLng: req.Longitude,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	scored := recommendationResult.Recommendations.Scored
	realDistances := recommendationResult.Recommendations.RealDistances

	// 转换响应
	var response []recommendedOrderResponse
	for _, s := range scored {
		resp := recommendedOrderResponse{
			OrderID:            s.OrderID,
			MerchantID:         s.PoolOrder.MerchantID,
			TotalScore:         s.TotalScore,
			DistanceScore:      s.DistanceScore,
			RouteScore:         s.RouteScore,
			UrgencyScore:       s.UrgencyScore,
			ProfitScore:        s.ProfitScore,
			DistanceToPickup:   s.DistanceToPickup,
			EstimatedMinutes:   s.EstimatedMinutes,
			DeliveryFee:        s.PoolOrder.DeliveryFee,
			Distance:           s.PoolOrder.Distance,
			PickupLongitude:    s.PoolOrder.PickupLocation.Longitude,
			PickupLatitude:     s.PoolOrder.PickupLocation.Latitude,
			DeliveryLongitude:  s.PoolOrder.DeliveryLocation.Longitude,
			DeliveryLatitude:   s.PoolOrder.DeliveryLocation.Latitude,
			ExpiresAt:          s.PoolOrder.ExpiresAt,
			ExpectedDeliveryAt: &s.PoolOrder.ExpectedDeliveryAt,
		}

		// 补充商户和订单信息
		if merchant, err := server.store.GetMerchant(ctx, s.PoolOrder.MerchantID); err == nil {
			resp.MerchantName = merchant.Name
			resp.MerchantAddress = merchant.Address
		}
		if delivery, err := server.store.GetDeliveryByOrderID(ctx, s.OrderID); err == nil {
			resp.CustomerAddress = delivery.DeliveryAddress
		}
		if count, err := server.store.CountOrderItems(ctx, s.OrderID); err == nil {
			resp.ItemCount = int(count)
		}

		// 添加真实距离（如果有）
		if rd, ok := realDistances[s.OrderID]; ok {
			resp.RealDistance = rd.Distance
			resp.RealDuration = rd.Duration
		}
		response = append(response, resp)
	}

	ctx.JSON(http.StatusOK, response)
}

// ==================== 抢单 ====================

type grabOrderRequest struct {
	OrderID int64 `uri:"order_id" binding:"required,min=1"`
}

type deliveryResponse struct {
	ID                  int64          `json:"id"`
	OrderID             int64          `json:"order_id"`
	OrderNo             string         `json:"order_no,omitempty"`
	RiderID             *int64         `json:"rider_id,omitempty"`
	MerchantName        string         `json:"merchant_name,omitempty"`
	PickupAddress       string         `json:"pickup_address"`
	PickupLongitude     float64        `json:"pickup_longitude"`
	PickupLatitude      float64        `json:"pickup_latitude"`
	PickupContact       string         `json:"pickup_contact,omitempty"`
	PickupPhone         string         `json:"pickup_phone,omitempty"`
	DeliveryAddress     string         `json:"delivery_address"`
	DeliveryLongitude   float64        `json:"delivery_longitude"`
	DeliveryLatitude    float64        `json:"delivery_latitude"`
	DeliveryContact     string         `json:"delivery_contact,omitempty"`
	DeliveryPhone       string         `json:"delivery_phone,omitempty"`
	Distance            int32          `json:"distance"`
	DeliveryFee         int64          `json:"delivery_fee"`
	RiderEarnings       int64          `json:"rider_earnings"`
	FreezeAmount        int64          `json:"freeze_amount,omitempty"`
	ItemCount           int            `json:"item_count,omitempty"`
	Status              string         `json:"status"`
	EstimatedPickupAt   *time.Time     `json:"estimated_pickup_at,omitempty"`
	EstimatedDeliveryAt *time.Time     `json:"estimated_delivery_at,omitempty"`
	PickedAt            *time.Time     `json:"picked_at,omitempty"`
	DeliveredAt         *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	AssignedAt          *time.Time     `json:"assigned_at,omitempty"`
	CompletedAt         *time.Time     `json:"completed_at,omitempty"`
	Notes               string         `json:"notes,omitempty"`
	Items               []deliveryItem `json:"items,omitempty"`
}

type deliveryItem struct {
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

func (server *Server) newDeliveryResponse(ctx context.Context, d db.Delivery) deliveryResponse {
	pickupLng, _ := d.PickupLongitude.Float64Value()
	pickupLat, _ := d.PickupLatitude.Float64Value()
	deliveryLng, _ := d.DeliveryLongitude.Float64Value()
	deliveryLat, _ := d.DeliveryLatitude.Float64Value()

	resp := deliveryResponse{
		ID:                d.ID,
		OrderID:           d.OrderID,
		PickupAddress:     d.PickupAddress,
		PickupLongitude:   pickupLng.Float64,
		PickupLatitude:    pickupLat.Float64,
		DeliveryAddress:   d.DeliveryAddress,
		DeliveryLongitude: deliveryLng.Float64,
		DeliveryLatitude:  deliveryLat.Float64,
		Distance:          d.Distance,
		DeliveryFee:       d.DeliveryFee,
		RiderEarnings:     d.RiderEarnings,
		Status:            d.Status,
		CreatedAt:         d.CreatedAt,
	}

	// 补充信息
	if order, err := server.store.GetOrder(ctx, d.OrderID); err == nil {
		resp.OrderNo = order.OrderNo
		resp.FreezeAmount = logic.OrderFreezeAmount(order)
		if merchant, err := server.store.GetMerchant(ctx, order.MerchantID); err == nil {
			resp.MerchantName = merchant.Name
		}
		resp.Notes = order.Notes.String
	}
	if count, err := server.store.CountOrderItems(ctx, d.OrderID); err == nil {
		resp.ItemCount = int(count)
	}
	if items, err := server.store.ListOrderItemsByOrder(ctx, d.OrderID); err == nil {
		for _, item := range items {
			resp.Items = append(resp.Items, deliveryItem{
				Name:     item.Name,
				Quantity: int32(item.Quantity),
			})
		}
	}

	if d.RiderID.Valid {
		resp.RiderID = &d.RiderID.Int64
	}
	if d.PickupContact.Valid {
		resp.PickupContact = d.PickupContact.String
	}
	if d.PickupPhone.Valid {
		resp.PickupPhone = d.PickupPhone.String
	}
	if d.DeliveryContact.Valid {
		resp.DeliveryContact = d.DeliveryContact.String
	}
	if d.DeliveryPhone.Valid {
		resp.DeliveryPhone = d.DeliveryPhone.String
	}
	if d.EstimatedPickupAt.Valid {
		resp.EstimatedPickupAt = &d.EstimatedPickupAt.Time
	}
	if d.EstimatedDeliveryAt.Valid {
		resp.EstimatedDeliveryAt = &d.EstimatedDeliveryAt.Time
	}
	if d.PickedAt.Valid {
		resp.PickedAt = &d.PickedAt.Time
	}
	if d.DeliveredAt.Valid {
		resp.DeliveredAt = &d.DeliveredAt.Time
	}
	if d.AssignedAt.Valid {
		resp.AssignedAt = &d.AssignedAt.Time
	}
	if d.CompletedAt.Valid {
		resp.CompletedAt = &d.CompletedAt.Time
	}

	return resp
}

// grabOrder godoc
// @Summary 抢单
// @Description 骑手抢单接单，会自动冻结押金并从订单池移除
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param order_id path int true "订单ID" minimum(1)
// @Success 200 {object} deliveryResponse "抢单成功，返回配送单详情"
// @Failure 400 {object} ErrorResponse "参数校验失败或骑手未上线/押金不足/订单已过期"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "高值单资格不足或订单不在服务区域"
// @Failure 404 {object} ErrorResponse "非骑手用户或订单不存在/已被接走"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/grab/:order_id [post]
// @Security BearerAuth
func (server *Server) grabOrder(ctx *gin.Context) {
	var req grabOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.GrabDeliveryOrder(ctx, server.store, logic.GrabOrderInput{
		UserID:            authPayload.UserID,
		OrderID:           req.OrderID,
		MaxDistanceMeters: MaxGrabOrderDistanceMeters,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order := result.Order
	merchant := result.Merchant
	rider := result.Rider
	delivery := result.Delivery

	// 骑手接单后，使用自建 LBS 重新计算更精确的预估送达时间
	// 预估送达时间 = 当前时间 + 骑手到商户时间 + 出餐等待时间 + 商户到顾客配送时间
	estimateResult, err := logic.UpdateDeliveryEstimatedTime(ctx, server.store, logic.DeliveryEstimateInput{
		Delivery:                delivery,
		Rider:                   rider,
		Merchant:                merchant,
		RiderSpeedMetersPerHour: server.config.RiderAverageSpeed,
		MinTotalMinutes:         10,
		MapClient:               server.mapClient,
	})
	if err != nil {
		log.Error().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to update estimated delivery time")
	} else if estimateResult.Skipped {
		log.Debug().Int64("rider_id", rider.ID).Msg("rider location not available, skip estimated time update")
	} else {
		if estimateResult.MapError != nil {
			log.Warn().Err(estimateResult.MapError).Msg("failed to get distance matrix from LBS, fallback to manual estimation")
		}
		log.Info().
			Int64("delivery_id", delivery.ID).
			Int64("rider_id", rider.ID).
			Int("rider_to_merchant_m", estimateResult.RiderToMerchantDistance).
			Int("merchant_to_customer_m", estimateResult.MerchantToCustomerDistance).
			Float64("prepare_wait_min", estimateResult.PrepareWaitMinutes).
			Float64("total_min", estimateResult.TotalMinutes).
			Time("new_estimated_at", estimateResult.NewEstimatedDeliveryAt).
			Msg("✅ updated delivery estimated time after rider accepted")
	}

	// 📢 P1: 异步发送骑手接单通知给商家
	server.sendDeliveryStatusNotification(
		ctx,
		merchant.OwnerUserID,
		req.OrderID,
		delivery.ID,
		"assigned",
		"骑手已接单",
		fmt.Sprintf("订单%s已有骑手接单，正在前往取餐", order.OrderNo),
	)

	// 📢 M8: 实时广播通知取餐点附近骑手：订单已被抢，请从大厅移除
	if server.deliveryBroadcast != nil {
		pickupLng, lngErr := delivery.PickupLongitude.Float64Value()
		pickupLat, latErr := delivery.PickupLatitude.Float64Value()
		if lngErr == nil && latErr == nil {
			_ = server.deliveryBroadcast.BroadcastOrderGone(ctx, order.ID, pickupLat.Float64, pickupLng.Float64)
		}
	}

	// 重新获取更新后的配送单
	updatedDelivery, err := server.store.GetDelivery(ctx, delivery.ID)
	if err != nil {
		// 即使获取失败也返回原结果
		ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, delivery))
		return
	}

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, updatedDelivery))
}

// ==================== 配送状态更新 ====================

type updateDeliveryRequest struct {
	ID int64 `uri:"delivery_id" binding:"required,min=1"`
}

// startPickup godoc
// @Summary 开始取餐
// @Description 骑手开始前往商家取餐。只能在assigned状态下调用
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Success 200 {object} deliveryResponse "状态更新成功"
// @Failure 400 {object} ErrorResponse "参数校验失败或状态不允许"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此配送单"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/start-pickup [post]
// @Security BearerAuth
func (server *Server) startPickup(ctx *gin.Context) {
	var req updateDeliveryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.StartPickup(ctx, server.store, logic.DeliveryStatusInput{
		UserID:     authPayload.UserID,
		DeliveryID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery
	order := result.Order

	// 📢 P1: 异步发送骑手开始取餐通知给用户
	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		updated.OrderID,
		updated.ID,
		"picking",
		"骑手正在取餐",
		fmt.Sprintf("订单%s骑手正在前往商家取餐", order.OrderNo),
	)

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, updated))
}

// confirmPickup godoc
// @Summary 确认取餐
// @Description 骑手确认已从商家取到餐品。只能在picking状态下调用
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Success 200 {object} deliveryResponse "状态更新成功"
// @Failure 400 {object} ErrorResponse "参数校验失败或状态不允许"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此配送单"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/confirm-pickup [post]
// @Security BearerAuth
func (server *Server) confirmPickup(ctx *gin.Context) {
	var req updateDeliveryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.ConfirmPickup(ctx, server.store, logic.DeliveryStatusInput{
		UserID:     authPayload.UserID,
		DeliveryID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery
	order := result.Order

	// 📢 P1: 异步发送骑手已取餐通知给用户
	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		updated.OrderID,
		updated.ID,
		"picked",
		"骑手已取餐",
		fmt.Sprintf("订单%s骑手已取到餐品，即将配送", order.OrderNo),
	)

	// 📦 合规：骑手取货即视为「货已发出」，异步上报微信发货信息
	server.uploadShippingInfoAsync(ctx, order.ID, order.UserID)

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, updated))
}

// startDelivery godoc
// @Summary 开始配送
// @Description 骑手开始配送餐品给顾客。只能在picked状态下调用
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Success 200 {object} deliveryResponse "状态更新成功"
// @Failure 400 {object} ErrorResponse "参数校验失败或状态不允许"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此配送单"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/start-delivery [post]
// @Security BearerAuth
func (server *Server) startDelivery(ctx *gin.Context) {
	var req updateDeliveryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.StartDelivery(ctx, server.store, logic.DeliveryStatusInput{
		UserID:     authPayload.UserID,
		DeliveryID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery
	order := result.Order

	// 📢 P1: 异步发送骑手配送中通知给用户
	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		updated.OrderID,
		updated.ID,
		"delivering",
		"骑手配送中",
		fmt.Sprintf("订单%s骑手正在配送途中，请保持电话畅通", order.OrderNo),
	)

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, updated))
}

// confirmDelivery godoc
// @Summary 确认送达
// @Description 骑手确认已将餐品送达给顾客，会自动解冻押金并结算配送费。只能在delivering状态下调用
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Success 200 {object} deliveryResponse "送达成功"
// @Failure 400 {object} ErrorResponse "参数校验失败或状态不允许"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作此配送单"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/confirm-delivery [post]
// @Security BearerAuth
func (server *Server) confirmDelivery(ctx *gin.Context) {
	var req updateDeliveryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.ConfirmDelivery(ctx, server.store, logic.ConfirmDeliveryInput{
		UserID:              authPayload.UserID,
		DeliveryID:          req.ID,
		ConfirmRadiusMeters: DeliveryConfirmRadiusMeters,
		LocationMaxAgeSec:   DeliveryConfirmLocationMaxAgeSec,
	})
	if err != nil {
		var confirmErr *logic.DeliveryConfirmValidationError
		if errors.As(err, &confirmErr) {
			log.Warn().
				Err(err).
				Int64("delivery_id", req.ID).
				Int64("user_id", authPayload.UserID).
				Str("reason", confirmErr.Reason).
				Int("distance_m", confirmErr.DistanceMeters).
				Int("radius_m", confirmErr.RadiusMeters).
				Int("location_age_s", confirmErr.LocationAgeSec).
				Int("location_max_age_s", confirmErr.MaxAgeSec).
				Msg("delivery confirm validation failed")
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order := result.Order
	updated := result.Delivery

	// 📢 P1: 异步发送送达通知给用户
	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		updated.OrderID,
		updated.ID,
		"delivered",
		"订单已送达",
		fmt.Sprintf("您的订单%s已送达，请确认收餐", order.OrderNo),
	)

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, updated))
}

// ==================== 顾客查询配送状态 ====================

type getDeliveryRequest struct {
	OrderID int64 `uri:"order_id" binding:"required,min=1"`
}

// getDeliveryByOrder godoc
// @Summary 根据订单查询配送信息
// @Description 获取指定订单的配送信息，仅订单所有者可查看
// @Tags 配送管理-顾客
// @Accept json
// @Produce json
// @Param order_id path int true "订单ID" minimum(1)
// @Success 200 {object} deliveryResponse "配送单详情"
// @Failure 400 {object} ErrorResponse "参数校验失败"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权查看此订单配送信息"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/order/:order_id [get]
// @Security BearerAuth
func (server *Server) getDeliveryByOrder(ctx *gin.Context) {
	var req getDeliveryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	delivery, err := logic.GetDeliveryForViewerByOrder(ctx, server.store, logic.DeliveryOrderViewerInput{
		UserID:           authPayload.UserID,
		OrderID:          req.OrderID,
		ForbiddenMessage: "无权查看此订单配送信息",
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, server.newDeliveryResponse(ctx, delivery))
}

// ==================== 骑手轨迹 ====================

type locationResponse struct {
	Longitude  float64   `json:"longitude"`
	Latitude   float64   `json:"latitude"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// getDeliveryTrack godoc
// @Summary 获取配送轨迹
// @Description 获取骑手配送过程中的位置历史，仅订单所有者或配送骑手可查看
// @Tags 配送管理-顾客
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Param since query string false "获取指定时间之后的位置" format(date-time)
// @Success 200 {array} locationResponse "位置历史列表"
// @Failure 400 {object} ErrorResponse "参数校验失败"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权查看此配送单轨迹"
// @Failure 404 {object} ErrorResponse "配送单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/track [get]
// @Security BearerAuth
func (server *Server) getDeliveryTrack(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"delivery_id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	_, err := logic.ValidateDeliveryViewer(ctx, server.store, logic.DeliveryViewerInput{
		UserID:           authPayload.UserID,
		DeliveryID:       uriReq.ID,
		ForbiddenMessage: "无权查看此配送单轨迹",
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var queryReq struct {
		Since time.Time `form:"since"`
	}
	_ = ctx.ShouldBindQuery(&queryReq)

	var locations []db.RiderLocation

	if queryReq.Since.IsZero() {
		locations, err = server.store.ListDeliveryLocations(ctx, pgtype.Int8{Int64: uriReq.ID, Valid: true})
	} else {
		locations, err = server.store.ListDeliveryLocationsSince(ctx, db.ListDeliveryLocationsSinceParams{
			DeliveryID: pgtype.Int8{Int64: uriReq.ID, Valid: true},
			RecordedAt: queryReq.Since,
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []locationResponse
	for _, loc := range locations {
		lng, _ := loc.Longitude.Float64Value()
		lat, _ := loc.Latitude.Float64Value()

		item := locationResponse{
			Longitude:  lng.Float64,
			Latitude:   lat.Float64,
			RecordedAt: loc.RecordedAt,
		}

		if loc.Accuracy.Valid {
			acc, _ := loc.Accuracy.Float64Value()
			item.Accuracy = &acc.Float64
		}
		if loc.Speed.Valid {
			spd, _ := loc.Speed.Float64Value()
			item.Speed = &spd.Float64
		}
		if loc.Heading.Valid {
			hdg, _ := loc.Heading.Float64Value()
			item.Heading = &hdg.Float64
		}

		response = append(response, item)
	}

	ctx.JSON(http.StatusOK, response)
}

// getRiderLatestLocation godoc
// @Summary 获取骑手最新位置
// @Description 获取配送骑手的最新位置，仅订单所有者或配送骑手可查看
// @Tags 配送管理-顾客
// @Accept json
// @Produce json
// @Param delivery_id path int true "配送单ID" minimum(1)
// @Success 200 {object} locationResponse "骑手最新位置"
// @Failure 400 {object} ErrorResponse "参数校验失败"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权查看此配送单位置"
// @Failure 404 {object} ErrorResponse "配送单不存在或无位置信息"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/:delivery_id/rider-location [get]
// @Security BearerAuth
func (server *Server) getRiderLatestLocation(ctx *gin.Context) {
	var req struct {
		DeliveryID int64 `uri:"delivery_id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	_, err := logic.ValidateDeliveryViewer(ctx, server.store, logic.DeliveryViewerInput{
		UserID:           authPayload.UserID,
		DeliveryID:       req.DeliveryID,
		ForbiddenMessage: "无权查看此配送单位置",
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	location, err := server.store.GetDeliveryLatestLocation(ctx, pgtype.Int8{Int64: req.DeliveryID, Valid: true})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrNoLocationAvailable))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	lng, _ := location.Longitude.Float64Value()
	lat, _ := location.Latitude.Float64Value()

	resp := locationResponse{
		Longitude:  lng.Float64,
		Latitude:   lat.Float64,
		RecordedAt: location.RecordedAt,
	}

	if location.Accuracy.Valid {
		acc, _ := location.Accuracy.Float64Value()
		resp.Accuracy = &acc.Float64
	}
	if location.Speed.Valid {
		spd, _ := location.Speed.Float64Value()
		resp.Speed = &spd.Float64
	}
	if location.Heading.Valid {
		hdg, _ := location.Heading.Float64Value()
		resp.Heading = &hdg.Float64
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 骑手查询自己的配送单 ====================

type listMyDeliveriesResponse struct {
	Deliveries []deliveryResponse `json:"deliveries"`
	Total      int64              `json:"total"`
	PageID     int32              `json:"page_id"`
	PageSize   int32              `json:"page_size"`
}

// listMyDeliveries godoc
// @Summary 查询配送历史
// @Description 获取骑手的配送历史列表，支持状态过滤和分页
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param status query string false "状态过滤" Enums(assigned, picking, picked, delivering, delivered, completed, cancelled)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} listMyDeliveriesResponse "配送单列表"
// @Failure 400 {object} ErrorResponse "参数校验失败"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "非骑手用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/history [get]
// @Security BearerAuth
func (server *Server) listMyDeliveries(ctx *gin.Context) {
	var req struct {
		Status string `form:"status" binding:"omitempty,oneof=assigned picking picked delivering delivered completed cancelled"`
		Page   int32  `form:"page" binding:"min=1"`
		Limit  int32  `form:"limit" binding:"min=1,max=100"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var deliveries []db.Delivery

	if req.Status != "" {
		deliveries, err = server.store.ListDeliveriesByRiderAndStatus(ctx, db.ListDeliveriesByRiderAndStatusParams{
			RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
			Status:  req.Status,
			Limit:   req.Limit,
			Offset:  pageOffset(req.Page, req.Limit),
		})
	} else {
		deliveries, err = server.store.ListDeliveriesByRider(ctx, db.ListDeliveriesByRiderParams{
			RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
			Limit:   req.Limit,
			Offset:  pageOffset(req.Page, req.Limit),
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []deliveryResponse
	for _, d := range deliveries {
		response = append(response, server.newDeliveryResponse(ctx, d))
	}

	ctx.JSON(http.StatusOK, listMyDeliveriesResponse{
		Deliveries: response,
		Total:      int64(len(response)),
		PageID:     req.Page,
		PageSize:   req.Limit,
	})
}

// listMyActiveDeliveries godoc
// @Summary 查询当前活跃配送
// @Description 获取骑手当前正在进行的配送单列表
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Success 200 {array} deliveryResponse "活跃配送单列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "非骑手用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/delivery/active [get]
// @Security BearerAuth
func (server *Server) listMyActiveDeliveries(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	deliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []deliveryResponse
	for _, d := range deliveries {
		response = append(response, server.newDeliveryResponse(ctx, d))
	}

	ctx.JSON(http.StatusOK, response)
}

// 确保 context 包被使用
var _ context.Context
