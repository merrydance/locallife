package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestNewMerchantApplicationDraftResponse_PreservesAsyncOCRFields(t *testing.T) {
	jobID := int64(101)
	businessLicenseOCR, err := json.Marshal(BusinessLicenseOCRData{
		Status:         "processing",
		ErrorCode:      "ocr_rate_limited",
		AlertEmittedAt: "2026-03-25T16:00:00Z",
		QueuedAt:       "2026-03-25T15:59:00Z",
		StartedAt:      "2026-03-25T15:59:05Z",
		OCRJobID:       &jobID,
		EnterpriseName: "测试商户",
	})
	require.NoError(t, err)

	foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
		Status:       "done",
		QueuedAt:     "2026-03-25T15:58:00Z",
		StartedAt:    "2026-03-25T15:58:03Z",
		OCRJobID:     &jobID,
		PermitNo:     "JY12345678901234",
		CompanyName:  "测试商户",
		OperatorName: "张三",
	})
	require.NoError(t, err)

	idCardFrontOCR, err := json.Marshal(MerchantIDCardOCRData{
		Status:         "failed",
		Error:          "bad image",
		ErrorCode:      "ocr_bad_request",
		AlertEmittedAt: "2026-03-25T16:01:00Z",
		QueuedAt:       "2026-03-25T16:00:30Z",
		StartedAt:      "2026-03-25T16:00:31Z",
		OCRJobID:       &jobID,
		Name:           "张三",
	})
	require.NoError(t, err)

	app := db.MerchantApplication{
		ID:                          1,
		UserID:                      2,
		MerchantName:                "测试商户",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 10, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 11, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 12, Valid: true},
		BusinessLicenseOcr:          businessLicenseOCR,
		FoodPermitOcr:               foodPermitOCR,
		IDCardFrontOcr:              idCardFrontOCR,
		Status:                      "draft",
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}

	server := &Server{}
	resp, err := server.newMerchantApplicationDraftResponse(context.Background(), app)
	require.NoError(t, err)

	require.NotNil(t, resp.BusinessLicenseOCR)
	require.Equal(t, "processing", resp.BusinessLicenseOCR.Status)
	require.Equal(t, "ocr_rate_limited", resp.BusinessLicenseOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:00:00Z", resp.BusinessLicenseOCR.AlertEmittedAt)
	require.Equal(t, "2026-03-25T15:59:00Z", resp.BusinessLicenseOCR.QueuedAt)
	require.Equal(t, "2026-03-25T15:59:05Z", resp.BusinessLicenseOCR.StartedAt)
	require.NotNil(t, resp.BusinessLicenseOCR.OCRJobID)
	require.Equal(t, jobID, *resp.BusinessLicenseOCR.OCRJobID)

	require.NotNil(t, resp.FoodPermitOCR)
	require.Equal(t, "2026-03-25T15:58:00Z", resp.FoodPermitOCR.QueuedAt)
	require.Equal(t, "2026-03-25T15:58:03Z", resp.FoodPermitOCR.StartedAt)
	require.NotNil(t, resp.FoodPermitOCR.OCRJobID)
	require.Equal(t, jobID, *resp.FoodPermitOCR.OCRJobID)

	require.NotNil(t, resp.IDCardFrontOCR)
	require.Equal(t, "failed", resp.IDCardFrontOCR.Status)
	require.Equal(t, "ocr_bad_request", resp.IDCardFrontOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:01:00Z", resp.IDCardFrontOCR.AlertEmittedAt)
	require.NotNil(t, resp.IDCardFrontOCR.OCRJobID)
	require.Equal(t, jobID, *resp.IDCardFrontOCR.OCRJobID)
}

func TestNewOperatorApplicationResponse_PreservesAsyncOCRFields(t *testing.T) {
	jobID := int64(202)
	businessLicenseOCR, err := json.Marshal(BusinessLicenseOCRData{
		Status:         "done",
		QueuedAt:       "2026-03-25T16:10:00Z",
		StartedAt:      "2026-03-25T16:10:02Z",
		OCRJobID:       &jobID,
		EnterpriseName: "测试运营商",
	})
	require.NoError(t, err)

	idCardFrontOCR, err := json.Marshal(OperatorIDCardOCRData{
		Status:         "processing",
		ErrorCode:      "ocr_retryable_error",
		AlertEmittedAt: "2026-03-25T16:12:00Z",
		QueuedAt:       "2026-03-25T16:11:00Z",
		StartedAt:      "2026-03-25T16:11:03Z",
		OCRJobID:       &jobID,
		Name:           "李四",
	})
	require.NoError(t, err)

	idCardBackOCR, err := json.Marshal(OperatorIDCardBackOCR{
		Status:    "done",
		QueuedAt:  "2026-03-25T16:13:00Z",
		StartedAt: "2026-03-25T16:13:04Z",
		OCRJobID:  &jobID,
		ValidEnd:  "2036-03-25",
	})
	require.NoError(t, err)

	app := db.OperatorApplication{
		ID:                          3,
		UserID:                      4,
		RegionID:                    5,
		BusinessLicenseOcr:          businessLicenseOCR,
		IDCardFrontOcr:              idCardFrontOCR,
		IDCardBackOcr:               idCardBackOCR,
		RequestedContractYears:      2,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 20, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 21, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 22, Valid: true},
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}

	resp, err := newOperatorApplicationResponse(app, "测试区域")
	require.NoError(t, err)

	require.NotNil(t, resp.BusinessLicenseOCR)
	require.Equal(t, "2026-03-25T16:10:00Z", resp.BusinessLicenseOCR.QueuedAt)
	require.NotNil(t, resp.BusinessLicenseOCR.OCRJobID)
	require.Equal(t, jobID, *resp.BusinessLicenseOCR.OCRJobID)

	require.NotNil(t, resp.IDCardFrontOCR)
	require.Equal(t, "processing", resp.IDCardFrontOCR.Status)
	require.Equal(t, "ocr_retryable_error", resp.IDCardFrontOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:12:00Z", resp.IDCardFrontOCR.AlertEmittedAt)
	require.NotNil(t, resp.IDCardFrontOCR.OCRJobID)
	require.Equal(t, jobID, *resp.IDCardFrontOCR.OCRJobID)

	require.NotNil(t, resp.IDCardBackOCR)
	require.Equal(t, "2026-03-25T16:13:00Z", resp.IDCardBackOCR.QueuedAt)
	require.Equal(t, "2026-03-25T16:13:04Z", resp.IDCardBackOCR.StartedAt)
	require.NotNil(t, resp.IDCardBackOCR.OCRJobID)
	require.Equal(t, jobID, *resp.IDCardBackOCR.OCRJobID)
}

func TestNewRiderApplicationResponse_PreservesAsyncOCRFields(t *testing.T) {
	jobID := int64(303)
	idCardOCR, err := json.Marshal(IDCardOCRData{
		Status:         "done",
		QueuedAt:       "2026-03-25T16:20:00Z",
		StartedAt:      "2026-03-25T16:20:05Z",
		OCRJobID:       &jobID,
		ErrorCode:      "",
		AlertEmittedAt: "",
		Name:           "王五",
	})
	require.NoError(t, err)

	healthCertOCR, err := json.Marshal(HealthCertOCRData{
		Status:         "failed",
		Error:          "provider unavailable",
		ErrorCode:      "ocr_provider_unavailable",
		AlertEmittedAt: "2026-03-25T16:22:00Z",
		QueuedAt:       "2026-03-25T16:21:00Z",
		StartedAt:      "2026-03-25T16:21:01Z",
		OCRJobID:       &jobID,
		Name:           "王五",
	})
	require.NoError(t, err)

	app := db.RiderApplication{
		ID:                      6,
		UserID:                  7,
		IDCardOcr:               idCardOCR,
		HealthCertOcr:           healthCertOCR,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 30, Valid: true},
		IDCardBackMediaAssetID:  pgtype.Int8{Int64: 31, Valid: true},
		HealthCertMediaAssetID:  pgtype.Int8{Int64: 32, Valid: true},
		Status:                  "draft",
		CreatedAt:               time.Now(),
	}

	server := &Server{}
	resp := server.newRiderApplicationResponse(context.Background(), app)

	require.NotNil(t, resp.IDCardOCR)
	require.Equal(t, "2026-03-25T16:20:00Z", resp.IDCardOCR.QueuedAt)
	require.Equal(t, "2026-03-25T16:20:05Z", resp.IDCardOCR.StartedAt)
	require.NotNil(t, resp.IDCardOCR.OCRJobID)
	require.Equal(t, jobID, *resp.IDCardOCR.OCRJobID)

	require.NotNil(t, resp.HealthCertOCR)
	require.Equal(t, "failed", resp.HealthCertOCR.Status)
	require.Equal(t, "ocr_provider_unavailable", resp.HealthCertOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:22:00Z", resp.HealthCertOCR.AlertEmittedAt)
	require.NotNil(t, resp.HealthCertOCR.OCRJobID)
	require.Equal(t, jobID, *resp.HealthCertOCR.OCRJobID)
}

func TestNewRiderApplicationResponse_DecodesStringifiedHealthCertOCR(t *testing.T) {
	jobID := int64(404)
	inner, err := json.Marshal(HealthCertOCRData{
		Status:   "done",
		OCRJobID: &jobID,
		Name:     "张三",
	})
	require.NoError(t, err)
	wrapped, err := json.Marshal(string(inner))
	require.NoError(t, err)

	app := db.RiderApplication{
		ID:            8,
		UserID:        9,
		HealthCertOcr: wrapped,
		Status:        "draft",
		CreatedAt:     time.Now(),
	}

	server := &Server{}
	resp := server.newRiderApplicationResponse(context.Background(), app)

	require.NotNil(t, resp.HealthCertOCR)
	require.Equal(t, "done", resp.HealthCertOCR.Status)
	require.Equal(t, "张三", resp.HealthCertOCR.Name)
	require.NotNil(t, resp.HealthCertOCR.OCRJobID)
	require.Equal(t, jobID, *resp.HealthCertOCR.OCRJobID)
}
