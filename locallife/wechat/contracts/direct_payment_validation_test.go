package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDirectJSAPIOrderRequest_PreservesExistingMessage(t *testing.T) {
	err := ValidateDirectJSAPIOrderRequest(&DirectJSAPIOrderRequest{
		Description:   "test",
		OutTradeNo:    "order-1",
		TotalAmount:   100,
		NotifyURL:     "https://example.com/notify",
		PayerOpenID:   "openid",
		PayerClientIP: "127.0.0.1",
		StoreInfo:     &DirectOrderStoreInfo{},
	})
	require.EqualError(t, err, "create direct jsapi order: scene_info.store_info.id is required when store_info is provided")
}

func TestValidateDirectOrderQueryResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectOrderQueryResponse("query direct order by out_trade_no", &DirectOrderQueryResponse{
		AppID:          "wx123",
		MchID:          "1900000109",
		OutTradeNo:     "order-1",
		TradeType:      DirectTradeTypeJSAPI,
		TradeState:     DirectTradeStateSuccess,
		TradeStateDesc: "success",
	}, true)

	var contractErr *DirectOrderQueryContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "query direct order by out_trade_no: wechat response missing transaction_id")
}

func TestValidateDirectPaymentNotificationResource_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectPaymentNotificationResource("decrypt direct payment notification", &DirectPaymentNotificationResource{
		AppID:          "wx123",
		MchID:          "1900000109",
		OutTradeNo:     "order-1",
		TradeType:      DirectTradeTypeJSAPI,
		TradeState:     DirectTradeStateSuccess,
		TradeStateDesc: "success",
		BankType:       "OTHERS",
		SuccessTime:    "2026-04-16T10:00:00+08:00",
		Amount: DirectOrderQueryAmount{
			Currency:      DirectPaymentCurrencyCNY,
			PayerCurrency: DirectPaymentCurrencyCNY,
		},
	})

	var contractErr *DirectPaymentNotificationContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "decrypt direct payment notification: wechat response missing transaction_id")
}

func TestValidateDirectPaymentNotificationResource_RejectsInvalidSuccessTime(t *testing.T) {
	err := ValidateDirectPaymentNotificationResource("decrypt direct payment notification", &DirectPaymentNotificationResource{
		AppID:          "wx123",
		MchID:          "1900000109",
		OutTradeNo:     "order-1",
		TransactionID:  "wx-order-1",
		TradeType:      DirectTradeTypeJSAPI,
		TradeState:     DirectTradeStateSuccess,
		TradeStateDesc: "success",
		BankType:       "OTHERS",
		SuccessTime:    "2026/04/16 10:00:00",
		Amount: DirectOrderQueryAmount{
			Currency:      DirectPaymentCurrencyCNY,
			PayerCurrency: DirectPaymentCurrencyCNY,
		},
	})

	require.ErrorContains(t, err, "success_time must be RFC3339")
}
