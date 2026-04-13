package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/merrydance/locallife/logic"
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

// merchantBindBankRequest 商户绑定银行卡请求
type merchantBindBankRequest struct {
	applymentBindBankFields
	applymentContactFields
}

// merchantBindBankResponse 商户绑定银行卡响应
type merchantBindBankResponse struct {
	ApplymentID        int64                               `json:"applyment_id"`                   // 微信申请单号
	Status             string                              `json:"status"`                         // 状态
	StatusDesc         string                              `json:"status_desc,omitempty"`          // 状态描述
	Message            string                              `json:"message"`                        // 消息
	SignURL            *string                             `json:"sign_url,omitempty"`             // 签约链接
	SignState          *string                             `json:"sign_state,omitempty"`           // 签约状态
	LegalValidationURL *string                             `json:"legal_validation_url,omitempty"` // 法人扫码验证链接
	AccountValidation  *applymentAccountValidationResponse `json:"account_validation,omitempty"`   // 汇款验证信息
	SubMchID           *string                             `json:"sub_mch_id,omitempty"`           // 二级商户号
	RejectReason       *string                             `json:"reject_reason,omitempty"`        // 拒绝原因
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
	// 检查是否已有进行中的进件申请
	existingApplyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	var existingApplymentRecord *db.EcommerceApplyment
	if err == nil {
		existingApplymentRecord = &existingApplyment
	}
	if validateErr := logic.ValidateMerchantApplymentSubmissionState(merchant.Status, existingApplymentRecord); validateErr != nil {
		switch {
		case errors.Is(validateErr, logic.ErrApplymentSubmissionPending):
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountApplymentPending))
		case errors.Is(validateErr, logic.ErrApplymentAlreadyRegistered):
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountAlreadyRegistered))
		default:
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid merchant status: %s", merchant.Status)))
		}
		return
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

	organizationType := logic.ResolveApplymentOrganizationType(
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

	idCardValidTimeBegin, idCardValidTime := logic.ParseApplymentIDCardValidPeriod(idCardBackOCR.ValidDate)
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
	// 解析媒体资产 URL 用于存档。
	// 这些快照字段不能作为未来重提微信进件时的 media_id 来源；若要重提，必须回到 media asset ID 并重新上传微信拿新的 media_id。
	bizLicenseURL := server.publicImageURL(ctx, pgInt8ToPtr(application.BusinessLicenseMediaAssetID), media.VariantOriginal)
	idCardFrontURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardFrontMediaAssetID), media.VariantOriginal)
	idCardBackURL := server.publicImageURL(ctx, pgInt8ToPtr(application.IDCardBackMediaAssetID), media.VariantOriginal)

	// 创建进件记录
	applyment, err := server.store.CreateEcommerceApplyment(ctx, logic.BuildCreateEcommerceApplymentParams(logic.ApplymentLocalRecordInput{
		SubjectType:           "merchant",
		SubjectID:             merchant.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: application.BusinessLicenseNumber,
		BusinessLicenseCopy:   bizLicenseURL,
		MerchantName:          application.MerchantName,
		LegalPerson:           application.LegalPersonName,
		IDCardNumber:          encryptedIDCardNumber,
		IDCardName:            application.LegalPersonName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       idCardFrontURL,
		IDCardBackCopy:        idCardBackURL,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		AccountBankCode:       req.AccountBankCode,
		BankAlias:             req.BankAlias,
		BankAliasCode:         req.BankAliasCode,
		BankAddressCode:       req.BankAddressCode,
		BankBranchID:          req.BankBranchID,
		BankName:              req.BankName,
		AccountNumber:         encryptedAccountNumber,
		AccountName:           req.AccountName,
		ContactName:           resolvedContact.ContactName,
		ContactIDCardNumber:   encryptedContactIDCardNumber,
		MobilePhone:           contactPhone,
		MerchantShortname:     merchant.Name,
	}))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("创建进件记录失败: %w", err)))
		return
	}
	updateMerchantStatus := func(updateCtx context.Context, status string) error {
		_, updateErr := server.store.UpdateMerchantStatus(updateCtx, db.UpdateMerchantStatusParams{
			ID:     merchant.ID,
			Status: status,
		})
		if updateErr != nil {
			log.Error().Err(updateErr).Msg("更新商户状态失败")
		}
		return updateErr
	}

	// 检查是否配置了微信支付客户端
	if server.ecommerceClient == nil {
		log.Warn().Msg("微信支付客户端未配置，跳过提交微信进件")
		submissionResult, submitErr := logic.SubmitEcommerceApplyment(ctx, server.store, nil, updateMerchantStatus, logic.SubmitEcommerceApplymentInput{
			Applyment: applyment,
		})
		if submitErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, submitErr))
			return
		}

		ctx.JSON(http.StatusOK, merchantBindBankResponse{
			ApplymentID: submissionResult.ApplymentID,
			Status:      submissionResult.Status,
			StatusDesc:  submissionResult.StatusDesc,
			Message:     submissionResult.Message,
		})
		return
	}

	// ==================== 下载证件图片并上传到微信获取 MediaID ====================
	idCardFrontMediaID := ""
	if application.IDCardFrontMediaAssetID.Valid {
		idCardFrontMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.IDCardFrontMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载商户身份证正面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
	}

	idCardBackMediaID := ""
	if application.IDCardBackMediaAssetID.Valid {
		idCardBackMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.IDCardBackMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载商户身份证背面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
	}

	var businessLicenseMediaID string
	if application.BusinessLicenseMediaAssetID.Valid {
		businessLicenseMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.BusinessLicenseMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载商户营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取营业执照图片失败")))
			return
		}
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	encryptedWechatFields, err := logic.EncryptApplymentWechatSensitiveFields(server.ecommerceClient, logic.ApplymentWechatSensitiveInput{
		IDCardName:          application.LegalPersonName,
		IDCardNumber:        application.LegalPersonIDNumber,
		ContactName:         resolvedContact.ContactName,
		ContactIDCardNumber: resolvedContact.ContactIDCardNumber,
		AccountName:         req.AccountName,
		AccountNumber:       req.AccountNumber,
		MobilePhone:         contactPhone,
	})
	if err != nil {
		var encryptionErr *logic.ApplymentSensitiveEncryptionError
		if errors.As(err, &encryptionErr) {
			log.Error().Err(err).Str("field", encryptionErr.Field).Msg("加密敏感信息失败")
		} else {
			log.Error().Err(err).Msg("加密敏感信息失败")
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// ==================== 构建微信进件请求 ====================
	businessLicenseInfo := logic.BuildApplymentBusinessLicenseInfo(
		businessLicenseMediaID,
		application.BusinessLicenseNumber,
		application.MerchantName,
		application.LegalPersonName,
		application.BusinessAddress,
		logic.ApplymentBusinessLicenseOCRInput{Address: businessLicenseOCR.Address, ValidPeriod: businessLicenseOCR.ValidPeriod},
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

	contactInfo := logic.BuildWechatApplymentContactInfo(logic.ApplymentWechatContactInput{
		ContactType:             resolvedContact.ContactType,
		ContactName:             encryptedWechatFields.ContactName,
		ContactIDDocType:        resolvedContact.ContactIDDocType,
		ContactIDCardNumber:     encryptedWechatFields.ContactIDCardNumber,
		ContactIDDocPeriodBegin: resolvedContact.ContactIDDocPeriodBegin,
		ContactIDDocPeriodEnd:   resolvedContact.ContactIDDocPeriodEnd,
		MobilePhone:             encryptedWechatFields.MobilePhone,
	})
	if resolvedContact.requiresIdentityDocumentAssets() {
		contactInfo.ContactIDDocCopy, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, resolvedContact.ContactIDDocCopyAssetID)
		if err != nil {
			log.Error().Err(err).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("下载超级管理员身份证人像面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}

		contactInfo.ContactIDDocCopyBack, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, resolvedContact.ContactIDDocCopyBackAssetID)
		if err != nil {
			log.Error().Err(err).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("下载超级管理员身份证国徽面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
	}

	applymentReq := logic.BuildWechatApplymentRequest(logic.ApplymentWechatRequestInput{
		OutRequestNo:      outRequestNo,
		OrganizationType:  organizationType,
		BusinessLicense:   businessLicenseInfo,
		MerchantShortname: merchant.Name,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:           idCardFrontMediaID,
			IDCardNational:       idCardBackMediaID,
			IDCardName:           encryptedWechatFields.IDCardName,
			IDCardNumber:         encryptedWechatFields.IDCardNumber,
			IDCardValidTimeBegin: idCardValidTimeBegin,
			IDCardValidTime:      idCardValidTime,
		},
		AccountInfo: logic.ApplymentWechatAccountInput{
			AccountType:     req.AccountType,
			AccountBank:     req.AccountBank,
			AccountBankCode: req.AccountBankCode,
			AccountName:     encryptedWechatFields.AccountName,
			BankAddressCode: req.BankAddressCode,
			BankBranchID:    req.BankBranchID,
			BankName:        req.BankName,
			AccountNumber:   encryptedWechatFields.AccountNumber,
		},
		ContactInfo: logic.ApplymentWechatContactInput{
			ContactType:             contactInfo.ContactType,
			ContactName:             contactInfo.ContactName,
			ContactIDDocType:        contactInfo.ContactIDDocType,
			ContactIDCardNumber:     contactInfo.ContactIDCardNumber,
			ContactIDDocPeriodBegin: contactInfo.ContactIDDocPeriodBegin,
			ContactIDDocPeriodEnd:   contactInfo.ContactIDDocPeriodEnd,
			ContactIDDocCopy:        contactInfo.ContactIDDocCopy,
			ContactIDDocCopyBack:    contactInfo.ContactIDDocCopyBack,
			MobilePhone:             contactInfo.MobilePhone,
		},
		StoreName:   storeName,
		StoreQRCode: storeQRCodeUploadResp.MediaID,
	})

	submissionResult, err := logic.SubmitEcommerceApplyment(ctx, server.store, server.ecommerceClient, updateMerchantStatus, logic.SubmitEcommerceApplymentInput{
		Applyment:     applyment,
		WechatRequest: applymentReq,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	snapshot := buildApplymentSubmissionStatusSnapshot(submissionResult, server.ecommerceClient)

	ctx.JSON(http.StatusOK, merchantBindBankResponse{
		ApplymentID:        submissionResult.ApplymentID,
		Status:             snapshot.Status,
		StatusDesc:         snapshot.StatusDesc,
		Message:            snapshot.Message,
		SignURL:            snapshot.SignURL,
		SignState:          snapshot.SignState,
		LegalValidationURL: snapshot.LegalValidationURL,
		AccountValidation:  snapshot.AccountValidation,
		SubMchID:           snapshot.SubMchID,
		RejectReason:       snapshot.RejectReason,
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

func buildStoredApplymentAccountValidationResponse(raw []byte, decryptor applymentSensitiveDecryptor) *applymentAccountValidationResponse {
	validation, err := wechat.UnmarshalEcommerceApplymentAccountValidation(raw)
	if err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal stored applyment account validation")
		return nil
	}

	return buildApplymentAccountValidationResponse(validation, decryptor)
}

type applymentSubmissionStatusSnapshot struct {
	Status             string
	StatusDesc         string
	Message            string
	SignURL            *string
	SignState          *string
	LegalValidationURL *string
	AccountValidation  *applymentAccountValidationResponse
	SubMchID           *string
	RejectReason       *string
}

func buildApplymentSubmissionStatusSnapshot(result logic.SubmitEcommerceApplymentResult, ecommerceClient wechat.EcommerceClientInterface) applymentSubmissionStatusSnapshot {
	snapshot := applymentSubmissionStatusSnapshot{
		Status:     result.Status,
		StatusDesc: result.StatusDesc,
		Message:    result.Message,
	}

	queryResp := result.InitialQueryResponse
	if queryResp == nil {
		return snapshot
	}

	decryptor := resolveApplymentSensitiveDecryptor(ecommerceClient)
	trimmedSignURL := strings.TrimSpace(queryResp.SignURL)
	if trimmedSignURL != "" {
		snapshot.SignURL = &trimmedSignURL
	}
	trimmedSignState := strings.TrimSpace(queryResp.SignState)
	if trimmedSignState != "" {
		snapshot.SignState = &trimmedSignState
	}
	trimmedLegalValidationURL := strings.TrimSpace(queryResp.LegalValidationURL)
	if trimmedLegalValidationURL != "" {
		snapshot.LegalValidationURL = &trimmedLegalValidationURL
	}
	trimmedSubMchID := strings.TrimSpace(queryResp.SubMchID)
	if trimmedSubMchID != "" {
		snapshot.SubMchID = &trimmedSubMchID
	}
	if accountValidation := buildApplymentAccountValidationResponse(queryResp.AccountValidation, decryptor); accountValidation != nil {
		snapshot.AccountValidation = accountValidation
	}
	if rejectReason := getRejectReasonFromAuditDetail(queryResp.AuditDetail); rejectReason.Valid {
		snapshot.RejectReason = &rejectReason.String
	}

	if snapshot.StatusDesc == "" {
		snapshot.StatusDesc = getApplymentStatusDesc(snapshot.Status)
	}
	if snapshot.Message == "" {
		snapshot.Message = snapshot.StatusDesc
	}

	return snapshot
}

func shouldQueryApplymentRemoteStatus(applyment db.EcommerceApplyment, subjectStatus string) bool {
	normalizedStatus := normalizeApplymentStatus(applyment.Status, applyment.SubMchID.Valid && strings.TrimSpace(applyment.SubMchID.String) != "")
	if normalizedStatus == "finish" {
		return applyment.ApplymentID.Valid || strings.TrimSpace(applyment.OutRequestNo) != ""
	}
	return logic.IsApplymentSubmissionInFlight(normalizedStatus, subjectStatus, applyment.OutRequestNo)
}

func (server *Server) queryEcommerceApplymentStatus(ctx context.Context, applyment db.EcommerceApplyment) (*wechat.EcommerceApplymentQueryResponse, error) {
	if server.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}

	if applyment.ApplymentID.Valid {
		resp, err := server.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
		if err == nil {
			log.Info().
				Int64("applyment_id", applyment.ID).
				Str("query_key", "applyment_id").
				Int64("wechat_applyment_id", resp.ApplymentID).
				Str("out_request_no", strings.TrimSpace(resp.OutRequestNo)).
				Str("applyment_state", strings.TrimSpace(resp.ApplymentState)).
				Str("sign_state", strings.TrimSpace(resp.SignState)).
				Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
				Bool("has_legal_validation_url", strings.TrimSpace(resp.LegalValidationURL) != "").
				Bool("has_account_validation", resp.AccountValidation != nil).
				Msg("query applyment status succeeded")
			return resp, nil
		}
		if strings.TrimSpace(applyment.OutRequestNo) == "" {
			return nil, err
		}

		log.Warn().Err(err).
			Int64("applyment_id", applyment.ID).
			Int64("wechat_applyment_id", applyment.ApplymentID.Int64).
			Str("out_request_no", applyment.OutRequestNo).
			Msg("query applyment by id failed, fallback to out_request_no")
	}

	if strings.TrimSpace(applyment.OutRequestNo) == "" {
		return nil, fmt.Errorf("applyment out_request_no is empty")
	}

	resp, err := server.ecommerceClient.QueryEcommerceApplymentByOutRequestNo(ctx, applyment.OutRequestNo)
	if err == nil {
		log.Info().
			Int64("applyment_id", applyment.ID).
			Str("query_key", "out_request_no").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("out_request_no", strings.TrimSpace(resp.OutRequestNo)).
			Str("applyment_state", strings.TrimSpace(resp.ApplymentState)).
			Str("sign_state", strings.TrimSpace(resp.SignState)).
			Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
			Bool("has_legal_validation_url", strings.TrimSpace(resp.LegalValidationURL) != "").
			Bool("has_account_validation", resp.AccountValidation != nil).
			Msg("query applyment status succeeded")
	}
	return resp, err
}

func buildApplymentText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func resolveRemoteApplymentStatus(currentStatus, remoteStatus string) string {
	if strings.TrimSpace(remoteStatus) == "" {
		return currentStatus
	}
	return remoteStatus
}

func shouldUseRemoteApplymentStatusDesc(status string, signState, legalValidationURL pgtype.Text, accountValidation []byte) bool {
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus == "" || normalizedStatus == "submitted" {
		return false
	}

	normalizedSignState := strings.ToUpper(strings.TrimSpace(signState.String))
	if signState.Valid && normalizedSignState == "UNSIGNED" {
		return false
	}

	hasLegalValidation := legalValidationURL.Valid && strings.TrimSpace(legalValidationURL.String) != ""
	hasAccountValidation := len(accountValidation) > 0
	if normalizedStatus == "account_need_verify" || normalizedStatus == "to_be_confirmed" || normalizedStatus == "to_be_signed" || normalizedStatus == "signing" || hasLegalValidation || hasAccountValidation {
		return false
	}

	return true
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

	// 进件处理中时优先查询微信实时状态；若本地丢失 applyment_id，则回退到 out_request_no。
	if server.ecommerceClient != nil && shouldQueryApplymentRemoteStatus(applyment, merchant.Status) {
		wxResp, err := server.queryEcommerceApplymentStatus(ctx, applyment)
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("查询微信进件状态失败")
		} else {
			if !applyment.ApplymentID.Valid && wxResp.ApplymentID > 0 {
				applyment.ApplymentID = pgtype.Int8{Int64: wxResp.ApplymentID, Valid: true}
			}

			updateStatus := resolveRemoteApplymentStatus(applyment.Status, mapWechatApplymentStatus(wxResp.ApplymentState))
			nextRejectReason := getRejectReasonFromAuditDetail(wxResp.AuditDetail)
			nextSignURL := buildApplymentText(wxResp.SignURL)
			nextSignState := buildApplymentText(wxResp.SignState)
			nextLegalValidationURL := buildApplymentText(wxResp.LegalValidationURL)
			nextAccountValidation := wechat.MarshalEcommerceApplymentAccountValidation(wxResp.AccountValidation)
			nextSubMchID := buildApplymentText(wxResp.SubMchID)

			if updateStatus != applyment.Status ||
				(!applyment.ApplymentID.Valid && wxResp.ApplymentID > 0) ||
				applymentTextChanged(applyment.RejectReason, nextRejectReason) ||
				applymentTextChanged(applyment.SignUrl, nextSignURL) ||
				applymentTextChanged(applyment.SignState, nextSignState) ||
				applymentTextChanged(applyment.LegalValidationUrl, nextLegalValidationURL) ||
				!bytes.Equal(applyment.AccountValidation, nextAccountValidation) ||
				applymentTextChanged(applyment.SubMchID, nextSubMchID) {
				_, err = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
					ID:                 applyment.ID,
					ApplymentID:        pgtype.Int8{Int64: wxResp.ApplymentID, Valid: wxResp.ApplymentID > 0},
					Status:             updateStatus,
					RejectReason:       nextRejectReason,
					SignUrl:            nextSignURL,
					SignState:          nextSignState,
					LegalValidationUrl: nextLegalValidationURL,
					AccountValidation:  nextAccountValidation,
					SubMchID:           nextSubMchID,
				})
				if err != nil {
					log.Error().Err(err).Msg("更新进件状态失败")
				}
			}

			applyment.Status = updateStatus
			applyment.RejectReason = nextRejectReason
			applyment.SignUrl = nextSignURL
			applyment.SignState = nextSignState
			applyment.LegalValidationUrl = nextLegalValidationURL
			applyment.AccountValidation = nextAccountValidation
			applyment.SubMchID = nextSubMchID
			if shouldUseRemoteApplymentStatusDesc(updateStatus, nextSignState, nextLegalValidationURL, nextAccountValidation) {
				remoteStatusDesc = strings.TrimSpace(wxResp.ApplymentStateDesc)
			}
			remoteLegalValidationURL = strings.TrimSpace(wxResp.LegalValidationURL)
			remoteAccountValidation = buildApplymentAccountValidationResponse(wxResp.AccountValidation, decryptor)

			// 仅在申请单真正完成时才激活支付能力；提前返回 sub_mch_id 只用于后续签约/验证流程。
			if updateStatus == "finish" && wxResp.SubMchID != "" && merchant.Status != "active" {
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
	if applyment.LegalValidationUrl.Valid && applyment.LegalValidationUrl.String != "" {
		resp.LegalValidationURL = &applyment.LegalValidationUrl.String
	}
	if storedAccountValidation := buildStoredApplymentAccountValidationResponse(applyment.AccountValidation, decryptor); storedAccountValidation != nil {
		resp.AccountValidation = storedAccountValidation
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
		return ""
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
	ApplymentID        int64                               `json:"applyment_id"`                   // 微信申请单号
	Status             string                              `json:"status"`                         // 状态
	StatusDesc         string                              `json:"status_desc,omitempty"`          // 状态描述
	Message            string                              `json:"message"`                        // 消息
	SignURL            *string                             `json:"sign_url,omitempty"`             // 签约链接
	SignState          *string                             `json:"sign_state,omitempty"`           // 签约状态
	LegalValidationURL *string                             `json:"legal_validation_url,omitempty"` // 法人扫码验证链接
	AccountValidation  *applymentAccountValidationResponse `json:"account_validation,omitempty"`   // 汇款验证信息
	SubMchID           *string                             `json:"sub_mch_id,omitempty"`           // 二级商户号
	RejectReason       *string                             `json:"reject_reason,omitempty"`        // 拒绝原因
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
	// 检查是否已有进行中的进件申请
	existingApplyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "operator",
		SubjectID:   operator.ID,
	})
	var existingOperatorApplyment *db.EcommerceApplyment
	if err == nil {
		existingOperatorApplyment = &existingApplyment
	}
	if validateErr := logic.ValidateOperatorApplymentSubmissionState(operator.Status, existingOperatorApplyment); validateErr != nil {
		switch {
		case errors.Is(validateErr, logic.ErrApplymentSubmissionPending):
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountApplymentPending))
		case errors.Is(validateErr, logic.ErrApplymentAlreadyRegistered):
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAccountAlreadyRegistered))
		default:
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid operator status: %s", operator.Status)))
		}
		return
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
	// 这些本地 URL 快照仅用于审计与状态展示，不能在未来重提时直接当成 WeChat media_id 使用。
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

	idCardValidTimeBegin, idCardValidTime := logic.ParseApplymentOperatorIDCardValidPeriod(idCardBackOCR.ValidStart, idCardBackOCR.ValidEnd)
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
	organizationType := logic.ResolveApplymentOrganizationType(
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
	applyment, err := server.store.CreateEcommerceApplyment(ctx, logic.BuildCreateEcommerceApplymentParams(logic.ApplymentLocalRecordInput{
		SubjectType:           "operator",
		SubjectID:             operator.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: businessLicenseNumber,
		BusinessLicenseCopy:   businessLicenseURL,
		MerchantName:          operatorName,
		LegalPerson:           legalPersonName,
		IDCardNumber:          encryptedIDCardNumber,
		IDCardName:            legalPersonName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       idCardFrontURL,
		IDCardBackCopy:        idCardBackURL,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		AccountBankCode:       req.AccountBankCode,
		BankAlias:             req.BankAlias,
		BankAliasCode:         req.BankAliasCode,
		BankAddressCode:       req.BankAddressCode,
		BankBranchID:          req.BankBranchID,
		BankName:              req.BankName,
		AccountNumber:         encryptedAccountNumber,
		AccountName:           req.AccountName,
		ContactName:           resolvedContact.ContactName,
		ContactIDCardNumber:   encryptedContactIDCardNumber,
		MobilePhone:           contactPhone,
		MerchantShortname:     operatorName,
	}))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("创建进件记录失败: %w", err)))
		return
	}
	updateOperatorStatus := func(updateCtx context.Context, status string) error {
		_, updateErr := server.store.UpdateOperatorStatus(updateCtx, db.UpdateOperatorStatusParams{
			ID:     operator.ID,
			Status: status,
		})
		if updateErr != nil {
			log.Error().Err(updateErr).Msg("更新运营商状态失败")
		}
		return updateErr
	}

	if server.ecommerceClient == nil {
		log.Warn().Msg("微信支付客户端未配置，跳过提交微信进件")
		submissionResult, submitErr := logic.SubmitEcommerceApplyment(ctx, server.store, nil, updateOperatorStatus, logic.SubmitEcommerceApplymentInput{
			Applyment: applyment,
		})
		if submitErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, submitErr))
			return
		}

		ctx.JSON(http.StatusOK, operatorBindBankResponse{
			ApplymentID: submissionResult.ApplymentID,
			Status:      submissionResult.Status,
			StatusDesc:  submissionResult.StatusDesc,
			Message:     submissionResult.Message,
		})
		return
	}

	// ==================== 下载证件图片并上传到微信获取 MediaID ====================
	idCardFrontMediaID := ""
	if application.IDCardFrontMediaAssetID.Valid {
		idCardFrontMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.IDCardFrontMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载运营商身份证正面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
	}

	idCardBackMediaID := ""
	if application.IDCardBackMediaAssetID.Valid {
		idCardBackMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.IDCardBackMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载运营商身份证背面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取身份证图片失败")))
			return
		}
	}

	var businessLicenseMediaID string
	if application.BusinessLicenseMediaAssetID.Valid {
		businessLicenseMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, application.BusinessLicenseMediaAssetID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("下载运营商营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取营业执照图片失败")))
			return
		}
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	encryptedWechatFields, err := logic.EncryptApplymentWechatSensitiveFields(server.ecommerceClient, logic.ApplymentWechatSensitiveInput{
		IDCardName:          legalPersonName,
		IDCardNumber:        legalPersonIDNumber,
		ContactName:         resolvedContact.ContactName,
		ContactIDCardNumber: resolvedContact.ContactIDCardNumber,
		AccountName:         req.AccountName,
		AccountNumber:       req.AccountNumber,
		MobilePhone:         contactPhone,
	})
	if err != nil {
		var encryptionErr *logic.ApplymentSensitiveEncryptionError
		if errors.As(err, &encryptionErr) {
			log.Error().Err(err).Str("field", encryptionErr.Field).Msg("加密敏感信息失败")
		} else {
			log.Error().Err(err).Msg("加密敏感信息失败")
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// ==================== 构建微信进件请求 ====================
	businessLicenseInfo := logic.BuildApplymentBusinessLicenseInfo(
		businessLicenseMediaID,
		businessLicenseNumber,
		operatorName,
		legalPersonName,
		"",
		logic.ApplymentBusinessLicenseOCRInput{Address: businessLicenseOCR.Address, ValidPeriod: businessLicenseOCR.ValidPeriod},
	)
	storeURL := logic.BuildApplymentStoreURL(server.config.WebBaseURL, server.config.ExternalBaseURL)
	if storeURL == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplymentWebSceneDomainRequired))
		return
	}

	contactInfo := logic.BuildWechatApplymentContactInfo(logic.ApplymentWechatContactInput{
		ContactType:             resolvedContact.ContactType,
		ContactName:             encryptedWechatFields.ContactName,
		ContactIDDocType:        resolvedContact.ContactIDDocType,
		ContactIDCardNumber:     encryptedWechatFields.ContactIDCardNumber,
		ContactIDDocPeriodBegin: resolvedContact.ContactIDDocPeriodBegin,
		ContactIDDocPeriodEnd:   resolvedContact.ContactIDDocPeriodEnd,
		MobilePhone:             encryptedWechatFields.MobilePhone,
	})
	if resolvedContact.requiresIdentityDocumentAssets() {
		contactInfo.ContactIDDocCopy, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, resolvedContact.ContactIDDocCopyAssetID)
		if err != nil {
			log.Error().Err(err).Int64("asset_id", resolvedContact.ContactIDDocCopyAssetID).Msg("下载运营商超级管理员身份证人像面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}

		contactInfo.ContactIDDocCopyBack, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, server.ecommerceClient, resolvedContact.ContactIDDocCopyBackAssetID)
		if err != nil {
			log.Error().Err(err).Int64("asset_id", resolvedContact.ContactIDDocCopyBackAssetID).Msg("下载运营商超级管理员身份证国徽面失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取超级管理员证件图片失败")))
			return
		}
	}

	applymentReq := logic.BuildWechatApplymentRequest(logic.ApplymentWechatRequestInput{
		OutRequestNo:      outRequestNo,
		OrganizationType:  organizationType,
		BusinessLicense:   businessLicenseInfo,
		MerchantShortname: operatorName,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:           idCardFrontMediaID,
			IDCardNational:       idCardBackMediaID,
			IDCardName:           encryptedWechatFields.IDCardName,
			IDCardNumber:         encryptedWechatFields.IDCardNumber,
			IDCardValidTimeBegin: idCardValidTimeBegin,
			IDCardValidTime:      idCardValidTime,
		},
		AccountInfo: logic.ApplymentWechatAccountInput{
			AccountType:     req.AccountType,
			AccountBank:     req.AccountBank,
			AccountBankCode: req.AccountBankCode,
			AccountName:     encryptedWechatFields.AccountName,
			BankAddressCode: req.BankAddressCode,
			BankBranchID:    req.BankBranchID,
			BankName:        req.BankName,
			AccountNumber:   encryptedWechatFields.AccountNumber,
		},
		ContactInfo: logic.ApplymentWechatContactInput{
			ContactType:             contactInfo.ContactType,
			ContactName:             contactInfo.ContactName,
			ContactIDDocType:        contactInfo.ContactIDDocType,
			ContactIDCardNumber:     contactInfo.ContactIDCardNumber,
			ContactIDDocPeriodBegin: contactInfo.ContactIDDocPeriodBegin,
			ContactIDDocPeriodEnd:   contactInfo.ContactIDDocPeriodEnd,
			ContactIDDocCopy:        contactInfo.ContactIDDocCopy,
			ContactIDDocCopyBack:    contactInfo.ContactIDDocCopyBack,
			MobilePhone:             contactInfo.MobilePhone,
		},
		StoreName: operatorName,
		StoreURL:  storeURL,
	})

	submissionResult, err := logic.SubmitEcommerceApplyment(ctx, server.store, server.ecommerceClient, updateOperatorStatus, logic.SubmitEcommerceApplymentInput{
		Applyment:     applyment,
		WechatRequest: applymentReq,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	snapshot := buildApplymentSubmissionStatusSnapshot(submissionResult, server.ecommerceClient)

	ctx.JSON(http.StatusOK, operatorBindBankResponse{
		ApplymentID:        submissionResult.ApplymentID,
		Status:             snapshot.Status,
		StatusDesc:         snapshot.StatusDesc,
		Message:            snapshot.Message,
		SignURL:            snapshot.SignURL,
		SignState:          snapshot.SignState,
		LegalValidationURL: snapshot.LegalValidationURL,
		AccountValidation:  snapshot.AccountValidation,
		SubMchID:           snapshot.SubMchID,
		RejectReason:       snapshot.RejectReason,
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

	// 进件处理中时优先查询微信实时状态；若本地丢失 applyment_id，则回退到 out_request_no。
	if server.ecommerceClient != nil && shouldQueryApplymentRemoteStatus(applyment, operator.Status) {
		wxResp, err := server.queryEcommerceApplymentStatus(ctx, applyment)
		if err == nil {
			if !applyment.ApplymentID.Valid && wxResp.ApplymentID > 0 {
				applyment.ApplymentID = pgtype.Int8{Int64: wxResp.ApplymentID, Valid: true}
			}

			newStatus := resolveRemoteApplymentStatus(applyment.Status, mapWechatApplymentStatus(wxResp.ApplymentState))
			nextRejectReason := getRejectReasonFromAuditDetail(wxResp.AuditDetail)
			nextSignURL := buildApplymentText(wxResp.SignURL)
			nextSignState := buildApplymentText(wxResp.SignState)
			nextLegalValidationURL := buildApplymentText(wxResp.LegalValidationURL)
			nextAccountValidation := wechat.MarshalEcommerceApplymentAccountValidation(wxResp.AccountValidation)
			nextSubMchID := buildApplymentText(wxResp.SubMchID)
			if newStatus == "rejected" || newStatus == "canceled" {
				_, _ = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
					ID:     operator.ID,
					Status: "active",
				})
			}
			if newStatus != applyment.Status ||
				(!applyment.ApplymentID.Valid && wxResp.ApplymentID > 0) ||
				applymentTextChanged(applyment.RejectReason, nextRejectReason) ||
				applymentTextChanged(applyment.SignUrl, nextSignURL) ||
				applymentTextChanged(applyment.SignState, nextSignState) ||
				applymentTextChanged(applyment.LegalValidationUrl, nextLegalValidationURL) ||
				!bytes.Equal(applyment.AccountValidation, nextAccountValidation) ||
				applymentTextChanged(applyment.SubMchID, nextSubMchID) {
				_, _ = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
					ID:                 applyment.ID,
					ApplymentID:        pgtype.Int8{Int64: wxResp.ApplymentID, Valid: wxResp.ApplymentID > 0},
					Status:             newStatus,
					RejectReason:       nextRejectReason,
					SignUrl:            nextSignURL,
					SignState:          nextSignState,
					LegalValidationUrl: nextLegalValidationURL,
					AccountValidation:  nextAccountValidation,
					SubMchID:           nextSubMchID,
				})
			}

			if newStatus == "finish" && wxResp.SubMchID != "" {
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
			applyment.LegalValidationUrl = nextLegalValidationURL
			applyment.AccountValidation = nextAccountValidation
			applyment.SubMchID = nextSubMchID
			if shouldUseRemoteApplymentStatusDesc(newStatus, nextSignState, nextLegalValidationURL, nextAccountValidation) {
				remoteStatusDesc = strings.TrimSpace(wxResp.ApplymentStateDesc)
			}
			remoteLegalValidationURL = strings.TrimSpace(wxResp.LegalValidationURL)
			remoteAccountValidation = buildApplymentAccountValidationResponse(wxResp.AccountValidation, decryptor)
		}
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("查询运营商微信进件状态失败")
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
	if applyment.LegalValidationUrl.Valid && applyment.LegalValidationUrl.String != "" {
		resp.LegalValidationURL = &applyment.LegalValidationUrl.String
	}
	if storedAccountValidation := buildStoredApplymentAccountValidationResponse(applyment.AccountValidation, decryptor); storedAccountValidation != nil {
		resp.AccountValidation = storedAccountValidation
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
