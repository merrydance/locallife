package worker

import (
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

const paymentTypeProfitSharing = "profit_sharing"

func refundTypeForPaymentOrder(paymentOrder db.PaymentOrder) string {
	if paymentOrder.RequiresProfitSharing {
		return paymentTypeProfitSharing
	}
	if paymentOrder.PaymentType == "native" {
		return "miniprogram"
	}
	return paymentOrder.PaymentType
}

func mainBusinessRefundChannelDriftError(paymentOrder db.PaymentOrder, action string) error {
	return fmt.Errorf("main-business payment order %d with payment_channel %s and payment_type %s cannot %s outside baofu channel", paymentOrder.ID, paymentOrder.PaymentChannel, paymentOrder.PaymentType, action)
}
