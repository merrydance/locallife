package errorcodes

import "testing"

func TestComplaintDocumentedCodeSets(t *testing.T) {
	if !ComplaintQueryDocumentedCodes.Has(ComplaintCodeSystemError) {
		t.Fatalf("expected complaint query documented codes to include SYSTEM_ERROR")
	}
	if !ComplaintNotificationConfigDocumentedCodes.Has(ComplaintCodeSignError) {
		t.Fatalf("expected complaint notification config documented codes to include SIGN_ERROR")
	}
	if !ComplaintHandlingDocumentedCodes.Has(ComplaintCodeInvalidRequest) {
		t.Fatalf("expected complaint handling documented codes to include INVALID_REQUEST")
	}
	if !ComplaintImageDocumentedCodes.Has(ComplaintCodeParamError) {
		t.Fatalf("expected complaint image documented codes to include PARAM_ERROR")
	}
}
