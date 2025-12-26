package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/rs/zerolog/log"
)

// loggingReader wraps io.ReadCloser to log progress
type loggingReader struct {
	r       io.ReadCloser
	total   int64
	lastLog time.Time
	reqID   string
}

func (l *loggingReader) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	l.total += int64(n)
	// Log every 1 second or on error/EOF
	if time.Since(l.lastLog) > 1*time.Second || err != nil {
		log.Info().Str("request_id", l.reqID).Int64("bytes_read", l.total).Msg("merchant OCR: upload progress")
		l.lastLog = time.Now()
	}
	return
}

func (l *loggingReader) Close() error {
	log.Info().Str("request_id", l.reqID).Int64("total_bytes", l.total).Msg("merchant OCR: upload body closed")
	return l.r.Close()
}

// ==================== 商户申请数据结构 ====================

// BusinessLicenseOCRData 营业执照OCR识别数据
type BusinessLicenseOCRData struct {
	Status              string `json:"status,omitempty"`               // pending/processing/done/failed
	Error               string `json:"error,omitempty"`                // failure reason (if any)
	QueuedAt            string `json:"queued_at,omitempty"`            // task enqueued time
	StartedAt           string `json:"started_at,omitempty"`           // task started processing time
	RegNum              string `json:"reg_num,omitempty"`              // 注册号
	EnterpriseName      string `json:"enterprise_name,omitempty"`      // 企业名称
	LegalRepresentative string `json:"legal_representative,omitempty"` // 法定代表人
	TypeOfEnterprise    string `json:"type_of_enterprise,omitempty"`   // 类型
	Address             string `json:"address,omitempty"`              // 地址
	BusinessScope       string `json:"business_scope,omitempty"`       // 经营范围
	RegisteredCapital   string `json:"registered_capital,omitempty"`   // 注册资本
	ValidPeriod         string `json:"valid_period"`                   // 营业期限（如：2020年01月01日至2040年01月01日 或 长期）
	CreditCode          string `json:"credit_code,omitempty"`          // 统一社会信用代码
	OCRAt               string `json:"ocr_at,omitempty"`               // OCR识别时间
}

// FoodPermitOCRData 食品经营许可证OCR识别数据（通用印刷体识别后解析）
type FoodPermitOCRData struct {
	Status      string `json:"status,omitempty"`       // pending/processing/done/failed
	Error       string `json:"error,omitempty"`        // failure reason (if any)
	QueuedAt    string `json:"queued_at,omitempty"`    // task enqueued time
	StartedAt   string `json:"started_at,omitempty"`   // task started processing time
	RawText     string `json:"raw_text,omitempty"`     // 原始OCR文本
	PermitNo    string `json:"permit_no,omitempty"`    // 许可证编号
	CompanyName string `json:"company_name,omitempty"` // 企业名称
	ValidFrom   string `json:"valid_from,omitempty"`   // 有效期起
	ValidTo     string `json:"valid_to,omitempty"`     // 有效期止（如：2025年12月31日 或 长期）
	OCRAt       string `json:"ocr_at,omitempty"`       // OCR识别时间
}

// MerchantIDCardOCRData 商户法人身份证OCR识别数据
type MerchantIDCardOCRData struct {
	Status    string `json:"status,omitempty"`     // pending/processing/done/failed
	Error     string `json:"error,omitempty"`      // failure reason (if any)
	QueuedAt  string `json:"queued_at,omitempty"`  // task enqueued time
	StartedAt string `json:"started_at,omitempty"` // task started processing time
	Name      string `json:"name,omitempty"`       // 姓名
	IDNumber  string `json:"id_number,omitempty"`  // 身份证号
	Gender    string `json:"gender,omitempty"`     // 性别
	Nation    string `json:"nation,omitempty"`     // 民族
	Address   string `json:"address,omitempty"`    // 地址
	ValidDate string `json:"valid_date,omitempty"` // 有效期（背面）
	OCRAt     string `json:"ocr_at,omitempty"`     // OCR识别时间
}

// merchantApplicationDraftResponse 商户申请草稿响应
type merchantApplicationDraftResponse struct {
	ID                      int64                   `json:"id"`
	UserID                  int64                   `json:"user_id"`
	MerchantName            string                  `json:"merchant_name"`
	ContactPhone            string                  `json:"contact_phone"`
	BusinessAddress         string                  `json:"business_address"`
	Longitude               *string                 `json:"longitude,omitempty"`
	Latitude                *string                 `json:"latitude,omitempty"`
	RegionID                *int64                  `json:"region_id,omitempty"`
	BusinessLicenseImageURL string                  `json:"business_license_image_url"`
	BusinessLicenseNumber   string                  `json:"business_license_number"`
	BusinessScope           *string                 `json:"business_scope,omitempty"`
	BusinessLicenseOCR      *BusinessLicenseOCRData `json:"business_license_ocr,omitempty"`
	FoodPermitURL           *string                 `json:"food_permit_url,omitempty"`
	FoodPermitOCR           *FoodPermitOCRData      `json:"food_permit_ocr,omitempty"`
	LegalPersonName         string                  `json:"legal_person_name"`
	LegalPersonIDNumber     string                  `json:"legal_person_id_number"`
	LegalPersonIDFrontURL   string                  `json:"legal_person_id_front_url"`
	LegalPersonIDBackURL    string                  `json:"legal_person_id_back_url"`
	IDCardFrontOCR          *MerchantIDCardOCRData  `json:"id_card_front_ocr,omitempty"`
	IDCardBackOCR           *MerchantIDCardOCRData  `json:"id_card_back_ocr,omitempty"`
	StorefrontImages        []string                `json:"storefront_images,omitempty"`
	EnvironmentImages       []string                `json:"environment_images,omitempty"`
	Status                  string                  `json:"status"`
	RejectReason            *string                 `json:"reject_reason,omitempty"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

func marshalOCRTaskPending() []byte {
	b, _ := json.Marshal(map[string]any{
		"status":    "pending",
		"queued_at": time.Now().Format(time.RFC3339),
	})
	return b
}

// checkApplicationEditable 检查申请是否可编辑
// 返回值: (是否可编辑, 是否需要重置为draft, 错误信息)
func checkApplicationEditable(status string) (editable bool, needReset bool, errMsg string) {
	switch status {
	case "draft":
		return true, false, ""
	case "submitted", "rejected", "approved":
		// 待审核、被拒绝或已通过的申请都可以编辑，但需要重置为草稿状态
		return true, true, ""
	default:
		return false, false, "申请状态异常"
	}
}

func newMerchantApplicationDraftResponse(app db.MerchantApplication) merchantApplicationDraftResponse {
	resp := merchantApplicationDraftResponse{
		ID:                      app.ID,
		UserID:                  app.UserID,
		MerchantName:            app.MerchantName,
		ContactPhone:            app.ContactPhone,
		BusinessAddress:         app.BusinessAddress,
		BusinessLicenseImageURL: app.BusinessLicenseImageUrl,
		BusinessLicenseNumber:   app.BusinessLicenseNumber,
		LegalPersonName:         app.LegalPersonName,
		LegalPersonIDNumber:     app.LegalPersonIDNumber,
		LegalPersonIDFrontURL:   app.LegalPersonIDFrontUrl,
		LegalPersonIDBackURL:    app.LegalPersonIDBackUrl,
		Status:                  app.Status,
		CreatedAt:               app.CreatedAt,
		UpdatedAt:               app.UpdatedAt,
	}

	// 经纬度
	if app.Longitude.Valid {
		lonStr := app.Longitude.Int.String()
		if app.Longitude.Exp < 0 {
			// 处理小数
			intStr := app.Longitude.Int.String()
			exp := int(-app.Longitude.Exp)
			if len(intStr) <= exp {
				intStr = strings.Repeat("0", exp-len(intStr)+1) + intStr
			}
			lonStr = intStr[:len(intStr)-exp] + "." + intStr[len(intStr)-exp:]
		}
		resp.Longitude = &lonStr
	}
	if app.Latitude.Valid {
		latStr := app.Latitude.Int.String()
		if app.Latitude.Exp < 0 {
			intStr := app.Latitude.Int.String()
			exp := int(-app.Latitude.Exp)
			if len(intStr) <= exp {
				intStr = strings.Repeat("0", exp-len(intStr)+1) + intStr
			}
			latStr = intStr[:len(intStr)-exp] + "." + intStr[len(intStr)-exp:]
		}
		resp.Latitude = &latStr
	}

	// 区域ID
	if app.RegionID.Valid {
		resp.RegionID = &app.RegionID.Int64
	}

	// 经营范围
	if app.BusinessScope.Valid {
		resp.BusinessScope = &app.BusinessScope.String
	}

	// 食品许可证URL
	if app.FoodPermitUrl.Valid {
		resp.FoodPermitURL = &app.FoodPermitUrl.String
	}

	// 拒绝原因
	if app.RejectReason.Valid {
		resp.RejectReason = &app.RejectReason.String
	}

	// 解析OCR数据
	if len(app.BusinessLicenseOcr) > 0 {
		var ocr BusinessLicenseOCRData
		if json.Unmarshal(app.BusinessLicenseOcr, &ocr) == nil {
			if ocr.Status == "" {
				ocr.Status = "done"
			}
			resp.BusinessLicenseOCR = &ocr
		}
	}
	if len(app.FoodPermitOcr) > 0 {
		var ocr FoodPermitOCRData
		if json.Unmarshal(app.FoodPermitOcr, &ocr) == nil {
			if ocr.Status == "" {
				ocr.Status = "done"
			}
			resp.FoodPermitOCR = &ocr
		}
	}
	if len(app.IDCardFrontOcr) > 0 {
		var ocr MerchantIDCardOCRData
		if json.Unmarshal(app.IDCardFrontOcr, &ocr) == nil {
			if ocr.Status == "" {
				ocr.Status = "done"
			}
			resp.IDCardFrontOCR = &ocr
		}
	}
	if len(app.IDCardBackOcr) > 0 {
		var ocr MerchantIDCardOCRData
		if json.Unmarshal(app.IDCardBackOcr, &ocr) == nil {
			if ocr.Status == "" {
				ocr.Status = "done"
			}
			resp.IDCardBackOCR = &ocr
		}
	}

	// 解析门头照和环境照（jsonb数组）
	if len(app.StorefrontImages) > 0 {
		var images []string
		if json.Unmarshal(app.StorefrontImages, &images) == nil {
			resp.StorefrontImages = images
		}
	}
	if len(app.EnvironmentImages) > 0 {
		var images []string
		if json.Unmarshal(app.EnvironmentImages, &images) == nil {
			resp.EnvironmentImages = images
		}
	}

	return resp
}

// ==================== 获取或创建草稿 ====================

// getOrCreateMerchantApplicationDraft godoc
// @Summary 获取或创建商户入驻申请草稿
// @Description 获取当前用户的商户入驻申请草稿，如果没有则创建一个空草稿。已通过的申请不会返回。
// @Tags 商户申请
// @Accept json
// @Produce json
// @Success 200 {object} merchantApplicationDraftResponse "获取成功"
// @Success 201 {object} merchantApplicationDraftResponse "创建成功"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 409 {object} ErrorResponse "已有通过或待审核的申请"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application [get]
// @Security BearerAuth
func (server *Server) getOrCreateMerchantApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 尝试获取草稿、待审核、被拒绝或已通过的申请（以便随时编辑）
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 创建新草稿
			newApp, err := server.store.CreateMerchantApplicationDraft(ctx, authPayload.UserID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			ctx.JSON(http.StatusCreated, newMerchantApplicationDraftResponse(newApp))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(app))
}

// ==================== 更新基础信息 ====================

type updateMerchantBasicInfoRequest struct {
	MerchantName    string  `json:"merchant_name" binding:"omitempty,min=2,max=50"`
	ContactPhone    string  `json:"contact_phone" binding:"omitempty,len=11"`
	BusinessAddress string  `json:"business_address" binding:"omitempty,min=5,max=200"`
	Longitude       *string `json:"longitude" binding:"omitempty"`
	Latitude        *string `json:"latitude" binding:"omitempty"`
	RegionID        *int64  `json:"region_id" binding:"omitempty"`
}

// updateMerchantApplicationBasicInfo godoc
// @Summary 更新商户申请基础信息
// @Description 更新商户名称、联系电话、地址、经纬度等基础信息。若提供经纬度但未提供 region_id，后端将自动匹配最近区域。支持在 approved/rejected 状态下直接编辑（会自动重置为 draft）。
// @Tags 商户申请
// @Accept json
// @Produce json
// @Param request body updateMerchantBasicInfoRequest true "基础信息"
// @Success 200 {object} merchantApplicationDraftResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/basic [put]
// @Security BearerAuth
func (server *Server) updateMerchantApplicationBasicInfo(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req updateMerchantBasicInfoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("update merchant basic info: JSON binding failed")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}
	log.Info().Str("request_id", requestID).Interface("req_payload", req).Msg("update merchant basic info received")

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请是否可编辑
	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
	// 如果是 rejected/approved 状态，自动重置为 draft
	if needReset {
		resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		app = resetResult.Application
	}

	// 构建更新参数
	arg := db.UpdateMerchantApplicationBasicInfoParams{
		ID: app.ID,
	}

	if req.MerchantName != "" {
		arg.MerchantName = pgtype.Text{String: req.MerchantName, Valid: true}
	}
	if req.ContactPhone != "" {
		arg.ContactPhone = pgtype.Text{String: req.ContactPhone, Valid: true}
	}
	if req.BusinessAddress != "" {
		arg.BusinessAddress = pgtype.Text{String: req.BusinessAddress, Valid: true}
	}
	if req.Longitude != nil {
		lon, err := parseNumericString(*req.Longitude)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("经度格式错误")))
			return
		}
		arg.Longitude = lon
	}
	if req.Latitude != nil {
		lat, err := parseNumericString(*req.Latitude)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("纬度格式错误")))
			return
		}
		arg.Latitude = lat
	}

	// 如果提供了经纬度但没有提供 RegionID（nil 或 0），尝试自动匹配最近的区域
	if (req.Longitude != nil || req.Latitude != nil) && (req.RegionID == nil || *req.RegionID == 0) {
		var lat, lon float64
		var err error

		if req.Latitude != nil {
			lat, err = parseNumericToFloat(arg.Latitude)
		} else if app.Latitude.Valid {
			lat, err = parseNumericToFloat(app.Latitude)
		}

		if err == nil {
			if req.Longitude != nil {
				lon, err = parseNumericToFloat(arg.Longitude)
			} else if app.Longitude.Valid {
				lon, err = parseNumericToFloat(app.Longitude)
			}
		}

		if err == nil {
			log.Info().Str("request_id", requestID).Float64("lat", lat).Float64("lon", lon).Msg("attempting auto region match")
			regionID, matchErr := server.matchRegionID(ctx, lat, lon)
			if matchErr == nil {
				arg.RegionID = pgtype.Int8{Int64: regionID, Valid: true}
				log.Info().Str("request_id", requestID).Int64("matched_region_id", regionID).Msg("auto region match success")
			} else {
				log.Warn().Str("request_id", requestID).Err(matchErr).Msg("auto region match failed")
			}
		} else {
			log.Warn().Str("request_id", requestID).Err(err).Msg("cannot parse lat/lon for region matching")
		}
	}

	if req.RegionID != nil && *req.RegionID > 0 {
		arg.RegionID = pgtype.Int8{Int64: *req.RegionID, Valid: true}
	}

	updatedApp, err := server.store.UpdateMerchantApplicationBasicInfo(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(updatedApp))
}

// ==================== 更新门头照和环境照 ====================

type updateMerchantImagesRequest struct {
	StorefrontImages  []string `json:"storefront_images"`  // 门头照URL数组，最多3张
	EnvironmentImages []string `json:"environment_images"` // 环境照URL数组，最多5张
}

// updateMerchantApplicationImages godoc
// @Summary 更新门头照和环境照
// @Description 保存商户门头照和店内环境照URL数组到草稿
// @Tags 商户申请
// @Accept json
// @Produce json
// @Param request body updateMerchantImagesRequest true "图片URL数组"
// @Success 200 {object} merchantApplicationDraftResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/images [put]
// @Security BearerAuth
func (server *Server) updateMerchantApplicationImages(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req updateMerchantImagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证图片数量限制
	if len(req.StorefrontImages) > 3 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("门头照最多3张")))
		return
	}
	if len(req.EnvironmentImages) > 5 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("环境照最多5张")))
		return
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请是否可编辑
	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
	// 如果是 rejected/approved 状态，自动重置为 draft
	if needReset {
		resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		app = resetResult.Application
	}

	// 构建更新参数
	arg := db.UpdateMerchantApplicationImagesParams{
		ID: app.ID,
	}

	if len(req.StorefrontImages) > 0 {
		for i, img := range req.StorefrontImages {
			req.StorefrontImages[i] = normalizeImageURLForStorage(img)
		}
		jsonData, _ := json.Marshal(req.StorefrontImages)
		arg.StorefrontImages = jsonData
	}
	if len(req.EnvironmentImages) > 0 {
		for i, img := range req.EnvironmentImages {
			req.EnvironmentImages[i] = normalizeImageURLForStorage(img)
		}
		jsonData, _ := json.Marshal(req.EnvironmentImages)
		arg.EnvironmentImages = jsonData
	}

	updatedApp, err := server.store.UpdateMerchantApplicationImages(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(updatedApp))
}

// ==================== 上传营业执照并OCR识别 ====================

// uploadMerchantBusinessLicenseOCR godoc
// @Summary 上传营业执照并OCR识别
// @Description 上传营业执照图片，调用微信OCR识别并保存结果，自动填充企业名称、信用代码、经营范围等
// @Tags 商户申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file false "营业执照图片（可选；不传则使用已上传的营业执照图片）"
// @Success 200 {object} merchantApplicationDraftResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/license/ocr [post]
// @Security BearerAuth
func (server *Server) uploadMerchantBusinessLicenseOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}

	start := time.Now()
	log.Info().Str("request_id", requestID).Msg("merchant OCR: handler started")

	// 获取申请
	t1 := time.Now()
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	log.Info().Str("request_id", requestID).Dur("duration", time.Since(t1)).Msg("merchant OCR: GetMerchantApplicationDraft done")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请是否可编辑
	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
	// 如果是 rejected/approved 状态，自动重置为 draft
	if needReset {
		resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		app = resetResult.Application
	}

	hasExistingOCR := len(app.BusinessLicenseOcr) > 0
	log.Info().
		Str("request_id", requestID).
		Int64("user_id", authPayload.UserID).
		Bool("has_existing_ocr", hasExistingOCR).
		Str("stored_image_url", app.BusinessLicenseImageUrl).
		Msg("merchant business license ocr: request received")

	// 获取上传的文件（兼容 image/file 字段）；如果未传文件则回退使用已上传的营业执照图片。
	var (
		file       multipart.File
		fileHeader *multipart.FileHeader
		fromUpload bool
	)

	// 调试日志：记录请求的 Content-Type 和表单解析状态
	contentType := ctx.Request.Header.Get("Content-Type")
	log.Info().
		Str("request_id", requestID).
		Str("content_type", contentType).
		Int64("content_length", ctx.Request.ContentLength).
		Dur("elapsed_since_start", time.Since(start)).
		Msg("merchant business license ocr: start parsing request body")

	// Wrap body for detailed logging
	ctx.Request.Body = &loggingReader{
		r:       ctx.Request.Body,
		reqID:   requestID,
		lastLog: time.Now(),
	}

	t2 := time.Now()
	// 注意：FormFile 会触发 ParseMultipartForm，这会读取整个 body。
	// 如果网络慢，这里会阻塞直到读取完成或超时。
	file, fileHeader, err = ctx.Request.FormFile("image")
	imageErr := err
	if err != nil {
		file, fileHeader, err = ctx.Request.FormFile("file")
	}
	fileErr := err

	log.Info().Str("request_id", requestID).Dur("duration", time.Since(t2)).Msg("merchant OCR: FormFile/ParseMultipartForm done")

	if err == nil {
		fromUpload = true
		defer file.Close()
		log.Info().
			Str("request_id", requestID).
			Str("filename", fileHeader.Filename).
			Int64("size", fileHeader.Size).
			Msg("merchant business license ocr: file received")
	} else {
		log.Warn().
			Str("request_id", requestID).
			Err(imageErr).
			AnErr("file_field_err", fileErr).
			Str("content_type", contentType).
			Msg("merchant business license ocr: no file in request")
	}

	// 未上传新文件且已有OCR：直接返回（避免重复OCR）
	if !fromUpload && hasExistingOCR {
		ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(app))
		return
	}

	updatedApp := app
	localPath := ""

	if fromUpload {
		t3 := time.Now()
		uploader := util.NewFileUploader("uploads")
		uploadedURL, err := uploader.UploadMerchantImage(authPayload.UserID, "business_license", file, fileHeader)
		log.Info().Str("request_id", requestID).Dur("duration", time.Since(t3)).Msg("merchant OCR: file saved to disk")
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		localPath = uploadedURL
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照图片地址不合法")))
			return
		}

		// 更新图片URL，并清空旧 OCR（避免旧 OCR 与新图不一致）
		saveArg := db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                      app.ID,
			BusinessLicenseImageUrl: pgtype.Text{String: uploadedURL, Valid: true},
			BusinessLicenseOcr:      marshalOCRTaskPending(),
		}
		updated, err := server.store.UpdateMerchantApplicationBusinessLicense(ctx, saveArg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		log.Info().Str("request_id", requestID).Msg("merchant OCR: DB updated with new image URL")
		updatedApp = updated
	} else {
		if app.BusinessLicenseImageUrl == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上传营业执照图片")))
			return
		}
		localPath = app.BusinessLicenseImageUrl
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照图片地址不合法")))
			return
		}

		// 使用已上传的图片触发OCR：写入 pending 状态
		saveArg := db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                 app.ID,
			BusinessLicenseOcr: marshalOCRTaskPending(),
		}
		updated, err := server.store.UpdateMerchantApplicationBusinessLicense(ctx, saveArg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		log.Info().Str("request_id", requestID).Msg("merchant OCR: DB updated (using existing image)")
		updatedApp = updated
	}

	if err := server.taskDistributor.DistributeTaskMerchantApplicationBusinessLicenseOCR(ctx, app.ID, localPath); err != nil {
		log.Error().
			Str("request_id", requestID).
			Int64("application_id", app.ID).
			Str("image_path", localPath).
			Err(err).
			Msg("merchant business license ocr: enqueue task failed")
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}
	log.Info().Str("request_id", requestID).Msg("merchant OCR: task distributed successfully")

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(updatedApp))
	log.Info().Str("request_id", requestID).Msg("merchant OCR: response sent")
}

// ==================== 上传食品经营许可证并OCR识别 ====================

// uploadMerchantFoodPermitOCR godoc
// @Summary 上传食品经营许可证并OCR识别
// @Description 上传食品经营许可证图片，使用通用印刷体OCR识别并解析有效期等信息
// @Tags 商户申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "食品经营许可证图片"
// @Success 200 {object} merchantApplicationDraftResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/foodpermit/ocr [post]
// @Security BearerAuth
func (server *Server) uploadMerchantFoodPermitOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请是否可编辑
	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
	// 如果是 rejected/approved 状态，自动重置为 draft
	if needReset {
		resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		app = resetResult.Application
	}

	hasExistingOCR := len(app.FoodPermitOcr) > 0
	log.Info().
		Str("request_id", requestID).
		Int64("user_id", authPayload.UserID).
		Bool("has_existing_ocr", hasExistingOCR).
		Str("stored_image_url", app.FoodPermitUrl.String).
		Msg("merchant food permit ocr: request received")

	// 获取上传的文件（兼容 image/file 字段）；如果未传文件则回退使用已上传的食品证图片。
	var (
		file       multipart.File
		fileHeader *multipart.FileHeader
		fromUpload bool
	)
	file, fileHeader, err = ctx.Request.FormFile("image")
	if err != nil {
		file, fileHeader, err = ctx.Request.FormFile("file")
	}
	if err == nil {
		fromUpload = true
		defer file.Close()
	}

	// 未上传新文件且已有OCR：直接返回（避免重复OCR）
	if !fromUpload && hasExistingOCR {
		ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(app))
		return
	}

	updatedApp := app
	localPath := ""

	if fromUpload {
		uploader := util.NewFileUploader("uploads")
		uploadedURL, err := uploader.UploadMerchantImage(authPayload.UserID, "food_permit", file, fileHeader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		localPath = uploadedURL
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证图片地址不合法")))
			return
		}

		saveArg := db.UpdateMerchantApplicationFoodPermitParams{
			ID:            app.ID,
			FoodPermitUrl: pgtype.Text{String: uploadedURL, Valid: true},
			FoodPermitOcr: marshalOCRTaskPending(),
		}
		updated, err := server.store.UpdateMerchantApplicationFoodPermit(ctx, saveArg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		updatedApp = updated
	} else {
		if !app.FoodPermitUrl.Valid || app.FoodPermitUrl.String == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上传食品经营许可证图片")))
			return
		}
		localPath = app.FoodPermitUrl.String
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证图片地址不合法")))
			return
		}

		// 使用已上传的图片触发OCR：写入 pending 状态
		saveArg := db.UpdateMerchantApplicationFoodPermitParams{
			ID:            app.ID,
			FoodPermitOcr: marshalOCRTaskPending(),
		}
		updated, err := server.store.UpdateMerchantApplicationFoodPermit(ctx, saveArg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		updatedApp = updated
	}

	if err := server.taskDistributor.DistributeTaskMerchantApplicationFoodPermitOCR(ctx, app.ID, localPath); err != nil {
		log.Error().
			Str("request_id", requestID).
			Int64("application_id", app.ID).
			Str("image_path", localPath).
			Err(err).
			Msg("merchant food permit ocr: enqueue task failed")
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(updatedApp))
}

// parseFoodPermitOCRText 从OCR文本中解析食品经营许可证信息
// ==================== 上传身份证并OCR识别 ====================

// uploadMerchantIDCardOCR godoc
// @Summary 上传法人身份证并OCR识别
// @Description 上传法人身份证照片（正面或背面），调用微信OCR识别并保存结果
// @Tags 商户申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "身份证图片"
// @Param side formData string true "正面Front/背面Back"
// @Success 200 {object} merchantApplicationDraftResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/idcard/ocr [post]
// @Security BearerAuth
func (server *Server) uploadMerchantIDCardOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请是否可编辑
	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
	// 如果是 rejected/approved 状态，自动重置为 draft
	if needReset {
		resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		app = resetResult.Application
	}

	side := ctx.PostForm("side")
	if side != "Front" && side != "Back" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("side参数必须是Front或Back")))
		return
	}

	var (
		file       multipart.File
		fileHeader *multipart.FileHeader
		fromUpload bool
	)
	file, fileHeader, err = ctx.Request.FormFile("image")
	if err != nil {
		file, fileHeader, err = ctx.Request.FormFile("file")
	}
	if err == nil {
		fromUpload = true
		defer file.Close()
	}

	hasExistingOCR := false
	storedPath := ""
	if side == "Front" {
		hasExistingOCR = len(app.IDCardFrontOcr) > 0
		storedPath = app.LegalPersonIDFrontUrl
	} else {
		hasExistingOCR = len(app.IDCardBackOcr) > 0
		storedPath = app.LegalPersonIDBackUrl
	}

	log.Info().
		Str("request_id", requestID).
		Int64("user_id", authPayload.UserID).
		Str("side", side).
		Bool("has_existing_ocr", hasExistingOCR).
		Str("stored_image_url", storedPath).
		Msg("merchant id card ocr: request received")

	// 未上传新文件且已有OCR：直接返回（避免重复OCR）
	if !fromUpload && hasExistingOCR {
		ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(app))
		return
	}

	updatedApp := app
	localPath := ""

	if fromUpload {
		category := "id_front"
		if side == "Back" {
			category = "id_back"
		}
		uploader := util.NewFileUploader("uploads")
		uploadedURL, err := uploader.UploadMerchantImage(authPayload.UserID, category, file, fileHeader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		localPath = uploadedURL
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("身份证图片地址不合法")))
			return
		}

		if side == "Front" {
			saveArg := db.UpdateMerchantApplicationIDCardFrontParams{
				ID:                    app.ID,
				LegalPersonIDFrontUrl: pgtype.Text{String: uploadedURL, Valid: true},
				IDCardFrontOcr:        marshalOCRTaskPending(),
			}
			updated, err := server.store.UpdateMerchantApplicationIDCardFront(ctx, saveArg)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			updatedApp = updated
		} else {
			saveArg := db.UpdateMerchantApplicationIDCardBackParams{
				ID:                   app.ID,
				LegalPersonIDBackUrl: pgtype.Text{String: uploadedURL, Valid: true},
				IDCardBackOcr:        marshalOCRTaskPending(),
			}
			updated, err := server.store.UpdateMerchantApplicationIDCardBack(ctx, saveArg)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			updatedApp = updated
		}
	} else {
		if storedPath == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上传身份证图片")))
			return
		}
		localPath = storedPath
		if strings.Contains(localPath, "/uploads/") {
			localPath = localPath[strings.Index(localPath, "/uploads/")+1:]
		}
		localPath = filepath.Clean(localPath)
		if !strings.HasPrefix(localPath, "uploads/") {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("身份证图片地址不合法")))
			return
		}

		// 使用已上传的图片触发OCR：写入 pending 状态
		if side == "Front" {
			saveArg := db.UpdateMerchantApplicationIDCardFrontParams{
				ID:             app.ID,
				IDCardFrontOcr: marshalOCRTaskPending(),
			}
			updated, err := server.store.UpdateMerchantApplicationIDCardFront(ctx, saveArg)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			updatedApp = updated
		} else {
			saveArg := db.UpdateMerchantApplicationIDCardBackParams{
				ID:            app.ID,
				IDCardBackOcr: marshalOCRTaskPending(),
			}
			updated, err := server.store.UpdateMerchantApplicationIDCardBack(ctx, saveArg)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			updatedApp = updated
		}
	}

	if err := server.taskDistributor.DistributeTaskMerchantApplicationIDCardOCR(ctx, app.ID, localPath, side); err != nil {
		log.Error().
			Str("request_id", requestID).
			Int64("application_id", app.ID).
			Str("side", side).
			Str("image_path", localPath).
			Err(err).
			Msg("merchant id card ocr: enqueue task failed")
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(updatedApp))
}

// ==================== 提交申请 ====================

// submitMerchantApplication godoc
// @Summary 提交商户入驻申请
// @Description 提交商户入驻申请，系统自动审核。提交前必须确保所有必填项（包括 region_id）已填充。支持从 draft/rejected/approved 状态提交。
// @Tags 商户申请
// @Accept json
// @Produce json
// @Success 200 {object} merchantApplicationDraftResponse "申请结果（approved/rejected）"
// @Failure 400 {object} ErrorResponse "参数不完整或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/submit [post]
// @Security BearerAuth
func (server *Server) submitMerchantApplication(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("请先创建申请")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 允许提交的状态：draft, rejected, approved, submitted (用于重试)
	if app.Status != "draft" && app.Status != "rejected" && app.Status != "approved" && app.Status != "submitted" {
		log.Warn().Str("request_id", requestID).Str("current_status", app.Status).Msg("submit failed: invalid status")
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("当前申请状态不可提交")))
		return
	}

	// 检查必填字段
	if err := validateMerchantApplicationRequired(app); err != nil {
		log.Warn().Str("request_id", requestID).Err(err).Msg("submit failed: validation error")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 提交申请：支持从 draft, rejected, approved 状态提交
	// 如果已经是 submitted 状态，则直接使用当前记录（支持重试自动审核）
	var submittedApp db.MerchantApplication
	if app.Status == "submitted" {
		submittedApp = app
	} else {
		var err error
		submittedApp, err = server.store.SubmitMerchantApplication(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 执行自动审核
	approved, rejectReason := server.checkMerchantApplicationApproval(ctx, submittedApp)

	if approved {
		// 审核通过 - 使用事务确保原子性
		appData, _ := json.Marshal(map[string]interface{}{
			"business_license_number":    submittedApp.BusinessLicenseNumber,
			"legal_person_name":          submittedApp.LegalPersonName,
			"legal_person_id_number":     submittedApp.LegalPersonIDNumber,
			"business_license_image_url": submittedApp.BusinessLicenseImageUrl,
			"legal_person_id_front_url":  submittedApp.LegalPersonIDFrontUrl,
			"legal_person_id_back_url":   submittedApp.LegalPersonIDBackUrl,
			"food_permit_url":            submittedApp.FoodPermitUrl.String,
		})

		var regionID int64
		if submittedApp.RegionID.Valid {
			regionID = submittedApp.RegionID.Int64
		}

		// 事务性审核：更新申请状态 + 创建商户 + 创建用户角色
		txResult, err := server.store.ApproveMerchantApplicationTx(ctx, db.ApproveMerchantApplicationTxParams{
			ApplicationID: submittedApp.ID,
			UserID:        submittedApp.UserID,
			MerchantName:  submittedApp.MerchantName,
			Phone:         submittedApp.ContactPhone,
			Address:       submittedApp.BusinessAddress,
			Latitude:      submittedApp.Latitude,
			Longitude:     submittedApp.Longitude,
			RegionID:      regionID,
			AppData:       appData,
		})
		if err != nil {
			log.Error().Err(err).Int64("application_id", submittedApp.ID).Msg("商户审核通过事务失败")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		log.Info().
			Int64("application_id", txResult.Application.ID).
			Int64("merchant_id", txResult.Merchant.ID).
			Int64("user_role_id", txResult.UserRole.ID).
			Msg("商户审核通过事务完成")

		ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(txResult.Application))
		return
	}

	// 审核拒绝
	rejectedApp, err := server.store.RejectMerchantApplication(ctx, db.RejectMerchantApplicationParams{
		ID:           submittedApp.ID,
		RejectReason: pgtype.Text{String: rejectReason, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(rejectedApp))
}

// validateMerchantApplicationRequired 验证必填字段
func validateMerchantApplicationRequired(app db.MerchantApplication) error {
	if app.MerchantName == "" {
		return errors.New("商户名称不能为空")
	}
	if app.ContactPhone == "" {
		return errors.New("联系电话不能为空")
	}
	if app.BusinessAddress == "" {
		return errors.New("商户地址不能为空")
	}
	if !app.Longitude.Valid || !app.Latitude.Valid {
		return errors.New("请选择商户地理位置")
	}
	if !app.RegionID.Valid {
		return errors.New("请选择所属区域")
	}
	if app.BusinessLicenseImageUrl == "" {
		return errors.New("请上传营业执照")
	}
	if !app.FoodPermitUrl.Valid || app.FoodPermitUrl.String == "" {
		return errors.New("请上传食品经营许可证")
	}
	if app.LegalPersonIDFrontUrl == "" {
		return errors.New("请上传身份证正面照")
	}
	if app.LegalPersonIDBackUrl == "" {
		return errors.New("请上传身份证背面照")
	}
	return nil
}

// checkMerchantApplicationApproval 检查商户申请是否符合自动通过条件
// 审核规则：
// 1. 营业执照在有效期内
// 2. 营业执照经营范围或名称包含餐饮相关关键词
// 3. 营业执照地址与商户定位地址模糊匹配
// 4. 食品经营许可证在有效期内
// 5. 法人身份证在有效期内
// 6. 有经纬度信息
// 7. 地址未被其他商户占用
func (server *Server) checkMerchantApplicationApproval(ctx *gin.Context, app db.MerchantApplication) (bool, string) {
	// 1. 检查经纬度
	if !app.Longitude.Valid || !app.Latitude.Valid {
		return false, "请选择商户地理位置"
	}

	// 2. 检查营业执照OCR数据
	if len(app.BusinessLicenseOcr) == 0 {
		return false, "营业执照信息未识别，请重新上传清晰的营业执照照片"
	}

	var licenseOCR BusinessLicenseOCRData
	if err := json.Unmarshal(app.BusinessLicenseOcr, &licenseOCR); err != nil {
		return false, "营业执照信息解析失败，请重新上传"
	}

	// 3. 检查营业执照有效期
	if !isValidPeriodValid(licenseOCR.ValidPeriod) {
		return false, "营业执照已过期或有效期无法识别"
	}

	// 4. 检查经营范围是否包含餐饮相关
	if !isCateringBusiness(licenseOCR.EnterpriseName, licenseOCR.BusinessScope) {
		return false, "经营范围不包含餐饮相关内容"
	}

	// 5. 检查地址匹配（营业执照地址与填写的商户地址）
	if !isAddressMatch(licenseOCR.Address, app.BusinessAddress) {
		return false, "营业执照地址与商户地址不匹配"
	}

	// 6. 检查地址是否已被占用
	addressExists, err := server.store.CheckMerchantAddressExists(ctx, db.CheckMerchantAddressExistsParams{
		Address:     app.BusinessAddress,
		OwnerUserID: app.UserID,
	})
	if err != nil {
		log.Error().Err(err).Msg("检查地址唯一性失败")
		return false, "系统错误，请稍后重试"
	}
	if addressExists {
		return false, "该地址已有商户入驻，同一地址不能注册两家餐厅"
	}

	// 7. 检查食品经营许可证
	if len(app.FoodPermitOcr) == 0 {
		return false, "食品经营许可证信息未识别，请重新上传清晰的照片"
	}

	var foodPermitOCR FoodPermitOCRData
	if err := json.Unmarshal(app.FoodPermitOcr, &foodPermitOCR); err != nil {
		return false, "食品经营许可证信息解析失败，请重新上传"
	}

	// 检查食品经营许可证有效期
	if !isFoodPermitValid(foodPermitOCR.ValidTo) {
		return false, "食品经营许可证已过期或有效期无法识别"
	}

	// 新增规则：食品经营许可证企业名称必须与营业执照企业名称一致
	licenseName := normalizeCompanyName(licenseOCR.EnterpriseName)
	permitName := normalizeCompanyName(foodPermitOCR.CompanyName)
	if licenseName == "" {
		return false, "营业执照企业名称未识别，请重新上传清晰的营业执照照片"
	}
	if permitName == "" {
		return false, "食品经营许可证企业名称未识别，请重新上传清晰的食品经营许可证照片"
	}
	if licenseName != permitName {
		return false, "食品经营许可证企业名称与营业执照企业名称不一致"
	}

	// 新增规则：食品经营许可证有效期需超过提交当日30天
	if !isChineseDateAtLeastDaysAfterNow(foodPermitOCR.ValidTo, 30) {
		return false, "食品经营许可证有效期需超过提交当日30天"
	}

	// 7. 检查身份证正面信息（姓名）
	if len(app.IDCardFrontOcr) == 0 {
		return false, "身份证正面信息未识别，请重新上传身份证正面照片"
	}

	var idCardFrontOCR MerchantIDCardOCRData
	if err := json.Unmarshal(app.IDCardFrontOcr, &idCardFrontOCR); err != nil {
		return false, "身份证正面信息解析失败，请重新上传"
	}

	// 新增规则：身份证姓名必须与营业执照法人一致
	licenseLegalPerson := strings.TrimSpace(licenseOCR.LegalRepresentative)
	idCardName := strings.TrimSpace(idCardFrontOCR.Name)
	if licenseLegalPerson == "" {
		return false, "营业执照法人姓名未识别，请重新上传清晰的营业执照照片"
	}
	if idCardName == "" {
		return false, "身份证姓名未识别，请重新上传清晰的身份证正面照片"
	}
	if licenseLegalPerson != idCardName {
		return false, fmt.Sprintf("身份证姓名（%s）与营业执照法人（%s）不一致", idCardName, licenseLegalPerson)
	}

	// 8. 检查身份证背面信息（有效期）
	if len(app.IDCardBackOcr) == 0 {
		return false, "身份证背面信息未识别，请重新上传身份证背面照片"
	}

	var idCardBackOCR MerchantIDCardOCRData
	if err := json.Unmarshal(app.IDCardBackOcr, &idCardBackOCR); err != nil {
		return false, "身份证背面信息解析失败，请重新上传"
	}

	// 检查身份证有效期
	if !isIDCardValidPeriodValid(idCardBackOCR.ValidDate) {
		return false, "法人身份证已过期"
	}

	return true, ""
}

func normalizeCompanyName(name string) string {
	name = strings.TrimSpace(name)
	// 去掉常见空白
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	// 统一括号形态
	name = strings.ReplaceAll(name, "（", "(")
	name = strings.ReplaceAll(name, "）", ")")
	return name
}

// isChineseDateAtLeastDaysAfterNow 判断形如 2025年12月31日/长期 的日期是否至少晚于 now + days。
// 长期/永久 视为满足。
func isChineseDateAtLeastDaysAfterNow(dateStr string, days int) bool {
	if dateStr == "" {
		return false
	}
	if strings.Contains(dateStr, "长期") || strings.Contains(dateStr, "永久") {
		return true
	}
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	match := dateRegex.FindStringSubmatch(dateStr)
	if len(match) < 4 {
		return false
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
	parsed, err := time.Parse("2006-01-02", year+"-"+month+"-"+day)
	if err != nil {
		return false
	}
	threshold := time.Now().AddDate(0, 0, days)
	return parsed.After(threshold)
}

// isValidPeriodValid 检查营业期限是否有效
// 格式：长期、长期有效、2020年01月01日至2040年01月01日、2040年01月01日
func isValidPeriodValid(validPeriod string) bool {
	if validPeriod == "" {
		return false
	}

	// 长期有效
	if strings.Contains(validPeriod, "长期") || strings.Contains(validPeriod, "永久") {
		return true
	}

	// 尝试解析日期
	// 格式1: 至2040年01月01日
	// 格式2: 2020年01月01日至2040年01月01日
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	matches := dateRegex.FindAllStringSubmatch(validPeriod, -1)
	if len(matches) == 0 {
		return false
	}

	// 取最后一个日期作为有效期止
	lastMatch := matches[len(matches)-1]
	if len(lastMatch) < 4 {
		return false
	}

	year := lastMatch[1]
	month := lastMatch[2]
	day := lastMatch[3]

	// 补零
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}

	// 解析日期
	dateStr := year + "-" + month + "-" + day
	expDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	return expDate.After(time.Now())
}

// isCateringBusiness 检查是否为餐饮相关经营
func isCateringBusiness(enterpriseName, businessScope string) bool {
	keywords := []string{
		"餐饮", "餐厅", "饭店", "饭馆", "酒楼", "酒家",
		"快餐", "小吃", "面馆", "面店", "粉店",
		"火锅", "烧烤", "串串", "麻辣烫",
		"奶茶", "茶饮", "咖啡", "甜品",
		"食品", "食堂", "厨房", "外卖",
		"菜", "料理", "美食",
	}

	text := enterpriseName + " " + businessScope
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// isAddressMatch 检查营业执照地址与商户地址是否匹配
// 逻辑：
// 1. 首先比较省市区县是否一致
// 2. 然后比较详细地址（路名+门牌号）
// 3. 特殊处理：如果营业执照地址使用模糊位置描述（如"中段东侧"），只要求路名相同即可
func isAddressMatch(licenseAddr, businessAddr string) bool {
	if licenseAddr == "" || businessAddr == "" {
		return false
	}

	// 解析地址，提取各级行政区划和详细地址
	parsed1 := parseChineseAddress(licenseAddr)
	parsed2 := parseChineseAddress(businessAddr)

	// 如果能解析出省市区，则比较是否一致
	if parsed1.Province != "" && parsed2.Province != "" && parsed1.Province != parsed2.Province {
		return false
	}
	if parsed1.City != "" && parsed2.City != "" && parsed1.City != parsed2.City {
		return false
	}
	if parsed1.District != "" && parsed2.District != "" && parsed1.District != parsed2.District {
		return false
	}

	// 比较详细地址部分（街道、路、门牌号等）
	detail1 := parsed1.Detail
	detail2 := parsed2.Detail

	if detail1 == "" || detail2 == "" {
		// 如果没有详细地址，降级到简单匹配
		return simpleAddressMatch(licenseAddr, businessAddr)
	}

	// 详细地址相似度匹配
	// 1. 完全相同
	if detail1 == detail2 {
		return true
	}

	// 2. 一个包含另一个（允许一方更详细）
	if strings.Contains(detail1, detail2) || strings.Contains(detail2, detail1) {
		return true
	}

	// 3. 提取路名和门牌号/位置描述进行比较
	road1, number1, isFuzzy1 := extractRoadAndNumberWithFuzzy(detail1)
	road2, number2, isFuzzy2 := extractRoadAndNumberWithFuzzy(detail2)

	// 路名必须相同（这是核心匹配条件）
	if road1 == "" || road2 == "" || road1 != road2 {
		// 路名不同或无法提取，降级到简单匹配
		return simpleAddressMatch(licenseAddr, businessAddr)
	}

	// 路名相同的情况下：
	// 4a. 如果任一方使用模糊位置描述（如"中段东侧"、"路口"），视为匹配
	if isFuzzy1 || isFuzzy2 {
		return true
	}

	// 4b. 门牌号相同
	if number1 == number2 {
		return true
	}

	// 4c. 门牌号标准化后相同，如"100号"和"100"
	if normalizeNumber(number1) == normalizeNumber(number2) {
		return true
	}

	// 4d. 一方有门牌号，另一方为空（可能是一方写得不完整）
	if (number1 != "" && number2 == "") || (number1 == "" && number2 != "") {
		return true
	}

	return false
}

// parsedAddress 解析后的地址结构
type parsedAddress struct {
	Province string // 省/直辖市
	City     string // 市
	District string // 区/县
	Street   string // 街道/镇
	Detail   string // 详细地址（路、门牌号等）
}

// parseChineseAddress 解析中国地址
func parseChineseAddress(addr string) parsedAddress {
	result := parsedAddress{}
	remaining := addr

	// 提取省份（含直辖市）
	provinceRegex := regexp.MustCompile(`^(北京|天津|上海|重庆|河北|山西|辽宁|吉林|黑龙江|江苏|浙江|安徽|福建|江西|山东|河南|湖北|湖南|广东|海南|四川|贵州|云南|陕西|甘肃|青海|台湾|内蒙古|广西|西藏|宁夏|新疆|香港|澳门)(省|市|自治区|特别行政区)?`)
	if match := provinceRegex.FindString(remaining); match != "" {
		result.Province = match
		remaining = strings.TrimPrefix(remaining, match)
	}

	// 提取市（使用更宽松的匹配，只排除行政区划结尾字）
	cityRegex := regexp.MustCompile(`^(.+?)(市|地区|自治州|盟)`)
	if match := cityRegex.FindStringSubmatch(remaining); len(match) > 0 {
		result.City = match[0]
		remaining = strings.TrimPrefix(remaining, match[0])
	}

	// 提取区/县（使用更精确的匹配）
	districtRegex := regexp.MustCompile(`^(.+?)(区|县|旗)`)
	if match := districtRegex.FindStringSubmatch(remaining); len(match) > 0 {
		result.District = match[0]
		remaining = strings.TrimPrefix(remaining, match[0])
	}

	// 提取街道/镇
	streetRegex := regexp.MustCompile(`^([^省市区县路号]+?)(街道|镇|乡|村)`)
	if match := streetRegex.FindStringSubmatch(remaining); len(match) > 0 {
		result.Street = match[0]
		remaining = strings.TrimPrefix(remaining, match[0])
	}

	// 剩余部分为详细地址
	detail := strings.TrimSpace(remaining)

	// 清理Detail开头的"城"字（如"县城向阳街" -> "向阳街"）
	// 这是因为有些地址写成"XX县城XX路"的格式
	if strings.HasPrefix(detail, "城") {
		detail = strings.TrimPrefix(detail, "城")
	}

	result.Detail = detail

	return result
}

// extractRoadAndNumberWithFuzzy 从详细地址中提取路名和门牌号，同时识别模糊位置描述
// 返回值：road（路名）, number（门牌号或位置描述）, isFuzzy（是否为模糊位置描述）
func extractRoadAndNumberWithFuzzy(detail string) (road, number string, isFuzzy bool) {
	// 匹配 "XX路/街/巷/弄 + 门牌号或位置描述"
	// 路名后可能跟随：数字门牌号（如"100号"）或模糊描述（如"中段东侧"）
	roadRegex := regexp.MustCompile(`([^0-9]+(?:路|街|道|巷|弄|大街|大道|胡同))(.+)?`)
	if match := roadRegex.FindStringSubmatch(detail); len(match) > 1 {
		road = match[1]
		if len(match) > 2 && match[2] != "" {
			number = strings.TrimSpace(match[2])
			// 检查是否为模糊位置描述
			isFuzzy = isFuzzyLocation(number)
		}
	}
	return
}

// isFuzzyLocation 判断是否为模糊位置描述（非精确门牌号）
func isFuzzyLocation(location string) bool {
	if location == "" {
		return false
	}

	// 模糊位置描述的关键词
	fuzzyKeywords := []string{
		"中段", "东段", "西段", "南段", "北段", // 路段描述
		"东侧", "西侧", "南侧", "北侧", // 方位描述
		"东边", "西边", "南边", "北边",
		"路口", "交叉口", "十字路口", // 路口
		"对面", "旁边", "附近", "斜对面",
		"内", "里", // 区域内
	}

	for _, keyword := range fuzzyKeywords {
		if strings.Contains(location, keyword) {
			return true
		}
	}

	// 检查是否不包含数字（没有精确门牌号）
	numRegex := regexp.MustCompile(`\d+`)
	if !numRegex.MatchString(location) {
		return true
	}

	return false
}

// normalizeNumber 标准化门牌号（去除"号"字等）
func normalizeNumber(num string) string {
	num = strings.TrimSuffix(num, "号")
	num = strings.TrimSpace(num)
	return num
}

// simpleAddressMatch 简单地址匹配（降级方案）
// 注意：此函数已加强安全性，必须路名相同才能匹配，防止仅靠数字相同就通过
func simpleAddressMatch(addr1, addr2 string) bool {
	// 1. 首先提取路名进行比较
	keywords1 := extractAddressKeywords(addr1)
	keywords2 := extractAddressKeywords(addr2)

	// 检查是否有共同的路名关键词
	hasCommonRoad := false
	for _, kw1 := range keywords1 {
		if len(kw1) < 2 {
			continue
		}
		for _, kw2 := range keywords2 {
			if kw1 == kw2 {
				hasCommonRoad = true
				break
			}
		}
		if hasCommonRoad {
			break
		}
	}

	// 如果没有共同的路名，直接返回失败
	if !hasCommonRoad {
		return false
	}

	// 2. 有共同路名的情况下，再检查是否有共同的门牌号特征
	numRegex := regexp.MustCompile(`\d+`)
	nums1 := numRegex.FindAllString(addr1, -1)
	nums2 := numRegex.FindAllString(addr2, -1)

	// 如果有共同的数字特征（至少3位数字，避免年份等干扰）
	for _, n1 := range nums1 {
		for _, n2 := range nums2 {
			if n1 == n2 && len(n1) >= 1 { // 至少1位数字
				return true
			}
		}
	}

	// 3. 有共同路名但没有共同数字，检查是否存在模糊位置描述
	// 如果任一地址使用模糊描述，视为匹配
	if containsFuzzyLocation(addr1) || containsFuzzyLocation(addr2) {
		return true
	}

	return false
}

// containsFuzzyLocation 检查地址中是否包含模糊位置描述
func containsFuzzyLocation(addr string) bool {
	fuzzyKeywords := []string{
		"中段", "东段", "西段", "南段", "北段",
		"东侧", "西侧", "南侧", "北侧",
		"路口", "交叉口",
		"对面", "旁边", "附近",
	}

	for _, keyword := range fuzzyKeywords {
		if strings.Contains(addr, keyword) {
			return true
		}
	}
	return false
}

// extractAddressKeywords 提取地址关键词
func extractAddressKeywords(addr string) []string {
	var keywords []string
	// 提取路名
	roadRegex := regexp.MustCompile(`[^省市区县镇乡村街道]+(?:路|街|道|巷|弄|大街|大道|胡同)`)
	roads := roadRegex.FindAllString(addr, -1)
	keywords = append(keywords, roads...)
	return keywords
}

// isFoodPermitValid 检查食品经营许可证是否有效
func isFoodPermitValid(validTo string) bool {
	if validTo == "" {
		return false
	}

	// 长期有效
	if strings.Contains(validTo, "长期") || strings.Contains(validTo, "永久") {
		return true
	}

	// 解析日期
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	match := dateRegex.FindStringSubmatch(validTo)
	if len(match) < 4 {
		return false
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

	dateStr := year + "-" + month + "-" + day
	expDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	return expDate.After(time.Now())
}

// isIDCardValidPeriodValid 检查身份证有效期是否有效
// 格式：2020.01.01-2030.01.01 或 2020.01.01-长期
func isIDCardValidPeriodValid(validDate string) bool {
	if validDate == "" {
		return false
	}

	// 长期有效
	if strings.Contains(validDate, "长期") || strings.Contains(validDate, "永久") {
		return true
	}

	// 解析有效期止日期
	// 微信OCR可能返回的格式：
	// - 20200101-20300101 (纯数字)
	// - 2020.01.01-2030.01.01 (点分隔)
	// - 2020-01-01-2030-01-01 (横线分隔，但这会被错误分割)
	// - 2020年01月01日-2030年01月01日 (中文格式)

	// 先尝试找到最后的日期部分
	var endDate string

	// 方法1：查找"至"或最后一个完整日期
	if idx := strings.LastIndex(validDate, "-"); idx > 4 {
		// 确保不是日期中的横线（检查后面是否还有足够字符构成日期）
		after := validDate[idx+1:]
		if len(after) >= 6 { // 至少 YYMMDD
			endDate = strings.TrimSpace(after)
		}
	}

	// 如果没找到，尝试用"至"分割
	if endDate == "" && strings.Contains(validDate, "至") {
		parts := strings.Split(validDate, "至")
		if len(parts) >= 2 {
			endDate = strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// 如果还没找到，整个字符串可能就是结束日期
	if endDate == "" {
		endDate = validDate
	}

	// 如果结束日期包含"长期"
	if strings.Contains(endDate, "长期") || strings.Contains(endDate, "永久") {
		return true
	}

	// 尝试解析多种日期格式
	expDate, ok := parseFlexibleDate(endDate)
	if !ok {
		log.Warn().Str("valid_date", validDate).Str("end_date", endDate).Msg("无法解析身份证有效期")
		return false
	}

	return expDate.After(time.Now())
}

// parseFlexibleDate 灵活解析多种日期格式
func parseFlexibleDate(dateStr string) (time.Time, bool) {
	// 移除空白和常见无关字符
	dateStr = strings.TrimSpace(dateStr)
	dateStr = strings.ReplaceAll(dateStr, " ", "")

	// 尝试提取纯数字（年月日）
	var year, month, day string

	// 格式1: 中文格式 2030年01月01日
	if strings.Contains(dateStr, "年") && strings.Contains(dateStr, "月") && strings.Contains(dateStr, "日") {
		re := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
		if matches := re.FindStringSubmatch(dateStr); len(matches) == 4 {
			year, month, day = matches[1], matches[2], matches[3]
		}
	}

	// 格式2: 点分隔 2030.01.01
	if year == "" && strings.Count(dateStr, ".") >= 2 {
		parts := strings.Split(dateStr, ".")
		if len(parts) >= 3 {
			year, month, day = parts[0], parts[1], parts[2]
		}
	}

	// 格式3: 纯数字 20300101
	if year == "" && len(dateStr) >= 8 {
		// 尝试匹配 YYYYMMDD
		re := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})`)
		if matches := re.FindStringSubmatch(dateStr); len(matches) == 4 {
			year, month, day = matches[1], matches[2], matches[3]
		}
	}

	// 格式4: 横线分隔 2030-01-01（但要注意不要误匹配有效期范围分隔符）
	if year == "" && strings.Count(dateStr, "-") == 2 {
		parts := strings.Split(dateStr, "-")
		if len(parts) == 3 && len(parts[0]) == 4 {
			year, month, day = parts[0], parts[1], parts[2]
		}
	}

	if year == "" || month == "" || day == "" {
		return time.Time{}, false
	}

	// 构建日期字符串
	dateFormatted := fmt.Sprintf("%s-%02s-%02s", year, month, day)
	t, err := time.Parse("2006-01-02", dateFormatted)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

// ==================== 重置申请 ====================

// resetMerchantApplication godoc
// @Summary 重置被拒绝的商户申请
// @Description 将被拒绝的申请重置为草稿状态，可重新编辑提交
// @Tags 商户申请
// @Accept json
// @Produce json
// @Success 200 {object} merchantApplicationDraftResponse "重置成功"
// @Failure 400 {object} ErrorResponse "非拒绝状态无法重置"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/reset [post]
// @Security BearerAuth
func (server *Server) resetMerchantApplication(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取申请
	app, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("申请不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "rejected" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只能重置被拒绝的申请")))
		return
	}

	resetResult, err := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
		ApplicationID: app.ID,
		UserID:        authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationDraftResponse(resetResult.Application))
}
