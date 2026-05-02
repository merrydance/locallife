package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

const (
	profitSharingServiceUnavailableMessage           = "分账服务暂不可用，请稍后重试或联系平台管理员检查微信支付配置"
	queryProfitSharingAmountsUnavailableMessage      = "微信分账金额查询暂不可用，请稍后重试；如持续失败请联系平台管理员排查微信请求记录"
	deleteProfitSharingReceiverServiceUnavailable    = "分账接收方删除服务暂不可用，请稍后重试或联系平台管理员检查微信支付配置"
	deleteProfitSharingReceiverUnavailableMessage    = "微信分账接收方删除失败，请稍后重试；如持续失败请联系平台管理员排查微信请求记录"
	deleteProfitSharingReceiverInvalidRequestMessage = "分账接收方信息格式无效，请填写接收方类型和账号后重试"
	deleteProfitSharingReceiverValidationMessage     = "分账接收方信息不符合微信要求，请核对接收方类型、账号和 AppID 后重试"
	profitSharingPaymentOrderNotFoundMessage         = "未找到支付单，请刷新后重试"
	profitSharingUnsupportedPaymentOrderMessage      = "该支付单不是可分账订单，无法查询或操作微信分账"
	profitSharingMissingTransactionIDMessage         = "该支付单尚未生成微信支付交易号，请等待支付完成后重试"
	profitSharingOrderNotFoundMessage                = "未找到该支付单的分账记录，请确认订单已完成分账初始化"
	profitSharingMerchantPaymentConfigMissingMessage = "未找到商户微信支付配置，请联系平台管理员补齐普通服务商特约商户配置"
)

func paymentOrderSupportsProfitSharingCapability(paymentOrder db.PaymentOrder) bool {
	return (db.PaymentOrderUsesEcommerceChannel(paymentOrder) || db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder)) && db.PaymentOrderRequiresProfitSharing(paymentOrder)
}

type profitSharingAmountsResponse struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	TransactionID  string `json:"transaction_id"`
	UnsplitAmount  int64  `json:"unsplit_amount"`
}

// getProfitSharingAmounts 查询订单剩余待分账金额。
// @Summary 查询待分账金额
// @Description 运营商按支付单查询微信侧剩余待分账金额，适用于普通服务商分账订单；平台收付通仅作为历史冷备订单能力保留
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param id path int true "支付单ID"
// @Security BearerAuth
// @Success 200 {object} profitSharingAmountsResponse "待分账金额"
// @Failure 400 {object} ErrorResponse "请求参数错误或支付单类型不支持"
// @Failure 404 {object} ErrorResponse "支付单不存在"
// @Failure 502 {object} ErrorResponse "微信支付接口不可用"
// @Router /v1/operators/me/payment-orders/{id}/profit-sharing/amounts [get]
func (server *Server) getProfitSharingAmounts(ctx *gin.Context) {
	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		respondProfitSharingCapabilityClientError(ctx, "query_profit_sharing_amounts_bind_payment_order_id", 0, err, "支付单编号无效，请刷新页面后重试")
		return
	}

	paymentOrder, err := server.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New(profitSharingPaymentOrderNotFoundMessage)))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if !paymentOrderSupportsProfitSharingCapability(paymentOrder) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingUnsupportedPaymentOrderMessage)))
		return
	}

	if !paymentOrder.TransactionID.Valid || paymentOrder.TransactionID.String == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingMissingTransactionIDMessage)))
		return
	}

	transactionID := paymentOrder.TransactionID.String
	unsplitAmount := int64(0)
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if server.ordinarySPClient == nil {
			err := errors.New("ordinary service provider payment service not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, profitSharingServiceUnavailableMessage, "query ordinary profit sharing amounts payment service not configured"))
			return
		}
		profitSharingOrder, orderErr := server.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
		if orderErr != nil {
			if isNotFoundError(orderErr) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingOrderNotFoundMessage)))
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, orderErr))
			}
			return
		}
		paymentConfig, configErr := server.store.GetMerchantPaymentConfig(ctx, profitSharingOrder.MerchantID)
		if configErr != nil {
			if isNotFoundError(configErr) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingMerchantPaymentConfigMissingMessage)))
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, configErr))
			}
			return
		}
		resp, queryErr := server.ordinarySPClient.QueryProfitSharingRemainingAmount(ctx, ospcontracts.ProfitSharingRemainingAmountRequest{SubMchID: paymentConfig.SubMchID, TransactionID: transactionID})
		if queryErr != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, queryErr, queryProfitSharingAmountsUnavailableMessage, "query ordinary profit sharing amounts api failed"))
			return
		}
		if resp == nil {
			err := errors.New("query ordinary profit sharing amounts returned nil response")
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, queryProfitSharingAmountsUnavailableMessage, "query ordinary profit sharing amounts returned nil response"))
			return
		}
		unsplitAmount = resp.Amount
	} else {
		if server.ecommerceClient == nil {
			err := errors.New("ecommerce payment service not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, profitSharingServiceUnavailableMessage, "query ecommerce profit sharing amounts payment service not configured"))
			return
		}
		resp, queryErr := server.ecommerceClient.QueryProfitSharingAmounts(ctx, transactionID)
		if queryErr != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, queryErr, queryProfitSharingAmountsUnavailableMessage, "query profit sharing amounts api failed"))
			return
		}
		if resp == nil {
			err := errors.New("query profit sharing amounts returned nil response")
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, queryProfitSharingAmountsUnavailableMessage, "query profit sharing amounts returned nil response"))
			return
		}
		transactionID = resp.TransactionID
		unsplitAmount = resp.UnsplitAmount
	}

	ctx.JSON(http.StatusOK, profitSharingAmountsResponse{
		PaymentOrderID: paymentOrderID,
		TransactionID:  transactionID,
		UnsplitAmount:  unsplitAmount,
	})
}

type deleteProfitSharingReceiverRequest struct {
	AppID   string `json:"appid"`
	Type    string `json:"type" binding:"required"`
	Account string `json:"account" binding:"required"`
}

type deleteProfitSharingReceiverResponse struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	Type           string `json:"type"`
	Account        string `json:"account"`
}

// deleteProfitSharingReceiver 删除分账接收方关系。
// @Summary 删除分账接收方关系
// @Description 运营商按支付单上下文删除微信分账接收方关系，适用于普通服务商分账订单；平台收付通仅作为历史冷备订单能力保留
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param id path int true "支付单ID"
// @Param request body deleteProfitSharingReceiverRequest true "删除请求"
// @Security BearerAuth
// @Success 200 {object} deleteProfitSharingReceiverResponse "删除结果"
// @Failure 400 {object} ErrorResponse "请求参数错误或支付单类型不支持"
// @Failure 404 {object} ErrorResponse "支付单不存在"
// @Failure 502 {object} ErrorResponse "微信支付接口不可用"
// @Router /v1/operators/me/payment-orders/{id}/profit-sharing/receivers/delete [post]
func (server *Server) deleteProfitSharingReceiver(ctx *gin.Context) {
	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		respondProfitSharingCapabilityClientError(ctx, "delete_profit_sharing_receiver_bind_payment_order_id", 0, err, "支付单编号无效，请刷新页面后重试")
		return
	}

	paymentOrder, err := server.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New(profitSharingPaymentOrderNotFoundMessage)))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if !paymentOrderSupportsProfitSharingCapability(paymentOrder) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingUnsupportedPaymentOrderMessage)))
		return
	}

	var req deleteProfitSharingReceiverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondProfitSharingCapabilityClientError(ctx, "delete_profit_sharing_receiver_bind_request", paymentOrderID, err, deleteProfitSharingReceiverInvalidRequestMessage)
		return
	}

	responseType := ""
	responseAccount := ""
	var deleteErr error
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if server.ordinarySPClient == nil {
			err := errors.New("ordinary service provider payment service not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, deleteProfitSharingReceiverServiceUnavailable, "delete ordinary profit sharing receiver payment service not configured"))
			return
		}
		profitSharingOrder, orderErr := server.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
		if orderErr != nil {
			if isNotFoundError(orderErr) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingOrderNotFoundMessage)))
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, orderErr))
			}
			return
		}
		paymentConfig, configErr := server.store.GetMerchantPaymentConfig(ctx, profitSharingOrder.MerchantID)
		if configErr != nil {
			if isNotFoundError(configErr) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(profitSharingMerchantPaymentConfigMissingMessage)))
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, configErr))
			}
			return
		}
		resp, err := server.ordinarySPClient.DeleteProfitSharingReceiver(ctx, ospcontracts.ProfitSharingReceiverDeleteRequest{
			SubMchID: paymentConfig.SubMchID,
			AppID:    req.AppID,
			Type:     ospcontracts.ReceiverType(req.Type),
			Account:  req.Account,
		})
		deleteErr = err
		if resp != nil {
			responseType = string(resp.Type)
			responseAccount = resp.Account
		}
	} else {
		if server.ecommerceClient == nil {
			err := errors.New("ecommerce payment service not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, deleteProfitSharingReceiverServiceUnavailable, "delete ecommerce profit sharing receiver payment service not configured"))
			return
		}
		resp, err := server.ecommerceClient.DeleteProfitSharingReceiver(ctx, &wechatcontracts.DeleteReceiverRequest{
			AppID:   req.AppID,
			Type:    req.Type,
			Account: req.Account,
		})
		deleteErr = err
		if resp != nil {
			responseType = resp.Type
			responseAccount = resp.Account
		}
	}
	if deleteErr != nil {
		var validationErr *wechatcontracts.ProfitSharingValidationError
		if errors.As(deleteErr, &validationErr) {
			respondProfitSharingCapabilityClientError(ctx, "delete_profit_sharing_receiver_validate_request", paymentOrderID, validationErr, deleteProfitSharingReceiverValidationMessage)
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, deleteErr, deleteProfitSharingReceiverUnavailableMessage, "delete profit sharing receiver api failed"))
		return
	}
	if responseType == "" && responseAccount == "" {
		err := errors.New("delete profit sharing receiver returned nil response")
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, deleteProfitSharingReceiverUnavailableMessage, "delete profit sharing receiver returned nil response"))
		return
	}

	ctx.JSON(http.StatusOK, deleteProfitSharingReceiverResponse{
		PaymentOrderID: paymentOrderID,
		Type:           responseType,
		Account:        responseAccount,
	})
}

func respondProfitSharingCapabilityClientError(ctx *gin.Context, operation string, paymentOrderID int64, err error, publicMessage string) {
	_ = ctx.Error(err)
	log.Warn().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Int64("payment_order_id", paymentOrderID).
		Msg("profit sharing capability request rejected")
	ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(publicMessage)))
}
