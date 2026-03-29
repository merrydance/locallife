package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	mockworker "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type auditSpyWriter struct {
	mu      sync.Mutex
	entries []AuditLogInput
}

func (w *auditSpyWriter) Write(_ *gin.Context, input AuditLogInput) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, input)
}

func (w *auditSpyWriter) Entries() []AuditLogInput {
	w.mu.Lock()
	defer w.mu.Unlock()
	entries := make([]AuditLogInput, len(w.entries))
	copy(entries, w.entries)
	return entries
}

func TestCreateOCRJob_CreatesMerchantBusinessLicenseJob(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)
	job := db.OcrJob{ID: 301, Status: "pending", DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: 501, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: app.ID, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	gomock.InOrder(
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(501)).Return(db.MediaAsset{ID: 501, ModerationStatus: "approved"}, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, int64(501), arg.MediaAssetID)
			require.Equal(t, "merchant_application", arg.OwnerType)
			return job, nil
		}),
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().UpdateMerchantApplicationBusinessLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpdateMerchantApplicationBusinessLicenseParams) (db.MerchantApplication, error) {
			require.Equal(t, app.ID, arg.ID)
			require.True(t, arg.BusinessLicenseMediaAssetID.Valid)
			require.Equal(t, int64(501), arg.BusinessLicenseMediaAssetID.Int64)
			return app, nil
		}),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationBusinessLicenseOCR(gomock.Any(), app.ID, int64(501), int64(301)).Return(nil)

	server := newTestServer(t, store)
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter
	server.SetTaskDistributorForTest(distributor)

	body, err := json.Marshal(createOCRJobRequest{DocumentType: "business_license", MediaAssetID: 501, OwnerType: "merchant_application", OwnerID: app.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ocrJobResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(301), resp.OCRJobID)
	require.Equal(t, "pending", resp.Status)
	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, "ocr_job_created", entries[0].Action)
	require.Equal(t, "ocr_job", entries[0].TargetType)
	require.NotNil(t, entries[0].TargetID)
	require.Equal(t, int64(301), *entries[0].TargetID)
	require.Equal(t, int64(501), entries[0].Metadata["media_asset_id"])
}

func TestCreateOCRJob_SetsIDCardRetentionUntil(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)
	job := db.OcrJob{ID: 311, Status: "pending", DocumentType: string(ocr.DocumentTypeIDCard), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: 601, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: app.ID, Side: string(ocr.DocumentSideFront), CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	gomock.InOrder(
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(601)).Return(db.MediaAsset{ID: 601, ModerationStatus: "approved"}, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, string(ocr.DocumentTypeIDCard), arg.DocumentType)
			require.Equal(t, string(ocr.DocumentSideFront), arg.Side)
			require.True(t, arg.RetentionUntil.Valid)
			retention := time.Until(arg.RetentionUntil.Time)
			require.Greater(t, retention, 6*24*time.Hour)
			require.Less(t, retention, 8*24*time.Hour)
			return job, nil
		}),
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().UpdateMerchantApplicationIDCardFront(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpdateMerchantApplicationIDCardFrontParams) (db.MerchantApplication, error) {
			require.Equal(t, app.ID, arg.ID)
			require.True(t, arg.IDCardFrontMediaAssetID.Valid)
			require.Equal(t, int64(601), arg.IDCardFrontMediaAssetID.Int64)
			return app, nil
		}),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationIDCardOCR(gomock.Any(), app.ID, int64(601), int64(311), "Front").Return(nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(distributor)

	body, err := json.Marshal(createOCRJobRequest{DocumentType: "id_card", MediaAssetID: 601, OwnerType: "merchant_application", OwnerID: app.ID, Side: "front"})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestCreateOCRJob_MarksRiderHealthCertPending(t *testing.T) {
	user, _ := randomUser(t)
	app := randomRiderApplication(user.ID)
	job := db.OcrJob{ID: 321, Status: "pending", DocumentType: string(ocr.DocumentTypeHealthCert), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: 701, OwnerType: string(ocr.OwnerTypeRiderApplication), OwnerID: app.ID, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	gomock.InOrder(
		store.EXPECT().GetRiderApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(701)).Return(db.MediaAsset{ID: 701, ModerationStatus: "approved"}, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, string(ocr.DocumentTypeHealthCert), arg.DocumentType)
			require.Equal(t, string(ocr.OwnerTypeRiderApplication), arg.OwnerType)
			return job, nil
		}),
		store.EXPECT().GetRiderApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().UpdateRiderApplicationHealthCert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpdateRiderApplicationHealthCertParams) (db.RiderApplication, error) {
			require.Equal(t, app.ID, arg.ID)
			require.True(t, arg.HealthCertMediaAssetID.Valid)
			require.Equal(t, int64(701), arg.HealthCertMediaAssetID.Int64)

			var payload HealthCertOCRData
			require.NoError(t, json.Unmarshal(arg.HealthCertOcr, &payload))
			require.Equal(t, "pending", payload.Status)
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, int64(321), *payload.OCRJobID)
			return app, nil
		}),
	)
	distributor.EXPECT().DistributeTaskRiderApplicationHealthCertOCR(gomock.Any(), app.ID, int64(701), int64(321)).Return(nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(distributor)

	body, err := json.Marshal(createOCRJobRequest{DocumentType: "health_cert", MediaAssetID: 701, OwnerType: "rider_application", OwnerID: app.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestCreateOCRJob_MarksGroupBusinessLicensePending(t *testing.T) {
	user, _ := randomUser(t)
	app := randomGroupApplication(user.ID)
	app.ApplicationData = []byte(`{"existing":{"ok":true}}`)
	job := db.OcrJob{ID: 331, Status: "pending", DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: 801, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: app.ID, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	gomock.InOrder(
		store.EXPECT().GetGroupApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(801)).Return(db.MediaAsset{ID: 801, ModerationStatus: "approved"}, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, string(ocr.DocumentTypeBusinessLicense), arg.DocumentType)
			require.Equal(t, string(ocr.OwnerTypeGroupApplication), arg.OwnerType)
			return job, nil
		}),
		store.EXPECT().GetGroupApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().UpdateGroupApplicationLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpdateGroupApplicationLicenseParams) (db.MerchantGroupApplication, error) {
			require.Equal(t, app.ID, arg.ID)
			require.True(t, arg.LicenseMediaAssetID.Valid)
			require.Equal(t, int64(801), arg.LicenseMediaAssetID.Int64)

			var merged map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(arg.ApplicationData, &merged))
			require.Contains(t, merged, "existing")
			require.Contains(t, merged, "business_license_ocr")

			var payload BusinessLicenseOCRData
			require.NoError(t, json.Unmarshal(merged["business_license_ocr"], &payload))
			require.Equal(t, "pending", payload.Status)
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, int64(331), *payload.OCRJobID)
			return app, nil
		}),
	)
	distributor.EXPECT().DistributeTaskGroupApplicationBusinessLicenseOCR(gomock.Any(), app.ID, int64(801), int64(331)).Return(nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(distributor)

	body, err := json.Marshal(createOCRJobRequest{DocumentType: "business_license", MediaAssetID: 801, OwnerType: "group_application", OwnerID: app.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestCreateOCRJob_RejectsPendingModerationMedia(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)

	gomock.InOrder(
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(901)).Return(db.MediaAsset{ID: 901, ModerationStatus: "pending"}, nil),
	)

	server := newTestServer(t, store)
	body, err := json.Marshal(createOCRJobRequest{DocumentType: "business_license", MediaAssetID: 901, OwnerType: "merchant_application", OwnerID: app.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), ErrImageModerationPending.Message)
}

func TestCreateOCRJob_RejectsRejectedModerationMedia(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)

	gomock.InOrder(
		store.EXPECT().GetMerchantApplication(gomock.Any(), app.ID).Return(app, nil),
		store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(902)).Return(db.MediaAsset{ID: 902, ModerationStatus: "rejected"}, nil),
	)

	server := newTestServer(t, store)
	body, err := json.Marshal(createOCRJobRequest{DocumentType: "business_license", MediaAssetID: 902, OwnerType: "merchant_application", OwnerID: app.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), ErrImageContentSafetyFailed.Message)
}

func TestGetOCRJob_ReturnsOwnedJob(t *testing.T) {
	user, _ := randomUser(t)
	job := db.OcrJob{ID: 302, Status: "processing", DocumentType: "business_license", Provider: "aliyun", OwnerType: "merchant_application", OwnerID: 1, RequestedBy: user.ID, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOCRJob(gomock.Any(), int64(302)).Return(job, nil)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/ocr/jobs/302", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ocrJobResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(302), resp.OCRJobID)
	require.Equal(t, "processing", resp.Status)
}

func TestListOCRDeadLetterJobs_AdminCanQuery(t *testing.T) {
	admin, _ := randomUser(t)
	job := db.OcrJob{
		ID:           399,
		Status:       string(ocr.JobStatusFailed),
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameAliyun),
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      88,
		AttemptCount: 3,
		MaxAttempts:  3,
		RequestedBy:  77,
		ErrorCode:    pgtype.Text{String: "ocr_retryable_error", Valid: true},
		CreatedAt:    time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	gomock.InOrder(
		store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil),
		store.EXPECT().ListOCRDeadLetterJobs(gomock.Any(), db.ListOCRDeadLetterJobsParams{
			OwnerType:    string(ocr.OwnerTypeMerchantApplication),
			DocumentType: string(ocr.DocumentTypeIDCard),
			PageLimit:    10,
			PageOffset:   5,
		}).Return([]db.OcrJob{job}, nil),
	)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/ocr/jobs/dead-letter?owner_type=merchant_application&document_type=id_card&limit=10&offset=5", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp listOCRDeadLetterJobsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Jobs, 1)
	require.Equal(t, int64(399), resp.Jobs[0].OCRJobID)
	require.Equal(t, int32(3), resp.Jobs[0].AttemptCount)
	require.Equal(t, "attempts_exhausted", resp.Jobs[0].ManualReason)
	if resp.Jobs[0].ErrorCode == nil {
		t.Fatal("expected error code in dead-letter response")
	}
	require.Equal(t, "ocr_retryable_error", *resp.Jobs[0].ErrorCode)
}

func TestListOCRDeadLetterJobs_RequiresAdmin(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ListUserRoles(gomock.Any(), user.ID).Return([]db.UserRole{{UserID: user.ID, Role: "customer", Status: "active"}}, nil)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/ocr/jobs/dead-letter", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetOCRJobResult_ReturnsNormalizedResult(t *testing.T) {
	user, _ := randomUser(t)
	job := db.OcrJob{ID: 303, Status: "succeeded", ResultVersion: 1, RequestedBy: user.ID, NormalizedResult: []byte(`{"document_type":"business_license","business_license":{"credit_code":"91310000123456789A"}}`)}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOCRJob(gomock.Any(), int64(303)).Return(job, nil)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/ocr/jobs/303/result", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ocrJobResultResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(303), resp.OCRJobID)
	require.Equal(t, "succeeded", resp.Status)
	resultMap, ok := resp.NormalizedResult.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "business_license", resultMap["document_type"])
}

func TestRetryOCRJob_CreatesNewJobAndEnqueues(t *testing.T) {
	user, _ := randomUser(t)
	oldJob := db.OcrJob{ID: 304, Status: string(ocr.JobStatusFailed), AttemptCount: 1, MaxAttempts: 3, IdempotencyKey: "501:business_license:merchant_application:88:", DocumentType: "business_license", Provider: "aliyun", MediaAssetID: 501, OwnerType: "merchant_application", OwnerID: 88, RequestedBy: user.ID, CreatedAt: time.Now()}
	newJob := db.OcrJob{ID: 305, Status: "pending", DocumentType: oldJob.DocumentType, Provider: oldJob.Provider, MediaAssetID: oldJob.MediaAssetID, OwnerType: oldJob.OwnerType, OwnerID: oldJob.OwnerID, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)
	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(304)).Return(oldJob, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.Contains(t, arg.IdempotencyKey, oldJob.IdempotencyKey+":retry:")
			return newJob, nil
		}),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationBusinessLicenseOCR(gomock.Any(), int64(88), int64(501), int64(305)).Return(nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(distributor)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs/304/retry", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ocrJobResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(305), resp.OCRJobID)
}

func TestRetryOCRJob_IDCardSetsRetentionUntil(t *testing.T) {
	user, _ := randomUser(t)
	oldJob := db.OcrJob{ID: 314, Status: string(ocr.JobStatusFailed), AttemptCount: 1, MaxAttempts: 3, IdempotencyKey: "701:id_card:merchant_application:98:front", DocumentType: string(ocr.DocumentTypeIDCard), Provider: "aliyun", MediaAssetID: 701, OwnerType: "merchant_application", OwnerID: 98, Side: string(ocr.DocumentSideFront), RequestedBy: user.ID, CreatedAt: time.Now()}
	newJob := db.OcrJob{ID: 315, Status: "pending", DocumentType: oldJob.DocumentType, Provider: oldJob.Provider, MediaAssetID: oldJob.MediaAssetID, OwnerType: oldJob.OwnerType, OwnerID: oldJob.OwnerID, Side: oldJob.Side, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)
	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(314)).Return(oldJob, nil),
		store.EXPECT().UpsertOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			require.True(t, arg.RetentionUntil.Valid)
			retention := time.Until(arg.RetentionUntil.Time)
			require.Greater(t, retention, 6*24*time.Hour)
			require.Less(t, retention, 8*24*time.Hour)
			return newJob, nil
		}),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationIDCardOCR(gomock.Any(), int64(98), int64(701), int64(315), "Front").Return(nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(distributor)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs/314/retry", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestBatchQueryOCRJobs_ReturnsJobs(t *testing.T) {
	user, _ := randomUser(t)
	job1 := db.OcrJob{ID: 306, Status: "pending", RequestedBy: user.ID, CreatedAt: time.Now()}
	job2 := db.OcrJob{ID: 307, Status: "succeeded", RequestedBy: user.ID, ResultVersion: 1, StartedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(306)).Return(job1, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(307)).Return(job2, nil),
	)

	server := newTestServer(t, store)
	body, err := json.Marshal(batchQueryOCRJobsRequest{JobIDs: []int64{306, 307}})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/ocr/jobs/batch-query", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp batchQueryOCRJobsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Jobs, 2)
	require.Equal(t, int64(306), resp.Jobs[0].OCRJobID)
	require.Equal(t, int64(307), resp.Jobs[1].OCRJobID)
}
