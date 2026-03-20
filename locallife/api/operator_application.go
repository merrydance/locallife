package api

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// ==================== 运营商入驻申请 API ====================
// 与商户/骑手入驻不同：
// 1. 需要人工审核（商户/骑手是自动审核）
// 2. 区域独占（一区一运营商）
// 3. 有合同期限

// ==================== 数据结构 ====================

// operatorApplicationResponse 运营商申请响应
type operatorApplicationResponse struct {
	ID                     int64                   `json:"id"`
	UserID                 int64                   `json:"user_id"`
	RegionID               int64                   `json:"region_id"`
	RegionName             string                  `json:"region_name,omitempty"`
	Name                   string                  `json:"name,omitempty"`
	ContactName            string                  `json:"contact_name,omitempty"`
	ContactPhone           string                  `json:"contact_phone,omitempty"`
	BusinessLicenseAssetID *int64                  `json:"business_license_asset_id,omitempty"`
	BusinessLicenseNumber  string                  `json:"business_license_number,omitempty"`
	BusinessLicenseOCR     *BusinessLicenseOCRData `json:"business_license_ocr,omitempty"`
	LegalPersonName        string                  `json:"legal_person_name,omitempty"`
	LegalPersonIDNumber    string                  `json:"legal_person_id_number,omitempty"`
	IDCardFrontAssetID     *int64                  `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID      *int64                  `json:"id_card_back_asset_id,omitempty"`
	IDCardFrontOCR         *OperatorIDCardOCRData  `json:"id_card_front_ocr,omitempty"`
	IDCardBackOCR          *OperatorIDCardBackOCR  `json:"id_card_back_ocr,omitempty"`
	RequestedContractYears int32                   `json:"requested_contract_years"`
	Status                 string                  `json:"status"`
	RejectReason           string                  `json:"reject_reason,omitempty"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
	SubmittedAt            *time.Time              `json:"submitted_at,omitempty"`
	ReviewedAt             *time.Time              `json:"reviewed_at,omitempty"`
	// IsOperator 表示申请已通过且运营商账号已建立（即用户已是正式运营商）
	IsOperator bool `json:"is_operator,omitempty"`
}

// OperatorIDCardOCRData 运营商身份证正面OCR数据
type OperatorIDCardOCRData struct {
	Name     string `json:"name,omitempty"`
	IDNumber string `json:"id_number,omitempty"`
	Gender   string `json:"gender,omitempty"`
	Nation   string `json:"nation,omitempty"`
	Address  string `json:"address,omitempty"`
	OCRAt    string `json:"ocr_at,omitempty"`
}

// OperatorIDCardBackOCR 运营商身份证背面OCR数据
type OperatorIDCardBackOCR struct {
	ValidStart string `json:"valid_start,omitempty"`
	ValidEnd   string `json:"valid_end,omitempty"`
	OCRAt      string `json:"ocr_at,omitempty"`
}

func newOperatorApplicationResponse(app db.OperatorApplication, regionName string) operatorApplicationResponse {
	resp := operatorApplicationResponse{
		ID:                     app.ID,
		UserID:                 app.UserID,
		RegionID:               app.RegionID,
		RegionName:             regionName,
		RequestedContractYears: app.RequestedContractYears,
		Status:                 app.Status,
		CreatedAt:              app.CreatedAt,
		UpdatedAt:              app.UpdatedAt,
	}

	if app.Name.Valid {
		resp.Name = app.Name.String
	}
	if app.ContactName.Valid {
		resp.ContactName = app.ContactName.String
	}
	if app.ContactPhone.Valid {
		resp.ContactPhone = app.ContactPhone.String
	}
	if app.BusinessLicenseMediaAssetID.Valid {
		v := app.BusinessLicenseMediaAssetID.Int64
		resp.BusinessLicenseAssetID = &v
	}
	if app.BusinessLicenseNumber.Valid {
		resp.BusinessLicenseNumber = app.BusinessLicenseNumber.String
	}
	if app.LegalPersonName.Valid {
		resp.LegalPersonName = app.LegalPersonName.String
	}
	if app.LegalPersonIDNumber.Valid {
		resp.LegalPersonIDNumber = app.LegalPersonIDNumber.String
	}
	if app.IDCardFrontMediaAssetID.Valid {
		v := app.IDCardFrontMediaAssetID.Int64
		resp.IDCardFrontAssetID = &v
	}
	if app.IDCardBackMediaAssetID.Valid {
		v := app.IDCardBackMediaAssetID.Int64
		resp.IDCardBackAssetID = &v
	}
	if app.RejectReason.Valid {
		resp.RejectReason = app.RejectReason.String
	}
	if app.SubmittedAt.Valid {
		resp.SubmittedAt = &app.SubmittedAt.Time
	}
	if app.ReviewedAt.Valid {
		resp.ReviewedAt = &app.ReviewedAt.Time
	}

	// 解析OCR数据
	if len(app.BusinessLicenseOcr) > 0 {
		var ocr BusinessLicenseOCRData
		if json.Unmarshal(app.BusinessLicenseOcr, &ocr) == nil {
			resp.BusinessLicenseOCR = &ocr
		}
	}
	if len(app.IDCardFrontOcr) > 0 {
		var ocr OperatorIDCardOCRData
		if json.Unmarshal(app.IDCardFrontOcr, &ocr) == nil {
			resp.IDCardFrontOCR = &ocr
		}
	}
	if len(app.IDCardBackOcr) > 0 {
		var ocr OperatorIDCardBackOCR
		if json.Unmarshal(app.IDCardBackOcr, &ocr) == nil {
			resp.IDCardBackOCR = &ocr
		}
	}

	return resp
}

// ==================== 获取或创建草稿 ====================

type createOperatorApplicationRequest struct {
	RegionID int64 `json:"region_id" binding:"required,gt=0"`
}

// getOrCreateOperatorApplicationDraft godoc
// @Summary 获取或创建运营商入驻申请草稿
// @Description 获取当前用户的运营商申请草稿，如果没有则需要提供区域ID创建新草稿。已通过的申请不会返回。
// @Tags 运营商申请
// @Accept json
// @Produce json
// @Param request body createOperatorApplicationRequest false "创建草稿需要提供区域ID"
// @Success 200 {object} operatorApplicationResponse "获取成功"
// @Success 201 {object} operatorApplicationResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 409 {object} ErrorResponse "已有通过或待审核的申请，或区域已被占用"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application [post]
// @Security BearerAuth
func (server *Server) getOrCreateOperatorApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 首先检查是否已有申请
	existingApp, err := server.store.GetOperatorApplicationByUserID(ctx, authPayload.UserID)
	if err == nil {
		if existingApp.Status == "approved" {
			ctx.JSON(http.StatusConflict, errorResponse(ErrAlreadyOperator))
			return
		}
		if existingApp.Status == "submitted" {
			ctx.JSON(http.StatusConflict, errorResponse(ErrOperatorApplicationPending))
			return
		}
		// 返回草稿或被拒绝的申请
		regionName := server.getRegionName(ctx, existingApp.RegionID)
		ctx.JSON(http.StatusOK, newOperatorApplicationResponse(existingApp, regionName))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查用户是否已经是运营商
	_, err = server.store.GetOperatorByUser(ctx, authPayload.UserID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrAlreadyOperator))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 需要创建新草稿，必须提供区域ID
	var req createOperatorApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRegionSelectionRequired))
		return
	}

	// 验证区域存在
	region, err := server.store.GetRegion(ctx, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查区域是否已被其他运营商占用
	_, err = server.store.GetOperatorByRegion(ctx, req.RegionID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasOperator))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有其他人正在申请该区域
	_, err = server.store.GetPendingOperatorApplicationByRegion(ctx, req.RegionID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasPendingApplication))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建草稿
	newApp, err := server.store.CreateOperatorApplicationDraft(ctx, db.CreateOperatorApplicationDraftParams{
		UserID:   authPayload.UserID,
		RegionID: req.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newOperatorApplicationResponse(newApp, region.Name))
}

// getOperatorApplication godoc
// @Summary 获取当前运营商申请状态
// @Description 获取当前用户的运营商申请信息
// @Tags 运营商申请
// @Produce json
// @Success 200 {object} operatorApplicationResponse "申请信息"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application [get]
// @Security BearerAuth
func (server *Server) getOperatorApplication(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetOperatorApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusOK, struct{}{})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newOperatorApplicationResponse(app, server.getRegionName(ctx, app.RegionID))
	// 若申请已通过，进一步检查运营商账号是否已建立
	if app.Status == "approved" {
		if _, opErr := server.store.GetOperatorByUser(ctx, authPayload.UserID); opErr == nil {
			resp.IsOperator = true
		}
	}
	ctx.JSON(http.StatusOK, resp)
}

// ==================== 更新区域 ====================

type updateOperatorApplicationRegionRequest struct {
	RegionID int64 `json:"region_id" binding:"required,gt=0"`
}

// updateOperatorApplicationRegion godoc
// @Summary 更新申请的区域
// @Description 更新运营商申请的目标区域（仅草稿状态可修改）
// @Tags 运营商申请
// @Accept json
// @Produce json
// @Param request body updateOperatorApplicationRegionRequest true "区域信息"
// @Success 200 {object} operatorApplicationResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 409 {object} ErrorResponse "区域已被占用"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/region [put]
// @Security BearerAuth
func (server *Server) updateOperatorApplicationRegion(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req updateOperatorApplicationRegionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取申请
	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	// 如果区域没变，直接返回
	if app.RegionID == req.RegionID {
		regionName := server.getRegionName(ctx, app.RegionID)
		ctx.JSON(http.StatusOK, newOperatorApplicationResponse(app, regionName))
		return
	}

	// 验证新区域存在
	region, err := server.store.GetRegion(ctx, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查新区域是否已被占用
	_, err = server.store.GetOperatorByRegion(ctx, req.RegionID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasOperator))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有其他人正在申请该区域
	pendingApp, err := server.store.GetPendingOperatorApplicationByRegion(ctx, req.RegionID)
	if err == nil && pendingApp.UserID != authPayload.UserID {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasPendingApplication))
		return
	}
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 更新区域
	updatedApp, err := server.store.UpdateOperatorApplicationRegion(ctx, db.UpdateOperatorApplicationRegionParams{
		ID:       app.ID,
		RegionID: req.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(updatedApp, region.Name))
}

// ==================== 更新基础信息 ====================

type updateOperatorApplicationBasicRequest struct {
	Name                   *string `json:"name" binding:"omitempty,max=50"`
	ContactName            *string `json:"contact_name" binding:"omitempty,min=2,max=20"`
	ContactPhone           *string `json:"contact_phone" binding:"omitempty,len=11"`
	RequestedContractYears *int32  `json:"requested_contract_years" binding:"omitempty,min=1,max=10"`
}

// updateOperatorApplicationBasicInfo godoc
// @Summary 更新运营商申请基础信息
// @Description 更新运营商名称、联系人、联系电话、申请合同年限（仅草稿状态可编辑）
// @Tags 运营商申请
// @Accept json
// @Produce json
// @Param request body updateOperatorApplicationBasicRequest true "基础信息"
// @Success 200 {object} operatorApplicationResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/basic [put]
// @Security BearerAuth
func (server *Server) updateOperatorApplicationBasicInfo(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req updateOperatorApplicationBasicRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取申请
	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	// 构建更新参数
	arg := db.UpdateOperatorApplicationBasicInfoParams{
		ID: app.ID,
	}
	if req.Name != nil {
		arg.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.ContactName != nil {
		arg.ContactName = pgtype.Text{String: *req.ContactName, Valid: true}
	}
	if req.ContactPhone != nil {
		arg.ContactPhone = pgtype.Text{String: *req.ContactPhone, Valid: true}
	}
	if req.RequestedContractYears != nil {
		arg.RequestedContractYears = pgtype.Int4{Int32: *req.RequestedContractYears, Valid: true}
	}

	updatedApp, err := server.store.UpdateOperatorApplicationBasicInfo(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, updatedApp.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(updatedApp, regionName))
}

// ==================== 上传营业执照并OCR识别 ====================

// uploadOperatorBusinessLicenseOCR godoc
// @Summary 上传营业执照并OCR识别
// @Description 上传营业执照图片，调用微信OCR识别并保存结果，自动填充企业名称、信用代码等
// @Tags 运营商申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "营业执照图片"
// @Success 200 {object} operatorApplicationResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/license/ocr [post]
// @Security BearerAuth
func (server *Server) uploadOperatorBusinessLicenseOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取申请
	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	// 获取上传的文件；若未提供文件则检查 media_asset_id（媒体服务流程）
	file, fileHeader, err := ctx.Request.FormFile("image")
	var fromAssetID bool
	var assetFileBytes []byte
	var mediaAssetID int64
	if err != nil {
		if assetIDStr := ctx.PostForm("media_asset_id"); assetIDStr != "" {
			if id, parseErr := strconv.ParseInt(assetIDStr, 10, 64); parseErr == nil && id > 0 {
				mediaAssetID = id
				localPath := server.mediaAssetLocalPath(ctx, id)
				if localPath == "" {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidDocumentImageURL))
					return
				}
				data, readErr := os.ReadFile(localPath)
				if readErr != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, readErr))
					return
				}
				assetFileBytes = data
				fromAssetID = true
			}
		}
		if !fromAssetID {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrBusinessLicenseRequired))
			return
		}
	} else {
		defer file.Close()
	}

	var ocrReader multipart.File
	if fromAssetID {
		ocrReader = util.NewBytesFile(assetFileBytes)
	} else {
		// 内容安全检测（文件上传路径）
		if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
			if errors.Is(err, wechat.ErrRiskyContent) {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageContentSafetyFailed))
				return
			}
			if errors.Is(err, wechat.ErrImageTooLarge) {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageTooLarge))
				return
			}
			ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		uploader := util.NewFileUploader("uploads")
		_, err = uploader.UploadOperatorImage(authPayload.UserID, "license", file, fileHeader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if _, err := file.Seek(0, 0); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ocrReader = file
	}

	// 调用微信OCR
	ocrResult, err := server.wechatClient.OCRBusinessLicense(ctx, ocrReader)
	if err != nil {
		log.Error().Err(err).Msg("营业执照OCR识别失败")
		// OCR失败不阻止保存，允许手动填写
	}

	// 构建更新参数
	arg := db.UpdateOperatorApplicationBusinessLicenseParams{
		ID: app.ID,
	}
	if fromAssetID {
		arg.BusinessLicenseMediaAssetID = pgtype.Int8{Int64: mediaAssetID, Valid: true}
	}

	if ocrResult != nil {
		// 保存OCR结果
		ocrData := BusinessLicenseOCRData{
			RegNum:              ocrResult.RegNum,
			EnterpriseName:      ocrResult.EnterpriseEName,
			LegalRepresentative: ocrResult.LegalRepresentative,
			TypeOfEnterprise:    ocrResult.TypeOfEnterprise,
			Address:             ocrResult.Address,
			BusinessScope:       ocrResult.BusinessScope,
			RegisteredCapital:   ocrResult.RegisteredCapital,
			ValidPeriod:         ocrResult.ValidPeriod,
			CreditCode:          ocrResult.CreditCode,
			OCRAt:               time.Now().Format(time.RFC3339),
		}
		ocrJSON, _ := json.Marshal(ocrData)
		arg.BusinessLicenseOcr = ocrJSON

		// 自动回填信息
		if ocrResult.CreditCode != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: ocrResult.CreditCode, Valid: true}
		} else if ocrResult.RegNum != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: ocrResult.RegNum, Valid: true}
		}
		if ocrResult.EnterpriseEName != "" {
			arg.Name = pgtype.Text{String: ocrResult.EnterpriseEName, Valid: true}
		}
	}

	updatedApp, err := server.store.UpdateOperatorApplicationBusinessLicense(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, updatedApp.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(updatedApp, regionName))
}

// ==================== 上传身份证并OCR识别 ====================

// uploadOperatorIDCardOCR godoc
// @Summary 上传法人身份证并OCR识别
// @Description 上传法人身份证照片（正面或背面），调用微信OCR识别并保存结果
// @Tags 运营商申请
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "身份证图片"
// @Param side formData string true "正面Front/背面Back"
// @Success 200 {object} operatorApplicationResponse "识别结果"
// @Failure 400 {object} ErrorResponse "参数错误或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/idcard/ocr [post]
// @Security BearerAuth
func (server *Server) uploadOperatorIDCardOCR(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取申请
	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	// 获取上传的文件；若未提供文件则检查 media_asset_id（媒体服务流程）
	file, fileHeader, err := ctx.Request.FormFile("image")
	var fromAssetID bool
	var assetFileBytes []byte
	var mediaAssetID int64
	if err != nil {
		if assetIDStr := ctx.PostForm("media_asset_id"); assetIDStr != "" {
			if id, parseErr := strconv.ParseInt(assetIDStr, 10, 64); parseErr == nil && id > 0 {
				mediaAssetID = id
				localPath := server.mediaAssetLocalPath(ctx, id)
				if localPath == "" {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidDocumentImageURL))
					return
				}
				data, readErr := os.ReadFile(localPath)
				if readErr != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, readErr))
					return
				}
				assetFileBytes = data
				fromAssetID = true
			}
		}
		if !fromAssetID {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrIDCardImageRequired))
			return
		}
	} else {
		defer file.Close()
	}

	side := ctx.PostForm("side")
	if side != "Front" && side != "Back" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidIDCardSide))
		return
	}

	var ocrReader multipart.File
	if fromAssetID {
		ocrReader = util.NewBytesFile(assetFileBytes)
	} else {
		// 内容安全检测（文件上传路径）
		if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
			if errors.Is(err, wechat.ErrRiskyContent) {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageContentSafetyFailed))
				return
			}
			if errors.Is(err, wechat.ErrImageTooLarge) {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrImageTooLarge))
				return
			}
			ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		uploader := util.NewFileUploader("uploads")
		_, err = uploader.UploadOperatorImage(authPayload.UserID, "idcard_"+side, file, fileHeader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if _, err := file.Seek(0, 0); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ocrReader = file
	}

	// 调用微信OCR
	ocrResult, err := server.wechatClient.OCRIDCard(ctx, ocrReader, side)
	if err != nil {
		log.Error().Err(err).Msg("身份证OCR识别失败")
	}

	var updatedApp db.OperatorApplication

	if side == "Front" {
		arg := db.UpdateOperatorApplicationIDCardFrontParams{
			ID: app.ID,
		}
		if fromAssetID {
			arg.IDCardFrontMediaAssetID = pgtype.Int8{Int64: mediaAssetID, Valid: true}
		}

		if ocrResult != nil {
			ocrData := OperatorIDCardOCRData{
				Name:     ocrResult.Name,
				IDNumber: ocrResult.ID,
				Gender:   ocrResult.Gender,
				Nation:   ocrResult.Nation,
				Address:  ocrResult.Addr,
				OCRAt:    time.Now().Format(time.RFC3339),
			}
			ocrJSON, _ := json.Marshal(ocrData)
			arg.IDCardFrontOcr = ocrJSON

			// 自动回填
			if ocrResult.Name != "" {
				arg.LegalPersonName = pgtype.Text{String: ocrResult.Name, Valid: true}
			}
			if ocrResult.ID != "" {
				arg.LegalPersonIDNumber = pgtype.Text{String: ocrResult.ID, Valid: true}
			}
		}

		updatedApp, err = server.store.UpdateOperatorApplicationIDCardFront(ctx, arg)
	} else {
		arg := db.UpdateOperatorApplicationIDCardBackParams{
			ID: app.ID,
		}
		if fromAssetID {
			arg.IDCardBackMediaAssetID = pgtype.Int8{Int64: mediaAssetID, Valid: true}
		}

		if ocrResult != nil && ocrResult.ValidDate != "" {
			ocrData := OperatorIDCardBackOCR{
				ValidEnd: ocrResult.ValidDate,
				OCRAt:    time.Now().Format(time.RFC3339),
			}
			ocrJSON, _ := json.Marshal(ocrData)
			arg.IDCardBackOcr = ocrJSON
		}

		updatedApp, err = server.store.UpdateOperatorApplicationIDCardBack(ctx, arg)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, updatedApp.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(updatedApp, regionName))
}

// ==================== 提交申请 ====================

// submitOperatorApplication godoc
// @Summary 提交运营商入驻申请
// @Description 提交申请进入人工审核流程。与商户/骑手不同，运营商需要人工审核。
// @Tags 运营商申请
// @Produce json
// @Success 200 {object} operatorApplicationResponse "提交成功，等待审核"
// @Failure 400 {object} ErrorResponse "信息不完整或非草稿状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 409 {object} ErrorResponse "区域已被占用"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/submit [post]
// @Security BearerAuth
func (server *Server) submitOperatorApplication(ctx *gin.Context) {
	consentReq, err := parseAgreementConsentRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取申请
	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationSubmitDraft))
		return
	}

	server.writeAgreementConsentAudit(ctx, authPayload.UserID, "operator_application_consent_confirmed", "operator_application", app.ID, consentReq)

	// 个人运营商兜底：若运营商名称为空，自动使用法人/个人姓名
	if (!app.Name.Valid || strings.TrimSpace(app.Name.String) == "") && app.LegalPersonName.Valid {
		legalName := strings.TrimSpace(app.LegalPersonName.String)
		if legalName != "" {
			updatedApp, updateErr := server.store.UpdateOperatorApplicationBasicInfo(ctx, db.UpdateOperatorApplicationBasicInfoParams{
				ID:   app.ID,
				Name: pgtype.Text{String: legalName, Valid: true},
			})
			if updateErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, updateErr))
				return
			}
			app = updatedApp
		}
	}

	// 验证必填字段
	if err := validateOperatorApplicationRequired(app); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 再次检查区域是否已被占用（防止竞态条件）
	_, err = server.store.GetOperatorByRegion(ctx, app.RegionID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasOperator))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有其他已提交的申请
	pendingApp, err := server.store.GetPendingOperatorApplicationByRegion(ctx, app.RegionID)
	if err == nil && pendingApp.ID != app.ID {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasPendingApplication))
		return
	}
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 提交申请
	submittedApp, err := server.store.SubmitOperatorApplication(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, submittedApp.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(submittedApp, regionName))
}

// validateOperatorApplicationRequired 验证必填字段
func validateOperatorApplicationRequired(app db.OperatorApplication) error {
	if !app.Name.Valid || app.Name.String == "" {
		return ErrOperatorNameRequired
	}
	if !app.ContactName.Valid || app.ContactName.String == "" {
		return ErrContactNameRequired
	}
	if !app.ContactPhone.Valid || app.ContactPhone.String == "" {
		return ErrPhoneRequired
	}
	if !app.IDCardFrontMediaAssetID.Valid {
		return ErrLegalRepIDCardFrontRequired
	}
	if !app.IDCardBackMediaAssetID.Valid {
		return ErrLegalRepIDCardBackRequired
	}
	return nil
}

// ==================== 重置申请为草稿 ====================

// resetOperatorApplicationToDraft godoc
// @Summary 重置被拒绝的申请为草稿
// @Description 将被拒绝的申请重置为草稿状态，允许重新编辑提交
// @Tags 运营商申请
// @Produce json
// @Success 200 {object} operatorApplicationResponse "重置成功"
// @Failure 400 {object} ErrorResponse "只能重置被拒绝的申请"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/reset [post]
// @Security BearerAuth
func (server *Server) resetOperatorApplicationToDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetOperatorApplicationByUserID(ctx, authPayload.UserID)
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

	resetApp, err := server.store.ResetOperatorApplicationToDraft(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, resetApp.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(resetApp, regionName))
}

// ==================== 辅助函数 ====================

func (server *Server) getRegionName(ctx *gin.Context, regionID int64) string {
	region, err := server.store.GetRegion(ctx, regionID)
	if err != nil {
		return ""
	}
	if region.ParentID.Valid && region.ParentID.Int64 > 0 {
		parent, parentErr := server.store.GetRegion(ctx, region.ParentID.Int64)
		if parentErr == nil && strings.TrimSpace(parent.Name) != "" {
			return parent.Name + " - " + region.Name
		}
	}
	return region.Name
}
