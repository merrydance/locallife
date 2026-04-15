package errorcodes

import "testing"

func TestCanonicalCancelWithdrawCode(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{input: "param_error", expected: CancelWithdrawCodeParamError},
		{input: "  frequency_limit ", expected: CancelWithdrawCodeFrequencyLimit},
		{input: "already_exists", expected: CancelWithdrawCodeAlreadyExists},
	}

	for _, tc := range testCases {
		if actual := CanonicalCancelWithdrawCode(tc.input); actual != tc.expected {
			t.Fatalf("CanonicalCancelWithdrawCode(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestCancelWithdrawDocumentedCodes(t *testing.T) {
	if !EcommerceCancelWithdrawCreateDocumentedCodes.Has(CancelWithdrawCodeAlreadyExists) {
		t.Fatalf("expected create documented set to include ALREADY_EXISTS")
	}
	if !EcommerceCancelWithdrawCreateDocumentedCodes.Has(CancelWithdrawCodeBizErrNeedRetry) {
		t.Fatalf("expected create documented set to include BIZ_ERR_NEED_RETRY")
	}
	if !EcommerceCancelWithdrawValidateDocumentedCodes.Has(CancelWithdrawCodeNoAuth) {
		t.Fatalf("expected validate documented set to include NO_AUTH")
	}
	if !EcommerceCancelWithdrawQueryDocumentedCodes.Has(CancelWithdrawCompatCodeFrequencyLimit) {
		t.Fatalf("expected query documented set to include FREQUENCY_LIMIT alias")
	}
}
