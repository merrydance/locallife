package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSubsidyRequest_RequiresDescription(t *testing.T) {
	err := ValidateSubsidyRequest(SubsidyRequest{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		OutSubsidyNo:  "subsidy-1",
		Amount:        1,
	})
	require.EqualError(t, err, "create subsidy: description is required")
}

func TestValidateSubsidyReturnRequest_ValidatesFromEntries(t *testing.T) {
	err := ValidateSubsidyReturnRequest(SubsidyReturnRequest{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		OutOrderNo:    "order-1",
		Amount:        1,
		Description:   "desc",
		From:          []SubsidyReturnFrom{{Amount: 1}},
	})
	require.EqualError(t, err, "return subsidy: from[0].account is required")
}

func TestValidateSubsidyCancelRequest_PassesForValidRequest(t *testing.T) {
	err := ValidateSubsidyCancelRequest(SubsidyCancelRequest{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		Description:   "cancel",
	})
	require.NoError(t, err)
}

func TestValidateSubsidyCreateResponse_AllowsEmptyBody(t *testing.T) {
	err := ValidateSubsidyCreateResponse("create subsidy", &SubsidyResponse{})
	require.NoError(t, err)
}

func TestValidateSubsidyCreateResponse_RejectsUnknownResult(t *testing.T) {
	err := ValidateSubsidyCreateResponse("create subsidy", &SubsidyResponse{Result: "UNKNOWN"})
	require.EqualError(t, err, "create subsidy: result has unsupported value \"UNKNOWN\"")
}

func TestValidateSubsidyReturnResponse_RejectsInvalidFromEntry(t *testing.T) {
	err := ValidateSubsidyReturnResponse("return subsidy", &SubsidyReturnResponse{
		Result: SubsidyResultSuccess,
		From:   []SubsidyReturnFrom{{Amount: 1}},
	})
	require.EqualError(t, err, "return subsidy: from[0].account is required")
}

func TestValidateSubsidyCancelResponse_RejectsUnknownResult(t *testing.T) {
	err := ValidateSubsidyCancelResponse("cancel subsidy", &SubsidyCancelResponse{Result: "UNKNOWN"})
	require.EqualError(t, err, "cancel subsidy: result has unsupported value \"UNKNOWN\"")
}
