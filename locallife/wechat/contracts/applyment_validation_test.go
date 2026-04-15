package contracts

import "testing"

func TestValidateEcommerceApplymentRequest_RejectsUnsupportedOrganizationType(t *testing.T) {
	req := validEnterpriseApplymentRequest()
	req.OrganizationType = "2500"

	err := ValidateEcommerceApplymentRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "validate ecommerce applyment request: organization_type must be \"2\" or \"4\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEcommerceApplymentRequest_RequiresPrivateOrBusinessAccountForIndividual(t *testing.T) {
	req := validEnterpriseApplymentRequest()
	req.OrganizationType = ApplymentOrganizationTypeIndividualBusiness
	req.AccountInfo.BankAccountType = "99"

	err := ValidateEcommerceApplymentRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateEcommerceApplymentRequest_RequiresStoreURLOrQRCode(t *testing.T) {
	req := validEnterpriseApplymentRequest()
	req.SalesSceneInfo.StoreURL = ""
	req.SalesSceneInfo.StoreQRCode = ""

	err := ValidateEcommerceApplymentRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateEcommerceApplymentQueryByOutRequestNoResponse_RequiresAccountValidationForVerifyState(t *testing.T) {
	resp := &EcommerceApplymentQueryResponse{
		ApplymentState:     ApplymentStateAccountNeedVerify,
		ApplymentStateDesc: "待账户验证",
		OutRequestNo:       "REQ-1",
		ApplymentID:        1001,
	}

	err := ValidateEcommerceApplymentQueryByOutRequestNoResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
}

func TestValidateEcommerceApplymentQueryByIDResponse_RequiresSignURLForUnsignedSignState(t *testing.T) {
	resp := &EcommerceApplymentQueryResponse{
		ApplymentState:     ApplymentStateAuditing,
		ApplymentStateDesc: "审核中",
		OutRequestNo:       "REQ-2",
		ApplymentID:        1002,
		SignState:          ApplymentSignStateUnsigned,
	}

	err := ValidateEcommerceApplymentQueryByIDResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
}

func TestValidateEcommerceApplymentQueryByOutRequestNoResponse_RejectsUnexpectedSignURL(t *testing.T) {
	resp := &EcommerceApplymentQueryResponse{
		ApplymentState:     ApplymentStateAuditing,
		ApplymentStateDesc: "审核中",
		OutRequestNo:       "REQ-3",
		ApplymentID:        1003,
		SignURL:            "https://example.com/sign",
	}

	err := ValidateEcommerceApplymentQueryByOutRequestNoResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
}

func TestValidateSubMerchantSettlementResponse_RequiresFailReasonOnFailure(t *testing.T) {
	resp := &SubMerchantSettlementResponse{
		AccountType:   SubMerchantSettlementAccountTypeBusiness,
		AccountBank:   "工商银行",
		AccountNumber: "62*************78",
		VerifyResult:  SubMerchantSettlementVerifyResultFail,
	}

	err := ValidateSubMerchantSettlementResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
}

func TestValidateSubMerchantSettlementApplicationResponse_RejectsInvalidFinishTime(t *testing.T) {
	resp := &QuerySubMerchantSettlementApplicationResponse{
		AccountName:      "张*",
		AccountType:      SubMerchantSettlementAccountTypeBusiness,
		AccountBank:      "工商银行",
		AccountNumber:    "62*************78",
		VerifyResult:     SubMerchantSettlementApplicationAuditSuccess,
		VerifyFinishTime: "2026/04/15 12:00:00",
	}

	err := ValidateSubMerchantSettlementApplicationResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
}

func TestValidateEcommerceApplymentRequest_AcceptsEnterpriseRequest(t *testing.T) {
	if err := ValidateEcommerceApplymentRequest(validEnterpriseApplymentRequest()); err != nil {
		t.Fatalf("expected request to be valid, got %v", err)
	}
}

func validEnterpriseApplymentRequest() *EcommerceApplymentRequest {
	return &EcommerceApplymentRequest{
		OutRequestNo:     "APPLYMENT_0001",
		OrganizationType: ApplymentOrganizationTypeEnterprise,
		BusinessLicenseInfo: &ApplymentBusinessLicenseInfo{
			BusinessLicenseCopy:   "media-license",
			BusinessLicenseNumber: "91350000123456789A",
			MerchantName:          "测试企业",
			LegalPerson:           "张三",
		},
		IDCardInfo: &ApplymentIDCardInfo{
			IDCardCopy:           "media-id-front",
			IDCardNational:       "media-id-back",
			IDCardName:           "cipher-name",
			IDCardNumber:         "cipher-id-no",
			IDCardValidTimeBegin: "2020-01-01",
			IDCardValidTime:      "长期",
		},
		AccountInfo: &ApplymentBankAccountInfo{
			BankAccountType: ApplymentBankAccountTypeBusiness,
			AccountBank:     "工商银行",
			AccountName:     "cipher-account-name",
			AccountNumber:   "cipher-account-number",
		},
		ContactInfo: &ApplymentContactInfo{
			ContactType: ApplymentContactTypeLegal,
			ContactName: "cipher-contact-name",
			MobilePhone: "cipher-mobile",
		},
		SalesSceneInfo: &ApplymentSalesSceneInfo{
			StoreName: "测试门店",
			StoreURL:  "https://example.com/store",
		},
		MerchantShortname: "测试简称",
	}
}
