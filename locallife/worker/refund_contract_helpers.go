package worker

import (
	"context"
	"fmt"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

func createDirectRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	if ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
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
		AmountFrom:    req.Amount.From,
		GoodsDetail:   req.GoodsDetail,
	}
	if req.Amount != nil {
		runtimeReq.RefundAmount = req.Amount.Refund
		runtimeReq.TotalAmount = req.Amount.Total
	}

	resp, err := ecommerceClient.CreateRefund(ctx, runtimeReq)
	if err != nil {
		return nil, err
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
	}, nil
}

func createEcommerceRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
	if ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}

	runtimeReq := &wechat.EcommerceRefundRequest{
		SubMchID:      req.SubMchID,
		SubAppID:      req.SubAppID,
		TransactionID: req.TransactionID,
		OutTradeNo:    req.OutTradeNo,
		OutRefundNo:   req.OutRefundNo,
		Reason:        req.Reason,
		NotifyURL:     req.NotifyURL,
		RefundAccount: req.RefundAccount,
		FundsAccount:  req.FundsAccount,
	}
	if req.Amount != nil {
		runtimeReq.RefundAmount = req.Amount.Refund
		runtimeReq.TotalAmount = req.Amount.Total
	}

	resp, err := ecommerceClient.CreateEcommerceRefund(ctx, runtimeReq)
	if err != nil {
		return nil, err
	}

	return &wechatcontracts.EcommerceRefundCreateResponse{
		RefundID:        resp.RefundID,
		OutRefundNo:     resp.OutRefundNo,
		CreateTime:      resp.CreateTime,
		Amount:          toEcommerceRefundAmountContract(resp.Amount),
		PromotionDetail: toEcommerceRefundPromotionDetailsContract(resp.PromotionDetail),
		RefundAccount:   resp.RefundAccount,
	}, nil
}

func toEcommerceRefundAmountContract(amount wechat.EcommerceRefundAmount) wechatcontracts.EcommerceRefundAmount {
	return wechatcontracts.EcommerceRefundAmount{
		Refund:         amount.Refund,
		From:           toEcommerceRefundAmountFromContract(amount.From),
		PayerRefund:    amount.PayerRefund,
		DiscountRefund: amount.DiscountRefund,
		Currency:       amount.Currency,
		Advance:        amount.Advance,
	}
}

func toEcommerceRefundAmountFromContract(entries []wechat.EcommerceRefundAmountFrom) []wechatcontracts.EcommerceRefundAmountFrom {
	if len(entries) == 0 {
		return nil
	}
	result := make([]wechatcontracts.EcommerceRefundAmountFrom, 0, len(entries))
	for _, entry := range entries {
		result = append(result, wechatcontracts.EcommerceRefundAmountFrom{
			Account: entry.Account,
			Amount:  entry.Amount,
		})
	}
	return result
}

func toEcommerceRefundPromotionDetailsContract(details []wechat.EcommerceRefundPromotionDetail) []wechatcontracts.EcommerceRefundPromotionDetail {
	if len(details) == 0 {
		return nil
	}
	result := make([]wechatcontracts.EcommerceRefundPromotionDetail, 0, len(details))
	for _, detail := range details {
		result = append(result, wechatcontracts.EcommerceRefundPromotionDetail{
			PromotionID:  detail.PromotionID,
			Scope:        detail.Scope,
			Type:         detail.Type,
			Amount:       detail.Amount,
			RefundAmount: detail.RefundAmount,
		})
	}
	return result
}

func logRefundRequestFailure(refundOrderID, paymentOrderID int64, outRefundNo, refundChannel string, err error) {
	log.Error().
		Err(err).
		Int64("refund_order_id", refundOrderID).
		Int64("payment_order_id", paymentOrderID).
		Str("out_refund_no", outRefundNo).
		Str("refund_channel", refundChannel).
		Msg("wechat refund request failed")
}
