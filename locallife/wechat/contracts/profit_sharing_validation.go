package contracts

import (
	"fmt"
	"strings"
)

type ProfitSharingValidationError struct {
	Message string
}

type ProfitSharingContractError struct {
	Message string
}

func (e *ProfitSharingValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "profit sharing: validation failed"
	}
	return e.Message
}

func (e *ProfitSharingContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "profit sharing: upstream contract validation failed"
	}
	return e.Message
}

var allowedProfitSharingStatuses = map[string]struct{}{
	ProfitSharingStatusProcessing: {},
	ProfitSharingStatusFinished:   {},
}

var allowedProfitSharingReceiverTypes = map[string]struct{}{
	ReceiverTypeMerchant: {},
	ReceiverTypePersonal: {},
}

var allowedProfitSharingReceiverResults = map[string]struct{}{
	ProfitSharingResultPending: {},
	ProfitSharingResultSuccess: {},
	ProfitSharingResultClosed:  {},
	"FAILED":                   {},
}

var allowedProfitSharingFailReasons = map[string]struct{}{
	ProfitSharingFailReasonAccountAbnormal:             {},
	ProfitSharingFailReasonNoRelation:                  {},
	ProfitSharingFailReasonReceiverHighRisk:            {},
	ProfitSharingFailReasonReceiverRealNameNotVerified: {},
	ProfitSharingFailReasonNoAuth:                      {},
	ProfitSharingFailReasonReceiverReceiptLimit:        {},
	ProfitSharingFailReasonPayerAccountAbnormal:        {},
	ProfitSharingFailReasonInvalidRequest:              {},
}

var allowedProfitSharingAbnormalStatuses = map[string]struct{}{
	ProfitSharingAbnormalStatusPending:  {},
	ProfitSharingAbnormalStatusFinished: {},
	ProfitSharingAbnormalStatusClosed:   {},
}

var allowedProfitSharingAbnormalClosedReasons = map[string]struct{}{
	ProfitSharingAbnormalClosedReasonTimeout:          {},
	ProfitSharingAbnormalClosedReasonRestrictTransfer: {},
}

var allowedProfitSharingReturnResults = map[string]struct{}{
	ProfitSharingReturnResultProcessing: {},
	ProfitSharingReturnResultSuccess:    {},
	ProfitSharingReturnResultFailed:     {},
}

func ValidateProfitSharingRequest(req *ProfitSharingRequest, fallbackAppID string) error {
	if req == nil {
		return newProfitSharingValidationError("create profit sharing", "request is nil")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.TransactionID) == "" || strings.TrimSpace(req.OutOrderNo) == "" {
		return newProfitSharingValidationError("create profit sharing", "sub_mchid, transaction_id and out_order_no are required")
	}
	if len(req.Receivers) == 0 {
		return newProfitSharingValidationError("create profit sharing", "receivers are required")
	}

	seenReceivers := make(map[string]struct{}, len(req.Receivers))
	resolvedAppID := strings.TrimSpace(req.AppID)
	if resolvedAppID == "" {
		resolvedAppID = strings.TrimSpace(fallbackAppID)
	}
	for _, receiver := range req.Receivers {
		receiverType := strings.TrimSpace(receiver.Type)
		receiverAccount := strings.TrimSpace(receiver.ReceiverAccount)
		if receiverType == "" || receiverAccount == "" {
			return newProfitSharingValidationError("create profit sharing", "receiver type and account are required")
		}
		if receiver.Amount <= 0 {
			return newProfitSharingValidationError("create profit sharing", "receiver amount must be positive")
		}
		if strings.TrimSpace(receiver.Description) == "" {
			return newProfitSharingValidationError("create profit sharing", "receiver description is required")
		}
		if receiverType == ReceiverTypePersonal && resolvedAppID == "" {
			return newProfitSharingValidationError("create profit sharing", "appid is required for personal receivers")
		}
		if req.Finish && receiverType == ReceiverTypeMerchant && receiverAccount == strings.TrimSpace(req.SubMchID) {
			return newProfitSharingValidationError("create profit sharing", "finish=true does not allow sub_mchid as receiver")
		}

		receiverKey := receiverType + ":" + receiverAccount
		if _, exists := seenReceivers[receiverKey]; exists {
			return newProfitSharingValidationError("create profit sharing", "duplicate receiver %s", receiverKey)
		}
		seenReceivers[receiverKey] = struct{}{}
	}

	return nil
}

func ValidateAddReceiverRequest(req *AddReceiverRequest) error {
	if req == nil {
		return newProfitSharingValidationError("add profit sharing receiver", "request is nil")
	}
	if strings.TrimSpace(req.Type) == "" || strings.TrimSpace(req.Account) == "" {
		return newProfitSharingValidationError("add profit sharing receiver", "type and account are required")
	}
	if strings.TrimSpace(req.Type) == ReceiverTypePersonal && strings.TrimSpace(req.AppID) == "" {
		return newProfitSharingValidationError("add profit sharing receiver", "appid is required for personal receivers")
	}
	if !isSupportedProfitSharingRelationType(req.RelationType) {
		return newProfitSharingValidationError("add profit sharing receiver", "unsupported relation_type %q", req.RelationType)
	}
	return nil
}

func ValidateDeleteReceiverRequest(req *DeleteReceiverRequest) error {
	if req == nil {
		return newProfitSharingValidationError("delete profit sharing receiver", "request is nil")
	}
	if strings.TrimSpace(req.Type) == "" || strings.TrimSpace(req.Account) == "" {
		return newProfitSharingValidationError("delete profit sharing receiver", "type and account are required")
	}
	if strings.TrimSpace(req.Type) == ReceiverTypePersonal && strings.TrimSpace(req.AppID) == "" {
		return newProfitSharingValidationError("delete profit sharing receiver", "appid is required for personal receivers")
	}
	return nil
}

func ValidateProfitSharingReturnRequest(req *ProfitSharingReturnRequest) error {
	if req == nil {
		return newProfitSharingValidationError("create profit sharing return", "request is nil")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.OutReturnNo) == "" {
		return newProfitSharingValidationError("create profit sharing return", "sub_mchid and out_return_no are required")
	}
	if strings.TrimSpace(req.OrderID) == "" && strings.TrimSpace(req.OutOrderNo) == "" {
		return newProfitSharingValidationError("create profit sharing return", "order_id or out_order_no is required")
	}
	if req.Amount <= 0 {
		return newProfitSharingValidationError("create profit sharing return", "amount must be positive")
	}
	if strings.TrimSpace(req.Description) == "" {
		return newProfitSharingValidationError("create profit sharing return", "description is required")
	}
	if strings.TrimSpace(req.ReturnMchID) == "" {
		return newProfitSharingValidationError("create profit sharing return", "return_mchid is required")
	}
	return nil
}

func ValidateProfitSharingCreateResponse(operation string, resp *ProfitSharingResponse) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resp.OutOrderNo) == "" {
		return newProfitSharingContractError(operation, "wechat response missing out_order_no")
	}
	if strings.TrimSpace(resp.OrderID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing order_id")
	}
	if strings.TrimSpace(resp.Status) == "" {
		return newProfitSharingContractError(operation, "wechat response missing status")
	}
	if _, ok := allowedProfitSharingStatuses[strings.ToUpper(strings.TrimSpace(resp.Status))]; !ok {
		return newProfitSharingContractError(operation, "unsupported status %q", resp.Status)
	}
	return validateProfitSharingReceiverResults(operation, "receivers", resp.Receivers)
}

func ValidateProfitSharingFinishResponse(operation string, resp *ProfitSharingResponse) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.Status) != "" {
		if _, ok := allowedProfitSharingStatuses[strings.ToUpper(strings.TrimSpace(resp.Status))]; !ok {
			return newProfitSharingContractError(operation, "unsupported status %q", resp.Status)
		}
	}
	if strings.TrimSpace(resp.OrderID) == "" && strings.TrimSpace(resp.OutOrderNo) == "" {
		return newProfitSharingContractError(operation, "wechat response missing order_id and out_order_no")
	}
	return validateProfitSharingReceiverResults(operation, "receivers", resp.Receivers)
}

func ValidateProfitSharingAmountsResponse(operation string, resp *ProfitSharingAmountsResponse, fallbackTransactionID string) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.TransactionID) == "" {
		resp.TransactionID = strings.TrimSpace(fallbackTransactionID)
	}
	if strings.TrimSpace(resp.TransactionID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing transaction_id")
	}
	if resp.UnsplitAmount < 0 {
		return newProfitSharingContractError(operation, "unsplit_amount must be non-negative")
	}
	return nil
}

func ValidateProfitSharingQueryResponse(operation string, resp *ProfitSharingQueryResponse) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resp.OutOrderNo) == "" {
		return newProfitSharingContractError(operation, "wechat response missing out_order_no")
	}
	if strings.TrimSpace(resp.OrderID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing order_id")
	}
	if strings.TrimSpace(resp.Status) == "" {
		return newProfitSharingContractError(operation, "wechat response missing status")
	}
	if _, ok := allowedProfitSharingStatuses[strings.ToUpper(strings.TrimSpace(resp.Status))]; !ok {
		return newProfitSharingContractError(operation, "unsupported status %q", resp.Status)
	}
	return validateProfitSharingReceiverResults(operation, "receivers", resp.Receivers)
}

func ValidateAddReceiverResponse(operation string, resp *AddReceiverResponse) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.Type) == "" {
		return newProfitSharingContractError(operation, "wechat response missing type")
	}
	if _, ok := allowedProfitSharingReceiverTypes[strings.ToUpper(strings.TrimSpace(resp.Type))]; !ok {
		return newProfitSharingContractError(operation, "unsupported receiver type %q", resp.Type)
	}
	if strings.TrimSpace(resp.Account) == "" {
		return newProfitSharingContractError(operation, "wechat response missing account")
	}
	return nil
}

func ValidateDeleteReceiverResponse(operation string, resp *DeleteReceiverResponse) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.Type) == "" {
		return newProfitSharingContractError(operation, "wechat response missing type")
	}
	if _, ok := allowedProfitSharingReceiverTypes[strings.ToUpper(strings.TrimSpace(resp.Type))]; !ok {
		return newProfitSharingContractError(operation, "unsupported receiver type %q", resp.Type)
	}
	if strings.TrimSpace(resp.Account) == "" {
		return newProfitSharingContractError(operation, "wechat response missing account")
	}
	return nil
}

func ValidateProfitSharingReturnResponse(operation string, resp *ProfitSharingReturnResponse, fallbackOutReturnNo string) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.OutReturnNo) == "" {
		resp.OutReturnNo = strings.TrimSpace(fallbackOutReturnNo)
	}
	if strings.TrimSpace(resp.OutReturnNo) == "" {
		return newProfitSharingContractError(operation, "wechat response missing out_return_no")
	}
	if strings.TrimSpace(resp.Result) == "" {
		return newProfitSharingContractError(operation, "wechat response missing result")
	}
	if _, ok := allowedProfitSharingReturnResults[strings.ToUpper(strings.TrimSpace(resp.Result))]; !ok {
		return newProfitSharingContractError(operation, "unsupported result %q", resp.Result)
	}
	if resp.Amount < 0 {
		return newProfitSharingContractError(operation, "amount must be non-negative")
	}
	return nil
}

func ValidateProfitSharingNotification(operation string, resp *ProfitSharingNotification) error {
	if resp == nil {
		return newProfitSharingContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.SPMchID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing sp_mchid")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resp.TransactionID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing transaction_id")
	}
	if strings.TrimSpace(resp.OrderID) == "" {
		return newProfitSharingContractError(operation, "wechat response missing order_id")
	}
	if strings.TrimSpace(resp.OutOrderNo) == "" {
		return newProfitSharingContractError(operation, "wechat response missing out_order_no")
	}
	if strings.TrimSpace(resp.Receiver.Type) == "" {
		return newProfitSharingContractError(operation, "wechat response missing receiver.type")
	}
	if _, ok := allowedProfitSharingReceiverTypes[strings.ToUpper(strings.TrimSpace(resp.Receiver.Type))]; !ok {
		return newProfitSharingContractError(operation, "receiver.type has unsupported value %q", resp.Receiver.Type)
	}
	if strings.TrimSpace(resp.Receiver.Account) == "" {
		return newProfitSharingContractError(operation, "wechat response missing receiver.account")
	}
	if resp.Receiver.Amount <= 0 {
		return newProfitSharingContractError(operation, "receiver.amount must be positive")
	}
	if strings.TrimSpace(resp.Receiver.Description) == "" {
		return newProfitSharingContractError(operation, "wechat response missing receiver.description")
	}
	return nil
}

func newProfitSharingValidationError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "profit sharing"
	}
	return &ProfitSharingValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newProfitSharingContractError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "profit sharing"
	}
	return &ProfitSharingContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func validateProfitSharingReceiverResults(operation, field string, receivers []ProfitSharingReceiverResult) error {
	for index, receiver := range receivers {
		if strings.TrimSpace(receiver.Type) == "" {
			return newProfitSharingContractError(operation, "%s[%d].type is required", field, index)
		}
		if _, ok := allowedProfitSharingReceiverTypes[strings.ToUpper(strings.TrimSpace(receiver.Type))]; !ok {
			return newProfitSharingContractError(operation, "%s[%d].type has unsupported value %q", field, index, receiver.Type)
		}
		if strings.TrimSpace(receiver.ReceiverAccount) == "" {
			return newProfitSharingContractError(operation, "%s[%d].receiver_account is required", field, index)
		}
		if strings.TrimSpace(receiver.Result) == "" {
			return newProfitSharingContractError(operation, "%s[%d].result is required", field, index)
		}
		if _, ok := allowedProfitSharingReceiverResults[strings.ToUpper(strings.TrimSpace(receiver.Result))]; !ok {
			return newProfitSharingContractError(operation, "%s[%d].result has unsupported value %q", field, index, receiver.Result)
		}
		if receiver.Amount < 0 {
			return newProfitSharingContractError(operation, "%s[%d].amount must be non-negative", field, index)
		}
		if receiver.FailReason != "" {
			if _, ok := allowedProfitSharingFailReasons[strings.ToUpper(strings.TrimSpace(receiver.FailReason))]; !ok {
				return newProfitSharingContractError(operation, "%s[%d].fail_reason has unsupported value %q", field, index, receiver.FailReason)
			}
		}
		if receiver.AbnormalStatus != "" {
			if _, ok := allowedProfitSharingAbnormalStatuses[strings.ToUpper(strings.TrimSpace(receiver.AbnormalStatus))]; !ok {
				return newProfitSharingContractError(operation, "%s[%d].abnormal_status has unsupported value %q", field, index, receiver.AbnormalStatus)
			}
		}
		if receiver.FundsAbnormalClosedReason != "" {
			if _, ok := allowedProfitSharingAbnormalClosedReasons[strings.ToUpper(strings.TrimSpace(receiver.FundsAbnormalClosedReason))]; !ok {
				return newProfitSharingContractError(operation, "%s[%d].funds_abnormal_closed_reason has unsupported value %q", field, index, receiver.FundsAbnormalClosedReason)
			}
		}
		for abnormalIndex, abnormalReceiver := range receiver.FundsAbnormalReceivers {
			if strings.TrimSpace(abnormalReceiver.MchID) == "" {
				return newProfitSharingContractError(operation, "%s[%d].funds_abnormal_receivers[%d].mchid is required", field, index, abnormalIndex)
			}
		}
	}
	return nil
}

func isSupportedProfitSharingRelationType(relationType string) bool {
	switch strings.TrimSpace(relationType) {
	case RelationServiceProvider, RelationDistributor, RelationSupplier, RelationPlatform, RelationOthers:
		return true
	default:
		return false
	}
}
