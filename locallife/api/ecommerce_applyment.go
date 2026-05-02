package api

import (
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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

var applymentDateTokenPattern = regexp.MustCompile(`\d{4}年\d{1,2}月\d{1,2}日|\d{4}[./-]\d{1,2}[./-]\d{1,2}|\d{8}|长期|永久`)
var applymentWechatRequestIDPattern = regexp.MustCompile(`request_id=([A-Za-z0-9._-]+)`)

const applymentDefaultContactIDDocType = "IDENTIFICATION_TYPE_MAINLAND_IDCARD"

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
	ContactIDDocType            string `json:"contact_id_doc_type" binding:"omitempty,oneof=IDENTIFICATION_TYPE_MAINLAND_IDCARD"`
	ContactIDCardNumber         string `json:"contact_id_card_number" binding:"omitempty,min=15,max=32"`
	ContactEmail                string `json:"contact_email" binding:"omitempty,email,max=128"`
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
	f.ContactEmail = strings.TrimSpace(f.ContactEmail)
	f.ContactIDDocPeriodBegin = strings.TrimSpace(f.ContactIDDocPeriodBegin)
	f.ContactIDDocPeriodEnd = strings.TrimSpace(f.ContactIDDocPeriodEnd)
}

func (f applymentContactFields) resolve(legalPersonName, legalPersonIDNumber string) (resolvedApplymentContact, error) {
	switch f.ContactType {
	case "", "LEGAL", "65":
		if strings.TrimSpace(legalPersonName) == "" || strings.TrimSpace(legalPersonIDNumber) == "" {
			return resolvedApplymentContact{}, fmt.Errorf("商户法人身份信息缺失，请先完善商户申请资料后再提交")
		}
		return resolvedApplymentContact{
			ContactType:         "LEGAL",
			ContactName:         strings.TrimSpace(legalPersonName),
			ContactIDCardNumber: strings.TrimSpace(legalPersonIDNumber),
		}, nil
	case "SUPER", "66":
		if f.ContactName == "" {
			return resolvedApplymentContact{}, fmt.Errorf("超级管理员姓名缺失，请填写管理员姓名后再提交")
		}
		if f.ContactIDCardNumber == "" {
			return resolvedApplymentContact{}, fmt.Errorf("超级管理员身份证号缺失，请填写身份证号后再提交")
		}
		if f.ContactIDDocCopyAssetID <= 0 {
			return resolvedApplymentContact{}, fmt.Errorf("超级管理员身份证人像面图片缺失，请上传后再提交")
		}
		if f.ContactIDDocCopyBackAssetID <= 0 {
			return resolvedApplymentContact{}, fmt.Errorf("超级管理员身份证国徽面图片缺失，请上传后再提交")
		}

		contactIDDocType := f.ContactIDDocType
		if contactIDDocType == "" {
			contactIDDocType = applymentDefaultContactIDDocType
		}
		if contactIDDocType != applymentDefaultContactIDDocType {
			return resolvedApplymentContact{}, fmt.Errorf("暂不支持该超级管理员证件类型，请使用中国大陆居民身份证后再提交")
		}

		periodBegin := normalizeApplymentDate(f.ContactIDDocPeriodBegin)
		periodEnd := normalizeApplymentDate(f.ContactIDDocPeriodEnd)
		if err := validateApplymentIDCardValidity(periodBegin, periodEnd); err != nil {
			return resolvedApplymentContact{}, fmt.Errorf("超级管理员证件有效期不完整或格式不正确，请核对证件有效期后再提交")
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
		return resolvedApplymentContact{}, fmt.Errorf("暂不支持该管理员类型，请选择法人或超级管理员后再提交")
	}
}

func (c resolvedApplymentContact) requiresIdentityDocumentAssets() bool {
	return c.ContactType == "SUPER"
}

func (server *Server) validateApplymentContactDocumentAsset(ctx context.Context, userID, assetID int64, expectedCategory media.Category) error {
	asset, err := server.store.GetMediaAssetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("超级管理员证件图片不存在，请重新上传后再提交")
	}
	if asset.UploadedBy != userID {
		return fmt.Errorf("超级管理员证件图片不属于当前用户，请重新上传后再提交")
	}
	if asset.Visibility != string(media.VisibilityPrivate) {
		return fmt.Errorf("超级管理员证件图片权限异常，请重新上传私有证件图片后再提交")
	}
	if asset.MediaCategory != string(expectedCategory) {
		return fmt.Errorf("超级管理员证件图片类型不匹配，请按人像面和国徽面分别上传后再提交")
	}
	if asset.UploadStatus == "deleted" {
		return fmt.Errorf("超级管理员证件图片已不可用，请重新上传后再提交")
	}
	return nil
}

func (f applymentBindBankFields) validateSelection() error {
	if f.AccountBank == "" {
		return fmt.Errorf("开户银行缺失，请选择开户银行后再提交")
	}
	if f.AccountNumber == "" {
		return fmt.Errorf("结算银行账号缺失，请填写结算银行账号后再提交")
	}
	if f.AccountName == "" {
		return fmt.Errorf("结算账户户名缺失，请填写结算账户户名后再提交")
	}
	if f.NeedBankBranch {
		if f.BankAliasCode == "" {
			return fmt.Errorf("开户银行编码缺失，请重新选择开户银行后再提交")
		}
		if f.BankAddressCode == "" {
			return fmt.Errorf("开户银行地区缺失，请选择开户银行所在地区后再提交")
		}
		if f.BankBranchID == "" {
			return fmt.Errorf("开户支行缺失，请选择开户支行后再提交")
		}
		if f.BankName == "" {
			return fmt.Errorf("开户支行名称缺失，请选择开户支行后再提交")
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
// @Description 商户审核通过后，绑定结算账户信息并提交微信普通服务商特约商户进件。
// @Description 本地资料校验失败、微信参数校验失败返回 400；微信签名失败返回 401；微信权限或商户管控返回 403；微信申请不存在返回 404；已有进行中的申请或已完成开户返回 409；微信临时不可用或本地配置缺失返回 502/503 并给出可执行处理指引。
// @Description 所有意外错误均写入结构化请求日志，响应 message/error 字段只返回稳定、清晰且不泄漏内部细节的说明或操作指引。
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body merchantBindBankRequest true "银行卡信息"
// @Success 200 {object} merchantBindBankResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/bindbank [post]
// @Security BearerAuth
func (server *Server) merchantBindBank(ctx *gin.Context) {
	var req merchantBindBankRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}
	req.applymentBindBankFields.normalize()
	req.applymentContactFields.normalize()
	if err := req.validateSelection(); err != nil {
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			respondApplymentClientError(ctx, http.StatusNotFound, ErrMerchantNotFound)
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
			respondApplymentClientError(ctx, http.StatusConflict, ErrAccountApplymentPending)
		case errors.Is(validateErr, logic.ErrApplymentAlreadyRegistered):
			respondApplymentClientError(ctx, http.StatusConflict, ErrAccountAlreadyRegistered)
		default:
			respondApplymentClientError(ctx, http.StatusBadRequest, fmt.Errorf("当前商户状态暂不允许提交微信支付开户申请，请完成商户审核或刷新商户状态后再提交"))
		}
		return
	}

	// 获取商户申请信息（包含营业执照、身份证等OCR数据）
	application, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("get merchant application for applyment: %w", err), "获取商户开户资料失败，请稍后重试；如持续失败请联系平台管理员处理", "get merchant applyment source application failed"))
		return
	}

	contactPhone, err := server.resolveApplymentContactPhone(ctx, authPayload.UserID, application.ContactPhone, merchant.Phone)
	if err != nil {
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
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
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := validateApplymentBusinessLicenseValidity(businessLicenseOCR.ValidPeriod); err != nil {
		log.Warn().Int64("merchant_id", merchant.ID).Str("valid_period", businessLicenseOCR.ValidPeriod).Msg("商户营业期限无效，拒绝提交微信进件")
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}

	idCardValidTimeBegin, idCardValidTime := logic.ParseApplymentIDCardValidPeriod(idCardBackOCR.ValidDate)
	if err := validateApplymentIDCardValidity(idCardValidTimeBegin, idCardValidTime); err != nil {
		log.Warn().Int64("merchant_id", merchant.ID).Str("valid_date", idCardBackOCR.ValidDate).Msg("商户身份证有效期无效，拒绝提交微信进件")
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}

	resolvedContact, err := req.applymentContactFields.resolve(application.LegalPersonName, application.LegalPersonIDNumber)
	if err != nil {
		respondApplymentClientError(ctx, http.StatusBadRequest, err)
		return
	}
	if resolvedContact.requiresIdentityDocumentAssets() {
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyAssetID, media.CategoryIDCardFront); err != nil {
			respondApplymentClientError(ctx, http.StatusBadRequest, err)
			return
		}
		if err := server.validateApplymentContactDocumentAsset(ctx, authPayload.UserID, resolvedContact.ContactIDDocCopyBackAssetID, media.CategoryIDCardBack); err != nil {
			respondApplymentClientError(ctx, http.StatusBadRequest, err)
			return
		}
	}

	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商进件服务暂不可用，请联系平台管理员检查微信支付普通服务商证书、公钥、进件和回调配置后重试", "merchant applyment ordinary service provider client not configured"))
		return
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail == "" {
		contactEmail = strings.TrimSpace(server.config.WechatOrdinaryApplymentContactEmail)
	}
	if contactEmail == "" {
		respondApplymentClientError(ctx, http.StatusBadRequest, fmt.Errorf("普通服务商进件联系人邮箱缺失，请填写商户联系人邮箱，或联系平台管理员配置 WECHAT_ORDINARY_APPLYMENT_CONTACT_EMAIL 后重试"))
		return
	}
	settlementID, settlementErr := resolveOrdinaryApplymentSettlementID(server.config, organizationType)
	qualificationType := strings.TrimSpace(server.config.WechatOrdinaryApplymentQualification)
	if settlementErr != nil || qualificationType == "" {
		err := fmt.Errorf("ordinary service provider applyment settlement config not configured")
		if settlementErr != nil {
			err = settlementErr
		} else if qualificationType == "" {
			err = fmt.Errorf("WECHAT_ORDINARY_APPLYMENT_QUALIFICATION_TYPE is required for ordinary service provider applyment")
		}
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商进件结算规则未配置，请联系平台管理员配置微信支付进件结算规则后重试", "merchant applyment ordinary service provider settlement config not configured"))
		return
	}
	servicePhone := strings.TrimSpace(server.config.WechatOrdinaryApplymentServicePhone)
	if servicePhone == "" {
		servicePhone = contactPhone
	}

	// 加密敏感数据（本地存储）
	encryptedIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, application.LegalPersonIDNumber)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("encrypt legal person id card number: %w", err), "商户开户资料加密失败，请稍后重试；如持续失败请联系平台管理员检查数据加密配置", "encrypt merchant applyment legal person id card number failed"))
		return
	}
	encryptedAccountNumber, err := util.EncryptSensitiveField(server.dataEncryptor, req.AccountNumber)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("encrypt settlement account number: %w", err), "商户开户资料加密失败，请稍后重试；如持续失败请联系平台管理员检查数据加密配置", "encrypt merchant applyment settlement account number failed"))
		return
	}
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, resolvedContact.ContactIDCardNumber)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("encrypt applyment contact id card number: %w", err), "商户开户资料加密失败，请稍后重试；如持续失败请联系平台管理员检查数据加密配置", "encrypt merchant applyment contact id card number failed"))
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
		ContactEmail:          contactEmail,
		MerchantShortname:     merchant.Name,
	}))
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("create merchant applyment record: %w", err), "创建商户开户记录失败，请稍后重试；如持续失败请联系平台管理员处理", "create merchant applyment record failed"))
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

	// ==================== 下载证件图片并上传到微信获取 MediaID ====================
	ordinaryUploader := ordinaryApplymentImageUploader{client: server.ordinarySPClient}
	idCardFrontMediaID := ""
	if application.IDCardFrontMediaAssetID.Valid {
		idCardFrontMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, ordinaryUploader, application.IDCardFrontMediaAssetID.Int64)
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			writeApplymentMediaUploadServerError(ctx, err, "身份证图片", "获取身份证图片失败")
			return
		}
	}

	idCardBackMediaID := ""
	if application.IDCardBackMediaAssetID.Valid {
		idCardBackMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, ordinaryUploader, application.IDCardBackMediaAssetID.Int64)
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			writeApplymentMediaUploadServerError(ctx, err, "身份证图片", "获取身份证图片失败")
			return
		}
	}

	var businessLicenseMediaID string
	if application.BusinessLicenseMediaAssetID.Valid {
		businessLicenseMediaID, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, ordinaryUploader, application.BusinessLicenseMediaAssetID.Int64)
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			writeApplymentMediaUploadServerError(ctx, err, "营业执照图片", "获取营业执照图片失败")
			return
		}
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	encryptedWechatFields, err := logic.EncryptApplymentWechatSensitiveFields(server.ordinarySPClient, logic.ApplymentWechatSensitiveInput{
		IDCardName:          application.LegalPersonName,
		IDCardNumber:        application.LegalPersonIDNumber,
		ContactName:         resolvedContact.ContactName,
		ContactIDCardNumber: resolvedContact.ContactIDCardNumber,
		AccountName:         req.AccountName,
		AccountNumber:       req.AccountNumber,
		MobilePhone:         contactPhone,
		ContactEmail:        contactEmail,
	})
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "商户开户资料加密失败，请稍后重试；如持续失败请联系平台管理员检查微信支付敏感信息加密配置", "encrypt ordinary service provider applyment sensitive fields failed"))
		return
	}

	// ==================== 构建微信普通服务商进件请求 ====================
	storeName := strings.TrimSpace(application.MerchantName)
	if storeName == "" {
		storeName = merchant.Name
	}
	storeQRCodeObjectKey, err := server.ensureMerchantStorefrontQRCode(ctx, authPayload.UserID, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("generate merchant storefront qrcode: %w", err), "生成店铺首页二维码失败，请稍后重试；如持续失败请联系平台管理员检查小程序码配置", "generate merchant storefront qrcode failed"))
		return
	}
	storeQRCodeReader, err := server.mediaStorage.ReadObject(ctx, server.mediaStorage.PublicBucket(), storeQRCodeObjectKey)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("read merchant storefront qrcode object %s: %w", storeQRCodeObjectKey, err), "读取店铺首页二维码失败，请稍后重试；如持续失败请联系平台管理员处理", "read merchant storefront qrcode object failed"))
		return
	}
	storeQRCodeData, err := io.ReadAll(storeQRCodeReader)
	if closeErr := storeQRCodeReader.Close(); closeErr != nil {
		log.Warn().Err(closeErr).Str("object_key", storeQRCodeObjectKey).Msg("close merchant storefront qrcode reader failed")
	}
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("read merchant storefront qrcode bytes %s: %w", storeQRCodeObjectKey, err), "读取店铺首页二维码失败，请稍后重试；如持续失败请联系平台管理员处理", "read merchant storefront qrcode bytes failed"))
		return
	}
	storeQRCodeUploadResp, err := server.ordinarySPClient.UploadImage(ctx, path.Base(storeQRCodeObjectKey), storeQRCodeData)
	if err != nil {
		writeApplymentMediaUploadServerError(ctx, err, "店铺首页二维码", "上传店铺首页二维码失败")
		return
	}
	log.Info().
		Int64("merchant_id", merchant.ID).
		Str("store_qr_code_media_id", storeQRCodeUploadResp.MediaID).
		Str("storefront_qr_object_key", storeQRCodeObjectKey).
		Str("storefront_qr_page", merchantStorefrontQRCodePage).
		Str("storefront_qr_scene", buildMerchantStorefrontQRCodeScene(merchant.ID)).
		Msg("merchant applyment storefront qrcode uploaded as wechat media id")

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
		contactInfo.ContactIDDocCopy, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, ordinaryUploader, resolvedContact.ContactIDDocCopyAssetID)
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			writeApplymentMediaUploadServerError(ctx, err, "超级管理员证件图片", "获取超级管理员证件图片失败")
			return
		}

		contactInfo.ContactIDDocCopyBack, err = logic.UploadApplymentAsset(ctx, server.mediaRegistry, ordinaryUploader, resolvedContact.ContactIDDocCopyBackAssetID)
		if err != nil {
			if writeLogicRequestError(ctx, err) {
				return
			}
			writeApplymentMediaUploadServerError(ctx, err, "超级管理员证件图片", "获取超级管理员证件图片失败")
			return
		}
	}

	applymentReq := logic.BuildOrdinaryServiceProviderApplymentRequest(logic.ApplymentOrdinaryRequestInput{
		BusinessCode:      outRequestNo,
		OrganizationType:  organizationType,
		BusinessLicense:   logic.ApplymentBusinessLicenseOCRInput{Address: businessLicenseOCR.Address, ValidPeriod: businessLicenseOCR.ValidPeriod},
		BusinessLicenseID: application.BusinessLicenseNumber,
		LicenseCopy:       businessLicenseMediaID,
		MerchantName:      application.MerchantName,
		LegalPerson:       application.LegalPersonName,
		BusinessAddress:   application.BusinessAddress,
		MerchantShortname: merchant.Name,
		ServicePhone:      servicePhone,
		MiniProgramAppID:  server.config.WechatMiniAppID,
		StoreName:         storeName,
		StoreQRCode:       storeQRCodeUploadResp.MediaID,
		IDCardInfo: logic.ApplymentOrdinaryIDCardInput{
			IDCardCopy:      idCardFrontMediaID,
			IDCardNational:  idCardBackMediaID,
			IDCardName:      encryptedWechatFields.IDCardName,
			IDCardNumber:    encryptedWechatFields.IDCardNumber,
			CardPeriodBegin: idCardValidTimeBegin,
			CardPeriodEnd:   idCardValidTime,
		},
		AccountInfo: logic.ApplymentWechatAccountInput{
			AccountType:     req.AccountType,
			AccountBank:     req.AccountBank,
			AccountName:     encryptedWechatFields.AccountName,
			BankAddressCode: req.BankAddressCode,
			BankBranchID:    req.BankBranchID,
			BankName:        req.BankName,
			AccountNumber:   encryptedWechatFields.AccountNumber,
		},
		ContactInfo: logic.ApplymentOrdinaryContactInput{
			ContactType:          contactInfo.ContactType,
			ContactName:          contactInfo.ContactName,
			ContactIDDocType:     contactInfo.ContactIDDocType,
			ContactIDNumber:      contactInfo.ContactIDCardNumber,
			ContactPeriodBegin:   contactInfo.ContactIDDocPeriodBegin,
			ContactPeriodEnd:     contactInfo.ContactIDDocPeriodEnd,
			ContactIDDocCopy:     contactInfo.ContactIDDocCopy,
			ContactIDDocCopyBack: contactInfo.ContactIDDocCopyBack,
			MobilePhone:          contactInfo.MobilePhone,
			ContactEmail:         encryptedWechatFields.ContactEmail,
		},
		SettlementID:         settlementID,
		QualificationType:    qualificationType,
		ActivitiesID:         server.config.WechatOrdinaryApplymentActivitiesID,
		DebitActivitiesRate:  server.config.WechatOrdinaryApplymentDebitActivitiesRate,
		CreditActivitiesRate: server.config.WechatOrdinaryApplymentCreditActivitiesRate,
		ActivitiesAdditions:  splitOrdinaryApplymentActivitiesAdditions(server.config.WechatOrdinaryApplymentActivitiesAdditions),
	})

	submissionResult, err := logic.SubmitOrdinaryServiceProviderApplyment(ctx, server.store, server.ordinarySPClient, updateMerchantStatus, logic.SubmitOrdinaryServiceProviderApplymentInput{
		Applyment:     applyment,
		WechatRequest: applymentReq,
	})
	if err != nil {
		if respondApplymentOrdinaryProviderError(ctx, merchant.ID, outRequestNo, err) {
			return
		}
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "微信支付开户申请已提交前后发生状态同步异常，请联系平台管理员核对进件状态，避免重复提交", "ordinary service provider applyment submission failed after local processing"))
		return
	}
	snapshot := buildOrdinaryApplymentSubmissionStatusSnapshot(submissionResult)

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
	Status                string                              `json:"status"`                            // 状态
	StatusDesc            string                              `json:"status_desc"`                       // 状态描述
	CanSubmit             bool                                `json:"can_submit"`                        // 是否允许提交或重新提交进件
	BlockReason           string                              `json:"block_reason,omitempty"`            // 不允许提交时的阻塞原因
	AccountAuthorizeState string                              `json:"account_authorize_state,omitempty"` // 普通服务商开户意愿授权状态
	ActionHint            string                              `json:"action_hint,omitempty"`             // 前端/商户下一步行动指引
	SignURL               *string                             `json:"sign_url,omitempty"`                // 签约链接
	SignState             *string                             `json:"sign_state,omitempty"`              // 签约状态
	LegalValidationURL    *string                             `json:"legal_validation_url,omitempty"`    // 法人扫码验证链接
	AccountValidation     *applymentAccountValidationResponse `json:"account_validation,omitempty"`      // 汇款验证信息
	SubMchID              *string                             `json:"sub_mch_id,omitempty"`              // 二级商户号
	RejectReason          *string                             `json:"reject_reason,omitempty"`           // 拒绝原因
}

type applymentSensitiveDecryptor interface {
	DecryptSensitiveResponseData(ciphertext string) (string, error)
}

type ordinaryApplymentImageUploader struct {
	client ordinaryserviceprovider.OrdinaryServiceProviderClientInterface
}

func (u ordinaryApplymentImageUploader) UploadImage(ctx context.Context, filename string, fileData []byte) (*wechat.ImageUploadResponse, error) {
	if u.client == nil {
		return nil, fmt.Errorf("ordinary service provider client not configured")
	}
	resp, err := u.client.UploadImage(ctx, filename, fileData)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("ordinary service provider upload image returned empty response")
	}
	return &wechat.ImageUploadResponse{MediaID: resp.MediaID}, nil
}

func resolveApplymentSensitiveDecryptor(client any) applymentSensitiveDecryptor {
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

func resolveApplymentSensitiveFieldValue(decryptor applymentSensitiveDecryptor, value, rawCiphertext string) string {
	if strings.TrimSpace(rawCiphertext) != "" {
		return strings.TrimSpace(value)
	}
	return decryptApplymentSensitiveField(decryptor, value)
}

func buildApplymentAccountValidationResponse(validation *wechatcontracts.EcommerceApplymentAccountValidation, decryptor applymentSensitiveDecryptor) *applymentAccountValidationResponse {
	if validation == nil {
		return nil
	}

	return &applymentAccountValidationResponse{
		AccountName:              resolveApplymentSensitiveFieldValue(decryptor, validation.AccountName, validation.RawAccountName),
		AccountNo:                resolveApplymentSensitiveFieldValue(decryptor, validation.AccountNo, validation.RawAccountNo),
		PayAmount:                validation.PayAmount,
		DestinationAccountNumber: strings.TrimSpace(validation.DestinationAccountNumber),
		DestinationAccountName:   strings.TrimSpace(validation.DestinationAccountName),
		DestinationAccountBank:   strings.TrimSpace(validation.DestinationAccountBank),
		City:                     strings.TrimSpace(validation.City),
		Remark:                   strings.TrimSpace(validation.Remark),
		Deadline:                 strings.TrimSpace(validation.Deadline),
	}
}

func buildStoredApplymentAccountValidationResponse(raw []byte, decryptor applymentSensitiveDecryptor) (*applymentAccountValidationResponse, error) {
	validation, err := wechat.UnmarshalEcommerceApplymentAccountValidation(raw)
	if err != nil {
		return nil, fmt.Errorf("decode stored applyment account validation: %w", err)
	}

	return buildApplymentAccountValidationResponse(validation, decryptor), nil
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

func buildOrdinaryApplymentSubmissionStatusSnapshot(result logic.SubmitOrdinaryServiceProviderApplymentResult) applymentSubmissionStatusSnapshot {
	snapshot := applymentSubmissionStatusSnapshot{
		Status:     result.Status,
		StatusDesc: result.StatusDesc,
		Message:    result.Message,
	}

	queryResp := result.InitialQueryResponse
	if queryResp == nil {
		return snapshot
	}

	trimmedSignURL := strings.TrimSpace(queryResp.SignURL)
	if trimmedSignURL != "" {
		snapshot.SignURL = &trimmedSignURL
	}
	trimmedSubMchID := strings.TrimSpace(queryResp.SubMchID)
	if trimmedSubMchID != "" {
		snapshot.SubMchID = &trimmedSubMchID
	}
	if rejectReason := ordinaryApplymentRejectReasonText(queryResp.AuditDetail); rejectReason != "" {
		snapshot.RejectReason = &rejectReason
	}

	if snapshot.StatusDesc == "" {
		snapshot.StatusDesc = getApplymentStatusDesc(snapshot.Status)
	}
	if snapshot.Message == "" {
		snapshot.Message = logic.OrdinaryApplymentFrontendMessage(snapshot.Status)
	}

	return snapshot
}

func ordinaryApplymentRejectReasonText(details []ospcontracts.ApplymentAuditDetail) string {
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

func shouldQueryApplymentRemoteStatus(applyment db.EcommerceApplyment, subjectStatus string) bool {
	normalizedStatus := logic.NormalizeResolvedApplymentStatus(applyment.Status, applyment.SubMchID.Valid && strings.TrimSpace(applyment.SubMchID.String) != "")
	if normalizedStatus == "finish" {
		return applyment.ApplymentID.Valid || strings.TrimSpace(applyment.OutRequestNo) != ""
	}
	return logic.IsApplymentSubmissionInFlight(normalizedStatus, subjectStatus, applyment.OutRequestNo)
}

func (server *Server) queryOrdinaryApplymentStatus(ctx context.Context, applyment db.EcommerceApplyment) (*ospcontracts.ApplymentQueryResponse, error) {
	if server.ordinarySPClient == nil {
		return nil, fmt.Errorf("ordinary service provider client not configured")
	}

	if applyment.ApplymentID.Valid {
		resp, err := server.ordinarySPClient.QueryApplymentByID(ctx, ospcontracts.ApplymentQueryByIDRequest{ApplymentID: applyment.ApplymentID.Int64})
		if err == nil {
			log.Info().
				Int64("applyment_id", applyment.ID).
				Str("query_key", "applyment_id").
				Int64("wechat_applyment_id", resp.ApplymentID).
				Str("business_code", strings.TrimSpace(resp.BusinessCode)).
				Str("applyment_state", string(resp.ApplymentState)).
				Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
				Msg("query ordinary service provider applyment status succeeded")
			return resp, nil
		}
		if strings.TrimSpace(applyment.OutRequestNo) == "" {
			return nil, err
		}

		log.Warn().Err(err).
			Int64("applyment_id", applyment.ID).
			Int64("wechat_applyment_id", applyment.ApplymentID.Int64).
			Str("out_request_no", applyment.OutRequestNo).
			Msg("query ordinary service provider applyment by id failed, fallback to business_code")
	}

	if strings.TrimSpace(applyment.OutRequestNo) == "" {
		return nil, fmt.Errorf("applyment out_request_no is empty")
	}

	resp, err := server.ordinarySPClient.QueryApplymentByBusinessCode(ctx, ospcontracts.ApplymentQueryByBusinessCodeRequest{BusinessCode: applyment.OutRequestNo})
	if err == nil {
		log.Info().
			Int64("applyment_id", applyment.ID).
			Str("query_key", "business_code").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("business_code", strings.TrimSpace(resp.BusinessCode)).
			Str("applyment_state", string(resp.ApplymentState)).
			Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
			Msg("query ordinary service provider applyment status succeeded")
	}
	return resp, err
}

func applymentAuthorizationActionHint(status, accountAuthorizeState string, authorizeStateQueryFailed bool) string {
	switch strings.TrimSpace(status) {
	case "finish":
		if authorizeStateQueryFailed {
			return "暂时无法确认开户意愿授权状态，请稍后刷新；如果持续失败，请联系平台管理员核对微信支付普通服务商配置和商户号状态"
		}
		if strings.TrimSpace(accountAuthorizeState) == db.AccountAuthorizeStateAuthorized {
			return "普通服务商特约商户账户已完成开户意愿确认，可以使用交易、退款、分账和结算账户能力"
		}
		return "请商户负责人进入微信支付商户平台或微信支付商家助手完成开户意愿确认；完成后返回本页刷新状态"
	case "submitted", "checking", "auditing":
		return "请等待微信支付审核；如微信返回签约、确认或账户验证链接，请按页面提示完成"
	case "account_need_verify":
		return "请按微信页面提示完成汇款账户验证，完成后刷新状态"
	case "to_be_confirmed":
		return "请商户负责人按微信支付页面提示完成确认，完成后刷新状态"
	case "to_be_signed", "signing":
		return "请点击签约链接完成微信支付签约，完成后刷新状态"
	case "rejected":
		return "请根据驳回原因修正资料后重新提交普通服务商进件"
	default:
		return ""
	}
}

func buildApplymentText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
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

func respondApplymentClientError(ctx *gin.Context, status int, err error) {
	_ = ctx.Error(err)
	ctx.JSON(status, errorResponse(err))
}

func respondApplymentWechatError(ctx *gin.Context, merchantID int64, outRequestNo string, err error) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) || wxErr == nil {
		return false
	}

	evt := log.Error()
	if wxErr.StatusCode < http.StatusInternalServerError && wxErr.Code != "INVALID_REQUEST" && wxErr.Code != "SIGN_ERROR" {
		evt = log.Warn()
	}
	evt.
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Int64("merchant_id", merchantID).
		Str("out_request_no", strings.TrimSpace(outRequestNo)).
		Int("wechat_status_code", wxErr.StatusCode).
		Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
		Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
		Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail)).
		Msg("wechat applyment request failed")

	writeClientError := func(status int, responseErr error) {
		_ = ctx.Error(err)
		ctx.JSON(status, errorResponse(responseErr))
	}

	switch strings.TrimSpace(wxErr.Code) {
	case "RESOURCE_ALREADY_EXISTS":
		writeClientError(http.StatusBadRequest, ErrAccountApplymentPending)
	case "PARAM_ERROR":
		writeClientError(http.StatusBadRequest, ErrApplymentWechatParamError)
	case "NO_AUTH":
		writeClientError(http.StatusForbidden, ErrApplymentWechatNoAuth)
	case "RESOURCE_NOT_EXISTS":
		writeClientError(http.StatusNotFound, ErrApplymentWechatNotFound)
	case "INVALID_REQUEST":
		writeClientError(http.StatusBadRequest, ErrApplymentWechatInvalidRequest)
	case "SIGN_ERROR":
		writeClientError(http.StatusUnauthorized, ErrApplymentWechatSignError)
	case "SYSTEM_ERROR":
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
	default:
		writeClientError(http.StatusBadGateway, ErrApplymentWechatServiceUnavailable)
	}

	return true
}

func respondApplymentOrdinaryProviderError(ctx *gin.Context, merchantID int64, businessCode string, err error) bool {
	var providerErr *ordinaryserviceprovider.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}

	_ = ctx.Error(err)

	messageWithAction := func(fallback string) error {
		message := strings.TrimSpace(providerErr.Frontend.Message)
		if action := strings.TrimSpace(providerErr.Frontend.Action); action != "" {
			message = strings.TrimSpace(message + "，" + action)
		}
		if message == "" {
			message = fallback
		}
		return errors.New(message)
	}

	log.Warn().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Int64("merchant_id", merchantID).
		Str("business_code", strings.TrimSpace(businessCode)).
		Str("wechat_operation", providerErr.Operation).
		Str("wechat_request_id", providerErr.RequestID).
		Str("wechat_error_code", providerErr.ProviderCode).
		Str("wechat_error_message", strings.TrimSpace(providerErr.ProviderMessage)).
		Str("error_category", string(providerErr.Category)).
		Str("frontend_code", strings.TrimSpace(providerErr.Frontend.Code)).
		Str("frontend_message", strings.TrimSpace(providerErr.Frontend.Message)).
		Str("frontend_action", strings.TrimSpace(providerErr.Frontend.Action)).
		Bool("retryable", providerErr.Frontend.Retryable).
		Msg("ordinary service provider applyment request failed")

	switch providerErr.Category {
	case ordinaryserviceprovider.ErrorCategoryValidation:
		ctx.JSON(http.StatusBadRequest, errorResponse(messageWithAction("进件资料不完整或格式不正确，请补充资料后重新提交")))
	case ordinaryserviceprovider.ErrorCategoryBusinessConflict:
		ctx.JSON(http.StatusConflict, errorResponse(messageWithAction("微信支付已有进行中的进件申请，请等待当前申请处理完成后刷新状态")))
	case ordinaryserviceprovider.ErrorCategoryMerchantControl:
		ctx.JSON(http.StatusForbidden, errorResponse(messageWithAction("微信支付限制了该商户进件能力，请联系平台管理员查看商户管控诊断并按微信指引处理")))
	case ordinaryserviceprovider.ErrorCategoryAuthConfig:
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商进件服务配置不可用，请联系平台管理员检查微信支付服务商证书、公钥、API 权限和进件结算规则后重试", "ordinary service provider applyment auth/config failure"))
	case ordinaryserviceprovider.ErrorCategoryRetryableProvider:
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信支付普通服务商进件接口暂时不可用，请稍后重试；如持续失败请联系平台管理员查看进件服务日志并处理", "ordinary service provider applyment retryable provider failure"))
	default:
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信支付普通服务商进件返回异常，请稍后重试或联系平台管理员处理", "ordinary service provider applyment provider failure"))
	}

	return true
}

func writeApplymentMediaUploadServerError(ctx *gin.Context, err error, assetLabel string, fallbackMessage string) {
	publicMessage := buildApplymentMediaUploadErrorMessage(err, assetLabel, fallbackMessage)
	ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, fmt.Errorf("%s upload image to wechat: %w", assetLabel, err), publicMessage, "ordinary service provider applyment media upload failed"))
}

func buildApplymentMediaUploadErrorMessage(err error, assetLabel string, fallbackMessage string) string {
	if err == nil {
		return fallbackMessage
	}

	requestID := extractApplymentWechatRequestID(err)
	appendRequestID := func(message string) string {
		if requestID == "" {
			return message
		}
		return fmt.Sprintf("%s；如需排查，请联系平台管理员查看微信图片上传失败日志", message)
	}

	lowerMessage := strings.ToLower(err.Error())
	if strings.Contains(lowerMessage, "service provider merchant id must be configured explicitly") {
		return appendRequestID(fmt.Sprintf("%s上传到微信失败，普通服务商图片上传配置不完整，请联系平台管理员检查微信支付普通服务商商户号、证书和上传权限后重试", assetLabel))
	}

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		switch strings.TrimSpace(wxErr.Code) {
		case "PARAM_ERROR":
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，请检查图片格式、内容和大小后重试", assetLabel))
		case "NO_AUTH":
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，当前平台未开通微信图片上传权限，请联系平台管理员处理", assetLabel))
		case "FREQUENCY_LIMIT_EXCEED":
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，微信接口限流，请稍后重试", assetLabel))
		case "SIGN_ERROR":
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，平台签名配置异常，请联系平台管理员处理", assetLabel))
		case "SYSTEM_ERROR":
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，微信服务暂时异常，请稍后重试", assetLabel))
		default:
			return appendRequestID(fmt.Sprintf("%s上传到微信失败，请稍后重试", assetLabel))
		}
	}

	if requestID != "" {
		return fmt.Sprintf("%s上传到微信失败，请稍后重试；如需排查，请联系平台管理员查看微信图片上传失败日志", assetLabel)
	}

	return fallbackMessage
}

func extractApplymentWechatRequestID(err error) string {
	if err == nil {
		return ""
	}
	matches := applymentWechatRequestIDPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
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
	accountAuthorizeStateQueryFailed := false
	decryptor := resolveApplymentSensitiveDecryptor(server.ordinarySPClient)

	// 进件处理中时优先查询微信普通服务商实时状态；若本地丢失 applyment_id，则回退到 business_code。
	if shouldQueryApplymentRemoteStatus(applyment, merchant.Status) {
		if server.ordinarySPClient == nil {
			log.Warn().
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("ordinary service provider client not configured, cannot query merchant applyment status")
		} else if wxResp, err := server.queryOrdinaryApplymentStatus(ctx, applyment); err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("查询微信普通服务商进件状态失败")
		} else {
			if !applyment.ApplymentID.Valid && wxResp.ApplymentID > 0 {
				applyment.ApplymentID = pgtype.Int8{Int64: wxResp.ApplymentID, Valid: true}
			}

			nextSubMchID := buildApplymentText(wxResp.SubMchID)
			updateStatus := logic.NormalizeResolvedApplymentStatus(
				logic.MapOrdinaryApplymentStateToStatus(wxResp.ApplymentState),
				nextSubMchID.Valid && strings.TrimSpace(nextSubMchID.String) != "",
			)
			if updateStatus == "" {
				updateStatus = applyment.Status
			}
			nextRejectReason := buildApplymentText(ordinaryApplymentRejectReasonText(wxResp.AuditDetail))
			nextSignURL := buildApplymentText(wxResp.SignURL)
			nextSignState := applyment.SignState
			nextLegalValidationURL := applyment.LegalValidationUrl
			nextAccountValidation := applyment.AccountValidation

			if updateStatus != applyment.Status ||
				(!applyment.ApplymentID.Valid && wxResp.ApplymentID > 0) ||
				applymentTextChanged(applyment.RejectReason, nextRejectReason) ||
				applymentTextChanged(applyment.SignUrl, nextSignURL) ||
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
					log.Error().Err(err).Msg("更新普通服务商进件状态失败")
				}
			}

			applyment.Status = updateStatus
			applyment.RejectReason = nextRejectReason
			applyment.SignUrl = nextSignURL
			applyment.SignState = nextSignState
			applyment.LegalValidationUrl = nextLegalValidationURL
			applyment.AccountValidation = nextAccountValidation
			applyment.SubMchID = nextSubMchID
			if strings.TrimSpace(wxResp.ApplymentStateMsg) != "" {
				remoteStatusDesc = strings.TrimSpace(wxResp.ApplymentStateMsg)
			}

			// 进件完成后必须继续确认开户意愿授权状态；未授权时仅保存 sub_mch_id，不开放交易能力。
			if updateStatus == "finish" && nextSubMchID.Valid {
				accountAuthorizeState := strings.TrimSpace(applyment.AccountAuthorizeState.String)
				if accountAuthorizeState == "" {
					accountAuthorizeState = db.AccountAuthorizeStateUnauthorized
				}
				authResp, authErr := server.ordinarySPClient.QueryAccountAuthorizeState(ctx, ospcontracts.AccountAuthorizeStateRequest{SubMchID: nextSubMchID.String})
				if authErr != nil {
					accountAuthorizeStateQueryFailed = true
					log.Error().Err(authErr).
						Int64("applyment_id", applyment.ID).
						Int64("merchant_id", merchant.ID).
						Str("sub_mch_id", nextSubMchID.String).
						Msg("query ordinary service provider account authorize state failed")
				} else if authResp != nil && strings.TrimSpace(string(authResp.AuthorizeState)) != "" {
					accountAuthorizeState = strings.TrimSpace(string(authResp.AuthorizeState))
				}

				err = server.store.ApplymentSubMchActivationTx(ctx, db.ApplymentSubMchActivationTxParams{
					ApplymentID:           applyment.ID,
					SubjectType:           applyment.SubjectType,
					SubjectID:             applyment.SubjectID,
					SubMchID:              nextSubMchID.String,
					AccountAuthorizeState: accountAuthorizeState,
				})
				if err != nil {
					log.Error().Err(err).
						Int64("applyment_id", applyment.ID).
						Int64("merchant_id", merchant.ID).
						Str("sub_mch_id", nextSubMchID.String).
						Str("account_authorize_state", accountAuthorizeState).
						Msg("sync merchant applyment completion and authorization state failed")
				} else {
					applyment.AccountAuthorizeState = buildApplymentText(accountAuthorizeState)
					applyment.AccountAuthorizeStateCheckedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
					if accountAuthorizeState == db.AccountAuthorizeStateAuthorized {
						merchant.Status = db.MerchantStatusActive
					}
				}
			}
		}
	}

	normalizedStatus := logic.NormalizeResolvedApplymentStatus(applyment.Status, applyment.SubMchID.Valid && applyment.SubMchID.String != "")
	accountAuthorizeState := strings.TrimSpace(applyment.AccountAuthorizeState.String)
	statusDesc := getApplymentStatusDesc(normalizedStatus)
	if remoteStatusDesc != "" {
		statusDesc = remoteStatusDesc
	}
	if normalizedStatus == "finish" {
		if accountAuthorizeStateQueryFailed {
			statusDesc = "进件已完成，但暂时无法确认开户意愿授权状态"
		} else if accountAuthorizeState == db.AccountAuthorizeStateAuthorized {
			statusDesc = "普通服务商账户已开通"
		} else {
			statusDesc = "进件已完成，待商户完成微信开户意愿确认"
		}
	}
	canSubmit, blockReason := getMerchantApplymentSubmitCapability(merchant.Status, normalizedStatus)
	resp := merchantApplymentStatusResponse{
		Status:                normalizedStatus,
		StatusDesc:            statusDesc,
		CanSubmit:             canSubmit,
		BlockReason:           blockReason,
		AccountAuthorizeState: accountAuthorizeState,
		ActionHint:            applymentAuthorizationActionHint(normalizedStatus, accountAuthorizeState, accountAuthorizeStateQueryFailed),
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
	storedAccountValidation, err := buildStoredApplymentAccountValidationResponse(applyment.AccountValidation, decryptor)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if storedAccountValidation != nil {
		resp.AccountValidation = storedAccountValidation
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

func resolveOrdinaryApplymentSettlementID(config util.Config, organizationType string) (string, error) {
	var fieldName string
	var settlementID string

	switch strings.TrimSpace(organizationType) {
	case "4":
		fieldName = "WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_INDIVIDUAL"
		settlementID = config.WechatOrdinaryApplymentSettlementIDIndividual
	case "2":
		fieldName = "WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE"
		settlementID = config.WechatOrdinaryApplymentSettlementIDEnterprise
	default:
		return "", fmt.Errorf("unsupported ordinary service provider applyment organization type: %s", organizationType)
	}

	settlementID = strings.TrimSpace(settlementID)
	if settlementID == "" {
		return "", fmt.Errorf("%s is required for ordinary service provider applyment organization type %s", fieldName, organizationType)
	}
	return settlementID, nil
}

func splitOrdinaryApplymentActivitiesAdditions(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	additions := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			additions = append(additions, trimmed)
		}
	}
	if len(additions) == 0 {
		return nil
	}
	return additions
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
			return false, "当前商户状态不可用，暂不支持提交普通服务商进件。"
		}
		return false, "当前商户状态暂不支持提交普通服务商进件。"
	case "canceled":
		if merchantStatus == "approved" || merchantStatus == "pending_bindbank" {
			return true, ""
		}
		if merchantStatus == "active" {
			return false, "当前账户已开通，无需重复提交进件资料。"
		}
		if merchantStatus == "suspended" || merchantStatus == "expired" {
			return false, "当前商户状态不可用，暂不支持提交普通服务商进件。"
		}
		return false, "当前商户状态暂不支持提交普通服务商进件。"
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
		return false, "当前进件状态不可用，暂不支持提交普通服务商进件。"
	default:
		return false, "当前状态暂不支持提交普通服务商进件。"
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
