package api

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

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

// orderNullableFields 封装所有订单可选字段的 pgtype 值，
// 用于统一从 db.Order / GetOrderWithDetailsRow / ListOrdersByUserWithFiltersRow 构建 response。
type orderNullableFields struct {
	AddressID           pgtype.Int8
	DeliveryDistance     pgtype.Int4
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
	CourierAcceptAt      pgtype.Timestamptz
	PickedAt            pgtype.Timestamptz
	RiderDeliveredAt    pgtype.Timestamptz
	UserDeliveredAt     pgtype.Timestamptz
	AutoUserDeliveredAt pgtype.Timestamptz
	DeliveryDuration    pgtype.Int4
	Badges              []byte
}

// applyTo 将所有可选字段从 pgtype 转换并写入 orderResponse。
func (f orderNullableFields) applyTo(resp *orderResponse) {
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
		resp.Badges = decodeOrderBadges(f.Badges)
	}

	// DeliveryEtaMinutes 从 DeliveryDuration 推算
	if f.DeliveryDuration.Valid && f.DeliveryDuration.Int32 > 0 {
		etaMinutes := f.DeliveryDuration.Int32 / 60
		resp.DeliveryEtaMinutes = &etaMinutes
	}
}
