package contracts

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRefundCreateRequestExposesOfficialFundsAccountWhenSet(t *testing.T) {
	requestType := reflect.TypeOf(RefundCreateRequest{})
	field, ok := requestType.FieldByName("FundsAccount")
	if !ok {
		t.Fatal("ordinary service provider refund request must expose official funds_account")
	}
	if field.Tag.Get("json") != "funds_account,omitempty" {
		t.Fatalf("FundsAccount json tag = %q, want funds_account,omitempty", field.Tag.Get("json"))
	}

	req := validRefundCreateRequest()
	req.FundsAccount = RefundFundsAccountAvailable
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal refund request: %v", err)
	}
	if !strings.Contains(string(body), "funds_account") {
		t.Fatalf("refund request JSON must contain official funds_account when set, got %s", string(body))
	}
}

func TestRefundCreateRequestValidation(t *testing.T) {
	req := validRefundCreateRequest()
	req.TransactionID = ""
	req.OutTradeNo = ""

	err := req.Validate()
	if err == nil {
		t.Fatal("expected missing order identity to fail")
	}
	if !strings.Contains(err.Error(), "transaction_id") {
		t.Fatalf("expected transaction_id/out_trade_no in validation error, got %v", err)
	}

	req = validRefundCreateRequest()
	req.Amount.Refund = 0
	err = req.Validate()
	if err == nil {
		t.Fatal("expected zero refund amount to fail")
	}
	if !strings.Contains(err.Error(), "amount.refund") {
		t.Fatalf("expected amount.refund in validation error, got %v", err)
	}
}

func TestProfitSharingOrderRequestValidation(t *testing.T) {
	req := validProfitSharingOrderRequest()
	req.Receivers[0].Name = ""

	err := req.Validate()
	if err == nil {
		t.Fatal("expected merchant receiver without encrypted name to fail")
	}
	if !strings.Contains(err.Error(), "receivers[0].name") {
		t.Fatalf("expected receivers[0].name in validation error, got %v", err)
	}

	req = validProfitSharingOrderRequest()
	req.Receivers[0].Amount = 0
	err = req.Validate()
	if err == nil {
		t.Fatal("expected zero receiver amount to fail")
	}
	if !strings.Contains(err.Error(), "receivers[0].amount") {
		t.Fatalf("expected receivers[0].amount in validation error, got %v", err)
	}
}

func validRefundCreateRequest() RefundCreateRequest {
	return RefundCreateRequest{
		SubMchID:      "1900000109",
		TransactionID: "4208450740201411110007820472",
		OutRefundNo:   "refund-001",
		Reason:        "用户取消订单",
		NotifyURL:     "https://example.test/wechat/refund",
		Amount: RefundAmountRequest{
			Refund:   100,
			Total:    100,
			Currency: CurrencyCNY,
		},
	}
}

func validProfitSharingOrderRequest() ProfitSharingOrderRequest {
	return ProfitSharingOrderRequest{
		SubMchID:        "1900000109",
		SubAppID:        "wx-sub-appid",
		TransactionID:   "4208450740201411110007820472",
		OutOrderNo:      "profit-sharing-001",
		UnfreezeUnsplit: true,
		Receivers: []ProfitSharingReceiver{
			{
				Type:        ReceiverTypeMerchantID,
				Account:     "1900000200",
				Name:        "encrypted-merchant-name",
				Amount:      100,
				Description: "平台服务费分账",
			},
		},
	}
}
