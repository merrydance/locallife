package worker

import (
	"context"
	"encoding/json"
	"errors"
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

type failingIDCardOCRProvider struct{}

func (failingIDCardOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

func (failingIDCardOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	return ocr.RecognizeResponse{}, errors.New("provider failed")
}

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
	app := db.MerchantGroupApplication{
		ID:                  91,
		Status:              "draft",
		LicenseMediaAssetID: pgtype.Int8{Int64: 232, Valid: true},
		ApplicationData:     []byte(`{"business_license_ocr":{"status":"pending","ocr_job_id":132}}`),
	}
	createdAt := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 15, 0, 10, 0, time.UTC)
	baseJob := db.OcrJob{ID: 132, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 232, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 91, Status: string(ocr.JobStatusPending), CreatedAt: createdAt}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(132)).Return(baseJob, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(91)).Return(app, nil),
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
			return app, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 91, MediaAssetID: 232, OCRJobID: 132})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider(t *testing.T) {
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
	job := db.OcrJob{ID: 13201, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23201, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9101, Status: string(ocr.JobStatusPending), CreatedAt: time.Now()}
	app := db.MerchantGroupApplication{
		ID:                  9101,
		Status:              "draft",
		LicenseMediaAssetID: pgtype.Int8{Int64: 23202, Valid: true},
		ApplicationData:     []byte(`{"business_license_ocr":{"status":"pending","ocr_job_id":13201}}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13201)).Return(job, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9101)).Return(app, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9101, MediaAssetID: 23201, OCRJobID: 13201})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsNonDraftApplication(t *testing.T) {
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
	job := db.OcrJob{ID: 13202, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23203, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9102, Status: string(ocr.JobStatusPending), CreatedAt: time.Now()}
	app := db.MerchantGroupApplication{
		ID:                  9102,
		Status:              "submitted",
		LicenseMediaAssetID: pgtype.Int8{Int64: 23203, Valid: true},
		ApplicationData:     []byte(`{"business_license_ocr":{"status":"pending","ocr_job_id":13202}}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13202)).Return(job, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9102)).Return(app, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9102, MediaAssetID: 23203, OCRJobID: 13202})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsStaleAssetAfterProvider(t *testing.T) {
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
	initialApp := db.MerchantGroupApplication{
		ID:                  9103,
		Status:              "draft",
		LicenseMediaAssetID: pgtype.Int8{Int64: 23204, Valid: true},
		ApplicationData:     []byte(`{"business_license_ocr":{"status":"pending","ocr_job_id":13203}}`),
	}
	updatedApp := initialApp
	updatedApp.LicenseMediaAssetID = pgtype.Int8{Int64: 23205, Valid: true}

	createdAt := time.Date(2026, 3, 25, 15, 30, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 15, 30, 10, 0, time.UTC)
	baseJob := db.OcrJob{ID: 13203, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23204, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9103, Status: string(ocr.JobStatusPending), CreatedAt: createdAt}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13203)).Return(baseJob, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9103)).Return(initialApp, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13203)).Return(baseJob, nil),
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
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9103)).Return(updatedApp, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9103, MediaAssetID: 23204, OCRJobID: 13203})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationIDCardOCR_UsesOCRJob(t *testing.T) {
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

	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	createdAt := time.Date(2026, 3, 25, 16, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 16, 0, 10, 0, time.UTC)
	baseJob := db.OcrJob{ID: 133, DocumentType: string(ocr.DocumentTypeIDCard), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 233, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 92, Status: string(ocr.JobStatusPending), Side: string(ocr.DocumentSideFront), CreatedAt: createdAt}
	app := db.MerchantGroupApplication{ID: 92, Status: "draft", ApplicationData: []byte(`{"group_name":"测试集团","id_card_front_asset_id":233,"id_card_front_ocr":{"status":"pending","ocr_job_id":133}}`)}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(133)).Return(baseJob, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(92)).Return(app, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(133)).Return(baseJob, nil),
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
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(92)).Return(app, nil),
		store.EXPECT().UpdateGroupApplicationLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateGroupApplicationLicenseParams) (db.MerchantGroupApplication, error) {
			require.Equal(t, int64(92), arg.ID)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.ApplicationData, &payload))
			require.Equal(t, "张三", payload["legal_person_name"])
			require.Equal(t, "110101199001011234", payload["legal_person_id_number"])
			ocrPayload := payload["id_card_front_ocr"].(map[string]any)
			require.Equal(t, "done", ocrPayload["status"])
			require.Equal(t, float64(133), ocrPayload["ocr_job_id"])
			return app, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 92, MediaAssetID: 233, OCRJobID: 133, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationIDCardOCR, payload)
	err = processor.ProcessTaskGroupApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationIDCardOCR_SkipsStaleAssetBeforeProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeIDCard: {
			Provider:   failingIDCardOCRProvider{},
			Capability: ocr.CapabilityWechatIDCard,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	job := db.OcrJob{ID: 13301, DocumentType: string(ocr.DocumentTypeIDCard), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23301, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9201, Status: string(ocr.JobStatusPending), Side: string(ocr.DocumentSideFront), CreatedAt: time.Now()}
	app := db.MerchantGroupApplication{
		ID:              9201,
		Status:          "draft",
		ApplicationData: []byte(`{"id_card_front_asset_id":23302,"id_card_front_ocr":{"status":"pending","ocr_job_id":13301}}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13301)).Return(job, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9201)).Return(app, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9201, MediaAssetID: 23301, OCRJobID: 13301, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationIDCardOCR, payload)
	err = processor.ProcessTaskGroupApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationIDCardOCR_SkipsStaleFailureAfterProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeIDCard: {
			Provider:   failingIDCardOCRProvider{},
			Capability: ocr.CapabilityWechatIDCard,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{store: store, ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{})}
	initialApp := db.MerchantGroupApplication{
		ID:              9202,
		Status:          "draft",
		ApplicationData: []byte(`{"id_card_front_asset_id":23303,"id_card_front_ocr":{"status":"pending","ocr_job_id":13303}}`),
	}
	updatedApp := initialApp
	updatedApp.ApplicationData = []byte(`{"id_card_front_asset_id":23304,"id_card_front_ocr":{"status":"pending","ocr_job_id":13303}}`)
	createdAt := time.Date(2026, 3, 25, 16, 30, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 16, 30, 10, 0, time.UTC)
	baseJob := db.OcrJob{ID: 13303, DocumentType: string(ocr.DocumentTypeIDCard), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23303, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9202, Status: string(ocr.JobStatusPending), Side: string(ocr.DocumentSideFront), CreatedAt: createdAt}
	failedJob := baseJob
	failedJob.Status = string(ocr.JobStatusFailed)
	failedJob.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13303)).Return(baseJob, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9202)).Return(initialApp, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13303)).Return(baseJob, nil),
		store.EXPECT().MarkOCRJobProcessing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusProcessing)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			return job, nil
		}),
		store.EXPECT().FailOCRJob(gomock.Any(), gomock.Any()).Return(failedJob, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9202)).Return(updatedApp, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9202, MediaAssetID: 23303, OCRJobID: 13303, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationIDCardOCR, payload)
	err = processor.ProcessTaskGroupApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsMalformedApplicationDataBeforeProvider(t *testing.T) {
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
	job := db.OcrJob{ID: 13202, DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameWechat), MediaAssetID: 23203, OwnerType: string(ocr.OwnerTypeGroupApplication), OwnerID: 9102, Status: string(ocr.JobStatusPending), CreatedAt: time.Now()}
	app := db.MerchantGroupApplication{
		ID:                  9102,
		Status:              "draft",
		LicenseMediaAssetID: pgtype.Int8{Int64: 23203, Valid: true},
		ApplicationData:     []byte(`{"business_license_ocr":`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(13202)).Return(job, nil),
		store.EXPECT().GetGroupApplication(gomock.Any(), int64(9102)).Return(app, nil),
	)

	payload, err := json.Marshal(groupApplicationOCRPayload{ApplicationID: 9102, MediaAssetID: 23203, OCRJobID: 13202})
	require.NoError(t, err)
	task := asynq.NewTask(TaskGroupApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskGroupApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}
