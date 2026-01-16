package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
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
	if server.taskDistributor == nil {
		return
	}
	expiresAt := time.Now().Add(1 * time.Hour)
	_ = server.taskDistributor.DistributeTaskSendNotification(
		ctx,
		&worker.SendNotificationPayload{
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
		},
		asynq.Queue(worker.QueueDefault),
	)
}

// ==================== 推荐订单 ====================

type getRecommendedOrdersRequest struct {
	Longitude float64 `form:"longitude" binding:"required,gte=-180,lte=180"`
	Latitude  float64 `form:"latitude" binding:"required,gte=-90,lte=90"`
}

type recommendedOrderResponse struct {
	OrderID           int64     `json:"order_id"`
	MerchantID        int64     `json:"merchant_id"`
	TotalScore        int       `json:"total_score"`
	DistanceScore     int       `json:"distance_score"`
	RouteScore        int       `json:"route_score"`
	UrgencyScore      int       `json:"urgency_score"`
	ProfitScore       int       `json:"profit_score"`
	DistanceToPickup  int       `json:"distance_to_pickup"`      // 直线距离（米）
	RealDistance      int       `json:"real_distance,omitempty"` // 真实骑行距离（米）
	EstimatedMinutes  int       `json:"estimated_minutes"`       // 预估时间（分钟）
	RealDuration      int       `json:"real_duration,omitempty"` // 真实骑行时间（秒）
	DeliveryFee       int64     `json:"delivery_fee"`
	Distance          int       `json:"distance"` // 商家到顾客距离
	PickupLongitude   float64   `json:"pickup_longitude"`
	PickupLatitude    float64   `json:"pickup_latitude"`
	DeliveryLongitude float64   `json:"delivery_longitude"`
	DeliveryLatitude  float64   `json:"delivery_latitude"`
	ExpiresAt         time.Time `json:"expires_at"`
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上线")))
		return
	}

	// 检查骑手是否已分配区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您尚未分配服务区域，请联系管理员")))
		return
	}

	// 获取推荐配置
	config := algorithm.DefaultConfig()
	dbConfig, err := server.store.GetActiveRecommendConfig(ctx)
	if err == nil {
		dw, _ := dbConfig.DistanceWeight.Float64Value()
		rw, _ := dbConfig.RouteWeight.Float64Value()
		uw, _ := dbConfig.UrgencyWeight.Float64Value()
		pw, _ := dbConfig.ProfitWeight.Float64Value()
		config.DistanceWeight = dw.Float64
		config.RouteWeight = rw.Float64
		config.UrgencyWeight = uw.Float64
		config.ProfitWeight = pw.Float64
		config.MaxDistance = int(dbConfig.MaxDistance)
		config.MaxResults = int(dbConfig.MaxResults)
	}

	// 获取可接订单池（按骑手所属区域过滤）
	poolItems, err := server.store.ListDeliveryPoolNearbyByRegion(ctx, db.ListDeliveryPoolNearbyByRegionParams{
		RegionID:    rider.RegionID.Int64,
		RiderLat:    req.Latitude,
		RiderLng:    req.Longitude,
		MaxDistance: float64(config.MaxDistance),
		ResultLimit: 100,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为算法输入格式
	var availablePool []algorithm.PoolOrder
	for _, item := range poolItems {
		pickupLng, _ := item.PickupLongitude.Float64Value()
		pickupLat, _ := item.PickupLatitude.Float64Value()
		deliveryLng, _ := item.DeliveryLongitude.Float64Value()
		deliveryLat, _ := item.DeliveryLatitude.Float64Value()

		availablePool = append(availablePool, algorithm.PoolOrder{
			OrderID:    item.OrderID,
			MerchantID: item.MerchantID,
			PickupLocation: algorithm.Location{
				Longitude: pickupLng.Float64,
				Latitude:  pickupLat.Float64,
			},
			DeliveryLocation: algorithm.Location{
				Longitude: deliveryLng.Float64,
				Latitude:  deliveryLat.Float64,
			},
			Distance:         int(item.Distance),
			DeliveryFee:      item.DeliveryFee,
			ExpectedPickupAt: item.ExpectedPickupAt,
			ExpiresAt:        item.ExpiresAt,
			Priority:         int(item.Priority),
			CreatedAt:        item.CreatedAt,
		})
	}

	// 获取骑手当前活跃订单
	var activeOrders []algorithm.ActiveDelivery
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err == nil {
		for _, d := range activeDeliveries {
			pickupLng, _ := d.PickupLongitude.Float64Value()
			pickupLat, _ := d.PickupLatitude.Float64Value()
			deliveryLng, _ := d.DeliveryLongitude.Float64Value()
			deliveryLat, _ := d.DeliveryLatitude.Float64Value()

			ad := algorithm.ActiveDelivery{
				DeliveryID: d.ID,
				OrderID:    d.OrderID,
				PickupLocation: algorithm.Location{
					Longitude: pickupLng.Float64,
					Latitude:  pickupLat.Float64,
				},
				DeliveryLocation: algorithm.Location{
					Longitude: deliveryLng.Float64,
					Latitude:  deliveryLat.Float64,
				},
				Status: d.Status,
			}
			if d.PickedAt.Valid {
				ad.PickedAt = d.PickedAt.Time
			}
			activeOrders = append(activeOrders, ad)
		}
	}

	// 调用推荐算法
	recommender := algorithm.NewSimpleRecommender()
	input := algorithm.RecommendInput{
		RiderID: rider.ID,
		RiderLocation: algorithm.Location{
			Longitude: req.Longitude,
			Latitude:  req.Latitude,
		},
		ActiveOrders:  activeOrders,
		AvailablePool: availablePool,
		Config:        config,
	}

	scored, err := recommender.Recommend(ctx, input)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 对 Top N 个订单调用自建 OSM 获取真实骑行距离
	realDistances := server.enrichWithRealDistance(ctx, req.Latitude, req.Longitude, scored)

	// 转换响应
	var response []recommendedOrderResponse
	for _, s := range scored {
		resp := recommendedOrderResponse{
			OrderID:           s.OrderID,
			MerchantID:        s.PoolOrder.MerchantID,
			TotalScore:        s.TotalScore,
			DistanceScore:     s.DistanceScore,
			RouteScore:        s.RouteScore,
			UrgencyScore:      s.UrgencyScore,
			ProfitScore:       s.ProfitScore,
			DistanceToPickup:  s.DistanceToPickup,
			EstimatedMinutes:  s.EstimatedMinutes,
			DeliveryFee:       s.PoolOrder.DeliveryFee,
			Distance:          s.PoolOrder.Distance,
			PickupLongitude:   s.PoolOrder.PickupLocation.Longitude,
			PickupLatitude:    s.PoolOrder.PickupLocation.Latitude,
			DeliveryLongitude: s.PoolOrder.DeliveryLocation.Longitude,
			DeliveryLatitude:  s.PoolOrder.DeliveryLocation.Latitude,
			ExpiresAt:         s.PoolOrder.ExpiresAt,
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

// enrichWithRealDistance 对 Top N 订单调用自建 OSM 获取真实骑行距离
// 返回 map[orderID] => RouteResult
func (server *Server) enrichWithRealDistance(ctx context.Context, riderLat, riderLng float64, scored []algorithm.ScoredOrder) map[int64]*maps.RouteResult {
	result := make(map[int64]*maps.RouteResult)

	// 如果没有配置地图客户端，直接返回空
	if server.mapClient == nil {
		return result
	}

	// 只对前 10 个订单计算真实距离（节省 API 调用）
	maxOrders := 10
	if len(scored) < maxOrders {
		maxOrders = len(scored)
	}

	riderLocation := maps.Location{Lat: riderLat, Lng: riderLng}

	for i := 0; i < maxOrders; i++ {
		order := scored[i]
		pickupLocation := maps.Location{
			Lat: order.PoolOrder.PickupLocation.Latitude,
			Lng: order.PoolOrder.PickupLocation.Longitude,
		}

		route, err := server.mapClient.GetBicyclingRoute(ctx, riderLocation, pickupLocation)
		if err != nil {
			log.Warn().Err(err).Int64("order_id", order.OrderID).Msg("failed to get bicycling route")
			continue
		}

		result[order.OrderID] = route
	}

	return result
}

// ==================== 抢单 ====================

type grabOrderRequest struct {
	OrderID int64 `uri:"order_id" binding:"required,min=1"`
}

type deliveryResponse struct {
	ID                  int64      `json:"id"`
	OrderID             int64      `json:"order_id"`
	RiderID             *int64     `json:"rider_id,omitempty"`
	PickupAddress       string     `json:"pickup_address"`
	PickupLongitude     float64    `json:"pickup_longitude"`
	PickupLatitude      float64    `json:"pickup_latitude"`
	PickupContact       string     `json:"pickup_contact,omitempty"`
	PickupPhone         string     `json:"pickup_phone,omitempty"`
	DeliveryAddress     string     `json:"delivery_address"`
	DeliveryLongitude   float64    `json:"delivery_longitude"`
	DeliveryLatitude    float64    `json:"delivery_latitude"`
	DeliveryContact     string     `json:"delivery_contact,omitempty"`
	DeliveryPhone       string     `json:"delivery_phone,omitempty"`
	Distance            int32      `json:"distance"`
	DeliveryFee         int64      `json:"delivery_fee"`
	RiderEarnings       int64      `json:"rider_earnings"`
	Status              string     `json:"status"`
	EstimatedPickupAt   *time.Time `json:"estimated_pickup_at,omitempty"`
	EstimatedDeliveryAt *time.Time `json:"estimated_delivery_at,omitempty"`
	PickedAt            *time.Time `json:"picked_at,omitempty"`
	DeliveredAt         *time.Time `json:"delivered_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	AssignedAt          *time.Time `json:"assigned_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

func newDeliveryResponse(d db.Delivery) deliveryResponse {
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上线")))
		return
	}

	// 检查骑手是否已分配区域
	if !rider.RegionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您尚未分配服务区域，请联系管理员")))
		return
	}

	// 检查押金余额
	minDeposit := int64(5000) // 接单需要 50 元押金
	availableDeposit := rider.DepositAmount - rider.FrozenDeposit
	if availableDeposit < minDeposit {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("押金余额不足，无法接单")))
		return
	}

	// 检查订单是否在池中（先获取用于高值单校验）
	poolItem, err := server.store.GetDeliveryPoolByOrderID(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在或已被接走")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否过期
	if poolItem.ExpiresAt.Before(time.Now()) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("订单已过期")))
		return
	}

	// 高值单资格校验：运费 >= 10 元（1000分）的订单需要骑手高值单资格积分 >= 0
	highValueThreshold := int64(1000) // 10 元 = 1000 分
	isHighValueOrder := poolItem.DeliveryFee >= highValueThreshold

	// 获取骑手高值单资格积分
	premiumScore, err := server.store.GetRiderPremiumScore(ctx, rider.ID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		// 如果rider_profiles不存在，默认积分为0
		premiumScore = 0
	}

	if isHighValueOrder && premiumScore < 0 {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("高值单资格积分不足，无法接取高值单（运费≥10元），请先完成普通订单积累积分")))
		return
	}

	// 检查订单是否在骑手服务区域内
	merchant, err := server.store.GetMerchant(ctx, poolItem.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if merchant.RegionID != rider.RegionID.Int64 {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("该订单不在您的服务区域内")))
		return
	}

	// 获取配送单
	delivery, err := server.store.GetDeliveryByOrderID(ctx, req.OrderID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单用于状态同步与日志记录
	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	oldStatus := order.Status

	// 使用事务执行抢单操作：分配骑手 + 移除订单池 + 冻结押金 + 创建流水
	freezeAmount := int64(5000) // 冻结 50 元
	result, err := server.store.GrabOrderTx(ctx, db.GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		OrderID:      req.OrderID,
		FreezeAmount: freezeAmount,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 同步订单状态为骑手已接单（忽略状态不匹配）
	if _, err := server.store.UpdateOrderToCourierAccepted(ctx, req.OrderID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if oldStatus != OrderStatusCourierAccepted {
		if _, err := server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     OrderStatusCourierAccepted,
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手接单", Valid: true},
		}); err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("failed to create order status log for courier_accepted")
		}
	}

	order.Status = OrderStatusCourierAccepted

	// 骑手接单后，使用自建 LBS 重新计算更精确的预估送达时间
	// 预估送达时间 = 当前时间 + 骑手到商户时间 + 出餐等待时间 + 商户到顾客配送时间
	server.updateDeliveryEstimatedTime(ctx, result.Delivery, rider, merchant)

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

	// 重新获取更新后的配送单
	updatedDelivery, err := server.store.GetDelivery(ctx, result.Delivery.ID)
	if err != nil {
		// 即使获取失败也返回原结果
		ctx.JSON(http.StatusOK, newDeliveryResponse(result.Delivery))
		return
	}

	ctx.JSON(http.StatusOK, newDeliveryResponse(updatedDelivery))
}

// updateDeliveryEstimatedTime 骑手接单后重新计算预估送达时间
// 使用自建 LBS 计算真实骑行距离，预估时间 = 骑手到商户时间 + 出餐等待时间 + 商户到顾客配送时间
func (server *Server) updateDeliveryEstimatedTime(ctx context.Context, delivery db.Delivery, rider db.Rider, merchant db.Merchant) {
	const riderSpeedMetersPerHour = 15000 // 骑手平均速度：15km/h

	// 检查骑手位置是否有效
	if !rider.CurrentLatitude.Valid || !rider.CurrentLongitude.Valid {
		log.Debug().Int64("rider_id", rider.ID).Msg("rider location not available, skip estimated time update")
		return
	}

	riderLat, _ := rider.CurrentLatitude.Float64Value()
	riderLng, _ := rider.CurrentLongitude.Float64Value()
	merchantLat, _ := merchant.Latitude.Float64Value()
	merchantLng, _ := merchant.Longitude.Float64Value()
	deliveryLat, _ := delivery.DeliveryLatitude.Float64Value()
	deliveryLng, _ := delivery.DeliveryLongitude.Float64Value()

	// 计算骑手到商户的距离（米）
	var riderToMerchantDistance int
	// 计算商户到顾客的距离（米）
	var merchantToCustomerDistance int = int(delivery.Distance) // 使用已存储的距离

	// 使用自建 LBS 计算真实骑行距离
	if server.mapClient != nil {
		riderLoc := maps.Location{Lat: riderLat.Float64, Lng: riderLng.Float64}
		merchantLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
		deliveryLoc := maps.Location{Lat: deliveryLat.Float64, Lng: deliveryLng.Float64}

		// 批量查询：骑手→商户，商户→顾客
		froms := []maps.Location{riderLoc, merchantLoc}
		tos := []maps.Location{merchantLoc, deliveryLoc}

		result, err := server.mapClient.GetDistanceMatrix(ctx, froms, tos, "bicycling")
		if err == nil && len(result.Rows) >= 2 {
			if len(result.Rows[0].Elements) > 0 {
				riderToMerchantDistance = result.Rows[0].Elements[0].Distance
			}
			if len(result.Rows[1].Elements) > 0 {
				merchantToCustomerDistance = result.Rows[1].Elements[0].Distance
			}
		} else {
			log.Warn().Err(err).Msg("failed to get distance matrix from LBS, using stored distance")
		}
	}

	// 如果LBS调用失败，使用直线距离估算骑手到商户距离
	if riderToMerchantDistance == 0 {
		riderToMerchantDistance = algorithm.HaversineDistance(
			algorithm.Location{Latitude: riderLat.Float64, Longitude: riderLng.Float64},
			algorithm.Location{Latitude: merchantLat.Float64, Longitude: merchantLng.Float64},
		)
		// 直线距离乘以1.3作为实际骑行距离估算
		riderToMerchantDistance = int(float64(riderToMerchantDistance) * 1.3)
	}

	// 计算时间（分钟）
	// 骑手到商户时间
	riderToMerchantMinutes := float64(riderToMerchantDistance) / float64(riderSpeedMetersPerHour) * 60
	// 商户到顾客时间
	merchantToCustomerMinutes := float64(merchantToCustomerDistance) / float64(riderSpeedMetersPerHour) * 60

	// 出餐等待时间：使用已有的预估出餐时间减去当前时间（如果还没出餐）
	var prepareWaitMinutes float64 = 0
	if delivery.EstimatedPickupAt.Valid {
		waitDuration := time.Until(delivery.EstimatedPickupAt.Time)
		if waitDuration > 0 {
			prepareWaitMinutes = waitDuration.Minutes()
		}
	}

	// 总配送时间 = 骑手到商户 + 出餐等待 + 商户到顾客
	totalMinutes := riderToMerchantMinutes + prepareWaitMinutes + merchantToCustomerMinutes
	// 最少配送时间：10分钟
	if totalMinutes < 10 {
		totalMinutes = 10
	}

	// 新的预估送达时间
	newEstimatedDeliveryAt := time.Now().Add(time.Duration(totalMinutes) * time.Minute)

	// 更新配送单的预估送达时间
	_, err := server.store.UpdateDeliveryEstimatedTime(ctx, db.UpdateDeliveryEstimatedTimeParams{
		ID:                  delivery.ID,
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: newEstimatedDeliveryAt, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to update estimated delivery time")
		return
	}

	log.Info().
		Int64("delivery_id", delivery.ID).
		Int64("rider_id", rider.ID).
		Int("rider_to_merchant_m", riderToMerchantDistance).
		Int("merchant_to_customer_m", merchantToCustomerDistance).
		Float64("prepare_wait_min", prepareWaitMinutes).
		Float64("total_min", totalMinutes).
		Time("new_estimated_at", newEstimatedDeliveryAt).
		Msg("✅ updated delivery estimated time after rider accepted")
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 先获取配送单检查状态和归属
	delivery, err := server.store.GetDelivery(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否是该骑手的配送单
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权操作此配送单")))
		return
	}

	// 检查状态是否允许开始取餐（只有assigned状态可以）
	if delivery.Status != "assigned" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("当前状态(%s)不允许开始取餐", delivery.Status)))
		return
	}

	result, err := server.store.UpdateDeliveryToPickupTx(ctx, db.UpdateDeliveryToPickupTxParams{
		DeliveryID: req.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery

	// 📢 P1: 异步发送骑手开始取餐通知给用户
	if order, err := server.store.GetOrder(ctx, updated.OrderID); err == nil {
		server.sendDeliveryStatusNotification(
			ctx,
			order.UserID,
			updated.OrderID,
			updated.ID,
			"picking",
			"骑手正在取餐",
			fmt.Sprintf("订单%s骑手正在前往商家取餐", order.OrderNo),
		)
	}

	ctx.JSON(http.StatusOK, newDeliveryResponse(updated))
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 先获取配送单检查状态和归属
	delivery, err := server.store.GetDelivery(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否是该骑手的配送单
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权操作此配送单")))
		return
	}

	// 检查状态是否允许确认取餐（只有picking状态可以）
	if delivery.Status != "picking" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("当前状态(%s)不允许确认取餐", delivery.Status)))
		return
	}

	order, err := server.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	oldStatus := order.Status

	result, err := server.store.UpdateDeliveryToPickedTx(ctx, db.UpdateDeliveryToPickedTxParams{
		DeliveryID: req.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery

	if oldStatus != OrderStatusPicked {
		if _, err := server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     OrderStatusPicked,
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手确认取餐", Valid: true},
		}); err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("failed to create order status log for picked")
		}
	}

	order.Status = OrderStatusPicked

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

	ctx.JSON(http.StatusOK, newDeliveryResponse(updated))
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 先获取配送单检查状态和归属
	delivery, err := server.store.GetDelivery(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否是该骑手的配送单
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权操作此配送单")))
		return
	}

	// 检查状态是否允许开始配送（只有picked状态可以）
	if delivery.Status != "picked" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("当前状态(%s)不允许开始配送", delivery.Status)))
		return
	}

	order, err := server.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	oldStatus := order.Status

	result, err := server.store.UpdateDeliveryToDeliveringTx(ctx, db.UpdateDeliveryToDeliveringTxParams{
		DeliveryID: req.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated := result.Delivery

	if oldStatus != OrderStatusDelivering {
		if _, err := server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     OrderStatusDelivering,
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手开始配送", Valid: true},
		}); err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("failed to create order status log for delivering")
		}
	}

	order.Status = OrderStatusDelivering

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

	ctx.JSON(http.StatusOK, newDeliveryResponse(updated))
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

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	delivery, err := server.store.GetDelivery(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权操作此配送单")))
		return
	}

	order, err := server.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	oldStatus := order.Status

	// 使用事务执行送达操作：更新状态 + 解冻押金 + 创建流水 + 更新统计
	unfreezeAmount := int64(5000)
	result, err := server.store.CompleteDeliveryTx(ctx, db.CompleteDeliveryTxParams{
		DeliveryID:     req.ID,
		RiderID:        rider.ID,
		OrderID:        delivery.OrderID,
		UnfreezeAmount: unfreezeAmount,
		DeliveryFee:    delivery.DeliveryFee,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if oldStatus != OrderStatusRiderDelivered {
		if _, err := server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     OrderStatusRiderDelivered,
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手确认送达", Valid: true},
		}); err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("failed to create order status log for rider_delivered")
		}
	}

	order.Status = OrderStatusRiderDelivered

	// 📢 P1: 异步发送送达通知给用户
	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		delivery.OrderID,
		req.ID,
		"delivered",
		"订单已送达",
		fmt.Sprintf("您的订单%s已送达，请确认收餐", order.OrderNo),
	)

	ctx.JSON(http.StatusOK, newDeliveryResponse(result.Delivery))
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

	// 获取订单，验证归属权
	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("订单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 只有订单所有者可以查看配送信息
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权查看此订单配送信息")))
		return
	}

	delivery, err := server.store.GetDeliveryByOrderID(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newDeliveryResponse(delivery))
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

	// 获取配送单信息
	delivery, err := server.store.GetDelivery(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：只有订单所有者或配送骑手可以查看
	order, err := server.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否是订单所有者
	isOrderOwner := order.UserID == authPayload.UserID

	// 检查是否是配送骑手（只有非订单所有者才需要检查）
	isDeliveryRider := false
	if !isOrderOwner && delivery.RiderID.Valid {
		rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
		if err == nil && rider.ID == delivery.RiderID.Int64 {
			isDeliveryRider = true
		}
	}

	if !isOrderOwner && !isDeliveryRider {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权查看此配送单轨迹")))
		return
	}

	var queryReq struct {
		Since time.Time `form:"since"`
	}
	ctx.ShouldBindQuery(&queryReq)

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

	// 获取配送单信息
	delivery, err := server.store.GetDelivery(ctx, req.DeliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("配送单不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：只有订单所有者或配送骑手可以查看
	order, err := server.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否是订单所有者
	isOrderOwner := order.UserID == authPayload.UserID

	// 检查是否是配送骑手（只有非订单所有者才需要检查）
	isDeliveryRider := false
	if !isOrderOwner && delivery.RiderID.Valid {
		rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
		if err == nil && rider.ID == delivery.RiderID.Int64 {
			isDeliveryRider = true
		}
	}

	if !isOrderOwner && !isDeliveryRider {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权查看此配送单位置")))
		return
	}

	location, err := server.store.GetDeliveryLatestLocation(ctx, pgtype.Int8{Int64: req.DeliveryID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("暂无位置信息")))
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

// listMyDeliveries godoc
// @Summary 查询配送历史
// @Description 获取骑手的配送历史列表，支持状态过滤和分页
// @Tags 配送管理-骑手
// @Accept json
// @Produce json
// @Param status query string false "状态过滤" Enums(assigned, picking, picked, delivering, delivered, completed, cancelled)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {array} deliveryResponse "配送单列表"
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
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
			Offset:  (req.Page - 1) * req.Limit,
		})
	} else {
		deliveries, err = server.store.ListDeliveriesByRider(ctx, db.ListDeliveriesByRiderParams{
			RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
			Limit:   req.Limit,
			Offset:  (req.Page - 1) * req.Limit,
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []deliveryResponse
	for _, d := range deliveries {
		response = append(response, newDeliveryResponse(d))
	}

	ctx.JSON(http.StatusOK, response)
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
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
		response = append(response, newDeliveryResponse(d))
	}

	ctx.JSON(http.StatusOK, response)
}

// 确保 context 包被使用
var _ context.Context
