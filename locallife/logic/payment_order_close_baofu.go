package logic

import (
	"context"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

func (svc *PaymentOrderService) closeBaofuAggregatePaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.baofuPaymentService == nil {
		return ClosePaymentOrderResult{}, ErrBaofuPaymentServiceNotConfigured
	}
	if _, err := svc.baofuPaymentService.CloseOrder(ctx, CloseBaofuOrderInput{
		PaymentOrder:  paymentOrder,
		BusinessOwner: paymentOrder.BusinessType,
	}); err != nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("close baofu payment order: %w", err)
	}
	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}
