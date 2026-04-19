package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePartnerJSAPIOrderRequest_PreservesExistingMessage(t *testing.T) {
	err := ValidatePartnerJSAPIOrderRequest(&PartnerJSAPIOrderRequest{
		SubMchID:      "1900000109",
		Description:   "test",
		OutTradeNo:    "order-1",
		TotalAmount:   100,
		PayerOpenID:   "openid",
		PayerClientIP: "127.0.0.1",
		StoreInfo:     &PartnerOrderStoreInfo{},
	})
	require.EqualError(t, err, "create partner jsapi order: scene_info.store_info.id is required when store_info is provided")
}

func TestValidateCombineOrderRequest_ValidatesSubOrders(t *testing.T) {
	err := ValidateCombineOrderRequest(&CombineOrderRequest{
		CombineOutTradeNo: "combine-1",
		PayerOpenID:       "openid",
		SubOrders: []SubOrder{
			{
				OutTradeNo:  "sub-1",
				Description: "sub order",
				Amount:      100,
			},
		},
	})
	require.EqualError(t, err, "create combine order: sub_orders[0].attach is required")
}

func TestValidatePartnerOrderQueryResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidatePartnerOrderQueryResponse("query partner order by transaction_id", &PartnerOrderQueryResponse{
		SpAppID:        "wx123",
		SpMchID:        "1900000109",
		SubMchID:       "1900000209",
		OutTradeNo:     "order-1",
		TradeType:      "JSAPI",
		TradeState:     "SUCCESS",
		TradeStateDesc: "success",
	}, true)

	var contractErr *PartnerOrderQueryContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "query partner order by transaction_id: wechat response missing transaction_id")
}

func TestValidateCombineOrderQueryResponse_RejectsIncompleteSceneInfo(t *testing.T) {
	err := ValidateCombineOrderQueryResponse("query combine order", &CombineQueryResponseBody{
		CombineAppID:      "wx123",
		CombineMchID:      "1900000109",
		CombineOutTradeNo: "combine-1",
		SceneInfo:         &CombineQuerySceneInfo{},
	})
	require.EqualError(t, err, "query combine order: scene_info.device_id is required when scene_info is present")
}

func TestValidatePartnerPaymentNotification_ReturnsTypedContractError(t *testing.T) {
	err := ValidatePartnerPaymentNotification("decrypt partner payment notification", &PartnerPaymentNotificationResource{
		SpAppID:        "wx123",
		SpMchID:        "1900000109",
		SubMchID:       "1900000209",
		OutTradeNo:     "order-1",
		TradeType:      "JSAPI",
		TradeState:     "SUCCESS",
		TradeStateDesc: "success",
		BankType:       "OTHERS",
		SuccessTime:    "2026-04-16T10:00:00+08:00",
		Amount: PartnerOrderQueryAmount{
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	})

	var contractErr *PartnerPaymentNotificationContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "decrypt partner payment notification: wechat response missing transaction_id")
}

func TestValidatePartnerPaymentNotification_RejectsInvalidSuccessTime(t *testing.T) {
	err := ValidatePartnerPaymentNotification("decrypt partner payment notification", &PartnerPaymentNotificationResource{
		SpAppID:        "wx123",
		SpMchID:        "1900000109",
		SubMchID:       "1900000209",
		OutTradeNo:     "order-1",
		TransactionID:  "wx-order-1",
		TradeType:      "JSAPI",
		TradeState:     "SUCCESS",
		TradeStateDesc: "success",
		BankType:       "OTHERS",
		SuccessTime:    "2026/04/16 10:00:00",
		Amount: PartnerOrderQueryAmount{
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	})

	require.ErrorContains(t, err, "success_time must be RFC3339")
}

func TestValidateCombinePaymentNotification_RequiresPayerOpenID(t *testing.T) {
	err := ValidateCombinePaymentNotification("decrypt combine payment notification", &CombinePaymentNotification{
		CombineAppID:      "wx123",
		CombineMchID:      "1900000109",
		CombineOutTradeNo: "combine-1",
		SubOrders: []CombinePaymentNotificationSubOrder{{
			MchID:         "1900000109",
			OutTradeNo:    "sub-1",
			TransactionID: "wx-sub-1",
			TradeType:     "JSAPI",
			TradeState:    "SUCCESS",
			BankType:      "OTHERS",
			SuccessTime:   "2026-04-16T10:00:00.123+08:00",
			Amount: struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}},
	})

	var contractErr *CombinePaymentNotificationContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "decrypt combine payment notification: combine_payer_info is required")
}
