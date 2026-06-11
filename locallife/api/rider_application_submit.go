package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

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

	if app.Status != db.RiderApplicationStatusDraft {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationSubmitDraft))
		return
	}

	server.writeAgreementConsentAudit(ctx, authPayload.UserID, "rider_application_consent_confirmed", "rider_application", app.ID, consentReq, nil)

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
