package errorcodes

import "testing"

func TestRefundDocumentedCodeSets(t *testing.T) {
	if !EcommerceRefundCreateDocumentedCodes.Has(RefundCodeUserAccountAbnormal) {
		t.Fatalf("expected create documented codes to include USER_ACCOUNT_ABNORMAL")
	}
	if !EcommerceRefundQueryDocumentedCodes.Has(RefundCodeRequestBlocked) {
		t.Fatalf("expected query documented codes to include REQUEST_BLOCKED")
	}
	if !EcommerceRefundAdvanceReturnCreateDocumentedCodes.Has(RefundCodeNotEnough) {
		t.Fatalf("expected return-advance create documented codes to include NOT_ENOUGH")
	}
	if !EcommerceRefundAbnormalDocumentedCodes.Has(RefundCodeSystemError) {
		t.Fatalf("expected abnormal documented codes to include SYSTEM_ERROR")
	}
}
