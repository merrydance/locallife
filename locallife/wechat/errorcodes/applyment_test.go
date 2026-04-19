package errorcodes

import "testing"

func TestCanonicalApplymentCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "TrimAndUpper", input: "  system_error ", expected: ApplymentCodeSystemError},
		{name: "LegacyFrequencyLimit", input: ApplymentCompatCodeFrequencyLimit, expected: ApplymentCodeFrequencyLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if actual := CanonicalApplymentCode(tc.input); actual != tc.expected {
				t.Fatalf("CanonicalApplymentCode(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestApplymentCodeSetsUseCanonicalAliases(t *testing.T) {
	if !EcommerceApplymentCreateDocumentedCodes.Has(ApplymentCodeResourceAlreadyExists) {
		t.Fatalf("expected applyment create documented set to include RESOURCE_ALREADY_EXISTS")
	}
	if !EcommerceApplymentQueryDocumentedCodes.Has(ApplymentCodeRateLimitExceeded) {
		t.Fatalf("expected applyment query documented set to include RATELIMIT_EXCEEDED")
	}
	if !SubMerchantSettlementModifyDocumentedCodes.Has(ApplymentCompatCodeFrequencyLimit) {
		t.Fatalf("expected settlement modify documented set to accept FREQENCY_LIMIT alias via canonicalization")
	}
	if !SubMerchantSettlementApplicationQueryDocumentedCodes.Has(ApplymentCodeOrderNotExist) {
		t.Fatalf("expected settlement application query documented set to include ORDER_NOT_EXIST")
	}
	if !MerchantMediaUploadDocumentedCodes.Has(ApplymentCodeFrequencyLimitExceed) {
		t.Fatalf("expected media upload documented set to include FREQUENCY_LIMIT_EXCEED")
	}
	if !CapitalPersonalBankListDocumentedCodes.Has(ApplymentCodeNotFound) {
		t.Fatalf("expected personal bank list documented set to include NOT_FOUND")
	}
	if !CapitalCorporateBankListDocumentedCodes.Has(ApplymentCodeFrequencyLimited) {
		t.Fatalf("expected corporate bank list documented set to include FREQUENCY_LIMITED")
	}
	if !CapitalBankAccountSearchKnownCodes.Has(ApplymentCodeInvalidRequest) {
		t.Fatalf("expected bank account search known set to include INVALID_REQUEST")
	}
	if !CapitalProvinceListKnownCodes.Has(ApplymentCodeSystemError) {
		t.Fatalf("expected province list known set to include SYSTEM_ERROR")
	}
	if !CapitalCityListKnownCodes.Has(ApplymentCodeSignError) {
		t.Fatalf("expected city list known set to include SIGN_ERROR")
	}
	if !CapitalBankBranchListDocumentedCodes.Has(ApplymentCodeNotFound) {
		t.Fatalf("expected bank branch list documented set to include NOT_FOUND")
	}
	if !ApplymentCommonCodes.Has(ApplymentCodeSignError) {
		t.Fatalf("expected applyment common codes to include SIGN_ERROR")
	}
}
