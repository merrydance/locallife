package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// ==================== 商户开户 ====================

// merchantBindBankRequest 商户绑定银行卡请求
type merchantBindBankRequest struct {
	// 银行账户信息
	AccountType     string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"` // 对公/对私
	AccountBank     string `json:"account_bank" binding:"required,max=128"`                                          // 开户银行
	BankAddressCode string `json:"bank_address_code" binding:"required"`                                             // 开户银行省市编码
	BankName        string `json:"bank_name"`                                                                        // 开户银行全称（支行）
	AccountNumber   string `json:"account_number" binding:"required"`                                                // 银行账号
	AccountName     string `json:"account_name" binding:"required,max=128"`                                          // 开户名称

	// 联系信息
	ContactPhone string `json:"contact_phone" binding:"required"`        // 联系手机号
	ContactEmail string `json:"contact_email" binding:"omitempty,email"` // 联系邮箱（可选）
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
// @Router /v1/merchant/bindbank [post]
// @Security BearerAuth
func (server *Server) merchantBindBank(ctx *gin.Context) {
	var req merchantBindBankRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("商户不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查商户状态：必须是 approved 或 pending_bindbank
	if merchant.Status != "approved" && merchant.Status != "pending_bindbank" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("商户状态不正确，当前状态: %s", merchant.Status)))
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
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已有进行中的开户申请，请等待审核结果")))
			return
		}
		if existingApplyment.Status == "finish" {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已完成开户，无需重复提交")))
			return
		}
	}

	// 获取商户申请信息（包含营业执照、身份证等OCR数据）
	application, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取商户申请信息失败: %w", err)))
		return
	}

	// 解析身份证OCR信息
	var idCardBackOCR MerchantIDCardOCRData
	if len(application.IDCardBackOcr) > 0 {
		if err := json.Unmarshal(application.IDCardBackOcr, &idCardBackOCR); err != nil {
			log.Error().Err(err).Msg("解析身份证OCR失败")
		}
	}

	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("M%d%d", merchant.ID, time.Now().Unix())

	// 判断主体类型
	// 如果有营业执照号，认为是个体工商户(2500)或企业(2600)
	// 否则是小微商户(2401)
	organizationType := "2401" // 默认小微商户
	if application.BusinessLicenseNumber != "" {
		organizationType = "2500" // 个体工商户
	}

	// 解析身份证有效期
	idCardValidTime := "长期"
	if idCardBackOCR.ValidDate != "" {
		// ValidDate 格式: "2020.01.01-2030.01.01" 或 "2020.01.01-长期"
		// 需要转换为微信格式: "YYYY-MM-DD" 或 "长期"
		idCardValidTime = parseIDCardValidTime(idCardBackOCR.ValidDate)
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

	// 创建进件记录
	applyment, err := server.store.CreateEcommerceApplyment(ctx, db.CreateEcommerceApplymentParams{
		SubjectType:           "merchant",
		SubjectID:             merchant.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: pgtype.Text{String: application.BusinessLicenseNumber, Valid: application.BusinessLicenseNumber != ""},
		BusinessLicenseCopy:   pgtype.Text{String: application.BusinessLicenseImageUrl, Valid: application.BusinessLicenseImageUrl != ""},
		MerchantName:          application.MerchantName,
		LegalPerson:           application.LegalPersonName,
		IDCardNumber:          encryptedIDCardNumber, // AES 加密存储
		IDCardName:            application.LegalPersonName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       application.LegalPersonIDFrontUrl,
		IDCardBackCopy:        application.LegalPersonIDBackUrl,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		BankAddressCode:       req.BankAddressCode,
		BankName:              pgtype.Text{String: req.BankName, Valid: req.BankName != ""},
		AccountNumber:         encryptedAccountNumber, // AES 加密存储
		AccountName:           req.AccountName,
		ContactName:           application.LegalPersonName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true}, // AES 加密存储
		MobilePhone:           req.ContactPhone,
		ContactEmail:          pgtype.Text{String: req.ContactEmail, Valid: req.ContactEmail != ""},
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

	// ==================== 上传图片到微信获取 MediaID ====================
	// 上传身份证正面
	idCardFrontMediaID, err := server.uploadImageToWechat(ctx, application.LegalPersonIDFrontUrl, "id_front.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传身份证正面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证正面失败: %w", err)))
		return
	}

	// 上传身份证背面
	idCardBackMediaID, err := server.uploadImageToWechat(ctx, application.LegalPersonIDBackUrl, "id_back.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传身份证背面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证背面失败: %w", err)))
		return
	}

	// 上传营业执照（如有）
	var businessLicenseMediaID string
	if application.BusinessLicenseImageUrl != "" {
		businessLicenseMediaID, err = server.uploadImageToWechat(ctx, application.BusinessLicenseImageUrl, "license.jpg")
		if err != nil {
			log.Error().Err(err).Msg("上传营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传营业执照失败: %w", err)))
			return
		}
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
	wxEncryptedMobilePhone, err := server.ecommerceClient.EncryptSensitiveData(req.ContactPhone)
	if err != nil {
		log.Error().Err(err).Msg("加密联系手机号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密联系邮箱（如有）
	var wxEncryptedContactEmail string
	if req.ContactEmail != "" {
		wxEncryptedContactEmail, err = server.ecommerceClient.EncryptSensitiveData(req.ContactEmail)
		if err != nil {
			log.Error().Err(err).Msg("加密联系邮箱失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
			return
		}
	}

	// ==================== 构建微信进件请求 ====================
	// 构建营业执照信息（如有）
	var businessLicenseInfo *wechat.BusinessLicenseInfo
	if businessLicenseMediaID != "" {
		businessLicenseInfo = &wechat.BusinessLicenseInfo{
			BusinessLicenseCopy:   businessLicenseMediaID,
			BusinessLicenseNumber: application.BusinessLicenseNumber,
			MerchantName:          application.MerchantName,
			LegalPerson:           application.LegalPersonName,
		}
	}

	applymentReq := &wechat.EcommerceApplymentRequest{
		OutRequestNo:      outRequestNo,
		OrganizationType:  organizationType,
		BusinessLicense:   businessLicenseInfo,
		MerchantShortname: merchant.Name,
		NeedAccountInfo:   true,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:      idCardFrontMediaID,
			IDCardNational:  idCardBackMediaID,
			IDCardName:      wxEncryptedIDCardName,
			IDCardNumber:    wxEncryptedIDCardNumber,
			IDCardValidTime: idCardValidTime,
		},
		AccountInfo: &wechat.ApplymentBankAccountInfo{
			BankAccountType: req.AccountType,
			AccountBank:     req.AccountBank,
			AccountName:     wxEncryptedAccountName,
			BankAddressCode: req.BankAddressCode,
			BankName:        req.BankName,
			AccountNumber:   wxEncryptedAccountNumber,
		},
		ContactInfo: &wechat.ApplymentContactInfo{
			ContactType:         "LEGAL",
			ContactName:         wxEncryptedIDCardName,
			ContactIDCardNumber: wxEncryptedIDCardNumber,
			MobilePhone:         wxEncryptedMobilePhone,
			ContactEmail:        wxEncryptedContactEmail,
		},
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName: merchant.Name,
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("商户不存在")))
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("未找到开户申请记录")))
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

	resp := merchantApplymentStatusResponse{
		Status:     applyment.Status,
		StatusDesc: getApplymentStatusDesc(applyment.Status),
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

// ==================== 辅助函数 ====================

// uploadImageToWechat 从 URL 下载图片并上传到微信支付获取 MediaID
// imageURL: 图片 URL（可以是本地存储或 OSS 的 URL）
// filename: 文件名（用于指定 Content-Type）
// 返回微信的 MediaID
func (server *Server) uploadImageToWechat(ctx *gin.Context, imageURL string, filename string) (string, error) {
	var imageData []byte

	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		// 外部URL：服务端拉取
		resp, err := http.Get(imageURL)
		if err != nil {
			return "", fmt.Errorf("下载图片失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("下载图片失败: HTTP %d", resp.StatusCode)
		}

		imageData, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取图片数据失败: %w", err)
		}
	} else {
		// 本地路径：直接读文件（不依赖公网/uploads）
		localPath := strings.TrimPrefix(normalizeUploadPath(imageURL), "/")
		f, err := os.Open(localPath)
		if err != nil {
			return "", fmt.Errorf("读取本地图片失败: %w", err)
		}
		defer f.Close()

		imageData, err = io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("读取图片数据失败: %w", err)
		}
	}

	// 检查图片大小（微信限制 2MB）
	if len(imageData) > 2*1024*1024 {
		return "", fmt.Errorf("图片大小超过 2MB 限制")
	}

	// 上传到微信
	uploadResp, err := server.ecommerceClient.UploadImage(ctx, filename, imageData)
	if err != nil {
		return "", fmt.Errorf("上传到微信失败: %w", err)
	}

	return uploadResp.MediaID, nil
}

// parseIDCardValidTime 解析身份证有效期
func parseIDCardValidTime(validDate string) string {
	// 输入格式: "2020.01.01-2030.01.01" 或 "2020.01.01-长期"
	// 输出格式: "2030-01-01" 或 "长期"
	if validDate == "" {
		return "长期"
	}

	// 分割获取结束日期
	parts := splitByDash(validDate)
	if len(parts) != 2 {
		return "长期"
	}

	endDate := parts[1]
	if endDate == "长期" {
		return "长期"
	}

	// 转换格式: 2030.01.01 -> 2030-01-01
	endDate = replaceAll(endDate, ".", "-")
	return endDate
}

// splitByDash 按 "-" 分割字符串
func splitByDash(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// replaceAll 替换所有匹配的字符
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if string(s[i]) == old {
			result += new
		} else {
			result += string(s[i])
		}
	}
	return result
}

// mapWechatApplymentStatus 映射微信进件状态到本地状态
func mapWechatApplymentStatus(wxStatus string) string {
	switch wxStatus {
	case "APPLYMENT_STATE_EDITTING":
		return "pending"
	case "APPLYMENT_STATE_AUDITING":
		return "auditing"
	case "APPLYMENT_STATE_REJECTED":
		return "rejected"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED":
		return "to_be_signed"
	case "APPLYMENT_STATE_TO_BE_SIGNED":
		return "to_be_signed"
	case "APPLYMENT_STATE_SIGNING":
		return "signing"
	case "APPLYMENT_STATE_FINISHED":
		return "finish"
	case "APPLYMENT_STATE_CANCELED":
		return "rejected"
	case "APPLYMENT_STATE_FROZEN":
		return "frozen"
	default:
		return "submitted"
	}
}

// getApplymentStatusDesc 获取进件状态描述
func getApplymentStatusDesc(status string) string {
	switch status {
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

// ==================== 骑手开户 ====================

// riderBindBankRequest 骑手绑定银行卡请求
type riderBindBankRequest struct {
	// 银行账户信息 - 骑手都是个人，使用对私账户
	AccountType     string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_PRIVATE"` // 账户类型（骑手只支持对私）
	AccountBank     string `json:"account_bank" binding:"required"`                            // 开户银行
	AccountName     string `json:"account_name" binding:"required"`                            // 账户名称
	BankAddressCode string `json:"bank_address_code" binding:"required"`                       // 开户银行省市编码
	BankName        string `json:"bank_name,omitempty"`                                        // 开户银行全称
	AccountNumber   string `json:"account_number" binding:"required"`                          // 银行账号
	ContactPhone    string `json:"contact_phone" binding:"required,validPhone"`                // 联系手机号
}

// riderBindBankResponse 骑手开户响应
type riderBindBankResponse struct {
	ApplymentID int64  `json:"applyment_id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// riderBindBank godoc
// @Summary 骑手绑定银行卡开户
// @Description 骑手审核通过后，提交银行卡信息进行微信支付二级商户开户
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body riderBindBankRequest true "银行卡信息"
// @Success 200 {object} riderBindBankResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/rider/applyment/bindbank [post]
// @Security BearerAuth
func (server *Server) riderBindBank(ctx *gin.Context) {
	var req riderBindBankRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取骑手信息
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查骑手状态：必须是 approved 或 pending_bindbank
	if rider.Status != "approved" && rider.Status != "pending_bindbank" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("骑手状态不正确，当前状态: %s", rider.Status)))
		return
	}

	// 检查是否已有进行中的进件申请
	existingApplyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "rider",
		SubjectID:   rider.ID,
	})
	if err == nil {
		if existingApplyment.Status == "submitted" || existingApplyment.Status == "auditing" ||
			existingApplyment.Status == "to_be_signed" || existingApplyment.Status == "signing" {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已有进行中的开户申请，请等待审核结果")))
			return
		}
		if existingApplyment.Status == "finish" {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已完成开户，无需重复提交")))
			return
		}
	}

	// 获取骑手申请信息（包含身份证等详细信息）
	riderApplication, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		log.Error().Err(err).Int64("rider_id", rider.ID).Msg("获取骑手申请信息失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取骑手申请信息失败")))
		return
	}

	// 解析身份证OCR信息
	var idCardOCR IDCardOCRData
	if len(riderApplication.IDCardOcr) > 0 {
		if err := json.Unmarshal(riderApplication.IDCardOcr, &idCardOCR); err != nil {
			log.Error().Err(err).Msg("解析骑手身份证OCR失败")
		}
	}

	// 获取身份证信息
	realName := ""
	if riderApplication.RealName.Valid {
		realName = riderApplication.RealName.String
	}
	idCardFrontURL := ""
	if riderApplication.IDCardFrontUrl.Valid {
		idCardFrontURL = riderApplication.IDCardFrontUrl.String
	}
	idCardBackURL := ""
	if riderApplication.IDCardBackUrl.Valid {
		idCardBackURL = riderApplication.IDCardBackUrl.String
	}

	// 从 OCR 获取身份证号和有效期
	idCardNumber := idCardOCR.IDNumber
	idCardValidTime := "长期"
	if idCardOCR.ValidEnd != "" {
		idCardValidTime = idCardOCR.ValidEnd
	}

	// 检查必要信息
	if realName == "" || idCardNumber == "" || idCardFrontURL == "" || idCardBackURL == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("骑手身份信息不完整，请先完善身份证信息")))
		return
	}

	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("R%d%d", rider.ID, time.Now().Unix())

	// 加密敏感数据（本地存储）
	encryptedIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, idCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedAccountNumber, err := util.EncryptSensitiveField(server.dataEncryptor, req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}
	encryptedContactIDCardNumber, err := util.EncryptSensitiveField(server.dataEncryptor, idCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手联系人身份证号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("数据加密失败")))
		return
	}

	// 创建进件记录
	applyment, err := server.store.CreateEcommerceApplyment(ctx, db.CreateEcommerceApplymentParams{
		SubjectType:           "rider",
		SubjectID:             rider.ID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      "2401", // 小微商户
		BusinessLicenseNumber: pgtype.Text{},
		BusinessLicenseCopy:   pgtype.Text{},
		MerchantName:          "骑手-" + realName,
		LegalPerson:           realName,
		IDCardNumber:          encryptedIDCardNumber, // AES 加密存储
		IDCardName:            realName,
		IDCardValidTime:       idCardValidTime,
		IDCardFrontCopy:       idCardFrontURL,
		IDCardBackCopy:        idCardBackURL,
		AccountType:           req.AccountType,
		AccountBank:           req.AccountBank,
		BankAddressCode:       req.BankAddressCode,
		BankName:              pgtype.Text{String: req.BankName, Valid: req.BankName != ""},
		AccountNumber:         encryptedAccountNumber, // AES 加密存储
		AccountName:           req.AccountName,
		ContactName:           realName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true}, // AES 加密存储
		MobilePhone:           req.ContactPhone,
		ContactEmail:          pgtype.Text{},
		MerchantShortname:     "骑手-" + realName,
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

		// 更新骑手状态
		_, _ = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
			ID:     rider.ID,
			Status: "bindbank_submitted",
		})

		ctx.JSON(http.StatusOK, riderBindBankResponse{
			ApplymentID: applyment.ID,
			Status:      "submitted",
			Message:     "银行卡信息已保存，待人工处理",
		})
		return
	}

	// ==================== 上传图片到微信获取 MediaID ====================
	// 上传身份证正面
	idCardFrontMediaID, err := server.uploadImageToWechat(ctx, idCardFrontURL, "id_front.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传骑手身份证正面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证正面失败: %w", err)))
		return
	}

	// 上传身份证背面
	idCardBackMediaID, err := server.uploadImageToWechat(ctx, idCardBackURL, "id_back.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传骑手身份证背面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证背面失败: %w", err)))
		return
	}

	// ==================== 加密敏感信息（用于发送给微信） ====================
	// 加密身份证姓名
	wxEncryptedIDCardName, err := server.ecommerceClient.EncryptSensitiveData(realName)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手身份证姓名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密身份证号码
	wxEncryptedIDCardNumber, err := server.ecommerceClient.EncryptSensitiveData(idCardNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手身份证号码失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账户名
	wxEncryptedAccountName, err := server.ecommerceClient.EncryptSensitiveData(req.AccountName)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手银行账户名失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密银行账号
	wxEncryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手银行账号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密联系手机号
	wxEncryptedMobilePhone, err := server.ecommerceClient.EncryptSensitiveData(req.ContactPhone)
	if err != nil {
		log.Error().Err(err).Msg("加密骑手联系手机号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// ==================== 构建微信进件请求（骑手作为小微商户） ====================
	applymentReq := &wechat.EcommerceApplymentRequest{
		OutRequestNo:      outRequestNo,
		OrganizationType:  "2401", // 小微商户
		MerchantShortname: "骑手-" + realName,
		NeedAccountInfo:   true,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:      idCardFrontMediaID,
			IDCardNational:  idCardBackMediaID,
			IDCardName:      wxEncryptedIDCardName,
			IDCardNumber:    wxEncryptedIDCardNumber,
			IDCardValidTime: idCardValidTime,
		},
		AccountInfo: &wechat.ApplymentBankAccountInfo{
			BankAccountType: req.AccountType,
			AccountBank:     req.AccountBank,
			AccountName:     wxEncryptedAccountName,
			BankAddressCode: req.BankAddressCode,
			BankName:        req.BankName,
			AccountNumber:   wxEncryptedAccountNumber,
		},
		ContactInfo: &wechat.ApplymentContactInfo{
			ContactType:         "65", // 经营者/法人
			ContactName:         wxEncryptedIDCardName,
			ContactIDCardNumber: wxEncryptedIDCardNumber,
			MobilePhone:         wxEncryptedMobilePhone,
		},
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName: "骑手-" + realName,
		},
	}

	// 提交微信进件
	resp, err := server.ecommerceClient.CreateEcommerceApplyment(ctx, applymentReq)
	if err != nil {
		log.Error().Err(err).Msg("提交微信骑手进件失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("提交微信开户失败: %w", err)))
		return
	}

	// 更新进件记录状态
	_, err = server.store.UpdateEcommerceApplymentToSubmitted(ctx, db.UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: resp.ApplymentID, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Msg("更新骑手进件状态失败")
	}

	// 更新骑手状态为 bindbank_submitted
	_, _ = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: "bindbank_submitted",
	})

	ctx.JSON(http.StatusOK, riderBindBankResponse{
		ApplymentID: resp.ApplymentID,
		Status:      "submitted",
		Message:     "开户申请已提交，请等待微信审核（通常1-3个工作日）",
	})
}

// riderApplymentStatusResponse 骑手开户状态响应
type riderApplymentStatusResponse struct {
	Status       string    `json:"status"`                  // 状态
	StatusDesc   string    `json:"status_desc"`             // 状态描述
	ApplymentID  *int64    `json:"applyment_id,omitempty"`  // 微信进件ID
	SubMchID     string    `json:"sub_mch_id,omitempty"`    // 二级商户号（开户成功后返回）
	RejectReason string    `json:"reject_reason,omitempty"` // 拒绝原因
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// getRiderApplymentStatus godoc
// @Summary 获取骑手开户状态
// @Description 获取骑手微信支付开户申请状态
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderApplymentStatusResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/rider/applyment/status [get]
// @Security BearerAuth
func (server *Server) getRiderApplymentStatus(ctx *gin.Context) {
	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取骑手信息
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取最新进件记录
	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "rider",
		SubjectID:   rider.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("未找到开户申请记录")))
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
						// 更新骑手的二级商户号
						_, _ = server.store.UpdateRiderSubMchID(ctx, db.UpdateRiderSubMchIDParams{
							ID:       rider.ID,
							SubMchID: pgtype.Text{String: wxResp.SubMchID, Valid: true},
						})
						// 更新骑手状态为 active
						_, _ = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
							ID:     rider.ID,
							Status: "active",
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

	resp := riderApplymentStatusResponse{
		Status:     applyment.Status,
		StatusDesc: getApplymentStatusDesc(applyment.Status),
		CreatedAt:  applyment.CreatedAt,
		UpdatedAt:  applyment.UpdatedAt,
	}

	if applyment.ApplymentID.Valid {
		resp.ApplymentID = &applyment.ApplymentID.Int64
	}
	if applyment.SubMchID.Valid {
		resp.SubMchID = applyment.SubMchID.String
	}
	if applyment.RejectReason.Valid {
		resp.RejectReason = applyment.RejectReason.String
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 运营商开户 ====================

// operatorBindBankRequest 运营商绑定银行卡请求
type operatorBindBankRequest struct {
	// 银行账户信息
	AccountType     string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"` // 对公/对私
	AccountBank     string `json:"account_bank" binding:"required,max=128"`                                          // 开户银行
	BankAddressCode string `json:"bank_address_code" binding:"required"`                                             // 开户银行省市编码
	BankName        string `json:"bank_name"`                                                                        // 开户银行全称（支行）
	AccountNumber   string `json:"account_number" binding:"required"`                                                // 银行账号
	AccountName     string `json:"account_name" binding:"required,max=128"`                                          // 开户名称

	// 联系信息
	ContactPhone string `json:"contact_phone" binding:"required"`        // 联系手机号
	ContactEmail string `json:"contact_email" binding:"omitempty,email"` // 联系邮箱（可选）
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

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取运营商信息
	operator, err := server.store.GetOperatorByUser(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("运营商不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查运营商状态：必须是 pending_bindbank
	if operator.Status != "pending_bindbank" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("运营商状态不正确，当前状态: %s", operator.Status)))
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
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已有进行中的开户申请，请等待审核结果")))
			return
		}
		if existingApplyment.Status == "finish" {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("已完成开户，无需重复提交")))
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

	// 解析身份证背面OCR信息（获取有效期）
	var idCardBackOCR OperatorIDCardBackOCR
	if len(application.IDCardBackOcr) > 0 {
		if err := json.Unmarshal(application.IDCardBackOcr, &idCardBackOCR); err != nil {
			log.Error().Err(err).Msg("解析运营商身份证背面OCR失败")
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
	idCardFrontURL := ""
	if application.IDCardFrontUrl.Valid {
		idCardFrontURL = application.IDCardFrontUrl.String
	}
	idCardBackURL := ""
	if application.IDCardBackUrl.Valid {
		idCardBackURL = application.IDCardBackUrl.String
	}
	businessLicenseURL := ""
	if application.BusinessLicenseUrl.Valid {
		businessLicenseURL = application.BusinessLicenseUrl.String
	}
	businessLicenseNumber := ""
	if application.BusinessLicenseNumber.Valid {
		businessLicenseNumber = application.BusinessLicenseNumber.String
	}
	operatorName := ""
	if application.Name.Valid {
		operatorName = application.Name.String
	}

	// 检查必要信息
	if legalPersonName == "" || legalPersonIDNumber == "" || idCardFrontURL == "" || idCardBackURL == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("运营商身份信息不完整，请先完善身份证信息")))
		return
	}

	// 解析身份证有效期
	idCardValidTime := "长期"
	if idCardBackOCR.ValidEnd != "" {
		idCardValidTime = idCardBackOCR.ValidEnd
	}

	// 生成业务申请编号
	outRequestNo := fmt.Sprintf("O%d%d", operator.ID, time.Now().Unix())

	// 判断主体类型：有营业执照号则为个体工商户/企业，否则为小微商户
	organizationType := "2401" // 默认小微商户
	if businessLicenseNumber != "" {
		organizationType = "2500" // 个体工商户
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
		BankAddressCode:       req.BankAddressCode,
		BankName:              pgtype.Text{String: req.BankName, Valid: req.BankName != ""},
		AccountNumber:         encryptedAccountNumber, // AES 加密存储
		AccountName:           req.AccountName,
		ContactName:           legalPersonName,
		ContactIDCardNumber:   pgtype.Text{String: encryptedContactIDCardNumber, Valid: true}, // AES 加密存储
		MobilePhone:           req.ContactPhone,
		ContactEmail:          pgtype.Text{String: req.ContactEmail, Valid: req.ContactEmail != ""},
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

	// ==================== 上传图片到微信获取 MediaID ====================
	// 上传身份证正面
	idCardFrontMediaID, err := server.uploadImageToWechat(ctx, idCardFrontURL, "id_front.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传运营商身份证正面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证正面失败: %w", err)))
		return
	}

	// 上传身份证背面
	idCardBackMediaID, err := server.uploadImageToWechat(ctx, idCardBackURL, "id_back.jpg")
	if err != nil {
		log.Error().Err(err).Msg("上传运营商身份证背面失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传身份证背面失败: %w", err)))
		return
	}

	// 上传营业执照（如有）
	var businessLicenseMediaID string
	if businessLicenseURL != "" {
		businessLicenseMediaID, err = server.uploadImageToWechat(ctx, businessLicenseURL, "license.jpg")
		if err != nil {
			log.Error().Err(err).Msg("上传运营商营业执照失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("上传营业执照失败: %w", err)))
			return
		}
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
	wxEncryptedMobilePhone, err := server.ecommerceClient.EncryptSensitiveData(req.ContactPhone)
	if err != nil {
		log.Error().Err(err).Msg("加密运营商联系手机号失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
		return
	}

	// 加密联系邮箱（如有）
	var wxEncryptedContactEmail string
	if req.ContactEmail != "" {
		wxEncryptedContactEmail, err = server.ecommerceClient.EncryptSensitiveData(req.ContactEmail)
		if err != nil {
			log.Error().Err(err).Msg("加密运营商联系邮箱失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("加密敏感信息失败")))
			return
		}
	}

	// ==================== 构建微信进件请求 ====================
	// 构建营业执照信息（如有）
	var businessLicenseInfo *wechat.BusinessLicenseInfo
	if businessLicenseMediaID != "" {
		businessLicenseInfo = &wechat.BusinessLicenseInfo{
			BusinessLicenseCopy:   businessLicenseMediaID,
			BusinessLicenseNumber: businessLicenseNumber,
			MerchantName:          operatorName,
			LegalPerson:           legalPersonName,
		}
	}

	applymentReq := &wechat.EcommerceApplymentRequest{
		OutRequestNo:      outRequestNo,
		OrganizationType:  organizationType,
		BusinessLicense:   businessLicenseInfo,
		MerchantShortname: operatorName,
		NeedAccountInfo:   true,
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:      idCardFrontMediaID,
			IDCardNational:  idCardBackMediaID,
			IDCardName:      wxEncryptedIDCardName,
			IDCardNumber:    wxEncryptedIDCardNumber,
			IDCardValidTime: idCardValidTime,
		},
		AccountInfo: &wechat.ApplymentBankAccountInfo{
			BankAccountType: req.AccountType,
			AccountBank:     req.AccountBank,
			AccountName:     wxEncryptedAccountName,
			BankAddressCode: req.BankAddressCode,
			BankName:        req.BankName,
			AccountNumber:   wxEncryptedAccountNumber,
		},
		ContactInfo: &wechat.ApplymentContactInfo{
			ContactType:         "LEGAL",
			ContactName:         wxEncryptedIDCardName,
			ContactIDCardNumber: wxEncryptedIDCardNumber,
			MobilePhone:         wxEncryptedMobilePhone,
			ContactEmail:        wxEncryptedContactEmail,
		},
		SalesSceneInfo: &wechat.ApplymentSalesSceneInfo{
			StoreName: operatorName,
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("运营商不存在")))
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("未找到开户申请记录")))
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
		Status:     applyment.Status,
		StatusDesc: getApplymentStatusDesc(applyment.Status),
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
