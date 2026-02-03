package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
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
	OrderStatusPending         = "pending"          // 待支付
	OrderStatusPaid            = "paid"             // 已支付
	OrderStatusPreparing       = "preparing"        // 制作中
	OrderStatusReady           = "ready"            // 待取餐/待配送
	OrderStatusCourierAccepted = "courier_accepted" // 骑手已接单
	OrderStatusPicked          = "picked"           // 已取餐
	OrderStatusDelivering      = "delivering"       // 配送中
	OrderStatusRiderDelivered  = "rider_delivered"  // 骑手送达（待用户确认）
	OrderStatusUserDelivered   = "user_delivered"   // 用户确认送达
	OrderStatusCompleted       = "completed"        // 已完成
	OrderStatusCancelled       = "cancelled"        // 已取消
)

// 履约状态常量
const (
	FulfillmentStatusScheduled      = "scheduled"       // 已排期，等待开始（预定/预约）
	FulfillmentStatusPendingKitchen = "pending_kitchen" // 待出餐（已支付，等待接单/下发厨房）
	FulfillmentStatusPreparing      = "preparing"       // 厨房制作中
	FulfillmentStatusReady          = "ready"           // 出餐完成，等待取/配送
	FulfillmentStatusCompleted      = "completed"       // 履约完成
	FulfillmentStatusCancelled      = "cancelled"       // 履约取消
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

	// 定制化选项（选填，格式：{group_id: option_id}）
	Customizations map[string]interface{} `json:"customizations,omitempty"`
}

type orderCustomizationItem struct {
	// 分组ID
	GroupID int64 `json:"group_id,omitempty"`

	// 选项ID
	OptionID int64 `json:"option_id,omitempty"`

	// 标签ID
	TagID int64 `json:"tag_id,omitempty"`

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

	// 账单组ID (堂食可选，用于拼桌/单独结算)
	BillingGroupID *int64 `json:"billing_group_id,omitempty" binding:"omitempty,min=1" example:"12001"`

	// 订单商品列表 (必填，至少包含1个商品，最多50个)
	Items []orderItemRequest `json:"items" binding:"required,min=1,max=50,dive"`

	// 订单备注 (选填，如忌口、特殊要求等，最多500字符)
	Notes string `json:"notes,omitempty" binding:"max=500" example:"不要香菜"`

	// 用户优惠券ID (选填，使用已领取的优惠券抵扣)
	UserVoucherID *int64 `json:"user_voucher_id,omitempty" binding:"omitempty,min=1" example:"9001"`

	// 前端已计算的配送费（分），用于直落且供服务端校验
	DeliveryFee int64 `json:"delivery_fee,omitempty" example:"500"`
	// 前端已计算的配送费优惠（分）
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount,omitempty" example:"200"`
	// 前端已计算的配送距离（米）
	DeliveryDistance int32 `json:"delivery_distance,omitempty" example:"2500"`

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

type orderBadge struct {
	Text   string `json:"text,omitempty"`
	Type   string `json:"type,omitempty"`
	Locale string `json:"locale,omitempty"`
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

	// 预计送达总时长（分钟），用于前端 ETA 展示
	DeliveryEtaMinutes *int32 `json:"delivery_eta_minutes,omitempty" example:"38"`

	// 预计送达时间（时间戳），用于订单详情展示送达时间段
	EstimatedDeliveryAt *time.Time `json:"estimated_delivery_at,omitempty" example:"2025-12-01T12:30:00Z"`

	// 三方派单/履约信息
	DispatchOrderID  *int64  `json:"dispatch_order_id,omitempty"`
	FlowID           *int64  `json:"flow_id,omitempty"`
	PickupCode       *string `json:"pickup_code,omitempty"`
	PickupCodeMasked *string `json:"pickup_code_masked,omitempty"`

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

	// 订单状态 (枚举: pending-待支付, paid-已支付, preparing-制作中, ready-待取餐/待配送, courier_accepted-骑手已接单, picked-已取餐, delivering-配送中, rider_delivered-骑手送达, user_delivered-用户确认送达, completed-已完成, cancelled-已取消)
	Status string `json:"status" enums:"pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled" example:"paid"`

	// 状态提示与徽标
	StatusHint *string      `json:"status_hint,omitempty"`
	Badges     []orderBadge `json:"badges,omitempty"`
	Actions    []string     `json:"actions,omitempty"`

	// 异常/投诉通道
	ExceptionState *string `json:"exception_state,omitempty"`
	ClaimChannel   *string `json:"claim_channel,omitempty"`
	Overtime       bool    `json:"overtime,omitempty"`

	// 履约状态 (枚举: scheduled-已排期, pending_kitchen-待出餐, preparing-制作中, ready-已出餐, completed-履约完成, cancelled-已取消)
	FulfillmentStatus string `json:"fulfillment_status" enums:"scheduled,pending_kitchen,preparing,ready,completed,cancelled" example:"pending_kitchen"`

	// 支付方式 (枚举: wechat-微信支付, balance-余额支付)
	PaymentMethod *string `json:"payment_method,omitempty" enums:"wechat,balance" example:"wechat"`

	// 订单备注
	Notes *string `json:"notes,omitempty" example:"不要香菜"`

	// 订单商品列表
	Items []orderItemResponse `json:"items,omitempty"`

	// 支付时间
	PaidAt *time.Time `json:"paid_at,omitempty" example:"2025-12-01T12:30:00Z"`

	// 准备/配送关键时间点
	PrepStartAt         *time.Time `json:"prep_start_at,omitempty"`
	ReadyAt             *time.Time `json:"ready_at,omitempty"`
	CourierAcceptAt     *time.Time `json:"courier_accept_at,omitempty"`
	PickedAt            *time.Time `json:"picked_at,omitempty"`
	RiderDeliveredAt    *time.Time `json:"rider_delivered_at,omitempty"`
	UserDeliveredAt     *time.Time `json:"user_delivered_at,omitempty"`
	AutoUserDeliveredAt *time.Time `json:"auto_user_delivered_at,omitempty"`

	// 完成时间（历史兼容）
	CompletedAt *time.Time `json:"completed_at,omitempty" example:"2025-12-01T13:15:00Z"`

	// 取消时间
	CancelledAt *time.Time `json:"cancelled_at,omitempty" example:"2025-12-01T12:25:00Z"`

	// 取消原因
	CancelReason *string `json:"cancel_reason,omitempty" example:"商品缺货"`

	// 替换的新订单ID（仅当此订单被更新菜单替换时存在）
	ReplacedByOrderID *int64 `json:"replaced_by_order_id,omitempty" example:"100009"`

	// 创建时间
	CreatedAt time.Time `json:"created_at" example:"2025-12-01T12:20:00Z"`

	// 更新时间
	UpdatedAt *time.Time `json:"updated_at,omitempty" example:"2025-12-01T12:30:00Z"`

	// 商户电话
	MerchantPhone *string `json:"merchant_phone,omitempty" example:"13800138000"`

	// 配送联系人
	DeliveryContactName *string `json:"delivery_contact_name,omitempty" example:"张三"`

	// 配送联系电话
	DeliveryContactPhone *string `json:"delivery_contact_phone,omitempty" example:"13800138000"`

	// 配送地址
	DeliveryAddress *string `json:"delivery_address,omitempty" example:"北京市朝阳区某小区1号楼"`
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
		FulfillmentStatus:   o.FulfillmentStatus,
		CreatedAt:           o.CreatedAt,
		Overtime:            o.Overtime,
	}

	if len(o.Badges) > 0 {
		resp.Badges = decodeOrderBadges(o.Badges)
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
	if o.ReplacedByOrderID.Valid {
		resp.ReplacedByOrderID = &o.ReplacedByOrderID.Int64
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}
	if o.PickupCode.Valid {
		resp.PickupCode = &o.PickupCode.String
		resp.PickupCodeMasked = maskPickupCode(o.PickupCode.String)
	}
	if o.DispatchOrderID.Valid {
		resp.DispatchOrderID = &o.DispatchOrderID.Int64
	}
	if o.FlowID.Valid {
		resp.FlowID = &o.FlowID.Int64
	}
	if o.StatusHint.Valid {
		resp.StatusHint = &o.StatusHint.String
	}
	if o.ExceptionState.Valid {
		resp.ExceptionState = &o.ExceptionState.String
	}
	if o.ClaimChannel.Valid {
		resp.ClaimChannel = &o.ClaimChannel.String
	}
	if o.PrepStartAt.Valid {
		resp.PrepStartAt = &o.PrepStartAt.Time
	}
	if o.ReadyAt.Valid {
		resp.ReadyAt = &o.ReadyAt.Time
	}
	if o.CourierAcceptAt.Valid {
		resp.CourierAcceptAt = &o.CourierAcceptAt.Time
	}
	if o.PickedAt.Valid {
		resp.PickedAt = &o.PickedAt.Time
	}
	if o.RiderDeliveredAt.Valid {
		resp.RiderDeliveredAt = &o.RiderDeliveredAt.Time
	}
	if o.UserDeliveredAt.Valid {
		resp.UserDeliveredAt = &o.UserDeliveredAt.Time
	}
	if o.AutoUserDeliveredAt.Valid {
		resp.AutoUserDeliveredAt = &o.AutoUserDeliveredAt.Time
	}

	// 配送预计在途时长 (ETA)
	if o.DeliveryDuration.Valid && o.DeliveryDuration.Int32 > 0 {
		etaMinutes := o.DeliveryDuration.Int32 / 60
		resp.DeliveryEtaMinutes = &etaMinutes
	}

	resp.Actions = orderActions(o)

	return resp
}

func (server *Server) buildOrderSnapshotWithItems(ctx *gin.Context, order db.Order) (orderResponse, error) {
	items, err := server.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return orderResponse{}, err
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

	return resp, nil
}

func (server *Server) pushMerchantOrderSnapshot(ctx *gin.Context, merchantID int64, order db.Order, msgType string) {
	if server.wsHub == nil {
		return
	}

	resp, err := server.buildOrderSnapshotWithItems(ctx, order)
	if err != nil {
		log.Warn().Err(err).Int64("order_id", order.ID).Msg("build order snapshot failed")
		resp = newOrderResponse(order)
	}

	payload, _ := json.Marshal(resp)
	server.wsHub.SendToMerchant(merchantID, websocket.Message{
		Type:      msgType,
		Data:      payload,
		Timestamp: time.Now(),
	})
}

// newOrderWithDetailsResponse 用于订单详情，包含商户信息和配送地址
func newOrderWithDetailsResponse(o db.GetOrderWithDetailsRow) orderResponse {
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
		FulfillmentStatus:   o.FulfillmentStatus,
		CreatedAt:           o.CreatedAt,
		Overtime:            o.Overtime,
	}

	if len(o.Badges) > 0 {
		resp.Badges = decodeOrderBadges(o.Badges)
	}

	// 商户电话
	if o.MerchantPhone != "" {
		resp.MerchantPhone = &o.MerchantPhone
	}

	// 配送地址信息
	if o.DeliveryContactName.Valid {
		resp.DeliveryContactName = &o.DeliveryContactName.String
	}
	if o.DeliveryContactPhone.Valid {
		resp.DeliveryContactPhone = &o.DeliveryContactPhone.String
	}
	if o.DeliveryAddress.Valid {
		resp.DeliveryAddress = &o.DeliveryAddress.String
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
	if o.ReplacedByOrderID.Valid {
		resp.ReplacedByOrderID = &o.ReplacedByOrderID.Int64
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}
	if o.PickupCode.Valid {
		resp.PickupCode = &o.PickupCode.String
		resp.PickupCodeMasked = maskPickupCode(o.PickupCode.String)
	}
	if o.DispatchOrderID.Valid {
		resp.DispatchOrderID = &o.DispatchOrderID.Int64
	}
	if o.FlowID.Valid {
		resp.FlowID = &o.FlowID.Int64
	}
	if o.StatusHint.Valid {
		resp.StatusHint = &o.StatusHint.String
	}
	if o.ExceptionState.Valid {
		resp.ExceptionState = &o.ExceptionState.String
	}
	if o.ClaimChannel.Valid {
		resp.ClaimChannel = &o.ClaimChannel.String
	}
	if o.PrepStartAt.Valid {
		resp.PrepStartAt = &o.PrepStartAt.Time
	}
	if o.ReadyAt.Valid {
		resp.ReadyAt = &o.ReadyAt.Time
	}
	if o.CourierAcceptAt.Valid {
		resp.CourierAcceptAt = &o.CourierAcceptAt.Time
	}
	if o.PickedAt.Valid {
		resp.PickedAt = &o.PickedAt.Time
	}
	if o.RiderDeliveredAt.Valid {
		resp.RiderDeliveredAt = &o.RiderDeliveredAt.Time
	}
	if o.UserDeliveredAt.Valid {
		resp.UserDeliveredAt = &o.UserDeliveredAt.Time
	}
	if o.AutoUserDeliveredAt.Valid {
		resp.AutoUserDeliveredAt = &o.AutoUserDeliveredAt.Time
	}

	resp.Actions = orderActions(db.Order{
		ID:                o.ID,
		Status:            o.Status,
		FulfillmentStatus: o.FulfillmentStatus,
	})

	return resp
}

func decodeOrderBadges(raw []byte) []orderBadge {
	if len(raw) == 0 {
		return nil
	}

	var badges []orderBadge
	if err := json.Unmarshal(raw, &badges); err != nil {
		log.Warn().Err(err).Msg("failed to decode order badges")
		return nil
	}

	return badges
}

func maskPickupCode(code string) *string {
	if code == "" {
		return nil
	}
	if len(code) <= 2 {
		return ptrString(code)
	}
	masked := "****" + code[len(code)-2:]
	return &masked
}

func orderActions(o db.Order) []string {
	switch o.Status {
	case OrderStatusPending:
		return []string{"pay", "cancel"}
	case OrderStatusPaid:
		return []string{"cancel", "urge"}
	case OrderStatusPreparing, OrderStatusReady:
		return []string{"urge"}
	case OrderStatusCourierAccepted, OrderStatusPicked, OrderStatusDelivering:
		return []string{"urge"}
	case OrderStatusRiderDelivered:
		return []string{"confirm", "complain"}
	case OrderStatusUserDelivered, OrderStatusCompleted:
		return []string{"complain"}
	default:
		return nil
	}
}

func ptrString(v string) *string {
	return &v
}

// replaceOrderRequest 替换已支付的预订订单（全款改菜单）
type replaceOrderRequest struct {
	Items []orderItemRequest `json:"items" binding:"required,min=1,max=50,dive"`
	Notes string             `json:"notes,omitempty" binding:"omitempty,max=500"`
}

// replaceOrderResponse 替换订单响应
type replaceOrderResponse struct {
	Order           orderResponse `json:"order"`
	Delta           int64         `json:"delta"`
	PaymentOrderID  *int64        `json:"payment_order_id,omitempty"`
	RefundInitiated bool          `json:"refund_initiated"`
}

var _ = replaceOrderResponse{}

// newOrderWithMerchantFromFilterResponse converts filtered list rows to API response
func newOrderWithMerchantFromFilterResponse(o db.ListOrdersByUserWithFiltersRow) orderResponse {
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
		FulfillmentStatus:   o.FulfillmentStatus,
		CreatedAt:           o.CreatedAt,
		Overtime:            o.Overtime,
	}

	if len(o.Badges) > 0 {
		resp.Badges = decodeOrderBadges(o.Badges)
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
	if o.ReplacedByOrderID.Valid {
		resp.ReplacedByOrderID = &o.ReplacedByOrderID.Int64
	}
	if o.UpdatedAt.Valid {
		resp.UpdatedAt = &o.UpdatedAt.Time
	}
	if o.PickupCode.Valid {
		resp.PickupCode = &o.PickupCode.String
	}
	if o.DispatchOrderID.Valid {
		resp.DispatchOrderID = &o.DispatchOrderID.Int64
	}
	if o.FlowID.Valid {
		resp.FlowID = &o.FlowID.Int64
	}
	if o.StatusHint.Valid {
		resp.StatusHint = &o.StatusHint.String
	}
	if o.ExceptionState.Valid {
		resp.ExceptionState = &o.ExceptionState.String
	}
	if o.ClaimChannel.Valid {
		resp.ClaimChannel = &o.ClaimChannel.String
	}
	if o.PrepStartAt.Valid {
		resp.PrepStartAt = &o.PrepStartAt.Time
	}
	if o.ReadyAt.Valid {
		resp.ReadyAt = &o.ReadyAt.Time
	}
	if o.CourierAcceptAt.Valid {
		resp.CourierAcceptAt = &o.CourierAcceptAt.Time
	}
	if o.PickedAt.Valid {
		resp.PickedAt = &o.PickedAt.Time
	}
	if o.RiderDeliveredAt.Valid {
		resp.RiderDeliveredAt = &o.RiderDeliveredAt.Time
	}
	if o.UserDeliveredAt.Valid {
		resp.UserDeliveredAt = &o.UserDeliveredAt.Time
	}
	if o.AutoUserDeliveredAt.Valid {
		resp.AutoUserDeliveredAt = &o.AutoUserDeliveredAt.Time
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

// generatePickupCode 生成6位取餐码（数字）
func generatePickupCode() string {
	b := make([]byte, 3)
	rand.Read(b)
	num := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", num%1000000)
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
		if isNotFoundError(err) {
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
	if !merchant.IsOpen {
		log.Warn().Int64("merchant_id", merchant.ID).Msg("[DEBUG] createOrder: merchant is closed")
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("商户已打烊，暂时无法接单")))
		return
	}

	log.Info().Msg("[DEBUG] createOrder: merchant validated")

	// P0安全: 堂食订单验证桌台归属商户
	if req.OrderType == OrderTypeDineIn && req.TableID != nil {
		table, err := server.store.GetTable(ctx, *req.TableID)
		if err != nil {
			if isNotFoundError(err) {
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

	// 堂食/预订订单需要开放用餐会话，绑定会话到订单
	var diningSession *db.DiningSession
	var reservation *db.TableReservation
	switch req.OrderType {
	case OrderTypeReservation:
		if req.ReservationID == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("reservation_id is required for reservation orders")))
			return
		}

		res, err := server.store.GetTableReservation(ctx, *req.ReservationID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if res.UserID != authPayload.UserID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to you")))
			return
		}
		if res.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("reservation does not belong to this merchant")))
			return
		}
		if res.Status != ReservationStatusPending && res.Status != ReservationStatusPaid && res.Status != ReservationStatusConfirmed && res.Status != ReservationStatusCheckedIn {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is in an invalid state for ordering")))
			return
		}
		if req.TableID != nil && res.TableID != *req.TableID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table does not match reservation")))
			return
		}

		reservation = &res

		session, err := server.store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: res.ID, Valid: true})
		if err == nil {
			if session.TableID != res.TableID {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session table mismatch")))
				return
			}
			if session.MerchantID != req.MerchantID {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("dining session merchant mismatch")))
				return
			}
			diningSession = &session
		} else if !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		// 如果是预订点餐且未到店（无会话），允许继续创建订单，后续到店核销时再绑定

		tableID := res.TableID
		req.TableID = &tableID

	case OrderTypeDineIn:
		if req.TableID == nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table_id is required for dine-in orders")))
			return
		}

		session, err := server.store.GetActiveDiningSessionByTable(ctx, *req.TableID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("no active dining session for table")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if session.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("dining session merchant mismatch")))
			return
		}
		if req.ReservationID != nil {
			if !session.ReservationID.Valid || session.ReservationID.Int64 != *req.ReservationID {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session reservation mismatch")))
				return
			}

			res, err := server.store.GetTableReservation(ctx, *req.ReservationID)
			if err != nil {
				if isNotFoundError(err) {
					ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
					return
				}
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			if res.UserID != authPayload.UserID {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to you")))
				return
			}
			if res.MerchantID != req.MerchantID {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("reservation does not belong to this merchant")))
				return
			}
			if res.TableID != *req.TableID {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table does not match reservation")))
				return
			}
			if res.PaymentMode != PaymentModeDeposit {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not in deposit mode")))
				return
			}
			if res.Status != ReservationStatusPaid && res.Status != ReservationStatusConfirmed && res.Status != ReservationStatusCheckedIn {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not ready for dining")))
				return
			}

			reservation = &res
		}

		diningSession = &session
	}

	// 账单组校验（仅堂食/预订）
	if req.BillingGroupID != nil && req.OrderType != OrderTypeDineIn && req.OrderType != OrderTypeReservation {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("billing_group_id is only allowed for dine-in or reservation orders")))
		return
	}
	if req.OrderType == OrderTypeDineIn || req.OrderType == OrderTypeReservation {
		if diningSession == nil {
			if req.OrderType == OrderTypeDineIn {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("no active dining session for billing group")))
				return
			}
			if req.BillingGroupID != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("billing_group_id requires an active session")))
				return
			}
		} else {

			var bg db.BillingGroup
			if req.BillingGroupID != nil {
				var err error
				bg, err = server.store.GetBillingGroup(ctx, *req.BillingGroupID)
				if err != nil {
					if isNotFoundError(err) {
						ctx.JSON(http.StatusNotFound, errorResponse(errors.New("billing group not found")))
						return
					}
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			} else {
				var err error
				bg, err = server.store.GetDefaultBillingGroupBySession(ctx, diningSession.ID)
				if err != nil {
					if isNotFoundError(err) {
						ctx.JSON(http.StatusConflict, errorResponse(errors.New("default billing group not found")))
						return
					}
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			}

			if bg.DiningSessionID != diningSession.ID {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("billing group does not belong to this dining session")))
				return
			}
			if bg.Status == "closed" {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("billing group is closed")))
				return
			}
			if _, err := server.store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
				BillingGroupID: bg.ID,
				UserID:         authPayload.UserID,
			}); err != nil {
				if isNotFoundError(err) {
					ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a member of the billing group")))
					return
				}
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}

			if req.BillingGroupID == nil {
				bgID := bg.ID
				req.BillingGroupID = &bgID
			}
		}
	}

	// 定金模式仅允许一笔尾款订单，避免重复抵扣
	if reservation != nil && reservation.PaymentMode == PaymentModeDeposit {
		existing, err := server.store.GetLatestOrderByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true})
		if err == nil {
			if existing.Status != OrderStatusCancelled && !existing.ReplacedByOrderID.Valid {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation already has an active order")))
				return
			}
		} else if !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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
	// 优先使用前端已计算的费用，后端再校验/兜底，避免重算失败导致 0 元
	var deliveryFee int64 = req.DeliveryFee
	var deliveryDistance int32 = req.DeliveryDistance
	var deliveryFeeDiscount int64 = req.DeliveryFeeDiscount
	var serverFee int64
	var serverFeeDiscount int64
	var deliveryDuration int32
	if req.OrderType == OrderTypeTakeout && req.AddressID != nil {
		// 获取用户地址
		address, err := server.store.GetUserAddress(ctx, *req.AddressID)
		if err != nil {
			if isNotFoundError(err) {
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

		// 若前端未提供距离，则计算配送距离（用于兜底/校验）
		if deliveryDistance == 0 {
			deliveryDistance = DefaultDeliveryDistance
			if address.Latitude.Valid && address.Longitude.Valid && merchant.Latitude.Valid && merchant.Longitude.Valid {
				userLat, _ := address.Latitude.Float64Value()
				userLng, _ := address.Longitude.Float64Value()
				merchantLat, _ := merchant.Latitude.Float64Value()
				merchantLng, _ := merchant.Longitude.Float64Value()

				// 优先使用自建 OSM 计算骑行距离
				if server.mapClient != nil {
					fromLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
					toLoc := maps.Location{Lat: userLat.Float64, Lng: userLng.Float64}
					routeResult, err := server.mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
					if err == nil && routeResult != nil {
						deliveryDistance = int32(routeResult.Distance)
						deliveryDuration = int32(routeResult.Duration)
					}
					if err != nil {
						log.Warn().Err(err).
							Int64("merchant_id", req.MerchantID).
							Int64("address_id", *req.AddressID).
							Msg("createOrder: route calculation failed, using fallback distance")
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
		}

		// 服务端校验运费（不中断，主要为兜底与记录差异）
		if deliveryDistance > 0 {
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, address.RegionID, req.MerchantID, deliveryDistance, subtotal)
			if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
				serverFee = feeResult.FinalFee
				serverFeeDiscount = feeResult.PromotionDiscount
			} else if err != nil {
				log.Warn().Err(err).
					Int64("merchant_id", req.MerchantID).
					Int64("address_id", *req.AddressID).
					Int32("delivery_distance", deliveryDistance).
					Int64("subtotal", subtotal).
					Msg("createOrder: delivery fee calculation failed, fallback to client fee")
			}
		}

		// 选择落库的运费：优先使用前端正数，其次使用服务端兜底
		if deliveryFee <= 0 && serverFee > 0 {
			deliveryFee = serverFee
			deliveryFeeDiscount = serverFeeDiscount
		} else if deliveryFee > 0 && serverFee > 0 {
			// 若差异较大，记录日志，仍以前端为主避免 0 元问题
			if feeDiffSignificant(deliveryFee, serverFee) {
				log.Warn().
					Int64("merchant_id", req.MerchantID).
					Int64("address_id", *req.AddressID).
					Int64("client_fee", deliveryFee).
					Int64("server_fee", serverFee).
					Int64("client_discount", deliveryFeeDiscount).
					Int64("server_discount", serverFeeDiscount).
					Msg("createOrder: delivery fee mismatch, keeping client value")
			}
			// 补全优惠折扣（前端未带时）
			if deliveryFeeDiscount == 0 {
				deliveryFeeDiscount = serverFeeDiscount
			}
		}

		log.Info().
			Int64("merchant_id", req.MerchantID).
			Int64("address_id", *req.AddressID).
			Int32("delivery_distance", deliveryDistance).
			Int64("delivery_fee", deliveryFee).
			Int64("delivery_fee_discount", deliveryFeeDiscount).
			Int64("server_fee", serverFee).
			Int64("server_fee_discount", serverFeeDiscount).
			Msg("createOrder: delivery fee decided")
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
			if isNotFoundError(err) {
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

	// 定金抵扣：预订定金到店点菜直接抵扣应付
	var depositDeduction int64
	if reservation != nil && reservation.PaymentMode == PaymentModeDeposit {
		depositDeduction = reservation.DepositAmount
	}

	// ==================== 会员余额验证 ====================
	var membershipID *int64
	var balancePaid int64 = 0
	var membership *db.MerchantMembership

	if req.UseBalance {
		// 验证订单类型是否支持余额支付（仅堂食、自提和预定）
		if req.OrderType != OrderTypeDineIn && req.OrderType != OrderTypeTakeaway && req.OrderType != OrderTypeReservation {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("外卖订单暂不支持余额支付")))
			return
		}

		// 获取用户在该商户的会员卡
		mem, err := server.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
			MerchantID: req.MerchantID,
			UserID:     authPayload.UserID,
		})
		if err != nil {
			if isNotFoundError(err) {
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

	// 定金抵扣应付金额
	if depositDeduction > 0 {
		if depositDeduction > totalAmount {
			depositDeduction = totalAmount
		}
		totalAmount -= depositDeduction
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

	// 外卖拒绝服务检查（仅外卖，不影响预订/堂食）
	if req.OrderType == OrderTypeTakeout {
		blocked, err := server.checkTakeoutBlocklist(ctx, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if blocked {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("外卖服务已被限制：该账号存在异常索赔记录")))
			return
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
		FulfillmentStatus:   "scheduled",
	}

	if req.OrderType == OrderTypeTakeout || req.OrderType == OrderTypeTakeaway {
		arg.PickupCode = pgtype.Text{String: generatePickupCode(), Valid: true}
	}

	// 设置可选字段
	if req.AddressID != nil {
		arg.AddressID = pgtype.Int8{Int64: *req.AddressID, Valid: true}
	}
	if deliveryDistance > 0 {
		arg.DeliveryDistance = pgtype.Int4{Int32: deliveryDistance, Valid: true}
	}
	if deliveryDuration > 0 {
		arg.DeliveryDuration = pgtype.Int4{Int32: deliveryDuration, Valid: true}
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
		BillingGroupID:    req.BillingGroupID,
		UserVoucherID:     userVoucherID,
		VoucherAmount:     voucherAmount,
		MembershipID:      membershipID,
		BalancePaid:       balancePaid,
		DeliveryDuration:  deliveryDuration,
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

	// 如果是余额全额支付，则自动推进订单状态和后续流程（扣库存、发配送等）
	if req.UseBalance && balancePaid >= totalAmount && txResult.Order.Status == OrderStatusPending {
		paymentResult, err := server.store.ProcessOrderPaymentTx(ctx, db.ProcessOrderPaymentTxParams{
			OrderID:            txResult.Order.ID,
			RiderAverageSpeed:  server.config.RiderAverageSpeed,
			DefaultPrepareTime: server.config.DefaultPrepareTime,
		})
		if err != nil {
			log.Error().Err(err).Int64("order_id", txResult.Order.ID).Msg("failed to process automatic balance payment")
			// 即使失败也继续，因为订单已创建且余额已扣，只是状态未同步更新，可以通过管理后台手动补推
		} else {
			txResult.Order = paymentResult.Order
		}
	}

	resp := newOrderResponse(txResult.Order)

	// 堂食扫码点餐：若用户在外卖拒绝服务名单中，实时提醒商户后台
	if req.OrderType == OrderTypeDineIn && server.wsHub != nil {
		block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   authPayload.UserID,
		})
		if err == nil {
			alertPayload, _ := json.Marshal(map[string]any{
				"user_id":     authPayload.UserID,
				"order_id":    txResult.Order.ID,
				"order_no":    txResult.Order.OrderNo,
				"reason_code": block.ReasonCode,
				"message":     "该顾客有多次恶意索赔记录，谨慎服务",
			})
			server.wsHub.SendToMerchant(req.MerchantID, websocket.Message{
				Type:      "merchant_user_risk_alert",
				Data:      alertPayload,
				Timestamp: time.Now(),
			})
		}
	}

	// 绑定用餐会话的活跃订单（失败不影响下单返回，但记录日志）
	if diningSession != nil {
		_, err := server.store.UpdateDiningSessionActiveOrder(ctx, db.UpdateDiningSessionActiveOrderParams{
			ID:            diningSession.ID,
			ActiveOrderID: pgtype.Int8{Int64: txResult.Order.ID, Valid: true},
		})
		if err != nil {
			log.Warn().Err(err).
				Int64("session_id", diningSession.ID).
				Int64("order_id", txResult.Order.ID).
				Msg("createOrder: failed to bind dining session active order")
		}
	}

	// 清空堂食/预订购物车（外卖保留移除已支付商品逻辑）
	if req.OrderType == OrderTypeDineIn || req.OrderType == OrderTypeReservation {
		tableID := pgtype.Int8{}
		if req.TableID != nil {
			tableID = pgtype.Int8{Int64: *req.TableID, Valid: true}
		}
		reservationID := pgtype.Int8{}
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
		if err == nil {
			_ = server.store.ClearCart(ctx, cart.ID)
		}
	}

	// 调度订单支付超时任务：对于仍处于 pending 状态（需在线支付）的订单
	// 30分钟后自动取消
	if txResult.Order.Status == OrderStatusPending && server.taskDistributor != nil {
		timeoutAt := time.Now().Add(worker.OrderPaymentTimeoutMinutes * time.Minute)
		_ = server.taskDistributor.DistributeTaskOrderPaymentTimeout(
			ctx,
			&worker.PayloadOrderPaymentTimeout{OrderID: txResult.Order.ID},
			asynq.ProcessAt(timeoutAt),
		)
	}

	ctx.JSON(http.StatusOK, resp)
}

func (server *Server) checkTakeoutBlocklist(ctx *gin.Context, userID int64) (bool, error) {
	block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	if block.BlockUntil.Valid && time.Now().After(block.BlockUntil.Time) {
		_ = server.store.UpdateBehaviorBlocklistStatus(ctx, db.UpdateBehaviorBlocklistStatusParams{
			ID:     block.ID,
			Status: "expired",
		})
		return false, nil
	}

	return true, nil
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
				if isNotFoundError(err) {
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
				if isNotFoundError(err) {
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

		var normalizedCustomizations []orderCustomizationItem
		var extraPrice int64 = 0

		if len(item.Customizations) > 0 {
			if item.DishID == nil {
				return 0, nil, fmt.Errorf("customizations only supported for dish items")
			}
			normalized, extra, _, err := server.normalizeDishCustomizations(ctx, dishID.Int64, item.Customizations)
			if err != nil {
				return 0, nil, err
			}
			normalizedCustomizations = normalized
			extraPrice = extra
		} else if item.DishID != nil {
			normalized, _, _, err := server.normalizeDishCustomizations(ctx, dishID.Int64, nil)
			if err != nil {
				return 0, nil, err
			}
			if len(normalized) > 0 {
				return 0, nil, fmt.Errorf("missing required customizations for dish %d", dishID.Int64)
			}
		}

		unitPrice += extraPrice

		itemSubtotal := unitPrice * int64(item.Quantity)
		subtotal += itemSubtotal

		// 序列化定制选项
		var customizations []byte
		if len(normalizedCustomizations) > 0 {
			customizations, _ = json.Marshal(normalizedCustomizations)
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

	// 使用 JOIN 查询，一次获取订单 + 商户信息 + 配送地址
	order, err := server.store.GetOrderWithDetails(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
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

	resp := newOrderWithDetailsResponse(order)
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

	// 获取配送预计到达时间用于前端展示 ETA 时间段
	if order.OrderType == OrderTypeTakeout && order.Status != OrderStatusCancelled {
		// 先尝试已有配送单的精确时间
		if delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID); err == nil {
			if delivery.EstimatedDeliveryAt.Valid {
				resp.EstimatedDeliveryAt = &delivery.EstimatedDeliveryAt.Time
				delta := time.Until(delivery.EstimatedDeliveryAt.Time)
				eta := int32(math.Ceil(delta.Minutes()))
				if eta < 0 {
					eta = 0
				}
				resp.DeliveryEtaMinutes = &eta
			} else {
				// 无精确送达时间时根据距离估算 ETA，避免前端空白
				distance := extractDistance(delivery.Distance, order.DeliveryDistance)
				eta := server.computeDeliveryETA(ctx, order.MerchantID, distance, estimateDurationSecByDistance(distance))
				resp.DeliveryEtaMinutes = &eta.DeliveryEtaMinutes
				est := time.Now().Add(time.Duration(eta.DeliveryEtaMinutes) * time.Minute)
				resp.EstimatedDeliveryAt = &est
			}
		} else {
			// 未生成配送单（如待支付）也给出基于距离的预计时间
			distance := extractDistance(0, order.DeliveryDistance)
			if distance > 0 {
				eta := server.computeDeliveryETA(ctx, order.MerchantID, distance, estimateDurationSecByDistance(distance))
				resp.DeliveryEtaMinutes = &eta.DeliveryEtaMinutes
				est := time.Now().Add(time.Duration(eta.DeliveryEtaMinutes) * time.Minute)
				resp.EstimatedDeliveryAt = &est
			}
			// err 在此分支恒为非 nil，记录非 NotFound 的情况
			if !isNotFoundError(err) {
				log.Warn().Err(err).Int64("order_id", order.ID).Msg("get delivery by order failed")
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// 判断运费差异是否显著（大于10元或10%）
func feeDiffSignificant(clientFee, serverFee int64) bool {
	if clientFee == serverFee {
		return false
	}
	// 绝对差异阈值：1000 分（10元）
	if abs64(clientFee-serverFee) > 1000 {
		return true
	}
	// 相对差异阈值：10%
	base := clientFee
	if serverFee > base {
		base = serverFee
	}
	return abs64(clientFee-serverFee)*10 > base
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// extractDistance 优先取配送单距离，其次取订单存储的距离
func extractDistance(deliveryDistance int32, orderDistance pgtype.Int4) int32 {
	if deliveryDistance > 0 {
		return deliveryDistance
	}
	if orderDistance.Valid {
		return orderDistance.Int32
	}
	return 0
}

// estimateDurationSecByDistance 给出基于距离的粗略秒级配送耗时估计（假设 15km/h）
func estimateDurationSecByDistance(distance int32) int {
	if distance <= 0 {
		return 0
	}
	// 15km/h ≈ 250 米/分钟 → 秒 = 距离/250*60
	return int(math.Round(float64(distance) / 250 * 60))
}

// listOrders 获取订单列表 (用户)
type listOrdersRequest struct {
	// 页码 (必填，从1开始)
	PageID int32 `form:"page_id" binding:"required,min=1" example:"1"`

	// 每页条数 (必填，范围：5-20)
	PageSize int32 `form:"page_size" binding:"required,min=5,max=20" example:"10"`

	// 订单状态筛选 (选填，枚举值: pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled)
	Status string `form:"status" binding:"omitempty,oneof=pending paid preparing ready courier_accepted picked delivering rider_delivered user_delivered completed cancelled" enums:"pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled" example:"paid"`

	// 订单类型筛选 (选填，枚举值: takeout,dine_in,takeaway,reservation)
	OrderType string `form:"order_type" binding:"omitempty,oneof=takeout dine_in takeaway reservation" enums:"takeout,dine_in,takeaway,reservation" example:"takeout"`

	// 预订ID筛选（仅预定点菜订单使用）
	ReservationID *int64 `form:"reservation_id" binding:"omitempty,min=1" example:"8001"`
}

type listOrdersResponse struct {
	Orders     []orderResponse `json:"orders"`
	TotalCount int64           `json:"total_count"`
	Total      int64           `json:"total"`
	PageID     int32           `json:"page_id"`
	PageSize   int32           `json:"page_size"`
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
// @Description - courier_accepted: 骑手已接单
// @Description - picked: 已取餐
// @Description - delivering: 配送中
// @Description - rider_delivered: 骑手送达
// @Description - user_delivered: 用户确认送达
// @Description - completed: 已完成
// @Description - cancelled: 已取消
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码(从1开始)" minimum(1)
// @Param page_size query int true "每页条数" minimum(5) maximum(20)
// @Param status query string false "订单状态筛选" Enums(pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled)
// @Param order_type query string false "订单类型筛选" Enums(takeout,dine_in,takeaway,reservation)
// @Param reservation_id query int false "预订ID筛选（仅预定点菜订单）" minimum(1)
// @Success 200 {object} listOrdersResponse "订单列表"
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

	offset := pageOffset(req.PageID, req.PageSize)

	status := pgtype.Text{}
	if req.Status != "" {
		status = pgtype.Text{String: req.Status, Valid: true}
	}
	orderType := pgtype.Text{}
	if req.OrderType != "" {
		orderType = pgtype.Text{String: req.OrderType, Valid: true}
	}
	reservationID := pgtype.Int8{}
	if req.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *req.ReservationID, Valid: true}
	}

	orders, err := server.store.ListOrdersByUserWithFilters(ctx, db.ListOrdersByUserWithFiltersParams{
		UserID:        authPayload.UserID,
		Status:        status,
		OrderType:     orderType,
		ReservationID: reservationID,
		Limit:         req.PageSize,
		Offset:        offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = newOrderWithMerchantFromFilterResponse(o)
	}

	ctx.JSON(http.StatusOK, listOrdersResponse{
		Orders:     resp,
		TotalCount: int64(len(resp)),
		Total:      int64(len(resp)),
		PageID:     req.PageID,
		PageSize:   req.PageSize,
	})
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
		if isNotFoundError(err) {
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

	lateStatuses := map[string]bool{
		OrderStatusPreparing:       true,
		OrderStatusReady:           true,
		OrderStatusCourierAccepted: true,
		OrderStatusPicked:          true,
		OrderStatusDelivering:      true,
		OrderStatusRiderDelivered:  true,
	}

	if lateStatuses[order.Status] {
		// 已进入制作/配送，记录异常通道，拒绝直接取消
		_, err := server.store.UpdateOrderExceptionState(ctx, db.UpdateOrderExceptionStateParams{
			ID:             order.ID,
			ExceptionState: pgtype.Text{String: "cancel_requested", Valid: true},
			ClaimChannel:   pgtype.Text{String: "user", Valid: true},
		})
		if err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("record cancel request exception state failed")
		}

		_, err = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: order.Status, Valid: true},
			ToStatus:     order.Status,
			OperatorID:   pgtype.Int8{Int64: authPayload.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "user", Valid: true},
			Notes:        pgtype.Text{String: "用户申请取消，进入售后通道", Valid: true},
		})
		if err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("record cancel request log failed")
		}

		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("订单已制作/配送，已记录取消诉求，请联系商户或客服处理")))
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

	server.pushMerchantOrderSnapshot(ctx, result.Order.MerchantID, result.Order, "order_update")

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
		if isNotFoundError(err) {
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
		OrderStatusPaid:            true,
		OrderStatusPreparing:       true,
		OrderStatusReady:           true,
		OrderStatusCourierAccepted: true,
		OrderStatusPicked:          true,
		OrderStatusDelivering:      true,
		OrderStatusRiderDelivered:  true,
	}
	if !allowedStatuses[order.Status] {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order cannot be urged in current status")))
		return
	}

	// 发送催单通知
	// 1. 发送给商户
	if order.Status == OrderStatusPaid || order.Status == OrderStatusPreparing {
		_ = server.SendNotification(ctx, SendNotificationParams{
			UserID:      order.MerchantID,
			Title:       "用户催单提醒",
			Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快处理", order.OrderNo),
			Type:        "order_urge",
			RelatedType: "order",
			RelatedID:   order.ID,
		})
	}

	// 2. 配送中的订单发送给骑手
	if order.Status == OrderStatusDelivering || order.Status == OrderStatusCourierAccepted || order.Status == OrderStatusPicked || order.Status == OrderStatusRiderDelivered {
		// 查询配送信息获取骑手ID
		delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil && delivery.RiderID.Valid {
			_ = server.SendNotification(ctx, SendNotificationParams{
				UserID:      delivery.RiderID.Int64,
				Title:       "用户催单提醒",
				Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快送达", order.OrderNo),
				Type:        "order_urge",
				RelatedType: "order",
				RelatedID:   order.ID,
			})
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

// replaceOrder godoc
// @Summary 全款预订改菜单，生成新订单并作废旧订单
// @Description 仅限支付模式为全款的预订订单，生成新的堂食订单，旧订单标记为被替换；差额自动生成支付/退款单。
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "原订单ID"
// @Param request body replaceOrderRequest true "新的菜品列表"
// @Success 200 {object} replaceOrderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/orders/{id}/replace [post]
// @Security BearerAuth
func (server *Server) replaceOrder(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req replaceOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	oldOrder, err := server.store.GetOrderForUpdate(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if oldOrder.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}
	if oldOrder.OrderType != OrderTypeReservation {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only reservation orders can be replaced")))
		return
	}
	if oldOrder.Status != OrderStatusPaid {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("order must be paid before replacement")))
		return
	}
	if oldOrder.ReplacedByOrderID.Valid {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("order already replaced")))
		return
	}
	if !oldOrder.ReservationID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order missing reservation")))
		return
	}

	reservation, err := server.store.GetTableReservation(ctx, oldOrder.ReservationID.Int64)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if reservation.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to you")))
		return
	}
	if reservation.PaymentMode != PaymentModeFull {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only full-payment reservations support replacement")))
		return
	}
	if reservation.Status != ReservationStatusPaid && reservation.Status != ReservationStatusConfirmed && reservation.Status != ReservationStatusCheckedIn {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not ready for replacement")))
		return
	}

	// 需要有开放的用餐会话确保桌台占用
	session, err := server.store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("no active dining session for reservation")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if session.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("dining session does not belong to you")))
		return
	}

	// 计算新菜品金额
	subtotal, items, err := server.calculateOrderItems(ctx, reservation.MerchantID, req.Items)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 简化：沿用满减优惠
	var discountAmount int64
	discountRules, err := server.store.ListActiveDiscountRules(ctx, reservation.MerchantID)
	if err == nil {
		var best db.DiscountRule
		var has bool
		for _, d := range discountRules {
			if subtotal >= d.MinOrderAmount {
				if !has || d.DiscountAmount > best.DiscountAmount {
					best = d
					has = true
				}
			}
		}
		if has {
			discountAmount = best.DiscountAmount
		}
	}

	newTotal := subtotal - discountAmount
	if newTotal < 0 {
		newTotal = 0
	}

	delta := newTotal - oldOrder.TotalAmount
	newStatus := OrderStatusPaid
	newFulfillment := FulfillmentStatusPendingKitchen
	if delta > 0 {
		newStatus = OrderStatusPending
		newFulfillment = FulfillmentStatusScheduled
	}

	orderNo := generateOrderNo()
	arg := db.CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              authPayload.UserID,
		MerchantID:          reservation.MerchantID,
		OrderType:           OrderTypeDineIn,
		TableID:             pgtype.Int8{Int64: reservation.TableID, Valid: true},
		ReservationID:       pgtype.Int8{Int64: reservation.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		DeliveryFeeDiscount: 0,
		TotalAmount:         newTotal,
		Status:              newStatus,
		FulfillmentStatus:   newFulfillment,
	}
	if req.Notes != "" {
		arg.Notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	createTx, err := server.store.CreateOrderTx(ctx, db.CreateOrderTxParams{
		CreateOrderParams: arg,
		Items:             items,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	newOrder := createTx.Order

	// 标记旧订单为被替换
	_, err = server.store.MarkOrderReplaced(ctx, db.MarkOrderReplacedParams{
		ID:                oldOrder.ID,
		ReplacedByOrderID: pgtype.Int8{Int64: newOrder.ID, Valid: true},
		CancelReason:      pgtype.Text{String: "replaced by new order", Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var paymentOrderID *int64
	refundInitiated := false

	if delta > 0 {
		// 生成补差支付单
		outTradeNo := generateOutTradeNo()
		expiresAt := time.Now().Add(30 * time.Minute)
		var payOrder db.PaymentOrder
		for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
			outTradeNo = generateOutTradeNo()
			payOrder, err = server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
				OrderID:      pgtype.Int8{Int64: newOrder.ID, Valid: true},
				UserID:       authPayload.UserID,
				PaymentType:  PaymentTypeMiniProgram,
				BusinessType: BusinessTypeOrder,
				Amount:       delta,
				OutTradeNo:   outTradeNo,
				ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if err == nil {
				paymentOrderID = &payOrder.ID
				break
			}
			if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
				if !sleepWithContext(ctx.Request.Context(), outTradeNoRetryBaseBack*time.Duration(attempt)) {
					break
				}
				continue
			}
			break
		}
	} else if delta < 0 {
		// 发起差额退款（基于旧订单支付单）
		paymentOrders, err := server.store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: oldOrder.ID, Valid: true})
		if err == nil {
			for _, po := range paymentOrders {
				if po.Status == PaymentStatusPaid {
					refundAmount := -delta
					if server.taskDistributor != nil {
						_ = server.taskDistributor.DistributeTaskProcessRefund(ctx, &worker.PayloadProcessRefund{
							PaymentOrderID: po.ID,
							OrderID:        oldOrder.ID,
							RefundAmount:   refundAmount,
							Reason:         "order replaced diff refund",
						})
						refundInitiated = true
					}
					break
				}
			}
		}
	}

	// 差额为零或负数（无需补差），直接推进支付后履约流程：扣减库存、发厨房/通知
	if delta <= 0 {
		result, err := server.store.ProcessOrderPaymentTx(ctx, db.ProcessOrderPaymentTxParams{
			OrderID:            newOrder.ID,
			RiderAverageSpeed:  server.config.RiderAverageSpeed,
			DefaultPrepareTime: server.config.DefaultPrepareTime,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		newOrder = result.Order
	}

	resp := newOrderResponse(newOrder)
	ctx.JSON(http.StatusOK, gin.H{
		"order":            resp,
		"delta":            delta,
		"payment_order_id": paymentOrderID,
		"refund_initiated": refundInitiated,
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
		if isNotFoundError(err) {
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

	if order.Status != OrderStatusRiderDelivered {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order is not ready for confirmation")))
		return
	}

	// 更新订单状态为用户确认送达
	updatedOrder, err := server.store.UpdateOrderToUserDelivered(ctx, order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 记录状态变更日志
	_, _ = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     OrderStatusUserDelivered,
		OperatorID:   pgtype.Int8{Int64: authPayload.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "user", Valid: true},
		Notes:        pgtype.Text{String: "用户确认收货", Valid: true},
	})

	// 发送通知给商户和骑手
	_ = server.SendNotification(ctx, SendNotificationParams{
		UserID:      order.MerchantID,
		Title:       "订单已完成",
		Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
		Type:        "order_completed",
		RelatedType: "order",
		RelatedID:   order.ID,
	})

	// 通知骑手
	delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
	if err == nil && delivery.RiderID.Valid {
		_ = server.SendNotification(ctx, SendNotificationParams{
			UserID:      delivery.RiderID.Int64,
			Title:       "配送已完成",
			Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
			Type:        "delivery_completed",
			RelatedType: "order",
			RelatedID:   order.ID,
		})
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

	// 订单状态筛选 (选填，枚举值: pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled)
	Status string `form:"status" binding:"omitempty,oneof=pending paid preparing ready courier_accepted picked delivering rider_delivered user_delivered completed cancelled" enums:"pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled" example:"paid"`
}

type listMerchantOrdersResponse struct {
	Orders     []orderResponse `json:"orders"`
	TotalCount int64           `json:"total_count"`
	Total      int64           `json:"total"`
	PageID     int32           `json:"page_id"`
	PageSize   int32           `json:"page_size"`
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
// @Description - courier_accepted: 骑手已接单
// @Description - picked: 已取餐
// @Description - delivering: 配送中
// @Description - rider_delivered: 骑手送达
// @Description - user_delivered: 用户确认送达
// @Description - completed: 已完成
// @Description - cancelled: 已取消
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码(从1开始)" minimum(1)
// @Param page_size query int true "每页条数" minimum(5) maximum(50)
// @Param status query string false "订单状态筛选" Enums(pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled)
// @Success 200 {object} listMerchantOrdersResponse "订单列表"
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	var orders []db.Order
	var totalCount int64

	if req.Status != "" {
		orders, err = server.store.ListOrdersByMerchantAndStatus(ctx, db.ListOrdersByMerchantAndStatusParams{
			MerchantID: merchant.ID,
			Status:     req.Status,
			Limit:      req.PageSize,
			Offset:     offset,
		})
		if err == nil {
			totalCount, err = server.store.CountOrdersByMerchantAndStatus(ctx, db.CountOrdersByMerchantAndStatusParams{
				MerchantID: merchant.ID,
				Status:     req.Status,
			})
		}
	} else {
		orders, err = server.store.ListOrdersByMerchant(ctx, db.ListOrdersByMerchantParams{
			MerchantID: merchant.ID,
			Limit:      req.PageSize,
			Offset:     offset,
		})
		if err == nil {
			totalCount, err = server.store.CountOrdersByMerchant(ctx, merchant.ID)
		}
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = newOrderResponse(o)
	}

	ctx.JSON(http.StatusOK, listMerchantOrdersResponse{
		Orders:     resp,
		TotalCount: totalCount,
		Total:      totalCount,
		PageID:     req.PageID,
		PageSize:   req.PageSize,
	})
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order, err := server.store.GetOrder(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		server.writeAuditLog(ctx, auditLogInput{
			ActorUserID: authPayload.UserID,
			ActorRole:   "merchant",
			Action:      "merchant_resource_access_denied",
			TargetType:  "order",
			TargetID:    &order.ID,
			RegionID:    &merchant.RegionID,
			Metadata: map[string]any{
				"reason":      "order_not_belong_to_merchant",
				"merchant_id": merchant.ID,
				"order_id":    order.ID,
			},
		})
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		server.writeAuditLog(ctx, auditLogInput{
			ActorUserID: authPayload.UserID,
			ActorRole:   "merchant",
			Action:      "merchant_resource_access_denied",
			TargetType:  "order",
			TargetID:    &order.ID,
			RegionID:    &merchant.RegionID,
			Metadata: map[string]any{
				"reason":      "order_not_belong_to_merchant",
				"merchant_id": merchant.ID,
				"order_id":    order.ID,
			},
		})
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
		OrderID:              req.ID,
		NewStatus:            OrderStatusPreparing,
		OldStatus:            order.Status,
		OperatorID:           authPayload.UserID,
		OperatorType:         "merchant",
		NewFulfillmentStatus: ptrString(FulfillmentStatusPreparing),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 M14: 异步发送订单接单通知
	expiresAt := time.Now().Add(24 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
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
	})

	server.pushMerchantOrderSnapshot(ctx, merchant.ID, updatedOrder, "order_update")

	server.writeAuditLog(ctx, auditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_accepted",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": order.Status,
			"to_status":   updatedOrder.Status,
		},
	})

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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		server.writeAuditLog(ctx, auditLogInput{
			ActorUserID: authPayload.UserID,
			ActorRole:   "merchant",
			Action:      "merchant_resource_access_denied",
			TargetType:  "order",
			TargetID:    &order.ID,
			RegionID:    &merchant.RegionID,
			Metadata: map[string]any{
				"reason":      "order_not_belong_to_merchant",
				"merchant_id": merchant.ID,
				"order_id":    order.ID,
			},
		})
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
		OrderID:              uriReq.ID,
		NewStatus:            OrderStatusCancelled,
		OldStatus:            order.Status,
		OperatorID:           authPayload.UserID,
		OperatorType:         "merchant",
		Notes:                fmt.Sprintf("商户拒单：%s", bodyReq.Reason),
		NewFulfillmentStatus: ptrString(FulfillmentStatusCancelled),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 M14: 异步发送拒单通知
	expiresAt := time.Now().Add(24 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
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
	})

	server.pushMerchantOrderSnapshot(ctx, merchant.ID, updatedOrder, "order_update")

	server.writeAuditLog(ctx, auditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_rejected",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": order.Status,
			"to_status":   updatedOrder.Status,
			"reason":      bodyReq.Reason,
		},
	})

	// 自动退款：获取支付订单并发起退款
	paymentOrder, err := server.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: updatedOrder.ID, Valid: true},
		BusinessType: BusinessTypeOrder,
	})
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前商户
	if order.MerchantID != merchant.ID {
		server.writeAuditLog(ctx, auditLogInput{
			ActorUserID: authPayload.UserID,
			ActorRole:   "merchant",
			Action:      "merchant_resource_access_denied",
			TargetType:  "order",
			TargetID:    &order.ID,
			RegionID:    &merchant.RegionID,
			Metadata: map[string]any{
				"reason":      "order_not_belong_to_merchant",
				"merchant_id": merchant.ID,
				"order_id":    order.ID,
			},
		})
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
		OrderID:              req.ID,
		NewStatus:            OrderStatusReady,
		OldStatus:            order.Status,
		OperatorID:           authPayload.UserID,
		OperatorType:         "merchant",
		NewFulfillmentStatus: ptrString(FulfillmentStatusReady),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedOrder := result.Order

	// 📢 P1: 异步发送出餐完成通知
	expiresAt := time.Now().Add(24 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
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
	})

	server.pushMerchantOrderSnapshot(ctx, merchant.ID, updatedOrder, "order_update")

	server.writeAuditLog(ctx, auditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_ready",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": order.Status,
			"to_status":   updatedOrder.Status,
		},
	})

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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取订单（加锁）
	order, err := server.store.GetOrderForUpdate(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
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
	expiresAt := time.Now().Add(24 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
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
	})

	server.pushMerchantOrderSnapshot(ctx, merchant.ID, completedOrder, "order_update")

	server.writeAuditLog(ctx, auditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_completed",
		TargetType:  "order",
		TargetID:    &completedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    completedOrder.OrderNo,
			"order_type":  completedOrder.OrderType,
			"from_status": order.Status,
			"to_status":   completedOrder.Status,
		},
	})

	ctx.JSON(http.StatusOK, newOrderResponse(completedOrder))
}

// getOrderStats 获取订单统计
type getOrderStatsRequest struct {
	// 开始日期 (必填，格式: YYYY-MM-DD)
	StartDate string `form:"start_date" binding:"required" example:"2025-12-01"`

	// 结束日期 (必填，格式: YYYY-MM-DD)
	EndDate string `form:"end_date" binding:"required" example:"2025-12-31"`
}

type orderStatsResponse struct {
	PendingCount    int64 `json:"pending_count"`
	PaidCount       int64 `json:"paid_count"`
	PreparingCount  int64 `json:"preparing_count"`
	ReadyCount      int64 `json:"ready_count"`
	DeliveringCount int64 `json:"delivering_count"`
	CompletedCount  int64 `json:"completed_count"`
	CancelledCount  int64 `json:"cancelled_count"`
}

// getOrderStats godoc
// @Summary 获取订单统计
// @Description 获取商户在指定日期范围内的订单统计数据
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期(YYYY-MM-DD)"
// @Param end_date query string true "结束日期(YYYY-MM-DD)"
// @Success 200 {object} orderStatsResponse "订单统计数据"
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 解析日期
	startDate, err := parseISODate(req.StartDate, "invalid start_date format, use YYYY-MM-DD")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	endDate, err := parseISODate(req.EndDate, "invalid end_date format, use YYYY-MM-DD")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
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

	ctx.JSON(http.StatusOK, orderStatsResponse{
		PendingCount:    stats.PendingCount,
		PaidCount:       stats.PaidCount,
		PreparingCount:  stats.PreparingCount,
		ReadyCount:      stats.ReadyCount,
		DeliveringCount: stats.DeliveringCount,
		CompletedCount:  stats.CompletedCount,
		CancelledCount:  stats.CancelledCount,
	})
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
// @Param user_voucher_id query int64 false "用户优惠券ID"
// @Success 200 {object} orderCalculationResponse "计算结果"
// @Failure 400 {object} ErrorResponse "参数错误/购物车为空"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/calculate [get]
// @Security BearerAuth
func (server *Server) calculateOrder(ctx *gin.Context) {
	var req struct {
		MerchantID    int64    `form:"merchant_id" binding:"required,min=1"`
		OrderType     string   `form:"order_type" binding:"required,oneof=takeout dine_in takeaway"`
		Latitude      *float64 `form:"latitude"`
		Longitude     *float64 `form:"longitude"`
		AddressID     *int64   `form:"address_id"`
		UserVoucherID *int64   `form:"user_voucher_id"`
		VoucherCode   string   `form:"voucher_code"`
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
		if isNotFoundError(err) {
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

			var customizationMap map[string]interface{}
			if len(item.Customizations) > 0 {
				if err := json.Unmarshal(item.Customizations, &customizationMap); err != nil {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid customizations in cart")))
					return
				}
			}
			_, extraPrice, _, err := server.normalizeDishCustomizations(ctx, item.DishID.Int64, customizationMap)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			price += extraPrice
		} else if item.ComboID.Valid {
			name = item.ComboName.String
			price = item.ComboPrice.Int64
			if len(item.Customizations) > 0 {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("customizations not supported for combo items")))
				return
			}
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

			// 优先使用自建 OSM 计算骑行距离
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
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请使用 user_voucher_id 进行金额预览")))
		return
	}
	if req.UserVoucherID != nil {
		uv, err := server.store.GetUserVoucher(ctx, *req.UserVoucherID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("优惠券不存在")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if uv.UserID != userID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("优惠券不属于您")))
			return
		}

		if uv.Status != "unused" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已使用或已过期")))
			return
		}

		if time.Now().After(uv.ExpiresAt) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("优惠券已过期")))
			return
		}

		if uv.MerchantID != req.MerchantID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该优惠券不能在此商户使用")))
			return
		}

		if subtotal < uv.MinOrderAmount {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("未达到最低消费 %d 元", uv.MinOrderAmount/100)))
			return
		}

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

		response.DiscountAmount += uv.Amount
		response.Promotions = append(response.Promotions, orderPromotion{
			Type:   "voucher",
			Title:  uv.Name,
			Amount: uv.Amount,
		})
	}

	// 计算最终金额
	response.TotalAmount = response.Subtotal + response.DeliveryFee - response.DeliveryFeeDiscount - response.DiscountAmount
	if response.TotalAmount < 0 {
		response.TotalAmount = 0
	}

	ctx.JSON(http.StatusOK, response)
}
