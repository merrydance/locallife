package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// ==================== 骑手申请数据结构 ====================

// IDCardOCRData 身份证OCR识别数据
type IDCardOCRData struct {
	Name       string `json:"name,omitempty"`        // 姓名
	IDNumber   string `json:"id_number,omitempty"`   // 身份证号
	Gender     string `json:"gender,omitempty"`      // 性别
	Nation     string `json:"nation,omitempty"`      // 民族
	Address    string `json:"address,omitempty"`     // 地址
	ValidStart string `json:"valid_start,omitempty"` // 有效期起始
	ValidEnd   string `json:"valid_end,omitempty"`   // 有效期截止（"长期" 或日期）
	OCRAt      string `json:"ocr_at,omitempty"`      // OCR识别时间
}

// HealthCertOCRData 健康证OCR识别数据
type HealthCertOCRData struct {
	Name       string `json:"name,omitempty"`        // 姓名
	IDNumber   string `json:"id_number,omitempty"`   // 身份证号
	CertNumber string `json:"cert_number,omitempty"` // 证书编号
	ValidStart string `json:"valid_start,omitempty"` // 有效期起始
	ValidEnd   string `json:"valid_end,omitempty"`   // 有效期截止
	OCRAt      string `json:"ocr_at,omitempty"`      // OCR识别时间
}

func parseHealthCertOCRText(data *HealthCertOCRData, text string) {
	// 身份证号（18位，末位可能X）
	idRegex := regexp.MustCompile(`\b\d{17}[0-9Xx]\b`)
	if match := idRegex.FindString(text); match != "" {
		data.IDNumber = strings.ToUpper(match)
	}

	// 姓名（常见字段：姓名/持证人/从业人员姓名/体检者）
	nameRegex := regexp.MustCompile(`(?m)(?:从业人员姓名|持证人|体检者|姓名)\s*[:：]?\s*([^\n\r\s]{2,20})`)
	if match := nameRegex.FindStringSubmatch(text); len(match) > 1 {
		data.Name = strings.TrimSpace(match[1])
	}

	// 证书编号/证号/编号（尽量取一段不太短的字母数字串）
	certRegex := regexp.MustCompile(`(?m)(?:健康证号|证书编号|证号|编号)\s*[:：]?\s*([A-Za-z0-9\-]{5,})`)
	if match := certRegex.FindStringSubmatch(text); len(match) > 1 {
		data.CertNumber = strings.TrimSpace(match[1])
	}

	// 有效期（中文日期）
	// 1) 有效期至：2025年12月31日
	validToRegex := regexp.MustCompile(`(?:有效期至|有效期到|有效期)\s*[:：]?\s*(\d{4}年\d{1,2}月\d{1,2}日|长期)`)
	if match := validToRegex.FindStringSubmatch(text); len(match) > 1 {
		data.ValidEnd = strings.TrimSpace(match[1])
	}
	// 2) 起止：2020年01月01日至2025年12月31日
	validRangeRegex := regexp.MustCompile(`(\d{4}年\d{1,2}月\d{1,2}日)\s*[至到-]\s*(\d{4}年\d{1,2}月\d{1,2}日|长期)`)
	if match := validRangeRegex.FindStringSubmatch(text); len(match) > 2 {
		data.ValidStart = strings.TrimSpace(match[1])
		data.ValidEnd = strings.TrimSpace(match[2])
	}
}

func normalizePersonName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	return name
}

func parseChineseYMD(dateStr string) (time.Time, error) {
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	match := dateRegex.FindStringSubmatch(dateStr)
	if len(match) < 4 {
		return time.Time{}, fmt.Errorf("invalid chinese date: %s", dateStr)
	}
	year := match[1]
	month := match[2]
	day := match[3]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	return parseISODate(year+"-"+month+"-"+day, "")
}

// riderApplicationResponse 骑手申请响应
type riderApplicationResponse struct {
	ID                 int64              `json:"id"`
	UserID             int64              `json:"user_id"`
	RealName           *string            `json:"real_name,omitempty"`
	Phone              *string            `json:"phone,omitempty"`
	IDCardFrontAssetID *int64             `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID  *int64             `json:"id_card_back_asset_id,omitempty"`
	IDCardOCR          *IDCardOCRData     `json:"id_card_ocr,omitempty"`
	HealthCertAssetID  *int64             `json:"health_cert_asset_id,omitempty"`
	HealthCertOCR      *HealthCertOCRData `json:"health_cert_ocr,omitempty"`
	Status             string             `json:"status"`
	RejectReason       *string            `json:"reject_reason,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          *time.Time         `json:"updated_at,omitempty"`
	SubmittedAt        *time.Time         `json:"submitted_at,omitempty"`
}

func newRiderApplicationResponse(app db.RiderApplication) riderApplicationResponse {
	resp := riderApplicationResponse{
		ID:        app.ID,
		UserID:    app.UserID,
		Status:    app.Status,
		CreatedAt: app.CreatedAt,
	}

	if app.RealName.Valid {
		resp.RealName = &app.RealName.String
	}
	if app.Phone.Valid {
		resp.Phone = &app.Phone.String
	}
	resp.IDCardFrontAssetID = int64PtrFromPgInt8(app.IDCardFrontMediaAssetID)
	resp.IDCardBackAssetID = int64PtrFromPgInt8(app.IDCardBackMediaAssetID)
	resp.HealthCertAssetID = int64PtrFromPgInt8(app.HealthCertMediaAssetID)
	if app.RejectReason.Valid {
		resp.RejectReason = &app.RejectReason.String
	}
	if app.UpdatedAt.Valid {
		resp.UpdatedAt = &app.UpdatedAt.Time
	}
	if app.SubmittedAt.Valid {
		resp.SubmittedAt = &app.SubmittedAt.Time
	}

	// 解析身份证OCR数据
	if len(app.IDCardOcr) > 0 {
		var ocrData IDCardOCRData
		if err := json.Unmarshal(app.IDCardOcr, &ocrData); err == nil {
			resp.IDCardOCR = &ocrData
		}
	}

	// 解析健康证OCR数据
	if len(app.HealthCertOcr) > 0 {
		var ocrData HealthCertOCRData
		if err := json.Unmarshal(app.HealthCertOcr, &ocrData); err == nil {
			resp.HealthCertOCR = &ocrData
		}
	}

	return resp
}

// ==================== 创建/获取草稿 ====================

// createOrGetRiderApplicationDraft godoc
// @Summary 创建或获取骑手申请草稿
// @Description 如果用户已有申请则返回现有申请，否则创建新的草稿
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "申请信息"
// @Success 201 {object} riderApplicationResponse "新建草稿"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application [get]
// @Security BearerAuth
func (server *Server) createOrGetRiderApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查是否已有申请
	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err == nil {
		ctx.JSON(http.StatusOK, newRiderApplicationResponse(app))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	// 创建新草稿
	app, err = server.store.CreateRiderApplication(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create rider application draft: %w", err)))
		return
	}

	ctx.JSON(http.StatusCreated, newRiderApplicationResponse(app))
}

// ==================== 更新基础信息 ====================

type updateRiderApplicationBasicRequest struct {
	RealName *string `json:"real_name" binding:"omitempty,min=2,max=50"`
	Phone    *string `json:"phone" binding:"omitempty,validPhone"`
}

// updateRiderApplicationBasic godoc
// @Summary 更新骑手申请基础信息
// @Description 更新姓名、手机号等基础信息，仅草稿状态可修改
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Param request body updateRiderApplicationBasicRequest true "基础信息"
// @Success 200 {object} riderApplicationResponse "更新后的申请信息"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/basic [put]
// @Security BearerAuth
func (server *Server) updateRiderApplicationBasic(ctx *gin.Context) {
	var req updateRiderApplicationBasicRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	arg := db.UpdateRiderApplicationBasicInfoParams{
		ID: app.ID,
	}
	if req.RealName != nil {
		arg.RealName = pgtype.Text{String: *req.RealName, Valid: true}
	}
	if req.Phone != nil {
		arg.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}

	updated, err := server.store.UpdateRiderApplicationBasicInfo(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider application basic info: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newRiderApplicationResponse(updated))
}

// ==================== 身份证OCR识别 ====================

// uploadRiderIDCardOCR godoc
// @Summary 上传身份证并OCR识别
// @Description 上传身份证照片，调用微信OCR识别并保存结果
// @Tags 骑手申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "身份证图片"
// @Param side formData string true "正面Front/背面Back"
// @Success 200 {object} riderApplicationResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/idcard/ocr [post]
// @Security BearerAuth
func (server *Server) uploadRiderIDCardOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取申请
	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	// 获取上传的文件
	file, fileHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrIDCardImageRequired))
		return
	}
	defer file.Close()

	side := ctx.PostForm("side")
	if side != "Front" && side != "Back" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidIDCardSide))
		return
	}

	// 上传前内容安全检测：不通过则不保存
	if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
		if errors.Is(err, wechat.ErrRiskyContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageContentSafetyFailed))
			return
		}
		if errors.Is(err, wechat.ErrImageTooLarge) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageTooLarge))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat img sec check: %w", err)))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// TODO(media-service): oldIDCardURL was used to delete old file; now handled by media service
	var oldIDCardURL string
	_ = oldIDCardURL

	// 保存图片到本地
	uploader := util.NewFileUploader("uploads")
	_, err = uploader.UploadRiderImage(authPayload.UserID, "idcard", file, fileHeader)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("upload rider idcard image: %w", err)))
		return
	}

	// 重新打开文件用于OCR
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 调用微信OCR
	ocrResult, err := server.wechatClient.OCRIDCard(ctx, file, side)
	if err != nil {
		log.Error().Err(err).Msg("身份证OCR识别失败")
		// OCR失败不阻止保存图片URL，允许手动填写
	}

	// 准备更新参数
	arg := db.UpdateRiderApplicationIDCardParams{
		ID: app.ID,
	}

	if side == "Front" {
		// TODO(media-service): set IDCardFrontMediaAssetID after media upload

		if ocrResult != nil {
			// 构建OCR数据，合并已有数据
			var existingOCR IDCardOCRData
			if len(app.IDCardOcr) > 0 {
				_ = json.Unmarshal(app.IDCardOcr, &existingOCR)
			}
			existingOCR.Name = ocrResult.Name
			existingOCR.IDNumber = ocrResult.ID
			existingOCR.Gender = ocrResult.Gender
			existingOCR.Nation = ocrResult.Nation
			existingOCR.Address = ocrResult.Addr
			existingOCR.OCRAt = time.Now().Format(time.RFC3339)

			ocrJSON, _ := json.Marshal(existingOCR)
			arg.IDCardOcr = ocrJSON

			// 自动填充姓名
			if ocrResult.Name != "" {
				arg.RealName = pgtype.Text{String: ocrResult.Name, Valid: true}
			}
		}
	} else {
		// TODO(media-service): set IDCardBackMediaAssetID after media upload

		if ocrResult != nil && ocrResult.ValidDate != "" {
			// 解析有效期，格式可能是 "20200101-20300101" 或 "20200101-长期"
			var existingOCR IDCardOCRData
			if len(app.IDCardOcr) > 0 {
				_ = json.Unmarshal(app.IDCardOcr, &existingOCR)
			}
			existingOCR.ValidEnd = ocrResult.ValidDate
			existingOCR.OCRAt = time.Now().Format(time.RFC3339)

			ocrJSON, _ := json.Marshal(existingOCR)
			arg.IDCardOcr = ocrJSON
		}
	}

	updated, err := server.store.UpdateRiderApplicationIDCard(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider application idcard: %w", err)))
		return
	}
	server.deleteStoredImageAsync(oldIDCardURL)

	ctx.JSON(http.StatusOK, newRiderApplicationResponse(updated))
}

// ==================== 健康证上传 ====================

// uploadRiderHealthCert godoc
// @Summary 上传健康证
// @Description 上传健康证照片
// @Tags 骑手申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "健康证图片"
// @Success 200 {object} riderApplicationResponse "上传结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/healthcert [post]
// @Security BearerAuth
func (server *Server) uploadRiderHealthCert(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrHealthCertRequired))
		return
	}
	defer file.Close()

	// 上传前内容安全检测：不通过则不保存
	if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
		if errors.Is(err, wechat.ErrRiskyContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageContentSafetyFailed))
			return
		}
		if errors.Is(err, wechat.ErrImageTooLarge) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageTooLarge))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat img sec check: %w", err)))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// TODO(media-service): oldHealthCertURL was used to delete old file; now handled by media service
	oldHealthCertURL := ""
	_ = oldHealthCertURL

	uploader := util.NewFileUploader("uploads")
	_, err = uploader.UploadRiderImage(authPayload.UserID, "healthcert", file, fileHeader)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("upload rider healthcert image: %w", err)))
		return
	}

	// 重新回到文件开头用于OCR（通用印刷体，非证照接口）
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var healthOCRBytes []byte
	ocrResult, err := server.wechatClient.OCRPrintedText(ctx, file)
	if err != nil {
		log.Error().Err(err).Msg("健康证OCR识别失败")
	} else if ocrResult != nil {
		raw := ocrResult.GetAllText()
		ocrData := HealthCertOCRData{OCRAt: time.Now().Format(time.RFC3339)}
		parseHealthCertOCRText(&ocrData, raw)
		if b, err := json.Marshal(ocrData); err == nil {
			healthOCRBytes = b
		}
	}

	arg := db.UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertOcr: healthOCRBytes,
		// TODO(media-service): set HealthCertMediaAssetID after media upload
	}

	updated, err := server.store.UpdateRiderApplicationHealthCert(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider application health cert: %w", err)))
		return
	}
	server.deleteStoredImageAsync(oldHealthCertURL)

	ctx.JSON(http.StatusOK, newRiderApplicationResponse(updated))
}

// ==================== 提交申请 ====================

// submitRiderApplication godoc
// @Summary 提交骑手申请
// @Description 提交申请进行自动审核。条件：身份证在有效期内且健康证已上传则通过，否则直接拒绝
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "审核结果（approved或rejected）"
// @Failure 400 {object} ErrorResponse "信息不完整"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/submit [post]
// @Security BearerAuth
func (server *Server) submitRiderApplication(ctx *gin.Context) {
	consentReq, err := parseAgreementConsentRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationSubmitDraft))
		return
	}

	server.writeAgreementConsentAudit(ctx, authPayload.UserID, "rider_application_consent_confirmed", "rider_application", app.ID, consentReq)

	// 验证必填信息
	var missingFields []string
	if !app.RealName.Valid || app.RealName.String == "" {
		missingFields = append(missingFields, "真实姓名")
	}
	if !app.Phone.Valid || app.Phone.String == "" {
		missingFields = append(missingFields, "手机号")
	}
	if !app.IDCardFrontMediaAssetID.Valid {
		missingFields = append(missingFields, "身份证正面照片")
	}
	if !app.IDCardBackMediaAssetID.Valid {
		missingFields = append(missingFields, "身份证背面照片")
	}
	if !app.HealthCertMediaAssetID.Valid {
		missingFields = append(missingFields, "健康证照片")
	}

	if len(missingFields) > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("missing required fields: %s", joinStrings(missingFields, ", "))))
		return
	}

	// 自动审核：检查是否符合条件
	approved, rejectReason := server.checkRiderApplicationApproval(app)
	if server.config.RulesEngineEnabled && server.rulesEngine != nil {
		ruleInput := rules.Context{
			Domain: rules.DomainClaim,
			UserID: authPayload.UserID,
			Metadata: map[string]interface{}{
				"domain":               "rider_application",
				"health_cert_uploaded": app.HealthCertMediaAssetID.Valid,
				"idcard_ocr_valid":     len(app.IDCardOcr) > 0,
				"health_ocr_valid":     len(app.HealthCertOcr) > 0,
				"idcard_not_expired":   approved || rejectReason != "身份证已过期，请更换有效身份证后重新申请",
				"name_match":           approved || rejectReason != "健康证姓名与身份证姓名不一致",
			},
		}
		decision, err := server.rulesEngine.Evaluate(ctx, ruleInput)
		if err == nil {
			server.recordRuleHit(ctx, ruleInput, decision, RoleRider)
		}
	}

	if approved {
		// 先提交再通过
		submitted, err := server.store.SubmitRiderApplication(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("submit rider application: %w", err)))
			return
		}

		// 自动通过
		var ocrData IDCardOCRData
		if len(submitted.IDCardOcr) > 0 {
			_ = json.Unmarshal(submitted.IDCardOcr, &ocrData)
		}
		if ocrData.IDNumber == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrIDNumberRequired))
			return
		}

		approvedResult, err := server.store.ApproveRiderApplicationTx(ctx, db.ApproveRiderApplicationTxParams{
			ApplicationID: submitted.ID,
			ReviewedBy:    pgtype.Int8{},
			RiderRealName: submitted.RealName.String,
			RiderIDCardNo: ocrData.IDNumber,
			RiderPhone:    submitted.Phone.String,
			RegionID:      pgtype.Int8{},
		})
		if err != nil {
			log.Error().Err(err).Msg("审核骑手申请并创建骑手失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("approve rider application tx: %w", err)))
			return
		}

		ctx.JSON(http.StatusOK, newRiderApplicationResponse(approvedResult.Application))
		return
	}

	// 不符合条件，直接拒绝
	submitted, err := server.store.SubmitRiderApplication(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("submit rider application: %w", err)))
		return
	}

	rejected, err := server.store.RejectRiderApplication(ctx, db.RejectRiderApplicationParams{
		ID:           submitted.ID,
		RejectReason: pgtype.Text{String: rejectReason, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Msg("拒绝骑手申请失败")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reject rider application: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newRiderApplicationResponse(rejected))
}

// checkRiderApplicationApproval 检查申请是否符合通过条件
// 返回：是否通过，拒绝原因（如果不通过）
func (server *Server) checkRiderApplicationApproval(app db.RiderApplication) (bool, string) {
	// 1. 健康证必须已上传
	if !app.HealthCertMediaAssetID.Valid {
		return false, "健康证未上传"
	}

	// 2. 身份证OCR数据必须存在
	if len(app.IDCardOcr) == 0 {
		return false, "身份证信息未识别，请重新上传清晰的身份证照片"
	}

	var ocrData IDCardOCRData
	if err := json.Unmarshal(app.IDCardOcr, &ocrData); err != nil {
		return false, "身份证信息解析失败，请重新上传"
	}

	// 3. 身份证必须在有效期内
	if ocrData.ValidEnd == "" {
		return false, "身份证有效期未识别，请上传身份证背面照片"
	}

	// "长期"有效
	if ocrData.ValidEnd == "长期" {
		return true, ""
	}

	// 解析有效期
	validEnd := ocrData.ValidEnd
	if len(validEnd) > 8 {
		// 取最后8位作为结束日期
		validEnd = validEnd[len(validEnd)-8:]
	}

	endDate, err := time.Parse("20060102", validEnd)
	if err != nil {
		log.Error().Err(err).Str("valid_end", ocrData.ValidEnd).Msg("解析身份证有效期失败")
		return false, "身份证有效期格式无法识别，请联系客服"
	}

	if time.Now().After(endDate) {
		return false, "身份证已过期，请更换有效身份证后重新申请"
	}

	// 4. 健康证OCR数据必须存在（通用印刷体OCR解析）
	if len(app.HealthCertOcr) == 0 {
		return false, "健康证信息未识别，请重新上传清晰的健康证照片"
	}

	var healthOCR HealthCertOCRData
	if err := json.Unmarshal(app.HealthCertOcr, &healthOCR); err != nil {
		return false, "健康证信息解析失败，请重新上传"
	}

	// 5. 健康证必须与身份证一致（姓名+身份证号）
	idName := normalizePersonName(ocrData.Name)
	healthName := normalizePersonName(healthOCR.Name)
	if idName == "" {
		return false, "身份证姓名未识别，请重新上传清晰的身份证正面照片"
	}
	if healthName == "" {
		return false, "健康证姓名未识别，请重新上传清晰的健康证照片"
	}
	if idName != healthName {
		return false, "健康证姓名与身份证姓名不一致"
	}

	idNumber := strings.ToUpper(strings.TrimSpace(ocrData.IDNumber))
	healthID := strings.ToUpper(strings.TrimSpace(healthOCR.IDNumber))
	if idNumber == "" {
		return false, "身份证号码未识别，请重新上传清晰的身份证正面照片"
	}
	if healthID == "" {
		return false, "健康证身份证号码未识别，请重新上传清晰的健康证照片"
	}
	if idNumber != healthID {
		return false, "健康证身份证号码与身份证不一致"
	}

	// 6. 健康证有效期需超过当日7天
	if healthOCR.ValidEnd == "" {
		return false, "健康证有效期未识别，请重新上传清晰的健康证照片"
	}
	if strings.Contains(healthOCR.ValidEnd, "长期") || strings.Contains(healthOCR.ValidEnd, "永久") {
		return true, ""
	}
	validEndDate, err := parseChineseYMD(healthOCR.ValidEnd)
	if err != nil {
		log.Error().Err(err).Str("valid_end", healthOCR.ValidEnd).Msg("解析健康证有效期失败")
		return false, "健康证有效期格式无法识别，请重新上传"
	}
	if !validEndDate.After(time.Now().AddDate(0, 0, 7)) {
		return false, "健康证有效期需超过当日7天"
	}

	return true, ""
}

// ==================== 重置申请（被拒绝后） ====================

// resetRiderApplication godoc
// @Summary 重置骑手申请
// @Description 申请被拒绝后，重置为草稿状态以便重新编辑
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "重置后的申请"
// @Failure 400 {object} ErrorResponse "状态不允许重置"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/reset [post]
// @Security BearerAuth
func (server *Server) resetRiderApplication(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "rejected" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationCannotReset))
		return
	}

	reset, err := server.store.ResetRiderApplicationToDraft(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reset rider application: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newRiderApplicationResponse(reset))
}

// ==================== 辅助函数 ====================

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
