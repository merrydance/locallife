package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
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
	IsOperator        bool                              `json:"is_operator,omitempty"`
	SettlementAccount *baofuSettlementReadinessResponse `json:"settlement_account,omitempty"`
}

// OperatorIDCardOCRData 运营商身份证正面OCR数据
type OperatorIDCardOCRData struct {
	Status         string `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	AlertEmittedAt string `json:"alert_emitted_at,omitempty"`
	QueuedAt       string `json:"queued_at,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	OCRJobID       *int64 `json:"ocr_job_id,omitempty"`
	Name           string `json:"name,omitempty"`
	IDNumber       string `json:"id_number,omitempty"`
	Gender         string `json:"gender,omitempty"`
	Nation         string `json:"nation,omitempty"`
	Address        string `json:"address,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

// OperatorIDCardBackOCR 运营商身份证背面OCR数据
type OperatorIDCardBackOCR struct {
	Status         string `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	AlertEmittedAt string `json:"alert_emitted_at,omitempty"`
	QueuedAt       string `json:"queued_at,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	OCRJobID       *int64 `json:"ocr_job_id,omitempty"`
	ValidStart     string `json:"valid_start,omitempty"`
	ValidEnd       string `json:"valid_end,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

type operatorApplicationDocumentType string

const (
	operatorApplicationDocumentBusinessLicense operatorApplicationDocumentType = "business_license"
	operatorApplicationDocumentIDCardFront     operatorApplicationDocumentType = "id_card_front"
	operatorApplicationDocumentIDCardBack      operatorApplicationDocumentType = "id_card_back"
)

func newOperatorApplicationResponse(app db.OperatorApplication, regionName string) (operatorApplicationResponse, error) {
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
		if err := decodeOperatorApplicationJSONField(app.ID, "business_license_ocr", app.BusinessLicenseOcr, &ocr); err != nil {
			return operatorApplicationResponse{}, err
		}
		resp.BusinessLicenseOCR = &ocr
	}
	if len(app.IDCardFrontOcr) > 0 {
		var ocr OperatorIDCardOCRData
		if err := decodeOperatorApplicationJSONField(app.ID, "id_card_front_ocr", app.IDCardFrontOcr, &ocr); err != nil {
			return operatorApplicationResponse{}, err
		}
		resp.IDCardFrontOCR = &ocr
	}
	if len(app.IDCardBackOcr) > 0 {
		var ocr OperatorIDCardBackOCR
		if err := decodeOperatorApplicationJSONField(app.ID, "id_card_back_ocr", app.IDCardBackOcr, &ocr); err != nil {
			return operatorApplicationResponse{}, err
		}
		resp.IDCardBackOCR = &ocr
	}

	return resp, nil
}

func decodeOperatorApplicationJSONField(applicationID int64, field string, payload []byte, target interface{}) error {
	if len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode operator application %d %s: %w", applicationID, field, err)
	}
	return nil
}

func (server *Server) writeOperatorApplicationResponse(ctx *gin.Context, status int, app db.OperatorApplication, regionName string) bool {
	resp, err := newOperatorApplicationResponse(app, regionName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return false
	}
	ctx.JSON(status, resp)
	return true
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
		server.writeOperatorApplicationResponse(ctx, http.StatusOK, existingApp, regionName)
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
	_, err = server.getRegionActiveOperator(ctx, req.RegionID)
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

	server.writeOperatorApplicationResponse(ctx, http.StatusCreated, newApp, region.Name)
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

	resp, err := newOperatorApplicationResponse(app, server.getRegionName(ctx, app.RegionID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	// 若申请已通过，进一步检查运营商账号是否已建立
	if app.Status == "approved" {
		operator, opErr := server.store.GetOperatorByUser(ctx, authPayload.UserID)
		if opErr == nil {
			resp.IsOperator = true
			readiness, readinessErr := server.getOperatorBaofuSettlementReadiness(ctx, operator)
			if readinessErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, readinessErr))
				return
			}
			resp.SettlementAccount = newBaofuSettlementReadinessResponse(readiness)
		} else if !isNotFoundError(opErr) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, opErr))
			return
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
		server.writeOperatorApplicationResponse(ctx, http.StatusOK, app, regionName)
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
	_, err = server.getRegionActiveOperator(ctx, req.RegionID)
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

	server.writeOperatorApplicationResponse(ctx, http.StatusOK, updatedApp, region.Name)
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
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, updatedApp, regionName)
}

func (server *Server) deleteOperatorApplicationDocumentByType(ctx *gin.Context, documentType operatorApplicationDocumentType) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch documentType {
	case operatorApplicationDocumentBusinessLicense, operatorApplicationDocumentIDCardFront, operatorApplicationDocumentIDCardBack:
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid document type")))
		return
	}

	app, err := server.store.GetOperatorApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status == "rejected" {
		app, err = server.store.ResetOperatorApplicationToDraft(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	var (
		updatedApp db.OperatorApplication
		assetID    int64
	)

	switch documentType {
	case operatorApplicationDocumentBusinessLicense:
		if app.BusinessLicenseMediaAssetID.Valid {
			assetID = app.BusinessLicenseMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearOperatorApplicationBusinessLicense(ctx, app.ID)
	case operatorApplicationDocumentIDCardFront:
		if app.IDCardFrontMediaAssetID.Valid {
			assetID = app.IDCardFrontMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearOperatorApplicationIDCardFront(ctx, app.ID)
	default:
		if app.IDCardBackMediaAssetID.Valid {
			assetID = app.IDCardBackMediaAssetID.Int64
		}
		updatedApp, err = server.store.ClearOperatorApplicationIDCardBack(ctx, app.ID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if assetID > 0 {
		if err := server.mediaRegistry.SoftDelete(ctx, assetID, authPayload.UserID); err != nil {
			log.Warn().Err(err).Int64("asset_id", assetID).Str("document_type", string(documentType)).Msg("delete operator application document: soft delete media failed")
		}
	}

	regionName := server.getRegionName(ctx, updatedApp.RegionID)
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, updatedApp, regionName)
}

// deleteOperatorApplicationDocument godoc
// @Summary 删除运营商申请证照
// @Description 删除运营商草稿中的单个证照绑定，并清空对应 OCR 结果。支持证照类型：business_license、id_card_front、id_card_back。
// @Tags 运营商申请
// @Produce json
// @Param document_type path string true "证照类型: business_license|id_card_front|id_card_back"
// @Success 200 {object} operatorApplicationResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/operator/application/documents/{document_type} [delete]
// @Security BearerAuth
func (server *Server) deleteOperatorApplicationDocument(ctx *gin.Context) {
	documentType := operatorApplicationDocumentType(strings.TrimSpace(ctx.Param("document_type")))
	server.deleteOperatorApplicationDocumentByType(ctx, documentType)
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
	_, err = server.getRegionActiveOperator(ctx, app.RegionID)
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
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, submittedApp, regionName)
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
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, resetApp, regionName)
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
