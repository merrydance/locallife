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

func TestProcessTaskOperatorApplicationIDCardOCR_UsesOCRJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeIDCard: {
			Provider:   stubIDCardOCRProvider{},
			Capability: ocr.CapabilityWechatIDCard,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{
		store:      store,
		ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{}),
	}

	createdAt := time.Date(2026, 3, 25, 13, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 13, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           110,
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 210,
		OwnerType:    string(ocr.OwnerTypeOperatorApplication),
		OwnerID:      70,
		Status:       string(ocr.JobStatusPending),
		Side:         string(ocr.DocumentSideFront),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(110)).Return(baseJob, nil),
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
		store.EXPECT().UpdateOperatorApplicationIDCardFront(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateOperatorApplicationIDCardFrontParams) (db.OperatorApplication, error) {
			require.Equal(t, int64(70), arg.ID)
			require.Equal(t, "张三", arg.LegalPersonName.String)
			require.Equal(t, "110101199001011234", arg.LegalPersonIDNumber.String)
			var payload operatorIDCardFrontOCRData
			require.NoError(t, json.Unmarshal(arg.IDCardFrontOcr, &payload))
			require.Equal(t, "done", payload.Status)
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, int64(110), *payload.OCRJobID)
			return db.OperatorApplication{ID: 70}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(operatorApplicationOCRPayload{ApplicationID: 70, OCRJobID: 110, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskOperatorApplicationIDCardOCR, payload)
	err = processor.ProcessTaskOperatorApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}
