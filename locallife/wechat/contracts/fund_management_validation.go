package contracts

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	fundManagementMchIDMaxLength        = 32
	fundManagementOutRequestNoMaxLength = 128
	fundManagementWithdrawIDMaxLength   = 128
	fundManagementRemarkMaxLength       = 32
	fundManagementBankMemoMaxLength     = 32
	fundManagementNotifyURLMaxLength    = 256
	fundManagementBillDateLength        = 10
)

type FundManagementValidationError struct {
	Message string
}

func (e *FundManagementValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "fund management: validation failed"
	}
	return e.Message
}

type FundManagementContractError struct {
	Message string
}

func (e *FundManagementContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "fund management: upstream contract validation failed"
	}
	return e.Message
}

var allowedFundManagementRealtimeAccountTypes = map[string]struct{}{
	FundManagementAccountTypeBasic:     {},
	FundManagementAccountTypeFees:      {},
	FundManagementAccountTypeOperation: {},
	FundManagementAccountTypeDeposit:   {},
}

var allowedFundManagementDayEndAccountTypes = map[string]struct{}{
	FundManagementAccountTypeBasic:   {},
	FundManagementAccountTypeDeposit: {},
}

var allowedFundManagementPlatformAccountTypes = map[string]struct{}{
	FundManagementAccountTypeBasic:     {},
	FundManagementAccountTypeFees:      {},
	FundManagementAccountTypeOperation: {},
}

var allowedFundManagementWithdrawAccountTypes = map[string]struct{}{
	FundManagementAccountTypeBasic:     {},
	FundManagementAccountTypeFees:      {},
	FundManagementAccountTypeOperation: {},
}

var allowedFundManagementWithdrawStatuses = map[string]struct{}{
	FundManagementWithdrawStatusCreateSuccess: {},
	FundManagementWithdrawStatusSuccess:       {},
	FundManagementWithdrawStatusFail:          {},
	FundManagementWithdrawStatusRefund:        {},
	FundManagementWithdrawStatusClose:         {},
	FundManagementWithdrawStatusInit:          {},
}

var allowedFundManagementDayEndWithdrawStatuses = map[string]struct{}{
	FundManagementDayEndWithdrawStatusCreated:    {},
	FundManagementDayEndWithdrawStatusProcessing: {},
	FundManagementDayEndWithdrawStatusFinished:   {},
	FundManagementDayEndWithdrawStatusAbnormal:   {},
}

var allowedFundManagementCalculateAmountTypes = map[string]struct{}{
	FundManagementCalculateAmountTypeOnlyDayEndBalance:   {},
	FundManagementCalculateAmountTypeAllowCurrentBalance: {},
}

var allowedFundManagementBillTypes = map[string]struct{}{
	FundManagementBillTypeNoSucc: {},
}

var allowedFundManagementTarTypes = map[string]struct{}{
	FundManagementTarTypeGzip: {},
}

var allowedFundManagementHashTypes = map[string]struct{}{
	FundManagementHashTypeSHA1: {},
}

func newFundManagementValidationError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "fund management"
	}
	return &FundManagementValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newFundManagementContractError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "fund management"
	}
	return &FundManagementContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func ValidateFundManagementSubMchID(operation, subMchID string) (string, error) {
	trimmed := strings.TrimSpace(subMchID)
	if trimmed == "" {
		return "", newFundManagementValidationError(operation, "sub_mchid is required")
	}
	if len(trimmed) > fundManagementMchIDMaxLength {
		return "", newFundManagementValidationError(operation, "sub_mchid must not exceed %d characters", fundManagementMchIDMaxLength)
	}
	return trimmed, nil
}

func ValidateFundManagementAccountType(operation, fieldName, accountType string, allowed map[string]struct{}, required bool) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(accountType))
	if trimmed == "" {
		if required {
			return "", newFundManagementValidationError(operation, "%s is required", fieldName)
		}
		return "", nil
	}
	if _, ok := allowed[trimmed]; !ok {
		return "", newFundManagementValidationError(operation, "unsupported %s %q", fieldName, accountType)
	}
	return trimmed, nil
}

func NormalizeFundManagementSubMerchantRealtimeAccountType(accountType string) (string, error) {
	trimmedAccountType, err := ValidateFundManagementAccountType("normalize ecommerce fund balance account type", "account_type", accountType, allowedFundManagementRealtimeAccountTypes, false)
	if err != nil {
		return "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedAccountType, nil
}

func NormalizeFundManagementSubMerchantDayEndAccountType(accountType string) (string, error) {
	trimmedAccountType, err := ValidateFundManagementAccountType("normalize ecommerce fund day end balance account type", "account_type", accountType, allowedFundManagementDayEndAccountTypes, false)
	if err != nil {
		return "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedAccountType, nil
}

func NormalizeFundManagementPlatformAccountType(accountType string) (string, error) {
	trimmedAccountType, err := ValidateFundManagementAccountType("normalize platform fund balance account type", "account_type", accountType, allowedFundManagementPlatformAccountTypes, false)
	if err != nil {
		return "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedAccountType, nil
}

func NormalizeFundManagementTarType(tarType string) (string, error) {
	trimmedTarType := strings.ToUpper(strings.TrimSpace(tarType))
	if trimmedTarType == "" {
		return FundManagementTarTypeGzip, nil
	}
	if _, ok := allowedFundManagementTarTypes[trimmedTarType]; !ok {
		return "", newFundManagementValidationError("normalize fund management tar type", "unsupported tar_type %q", tarType)
	}
	return trimmedTarType, nil
}

func ValidateEcommerceFundBalanceQueryInput(subMchID, accountType string) (string, string, error) {
	trimmedSubMchID, err := ValidateFundManagementSubMchID("query ecommerce fund balance", subMchID)
	if err != nil {
		return "", "", err
	}
	trimmedAccountType, err := ValidateFundManagementAccountType("query ecommerce fund balance", "account_type", accountType, allowedFundManagementRealtimeAccountTypes, false)
	if err != nil {
		return "", "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedSubMchID, trimmedAccountType, nil
}

func ValidateEcommerceFundDayEndBalanceQueryInput(subMchID, date, accountType string) (string, string, string, error) {
	trimmedSubMchID, err := ValidateFundManagementSubMchID("query ecommerce fund day end balance", subMchID)
	if err != nil {
		return "", "", "", err
	}
	trimmedDate := strings.TrimSpace(date)
	if trimmedDate == "" {
		return "", "", "", newFundManagementValidationError("query ecommerce fund day end balance", "date is required")
	}
	if err := validateDateYYYYMMDD("query ecommerce fund day end balance", "date", trimmedDate); err != nil {
		return "", "", "", err
	}
	trimmedAccountType, err := ValidateFundManagementAccountType("query ecommerce fund day end balance", "account_type", accountType, allowedFundManagementDayEndAccountTypes, false)
	if err != nil {
		return "", "", "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedSubMchID, trimmedDate, trimmedAccountType, nil
}

func ValidatePlatformFundBalanceQueryInput(accountType string) (string, error) {
	trimmedAccountType, err := ValidateFundManagementAccountType("query platform fund balance", "account_type", accountType, allowedFundManagementPlatformAccountTypes, false)
	if err != nil {
		return "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedAccountType, nil
}

func ValidatePlatformFundDayEndBalanceQueryInput(accountType, date string) (string, string, error) {
	trimmedDate := strings.TrimSpace(date)
	if trimmedDate == "" {
		return "", "", newFundManagementValidationError("query platform fund day end balance", "date is required")
	}
	if err := validateDateYYYYMMDD("query platform fund day end balance", "date", trimmedDate); err != nil {
		return "", "", err
	}
	trimmedAccountType, err := ValidateFundManagementAccountType("query platform fund day end balance", "account_type", accountType, allowedFundManagementPlatformAccountTypes, false)
	if err != nil {
		return "", "", err
	}
	if trimmedAccountType == "" {
		trimmedAccountType = FundManagementAccountTypeBasic
	}
	return trimmedAccountType, trimmedDate, nil
}

func ValidateEcommerceFundBalanceResponse(operation string, resp *EcommerceFundBalanceResponse, expectedSubMchID, expectedAccountType string) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "sub_mchid", resp.SubMchID, fundManagementMchIDMaxLength); err != nil {
		return err
	}
	if expectedSubMchID != "" && strings.TrimSpace(resp.SubMchID) != strings.TrimSpace(expectedSubMchID) {
		return newFundManagementContractError(operation, "wechat response sub_mchid %q does not match request %q", resp.SubMchID, expectedSubMchID)
	}
	if err := validateFundManagementContractEnum(operation, "account_type", resp.AccountType, allowedFundManagementRealtimeAccountTypes, true); err != nil {
		return err
	}
	if expectedAccountType != "" && strings.TrimSpace(resp.AccountType) != strings.TrimSpace(expectedAccountType) {
		return newFundManagementContractError(operation, "wechat response account_type %q does not match request %q", resp.AccountType, expectedAccountType)
	}
	if resp.AvailableAmount < 0 {
		return newFundManagementContractError(operation, "available_amount must be non-negative")
	}
	if resp.PendingAmount < 0 {
		return newFundManagementContractError(operation, "pending_amount must be non-negative")
	}
	return nil
}

func ValidatePlatformFundBalanceResponse(operation string, resp *PlatformFundBalanceResponse) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if resp.AvailableAmount < 0 {
		return newFundManagementContractError(operation, "available_amount must be non-negative")
	}
	if resp.PendingAmount < 0 {
		return newFundManagementContractError(operation, "pending_amount must be non-negative")
	}
	return nil
}

func ValidateEcommerceWithdrawRequest(req *EcommerceWithdrawRequest) error {
	if req == nil {
		return newFundManagementValidationError("create ecommerce withdraw", "request is nil")
	}
	trimmedSubMchID, err := ValidateFundManagementSubMchID("create ecommerce withdraw", req.SubMchID)
	if err != nil {
		return err
	}
	req.SubMchID = trimmedSubMchID
	trimmedOutRequestNo, err := validateFundManagementOutRequestNo("create ecommerce withdraw", req.OutRequestNo)
	if err != nil {
		return err
	}
	req.OutRequestNo = trimmedOutRequestNo
	if req.Amount <= 0 {
		return newFundManagementValidationError("create ecommerce withdraw", "amount must be positive")
	}
	trimmedRemark, err := validateFundManagementOptionalString("create ecommerce withdraw", "remark", req.Remark, fundManagementRemarkMaxLength)
	if err != nil {
		return err
	}
	req.Remark = trimmedRemark
	trimmedBankMemo, err := validateFundManagementOptionalString("create ecommerce withdraw", "bank_memo", req.BankMemo, fundManagementBankMemoMaxLength)
	if err != nil {
		return err
	}
	req.BankMemo = trimmedBankMemo
	trimmedAccountType, err := ValidateFundManagementAccountType("create ecommerce withdraw", "account_type", req.AccountType, allowedFundManagementWithdrawAccountTypes, false)
	if err != nil {
		return err
	}
	req.AccountType = trimmedAccountType
	trimmedNotifyURL, err := validateFundManagementNotifyURL("create ecommerce withdraw", req.NotifyURL)
	if err != nil {
		return err
	}
	req.NotifyURL = trimmedNotifyURL
	return nil
}

func ValidateEcommerceWithdrawCreateResponse(operation string, resp *EcommerceWithdrawCreateResponse, expectedSubMchID, expectedOutRequestNo string) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "sub_mchid", resp.SubMchID, fundManagementMchIDMaxLength); err != nil {
		return err
	}
	if expectedSubMchID != "" && strings.TrimSpace(resp.SubMchID) != strings.TrimSpace(expectedSubMchID) {
		return newFundManagementContractError(operation, "wechat response sub_mchid %q does not match request %q", resp.SubMchID, expectedSubMchID)
	}
	if err := validateRequiredContractString(operation, "withdraw_id", resp.WithdrawID, fundManagementWithdrawIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_request_no", resp.OutRequestNo, fundManagementOutRequestNoMaxLength); err != nil {
		return err
	}
	if expectedOutRequestNo != "" && strings.TrimSpace(resp.OutRequestNo) != strings.TrimSpace(expectedOutRequestNo) {
		return newFundManagementContractError(operation, "wechat response out_request_no %q does not match request %q", resp.OutRequestNo, expectedOutRequestNo)
	}
	return nil
}

func ValidateEcommerceWithdrawQueryInput(subMchID, outRequestNo string) (string, string, error) {
	trimmedSubMchID, err := ValidateFundManagementSubMchID("query ecommerce withdraw by out_request_no", subMchID)
	if err != nil {
		return "", "", err
	}
	trimmedOutRequestNo, err := validateFundManagementOutRequestNo("query ecommerce withdraw by out_request_no", outRequestNo)
	if err != nil {
		return "", "", err
	}
	return trimmedSubMchID, trimmedOutRequestNo, nil
}

func ValidateEcommerceWithdrawQueryResponse(operation string, resp *EcommerceWithdrawQueryResponse, expectedSubMchID, expectedOutRequestNo string) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "sp_mchid", resp.SPMchID, fundManagementMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "sub_mchid", resp.SubMchID, fundManagementMchIDMaxLength); err != nil {
		return err
	}
	if expectedSubMchID != "" && strings.TrimSpace(resp.SubMchID) != strings.TrimSpace(expectedSubMchID) {
		return newFundManagementContractError(operation, "wechat response sub_mchid %q does not match request %q", resp.SubMchID, expectedSubMchID)
	}
	if err := validateFundManagementContractEnum(operation, "status", resp.Status, allowedFundManagementWithdrawStatuses, true); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "withdraw_id", resp.WithdrawID, fundManagementWithdrawIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_request_no", resp.OutRequestNo, fundManagementOutRequestNoMaxLength); err != nil {
		return err
	}
	if expectedOutRequestNo != "" && strings.TrimSpace(resp.OutRequestNo) != strings.TrimSpace(expectedOutRequestNo) {
		return newFundManagementContractError(operation, "wechat response out_request_no %q does not match request %q", resp.OutRequestNo, expectedOutRequestNo)
	}
	if resp.Amount <= 0 {
		return newFundManagementContractError(operation, "amount must be positive")
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "update_time", resp.UpdateTime, true); err != nil {
		return err
	}
	if err := validateFundManagementContractEnum(operation, "account_type", resp.AccountType, allowedFundManagementWithdrawAccountTypes, true); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "account_number", resp.AccountNumber, 0); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "account_bank", resp.AccountBank, 0); err != nil {
		return err
	}
	return nil
}

func ValidateWithdrawNotificationResource(operation string, resp *WithdrawNotificationResource) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.Status) == "" {
		return newFundManagementContractError(operation, "wechat response missing status")
	}
	if _, ok := allowedFundManagementWithdrawStatuses[strings.TrimSpace(resp.Status)]; !ok {
		if _, ok := allowedFundManagementDayEndWithdrawStatuses[strings.TrimSpace(resp.Status)]; !ok {
			return newFundManagementContractError(operation, "unsupported status %q", resp.Status)
		}
	}
	if err := validateRequiredContractString(operation, "withdraw_id", resp.WithdrawID, fundManagementWithdrawIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_request_no", resp.OutRequestNo, fundManagementOutRequestNoMaxLength); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "update_time", resp.UpdateTime, true); err != nil {
		return err
	}
	if err := validateFundManagementContractEnum(operation, "account_type", resp.AccountType, allowedFundManagementRealtimeAccountTypes, true); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "account_number", resp.AccountNumber, 0); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "account_bank", resp.AccountBank, 0); err != nil {
		return err
	}
	return nil
}

func ValidateWithdrawBillRequest(req *WithdrawBillRequest) error {
	if req == nil {
		return newFundManagementValidationError("get withdraw bill", "request is nil")
	}
	trimmedBillType, err := validateFundManagementRequiredEnum("get withdraw bill", "bill_type", req.BillType, allowedFundManagementBillTypes)
	if err != nil {
		return err
	}
	req.BillType = trimmedBillType
	trimmedDate := strings.TrimSpace(req.BillDate)
	if trimmedDate == "" {
		return newFundManagementValidationError("get withdraw bill", "bill_date is required")
	}
	if err := validateDateYYYYMMDD("get withdraw bill", "bill_date", trimmedDate); err != nil {
		return err
	}
	req.BillDate = trimmedDate
	trimmedTarType, err := ValidateFundManagementAccountType("get withdraw bill", "tar_type", req.TarType, allowedFundManagementTarTypes, false)
	if err != nil {
		return err
	}
	req.TarType = trimmedTarType
	return nil
}

func ValidateWithdrawBillResponse(operation string, resp *WithdrawBillResponse) error {
	if resp == nil {
		return newFundManagementContractError(operation, "empty wechat response")
	}
	if err := validateFundManagementContractEnum(operation, "hash_type", resp.HashType, allowedFundManagementHashTypes, true); err != nil {
		return err
	}
	if strings.TrimSpace(resp.HashValue) == "" {
		return newFundManagementContractError(operation, "wechat response missing hash_value")
	}
	if strings.TrimSpace(resp.DownloadURL) == "" {
		return newFundManagementContractError(operation, "wechat response missing download_url")
	}
	if _, err := url.ParseRequestURI(strings.TrimSpace(resp.DownloadURL)); err != nil {
		return newFundManagementContractError(operation, "download_url must be a valid absolute URL: %v", err)
	}
	return nil
}

func validateFundManagementOutRequestNo(operation, outRequestNo string) (string, error) {
	trimmed := strings.TrimSpace(outRequestNo)
	if trimmed == "" {
		return "", newFundManagementValidationError(operation, "out_request_no is required")
	}
	if len(trimmed) > fundManagementOutRequestNoMaxLength {
		return "", newFundManagementValidationError(operation, "out_request_no must not exceed %d characters", fundManagementOutRequestNoMaxLength)
	}
	return trimmed, nil
}

func validateFundManagementOptionalString(operation, fieldName, value string, maxLength int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if maxLength > 0 && len(trimmed) > maxLength {
		return "", newFundManagementValidationError(operation, "%s must not exceed %d characters", fieldName, maxLength)
	}
	return trimmed, nil
}

func validateFundManagementNotifyURL(operation, value string) (string, error) {
	trimmed, err := validateFundManagementOptionalString(operation, "notify_url", value, fundManagementNotifyURLMaxLength)
	if err != nil {
		return "", err
	}
	if trimmed == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return "", newFundManagementValidationError(operation, "notify_url must be a valid absolute URL: %v", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", newFundManagementValidationError(operation, "notify_url must use https")
	}
	return trimmed, nil
}

func validateFundManagementRequiredEnum(operation, fieldName, value string, allowed map[string]struct{}) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "", newFundManagementValidationError(operation, "%s is required", fieldName)
	}
	if _, ok := allowed[trimmed]; !ok {
		return "", newFundManagementValidationError(operation, "unsupported %s %q", fieldName, value)
	}
	return trimmed, nil
}

func validateFundManagementContractEnum(operation, fieldName, value string, allowed map[string]struct{}, required bool) error {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		if required {
			return newFundManagementContractError(operation, "wechat response missing %s", fieldName)
		}
		return nil
	}
	if _, ok := allowed[trimmed]; !ok {
		return newFundManagementContractError(operation, "%s has unsupported value %q", fieldName, value)
	}
	return nil
}

func validateDateYYYYMMDD(operation, fieldName, value string) error {
	if len(strings.TrimSpace(value)) != fundManagementBillDateLength {
		return newFundManagementValidationError(operation, "%s must use YYYY-MM-DD format", fieldName)
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(value)); err != nil {
		return newFundManagementValidationError(operation, "%s must use YYYY-MM-DD format", fieldName)
	}
	return nil
}
