package contracts

import (
	"fmt"
	"strings"
)

type SubsidyValidationError struct {
	Message string
}

type SubsidyContractError struct {
	Message string
}

func (e *SubsidyValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "subsidy: validation failed"
	}
	return e.Message
}

func (e *SubsidyContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "subsidy: upstream contract validation failed"
	}
	return e.Message
}

func ValidateSubsidyRequest(req SubsidyRequest) error {
	if req.Amount <= 0 {
		return newSubsidyValidationError("create subsidy", "amount must be positive")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.TransactionID) == "" || strings.TrimSpace(req.OutSubsidyNo) == "" {
		return newSubsidyValidationError("create subsidy", "sub_mchid, transaction_id and out_subsidy_no are required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return newSubsidyValidationError("create subsidy", "description is required")
	}
	return nil
}

func ValidateSubsidyReturnRequest(req SubsidyReturnRequest) error {
	if req.Amount <= 0 {
		return newSubsidyValidationError("return subsidy", "amount must be positive")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.TransactionID) == "" || strings.TrimSpace(req.OutOrderNo) == "" {
		return newSubsidyValidationError("return subsidy", "sub_mchid, transaction_id and out_order_no are required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return newSubsidyValidationError("return subsidy", "description is required")
	}
	for index, source := range req.From {
		if strings.TrimSpace(source.Account) == "" {
			return newSubsidyValidationError("return subsidy", "from[%d].account is required", index)
		}
		if source.Amount <= 0 {
			return newSubsidyValidationError("return subsidy", "from[%d].amount must be positive", index)
		}
	}
	return nil
}

func ValidateSubsidyCancelRequest(req SubsidyCancelRequest) error {
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.TransactionID) == "" {
		return newSubsidyValidationError("cancel subsidy", "sub_mchid and transaction_id are required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return newSubsidyValidationError("cancel subsidy", "description is required")
	}
	return nil
}

func ValidateSubsidyCreateResponse(operation string, resp *SubsidyResponse) error {
	if resp == nil {
		return newSubsidyContractError(operation, "empty wechat response")
	}
	if isEmptySubsidyResponse(resp) {
		return nil
	}
	if strings.TrimSpace(resp.Result) != "" {
		if !isSupportedSubsidyResult(resp.Result) {
			return newSubsidyContractError(operation, "result has unsupported value %q", resp.Result)
		}
	}
	if resp.Amount < 0 {
		return newSubsidyContractError(operation, "amount must be non-negative")
	}
	return nil
}

func ValidateSubsidyReturnResponse(operation string, resp *SubsidyReturnResponse) error {
	if resp == nil {
		return newSubsidyContractError(operation, "empty wechat response")
	}
	if isEmptySubsidyReturnResponse(resp) {
		return nil
	}
	if strings.TrimSpace(resp.Result) != "" {
		if !isSupportedSubsidyResult(resp.Result) {
			return newSubsidyContractError(operation, "result has unsupported value %q", resp.Result)
		}
	}
	if resp.Amount < 0 {
		return newSubsidyContractError(operation, "amount must be non-negative")
	}
	for index, source := range resp.From {
		if strings.TrimSpace(source.Account) == "" {
			return newSubsidyContractError(operation, "from[%d].account is required", index)
		}
		if source.Amount <= 0 {
			return newSubsidyContractError(operation, "from[%d].amount must be positive", index)
		}
	}
	return nil
}

func ValidateSubsidyCancelResponse(operation string, resp *SubsidyCancelResponse) error {
	if resp == nil {
		return newSubsidyContractError(operation, "empty wechat response")
	}
	if isEmptySubsidyCancelResponse(resp) {
		return nil
	}
	if strings.TrimSpace(resp.Result) != "" {
		if !isSupportedSubsidyResult(resp.Result) {
			return newSubsidyContractError(operation, "result has unsupported value %q", resp.Result)
		}
	}
	return nil
}

func newSubsidyValidationError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "subsidy"
	}
	return &SubsidyValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newSubsidyContractError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "subsidy"
	}
	return &SubsidyContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func isSupportedSubsidyResult(result string) bool {
	switch strings.ToUpper(strings.TrimSpace(result)) {
	case SubsidyResultSuccess, SubsidyResultFail, SubsidyResultRefund:
		return true
	default:
		return false
	}
}

func isEmptySubsidyResponse(resp *SubsidyResponse) bool {
	return strings.TrimSpace(resp.SubMchID) == "" &&
		strings.TrimSpace(resp.TransactionID) == "" &&
		strings.TrimSpace(resp.SubsidyID) == "" &&
		strings.TrimSpace(resp.Description) == "" &&
		resp.Amount == 0 &&
		strings.TrimSpace(resp.Result) == "" &&
		strings.TrimSpace(resp.SuccessTime) == "" &&
		strings.TrimSpace(resp.OutSubsidyNo) == ""
}

func isEmptySubsidyReturnResponse(resp *SubsidyReturnResponse) bool {
	return strings.TrimSpace(resp.SubMchID) == "" &&
		strings.TrimSpace(resp.TransactionID) == "" &&
		strings.TrimSpace(resp.SubsidyRefundID) == "" &&
		strings.TrimSpace(resp.RefundID) == "" &&
		strings.TrimSpace(resp.OutOrderNo) == "" &&
		resp.Amount == 0 &&
		strings.TrimSpace(resp.Description) == "" &&
		strings.TrimSpace(resp.Result) == "" &&
		strings.TrimSpace(resp.SuccessTime) == "" &&
		strings.TrimSpace(resp.SubsidyID) == "" &&
		len(resp.From) == 0
}

func isEmptySubsidyCancelResponse(resp *SubsidyCancelResponse) bool {
	return strings.TrimSpace(resp.SubMchID) == "" &&
		strings.TrimSpace(resp.TransactionID) == "" &&
		strings.TrimSpace(resp.Result) == "" &&
		strings.TrimSpace(resp.Description) == ""
}
