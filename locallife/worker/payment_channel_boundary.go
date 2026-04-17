package worker

import (
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

const paymentTypeEcommerce = "profit_sharing"

func paymentOrderUsesEcommerceChannel(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderUsesEcommerceChannel(paymentOrder)
}

func paymentOrderRequiresProfitSharing(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderRequiresProfitSharing(paymentOrder)
}

func refundTypeForPaymentOrder(paymentOrder db.PaymentOrder) string {
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		return paymentTypeEcommerce
	}
	if paymentOrder.PaymentType == "native" {
		return "miniprogram"
	}
	return paymentOrder.PaymentType
}

func requiresEcommerceRefund(paymentOrder db.PaymentOrder) bool {
	switch paymentOrder.BusinessType {
	case "order", "reservation", "reservation_addon":
		return true
	}
	return false
}

func mainBusinessRefundChannelDriftError(paymentOrder db.PaymentOrder, action string) error {
	return fmt.Errorf("main-business payment order %d with payment_channel %s and payment_type %s cannot %s outside ecommerce channel", paymentOrder.ID, paymentOrder.PaymentChannel, paymentOrder.PaymentType, action)
}
