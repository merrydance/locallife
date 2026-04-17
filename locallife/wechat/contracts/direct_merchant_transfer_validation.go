package contracts

import (
	"fmt"
	"strings"
)

type DirectMerchantTransferCreateRequestValidationError struct {
	Message string
}

func (e *DirectMerchantTransferCreateRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "create direct merchant transfer: validation failed"
	}
	return e.Message
}

type DirectMerchantTransferQueryValidationError struct {
	Message string
}

func (e *DirectMerchantTransferQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query direct merchant transfer: validation failed"
	}
	return e.Message
}

type DirectMerchantTransferContractError struct {
	Message string
}

func (e *DirectMerchantTransferContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "direct merchant transfer: upstream contract validation failed"
	}
	return e.Message
}

type DirectMerchantTransferNotificationContractError struct {
	Message string
}

func (e *DirectMerchantTransferNotificationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "decrypt direct merchant transfer notification: upstream contract validation failed"
	}
	return e.Message
}

var allowedDirectMerchantTransferStates = map[string]struct{}{
	DirectMerchantTransferStateAccepted:        {},
	DirectMerchantTransferStateProcessing:      {},
	DirectMerchantTransferStateWaitUserConfirm: {},
	DirectMerchantTransferStateTransfering:     {},
	DirectMerchantTransferStateSuccess:         {},
	DirectMerchantTransferStateFail:            {},
	DirectMerchantTransferStateCanceling:       {},
	DirectMerchantTransferStateCancelled:       {},
}

var allowedDirectMerchantTransferTerminalNotificationStates = map[string]struct{}{
	DirectMerchantTransferStateSuccess:   {},
	DirectMerchantTransferStateFail:      {},
	DirectMerchantTransferStateCancelled: {},
}

var allowedDirectMerchantTransferUserRecvPerceptions = map[string]struct{}{
	DirectMerchantTransferUserRecvPerceptionRefund:               {},
	DirectMerchantTransferUserRecvPerceptionMerchantCompensation: {},
}

var allowedDirectMerchantTransferPaymentMethodTypes = map[string]struct{}{
	DirectMerchantTransferPaymentMethodTypeWallet:   {},
	DirectMerchantTransferPaymentMethodTypeHKWallet: {},
}

func ValidateDirectMerchantTransferCreateRequest(req *DirectMerchantTransferCreateRequest) error {
	if req == nil {
		return newDirectMerchantTransferCreateRequestValidationError("request is nil")
	}
	if strings.TrimSpace(req.AppID) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("appid is required")
	}
	if strings.TrimSpace(req.OutBillNo) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("out_bill_no is required")
	}
	if strings.TrimSpace(req.TransferSceneID) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_scene_id is required")
	}
	if strings.TrimSpace(req.TransferSceneID) != DirectMerchantTransferSceneEnterpriseCompensation {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_scene_id must be %q", DirectMerchantTransferSceneEnterpriseCompensation)
	}
	if strings.TrimSpace(req.OpenID) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("openid is required")
	}
	if req.TransferAmount <= 0 {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_amount must be positive")
	}
	if strings.TrimSpace(req.TransferRemark) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_remark is required")
	}
	if req.TransferAmount < 30 && strings.TrimSpace(req.UserName) != "" {
		return newDirectMerchantTransferCreateRequestValidationError("user_name is not supported when transfer_amount is below 30 fen")
	}
	if req.TransferAmount >= 200000 && strings.TrimSpace(req.UserName) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("user_name is required when transfer_amount is at least 200000 fen")
	}
	if strings.TrimSpace(req.UserRecvPerception) != "" {
		if _, ok := allowedDirectMerchantTransferUserRecvPerceptions[strings.TrimSpace(req.UserRecvPerception)]; !ok {
			return newDirectMerchantTransferCreateRequestValidationError("user_recv_perception has unsupported value %q", req.UserRecvPerception)
		}
	}
	if len(req.TransferSceneReportInfos) != 1 {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_scene_report_infos must contain exactly one item for enterprise compensation")
	}
	report := req.TransferSceneReportInfos[0]
	if strings.TrimSpace(report.InfoType) != DirectMerchantTransferReportInfoTypeCompensationReason {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_scene_report_infos[0].info_type must be %q", DirectMerchantTransferReportInfoTypeCompensationReason)
	}
	if strings.TrimSpace(report.InfoContent) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("transfer_scene_report_infos[0].info_content is required")
	}
	return nil
}

func ValidateRequestMerchantTransferParams(params *RequestMerchantTransferParams) error {
	if params == nil {
		return newDirectMerchantTransferCreateRequestValidationError("requestMerchantTransfer params are nil")
	}
	if strings.TrimSpace(params.MchID) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("requestMerchantTransfer.mchId is required")
	}
	if strings.TrimSpace(params.AppID) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("requestMerchantTransfer.appId is required")
	}
	if strings.TrimSpace(params.Package) == "" {
		return newDirectMerchantTransferCreateRequestValidationError("requestMerchantTransfer.package is required")
	}
	return nil
}

func ValidateDirectMerchantTransferCreateResponse(operation string, resp *DirectMerchantTransferCreateResponse) error {
	if resp == nil {
		return NewDirectMerchantTransferContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.OutBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing out_bill_no")
	}
	if strings.TrimSpace(resp.TransferBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing transfer_bill_no")
	}
	if err := validateRFC3339Timestamp("create_time", resp.CreateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if strings.TrimSpace(resp.State) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing state")
	}
	if _, ok := allowedDirectMerchantTransferStates[strings.ToUpper(strings.TrimSpace(resp.State))]; !ok {
		return NewDirectMerchantTransferContractError(operation, "unsupported state %q", resp.State)
	}
	if strings.EqualFold(strings.TrimSpace(resp.State), DirectMerchantTransferStateWaitUserConfirm) {
		if strings.TrimSpace(resp.PackageInfo) == "" {
			return NewDirectMerchantTransferContractError(operation, "package_info is required when state=%s", DirectMerchantTransferStateWaitUserConfirm)
		}
	} else if strings.TrimSpace(resp.PackageInfo) != "" {
		return NewDirectMerchantTransferContractError(operation, "package_info is only allowed when state=%s", DirectMerchantTransferStateWaitUserConfirm)
	}
	return nil
}

func ValidateDirectMerchantTransferQueryByOutBillNoInput(outBillNo string) (string, error) {
	trimmed := strings.TrimSpace(outBillNo)
	if trimmed == "" {
		return "", NewDirectMerchantTransferQueryValidationError("query direct merchant transfer by out_bill_no", "out_bill_no is required")
	}
	return trimmed, nil
}

func ValidateDirectMerchantTransferQueryByTransferBillNoInput(transferBillNo string) (string, error) {
	trimmed := strings.TrimSpace(transferBillNo)
	if trimmed == "" {
		return "", NewDirectMerchantTransferQueryValidationError("query direct merchant transfer by transfer_bill_no", "transfer_bill_no is required")
	}
	return trimmed, nil
}

func ValidateDirectMerchantTransferQueryResponse(operation string, resp *DirectMerchantTransferQueryResponse) error {
	if resp == nil {
		return NewDirectMerchantTransferContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.MchID) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing mch_id")
	}
	if strings.TrimSpace(resp.OutBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing out_bill_no")
	}
	if strings.TrimSpace(resp.TransferBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing transfer_bill_no")
	}
	if strings.TrimSpace(resp.AppID) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing appid")
	}
	if strings.TrimSpace(resp.State) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing state")
	}
	if _, ok := allowedDirectMerchantTransferStates[strings.ToUpper(strings.TrimSpace(resp.State))]; !ok {
		return NewDirectMerchantTransferContractError(operation, "unsupported state %q", resp.State)
	}
	if resp.TransferAmount <= 0 {
		return NewDirectMerchantTransferContractError(operation, "transfer_amount must be positive")
	}
	if strings.TrimSpace(resp.TransferRemark) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing transfer_remark")
	}
	if err := validateRFC3339Timestamp("create_time", resp.CreateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if err := validateRFC3339Timestamp("update_time", resp.UpdateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	return nil
}

func ValidateDirectMerchantTransferCancelResponse(operation string, resp *DirectMerchantTransferCancelResponse) error {
	if resp == nil {
		return NewDirectMerchantTransferContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.OutBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing out_bill_no")
	}
	if strings.TrimSpace(resp.TransferBillNo) == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing transfer_bill_no")
	}
	state := strings.ToUpper(strings.TrimSpace(resp.State))
	if state == "" {
		return NewDirectMerchantTransferContractError(operation, "wechat response missing state")
	}
	if state != DirectMerchantTransferStateCanceling && state != DirectMerchantTransferStateCancelled {
		return NewDirectMerchantTransferContractError(operation, "cancel response state has unsupported value %q", resp.State)
	}
	if err := validateRFC3339Timestamp("update_time", resp.UpdateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	return nil
}

func ValidateDirectMerchantTransferNotificationResource(operation string, resource *DirectMerchantTransferNotificationResource) error {
	if resource == nil {
		return NewDirectMerchantTransferNotificationContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resource.OutBillNo) == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "wechat response missing out_bill_no")
	}
	if strings.TrimSpace(resource.TransferBillNo) == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "wechat response missing transfer_bill_no")
	}
	state := strings.ToUpper(strings.TrimSpace(resource.State))
	if state == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "wechat response missing state")
	}
	if _, ok := allowedDirectMerchantTransferTerminalNotificationStates[state]; !ok {
		return NewDirectMerchantTransferNotificationContractError(operation, "notification state has unsupported value %q", resource.State)
	}
	if strings.TrimSpace(resource.MchID) == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "wechat response missing mch_id")
	}
	if resource.TransferAmount <= 0 {
		return NewDirectMerchantTransferNotificationContractError(operation, "transfer_amount must be positive")
	}
	if strings.TrimSpace(resource.OpenID) == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "wechat response missing openid")
	}
	if err := validateRFC3339Timestamp("create_time", resource.CreateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferNotificationContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if err := validateRFC3339Timestamp("update_time", resource.UpdateTime, func(format string, args ...any) error {
		return NewDirectMerchantTransferNotificationContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if strings.TrimSpace(resource.PaymentMethodType) != "" {
		if _, ok := allowedDirectMerchantTransferPaymentMethodTypes[strings.ToUpper(strings.TrimSpace(resource.PaymentMethodType))]; !ok {
			return NewDirectMerchantTransferNotificationContractError(operation, "payment_method_type has unsupported value %q", resource.PaymentMethodType)
		}
	}
	if state == DirectMerchantTransferStateSuccess && strings.TrimSpace(resource.PaymentMethodType) == "" {
		return NewDirectMerchantTransferNotificationContractError(operation, "payment_method_type is required when state=%s", DirectMerchantTransferStateSuccess)
	}
	return nil
}

func NewDirectMerchantTransferQueryValidationError(operation string, format string, args ...any) error {
	return &DirectMerchantTransferQueryValidationError{Message: formatDirectMerchantTransferValidationMessage("query direct merchant transfer", operation, format, args...)}
}

func NewDirectMerchantTransferContractError(operation string, format string, args ...any) error {
	return &DirectMerchantTransferContractError{Message: formatDirectMerchantTransferValidationMessage("direct merchant transfer", operation, format, args...)}
}

func NewDirectMerchantTransferNotificationContractError(operation string, format string, args ...any) error {
	return &DirectMerchantTransferNotificationContractError{Message: formatDirectMerchantTransferValidationMessage("decrypt direct merchant transfer notification", operation, format, args...)}
}

func newDirectMerchantTransferCreateRequestValidationError(format string, args ...any) error {
	return &DirectMerchantTransferCreateRequestValidationError{Message: fmt.Sprintf("create direct merchant transfer: "+format, args...)}
}

func formatDirectMerchantTransferValidationMessage(defaultOperation, operation, format string, args ...any) string {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = defaultOperation
	}
	return fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))
}
