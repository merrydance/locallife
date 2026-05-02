package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplymentSubmitRequestRejectsUnsupportedSubjectType(t *testing.T) {
	req := validApplymentSubmitRequest()
	req.SubjectInfo.SubjectType = SubjectTypeGovernment

	err := req.Validate()
	if err == nil {
		t.Fatal("expected unsupported subject type to fail validation")
	}
	if !strings.Contains(err.Error(), "subject_info.subject_type") {
		t.Fatalf("expected subject_type in validation error, got %v", err)
	}
}

func TestApplymentSubmitRequestRequiresMiniProgramSceneBinding(t *testing.T) {
	req := validApplymentSubmitRequest()
	req.BusinessInfo.SalesInfo.MiniProgramInfo = nil

	err := req.Validate()
	if err == nil {
		t.Fatal("expected mini program sales scene without mini_program_info to fail")
	}
	if !strings.Contains(err.Error(), "business_info.sales_info.mini_program_info") {
		t.Fatalf("expected mini_program_info in validation error, got %v", err)
	}
}

func TestApplymentSubmitRequestValidatesOfficialStoreSceneFields(t *testing.T) {
	req := validApplymentSubmitRequest()
	req.BusinessInfo.SalesInfo.StoreInfo = &ApplymentStoreInfo{
		StoreName:        "测试门店",
		StoreEntrancePic: []string{"store-front-media"},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected partial biz_store_info to fail validation")
	}
	if !strings.Contains(err.Error(), "business_info.sales_info.biz_store_info.biz_address_code") {
		t.Fatalf("expected biz_address_code in validation error, got %v", err)
	}

	req.BusinessInfo.SalesInfo.StoreInfo.AddressCode = "440305"
	req.BusinessInfo.SalesInfo.StoreInfo.StoreAddress = "深圳市南山区测试路100号"
	req.BusinessInfo.SalesInfo.StoreInfo.IndoorPic = []string{"store-indoor-media"}
	if err := req.Validate(); err != nil {
		t.Fatalf("complete official biz_store_info should pass validation: %v", err)
	}
}

func TestApplymentSubmitRequestJSONUsesOfficialFields(t *testing.T) {
	req := validApplymentSubmitRequest()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal applyment request: %v", err)
	}
	jsonBody := string(body)

	for _, field := range []string{
		`"business_code"`,
		`"contact_info"`,
		`"subject_info"`,
		`"business_info"`,
		`"settlement_info"`,
		`"bank_account_info"`,
		`"mini_program_info"`,
	} {
		if !strings.Contains(jsonBody, field) {
			t.Fatalf("expected JSON to contain %s, got %s", field, jsonBody)
		}
	}
	if strings.Contains(jsonBody, "operator") || strings.Contains(jsonBody, "ecommerce") {
		t.Fatalf("applyment contract leaked non-ordinary-service-provider vocabulary: %s", jsonBody)
	}
}

func validApplymentSubmitRequest() ApplymentSubmitRequest {
	return ApplymentSubmitRequest{
		BusinessCode: "1900000001_merchant_001",
		ContactInfo: ApplymentContactInfo{
			ContactType:  ContactTypeLegal,
			ContactName:  "encrypted-contact-name",
			MobilePhone:  "encrypted-mobile-phone",
			ContactEmail: "encrypted-contact-email",
		},
		SubjectInfo: ApplymentSubjectInfo{
			SubjectType: SubjectTypeEnterprise,
			BusinessLicenseInfo: &ApplymentBusinessLicenseInfo{
				LicenseCopy:   "media-license",
				LicenseNumber: "91310000MA1K000000",
				MerchantName:  "本地生活测试商户",
				LegalPerson:   "张三",
			},
			IdentityInfo: ApplymentIdentityInfo{
				IDHolderType: ContactTypeLegal,
				IDDocType:    IdentificationTypeIDCard,
				IDCardInfo: &ApplymentIDCardInfo{
					IDCardCopy:      "media-id-card-front",
					IDCardNational:  "media-id-card-back",
					IDCardName:      "encrypted-name",
					IDCardNumber:    "encrypted-number",
					CardPeriodBegin: "2020-01-01",
					CardPeriodEnd:   "长期",
				},
			},
		},
		BusinessInfo: ApplymentBusinessInfo{
			MerchantShortname: "本地生活",
			ServicePhone:      "07550000000",
			SalesInfo: ApplymentSalesInfo{
				SalesScenesType: []ApplymentSalesSceneType{SalesSceneMiniProgram},
				MiniProgramInfo: &ApplymentMiniProgramInfo{
					MiniProgramAppID: "wx-service-provider",
					MiniProgramPics:  []string{"media-mini-program-pic"},
				},
			},
		},
		SettlementInfo: ApplymentSettlementInfo{
			SettlementID:      "719",
			QualificationType: "餐饮",
		},
		BankAccountInfo: ApplymentBankAccountInfo{
			BankAccountType: BankAccountTypeCorporate,
			AccountName:     "encrypted-account-name",
			AccountBank:     "工商银行",
			AccountNumber:   "encrypted-account-number",
		},
	}
}
