package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

type merchantApplicationDraftAPIResponse struct {
	ID            int64 `json:"id"`
	FoodPermitOCR *struct {
		Status         string `json:"status"`
		ErrorCode      string `json:"error_code"`
		AlertEmittedAt string `json:"alert_emitted_at"`
		QueuedAt       string `json:"queued_at"`
		StartedAt      string `json:"started_at"`
		OCRJobID       *int64 `json:"ocr_job_id"`
		PermitNo       string `json:"permit_no"`
	} `json:"food_permit_ocr"`
}

type riderApplicationAPIResponse struct {
	ID        int64 `json:"id"`
	IDCardOCR *struct {
		Status    string `json:"status"`
		QueuedAt  string `json:"queued_at"`
		StartedAt string `json:"started_at"`
		OCRJobID  *int64 `json:"ocr_job_id"`
		Name      string `json:"name"`
	} `json:"id_card_ocr"`
	HealthCertOCR *struct {
		Status         string `json:"status"`
		ErrorCode      string `json:"error_code"`
		AlertEmittedAt string `json:"alert_emitted_at"`
		QueuedAt       string `json:"queued_at"`
		StartedAt      string `json:"started_at"`
		OCRJobID       *int64 `json:"ocr_job_id"`
		Name           string `json:"name"`
	} `json:"health_cert_ocr"`
}

type operatorApplicationAPIResponse struct {
	ID                 int64  `json:"id"`
	RegionID           int64  `json:"region_id"`
	RegionName         string `json:"region_name"`
	BusinessLicenseOCR *struct {
		Status         string `json:"status"`
		QueuedAt       string `json:"queued_at"`
		StartedAt      string `json:"started_at"`
		AlertEmittedAt string `json:"alert_emitted_at"`
		OCRJobID       *int64 `json:"ocr_job_id"`
	} `json:"business_license_ocr"`
	IDCardFrontOCR *struct {
		Status         string `json:"status"`
		ErrorCode      string `json:"error_code"`
		AlertEmittedAt string `json:"alert_emitted_at"`
		QueuedAt       string `json:"queued_at"`
		StartedAt      string `json:"started_at"`
		OCRJobID       *int64 `json:"ocr_job_id"`
		Name           string `json:"name"`
	} `json:"id_card_front_ocr"`
	IDCardBackOCR *struct {
		Status    string `json:"status"`
		QueuedAt  string `json:"queued_at"`
		StartedAt string `json:"started_at"`
		OCRJobID  *int64 `json:"ocr_job_id"`
	} `json:"id_card_back_ocr"`
}

func TestMerchantApplicationFoodPermitAsyncOCRIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	user := createIntegrationUser(t, store)
	app, err := store.CreateMerchantApplicationDraft(ctx, user.ID)
	require.NoError(t, err)

	ocrJobID := int64(8801)
	foodPermitOCR, err := json.Marshal(map[string]any{
		"status":           "processing",
		"error_code":       "ocr_rate_limited",
		"alert_emitted_at": "2026-03-25T18:00:00Z",
		"queued_at":        "2026-03-25T17:59:00Z",
		"started_at":       "2026-03-25T17:59:05Z",
		"ocr_job_id":       ocrJobID,
		"permit_no":        "SP-TEST-001",
	})
	require.NoError(t, err)

	_, err = store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
		ID: app.ID,
		FoodPermitMediaAssetID: pgtype.Int8{
			Valid: false,
		},
		FoodPermitOcr: foodPermitOCR,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, integrationTokenMaker, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, app.ID, resp.ID)
	require.NotNil(t, resp.FoodPermitOCR)
	require.Equal(t, "processing", resp.FoodPermitOCR.Status)
	require.Equal(t, "ocr_rate_limited", resp.FoodPermitOCR.ErrorCode)
	require.Equal(t, "2026-03-25T18:00:00Z", resp.FoodPermitOCR.AlertEmittedAt)
	require.Equal(t, "2026-03-25T17:59:00Z", resp.FoodPermitOCR.QueuedAt)
	require.Equal(t, "2026-03-25T17:59:05Z", resp.FoodPermitOCR.StartedAt)
	require.Equal(t, "SP-TEST-001", resp.FoodPermitOCR.PermitNo)
	require.NotNil(t, resp.FoodPermitOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.FoodPermitOCR.OCRJobID)
}

func TestRiderApplicationAsyncOCRIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	user := createIntegrationUser(t, store)
	app, err := store.CreateRiderApplication(ctx, user.ID)
	require.NoError(t, err)

	idCardJobID := int64(8802)
	healthJobID := int64(8803)
	idCardOCR, err := json.Marshal(map[string]any{
		"status":     "done",
		"queued_at":  "2026-03-25T18:10:00Z",
		"started_at": "2026-03-25T18:10:02Z",
		"ocr_job_id": idCardJobID,
		"name":       "李四",
	})
	require.NoError(t, err)
	healthCertOCR, err := json.Marshal(map[string]any{
		"status":           "failed",
		"error_code":       "ocr_provider_unavailable",
		"alert_emitted_at": "2026-03-25T18:12:00Z",
		"queued_at":        "2026-03-25T18:11:00Z",
		"started_at":       "2026-03-25T18:11:01Z",
		"ocr_job_id":       healthJobID,
		"name":             "李四",
	})
	require.NoError(t, err)

	_, err = store.UpdateRiderApplicationIDCard(ctx, db.UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{Valid: false},
		IDCardBackMediaAssetID:  pgtype.Int8{Valid: false},
		IDCardOcr:               idCardOCR,
		RealName:                pgtype.Text{String: "李四", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{Valid: false},
		HealthCertOcr:          healthCertOCR,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodGet, "/v1/rider/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, integrationTokenMaker, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp riderApplicationAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, app.ID, resp.ID)
	require.NotNil(t, resp.IDCardOCR)
	require.Equal(t, "done", resp.IDCardOCR.Status)
	require.Equal(t, "2026-03-25T18:10:00Z", resp.IDCardOCR.QueuedAt)
	require.Equal(t, "2026-03-25T18:10:02Z", resp.IDCardOCR.StartedAt)
	require.Equal(t, "李四", resp.IDCardOCR.Name)
	require.NotNil(t, resp.IDCardOCR.OCRJobID)
	require.Equal(t, idCardJobID, *resp.IDCardOCR.OCRJobID)
	require.NotNil(t, resp.HealthCertOCR)
	require.Equal(t, "failed", resp.HealthCertOCR.Status)
	require.Equal(t, "ocr_provider_unavailable", resp.HealthCertOCR.ErrorCode)
	require.Equal(t, "2026-03-25T18:12:00Z", resp.HealthCertOCR.AlertEmittedAt)
	require.Equal(t, "2026-03-25T18:11:00Z", resp.HealthCertOCR.QueuedAt)
	require.Equal(t, "2026-03-25T18:11:01Z", resp.HealthCertOCR.StartedAt)
	require.Equal(t, "李四", resp.HealthCertOCR.Name)
	require.NotNil(t, resp.HealthCertOCR.OCRJobID)
	require.Equal(t, healthJobID, *resp.HealthCertOCR.OCRJobID)
}

func TestOperatorApplicationAsyncOCRIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	user := createIntegrationUser(t, store)
	region := createIntegrationRegion(t, store)
	app, err := store.CreateOperatorApplicationDraft(ctx, db.CreateOperatorApplicationDraftParams{
		UserID:   user.ID,
		RegionID: region.ID,
	})
	require.NoError(t, err)

	businessJobID := int64(8804)
	frontJobID := int64(8805)
	backJobID := int64(8806)
	businessLicenseOCR, err := json.Marshal(map[string]any{
		"status":           "done",
		"queued_at":        "2026-03-25T18:20:00Z",
		"started_at":       "2026-03-25T18:20:03Z",
		"alert_emitted_at": "2026-03-25T18:20:10Z",
		"ocr_job_id":       businessJobID,
	})
	require.NoError(t, err)
	idCardFrontOCR, err := json.Marshal(map[string]any{
		"status":           "processing",
		"error_code":       "ocr_retryable_error",
		"alert_emitted_at": "2026-03-25T18:22:00Z",
		"queued_at":        "2026-03-25T18:21:00Z",
		"started_at":       "2026-03-25T18:21:04Z",
		"ocr_job_id":       frontJobID,
		"name":             "赵六",
	})
	require.NoError(t, err)
	idCardBackOCR, err := json.Marshal(map[string]any{
		"status":     "done",
		"queued_at":  "2026-03-25T18:23:00Z",
		"started_at": "2026-03-25T18:23:02Z",
		"ocr_job_id": backJobID,
	})
	require.NoError(t, err)

	_, err = store.UpdateOperatorApplicationBusinessLicense(ctx, db.UpdateOperatorApplicationBusinessLicenseParams{
		ID:                          app.ID,
		BusinessLicenseMediaAssetID: pgtype.Int8{Valid: false},
		BusinessLicenseNumber:       pgtype.Text{String: "BL-INT-001", Valid: true},
		BusinessLicenseOcr:          businessLicenseOCR,
		Name:                        pgtype.Text{String: "测试运营商", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateOperatorApplicationIDCardFront(ctx, db.UpdateOperatorApplicationIDCardFrontParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{Valid: false},
		LegalPersonName:         pgtype.Text{String: "赵六", Valid: true},
		LegalPersonIDNumber:     pgtype.Text{String: "310101199001011234", Valid: true},
		IDCardFrontOcr:          idCardFrontOCR,
	})
	require.NoError(t, err)
	_, err = store.UpdateOperatorApplicationIDCardBack(ctx, db.UpdateOperatorApplicationIDCardBackParams{
		ID:                     app.ID,
		IDCardBackMediaAssetID: pgtype.Int8{Valid: false},
		IDCardBackOcr:          idCardBackOCR,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, integrationTokenMaker, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorApplicationAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, app.ID, resp.ID)
	require.Equal(t, region.ID, resp.RegionID)
	require.Equal(t, region.Name, resp.RegionName)
	require.NotNil(t, resp.BusinessLicenseOCR)
	require.Equal(t, "done", resp.BusinessLicenseOCR.Status)
	require.Equal(t, "2026-03-25T18:20:00Z", resp.BusinessLicenseOCR.QueuedAt)
	require.Equal(t, "2026-03-25T18:20:03Z", resp.BusinessLicenseOCR.StartedAt)
	require.Equal(t, "2026-03-25T18:20:10Z", resp.BusinessLicenseOCR.AlertEmittedAt)
	require.NotNil(t, resp.BusinessLicenseOCR.OCRJobID)
	require.Equal(t, businessJobID, *resp.BusinessLicenseOCR.OCRJobID)
	require.NotNil(t, resp.IDCardFrontOCR)
	require.Equal(t, "processing", resp.IDCardFrontOCR.Status)
	require.Equal(t, "ocr_retryable_error", resp.IDCardFrontOCR.ErrorCode)
	require.Equal(t, "2026-03-25T18:22:00Z", resp.IDCardFrontOCR.AlertEmittedAt)
	require.Equal(t, "2026-03-25T18:21:00Z", resp.IDCardFrontOCR.QueuedAt)
	require.Equal(t, "2026-03-25T18:21:04Z", resp.IDCardFrontOCR.StartedAt)
	require.Equal(t, "赵六", resp.IDCardFrontOCR.Name)
	require.NotNil(t, resp.IDCardFrontOCR.OCRJobID)
	require.Equal(t, frontJobID, *resp.IDCardFrontOCR.OCRJobID)
	require.NotNil(t, resp.IDCardBackOCR)
	require.Equal(t, "done", resp.IDCardBackOCR.Status)
	require.Equal(t, "2026-03-25T18:23:00Z", resp.IDCardBackOCR.QueuedAt)
	require.Equal(t, "2026-03-25T18:23:02Z", resp.IDCardBackOCR.StartedAt)
	require.NotNil(t, resp.IDCardBackOCR.OCRJobID)
	require.Equal(t, backJobID, *resp.IDCardBackOCR.OCRJobID)
}
