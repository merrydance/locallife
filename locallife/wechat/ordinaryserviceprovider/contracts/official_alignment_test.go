package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfitSharingRemainingAmountResponseUsesOfficialUnsplitAmountField(t *testing.T) {
	var response ProfitSharingRemainingAmountResponse
	err := json.Unmarshal([]byte(`{"transaction_id":"4200000000001","unsplit_amount":123}`), &response)
	if err != nil {
		t.Fatalf("unmarshal remaining amount response: %v", err)
	}
	if response.UnsplitAmount != 123 {
		t.Fatalf("unsplit_amount = %d, want 123", response.UnsplitAmount)
	}
}

func TestOfficialNotificationContractsUseEncryptedEnvelopeAndDecodedPayloadFields(t *testing.T) {
	envelope := NotificationRequest{
		ID:           "EV-001",
		CreateTime:   "2026-05-02T12:00:00+08:00",
		EventType:    "TRANSACTION.SUCCESS",
		ResourceType: "encrypt-resource",
		Summary:      "支付成功",
		Resource: &NotificationResource{
			Algorithm:      "AEAD_AES_256_GCM",
			Ciphertext:     "ciphertext",
			OriginalType:   "transaction",
			AssociatedData: "transaction",
			Nonce:          "nonce",
		},
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal notification envelope: %v", err)
	}
	jsonBody := string(body)
	for _, field := range []string{`"event_type"`, `"resource_type"`, `"summary"`, `"algorithm"`, `"associated_data"`, `"original_type"`, `"nonce"`} {
		if !strings.Contains(jsonBody, field) {
			t.Fatalf("expected official notification envelope field %s, got %s", field, jsonBody)
		}
	}

	var violation MerchantViolationNotificationPayload
	err = json.Unmarshal([]byte(`{"sub_mchid":"1900000109","company_name":"测试商户","record_id":"R1","punish_plan":"LIMIT","punish_time":"2026-05-02T12:00:00+08:00","punish_description":"限制收款","risk_type":"QUALIFICATION","risk_description":"资料异常"}`), &violation)
	if err != nil {
		t.Fatalf("unmarshal merchant violation notification: %v", err)
	}
	if violation.RecordID != "R1" || violation.RiskDescription == "" {
		t.Fatalf("merchant violation notification fields not mapped: %+v", violation)
	}

	var refund RefundNotificationPayload
	err = json.Unmarshal([]byte(`{"refund_status":"SUCCESS","refund_id":"r1","out_refund_no":"or1"}`), &refund)
	if err != nil {
		t.Fatalf("unmarshal refund notification: %v", err)
	}
	if refund.RefundStatus != RefundStatusSuccess {
		t.Fatalf("refund_status = %s, want SUCCESS", refund.RefundStatus)
	}

	var profitSharing ProfitSharingNotificationPayload
	err = json.Unmarshal([]byte(`{"receiver":{"type":"MERCHANT_ID","account":"1900000109","amount":88,"description":"分账"}}`), &profitSharing)
	if err != nil {
		t.Fatalf("unmarshal profit sharing notification: %v", err)
	}
	if profitSharing.Receiver.Amount != 88 {
		t.Fatalf("receiver amount = %d, want 88", profitSharing.Receiver.Amount)
	}
}
