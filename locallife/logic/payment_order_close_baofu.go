package logic

import (
	"context"
	"errors"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

func (svc *PaymentOrderService) closeBaofuAggregatePaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.baofuPaymentService == nil {
		return ClosePaymentOrderResult{}, ErrBaofuPaymentServiceNotConfigured
	}
	businessOwner := paymentOrder.BusinessType
	if paymentOrder.BusinessType == reservationAddonBusiness {
		businessOwner = db.ExternalPaymentBusinessOwnerReservation
	}
	if _, err := svc.baofuPaymentService.CloseOrder(ctx, CloseBaofuOrderInput{
		PaymentOrder:  paymentOrder,
		BusinessOwner: businessOwner,
	}); err != nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("close baofu payment order: %w", err)
	}
	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	if paymentOrder.BusinessType == reservationAddonBusiness {
		if _, err := svc.store.CloseReservationAdjustmentForPaymentTx(ctx, db.CloseReservationAdjustmentForPaymentTxParams{
			PaymentOrderID: paymentOrder.ID,
			Status:         db.ReservationAdjustmentStatusClosed,
			Reason:         "user closed payment order",
		}); err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return ClosePaymentOrderResult{}, fmt.Errorf("close reservation adjustment after payment close: %w", err)
		}
	}
	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}
