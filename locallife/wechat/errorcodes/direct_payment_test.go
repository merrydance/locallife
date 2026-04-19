package errorcodes

import "testing"

func TestDirectPaymentDocumentedCodeSets(t *testing.T) {
	if !DirectPaymentCreateDocumentedCodes.Has(DirectPaymentCodeOutTradeNoUsed) {
		t.Fatalf("expected direct payment create documented codes to include OUT_TRADE_NO_USED")
	}
	if !DirectPaymentQueryDocumentedCodes.Has(DirectPaymentCodeOrderNotExist) {
		t.Fatalf("expected direct payment query documented codes to include ORDER_NOT_EXIST")
	}
	if !DirectPaymentCloseDocumentedCodes.Has(DirectPaymentCodeTradeError) {
		t.Fatalf("expected direct payment close documented codes to include TRADE_ERROR")
	}
	if !DirectRefundCreateDocumentedCodes.Has(DirectPaymentCodeUserAccountAbnormal) {
		t.Fatalf("expected direct refund create documented codes to include USER_ACCOUNT_ABNORMAL")
	}
	if !DirectRefundCreateDocumentedCodes.Has(DirectPaymentCodeMchNotExists) {
		t.Fatalf("expected direct refund create documented codes to include MCH_NOT_EXISTS")
	}
	if !DirectRefundQueryDocumentedCodes.Has(DirectPaymentCodeResourceNotExists) {
		t.Fatalf("expected direct refund query documented codes to include RESOURCE_NOT_EXISTS")
	}
	if !DirectRefundQueryDocumentedCodes.Has(DirectPaymentCodeFrequencyLimited) {
		t.Fatalf("expected direct refund query documented codes to include FREQUENCY_LIMITED")
	}
	if !DirectRefundAbnormalDocumentedCodes.Has(DirectPaymentCodeFrequencyLimited) {
		t.Fatalf("expected direct refund abnormal documented codes to include FREQUENCY_LIMITED")
	}
}
