package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	req.normalize()
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
		if existingApplyment.Status == "submitted" || existingApplyment.Status == "auditing" ||
			existingApplyment.Status == "to_be_signed" || existingApplyment.Status == "signing" {
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

	// 解析身份证OCR信息
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
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, application.LegalPersonIDNumber)
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
		ContactName:           application.LegalPersonName,
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
	storeURL := buildApplymentStoreURL(server.config)
	storeName := strings.TrimSpace(application.MerchantName)
	if storeName == "" {
		storeName = merchant.Name
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
		ContactInfo: &wechat.ApplymentContactInfo{
			ContactType:         "65",
			ContactName:         wxEncryptedIDCardName,
			ContactIDCardNumber: wxEncryptedIDCardNumber,
			MobilePhone:         wxEncryptedMobilePhone,
		},
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName: storeName,
			StoreURL:  storeURL,
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
type merchantApplymentStatusResponse struct {
	Status       string  `json:"status"`                  // 状态
	StatusDesc   string  `json:"status_desc"`             // 状态描述
	SignURL      *string `json:"sign_url,omitempty"`      // 签约链接
	SubMchID     *string `json:"sub_mch_id,omitempty"`    // 二级商户号
	RejectReason *string `json:"reject_reason,omitempty"` // 拒绝原因
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
			ctx.JSON(http.StatusOK, merchantApplymentStatusResponse{
				Status:     status,
				StatusDesc: getApplymentStatusDesc(status),
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果有微信申请单号且客户端可用，查询最新状态
	if applyment.ApplymentID.Valid && server.ecommerceClient != nil {
		wxResp, err := server.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
		if err != nil {
			log.Error().Err(err).Msg("查询微信进件状态失败")
		} else {
			// 更新本地状态
			updateStatus := mapWechatApplymentStatus(wxResp.ApplymentState)
			if updateStatus != applyment.Status {
				_, err = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
					ID:           applyment.ID,
					Status:       updateStatus,
					RejectReason: getRejectReasonFromAuditDetail(wxResp.AuditDetail),
					SignUrl:      pgtype.Text{String: wxResp.SignURL, Valid: wxResp.SignURL != ""},
					SignState:    pgtype.Text{String: wxResp.SignState, Valid: wxResp.SignState != ""},
					SubMchID:     pgtype.Text{String: wxResp.SubMchID, Valid: wxResp.SubMchID != ""},
				})
				if err != nil {
					log.Error().Err(err).Msg("更新进件状态失败")
				}
				applyment.Status = updateStatus
				applyment.SignUrl = pgtype.Text{String: wxResp.SignURL, Valid: wxResp.SignURL != ""}
				applyment.SubMchID = pgtype.Text{String: wxResp.SubMchID, Valid: wxResp.SubMchID != ""}
			}

			// 如果开户成功，更新商户状态和支付配置
			if wxResp.SubMchID != "" && merchant.Status != "active" {
				// 创建或更新商户支付配置
				_, err = server.store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
					MerchantID: merchant.ID,
					SubMchID:   wxResp.SubMchID,
					Status:     "active",
				})
				if err != nil {
					log.Error().Err(err).Msg("创建商户支付配置失败")
				}

				// 更新商户状态为active
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
	resp := merchantApplymentStatusResponse{
		Status:     normalizedStatus,
		StatusDesc: getApplymentStatusDesc(normalizedStatus),
	}

	if applyment.SignUrl.Valid && applyment.SignUrl.String != "" {
		resp.SignURL = &applyment.SignUrl.String
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
	if strings.TrimSpace(validPeriod) == "" {
		return nil
	}
	begin, end := parseApplymentDateRange(validPeriod)
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

// mapWechatApplymentStatus 映射微信进件状态到本地状态
func mapWechatApplymentStatus(wxStatus string) string {
	switch wxStatus {
	case "APPLYMENT_STATE_EDITTING":
		return "pending"
	case "CHECKING", "ACCOUNT_NEED_VERIFY", "APPLYMENT_STATE_AUDITING", "AUDITING":
		return "auditing"
	case "APPLYMENT_STATE_REJECTED", "REJECTED", "APPLYMENT_STATE_CANCELED", "CANCELED":
		return "rejected"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED", "APPLYMENT_STATE_TO_BE_SIGNED", "NEED_SIGN":
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
	case "auditing":
		return "审核中"
	case "rejected":
		return "审核被拒绝"
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
	req.normalize()
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
		if existingApplyment.Status == "submitted" || existingApplyment.Status == "auditing" ||
			existingApplyment.Status == "to_be_signed" || existingApplyment.Status == "signing" {
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

	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("O%d%d", operator.ID, time.Now().Unix())
	organizationType := resolveApplymentOrganizationType(
		businessLicenseNumber,
		businessLicenseOCR.TypeOfEnterprise,
		operatorName,
		"2",
	)
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
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, legalPersonIDNumber)
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
		IDCardNumber:          encryptedIDCardNumber, // AES 加密存储
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
		AccountNumber:         encryptedAccountNumber, // AES 加密存储
		AccountName:           req.AccountName,
		ContactName:           legalPersonName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true}, // AES 加密存储
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

	// 检查是否配置了微信支付客户端
	if server.ecommerceClient == nil {
		log.Warn().Msg("微信支付客户端未配置，跳过提交微信进件")

		// 更新运营商状态
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
		ContactInfo: &wechat.ApplymentContactInfo{
			ContactType:         "65",
			ContactName:         wxEncryptedIDCardName,
			ContactIDCardNumber: wxEncryptedIDCardNumber,
			MobilePhone:         wxEncryptedMobilePhone,
		},
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
	Status       string    `json:"status"`                  // 状态
	StatusDesc   string    `json:"status_desc"`             // 状态描述
	ApplymentID  *int64    `json:"applyment_id,omitempty"`  // 微信进件ID
	SubMchID     string    `json:"sub_mch_id,omitempty"`    // 二级商户号（开户成功后返回）
	SignURL      *string   `json:"sign_url,omitempty"`      // 签约链接
	RejectReason string    `json:"reject_reason,omitempty"` // 拒绝原因
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
			status := mapOperatorStatusToApplymentStatus(operator.Status)
			updatedAt := operator.CreatedAt
			if operator.UpdatedAt.Valid {
				updatedAt = operator.UpdatedAt.Time
			}

			ctx.JSON(http.StatusOK, operatorApplymentStatusResponse{
				Status:     status,
				StatusDesc: getApplymentStatusDesc(status),
				CreatedAt:  operator.CreatedAt,
				UpdatedAt:  updatedAt,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果状态是已提交且配置了微信客户端，尝试查询微信最新状态
	if applyment.ApplymentID.Valid && server.ecommerceClient != nil {
		if applyment.Status == "submitted" || applyment.Status == "auditing" ||
			applyment.Status == "to_be_signed" || applyment.Status == "signing" {
			wxResp, err := server.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
			if err == nil {
				newStatus := mapWechatApplymentStatus(wxResp.ApplymentState)
				if newStatus == "finish" && wxResp.SubMchID == "" {
					newStatus = "submitted"
				}
				if newStatus == "rejected" || newStatus == "canceled" {
					_, _ = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
						ID:     operator.ID,
						Status: "active",
					})
				}
				if newStatus != applyment.Status {
					// 状态有变化，更新数据库
					updateParams := db.UpdateEcommerceApplymentStatusParams{
						ID:           applyment.ID,
						Status:       newStatus,
						RejectReason: getRejectReasonFromAuditDetail(wxResp.AuditDetail),
					}

					if wxResp.SubMchID != "" {
						// 开户成功，保存二级商户号
						_, _ = server.store.UpdateEcommerceApplymentSubMchID(ctx, db.UpdateEcommerceApplymentSubMchIDParams{
							ID:       applyment.ID,
							SubMchID: pgtype.Text{String: wxResp.SubMchID, Valid: true},
						})
						// 更新运营商的二级商户号
						_, _ = server.store.UpdateOperatorSubMchID(ctx, db.UpdateOperatorSubMchIDParams{
							ID:       operator.ID,
							SubMchID: pgtype.Text{String: wxResp.SubMchID, Valid: true},
						})
						applyment.SubMchID = pgtype.Text{String: wxResp.SubMchID, Valid: true}
					} else {
						_, _ = server.store.UpdateEcommerceApplymentStatus(ctx, updateParams)
					}
					applyment.Status = newStatus
					if len(wxResp.AuditDetail) > 0 {
						applyment.RejectReason = getRejectReasonFromAuditDetail(wxResp.AuditDetail)
					}
				}
			}
		}
	}

	resp := operatorApplymentStatusResponse{
		Status:     normalizeApplymentStatus(applyment.Status, applyment.SubMchID.Valid && applyment.SubMchID.String != ""),
		StatusDesc: getApplymentStatusDesc(normalizeApplymentStatus(applyment.Status, applyment.SubMchID.Valid && applyment.SubMchID.String != "")),
		CreatedAt:  applyment.CreatedAt,
		UpdatedAt:  applyment.UpdatedAt,
	}

	if applyment.ApplymentID.Valid {
		resp.ApplymentID = &applyment.ApplymentID.Int64
	}
	if applyment.SubMchID.Valid {
		resp.SubMchID = applyment.SubMchID.String
	}
	if applyment.SignUrl.Valid {
		resp.SignURL = &applyment.SignUrl.String
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
	default:
		// active / suspended / expired: 尚未发起或不再需要绑卡
		return "pending"
	}
}
