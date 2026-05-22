package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/rs/zerolog/log"
)

var errGroupOCRApplicationDataMalformed = errors.New("malformed group application data")

type workerOCRPayloadBinding struct {
	ApplicationID int64
	MediaAssetID  int64
	OCRJobID      int64
	Side          string
}

type workerOCRPendingPayload struct {
	OCRJobID *int64 `json:"ocr_job_id,omitempty"`
}

func workerOCRSide(side string) ocr.DocumentSide {
	return ocr.DocumentSide(strings.ToLower(strings.TrimSpace(side)))
}

func workerOCRJobMatchesPayload(job db.OcrJob, payload workerOCRPayloadBinding, ownerType ocr.OwnerType, documentType ocr.DocumentType, side ocr.DocumentSide) bool {
	if job.ID != payload.OCRJobID ||
		job.OwnerType != string(ownerType) ||
		job.OwnerID != payload.ApplicationID ||
		job.DocumentType != string(documentType) ||
		job.MediaAssetID != payload.MediaAssetID {
		return false
	}
	if side == ocr.DocumentSideUnknown {
		return strings.TrimSpace(job.Side) == ""
	}
	return strings.EqualFold(job.Side, string(side))
}

func workerOCRMatchesMerchantApplication(app db.MerchantApplication, payload workerOCRPayloadBinding, documentType ocr.DocumentType, side ocr.DocumentSide, expectedJobID int64) bool {
	bound, ocrPayload := merchantOCRCurrentBinding(app, documentType, side)
	if !workerOCRPgInt8Matches(bound, payload.MediaAssetID) {
		return false
	}
	return workerOCROCRJobIDMatches(ocrPayload, expectedJobID)
}

func workerOCRMatchesOperatorApplication(app db.OperatorApplication, payload workerOCRPayloadBinding, documentType ocr.DocumentType, side ocr.DocumentSide, expectedJobID int64) bool {
	bound, ocrPayload := operatorOCRCurrentBinding(app, documentType, side)
	if !workerOCRPgInt8Matches(bound, payload.MediaAssetID) {
		return false
	}
	return workerOCROCRJobIDMatches(ocrPayload, expectedJobID)
}

func workerOCRMatchesGroupApplication(app db.MerchantGroupApplication, payload workerOCRPayloadBinding, documentType ocr.DocumentType, side ocr.DocumentSide, expectedJobID int64) (bool, error) {
	assetID, ocrPayload, err := groupOCRCurrentBinding(app, documentType, side)
	if err != nil {
		return false, err
	}
	if assetID != payload.MediaAssetID {
		return false, nil
	}
	return workerOCROCRJobIDMatches(ocrPayload, expectedJobID), nil
}

func workerOCROCRJobIDMatches(data []byte, expectedJobID int64) bool {
	var payload workerOCRPendingPayload
	if err := decodeWorkerOCRPayload(data, &payload); err != nil {
		return false
	}
	return payload.OCRJobID != nil && *payload.OCRJobID == expectedJobID
}

func workerOCRPgInt8Matches(value pgtype.Int8, expected int64) bool {
	return expected > 0 && value.Valid && value.Int64 == expected
}

func workerOCRLogStale(ownerType ocr.OwnerType, payload workerOCRPayloadBinding, reason string) {
	log.Info().
		Str("owner_type", string(ownerType)).
		Int64("application_id", payload.ApplicationID).
		Int64("ocr_job_id", payload.OCRJobID).
		Int64("media_asset_id", payload.MediaAssetID).
		Str("side", payload.Side).
		Str("reason", reason).
		Msg("skip stale OCR task")
}

func (processor *RedisTaskProcessor) guardMerchantOCRWriteback(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.OcrJob, db.MerchantApplication, bool, error) {
	job, err := processor.store.GetOCRJob(ctx, payload.OCRJobID)
	if err != nil {
		return db.OcrJob{}, db.MerchantApplication{}, false, err
	}
	app, err := processor.store.GetMerchantApplication(ctx, payload.ApplicationID)
	if err != nil {
		return db.OcrJob{}, db.MerchantApplication{}, false, err
	}
	if !workerOCRJobMatchesPayload(job, payload, ocr.OwnerTypeMerchantApplication, documentType, side) {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "job_payload_mismatch")
		return job, app, true, nil
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "application_not_draft")
		return job, app, true, nil
	}

	bound, ocrPayload := merchantOCRCurrentBinding(app, documentType, side)
	if !workerOCRPgInt8Matches(bound, payload.MediaAssetID) {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "media_asset_mismatch")
		return job, app, true, nil
	}
	if !workerOCROCRJobIDMatches(ocrPayload, payload.OCRJobID) {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "ocr_job_id_mismatch")
		return job, app, true, nil
	}
	return job, app, false, nil
}

func (processor *RedisTaskProcessor) guardMerchantOCRCurrentBinding(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.MerchantApplication, bool, error) {
	app, err := processor.store.GetMerchantApplication(ctx, payload.ApplicationID)
	if err != nil {
		return db.MerchantApplication{}, false, err
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "application_not_draft")
		return app, true, nil
	}
	if !workerOCRMatchesMerchantApplication(app, payload, documentType, side, payload.OCRJobID) {
		workerOCRLogStale(ocr.OwnerTypeMerchantApplication, payload, "current_binding_mismatch")
		return app, true, nil
	}
	return app, false, nil
}

func merchantOCRCurrentBinding(app db.MerchantApplication, documentType ocr.DocumentType, side ocr.DocumentSide) (pgtype.Int8, []byte) {
	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return app.BusinessLicenseMediaAssetID, app.BusinessLicenseOcr
	case ocr.DocumentTypeFoodPermit:
		return app.FoodPermitMediaAssetID, app.FoodPermitOcr
	case ocr.DocumentTypeIDCard:
		if side == ocr.DocumentSideBack {
			return app.IDCardBackMediaAssetID, app.IDCardBackOcr
		}
		return app.IDCardFrontMediaAssetID, app.IDCardFrontOcr
	default:
		return pgtype.Int8{}, nil
	}
}

func (processor *RedisTaskProcessor) guardOperatorOCRWriteback(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.OcrJob, db.OperatorApplication, bool, error) {
	job, err := processor.store.GetOCRJob(ctx, payload.OCRJobID)
	if err != nil {
		return db.OcrJob{}, db.OperatorApplication{}, false, err
	}
	app, err := processor.store.GetOperatorApplicationByID(ctx, payload.ApplicationID)
	if err != nil {
		return db.OcrJob{}, db.OperatorApplication{}, false, err
	}
	if !workerOCRJobMatchesPayload(job, payload, ocr.OwnerTypeOperatorApplication, documentType, side) {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "job_payload_mismatch")
		return job, app, true, nil
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "application_not_draft")
		return job, app, true, nil
	}

	bound, ocrPayload := operatorOCRCurrentBinding(app, documentType, side)
	if !workerOCRPgInt8Matches(bound, payload.MediaAssetID) {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "media_asset_mismatch")
		return job, app, true, nil
	}
	if !workerOCROCRJobIDMatches(ocrPayload, payload.OCRJobID) {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "ocr_job_id_mismatch")
		return job, app, true, nil
	}
	return job, app, false, nil
}

func (processor *RedisTaskProcessor) guardOperatorOCRCurrentBinding(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.OperatorApplication, bool, error) {
	app, err := processor.store.GetOperatorApplicationByID(ctx, payload.ApplicationID)
	if err != nil {
		return db.OperatorApplication{}, false, err
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "application_not_draft")
		return app, true, nil
	}
	if !workerOCRMatchesOperatorApplication(app, payload, documentType, side, payload.OCRJobID) {
		workerOCRLogStale(ocr.OwnerTypeOperatorApplication, payload, "current_binding_mismatch")
		return app, true, nil
	}
	return app, false, nil
}

func operatorOCRCurrentBinding(app db.OperatorApplication, documentType ocr.DocumentType, side ocr.DocumentSide) (pgtype.Int8, []byte) {
	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return app.BusinessLicenseMediaAssetID, app.BusinessLicenseOcr
	case ocr.DocumentTypeIDCard:
		if side == ocr.DocumentSideBack {
			return app.IDCardBackMediaAssetID, app.IDCardBackOcr
		}
		return app.IDCardFrontMediaAssetID, app.IDCardFrontOcr
	default:
		return pgtype.Int8{}, nil
	}
}

func (processor *RedisTaskProcessor) guardGroupOCRWriteback(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.OcrJob, db.MerchantGroupApplication, bool, error) {
	job, err := processor.store.GetOCRJob(ctx, payload.OCRJobID)
	if err != nil {
		return db.OcrJob{}, db.MerchantGroupApplication{}, false, err
	}
	app, err := processor.store.GetGroupApplication(ctx, payload.ApplicationID)
	if err != nil {
		return db.OcrJob{}, db.MerchantGroupApplication{}, false, err
	}
	if !workerOCRJobMatchesPayload(job, payload, ocr.OwnerTypeGroupApplication, documentType, side) {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "job_payload_mismatch")
		return job, app, true, nil
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "application_not_draft")
		return job, app, true, nil
	}

	assetID, ocrPayload, err := groupOCRCurrentBinding(app, documentType, side)
	if err != nil {
		if errors.Is(err, errGroupOCRApplicationDataMalformed) {
			workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "application_data_malformed")
			return job, app, true, nil
		}
		return job, app, false, err
	}
	if assetID != payload.MediaAssetID {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "media_asset_mismatch")
		return job, app, true, nil
	}
	if !workerOCROCRJobIDMatches(ocrPayload, payload.OCRJobID) {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "ocr_job_id_mismatch")
		return job, app, true, nil
	}
	return job, app, false, nil
}

func (processor *RedisTaskProcessor) guardGroupOCRCurrentBinding(
	ctx context.Context,
	payload workerOCRPayloadBinding,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
) (db.MerchantGroupApplication, bool, error) {
	app, err := processor.store.GetGroupApplication(ctx, payload.ApplicationID)
	if err != nil {
		return db.MerchantGroupApplication{}, false, err
	}
	if app.Status != "draft" {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "application_not_draft")
		return app, true, nil
	}
	matches, err := workerOCRMatchesGroupApplication(app, payload, documentType, side, payload.OCRJobID)
	if err != nil {
		if errors.Is(err, errGroupOCRApplicationDataMalformed) {
			workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "application_data_malformed")
			return app, true, nil
		}
		return app, false, err
	}
	if !matches {
		workerOCRLogStale(ocr.OwnerTypeGroupApplication, payload, "current_binding_mismatch")
		return app, true, nil
	}
	return app, false, nil
}

func groupOCRCurrentBinding(app db.MerchantGroupApplication, documentType ocr.DocumentType, side ocr.DocumentSide) (int64, []byte, error) {
	data := map[string]json.RawMessage{}
	if len(app.ApplicationData) > 0 {
		if err := json.Unmarshal(app.ApplicationData, &data); err != nil {
			return 0, nil, errGroupOCRApplicationDataMalformed
		}
	}

	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return pgInt8ToInt64(app.LicenseMediaAssetID), data["business_license_ocr"], nil
	case ocr.DocumentTypeIDCard:
		if side == ocr.DocumentSideBack {
			return groupOCRAssetID(data["id_card_back_asset_id"]), data["id_card_back_ocr"], nil
		}
		return groupOCRAssetID(data["id_card_front_asset_id"]), data["id_card_front_ocr"], nil
	default:
		return 0, nil, nil
	}
}

func pgInt8ToInt64(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func groupOCRAssetID(raw json.RawMessage) int64 {
	var value int64
	if len(raw) == 0 {
		return 0
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
}
