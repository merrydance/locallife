package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// ResolveCompletedOrderBaofuProfitSharingOrder returns the existing Baofu share
// bill that may be enqueued after an order reaches completed.
func ResolveCompletedOrderBaofuProfitSharingOrder(ctx context.Context, store db.Store, order db.Order) (db.ProfitSharingOrder, error) {
	if order.ID <= 0 {
		return db.ProfitSharingOrder{}, nil
	}
	if order.Status != db.OrderStatusCompleted {
		return db.ProfitSharingOrder{}, nil
	}

	paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ProfitSharingOrder{}, nil
		}
		return db.ProfitSharingOrder{}, fmt.Errorf("get latest payment order for baofu profit sharing: %w", err)
	}
	if !db.PaymentOrderRequiresProfitSharing(paymentOrder) {
		return db.ProfitSharingOrder{}, nil
	}
	if paymentOrder.Status != paymentStatusPaid {
		return db.ProfitSharingOrder{}, nil
	}
	if !paymentOrder.OrderID.Valid || paymentOrder.OrderID.Int64 != order.ID || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return db.ProfitSharingOrder{}, fmt.Errorf("payment order %d does not match completed order %d", paymentOrder.ID, order.ID)
	}

	profitSharingOrder, err := store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ProfitSharingOrder{}, nil
		}
		return db.ProfitSharingOrder{}, fmt.Errorf("get baofu profit sharing bill: %w", err)
	}
	if err := ValidateBaofuProfitSharingOrderReadyForCommand(profitSharingOrder, paymentOrder); err != nil {
		return db.ProfitSharingOrder{}, err
	}
	if err := ValidateBaofuProfitSharingRefundSafety(ctx, store, paymentOrder.ID); err != nil {
		return db.ProfitSharingOrder{}, err
	}
	return profitSharingOrder, nil
}

func ValidateBaofuProfitSharingOrderReadyForCommand(profitSharingOrder db.ProfitSharingOrder, paymentOrder db.PaymentOrder) error {
	if profitSharingOrder.PaymentOrderID != paymentOrder.ID {
		return fmt.Errorf("profit sharing bill %d does not belong to payment order %d", profitSharingOrder.ID, paymentOrder.ID)
	}
	if profitSharingOrder.Provider != db.ExternalPaymentProviderBaofu || profitSharingOrder.Channel != db.PaymentChannelBaofuAggregate {
		return fmt.Errorf("profit sharing bill %d is not baofu aggregate", profitSharingOrder.ID)
	}
	if profitSharingOrder.Status != db.ProfitSharingOrderStatusPending && profitSharingOrder.Status != db.ProfitSharingOrderStatusFailed {
		return fmt.Errorf("profit sharing bill %d status %q cannot be triggered", profitSharingOrder.ID, profitSharingOrder.Status)
	}
	if profitSharingOrder.OrderSource == db.OrderTypeTakeout && profitSharingOrder.DeliveryFee > 0 {
		if !profitSharingOrder.RiderID.Valid ||
			strings.TrimSpace(profitSharingOrder.RiderSharingMerID.String) == "" ||
			profitSharingOrder.RiderGrossAmount <= 0 ||
			profitSharingOrder.RiderAmount < 0 {
			return fmt.Errorf("takeout profit sharing bill %d rider portion is incomplete", profitSharingOrder.ID)
		}
	}
	return nil
}

func ValidateBaofuProfitSharingRefundSafety(ctx context.Context, store db.Store, paymentOrderID int64) error {
	refundedAmount, err := store.GetTotalRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return fmt.Errorf("get refunded amount before baofu profit sharing: %w", err)
	}
	if refundedAmount > 0 {
		return fmt.Errorf("payment order %d has active refund amount %d before baofu profit sharing", paymentOrderID, refundedAmount)
	}
	return nil
}
