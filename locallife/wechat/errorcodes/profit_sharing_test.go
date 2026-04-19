package errorcodes

import "testing"

func TestCanonicalProfitSharingCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "TrimAndUpper", input: "  system_error ", expected: ProfitSharingCodeSystemError},
		{name: "KeepKnownValue", input: ProfitSharingCodeNoAuth, expected: ProfitSharingCodeNoAuth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if actual := CanonicalProfitSharingCode(tc.input); actual != tc.expected {
				t.Fatalf("CanonicalProfitSharingCode(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestProfitSharingHelpers(t *testing.T) {
	if !ProfitSharingCodeEquals(" not_enough ", ProfitSharingCodeNotEnough) {
		t.Fatalf("expected ProfitSharingCodeEquals to canonicalize values")
	}
	if !IsProfitSharingReturnProcessingCode(ProfitSharingCodeNotEnough) {
		t.Fatalf("expected NOT_ENOUGH to be treated as processing")
	}
	if IsProfitSharingReturnProcessingCode(ProfitSharingCodeParamError) {
		t.Fatalf("did not expect PARAM_ERROR to be treated as processing")
	}
}

func TestProfitSharingDocumentedCodeSets(t *testing.T) {
	if !ProfitSharingCreateDocumentedCodes.Has(ProfitSharingCodeRuleLimit) {
		t.Fatalf("expected create documented codes to include RULE_LIMIT")
	}
	if !ProfitSharingQueryDocumentedCodes.Has(ProfitSharingCodeResourceNotExist) {
		t.Fatalf("expected query documented codes to include RESOURCE_NOT_EXISTS")
	}
	if !ProfitSharingReturnQueryDocumentedCodes.Has(ProfitSharingCodeFrequencyLimited) {
		t.Fatalf("expected return query documented codes to include FREQUENCY_LIMITED")
	}
}
