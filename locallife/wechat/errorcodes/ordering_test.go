package errorcodes

import "testing"

func TestCanonicalOrderingCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "TrimAndUpper", input: "  system_error ", expected: OrderingCodeSystemError},
		{name: "LegacyNoAuth", input: OrderingCompatCodeNoAuth, expected: OrderingCodeNoAuth},
		{name: "LegacyOrderNotExist", input: OrderingCompatCodeOrderNotExist, expected: OrderingCodeOrderNotExist},
		{name: "LegacyRuleLimit", input: OrderingCompatCodeRuleLimit, expected: OrderingCodeRuleLimit},
		{name: "LegacyAccountError", input: OrderingCompatCodeAccountError, expected: OrderingCodeAccountError},
		{name: "LegacyBankError", input: OrderingCompatCodeBankError, expected: OrderingCodeBankError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if actual := CanonicalOrderingCode(tc.input); actual != tc.expected {
				t.Fatalf("CanonicalOrderingCode(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestOrderingCodeSetsUseCanonicalAliases(t *testing.T) {
	if !OrderingInfrastructureCodes.Has(OrderingCompatCodeRuleLimit) {
		t.Fatalf("expected infrastructure codes to include legacy RULELIMIT alias")
	}
	if !OrderingInfrastructureCodes.Has(OrderingCompatCodeRateLimit) {
		t.Fatalf("expected infrastructure codes to include RATELIMIT_EXCEEDED compatibility code")
	}
	if !OrderingConfigurationCodes.Has(OrderingCompatCodeNoAuth) {
		t.Fatalf("expected configuration codes to include legacy NOAUTH alias")
	}
	if !PartnerSingleCreateDocumentedCodes.Has(OrderingCodeInvalidTransactionID) {
		t.Fatalf("expected partner single create documented set to include INVALID_TRANSACTIONID")
	}
	if !PartnerSingleCreateDocumentedCodes.Has(OrderingCodeOrderNotExist) {
		t.Fatalf("expected partner single create documented set to include ORDER_NOT_EXIST")
	}
	if !PartnerSingleQueryDocumentedCodes.Has(OrderingCodeTradeError) {
		t.Fatalf("expected partner single query documented set to include TRADE_ERROR")
	}
	if !PartnerSingleQueryCompatibilityCodes.Has(OrderingCodeBankError) {
		t.Fatalf("expected partner single query compatibility set to include BANK_ERROR")
	}
	if PartnerSingleQueryDocumentedCodes.Has(OrderingCodeBankError) {
		t.Fatalf("did not expect partner single query documented set to include BANK_ERROR")
	}
	if !PartnerSingleCloseDocumentedCodes.Has(OrderingCodeSignError) {
		t.Fatalf("expected partner single close documented set to include SIGN_ERROR")
	}
	if !PartnerSingleCloseCompatibilityCodes.Has(OrderingCodeOrderClosed) {
		t.Fatalf("expected partner single close compatibility set to include ORDER_CLOSED")
	}
	if PartnerSingleCloseDocumentedCodes.Has(OrderingCodeOrderClosed) {
		t.Fatalf("did not expect partner single close documented set to include ORDER_CLOSED")
	}
	if !CombineQueryDocumentedCodes.Has(OrderingCompatCodeOrderNotExist) {
		t.Fatalf("expected combine query documented set to accept ORDERNOTEXIST alias via canonicalization")
	}
	if !CombineQueryDocumentedCodes.Has(OrderingCodeInvalidTransactionID) {
		t.Fatalf("expected combine query documented set to include INVALID_TRANSACTIONID")
	}
	if !CombineCloseDocumentedCodes.Has(OrderingCodeNotEnough) {
		t.Fatalf("expected combine close documented set to include NOTENOUGH")
	}
	if !CombineCloseDocumentedCodes.Has(OrderingCodeUserPaying) {
		t.Fatalf("expected combine close documented set to include USERPAYING")
	}
}
