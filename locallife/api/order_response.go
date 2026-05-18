package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/rs/zerolog/log"
)

const merchantOrderFeeBreakdownUnavailableMessage = "订单费用明细暂不可用，请稍后重试"

// ==================== pgtype → 指针 转换 helpers ====================
// 消除 order response 构建中的重复样板代码。

func pgtypeInt8Ptr(v pgtype.Int8) *int64 {
	if v.Valid {
		return &v.Int64
	}
	return nil
}

func pgtypeInt4Ptr(v pgtype.Int4) *int32 {
	if v.Valid {
		return &v.Int32
	}
	return nil
}

func pgtypeTextPtr(v pgtype.Text) *string {
	if v.Valid {
		return &v.String
	}
	return nil
}

func pgtypeTimestamptzPtr(v pgtype.Timestamptz) *time.Time {
	if v.Valid {
		return &v.Time
	}
	return nil
}

type merchantOrderFeeBreakdownResponse struct {
	FoodAmount                int64 `json:"food_amount" example:"10000"`
	MerchantDiscountAmount    int64 `json:"merchant_discount_amount" example:"300"`
	VoucherDiscountAmount     int64 `json:"voucher_discount_amount" example:"200"`
	FoodPayableAmount         int64 `json:"food_payable_amount" example:"9500"`
	DeliveryFeeAmount         int64 `json:"delivery_fee_amount" example:"800"`
	DeliveryFeeDiscountAmount int64 `json:"delivery_fee_discount_amount" example:"0"`
	DeliveryPayableAmount     int64 `json:"delivery_payable_amount" example:"800"`
	CustomerPayableAmount     int64 `json:"customer_payable_amount" example:"10300"`
	PlatformServiceFeeAmount  int64 `json:"platform_service_fee_amount" example:"475"`
	PaymentChannelFeeAmount   int64 `json:"payment_channel_fee_amount" example:"57"`
	MerchantReceivableAmount  int64 `json:"merchant_receivable_amount" example:"8968"`
}

func newMerchantOrderFeeBreakdownResponse(b logic.MerchantOrderFeeBreakdown) *merchantOrderFeeBreakdownResponse {
	return &merchantOrderFeeBreakdownResponse{
		FoodAmount:                b.FoodAmount,
		MerchantDiscountAmount:    b.MerchantDiscountAmount,
		VoucherDiscountAmount:     b.VoucherDiscountAmount,
		FoodPayableAmount:         b.FoodPayableAmount,
		DeliveryFeeAmount:         b.DeliveryFeeAmount,
		DeliveryFeeDiscountAmount: b.DeliveryFeeDiscountAmount,
		DeliveryPayableAmount:     b.DeliveryPayableAmount,
		CustomerPayableAmount:     b.CustomerPayableAmount,
		PlatformServiceFeeAmount:  b.PlatformServiceFeeAmount,
		PaymentChannelFeeAmount:   b.PaymentChannelFeeAmount,
		MerchantReceivableAmount:  b.MerchantReceivableAmount,
	}
}

func (server *Server) loadMerchantOrderFeeBreakdowns(ctx context.Context, merchantID int64, orders []db.Order) (map[int64]logic.MerchantOrderFeeBreakdown, error) {
	if len(orders) == 0 {
		return map[int64]logic.MerchantOrderFeeBreakdown{}, nil
	}

	orderIDs := make([]int64, 0, len(orders))
	ordersByID := make(map[int64]db.Order, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		ordersByID[order.ID] = order
	}

	rows, err := server.store.ListProfitSharingOrdersByOrderIDsForMerchant(ctx, db.ListProfitSharingOrdersByOrderIDsForMerchantParams{
		MerchantID: merchantID,
		OrderIds:   orderIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list merchant order fee breakdown profit sharing orders: %w", err)
	}

	breakdowns := make(map[int64]logic.MerchantOrderFeeBreakdown, len(orders))
	for _, row := range rows {
		if _, exists := breakdowns[row.OrderID]; exists {
			continue
		}
		order, ok := ordersByID[row.OrderID]
		if !ok {
			continue
		}
		profitSharingOrder := row.ProfitSharingOrder
		breakdown, err := logic.BuildMerchantOrderFeeBreakdown(logic.BuildMerchantOrderFeeBreakdownInput{
			Order:              order,
			ProfitSharingOrder: &profitSharingOrder,
		})
		if err != nil {
			return nil, fmt.Errorf("build merchant order fee breakdown: %w", err)
		}
		breakdowns[row.OrderID] = breakdown
	}

	for _, order := range orders {
		if _, ok := breakdowns[order.ID]; !ok {
			return nil, fmt.Errorf("%w: order_id=%d merchant_id=%d", logic.ErrMerchantFeeBreakdownUnavailable, order.ID, merchantID)
		}
	}
	return breakdowns, nil
}

func writeMerchantOrderFeeBreakdownError(ctx *gin.Context, merchantID int64, orders []db.Order, err error) {
	orderIDs := make([]int64, 0, len(orders))
	statuses := make([]string, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		statuses = append(statuses, order.Status)
	}

	evt := log.Error().
		Err(err).
		Int64("merchant_id", merchantID).
		Ints64("order_ids", orderIDs).
		Strs("statuses", statuses)
	if errors.Is(err, logic.ErrMerchantFeeBreakdownUnavailable) || errors.Is(err, logic.ErrMerchantFeeBreakdownInconsistent) {
		evt.Msg("merchant order fee breakdown unavailable")
	} else {
		evt.Msg("merchant order fee breakdown query failed")
	}
	_ = ctx.Error(err)
	ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: merchantOrderFeeBreakdownUnavailableMessage})
}

// orderNullableFields 封装所有订单可选字段的 pgtype 值，
// 用于统一从 db.Order / GetOrderWithDetailsRow / ListOrdersByUserWithFiltersRow 构建 response。
type orderNullableFields struct {
	AddressID           pgtype.Int8
	DeliveryDistance    pgtype.Int4
	TableID             pgtype.Int8
	ReservationID       pgtype.Int8
	PaymentMethod       pgtype.Text
	Notes               pgtype.Text
	PaidAt              pgtype.Timestamptz
	CompletedAt         pgtype.Timestamptz
	CancelledAt         pgtype.Timestamptz
	CancelReason        pgtype.Text
	ReplacedByOrderID   pgtype.Int8
	UpdatedAt           pgtype.Timestamptz
	PickupCode          pgtype.Text
	DispatchOrderID     pgtype.Int8
	FlowID              pgtype.Int8
	StatusHint          pgtype.Text
	ExceptionState      pgtype.Text
	ClaimChannel        pgtype.Text
	PrepStartAt         pgtype.Timestamptz
	ReadyAt             pgtype.Timestamptz
	CourierAcceptAt     pgtype.Timestamptz
	PickedAt            pgtype.Timestamptz
	RiderDeliveredAt    pgtype.Timestamptz
	UserDeliveredAt     pgtype.Timestamptz
	AutoUserDeliveredAt pgtype.Timestamptz
	DeliveryDuration    pgtype.Int4
	Badges              []byte
}

// applyTo 将所有可选字段从 pgtype 转换并写入 orderResponse。
func (f orderNullableFields) applyTo(resp *orderResponse) error {
	resp.AddressID = pgtypeInt8Ptr(f.AddressID)
	resp.DeliveryDistance = pgtypeInt4Ptr(f.DeliveryDistance)
	resp.TableID = pgtypeInt8Ptr(f.TableID)
	resp.ReservationID = pgtypeInt8Ptr(f.ReservationID)
	resp.PaymentMethod = pgtypeTextPtr(f.PaymentMethod)
	resp.Notes = pgtypeTextPtr(f.Notes)
	resp.PaidAt = pgtypeTimestamptzPtr(f.PaidAt)
	resp.CompletedAt = pgtypeTimestamptzPtr(f.CompletedAt)
	resp.CancelledAt = pgtypeTimestamptzPtr(f.CancelledAt)
	resp.CancelReason = pgtypeTextPtr(f.CancelReason)
	resp.ReplacedByOrderID = pgtypeInt8Ptr(f.ReplacedByOrderID)
	resp.UpdatedAt = pgtypeTimestamptzPtr(f.UpdatedAt)
	resp.DispatchOrderID = pgtypeInt8Ptr(f.DispatchOrderID)
	resp.FlowID = pgtypeInt8Ptr(f.FlowID)
	resp.StatusHint = pgtypeTextPtr(f.StatusHint)
	resp.ExceptionState = pgtypeTextPtr(f.ExceptionState)
	resp.ClaimChannel = pgtypeTextPtr(f.ClaimChannel)
	resp.PrepStartAt = pgtypeTimestamptzPtr(f.PrepStartAt)
	resp.ReadyAt = pgtypeTimestamptzPtr(f.ReadyAt)
	resp.CourierAcceptAt = pgtypeTimestamptzPtr(f.CourierAcceptAt)
	resp.PickedAt = pgtypeTimestamptzPtr(f.PickedAt)
	resp.RiderDeliveredAt = pgtypeTimestamptzPtr(f.RiderDeliveredAt)
	resp.UserDeliveredAt = pgtypeTimestamptzPtr(f.UserDeliveredAt)
	resp.AutoUserDeliveredAt = pgtypeTimestamptzPtr(f.AutoUserDeliveredAt)

	// PickupCode 有特殊处理：需要同时生成 masked 版本
	if f.PickupCode.Valid {
		resp.PickupCode = &f.PickupCode.String
		resp.PickupCodeMasked = maskPickupCode(f.PickupCode.String)
	}

	// Badges
	if len(f.Badges) > 0 {
		badges, err := decodeOrderBadges(f.Badges)
		if err != nil {
			return err
		}
		resp.Badges = badges
	}

	// DeliveryEtaMinutes 从 DeliveryDuration 推算
	if f.DeliveryDuration.Valid && f.DeliveryDuration.Int32 > 0 {
		etaMinutes := f.DeliveryDuration.Int32 / 60
		resp.DeliveryEtaMinutes = &etaMinutes
	}

	return nil
}

func (server *Server) newOrderItemResponses(ctx *gin.Context, views []logic.OrderItemView, includeImages bool) []orderItemResponse {
	responses := make([]orderItemResponse, len(views))
	for index, view := range views {
		customizations := make([]orderCustomizationItem, len(view.Customizations))
		for customizationIndex, customization := range view.Customizations {
			customizations[customizationIndex] = orderCustomizationItem{
				GroupID:    customization.GroupID,
				OptionID:   customization.OptionID,
				TagID:      customization.TagID,
				Name:       customization.Name,
				Value:      customization.Value,
				ExtraPrice: customization.ExtraPrice,
			}
		}

		responses[index] = orderItemResponse{
			ID:             view.ID,
			DishID:         view.DishID,
			ComboID:        view.ComboID,
			Name:           view.Name,
			UnitPrice:      view.UnitPrice,
			Quantity:       view.Quantity,
			Subtotal:       view.Subtotal,
			SpecsText:      view.SpecsText,
			Customizations: customizations,
			ImageAssetID:   view.ImageAssetID,
		}
		if includeImages && view.ImageAssetID != nil {
			responses[index].ImageURL = server.publicImageURL(ctx, view.ImageAssetID, media.VariantCard)
		}
	}
	return responses
}
