package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountWillingnessSubmitRequestJSONUsesOfficialNestedStructure(t *testing.T) {
	req := AccountWillingnessSubmitRequest{
		BusinessCode: "1900000001_willingness_001",
		ContactInfo: AccountWillingnessContactInfo{
			Name:             "encrypted-name",
			Mobile:           "encrypted-mobile",
			IDCardNumber:     "encrypted-id",
			ContactType:      ContactTypeLegal,
			ContactIDDocType: IdentificationTypeIDCard,
		},
		SubjectInfo: AccountWillingnessSubjectInfo{
			SubjectType:          SubjectTypeIndividual,
			IsFinanceInstitution: false,
			BusinessLicenceInfo: &AccountWillingnessBusinessLicenceInfo{
				LicenceNumber:    "91440300MA00000000",
				LicenceCopy:      "license-media",
				MerchantName:     "测试餐饮店",
				LegalPerson:      "张三",
				CompanyAddress:   "深圳市南山区测试路",
				LicenceValidDate: `["2020-01-01","长期"]`,
			},
			AssistProveInfo: &AccountWillingnessAssistProveInfo{
				MicroBizType:     MicroBizTypeStore,
				StoreName:        "测试门店",
				StoreAddressCode: "440305",
				StoreAddress:     "深圳市南山区测试路100号",
				StoreHeaderCopy:  "store-header-media",
				StoreIndoorCopy:  "store-indoor-media",
			},
		},
		IdentificationInfo: AccountWillingnessIdentificationInfo{
			IDHolderType:            ContactTypeLegal,
			IdentificationType:      IdentificationTypeIDCard,
			IdentificationName:      "encrypted-id-name",
			IdentificationNumber:    "encrypted-id-number",
			IdentificationValidDate: `["2020-01-01","长期"]`,
			IdentificationFrontCopy: "id-front-media",
			IdentificationBackCopy:  "id-back-media",
			Owner:                   true,
		},
		AdditionInfo: &AccountWillingnessAdditionInfo{ConfirmMchIDList: []string{"1900000109"}},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal account willingness request: %v", err)
	}
	jsonBody := string(body)
	for _, field := range []string{
		`"contact_info"`,
		`"subject_info"`,
		`"business_licence_info"`,
		`"assist_prove_info"`,
		`"store_address_code"`,
		`"identification_info"`,
		`"identification_valid_date"`,
		`"confirm_mchid_list"`,
	} {
		if !strings.Contains(jsonBody, field) {
			t.Fatalf("expected account willingness JSON to contain %s, got %s", field, jsonBody)
		}
	}
	if strings.Contains(jsonBody, `"contact_info":"`) {
		t.Fatalf("account willingness contact_info must be official object structure, got %s", jsonBody)
	}
}

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
