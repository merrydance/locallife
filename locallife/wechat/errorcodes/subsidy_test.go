package errorcodes

import "testing"

func TestCanonicalSubsidyCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "TrimAndUpper", input: "  system_error ", expected: SubsidyCodeSystemError},
		{name: "KeepKnownValue", input: SubsidyCodeNoAuth, expected: SubsidyCodeNoAuth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if actual := CanonicalSubsidyCode(tc.input); actual != tc.expected {
				t.Fatalf("CanonicalSubsidyCode(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestSubsidyCodeEquals(t *testing.T) {
	if !SubsidyCodeEquals(" not_enough ", SubsidyCodeNotEnough) {
		t.Fatalf("expected SubsidyCodeEquals to canonicalize values")
	}
	if SubsidyCodeEquals(SubsidyCodeInvalidRequest, SubsidyCodeSignError) {
		t.Fatalf("did not expect different codes to compare equal")
	}
}

func TestSubsidyDocumentedCodeSets(t *testing.T) {
	if !SubsidyCreateDocumentedCodes.Has(SubsidyCodeFrequencyLimited) {
		t.Fatalf("expected create documented codes to include FREQUENCY_LIMITED")
	}
	if !SubsidyReturnDocumentedCodes.Has(SubsidyCodeSystemError) {
		t.Fatalf("expected return documented codes to include SYSTEM_ERROR")
	}
	if !SubsidyCancelDocumentedCodes.Has(SubsidyCodeParamError) {
		t.Fatalf("expected cancel documented codes to include PARAM_ERROR")
	}
}
