package wechatruntime

import (
	"context"
	"fmt"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func CreateDirectRefundContract(ctx context.Context, paymentClient wechat.DirectPaymentClientInterface, req *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	if paymentClient == nil {
		return nil, fmt.Errorf("payment client not configured")
	}
	if err := wechatcontracts.ValidateDirectRefundRequest(req); err != nil {
		return nil, err
	}

	runtimeReq := &wechat.RefundRequest{
		TransactionID: req.TransactionID,
		OutTradeNo:    req.OutTradeNo,
		OutRefundNo:   req.OutRefundNo,
		Reason:        req.Reason,
		NotifyURL:     req.NotifyURL,
		FundsAccount:  req.FundsAccount,
		GoodsDetail:   req.GoodsDetail,
	}
	if req.Amount != nil {
		runtimeReq.RefundAmount = req.Amount.Refund
		runtimeReq.TotalAmount = req.Amount.Total
		runtimeReq.AmountFrom = req.Amount.From
	}

	resp, err := paymentClient.CreateRefund(ctx, runtimeReq)
	if err != nil {
		return nil, err
	}

	return toDirectRefundContractResponse(resp), nil
}

func toDirectRefundContractResponse(resp *wechat.RefundResponse) *wechatcontracts.DirectRefundResponse {
	if resp == nil {
		return nil
	}

	return &wechatcontracts.DirectRefundResponse{
		RefundID:            resp.RefundID,
		OutRefundNo:         resp.OutRefundNo,
		TransactionID:       resp.TransactionID,
		OutTradeNo:          resp.OutTradeNo,
		Channel:             resp.Channel,
		UserReceivedAccount: resp.UserReceivedAccount,
		SuccessTime:         resp.SuccessTime,
		CreateTime:          resp.CreateTime,
		Status:              resp.Status,
		FundsAccount:        resp.FundsAccount,
		Amount:              resp.Amount,
		PromotionDetail:     resp.PromotionDetail,
	}
}
