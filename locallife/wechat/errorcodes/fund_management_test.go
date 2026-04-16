package errorcodes

import "testing"

func TestFundManagementDocumentedCodeSets(t *testing.T) {
	if !FundManagementBalanceDocumentedCodes.Has(FundManagementCodeNoAuth) {
		t.Fatalf("expected balance documented codes to include NO_AUTH")
	}
	if !FundManagementWithdrawDocumentedCodes.Has(FundManagementCodeAccountNotVerified) {
		t.Fatalf("expected withdraw documented codes to include ACCOUNT_NOT_VERIFIED")
	}
	if !FundManagementWithdrawDocumentedCodes.Has(FundManagementCodeOrderNotExist) {
		t.Fatalf("expected withdraw documented codes to include ORDER_NOT_EXIST")
	}
	if !FundManagementWithdrawBillDocumentedCodes.Has(FundManagementCodeStatementCreating) {
		t.Fatalf("expected withdraw bill documented codes to include STATEMENT_CREATING")
	}
}

func TestFundManagementCodeHelpers(t *testing.T) {
	if !FundManagementCodeEquals(" order_not_exist ", FundManagementCodeOrderNotExist) {
		t.Fatalf("expected ORDER_NOT_EXIST comparison to ignore case and whitespace")
	}
	if !FundManagementCodeIn(" statement_creating ", FundManagementCodeNoStatementExist, FundManagementCodeStatementCreating) {
		t.Fatalf("expected STATEMENT_CREATING to match candidate set")
	}
}
