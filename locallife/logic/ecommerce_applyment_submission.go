package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

var applymentDateTokenPattern = regexp.MustCompile(`\d{4}年\d{1,2}月\d{1,2}日|\d{4}[./-]\d{1,2}[./-]\d{1,2}|\d{8}|长期|永久`)

const (
	ApplymentSubjectStatusBindbankSubmitted = "bindbank_submitted"
	ApplymentSubmissionResultStatus         = "submitted"
	ApplymentSubmissionSuccessMessage       = "开户申请已提交，请立即进入状态页查看签约与账户验证进度，审核将并行进行"
	ApplymentSubmissionFallbackMessage      = "银行卡信息已保存，待人工处理"
	applymentInitialQueryDelay              = 3 * time.Second
)

var (
	ErrMerchantApplymentSubmissionStatusInvalid = errors.New("merchant status does not allow applyment submission")
	ErrApplymentSubmissionPending               = errors.New("applyment submission pending")
	ErrApplymentAlreadyRegistered               = errors.New("applyment already registered")
)

type ApplymentSubmissionStore interface {
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
	UpdateEcommerceApplymentToSubmitted(ctx context.Context, arg db.UpdateEcommerceApplymentToSubmittedParams) (db.EcommerceApplyment, error)
	UpdateEcommerceApplymentStatus(ctx context.Context, arg db.UpdateEcommerceApplymentStatusParams) (db.EcommerceApplyment, error)
}

type ApplymentAssetDownloader interface {
	DownloadObject(ctx context.Context, assetID int64) (string, []byte, error)
}

type ApplymentImageUploader interface {
	UploadImage(ctx context.Context, filename string, fileData []byte) (*wechat.ImageUploadResponse, error)
}

type ApplymentSensitiveEncryptor interface {
	EncryptSensitiveData(plaintext string) (string, error)
}

type ApplymentSubjectStatusUpdater func(ctx context.Context, status string) error

type SubmitEcommerceApplymentInput struct {
	Applyment     db.EcommerceApplyment
	WechatRequest *wechat.EcommerceApplymentRequest
}

type SubmitEcommerceApplymentResult struct {
	ApplymentID          int64
	Status               string
	StatusDesc           string
	Message              string
	InitialQueryResponse *wechatcontracts.EcommerceApplymentQueryResponse
}

func waitApplymentInitialQuery(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildApplymentSubmissionText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func getRejectReasonFromApplymentAuditDetails(details []wechatcontracts.ApplymentAuditDetail) pgtype.Text {
	if len(details) == 0 {
		return pgtype.Text{}
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		parts = append(parts, fmt.Sprintf("%s: %s", detail.ParamName, detail.RejectReason))
	}

	return pgtype.Text{String: strings.Join(parts, "; "), Valid: true}
}

func syncInitialApplymentQueryResult(
	ctx context.Context,
	store ApplymentSubmissionStore,
	localApplymentID int64,
	queryResp *wechatcontracts.EcommerceApplymentQueryResponse,
) error {
	if queryResp == nil {
		return nil
	}

	resolvedStatus := NormalizeResolvedApplymentStatus(
		ResolveWechatApplymentStatus("", queryResp.ApplymentState, queryResp.SignState),
		strings.TrimSpace(queryResp.SubMchID) != "",
	)
	if resolvedStatus == "" {
		resolvedStatus = ApplymentSubmissionResultStatus
	}

	_, err := store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
		ID:                 localApplymentID,
		ApplymentID:        pgtype.Int8{Int64: queryResp.ApplymentID, Valid: queryResp.ApplymentID > 0},
		Status:             resolvedStatus,
		RejectReason:       getRejectReasonFromApplymentAuditDetails(queryResp.AuditDetail),
		SignUrl:            buildApplymentSubmissionText(queryResp.SignURL),
		SignState:          buildApplymentSubmissionText(queryResp.SignState),
		LegalValidationUrl: buildApplymentSubmissionText(queryResp.LegalValidationURL),
		AccountValidation:  wechat.MarshalEcommerceApplymentAccountValidation(queryResp.AccountValidation),
		SubMchID:           buildApplymentSubmissionText(queryResp.SubMchID),
	})
	return err
}

type ApplymentLocalRecordInput struct {
	SubjectType           string
	SubjectID             int64
	OutRequestNo          string
	OrganizationType      string
	BusinessLicenseNumber string
	// Snapshot URL for audit/display only. Future resubmission must re-upload from asset IDs instead of reusing this value as a WeChat media_id.
	BusinessLicenseCopy string
	MerchantName        string
	LegalPerson         string
	IDCardNumber        string
	IDCardName          string
	IDCardValidTime     string
	// Snapshot URL for audit/display only. Future resubmission must re-upload from asset IDs instead of reusing this value as a WeChat media_id.
	IDCardFrontCopy string
	// Snapshot URL for audit/display only. Future resubmission must re-upload from asset IDs instead of reusing this value as a WeChat media_id.
	IDCardBackCopy      string
	AccountType         string
	AccountBank         string
	AccountBankCode     int64
	BankAlias           string
	BankAliasCode       string
	BankAddressCode     string
	BankBranchID        string
	BankName            string
	AccountNumber       string
	AccountName         string
	ContactName         string
	ContactIDCardNumber string
	MobilePhone         string
	MerchantShortname   string
}

type ApplymentWechatAccountInput struct {
	AccountType     string
	AccountBank     string
	AccountName     string
	BankAddressCode string
	BankBranchID    string
	BankName        string
	AccountNumber   string
}

type ApplymentWechatContactInput struct {
	ContactType             string
	ContactName             string
	ContactIDDocType        string
	ContactIDCardNumber     string
	ContactIDDocPeriodBegin string
	ContactIDDocPeriodEnd   string
	ContactIDDocCopy        string
	ContactIDDocCopyBack    string
	MobilePhone             string
}

type ApplymentWechatRequestInput struct {
	OutRequestNo      string
	OrganizationType  string
	BusinessLicense   *wechatcontracts.ApplymentBusinessLicenseInfo
	MerchantShortname string
	IDCardInfo        *wechatcontracts.ApplymentIDCardInfo
	AccountInfo       ApplymentWechatAccountInput
	ContactInfo       ApplymentWechatContactInput
	StoreName         string
	StoreURL          string
	StoreQRCode       string
}

type ApplymentBusinessLicenseOCRInput struct {
	Address     string
	ValidPeriod string
}

type ApplymentWechatSensitiveInput struct {
	IDCardName          string
	IDCardNumber        string
	ContactName         string
	ContactIDCardNumber string
	AccountName         string
	AccountNumber       string
	MobilePhone         string
}

type ApplymentWechatSensitiveOutput struct {
	IDCardName          string
	IDCardNumber        string
	ContactName         string
	ContactIDCardNumber string
	AccountName         string
	AccountNumber       string
	MobilePhone         string
}

type ApplymentSensitiveEncryptionError struct {
	Field string
	Err   error
}

func (e *ApplymentSensitiveEncryptionError) Error() string {
	return fmt.Sprintf("encrypt applyment field %s: %v", e.Field, e.Err)
}

func (e *ApplymentSensitiveEncryptionError) Unwrap() error {
	return e.Err
}

func ValidateMerchantApplymentSubmissionState(merchantStatus string, existing *db.EcommerceApplyment) error {
	if merchantStatus != "approved" && merchantStatus != "pending_bindbank" {
		return fmt.Errorf("%w: %s", ErrMerchantApplymentSubmissionStatusInvalid, merchantStatus)
	}
	if existing == nil {
		return nil
	}
	if IsApplymentSubmissionInFlight(existing.Status, merchantStatus, existing.OutRequestNo) {
		return ErrApplymentSubmissionPending
	}
	if existing.Status == "finish" {
		return ErrApplymentAlreadyRegistered
	}
	return nil
}

func BuildCreateEcommerceApplymentParams(input ApplymentLocalRecordInput) db.CreateEcommerceApplymentParams {
	return db.CreateEcommerceApplymentParams{
		SubjectType:           input.SubjectType,
		SubjectID:             input.SubjectID,
		OutRequestNo:          input.OutRequestNo,
		OrganizationType:      input.OrganizationType,
		BusinessLicenseNumber: pgtype.Text{String: input.BusinessLicenseNumber, Valid: input.BusinessLicenseNumber != ""},
		BusinessLicenseCopy:   pgtype.Text{String: input.BusinessLicenseCopy, Valid: input.BusinessLicenseCopy != ""},
		MerchantName:          input.MerchantName,
		LegalPerson:           input.LegalPerson,
		IDCardNumber:          input.IDCardNumber,
		IDCardName:            input.IDCardName,
		IDCardValidTime:       input.IDCardValidTime,
		IDCardFrontCopy:       input.IDCardFrontCopy,
		IDCardBackCopy:        input.IDCardBackCopy,
		AccountType:           input.AccountType,
		AccountBank:           input.AccountBank,
		AccountBankCode:       pgtype.Int8{Int64: input.AccountBankCode, Valid: input.AccountBankCode > 0},
		BankAlias:             pgtype.Text{String: input.BankAlias, Valid: input.BankAlias != ""},
		BankAliasCode:         pgtype.Text{String: input.BankAliasCode, Valid: input.BankAliasCode != ""},
		BankAddressCode:       input.BankAddressCode,
		BankBranchID:          pgtype.Text{String: input.BankBranchID, Valid: input.BankBranchID != ""},
		BankName:              pgtype.Text{String: input.BankName, Valid: input.BankName != ""},
		AccountNumber:         input.AccountNumber,
		AccountName:           input.AccountName,
		ContactName:           input.ContactName,
		ContactIDCardNumber:   pgtype.Text{String: input.ContactIDCardNumber, Valid: input.ContactIDCardNumber != ""},
		MobilePhone:           input.MobilePhone,
		MerchantShortname:     input.MerchantShortname,
		Qualifications:        []byte("[]"),
		BusinessAdditionPics:  []string{},
		BusinessAdditionDesc:  pgtype.Text{},
	}
}

func BuildWechatApplymentAccountInfo(input ApplymentWechatAccountInput) *wechatcontracts.ApplymentBankAccountInfo {
	return &wechatcontracts.ApplymentBankAccountInfo{
		BankAccountType: input.AccountType,
		AccountBank:     input.AccountBank,
		AccountName:     input.AccountName,
		BankAddressCode: input.BankAddressCode,
		BankBranchID:    input.BankBranchID,
		BankName:        input.BankName,
		AccountNumber:   input.AccountNumber,
	}
}

func BuildWechatApplymentContactInfo(input ApplymentWechatContactInput) *wechatcontracts.ApplymentContactInfo {
	info := &wechatcontracts.ApplymentContactInfo{
		ContactType:          input.ContactType,
		ContactName:          input.ContactName,
		ContactIDCardNumber:  input.ContactIDCardNumber,
		MobilePhone:          input.MobilePhone,
		ContactIDDocCopy:     input.ContactIDDocCopy,
		ContactIDDocCopyBack: input.ContactIDDocCopyBack,
	}

	if input.ContactIDDocType != "" {
		info.ContactIDDocType = input.ContactIDDocType
	}
	if input.ContactIDDocPeriodBegin != "" {
		info.ContactIDDocPeriodBegin = input.ContactIDDocPeriodBegin
	}
	if input.ContactIDDocPeriodEnd != "" {
		info.ContactIDDocPeriodEnd = input.ContactIDDocPeriodEnd
	}

	return info
}

func BuildWechatApplymentRequest(input ApplymentWechatRequestInput) *wechat.EcommerceApplymentRequest {
	request := &wechat.EcommerceApplymentRequest{
		OutRequestNo:       input.OutRequestNo,
		OrganizationType:   input.OrganizationType,
		FinanceInstitution: false,
		BusinessLicense:    input.BusinessLicense,
		MerchantShortname:  input.MerchantShortname,
		IDCardInfo:         input.IDCardInfo,
		AccountInfo:        BuildWechatApplymentAccountInfo(input.AccountInfo),
		ContactInfo:        BuildWechatApplymentContactInfo(input.ContactInfo),
	}

	if input.StoreName != "" || input.StoreURL != "" || input.StoreQRCode != "" {
		request.SalesSceneInfo = &wechatcontracts.ApplymentSalesSceneInfo{
			StoreName:   input.StoreName,
			StoreURL:    input.StoreURL,
			StoreQRCode: input.StoreQRCode,
		}
	}

	return request
}

func ParseApplymentIDCardValidPeriod(validDate string) (string, string) {
	begin, end := parseApplymentDateRange(validDate)
	if begin == "" && end == "" {
		return "", "长期"
	}
	return begin, end
}

func BuildApplymentBusinessLicenseInfo(copyMediaID, businessLicenseNumber, merchantName, legalPerson, fallbackAddress string, ocr ApplymentBusinessLicenseOCRInput) *wechatcontracts.ApplymentBusinessLicenseInfo {
	if copyMediaID == "" {
		return nil
	}

	info := &wechatcontracts.ApplymentBusinessLicenseInfo{
		BusinessLicenseCopy:   copyMediaID,
		BusinessLicenseNumber: businessLicenseNumber,
		MerchantName:          merchantName,
		LegalPerson:           legalPerson,
	}

	if address := strings.TrimSpace(ocr.Address); address != "" {
		info.CompanyAddress = address
	}
	if businessTime := BuildApplymentBusinessTime(ocr.ValidPeriod); businessTime != "" {
		info.BusinessTime = businessTime
	}
	if info.CompanyAddress == "" {
		info.CompanyAddress = strings.TrimSpace(fallbackAddress)
	}

	return info
}

func BuildApplymentBusinessTime(validPeriod string) string {
	trimmed := strings.TrimSpace(validPeriod)
	if trimmed == "" || trimmed == "长期" {
		return ""
	}

	start, end := parseApplymentDateRange(trimmed)
	if start == "" || end == "" {
		return ""
	}

	return fmt.Sprintf("[\"%s\",\"%s\"]", start, end)
}

func ResolveApplymentOrganizationType(businessLicenseNumber, licenseType, subjectName, defaultLicensedType string) string {
	if strings.TrimSpace(businessLicenseNumber) == "" {
		return "2401"
	}

	trimmedType := strings.TrimSpace(licenseType)
	switch {
	case strings.Contains(trimmedType, "个体"):
		return "4"
	case strings.Contains(trimmedType, "事业单位"):
		return "3"
	case strings.Contains(trimmedType, "政府"):
		return "2502"
	case strings.Contains(trimmedType, "社会组织"), strings.Contains(trimmedType, "社会团体"), strings.Contains(trimmedType, "基金会"), strings.Contains(trimmedType, "民办非企业"), strings.Contains(trimmedType, "基层群众性自治组织"), strings.Contains(trimmedType, "农村集体经济组织"):
		return "1708"
	case strings.Contains(trimmedType, "公司"), strings.Contains(trimmedType, "企业"), strings.Contains(trimmedType, "合伙"), strings.Contains(trimmedType, "股份"):
		return "2"
	}

	trimmedName := strings.TrimSpace(subjectName)
	if strings.Contains(trimmedName, "公司") || strings.Contains(trimmedName, "有限") {
		return "2"
	}

	if defaultLicensedType != "" {
		return defaultLicensedType
	}

	return "4"
}

func UploadApplymentAsset(ctx context.Context, downloader ApplymentAssetDownloader, uploader ApplymentImageUploader, assetID int64) (string, error) {
	filename, fileData, err := downloader.DownloadObject(ctx, assetID)
	if err != nil {
		return "", err
	}

	response, err := uploader.UploadImage(ctx, filename, fileData)
	if err != nil {
		if wechat.IsUploadImageValidationError(err) {
			return "", NewRequestError(http.StatusBadRequest, err)
		}
		return "", err
	}
	if response == nil || strings.TrimSpace(response.MediaID) == "" {
		return "", fmt.Errorf("wechat upload returned empty media id for asset %d", assetID)
	}

	return strings.TrimSpace(response.MediaID), nil
}

func EncryptApplymentWechatSensitiveFields(encryptor ApplymentSensitiveEncryptor, input ApplymentWechatSensitiveInput) (ApplymentWechatSensitiveOutput, error) {
	var output ApplymentWechatSensitiveOutput

	encryptField := func(fieldName, plaintext string) (string, error) {
		ciphertext, err := encryptor.EncryptSensitiveData(plaintext)
		if err != nil {
			return "", &ApplymentSensitiveEncryptionError{Field: fieldName, Err: err}
		}
		return ciphertext, nil
	}

	var err error
	if output.IDCardName, err = encryptField("id_card_name", input.IDCardName); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.IDCardNumber, err = encryptField("id_card_number", input.IDCardNumber); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.ContactName, err = encryptField("contact_name", input.ContactName); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.ContactIDCardNumber, err = encryptField("contact_id_card_number", input.ContactIDCardNumber); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.AccountName, err = encryptField("account_name", input.AccountName); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.AccountNumber, err = encryptField("account_number", input.AccountNumber); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}
	if output.MobilePhone, err = encryptField("mobile_phone", input.MobilePhone); err != nil {
		return ApplymentWechatSensitiveOutput{}, err
	}

	return output, nil
}

func parseApplymentDateRange(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	if strings.Contains(trimmed, "至") {
		parts := strings.SplitN(trimmed, "至", 2)
		return normalizeApplymentDate(parts[0]), normalizeApplymentDate(parts[1])
	}

	tokens := applymentDateTokenPattern.FindAllString(trimmed, -1)
	if len(tokens) >= 2 {
		return normalizeApplymentDate(tokens[0]), normalizeApplymentDate(tokens[len(tokens)-1])
	}

	if len(tokens) == 1 {
		normalized := normalizeApplymentDate(tokens[0])
		if normalized == "长期" {
			return "", normalized
		}
		if strings.Contains(trimmed, "长期") || strings.Contains(trimmed, "永久") {
			return normalized, "长期"
		}
		return "", normalized
	}

	normalized := normalizeApplymentDate(trimmed)
	if normalized == "长期" {
		return "", normalized
	}

	return "", normalized
}

func normalizeApplymentDate(raw string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return normalized
	}
	if strings.Contains(normalized, "长期") || strings.Contains(normalized, "永久") {
		return "长期"
	}
	if parsed, ok := parseFlexibleDate(normalized); ok {
		return parsed.Format("2006-01-02")
	}

	replacer := strings.NewReplacer(
		"年", "-",
		"月", "-",
		"日", "",
		".", "-",
		"/", "-",
	)
	normalized = replacer.Replace(normalized)
	normalized = strings.Trim(normalized, " -")

	for _, layout := range []string{"2006-01-02", "2006-1-2"} {
		if parsed, err := time.Parse(layout, normalized); err == nil {
			return parsed.Format("2006-01-02")
		}
	}

	return normalized
}

func parseFlexibleDate(dateStr string) (time.Time, bool) {
	dateStr = strings.TrimSpace(dateStr)
	dateStr = strings.ReplaceAll(dateStr, " ", "")

	var year, month, day string

	if strings.Contains(dateStr, "年") && strings.Contains(dateStr, "月") && strings.Contains(dateStr, "日") {
		re := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
		if matches := re.FindStringSubmatch(dateStr); len(matches) == 4 {
			year, month, day = matches[1], matches[2], matches[3]
		}
	}

	if year == "" && strings.Count(dateStr, ".") >= 2 {
		parts := strings.Split(dateStr, ".")
		if len(parts) >= 3 {
			year, month, day = parts[0], parts[1], parts[2]
		}
	}

	if year == "" && len(dateStr) >= 8 {
		re := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})`)
		if matches := re.FindStringSubmatch(dateStr); len(matches) == 4 {
			year, month, day = matches[1], matches[2], matches[3]
		}
	}

	if year == "" && strings.Count(dateStr, "-") == 2 {
		parts := strings.Split(dateStr, "-")
		if len(parts) == 3 && len(parts[0]) == 4 {
			year, month, day = parts[0], parts[1], parts[2]
		}
	}

	if year == "" || month == "" || day == "" {
		return time.Time{}, false
	}

	dateFormatted := fmt.Sprintf("%s-%02s-%02s", year, month, day)
	parsed, err := time.Parse("2006-01-02", dateFormatted)
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}

func IsApplymentSubmissionRecovering(status, subjectStatus, outRequestNo string) bool {
	return strings.TrimSpace(status) == "pending" &&
		strings.TrimSpace(subjectStatus) == ApplymentSubjectStatusBindbankSubmitted &&
		strings.TrimSpace(outRequestNo) != ""
}

func IsApplymentSubmissionInFlight(status, subjectStatus, outRequestNo string) bool {
	return isApplymentPendingSubmissionStatus(status) ||
		IsApplymentSubmissionRecovering(status, subjectStatus, outRequestNo)
}

func SubmitEcommerceApplyment(
	ctx context.Context,
	store ApplymentSubmissionStore,
	ecommerceClient wechat.EcommerceClientInterface,
	updateSubjectStatus ApplymentSubjectStatusUpdater,
	input SubmitEcommerceApplymentInput,
) (SubmitEcommerceApplymentResult, error) {
	var result SubmitEcommerceApplymentResult

	if ecommerceClient == nil {
		if updateSubjectStatus != nil {
			_ = updateSubjectStatus(ctx, ApplymentSubjectStatusBindbankSubmitted)
		}
		result.ApplymentID = input.Applyment.ID
		result.Status = ApplymentSubmissionResultStatus
		result.StatusDesc = ApplymentSubmissionFallbackMessage
		result.Message = ApplymentSubmissionFallbackMessage
		return result, nil
	}

	resp, err := ecommerceClient.CreateEcommerceApplyment(ctx, input.WechatRequest)
	if err != nil {
		return result, fmt.Errorf("提交微信开户失败: %w", err)
	}

	_, err = store.UpdateEcommerceApplymentToSubmitted(ctx, db.UpdateEcommerceApplymentToSubmittedParams{
		ID:          input.Applyment.ID,
		ApplymentID: pgtype.Int8{Int64: resp.ApplymentID, Valid: resp.ApplymentID > 0},
	})
	if err != nil {
		if updateSubjectStatus != nil {
			_ = updateSubjectStatus(ctx, ApplymentSubjectStatusBindbankSubmitted)
		}
		return result, fmt.Errorf("同步微信进件提交状态失败: %w", err)
	}
	recordEcommerceApplymentCommandAccepted(ctx, store, input.Applyment, resp)

	if updateSubjectStatus != nil {
		_ = updateSubjectStatus(ctx, ApplymentSubjectStatusBindbankSubmitted)
	}

	result.ApplymentID = resp.ApplymentID
	result.Status = ApplymentSubmissionResultStatus
	result.StatusDesc = ApplymentSubmissionSuccessMessage
	result.Message = ApplymentSubmissionSuccessMessage
	if err := waitApplymentInitialQuery(ctx, applymentInitialQueryDelay); err != nil {
		return result, nil
	}

	initialQueryResp, err := queryInitialApplymentStatus(ctx, ecommerceClient, resp.ApplymentID, input.Applyment.OutRequestNo)
	if err == nil && initialQueryResp != nil {
		resolvedStatus := NormalizeResolvedApplymentStatus(
			ResolveWechatApplymentStatus(result.Status, initialQueryResp.ApplymentState, initialQueryResp.SignState),
			strings.TrimSpace(initialQueryResp.SubMchID) != "",
		)
		if resolvedStatus == "" {
			resolvedStatus = ApplymentSubmissionResultStatus
		}
		statusDesc := getApplymentSubmissionStatusDesc(resolvedStatus)
		if statusDesc == "未知状态" && strings.TrimSpace(initialQueryResp.ApplymentStateDesc) != "" {
			statusDesc = strings.TrimSpace(initialQueryResp.ApplymentStateDesc)
		}

		result.Status = resolvedStatus
		result.StatusDesc = statusDesc
		result.Message = statusDesc
		result.InitialQueryResponse = initialQueryResp

		if syncErr := syncInitialApplymentQueryResult(ctx, store, input.Applyment.ID, initialQueryResp); syncErr != nil {
			log.Error().Err(syncErr).
				Int64("local_applyment_id", input.Applyment.ID).
				Int64("wechat_applyment_id", initialQueryResp.ApplymentID).
				Msg("sync initial applyment query result failed")
		}
	}

	return result, nil
}

func recordEcommerceApplymentCommandAccepted(ctx context.Context, store ApplymentSubmissionStore, applyment db.EcommerceApplyment, resp *wechatcontracts.EcommerceApplymentResponse) {
	if resp == nil {
		return
	}
	outRequestNo := strings.TrimSpace(applyment.OutRequestNo)
	if outRequestNo == "" {
		outRequestNo = strings.TrimSpace(resp.OutRequestNo)
	}
	applymentID := ""
	if resp.ApplymentID > 0 {
		applymentID = fmt.Sprintf("%d", resp.ApplymentID)
	}
	secondaryKey := applymentStringPtrIfNotEmpty(applymentID)
	businessObjectType := "ecommerce_applyment"
	businessObjectID := applyment.ID
	if _, err := NewPaymentCommandService(store).RecordExternalPaymentCommand(ctx, RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityApplyment,
		CommandType:          db.ExternalPaymentCommandTypeCreateApplyment,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerApplyment,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectApplyment,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: secondaryKey,
		CommandStatus:        db.ExternalPaymentCommandStatusAccepted,
		ResponseSnapshot: ecommerceApplymentCommandSnapshot(map[string]string{
			"out_request_no":          outRequestNo,
			"response_out_request_no": strings.TrimSpace(resp.OutRequestNo),
			"applyment_id":            applymentID,
			"applyment_status":        ApplymentSubmissionResultStatus,
		}),
	}); err != nil {
		log.Warn().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("out_request_no", outRequestNo).
			Msg("record ecommerce applyment command accepted failed")
	}
}

func ecommerceApplymentCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			filtered[key] = trimmedValue
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func applymentStringPtrIfNotEmpty(value string) *string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	return &trimmedValue
}

func queryInitialApplymentStatus(
	ctx context.Context,
	ecommerceClient wechat.EcommerceClientInterface,
	applymentID int64,
	outRequestNo string,
) (*wechatcontracts.EcommerceApplymentQueryResponse, error) {
	if ecommerceClient == nil {
		return nil, nil
	}

	resp, err := ecommerceClient.QueryEcommerceApplymentByID(ctx, applymentID)
	if err == nil {
		log.Info().
			Str("query_key", "applyment_id").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("out_request_no", strings.TrimSpace(resp.OutRequestNo)).
			Str("applyment_state", strings.TrimSpace(resp.ApplymentState)).
			Str("sign_state", strings.TrimSpace(resp.SignState)).
			Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
			Bool("has_legal_validation_url", strings.TrimSpace(resp.LegalValidationURL) != "").
			Bool("has_account_validation", resp.AccountValidation != nil).
			Msg("query initial applyment status succeeded")
		return resp, nil
	}
	if strings.TrimSpace(outRequestNo) == "" {
		return nil, err
	}

	resp, err = ecommerceClient.QueryEcommerceApplymentByOutRequestNo(ctx, outRequestNo)
	if err == nil {
		log.Info().
			Str("query_key", "out_request_no").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("out_request_no", strings.TrimSpace(resp.OutRequestNo)).
			Str("applyment_state", strings.TrimSpace(resp.ApplymentState)).
			Str("sign_state", strings.TrimSpace(resp.SignState)).
			Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
			Bool("has_legal_validation_url", strings.TrimSpace(resp.LegalValidationURL) != "").
			Bool("has_account_validation", resp.AccountValidation != nil).
			Msg("query initial applyment status succeeded")
	}
	return resp, err
}

func getApplymentSubmissionStatusDesc(status string) string {
	switch status {
	case "not_applied":
		return "尚未提交开户申请"
	case "pending":
		return "待提交"
	case ApplymentSubmissionResultStatus:
		return "已提交，请立即进入状态页查看签约与账户验证进度"
	case "checking":
		return "资料校验中"
	case "account_need_verify":
		return "待账户验证"
	case "auditing":
		return "审核中"
	case "to_be_confirmed":
		return "待确认"
	case "rejected":
		return "审核被拒绝"
	case "canceled":
		return "已作废"
	case "frozen":
		return "已冻结"
	case "to_be_signed":
		return "待签约，请点击签约链接完成签约"
	case "signing":
		return "签约中"
	case "rejected_sign":
		return "签约失败"
	case "finish":
		return "开户成功"
	case "active":
		return "账户已开通"
	default:
		return "未知状态"
	}
}

func isApplymentPendingSubmissionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "submitted", "checking", "auditing", "account_need_verify", "to_be_confirmed", "to_be_signed", "signing":
		return true
	default:
		return false
	}
}
