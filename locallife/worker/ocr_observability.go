package worker

import (
	"context"
	"encoding/json"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
)

var (
	ocrJobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocr_jobs_total",
			Help: "Total number of OCR jobs observed by worker final status",
		},
		[]string{"owner_type", "document_type", "provider", "status", "error_code"},
	)

	ocrJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ocr_job_duration_seconds",
			Help:    "OCR job execution duration in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60},
		},
		[]string{"owner_type", "document_type", "provider", "status"},
	)

	ocrAlertsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocr_alerts_total",
			Help: "Total number of OCR alerts published for operator attention",
		},
		[]string{"alert_type", "level", "owner_type", "document_type"},
	)

	ocrRetrySuppressedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocr_retry_suppressed_total",
			Help: "Total number of OCR task retries suppressed due to permanent errors or exhausted attempts",
		},
		[]string{"owner_type", "document_type", "provider", "reason"},
	)
)

func normalizedOCRLabel(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func observeOCRJob(job db.OcrJob) {
	errorCode := "none"
	if job.ErrorCode.Valid && job.ErrorCode.String != "" {
		errorCode = job.ErrorCode.String
	}
	ocrJobsTotal.WithLabelValues(
		normalizedOCRLabel(job.OwnerType),
		normalizedOCRLabel(job.DocumentType),
		normalizedOCRLabel(job.Provider),
		normalizedOCRLabel(job.Status),
		errorCode,
	).Inc()
	if !job.StartedAt.Valid {
		return
	}
	endedAt := time.Now().UTC()
	if job.FinishedAt.Valid {
		endedAt = job.FinishedAt.Time
	}
	duration := endedAt.Sub(job.StartedAt.Time).Seconds()
	if duration < 0 {
		return
	}
	ocrJobDuration.WithLabelValues(
		normalizedOCRLabel(job.OwnerType),
		normalizedOCRLabel(job.DocumentType),
		normalizedOCRLabel(job.Provider),
		normalizedOCRLabel(job.Status),
	).Observe(duration)
}

func ocrRetrySuppressionReason(job db.OcrJob, err error) string {
	if ocr.IsRetryableError(err) && job.MaxAttempts > 0 && job.AttemptCount >= job.MaxAttempts {
		return "attempts_exhausted"
	}
	return "permanent_error"
}

func recordOCRRetrySuppressed(job db.OcrJob, reason string) {
	ocrRetrySuppressedTotal.WithLabelValues(
		normalizedOCRLabel(job.OwnerType),
		normalizedOCRLabel(job.DocumentType),
		normalizedOCRLabel(job.Provider),
		reason,
	).Inc()
}

func ocrAlertKind(job db.OcrJob, err error) (AlertType, AlertLevel) {
	if ocr.IsRetryableError(err) && job.MaxAttempts > 0 && job.AttemptCount >= job.MaxAttempts {
		return AlertTypeOCRRetryExhausted, AlertLevelCritical
	}
	switch ocr.ErrorCode(err) {
	case "ocr_provider_unauthorized", "ocr_provider_forbidden", "ocr_provider_unavailable", "ocr_rate_limited", "ocr_retryable_error":
		return AlertTypeOCRJobFailed, AlertLevelCritical
	default:
		return AlertTypeOCRJobFailed, AlertLevelWarning
	}
}

func (processor *RedisTaskProcessor) publishOCRFailureAlert(ctx context.Context, job db.OcrJob, err error) *time.Time {
	observeOCRJob(job)
	if !shouldSkipOCRError(job, err) {
		return nil
	}
	reason := ocrRetrySuppressionReason(job, err)
	recordOCRRetrySuppressed(job, reason)
	alertType, level := ocrAlertKind(job, err)
	emittedAt := time.Now().UTC()
	alert := AlertData{
		AlertType:   alertType,
		Level:       level,
		Title:       "OCR任务失败需人工关注",
		Message:     err.Error(),
		RelatedID:   job.ID,
		RelatedType: "ocr_job",
		Extra: map[string]interface{}{
			"ocr_job_id":     job.ID,
			"owner_type":     job.OwnerType,
			"owner_id":       job.OwnerID,
			"document_type":  job.DocumentType,
			"provider":       job.Provider,
			"media_asset_id": job.MediaAssetID,
			"side":           job.Side,
			"attempt_count":  job.AttemptCount,
			"max_attempts":   job.MaxAttempts,
			"error_code":     ocr.ErrorCode(err),
			"retryable":      ocr.IsRetryableError(err),
			"reason":         reason,
		},
		Timestamp: emittedAt,
	}
	ocrAlertsTotal.WithLabelValues(string(alertType), string(level), normalizedOCRLabel(job.OwnerType), normalizedOCRLabel(job.DocumentType)).Inc()

	if processor.pubSubPublisher == nil {
		log.Warn().Int64("ocr_job_id", job.ID).Msg("ocr alert skipped because pubsub publisher is not configured")
		return &emittedAt
	}
	message := map[string]any{
		"type":      "alert",
		"data":      alert,
		"timestamp": emittedAt,
	}
	payload, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		log.Error().Err(marshalErr).Int64("ocr_job_id", job.ID).Msg("failed to marshal ocr alert message")
		return nil
	}
	if publishErr := processor.pubSubPublisher.Publish(ctx, AlertChannel, payload); publishErr != nil {
		log.Error().Err(publishErr).Int64("ocr_job_id", job.ID).Str("alert_type", string(alertType)).Msg("failed to publish ocr alert")
		return nil
	}
	return &emittedAt
}

func formatOCRAlertEmittedAt(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
