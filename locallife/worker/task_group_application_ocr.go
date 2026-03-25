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

const TaskGroupApplicationBusinessLicenseOCR = "group_application:ocr_business_license"

type groupApplicationOCRPayload struct {
	ApplicationID int64 `json:"application_id"`
	MediaAssetID  int64 `json:"media_asset_id,omitempty"`
	OCRJobID      int64 `json:"ocr_job_id,omitempty"`
}

func mergeGroupApplicationData(data []byte) map[string]any {
	result := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &result)
	}
	return result
}

func (distributor *RedisTaskDistributor) DistributeTaskGroupApplicationBusinessLicenseOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	payloadBytes, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: applicationID, MediaAssetID: mediaAssetID, OCRJobID: ocrJobID})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskGroupApplicationBusinessLicenseOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (processor *RedisTaskProcessor) ProcessTaskGroupApplicationBusinessLicenseOCR(ctx context.Context, task *asynq.Task) error {
	var payload groupApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || payload.OCRJobID <= 0 {
		return fmt.Errorf("invalid group business license payload: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for group business license: %w", asynq.SkipRetry)
	}

	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{JobID: payload.OCRJobID, LeaseOwner: "worker:group_business_license"})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		app, getErr := processor.store.GetGroupApplication(ctx, payload.ApplicationID)
		if getErr == nil {
			data := mergeGroupApplicationData(app.ApplicationData)
			data["business_license_ocr"] = map[string]any{"status": string(ocr.JobStatusFailed), "queued_at": job.CreatedAt.Format(time.RFC3339), "started_at": formatPgTimestamp(job.StartedAt), "ocr_job_id": payload.OCRJobID, "error": err.Error(), "error_code": ocr.ErrorCode(err), "alert_emitted_at": formatOCRAlertEmittedAt(alertEmittedAt)}
			merged, _ := json.Marshal(data)
			_, _ = processor.store.UpdateGroupApplicationLicense(ctx, db.UpdateGroupApplicationLicenseParams{ID: payload.ApplicationID, ApplicationData: merged})
		}
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)

	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized group business license result: %w", decodeErr)
	}
	app, err := processor.store.GetGroupApplication(ctx, payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("get group application: %w", err)
	}

	data := mergeGroupApplicationData(app.ApplicationData)
	ocrData := map[string]any{"status": "done", "queued_at": job.CreatedAt.Format(time.RFC3339), "started_at": formatPgTimestamp(job.StartedAt), "ocr_job_id": job.ID, "ocr_at": normalized.RecognizedAt.Format(time.RFC3339)}
	arg := db.UpdateGroupApplicationLicenseParams{ID: payload.ApplicationID}
	if normalized.BusinessLicense != nil {
		ocrData["credit_code"] = normalized.BusinessLicense.CreditCode
		ocrData["reg_num"] = normalized.BusinessLicense.RegistrationNumber
		ocrData["enterprise_name"] = normalized.BusinessLicense.EnterpriseName
		ocrData["legal_representative"] = normalized.BusinessLicense.LegalRepresentative
		ocrData["address"] = normalized.BusinessLicense.Address
		ocrData["business_scope"] = normalized.BusinessLicense.BusinessScope
		ocrData["valid_period"] = normalizeValidPeriod(normalized.BusinessLicense.ValidPeriod)
		licenseNumber := normalized.BusinessLicense.CreditCode
		if licenseNumber == "" {
			licenseNumber = normalized.BusinessLicense.RegistrationNumber
		}
		if licenseNumber != "" {
			arg.LicenseNumber = pgtype.Text{String: licenseNumber, Valid: true}
		}
	}
	data["business_license_ocr"] = ocrData
	merged, _ := json.Marshal(data)
	arg.ApplicationData = merged
	_, err = processor.store.UpdateGroupApplicationLicense(ctx, arg)
	if err != nil {
		return fmt.Errorf("update group application license: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})

	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Msg("✅ group business license OCR updated from ocr job")
	return nil
}
