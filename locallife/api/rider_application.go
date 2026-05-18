package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

// ==================== 骑手申请数据结构 ====================

func buildRiderApplicationMissingFieldsError(missingFields []string) error {
	return &APIError{
		Code:    40105,
		Message: fmt.Sprintf("请先补充以下资料后再提交：%s", joinStrings(missingFields, "、")),
	}
}

// riderApplicationResponse 骑手申请响应
type riderApplicationResponse struct {
	ID                 int64                             `json:"id"`
	UserID             int64                             `json:"user_id"`
	RealName           *string                           `json:"real_name,omitempty"`
	Phone              *string                           `json:"phone,omitempty"`
	IDCardFrontAssetID *int64                            `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID  *int64                            `json:"id_card_back_asset_id,omitempty"`
	IDCardOCR          *IDCardOCRData                    `json:"id_card_ocr,omitempty"`
	HealthCertAssetID  *int64                            `json:"health_cert_asset_id,omitempty"`
	HealthCertOCR      *HealthCertOCRData                `json:"health_cert_ocr,omitempty"`
	Status             string                            `json:"status"`
	RejectReason       *string                           `json:"reject_reason,omitempty"`
	ReviewSummary      *onboardingReviewSummaryResponse  `json:"review_summary,omitempty"`
	ActiveCredentials  []activeCredentialSummaryResponse `json:"active_credentials,omitempty"`
	CreatedAt          time.Time                         `json:"created_at"`
	UpdatedAt          *time.Time                        `json:"updated_at,omitempty"`
	SubmittedAt        *time.Time                        `json:"submitted_at,omitempty"`
}

type riderApplicationDocumentType string

const (
	riderApplicationDocumentIDCardFront riderApplicationDocumentType = "id_card_front"
	riderApplicationDocumentIDCardBack  riderApplicationDocumentType = "id_card_back"
	riderApplicationDocumentHealthCert  riderApplicationDocumentType = "health_cert"
)

func (server *Server) newRiderApplicationResponse(ctx context.Context, app db.RiderApplication) riderApplicationResponse {
	resp := riderApplicationResponse{
		ID:                app.ID,
		UserID:            app.UserID,
		Status:            app.Status,
		CreatedAt:         app.CreatedAt,
		ReviewSummary:     decodeOnboardingReviewSummary(app.ReviewSummary),
		ActiveCredentials: server.loadRiderActiveCredentialSummaries(ctx, app.UserID),
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
		ocrData, err := decodeIDCardOCRData(app.IDCardOcr)
		if err == nil {
			resp.IDCardOCR = ocrData
		}
	}

	// 解析健康证OCR数据
	if len(app.HealthCertOcr) > 0 {
		ocrData, err := decodeHealthCertOCRData(app.HealthCertOcr)
		if err == nil {
			resp.HealthCertOCR = ocrData
		}
	}

	return resp
}

func (server *Server) ensureEditableRiderApplication(app db.RiderApplication) (db.RiderApplication, error) {
	if app.Status == "draft" {
		return app, nil
	}

	return app, ErrApplicationNotDraft
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
		ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, app))
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

	ctx.JSON(http.StatusCreated, server.newRiderApplicationResponse(ctx, app))
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

	app, err = server.ensureEditableRiderApplication(app)
	if err != nil {
		if errors.Is(err, ErrApplicationNotDraft) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("ensure rider application editable: %w", err)))
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

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, updated))
}

func (server *Server) deleteRiderApplicationDocumentByType(ctx *gin.Context, documentType riderApplicationDocumentType) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch documentType {
	case riderApplicationDocumentIDCardFront, riderApplicationDocumentIDCardBack, riderApplicationDocumentHealthCert:
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid document type")))
		return
	}

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	app, err = server.ensureEditableRiderApplication(app)
	if err != nil {
		if errors.Is(err, ErrApplicationNotDraft) {
			log.Warn().
				Int64("application_id", app.ID).
				Int64("user_id", authPayload.UserID).
				Str("status", app.Status).
				Str("document_type", string(documentType)).
				Bool("front_asset_bound", app.IDCardFrontMediaAssetID.Valid).
				Bool("back_asset_bound", app.IDCardBackMediaAssetID.Valid).
				Bool("health_asset_bound", app.HealthCertMediaAssetID.Valid).
				Int64("front_asset_id", app.IDCardFrontMediaAssetID.Int64).
				Int64("back_asset_id", app.IDCardBackMediaAssetID.Int64).
				Int64("health_asset_id", app.HealthCertMediaAssetID.Int64).
				Msg("reject deleting rider application document because application is not draft")
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("ensure rider application editable: %w", err)))
		return
	}

	if app.Status != "draft" {
		log.Warn().
			Int64("application_id", app.ID).
			Int64("user_id", authPayload.UserID).
			Str("status", app.Status).
			Str("document_type", string(documentType)).
			Bool("front_asset_bound", app.IDCardFrontMediaAssetID.Valid).
			Bool("back_asset_bound", app.IDCardBackMediaAssetID.Valid).
			Bool("health_asset_bound", app.HealthCertMediaAssetID.Valid).
			Int64("front_asset_id", app.IDCardFrontMediaAssetID.Int64).
			Int64("back_asset_id", app.IDCardBackMediaAssetID.Int64).
			Int64("health_asset_id", app.HealthCertMediaAssetID.Int64).
			Msg("reject deleting rider application document because application is not draft")
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	var (
		updated db.RiderApplication
		assetID int64
	)

	switch documentType {
	case riderApplicationDocumentIDCardFront:
		if app.IDCardFrontMediaAssetID.Valid {
			assetID = app.IDCardFrontMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationIDCardFront(ctx, app.ID)
	case riderApplicationDocumentIDCardBack:
		if app.IDCardBackMediaAssetID.Valid {
			assetID = app.IDCardBackMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationIDCardBack(ctx, app.ID)
	default:
		if app.HealthCertMediaAssetID.Valid {
			assetID = app.HealthCertMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationHealthCert(ctx, app.ID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("clear rider application document: %w", err)))
		return
	}

	log.Info().
		Int64("application_id", updated.ID).
		Int64("user_id", authPayload.UserID).
		Str("document_type", string(documentType)).
		Int64("deleted_asset_id", assetID).
		Bool("front_asset_bound", updated.IDCardFrontMediaAssetID.Valid).
		Bool("back_asset_bound", updated.IDCardBackMediaAssetID.Valid).
		Bool("health_asset_bound", updated.HealthCertMediaAssetID.Valid).
		Int64("front_asset_id", updated.IDCardFrontMediaAssetID.Int64).
		Int64("back_asset_id", updated.IDCardBackMediaAssetID.Int64).
		Int64("health_asset_id", updated.HealthCertMediaAssetID.Int64).
		Msg("rider application document cleared")

	if assetID > 0 {
		if err := server.mediaRegistry.SoftDelete(ctx, assetID, authPayload.UserID); err != nil {
			log.Warn().Err(err).Int64("asset_id", assetID).Str("document_type", string(documentType)).Msg("delete rider application document: soft delete media failed")
		}
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, updated))
}

// deleteRiderApplicationDocument godoc
// @Summary 删除骑手申请证照
// @Description 删除骑手草稿中的单个证照绑定，并清空对应 OCR 结果。支持证照类型：id_card_front、id_card_back、health_cert。
// @Tags 骑手申请
// @Produce json
// @Param document_type path string true "证照类型: id_card_front|id_card_back|health_cert"
// @Success 200 {object} riderApplicationResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/documents/{document_type} [delete]
// @Security BearerAuth
func (server *Server) deleteRiderApplicationDocument(ctx *gin.Context) {
	documentType := riderApplicationDocumentType(strings.TrimSpace(ctx.Param("document_type")))
	server.deleteRiderApplicationDocumentByType(ctx, documentType)
}

// deleteRiderApplicationHealthCert godoc
// @Summary 删除骑手申请健康证
// @Description 删除骑手草稿中的健康证绑定，并清空对应 OCR 结果。
// @Tags 骑手申请
// @Produce json
// @Success 200 {object} riderApplicationResponse "删除成功"
// @Failure 400 {object} ErrorResponse "状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/health-cert [delete]
// @Security BearerAuth
func (server *Server) deleteRiderApplicationHealthCert(ctx *gin.Context) {
	server.deleteRiderApplicationDocumentByType(ctx, riderApplicationDocumentHealthCert)
}

// ==================== 提交申请 ====================

// submitRiderApplication godoc
// @Summary 提交骑手申请
// @Description 提交申请进行自动审核。条件：身份证在有效期内，且健康证姓名与身份证一致并且有效期超过当前日期7天则通过，否则退回草稿并保留失败原因
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "审核结果（approved或draft）"
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
		ctx.JSON(http.StatusBadRequest, errorResponse(buildRiderApplicationMissingFieldsError(missingFields)))
		return
	}

	idCardOCR, err := validateRiderApplicationSubmissionReadiness(app)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	submitted, err := server.store.SubmitRiderApplication(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("submit rider application: %w", err)))
		return
	}

	reviewExecutor := logic.NewRiderOnboardingReviewService(server.store, server.onboardingReviewService, server.credentialGovernanceService)
	var queuedRun *db.OnboardingReviewRun
	if server.onboardingReviewService != nil && server.taskDistributor != nil {
		run, err := server.onboardingReviewService.CreateRiderReviewRun(ctx, submitted.ID, logic.OnboardingReviewDecision{
			RequestedBy: &authPayload.UserID,
			OCRJobRefs:  riderApplicationOCRJobRefs(submitted, idCardOCR),
			Snapshot: map[string]any{
				"application_id":   submitted.ID,
				"application_type": "rider",
				"status":           submitted.Status,
				"user_id":          submitted.UserID,
			},
		})
		if err != nil {
			log.Error().Err(err).Int64("application_id", submitted.ID).Msg("create rider onboarding review run failed, fallback to sync review")
		} else {
			queuedRun = &run
			err = server.taskDistributor.DistributeTaskOnboardingReview(ctx, &worker.OnboardingReviewPayload{
				ReviewRunID:     run.ID,
				ApplicationID:   submitted.ID,
				ApplicationType: "rider",
				RequestedBy:     authPayload.UserID,
			})
			if err == nil {
				attachRiderReviewSummary(&submitted, queuedRun)
				ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, submitted))
				return
			}
			log.Error().Err(err).Int64("application_id", submitted.ID).Int64("review_run_id", run.ID).Msg("enqueue rider onboarding review failed, fallback to sync review")
		}
	}

	result, err := reviewExecutor.ProcessSubmittedApplication(ctx, submitted, authPayload.UserID, onboardingReviewRunID(queuedRun))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if result.RestoreReleased && result.Rider != nil {
		server.notifyCredentialGovernanceRestored(ctx, "rider", result.Rider.ID, result.Application.ID, result.ReviewRun, result.CredentialEntries)
	}

	if server.config.RulesEngineEnabled && server.rulesEngine != nil {
		ruleInput := rules.Context{
			Domain: rules.DomainClaim,
			UserID: authPayload.UserID,
			Metadata: map[string]interface{}{
				"domain":               "rider_application",
				"health_cert_uploaded": app.HealthCertMediaAssetID.Valid,
				"idcard_ocr_valid":     len(app.IDCardOcr) > 0,
				"health_ocr_valid":     len(app.HealthCertOcr) > 0,
				"idcard_not_expired":   result.Approved || result.RejectReason != "身份证已过期，请更换有效身份证后重新申请",
				"name_match":           result.Approved || result.RejectReason != "健康证姓名与身份证姓名不一致",
			},
		}
		decision, err := server.rulesEngine.Evaluate(ctx, ruleInput)
		if err == nil {
			server.recordRuleHit(ctx, ruleInput, decision, RoleRider)
		}
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, result.Application))
}

func validateRiderApplicationSubmissionReadiness(app db.RiderApplication) (*IDCardOCRData, error) {
	if len(app.IDCardOcr) == 0 {
		return nil, errors.New("身份证信息未识别，请重新上传清晰的身份证照片")
	}

	decodedIDCardOCR, err := decodeIDCardOCRData(app.IDCardOcr)
	if err != nil || decodedIDCardOCR == nil {
		return nil, errors.New("身份证信息解析失败，请重新上传")
	}
	switch strings.TrimSpace(decodedIDCardOCR.Status) {
	case string(ocr.JobStatusPending), string(ocr.JobStatusProcessing):
		return nil, errors.New("身份证OCR处理中，请稍后再提交")
	case string(ocr.JobStatusFailed):
		return nil, errors.New("身份证OCR处理失败，请重新上传清晰的身份证照片")
	}
	if err := submissionReadinessError(
		decodedIDCardOCR.Readiness,
		map[string]string{
			"name":      "身份证姓名未识别，请重新上传清晰的身份证正面照片",
			"id_number": "身份证号未识别，请重新上传清晰的身份证正面照片",
			"valid_end": "身份证有效期未识别，请上传身份证背面照片",
		},
		"身份证信息未识别，请重新上传清晰的身份证照片",
		"身份证信息解析失败，请重新上传",
		"身份证OCR处理失败，请重新上传清晰的身份证照片",
	); err != nil {
		return nil, err
	}

	idName := normalizePersonName(decodedIDCardOCR.Name)
	if idName == "" && app.RealName.Valid {
		idName = normalizePersonName(app.RealName.String)
	}
	if idName == "" {
		return nil, errors.New("身份证姓名未识别，请重新上传清晰的身份证正面照片")
	}
	if strings.TrimSpace(decodedIDCardOCR.IDNumber) == "" {
		return nil, errors.New("身份证号未识别，请重新上传清晰的身份证正面照片")
	}
	if strings.TrimSpace(decodedIDCardOCR.ValidEnd) == "" {
		return nil, errors.New("身份证有效期未识别，请上传身份证背面照片")
	}

	if len(app.HealthCertOcr) == 0 {
		return nil, errors.New("健康证信息未识别，请重新上传清晰的健康证照片")
	}

	decodedHealthOCR, err := decodeHealthCertOCRData(app.HealthCertOcr)
	if err != nil || decodedHealthOCR == nil {
		return nil, errors.New("健康证信息解析失败，请重新上传")
	}
	switch strings.TrimSpace(decodedHealthOCR.Status) {
	case string(ocr.JobStatusPending), string(ocr.JobStatusProcessing):
		return nil, errors.New("健康证OCR处理中，请稍后再提交")
	case string(ocr.JobStatusFailed):
		return nil, errors.New("健康证OCR处理失败，请重新上传清晰的健康证照片")
	}
	if err := submissionReadinessError(
		decodedHealthOCR.Readiness,
		map[string]string{
			"name":      "健康证姓名未识别，请重新上传清晰的健康证照片",
			"valid_end": "健康证有效期未识别，请重新上传清晰的健康证照片",
		},
		"健康证信息未识别，请重新上传清晰的健康证照片",
		"健康证信息解析失败，请重新上传",
		"健康证OCR处理失败，请重新上传清晰的健康证照片",
	); err != nil {
		return nil, err
	}
	if normalizePersonName(decodedHealthOCR.Name) == "" {
		return nil, errors.New("健康证姓名未识别，请重新上传清晰的健康证照片")
	}
	if strings.TrimSpace(decodedHealthOCR.ValidEnd) == "" {
		return nil, errors.New("健康证有效期未识别，请重新上传清晰的健康证照片")
	}

	return decodedIDCardOCR, nil
}

// ==================== 重置申请（处理中） ====================

// resetRiderApplication godoc
// @Summary 重置骑手申请
// @Description 将待处理申请重置为草稿状态并清空审核痕迹
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

	if app.Status != db.RiderApplicationStatusSubmitted {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationCannotReset))
		return
	}

	reset, err := server.store.ResetRiderApplicationToDraft(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reset rider application: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, reset))
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
