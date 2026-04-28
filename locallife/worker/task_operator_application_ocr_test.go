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
		store.EXPECT().UpdateOperatorApplicationBusinessLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateOperatorApplicationBusinessLicenseParams) (db.OperatorApplication, error) {
			require.Equal(t, int64(69), arg.ID)
			require.Equal(t, "91310000123456789A", arg.BusinessLicenseNumber.String)
			require.False(t, arg.Name.Valid)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
			require.Equal(t, "done", payload["status"])
			require.Equal(t, float64(88), payload["ocr_job_id"])
			return db.OperatorApplication{ID: 69}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 69, OCRJobID: 88})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskOperatorApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}
