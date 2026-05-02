package worker

import (
	"context"
	"errors"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

func isProfitSharingReturnProcessingCommandError(err error) bool {
	if err == nil {
		return false
	}
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		return wechaterrorcodes.IsProfitSharingReturnProcessingCode(wxErr.Code)
	}
	var ordinaryErr *ordinaryserviceprovider.ProviderError
	if errors.As(err, &ordinaryErr) {
		return wechaterrorcodes.IsProfitSharingReturnProcessingCode(ordinaryErr.ProviderCode)
	}
	return false
}

func (processor *RedisTaskProcessor) createWechatProfitSharing(ctx context.Context, paymentOrder db.PaymentOrder, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing")
		}
		resp, err := processor.ordinarySPClient.CreateProfitSharingOrder(ctx, ospcontracts.ProfitSharingOrderRequest{
			SubMchID:        req.SubMchID,
			AppID:           req.AppID,
			TransactionID:   req.TransactionID,
			OutOrderNo:      req.OutOrderNo,
			Receivers:       ordinaryProfitSharingReceivers(req.Receivers),
			UnfreezeUnsplit: req.Finish,
		})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingOrderResponse(resp), nil
	}
	if processor.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing")
	}
	return processor.ecommerceClient.CreateProfitSharing(ctx, req)
}

func (processor *RedisTaskProcessor) queryWechatProfitSharing(ctx context.Context, paymentOrder db.PaymentOrder, subMchID string, transactionID string, outOrderNo string) (*wechatcontracts.ProfitSharingQueryResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing query")
		}
		resp, err := processor.ordinarySPClient.QueryProfitSharingOrder(ctx, ospcontracts.ProfitSharingQueryRequest{SubMchID: subMchID, TransactionID: transactionID, OutOrderNo: outOrderNo})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingQueryResponse(resp), nil
	}
	if processor.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing query")
	}
	return processor.ecommerceClient.QueryProfitSharing(ctx, subMchID, transactionID, outOrderNo)
}

func (processor *RedisTaskProcessor) finishWechatProfitSharing(ctx context.Context, paymentOrder db.PaymentOrder, subMchID string, transactionID string, outOrderNo string, description string) (*wechatcontracts.ProfitSharingResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing finish")
		}
		resp, err := processor.ordinarySPClient.UnfreezeProfitSharing(ctx, ospcontracts.ProfitSharingUnfreezeRequest{SubMchID: subMchID, TransactionID: transactionID, OutOrderNo: outOrderNo, Description: description})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingUnfreezeResponse(resp), nil
	}
	if processor.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing finish")
	}
	return processor.ecommerceClient.FinishProfitSharing(ctx, subMchID, transactionID, outOrderNo, description)
}

func (processor *RedisTaskProcessor) createWechatProfitSharingReturn(ctx context.Context, paymentOrder db.PaymentOrder, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing return")
		}
		resp, err := processor.ordinarySPClient.CreateProfitSharingReturn(ctx, ospcontracts.ProfitSharingReturnRequest{
			SubMchID:    req.SubMchID,
			OrderID:     req.OrderID,
			OutOrderNo:  req.OutOrderNo,
			OutReturnNo: req.OutReturnNo,
			ReturnMchID: req.ReturnMchID,
			Amount:      req.Amount,
			Description: req.Description,
		})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingReturnResponse(resp, req.TransactionID), nil
	}
	if processor.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing return")
	}
	return processor.ecommerceClient.CreateProfitSharingReturn(ctx, req)
}

func (processor *RedisTaskProcessor) queryWechatProfitSharingReturn(ctx context.Context, paymentOrder db.PaymentOrder, returnRecord db.ProfitSharingReturn) (*wechatcontracts.ProfitSharingReturnResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing return query")
		}
		resp, err := processor.ordinarySPClient.QueryProfitSharingReturn(ctx, ospcontracts.ProfitSharingReturnQueryRequest{SubMchID: returnRecord.SubMchid, OutReturnNo: returnRecord.OutReturnNo, OutOrderNo: returnRecord.OutOrderNo})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingReturnResponse(resp, ""), nil
	}
	if processor.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing return query")
	}
	return processor.ecommerceClient.QueryProfitSharingReturn(ctx, returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo)
}

func ordinaryProfitSharingReceivers(receivers []wechatcontracts.ProfitSharingReceiver) []ospcontracts.ProfitSharingReceiver {
	result := make([]ospcontracts.ProfitSharingReceiver, 0, len(receivers))
	for _, receiver := range receivers {
		result = append(result, ospcontracts.ProfitSharingReceiver{
			Type:        ospcontracts.ReceiverType(receiver.Type),
			Account:     receiver.ReceiverAccount,
			Name:        receiver.ReceiverName,
			Amount:      receiver.Amount,
			Description: receiver.Description,
		})
	}
	return result
}

func ordinaryProfitSharingOrderResponse(resp *ospcontracts.ProfitSharingOrderResponse) *wechatcontracts.ProfitSharingResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingResponse{
		SubMchID:      resp.SubMchID,
		TransactionID: resp.TransactionID,
		OutOrderNo:    resp.OutOrderNo,
		OrderID:       resp.OrderID,
		Status:        string(resp.State),
		Receivers:     ordinaryProfitSharingReceiverResults(resp.Receivers),
	}
}

func ordinaryProfitSharingQueryResponse(resp *ospcontracts.ProfitSharingOrderResponse) *wechatcontracts.ProfitSharingQueryResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingQueryResponse{
		SubMchID:      resp.SubMchID,
		TransactionID: resp.TransactionID,
		OutOrderNo:    resp.OutOrderNo,
		OrderID:       resp.OrderID,
		Status:        string(resp.State),
		Receivers:     ordinaryProfitSharingReceiverResults(resp.Receivers),
	}
}

func ordinaryProfitSharingUnfreezeResponse(resp *ospcontracts.ProfitSharingUnfreezeResponse) *wechatcontracts.ProfitSharingResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingResponse{SubMchID: resp.SubMchID, TransactionID: resp.TransactionID, OutOrderNo: resp.OutOrderNo, OrderID: resp.OrderID, Status: string(resp.State)}
}

func ordinaryProfitSharingReceiverResults(receivers []ospcontracts.ProfitSharingReceiverDetail) []wechatcontracts.ProfitSharingReceiverResult {
	result := make([]wechatcontracts.ProfitSharingReceiverResult, 0, len(receivers))
	for _, receiver := range receivers {
		result = append(result, wechatcontracts.ProfitSharingReceiverResult{
			Type:            string(receiver.Type),
			ReceiverAccount: receiver.Account,
			Amount:          receiver.Amount,
			Description:     receiver.Description,
			Result:          string(receiver.Result),
			FinishTime:      receiver.FinishTime,
			FailReason:      string(receiver.FailReason),
			DetailID:        receiver.DetailID,
		})
	}
	return result
}

func ordinaryProfitSharingReturnResponse(resp *ospcontracts.ProfitSharingReturnResponse, transactionID string) *wechatcontracts.ProfitSharingReturnResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:      resp.SubMchID,
		OrderID:       resp.OrderID,
		OutOrderNo:    resp.OutOrderNo,
		OutReturnNo:   resp.OutReturnNo,
		ReturnID:      resp.ReturnID,
		ReturnMchID:   resp.ReturnMchID,
		Amount:        resp.Amount,
		Result:        string(resp.State),
		FinishTime:    resp.FinishTime,
		FailReason:    resp.FailReason,
		TransactionID: transactionID,
	}
}
