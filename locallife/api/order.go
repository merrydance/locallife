package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// ==================== 订单管理 ====================

// 订单类型常量
const (
	OrderTypeTakeout     = "takeout"     // 外卖
	OrderTypeDineIn      = "dine_in"     // 堂食
	OrderTypeTakeaway    = "takeaway"    // 打包自取
	OrderTypeReservation = "reservation" // 预定包间点菜
)

// 订单状态常量
const (
	OrderStatusPending    = "pending"    // 待支付
	OrderStatusPaid       = "paid"       // 已支付
	OrderStatusPreparing  = "preparing"  // 制作中
	OrderStatusReady      = "ready"      // 待取餐/待配送
	OrderStatusDelivering = "delivering" // 配送中
	OrderStatusCompleted  = "completed"  // 已完成
	OrderStatusCancelled  = "cancelled"  // 已取消
)

// 支付方式常量
const (
	PaymentMethodWechat  = "wechat"
	PaymentMethodBalance = "balance"
)

// 配送距离常量（单位：米）
const (
	DefaultDeliveryDistance = 3000 // 默认配送距离 3km
	MinDeliveryDistance     = 500  // 最小配送距离 500m
)

// 地理计算常量（用于经纬度转换为米）
const (
	MetersPerLatDegree = 111000 // 1度纬度约等于111公里
	MetersPerLngDegree = 96000  // 1度经度在纬度30度左右约等于96公里
)

// ==================== 请求/响应结构体 ====================

type orderItemRequest struct {
	// 菜品ID (dish_id和combo_id二选一，不能同时为空)
	DishID *int64 `json:"dish_id,omitempty" binding:"omitempty,min=1" example:"1001"`

	// 套餐ID (dish_id和combo_id二选一，不能同时为空)
	ComboID *int64 `json:"combo_id,omitempty" binding:"omitempty,min=1" example:"2001"`

	// 数量 (必填，范围：1-99)
	Quantity int16 `json:"quantity" binding:"required,min=1,max=99" example:"2"`

	// 定制化选项列表 (选填，如规格、口味等，最多10个)
	Customizations []orderCustomizationItem `json:"customizations,omitempty" binding:"omitempty,max=10,dive"`
}

type orderCustomizationItem struct {
	// 定制项名称 (如: "辣度", "甜度", "规格"，最多50字符)
	Name string `json:"name" binding:"required,max=50" example:"辣度"`

	// 定制项取值 (如: "微辣", "中辣", "特辣"，最多50字符)
	Value string `json:"value" binding:"required,max=50" example:"中辣"`

	// 额外价格 (单位：分，0表示不加价，最大100元)
	ExtraPrice int64 `json:"extra_price" binding:"min=0,max=10000" example:"0"`
}

type createOrderRequest struct {
	// 商户ID (必填)
	MerchantID int64 `json:"merchant_id" binding:"required,min=1" example:"10001"`

	// 订单类型 (必填，枚举值: takeout-外卖, dine_in-堂食, takeaway-打包自取, reservation-预定点菜)
	OrderType string `json:"order_type" binding:"required,oneof=takeout dine_in takeaway reservation" enums:"takeout,dine_in,takeaway,reservation" example:"takeout"`

	// 配送地址ID (外卖订单必填)
	AddressID *int64 `json:"address_id,omitempty" binding:"omitempty,min=1" example:"5001"`

	// 桌台ID (堂食订单必填)
	TableID *int64 `json:"table_id,omitempty" binding:"omitempty,min=1" example:"301"`

	// 预订ID (预定点菜时必填)
	ReservationID *int64 `json:"reservation_id,omitempty" binding:"omitempty,min=1" example:"8001"`

	// 订单商品列表 (必填，至少包含1个商品，最多50个)
	Items []orderItemRequest `json:"items" binding:"required,min=1,max=50,dive"`

	// 订单备注 (选填，如忌口、特殊要求等，最多500字符)
	Notes string `json:"notes,omitempty" binding:"max=500" example:"不要香菜"`

	// 用户优惠券ID (选填，使用已领取的优惠券抵扣)
	UserVoucherID *int64 `json:"user_voucher_id,omitempty" binding:"omitempty,min=1" example:"9001"`

	// 是否使用会员余额支付 (选填，仅堂食和自提支持)
	UseBalance bool `json:"use_balance,omitempty" example:"false"`
}

type orderItemResponse struct {
	// 订单明细ID
	ID int64 `json:"id" example:"20001"`

	// 菜品ID (菜品订单时有值)
	DishID *int64 `json:"dish_id,omitempty" example:"1001"`

	// 套餐ID (套餐订单时有值)
	ComboID *int64 `json:"combo_id,omitempty" example:"2001"`

	// 商品名称
	Name string `json:"name" example:"宫保鸡丁"`

	// 单价 (单位：分)
	UnitPrice int64 `json:"unit_price" example:"2880"`

	// 数量
	Quantity int16 `json:"quantity" example:"2"`

	// 小计金额 (单位：分，含定制化加价)
	Subtotal int64 `json:"subtotal" example:"5760"`

	// 定制化选项列表
	Customizations []orderCustomizationItem `json:"customizations,omitempty"`

	// 商品图片URL
	ImageURL *string `json:"image_url,omitempty" example:"https://example.com/images/dish1001.jpg"`
}

type orderResponse struct {
	// 订单ID
	ID int64 `json:"id" example:"100001"`

	// 订单编号 (唯一订单号)
	OrderNo string `json:"order_no" example:"ORD20251201123456"`

	// 用户ID
	UserID int64 `json:"user_id" example:"10001"`

	// 商户ID
	MerchantID int64 `json:"merchant_id" example:"20001"`

	// 商户名称
	MerchantName string `json:"merchant_name,omitempty" example:"张三餐厅"`

	// 订单类型 (枚举: takeout-外卖, dine_in-堂食, takeaway-打包自取, reservation-预定点菜)
	OrderType string `json:"order_type" enums:"takeout,dine_in,takeaway,reservation" example:"takeout"`

	// 配送地址ID (外卖订单时有值)
	AddressID *int64 `json:"address_id,omitempty" example:"5001"`

	// 配送费 (单位：分)
	DeliveryFee int64 `json:"delivery_fee" example:"500"`

	// 配送距离 (单位：米)
	DeliveryDistance *int32 `json:"delivery_distance,omitempty" example:"2500"`

	// 桌台ID (堂食订单时有值)
	TableID *int64 `json:"table_id,omitempty" example:"301"`

	// 预订ID (预定点菜时有值)
	ReservationID *int64 `json:"reservation_id,omitempty" example:"8001"`

	// 商品小计 (单位：分，不含配送费)
	Subtotal int64 `json:"subtotal" example:"5760"`

	// 优惠金额 (单位：分)
	DiscountAmount int64 `json:"discount_amount" example:"500"`

	// 配送费优惠 (单位：分)
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount" example:"200"`

	// 订单总金额 (单位：分)
	TotalAmount int64 `json:"total_amount" example:"5760"`

	// 订单状态 (枚举: pending-待支付, paid-已支付, preparing-制作中, ready-待取餐/待配送, delivering-配送中, completed-已完成, cancelled-已取消)
	Status string `json:"status" enums:"pending,paid,preparing,ready,delivering,completed,cancelled" example:"paid"`

	// 支付方式 (枚举: wechat-微信支付, balance-余额支付)
	PaymentMethod *string `json:"payment_method,omitempty" enums:"wechat,balance" example:"wechat"`

	// 订单备注
	Notes *string `json:"notes,omitempty" example:"不要香菜"`

	// 订单商品列表
	Items []orderItemResponse `json:"items,omitempty"`

	// 支付时间
	PaidAt *time.Time `json:"paid_at,omitempty" example:"2025-12-01T12:30:00Z"`

	// 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty" example:"2025-12-01T13:15:00Z"`

	// 取消时间
	CancelledAt *time.Time `json:"cancelled_at,omitempty" example:"2025-12-01T12:25:00Z"`

	// 取消原因
	CancelReason *string `json:"cancel_reason,omitempty" example:"商品缺货"`

	// 创建时间
	CreatedAt time.Time `json:"created_at" example:"2025-12-01T12:20:00Z"`

	// 更新时间
	UpdatedAt *time.Time `json:"updated_at,omitempty" example:"2025-12-01T12:30:00Z"`
}

func newOrderResponse(o db.Order) orderResponse {
	resp := orderResponse{
		ID:                  o.ID,
		OrderNo:             o.OrderNo,
		UserID:              o.UserID,
		MerchantID:          o.MerchantID,
		OrderType:           o.OrderType,
		DeliveryFee:         o.DeliveryFee,
		Subtotal:            o.Subtotal,
		DiscountAmount:      o.DiscountAmount,
		DeliveryFeeDiscount: o.DeliveryFeeDiscount,
		TotalAmount:         o.TotalAmount,
		Status:              o.Status,
		CreatedAt:           o.CreatedAt,
	}

	if o.AddressID.Valid {
		resp.AddressID = &o.AddressID.Int64
	}
	if o.DeliveryDistance.Valid {
		resp.DeliveryDistance = &o.DeliveryDistance.Int32
	}
	if o.TableID.Valid {
		resp.TableID = &o.TableID.Int64
	}
	if o.ReservationID.Valid {
		resp.ReservationID = &o.ReservationID.Int64
	}
	if o.PaymentMethod.Valid {
		resp.PaymentMethod = &o.PaymentMethod.String
	}
	if o.Notes.Valid {
		resp.Notes = &o.Notes.String
	}
	if o.PaidAt.Valid {
		resp.PaidAt = &o.PaidAt.Time
	}
	if o.CompletedAt.Valid {
		resp.CompletedAt = &o.CompletedAt.Time
	}
	if o.CancelledAt.Valid {
		resp.CancelledAt = &o.CancelledAt.Time
	}
	if o.CancelReason.Valid {
		resp.CancelReason = &o.CancelReason.String
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}

	return resp
}

func newOrderWithMerchantResponse(o db.ListOrdersByUserRow) orderResponse {
	resp := orderResponse{
		ID:                  o.ID,
		OrderNo:             o.OrderNo,
		UserID:              o.UserID,
		MerchantID:          o.MerchantID,
		MerchantName:        o.MerchantName,
		OrderType:           o.OrderType,
		DeliveryFee:         o.DeliveryFee,
		Subtotal:            o.Subtotal,
		DiscountAmount:      o.DiscountAmount,
		DeliveryFeeDiscount: o.DeliveryFeeDiscount,
		TotalAmount:         o.TotalAmount,
		Status:              o.Status,
		CreatedAt:           o.CreatedAt,
	}

	if o.AddressID.Valid {
		resp.AddressID = &o.AddressID.Int64
	}
	if o.DeliveryDistance.Valid {
		resp.DeliveryDistance = &o.DeliveryDistance.Int32
	}
	if o.TableID.Valid {
		resp.TableID = &o.TableID.Int64
	}
	if o.ReservationID.Valid {
		resp.ReservationID = &o.ReservationID.Int64
	}
	if o.PaymentMethod.Valid {
		resp.PaymentMethod = &o.PaymentMethod.String
	}
	if o.Notes.Valid {
		resp.Notes = &o.Notes.String
	}
	if o.PaidAt.Valid {
		resp.PaidAt = &o.PaidAt.Time
	}
	if o.CompletedAt.Valid {
		resp.CompletedAt = &o.CompletedAt.Time
	}
	if o.CancelledAt.Valid {
		resp.CancelledAt = &o.CancelledAt.Time
	}
	if o.CancelReason.Valid {
		resp.CancelReason = &o.CancelReason.String
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}

	return resp
}

func newOrderWithMerchantFromStatusResponse(o db.ListOrdersByUserAndStatusRow) orderResponse {
	resp := orderResponse{
		ID:                  o.ID,
		OrderNo:             o.OrderNo,
		UserID:              o.UserID,
		MerchantID:          o.MerchantID,
		MerchantName:        o.MerchantName,
		OrderType:           o.OrderType,
		DeliveryFee:         o.DeliveryFee,
		Subtotal:            o.Subtotal,
		DiscountAmount:      o.DiscountAmount,
		DeliveryFeeDiscount: o.DeliveryFeeDiscount,
		TotalAmount:         o.TotalAmount,
		Status:              o.Status,
		CreatedAt:           o.CreatedAt,
	}

	if o.AddressID.Valid {
		resp.AddressID = &o.AddressID.Int64
	}
	if o.DeliveryDistance.Valid {
		resp.DeliveryDistance = &o.DeliveryDistance.Int32
	}
	if o.TableID.Valid {
		resp.TableID = &o.TableID.Int64
	}
	if o.ReservationID.Valid {
		resp.ReservationID = &o.ReservationID.Int64
	}
	if o.PaymentMethod.Valid {
		resp.PaymentMethod = &o.PaymentMethod.String
	}
	if o.Notes.Valid {
		resp.Notes = &o.Notes.String
	}
	if o.PaidAt.Valid {
		resp.PaidAt = &o.PaidAt.Time
	}
	if o.CompletedAt.Valid {
		resp.CompletedAt = &o.CompletedAt.Time
	}
	if o.CancelledAt.Valid {
		resp.CancelledAt = &o.CancelledAt.Time
	}
	if o.CancelReason.Valid {
		resp.CancelReason = &o.CancelReason.String
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}

	return resp
}

// generateOrderNo 生成订单号
func generateOrderNo() string {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	// 生成6位随机数
	b := make([]byte, 3)
	rand.Read(b)
	randomNum := fmt.Sprintf("%06d", int(b[0])*10000+int(b[1])*100+int(b[2]))

	return dateStr + randomNum[:6]
}

// ==================== 订单API ====================

// createOrder godoc
// @Summary 创建订单
// @Description 创建新订单，支持四种类型：外卖(takeout)、堂食(dine_in)、打包自取(takeaway)、预定点菜(reservation)。
// @Description
// @Description **订单类型与必填字段：**
// @Description - takeout: 必须提供address_id
// @Description - dine_in: 必须提供table_id
// @Description - takeaway: 无额外必填字段
// @Description - reservation: 必须提供reservation_id
// @Description
// @Description **安全限制：**
// @Description - 外卖订单的地址必须属于当前用户
// @Description - 堂食订单的桌台必须属于指定商户
// @Description - 商户必须处于active状态才能下单
// @Description - 订单中的菜品必须在线且可售
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param request body createOrderRequest true "订单创建参数"
// @Success 200 {object} orderResponse "创建成功"
// @Failure 400 {object} ErrorResponse "请求参数错误 / 商户未激活 / 菜品下线 / 桌台不属于该商户"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "地址不属于当前用户"
// @Failure 404 {object} ErrorResponse "商户/地址/桌台/菜品不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders [post]
// @Security BearerAuth
func (server *Server) createOrder(ctx *gin.Context) {
	var req createOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("[DEBUG] createOrder: binding failed")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	log.Info().
		Int64("merchant_id", req.MerchantID).
		Str("order_type", req.OrderType).
		Int("items_count", len(req.Items)).
		Interface("address_id", req.AddressID).
		Msg("[DEBUG] createOrder: request parsed")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证订单类型与关联字段
	if err := validateOrderTypeFields(req); err != nil {
		log.Warn().Err(err).Msg("[DEBUG] createOrder: validateOrderTypeFields failed")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户存在且正常营业
	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn().Msg("[DEBUG] createOrder: merchant not found")
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if merchant.Status != "active" {
		log.Warn().Str("status", merchant.Status).Msg("[DEBUG] createOrder: merchant not active")
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant is not active")))
		return
	}

	log.Info().Msg("[DEBUG] createOrder: merchant validated")

	// P0安全: 堂食订单验证桌台归属商户
	if req.OrderType == OrderTypeDineIn && req.TableID != nil {
		table, err := server.store.GetTable(ctx, *req.TableID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if table.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table does not belong to this merchant")))
			return
		}
	}

	// 计算订单金额
	subtotal, items, err := server.calculateOrderItems(ctx, req.MerchantID, req.Items)
	if err != nil {
		log.Warn().Err(err).Msg("[DEBUG] createOrder: calculateOrderItems failed")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	log.Info().Int64("subtotal", subtotal).Int("items_count", len(items)).Msg("[DEBUG] createOrder: items calculated")

	// 计算配送费（仅外卖订单）
	var deliveryFee int64 = 0
	var deliveryDistance int32 = 0
	var deliveryFeeDiscount int64 = 0
	if req.OrderType == OrderTypeTakeout && req.AddressID != nil {
		// 获取用户地址
		address, err := server.store.GetUserAddress(ctx, *req.AddressID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("address not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// P0安全: 验证地址属于当前用户
		if address.UserID != authPayload.UserID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("address does not belong to you")))
			return
		}

		// 计算配送距离 - 使用腾讯地图 API 计算骑行距离
		deliveryDistance = DefaultDeliveryDistance
		if address.Latitude.Valid && address.Longitude.Valid && merchant.Latitude.Valid && merchant.Longitude.Valid {
			userLat, _ := address.Latitude.Float64Value()
			userLng, _ := address.Longitude.Float64Value()
			merchantLat, _ := merchant.Latitude.Float64Value()
			merchantLng, _ := merchant.Longitude.Float64Value()

			// 优先使用腾讯地图 API 计算骑行距离
			if server.mapClient != nil {
				fromLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
				toLoc := maps.Location{Lat: userLat.Float64, Lng: userLng.Float64}
				routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
				if err == nil && routeResult != nil {
					deliveryDistance = int32(routeResult.Distance)
				}
			} else {
				// 降级：使用简化的距离计算
				latDiff := (userLat.Float64 - merchantLat.Float64) * MetersPerLatDegree
				lngDiff := (userLng.Float64 - merchantLng.Float64) * MetersPerLngDegree
				deliveryDistance = int32(math.Sqrt(latDiff*latDiff + lngDiff*lngDiff))
			}
			if deliveryDistance < MinDeliveryDistance {
				deliveryDistance = MinDeliveryDistance
			}
		}

		// 调用运费计算服务
		feeResult, err := server.calculateDeliveryFeeInternal(ctx, address.RegionID, req.MerchantID, deliveryDistance, subtotal)
		if err == nil {
			deliveryFee = feeResult.FinalFee
			deliveryFeeDiscount = feeResult.PromotionDiscount
		}
	}

	// 计算满减优惠
	var discountAmount int64 = 0
	discountRules, err := server.store.ListActiveDiscountRules(ctx, req.MerchantID)
	if err == nil {
		// 选择最优满减（金额最大的）
		var bestDiscount db.DiscountRule
		var bestFound bool
		for _, d := range discountRules {
			if subtotal >= d.MinOrderAmount {
				if !bestFound || d.DiscountAmount > bestDiscount.DiscountAmount {
					bestDiscount = d
					bestFound = true
				}
			}
		}
		if bestFound {
			discountAmount = bestDiscount.DiscountAmount
		}
	}

	// ==================== 优惠券验证 ====================
	var userVoucherID *int64
	var voucherAmount int64 = 0

	if req.UserVoucherID != nil {
		// 获取用户优惠券信息（包含优惠券模板信息）
		uv, err := server.store.GetUserVoucher(ctx, *req.UserVoucherID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("优惠券不存在")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 验证优惠券属于当前用户
		if uv.UserID != authPayload.UserID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("优惠券不属于您")))
			return
		}

		// 验证优惠券状态
		if uv.Status != "unused" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已使用或已过期")))
			return
		}

		// 验证优惠券是否过期
		if time.Now().After(uv.ExpiresAt) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已过期")))
			return
		}

		// 验证优惠券属于当前商户（uv已包含MerchantID）
		if uv.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该优惠券不能在此商户使用")))
			return
		}

		// 验证最低消费（uv已包含MinOrderAmount）
		if subtotal < uv.MinOrderAmount {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("未达到最低消费 %d 元", uv.MinOrderAmount/100)))
			return
		}

		// 验证代金券是否适用于当前订单类型
		orderTypeAllowed := false
		for _, allowedType := range uv.AllowedOrderTypes {
			if allowedType == req.OrderType {
				orderTypeAllowed = true
				break
			}
		}
		if !orderTypeAllowed {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该代金券不适用于此订单类型")))
			return
		}

		userVoucherID = req.UserVoucherID
		voucherAmount = uv.Amount
	}

	// ==================== 会员余额验证 ====================
	var membershipID *int64
	var balancePaid int64 = 0
	var membership *db.MerchantMembership

	if req.UseBalance {
		// 验证订单类型是否支持余额支付（仅堂食和自提）
		if req.OrderType != OrderTypeDineIn && req.OrderType != OrderTypeTakeaway {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("外卖和预定订单暂不支持余额支付")))
			return
		}

		// 获取用户在该商户的会员卡
		mem, err := server.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
			MerchantID: req.MerchantID,
			UserID:     authPayload.UserID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您还不是该商户的会员，请先加入会员")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查商户会员设置
		settings, err := server.store.GetMerchantMembershipSettings(ctx, req.MerchantID)
		if err == nil {
			// 检查该场景是否允许使用余额
			sceneAllowed := false
			for _, scene := range settings.BalanceUsableScenes {
				if scene == req.OrderType {
					sceneAllowed = true
					break
				}
			}
			if !sceneAllowed {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("商户设置不允许在此场景使用余额支付")))
				return
			}
		}
		// 如果没有设置，默认允许堂食和自提

		if mem.Balance <= 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("会员余额不足")))
			return
		}

		membershipID = &mem.ID
		membership = &mem
	}

	// 计算总金额（扣除优惠券后）
	totalAmount := subtotal - discountAmount - voucherAmount + deliveryFee - deliveryFeeDiscount
	if totalAmount < 0 {
		totalAmount = 0
	}

	// 计算余额支付金额
	if req.UseBalance && membership != nil {
		// 使用全部余额或订单金额，取较小值
		if membership.Balance >= totalAmount {
			balancePaid = totalAmount
		} else {
			balancePaid = membership.Balance
		}
	}

	// 生成订单号
	orderNo := generateOrderNo()

	// 构建创建参数
	arg := db.CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              authPayload.UserID,
		MerchantID:          req.MerchantID,
		OrderType:           req.OrderType,
		DeliveryFee:         deliveryFee,
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		DeliveryFeeDiscount: deliveryFeeDiscount,
		TotalAmount:         totalAmount,
		Status:              OrderStatusPending,
	}

	// 设置可选字段
	if req.AddressID != nil {
		arg.AddressID = pgtype.Int8{Int64: *req.AddressID, Valid: true}
	}
	if deliveryDistance > 0 {
		arg.DeliveryDistance = pgtype.Int4{Int32: deliveryDistance, Valid: true}
	}
	if req.TableID != nil {
		arg.TableID = pgtype.Int8{Int64: *req.TableID, Valid: true}
	}
	if req.ReservationID != nil {
		arg.ReservationID = pgtype.Int8{Int64: *req.ReservationID, Valid: true}
	}
	if req.Notes != "" {
		arg.Notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	// 设置优惠券字段
	if userVoucherID != nil {
		arg.UserVoucherID = pgtype.Int8{Int64: *userVoucherID, Valid: true}
	}
	arg.VoucherAmount = voucherAmount

	// 设置余额支付字段
	if membershipID != nil {
		arg.MembershipID = pgtype.Int8{Int64: *membershipID, Valid: true}
	}
	arg.BalancePaid = balancePaid

	// 创建订单（使用事务，同时处理优惠券核销和余额扣减）
	txResult, err := server.store.CreateOrderTx(ctx, db.CreateOrderTxParams{
		CreateOrderParams: arg,
		Items:             items,
		UserVoucherID:     userVoucherID,
		VoucherAmount:     voucherAmount,
		MembershipID:      membershipID,
		BalancePaid:       balancePaid,
	})
	if err != nil {
		// 处理特定错误
		if errors.Is(err, fmt.Errorf("voucher already used")) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已被使用")))
			return
		}
		if errors.Is(err, fmt.Errorf("voucher has expired")) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已过期")))
			return
		}
		if errors.Is(err, fmt.Errorf("insufficient balance")) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("会员余额不足")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newOrderResponse(txResult.Order)
	ctx.JSON(http.StatusOK, resp)
}

// validateOrderTypeFields 验证订单类型与关联字段
func validateOrderTypeFields(req createOrderRequest) error {
	switch req.OrderType {
	case OrderTypeTakeout:
		if req.AddressID == nil {
			return errors.New("address_id is required for takeout orders")
		}
	case OrderTypeDineIn:
		if req.TableID == nil {
			return errors.New("table_id is required for dine-in orders")
		}
	case OrderTypeReservation:
		if req.ReservationID == nil {
			return errors.New("reservation_id is required for reservation orders")
		}
	case OrderTypeTakeaway:
		// 打包自取不需要额外字段
	}

	// 验证商品项
	for _, item := range req.Items {
		if item.DishID == nil && item.ComboID == nil {
			return errors.New("each item must have either dish_id or combo_id")
		}
		if item.DishID != nil && item.ComboID != nil {
			return errors.New("each item can only have one of dish_id or combo_id")
		}
	}

	return nil
}

// calculateOrderItems 计算订单商品金额
func (server *Server) calculateOrderItems(ctx *gin.Context, merchantID int64, items []orderItemRequest) (int64, []db.CreateOrderItemParams, error) {
	var subtotal int64 = 0
	orderItems := make([]db.CreateOrderItemParams, 0, len(items))

	for _, item := range items {
		var name string
		var unitPrice int64
		var dishID, comboID pgtype.Int8

		if item.DishID != nil {
			// 查询菜品
			dish, err := server.store.GetDish(ctx, *item.DishID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return 0, nil, fmt.Errorf("dish %d not found", *item.DishID)
				}
				return 0, nil, err
			}
			// 验证菜品属于该商户
			if dish.MerchantID != merchantID {
				return 0, nil, fmt.Errorf("dish %d does not belong to this merchant", *item.DishID)
			}
			// 验证菜品上架且可售
			if !dish.IsOnline {
				return 0, nil, fmt.Errorf("dish %s is offline", dish.Name)
			}
			if !dish.IsAvailable {
				return 0, nil, fmt.Errorf("dish %s is not available today", dish.Name)
			}

			name = dish.Name
			unitPrice = dish.Price
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		} else if item.ComboID != nil {
			// 查询套餐
			combo, err := server.store.GetComboSet(ctx, *item.ComboID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return 0, nil, fmt.Errorf("combo %d not found", *item.ComboID)
				}
				return 0, nil, err
			}
			// 验证套餐属于该商户
			if combo.MerchantID != merchantID {
				return 0, nil, fmt.Errorf("combo %d does not belong to this merchant", *item.ComboID)
			}
			// 验证套餐上架
			if !combo.IsOnline {
				return 0, nil, fmt.Errorf("combo %s is offline", combo.Name)
			}

			name = combo.Name
			unitPrice = combo.ComboPrice
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}

		// 计算定制选项额外费用
		var extraPrice int64 = 0
		for _, c := range item.Customizations {
			extraPrice += c.ExtraPrice
		}
		unitPrice += extraPrice

		itemSubtotal := unitPrice * int64(item.Quantity)
		subtotal += itemSubtotal

		// 序列化定制选项
		var customizations []byte
		if len(item.Customizations) > 0 {
			customizations, _ = json.Marshal(item.Customizations)
		}

		orderItems = append(orderItems, db.CreateOrderItemParams{
			DishID:         dishID,
			ComboID:        comboID,
			Name:           name,
			UnitPrice:      unitPrice,
			Quantity:       item.Quantity,
			Subtotal:       itemSubtotal,
			Customizations: customizations,
		})
	}

	return subtotal, orderItems, nil
}

type getOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// getOrder godoc
// @Summary 获取订单详情
// @Description 获取指定订单的详细信息，包含订单商品明细。仅订单所有者可查看。
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "订单详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/{id} [get]
// @Security BearerAuth
func (server *Server) getOrder(ctx *gin.Context) {
	var req getOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	order, err := server.store.GetOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}

	// 获取订单明细
	items, err := server.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newOrderResponse(order)
	resp.Items = make([]orderItemResponse, len(items))
	for i, item := range items {
		resp.Items[i] = orderItemResponse{
			ID:        item.ID,
			Name:      item.Name,
			UnitPrice: item.UnitPrice,
			Quantity:  item.Quantity,
			Subtotal:  item.Subtotal,
		}
		if item.DishID.Valid {
			resp.Items[i].DishID = &item.DishID.Int64
		}
		if item.ComboID.Valid {
			resp.Items[i].ComboID = &item.ComboID.Int64
		}
		if item.DishImageUrl.Valid {
			img := normalizeUploadURLForClient(item.DishImageUrl.String)
			resp.Items[i].ImageURL = &img
		}
		if item.Customizations != nil {
			json.Unmarshal(item.Customizations, &resp.Items[i].Customizations)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// listOrders 获取订单列表 (用户)
type listOrdersRequest struct {
	// 页码 (必填，从1开始)
	PageID int32 `form:"page_id" binding:"required,min=1" example:"1"`

	// 每页条数 (必填，范围：5-20)
	PageSize int32 `form:"page_size" binding:"required,min=5,max=20" example:"10"`

	// 订单状态筛选 (选填，枚举值: pending,paid,preparing,ready,delivering,completed,cancelled)
	Status string `form:"status" binding:"omitempty,oneof=pending paid preparing ready delivering completed cancelled" enums:"pending,paid,preparing,ready,delivering,completed,cancelled" example:"paid"`
}

// listOrders godoc
// @Summary 获取订单列表
// @Description 分页获取当前用户的订单列表，支持按状态筛选。
// @Description
// @Description **订单状态枚举：**
// @Description - pending: 待支付
// @Description - paid: 已支付
// @Description - preparing: 制作中
// @Description - ready: 待配送/待取餐
// @Description - delivering: 配送中
// @Description - completed: 已完成
// @Description - cancelled: 已取消
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码(从1开始)" minimum(1)
// @Param page_size query int true "每页条数" minimum(5) maximum(20)
// @Param status query string false "订单状态筛选" Enums(pending,paid,preparing,ready,delivering,completed,cancelled)
// @Success 200 {array} orderResponse "订单列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders [get]
// @Security BearerAuth
func (server *Server) listOrders(ctx *gin.Context) {
	var req listOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	offset := (req.PageID - 1) * req.PageSize

	var resp []orderResponse

	if req.Status != "" {
		// 按状态过滤
		orders, err := server.store.ListOrdersByUserAndStatus(ctx, db.ListOrdersByUserAndStatusParams{
			UserID: authPayload.UserID,
			Status: req.Status,
			Limit:  req.PageSize,
			Offset: offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		resp = make([]orderResponse, len(orders))
		for i, o := range orders {
			resp[i] = newOrderWithMerchantFromStatusResponse(o)
		}
	} else {
		orders, err := server.store.ListOrdersByUser(ctx, db.ListOrdersByUserParams{
			UserID: authPayload.UserID,
			Limit:  req.PageSize,
			Offset: offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		resp = make([]orderResponse, len(orders))
		for i, o := range orders {
			resp[i] = newOrderWithMerchantResponse(o)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// cancelOrder godoc
// @Summary 取消订单
// @Description 取消订单。仅pending(待支付)或paid(已支付)状态的订单可以取消。
// @Description
// @Description **业务规则：**
// @Description - 用户只能取消自己的订单
// @Description - preparing(制作中)及之后状态的订单无法取消
// @Description - 已支付订单取消后会触发退款流程
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param request body object{reason=string} false "取消原因(选填)"
// @Success 200 {object} orderResponse "取消成功"
// @Failure 400 {object} ErrorResponse "订单状态不允许取消"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/{id}/cancel [post]
// @Security BearerAuth
func (server *Server) cancelOrder(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var bodyReq struct {
		Reason string `json:"reason,omitempty" binding:"max=500"`
	}
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}

	// 验证订单状态可取消：只允许 pending(待支付) 和 paid(已支付,商户未接单) 状态
	if order.Status != OrderStatusPending && order.Status != OrderStatusPaid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("订单当前状态无法取消，商户已接单后请联系商户处理")))
		return
	}

	// 使用事务更新订单状态并创建日志
	cancelReason := "用户取消"
	if bodyReq.Reason != "" {
		cancelReason = bodyReq.Reason
	}

	result, err := server.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
		OrderID:      uriReq.ID,
		OldStatus:    order.Status,
		CancelReason: cancelReason,
		OperatorID:   authPayload.UserID,
		OperatorType: "user",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果是已支付订单，触发退款流程
	if order.Status == OrderStatusPaid && server.taskDistributor != nil {
		// 查询支付订单
		paymentOrders, err := server.store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: order.ID, Valid: true})
		if err == nil && len(paymentOrders) > 0 {
			// 找到状态为 paid 的支付订单
			for _, paymentOrder := range paymentOrders {
				if paymentOrder.Status == "paid" {
					// 异步发起退款任务
					err = server.taskDistributor.DistributeTaskProcessRefund(ctx, &worker.PayloadProcessRefund{
						PaymentOrderID: paymentOrder.ID,
						OrderID:        order.ID,
						RefundAmount:   paymentOrder.Amount,
						Reason:         cancelReason,
					})
					if err != nil {
						// 退款任务分发失败，记录日志但不影响取消结果
						log.Error().Err(err).Int64("order_id", order.ID).Msg("failed to distribute refund task")
					}
					break
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, newOrderResponse(result.Order))
}

// urgeOrder godoc
// @Summary 催单
// @Description 向商户/骑手发送催单通知以加快订单处理
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} MessageResponse "催单成功"
// @Failure 400 {object} ErrorResponse "订单状态不允许催单"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/{id}/urge [post]
// @Security BearerAuth
func (server *Server) urgeOrder(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取订单
	order, err := server.store.GetOrder(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}

	// 验证订单状态允许催单（已支付、制作中、待取餐、配送中）
	allowedStatuses := map[string]bool{
		OrderStatusPaid:       true,
		OrderStatusPreparing:  true,
		OrderStatusReady:      true,
		OrderStatusDelivering: true,
	}
	if !allowedStatuses[order.Status] {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order cannot be urged in current status")))
		return
	}

	// 发送催单通知
	// 1. 发送给商户
	if order.Status == OrderStatusPaid || order.Status == OrderStatusPreparing {
		if server.taskDistributor != nil {
			_ = server.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
				UserID:      order.MerchantID, // 商户ID
				Title:       "用户催单提醒",
				Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快处理", order.OrderNo),
				Type:        "order_urge",
				RelatedType: "order",
				RelatedID:   order.ID,
			}, asynq.MaxRetry(2))
		}
	}

	// 2. 配送中的订单发送给骑手
	if order.Status == OrderStatusDelivering {
		// 查询配送信息获取骑手ID
		delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil && delivery.RiderID.Valid {
			if server.taskDistributor != nil {
				_ = server.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
					UserID:      delivery.RiderID.Int64,
					Title:       "用户催单提醒",
					Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快送达", order.OrderNo),
					Type:        "order_urge",
					RelatedType: "order",
					RelatedID:   order.ID,
				}, asynq.MaxRetry(2))
			}
		}
	}

	// 记录催单日志
	_, _ = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     order.Status, // 状态不变，只记录催单事件
		OperatorID:   pgtype.Int8{Int64: authPayload.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "user", Valid: true},
		Notes:        pgtype.Text{String: "用户催单", Valid: true},
	})

	ctx.JSON(http.StatusOK, gin.H{
		"message":  "urge notification sent",
		"order_id": order.ID,
	})
}

// confirmOrder godoc
// @Summary 确认收货
// @Description 用户确认已收到订单（适用于外卖订单）
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "确认成功"
// @Failure 400 {object} ErrorResponse "订单类型或状态不允许确认"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/{id}/confirm [post]
// @Security BearerAuth
func (server *Server) confirmOrder(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}

	// 验证订单类型是外卖且状态是配送中
	if order.OrderType != OrderTypeTakeout {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only takeout orders can be confirmed")))
		return
	}

	if order.Status != OrderStatusDelivering {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order is not in delivering status")))
		return
	}

	// 更新订单状态为已完成
	updatedOrder, err := server.store.UpdateOrderToCompleted(ctx, order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 记录状态变更日志
	_, _ = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     OrderStatusCompleted,
		OperatorID:   pgtype.Int8{Int64: authPayload.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "user", Valid: true},
		Notes:        pgtype.Text{String: "用户确认收货", Valid: true},
	})

	// 发送通知给商户和骑手
	if server.taskDistributor != nil {
		// 通知商户
		_ = server.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
			UserID:      order.MerchantID,
			Title:       "订单已完成",
			Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
			Type:        "order_completed",
			RelatedType: "order",
			RelatedID:   order.ID,
		}, asynq.MaxRetry(2))

		// 通知骑手
		delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil && delivery.RiderID.Valid {
			_ = server.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
				UserID:      delivery.RiderID.Int64,
				Title:       "配送已完成",
				Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
				Type:        "delivery_completed",
				RelatedType: "order",
				RelatedID:   order.ID,
			}, asynq.MaxRetry(2))
		}
	}

	ctx.JSON(http.StatusOK, newOrderResponse(updatedOrder))
}

// ==================== 商户端订单API ====================

// listMerchantOrders 获取商户订单列表
type listMerchantOrdersRequest struct {
	// 页码 (必填，从1开始)
	PageID int32 `form:"page_id" binding:"required,min=1" example:"1"`

	// 每页条数 (必填，范围：5-50)
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50" example:"20"`

	// 订单状态筛选 (选填，枚举值: pending,paid,preparing,ready,delivering,completed,cancelled)
	Status string `form:"status" binding:"omitempty,oneof=pending paid preparing ready delivering completed cancelled" enums:"pending,paid,preparing,ready,delivering,completed,cancelled" example:"paid"`
}

// listMerchantOrders godoc
// @Summary 获取商户订单列表
// @Description 分页获取当前商户的订单列表，支持按状态筛选
// @Description
// @Description **订单状态枚举：**
// @Description - pending: 待支付
// @Description - paid: 已支付
// @Description - preparing: 制作中
// @Description - ready: 待配送/待取餐
// @Description - delivering: 配送中
// @Description - completed: 已完成
// @Description - cancelled: 已取消
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码(从1开始)" minimum(1)
// @Param page_size query int true "每页条数" minimum(5) maximum(50)
// @Param status query string false "订单状态筛选" Enums(pending,paid,preparing,ready,delivering,completed,cancelled)
// @Success 200 {array} orderResponse "订单列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders [get]
// @Security BearerAuth
func (server *Server) listMerchantOrders(ctx *gin.Context) {
	var req listMerchantOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	var orders []db.Order

	if req.Status != "" {
		orders, err = server.store.ListOrdersByMerchantAndStatus(ctx, db.ListOrdersByMerchantAndStatusParams{
			MerchantID: merchant.ID,
			Status:     req.Status,
			Limit:      req.PageSize,
			Offset:     offset,
		})
	} else {
		orders, err = server.store.ListOrdersByMerchant(ctx, db.ListOrdersByMerchantParams{
			MerchantID: merchant.ID,
			Limit:      req.PageSize,
			Offset:     offset,
		})
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = newOrderResponse(o)
	}

	ctx.JSON(http.StatusOK, resp)
}

// getMerchantOrder 获取商户单个订单详情
type getMerchantOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// getMerchantOrder godoc
// @Summary 获取商户订单详情
// @Description 获取指定订单的详细信息（商户视角）
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "订单详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id} [get]
// @Security BearerAuth
func (server *Server) getMerchantOrder(ctx *gin.Context) {
	var req getMerchantOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order, err := server.store.GetOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 获取订单明细
	items, err := server.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newOrderResponse(order)
	resp.Items = make([]orderItemResponse, len(items))
	for i, item := range items {
		resp.Items[i] = orderItemResponse{
			ID:        item.ID,
			Name:      item.Name,
			UnitPrice: item.UnitPrice,
			Quantity:  item.Quantity,
			Subtotal:  item.Subtotal,
		}
		if item.DishID.Valid {
			resp.Items[i].DishID = &item.DishID.Int64
		}
		if item.ComboID.Valid {
			resp.Items[i].ComboID = &item.ComboID.Int64
		}
		if item.DishImageUrl.Valid {
			img := normalizeUploadURLForClient(item.DishImageUrl.String)
			resp.Items[i].ImageURL = &img
		}
		if item.Customizations != nil {
			json.Unmarshal(item.Customizations, &resp.Items[i].Customizations)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// acceptOrder 商户接单
type acceptOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// acceptOrder godoc
// @Summary 商户接单
// @Description 商户接受订单并开始准备
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "接单成功"
// @Failure 400 {object} ErrorResponse "订单状态不允许接单"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/accept [post]
// @Security BearerAuth
func (server *Server) acceptOrder(ctx *gin.Context) {
	var req acceptOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 验证订单状态
	if order.Status != OrderStatusPaid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only paid orders can be accepted")))
		return
	}

	// 使用事务更新订单状态并创建日志
	result, err := server.store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:      req.ID,
		NewStatus:    OrderStatusPreparing,
		OldStatus:    order.Status,
		OperatorID:   authPayload.UserID,
		OperatorType: "merchant",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 M14: 异步发送订单接单通知
	if server.taskDistributor != nil {
		expiresAt := time.Now().Add(24 * time.Hour)
		_ = server.taskDistributor.DistributeTaskSendNotification(
			ctx,
			&worker.SendNotificationPayload{
				UserID:      updatedOrder.UserID,
				Type:        "order",
				Title:       "商家已接单",
				Content:     fmt.Sprintf("您的订单%s已被商家接单，正在准备中", updatedOrder.OrderNo),
				RelatedType: "order",
				RelatedID:   updatedOrder.ID,
				ExtraData: map[string]any{
					"order_no": updatedOrder.OrderNo,
					"status":   OrderStatusPreparing,
				},
				ExpiresAt: &expiresAt,
			},
			asynq.Queue(worker.QueueDefault),
		)
	}

	ctx.JSON(http.StatusOK, newOrderResponse(updatedOrder))
}

// rejectOrder 商户拒单
type rejectOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

type rejectOrderBody struct {
	// 拒单原因 (必填)
	Reason string `json:"reason" binding:"required,min=2,max=200" example:"材料售罄"`
}

// rejectOrder godoc
// @Summary 商户拒单
// @Description 商户拒绝订单并说明原因，将触发自动退款
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param request body rejectOrderBody true "拒单原因"
// @Success 200 {object} orderResponse "拒单成功"
// @Failure 400 {object} ErrorResponse "订单状态不允许拒单"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/reject [post]
// @Security BearerAuth
func (server *Server) rejectOrder(ctx *gin.Context) {
	var uriReq rejectOrderRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var bodyReq rejectOrderBody
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 验证订单状态（只有已支付的订单可以被拒单）
	if order.Status != OrderStatusPaid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only paid orders can be rejected")))
		return
	}

	// 使用事务更新订单状态为已取消，并记录拒单原因
	result, err := server.store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:      uriReq.ID,
		NewStatus:    OrderStatusCancelled,
		OldStatus:    order.Status,
		OperatorID:   authPayload.UserID,
		OperatorType: "merchant",
		Notes:        fmt.Sprintf("商户拒单：%s", bodyReq.Reason),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 M14: 异步发送拒单通知
	if server.taskDistributor != nil {
		expiresAt := time.Now().Add(24 * time.Hour)
		_ = server.taskDistributor.DistributeTaskSendNotification(
			ctx,
			&worker.SendNotificationPayload{
				UserID:      updatedOrder.UserID,
				Type:        "order",
				Title:       "订单被商家取消",
				Content:     fmt.Sprintf("您的订单%s已被商家取消，原因：%s。支付金额将原路退回", updatedOrder.OrderNo, bodyReq.Reason),
				RelatedType: "order",
				RelatedID:   updatedOrder.ID,
				ExtraData: map[string]any{
					"order_no": updatedOrder.OrderNo,
					"status":   OrderStatusCancelled,
					"reason":   bodyReq.Reason,
				},
				ExpiresAt: &expiresAt,
			},
			asynq.Queue(worker.QueueDefault),
		)
	}

	// 自动退款：获取支付订单并发起退款
	paymentOrder, err := server.store.GetLatestPaymentOrderByOrder(ctx, pgtype.Int8{Int64: updatedOrder.ID, Valid: true})
	if err == nil && paymentOrder.Status == PaymentStatusPaid {
		// 生成退款单号
		outRefundNo := generateOutRefundNo()

		// 创建退款订单
		refundOrder, err := server.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     "full",
			RefundAmount:   paymentOrder.Amount,
			RefundReason:   pgtype.Text{String: fmt.Sprintf("商户拒单：%s", bodyReq.Reason), Valid: true},
			OutRefundNo:    outRefundNo,
			Status:         "pending",
		})
		if err != nil {
			log.Error().Err(err).Int64("order_id", updatedOrder.ID).Msg("failed to create refund order for rejected order")
		} else if server.paymentClient != nil {
			// 调用微信退款 API
			wxRefund, err := server.paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
				OutTradeNo:   paymentOrder.OutTradeNo,
				OutRefundNo:  outRefundNo,
				Reason:       fmt.Sprintf("商户拒单：%s", bodyReq.Reason),
				RefundAmount: paymentOrder.Amount,
				TotalAmount:  paymentOrder.Amount,
			})
			if err != nil {
				log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Msg("wechat refund API failed")
				server.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			} else {
				switch wxRefund.Status {
				case wechat.RefundStatusSuccess:
					server.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
					server.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
				case wechat.RefundStatusProcessing:
					server.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
						ID:       refundOrder.ID,
						RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
					})
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, newOrderResponse(updatedOrder))
}

// markOrderReady 标记订单出餐完成
type markOrderReadyRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// markOrderReady godoc
// @Summary 标记出餐完成
// @Description 商户标记订单已出餐，等待配送或顾客取餐
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "标记成功"
// @Failure 400 {object} ErrorResponse "订单状态不允许标记出餐"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/ready [post]
// @Security BearerAuth
func (server *Server) markOrderReady(ctx *gin.Context) {
	var req markOrderReadyRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 验证订单状态
	if order.Status != OrderStatusPreparing {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only preparing orders can be marked as ready")))
		return
	}

	// 使用事务更新订单状态并创建日志
	result, err := server.store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:      req.ID,
		NewStatus:    OrderStatusReady,
		OldStatus:    order.Status,
		OperatorID:   authPayload.UserID,
		OperatorType: "merchant",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 P1: 异步发送出餐完成通知
	if server.taskDistributor != nil {
		expiresAt := time.Now().Add(24 * time.Hour)
		_ = server.taskDistributor.DistributeTaskSendNotification(
			ctx,
			&worker.SendNotificationPayload{
				UserID:      updatedOrder.UserID,
				Type:        "order",
				Title:       "订单已出餐",
				Content:     fmt.Sprintf("您的订单%s已出餐，请及时取餐", updatedOrder.OrderNo),
				RelatedType: "order",
				RelatedID:   updatedOrder.ID,
				ExtraData: map[string]any{
					"order_no": updatedOrder.OrderNo,
					"status":   OrderStatusReady,
				},
				ExpiresAt: &expiresAt,
			},
			asynq.Queue(worker.QueueDefault),
		)
	}

	ctx.JSON(http.StatusOK, newOrderResponse(updatedOrder))
}

// completeOrder 完成订单（堂食/打包自取）
type completeOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// completeOrder godoc
// @Summary 完成订单
// @Description 商户标记订单已完成（堂食/打包自取订单）
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "订单已完成"
// @Failure 400 {object} ErrorResponse "订单状态或类型不允许完成"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/complete [post]
// @Security BearerAuth
func (server *Server) completeOrder(ctx *gin.Context) {
	var req completeOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 验证订单类型和状态
	if order.OrderType == OrderTypeTakeout {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("takeout orders cannot be completed by merchant")))
		return
	}
	if order.Status != OrderStatusReady {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only ready orders can be completed")))
		return
	}

	// 使用事务更新订单状态并创建日志
	result, err := server.store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      req.ID,
		OldStatus:    order.Status,
		OperatorID:   authPayload.UserID,
		OperatorType: "merchant",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	completedOrder := result.Order

	// 📢 P1: 异步发送订单完成通知
	if server.taskDistributor != nil {
		expiresAt := time.Now().Add(24 * time.Hour)
		_ = server.taskDistributor.DistributeTaskSendNotification(
			ctx,
			&worker.SendNotificationPayload{
				UserID:      completedOrder.UserID,
				Type:        "order",
				Title:       "订单已完成",
				Content:     fmt.Sprintf("您的订单%s已完成，欢迎再次光临", completedOrder.OrderNo),
				RelatedType: "order",
				RelatedID:   completedOrder.ID,
				ExtraData: map[string]any{
					"order_no": completedOrder.OrderNo,
					"status":   OrderStatusCompleted,
				},
				ExpiresAt: &expiresAt,
			},
			asynq.Queue(worker.QueueDefault),
		)
	}

	ctx.JSON(http.StatusOK, newOrderResponse(completedOrder))
}

// getOrderStats 获取订单统计
type getOrderStatsRequest struct {
	// 开始日期 (必填，格式: YYYY-MM-DD)
	StartDate string `form:"start_date" binding:"required" example:"2025-12-01"`

	// 结束日期 (必填，格式: YYYY-MM-DD)
	EndDate string `form:"end_date" binding:"required" example:"2025-12-31"`
}

// getOrderStats godoc
// @Summary 获取订单统计
// @Description 获取商户在指定日期范围内的订单统计数据
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期(YYYY-MM-DD)"
// @Param end_date query string true "结束日期(YYYY-MM-DD)"
// @Success 200 {object} db.GetOrderStatsRow "订单统计数据"
// @Failure 400 {object} ErrorResponse "日期格式错误/范围超限"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/stats [get]
// @Security BearerAuth
func (server *Server) getOrderStats(ctx *gin.Context) {
	var req getOrderStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid start_date format, use YYYY-MM-DD")))
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid end_date format, use YYYY-MM-DD")))
		return
	}

	// 验证日期范围：开始日期不能大于结束日期，范围不能超过90天
	if startDate.After(endDate) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("start_date cannot be after end_date")))
		return
	}
	if endDate.Sub(startDate) > 90*24*time.Hour {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("date range cannot exceed 90 days")))
		return
	}

	// 结束日期包含整天
	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	stats, err := server.store.GetOrderStats(ctx, db.GetOrderStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// orderCalculationResponse 订单金额计算响应
type orderCalculationResponse struct {
	// 商品小计 (单位：分)
	Subtotal int64 `json:"subtotal" example:"5760"`
	// 配送费 (单位：分)
	DeliveryFee int64 `json:"delivery_fee" example:"500"`
	// 配送费优惠 (单位：分)
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount" example:"200"`
	// 满减优惠 (单位：分)
	DiscountAmount int64 `json:"discount_amount" example:"500"`
	// 最终应付金额 (单位：分)
	TotalAmount int64 `json:"total_amount" example:"5560"`
	// 优惠明细
	Promotions []orderPromotion `json:"promotions,omitempty"`
	// 商品明细
	Items []orderCalculationItem `json:"items"`
}

type orderPromotion struct {
	Type   string `json:"type" example:"discount"` // discount, delivery_fee_return, voucher
	Title  string `json:"title" example:"满50减10"`  // 优惠名称
	Amount int64  `json:"amount" example:"1000"`   // 优惠金额（分）
}

type orderCalculationItem struct {
	DishID    *int64 `json:"dish_id,omitempty" example:"1001"`
	ComboID   *int64 `json:"combo_id,omitempty" example:"2001"`
	Name      string `json:"name" example:"宫保鸡丁"`
	UnitPrice int64  `json:"unit_price" example:"2880"`
	Quantity  int16  `json:"quantity" example:"2"`
	Subtotal  int64  `json:"subtotal" example:"5760"`
}

// calculateOrder godoc
// @Summary 计算订单金额
// @Description 根据购物车商品计算订单金额，用于下单前预览。
// @Description
// @Description **计算内容：**
// @Description - 商品小计（基于购物车商品）
// @Description - 配送费（外卖订单，基于实时位置或配送地址）
// @Description - 满减优惠
// @Description - 满返运费优惠
// @Description - 优惠券抵扣
// @Description
// @Description **配送费计算方式：**
// @Description - 传入 latitude/longitude：使用实时位置计算（浏览阶段）
// @Description - 传入 address_id：使用已保存地址计算（下单阶段）
// @Description - 两者都传：优先使用 address_id
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param merchant_id query int64 true "商户ID" minimum(1)
// @Param order_type query string true "订单类型" Enums(takeout,dine_in,takeaway)
// @Param latitude query number false "用户实时纬度（浏览阶段使用）"
// @Param longitude query number false "用户实时经度（浏览阶段使用）"
// @Param address_id query int64 false "配送地址ID（下单阶段使用）"
// @Param voucher_code query string false "优惠券码"
// @Success 200 {object} orderCalculationResponse "计算结果"
// @Failure 400 {object} ErrorResponse "参数错误/购物车为空"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/calculate [get]
// @Security BearerAuth
func (server *Server) calculateOrder(ctx *gin.Context) {
	var req struct {
		MerchantID  int64    `form:"merchant_id" binding:"required,min=1"`
		OrderType   string   `form:"order_type" binding:"required,oneof=takeout dine_in takeaway"`
		Latitude    *float64 `form:"latitude"`
		Longitude   *float64 `form:"longitude"`
		AddressID   *int64   `form:"address_id"`
		VoucherCode string   `form:"voucher_code"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	userID := authPayload.UserID

	// 获取用户购物车
	cart, err := server.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:     userID,
		MerchantID: req.MerchantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("cart is empty")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	cartItems, err := server.store.ListCartItems(ctx, cart.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if len(cartItems) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("cart is empty")))
		return
	}

	// 计算商品小计
	var subtotal int64 = 0
	items := make([]orderCalculationItem, len(cartItems))
	for i, item := range cartItems {
		var name string
		var price int64
		if item.DishID.Valid {
			name = item.DishName.String
			price = item.DishPrice.Int64
		} else if item.ComboID.Valid {
			name = item.ComboName.String
			price = item.ComboPrice.Int64
		}

		itemSubtotal := price * int64(item.Quantity)
		items[i] = orderCalculationItem{
			Name:      name,
			UnitPrice: price,
			Quantity:  item.Quantity,
			Subtotal:  itemSubtotal,
		}
		if item.DishID.Valid {
			dishID := item.DishID.Int64
			items[i].DishID = &dishID
		}
		if item.ComboID.Valid {
			comboID := item.ComboID.Int64
			items[i].ComboID = &comboID
		}
		subtotal += itemSubtotal
	}

	response := orderCalculationResponse{
		Subtotal:   subtotal,
		Promotions: []orderPromotion{},
		Items:      items,
	}

	// 计算配送费
	if req.OrderType == "takeout" {
		// 获取商户信息
		merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		var userLat, userLng float64
		var regionID int64 = merchant.RegionID // 默认使用商户所在区域

		// 确定用户位置：优先使用 address_id，其次使用经纬度
		if req.AddressID != nil {
			// 下单阶段：使用已保存地址
			address, err := server.store.GetUserAddress(ctx, *req.AddressID)
			if err == nil && address.UserID == userID {
				if address.Latitude.Valid && address.Longitude.Valid {
					lat, _ := address.Latitude.Float64Value()
					lng, _ := address.Longitude.Float64Value()
					userLat = lat.Float64
					userLng = lng.Float64
				}
				regionID = address.RegionID
			}
		} else if req.Latitude != nil && req.Longitude != nil {
			// 浏览阶段：使用实时位置
			userLat = *req.Latitude
			userLng = *req.Longitude
			// regionID 保持使用商户所在区域（用户实时位置通常和商户在同一区域）
		}

		// 计算配送距离
		var deliveryDistance int32 = DefaultDeliveryDistance
		if userLat != 0 && userLng != 0 && merchant.Latitude.Valid && merchant.Longitude.Valid {
			merchantLat, _ := merchant.Latitude.Float64Value()
			merchantLng, _ := merchant.Longitude.Float64Value()

			// 优先使用腾讯地图 API 计算骑行距离
			if server.mapClient != nil {
				fromLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
				toLoc := maps.Location{Lat: userLat, Lng: userLng}
				routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
				if err == nil && routeResult != nil {
					deliveryDistance = int32(routeResult.Distance)
				}
			} else {
				// 降级：使用简化的距离计算
				latDiff := (userLat - merchantLat.Float64) * MetersPerLatDegree
				lngDiff := (userLng - merchantLng.Float64) * MetersPerLngDegree
				dist := int32(latDiff*latDiff + lngDiff*lngDiff)
				if dist > 0 {
					deliveryDistance = int32(float64(dist) / 1000)
					if deliveryDistance < MinDeliveryDistance {
						deliveryDistance = MinDeliveryDistance
					}
				}
			}
		}

		// 使用配送费计算内部方法
		feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, req.MerchantID, deliveryDistance, subtotal)
		if err == nil {
			response.DeliveryFee = feeResult.FinalFee
			if feeResult.PromotionDiscount > 0 {
				response.DeliveryFeeDiscount = feeResult.PromotionDiscount
				response.Promotions = append(response.Promotions, orderPromotion{
					Type:   "delivery_fee_return",
					Title:  "满额返运费",
					Amount: feeResult.PromotionDiscount,
				})
			}
		}
	}

	// 计算满减
	discountRules, err := server.store.ListActiveDiscountRules(ctx, req.MerchantID)
	if err == nil {
		// 按 min_order_amount 降序，选择最优满减
		var bestDiscount db.DiscountRule
		var bestFound bool
		for _, d := range discountRules {
			if subtotal >= d.MinOrderAmount {
				if !bestFound || d.DiscountAmount > bestDiscount.DiscountAmount {
					bestDiscount = d
					bestFound = true
				}
			}
		}
		if bestFound {
			response.DiscountAmount = bestDiscount.DiscountAmount
			response.Promotions = append(response.Promotions, orderPromotion{
				Type:   "discount",
				Title:  fmt.Sprintf("满%d减%d", bestDiscount.MinOrderAmount/100, bestDiscount.DiscountAmount/100),
				Amount: bestDiscount.DiscountAmount,
			})
		}
	}

	// 计算优惠券
	if req.VoucherCode != "" {
		voucher, err := server.store.GetVoucherByCode(ctx, req.VoucherCode)
		if err == nil && voucher.MerchantID == req.MerchantID {
			if subtotal >= voucher.MinOrderAmount {
				// 检查是否过期、是否还有余量
				now := time.Now()
				if now.After(voucher.ValidFrom) && now.Before(voucher.ValidUntil) {
					remaining := voucher.TotalQuantity - voucher.ClaimedQuantity
					if remaining > 0 {
						response.DiscountAmount += voucher.Amount
						response.Promotions = append(response.Promotions, orderPromotion{
							Type:   "voucher",
							Title:  voucher.Name,
							Amount: voucher.Amount,
						})
					}
				}
			}
		}
	}

	// 计算最终金额
	response.TotalAmount = response.Subtotal + response.DeliveryFee - response.DeliveryFeeDiscount - response.DiscountAmount
	if response.TotalAmount < 0 {
		response.TotalAmount = 0
	}

	ctx.JSON(http.StatusOK, response)
}
