package contracts

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// CancelWithdrawRequestValidationError 注销提现请求本地校验失败（上游调用前）
type CancelWithdrawRequestValidationError struct {
	Message string
}

func (e *CancelWithdrawRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "merchant cancel withdraw: validation failed"
	}
	return e.Message
}

// CancelWithdrawQueryContractError 注销提现响应契约校验失败（上游调用后）
type CancelWithdrawQueryContractError struct {
	Message string
}

func (e *CancelWithdrawQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "merchant cancel withdraw: upstream contract validation failed"
	}
	return e.Message
}

func newCancelWithdrawValidationError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "merchant cancel withdraw"
	}
	return &CancelWithdrawRequestValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newCancelWithdrawContractError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "merchant cancel withdraw"
	}
	return &CancelWithdrawQueryContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

// ValidateCancelWithdrawIdentifier 校验 sub_mchid / out_request_no / applyment_id 等标识符字段。
// 返回 trim 后的值。
func ValidateCancelWithdrawIdentifier(operation string, fieldName string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", newCancelWithdrawValidationError(operation, "%s is required", fieldName)
	}
	if utf8.RuneCountInString(trimmed) > 32 {
		return "", newCancelWithdrawValidationError(operation, "%s must not exceed 32 characters", fieldName)
	}
	return trimmed, nil
}

var allowedCancelWithdrawMerchantStates = map[string]struct{}{
	CancelWithdrawMerchantStateNormal:    {},
	CancelWithdrawMerchantStateCancelled: {},
}

var allowedCancelWithdrawValidateResults = map[string]struct{}{
	CancelWithdrawValidateResultAllow:    {},
	CancelWithdrawValidateResultNotAllow: {},
}

var allowedCancelWithdrawBlockReasonTypes = map[string]struct{}{
	CancelWithdrawBlockReasonTypeConsumerComplaint: {},
	CancelWithdrawBlockReasonTypeBlockingControl:   {},
	CancelWithdrawBlockReasonTypeFundsPending:      {},
	CancelWithdrawBlockReasonTypeOtherReason:       {},
}

var allowedCancelWithdrawModes = map[string]struct{}{
	CancelWithdrawModeNotApply: {},
	CancelWithdrawModeApply:    {},
}

var allowedCancelWithdrawAccountTypes = map[string]struct{}{
	CancelWithdrawAccountTypeCorporate: {},
	CancelWithdrawAccountTypePersonal:  {},
}

var allowedCancelWithdrawIDDocTypes = map[string]struct{}{
	CancelWithdrawIDDocTypeIDCard:                {},
	CancelWithdrawIDDocTypeOverseaPassport:       {},
	CancelWithdrawIDDocTypeHongkongPassport:      {},
	CancelWithdrawIDDocTypeMacaoPassport:         {},
	CancelWithdrawIDDocTypeTaiwanPassport:        {},
	CancelWithdrawIDDocTypeForeignResident:       {},
	CancelWithdrawIDDocTypeHongkongMacaoResident: {},
	CancelWithdrawIDDocTypeTaiwanResident:        {},
}

var allowedCancelWithdrawCancelStates = map[string]struct{}{
	CancelWithdrawCancelStateAccepted:               {},
	CancelWithdrawCancelStateReviewing:              {},
	CancelWithdrawCancelStateRejected:               {},
	CancelWithdrawCancelStateWaitingMerchantConfirm: {},
	CancelWithdrawCancelStateRevoked:                {},
	CancelWithdrawCancelStateSystemProcessing:       {},
	CancelWithdrawCancelStateCanceled:               {},
	CancelWithdrawCancelStateFundProcessing:         {},
	CancelWithdrawCancelStateFinish:                 {},
}

// 仅进入资金处理阶段后才会携带 withdraw_state
var allowedCancelWithdrawCancelStatesWithWithdrawProgress = map[string]struct{}{
	CancelWithdrawCancelStateFundProcessing: {},
	CancelWithdrawCancelStateFinish:         {},
}

var allowedCancelWithdrawWithdrawStates = map[string]struct{}{
	CancelWithdrawWithdrawStateProcessing: {},
	CancelWithdrawWithdrawStateException:  {},
	CancelWithdrawWithdrawStateSucceed:    {},
}

var allowedCancelWithdrawOutAccountTypes = map[string]struct{}{
	CancelWithdrawOutAccountTypeBasic:    {},
	CancelWithdrawOutAccountTypeOperate:  {},
	CancelWithdrawOutAccountTypeMargin:   {},
	CancelWithdrawOutAccountTypeTradeFee: {},
}

var allowedCancelWithdrawPayStates = map[string]struct{}{
	CancelWithdrawPayStateProcessing:   {},
	CancelWithdrawPayStateSucceed:      {},
	CancelWithdrawPayStateFail:         {},
	CancelWithdrawPayStateBankRefunded: {},
}

// ValidateCancelWithdrawCreateRequest 校验提交注销提现申请的请求体。
// 会原地规范化（trim）已校验通过的字段值。
func ValidateCancelWithdrawCreateRequest(req *CancelWithdrawRequest) error {
	if req == nil {
		return newCancelWithdrawValidationError("create merchant cancel withdraw", "request is nil")
	}
	trimmedSubMchID, err := ValidateCancelWithdrawIdentifier("create merchant cancel withdraw", "sub_mchid", req.SubMchID)
	if err != nil {
		return err
	}
	req.SubMchID = trimmedSubMchID
	trimmedOutRequestNo, err := ValidateCancelWithdrawIdentifier("create merchant cancel withdraw", "out_request_no", req.OutRequestNo)
	if err != nil {
		return err
	}
	for _, r := range trimmedOutRequestNo {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return newCancelWithdrawValidationError("create merchant cancel withdraw", "out_request_no must contain only letters and digits")
		}
	}
	req.OutRequestNo = trimmedOutRequestNo
	if req.Withdraw != "" {
		trimmedWithdraw := strings.TrimSpace(req.Withdraw)
		if _, ok := allowedCancelWithdrawModes[trimmedWithdraw]; !ok {
			return newCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported withdraw %q", req.Withdraw)
		}
		req.Withdraw = trimmedWithdraw
	}
	if req.PayeeInfo != nil {
		trimmedAccountType := strings.TrimSpace(req.PayeeInfo.AccountType)
		if trimmedAccountType == "" {
			return newCancelWithdrawValidationError("create merchant cancel withdraw", "payee_info.account_type is required when payee_info is provided")
		}
		if _, ok := allowedCancelWithdrawAccountTypes[trimmedAccountType]; !ok {
			return newCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported payee_info.account_type %q", req.PayeeInfo.AccountType)
		}
		req.PayeeInfo.AccountType = trimmedAccountType
		if req.PayeeInfo.IdentityInfo != nil {
			trimmedDocType := strings.TrimSpace(req.PayeeInfo.IdentityInfo.IDDocType)
			if trimmedDocType != "" {
				if _, ok := allowedCancelWithdrawIDDocTypes[trimmedDocType]; !ok {
					return newCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported payee_info.identity_info.id_doc_type %q", req.PayeeInfo.IdentityInfo.IDDocType)
				}
				req.PayeeInfo.IdentityInfo.IDDocType = trimmedDocType
			}
		}
	}
	if len(req.AdditionalMaterials) > 10 {
		return newCancelWithdrawValidationError("create merchant cancel withdraw", "additional_materials must not exceed 10 items")
	}
	if utf8.RuneCountInString(strings.TrimSpace(req.Remark)) > 32 {
		return newCancelWithdrawValidationError("create merchant cancel withdraw", "remark must not exceed 32 characters")
	}
	req.Remark = strings.TrimSpace(req.Remark)
	return nil
}

// ValidateCancelWithdrawEligibilityResponse 校验注销资格检查的响应体。
func ValidateCancelWithdrawEligibilityResponse(resp *CancelWithdrawEligibilityResponse) error {
	if resp == nil {
		return newCancelWithdrawContractError("validate merchant cancel withdraw", "empty wechat response")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newCancelWithdrawContractError("validate merchant cancel withdraw", "wechat response missing sub_mchid")
	}
	if _, ok := allowedCancelWithdrawMerchantStates[strings.TrimSpace(resp.MerchantState)]; !ok {
		return newCancelWithdrawContractError("validate merchant cancel withdraw", "unsupported merchant_state %q", resp.MerchantState)
	}
	if _, ok := allowedCancelWithdrawValidateResults[strings.TrimSpace(resp.ValidateResult)]; !ok {
		return newCancelWithdrawContractError("validate merchant cancel withdraw", "unsupported validate_result %q", resp.ValidateResult)
	}
	for index, account := range resp.AccountInfo {
		if _, ok := allowedCancelWithdrawOutAccountTypes[strings.TrimSpace(account.OutAccountType)]; !ok {
			return newCancelWithdrawContractError("validate merchant cancel withdraw", "account_info[%d].out_account_type has unsupported value %q", index, account.OutAccountType)
		}
	}
	for index, reason := range resp.BlockReasons {
		trimmedType := strings.TrimSpace(reason.Type)
		if trimmedType == "" {
			continue
		}
		if _, ok := allowedCancelWithdrawBlockReasonTypes[trimmedType]; !ok {
			return newCancelWithdrawContractError("validate merchant cancel withdraw", "block_reasons[%d].type has unsupported value %q", index, reason.Type)
		}
	}
	return nil
}

// ValidateCancelWithdrawQueryResponse 校验查询注销提现申请单的响应体。
func ValidateCancelWithdrawQueryResponse(operation string, resp *CancelWithdrawQueryResponse) error {
	if resp == nil {
		return newCancelWithdrawContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.ApplymentID) == "" {
		return newCancelWithdrawContractError(operation, "wechat response missing applyment_id")
	}
	if strings.TrimSpace(resp.OutRequestNo) == "" {
		return newCancelWithdrawContractError(operation, "wechat response missing out_request_no")
	}
	if _, ok := allowedCancelWithdrawCancelStates[strings.TrimSpace(resp.CancelState)]; !ok {
		return newCancelWithdrawContractError(operation, "unsupported cancel_state %q", resp.CancelState)
	}
	if strings.TrimSpace(resp.CancelStateDescription) == "" {
		return newCancelWithdrawContractError(operation, "wechat response missing cancel_state_description")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newCancelWithdrawContractError(operation, "wechat response missing sub_mchid")
	}
	if trimmedWithdraw := strings.TrimSpace(resp.Withdraw); trimmedWithdraw != "" {
		if _, ok := allowedCancelWithdrawModes[trimmedWithdraw]; !ok {
			return newCancelWithdrawContractError(operation, "unsupported withdraw %q", resp.Withdraw)
		}
	}
	if trimmedWithdrawState := strings.TrimSpace(resp.WithdrawState); trimmedWithdrawState != "" {
		if _, ok := allowedCancelWithdrawWithdrawStates[trimmedWithdrawState]; !ok {
			return newCancelWithdrawContractError(operation, "unsupported withdraw_state %q", resp.WithdrawState)
		}
		if _, ok := allowedCancelWithdrawCancelStatesWithWithdrawProgress[strings.TrimSpace(resp.CancelState)]; !ok {
			return newCancelWithdrawContractError(operation, "withdraw_state is only allowed after the request reaches a withdraw-processing state")
		}
	}
	if strings.TrimSpace(resp.ModifyTime) != "" {
		if _, err := time.Parse(time.RFC3339, resp.ModifyTime); err != nil {
			return newCancelWithdrawContractError(operation, "modify_time must be RFC3339: %v", err)
		}
	}
	for index, account := range resp.AccountInfo {
		if _, ok := allowedCancelWithdrawOutAccountTypes[strings.TrimSpace(account.OutAccountType)]; !ok {
			return newCancelWithdrawContractError(operation, "account_info[%d].out_account_type has unsupported value %q", index, account.OutAccountType)
		}
	}
	for index, result := range resp.AccountWithdrawResult {
		if _, ok := allowedCancelWithdrawOutAccountTypes[strings.TrimSpace(result.OutAccountType)]; !ok {
			return newCancelWithdrawContractError(operation, "account_withdraw_result[%d].out_account_type has unsupported value %q", index, result.OutAccountType)
		}
		if _, ok := allowedCancelWithdrawPayStates[strings.TrimSpace(result.PayState)]; !ok {
			return newCancelWithdrawContractError(operation, "account_withdraw_result[%d].pay_state has unsupported value %q", index, result.PayState)
		}
		if strings.TrimSpace(result.StateDescription) == "" {
			return newCancelWithdrawContractError(operation, "account_withdraw_result[%d].state_description is required", index)
		}
	}
	confirmCancelURL := ""
	if resp.ConfirmCancel != nil {
		confirmCancelURL = strings.TrimSpace(resp.ConfirmCancel.ConfirmCancelURL)
	}
	if confirmCancelURL != "" && strings.TrimSpace(resp.CancelState) != CancelWithdrawCancelStateWaitingMerchantConfirm {
		return newCancelWithdrawContractError(operation, "confirm_cancel.confirm_cancel_url is only allowed when cancel_state=WAITING_MERCHANT_CONFIRM")
	}
	return nil
}
