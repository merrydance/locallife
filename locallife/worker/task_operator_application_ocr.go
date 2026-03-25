package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/rs/zerolog/log"
)

const (
	TaskOperatorApplicationBusinessLicenseOCR = "operator_application:ocr_business_license"
	TaskOperatorApplicationIDCardOCR          = "operator_application:ocr_id_card"
)

type operatorApplicationOCRPayload struct {
	ApplicationID int64  `json:"application_id"`
	MediaAssetID  int64  `json:"media_asset_id,omitempty"`
	OCRJobID      int64  `json:"ocr_job_id,omitempty"`
	Side          string `json:"side,omitempty"`
}

type operatorIDCardFrontOCRData struct {
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

type operatorIDCardBackOCRData struct {
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

func (distributor *RedisTaskDistributor) DistributeTaskOperatorApplicationBusinessLicenseOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(operatorApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskOperatorApplicationBusinessLicenseOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (processor *RedisTaskProcessor) ProcessTaskOperatorApplicationBusinessLicenseOCR(ctx context.Context, task *asynq.Task) error {
	var payload operatorApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || payload.OCRJobID <= 0 {
		return fmt.Errorf("invalid operator business license payload: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for operator business license: %w", asynq.SkipRetry)
	}
	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:operator_business_license",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		failed := map[string]any{
			"status":           string(ocr.JobStatusFailed),
			"queued_at":        job.CreatedAt.Format(time.RFC3339),
			"started_at":       formatPgTimestamp(job.StartedAt),
			"ocr_job_id":       payload.OCRJobID,
			"error_code":       ocr.ErrorCode(err),
			"alert_emitted_at": formatOCRAlertEmittedAt(alertEmittedAt),
			"error":            err.Error(),
		}
		failedJSON, _ := json.Marshal(failed)
		_, _ = processor.store.UpdateOperatorApplicationBusinessLicense(ctx, db.UpdateOperatorApplicationBusinessLicenseParams{
			ID:                 payload.ApplicationID,
			BusinessLicenseOcr: failedJSON,
		})
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)
	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized operator business license result: %w", decodeErr)
	}
	ocrData := map[string]any{
		"status":     "done",
		"queued_at":  job.CreatedAt.Format(time.RFC3339),
		"started_at": formatPgTimestamp(job.StartedAt),
		"ocr_job_id": job.ID,
		"ocr_at":     normalized.RecognizedAt.Format(time.RFC3339),
	}
	arg := db.UpdateOperatorApplicationBusinessLicenseParams{ID: payload.ApplicationID}
	if normalized.BusinessLicense != nil {
		validPeriod := normalizeValidPeriod(normalized.BusinessLicense.ValidPeriod)
		ocrData["reg_num"] = normalized.BusinessLicense.RegistrationNumber
		ocrData["enterprise_name"] = normalized.BusinessLicense.EnterpriseName
		ocrData["legal_representative"] = normalized.BusinessLicense.LegalRepresentative
		ocrData["address"] = normalized.BusinessLicense.Address
		ocrData["business_scope"] = normalized.BusinessLicense.BusinessScope
		ocrData["valid_period"] = validPeriod
		ocrData["credit_code"] = normalized.BusinessLicense.CreditCode
		if normalized.BusinessLicense.CreditCode != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: normalized.BusinessLicense.CreditCode, Valid: true}
		} else if normalized.BusinessLicense.RegistrationNumber != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: normalized.BusinessLicense.RegistrationNumber, Valid: true}
		}
		if normalized.BusinessLicense.EnterpriseName != "" {
			arg.Name = pgtype.Text{String: normalized.BusinessLicense.EnterpriseName, Valid: true}
		}
	}
	ocrJSON, _ := json.Marshal(ocrData)
	arg.BusinessLicenseOcr = ocrJSON
	_, err = processor.store.UpdateOperatorApplicationBusinessLicense(ctx, arg)
	if err != nil {
		return fmt.Errorf("update operator application business license: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Msg("✅ operator business license OCR updated from ocr job")
	return nil
}

func (distributor *RedisTaskDistributor) DistributeTaskOperatorApplicationIDCardOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	side string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(operatorApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
		Side:          side,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskOperatorApplicationIDCardOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (processor *RedisTaskProcessor) ProcessTaskOperatorApplicationIDCardOCR(ctx context.Context, task *asynq.Task) error {
	var payload operatorApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || payload.OCRJobID <= 0 || (payload.Side != "Front" && payload.Side != "Back") {
		return fmt.Errorf("invalid operator id card payload: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for operator id card: %w", asynq.SkipRetry)
	}
	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:operator_id_card",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		if payload.Side == "Front" {
			failed := map[string]any{"status": string(ocr.JobStatusFailed), "queued_at": job.CreatedAt.Format(time.RFC3339), "started_at": formatPgTimestamp(job.StartedAt), "ocr_job_id": payload.OCRJobID, "error": err.Error(), "error_code": ocr.ErrorCode(err), "alert_emitted_at": formatOCRAlertEmittedAt(alertEmittedAt)}
			failedJSON, _ := json.Marshal(failed)
			_, _ = processor.store.UpdateOperatorApplicationIDCardFront(ctx, db.UpdateOperatorApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: failedJSON})
		} else {
			failed := map[string]any{"status": string(ocr.JobStatusFailed), "queued_at": job.CreatedAt.Format(time.RFC3339), "started_at": formatPgTimestamp(job.StartedAt), "ocr_job_id": payload.OCRJobID, "error": err.Error(), "error_code": ocr.ErrorCode(err), "alert_emitted_at": formatOCRAlertEmittedAt(alertEmittedAt)}
			failedJSON, _ := json.Marshal(failed)
			_, _ = processor.store.UpdateOperatorApplicationIDCardBack(ctx, db.UpdateOperatorApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: failedJSON})
		}
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)
	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized operator id card result: %w", decodeErr)
	}
	if payload.Side == "Front" {
		ocrData := operatorIDCardFrontOCRData{Status: "done", QueuedAt: job.CreatedAt.Format(time.RFC3339), StartedAt: formatPgTimestamp(job.StartedAt), OCRJobID: int64Ptr(job.ID), OCRAt: normalized.RecognizedAt.Format(time.RFC3339)}
		arg := db.UpdateOperatorApplicationIDCardFrontParams{ID: payload.ApplicationID}
		if normalized.IDCard != nil {
			ocrData.Name = normalized.IDCard.Name
			ocrData.IDNumber = normalized.IDCard.IDNumber
			ocrData.Gender = normalized.IDCard.Gender
			ocrData.Nation = normalized.IDCard.Ethnicity
			ocrData.Address = normalized.IDCard.Address
			if normalized.IDCard.Name != "" {
				arg.LegalPersonName = pgtype.Text{String: normalized.IDCard.Name, Valid: true}
			}
			if normalized.IDCard.IDNumber != "" {
				arg.LegalPersonIDNumber = pgtype.Text{String: normalized.IDCard.IDNumber, Valid: true}
			}
		}
		ocrJSON, _ := json.Marshal(ocrData)
		arg.IDCardFrontOcr = ocrJSON
		_, err = processor.store.UpdateOperatorApplicationIDCardFront(ctx, arg)
		if err != nil {
			return fmt.Errorf("update operator application id card front: %w", err)
		}
	} else {
		ocrData := operatorIDCardBackOCRData{Status: "done", QueuedAt: job.CreatedAt.Format(time.RFC3339), StartedAt: formatPgTimestamp(job.StartedAt), OCRJobID: int64Ptr(job.ID), OCRAt: normalized.RecognizedAt.Format(time.RFC3339)}
		if normalized.IDCard != nil {
			ocrData.ValidEnd = normalized.IDCard.ValidPeriod
		}
		ocrJSON, _ := json.Marshal(ocrData)
		_, err = processor.store.UpdateOperatorApplicationIDCardBack(ctx, db.UpdateOperatorApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: ocrJSON})
		if err != nil {
			return fmt.Errorf("update operator application id card back: %w", err)
		}
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Str("side", payload.Side).Msg("✅ operator id card OCR updated from ocr job")
	return nil
}
