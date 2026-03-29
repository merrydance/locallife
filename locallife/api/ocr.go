package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

type createOCRJobRequest struct {
	DocumentType   string `json:"document_type" binding:"required"`
	MediaAssetID   int64  `json:"media_asset_id" binding:"required,min=1"`
	OwnerType      string `json:"owner_type" binding:"required"`
	OwnerID        int64  `json:"owner_id" binding:"required,min=1"`
	Side           string `json:"side,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type batchQueryOCRJobsRequest struct {
	JobIDs []int64 `json:"job_ids" binding:"required,min=1,max=50,dive,min=1"`
}

type ocrJobResponse struct {
	OCRJobID      int64      `json:"ocr_job_id"`
	Status        string     `json:"status"`
	DocumentType  string     `json:"document_type"`
	Provider      string     `json:"provider"`
	OwnerType     string     `json:"owner_type"`
	OwnerID       int64      `json:"owner_id"`
	Side          string     `json:"side,omitempty"`
	ErrorCode     *string    `json:"error_code,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	ResultVersion int32      `json:"result_version,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type ocrJobResultResponse struct {
	OCRJobID         int64  `json:"ocr_job_id"`
	Status           string `json:"status"`
	ResultVersion    int32  `json:"result_version"`
	NormalizedResult any    `json:"normalized_result,omitempty"`
}

type batchQueryOCRJobsResponse struct {
	Jobs []ocrJobResponse `json:"jobs"`
}

type ocrDeadLetterJobResponse struct {
	OCRJobID       int64      `json:"ocr_job_id"`
	Status         string     `json:"status"`
	DocumentType   string     `json:"document_type"`
	Provider       string     `json:"provider"`
	OwnerType      string     `json:"owner_type"`
	OwnerID        int64      `json:"owner_id"`
	Side           string     `json:"side,omitempty"`
	ErrorCode      *string    `json:"error_code,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	AttemptCount   int32      `json:"attempt_count"`
	MaxAttempts    int32      `json:"max_attempts"`
	RequestedBy    int64      `json:"requested_by"`
	ManualReason   string     `json:"manual_reason"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
}

type listOCRDeadLetterJobsResponse struct {
	Jobs []ocrDeadLetterJobResponse `json:"jobs"`
}

func newOCRJobResponse(job db.OcrJob) ocrJobResponse {
	return ocrJobResponse{
		OCRJobID:      job.ID,
		Status:        job.Status,
		DocumentType:  job.DocumentType,
		Provider:      job.Provider,
		OwnerType:     job.OwnerType,
		OwnerID:       job.OwnerID,
		Side:          job.Side,
		ErrorCode:     pgTextToPtr(job.ErrorCode),
		ErrorMessage:  pgTextToPtr(job.ErrorMessage),
		ResultVersion: job.ResultVersion,
		CreatedAt:     job.CreatedAt,
		StartedAt:     pgTimeToPtr(job.StartedAt),
		FinishedAt:    pgTimeToPtr(job.FinishedAt),
	}
}

func ocrManualReason(job db.OcrJob) string {
	if job.Status == string(ocr.JobStatusCancelled) {
		return "cancelled"
	}
	if job.MaxAttempts > 0 && job.AttemptCount >= job.MaxAttempts {
		return "attempts_exhausted"
	}
	if !job.ErrorCode.Valid {
		return "permanent_error"
	}
	switch job.ErrorCode.String {
	case "ocr_provider_unauthorized", "ocr_provider_forbidden":
		return "provider_permission_error"
	case "ocr_bad_request":
		return "bad_request"
	case "ocr_media_not_found":
		return "media_missing"
	default:
		return "permanent_error"
	}
}

func newOCRDeadLetterJobResponse(job db.OcrJob) ocrDeadLetterJobResponse {
	return ocrDeadLetterJobResponse{
		OCRJobID:       job.ID,
		Status:         job.Status,
		DocumentType:   job.DocumentType,
		Provider:       job.Provider,
		OwnerType:      job.OwnerType,
		OwnerID:        job.OwnerID,
		Side:           job.Side,
		ErrorCode:      pgTextToPtr(job.ErrorCode),
		ErrorMessage:   pgTextToPtr(job.ErrorMessage),
		AttemptCount:   job.AttemptCount,
		MaxAttempts:    job.MaxAttempts,
		RequestedBy:    job.RequestedBy,
		ManualReason:   ocrManualReason(job),
		CreatedAt:      job.CreatedAt,
		StartedAt:      pgTimeToPtr(job.StartedAt),
		FinishedAt:     pgTimeToPtr(job.FinishedAt),
		NextRetryAt:    pgTimeToPtr(job.NextRetryAt),
		RetentionUntil: pgTimeToPtr(job.RetentionUntil),
	}
}

func parseOCRListLimit(raw string, defaultValue int32) (int32, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed <= 0 || parsed > 100 {
		return 0, errors.New("invalid limit")
	}
	return int32(parsed), nil
}

func parseOCRListOffset(raw string) (int32, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed < 0 {
		return 0, errors.New("invalid offset")
	}
	return int32(parsed), nil
}

func normalizeOCRSide(input string) (ocr.DocumentSide, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	side := ocr.DocumentSide(value)
	if err := side.Validate(); err != nil {
		return "", err
	}
	return side, nil
}

func isSupportedOCRJob(ownerType ocr.OwnerType, documentType ocr.DocumentType, side ocr.DocumentSide) bool {
	switch ownerType {
	case ocr.OwnerTypeMerchantApplication:
		if documentType == ocr.DocumentTypeBusinessLicense || documentType == ocr.DocumentTypeFoodPermit {
			return side == ocr.DocumentSideUnknown
		}
		return documentType == ocr.DocumentTypeIDCard && (side == ocr.DocumentSideFront || side == ocr.DocumentSideBack)
	case ocr.OwnerTypeOperatorApplication:
		if documentType == ocr.DocumentTypeBusinessLicense {
			return side == ocr.DocumentSideUnknown
		}
		return documentType == ocr.DocumentTypeIDCard && (side == ocr.DocumentSideFront || side == ocr.DocumentSideBack)
	case ocr.OwnerTypeRiderApplication:
		if documentType == ocr.DocumentTypeHealthCert {
			return side == ocr.DocumentSideUnknown
		}
		return documentType == ocr.DocumentTypeIDCard && (side == ocr.DocumentSideFront || side == ocr.DocumentSideBack)
	case ocr.OwnerTypeGroupApplication:
		return documentType == ocr.DocumentTypeBusinessLicense && side == ocr.DocumentSideUnknown
	default:
		return false
	}
}

func (server *Server) isOCRAdmin(ctx *gin.Context, userID int64) (bool, error) {
	roles, err := server.store.ListUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.Role == RoleAdmin && role.Status == "active" {
			return true, nil
		}
	}
	return false, nil
}

func (server *Server) isOCROwner(ctx *gin.Context, ownerType ocr.OwnerType, ownerID int64, userID int64) (bool, error) {
	switch ownerType {
	case ocr.OwnerTypeMerchantApplication:
		app, err := server.store.GetMerchantApplication(ctx, ownerID)
		if err != nil {
			return false, err
		}
		return app.UserID == userID, nil
	case ocr.OwnerTypeOperatorApplication:
		app, err := server.store.GetOperatorApplicationByID(ctx, ownerID)
		if err != nil {
			return false, err
		}
		return app.UserID == userID, nil
	case ocr.OwnerTypeRiderApplication:
		app, err := server.store.GetRiderApplication(ctx, ownerID)
		if err != nil {
			return false, err
		}
		return app.UserID == userID, nil
	case ocr.OwnerTypeGroupApplication:
		app, err := server.store.GetGroupApplication(ctx, ownerID)
		if err != nil {
			return false, err
		}
		return app.ApplicantUserID == userID, nil
	default:
		return false, fmt.Errorf("unsupported ocr owner type: %s", ownerType)
	}
}

func (server *Server) canAccessOCRJob(ctx *gin.Context, authPayload *token.Payload, job db.OcrJob) (bool, error) {
	if job.RequestedBy == authPayload.UserID {
		return true, nil
	}
	owned, err := server.isOCROwner(ctx, ocr.OwnerType(job.OwnerType), job.OwnerID, authPayload.UserID)
	if err == nil && owned {
		return true, nil
	}
	if err != nil && !isNotFoundError(err) {
		return false, err
	}
	return server.isOCRAdmin(ctx, authPayload.UserID)
}

func (server *Server) enqueueOCRJob(ctx *gin.Context, job db.OcrJob) error {
	ownerType := ocr.OwnerType(job.OwnerType)
	documentType := ocr.DocumentType(job.DocumentType)
	side := strings.ToLower(strings.TrimSpace(job.Side))
	switch ownerType {
	case ocr.OwnerTypeMerchantApplication:
		switch documentType {
		case ocr.DocumentTypeBusinessLicense:
			return server.taskDistributor.DistributeTaskMerchantApplicationBusinessLicenseOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID)
		case ocr.DocumentTypeFoodPermit:
			return server.taskDistributor.DistributeTaskMerchantApplicationFoodPermitOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID)
		case ocr.DocumentTypeIDCard:
			return server.taskDistributor.DistributeTaskMerchantApplicationIDCardOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID, strings.Title(side))
		}
	case ocr.OwnerTypeOperatorApplication:
		switch documentType {
		case ocr.DocumentTypeBusinessLicense:
			return server.taskDistributor.DistributeTaskOperatorApplicationBusinessLicenseOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID)
		case ocr.DocumentTypeIDCard:
			return server.taskDistributor.DistributeTaskOperatorApplicationIDCardOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID, strings.Title(side))
		}
	case ocr.OwnerTypeRiderApplication:
		switch documentType {
		case ocr.DocumentTypeIDCard:
			return server.taskDistributor.DistributeTaskRiderApplicationIDCardOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID, strings.Title(side))
		case ocr.DocumentTypeHealthCert:
			return server.taskDistributor.DistributeTaskRiderApplicationHealthCertOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID)
		}
	case ocr.OwnerTypeGroupApplication:
		if documentType == ocr.DocumentTypeBusinessLicense {
			return server.taskDistributor.DistributeTaskGroupApplicationBusinessLicenseOCR(ctx, job.OwnerID, job.MediaAssetID, job.ID)
		}
	}
	return fmt.Errorf("unsupported ocr job dispatch: owner_type=%s document_type=%s side=%s", job.OwnerType, job.DocumentType, job.Side)
}

func (server *Server) markMerchantApplicationOCRPending(ctx *gin.Context, job db.OcrJob) error {
	app, err := server.store.GetMerchantApplication(ctx, job.OwnerID)
	if err != nil {
		return err
	}

	editable, needReset, errMsg := checkApplicationEditable(app.Status)
	if !editable {
		return errors.New(errMsg)
	}
	if needReset {
		resetResult, resetErr := server.store.ResetMerchantApplicationTx(ctx, db.ResetMerchantApplicationTxParams{
			ApplicationID: app.ID,
			UserID:        app.UserID,
		})
		if resetErr != nil {
			return resetErr
		}
		app = resetResult.Application
	}

	queuedAt := job.CreatedAt.Format(time.RFC3339)
	ocrJobID := job.ID

	switch ocr.DocumentType(job.DocumentType) {
	case ocr.DocumentTypeBusinessLicense:
		payload, marshalErr := json.Marshal(BusinessLicenseOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = server.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                          app.ID,
			BusinessLicenseMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			BusinessLicenseOcr:          payload,
		})
		return err
	case ocr.DocumentTypeFoodPermit:
		payload, marshalErr := json.Marshal(FoodPermitOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = server.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
			ID:                     app.ID,
			FoodPermitMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			FoodPermitOcr:          payload,
		})
		return err
	case ocr.DocumentTypeIDCard:
		payload, marshalErr := json.Marshal(MerchantIDCardOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		if strings.EqualFold(job.Side, string(ocr.DocumentSideBack)) {
			_, err = server.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{
				ID:                     app.ID,
				IDCardBackMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
				IDCardBackOcr:          payload,
			})
			return err
		}
		_, err = server.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{
			ID:                      app.ID,
			IDCardFrontMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			IDCardFrontOcr:          payload,
		})
		return err
	default:
		return nil
	}
}

func (server *Server) markOperatorApplicationOCRPending(ctx *gin.Context, job db.OcrJob) error {
	app, err := server.store.GetOperatorApplicationByID(ctx, job.OwnerID)
	if err != nil {
		return err
	}
	if app.Status != "draft" {
		return nil
	}

	queuedAt := job.CreatedAt.Format(time.RFC3339)
	ocrJobID := job.ID

	switch ocr.DocumentType(job.DocumentType) {
	case ocr.DocumentTypeBusinessLicense:
		payload, marshalErr := json.Marshal(BusinessLicenseOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = server.store.UpdateOperatorApplicationBusinessLicense(ctx, db.UpdateOperatorApplicationBusinessLicenseParams{
			ID:                          app.ID,
			BusinessLicenseMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			BusinessLicenseOcr:          payload,
		})
		return err
	case ocr.DocumentTypeIDCard:
		if strings.EqualFold(job.Side, string(ocr.DocumentSideBack)) {
			payload, marshalErr := json.Marshal(OperatorIDCardBackOCR{
				Status:   "pending",
				QueuedAt: queuedAt,
				OCRJobID: &ocrJobID,
			})
			if marshalErr != nil {
				return marshalErr
			}
			_, err = server.store.UpdateOperatorApplicationIDCardBack(ctx, db.UpdateOperatorApplicationIDCardBackParams{
				ID:                     app.ID,
				IDCardBackMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
				IDCardBackOcr:          payload,
			})
			return err
		}
		payload, marshalErr := json.Marshal(OperatorIDCardOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = server.store.UpdateOperatorApplicationIDCardFront(ctx, db.UpdateOperatorApplicationIDCardFrontParams{
			ID:                      app.ID,
			IDCardFrontMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			IDCardFrontOcr:          payload,
		})
		return err
	default:
		return nil
	}
}

func (server *Server) markRiderApplicationOCRPending(ctx *gin.Context, job db.OcrJob) error {
	app, err := server.store.GetRiderApplication(ctx, job.OwnerID)
	if err != nil {
		return err
	}
	if app.Status != "draft" {
		return nil
	}

	queuedAt := job.CreatedAt.Format(time.RFC3339)
	ocrJobID := job.ID

	switch ocr.DocumentType(job.DocumentType) {
	case ocr.DocumentTypeIDCard:
		payload, marshalErr := json.Marshal(IDCardOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		arg := db.UpdateRiderApplicationIDCardParams{
			ID:        app.ID,
			IDCardOcr: payload,
		}
		if strings.EqualFold(job.Side, string(ocr.DocumentSideBack)) {
			arg.IDCardBackMediaAssetID = pgtype.Int8{Int64: job.MediaAssetID, Valid: true}
		} else {
			arg.IDCardFrontMediaAssetID = pgtype.Int8{Int64: job.MediaAssetID, Valid: true}
		}
		_, err = server.store.UpdateRiderApplicationIDCard(ctx, arg)
		return err
	case ocr.DocumentTypeHealthCert:
		payload, marshalErr := json.Marshal(HealthCertOCRData{
			Status:   "pending",
			QueuedAt: queuedAt,
			OCRJobID: &ocrJobID,
		})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = server.store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{
			ID:                     app.ID,
			HealthCertMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
			HealthCertOcr:          payload,
		})
		return err
	default:
		return nil
	}
}

func mergeGroupApplicationData(data []byte) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &result)
	}
	return result
}

func (server *Server) markGroupApplicationOCRPending(ctx *gin.Context, job db.OcrJob) error {
	app, err := server.store.GetGroupApplication(ctx, job.OwnerID)
	if err != nil {
		return err
	}
	if app.Status != "draft" {
		return nil
	}
	if ocr.DocumentType(job.DocumentType) != ocr.DocumentTypeBusinessLicense {
		return nil
	}

	queuedAt := job.CreatedAt.Format(time.RFC3339)
	ocrJobID := job.ID
	payload, marshalErr := json.Marshal(BusinessLicenseOCRData{
		Status:   "pending",
		QueuedAt: queuedAt,
		OCRJobID: &ocrJobID,
	})
	if marshalErr != nil {
		return marshalErr
	}
	applicationData := mergeGroupApplicationData(app.ApplicationData)
	applicationData["business_license_ocr"] = payload
	merged, marshalErr := json.Marshal(applicationData)
	if marshalErr != nil {
		return marshalErr
	}
	_, err = server.store.UpdateGroupApplicationLicense(ctx, db.UpdateGroupApplicationLicenseParams{
		ID:                  app.ID,
		LicenseMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
		ApplicationData:     merged,
	})
	return err
}

func (server *Server) markOCRPending(ctx *gin.Context, job db.OcrJob) error {
	switch ocr.OwnerType(job.OwnerType) {
	case ocr.OwnerTypeMerchantApplication:
		return server.markMerchantApplicationOCRPending(ctx, job)
	case ocr.OwnerTypeOperatorApplication:
		return server.markOperatorApplicationOCRPending(ctx, job)
	case ocr.OwnerTypeRiderApplication:
		return server.markRiderApplicationOCRPending(ctx, job)
	case ocr.OwnerTypeGroupApplication:
		return server.markGroupApplicationOCRPending(ctx, job)
	default:
		return nil
	}
}

func (server *Server) validateOCRMediaAsset(ctx *gin.Context, mediaAssetID int64) error {
	asset, err := server.store.GetMediaAssetByID(ctx, mediaAssetID)
	if err != nil {
		return err
	}
	switch asset.ModerationStatus {
	case "approved":
		return nil
	case "pending":
		return ErrImageModerationPending
	default:
		return ErrImageContentSafetyFailed
	}
}

// createOCRJob godoc
// @Summary 创建统一 OCR 任务
// @Description 统一创建 OCR 任务并返回可轮询的 ocr_job_id
// @Tags OCR
// @Accept json
// @Produce json
// @Param request body createOCRJobRequest true "OCR 任务请求"
// @Success 200 {object} ocrJobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs [post]
// @Security BearerAuth
func (server *Server) createOCRJob(ctx *gin.Context) {
	var req createOCRJobRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	documentType := ocr.DocumentType(strings.TrimSpace(req.DocumentType))
	if err := documentType.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	ownerType := ocr.OwnerType(strings.TrimSpace(req.OwnerType))
	if err := ownerType.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	side, err := normalizeOCRSide(req.Side)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if !isSupportedOCRJob(ownerType, documentType, side) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("unsupported OCR owner/document combination")))
		return
	}
	owned, err := server.isOCROwner(ctx, ownerType, req.OwnerID, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("ocr owner not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !owned {
		isAdmin, roleErr := server.isOCRAdmin(ctx, authPayload.UserID)
		if roleErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, roleErr))
			return
		}
		if !isAdmin {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
			return
		}
	}
	if err := server.validateOCRMediaAsset(ctx, req.MediaAssetID); err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("media asset not found")))
			return
		}
		if apiErr := AsAPIError(err); apiErr != nil {
			status := http.StatusBadRequest
			if apiErr.Code >= 40900 && apiErr.Code < 41000 {
				status = http.StatusConflict
			}
			ctx.JSON(status, errorResponse(apiErr))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = ocr.BuildIdempotencyKey(req.MediaAssetID, documentType, ownerType, req.OwnerID, side)
	}
	retentionUntil := ocr.DefaultRetentionUntil(documentType, time.Now())
	job, err := server.store.UpsertOCRJob(ctx, db.UpsertOCRJobParams{
		IdempotencyKey: idempotencyKey,
		DocumentType:   string(documentType),
		Provider:       string(server.defaultOCRProviderName(documentType)),
		MediaAssetID:   req.MediaAssetID,
		OwnerType:      string(ownerType),
		OwnerID:        req.OwnerID,
		Side:           string(side),
		MaxAttempts:    3,
		RetentionUntil: pgTimePtrToTimestamptz(retentionUntil),
		RequestedBy:    authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if err := server.enqueueOCRJob(ctx, job); err != nil {
		log.Error().Int64("ocr_job_id", job.ID).Err(err).Msg("enqueue unified ocr job failed")
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}
	if err := server.markOCRPending(ctx, job); err != nil {
		log.Warn().Int64("ocr_job_id", job.ID).Err(err).Msg("mark unified ocr owner pending failed")
	}
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "user",
		Action:      "ocr_job_created",
		TargetType:  "ocr_job",
		TargetID:    &job.ID,
		Metadata: map[string]any{
			"ocr_job_id":      job.ID,
			"status":          job.Status,
			"idempotency_key": job.IdempotencyKey,
			"document_type":   job.DocumentType,
			"provider":        job.Provider,
			"media_asset_id":  job.MediaAssetID,
			"owner_type":      job.OwnerType,
			"owner_id":        job.OwnerID,
			"requested_by":    job.RequestedBy,
			"side":            job.Side,
		},
	})
	ctx.JSON(http.StatusOK, newOCRJobResponse(job))
}

// getOCRJob godoc
// @Summary 查询 OCR 任务状态
// @Tags OCR
// @Produce json
// @Success 200 {object} ocrJobResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs/{id} [get]
// @Security BearerAuth
func (server *Server) getOCRJob(ctx *gin.Context) {
	jobID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || jobID <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid ocr job id")))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	job, err := server.store.GetOCRJob(ctx, jobID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("ocr job not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	allowed, err := server.canAccessOCRJob(ctx, authPayload, job)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !allowed {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}
	ctx.JSON(http.StatusOK, newOCRJobResponse(job))
}

// listOCRDeadLetterJobs godoc
// @Summary 查询需要人工介入的 OCR 死信任务
// @Tags OCR
// @Produce json
// @Param owner_type query string false "业务主体类型过滤"
// @Param document_type query string false "证件类型过滤"
// @Param limit query int false "返回数量，默认 20，最大 100"
// @Param offset query int false "分页偏移量"
// @Success 200 {object} listOCRDeadLetterJobsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs/dead-letter [get]
// @Security BearerAuth
func (server *Server) listOCRDeadLetterJobs(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	isAdmin, err := server.isOCRAdmin(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !isAdmin {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}
	ownerType := strings.TrimSpace(ctx.Query("owner_type"))
	if ownerType != "" {
		if err := ocr.OwnerType(ownerType).Validate(); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}
	documentType := strings.TrimSpace(ctx.Query("document_type"))
	if documentType != "" {
		if err := ocr.DocumentType(documentType).Validate(); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}
	limit, err := parseOCRListLimit(ctx.Query("limit"), 20)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	offset, err := parseOCRListOffset(ctx.Query("offset"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	jobs, err := server.store.ListOCRDeadLetterJobs(ctx, db.ListOCRDeadLetterJobsParams{
		OwnerType:    ownerType,
		DocumentType: documentType,
		PageLimit:    limit,
		PageOffset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp := make([]ocrDeadLetterJobResponse, 0, len(jobs))
	for _, job := range jobs {
		resp = append(resp, newOCRDeadLetterJobResponse(job))
	}
	ctx.JSON(http.StatusOK, listOCRDeadLetterJobsResponse{Jobs: resp})
}

// getOCRJobResult godoc
// @Summary 查询 OCR 归一化结果
// @Tags OCR
// @Produce json
// @Success 200 {object} ocrJobResultResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs/{id}/result [get]
// @Security BearerAuth
func (server *Server) getOCRJobResult(ctx *gin.Context) {
	jobID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || jobID <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid ocr job id")))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	job, err := server.store.GetOCRJob(ctx, jobID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("ocr job not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	allowed, err := server.canAccessOCRJob(ctx, authPayload, job)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !allowed {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}
	var normalized any
	if len(job.NormalizedResult) > 0 {
		if err := json.Unmarshal(job.NormalizedResult, &normalized); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	ctx.JSON(http.StatusOK, ocrJobResultResponse{OCRJobID: job.ID, Status: job.Status, ResultVersion: job.ResultVersion, NormalizedResult: normalized})
}

// retryOCRJob godoc
// @Summary 重试失败的 OCR 任务
// @Tags OCR
// @Produce json
// @Success 200 {object} ocrJobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs/{id}/retry [post]
// @Security BearerAuth
func (server *Server) retryOCRJob(ctx *gin.Context) {
	jobID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || jobID <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid ocr job id")))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	job, err := server.store.GetOCRJob(ctx, jobID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("ocr job not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	allowed, err := server.canAccessOCRJob(ctx, authPayload, job)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !allowed {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}
	if job.Status != string(ocr.JobStatusFailed) && job.Status != string(ocr.JobStatusCancelled) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only failed or cancelled OCR jobs can be retried")))
		return
	}
	if job.AttemptCount >= job.MaxAttempts {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("OCR job reached max retry attempts")))
		return
	}
	newKey := fmt.Sprintf("%s:retry:%d", job.IdempotencyKey, time.Now().UnixNano())
	retentionUntil := ocr.DefaultRetentionUntil(ocr.DocumentType(job.DocumentType), time.Now())
	retried, err := server.store.UpsertOCRJob(ctx, db.UpsertOCRJobParams{
		IdempotencyKey: newKey,
		DocumentType:   job.DocumentType,
		Provider:       job.Provider,
		MediaAssetID:   job.MediaAssetID,
		OwnerType:      job.OwnerType,
		OwnerID:        job.OwnerID,
		Side:           job.Side,
		MaxAttempts:    job.MaxAttempts,
		RetentionUntil: pgTimePtrToTimestamptz(retentionUntil),
		RequestedBy:    authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if err := server.enqueueOCRJob(ctx, retried); err != nil {
		log.Error().Int64("ocr_job_id", retried.ID).Err(err).Msg("enqueue retried unified ocr job failed")
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, newOCRJobResponse(retried))
}

// batchQueryOCRJobs godoc
// @Summary 批量查询 OCR 任务状态
// @Tags OCR
// @Accept json
// @Produce json
// @Param request body batchQueryOCRJobsRequest true "批量查询请求"
// @Success 200 {object} batchQueryOCRJobsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/ocr/jobs/batch-query [post]
// @Security BearerAuth
func (server *Server) batchQueryOCRJobs(ctx *gin.Context) {
	var req batchQueryOCRJobsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	jobs := make([]ocrJobResponse, 0, len(req.JobIDs))
	for _, jobID := range req.JobIDs {
		job, err := server.store.GetOCRJob(ctx, jobID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("ocr job not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		allowed, err := server.canAccessOCRJob(ctx, authPayload, job)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if !allowed {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
			return
		}
		jobs = append(jobs, newOCRJobResponse(job))
	}
	ctx.JSON(http.StatusOK, batchQueryOCRJobsResponse{Jobs: jobs})
}
