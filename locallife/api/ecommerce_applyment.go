package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

var applymentDateTokenPattern = regexp.MustCompile(`\d{4}年\d{1,2}月\d{1,2}日|\d{4}[./-]\d{1,2}[./-]\d{1,2}|\d{8}|长期|永久`)

const applymentDefaultContactIDDocType = "IDENTIFICATION_TYPE_IDCARD"

// ==================== 商户开户 ====================

type applymentBindBankFields struct {
	AccountType     string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"`
	AccountBank     string `json:"account_bank" binding:"required,max=128"`
	AccountBankCode int64  `json:"account_bank_code"`
	BankAlias       string `json:"bank_alias"`
	BankAliasCode   string `json:"bank_alias_code"`
	NeedBankBranch  bool   `json:"need_bank_branch"`
	BankAddressCode string `json:"bank_address_code"`
	BankBranchID    string `json:"bank_branch_id"`
	BankName        string `json:"bank_name"`
	AccountNumber   string `json:"account_number" binding:"required"`
	AccountName     string `json:"account_name" binding:"required,max=128"`
}

type applymentContactFields struct {
	ContactType                 string `json:"contact_type" binding:"omitempty,oneof=LEGAL SUPER 65 66"`
	ContactName                 string `json:"contact_name" binding:"omitempty,min=2,max=64"`
	ContactIDDocType            string `json:"contact_id_doc_type" binding:"omitempty,oneof=IDENTIFICATION_TYPE_IDCARD"`
	ContactIDCardNumber         string `json:"contact_id_card_number" binding:"omitempty,min=15,max=32"`
	ContactIDDocCopyAssetID     int64  `json:"contact_id_doc_copy_asset_id"`
	ContactIDDocCopyBackAssetID int64  `json:"contact_id_doc_copy_back_asset_id"`
	ContactIDDocPeriodBegin     string `json:"contact_id_doc_period_begin"`
	ContactIDDocPeriodEnd       string `json:"contact_id_doc_period_end"`
}

type resolvedApplymentContact struct {
	ContactType                 string
	ContactName                 string
	ContactIDDocType            string
	ContactIDCardNumber         string
	ContactIDDocCopyAssetID     int64
	ContactIDDocCopyBackAssetID int64
	ContactIDDocPeriodBegin     string
	ContactIDDocPeriodEnd       string
}

func (f *applymentBindBankFields) normalize() {
	f.AccountType = strings.TrimSpace(f.AccountType)
	f.AccountBank = strings.TrimSpace(f.AccountBank)
	f.BankAlias = strings.TrimSpace(f.BankAlias)
	f.BankAliasCode = strings.TrimSpace(f.BankAliasCode)
	f.BankAddressCode = strings.TrimSpace(f.BankAddressCode)
	f.BankBranchID = strings.TrimSpace(f.BankBranchID)
	f.BankName = strings.TrimSpace(f.BankName)
	f.AccountNumber = strings.TrimSpace(f.AccountNumber)
	f.AccountName = strings.TrimSpace(f.AccountName)
	if f.AccountBank == "" && f.BankAlias != "" {
		f.AccountBank = f.BankAlias
	}
}

func (f *applymentContactFields) normalize() {
	f.ContactType = strings.ToUpper(strings.TrimSpace(f.ContactType))
	f.ContactName = strings.TrimSpace(f.ContactName)
	f.ContactIDDocType = strings.ToUpper(strings.TrimSpace(f.ContactIDDocType))
	f.ContactIDCardNumber = strings.TrimSpace(f.ContactIDCardNumber)
	f.ContactIDDocPeriodBegin = strings.TrimSpace(f.ContactIDDocPeriodBegin)
	f.ContactIDDocPeriodEnd = strings.TrimSpace(f.ContactIDDocPeriodEnd)
}

func (f applymentContactFields) resolve(legalPersonName, legalPersonIDNumber string) (resolvedApplymentContact, error) {
	switch f.ContactType {
	case "", "LEGAL", "65":
		if strings.TrimSpace(legalPersonName) == "" || strings.TrimSpace(legalPersonIDNumber) == "" {
			return resolvedApplymentContact{}, fmt.Errorf("legal person identity is required")
		}
		return resolvedApplymentContact{
			ContactType:         "LEGAL",
			ContactName:         strings.TrimSpace(legalPersonName),
			ContactIDCardNumber: strings.TrimSpace(legalPersonIDNumber),
		}, nil
	case "SUPER", "66":
		if f.ContactName == "" {
			return resolvedApplymentContact{}, fmt.Errorf("contact_name is required when contact_type is SUPER")
		}
		if f.ContactIDCardNumber == "" {
			return resolvedApplymentContact{}, fmt.Errorf("contact_id_card_number is required when contact_type is SUPER")
		}
		if f.ContactIDDocCopyAssetID <= 0 {
			return resolvedApplymentContact{}, fmt.Errorf("contact_id_doc_copy_asset_id is required when contact_type is SUPER")
		}
		if f.ContactIDDocCopyBackAssetID <= 0 {
			return resolvedApplymentContact{}, fmt.Errorf("contact_id_doc_copy_back_asset_id is required when contact_type is SUPER")
		}

		contactIDDocType := f.ContactIDDocType
		if contactIDDocType == "" {
			contactIDDocType = applymentDefaultContactIDDocType
		}
		if contactIDDocType != applymentDefaultContactIDDocType {
			return resolvedApplymentContact{}, fmt.Errorf("unsupported contact_id_doc_type: %s", contactIDDocType)
		}

		periodBegin := normalizeApplymentDate(f.ContactIDDocPeriodBegin)
		periodEnd := normalizeApplymentDate(f.ContactIDDocPeriodEnd)
		if err := validateApplymentIDCardValidity(periodBegin, periodEnd); err != nil {
			return resolvedApplymentContact{}, fmt.Errorf("contact_id_doc_period is invalid")
		}

		return resolvedApplymentContact{
			ContactType:                 "SUPER",
			ContactName:                 f.ContactName,
			ContactIDDocType:            contactIDDocType,
			ContactIDCardNumber:         f.ContactIDCardNumber,
			ContactIDDocCopyAssetID:     f.ContactIDDocCopyAssetID,
			ContactIDDocCopyBackAssetID: f.ContactIDDocCopyBackAssetID,
			ContactIDDocPeriodBegin:     periodBegin,
			ContactIDDocPeriodEnd:       periodEnd,
		}, nil
	default:
		return resolvedApplymentContact{}, fmt.Errorf("unsupported contact_type: %s", f.ContactType)
	}
}

func buildWechatApplymentContactInfo(contact resolvedApplymentContact, encryptedContactName, encryptedContactIDCardNumber, encryptedMobilePhone string) *wechat.ApplymentContactInfo {
	info := &wechat.ApplymentContactInfo{
		ContactType:         contact.ContactType,
		ContactName:         encryptedContactName,
		ContactIDCardNumber: encryptedContactIDCardNumber,
		MobilePhone:         encryptedMobilePhone,
	}

	if contact.ContactIDDocType != "" {
		info.ContactIDDocType = contact.ContactIDDocType
	}
	if contact.ContactIDDocPeriodBegin != "" {
		info.ContactIDDocPeriodBegin = contact.ContactIDDocPeriodBegin
	}
	if contact.ContactIDDocPeriodEnd != "" {
		info.ContactIDDocPeriodEnd = contact.ContactIDDocPeriodEnd
	}

	return info
}

func (c resolvedApplymentContact) requiresIdentityDocumentAssets() bool {
	return c.ContactType == "SUPER"
}

func (server *Server) validateApplymentContactDocumentAsset(ctx context.Context, userID, assetID int64, expectedCategory media.Category) error {
	asset, err := server.store.GetMediaAssetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("contact document asset %d not found", assetID)
	}
	if asset.UploadedBy != userID {
		return fmt.Errorf("contact document asset %d does not belong to current user", assetID)
	}
	if asset.Visibility != string(media.VisibilityPrivate) {
		return fmt.Errorf("contact document asset %d must be private", assetID)
	}
	if asset.MediaCategory != string(expectedCategory) {
		return fmt.Errorf("contact document asset %d category is invalid", assetID)
	}
	if asset.UploadStatus == "deleted" {
		return fmt.Errorf("contact document asset %d is unavailable", assetID)
	}
	return nil
}

func (f applymentBindBankFields) validateSelection() error {
	if f.AccountBank == "" {
		return fmt.Errorf("account_bank is required")
	}
	if f.AccountNumber == "" {
		return fmt.Errorf("account_number is required")
	}
	if f.AccountName == "" {
		return fmt.Errorf("account_name is required")
	}
	if f.NeedBankBranch {
		if f.BankAliasCode == "" {
			return fmt.Errorf("bank_alias_code is required when bank branch selection is needed")
		}
		if f.BankAddressCode == "" {
			return fmt.Errorf("bank_address_code is required when bank branch selection is needed")
		}
		if f.BankBranchID == "" {
			return fmt.Errorf("bank_branch_id is required when bank branch selection is needed")
		}
		if f.BankName == "" {
			return fmt.Errorf("bank_name is required when bank branch selection is needed")
		}
	}
	return nil
}

func (f applymentBindBankFields) toWechatAccountInfo(encryptedAccountName, encryptedAccountNumber string) *wechat.ApplymentBankAccountInfo {
	return &wechat.ApplymentBankAccountInfo{
		BankAccountType: f.AccountType,
		AccountBank:     f.AccountBank,
		AccountBankCode: f.AccountBankCode,
		AccountName:     encryptedAccountName,
		BankAddressCode: f.BankAddressCode,
		BankBranchID:    f.BankBranchID,
		BankName:        f.BankName,
		AccountNumber:   encryptedAccountNumber,
	}
}

// merchantBindBankRequest 商户绑定银行卡请求
type merchantBindBankRequest struct {
	applymentBindBankFields
	applymentContactFields
}

// merchantBindBankResponse 商户绑定银行卡响应
type merchantBindBankResponse struct {
	ApplymentID int64  `json:"applyment_id"` // 微信申请单号
	Status      string `json:"status"`       // 状态
	Message     string `json:"message"`      // 消息
}

// @Summary 商户绑定银行卡并提交微信开户
// @Description 商户审核通过后，绑定银行卡信息并提交微信二级商户进件
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body merchantBindBankRequest true "银行卡信息"
// @Success 200 {object} merchantBindBankResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/merchant/applyment/bindbank [post]
// @Security BearerAuth
func (server *Server) merchantBindBank(ctx *gin.Context) {
	var req merchantBindBankRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	req.applymentBindBankFields.normalize()
	req.applymentContactFields.normalize()
	if err := req.validateSelection(); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查商户状态：必须是 approved 或 pending_bindbank
	if merchant.Status != "approved" && merchant.Status != "pending_bindbank" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid merchant status: %s", merchant.Status)))
		return
	}

	// 检查是否已有进行中的进件申请
	existingApplyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	if err == nil {
		// 已有申请，检查状态
		if isApplymentPendingSubmissionStatus(existingApplyment.Status) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountApplymentPending))
			return
		}
		if existingApplyment.Status == "finish" {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountAlreadyRegistered))
			return
		}
	}

	// 获取商户申请信息（包含营业执照、身份证等OCR数据）
	application, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取商户申请信息失败: %w", err)))
		return
	}

	contactPhone, err := server.resolveApplymentContactPhone(ctx, authPayload.UserID, application.ContactPhone, merchant.Phone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var idCardBackOCR MerchantIDCardOCRData
	if len(application.IDCardBackOcr) > 0 {
		if err := json.Unmarshal(application.IDCardBackOcr, &idCardBackOCR); err != nil {
			log.Error().Err(err).Msg("解析身份证OCR失败")
		}
	}

	var businessLicenseOCR BusinessLicenseOCRData
	if len(application.BusinessLicenseOcr) > 0 {
		if err := json.Unmarshal(application.BusinessLicenseOcr, &businessLicenseOCR); err != nil {
			log.Error().Err(err).Msg("解析营业执照OCR失败")
		}
	}
	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("M%d%d", merchant.ID, time.Now().Unix())

	organizationType := resolveApplymentOrganizationType(
		application.BusinessLicenseNumber,
		businessLicenseOCR.TypeOfEnterprise,
		application.MerchantName,
		"4",
	)
	if err := validateMerchantApplymentScope(organizationType, req.AccountType); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := validateApplymentBusinessLicenseValidity(businessLicenseOCR.ValidPeriod); err != nil {
		log.Warn().Int64("merchant_id", merchant.ID).Str("valid_period", businessLicenseOCR.ValidPeriod).Msg("商户营业期限无效，拒绝提交微信进件")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	idCardValidTimeBegin, idCardValidTime := parseIDCardValidPeriod(idCardBackOCR.ValidDate)
	if err := validateApplymentIDCardValidity(idCardValidTimeBegin, idCardValidTime); err != nil {
		log.Warn().Int64("merchant_id", merchant.ID).Str("valid_date", idCardBackOCR.ValidDate).Msg("商户身份证有效期无效，拒绝提交微信进件")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	resolvedContact, err := req.applymentContactFields.resolve(application.LegalPersonName, application.LegalPersonIDNumber)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if resolvedContact.requiresIdentityDocumentAssets() {
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyAssetID, media.CategoryIDCardFront); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyBackAssetID, media.CategoryIDCardBack); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}

	// 加密敏感数据（本地存储）
	encryptedIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, application.LegalPersonIDNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedAccountNumber, err := util.EncryptSensitiveField(server.dataEncryptor, req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, resolvedContact.ContactIDCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密联系人身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	// 解析媒体资产 URL 用于存档
	bizLicenseURL := server.publicImageURL(ctx, pgInt8ToPtr(application.BusinessLicenseMediaAssetID), media.VariantOriginal)
	idCardFrontURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardFrontMediaAssetID), media.VariantOriginal)
	idCardBackURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardBackMediaAssetID), media.VariantOriginal)

	// 创建进件记录
	applyment, err := server.store.CreateEcommerceApplyment(ctx, db.CreateEcommerceApplymentParams{
		SubjectType:           "merchant",
		SubjectID:             merchant.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: pgtype.Text{String: application.BusinessLicenseNumber, Valid: application.BusinessLicenseNumber != ""},
		BusinessLicenseCopy:   pgtype.Text{String: bizLicenseURL, Valid: bizLicenseURL != ""},
		MerchantName:          application.MerchantName,
		LegalPerson:           application.LegalPersonName,
		IDCardNumber:          encryptedIDCardNumber, // AES 加密存储
		IDCardName:            application.LegalPersonName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       idCardFrontURL,
		IDCardBackCopy:        idCardBackURL,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		AccountBankCode:       pgtype.Int8{Int64: req.AccountBankCode, Valid: req.AccountBankCode > 0},
		BankAlias:             pgtype.Text{String: req.BankAlias, Valid: req.BankAlias != ""},
		BankAliasCode:         pgtype.Text{String: req.BankAliasCode, Valid: req.BankAliasCode != ""},
		BankAddressCode:       req.BankAddressCode,
		BankBranchID:          pgtype.Text{String: req.BankBranchID, Valid: req.BankBranchID != ""},
		BankName:              pgtype.Text{String: req.BankName, Valid: req.BankName != ""},
		AccountNumber:         encryptedAccountNumber, // AES 加密存储
		AccountName:           req.AccountName,
		ContactName:           resolvedContact.ContactName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true}, // AES 加密存储
		MobilePhone:           contactPhone,
		MerchantShortname:     merchant.Name,
		Qualifications:        []byte("[]"),
		BusinessAdditionPics:  []string{},
		BusinessAdditionDesc:  pgtype.Text{},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("创建进件记录失败: %w", err)))
		return
	}

	// 检查是否配置了微信支付客户端
	if server.ecommerceClient == nil {
		// 没有配置微信客户端，仅保存记录，不提交微信
		log.Warn().Msg("微信支付客户端未配置，跳过提交微信进件")

		// 更新商户状态
		_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
			ID:     merchant.ID,
			Status: "bindbank_submitted",
		})
		if err != nil {
			log.Error().Err(err).Msg("更新商户状态失败")
		}

		ctx.JSON(http.StatusOK, merchantBindBankResponse{
			ApplymentID: applyment.ID,
			Status:      "submitted",
			Message:     "银行卡信息已保存，待人工处理",
		})
		return
	}

	// ==================== 下载证件图片并上传到微信获取 MediaID ====================
	idCardFrontMediaID := ""
	if application.IDCardFrontMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.IDCardFrontMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载商户身份证正面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传商户身份证正面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		idCardFrontMediaID = upResp.MediaID
	}

	idCardBackMediaID := ""
	if application.IDCardBackMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.IDCardBackMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载商户身份证背面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传商户身份证背面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		idCardBackMediaID = upResp.MediaID
	}

	var businessLicenseMediaID string
	if application.BusinessLicenseMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.BusinessLicenseMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载商户营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取营业执照图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传商户营业执照到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		businessLicenseMediaID = upResp.MediaID
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	// 加密身份证姓名
	wxEncryptedIDCardName, err := server.ecommerceClient.EncryptSensitiveData(application.LegalPersonName)
	if err != nil {
		log.Error().Err(err).Msg("加密身份证姓名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密身份证号码
	wxEncryptedIDCardNumber, err := server.ecommerceClient.EncryptSensitiveData(application.LegalPersonIDNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密身份证号码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	wxEncryptedContactName, err := server.ecommerceClient.EncryptSensitiveData(resolvedContact.ContactName)
	if err != nil {
		log.Error().Err(err).Msg("加密联系人姓名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	wxEncryptedContactIDCardNumber, err := server.ecommerceClient.EncryptSensitiveData(resolvedContact.ContactIDCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密联系人身份证号码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账户名
	wxEncryptedAccountName, err := server.ecommerceClient.EncryptSensitiveData(req.AccountName)
	if err != nil {
		log.Error().Err(err).Msg("加密银行账户名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账号
	wxEncryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密联系手机号
	wxEncryptedMobilePhone, err := server.ecommerceClient.EncryptSensitiveData(contactPhone)
	if err != nil {
		log.Error().Err(err).Msg("加密联系手机号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// ==================== 构建微信进件请求 ====================
	businessLicenseInfo := buildApplymentBusinessLicenseInfo(
		businessLicenseMediaID,
		application.BusinessLicenseNumber,
		application.MerchantName,
		application.LegalPersonName,
		application.BusinessAddress,
		&businessLicenseOCR,
	)
	storeName := strings.TrimSpace(application.MerchantName)
	if storeName == "" {
		storeName = merchant.Name
	}
	storeQRCodeObjectKey, err := server.ensureMerchantStorefrontQRCode(ctx, authPayload.UserID, merchant.ID)
	if err != nil {
		log.Error().Err(err).Msg("生成商户店铺首页小程序码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("生成店铺首页二维码失败: %w", err)))
		return
	}
	storeQRCodeReader, err := server.mediaStorage.ReadObject(ctx, server.mediaStorage.PublicBucket(), storeQRCodeObjectKey)
	if err != nil {
		log.Error().Err(err).Str("object_key", storeQRCodeObjectKey).Msg("读取商户店铺首页小程序码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("读取店铺首页二维码失败: %w", err)))
		return
	}
	storeQRCodeData, err := io.ReadAll(storeQRCodeReader)
	_ = storeQRCodeReader.Close()
	if err != nil {
		log.Error().Err(err).Str("object_key", storeQRCodeObjectKey).Msg("读取商户店铺首页小程序码内容失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("读取店铺首页二维码失败: %w", err)))
		return
	}
	storeQRCodeUploadResp, err := server.ecommerceClient.UploadImage(ctx, path.Base(storeQRCodeObjectKey), storeQRCodeData)
	if err != nil {
		log.Error().Err(err).Msg("上传商户店铺首页小程序码到微信失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传店铺首页二维码失败: %w", err)))
		return
	}

	contactInfo := buildWechatApplymentContactInfo(resolvedContact, wxEncryptedContactName, wxEncryptedContactIDCardNumber, wxEncryptedMobilePhone)
	if resolvedContact.requiresIdentityDocumentAssets() {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, resolvedContact.ContactIDDocCopyAssetID)
		if dlErr != nil {
			log.Error().Err(dlErr).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("下载超级管理员身份证人像面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("上传超级管理员身份证人像面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传超级管理员证件图片失败")))
			return
		}
		contactInfo.ContactIDDocCopy = upResp.MediaID

		fname, fdata, dlErr = server.mediaRegistry.DownloadObject(ctx, resolvedContact.ContactIDDocCopyBackAssetID)
		if dlErr != nil {
			log.Error().Err(dlErr).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("下载超级管理员身份证国徽面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
		upResp, upErr = server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("上传超级管理员身份证国徽面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传超级管理员证件图片失败")))
			return
		}
		contactInfo.ContactIDDocCopyBack = upResp.MediaID
	}

	applymentReq := &wechat.EcommerceApplymentRequest{
		OutRequestNo:       outRequestNo,
		OrganizationType:   organizationType,
		FinanceInstitution: false,
		BusinessLicense:    businessLicenseInfo,
		MerchantShortname:  merchant.Name,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:           idCardFrontMediaID,
			IDCardNational:       idCardBackMediaID,
			IDCardName:           wxEncryptedIDCardName,
			IDCardNumber:         wxEncryptedIDCardNumber,
			IDCardValidTimeBegin: idCardValidTimeBegin,
			IDCardValidTime:      idCardValidTime,
		},
		AccountInfo: req.toWechatAccountInfo(wxEncryptedAccountName, wxEncryptedAccountNumber),
		ContactInfo: contactInfo,
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName:   storeName,
			StoreQRCode: storeQRCodeUploadResp.MediaID,
		},
	}

	// 提交微信进件
	resp, err := server.ecommerceClient.CreateEcommerceApplyment(ctx, applymentReq)
	if err != nil {
		log.Error().Err(err).Msg("提交微信进件失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("提交微信开户失败: %w", err)))
		return
	}

	// 更新进件记录状态
	_, err = server.store.UpdateEcommerceApplymentToSubmitted(ctx, db.UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: resp.ApplymentID, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Msg("更新进件状态失败")
	}

	// 更新商户状态为 bindbank_submitted
	_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "bindbank_submitted",
	})
	if err != nil {
		log.Error().Err(err).Msg("更新商户状态失败")
	}

	ctx.JSON(http.StatusOK, merchantBindBankResponse{
		ApplymentID: resp.ApplymentID,
		Status:      "submitted",
		Message:     "开户申请已提交，请等待微信审核（通常1-3个工作日）",
	})
}

// merchantApplymentStatusResponse 商户开户状态响应
type applymentAccountValidationResponse struct {
	AccountName              string `json:"account_name,omitempty"`
	AccountNo                string `json:"account_no,omitempty"`
	PayAmount                int64  `json:"pay_amount,omitempty"`
	DestinationAccountNumber string `json:"destination_account_number,omitempty"`
	DestinationAccountName   string `json:"destination_account_name,omitempty"`
	DestinationAccountBank   string `json:"destination_account_bank,omitempty"`
	City                     string `json:"city,omitempty"`
	Remark                   string `json:"remark,omitempty"`
	Deadline                 string `json:"deadline,omitempty"`
}

type merchantApplymentStatusResponse struct {
	Status             string                              `json:"status"`                         // 状态
	StatusDesc         string                              `json:"status_desc"`                    // 状态描述
	CanSubmit          bool                                `json:"can_submit"`                     // 是否允许提交或重新提交进件
	BlockReason        string                              `json:"block_reason,omitempty"`         // 不允许提交时的阻塞原因
	SignURL            *string                             `json:"sign_url,omitempty"`             // 签约链接
	SignState          *string                             `json:"sign_state,omitempty"`           // 签约状态
	LegalValidationURL *string                             `json:"legal_validation_url,omitempty"` // 法人扫码验证链接
	AccountValidation  *applymentAccountValidationResponse `json:"account_validation,omitempty"`   // 汇款验证信息
	SubMchID           *string                             `json:"sub_mch_id,omitempty"`           // 二级商户号
	RejectReason       *string                             `json:"reject_reason,omitempty"`        // 拒绝原因
}

type applymentSensitiveDecryptor interface {
	DecryptSensitiveResponseData(ciphertext string) (string, error)
}

func resolveApplymentSensitiveDecryptor(client wechat.EcommerceClientInterface) applymentSensitiveDecryptor {
	if client == nil {
		return nil
	}
	decryptor, ok := client.(applymentSensitiveDecryptor)
	if !ok {
		return nil
	}
	return decryptor
}

func decryptApplymentSensitiveField(decryptor applymentSensitiveDecryptor, ciphertext string) string {
	trimmedCiphertext := strings.TrimSpace(ciphertext)
	if trimmedCiphertext == "" {
		return ""
	}
	if decryptor == nil {
		return ""
	}

	plaintext, err := decryptor.DecryptSensitiveResponseData(trimmedCiphertext)
	if err != nil {
		log.Warn().Err(err).Msg("failed to decrypt applyment sensitive response field")
		return ""
	}
	return strings.TrimSpace(plaintext)
}

func buildApplymentAccountValidationResponse(validation *wechat.EcommerceApplymentAccountValidation, decryptor applymentSensitiveDecryptor) *applymentAccountValidationResponse {
	if validation == nil {
		return nil
	}

	return &applymentAccountValidationResponse{
		AccountName:              decryptApplymentSensitiveField(decryptor, validation.AccountName),
		AccountNo:                decryptApplymentSensitiveField(decryptor, validation.AccountNo),
		PayAmount:                validation.PayAmount,
		DestinationAccountNumber: strings.TrimSpace(validation.DestinationAccountNumber),
		DestinationAccountName:   strings.TrimSpace(validation.DestinationAccountName),
		DestinationAccountBank:   strings.TrimSpace(validation.DestinationAccountBank),
		City:                     strings.TrimSpace(validation.City),
		Remark:                   strings.TrimSpace(validation.Remark),
		Deadline:                 strings.TrimSpace(validation.Deadline),
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

func buildApplymentText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func applymentTextChanged(current, next pgtype.Text) bool {
	if current.Valid != next.Valid {
		return true
	}
	if !current.Valid {
		return false
	}
	return current.String != next.String
}

// @Summary 查询商户开户状态
// @Description 查询商户微信二级商户进件状态
// @Tags 商户
// @Accept json
// @Produce json
// @Success 200 {object} merchantApplymentStatusResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/merchant/applyment/status [get]
// @Security BearerAuth
func (server *Server) getMerchantApplymentStatus(ctx *gin.Context) {
	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取最新进件记录
	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			status := mapMerchantStatusToApplymentStatus(merchant.Status)
			canSubmit, blockReason := getMerchantApplymentSubmitCapability(merchant.Status, status)
			ctx.JSON(http.StatusOK, merchantApplymentStatusResponse{
				Status:      status,
				StatusDesc:  getApplymentStatusDesc(status),
				CanSubmit:   canSubmit,
				BlockReason: blockReason,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var remoteStatusDesc string
	var remoteLegalValidationURL string
	var remoteAccountValidation *applymentAccountValidationResponse
	decryptor := resolveApplymentSensitiveDecryptor(server.ecommerceClient)

	// 如果有微信申请单号且客户端可用，查询最新状态
	if applyment.ApplymentID.Valid && server.ecommerceClient != nil {
		wxResp, err := server.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("查询微信进件状态失败")
		} else {
			updateStatus := mapWechatApplymentStatus(wxResp.ApplymentState)
			nextRejectReason := getRejectReasonFromAuditDetail(wxResp.AuditDetail)
			nextSignURL := buildApplymentText(wxResp.SignURL)
			nextSignState := buildApplymentText(wxResp.SignState)
			nextSubMchID := buildApplymentText(wxResp.SubMchID)

			if updateStatus != applyment.Status ||
				applymentTextChanged(applyment.RejectReason, nextRejectReason) ||
				applymentTextChanged(applyment.SignUrl, nextSignURL) ||
				applymentTextChanged(applyment.SignState, nextSignState) ||
				applymentTextChanged(applyment.SubMchID, nextSubMchID) {
				_, err = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
					ID:           applyment.ID,
					Status:       updateStatus,
					RejectReason: nextRejectReason,
					SignUrl:      nextSignURL,
					SignState:    nextSignState,
					SubMchID:     nextSubMchID,
				})
				if err != nil {
					log.Error().Err(err).Msg("更新进件状态失败")
				}
			}

			applyment.Status = updateStatus
			applyment.RejectReason = nextRejectReason
			applyment.SignUrl = nextSignURL
			applyment.SignState = nextSignState
			applyment.SubMchID = nextSubMchID
			remoteStatusDesc = strings.TrimSpace(wxResp.ApplymentStateDesc)
			remoteLegalValidationURL = strings.TrimSpace(wxResp.LegalValidationURL)
			remoteAccountValidation = buildApplymentAccountValidationResponse(wxResp.AccountValidation, decryptor)

			// 如果开户成功，更新商户状态和支付配置
			if wxResp.SubMchID != "" && merchant.Status != "active" {
				_, err = server.store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
					MerchantID: merchant.ID,
					SubMchID:   wxResp.SubMchID,
					Status:     "active",
				})
				if err != nil {
					log.Error().Err(err).Msg("创建商户支付配置失败")
				}

				_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
					ID:     merchant.ID,
					Status: "active",
				})
				if err != nil {
					log.Error().Err(err).Msg("更新商户状态失败")
				}
			}
		}
	}

	normalizedStatus := normalizeApplymentStatus(applyment.Status, applyment.SubMchID.Valid && applyment.SubMchID.String != "")
	statusDesc := getApplymentStatusDesc(normalizedStatus)
	if remoteStatusDesc != "" {
		statusDesc = remoteStatusDesc
	}
	canSubmit, blockReason := getMerchantApplymentSubmitCapability(merchant.Status, normalizedStatus)
	resp := merchantApplymentStatusResponse{
		Status:      normalizedStatus,
		StatusDesc:  statusDesc,
		CanSubmit:   canSubmit,
		BlockReason: blockReason,
	}

	if applyment.SignUrl.Valid && applyment.SignUrl.String != "" {
		resp.SignURL = &applyment.SignUrl.String
	}
	if applyment.SignState.Valid && applyment.SignState.String != "" {
		resp.SignState = &applyment.SignState.String
	}
	if remoteLegalValidationURL != "" {
		resp.LegalValidationURL = &remoteLegalValidationURL
	}
	if remoteAccountValidation != nil {
		resp.AccountValidation = remoteAccountValidation
	}
	if applyment.SubMchID.Valid && applyment.SubMchID.String != "" {
		resp.SubMchID = &applyment.SubMchID.String
	}
	if applyment.RejectReason.Valid && applyment.RejectReason.String != "" {
		resp.RejectReason = &applyment.RejectReason.String
	}

	ctx.JSON(http.StatusOK, resp)
}

func parseIDCardValidPeriod(validDate string) (string, string) {
	begin, end := parseApplymentDateRange(validDate)
	if begin == "" && end == "" {
		return "", "长期"
	}
	return begin, end
}

func parseOperatorIDCardValidPeriod(ocr OperatorIDCardBackOCR) (string, string) {
	start := normalizeApplymentDate(ocr.ValidStart)
	end := normalizeApplymentDate(ocr.ValidEnd)
	if start != "" && end != "" {
		return start, end
	}

	if begin, rangedEnd := parseApplymentDateRange(ocr.ValidEnd); begin != "" || rangedEnd != "" {
		return begin, rangedEnd
	}

	if begin, rangedEnd := parseApplymentDateRange(ocr.ValidStart); begin != "" || rangedEnd != "" {
		return begin, rangedEnd
	}

	return start, end
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

func validateApplymentIDCardValidity(begin, end string) error {
	if begin == "" && end == "" {
		return ErrApplymentIDCardValidityInvalid
	}
	return validateApplymentDateWindow(begin, end, ErrApplymentIDCardValidityInvalid)
}

func validateApplymentBusinessLicenseValidity(validPeriod string) error {
	trimmed := strings.TrimSpace(validPeriod)
	if trimmed == "" {
		return nil
	}
	if normalizeApplymentDate(trimmed) == "长期" {
		return nil
	}
	begin, end := parseApplymentDateRange(trimmed)
	return validateApplymentDateWindow(begin, end, ErrApplymentBusinessLicenseValidityInvalid)
}

func validateApplymentDateWindow(begin, end string, invalidErr error) error {
	begin = strings.TrimSpace(begin)
	end = strings.TrimSpace(end)
	if begin == "" || end == "" {
		return invalidErr
	}

	parsedBegin, err := time.Parse("2006-01-02", begin)
	if err != nil {
		return invalidErr
	}

	minDate := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	if parsedBegin.Before(minDate) {
		return invalidErr
	}

	today, err := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	if err != nil {
		return invalidErr
	}
	if !parsedBegin.Before(today) {
		return invalidErr
	}

	if end == "长期" {
		return nil
	}

	parsedEnd, err := time.Parse("2006-01-02", end)
	if err != nil {
		return invalidErr
	}
	if !parsedEnd.After(parsedBegin) {
		return invalidErr
	}

	return nil
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

func buildApplymentBusinessLicenseInfo(copyMediaID, businessLicenseNumber, merchantName, legalPerson, fallbackAddress string, ocr *BusinessLicenseOCRData) *wechat.BusinessLicenseInfo {
	if copyMediaID == "" {
		return nil
	}

	info := &wechat.BusinessLicenseInfo{
		BusinessLicenseCopy:   copyMediaID,
		BusinessLicenseNumber: businessLicenseNumber,
		MerchantName:          merchantName,
		LegalPerson:           legalPerson,
	}

	if ocr != nil {
		if address := strings.TrimSpace(ocr.Address); address != "" {
			info.CompanyAddress = address
		}
		if businessTime := buildApplymentBusinessTime(ocr.ValidPeriod); businessTime != "" {
			info.BusinessTime = businessTime
		}
	}

	if info.CompanyAddress == "" {
		info.CompanyAddress = strings.TrimSpace(fallbackAddress)
	}

	return info
}

func buildApplymentBusinessTime(validPeriod string) string {
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

func buildApplymentStoreURL(config util.Config) string {
	if base := strings.TrimSpace(config.WebBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	if base := strings.TrimSpace(config.ExternalBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	return ""
}

func resolveApplymentOrganizationType(businessLicenseNumber, licenseType, subjectName, defaultLicensedType string) string {
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

func isMerchantApplymentOrganizationTypeSupported(organizationType string) bool {
	return organizationType == "2" || organizationType == "4"
}

func validateMerchantApplymentScope(organizationType, accountType string) error {
	if !isMerchantApplymentOrganizationTypeSupported(organizationType) {
		return ErrMerchantApplymentOrganizationUnsupported
	}
	if organizationType == "2" && accountType != "ACCOUNT_TYPE_BUSINESS" {
		return ErrApplymentEnterprisePublicAccountRequired
	}
	return nil
}

func operatorApplicationHasBusinessLicense(application db.OperatorApplication) bool {
	return application.BusinessLicenseNumber.Valid && strings.TrimSpace(application.BusinessLicenseNumber.String) != ""
}

func validateOperatorApplymentScope(application db.OperatorApplication, organizationType, accountType string) error {
	if !operatorApplicationHasBusinessLicense(application) {
		return ErrOperatorPersonalApplymentUnsupported
	}
	if organizationType != "2" {
		return ErrOperatorApplymentOrganizationUnsupported
	}
	if accountType != "ACCOUNT_TYPE_BUSINESS" {
		return ErrApplymentEnterprisePublicAccountRequired
	}
	return nil
}

func getPersonalOperatorApplymentBlockReason() string {
	return "个人运营商默认按微信 openid 分账，无需提交微信支付开户信息。"
}

// mapWechatApplymentStatus 映射微信进件状态到本地状态
func mapWechatApplymentStatus(wxStatus string) string {
	switch wxStatus {
	case "APPLYMENT_STATE_EDITTING":
		return "pending"
	case "CHECKING":
		return "checking"
	case "ACCOUNT_NEED_VERIFY":
		return "account_need_verify"
	case "APPLYMENT_STATE_AUDITING", "AUDITING":
		return "auditing"
	case "APPLYMENT_STATE_REJECTED", "REJECTED":
		return "rejected"
	case "APPLYMENT_STATE_CANCELED", "CANCELED":
		return "canceled"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED":
		return "to_be_confirmed"
	case "APPLYMENT_STATE_TO_BE_SIGNED", "NEED_SIGN":
		return "to_be_signed"
	case "APPLYMENT_STATE_SIGNING":
		return "signing"
	case "APPLYMENT_STATE_FINISHED", "FINISH":
		return "finish"
	case "APPLYMENT_STATE_FROZEN", "FROZEN":
		return "frozen"
	default:
		return "submitted"
	}
}

// getApplymentStatusDesc 获取进件状态描述
func getApplymentStatusDesc(status string) string {
	switch status {
	case "not_applied":
		return "尚未提交开户申请"
	case "pending":
		return "待提交"
	case "submitted":
		return "已提交，等待审核"
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

// getRejectReasonFromAuditDetail 从审核详情中获取拒绝原因
func getRejectReasonFromAuditDetail(details []wechat.ApplymentAuditDetail) pgtype.Text {
	if len(details) == 0 {
		return pgtype.Text{Valid: false}
	}

	reasons := ""
	for _, d := range details {
		if reasons != "" {
			reasons += "; "
		}
		reasons += fmt.Sprintf("%s: %s", d.ParamName, d.RejectReason)
	}
	return pgtype.Text{String: reasons, Valid: true}
}

// ==================== 运营商开户 ====================

// operatorBindBankRequest 运营商绑定银行卡请求
type operatorBindBankRequest struct {
	applymentBindBankFields
	applymentContactFields
}

// operatorBindBankResponse 运营商绑定银行卡响应
type operatorBindBankResponse struct {
	ApplymentID int64  `json:"applyment_id"` // 微信申请单号
	Status      string `json:"status"`       // 状态
	Message     string `json:"message"`      // 消息
}

// operatorBindBank godoc
// @Summary 运营商绑定银行卡并提交微信开户
// @Description 运营商审核通过后，绑定银行卡信息并提交微信二级商户进件
// @Tags 运营商
// @Accept json
// @Produce json
// @Param request body operatorBindBankRequest true "银行卡信息"
// @Success 200 {object} operatorBindBankResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operator/applyment/bindbank [post]
// @Security BearerAuth
func (server *Server) operatorBindBank(ctx *gin.Context) {
	var req operatorBindBankRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	req.applymentBindBankFields.normalize()
	req.applymentContactFields.normalize()
	if err := req.validateSelection(); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取运营商信息
	operator, err := server.store.GetOperatorByUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOperatorNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查运营商状态：已入驻（active）或绑卡进件进行中（bindbank_submitted）均可提交/重新提交绑卡
	if operator.Status != "active" && operator.Status != "bindbank_submitted" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid operator status: %s", operator.Status)))
		return
	}

	// 检查是否已有进行中的进件申请
	existingApplyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "operator",
		SubjectID:   operator.ID,
	})
	if err == nil {
		if isApplymentPendingSubmissionStatus(existingApplyment.Status) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountApplymentPending))
			return
		}
		if existingApplyment.Status == "finish" {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountAlreadyRegistered))
			return
		}
	}

	// 获取运营商审核通过的申请信息（包含营业执照、身份证等OCR数据）
	application, err := server.store.GetApprovedOperatorApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		log.Error().Err(err).Int64("operator_id", operator.ID).Msg("获取运营商申请信息失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取运营商申请信息失败: %w", err)))
		return
	}

	contactPhone, err := server.resolveApplymentContactPhone(ctx, authPayload.UserID, pgTextValue(application.ContactPhone), operator.ContactPhone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析身份证背面OCR信息（获取有效期）
	var idCardBackOCR OperatorIDCardBackOCR
	if len(application.IDCardBackOcr) > 0 {
		if err := json.Unmarshal(application.IDCardBackOcr, &idCardBackOCR); err != nil {
			log.Error().Err(err).Msg("解析运营商身份证背面OCR失败")
		}
	}

	var businessLicenseOCR BusinessLicenseOCRData
	if len(application.BusinessLicenseOcr) > 0 {
		if err := json.Unmarshal(application.BusinessLicenseOcr, &businessLicenseOCR); err != nil {
			log.Error().Err(err).Msg("解析运营商营业执照OCR失败")
		}
	}

	// 获取身份证信息
	legalPersonName := ""
	if application.LegalPersonName.Valid {
		legalPersonName = application.LegalPersonName.String
	}
	legalPersonIDNumber := ""
	if application.LegalPersonIDNumber.Valid {
		legalPersonIDNumber = application.LegalPersonIDNumber.String
	}
	idCardFrontURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardFrontMediaAssetID), media.VariantOriginal)
	idCardBackURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardBackMediaAssetID), media.VariantOriginal)
	businessLicenseURL := server.publicImageURL(ctx, pgInt8ToPtr(application.BusinessLicenseMediaAssetID), media.VariantOriginal)
	businessLicenseNumber := ""
	if application.BusinessLicenseNumber.Valid {
		businessLicenseNumber = application.BusinessLicenseNumber.String
	}
	operatorName := ""
	if application.Name.Valid {
		operatorName = application.Name.String
	}

	// 检查必要信息
	if legalPersonName == "" || legalPersonIDNumber == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrOperatorProfileIncomplete))
		return
	}

	idCardValidTimeBegin, idCardValidTime := parseOperatorIDCardValidPeriod(idCardBackOCR)
	if idCardValidTime == "" {
		idCardValidTime = "长期"
	}
	if err := validateApplymentIDCardValidity(idCardValidTimeBegin, idCardValidTime); err != nil {
		log.Warn().Int64("operator_id", operator.ID).Str("valid_start", idCardBackOCR.ValidStart).Str("valid_end", idCardBackOCR.ValidEnd).Msg("运营商身份证有效期无效，拒绝提交微信进件")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	resolvedContact, err := req.applymentContactFields.resolve(legalPersonName, legalPersonIDNumber)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if resolvedContact.requiresIdentityDocumentAssets() {
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyAssetID, media.CategoryIDCardFront); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyBackAssetID, media.CategoryIDCardBack); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}

	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("O%d%d", operator.ID, time.Now().Unix())
	organizationType := resolveApplymentOrganizationType(
		businessLicenseNumber,
		businessLicenseOCR.TypeOfEnterprise,
		operatorName,
		"2",
	)
	if err := validateOperatorApplymentScope(application, organizationType, req.AccountType); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := validateApplymentBusinessLicenseValidity(businessLicenseOCR.ValidPeriod); err != nil {
		log.Warn().Int64("operator_id", operator.ID).Str("valid_period", businessLicenseOCR.ValidPeriod).Msg("运营商营业期限无效，拒绝提交微信进件")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 加密敏感数据（本地存储）
	encryptedIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, legalPersonIDNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedAccountNumber, err := util.EncryptSensitiveField(server.dataEncryptor, req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, resolvedContact.ContactIDCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商联系人身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}

	// 创建进件记录
	applyment, err := server.store.CreateEcommerceApplyment(ctx, db.CreateEcommerceApplymentParams{
		SubjectType:           "operator",
		SubjectID:             operator.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: pgtype.Text{String: businessLicenseNumber, Valid: businessLicenseNumber != ""},
		BusinessLicenseCopy:   pgtype.Text{String: businessLicenseURL, Valid: businessLicenseURL != ""},
		MerchantName:          operatorName,
		LegalPerson:           legalPersonName,
		IDCardNumber:          encryptedIDCardNumber,
		IDCardName:            legalPersonName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       idCardFrontURL,
		IDCardBackCopy:        idCardBackURL,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		AccountBankCode:       pgtype.Int8{Int64: req.AccountBankCode, Valid: req.AccountBankCode > 0},
		BankAlias:             pgtype.Text{String: req.BankAlias, Valid: req.BankAlias != ""},
		BankAliasCode:         pgtype.Text{String: req.BankAliasCode, Valid: req.BankAliasCode != ""},
		BankAddressCode:       req.BankAddressCode,
		BankBranchID:          pgtype.Text{String: req.BankBranchID, Valid: req.BankBranchID != ""},
		BankName:              pgtype.Text{String: req.BankName, Valid: req.BankName != ""},
		AccountNumber:         encryptedAccountNumber,
		AccountName:           req.AccountName,
		ContactName:           resolvedContact.ContactName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true},
		MobilePhone:           contactPhone,
		MerchantShortname:     operatorName,
		Qualifications:        []byte("[]"),
		BusinessAdditionPics:  []string{},
		BusinessAdditionDesc:  pgtype.Text{},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("创建进件记录失败: %w", err)))
		return
	}

	if server.ecommerceClient == nil {
		log.Warn().Msg("微信支付客户端未配置，跳过提交微信进件")
		_, _ = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
			ID:     operator.ID,
			Status: "bindbank_submitted",
		})

		ctx.JSON(http.StatusOK, operatorBindBankResponse{
			ApplymentID: applyment.ID,
			Status:      "submitted",
			Message:     "银行卡信息已保存，待人工处理",
		})
		return
	}

	// ==================== 下载证件图片并上传到微信获取 MediaID ====================
	idCardFrontMediaID := ""
	if application.IDCardFrontMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.IDCardFrontMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载运营商身份证正面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传运营商身份证正面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		idCardFrontMediaID = upResp.MediaID
	}

	idCardBackMediaID := ""
	if application.IDCardBackMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.IDCardBackMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载运营商身份证背面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传运营商身份证背面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		idCardBackMediaID = upResp.MediaID
	}

	var businessLicenseMediaID string
	if application.BusinessLicenseMediaAssetID.Valid {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, application.BusinessLicenseMediaAssetID.Int64)
		if dlErr != nil {
			log.Error().Err(dlErr).Msg("下载运营商营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取营业执照图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Msg("上传运营商营业执照到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传证件图片失败")))
			return
		}
		businessLicenseMediaID = upResp.MediaID
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	// 加密身份证姓名
	wxEncryptedIDCardName, err := server.ecommerceClient.EncryptSensitiveData(legalPersonName)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商身份证姓名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密身份证号码
	wxEncryptedIDCardNumber, err := server.ecommerceClient.EncryptSensitiveData(legalPersonIDNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商身份证号码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	wxEncryptedContactName, err := server.ecommerceClient.EncryptSensitiveData(resolvedContact.ContactName)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商联系人姓名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	wxEncryptedContactIDCardNumber, err := server.ecommerceClient.EncryptSensitiveData(resolvedContact.ContactIDCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商联系人身份证号码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账户名
	wxEncryptedAccountName, err := server.ecommerceClient.EncryptSensitiveData(req.AccountName)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商银行账户名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账号
	wxEncryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密联系手机号
	wxEncryptedMobilePhone, err := server.ecommerceClient.EncryptSensitiveData(contactPhone)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商联系手机号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// ==================== 构建微信进件请求 ====================
	businessLicenseInfo := buildApplymentBusinessLicenseInfo(
		businessLicenseMediaID,
		businessLicenseNumber,
		operatorName,
		legalPersonName,
		"",
		&businessLicenseOCR,
	)
	storeURL := buildApplymentStoreURL(server.config)
	if storeURL == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplymentWebSceneDomainRequired))
		return
	}

	contactInfo := buildWechatApplymentContactInfo(resolvedContact, wxEncryptedContactName, wxEncryptedContactIDCardNumber, wxEncryptedMobilePhone)
	if resolvedContact.requiresIdentityDocumentAssets() {
		fname, fdata, dlErr := server.mediaRegistry.DownloadObject(ctx, resolvedContact.ContactIDDocCopyAssetID)
		if dlErr != nil {
			log.Error().Err(dlErr).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("下载运营商超级管理员身份证人像面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
		upResp, upErr := server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("上传运营商超级管理员身份证人像面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传超级管理员证件图片失败")))
			return
		}
		contactInfo.ContactIDDocCopy = upResp.MediaID

		fname, fdata, dlErr = server.mediaRegistry.DownloadObject(ctx, resolvedContact.ContactIDDocCopyBackAssetID)
		if dlErr != nil {
			log.Error().Err(dlErr).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("下载运营商超级管理员身份证国徽面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
		upResp, upErr = server.ecommerceClient.UploadImage(ctx, fname, fdata)
		if upErr != nil {
			log.Error().Err(upErr).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("上传运营商超级管理员身份证国徽面到微信失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传超级管理员证件图片失败")))
			return
		}
		contactInfo.ContactIDDocCopyBack = upResp.MediaID
	}

	applymentReq := &wechat.EcommerceApplymentRequest{
		OutRequestNo:       outRequestNo,
		OrganizationType:   organizationType,
		FinanceInstitution: false,
		BusinessLicense:    businessLicenseInfo,
		MerchantShortname:  operatorName,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:           idCardFrontMediaID,
			IDCardNational:       idCardBackMediaID,
			IDCardName:           wxEncryptedIDCardName,
			IDCardNumber:         wxEncryptedIDCardNumber,
			IDCardValidTimeBegin: idCardValidTimeBegin,
			IDCardValidTime:      idCardValidTime,
		},
		AccountInfo: req.toWechatAccountInfo(wxEncryptedAccountName, wxEncryptedAccountNumber),
		ContactInfo: contactInfo,
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName: operatorName,
			StoreURL:  storeURL,
		},
	}

	// 提交微信进件
	resp, err := server.ecommerceClient.CreateEcommerceApplyment(ctx, applymentReq)
	if err != nil {
		log.Error().Err(err).Msg("提交微信运营商进件失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("提交微信开户失败: %w", err)))
		return
	}

	// 更新进件记录状态
	_, err = server.store.UpdateEcommerceApplymentToSubmitted(ctx, db.UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: resp.ApplymentID, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Msg("更新运营商进件状态失败")
	}

	// 更新运营商状态
	_, _ = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
		ID:     operator.ID,
		Status: "bindbank_submitted",
	})

	ctx.JSON(http.StatusOK, operatorBindBankResponse{
		ApplymentID: resp.ApplymentID,
		Status:      "submitted",
		Message:     "开户申请已提交，请等待微信审核（通常1-3个工作日）",
	})
}

// operatorApplymentStatusResponse 运营商开户状态响应
type operatorApplymentStatusResponse struct {
	Status             string                              `json:"status"`                         // 状态
	StatusDesc         string                              `json:"status_desc"`                    // 状态描述
	CanSubmit          bool                                `json:"can_submit"`                     // 是否允许提交或重新提交进件
	BlockReason        string                              `json:"block_reason,omitempty"`         // 不允许提交时的阻塞原因
	ApplymentID        *int64                              `json:"applyment_id,omitempty"`         // 微信进件ID
	SubMchID           string                              `json:"sub_mch_id,omitempty"`           // 二级商户号（开户成功后返回）
	SignURL            *string                             `json:"sign_url,omitempty"`             // 签约链接
	SignState          *string                             `json:"sign_state,omitempty"`           // 签约状态
	LegalValidationURL *string                             `json:"legal_validation_url,omitempty"` // 法人扫码验证链接
	AccountValidation  *applymentAccountValidationResponse `json:"account_validation,omitempty"`   // 汇款验证信息
	RejectReason       string                              `json:"reject_reason,omitempty"`        // 拒绝原因
	CreatedAt          time.Time                           `json:"created_at"`
	UpdatedAt          time.Time                           `json:"updated_at"`
}

func getOperatorApplymentStatusDesc(status string, canSubmit bool) string {
	if status == "active" && canSubmit {
		return "可提交开户信息"
	}
	if status == "active" && !canSubmit {
		return "账户已开通"
	}
	if status == "frozen" && !canSubmit {
		return "当前账号状态不可用"
	}
	return getApplymentStatusDesc(status)
}

// getOperatorApplymentStatus godoc
// @Summary 获取运营商开户状态
// @Description 获取运营商微信支付开户申请状态
// @Tags 运营商
// @Accept json
// @Produce json
// @Success 200 {object} operatorApplymentStatusResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operator/applyment/status [get]
// @Security BearerAuth
func (server *Server) getOperatorApplymentStatus(ctx *gin.Context) {
	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取运营商信息
	operator, err := server.store.GetOperatorByUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOperatorNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取最新进件记录
	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "operator",
		SubjectID:   operator.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			if operator.Status == "active" || operator.Status == "bindbank_submitted" {
				application, appErr := server.store.GetApprovedOperatorApplicationByUserID(ctx, authPayload.UserID)
				if appErr == nil && !operatorApplicationHasBusinessLicense(application) {
					updatedAt := operator.CreatedAt
					if operator.UpdatedAt.Valid {
						updatedAt = operator.UpdatedAt.Time
					}

					ctx.JSON(http.StatusOK, operatorApplymentStatusResponse{
						Status:      "active",
						StatusDesc:  "当前无需提交开户信息",
						CanSubmit:   false,
						BlockReason: getPersonalOperatorApplymentBlockReason(),
						CreatedAt:   operator.CreatedAt,
						UpdatedAt:   updatedAt,
					})
					return
				}
				if appErr != nil && !isNotFoundError(appErr) {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, appErr))
					return
				}
			}

			status := mapOperatorStatusToApplymentStatus(operator.Status)
			canSubmit, blockReason := getOperatorApplymentSubmitCapability(operator.Status, status)
			statusDesc := getOperatorApplymentStatusDesc(status, canSubmit)
			updatedAt := operator.CreatedAt
			if operator.UpdatedAt.Valid {
				updatedAt = operator.UpdatedAt.Time
			}

			ctx.JSON(http.StatusOK, operatorApplymentStatusResponse{
				Status:      status,
				StatusDesc:  statusDesc,
				CanSubmit:   canSubmit,
				BlockReason: blockReason,
				CreatedAt:   operator.CreatedAt,
				UpdatedAt:   updatedAt,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var remoteStatusDesc string
	var remoteLegalValidationURL string
	var remoteAccountValidation *applymentAccountValidationResponse
	decryptor := resolveApplymentSensitiveDecryptor(server.ecommerceClient)

	// 如果状态是已提交且配置了微信客户端，尝试查询微信最新状态
	if applyment.ApplymentID.Valid && server.ecommerceClient != nil {
		if applyment.Status == "submitted" || applyment.Status == "checking" || applyment.Status == "auditing" ||
			applyment.Status == "account_need_verify" || applyment.Status == "to_be_confirmed" ||
			applyment.Status == "to_be_signed" || applyment.Status == "signing" {
			wxResp, err := server.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
			if err == nil {
				newStatus := mapWechatApplymentStatus(wxResp.ApplymentState)
				nextRejectReason := getRejectReasonFromAuditDetail(wxResp.AuditDetail)
				nextSignURL := buildApplymentText(wxResp.SignURL)
				nextSignState := buildApplymentText(wxResp.SignState)
				nextSubMchID := buildApplymentText(wxResp.SubMchID)
				if newStatus == "rejected" || newStatus == "canceled" {
					_, _ = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
						ID:     operator.ID,
						Status: "active",
					})
				}
				if newStatus != applyment.Status ||
					applymentTextChanged(applyment.RejectReason, nextRejectReason) ||
					applymentTextChanged(applyment.SignUrl, nextSignURL) ||
					applymentTextChanged(applyment.SignState, nextSignState) ||
					applymentTextChanged(applyment.SubMchID, nextSubMchID) {
					_, _ = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
						ID:           applyment.ID,
						Status:       newStatus,
						RejectReason: nextRejectReason,
						SignUrl:      nextSignURL,
						SignState:    nextSignState,
						SubMchID:     nextSubMchID,
					})
				}

				if wxResp.SubMchID != "" {
					_, _ = server.store.UpdateOperatorSubMchID(ctx, db.UpdateOperatorSubMchIDParams{
						ID:       operator.ID,
						SubMchID: pgtype.Text{String: wxResp.SubMchID, Valid: true},
					})
					applyment.SubMchID = pgtype.Text{String: wxResp.SubMchID, Valid: true}
					operator.SubMchID = pgtype.Text{String: wxResp.SubMchID, Valid: true}
				}

				applyment.Status = newStatus
				applyment.RejectReason = nextRejectReason
				applyment.SignUrl = nextSignURL
				applyment.SignState = nextSignState
				remoteStatusDesc = strings.TrimSpace(wxResp.ApplymentStateDesc)
				remoteLegalValidationURL = strings.TrimSpace(wxResp.LegalValidationURL)
				remoteAccountValidation = buildApplymentAccountValidationResponse(wxResp.AccountValidation, decryptor)
			}
		}
	}

	effectiveSubMchID := resolveApplymentSubMchID(applyment.SubMchID, operator.SubMchID)
	normalizedStatus := normalizeApplymentStatus(applyment.Status, effectiveSubMchID != "")
	canSubmit, blockReason := getOperatorApplymentSubmitCapability(operator.Status, normalizedStatus)
	statusDesc := getOperatorApplymentStatusDesc(normalizedStatus, canSubmit)
	if remoteStatusDesc != "" {
		statusDesc = remoteStatusDesc
	}
	resp := operatorApplymentStatusResponse{
		Status:      normalizedStatus,
		StatusDesc:  statusDesc,
		CanSubmit:   canSubmit,
		BlockReason: blockReason,
		CreatedAt:   applyment.CreatedAt,
		UpdatedAt:   applyment.UpdatedAt,
	}

	if applyment.ApplymentID.Valid {
		resp.ApplymentID = &applyment.ApplymentID.Int64
	}
	if effectiveSubMchID != "" {
		resp.SubMchID = effectiveSubMchID
	}
	if applyment.SignUrl.Valid {
		resp.SignURL = &applyment.SignUrl.String
	}
	if applyment.SignState.Valid {
		resp.SignState = &applyment.SignState.String
	}
	if remoteLegalValidationURL != "" {
		resp.LegalValidationURL = &remoteLegalValidationURL
	}
	if remoteAccountValidation != nil {
		resp.AccountValidation = remoteAccountValidation
	}
	if applyment.RejectReason.Valid {
		resp.RejectReason = applyment.RejectReason.String
	}

	ctx.JSON(http.StatusOK, resp)
}

func normalizeApplymentStatus(status string, hasSubMchID bool) string {
	if status == "finish" && !hasSubMchID {
		return "submitted"
	}
	return status
}

func resolveApplymentSubMchID(applymentSubMchID, operatorSubMchID pgtype.Text) string {
	if applymentSubMchID.Valid && applymentSubMchID.String != "" {
		return applymentSubMchID.String
	}
	if operatorSubMchID.Valid && operatorSubMchID.String != "" {
		return operatorSubMchID.String
	}
	return ""
}

func getMerchantApplymentSubmitCapability(merchantStatus, applymentStatus string) (bool, string) {
	switch applymentStatus {
	case "not_applied", "pending", "rejected", "rejected_sign":
		if merchantStatus == "approved" || merchantStatus == "pending_bindbank" {
			return true, ""
		}
		if merchantStatus == "active" {
			return false, "当前账户已开通，无需重复提交进件资料。"
		}
		if merchantStatus == "suspended" || merchantStatus == "expired" {
			return false, "当前商户状态不可用，暂不支持提交收付通进件。"
		}
		return false, "当前商户状态暂不支持提交收付通进件。"
	case "canceled":
		if merchantStatus == "approved" || merchantStatus == "pending_bindbank" {
			return true, ""
		}
		if merchantStatus == "active" {
			return false, "当前账户已开通，无需重复提交进件资料。"
		}
		if merchantStatus == "suspended" || merchantStatus == "expired" {
			return false, "当前商户状态不可用，暂不支持提交收付通进件。"
		}
		return false, "当前商户状态暂不支持提交收付通进件。"
	case "submitted", "checking", "auditing", "bindbank_submitted":
		return false, "当前资料正在审核中，暂不支持重复提交。"
	case "account_need_verify":
		return false, "当前申请待账户验证，请先完成验证后再刷新状态。"
	case "to_be_confirmed":
		return false, "当前申请待确认，请先完成确认后再刷新状态。"
	case "to_be_signed", "signing":
		return false, "当前已进入微信签约环节，请先完成签约。"
	case "finish", "active":
		return false, "当前账户已开通，无需重复提交进件资料。"
	case "frozen":
		return false, "当前进件状态不可用，暂不支持提交收付通进件。"
	default:
		return false, "当前状态暂不支持提交收付通进件。"
	}
}

func getOperatorApplymentSubmitCapability(operatorStatus, applymentStatus string) (bool, string) {
	switch applymentStatus {
	case "pending", "active", "rejected", "rejected_sign", "canceled":
		if operatorStatus == "active" || operatorStatus == "bindbank_submitted" {
			return true, ""
		}
		if operatorStatus == "suspended" || operatorStatus == "expired" {
			return false, "当前运营商状态不可用，暂不支持提交微信支付开户。"
		}
		return false, "当前运营商状态暂不支持提交微信支付开户。"
	case "submitted", "checking", "auditing", "bindbank_submitted":
		return false, "微信支付正在审核开户信息，审核期间无需重复提交。"
	case "account_need_verify":
		return false, "微信支付要求先完成账户验证，请先处理验证后再刷新状态。"
	case "to_be_confirmed":
		return false, "微信支付要求先完成确认，请先处理后再刷新状态。"
	case "to_be_signed", "signing":
		return false, "微信支付已进入签约阶段，请先完成签约确认。"
	case "finish":
		return false, "微信支付商户已开通，无需重复提交开户信息。"
	case "frozen":
		return false, "当前运营商状态不可用，暂不支持提交微信支付开户。"
	default:
		return false, "当前状态暂不支持提交微信支付开户。"
	}
}

func mapMerchantStatusToApplymentStatus(merchantStatus string) string {
	switch merchantStatus {
	case "bindbank_submitted":
		return "submitted"
	case "active":
		return "active"
	default:
		return "not_applied"
	}
}

func mapOperatorStatusToApplymentStatus(operatorStatus string) string {
	switch operatorStatus {
	case "bindbank_submitted":
		return "submitted"
	case "active":
		return "active"
	case "suspended", "expired":
		return "frozen"
	default:
		// 其他状态视为尚未进入进件流程
		return "pending"
	}
}
