package api

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/rs/zerolog/log"
)

func (server *Server) markRiderApplicationOCRPending(ctx *gin.Context, job db.OcrJob) error {
	app, err := server.store.GetRiderApplication(ctx, job.OwnerID)
	if err != nil {
		return err
	}
	if app.Status != "draft" {
		log.Warn().
			Int64("application_id", app.ID).
			Int64("ocr_job_id", job.ID).
			Int64("media_asset_id", job.MediaAssetID).
			Str("side", job.Side).
			Str("status", app.Status).
			Bool("front_asset_bound", app.IDCardFrontMediaAssetID.Valid).
			Bool("back_asset_bound", app.IDCardBackMediaAssetID.Valid).
			Int64("front_asset_id", app.IDCardFrontMediaAssetID.Int64).
			Int64("back_asset_id", app.IDCardBackMediaAssetID.Int64).
			Msg("skip marking rider OCR pending because application is not draft")
		return nil
	}

	switch ocr.DocumentType(job.DocumentType) {
	case ocr.DocumentTypeIDCard:
		return server.markRiderIDCardOCRPending(ctx, app, job)
	case ocr.DocumentTypeHealthCert:
		return server.markRiderHealthCertOCRPending(ctx, app, job)
	default:
		return nil
	}
}

func (server *Server) markRiderIDCardOCRPending(ctx *gin.Context, app db.RiderApplication, job db.OcrJob) error {
	log.Info().
		Int64("application_id", app.ID).
		Int64("ocr_job_id", job.ID).
		Int64("media_asset_id", job.MediaAssetID).
		Str("side", job.Side).
		Bool("front_asset_bound", app.IDCardFrontMediaAssetID.Valid).
		Bool("back_asset_bound", app.IDCardBackMediaAssetID.Valid).
		Int64("front_asset_id", app.IDCardFrontMediaAssetID.Int64).
		Int64("back_asset_id", app.IDCardBackMediaAssetID.Int64).
		Msg("mark rider id card OCR pending before binding media asset")

	payload := readRiderIDCardOCRData(app.IDCardOcr)
	resetRiderIDCardPendingPayload(&payload, job)
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return marshalErr
	}

	arg := db.UpdateRiderApplicationIDCardParams{
		ID:        app.ID,
		IDCardOcr: encoded,
	}
	if strings.EqualFold(job.Side, string(ocr.DocumentSideBack)) {
		arg.IDCardBackMediaAssetID = pgtype.Int8{Int64: job.MediaAssetID, Valid: true}
	} else {
		arg.IDCardFrontMediaAssetID = pgtype.Int8{Int64: job.MediaAssetID, Valid: true}
	}
	updated, err := server.store.UpdateRiderApplicationIDCard(ctx, arg)
	if err == nil {
		log.Info().
			Int64("application_id", updated.ID).
			Int64("ocr_job_id", job.ID).
			Int64("media_asset_id", job.MediaAssetID).
			Str("side", job.Side).
			Bool("front_asset_bound", updated.IDCardFrontMediaAssetID.Valid).
			Bool("back_asset_bound", updated.IDCardBackMediaAssetID.Valid).
			Int64("front_asset_id", updated.IDCardFrontMediaAssetID.Int64).
			Int64("back_asset_id", updated.IDCardBackMediaAssetID.Int64).
			Msg("mark rider id card OCR pending after binding media asset")
	}
	return err
}

func (server *Server) markRiderHealthCertOCRPending(ctx *gin.Context, app db.RiderApplication, job db.OcrJob) error {
	payload := HealthCertOCRData{}
	resetRiderHealthCertPendingPayload(&payload, job)
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return marshalErr
	}
	_, err := server.store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{Int64: job.MediaAssetID, Valid: true},
		HealthCertOcr:          encoded,
	})
	return err
}

func resetRiderIDCardPendingPayload(payload *IDCardOCRData, job db.OcrJob) {
	if strings.EqualFold(job.Side, string(ocr.DocumentSideBack)) {
		payload.ValidStart = ""
		payload.ValidEnd = ""
	} else {
		payload.Name = ""
		payload.IDNumber = ""
		payload.Gender = ""
		payload.Nation = ""
		payload.Address = ""
	}
	payload.Status = string(ocr.JobStatusPending)
	payload.Error = ""
	payload.ErrorCode = ""
	payload.AlertEmittedAt = ""
	payload.Readiness = nil
	payload.QueuedAt = job.CreatedAt.Format(time.RFC3339)
	payload.StartedAt = ""
	payload.OCRJobID = &job.ID
	payload.OCRAt = ""
}

func resetRiderHealthCertPendingPayload(payload *HealthCertOCRData, job db.OcrJob) {
	payload.Status = string(ocr.JobStatusPending)
	payload.Error = ""
	payload.ErrorCode = ""
	payload.AlertEmittedAt = ""
	payload.Readiness = nil
	payload.QueuedAt = job.CreatedAt.Format(time.RFC3339)
	payload.StartedAt = ""
	payload.OCRJobID = &job.ID
	payload.Name = ""
	payload.IDNumber = ""
	payload.CertNumber = ""
	payload.ValidStart = ""
	payload.ValidEnd = ""
	payload.OCRAt = ""
}
