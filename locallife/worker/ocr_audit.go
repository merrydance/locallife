package worker

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/rs/zerolog/log"
)

func (processor *RedisTaskProcessor) writeOCRJobAudit(ctx context.Context, job db.OcrJob, action string, extra map[string]any) {
	if processor == nil || processor.store == nil || job.ID <= 0 {
		return
	}

	metadata := map[string]any{
		"ocr_job_id":     job.ID,
		"status":         job.Status,
		"document_type":  job.DocumentType,
		"provider":       job.Provider,
		"media_asset_id": job.MediaAssetID,
		"owner_type":     job.OwnerType,
		"owner_id":       job.OwnerID,
		"requested_by":   job.RequestedBy,
		"result_version": job.ResultVersion,
	}
	if job.Side != "" {
		metadata["side"] = job.Side
	}
	if job.ErrorCode.Valid {
		metadata["error_code"] = job.ErrorCode.String
	}
	for key, value := range extra {
		metadata[key] = value
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		log.Warn().Err(err).Str("action", action).Int64("ocr_job_id", job.ID).Msg("failed to marshal ocr audit metadata")
		return
	}

	actorUserID := pgtype.Int8{}
	if job.RequestedBy > 0 {
		actorUserID = pgtype.Int8{Int64: job.RequestedBy, Valid: true}
	}

	_, err = processor.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: actorUserID,
		ActorRole:   "system",
		Action:      action,
		TargetType:  "ocr_job",
		TargetID:    pgtype.Int8{Int64: job.ID, Valid: true},
		RegionID:    pgtype.Int8{},
		RequestID:   pgtype.Text{},
		TraceID:     pgtype.Text{},
		ClientIp:    pgtype.Text{},
		UserAgent:   pgtype.Text{},
		Metadata:    metadataBytes,
	})
	if err != nil {
		log.Warn().Err(err).Str("action", action).Int64("ocr_job_id", job.ID).Msg("failed to write ocr audit log")
	}
}

func ocrFailureAuditMetadata(err error) map[string]any {
	if err == nil {
		return nil
	}
	metadata := map[string]any{
		"status": "failed",
	}
	if errorCode := ocr.ErrorCode(err); errorCode != "" {
		metadata["error_code"] = errorCode
	}
	return metadata
}
