package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func paymentOrderSupportsProfitSharingCapability(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderUsesEcommerceChannel(paymentOrder) && db.PaymentOrderRequiresProfitSharing(paymentOrder)
}

type profitSharingAmountsResponse struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	TransactionID  string `json:"transaction_id"`
	UnsplitAmount  int64  `json:"unsplit_amount"`
}

// getProfitSharingAmounts 查询订单剩余待分账金额。
// @Summary 查询待分账金额
// @Description 运营商按支付单查询微信侧剩余待分账金额，仅适用于收付通分账订单
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
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("payment service not configured")))
		return
	}

	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	paymentOrder, err := server.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if !paymentOrderSupportsProfitSharingCapability(paymentOrder) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order is not a profit sharing order")))
		return
	}

	if !paymentOrder.TransactionID.Valid || paymentOrder.TransactionID.String == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order has no transaction_id yet")))
		return
	}

	resp, err := server.ecommerceClient.QueryProfitSharingAmounts(ctx, paymentOrder.TransactionID.String)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("query profit sharing amounts api unavailable")))
		return
	}
	if resp == nil {
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("query profit sharing amounts api unavailable")))
		return
	}

	ctx.JSON(http.StatusOK, profitSharingAmountsResponse{
		PaymentOrderID: paymentOrderID,
		TransactionID:  resp.TransactionID,
		UnsplitAmount:  resp.UnsplitAmount,
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
// @Description 运营商按支付单上下文删除微信分账接收方关系，仅适用于收付通分账订单
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
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("payment service not configured")))
		return
	}

	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	paymentOrder, err := server.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if !paymentOrderSupportsProfitSharingCapability(paymentOrder) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order is not a profit sharing order")))
		return
	}

	var req deleteProfitSharingReceiverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	resp, err := server.ecommerceClient.DeleteProfitSharingReceiver(ctx, &wechatcontracts.DeleteReceiverRequest{
		AppID:   req.AppID,
		Type:    req.Type,
		Account: req.Account,
	})
	if err != nil {
		var validationErr *wechatcontracts.ProfitSharingValidationError
		if errors.As(err, &validationErr) {
			ctx.JSON(http.StatusBadRequest, errorResponse(validationErr))
			return
		}
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("delete profit sharing receiver api unavailable")))
		return
	}
	if resp == nil {
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("delete profit sharing receiver api unavailable")))
		return
	}

	ctx.JSON(http.StatusOK, deleteProfitSharingReceiverResponse{
		PaymentOrderID: paymentOrderID,
		Type:           resp.Type,
		Account:        resp.Account,
	})
}
