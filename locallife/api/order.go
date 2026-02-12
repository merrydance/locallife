package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"
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
	ID                   int64               `json:"id" example:"100001"`
	OrderNo              string              `json:"order_no" example:"ORD20251201123456"`
	UserID               int64               `json:"user_id" example:"10001"`
	MerchantID           int64               `json:"merchant_id" example:"20001"`
	MerchantName         string              `json:"merchant_name,omitempty" example:"张三餐厅"`
	OrderType            string              `json:"order_type" enums:"takeout,dine_in,takeaway,reservation" example:"takeout"`
	AddressID            *int64              `json:"address_id,omitempty" example:"5001"`
	DeliveryFee          int64               `json:"delivery_fee" example:"500"`
	DeliveryDistance     *int32              `json:"delivery_distance,omitempty" example:"2500"`
	DeliveryEtaMinutes   *int32              `json:"delivery_eta_minutes,omitempty" example:"38"`
	EstimatedDeliveryAt  *time.Time          `json:"estimated_delivery_at,omitempty" example:"2025-12-01T12:30:00Z"`
	DispatchOrderID      *int64              `json:"dispatch_order_id,omitempty"`
	FlowID               *int64              `json:"flow_id,omitempty"`
	PickupCode           *string             `json:"pickup_code,omitempty"`
	PickupCodeMasked     *string             `json:"pickup_code_masked,omitempty"`
	TableID              *int64              `json:"table_id,omitempty" example:"301"`
	ReservationID        *int64              `json:"reservation_id,omitempty" example:"8001"`
	Subtotal             int64               `json:"subtotal" example:"5760"`
	DiscountAmount       int64               `json:"discount_amount" example:"500"`
	DeliveryFeeDiscount  int64               `json:"delivery_fee_discount" example:"200"`
	TotalAmount          int64               `json:"total_amount" example:"5760"`
	Status               string              `json:"status" enums:"pending,paid,preparing,ready,courier_accepted,picked,delivering,rider_delivered,user_delivered,completed,cancelled" example:"paid"`
	StatusHint           *string             `json:"status_hint,omitempty"`
	Badges               []orderBadge        `json:"badges,omitempty"`
	Actions              []string            `json:"actions,omitempty"`
	ExceptionState       *string             `json:"exception_state,omitempty"`
	ClaimChannel         *string             `json:"claim_channel,omitempty"`
	Overtime             bool                `json:"overtime,omitempty"`
	FulfillmentStatus    string              `json:"fulfillment_status" enums:"scheduled,pending_kitchen,preparing,ready,completed,cancelled" example:"pending_kitchen"`
	PaymentMethod        *string             `json:"payment_method,omitempty" enums:"wechat,balance" example:"wechat"`
	Notes                *string             `json:"notes,omitempty" example:"不要香菜"`
	Items                []orderItemResponse `json:"items,omitempty"`
	PaidAt               *time.Time          `json:"paid_at,omitempty" example:"2025-12-01T12:30:00Z"`
	PrepStartAt          *time.Time          `json:"prep_start_at,omitempty"`
	ReadyAt              *time.Time          `json:"ready_at,omitempty"`
	CourierAcceptAt      *time.Time          `json:"courier_accept_at,omitempty"`
	PickedAt             *time.Time          `json:"picked_at,omitempty"`
	RiderDeliveredAt     *time.Time          `json:"rider_delivered_at,omitempty"`
	UserDeliveredAt      *time.Time          `json:"user_delivered_at,omitempty"`
	AutoUserDeliveredAt  *time.Time          `json:"auto_user_delivered_at,omitempty"`
	CompletedAt          *time.Time          `json:"completed_at,omitempty" example:"2025-12-01T13:15:00Z"`
	CancelledAt          *time.Time          `json:"cancelled_at,omitempty" example:"2025-12-01T12:25:00Z"`
	CancelReason         *string             `json:"cancel_reason,omitempty" example:"商品缺货"`
	ReplacedByOrderID    *int64              `json:"replaced_by_order_id,omitempty" example:"100009"`
	CreatedAt            time.Time           `json:"created_at" example:"2025-12-01T12:20:00Z"`
	UpdatedAt            *time.Time          `json:"updated_at,omitempty" example:"2025-12-01T12:30:00Z"`
	MerchantPhone        *string             `json:"merchant_phone,omitempty" example:"13800138000"`
	DeliveryContactName  *string             `json:"delivery_contact_name,omitempty" example:"张三"`
	DeliveryContactPhone *string             `json:"delivery_contact_phone,omitempty" example:"13800138000"`
	DeliveryAddress      *string             `json:"delivery_address,omitempty" example:"北京市朝阳区某小区1号楼"`
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
	merchant, err := logic.ValidateMerchantForOrder(ctx, server.store, req.MerchantID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if req.OrderType == OrderTypeTakeout {
		suspension, err := logic.GetTakeoutSuspension(ctx, server.store, merchant.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if suspension != nil {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error":          "merchant takeout ordering is suspended",
				"suspend_reason": suspension.Reason,
				"suspend_until":  suspension.Until,
			})
			return
		}
	}

	log.Info().Msg("[DEBUG] createOrder: merchant validated")

	ruleInput := rules.Context{
		Domain:     rules.DomainOrder,
		RegionID:   merchant.RegionID,
		MerchantID: merchant.ID,
		UserID:     authPayload.UserID,
		OrderType:  req.OrderType,
		Metadata: map[string]interface{}{
			"items_count": len(req.Items),
			"use_balance": req.UseBalance,
		},
	}
	ruleDecision, err := logic.EvaluateRules(ctx, logic.RuleEvaluationInput{
		Enabled:   server.rulesEngine != nil && server.config.RulesEngineEnabled,
		Engine:    server.rulesEngine,
		Context:   ruleInput,
		ActorRole: RoleCustomer,
		OnDecision: func(input rules.Context, decision rules.Decision, actorRole string) {
			server.recordRuleHit(ctx, input, decision, actorRole)
		},
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// P0安全: 堂食订单验证桌台归属商户
	if req.OrderType == OrderTypeDineIn && req.TableID != nil {
		if err := logic.ValidateTableOwnership(ctx, server.store, req.MerchantID, *req.TableID); err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 堂食/预订订单需要开放用餐会话，绑定会话到订单
	var diningSession *db.DiningSession
	var reservation *db.TableReservation
	if req.OrderType == OrderTypeDineIn || req.OrderType == OrderTypeReservation {
		result, err := logic.ValidateOrderSessionAndBilling(ctx, server.store, logic.OrderSessionInput{
			UserID:         authPayload.UserID,
			MerchantID:     req.MerchantID,
			OrderType:      req.OrderType,
			TableID:        req.TableID,
			ReservationID:  req.ReservationID,
			BillingGroupID: req.BillingGroupID,
		})
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if result.DiningSession != nil {
			diningSession = result.DiningSession
		}
		if result.Reservation != nil {
			reservation = result.Reservation
		}
		if result.BillingGroupID != nil {
			req.BillingGroupID = result.BillingGroupID
		}
		if result.TableID != nil {
			req.TableID = result.TableID
		}
	}

	// 定金模式仅允许一笔尾款订单，避免重复抵扣
	if reservation != nil && reservation.PaymentMode == PaymentModeDeposit {
		if err := logic.EnsureReservationSingleActiveOrder(ctx, server.store, reservation.ID); err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 计算订单金额
	subtotal, items, err := logic.CalculateOrderItems(ctx, server.store, req.MerchantID, toOrderItemInputs(req.Items),
		func(_ context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
			normalized, extra, _, err := server.normalizeDishCustomizations(ctx, dishID, customizations)
			if err != nil {
				return nil, 0, err
			}
			if len(normalized) == 0 {
				return nil, extra, nil
			}
			data, err := json.Marshal(normalized)
			if err != nil {
				return nil, 0, err
			}
			return data, extra, nil
		},
	)
	if err != nil {
		log.Warn().Err(err).Msg("[DEBUG] createOrder: calculateOrderItems failed")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	log.Info().Int64("subtotal", subtotal).Int("items_count", len(items)).Msg("[DEBUG] createOrder: items calculated")

	// 计算配送费（仅外卖订单）
	// P1-017 fix: 默认且强制初始化为0，仅在外卖逻辑中通过服务端计算赋值
	var deliveryFee int64 = 0
	var deliveryDistance int32 = 0
	var deliveryFeeDiscount int64 = 0
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

		quote, err := logic.ComputeDeliveryQuote(ctx, logic.DeliveryQuoteInput{
			UserID:    authPayload.UserID,
			OrderType: req.OrderType,
			Subtotal:  subtotal,
			Merchant:  merchant,
			Address:   address,
		}, server.mapClient, func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (logic.DeliveryFeeComputation, error) {
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, merchantID, distance, orderAmount)
			if err != nil {
				return logic.DeliveryFeeComputation{}, err
			}
			return logic.DeliveryFeeComputation{
				Fee:           feeResult.FinalFee,
				Discount:      feeResult.PromotionDiscount,
				Suspended:     feeResult.DeliverySuspended,
				SuspendReason: feeResult.SuspendReason,
			}, nil
		})
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			log.Error().Err(err).
				Int64("merchant_id", req.MerchantID).
				Int64("address_id", *req.AddressID).
				Msg("createOrder: failed to compute delivery fee")
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to calculate delivery fee")))
			return
		}

		deliveryDistance = quote.Distance
		deliveryDuration = quote.Duration
		deliveryFee = quote.Fee
		deliveryFeeDiscount = quote.Discount
	}

	// 计算满减优惠
	var discountAmount int64 = 0
	if bestAmount, err := logic.GetBestDiscountAmount(ctx, server.store, req.MerchantID, subtotal); err == nil {
		discountAmount = bestAmount
	}

	// ==================== 优惠券验证 ====================
	var userVoucherID *int64
	var voucherAmount int64 = 0

	if req.UserVoucherID != nil {
		voucherResult, err := logic.ValidateVoucher(ctx, server.store, logic.VoucherValidationInput{
			UserID:        authPayload.UserID,
			MerchantID:    req.MerchantID,
			OrderType:     req.OrderType,
			Subtotal:      subtotal,
			UserVoucherID: req.UserVoucherID,
		})
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		userVoucherID = voucherResult.UserVoucherID
		voucherAmount = voucherResult.VoucherAmount
	}

	// 定金抵扣：预订定金到店点菜直接抵扣应付
	var depositDeduction int64
	if reservation != nil && reservation.PaymentMode == PaymentModeDeposit {
		depositDeduction = reservation.DepositAmount
	}

	// ==================== 会员余额验证 ====================
	var membershipID *int64
	var balancePaid int64 = 0
	var totalAmount int64 = 0
	var membership *db.MerchantMembership

	if req.UseBalance {
		mem, err := logic.ValidateMembershipPayment(ctx, server.store, logic.MembershipPaymentInput{
			UserID:             authPayload.UserID,
			MerchantID:         req.MerchantID,
			OrderType:          req.OrderType,
			RulesEngineEnabled: server.config.RulesEngineEnabled,
		})
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		membershipID = &mem.ID
		membership = mem
	}

	membershipBalance := int64(0)
	if membership != nil {
		membershipBalance = membership.Balance
	}

	totals, err := logic.ComputeOrderTotals(logic.OrderTotalsInput{
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		VoucherAmount:       voucherAmount,
		DeliveryFee:         deliveryFee,
		DeliveryFeeDiscount: deliveryFeeDiscount,
		DepositDeduction:    depositDeduction,
		MembershipBalance:   membershipBalance,
		UseBalance:          req.UseBalance,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	// 计算总金额（扣除优惠券后）
	totalAmount = totals.TotalAmount
	// 计算余额支付金额
	balancePaid = totals.BalancePaid

	// 外卖拒绝服务检查（仅外卖，不影响预订/堂食）
	if req.OrderType == OrderTypeTakeout && !server.config.RulesEngineEnabled {
		blocked, err := logic.CheckTakeoutBlocklist(ctx, server.store, authPayload.UserID)
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
		CreateOrderParams:  arg,
		Items:              items,
		BillingGroupID:     req.BillingGroupID,
		UserVoucherID:      userVoucherID,
		VoucherAmount:      voucherAmount,
		MembershipID:       membershipID,
		BalancePaid:        balancePaid,
		DeliveryDuration:   deliveryDuration,
		RiderAverageSpeed:  server.config.RiderAverageSpeed,
		DefaultPrepareTime: server.config.DefaultPrepareTime,
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

	// 分发订单支付超时任务 (30分钟后执行)
	if server.taskDistributor != nil && txResult.Order.Status == OrderStatusPending {
		_ = server.taskDistributor.DistributeTaskOrderPaymentTimeout(
			ctx,
			&worker.PayloadOrderPaymentTimeout{OrderID: txResult.Order.ID},
			asynq.ProcessIn(worker.OrderPaymentTimeoutMinutes*time.Minute),
		)
	}

	// 堂食扫码点餐：若用户在外卖拒绝服务名单中，实时提醒商户后台
	if req.OrderType == OrderTypeDineIn && server.wsHub != nil {
		if server.config.RulesEngineEnabled && ruleDecision.Action == "alert" {
			message := ruleDecision.Reason
			if message == "" {
				message = "该顾客有多次恶意索赔记录，谨慎服务"
			}
			alertPayload, _ := json.Marshal(map[string]any{
				"user_id":  authPayload.UserID,
				"order_id": txResult.Order.ID,
				"order_no": txResult.Order.OrderNo,
				"message":  message,
			})
			server.wsHub.SendToMerchant(req.MerchantID, websocket.Message{
				Type:      "merchant_user_risk_alert",
				Data:      alertPayload,
				Timestamp: time.Now(),
			})
		} else {
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
	}

	// 绑定用餐会话的活跃订单（失败不影响下单返回，但记录日志）
	if diningSession != nil {
		if err := logic.BindDiningSessionActiveOrder(ctx, server.store, diningSession.ID, txResult.Order.ID); err != nil {
			log.Warn().Err(err).
				Int64("session_id", diningSession.ID).
				Int64("order_id", txResult.Order.ID).
				Msg("createOrder: failed to bind dining session active order")
		}
	}

	// 清空堂食/预订购物车（外卖保留移除已支付商品逻辑）
	if req.OrderType == OrderTypeDineIn || req.OrderType == OrderTypeReservation {
		_ = logic.ClearDiningOrderCart(ctx, server.store, logic.ClearDiningOrderCartInput{
			UserID:        authPayload.UserID,
			MerchantID:    req.MerchantID,
			OrderType:     req.OrderType,
			TableID:       req.TableID,
			ReservationID: req.ReservationID,
		})
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

func toOrderItemInputs(items []orderItemRequest) []logic.OrderItemInput {
	inputs := make([]logic.OrderItemInput, len(items))
	for i, item := range items {
		inputs[i] = logic.OrderItemInput{
			DishID:         item.DishID,
			ComboID:        item.ComboID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
		}
	}
	return inputs
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
				distance := logic.ExtractDistance(delivery.Distance, order.DeliveryDistance)
				eta := logic.ComputeDeliveryETA(ctx, server.store, order.MerchantID, distance, logic.EstimateDurationSecByDistance(distance))
				resp.DeliveryEtaMinutes = &eta.DeliveryEtaMinutes
				est := time.Now().Add(time.Duration(eta.DeliveryEtaMinutes) * time.Minute)
				resp.EstimatedDeliveryAt = &est
			}
		} else {
			// 未生成配送单（如待支付）也给出基于距离的预计时间
			distance := logic.ExtractDistance(0, order.DeliveryDistance)
			if distance > 0 {
				eta := logic.ComputeDeliveryETA(ctx, server.store, order.MerchantID, distance, logic.EstimateDurationSecByDistance(distance))
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
	result, err := logic.CancelOrder(ctx, server.store, logic.CancelOrderInput{
		UserID:  authPayload.UserID,
		OrderID: uriReq.ID,
		Reason:  bodyReq.Reason,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if result.Refund != nil && server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskProcessRefund(ctx, &worker.PayloadProcessRefund{
			PaymentOrderID: result.Refund.PaymentOrderID,
			OrderID:        result.Order.ID,
			RefundAmount:   result.Refund.Amount,
			Reason:         result.Refund.Reason,
		})
		if err != nil {
			log.Error().Err(err).Int64("order_id", result.Order.ID).Msg("failed to distribute refund task")
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

	result, err := logic.UrgeOrder(ctx, server.store, logic.UrgeOrderInput{
		UserID:          authPayload.UserID,
		OrderID:         uriReq.ID,
		RateLimitWindow: time.Duration(UrgeOrderRateLimitWindowSeconds) * time.Second,
		RateLimitMax:    int64(UrgeOrderRateLimitMaxCount),
		Now:             time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order := result.Order

	if result.NotifyMerchant {
		_ = server.SendNotification(ctx, SendNotificationParams{
			UserID:      order.MerchantID,
			Title:       "用户催单提醒",
			Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快处理", order.OrderNo),
			Type:        "order_urge",
			RelatedType: "order",
			RelatedID:   order.ID,
		})
	}

	if result.RiderID != nil {
		_ = server.SendNotification(ctx, SendNotificationParams{
			UserID:      *result.RiderID,
			Title:       "用户催单提醒",
			Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快送达", order.OrderNo),
			Type:        "order_urge",
			RelatedType: "order",
			RelatedID:   order.ID,
		})
	}

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
	result, err := logic.ReplaceReservationOrder(
		ctx,
		server.store,
		server.paymentClient,
		logic.ReplaceOrderInput{
			UserID:  authPayload.UserID,
			OrderID: uriReq.ID,
			Items:   toOrderItemInputs(req.Items),
			Notes:   req.Notes,
		},
		func(_ context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
			normalized, extra, _, err := server.normalizeDishCustomizations(ctx, dishID, customizations)
			if err != nil {
				return nil, 0, err
			}
			if len(normalized) == 0 {
				return nil, extra, nil
			}
			data, err := json.Marshal(normalized)
			if err != nil {
				return nil, 0, err
			}
			return data, extra, nil
		},
	)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, replaceOrderResponse{
		Order:           newOrderResponse(result.NewOrder),
		Delta:           result.Delta,
		PaymentOrderID:  result.PaymentOrderID,
		RefundInitiated: result.RefundInitiated,
	})
}

// confirmOrderRequest confirms a takeout order receipt.
type confirmOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// confirmOrder godoc
// @Summary 用户确认收货
// @Description 用户确认外卖已送达并完成订单
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} orderResponse "订单完成"
// @Failure 400 {object} ErrorResponse "请求参数错误 / 订单状态不允许"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/orders/{id}/confirm [post]
// @Security BearerAuth
func (server *Server) confirmOrder(ctx *gin.Context) {
	var req confirmOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.ConfirmTakeoutOrder(ctx, server.store, logic.ConfirmOrderInput{
		UserID:  authPayload.UserID,
		OrderID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	order := result.Order
	if result.AlreadyCompleted {
		ctx.JSON(http.StatusOK, newOrderResponse(order))
		return
	}

	// 发送通知给商户和骑手
	_ = server.SendNotification(ctx, SendNotificationParams{
		UserID:      order.MerchantID,
		Title:       "订单已完成",
		Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
		Type:        "order_completed",
		RelatedType: "order",
		RelatedID:   order.ID,
	})

	if result.RiderID != nil {
		_ = server.SendNotification(ctx, SendNotificationParams{
			UserID:      *result.RiderID,
			Title:       "配送已完成",
			Content:     fmt.Sprintf("订单 %s 用户已确认收货", order.OrderNo),
			Type:        "delivery_completed",
			RelatedType: "order",
			RelatedID:   order.ID,
		})
	}

	// 完成触发分账（若是 profit_sharing）
	if server.taskDistributor != nil {
		po, err := server.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: "order",
		})
		if err == nil && po.Status == "paid" && po.PaymentType == "profit_sharing" {
			_ = server.taskDistributor.DistributeTaskProcessProfitSharing(ctx, &worker.ProfitSharingPayload{
				PaymentOrderID: po.ID,
				OrderID:        order.ID,
			})
		}
	}

	ctx.JSON(http.StatusOK, newOrderResponse(order))
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
	}

	offset := pageOffset(req.PageID, req.PageSize)

	var orders []db.Order
	var totalCount int64
	var err error

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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
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
		server.writeAuditLog(ctx, AuditLogInput{
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
	}

	result, err := logic.AcceptMerchantOrder(ctx, server.store, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
		OperatorID: authPayload.UserID,
	})
	if err != nil {
		var reqErr *logic.RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusForbidden && reqErr.Err != nil && reqErr.Err.Error() == "order does not belong to your merchant" {
			server.writeAuditLog(ctx, AuditLogInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "merchant",
				Action:      "merchant_resource_access_denied",
				TargetType:  "order",
				TargetID:    &req.ID,
				RegionID:    &merchant.RegionID,
				Metadata: map[string]any{
					"reason":      "order_not_belong_to_merchant",
					"merchant_id": merchant.ID,
					"order_id":    req.ID,
				},
			})
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_accepted",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": result.Previous.Status,
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
	}

	result, err := logic.RejectMerchantOrder(ctx, server.store, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    uriReq.ID,
		OperatorID: authPayload.UserID,
		Reason:     bodyReq.Reason,
	})
	if err != nil {
		var reqErr *logic.RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusForbidden && reqErr.Err != nil && reqErr.Err.Error() == "order does not belong to your merchant" {
			server.writeAuditLog(ctx, AuditLogInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "merchant",
				Action:      "merchant_resource_access_denied",
				TargetType:  "order",
				TargetID:    &uriReq.ID,
				RegionID:    &merchant.RegionID,
				Metadata: map[string]any{
					"reason":      "order_not_belong_to_merchant",
					"merchant_id": merchant.ID,
					"order_id":    uriReq.ID,
				},
			})
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_rejected",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": result.Previous.Status,
			"to_status":   updatedOrder.Status,
			"reason":      bodyReq.Reason,
		},
	})

	refundResult, err := logic.ProcessMerchantRejectRefund(ctx, server.store, server.paymentClient, logic.MerchantRejectRefundInput{
		OrderID: updatedOrder.ID,
		Reason:  bodyReq.Reason,
	})
	if err != nil {
		if refundResult.RefundOrder != nil {
			log.Error().Err(err).Int64("refund_order_id", refundResult.RefundOrder.ID).Msg("merchant reject refund failed")
		} else {
			log.Error().Err(err).Int64("order_id", updatedOrder.ID).Msg("merchant reject refund failed")
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
	}

	result, err := logic.MarkMerchantOrderReady(ctx, server.store, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
		OperatorID: authPayload.UserID,
	})
	if err != nil {
		var reqErr *logic.RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusForbidden && reqErr.Err != nil && reqErr.Err.Error() == "order does not belong to your merchant" {
			server.writeAuditLog(ctx, AuditLogInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "merchant",
				Action:      "merchant_resource_access_denied",
				TargetType:  "order",
				TargetID:    &req.ID,
				RegionID:    &merchant.RegionID,
				Metadata: map[string]any{
					"reason":      "order_not_belong_to_merchant",
					"merchant_id": merchant.ID,
					"order_id":    req.ID,
				},
			})
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_ready",
		TargetType:  "order",
		TargetID:    &updatedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    updatedOrder.OrderNo,
			"order_type":  updatedOrder.OrderType,
			"from_status": result.Previous.Status,
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
	}

	result, err := logic.CompleteMerchantOrder(ctx, server.store, logic.MerchantOrderUpdateInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
		OperatorID: authPayload.UserID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
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

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_completed",
		TargetType:  "order",
		TargetID:    &completedOrder.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":    completedOrder.OrderNo,
			"order_type":  completedOrder.OrderType,
			"from_status": result.Previous.Status,
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
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		value, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		merchant = value
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
		MerchantID: merchant.ID,
		StartAt:    startDate,
		EndAt:      endDate,
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
	result, err := logic.CalculateOrderPreview(
		ctx,
		server.store,
		server.mapClient,
		logic.OrderCalculationInput{
			UserID:        authPayload.UserID,
			MerchantID:    req.MerchantID,
			OrderType:     req.OrderType,
			Latitude:      req.Latitude,
			Longitude:     req.Longitude,
			AddressID:     req.AddressID,
			UserVoucherID: req.UserVoucherID,
			VoucherCode:   req.VoucherCode,
		},
		func(_ context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
			normalized, extra, _, err := server.normalizeDishCustomizations(ctx, dishID, customizations)
			if err != nil {
				return nil, 0, err
			}
			if len(normalized) == 0 {
				return nil, extra, nil
			}
			data, err := json.Marshal(normalized)
			if err != nil {
				return nil, 0, err
			}
			return data, extra, nil
		},
		func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (logic.DeliveryFeeComputation, error) {
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, merchantID, distance, orderAmount)
			if err != nil {
				return logic.DeliveryFeeComputation{}, err
			}
			return logic.DeliveryFeeComputation{
				Fee:      feeResult.FinalFee,
				Discount: feeResult.PromotionDiscount,
			}, nil
		},
	)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := orderCalculationResponse{
		Subtotal:            result.Subtotal,
		DeliveryFee:         result.DeliveryFee,
		DeliveryFeeDiscount: result.DeliveryFeeDiscount,
		DiscountAmount:      result.DiscountAmount,
		TotalAmount:         result.TotalAmount,
		Items:               make([]orderCalculationItem, len(result.Items)),
		Promotions:          make([]orderPromotion, len(result.Promotions)),
	}

	for i, item := range result.Items {
		response.Items[i] = orderCalculationItem{
			DishID:    item.DishID,
			ComboID:   item.ComboID,
			Name:      item.Name,
			UnitPrice: item.UnitPrice,
			Quantity:  item.Quantity,
			Subtotal:  item.Subtotal,
		}
	}
	for i, promo := range result.Promotions {
		response.Promotions[i] = orderPromotion{
			Type:   promo.Type,
			Title:  promo.Title,
			Amount: promo.Amount,
		}
	}

	ctx.JSON(http.StatusOK, response)
}
