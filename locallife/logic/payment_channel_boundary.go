package logic

import (
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

func mainBusinessEcommerceOnlyError(action string) error {
	return NewRequestError(http.StatusConflict, errors.New("当前主营业务支付单不属于收付通链路，无法"+action+"，请联系平台处理"))
}

func paymentOrderUsesEcommerceChannel(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderUsesEcommerceChannel(paymentOrder)
}

func paymentOrderRequiresProfitSharing(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderRequiresProfitSharing(paymentOrder)
}

func refundTypeForPaymentOrder(paymentOrder db.PaymentOrder) string {
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		return paymentTypeProfitSharing
	}
	if paymentOrder.PaymentType == paymentTypeNative {
		return paymentTypeMiniProgram
	}
	return paymentOrder.PaymentType
}
