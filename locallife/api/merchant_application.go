package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

const merchantApplicationAddressValidationRadiusMeters = 1000

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
	Status              string        `json:"status,omitempty"`           // pending/processing/done/failed
	Error               string        `json:"error,omitempty"`            // failure reason (if any)
	ErrorCode           string        `json:"error_code,omitempty"`       // machine-readable failure code
	AlertEmittedAt      string        `json:"alert_emitted_at,omitempty"` // 平台告警发送时间
	Readiness           *OCRReadiness `json:"readiness,omitempty"`
	QueuedAt            string        `json:"queued_at,omitempty"`            // task enqueued time
	StartedAt           string        `json:"started_at,omitempty"`           // task started processing time
	OCRJobID            *int64        `json:"ocr_job_id,omitempty"`           // 统一 OCR 任务 ID
	RegNum              string        `json:"reg_num,omitempty"`              // 注册号
	EnterpriseName      string        `json:"enterprise_name,omitempty"`      // 企业名称
	LegalRepresentative string        `json:"legal_representative,omitempty"` // 法定代表人
	TypeOfEnterprise    string        `json:"type_of_enterprise,omitempty"`   // 类型
	Address             string        `json:"address,omitempty"`              // 地址
	BusinessScope       string        `json:"business_scope,omitempty"`       // 经营范围
	RegisteredCapital   string        `json:"registered_capital,omitempty"`   // 注册资本
	ValidPeriod         string        `json:"valid_period"`                   // 营业期限（如：2020年01月01日至2040年01月01日 或 长期）
	CreditCode          string        `json:"credit_code,omitempty"`          // 统一社会信用代码
	OCRAt               string        `json:"ocr_at,omitempty"`               // OCR识别时间
}

// FoodPermitOCRData 食品经营许可证OCR识别数据（通用印刷体识别后解析）
type FoodPermitOCRData struct {
	Status         string        `json:"status,omitempty"`           // pending/processing/done/failed
	Error          string        `json:"error,omitempty"`            // failure reason (if any)
	ErrorCode      string        `json:"error_code,omitempty"`       // machine-readable failure code
	AlertEmittedAt string        `json:"alert_emitted_at,omitempty"` // 平台告警发送时间
	Readiness      *OCRReadiness `json:"readiness,omitempty"`
	QueuedAt       string        `json:"queued_at,omitempty"`     // task enqueued time
	StartedAt      string        `json:"started_at,omitempty"`    // task started processing time
	OCRJobID       *int64        `json:"ocr_job_id,omitempty"`    // 统一 OCR 任务 ID
	RawText        string        `json:"raw_text,omitempty"`      // 原始OCR文本
	PermitNo       string        `json:"permit_no,omitempty"`     // 许可证编号
	CompanyName    string        `json:"company_name,omitempty"`  // 企业名称
	OperatorName   string        `json:"operator_name,omitempty"` // 经营者/法定代表人姓名
	ValidFrom      string        `json:"valid_from,omitempty"`    // 有效期起
	ValidTo        string        `json:"valid_to,omitempty"`      // 有效期止（如：2025年12月31日 或 长期）
	OCRAt          string        `json:"ocr_at,omitempty"`        // OCR识别时间
}

// MerchantIDCardOCRData 商户法人身份证OCR识别数据
type MerchantIDCardOCRData struct {
	Status         string        `json:"status,omitempty"`           // pending/processing/done/failed
	Error          string        `json:"error,omitempty"`            // failure reason (if any)
	ErrorCode      string        `json:"error_code,omitempty"`       // machine-readable failure code
	AlertEmittedAt string        `json:"alert_emitted_at,omitempty"` // 平台告警发送时间
	Readiness      *OCRReadiness `json:"readiness,omitempty"`
	QueuedAt       string        `json:"queued_at,omitempty"`  // task enqueued time
	StartedAt      string        `json:"started_at,omitempty"` // task started processing time
	OCRJobID       *int64        `json:"ocr_job_id,omitempty"` // 统一 OCR 任务 ID
	Name           string        `json:"name,omitempty"`       // 姓名
	IDNumber       string        `json:"id_number,omitempty"`  // 身份证号
	Gender         string        `json:"gender,omitempty"`     // 性别
	Nation         string        `json:"nation,omitempty"`     // 民族
	Address        string        `json:"address,omitempty"`    // 地址
	ValidDate      string        `json:"valid_date,omitempty"` // 有效期（背面）
	OCRAt          string        `json:"ocr_at,omitempty"`     // OCR识别时间
}

// merchantApplicationDraftResponse 商户申请草稿响应
type merchantApplicationDraftResponse struct {
	ID                          int64                             `json:"id"`
	UserID                      int64                             `json:"user_id"`
	MerchantName                string                            `json:"merchant_name"`
	ContactPhone                string                            `json:"contact_phone"`
	BusinessAddress             string                            `json:"business_address"`
	Longitude                   *string                           `json:"longitude,omitempty"`
	Latitude                    *string                           `json:"latitude,omitempty"`
	RegionID                    *int64                            `json:"region_id,omitempty"`
	BusinessLicenseMediaAssetID *int64                            `json:"business_license_media_asset_id,omitempty"`
	BusinessLicenseURL          *string                           `json:"business_license_url,omitempty"`
	BusinessLicenseNumber       string                            `json:"business_license_number"`
	BusinessScope               *string                           `json:"business_scope,omitempty"`
	BusinessLicenseOCR          *BusinessLicenseOCRData           `json:"business_license_ocr,omitempty"`
	FoodPermitMediaAssetID      *int64                            `json:"food_permit_media_asset_id,omitempty"`
	FoodPermitURL               *string                           `json:"food_permit_url,omitempty"`
	FoodPermitOCR               *FoodPermitOCRData                `json:"food_permit_ocr,omitempty"`
	LegalPersonName             string                            `json:"legal_person_name"`
	LegalPersonIDNumber         string                            `json:"legal_person_id_number"`
	IDCardFrontMediaAssetID     *int64                            `json:"id_card_front_media_asset_id,omitempty"`
	IDCardBackMediaAssetID      *int64                            `json:"id_card_back_media_asset_id,omitempty"`
	IDCardFrontOCR              *MerchantIDCardOCRData            `json:"id_card_front_ocr,omitempty"`
	IDCardBackOCR               *MerchantIDCardOCRData            `json:"id_card_back_ocr,omitempty"`
	StorefrontImages            []string                          `json:"storefront_images,omitempty"`
	EnvironmentImages           []string                          `json:"environment_images,omitempty"`
	ReviewSummary               *onboardingReviewSummaryResponse  `json:"review_summary,omitempty"`
	ActiveCredentials           []activeCredentialSummaryResponse `json:"active_credentials,omitempty"`
	Status                      string                            `json:"status"`
	RejectReason                *string                           `json:"reject_reason,omitempty"`
	CreatedAt                   time.Time                         `json:"created_at"`
	UpdatedAt                   time.Time                         `json:"updated_at"`
}

func (server *Server) defaultOCRProviderName(documentType ocr.DocumentType) ocr.ProviderName {
	if server.config.AliyunOCREnabled {
		return ocr.ProviderNameAliyun
	}
	switch documentType {
	case ocr.DocumentTypeBusinessLicense, ocr.DocumentTypeFoodPermit, ocr.DocumentTypeIDCard:
		return ocr.ProviderNameWechat
	default:
		return ocr.ProviderNameAliyun
	}
}

// checkApplicationEditable 检查申请是否可编辑
// 返回值: (是否可编辑, 是否需要重置为draft, 错误信息)
func checkApplicationEditable(status string) (editable bool, needReset bool, errMsg string) {
	switch status {
	case "draft":
		return true, false, ""
	case "rejected", "approved", "submitted":
		// 被拒绝、已通过或历史遗留 submitted 的申请允许编辑，但需要先重置为草稿状态。
		return true, true, ""
	default:
		return false, false, "申请状态异常"
	}
}

func (server *Server) applicantVisiblePublicMediaURL(ctx context.Context, assetID *int64, userID int64) *string {
	if assetID == nil || server.store == nil || server.mediaResolver == nil {
		return nil
	}

	asset, err := server.store.GetMediaAssetByID(ctx, *assetID)
	if err != nil || asset.Visibility != string(media.VisibilityPublic) {
		return nil
	}
	if asset.ModerationStatus != "approved" && asset.UploadedBy != userID {
		return nil
	}

	url := server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantOriginal)
	return &url
}

func (server *Server) newMerchantApplicationDraftResponse(ctx context.Context, app db.MerchantApplication) (merchantApplicationDraftResponse, error) {
	resp := merchantApplicationDraftResponse{
		ID:                          app.ID,
		UserID:                      app.UserID,
		MerchantName:                app.MerchantName,
		ContactPhone:                app.ContactPhone,
		BusinessAddress:             app.BusinessAddress,
		BusinessLicenseMediaAssetID: int64PtrFromPgInt8(app.BusinessLicenseMediaAssetID),
		BusinessLicenseNumber:       app.BusinessLicenseNumber,
		LegalPersonName:             app.LegalPersonName,
		LegalPersonIDNumber:         app.LegalPersonIDNumber,
		IDCardFrontMediaAssetID:     int64PtrFromPgInt8(app.IDCardFrontMediaAssetID),
		IDCardBackMediaAssetID:      int64PtrFromPgInt8(app.IDCardBackMediaAssetID),
		Status:                      app.Status,
		CreatedAt:                   app.CreatedAt,
		UpdatedAt:                   app.UpdatedAt,
		ReviewSummary:               decodeOnboardingReviewSummary(app.ReviewSummary),
		ActiveCredentials:           server.loadMerchantActiveCredentialSummaries(ctx, app.UserID),
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

	// 食品许可证媒体资产ID
	resp.FoodPermitMediaAssetID = int64PtrFromPgInt8(app.FoodPermitMediaAssetID)
	resp.BusinessLicenseURL = server.applicantVisiblePublicMediaURL(ctx, resp.BusinessLicenseMediaAssetID, app.UserID)
	resp.FoodPermitURL = server.applicantVisiblePublicMediaURL(ctx, resp.FoodPermitMediaAssetID, app.UserID)

	// 拒绝原因
	if app.RejectReason.Valid {
		resp.RejectReason = &app.RejectReason.String
	}

	// 解析OCR数据
	if len(app.BusinessLicenseOcr) > 0 {
		var ocr BusinessLicenseOCRData
		if err := decodeMerchantApplicationJSONField(app.ID, "business_license_ocr", app.BusinessLicenseOcr, &ocr); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		if ocr.Status == "" {
			ocr.Status = "done"
		}
		resp.BusinessLicenseOCR = &ocr
	}
	if len(app.FoodPermitOcr) > 0 {
		var ocr FoodPermitOCRData
		if err := decodeMerchantApplicationJSONField(app.ID, "food_permit_ocr", app.FoodPermitOcr, &ocr); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		if ocr.Status == "" {
			ocr.Status = "done"
		}
		resp.FoodPermitOCR = &ocr
	}
	if len(app.IDCardFrontOcr) > 0 {
		var ocr MerchantIDCardOCRData
		if err := decodeMerchantApplicationJSONField(app.ID, "id_card_front_ocr", app.IDCardFrontOcr, &ocr); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		if ocr.Status == "" {
			ocr.Status = "done"
		}
		resp.IDCardFrontOCR = &ocr
	}
	if len(app.IDCardBackOcr) > 0 {
		var ocr MerchantIDCardOCRData
		if err := decodeMerchantApplicationJSONField(app.ID, "id_card_back_ocr", app.IDCardBackOcr, &ocr); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		if ocr.Status == "" {
			ocr.Status = "done"
		}
		resp.IDCardBackOCR = &ocr
	}

	// 解析门头照和环境照（jsonb数组）
	if len(app.StorefrontImages) > 0 {
		var images []string
		if err := decodeMerchantApplicationJSONField(app.ID, "storefront_images", app.StorefrontImages, &images); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.StorefrontImages = images
	}
	if len(app.EnvironmentImages) > 0 {
		var images []string
		if err := decodeMerchantApplicationJSONField(app.ID, "environment_images", app.EnvironmentImages, &images); err != nil {
			return merchantApplicationDraftResponse{}, err
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.EnvironmentImages = images
	}

	return resp, nil
}

func decodeMerchantApplicationJSONField(applicationID int64, field string, payload []byte, target interface{}) error {
	if len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode merchant application %d %s: %w", applicationID, field, err)
	}
	return nil
}

func (server *Server) writeMerchantApplicationDraftResponse(ctx *gin.Context, status int, app db.MerchantApplication) bool {
	resp, err := server.newMerchantApplicationDraftResponse(ctx.Request.Context(), app)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return false
	}
	ctx.JSON(status, resp)
	return true
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
		if isNotFoundError(err) {
			// 创建新草稿
			newApp, err := server.store.CreateMerchantApplicationDraft(ctx, authPayload.UserID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			server.writeMerchantApplicationDraftResponse(ctx, http.StatusCreated, newApp)
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status == "submitted" {
		resetResult, resetErr := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        authPayload.UserID,
		})
		if resetErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, resetErr))
			return
		}
		app = resetResult.Application
	}

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, app)
}

// ==================== 更新基础信息 ====================

type updateMerchantBasicInfoRequest struct {
	MerchantName          string  `json:"merchant_name" binding:"omitempty,min=2,max=50"`
	ContactPhone          string  `json:"contact_phone" binding:"omitempty,len=11"`
	BusinessAddress       string  `json:"business_address" binding:"omitempty,min=5,max=200"`
	BusinessLicenseNumber string  `json:"business_license_number" binding:"omitempty,min=8,max=32"`
	BusinessScope         string  `json:"business_scope" binding:"omitempty,max=500"`
	LegalPersonName       string  `json:"legal_person_name" binding:"omitempty,min=2,max=50"`
	LegalPersonIDNumber   string  `json:"legal_person_id_number" binding:"omitempty,min=15,max=32"`
	Longitude             *string `json:"longitude" binding:"omitempty"`
	Latitude              *string `json:"latitude" binding:"omitempty"`
	RegionID              *int64  `json:"region_id" binding:"omitempty"`
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
	log.Info().Str("request_id", requestID).Msg("update merchant basic info received")

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
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
	if req.BusinessLicenseNumber != "" {
		arg.BusinessLicenseNumber = pgtype.Text{String: req.BusinessLicenseNumber, Valid: true}
	}
	if req.BusinessScope != "" {
		arg.BusinessScope = pgtype.Text{String: req.BusinessScope, Valid: true}
	}
	if req.LegalPersonName != "" {
		arg.LegalPersonName = pgtype.Text{String: req.LegalPersonName, Valid: true}
	}
	if req.LegalPersonIDNumber != "" {
		arg.LegalPersonIDNumber = pgtype.Text{String: req.LegalPersonIDNumber, Valid: true}
	}
	if req.Longitude != nil {
		lon, err := parseNumericString(*req.Longitude)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidLongitudeFormat))
			return
		}
		arg.Longitude = lon
	}
	if req.Latitude != nil {
		lat, err := parseNumericString(*req.Latitude)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidLatitudeFormat))
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

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, updatedApp)
}

// ==================== 更新门头照和环境照 ====================

type updateMerchantImagesRequest struct {
	StorefrontImages  []string `json:"storefront_images"`  // 门头照URL数组，最多3张
	EnvironmentImages []string `json:"environment_images"` // 环境照URL数组，最多5张
}

type merchantApplicationDocumentType string

const (
	merchantApplicationDocumentBusinessLicense merchantApplicationDocumentType = "business_license"
	merchantApplicationDocumentFoodPermit      merchantApplicationDocumentType = "food_permit"
	merchantApplicationDocumentIDCardFront     merchantApplicationDocumentType = "id_card_front"
	merchantApplicationDocumentIDCardBack      merchantApplicationDocumentType = "id_card_back"
)

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
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrTooManyStorefrontPhotos))
		return
	}
	if len(req.EnvironmentImages) > 5 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrTooManyAmbientPhotos))
		return
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
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

	// 记录更新前的旧图片列表，用于稍后删除被移除的图片
	var oldStorefront, oldEnvironment []string
	if req.StorefrontImages != nil {
		oldStorefront, err = decodeStoredMerchantApplicationImageList(app.ID, "storefront_images", app.StorefrontImages)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	if req.EnvironmentImages != nil {
		oldEnvironment, err = decodeStoredMerchantApplicationImageList(app.ID, "environment_images", app.EnvironmentImages)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 注意：用 != nil 而非 len > 0，使得前端传空数组 [] 时能正确清空图片
	if req.StorefrontImages != nil {
		for i, img := range req.StorefrontImages {
			req.StorefrontImages[i] = normalizeImageURLForStorage(img)
		}
		jsonData, _ := json.Marshal(req.StorefrontImages)
		arg.StorefrontImages = jsonData
	}
	if req.EnvironmentImages != nil {
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

	// 删除被移除的旧图片文件
	if req.StorefrontImages != nil {
		for _, old := range oldStorefront {
			found := false
			for _, cur := range req.StorefrontImages {
				if old == cur {
					found = true
					break
				}
			}
			if !found {
				server.deleteStoredImageAsync(old)
			}
		}
	}
	if req.EnvironmentImages != nil {
		for _, old := range oldEnvironment {
			found := false
			for _, cur := range req.EnvironmentImages {
				if old == cur {
					found = true
					break
				}
			}
			if !found {
				server.deleteStoredImageAsync(old)
			}
		}
	}

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, updatedApp)
}

func decodeStoredMerchantApplicationImageList(applicationID int64, field string, payload []byte) ([]string, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var images []string
	if err := json.Unmarshal(payload, &images); err != nil {
		return nil, fmt.Errorf("decode merchant application %d %s: %w", applicationID, field, err)
	}
	return images, nil
}

// deleteMerchantApplicationDocument godoc
// @Summary 删除商户申请证照
// @Description 删除草稿中的单个证照绑定，并清空对应 OCR 结果。支持在 approved 或 rejected 状态下自动重置为 draft 后删除。
// @Tags 商户申请
// @Produce json
// @Param document_type path string true "证照类型: business_license|food_permit|id_card_front|id_card_back"
// @Success 200 {object} merchantApplicationDraftResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误或非可编辑状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/documents/{document_type} [delete]
// @Security BearerAuth
func (server *Server) deleteMerchantApplicationDocument(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	documentType := merchantApplicationDocumentType(strings.TrimSpace(ctx.Param("document_type")))

	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(errMsg)))
		return
	}
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

	var updatedApp db.MerchantApplication
	var assetID int64

	switch documentType {
	case merchantApplicationDocumentBusinessLicense:
		if app.BusinessLicenseMediaAssetID.Valid {
			assetID = app.BusinessLicenseMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearMerchantApplicationBusinessLicense(ctx, app.ID)
	case merchantApplicationDocumentFoodPermit:
		if app.FoodPermitMediaAssetID.Valid {
			assetID = app.FoodPermitMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearMerchantApplicationFoodPermit(ctx, app.ID)
	case merchantApplicationDocumentIDCardFront:
		if app.IDCardFrontMediaAssetID.Valid {
			assetID = app.IDCardFrontMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearMerchantApplicationIDCardFront(ctx, app.ID)
	case merchantApplicationDocumentIDCardBack:
		if app.IDCardBackMediaAssetID.Valid {
			assetID = app.IDCardBackMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearMerchantApplicationIDCardBack(ctx, app.ID)
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid document type")))
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if assetID > 0 {
		if err := server.mediaRegistry.SoftDelete(ctx, assetID, authPayload.UserID); err != nil {
			log.Warn().Err(err).Int64("asset_id", assetID).Str("document_type", string(documentType)).Msg("delete merchant application document: soft delete media failed")
		}
	}

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, updatedApp)
}

// ==================== 上传营业执照并OCR识别 ====================

// parseFoodPermitOCRText 从OCR文本中解析食品经营许可证信息
// ==================== 上传身份证并OCR识别 ====================
// ==================== 提交申请 ====================

func (server *Server) backfillMerchantSubmissionDefaults(ctx context.Context, app db.MerchantApplication) (db.MerchantApplication, error) {
	updateArg := db.UpdateMerchantApplicationBasicInfoParams{ID: app.ID}
	shouldPersist := false

	if strings.TrimSpace(app.MerchantName) == "" && len(app.BusinessLicenseOcr) > 0 {
		var licenseOCR BusinessLicenseOCRData
		if err := json.Unmarshal(app.BusinessLicenseOcr, &licenseOCR); err != nil {
			log.Warn().Err(err).Int64("application_id", app.ID).Msg("submit merchant application: decode business license OCR failed")
		} else if merchantName := strings.TrimSpace(licenseOCR.EnterpriseName); merchantName != "" {
			app.MerchantName = merchantName
			if app.Status == "draft" {
				updateArg.MerchantName = pgtype.Text{String: merchantName, Valid: true}
				shouldPersist = true
			}
		}
	}

	if strings.TrimSpace(app.ContactPhone) == "" {
		user, err := server.store.GetUser(ctx, app.UserID)
		if err != nil {
			log.Warn().Err(err).Int64("application_id", app.ID).Int64("user_id", app.UserID).Msg("submit merchant application: load user phone fallback failed")
		} else if user.Phone.Valid {
			contactPhone := strings.TrimSpace(user.Phone.String)
			if contactPhone != "" {
				app.ContactPhone = contactPhone
				if app.Status == "draft" {
					updateArg.ContactPhone = pgtype.Text{String: contactPhone, Valid: true}
					shouldPersist = true
				}
			}
		}
	}

	if !shouldPersist || app.Status != "draft" {
		return app, nil
	}

	updatedApp, err := server.store.UpdateMerchantApplicationBasicInfo(ctx, updateArg)
	if err != nil {
		return app, err
	}

	return updatedApp, nil
}

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
	consentReq, err := parseAgreementConsentRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	requestID := ctx.GetString("request_id")
	if requestID == "" {
		requestID = ctx.GetHeader("X-Request-ID")
	}

	// 获取申请
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 允许提交的状态：draft, rejected, approved, submitted (用于重试)
	if app.Status != "draft" && app.Status != "rejected" && app.Status != "approved" && app.Status != "submitted" {
		log.Warn().Str("request_id", requestID).Str("current_status", app.Status).Msg("submit failed: invalid status")
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationInvalidState))
		return
	}

	app, err = server.backfillMerchantSubmissionDefaults(ctx, app)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeAgreementConsentAudit(ctx, authPayload.UserID, "merchant_application_consent_confirmed", "merchant_application", app.ID, consentReq)

	// 检查必填字段
	if err := validateMerchantApplicationRequired(app); err != nil {
		log.Warn().Str("request_id", requestID).Err(err).Msg("submit failed: validation error")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if err := server.checkMerchantApplicationApproval(ctx, app); err != nil {
		server.recordMerchantBlockedReview(ctx, app, authPayload.UserID, err)
		log.Warn().Str("request_id", requestID).Str("reject_reason", err.Error()).Msg("submit blocked: merchant application remains editable")
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

	reviewExecutor := logic.NewMerchantOnboardingReviewService(server.store, server.onboardingReviewService, server.credentialGovernanceService)
	var queuedRun *db.OnboardingReviewRun
	if server.onboardingReviewService != nil && server.taskDistributor != nil {
		run, err := server.onboardingReviewService.CreateMerchantReviewRun(ctx, submittedApp.ID, logic.OnboardingReviewDecision{
			RequestedBy: &authPayload.UserID,
			OCRJobRefs:  merchantApplicationOCRJobRefs(submittedApp),
			Snapshot: map[string]any{
				"application_id":   submittedApp.ID,
				"application_type": "merchant",
				"status":           submittedApp.Status,
				"user_id":          submittedApp.UserID,
				"merchant_name":    submittedApp.MerchantName,
			},
		})
		if err != nil {
			log.Error().Err(err).Int64("application_id", submittedApp.ID).Msg("create merchant onboarding review run failed, fallback to sync review")
		} else {
			queuedRun = &run
			err = server.taskDistributor.DistributeTaskOnboardingReview(ctx, &worker.OnboardingReviewPayload{
				ReviewRunID:     run.ID,
				ApplicationID:   submittedApp.ID,
				ApplicationType: "merchant",
				RequestedBy:     authPayload.UserID,
			})
			if err == nil {
				attachMerchantReviewSummary(&submittedApp, queuedRun)
				server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, submittedApp)
				return
			}
			log.Error().Err(err).Int64("application_id", submittedApp.ID).Int64("review_run_id", run.ID).Msg("enqueue merchant onboarding review failed, fallback to sync review")
		}
	}

	result, err := reviewExecutor.ProcessSubmittedApplication(ctx, submittedApp, authPayload.UserID, onboardingReviewRunID(queuedRun))
	if err != nil {
		log.Error().Err(err).Int64("application_id", submittedApp.ID).Msg("merchant onboarding review failed")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if result.RestoreReleased && result.Merchant != nil {
		server.notifyCredentialGovernanceRestored(ctx, "merchant", result.Merchant.ID, result.Application.ID, result.ReviewRun, result.CredentialEntries)
	}

	if strings.TrimSpace(result.Application.MerchantName) == "" {
		result.Application.MerchantName = submittedApp.MerchantName
	}
	if strings.TrimSpace(result.Application.ContactPhone) == "" {
		result.Application.ContactPhone = submittedApp.ContactPhone
	}

	merchantID := int64(0)
	if result.Merchant != nil {
		merchantID = result.Merchant.ID
	}
	log.Info().
		Int64("application_id", result.Application.ID).
		Int64("merchant_id", merchantID).
		Msg("商户审核通过事务完成")

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, result.Application)
}

// validateMerchantApplicationRequired 验证必填字段
func validateMerchantApplicationRequired(app db.MerchantApplication) error {
	if app.MerchantName == "" {
		return ErrMerchantNameRequired
	}
	if app.BusinessAddress == "" {
		return ErrMerchantAddressRequired
	}
	if !app.Longitude.Valid || !app.Latitude.Valid {
		return ErrMerchantLocationRequired
	}
	if !app.RegionID.Valid {
		return ErrMerchantRegionRequired
	}
	if !app.BusinessLicenseMediaAssetID.Valid {
		return ErrBusinessLicenseRequired
	}
	if !app.FoodPermitMediaAssetID.Valid {
		return ErrFoodLicenseRequired
	}
	if !app.IDCardFrontMediaAssetID.Valid {
		return ErrIDCardFrontRequired
	}
	if !app.IDCardBackMediaAssetID.Valid {
		return ErrIDCardBackRequired
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
func (server *Server) checkMerchantApplicationApproval(ctx *gin.Context, app db.MerchantApplication) error {
	// 1. 检查经纬度
	if !app.Longitude.Valid || !app.Latitude.Valid {
		return apierr(ErrMerchantLocationRequired.Code, "请选择商户地理位置")
	}

	payloadResult, err := logic.BuildMerchantDocumentReviewInputFromPayloads(logic.MerchantDocumentReviewPayloads{
		BusinessLicenseJSON: app.BusinessLicenseOcr,
		FoodPermitJSON:      app.FoodPermitOcr,
		IDCardFrontJSON:     app.IDCardFrontOcr,
		IDCardBackJSON:      app.IDCardBackOcr,
	})
	if err != nil {
		return merchantDocumentReviewAPIError(err)
	}

	if payloadResult.Input.BusinessLicense.ValidPeriod == "" && server.store != nil && payloadResult.Input.BusinessLicense.OCRJobID != nil && *payloadResult.Input.BusinessLicense.OCRJobID > 0 {
		ocrJobID := *payloadResult.Input.BusinessLicense.OCRJobID
		job, jobErr := server.store.GetOCRJob(ctx, ocrJobID)
		if jobErr != nil {
			log.Warn().Err(jobErr).Int64("application_id", app.ID).Int64("ocr_job_id", ocrJobID).Msg("submit merchant application: load business license ocr job for repair failed")
		} else if len(job.RawResult) > 0 {
			repairedBusinessLicenseJSON, changed, repairErr := logic.RepairMerchantBusinessLicenseFromRawResult(&payloadResult.Input.BusinessLicense, job.RawResult)
			if repairErr != nil {
				log.Warn().Err(repairErr).Int64("application_id", app.ID).Int64("ocr_job_id", job.ID).Msg("submit merchant application: decode business license ocr raw result failed")
			} else if changed {
				if _, updateErr := server.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
					ID:                 app.ID,
					BusinessLicenseOcr: repairedBusinessLicenseJSON,
				}); updateErr != nil {
					log.Warn().Err(updateErr).Int64("application_id", app.ID).Msg("submit merchant application: persist repaired business license ocr failed")
					return errors.New("系统繁忙，请稍后重试")
				}
			}
		}
	}

	if server.store != nil && payloadResult.Input.FoodPermit.OCRJobID != nil && *payloadResult.Input.FoodPermit.OCRJobID > 0 {
		ocrJobID := *payloadResult.Input.FoodPermit.OCRJobID
		job, jobErr := server.store.GetOCRJob(ctx, ocrJobID)
		if jobErr != nil {
			log.Warn().Err(jobErr).Int64("application_id", app.ID).Int64("ocr_job_id", ocrJobID).Msg("submit merchant application: load food permit ocr job for repair failed")
		} else {
			if payloadResult.FoodPermitNeedsNormalizedRepair && len(job.NormalizedResult) > 0 {
				repairedFoodPermitJSON, changed, repairErr := logic.RepairMerchantFoodPermitFromNormalized(&payloadResult.Input.FoodPermit, job.NormalizedResult)
				if repairErr != nil {
					log.Warn().Err(repairErr).Int64("application_id", app.ID).Int64("ocr_job_id", job.ID).Msg("submit merchant application: decode food permit ocr normalized result failed")
				} else if changed {
					payloadResult.RepairedFoodPermitJSON = repairedFoodPermitJSON
					log.Info().Int64("application_id", app.ID).Int64("ocr_job_id", job.ID).Str("ocr_provider", job.Provider).Msg("submit merchant application: repaired food permit ocr from job result")
				}
			}
			if merchantFoodPermitNeedsOfficialVerification(payloadResult.Input.BusinessLicense, payloadResult.Input.FoodPermit) && server.foodPermitOfficialVerifier != nil && len(job.RawResult) > 0 {
				verification, verifyErr := server.foodPermitOfficialVerifier.VerifyMerchantFoodPermit(ctx, job.RawResult)
				if verifyErr != nil {
					logFoodPermitOfficialVerificationFailure(verifyErr, app.ID, job.ID, job.Provider)
				} else if !merchantFoodPermitOfficialVerificationMatchesLicense(app, payloadResult.Input.BusinessLicense, verification) {
					log.Warn().
						Int64("application_id", app.ID).
						Int64("ocr_job_id", job.ID).
						Str("ocr_provider", job.Provider).
						Msg("submit merchant application: food permit official verification credit code mismatch")
				} else {
					repairedFoodPermitJSON, changed, repairErr := logic.RepairMerchantFoodPermitFromOfficialVerification(&payloadResult.Input.FoodPermit, verification)
					if repairErr != nil {
						log.Warn().Err(repairErr).Int64("application_id", app.ID).Int64("ocr_job_id", job.ID).Msg("submit merchant application: repair food permit from official verification failed")
					} else if changed {
						payloadResult.RepairedFoodPermitJSON = repairedFoodPermitJSON
						log.Info().Int64("application_id", app.ID).Int64("ocr_job_id", job.ID).Str("ocr_provider", job.Provider).Msg("submit merchant application: repaired food permit ocr from official verification")
					}
				}
			}
		}
	}

	if err := server.persistRepairedFoodPermitOCR(ctx, app.ID, payloadResult.RepairedFoodPermitJSON); err != nil {
		return errors.New("系统繁忙，请稍后重试")
	}

	documentReview, err := logic.EvaluateMerchantDocumentReview(payloadResult.Input, time.Now())
	if err != nil {
		if reviewErr, ok := err.(*logic.MerchantDocumentReviewError); ok {
			switch reviewErr.Code {
			case logic.MerchantDocumentReviewCodeFoodPermitNameUnreadable, logic.MerchantDocumentReviewCodeFoodPermitNameMismatch:
				server.logFoodPermitValidationFailure(ctx, app, payloadResult.Input.FoodPermit, strings.TrimSpace(payloadResult.Input.BusinessLicense.EnterpriseName), strings.TrimSpace(payloadResult.Input.FoodPermit.CompanyName), reviewErr.Code)
			}
		}
		return merchantDocumentReviewAPIError(err)
	}

	// 3. 检查地址匹配（营业执照地址与地图坐标反查地址）
	// 文本匹配失败时，再用营业执照地址正向解析坐标做 1000m 半径容差。
	// 这个半径只表示“证照地址与申请定位大致一致”，不参与门店重复判定。
	reviewAddresses, reviewErr := server.resolveMerchantLocationReviewAddresses(ctx, app)
	if reviewErr != nil {
		log.Warn().Err(reviewErr).Int64("application_id", app.ID).Msg("merchant application: reverse geocode failed, fallback to stored address")
	}
	matchedAddress, matched := matchMerchantLicenseAddress(documentReview.LicenseAddress, reviewAddresses)
	if !matched {
		distanceMeters, distanceMatched := server.merchantLicenseAddressWithinValidationRadius(ctx, app, documentReview.LicenseAddress)
		if distanceMatched {
			log.Info().
				Int64("application_id", app.ID).
				Str("license_address", documentReview.LicenseAddress).
				Int("distance_m", distanceMeters).
				Int("radius_m", merchantApplicationAddressValidationRadiusMeters).
				Msg("merchant application address matched by geocoded license radius")
		} else {
			query, poiDistanceMeters, poiMatched := server.merchantNameLocationWithinValidationRadius(ctx, app, documentReview)
			if poiMatched {
				log.Info().
					Int64("application_id", app.ID).
					Str("merchant_location_query", query).
					Int("distance_m", poiDistanceMeters).
					Int("radius_m", merchantApplicationAddressValidationRadiusMeters).
					Msg("merchant application address matched by geocoded merchant name")
			} else {
				reviewAddress := app.BusinessAddress
				if len(reviewAddresses) > 0 {
					reviewAddress = reviewAddresses[0]
				}
				log.Warn().
					Int64("application_id", app.ID).
					Str("license_address", documentReview.LicenseAddress).
					Str("review_address", strings.TrimSpace(reviewAddress)).
					Int("radius_m", merchantApplicationAddressValidationRadiusMeters).
					Msg("merchant application address evidence ambiguous; continue automatic approval")
			}
		}
	}
	if matchedAddress != "" && matchedAddress != app.BusinessAddress {
		log.Info().
			Int64("application_id", app.ID).
			Str("license_address", documentReview.LicenseAddress).
			Str("review_address", matchedAddress).
			Msg("merchant application address matched by geocoded location")
	}

	// 6. 检查地址是否已被占用（GPS 距离去重）
	// 五家店同写"希望路北段路东"但各自打了不同坐标 → 各 GPS 距离 > 20m，均可入驻。
	// 同一商户重复申请或两家店坐标几乎完全相同 → 距离 ≤ 20m，拒绝。
	// 字符串比较在模糊地址（无门牌号）场景下天生有歧义，GPS 是唯一可靠手段。
	// 这里的 20m 是物理门店点位去重阈值，不是证照地址匹配半径。
	if !app.Longitude.Valid || !app.Latitude.Valid {
		return apierr(ErrMerchantLocationRequired.Code, "请选择商户地理位置")
	}
	if app.RegionID.Valid {
		appLat := pgNumericToFloat64(app.Latitude)
		appLng := pgNumericToFloat64(app.Longitude)
		appLoc := algorithm.Location{Latitude: appLat, Longitude: appLng}

		locations, err := server.store.ListMerchantLocationsInRegion(ctx, app.RegionID.Int64)
		if err != nil {
			log.Warn().Err(err).Int64("region_id", app.RegionID.Int64).Msg("GPS 去重查询失败，跳过")
		} else {
			const duplicateThresholdMeters = 20
			for _, loc := range locations {
				if loc.OwnerUserID == app.UserID {
					continue // 排除申请人自己
				}
				existLat := pgNumericToFloat64(loc.Latitude)
				existLng := pgNumericToFloat64(loc.Longitude)
				existLoc := algorithm.Location{Latitude: existLat, Longitude: existLng}
				dist := algorithm.HaversineDistance(appLoc, existLoc)
				if dist <= duplicateThresholdMeters {
					log.Warn().
						Str("app_addr", app.BusinessAddress).
						Str("exist_addr", loc.Address).
						Int("dist_m", dist).
						Msg("GPS 地址重复检测命中")
					return apierr(ErrApplicationInvalidState.Code, "该位置附近已有其他商户完成入驻，请核对定位后重试")
				}
			}
		}
	}

	// P1-038: 防欺诈多重校验
	// 4. 检查营业执照是否已被使用
	licenseCount, err := server.store.CheckBusinessLicenseExists(ctx, db.CheckBusinessLicenseExistsParams{
		BusinessLicenseNumber: app.BusinessLicenseNumber,
		ID:                    app.ID,
	})
	if err != nil {
		log.Error().Err(err).Int64("application_id", app.ID).Msg("failed to check duplicate license")
		return errors.New("系统繁忙，请稍后重试")
	}
	if licenseCount > 0 {
		return apierr(ErrApplicationInvalidState.Code, "该营业执照号码已被其他商户使用，如非重复申请请联系客服处理") // 防止恶意抢注/重复入驻
	}

	// 5. 检查身份证是否已被使用
	// 注意：此处严格限制身份证唯一性，暂不支持同一法人开设多家店铺（防止欺诈多账号）
	// 如需支持连锁店，需放宽此处逻辑或增加连锁店审核流程
	idCardCount, err := server.store.CheckLegalPersonIDExists(ctx, db.CheckLegalPersonIDExistsParams{
		LegalPersonIDNumber: app.LegalPersonIDNumber,
		ID:                  app.ID,
	})
	if err != nil {
		log.Error().Err(err).Int64("application_id", app.ID).Msg("failed to check duplicate id card")
		return errors.New("系统繁忙，请稍后重试")
	}
	if idCardCount > 0 {
		return apierr(ErrApplicationInvalidState.Code, "该身份证号码已被用于其他商户入驻申请，如有疑问请联系客服处理")
	}

	return nil
}

func (server *Server) merchantLicenseAddressWithinValidationRadius(ctx context.Context, app db.MerchantApplication, licenseAddress string) (int, bool) {
	if server.mapClient == nil || !app.Latitude.Valid || !app.Longitude.Valid || strings.TrimSpace(licenseAddress) == "" {
		return 0, false
	}

	return server.merchantGeocodedAddressWithinValidationRadius(ctx, app, licenseAddress)
}

func (server *Server) merchantNameLocationWithinValidationRadius(ctx context.Context, app db.MerchantApplication, review logic.MerchantDocumentReviewResult) (string, int, bool) {
	if server.mapClient == nil || !app.Latitude.Valid || !app.Longitude.Valid {
		return "", 0, false
	}

	for _, query := range merchantNameLocationQueries(app, review) {
		distanceMeters, matched := server.merchantGeocodedAddressWithinValidationRadius(ctx, app, query)
		if matched {
			return query, distanceMeters, true
		}
	}

	return "", 0, false
}

func (server *Server) merchantGeocodedAddressWithinValidationRadius(ctx context.Context, app db.MerchantApplication, query string) (int, bool) {
	query = strings.TrimSpace(query)
	if server.mapClient == nil || !app.Latitude.Valid || !app.Longitude.Valid || query == "" {
		return 0, false
	}

	result, err := server.mapClient.Geocode(ctx, query)
	if err != nil || result == nil {
		if err != nil {
			log.Warn().Err(err).Int64("application_id", app.ID).Str("geocode_query", query).Msg("merchant application: geocode location for radius match failed")
		}
		return 0, false
	}

	appLoc := algorithm.Location{
		Latitude:  pgNumericToFloat64(app.Latitude),
		Longitude: pgNumericToFloat64(app.Longitude),
	}
	licenseLoc := algorithm.Location{
		Latitude:  result.Location.Lat,
		Longitude: result.Location.Lng,
	}
	distanceMeters := algorithm.HaversineDistance(appLoc, licenseLoc)
	return distanceMeters, distanceMeters <= merchantApplicationAddressValidationRadiusMeters
}

func merchantNameLocationQueries(app db.MerchantApplication, review logic.MerchantDocumentReviewResult) []string {
	names := merchantLocationEvidenceNames(app.MerchantName, review.LicenseName)
	if len(names) == 0 {
		return nil
	}

	prefixes := uniqueNonEmptyAddresses(
		merchantAdministrativeAddressPrefix(app.BusinessAddress),
		merchantAdministrativeAddressPrefix(review.LicenseAddress),
	)

	queries := make([]string, 0, len(names)*(len(prefixes)+2))
	for _, name := range names {
		for _, prefix := range prefixes {
			queries = append(queries, prefix+name)
		}
		if strings.TrimSpace(review.LicenseAddress) != "" {
			queries = append(queries, strings.TrimSpace(review.LicenseAddress)+" "+name)
		}
		queries = append(queries, name)
	}

	return uniqueNonEmptyAddresses(queries...)
}

func merchantLocationEvidenceNames(merchantName, licenseName string) []string {
	licenseName = strings.TrimSpace(licenseName)
	merchantName = strings.TrimSpace(merchantName)
	if licenseName == "" {
		return nil
	}
	if merchantName == "" || !merchantCompanyNamesRoughlyEqual(merchantName, licenseName) {
		return []string{licenseName}
	}
	return uniqueNonEmptyAddresses(merchantName, licenseName)
}

func merchantAdministrativeAddressPrefix(address string) string {
	parsed := parseChineseAddress(address)
	return strings.TrimSpace(parsed.Province + parsed.City + parsed.District)
}

func merchantDocumentReviewAPIError(err error) error {
	reviewErr, ok := err.(*logic.MerchantDocumentReviewError)
	if !ok {
		return err
	}

	switch reviewErr.Code {
	case logic.MerchantDocumentReviewCodeBusinessLicenseRequired:
		return apierr(ErrBusinessLicenseRequired.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeFoodLicenseRequired:
		return apierr(ErrFoodLicenseRequired.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeIDCardFrontRequired:
		return apierr(ErrIDCardFrontRequired.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeIDCardBackRequired:
		return apierr(ErrIDCardBackRequired.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeBusinessLicenseValidityInvalid:
		return apierr(ErrApplymentBusinessLicenseValidityInvalid.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeApplicationInvalidState:
		return apierr(ErrApplicationInvalidState.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeLicenseNameUnreadable:
		return apierr(ErrMerchantBusinessLicenseNameUnreadable.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeFoodPermitNameUnreadable:
		return apierr(ErrMerchantFoodPermitNameUnreadable.Code, reviewErr.Message)
	case logic.MerchantDocumentReviewCodeFoodPermitNameMismatch:
		return apierr(ErrMerchantFoodPermitNameMismatch.Code, reviewErr.Message)
	default:
		return err
	}
}

func merchantFoodPermitNeedsOfficialVerification(businessLicense logic.MerchantReviewBusinessLicenseOCRData, foodPermit logic.MerchantReviewFoodPermitOCRData) bool {
	licenseName := strings.TrimSpace(businessLicense.EnterpriseName)
	permitName := strings.TrimSpace(foodPermit.CompanyName)
	if licenseName == "" {
		return false
	}
	if permitName == "" || merchantFoodPermitNameLooksLikeCertificateText(permitName) {
		return true
	}
	return !merchantCompanyNamesRoughlyEqual(licenseName, permitName)
}

func merchantFoodPermitNameLooksLikeCertificateText(name string) bool {
	for _, keyword := range []string{"地址", "经营场所", "面积", "办理", "许可证", "登记证", "小餐饮", "小作坊", "《"} {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}

func merchantCompanyNamesRoughlyEqual(a string, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func merchantFoodPermitOfficialVerificationMatchesLicense(app db.MerchantApplication, businessLicense logic.MerchantReviewBusinessLicenseOCRData, verification logic.MerchantFoodPermitOfficialVerification) bool {
	officialCreditCode := merchantNormalizeBusinessCreditCode(verification.CreditCode)
	if officialCreditCode == "" {
		return true
	}
	for _, candidate := range []string{businessLicense.CreditCode, businessLicense.RegNum, app.BusinessLicenseNumber} {
		if merchantNormalizeBusinessCreditCode(candidate) == officialCreditCode {
			return true
		}
	}
	return false
}

func merchantNormalizeBusinessCreditCode(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\t", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ToUpper(value)
}

func logFoodPermitOfficialVerificationFailure(err error, applicationID int64, ocrJobID int64, provider string) {
	logger := log.Warn()
	if errors.Is(err, logic.ErrMerchantFoodPermitOfficialVerificationUnavailable) {
		logger = log.Info()
	}
	logger.Err(err).
		Int64("application_id", applicationID).
		Int64("ocr_job_id", ocrJobID).
		Str("ocr_provider", provider).
		Msg("submit merchant application: food permit official verification failed")
}

func (server *Server) logFoodPermitValidationFailure(ctx *gin.Context, app db.MerchantApplication, foodPermitOCR logic.MerchantReviewFoodPermitOCRData, licenseName, permitName, reason string) {
	logger := log.Warn().
		Int64("application_id", app.ID).
		Int64("user_id", app.UserID).
		Str("reason", reason).
		Str("license_name", licenseName).
		Str("permit_name", permitName)
	if foodPermitOCR.OCRJobID != nil && *foodPermitOCR.OCRJobID > 0 {
		logger = logger.Int64("ocr_job_id", *foodPermitOCR.OCRJobID)
		if server.store != nil {
			job, err := server.store.GetOCRJob(ctx, *foodPermitOCR.OCRJobID)
			if err != nil {
				log.Warn().Err(err).Int64("application_id", app.ID).Int64("ocr_job_id", *foodPermitOCR.OCRJobID).Msg("submit merchant application: load food permit ocr job failed")
			} else {
				logger = logger.Str("ocr_provider", job.Provider).Str("ocr_job_status", job.Status)
			}
		}
	}
	if foodPermitOCR.RawText != "" {
		logger = logger.Str("raw_text_preview", truncateMerchantOCRText(foodPermitOCR.RawText, 200))
	}
	logger.Msg("submit merchant application: food permit validation failed")
}

func (server *Server) persistRepairedFoodPermitOCR(ctx context.Context, appID int64, foodPermitOCR []byte) error {
	if len(foodPermitOCR) == 0 || server.store == nil {
		return nil
	}
	if _, err := server.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
		ID:            appID,
		FoodPermitOcr: foodPermitOCR,
	}); err != nil {
		log.Warn().Err(err).Int64("application_id", appID).Msg("submit merchant application: persist repaired food permit ocr failed")
		return err
	}
	return nil
}

func truncateMerchantOCRText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
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
	// District 解析时若地址缺少省/市前缀，正则可能把上级地名吞入 district（如"邢台宁晋县"）
	// 因此使用后缀匹配：只要一方是另一方的后缀即视为同一区县
	if parsed1.District != "" && parsed2.District != "" {
		d1, d2 := parsed1.District, parsed2.District
		if d1 != d2 && !strings.HasSuffix(d1, d2) && !strings.HasSuffix(d2, d1) {
			return false
		}
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

	roadsEquivalent := road1 != "" && road2 != "" && merchantRoadNamesEquivalent(road1, road2)

	// 路名必须相同（这是核心匹配条件）
	if road1 == "" || road2 == "" || (road1 != road2 && !roadsEquivalent) {
		// 检查路名后缀匹配：营业执照地址可能带区域前缀（如"经济开发区吉祥路" vs "吉祥路"）
		if road1 != "" && road2 != "" && (strings.HasSuffix(road1, road2) || strings.HasSuffix(road2, road1)) {
			// 任一方为模糊描述（无数字门牌，如"北段路东"、"晶龙集团"）：
			// 字符串层面无法判断是否同一地点（营业执照常带行政区前缀+模糊位置），
			// 接受此匹配，由 GPS 坐标去重兜底防止真正重复注册。
			if isFuzzy1 || isFuzzy2 {
				return true
			}
			// 双方均有数字门牌号：要求门牌号一致
			if normalizeNumber(number1) == normalizeNumber(number2) {
				return true
			}
			if (number1 != "" && number2 == "") || (number1 == "" && number2 != "") {
				return true
			}
		}
		// 路名不同或无法提取，降级到简单匹配
		return simpleAddressMatch(licenseAddr, businessAddr)
	}

	// 路名相同的情况下：
	// 4a. 如果任一方使用模糊位置描述（如"中段东侧"、"路口"），视为匹配
	if isFuzzy1 || isFuzzy2 {
		return true
	}

	if roadsEquivalent && road1 != road2 {
		return false
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

func (server *Server) resolveMerchantLocationReviewAddresses(ctx context.Context, app db.MerchantApplication) ([]string, error) {
	fallbackAddresses := uniqueNonEmptyAddresses(app.BusinessAddress)
	if server.mapClient == nil || !app.Latitude.Valid || !app.Longitude.Valid {
		return fallbackAddresses, nil
	}

	lat := pgNumericToFloat64(app.Latitude)
	lng := pgNumericToFloat64(app.Longitude)
	result, err := server.mapClient.ReverseGeocode(ctx, maps.Location{Lat: lat, Lng: lng})
	if err != nil {
		if len(fallbackAddresses) > 0 {
			return fallbackAddresses, err
		}
		return nil, err
	}

	geocodedAddresses := buildMerchantLocationReviewAddresses(result)
	if len(geocodedAddresses) == 0 {
		return fallbackAddresses, nil
	}
	return geocodedAddresses, nil
}

func buildMerchantLocationReviewAddresses(result *maps.ReverseGeocodeResult) []string {
	if result == nil {
		return nil
	}

	componentAddress := strings.TrimSpace(result.Province + result.City + result.District + result.Street + result.StreetNumber)
	districtStreetAddress := strings.TrimSpace(result.District + result.Street + result.StreetNumber)
	streetAddress := strings.TrimSpace(result.Street + result.StreetNumber)

	return uniqueNonEmptyAddresses(
		result.Address,
		result.FormattedAddress,
		componentAddress,
		districtStreetAddress,
		streetAddress,
	)
}

func uniqueNonEmptyAddresses(addresses ...string) []string {
	seen := make(map[string]struct{}, len(addresses))
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func matchMerchantLicenseAddress(licenseAddress string, reviewAddresses []string) (string, bool) {
	for _, reviewAddress := range reviewAddresses {
		if isAddressMatch(licenseAddress, reviewAddress) {
			return reviewAddress, true
		}
	}
	return "", false
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
	detail = strings.TrimPrefix(detail, "城")

	result.Detail = detail

	return result
}

// extractRoadAndNumberWithFuzzy 从详细地址中提取路名和门牌号，同时识别模糊位置描述
// 返回值：road（路名）, number（门牌号或位置描述）, isFuzzy（是否为模糊位置描述）
// 正则前缀字符集排除路名后缀字符（路/街/道/巷/弄），确保在第一个路名后缀处截止，
// 避免"希望路北段路东"被贪婪匹配为"希望路北段路"。多字后缀（大街/大道/胡同）使用优先候选。
func extractRoadAndNumberWithFuzzy(detail string) (road, number string, isFuzzy bool) {
	// 匹配 "XX路/街/巷/弄 + 门牌号或位置描述"
	// 路名后可能跟随：数字门牌号（如"100号"）或模糊描述（如"中段东侧"）
	roadRegex := regexp.MustCompile(`([^0-9路街道巷弄胡同]+(?:大街|大道|胡同|路|街|道|巷|弄))(.+)?`)
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

func merchantRoadNamesEquivalent(a, b string) bool {
	normalizedA := normalizeMerchantRoadName(a)
	normalizedB := normalizeMerchantRoadName(b)
	return normalizedA != "" && normalizedA == normalizedB
}

func normalizeMerchantRoadName(road string) string {
	road = strings.TrimSpace(road)
	road = strings.TrimSuffix(road, "辅路")
	for _, suffix := range []string{"西街", "东街", "南街", "北街", "西路", "东路", "南路", "北路"} {
		if strings.HasSuffix(road, suffix) {
			return strings.TrimSuffix(road, suffix) + string([]rune(suffix)[len([]rune(suffix))-1])
		}
	}
	return road
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
	return !numRegex.MatchString(location)
}

// normalizeNumber 标准化门牌号（提取前缀数字，忽略楼栋/单元等附加描述）
// 如 "133号 弘启名城(吉祥路)" 和 "133号" 均标准化为 "133"
func normalizeNumber(num string) string {
	num = strings.TrimSpace(num)
	// 优先提取前缀数字部分（门牌号后可能跟空格+楼栋名）
	numRegex := regexp.MustCompile(`^(\d+)`)
	if m := numRegex.FindString(num); m != "" {
		return m
	}
	// 无前缀数字，则仅去除尾部"号"
	return strings.TrimSpace(strings.TrimSuffix(num, "号"))
}

// simpleAddressMatch 简单地址匹配（降级方案）
// 必须：路名相同 + 门牌号相同，两者缺一不可。
// 纯模糊描述（如"希望路北段路东"）没有门牌号，不应在此通过——
// 此类情况应由 isAddressMatch 顶层的 Detail 包含检查处理。
func simpleAddressMatch(addr1, addr2 string) bool {
	// 1. 提取路名关键词，必须有共同路名
	keywords1 := extractAddressKeywords(addr1)
	keywords2 := extractAddressKeywords(addr2)

	hasCommonRoad := false
	for _, kw1 := range keywords1 {
		if len([]rune(kw1)) < 2 {
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
	if !hasCommonRoad {
		return false
	}

	// 2. 有共同路名 + 有共同门牌号数字 → 匹配
	numRegex := regexp.MustCompile(`\d+`)
	nums1 := numRegex.FindAllString(addr1, -1)
	nums2 := numRegex.FindAllString(addr2, -1)
	for _, n1 := range nums1 {
		for _, n2 := range nums2 {
			if n1 == n2 {
				return true
			}
		}
	}

	// 3. 有共同路名但双方均无门牌号（纯模糊地址）→ 不在此判断
	// 例：一方"希望路北段路东"，另一方"希望路 晶龙集团"，描述不同，拒绝
	return false
}

// extractAddressKeywords 提取地址关键词
func extractAddressKeywords(addr string) []string {
	var keywords []string
	// 提取路名：前缀字符集排除路名后缀字符，确保在第一个路名后缀处截止
	roadRegex := regexp.MustCompile(`[^省市区县镇乡村街道路巷弄]+(?:大街|大道|胡同|路|街|道|巷|弄)`)
	roads := roadRegex.FindAllString(addr, -1)
	keywords = append(keywords, roads...)
	return keywords
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
	t, err := parseISODate(dateFormatted, "")
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "rejected" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationCannotReset))
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

	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, resetResult.Application)
}
