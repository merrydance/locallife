package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPaymentPrepayRequestValidation(t *testing.T) {
	req := validPaymentPrepayRequest()
	req.NotifyURL = "http://example.test/notify"

	err := req.Validate()
	if err == nil {
		t.Fatal("expected non-https notify_url to fail")
	}
	if !strings.Contains(err.Error(), "notify_url") {
		t.Fatalf("expected notify_url in validation error, got %v", err)
	}

	req = validPaymentPrepayRequest()
	req.Payer = PaymentPayer{}
	err = req.Validate()
	if err == nil {
		t.Fatal("expected missing payer openid to fail")
	}
	if !strings.Contains(err.Error(), "payer") {
		t.Fatalf("expected payer in validation error, got %v", err)
	}
}

func TestPaymentPrepayRequestJSONUsesOfficialPartnerFields(t *testing.T) {
	req := validPaymentPrepayRequest()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal payment request: %v", err)
	}
	jsonBody := string(body)

	for _, field := range []string{
		`"sp_appid"`,
		`"sp_mchid"`,
		`"sub_mchid"`,
		`"out_trade_no"`,
		`"notify_url"`,
		`"settle_info"`,
		`"profit_sharing"`,
		`"sub_openid"`,
	} {
		if !strings.Contains(jsonBody, field) {
			t.Fatalf("expected JSON to contain %s, got %s", field, jsonBody)
		}
	}
	if strings.Contains(jsonBody, "combine_") {
		t.Fatalf("single payment contract leaked combine fields: %s", jsonBody)
	}
}

func TestCombinePrepayRequestValidation(t *testing.T) {
	req := validCombinePrepayRequest()
	req.SubOrders[0].MchID = "different-mchid"

	err := req.Validate()
	if err == nil {
		t.Fatal("expected mismatched sub order mchid to fail")
	}
	if !strings.Contains(err.Error(), "sub_orders[0].mchid") {
		t.Fatalf("expected sub order mchid in validation error, got %v", err)
	}

	req = validCombinePrepayRequest()
	req.SubOrders = make([]CombineSubOrder, 51)
	err = req.Validate()
	if err == nil {
		t.Fatal("expected more than 50 sub orders to fail")
	}
	if !strings.Contains(err.Error(), "sub_orders") {
		t.Fatalf("expected sub_orders in validation error, got %v", err)
	}
}

func validPaymentPrepayRequest() PaymentPrepayRequest {
	return PaymentPrepayRequest{
		SpAppID:     "wx-sp-appid",
		SpMchID:     "1900000001",
		SubAppID:    "wx-sub-appid",
		SubMchID:    "1900000109",
		Description: "本地生活订单",
		OutTradeNo:  "order-001",
		NotifyURL:   "https://example.test/wechat/pay",
		SettleInfo: &PaymentSettleInfo{
			ProfitSharing: true,
		},
		Amount: PaymentAmount{
			Total:    100,
			Currency: CurrencyCNY,
		},
		Payer: PaymentPayer{SubOpenID: "sub-openid"},
	}
}

func validCombinePrepayRequest() CombinePrepayRequest {
	return CombinePrepayRequest{
		CombineAppID:      "wx-sp-appid",
		CombineMchID:      "1900000001",
		CombineOutTradeNo: "combine-001",
		NotifyURL:         "https://example.test/wechat/combine-pay",
		CombinePayerInfo:  CombinePayerInfo{OpenID: "sp-openid"},
		SubOrders: []CombineSubOrder{
			{
				MchID:       "1900000001",
				SubMchID:    "1900000109",
				SubAppID:    "wx-sub-appid",
				OutTradeNo:  "order-001",
				Attach:      "merchant-001",
				Description: "本地生活订单",
				Amount: CombineAmount{
					TotalAmount: 100,
					Currency:    CurrencyCNY,
				},
				SettleInfo: &CombineSettleInfo{ProfitSharing: true},
			},
		},
	}
}
