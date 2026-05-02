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
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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

type SubmitOrdinaryServiceProviderApplymentInput struct {
	Applyment     db.EcommerceApplyment
	WechatRequest ospcontracts.ApplymentSubmitRequest
}

type SubmitEcommerceApplymentResult struct {
	ApplymentID          int64
	Status               string
	StatusDesc           string
	Message              string
	InitialQueryResponse *wechatcontracts.EcommerceApplymentQueryResponse
}

type SubmitOrdinaryServiceProviderApplymentResult struct {
	ApplymentID          int64
	Status               string
	StatusDesc           string
	Message              string
	InitialQueryResponse *ospcontracts.ApplymentQueryResponse
}

func updateApplymentSubjectStatusWithLog(ctx context.Context, updater ApplymentSubjectStatusUpdater, localApplymentID int64, status, reason string) {
	if updater == nil {
		return
	}
	if err := updater(ctx, status); err != nil {
		log.Error().Err(err).
			Int64("local_applyment_id", localApplymentID).
			Str("subject_status", status).
			Str("reason", reason).
			Msg("update applyment subject status failed")
	}
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
	ContactEmail        string
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

type ApplymentOrdinaryRequestInput struct {
	BusinessCode      string
	OrganizationType  string
	BusinessLicense   ApplymentBusinessLicenseOCRInput
	BusinessLicenseID string
	LicenseCopy       string
	MerchantName      string
	LegalPerson       string
	BusinessAddress   string
	MerchantShortname string
	ServicePhone      string
	MiniProgramAppID  string
	StoreName         string
	StoreQRCode       string
	IDCardInfo        ApplymentOrdinaryIDCardInput
	AccountInfo       ApplymentWechatAccountInput
	ContactInfo       ApplymentOrdinaryContactInput
	SettlementID         string
	QualificationType    string
	ActivitiesID         string
	DebitActivitiesRate  string
	CreditActivitiesRate string
	ActivitiesAdditions  []string
}

type ApplymentOrdinaryIDCardInput struct {
	IDCardCopy      string
	IDCardNational  string
	IDCardName      string
	IDCardNumber    string
	CardPeriodBegin string
	CardPeriodEnd   string
}

type ApplymentOrdinaryContactInput struct {
	ContactType          string
	ContactName          string
	ContactIDDocType     string
	ContactIDNumber      string
	ContactIDDocCopy     string
	ContactIDDocCopyBack string
	ContactPeriodBegin   string
	ContactPeriodEnd     string
	MobilePhone          string
	ContactEmail         string
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
		ContactEmail:          pgtype.Text{String: input.ContactEmail, Valid: strings.TrimSpace(input.ContactEmail) != ""},
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

func BuildOrdinaryServiceProviderApplymentRequest(input ApplymentOrdinaryRequestInput) ospcontracts.ApplymentSubmitRequest {
	licensePeriodBegin, licensePeriodEnd := parseApplymentDateRange(input.BusinessLicense.ValidPeriod)
	licenseAddress := strings.TrimSpace(input.BusinessLicense.Address)
	if licenseAddress == "" {
		licenseAddress = strings.TrimSpace(input.BusinessAddress)
	}

	request := ospcontracts.ApplymentSubmitRequest{
		BusinessCode: strings.TrimSpace(input.BusinessCode),
		ContactInfo: ospcontracts.ApplymentContactInfo{
			ContactType:          mapOrdinaryApplymentContactType(input.ContactInfo.ContactType),
			ContactName:          strings.TrimSpace(input.ContactInfo.ContactName),
			ContactIDDocType:     mapOrdinaryApplymentIdentificationType(input.ContactInfo.ContactIDDocType),
			ContactIDNumber:      strings.TrimSpace(input.ContactInfo.ContactIDNumber),
			ContactIDDocCopy:     strings.TrimSpace(input.ContactInfo.ContactIDDocCopy),
			ContactIDDocCopyBack: strings.TrimSpace(input.ContactInfo.ContactIDDocCopyBack),
			ContactPeriodBegin:   strings.TrimSpace(input.ContactInfo.ContactPeriodBegin),
			ContactPeriodEnd:     strings.TrimSpace(input.ContactInfo.ContactPeriodEnd),
			MobilePhone:          strings.TrimSpace(input.ContactInfo.MobilePhone),
			ContactEmail:         strings.TrimSpace(input.ContactInfo.ContactEmail),
		},
		SubjectInfo: ospcontracts.ApplymentSubjectInfo{
			SubjectType:        mapOrdinaryApplymentSubjectType(input.OrganizationType),
			FinanceInstitution: false,
			BusinessLicenseInfo: &ospcontracts.ApplymentBusinessLicenseInfo{
				LicenseCopy:    strings.TrimSpace(input.LicenseCopy),
				LicenseNumber:  strings.TrimSpace(input.BusinessLicenseID),
				MerchantName:   strings.TrimSpace(input.MerchantName),
				LegalPerson:    strings.TrimSpace(input.LegalPerson),
				LicenseAddress: licenseAddress,
				PeriodBegin:    licensePeriodBegin,
				PeriodEnd:      licensePeriodEnd,
			},
			IdentityInfo: ospcontracts.ApplymentIdentityInfo{
				IDHolderType: ospcontracts.ContactTypeLegal,
				IDDocType:    ospcontracts.IdentificationTypeIDCard,
				IDCardInfo: &ospcontracts.ApplymentIDCardInfo{
					IDCardCopy:      strings.TrimSpace(input.IDCardInfo.IDCardCopy),
					IDCardNational:  strings.TrimSpace(input.IDCardInfo.IDCardNational),
					IDCardName:      strings.TrimSpace(input.IDCardInfo.IDCardName),
					IDCardNumber:    strings.TrimSpace(input.IDCardInfo.IDCardNumber),
					CardPeriodBegin: strings.TrimSpace(input.IDCardInfo.CardPeriodBegin),
					CardPeriodEnd:   strings.TrimSpace(input.IDCardInfo.CardPeriodEnd),
				},
			},
		},
		BusinessInfo: ospcontracts.ApplymentBusinessInfo{
			MerchantShortname: strings.TrimSpace(input.MerchantShortname),
			ServicePhone:      strings.TrimSpace(input.ServicePhone),
			SalesInfo: ospcontracts.ApplymentSalesInfo{
				SalesScenesType: []ospcontracts.ApplymentSalesSceneType{ospcontracts.SalesSceneMiniProgram},
				MiniProgramInfo: &ospcontracts.ApplymentMiniProgramInfo{
					MiniProgramAppID: strings.TrimSpace(input.MiniProgramAppID),
				},
			},
		},
		SettlementInfo: ospcontracts.ApplymentSettlementInfo{
			SettlementID:         strings.TrimSpace(input.SettlementID),
			QualificationType:    strings.TrimSpace(input.QualificationType),
			ActivitiesID:         strings.TrimSpace(input.ActivitiesID),
			DebitActivitiesRate:  strings.TrimSpace(input.DebitActivitiesRate),
			CreditActivitiesRate: strings.TrimSpace(input.CreditActivitiesRate),
			ActivitiesAdditions:  trimOrdinaryApplymentActivitiesAdditions(input.ActivitiesAdditions),
		},
		BankAccountInfo: ospcontracts.ApplymentBankAccountInfo{
			BankAccountType: mapOrdinaryApplymentBankAccountType(input.AccountInfo.AccountType),
			AccountName:     strings.TrimSpace(input.AccountInfo.AccountName),
			AccountBank:     strings.TrimSpace(input.AccountInfo.AccountBank),
			BankAddressCode: strings.TrimSpace(input.AccountInfo.BankAddressCode),
			BankBranchID:    strings.TrimSpace(input.AccountInfo.BankBranchID),
			BankName:        strings.TrimSpace(input.AccountInfo.BankName),
			AccountNumber:   strings.TrimSpace(input.AccountInfo.AccountNumber),
		},
	}

	if storeQRCode := strings.TrimSpace(input.StoreQRCode); storeQRCode != "" {
		request.BusinessInfo.SalesInfo.MiniProgramInfo.MiniProgramPics = []string{storeQRCode}
		request.BusinessInfo.SalesInfo.StoreInfo = &ospcontracts.ApplymentStoreInfo{
			StoreName:        strings.TrimSpace(input.StoreName),
			StoreEntrancePic: []string{storeQRCode},
		}
	}

	return request
}

func trimOrdinaryApplymentActivitiesAdditions(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	additions := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			additions = append(additions, trimmed)
		}
	}
	if len(additions) == 0 {
		return nil
	}
	return additions
}

func mapOrdinaryApplymentSubjectType(organizationType string) ospcontracts.SubjectType {
	switch strings.TrimSpace(organizationType) {
	case "4":
		return ospcontracts.SubjectTypeIndividual
	case "2":
		return ospcontracts.SubjectTypeEnterprise
	default:
		return ospcontracts.SubjectType("")
	}
}

func mapOrdinaryApplymentBankAccountType(accountType string) ospcontracts.BankAccountType {
	switch strings.TrimSpace(accountType) {
	case "ACCOUNT_TYPE_BUSINESS":
		return ospcontracts.BankAccountTypeCorporate
	case "ACCOUNT_TYPE_PRIVATE":
		return ospcontracts.BankAccountTypePersonal
	default:
		return ospcontracts.BankAccountType("")
	}
}

func mapOrdinaryApplymentContactType(contactType string) ospcontracts.ContactType {
	switch strings.ToUpper(strings.TrimSpace(contactType)) {
	case "SUPER", "66":
		return ospcontracts.ContactTypeSuper
	default:
		return ospcontracts.ContactTypeLegal
	}
}

func mapOrdinaryApplymentIdentificationType(docType string) ospcontracts.IdentificationType {
	switch strings.ToUpper(strings.TrimSpace(docType)) {
	case "", "IDENTIFICATION_TYPE_MAINLAND_IDCARD", "IDENTIFICATION_TYPE_IDCARD":
		return ospcontracts.IdentificationTypeIDCard
	default:
		return ospcontracts.IdentificationType(docType)
	}
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
			return "", NewRequestError(http.StatusBadRequest, errors.New(applymentImageUploadValidationMessage(err)))
		}
		return "", err
	}
	if response == nil || strings.TrimSpace(response.MediaID) == "" {
		return "", fmt.Errorf("wechat upload returned empty media id for asset %d", assetID)
	}

	return strings.TrimSpace(response.MediaID), nil
}

func applymentImageUploadValidationMessage(err error) string {
	if err == nil {
		return "图片上传校验失败，请上传 JPG、JPEG、PNG 或 BMP 格式图片后重试"
	}
	lowerMessage := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowerMessage, "file is empty"):
		return "图片文件为空，请上传非空的 JPG、JPEG、PNG 或 BMP 图片后重试"
	case strings.Contains(lowerMessage, "filename is required"):
		return "图片文件名缺失或格式不支持，请上传 JPG、JPEG、PNG 或 BMP 图片后重试"
	case strings.Contains(lowerMessage, "2mb"):
		return "图片超过微信支付 2MB 限制，请压缩后重新上传"
	case strings.Contains(lowerMessage, "does not match"):
		return "图片内容与文件扩展名不一致，请重新上传真实的图片文件后重试"
	default:
		return "图片格式不符合微信支付上传要求，请上传 JPG、JPEG、PNG 或 BMP 图片后重试"
	}
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
		updateApplymentSubjectStatusWithLog(ctx, updateSubjectStatus, input.Applyment.ID, ApplymentSubjectStatusBindbankSubmitted, "ecommerce_client_missing")
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
		updateApplymentSubjectStatusWithLog(ctx, updateSubjectStatus, input.Applyment.ID, ApplymentSubjectStatusBindbankSubmitted, "submitted_state_sync_failed")
		return result, fmt.Errorf("同步微信进件提交状态失败: %w", err)
	}
	recordEcommerceApplymentCommandAccepted(ctx, store, input.Applyment, resp)

	updateApplymentSubjectStatusWithLog(ctx, updateSubjectStatus, input.Applyment.ID, ApplymentSubjectStatusBindbankSubmitted, "submit_applyment_accepted")

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

func SubmitOrdinaryServiceProviderApplyment(
	ctx context.Context,
	store ApplymentSubmissionStore,
	ordinaryClient ordinaryserviceprovider.OrdinaryServiceProviderClientInterface,
	updateSubjectStatus ApplymentSubjectStatusUpdater,
	input SubmitOrdinaryServiceProviderApplymentInput,
) (SubmitOrdinaryServiceProviderApplymentResult, error) {
	var result SubmitOrdinaryServiceProviderApplymentResult

	if ordinaryClient == nil {
		return result, fmt.Errorf("ordinary service provider client not configured")
	}

	resp, err := ordinaryClient.SubmitApplyment(ctx, input.WechatRequest)
	if err != nil {
		return result, fmt.Errorf("submit ordinary service provider applyment: %w", err)
	}

	_, err = store.UpdateEcommerceApplymentToSubmitted(ctx, db.UpdateEcommerceApplymentToSubmittedParams{
		ID:          input.Applyment.ID,
		ApplymentID: pgtype.Int8{Int64: resp.ApplymentID, Valid: resp.ApplymentID > 0},
	})
	if err != nil {
		updateApplymentSubjectStatusWithLog(ctx, updateSubjectStatus, input.Applyment.ID, ApplymentSubjectStatusBindbankSubmitted, "ordinary_submitted_state_sync_failed")
		return result, fmt.Errorf("sync ordinary service provider applyment submitted state: %w", err)
	}
	recordOrdinaryServiceProviderApplymentCommandAccepted(ctx, store, input.Applyment, resp)

	updateApplymentSubjectStatusWithLog(ctx, updateSubjectStatus, input.Applyment.ID, ApplymentSubjectStatusBindbankSubmitted, "ordinary_submit_applyment_accepted")

	result.ApplymentID = resp.ApplymentID
	result.Status = ApplymentSubmissionResultStatus
	result.StatusDesc = ApplymentSubmissionSuccessMessage
	result.Message = ApplymentSubmissionSuccessMessage
	if err := waitApplymentInitialQuery(ctx, applymentInitialQueryDelay); err != nil {
		return result, nil
	}

	initialQueryResp, err := queryInitialOrdinaryApplymentStatus(ctx, ordinaryClient, resp.ApplymentID, input.Applyment.OutRequestNo)
	if err == nil && initialQueryResp != nil {
		resolvedStatus := NormalizeResolvedApplymentStatus(
			MapOrdinaryApplymentStateToStatus(initialQueryResp.ApplymentState),
			strings.TrimSpace(initialQueryResp.SubMchID) != "",
		)
		if resolvedStatus == "" {
			resolvedStatus = ApplymentSubmissionResultStatus
		}
		statusDesc := getApplymentSubmissionStatusDesc(resolvedStatus)
		if statusDesc == "未知状态" && strings.TrimSpace(initialQueryResp.ApplymentStateMsg) != "" {
			statusDesc = strings.TrimSpace(initialQueryResp.ApplymentStateMsg)
		}

		result.Status = resolvedStatus
		result.StatusDesc = statusDesc
		result.Message = OrdinaryApplymentFrontendMessage(resolvedStatus)
		result.InitialQueryResponse = initialQueryResp

		if syncErr := syncInitialOrdinaryApplymentQueryResult(ctx, store, input.Applyment.ID, initialQueryResp); syncErr != nil {
			log.Error().Err(syncErr).
				Int64("local_applyment_id", input.Applyment.ID).
				Int64("wechat_applyment_id", initialQueryResp.ApplymentID).
				Msg("sync initial ordinary service provider applyment query result failed")
		}
	}

	return result, nil
}

func syncInitialOrdinaryApplymentQueryResult(
	ctx context.Context,
	store ApplymentSubmissionStore,
	localApplymentID int64,
	queryResp *ospcontracts.ApplymentQueryResponse,
) error {
	if queryResp == nil {
		return nil
	}
	resolvedStatus := NormalizeResolvedApplymentStatus(
		MapOrdinaryApplymentStateToStatus(queryResp.ApplymentState),
		strings.TrimSpace(queryResp.SubMchID) != "",
	)
	if resolvedStatus == "" {
		resolvedStatus = ApplymentSubmissionResultStatus
	}
	_, err := store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
		ID:          localApplymentID,
		ApplymentID: pgtype.Int8{Int64: queryResp.ApplymentID, Valid: queryResp.ApplymentID > 0},
		Status:      resolvedStatus,
		RejectReason: pgtype.Text{
			String: ordinaryApplymentRejectReason(queryResp.AuditDetail),
			Valid:  ordinaryApplymentRejectReason(queryResp.AuditDetail) != "",
		},
		SignUrl:            buildApplymentSubmissionText(queryResp.SignURL),
		SignState:          pgtype.Text{},
		LegalValidationUrl: pgtype.Text{},
		AccountValidation:  nil,
		SubMchID:           buildApplymentSubmissionText(queryResp.SubMchID),
	})
	return err
}

func recordOrdinaryServiceProviderApplymentCommandAccepted(ctx context.Context, store ApplymentSubmissionStore, applyment db.EcommerceApplyment, resp *ospcontracts.ApplymentSubmitResponse) {
	if resp == nil {
		return
	}
	outRequestNo := strings.TrimSpace(applyment.OutRequestNo)
	applymentID := ""
	if resp.ApplymentID > 0 {
		applymentID = fmt.Sprintf("%d", resp.ApplymentID)
	}
	secondaryKey := applymentStringPtrIfNotEmpty(applymentID)
	businessObjectType := "ordinary_service_provider_applyment"
	businessObjectID := applyment.ID
	if _, err := NewPaymentCommandService(store).RecordExternalPaymentCommand(ctx, RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
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
			"out_request_no":   outRequestNo,
			"applyment_id":     applymentID,
			"applyment_status": ApplymentSubmissionResultStatus,
		}),
	}); err != nil {
		log.Warn().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("out_request_no", outRequestNo).
			Msg("record ordinary service provider applyment command accepted failed")
	}
}

func queryInitialOrdinaryApplymentStatus(
	ctx context.Context,
	ordinaryClient ordinaryserviceprovider.OrdinaryServiceProviderClientInterface,
	applymentID int64,
	outRequestNo string,
) (*ospcontracts.ApplymentQueryResponse, error) {
	if ordinaryClient == nil {
		return nil, nil
	}
	if applymentID > 0 {
		resp, err := ordinaryClient.QueryApplymentByID(ctx, ospcontracts.ApplymentQueryByIDRequest{ApplymentID: applymentID})
		if err == nil {
			log.Info().
				Str("query_key", "applyment_id").
				Int64("wechat_applyment_id", resp.ApplymentID).
				Str("business_code", strings.TrimSpace(resp.BusinessCode)).
				Str("applyment_state", string(resp.ApplymentState)).
				Msg("query initial ordinary service provider applyment status succeeded")
			return resp, nil
		}
		if strings.TrimSpace(outRequestNo) == "" {
			return nil, err
		}
	}
	resp, err := ordinaryClient.QueryApplymentByBusinessCode(ctx, ospcontracts.ApplymentQueryByBusinessCodeRequest{BusinessCode: outRequestNo})
	if err == nil {
		log.Info().
			Str("query_key", "business_code").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("business_code", strings.TrimSpace(resp.BusinessCode)).
			Str("applyment_state", string(resp.ApplymentState)).
			Msg("query initial ordinary service provider applyment status succeeded")
	}
	return resp, err
}

func MapOrdinaryApplymentStateToStatus(state ospcontracts.ApplymentState) string {
	switch state {
	case ospcontracts.ApplymentStateEditing:
		return "submitted"
	case ospcontracts.ApplymentStateAuditing:
		return "auditing"
	case ospcontracts.ApplymentStateRejected:
		return "rejected"
	case ospcontracts.ApplymentStateToBeConfirmed:
		return "to_be_confirmed"
	case ospcontracts.ApplymentStateToBeSigned:
		return "to_be_signed"
	case ospcontracts.ApplymentStateSigning:
		return "signing"
	case ospcontracts.ApplymentStateFinished:
		return "finish"
	case ospcontracts.ApplymentStateCanceled:
		return "canceled"
	default:
		return ""
	}
}

func ordinaryApplymentRejectReason(details []ospcontracts.ApplymentAuditDetail) string {
	parts := make([]string, 0, len(details))
	for _, detail := range details {
		reason := strings.TrimSpace(detail.RejectReason)
		if reason == "" {
			continue
		}
		fieldName := strings.TrimSpace(detail.FieldName)
		if fieldName == "" {
			fieldName = strings.TrimSpace(detail.Field)
		}
		if fieldName != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", fieldName, reason))
		} else {
			parts = append(parts, reason)
		}
	}
	return strings.Join(parts, "; ")
}

func OrdinaryApplymentFrontendMessage(status string) string {
	switch strings.TrimSpace(status) {
	case "finish":
		return "进件已通过，正在确认微信开户意愿授权状态；授权完成后平台会开通普通服务商交易能力"
	case "to_be_confirmed":
		return "进件待商户确认，请按微信支付页面提示完成确认后刷新状态"
	case "to_be_signed", "signing":
		return "进件进入签约环节，请打开签约链接完成签约后刷新状态"
	case "rejected":
		return "进件被微信驳回，请根据驳回原因修正资料后重新提交"
	default:
		return getApplymentSubmissionStatusDesc(status)
	}
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
