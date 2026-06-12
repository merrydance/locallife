package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
)

// ==================== 订单管理 ====================

// 订单类型常量
const (
	OrderTypeTakeout     = "takeout"     // 外卖
	OrderTypeDineIn      = "dine_in"     // 堂食
	OrderTypeTakeaway    = "takeaway"    // 打包自取
	OrderTypeReservation = "reservation" // 预定包间点菜
)

// 订单状态常量 — 引用 db 层 SSOT，保持 API 层向后兼容
const (
	OrderStatusPending         = db.OrderStatusPending         // 待支付
	OrderStatusPaid            = db.OrderStatusPaid            // 已支付
	OrderStatusPreparing       = db.OrderStatusPreparing       // 制作中
	OrderStatusReady           = db.OrderStatusReady           // 待取餐/待代取
	OrderStatusCourierAccepted = db.OrderStatusCourierAccepted // 骑手已接单
	OrderStatusPicked          = db.OrderStatusPicked          // 已取餐
	OrderStatusDelivering      = db.OrderStatusDelivering      // 代取中
	OrderStatusRiderDelivered  = db.OrderStatusRiderDelivered  // 骑手送达（待用户确认）
	OrderStatusUserDelivered   = db.OrderStatusUserDelivered   // 用户确认送达
	OrderStatusCompleted       = db.OrderStatusCompleted       // 已完成
	OrderStatusCancelled       = db.OrderStatusCancelled       // 已取消
)

// 履约状态常量 — 引用 db 层 SSOT
const (
	FulfillmentStatusScheduled      = db.FulfillmentStatusScheduled      // 已排期
	FulfillmentStatusPendingKitchen = db.FulfillmentStatusPendingKitchen // 待出餐
	FulfillmentStatusPreparing      = db.FulfillmentStatusPreparing      // 制作中
	FulfillmentStatusReady          = db.FulfillmentStatusReady          // 出餐完成
	FulfillmentStatusCompleted      = db.FulfillmentStatusCompleted      // 履约完成
	FulfillmentStatusCancelled      = db.FulfillmentStatusCancelled      // 履约取消
)

// 支付方式常量
const (
	PaymentMethodWechat  = "wechat"
	PaymentMethodBalance = "balance"
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

	// 代取地址ID (外卖订单必填)
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

	// 前端已计算的代取费（分），用于直落且供服务端校验
	DeliveryFee int64 `json:"delivery_fee,omitempty" example:"500"`
	// 前端已计算的代取费优惠（分）
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount,omitempty" example:"200"`
	// 前端已计算的代取距离（米）
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

	// 规格文本（商户端稳定展示字段，无规格时为空字符串）
	SpecsText string `json:"specs_text" example:"规格：大份 / 辣度：微辣"`

	// 定制化选项列表
	Customizations []orderCustomizationItem `json:"customizations,omitempty"`

	// 商品图片URL
	ImageAssetID *int64 `json:"-"`
	ImageURL     string `json:"image_url,omitempty"`
}

type orderBadge struct {
	Text   string `json:"text,omitempty"`
	Type   string `json:"type,omitempty"`
	Locale string `json:"locale,omitempty"`
}

type orderPaymentContextResponse struct {
	CombinedPaymentID int64  `json:"combined_payment_id" example:"9001"`
	CombineOutTradeNo string `json:"combine_out_trade_no" example:"CP202604061234560001"`
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
	// 微信或上游支付交易号，用于支付查询与历史兼容展示
	WechatTransactionID *string                            `json:"wechat_transaction_id,omitempty"`
	PaymentContext      *orderPaymentContextResponse       `json:"payment_context,omitempty"`
	FeeBreakdown        *merchantOrderFeeBreakdownResponse `json:"fee_breakdown,omitempty"`
}

type orderRefundSubmissionResponse struct {
	Status      string `json:"status" example:"pending_recovery"`
	Message     string `json:"message" example:"订单已取消，退款提交暂未确认，系统会稍后自动重试。"`
	RefundID    *int64 `json:"refund_id,omitempty" example:"7001"`
	OutRefundNo string `json:"out_refund_no,omitempty" example:"RF202606010001"`
}

type merchantRejectOrderResponse struct {
	orderResponse
	Order            orderResponse                  `json:"order"`
	RefundSubmission *orderRefundSubmissionResponse `json:"refund_submission,omitempty"`
}

func newOrderPaymentContext(combinedPaymentID pgtype.Int8, combineOutTradeNo string) *orderPaymentContextResponse {
	if !combinedPaymentID.Valid || combineOutTradeNo == "" {
		return nil
	}

	return &orderPaymentContextResponse{
		CombinedPaymentID: combinedPaymentID.Int64,
		CombineOutTradeNo: combineOutTradeNo,
	}
}

func newOrderRefundSubmissionResponse(submission *logic.MerchantRefundSubmission) *orderRefundSubmissionResponse {
	if submission == nil || submission.Status == "" {
		return nil
	}
	resp := &orderRefundSubmissionResponse{
		Status:  submission.Status,
		Message: submission.Message,
	}
	if submission.RefundOrder != nil {
		refundID := submission.RefundOrder.ID
		resp.RefundID = &refundID
		resp.OutRefundNo = submission.RefundOrder.OutRefundNo
	}
	return resp
}

func newOrderResponse(o db.Order) (orderResponse, error) {
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

	nullableFields := orderNullableFields{
		AddressID: o.AddressID, DeliveryDistance: o.DeliveryDistance,
		TableID: o.TableID, ReservationID: o.ReservationID,
		PaymentMethod: o.PaymentMethod, Notes: o.Notes,
		PaidAt: o.PaidAt, CompletedAt: o.CompletedAt,
		CancelledAt: o.CancelledAt, CancelReason: o.CancelReason,
		ReplacedByOrderID: o.ReplacedByOrderID, UpdatedAt: o.UpdatedAt,
		PickupCode: o.PickupCode, DispatchOrderID: o.DispatchOrderID,
		FlowID: o.FlowID, StatusHint: o.StatusHint,
		ExceptionState: o.ExceptionState, ClaimChannel: o.ClaimChannel,
		PrepStartAt: o.PrepStartAt, ReadyAt: o.ReadyAt,
		CourierAcceptAt: o.CourierAcceptAt, PickedAt: o.PickedAt,
		RiderDeliveredAt: o.RiderDeliveredAt, UserDeliveredAt: o.UserDeliveredAt,
		AutoUserDeliveredAt: o.AutoUserDeliveredAt, DeliveryDuration: o.DeliveryDuration,
		Badges: o.Badges,
	}
	if err := nullableFields.applyTo(&resp); err != nil {
		return orderResponse{}, fmt.Errorf("decode order %d badges: %w", o.ID, err)
	}

	resp.Actions = orderActions(o)

	return resp, nil
}

func (server *Server) requireMerchantForOrder(ctx *gin.Context, userID int64) (db.Merchant, bool) {
	if merchant, ok := GetMerchantFromContext(ctx); ok {
		return merchant, true
	}

	merchant, err := server.resolveMerchantForUser(ctx, userID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return db.Merchant{}, false
	}

	return merchant, true
}

// newOrderWithDetailsResponse 用于订单详情，包含商户信息和代取地址
func newOrderWithDetailsResponse(o db.GetOrderWithDetailsRow) (orderResponse, error) {
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

	nullableFields := orderNullableFields{
		AddressID: o.AddressID, DeliveryDistance: o.DeliveryDistance,
		TableID: o.TableID, ReservationID: o.ReservationID,
		PaymentMethod: o.PaymentMethod, Notes: o.Notes,
		PaidAt: o.PaidAt, CompletedAt: o.CompletedAt,
		CancelledAt: o.CancelledAt, CancelReason: o.CancelReason,
		ReplacedByOrderID: o.ReplacedByOrderID, UpdatedAt: o.UpdatedAt,
		PickupCode: o.PickupCode, DispatchOrderID: o.DispatchOrderID,
		FlowID: o.FlowID, StatusHint: o.StatusHint,
		ExceptionState: o.ExceptionState, ClaimChannel: o.ClaimChannel,
		PrepStartAt: o.PrepStartAt, ReadyAt: o.ReadyAt,
		CourierAcceptAt: o.CourierAcceptAt, PickedAt: o.PickedAt,
		RiderDeliveredAt: o.RiderDeliveredAt, UserDeliveredAt: o.UserDeliveredAt,
		AutoUserDeliveredAt: o.AutoUserDeliveredAt, DeliveryDuration: o.DeliveryDuration,
		Badges: o.Badges,
	}
	if err := nullableFields.applyTo(&resp); err != nil {
		return orderResponse{}, fmt.Errorf("decode order %d badges: %w", o.ID, err)
	}

	// 订单详情独有：商户电话和代取地址
	if o.MerchantPhone != "" {
		resp.MerchantPhone = &o.MerchantPhone
	}
	if o.DeliveryContactName != "" {
		resp.DeliveryContactName = ptrString(o.DeliveryContactName)
	}
	if o.DeliveryContactPhone != "" {
		resp.DeliveryContactPhone = ptrString(o.DeliveryContactPhone)
	}
	if o.DeliveryAddress != "" {
		resp.DeliveryAddress = ptrString(o.DeliveryAddress)
	}
	resp.PaymentContext = newOrderPaymentContext(o.CombinedPaymentID, o.CombineOutTradeNo)

	resp.Actions = orderActions(db.Order{
		ID:                o.ID,
		Status:            o.Status,
		FulfillmentStatus: o.FulfillmentStatus,
	})

	return resp, nil
}

func decodeOrderBadges(raw []byte) ([]orderBadge, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var badges []orderBadge
	if err := json.Unmarshal(raw, &badges); err != nil {
		return nil, err
	}

	return badges, nil
}

func maskPickupCode(code string) *string {
	if code == "" {
		return nil
	}
	if len(code) <= 4 {
		return ptrString(code)
	}
	masked := "****" + code[len(code)-2:]
	return &masked
}

func formatPickupCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" || len(trimmed) > 4 {
		return trimmed
	}
	for _, r := range trimmed {
		if !unicode.IsDigit(r) {
			return trimmed
		}
	}
	return fmt.Sprintf("%04s", trimmed)
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
func newOrderWithMerchantFromFilterResponse(o db.ListOrdersByUserWithFiltersRow) (orderResponse, error) {
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

	nullableFields := orderNullableFields{
		AddressID: o.AddressID, DeliveryDistance: o.DeliveryDistance,
		TableID: o.TableID, ReservationID: o.ReservationID,
		PaymentMethod: o.PaymentMethod, Notes: o.Notes,
		PaidAt: o.PaidAt, CompletedAt: o.CompletedAt,
		CancelledAt: o.CancelledAt, CancelReason: o.CancelReason,
		ReplacedByOrderID: o.ReplacedByOrderID, UpdatedAt: o.UpdatedAt,
		PickupCode: o.PickupCode, DispatchOrderID: o.DispatchOrderID,
		FlowID: o.FlowID, StatusHint: o.StatusHint,
		ExceptionState: o.ExceptionState, ClaimChannel: o.ClaimChannel,
		PrepStartAt: o.PrepStartAt, ReadyAt: o.ReadyAt,
		CourierAcceptAt: o.CourierAcceptAt, PickedAt: o.PickedAt,
		RiderDeliveredAt: o.RiderDeliveredAt, UserDeliveredAt: o.UserDeliveredAt,
		AutoUserDeliveredAt: o.AutoUserDeliveredAt, DeliveryDuration: o.DeliveryDuration,
		Badges: o.Badges,
	}
	if err := nullableFields.applyTo(&resp); err != nil {
		return orderResponse{}, fmt.Errorf("decode order %d badges: %w", o.ID, err)
	}
	resp.PaymentContext = newOrderPaymentContext(o.CombinedPaymentID, o.CombineOutTradeNo)

	return resp, nil
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
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if err := validateOrderTypeFields(req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.CreateOrder(ctx, logic.CreateOrderCommandInput{
		UserID:             authPayload.UserID,
		MerchantID:         req.MerchantID,
		OrderType:          req.OrderType,
		AddressID:          req.AddressID,
		TableID:            req.TableID,
		ReservationID:      req.ReservationID,
		BillingGroupID:     req.BillingGroupID,
		Items:              toOrderItemInputs(req.Items),
		Notes:              req.Notes,
		UserVoucherID:      req.UserVoucherID,
		UseBalance:         req.UseBalance,
		RulesEngine:        server.rulesEngine,
		RulesEngineEnabled: server.config.RulesEngineEnabled,
		OnRuleDecision: func(input rules.Context, decision rules.Decision, actorRole string) {
			server.recordRuleHit(ctx, input, decision, actorRole)
		},
		MapClient: server.mapClient,
		DeliveryFeeCalculator: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (logic.DeliveryFeeComputation, error) {
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
		},
		RiderAverageSpeed:  server.config.RiderAverageSpeed,
		DefaultPrepareTime: server.config.DefaultPrepareTime,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp, err := newOrderResponse(result.Order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusCreated, resp)
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
		if req.AddressID != nil {
			return errors.New("address_id is not allowed for takeaway orders")
		}
		if req.DeliveryFee != 0 {
			return errors.New("delivery_fee is not allowed for takeaway orders")
		}
		if req.DeliveryFeeDiscount != 0 {
			return errors.New("delivery_fee_discount is not allowed for takeaway orders")
		}
		if req.DeliveryDistance != 0 {
			return errors.New("delivery_distance is not allowed for takeaway orders")
		}
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
// @Success 201 {object} orderResponse "订单详情"
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
	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	result, err := service.GetUserOrder(ctx, logic.GetUserOrderQueryInput{
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

	resp, err := newOrderWithDetailsResponse(order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	itemViews, err := logic.BuildOrderItemViews(result.Items)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.Items = server.newOrderItemResponses(ctx, itemViews, true)
	resp.DeliveryEtaMinutes = result.DeliveryEtaMinutes
	resp.EstimatedDeliveryAt = result.EstimatedDeliveryAt
	resp.WechatTransactionID = result.WechatTransactionID
	if result.FeeBreakdown != nil {
		resp.FeeBreakdown = newMerchantOrderFeeBreakdownResponse(*result.FeeBreakdown)
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
	Orders   []orderResponse `json:"orders"`
	Total    int64           `json:"total"`
	PageID   int32           `json:"page_id"`
	PageSize int32           `json:"page_size"`
}

type urgeOrderResponse struct {
	Message string `json:"message"`
	OrderID int64  `json:"order_id"`
}

// listOrders godoc
// @Summary 获取订单列表
// @Description 分页获取当前用户的订单列表，支持按状态筛选。
// @Description
// @Description **订单状态枚举：**
// @Description - pending: 待支付
// @Description - paid: 已支付
// @Description - preparing: 制作中
// @Description - ready: 待代取/待取餐
// @Description - courier_accepted: 骑手已接单
// @Description - picked: 已取餐
// @Description - delivering: 代取中
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
	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	var status *string
	if req.Status != "" {
		status = &req.Status
	}
	var orderType *string
	if req.OrderType != "" {
		orderType = &req.OrderType
	}

	result, err := service.ListUserOrders(ctx, logic.ListUserOrdersQueryInput{
		UserID:        authPayload.UserID,
		Status:        status,
		OrderType:     orderType,
		ReservationID: req.ReservationID,
		PageID:        req.PageID,
		PageSize:      req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	orders := result.Orders
	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		orderResp, err := newOrderWithMerchantFromFilterResponse(o)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		resp[i] = orderResp
	}

	ctx.JSON(http.StatusOK, listOrdersResponse{
		Orders:   resp,
		Total:    result.TotalCount,
		PageID:   req.PageID,
		PageSize: req.PageSize,
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
	result, err := server.orderCommandSvc.CancelOrder(ctx, logic.CancelOrderInput{
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

	resp, err := newOrderResponse(result.Order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
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

	result, err := server.orderCommandSvc.UrgeOrder(ctx, logic.UrgeOrderInput{
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

	ctx.JSON(http.StatusOK, urgeOrderResponse{Message: "urge notification sent", OrderID: order.ID})
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
		respondPaymentRequestError(ctx, "replace_order_bind_uri", err, "改菜订单编号无效，请刷新页面后重试")
		return
	}

	var req replaceOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondPaymentRequestError(ctx, "replace_order_bind_request", err, "改菜请求参数格式无效，请至少选择一个菜品后重试")
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := server.orderCommandSvc.ReplaceOrder(ctx, logic.ReplaceOrderInput{
		UserID:   authPayload.UserID,
		OrderID:  uriReq.ID,
		Items:    toOrderItemInputs(req.Items),
		Notes:    req.Notes,
		ClientIP: ctx.ClientIP(),
	})
	if err != nil {
		if isPaymentServiceNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "商户支付能力未完成配置，当前无法处理改菜支付或退款，请联系平台处理后重试", "replace order payment client not configured"))
			return
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	orderResp, err := newOrderResponse(result.NewOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, replaceOrderResponse{
		Order:           orderResp,
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

	result, err := server.orderCommandSvc.ConfirmOrder(ctx, logic.ConfirmOrderInput{
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
		resp, err := newOrderResponse(order)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusOK, resp)
		return
	}

	resp, err := newOrderResponse(order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
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

	// 订单类型筛选 (选填，枚举值: takeout,dine_in,takeaway,reservation)
	OrderType string `form:"order_type" binding:"omitempty,oneof=takeout dine_in takeaway reservation" enums:"takeout,dine_in,takeaway,reservation" example:"reservation"`
}

type listMerchantOrdersResponse struct {
	Orders   []orderResponse `json:"orders"`
	Total    int64           `json:"total"`
	PageID   int32           `json:"page_id"`
	PageSize int32           `json:"page_size"`
}

// listMerchantOrders godoc
// @Summary 获取商户订单列表
// @Description 分页获取当前商户的订单列表，支持按状态筛选
// @Description
// @Description **订单状态枚举：**
// @Description - pending: 待支付
// @Description - paid: 已支付
// @Description - preparing: 制作中
// @Description - ready: 待代取/待取餐
// @Description - courier_accepted: 骑手已接单
// @Description - picked: 已取餐
// @Description - delivering: 代取中
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
// @Param order_type query string false "订单类型筛选" Enums(takeout,dine_in,takeaway,reservation)
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	var status *string
	if req.Status != "" {
		status = &req.Status
	}
	var orderType *string
	if req.OrderType != "" {
		orderType = &req.OrderType
	}

	result, err := service.ListMerchantOrders(ctx, logic.ListMerchantOrdersQueryInput{
		MerchantID: merchant.ID,
		Status:     status,
		OrderType:  orderType,
		PageID:     req.PageID,
		PageSize:   req.PageSize,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	orders := result.Orders
	resp := make([]orderResponse, len(orders))
	for index, order := range orders {
		orderResp, err := newOrderResponse(order)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		itemRows := result.ItemsByOrderID[order.ID]
		itemViews, err := logic.BuildOrderItemViewsFromOrderIDs(itemRows)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		orderResp.Items = server.newOrderItemResponses(ctx, itemViews, false)
		resp[index] = orderResp
	}
	feeBreakdowns, err := server.loadMerchantOrderFeeBreakdowns(ctx, merchant.ID, orders)
	if err != nil {
		writeMerchantOrderFeeBreakdownError(ctx, merchant.ID, orders, err)
		return
	}
	for index, order := range orders {
		if breakdown, ok := feeBreakdowns[order.ID]; ok {
			resp[index].FeeBreakdown = newMerchantOrderFeeBreakdownResponse(breakdown)
		}
	}

	ctx.JSON(http.StatusOK, listMerchantOrdersResponse{
		Orders:   resp,
		Total:    result.TotalCount,
		PageID:   req.PageID,
		PageSize: req.PageSize,
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	result, err := service.GetMerchantOrder(ctx, logic.GetMerchantOrderQueryInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
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

	order := result.Order

	resp, err := newOrderResponse(order)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	itemViews, err := logic.BuildOrderItemViews(result.Items)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.Items = server.newOrderItemResponses(ctx, itemViews, false)
	feeBreakdowns, err := server.loadMerchantOrderFeeBreakdowns(ctx, merchant.ID, []db.Order{order})
	if err != nil {
		writeMerchantOrderFeeBreakdownError(ctx, merchant.ID, []db.Order{order}, err)
		return
	}
	if breakdown, ok := feeBreakdowns[order.ID]; ok {
		resp.FeeBreakdown = newMerchantOrderFeeBreakdownResponse(breakdown)
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.AcceptMerchantOrder(ctx, logic.MerchantOrderUpdateInput{
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

	resp, err := newOrderResponse(updatedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
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
// @Description 商户拒绝订单并说明原因，将触发自动退款；响应兼容顶层订单字段并通过 order/refund_submission 返回退款提交状态，订单取消成功不代表支付通道已受理退款
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param request body rejectOrderBody true "拒单原因"
// @Success 200 {object} merchantRejectOrderResponse "拒单成功，包含退款提交状态"
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.RejectMerchantOrder(ctx, logic.MerchantOrderUpdateInput{
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

	resp, err := newOrderResponse(updatedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, merchantRejectOrderResponse{
		orderResponse:    resp,
		Order:            resp,
		RefundSubmission: newOrderRefundSubmissionResponse(result.RefundSubmission),
	})
}

// markOrderReady 标记订单出餐完成
type markOrderReadyRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

// markOrderReady godoc
// @Summary 标记出餐完成
// @Description 商户标记订单已出餐，等待代取或顾客取餐
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.MarkMerchantOrderReady(ctx, logic.MerchantOrderUpdateInput{
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

	resp, err := newOrderResponse(updatedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// completeOrder 完成订单（堂食/打包自取）
type completeOrderRequest struct {
	// 订单ID (必填)
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

type printMerchantOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

type printMerchantOrderResponse struct {
	Message string `json:"message"`
	OrderID int64  `json:"order_id"`
	Trigger string `json:"trigger"`
}

type listMerchantOrderPrintJobsRequest struct {
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

type getMerchantOrderPrintJobStatusRequest struct {
	ID         int64 `uri:"id" binding:"required,min=1" example:"100001"`
	PrintLogID int64 `uri:"print_log_id" binding:"required,min=1" example:"200001"`
}

type merchantOrderPrintJobResponse struct {
	ID            int64      `json:"id"`
	OrderID       int64      `json:"order_id"`
	PrinterID     int64      `json:"printer_id"`
	PrinterName   string     `json:"printer_name"`
	PrinterType   string     `json:"printer_type"`
	Status        string     `json:"status"`
	VendorOrderID *string    `json:"vendor_order_id,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	PrintedAt     *time.Time `json:"printed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type listMerchantOrderPrintJobsResponse struct {
	OrderID int64                           `json:"order_id"`
	Items   []merchantOrderPrintJobResponse `json:"items"`
}

type merchantOrderPrintJobStatusResponse struct {
	PrintLogID          int64     `json:"print_log_id"`
	OrderID             int64     `json:"order_id"`
	PrinterID           int64     `json:"printer_id"`
	PrinterName         string    `json:"printer_name"`
	PrinterType         string    `json:"printer_type"`
	LocalStatus         string    `json:"local_status"`
	VendorOrderID       *string   `json:"vendor_order_id,omitempty"`
	CloudQueryAvailable bool      `json:"cloud_query_available"`
	CloudPrinted        *bool     `json:"cloud_printed,omitempty"`
	CheckedAt           time.Time `json:"checked_at"`
}

type listMerchantPrintAnomaliesRequest struct {
	PageID   int32  `form:"page_id" binding:"omitempty,min=1" example:"1"`
	PageSize int32  `form:"page_size" binding:"omitempty,min=5,max=50" example:"20"`
	Status   string `form:"status" binding:"omitempty,oneof=failed pending cancelled" example:"failed"`
}

type merchantPrintAnomalyResponse struct {
	PrintLogID    int64     `json:"print_log_id"`
	OrderID       int64     `json:"order_id"`
	OrderNo       string    `json:"order_no"`
	OrderType     string    `json:"order_type"`
	PrinterID     int64     `json:"printer_id"`
	PrinterName   string    `json:"printer_name"`
	PrinterType   string    `json:"printer_type"`
	LocalStatus   string    `json:"local_status"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
	VendorOrderID *string   `json:"vendor_order_id,omitempty"`
	LastAttemptAt time.Time `json:"last_attempt_at"`
	CanRetry      bool      `json:"can_retry"`
	RetryHint     string    `json:"retry_hint,omitempty"`
}

type listMerchantPrintAnomaliesResponse struct {
	Items    []merchantPrintAnomalyResponse `json:"items"`
	Total    int64                          `json:"total"`
	PageID   int32                          `json:"page_id"`
	PageSize int32                          `json:"page_size"`
}

type retryMerchantOrderPrintJobRequest struct {
	ID         int64 `uri:"id" binding:"required,min=1" example:"100001"`
	PrintLogID int64 `uri:"print_log_id" binding:"required,min=1" example:"200001"`
}

type retryMerchantOrderPrintJobResponse struct {
	Message    string `json:"message"`
	OrderID    int64  `json:"order_id"`
	PrintLogID int64  `json:"print_log_id"`
	Trigger    string `json:"trigger"`
}

type recordMerchantLocalPrintEventRequest struct {
	ID int64 `uri:"id" binding:"required,min=1" example:"100001"`
}

type recordMerchantLocalPrintEventBody struct {
	EventKey     string  `json:"event_key" binding:"required,max=160"`
	Status       string  `json:"status" binding:"required,oneof=started success failed"`
	PrinterName  *string `json:"printer_name" binding:"omitempty,max=100"`
	ErrorMessage *string `json:"error_message" binding:"omitempty,max=500"`
}

type merchantLocalPrintEventResponse struct {
	ID           int64      `json:"id"`
	MerchantID   int64      `json:"merchant_id"`
	OrderID      int64      `json:"order_id"`
	EventKey     string     `json:"event_key"`
	Source       string     `json:"source"`
	Status       string     `json:"status"`
	PrinterName  *string    `json:"printer_name,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	PrintedAt    *time.Time `json:"printed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

func toMerchantLocalPrintEventResponse(event db.MerchantLocalPrintEvent) merchantLocalPrintEventResponse {
	return merchantLocalPrintEventResponse{
		ID:           event.ID,
		MerchantID:   event.MerchantID,
		OrderID:      event.OrderID,
		EventKey:     event.EventKey,
		Source:       event.Source,
		Status:       event.Status,
		PrinterName:  pgTextToPtr(event.PrinterName),
		ErrorMessage: pgTextToPtr(event.ErrorMessage),
		PrintedAt:    pgTimeToPtr(event.PrintedAt),
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    pgTimeToPtr(event.UpdatedAt),
	}
}

func toMerchantOrderPrintJobResponse(item db.ListPrintLogsByOrderRow) merchantOrderPrintJobResponse {
	responseItem := merchantOrderPrintJobResponse{
		ID:          item.ID,
		OrderID:     item.OrderID,
		PrinterID:   item.PrinterID,
		PrinterName: item.PrinterName,
		PrinterType: item.PrinterType,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt,
	}
	if item.VendorOrderID.Valid {
		vendorOrderID := item.VendorOrderID.String
		responseItem.VendorOrderID = &vendorOrderID
	}
	if item.ErrorMessage.Valid {
		errorMessage := item.ErrorMessage.String
		responseItem.ErrorMessage = &errorMessage
	}
	if item.PrintedAt.Valid {
		printedAt := item.PrintedAt.Time
		responseItem.PrintedAt = &printedAt
	}
	return responseItem
}

func toMerchantPrintAnomalyResponse(item db.ListMerchantPrintAnomaliesRow) merchantPrintAnomalyResponse {
	responseItem := merchantPrintAnomalyResponse{
		PrintLogID:    item.ID,
		OrderID:       item.OrderID,
		OrderNo:       item.OrderNo,
		OrderType:     item.OrderType,
		PrinterID:     item.PrinterID,
		PrinterName:   item.PrinterName,
		PrinterType:   item.PrinterType,
		LocalStatus:   item.Status,
		LastAttemptAt: item.CreatedAt,
	}
	if item.ErrorMessage.Valid {
		errorMessage := item.ErrorMessage.String
		responseItem.ErrorMessage = &errorMessage
	}
	if item.VendorOrderID.Valid {
		vendorOrderID := item.VendorOrderID.String
		responseItem.VendorOrderID = &vendorOrderID
	}
	if !item.IsActive {
		responseItem.RetryHint = "printer is inactive"
	}
	return responseItem
}

func (server *Server) toMerchantPrintAnomalyResponse(item db.ListMerchantPrintAnomaliesRow) merchantPrintAnomalyResponse {
	responseItem := toMerchantPrintAnomalyResponse(item)
	if !item.IsActive {
		return responseItem
	}
	if !isPrintLogStatusRetryable(item.Status) {
		responseItem.RetryHint = "print job is still pending"
		return responseItem
	}
	if !server.cloudPrinterPrintRetryAvailable(item.PrinterType) {
		responseItem.RetryHint = "cloud printer provider is not configured"
		return responseItem
	}
	responseItem.CanRetry = true
	responseItem.RetryHint = ""
	return responseItem
}

func isPrintLogStatusRetryable(status string) bool {
	switch status {
	case db.PrintLogStatusFailed, db.PrintLogStatusCancelled:
		return true
	default:
		return false
	}
}

func (server *Server) cloudPrinterPrintRetryAvailable(providerType string) bool {
	if providerType == printerTypeYilianyun {
		return server != nil &&
			server.config.YilianyunEnabled &&
			strings.TrimSpace(server.config.YilianyunAPIBaseURL) != "" &&
			strings.TrimSpace(server.config.YilianyunAppID) != "" &&
			strings.TrimSpace(server.config.YilianyunAppSecret) != ""
	}
	_, ok := server.cloudPrinterProvider(providerType)
	return ok
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.CompleteMerchantOrder(ctx, logic.MerchantOrderUpdateInput{
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

	resp, err := newOrderResponse(completedOrder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// listMerchantOrderPrintJobs godoc
// @Summary 查询订单打印记录
// @Description 查询指定订单的打印任务执行记录，包括打印机、状态和失败原因
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} listMerchantOrderPrintJobsResponse "打印记录列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/print-jobs [get]
// @Security BearerAuth
func (server *Server) listMerchantOrderPrintJobs(ctx *gin.Context) {
	var req listMerchantOrderPrintJobsRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderQuerySvc
	if service == nil {
		if qs, ok := server.buildOrderCommandService().(logic.OrderQueryService); ok {
			service = qs
		}
	}
	if service == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("order query service is not configured")))
		return
	}

	result, err := service.GetMerchantOrder(ctx, logic.GetMerchantOrderQueryInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
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

	logs, err := server.store.ListPrintLogsByOrder(ctx, result.Order.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]merchantOrderPrintJobResponse, 0, len(logs))
	for _, item := range logs {
		items = append(items, toMerchantOrderPrintJobResponse(item))
	}

	ctx.JSON(http.StatusOK, listMerchantOrderPrintJobsResponse{
		OrderID: result.Order.ID,
		Items:   items,
	})
}

// getMerchantOrderPrintJobStatus godoc
// @Summary 查询打印任务云端状态
// @Description 查询指定订单打印任务在云打印平台侧的执行状态；易联云授权型打印机暂不支持此实时查询
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param print_log_id path int true "打印记录ID" minimum(1)
// @Success 200 {object} merchantOrderPrintJobStatusResponse "打印任务云端状态"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单或打印记录不存在"
// @Failure 501 {object} ErrorResponse "当前环境不支持查询"
// @Failure 502 {object} ErrorResponse "云打印平台调用失败"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/print-jobs/{print_log_id}/status [get]
// @Security BearerAuth
func (server *Server) getMerchantOrderPrintJobStatus(ctx *gin.Context) {
	var req getMerchantOrderPrintJobStatusRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderQuerySvc
	if service == nil {
		if qs, ok := server.buildOrderCommandService().(logic.OrderQueryService); ok {
			service = qs
		}
	}
	if service == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("order query service is not configured")))
		return
	}

	result, err := service.GetMerchantOrder(ctx, logic.GetMerchantOrderQueryInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	printLog, err := server.store.GetPrintLog(ctx, req.PrintLogID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("print log not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if printLog.OrderID != result.Order.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("print log not found")))
		return
	}

	printer, err := server.store.GetCloudPrinterIncludingDeleted(ctx, printLog.PrinterID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
		return
	}
	resp := merchantOrderPrintJobStatusResponse{
		PrintLogID:          printLog.ID,
		OrderID:             printLog.OrderID,
		PrinterID:           printLog.PrinterID,
		PrinterName:         printer.PrinterName,
		PrinterType:         printer.PrinterType,
		LocalStatus:         printLog.Status,
		CloudQueryAvailable: printLog.VendorOrderID.Valid && !printer.DeletedAt.Valid,
		CheckedAt:           time.Now(),
	}
	if printLog.VendorOrderID.Valid {
		vendorOrderID := printLog.VendorOrderID.String
		resp.VendorOrderID = &vendorOrderID
	}
	if resp.CloudQueryAvailable {
		printerProvider, ok := server.cloudPrinterProvider(printer.PrinterType)
		if !ok {
			ctx.JSON(http.StatusNotImplemented, errorResponse(errors.New("current printer type or environment does not support cloud print status query")))
			return
		}
		cloudPrinted, err := printerProvider.QueryOrderState(ctx, printLog.VendorOrderID.String)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud print status unavailable", "cloud print order state query failed"))
			return
		}
		resp.CloudPrinted = &cloudPrinted
	}

	ctx.JSON(http.StatusOK, resp)
}

// listMerchantPrintAnomalies godoc
// @Summary 查询打印异常列表
// @Description 查询商户当前仍未恢复的打印异常，按订单和打印机聚合最新一次结果
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param page_id query int false "页码(从1开始)" minimum(1)
// @Param page_size query int false "每页条数" minimum(5) maximum(50)
// @Param status query string false "异常状态过滤" Enums(failed,pending,cancelled)
// @Success 200 {object} listMerchantPrintAnomaliesResponse "打印异常列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/print-anomalies [get]
// @Security BearerAuth
func (server *Server) listMerchantPrintAnomalies(ctx *gin.Context) {
	var req listMerchantPrintAnomaliesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.PageID == 0 {
		req.PageID = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	status := pgtype.Text{}
	if req.Status != "" {
		status = pgtype.Text{String: req.Status, Valid: true}
	}
	offset := (req.PageID - 1) * req.PageSize
	items, err := server.store.ListMerchantPrintAnomalies(ctx, db.ListMerchantPrintAnomaliesParams{
		MerchantID: merchant.ID,
		Limit:      req.PageSize,
		Offset:     offset,
		Status:     status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	total, err := server.store.CountMerchantPrintAnomalies(ctx, db.CountMerchantPrintAnomaliesParams{
		MerchantID: merchant.ID,
		Status:     status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responseItems := make([]merchantPrintAnomalyResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, server.toMerchantPrintAnomalyResponse(item))
	}

	ctx.JSON(http.StatusOK, listMerchantPrintAnomaliesResponse{
		Items:    responseItems,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	})
}

// retryMerchantOrderPrintJob godoc
// @Summary 重试异常打印任务
// @Description 基于原始打印记录重新投递一次同打印机补打任务
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param print_log_id path int true "打印记录ID" minimum(1)
// @Success 200 {object} retryMerchantOrderPrintJobResponse "重试任务已创建"
// @Failure 400 {object} ErrorResponse "打印记录当前不可重试"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单或打印记录不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry [post]
// @Security BearerAuth
func (server *Server) retryMerchantOrderPrintJob(ctx *gin.Context) {
	var req retryMerchantOrderPrintJobRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderQuerySvc
	if service == nil {
		if qs, ok := server.buildOrderCommandService().(logic.OrderQueryService); ok {
			service = qs
		}
	}
	if service == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("order query service is not configured")))
		return
	}

	result, err := service.GetMerchantOrder(ctx, logic.GetMerchantOrderQueryInput{
		MerchantID: merchant.ID,
		OrderID:    req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	printLog, err := server.store.GetPrintLog(ctx, req.PrintLogID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("print log not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if printLog.OrderID != result.Order.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("print log not found")))
		return
	}
	if !isPrintLogStatusRetryable(printLog.Status) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only failed or cancelled print logs can be retried")))
		return
	}

	latestPrintLog, err := server.store.GetLatestPrintLogByOrderAndPrinter(ctx, db.GetLatestPrintLogByOrderAndPrinterParams{
		OrderID:   result.Order.ID,
		PrinterID: printLog.PrinterID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("print log not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if latestPrintLog.ID != printLog.ID {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only the latest failed or cancelled print log can be retried")))
		return
	}

	printer, err := server.store.GetCloudPrinterIncludingDeleted(ctx, printLog.PrinterID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
		return
	}
	if printer.DeletedAt.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("printer is deleted")))
		return
	}
	if !printer.IsActive {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("printer is inactive")))
		return
	}
	if !server.cloudPrinterPrintRetryAvailable(printer.PrinterType) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("cloud printer provider is not configured")))
		return
	}
	if server.taskDistributor == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("print scheduler is not configured")))
		return
	}

	if err := server.taskDistributor.DistributeTaskPrintOrder(ctx, &worker.PrintOrderPayload{
		OrderID:         req.ID,
		Trigger:         "retry",
		RetryPrintLogID: req.PrintLogID,
		TaskKey:         buildRetryOrderPrintTaskKey(req.ID, req.PrintLogID),
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_print_retry_requested",
		TargetType:  "print_log",
		TargetID:    &printLog.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_id":     result.Order.ID,
			"order_no":     result.Order.OrderNo,
			"printer_id":   printer.ID,
			"printer_name": printer.PrinterName,
			"trigger":      "retry",
		},
	})

	ctx.JSON(http.StatusOK, retryMerchantOrderPrintJobResponse{
		Message:    "print retry task scheduled",
		OrderID:    result.Order.ID,
		PrintLogID: printLog.ID,
		Trigger:    "retry",
	})
}

// recordMerchantLocalPrintEvent godoc
// @Summary 记录商户端本地打印事件
// @Description 商户 App 记录本地蓝牙小票打印的 started/success/failed 状态；服务端按商户和事件键幂等收敛，并校验订单归属
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Param request body recordMerchantLocalPrintEventBody true "本地打印事件"
// @Success 200 {object} merchantLocalPrintEventResponse "事件已记录"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "订单不存在或不属于当前商户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/local-print-events [post]
// @Security BearerAuth
func (server *Server) recordMerchantLocalPrintEvent(ctx *gin.Context) {
	var uri recordMerchantLocalPrintEventRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var body recordMerchantLocalPrintEventBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	eventKey := strings.TrimSpace(body.EventKey)
	if eventKey == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("event_key is required")))
		return
	}
	if eventKey != fmt.Sprintf("accepted-receipt:%d", uri.ID) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("event_key does not match order")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	params := db.UpsertMerchantLocalPrintEventParams{
		MerchantID: merchant.ID,
		OrderID:    uri.ID,
		EventKey:   eventKey,
		Source:     db.MerchantLocalPrintEventSourceBle,
		Status:     body.Status,
	}
	if body.PrinterName != nil {
		params.PrinterName = pgtype.Text{
			String: strings.TrimSpace(*body.PrinterName),
			Valid:  strings.TrimSpace(*body.PrinterName) != "",
		}
	}
	if body.ErrorMessage != nil {
		params.ErrorMessage = pgtype.Text{
			String: strings.TrimSpace(*body.ErrorMessage),
			Valid:  strings.TrimSpace(*body.ErrorMessage) != "",
		}
	}

	event, err := server.store.UpsertMerchantLocalPrintEvent(ctx, params)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, toMerchantLocalPrintEventResponse(event))
}

// printMerchantOrder godoc
// @Summary 手动创建订单打印任务
// @Description 在手动打印模式下，为指定订单创建一次异步打印任务
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID" minimum(1)
// @Success 200 {object} printMerchantOrderResponse "打印任务已创建"
// @Failure 400 {object} ErrorResponse "订单或打印配置不允许手动打印"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前商户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/{id}/print-jobs [post]
// @Security BearerAuth
func (server *Server) printMerchantOrder(ctx *gin.Context) {
	var req printMerchantOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	service := server.orderCommandSvc
	if service == nil {
		service = server.buildOrderCommandService()
	}

	result, err := service.PrintMerchantOrder(ctx, logic.MerchantOrderPrintInput{
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

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "merchant",
		Action:      "order_print_requested",
		TargetType:  "order",
		TargetID:    &result.Order.ID,
		RegionID:    &merchant.RegionID,
		Metadata: map[string]any{
			"order_no":   result.Order.OrderNo,
			"order_type": result.Order.OrderType,
			"trigger":    "manual",
		},
	})

	ctx.JSON(http.StatusOK, printMerchantOrderResponse{
		Message: "print task scheduled",
		OrderID: result.Order.ID,
		Trigger: "manual",
	})
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

type merchantOrderSummaryResponse struct {
	Total                int64 `json:"total"`
	PendingCount         int64 `json:"pending_count"`
	PaidCount            int64 `json:"paid_count"`
	PreparingCount       int64 `json:"preparing_count"`
	ReadyCount           int64 `json:"ready_count"`
	CourierAcceptedCount int64 `json:"courier_accepted_count"`
	PickedCount          int64 `json:"picked_count"`
	DeliveringCount      int64 `json:"delivering_count"`
	RiderDeliveredCount  int64 `json:"rider_delivered_count"`
	UserDeliveredCount   int64 `json:"user_delivered_count"`
	CompletedCount       int64 `json:"completed_count"`
	CancelledCount       int64 `json:"cancelled_count"`
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
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
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

	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	result, err := service.GetMerchantOrderStats(ctx, logic.GetMerchantOrderStatsQueryInput{
		MerchantID: merchant.ID,
		StartDate:  startDate,
		EndDate:    endDate,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	stats := result.Stats

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

// getMerchantOrderSummary godoc
// @Summary 获取商户订单汇总
// @Description 返回当前商户订单总数及各状态汇总，供工作台和筛选条使用
// @Tags 商户订单管理
// @Accept json
// @Produce json
// @Success 200 {object} merchantOrderSummaryResponse "订单汇总"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchant/orders/summary [get]
// @Security BearerAuth
func (server *Server) getMerchantOrderSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := server.requireMerchantForOrder(ctx, authPayload.UserID)
	if !ok {
		return
	}

	countStatus := func(status string) (int64, error) {
		return server.store.CountOrdersByMerchantAndStatus(ctx, db.CountOrdersByMerchantAndStatusParams{
			MerchantID: merchant.ID,
			Status:     status,
		})
	}

	pendingCount, err := countStatus(OrderStatusPending)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	paidCount, err := countStatus(OrderStatusPaid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	preparingCount, err := countStatus(OrderStatusPreparing)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	readyCount, err := countStatus(OrderStatusReady)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	courierAcceptedCount, err := countStatus(OrderStatusCourierAccepted)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	pickedCount, err := countStatus(OrderStatusPicked)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	deliveringCount, err := countStatus(OrderStatusDelivering)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	riderDeliveredCount, err := countStatus(OrderStatusRiderDelivered)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	userDeliveredCount, err := countStatus(OrderStatusUserDelivered)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	completedCount, err := countStatus(OrderStatusCompleted)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	cancelledCount, err := countStatus(OrderStatusCancelled)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total := pendingCount + paidCount + preparingCount + readyCount + courierAcceptedCount + pickedCount + deliveringCount + riderDeliveredCount + userDeliveredCount + completedCount + cancelledCount

	ctx.JSON(http.StatusOK, merchantOrderSummaryResponse{
		Total:                total,
		PendingCount:         pendingCount,
		PaidCount:            paidCount,
		PreparingCount:       preparingCount,
		ReadyCount:           readyCount,
		CourierAcceptedCount: courierAcceptedCount,
		PickedCount:          pickedCount,
		DeliveringCount:      deliveringCount,
		RiderDeliveredCount:  riderDeliveredCount,
		UserDeliveredCount:   userDeliveredCount,
		CompletedCount:       completedCount,
		CancelledCount:       cancelledCount,
	})
}

// orderCalculationResponse 订单金额计算响应
type orderCalculationResponse struct {
	// 商品小计 (单位：分)
	Subtotal int64 `json:"subtotal" example:"5760"`
	// 代取费 (单位：分)
	DeliveryFee int64 `json:"delivery_fee" example:"500"`
	// 代取费优惠 (单位：分)
	DeliveryFeeDiscount int64 `json:"delivery_fee_discount" example:"200"`
	// 营销优惠总减免（分），包含商户优惠和优惠券抵扣
	DiscountAmount int64 `json:"discount_amount" example:"500"`
	// 最终应付金额 (单位：分)
	TotalAmount int64 `json:"total_amount" example:"5560"`
	// 优惠明细
	Promotions []orderPromotion `json:"promotions,omitempty"`
	// 推荐可用优惠券（仅试算，不自动使用）
	SuggestedVoucher *logic.SuggestedVoucher `json:"suggested_voucher,omitempty"`
	// 阶梯优惠试算信息
	LadderPromotions []logic.LadderPromotion `json:"ladder_promotions,omitempty"`
	// 代金券试算信息
	VoucherTrials []logic.VoucherTrial `json:"voucher_trials,omitempty"`
	// 会员余额支付能力评估
	PaymentAssessment logic.PaymentAssessment `json:"payment_assessment"`
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
// @Description - 代取费（外卖订单，基于实时位置或代取地址）
// @Description - 商户营销优惠与优惠券抵扣
// @Description - 满返运费优惠
// @Description - 推荐优惠券、阶梯优惠和代金券试算
// @Description - 会员余额支付能力评估
// @Description
// @Description **代取费计算方式：**
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
// @Param address_id query int64 false "代取地址ID（下单阶段使用）"
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
	service := server.orderQuerySvc
	if service == nil {
		service = server.buildOrderQueryService()
	}

	result, err := service.CalculateOrderPreview(ctx, logic.CalculateOrderPreviewInput{
		OrderCalculationInput: logic.OrderCalculationInput{
			UserID:        authPayload.UserID,
			MerchantID:    req.MerchantID,
			OrderType:     req.OrderType,
			Latitude:      req.Latitude,
			Longitude:     req.Longitude,
			AddressID:     req.AddressID,
			UserVoucherID: req.UserVoucherID,
			VoucherCode:   req.VoucherCode,
		},
		MapClient: server.mapClient,
		Normalize: func(_ context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
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
		CalculateFee: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (logic.DeliveryFeeComputation, error) {
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, merchantID, distance, orderAmount)
			if err != nil {
				return logic.DeliveryFeeComputation{}, err
			}
			return logic.DeliveryFeeComputation{
				Fee:      feeResult.FinalFee,
				Discount: feeResult.PromotionDiscount,
			}, nil
		},
	})
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
		SuggestedVoucher:    result.SuggestedVoucher,
		LadderPromotions:    result.LadderPromotions,
		VoucherTrials:       result.VoucherTrials,
		PaymentAssessment:   result.PaymentAssessment,
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
