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

func TestProcessTaskGroupApplicationBusinessLicenseOCR_UsesOCRJob(t *testing.T) {
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
	createdAt := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 15, 0, 10, 0, time.UTC)
	baseJob := db.OcrJob{ID: 132, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 232, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 91, Status: string(ocr.JobStatusPending), CreatedAt: createdAt}
	app := db.MerchantGroupApplication{ID: 91, Status: "draft", ApplicationData: []byte(`{"group_name":"测试集团"}`)}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(132)).Return(baseJob, nil),
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
			job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(5 * time.Second), Valid: true}
			return job, nil
		}),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(91)).Return(app, nil),
		store.EXPECT().UpdateGroupApplicationLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateGroupApplicationLicenseParams) (db.MerchantGroupApplication, error) {
			require.Equal(t, int64(91), arg.ID)
			require.Equal(t, "91310000123456789A", arg.LicenseNumber.String)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.ApplicationData, &payload))
			ocrPayload := payload["business_license_ocr"].(map[string]any)
			require.Equal(t, "done", ocrPayload["status"])
			require.Equal(t, float64(132), ocrPayload["ocr_job_id"])
			return db.MerchantGroupApplication{ID: 91}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 91, OCRJobID: 132})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}
