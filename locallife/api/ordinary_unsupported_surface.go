package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const ordinaryServiceProviderUnsupportedFundsMessage = "普通服务商模式不支持在平台内查询余额、发起提现、注销提现、补差或垫付回补，请前往微信支付商户平台/商家助手处理资金操作；本平台仅支持结算账户查询和修改"
const ordinaryServiceProviderUnsupportedReceiverLifecycleMessage = "普通服务商模式不支持平台收付通分账接收方预热或修复；普通服务商分账接收方会按支付单和子商户号自动同步，如支付、退款或分账被限制，请联系平台管理员查看普通服务商商户管控诊断"

func (server *Server) gateEcommerceFundManagementWhenOrdinaryActive(operation string, _ gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := errors.New("platform ecommerce fund-management capability is disabled after ordinary service provider migration")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryServiceProviderUnsupportedFundsMessage, operation+" disabled under ordinary service provider migration"))
	}
}

func (server *Server) gateEcommerceReceiverLifecycleWhenOrdinaryActive(operation string, _ gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := errors.New("platform ecommerce profit-sharing receiver lifecycle is disabled after ordinary service provider migration")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryServiceProviderUnsupportedReceiverLifecycleMessage, operation+" disabled under ordinary service provider migration"))
	}
}
