package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func recordDirectPaymentTimeoutQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.DirectOrderQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	consumer, businessOwner, err := directPaymentFactTarget(paymentOrder)
	if err != nil {
		return nil, err
	}
	objectKey := paymentOrder.OutTradeNo
	if queryResp.OutTradeNo != "" {
		objectKey = queryResp.OutTradeNo
	}
	tradeState := strings.ToUpper(strings.TrimSpace(queryResp.TradeState))
	terminalStatus := logic.NormalizeDirectPaymentTerminalStatus(tradeState)
	occurredAt := orderPaymentParseFactTime(queryResp.SuccessTime)
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    objectKey,
		ExternalSecondaryKey: orderPaymentStringPtr(queryResp.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(businessOwner),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        tradeState,
		TerminalStatus:       terminalStatus,
		Amount:               orderPaymentInt64Ptr(queryResp.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          directPaymentTimeoutQueryFactResource(paymentOrder, queryResp),
		DedupeKey:            fmt.Sprintf("wechat:query:direct:payment:%s:%s", objectKey, strings.ToLower(terminalStatus)),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func directPaymentFactTarget(paymentOrder db.PaymentOrder) (string, string, error) {
	switch paymentOrder.BusinessType {
	case db.ExternalPaymentBusinessOwnerRiderDeposit:
		return riderDepositPaymentFactConsumerDomain, db.ExternalPaymentBusinessOwnerRiderDeposit, nil
	case db.ExternalPaymentBusinessOwnerClaimRecovery:
		return claimRecoveryPaymentFactConsumer, db.ExternalPaymentBusinessOwnerClaimRecovery, nil
	case db.ExternalPaymentBusinessOwnerBaofuVerifyFee:
		return baofuVerifyFeePaymentFactConsumerDomain, db.ExternalPaymentBusinessOwnerBaofuVerifyFee, nil
	default:
		return "", "", fmt.Errorf("unsupported direct payment timeout fact owner %q for payment order %d", paymentOrder.BusinessType, paymentOrder.ID)
	}
}

func shouldRecordDirectPaymentRecoveryFact(order db.PaymentOrder) bool {
	if order.PaymentChannel != db.PaymentChannelDirect {
		return false
	}
	switch order.BusinessType {
	case db.ExternalPaymentBusinessOwnerRiderDeposit, db.ExternalPaymentBusinessOwnerClaimRecovery, db.ExternalPaymentBusinessOwnerBaofuVerifyFee:
		return true
	default:
		return false
	}
}

func recordRecoveredDirectPaymentFact(ctx context.Context, store db.Store, order db.PaymentOrder) (*db.ExternalPaymentFactApplication, error) {
	consumer, businessOwner, err := directPaymentFactTarget(order)
	if err != nil {
		return nil, err
	}
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceManualReconciliation,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    order.OutTradeNo,
		ExternalSecondaryKey: orderPaymentOptionalStringPtr(order.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(businessOwner),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(order.ID),
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(order.Amount),
		Currency:             "CNY",
		RawResource:          recoveredDirectPaymentFactResource(order),
		DedupeKey:            recoveredDirectPaymentFactDedupeKey(order, businessOwner),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   order.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func directPaymentTimeoutQueryFactResource(paymentOrder db.PaymentOrder, queryResp *wechatcontracts.DirectOrderQueryResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": paymentOrder.ID,
		"business_type":    paymentOrder.BusinessType,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   queryResp.TransactionID,
		"trade_state":      queryResp.TradeState,
		"success_time":     queryResp.SuccessTime,
		"amount":           queryResp.Amount.Total,
	})
	if err != nil {
		return nil
	}
	return raw
}

func recoveredDirectPaymentFactResource(order db.PaymentOrder) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": order.ID,
		"business_type":    order.BusinessType,
		"out_trade_no":     order.OutTradeNo,
		"transaction_id":   orderPaymentTextValue(order.TransactionID),
		"payment_channel":  order.PaymentChannel,
		"recovery_reason":  "paid_unprocessed_scan",
	})
	if err != nil {
		return nil
	}
	return raw
}

func recoveredDirectPaymentFactDedupeKey(order db.PaymentOrder, businessOwner string) string {
	return fmt.Sprintf("wechat:manual_reconciliation:direct_payment:%s:%s:success", businessOwner, order.OutTradeNo)
}
