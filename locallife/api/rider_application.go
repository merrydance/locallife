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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
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
	return time.Parse("2006-01-02", year+"-"+month+"-"+day)
}

// riderApplicationResponse 骑手申请响应
type riderApplicationResponse struct {
	ID             int64              `json:"id"`
	UserID         int64              `json:"user_id"`
	RealName       *string            `json:"real_name,omitempty"`
	Phone          *string            `json:"phone,omitempty"`
	IDCardFrontURL *string            `json:"id_card_front_url,omitempty"`
	IDCardBackURL  *string            `json:"id_card_back_url,omitempty"`
	IDCardOCR      *IDCardOCRData     `json:"id_card_ocr,omitempty"`
	HealthCertURL  *string            `json:"health_cert_url,omitempty"`
	HealthCertOCR  *HealthCertOCRData `json:"health_cert_ocr,omitempty"`
	Status         string             `json:"status"`
	RejectReason   *string            `json:"reject_reason,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      *time.Time         `json:"updated_at,omitempty"`
	SubmittedAt    *time.Time         `json:"submitted_at,omitempty"`
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
	if app.IDCardFrontUrl.Valid {
		resp.IDCardFrontURL = &app.IDCardFrontUrl.String
	}
	if app.IDCardBackUrl.Valid {
		resp.IDCardBackURL = &app.IDCardBackUrl.String
	}
	if app.HealthCertUrl.Valid {
		resp.HealthCertURL = &app.HealthCertUrl.String
	}
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
	if !errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只能修改草稿状态的申请")))
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只能修改草稿状态的申请")))
		return
	}

	// 获取上传的文件
	file, fileHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请上传身份证图片")))
		return
	}
	defer file.Close()

	side := ctx.PostForm("side")
	if side != "Front" && side != "Back" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("side参数必须是Front或Back")))
		return
	}

	// 上传前内容安全检测：不通过则不保存
	if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
		if errors.Is(err, wechat.ErrRiskyContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片内容安全检测未通过")))
			return
		}
		if errors.Is(err, wechat.ErrImageTooLarge) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片过大，请压缩后再上传")))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat img sec check: %w", err)))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 保存图片到本地
	uploader := util.NewFileUploader("uploads")
	imageURL, err := uploader.UploadRiderImage(authPayload.UserID, "idcard", file, fileHeader)
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
		arg.IDCardFrontUrl = pgtype.Text{String: imageURL, Valid: true}

		if ocrResult != nil {
			// 构建OCR数据，合并已有数据
			var existingOCR IDCardOCRData
			if len(app.IDCardOcr) > 0 {
				json.Unmarshal(app.IDCardOcr, &existingOCR)
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
		arg.IDCardBackUrl = pgtype.Text{String: imageURL, Valid: true}

		if ocrResult != nil && ocrResult.ValidDate != "" {
			// 解析有效期，格式可能是 "20200101-20300101" 或 "20200101-长期"
			var existingOCR IDCardOCRData
			if len(app.IDCardOcr) > 0 {
				json.Unmarshal(app.IDCardOcr, &existingOCR)
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只能修改草稿状态的申请")))
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请上传健康证图片")))
		return
	}
	defer file.Close()

	// 上传前内容安全检测：不通过则不保存
	if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
		if errors.Is(err, wechat.ErrRiskyContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片内容安全检测未通过")))
			return
		}
		if errors.Is(err, wechat.ErrImageTooLarge) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片过大，请压缩后再上传")))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat img sec check: %w", err)))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	uploader := util.NewFileUploader("uploads")
	imageURL, err := uploader.UploadRiderImage(authPayload.UserID, "healthcert", file, fileHeader)
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
		HealthCertUrl: pgtype.Text{String: imageURL, Valid: true},
		HealthCertOcr: healthOCRBytes,
	}

	updated, err := server.store.UpdateRiderApplicationHealthCert(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider application health cert: %w", err)))
		return
	}

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
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只能提交草稿状态的申请")))
		return
	}

	// 验证必填信息
	var missingFields []string
	if !app.RealName.Valid || app.RealName.String == "" {
		missingFields = append(missingFields, "真实姓名")
	}
	if !app.Phone.Valid || app.Phone.String == "" {
		missingFields = append(missingFields, "手机号")
	}
	if !app.IDCardFrontUrl.Valid || app.IDCardFrontUrl.String == "" {
		missingFields = append(missingFields, "身份证正面照片")
	}
	if !app.IDCardBackUrl.Valid || app.IDCardBackUrl.String == "" {
		missingFields = append(missingFields, "身份证背面照片")
	}
	if !app.HealthCertUrl.Valid || app.HealthCertUrl.String == "" {
		missingFields = append(missingFields, "健康证照片")
	}

	if len(missingFields) > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请完善以下信息: "+joinStrings(missingFields, ", "))))
		return
	}

	// 自动审核：检查是否符合条件
	approved, rejectReason := server.checkRiderApplicationApproval(app)

	if approved {
		// 先提交再通过
		submitted, err := server.store.SubmitRiderApplication(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("submit rider application: %w", err)))
			return
		}

		// 自动通过
		approvedApp, err := server.store.ApproveRiderApplication(ctx, db.ApproveRiderApplicationParams{
			ID: submitted.ID,
		})
		if err != nil {
			log.Error().Err(err).Msg("审核骑手申请失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("approve rider application: %w", err)))
			return
		}

		// 创建骑手记录
		err = server.createRiderFromApplication(ctx, approvedApp)
		if err != nil {
			log.Error().Err(err).Msg("从申请创建骑手记录失败")
			// 回滚？暂时只记录日志
		}

		ctx.JSON(http.StatusOK, newRiderApplicationResponse(approvedApp))
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
	if !app.HealthCertUrl.Valid || app.HealthCertUrl.String == "" {
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

// createRiderFromApplication 从申请创建骑手记录
func (server *Server) createRiderFromApplication(ctx *gin.Context, app db.RiderApplication) error {
	// 获取身份证号
	var ocrData IDCardOCRData
	if len(app.IDCardOcr) > 0 {
		json.Unmarshal(app.IDCardOcr, &ocrData)
	}

	idCardNo := ocrData.IDNumber
	if idCardNo == "" {
		return errors.New("身份证号不能为空")
	}

	arg := db.CreateRiderParams{
		UserID:   app.UserID,
		RealName: app.RealName.String,
		IDCardNo: idCardNo,
		Phone:    app.Phone.String,
	}

	rider, err := server.store.CreateRider(ctx, arg)
	if err != nil {
		return err
	}

	// 更新申请表的关联（需要在riders表中保存application_id）
	// 这里简化处理，通过user_id关联
	log.Info().Int64("rider_id", rider.ID).Int64("application_id", app.ID).Msg("骑手记录创建成功")

	return nil
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
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("申请不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != "rejected" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只有被拒绝的申请才能重置")))
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
