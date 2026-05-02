package contracts

import (
	"reflect"
	"strings"
	"testing"
)

func TestOfficialPaymentNotificationContractUsesDedicatedDecodedPayload(t *testing.T) {
	assertEndpointResponseContract(t, EndpointPaymentNotify, PaymentNotificationPayload{})
	assertEndpointResponseContract(t, EndpointCombineNotify, CombinePaymentNotificationPayload{})
	assertEndpointResponseContract(t, EndpointRefundNotify, RefundNotificationPayload{})
	assertEndpointResponseContract(t, EndpointProfitSharingNotify, ProfitSharingNotificationPayload{})
}

func TestPaymentNotificationPayloadValidationRequiresOfficialFields(t *testing.T) {
	payload := validPaymentNotificationPayload()
	payload.SpAppID = ""

	err := ValidatePaymentNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "sp_appid") {
		t.Fatalf("expected payment notification to require sp_appid, got %v", err)
	}

	payload = validPaymentNotificationPayload()
	payload.Amount = nil
	err = ValidatePaymentNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "amount") {
		t.Fatalf("expected payment notification to require amount, got %v", err)
	}
}

func TestCombinePaymentNotificationPayloadValidationRequiresOfficialFields(t *testing.T) {
	payload := validCombinePaymentNotificationPayload()
	payload.SubOrders[0].SubMchID = ""

	err := ValidateCombinePaymentNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "sub_orders[0].sub_mchid") {
		t.Fatalf("expected combine notification to require sub order sub_mchid, got %v", err)
	}

	payload = validCombinePaymentNotificationPayload()
	payload.CombinePayerInfo.OpenID = ""
	err = ValidateCombinePaymentNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "combine_payer_info.openid") {
		t.Fatalf("expected combine notification to require payer openid, got %v", err)
	}
}

func TestRefundNotificationPayloadValidationRequiresOfficialFields(t *testing.T) {
	payload := validRefundNotificationPayload()
	payload.SubMchID = ""

	err := ValidateRefundNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "sub_mchid") {
		t.Fatalf("expected refund notification to require sub_mchid, got %v", err)
	}

	payload = validRefundNotificationPayload()
	payload.Amount = nil
	err = ValidateRefundNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "amount") {
		t.Fatalf("expected refund notification to require amount, got %v", err)
	}
}

func TestProfitSharingNotificationPayloadValidationRequiresOfficialFields(t *testing.T) {
	payload := validProfitSharingNotificationPayload()
	payload.Receiver.Account = ""

	err := ValidateProfitSharingNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "receiver.account") {
		t.Fatalf("expected profit sharing notification to require receiver.account, got %v", err)
	}

	payload = validProfitSharingNotificationPayload()
	payload.SpMchID = ""
	err = ValidateProfitSharingNotificationPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "sp_mchid") {
		t.Fatalf("expected profit sharing notification to require sp_mchid, got %v", err)
	}
}

func assertEndpointResponseContract(t *testing.T, endpointID EndpointID, contract any) {
	t.Helper()
	endpoint, ok := EndpointContractByID(endpointID)
	if !ok {
		t.Fatalf("missing endpoint contract %s", endpointID)
	}
	want := reflect.TypeOf(contract)
	if want.Kind() == reflect.Pointer {
		want = want.Elem()
	}
	for _, got := range endpoint.ResponseTypes {
		if got == want {
			return
		}
	}
	t.Fatalf("%s response contracts = %v, want %s", endpointID, endpoint.ResponseTypes, want.Name())
}

func validPaymentNotificationPayload() PaymentNotificationPayload {
	return PaymentNotificationPayload{
		SpAppID:        "wx8888888888888888",
		SpMchID:        "1230000109",
		SubMchID:       "1900000109",
		OutTradeNo:     "order-001",
		TransactionID:  "420000000000000001",
		TradeType:      "JSAPI",
		TradeState:     PaymentTradeStateSuccess,
		TradeStateDesc: "支付成功",
		BankType:       "OTHERS",
		SuccessTime:    "2026-05-02T12:00:00+08:00",
		Payer:          PaymentPayer{SpOpenID: "openid"},
		Amount:         &PaymentAmount{Total: 100, PayerTotal: 100, Currency: CurrencyCNY, PayerCurrency: CurrencyCNY},
	}
}

func validCombinePaymentNotificationPayload() CombinePaymentNotificationPayload {
	return CombinePaymentNotificationPayload{
		CombineAppID:      "wx8888888888888888",
		CombineMchID:      "1230000109",
		CombineOutTradeNo: "combine-001",
		CombinePayerInfo:  CombinePayerInfo{OpenID: "openid"},
		SubOrders: []CombineOrderState{{
			MchID:         "1230000109",
			SubMchID:      "1900000109",
			OutTradeNo:    "order-001",
			TransactionID: "420000000000000001",
			TradeType:     "JSAPI",
			TradeState:    PaymentTradeStateSuccess,
			BankType:      "OTHERS",
			SuccessTime:   "2026-05-02T12:00:00+08:00",
			Amount:        CombineAmount{TotalAmount: 100, PayerAmount: 100, Currency: CurrencyCNY, PayerCurrency: CurrencyCNY},
		}},
	}
}

func validRefundNotificationPayload() RefundNotificationPayload {
	return RefundNotificationPayload{
		SpMchID:       "1230000109",
		SubMchID:      "1900000109",
		OutTradeNo:    "order-001",
		TransactionID: "420000000000000001",
		OutRefundNo:   "refund-001",
		RefundID:      "503000000000000001",
		RefundStatus:  RefundStatusSuccess,
		Amount:        &RefundAmount{Total: 100, Refund: 100, PayerTotal: 100, PayerRefund: 100},
	}
}

func validProfitSharingNotificationPayload() ProfitSharingNotificationPayload {
	return ProfitSharingNotificationPayload{
		SpMchID:       "1230000109",
		SubMchID:      "1900000109",
		TransactionID: "420000000000000001",
		OrderID:       "300000000000000001",
		OutOrderNo:    "profit-sharing-001",
		Receiver: ProfitSharingReceiverDetail{
			Type:        ReceiverTypeMerchantID,
			Account:     "1900000200",
			Amount:      100,
			Description: "订单分账",
		},
	}
}
