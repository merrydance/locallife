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

func CreateEcommerceRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
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
		runtimeReq.AmountFrom = toRuntimeEcommerceRefundAmountFrom(req.Amount.From)
	}

	resp, err := ecommerceClient.CreateEcommerceRefund(ctx, runtimeReq)
	if err != nil {
		return nil, err
	}

	return toEcommerceRefundCreateContractResponse(resp), nil
}

func ApplyEcommerceAbnormalRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceAbnormalRefundRequest) (*wechatcontracts.EcommerceRefundQueryResponse, error) {
	if ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}
	if err := wechatcontracts.ValidateEcommerceAbnormalRefundRequest(req); err != nil {
		return nil, err
	}

	resp, err := ecommerceClient.ApplyEcommerceAbnormalRefund(ctx, &wechat.EcommerceAbnormalRefundRequest{
		RefundID:    req.RefundID,
		SubMchID:    req.SubMchID,
		OutRefundNo: req.OutRefundNo,
		Type:        req.Type,
		BankType:    req.BankType,
		BankAccount: req.BankAccount,
		RealName:    req.RealName,
	})
	if err != nil {
		return nil, err
	}

	return toEcommerceRefundQueryContractResponse(resp), nil
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

func toEcommerceRefundCreateContractResponse(resp *wechat.EcommerceRefundResponse) *wechatcontracts.EcommerceRefundCreateResponse {
	if resp == nil {
		return nil
	}

	return &wechatcontracts.EcommerceRefundCreateResponse{
		RefundID:        resp.RefundID,
		OutRefundNo:     resp.OutRefundNo,
		CreateTime:      resp.CreateTime,
		Amount:          toContractEcommerceRefundAmount(resp.Amount),
		PromotionDetail: toContractEcommerceRefundPromotionDetails(resp.PromotionDetail),
		RefundAccount:   resp.RefundAccount,
	}
}

func toEcommerceRefundQueryContractResponse(resp *wechat.EcommerceRefundResponse) *wechatcontracts.EcommerceRefundQueryResponse {
	if resp == nil {
		return nil
	}

	return &wechatcontracts.EcommerceRefundQueryResponse{
		RefundID:            resp.RefundID,
		OutRefundNo:         resp.OutRefundNo,
		TransactionID:       resp.TransactionID,
		OutTradeNo:          resp.OutTradeNo,
		Channel:             resp.Channel,
		UserReceivedAccount: resp.UserReceivedAccount,
		SuccessTime:         resp.SuccessTime,
		CreateTime:          resp.CreateTime,
		Status:              resp.Status,
		Amount:              toContractEcommerceRefundAmount(resp.Amount),
		PromotionDetail:     toContractEcommerceRefundPromotionDetails(resp.PromotionDetail),
		RefundAccount:       resp.RefundAccount,
		FundsAccount:        resp.FundsAccount,
	}
}

func toRuntimeEcommerceRefundAmountFrom(values []wechatcontracts.EcommerceRefundAmountFrom) []wechat.EcommerceRefundAmountFrom {
	if len(values) == 0 {
		return nil
	}

	runtimeValues := make([]wechat.EcommerceRefundAmountFrom, 0, len(values))
	for _, value := range values {
		runtimeValues = append(runtimeValues, wechat.EcommerceRefundAmountFrom{
			Account: value.Account,
			Amount:  value.Amount,
		})
	}
	return runtimeValues
}

func toContractEcommerceRefundAmount(amount wechat.EcommerceRefundAmount) wechatcontracts.EcommerceRefundAmount {
	return wechatcontracts.EcommerceRefundAmount{
		Refund:         amount.Refund,
		From:           toContractEcommerceRefundAmountFrom(amount.From),
		PayerRefund:    amount.PayerRefund,
		DiscountRefund: amount.DiscountRefund,
		Currency:       amount.Currency,
		Advance:        amount.Advance,
	}
}

func toContractEcommerceRefundAmountFrom(values []wechat.EcommerceRefundAmountFrom) []wechatcontracts.EcommerceRefundAmountFrom {
	if len(values) == 0 {
		return nil
	}

	contractValues := make([]wechatcontracts.EcommerceRefundAmountFrom, 0, len(values))
	for _, value := range values {
		contractValues = append(contractValues, wechatcontracts.EcommerceRefundAmountFrom{
			Account: value.Account,
			Amount:  value.Amount,
		})
	}
	return contractValues
}

func toContractEcommerceRefundPromotionDetails(values []wechat.EcommerceRefundPromotionDetail) []wechatcontracts.EcommerceRefundPromotionDetail {
	if len(values) == 0 {
		return nil
	}

	contractValues := make([]wechatcontracts.EcommerceRefundPromotionDetail, 0, len(values))
	for _, value := range values {
		contractValues = append(contractValues, wechatcontracts.EcommerceRefundPromotionDetail{
			PromotionID:  value.PromotionID,
			Scope:        value.Scope,
			Type:         value.Type,
			Amount:       value.Amount,
			RefundAmount: value.RefundAmount,
		})
	}
	return contractValues
}
