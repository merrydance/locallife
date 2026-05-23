package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskOperatorApplicationBusinessLicenseOCR_UsesOCRJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   stubBusinessLicenseOCRProvider{},
			Capability: ocr.CapabilityWechatBusinessLicense,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{
		store: store,
		ocrService: ocr.NewService(
			store,
			router,
			stubFoodPermitBinaryReader{},
		),
	}
	app := db.OperatorApplication{
		ID:                          69,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 188, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":88}`),
	}

	createdAt := time.Date(2026, 3, 25, 12, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 12, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           88,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 188,
		OwnerType:    string(ocr.OwnerTypeOperatorApplication),
		OwnerID:      69,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(88)).Return(baseJob, nil),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(69)).Return(app, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(88)).Return(baseJob, nil),
		store.EXPECT().MarkOCRJobProcessing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusProcessing)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			return job, nil
		}),
		store.EXPECT().CompleteOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusSucceeded)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			job.NormalizedResult = arg.NormalizedResult
			job.RawResult = arg.RawResult
			job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(10 * time.Second), Valid: true}
			return job, nil
		}),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(69)).Return(app, nil),
		store.EXPECT().UpdateOperatorApplicationBusinessLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateOperatorApplicationBusinessLicenseParams) (db.OperatorApplication, error) {
			require.Equal(t, int64(69), arg.ID)
			require.Equal(t, "91310000123456789A", arg.BusinessLicenseNumber.String)
			require.False(t, arg.Name.Valid)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
			require.Equal(t, "done", payload["status"])
			require.Equal(t, float64(88), payload["ocr_job_id"])
			return app, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 69, MediaAssetID: 188, OCRJobID: 88})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskOperatorApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOperatorApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   stubBusinessLicenseOCRProvider{},
			Capability: ocr.CapabilityWechatBusinessLicense,
		},
	})
	require.NoError(t, err)
	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	app := db.OperatorApplication{
		ID:                          6901,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 18802, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":8801}`),
	}

	job := db.OcrJob{
		ID:           8801,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 18801,
		OwnerType:    string(ocr.OwnerTypeOperatorApplication),
		OwnerID:      6901,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    time.Now(),
	}
	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(8801)).Return(job, nil),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(6901)).Return(app, nil),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 6901, MediaAssetID: 18802, OCRJobID: 8801})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskOperatorApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOperatorApplicationBusinessLicenseOCR_SkipsNonDraftApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   stubBusinessLicenseOCRProvider{},
			Capability: ocr.CapabilityWechatBusinessLicense,
		},
	})
	require.NoError(t, err)
	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	app := db.OperatorApplication{
		ID:                          6902,
		Status:                      "submitted",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 18803, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":8802}`),
	}

	job := db.OcrJob{
		ID:           8802,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 18803,
		OwnerType:    string(ocr.OwnerTypeOperatorApplication),
		OwnerID:      6902,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    time.Now(),
	}
	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(8802)).Return(job, nil),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(6902)).Return(app, nil),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 6902, MediaAssetID: 18803, OCRJobID: 8802})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskOperatorApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOperatorApplicationBusinessLicenseOCR_SkipsStaleAssetAfterProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   stubBusinessLicenseOCRProvider{},
			Capability: ocr.CapabilityWechatBusinessLicense,
		},
	})
	require.NoError(t, err)
	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	initialApp := db.OperatorApplication{
		ID:                          6903,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 18804, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":8803}`),
	}
	updatedApp := initialApp
	updatedApp.BusinessLicenseMediaAssetID = pgtype.Int8{Int64: 18805, Valid: true}

	createdAt := time.Date(2026, 3, 25, 13, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 13, 0, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           8803,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 18804,
		OwnerType:    string(ocr.OwnerTypeOperatorApplication),
		OwnerID:      6903,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(8803)).Return(baseJob, nil),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(6903)).Return(initialApp, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(8803)).Return(baseJob, nil),
		store.EXPECT().MarkOCRJobProcessing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusProcessing)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			return job, nil
		}),
		store.EXPECT().CompleteOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusSucceeded)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			job.NormalizedResult = arg.NormalizedResult
			job.RawResult = arg.RawResult
			job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(10 * time.Second), Valid: true}
			return job, nil
		}),
		store.EXPECT().GetOperatorApplicationByID(gomock.Any(), int64(6903)).Return(updatedApp, nil),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 6903, MediaAssetID: 18804, OCRJobID: 8803})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskOperatorApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}
