package logic

import (
	"errors"
	"fmt"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

func mainBusinessEcommerceOnlyError(action string) error {
	return NewRequestError(http.StatusConflict, errors.New("当前主营业务支付单不属于微信服务商链路，无法"+action+"，请联系平台处理"))
}

func paymentOrderUsesEcommerceChannel(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderUsesEcommerceChannel(paymentOrder)
}

func ensureWechatServiceProviderRefundClientConfigured(paymentOrder db.PaymentOrder, ecommerceClient wechat.EcommerceClientInterface, ordinaryClient ordinaryServiceProviderOrderClient, action string) error {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if ordinaryClient == nil {
			return fmt.Errorf("ordinary service provider client not configured for %s; contact platform to complete merchant payment capability configuration", action)
		}
		return nil
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		if ecommerceClient == nil {
			return fmt.Errorf("ecommerce client not configured for %s; contact platform to complete merchant payment capability configuration", action)
		}
		return nil
	}
	return mainBusinessEcommerceOnlyError(action)
}

func refundTypeForPaymentOrder(paymentOrder db.PaymentOrder) string {
	if paymentOrderUsesEcommerceChannel(paymentOrder) || db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return paymentTypeProfitSharing
	}
	if paymentOrder.PaymentType == paymentTypeNative {
		return paymentTypeMiniProgram
	}
	return paymentOrder.PaymentType
}
