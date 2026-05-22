package api

import (
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"

	"github.com/gin-gonic/gin"
)

type paymentCapabilitiesResponse struct {
	MainBusinessPaymentChannel        string `json:"main_business_payment_channel"`
	CombinedPaymentSupported          bool   `json:"combined_payment_supported"`
	SplitCheckoutRequired             bool   `json:"split_checkout_required"`
	CombinedPaymentUnavailableMessage string `json:"combined_payment_unavailable_message,omitempty"`
}

// getPaymentCapabilities exposes checkout-relevant payment capability switches.
// @Summary 查询支付能力
// @Description 返回当前主业务支付通道及合单支付可用性，供小程序购物车决定是否必须按商户拆单支付
// @Tags 支付管理
// @Produce json
// @Success 200 {object} paymentCapabilitiesResponse "支付能力"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /v1/payments/capabilities [get]
// @Security BearerAuth
func (server *Server) getPaymentCapabilities(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, paymentCapabilitiesResponse{
		MainBusinessPaymentChannel:        db.PaymentChannelBaofuAggregate,
		CombinedPaymentSupported:          false,
		SplitCheckoutRequired:             true,
		CombinedPaymentUnavailableMessage: "宝付暂不支持合单支付，请按商户分别下单支付",
	})
}
