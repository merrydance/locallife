package logic

import (
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

func paymentOrderUsesBaofuAggregateChannel(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate
}

func mainBusinessBaofuOnlyError(action string) error {
	return NewRequestErrorWithCause(http.StatusBadRequest, errors.New("当前主营业务支付单仅支持宝付链路，无法"+action+"，请联系平台处理"), nil)
}

func refundTypeForPaymentOrder(paymentOrder db.PaymentOrder) string {
	if paymentOrder.PaymentType == paymentTypeNative {
		return paymentTypeMiniProgram
	}
	return paymentOrder.PaymentType
}
