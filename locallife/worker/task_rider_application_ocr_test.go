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

type stubHealthCertOCRProvider struct{}

func (stubHealthCertOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

func (stubHealthCertOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityWechatPrintedText && capability != ocr.CapabilityAliyunHealthCert {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 14, 45, 0, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"ok"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeHealthCert,
			RecognizedAt: recognizedAt,
			HealthCert: &ocr.HealthCertResult{
				RawText:     "姓名：张三\n健康证号：JK20260001\n有效期至：2030年12月31日\n110101199001011234",
				Certificate: "JK20260001",
			},
		},
	}, nil
}

type noisyStructuredHealthCertOCRProvider struct{}

func (noisyStructuredHealthCertOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameAliyun
}

func (noisyStructuredHealthCertOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityAliyunHealthCert {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 4, 13, 4, 56, 56, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"ok"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeHealthCert,
			RecognizedAt: recognizedAt,
			HealthCert: &ocr.HealthCertResult{
				Name:        "人员健康合格证明安康姓全名周松涛",
				ValidPeriod: "人员健康合格证明 2026.12.06",
				RawText:     "姓名：周松涛性别：男\n从业类别：食品\n证书号：1305282025D590\n有效期至：2026.12.06",
			},
		},
	}, nil
}

func TestProcessTaskRiderApplicationIDCardOCR_UsesOCRJob(t *testing.T) {
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

	createdAt := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 14, 30, 10, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           130,
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 230,
		OwnerType:    string(ocr.OwnerTypeRiderApplication),
		OwnerID:      81,
		Status:       string(ocr.JobStatusPending),
		Side:         string(ocr.DocumentSideFront),
		CreatedAt:    createdAt,
	}
	app := db.RiderApplication{
		ID:                      81,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 230, Valid: true},
		IDCardOcr:               []byte(`{"valid_end":"长期"}`),
		Status:                  "draft",
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(130)).Return(baseJob, nil),
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
		store.EXPECT().GetRiderApplication(gomock.Any(), int64(81)).Return(app, nil),
		store.EXPECT().UpdateRiderApplicationIDCard(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateRiderApplicationIDCardParams) (db.RiderApplication, error) {
			require.Equal(t, int64(81), arg.ID)
			require.Equal(t, "张三", arg.RealName.String)
			var payload riderIDCardOCRData
			require.NoError(t, json.Unmarshal(arg.IDCardOcr, &payload))
			require.Equal(t, "done", payload.Status)
			require.Equal(t, "张三", payload.Name)
			require.Equal(t, "长期", payload.ValidEnd)
			require.NotNil(t, payload.Readiness)
			require.Equal(t, ocrReadinessStateReady, payload.Readiness.State)
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, int64(130), *payload.OCRJobID)
			return db.RiderApplication{ID: 81}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(riderApplicationOCRPayload{ApplicationID: 81, MediaAssetID: 230, OCRJobID: 130, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskRiderApplicationIDCardOCR, payload)
	err = processor.ProcessTaskRiderApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskRiderApplicationIDCardOCR_SkipsStaleBackWriteback(t *testing.T) {
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

	createdAt := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 15, 0, 10, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           132,
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 232,
		OwnerType:    string(ocr.OwnerTypeRiderApplication),
		OwnerID:      83,
		Status:       string(ocr.JobStatusPending),
		Side:         string(ocr.DocumentSideBack),
		CreatedAt:    createdAt,
	}
	app := db.RiderApplication{
		ID:                      83,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 231, Valid: true},
		Status:                  "draft",
	}

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
		store.EXPECT().GetRiderApplication(gomock.Any(), int64(83)).Return(app, nil),
	)

	payload, err := json.Marshal(riderApplicationOCRPayload{ApplicationID: 83, MediaAssetID: 232, OCRJobID: 132, Side: "Back"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskRiderApplicationIDCardOCR, payload)
	err = processor.ProcessTaskRiderApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskRiderApplicationHealthCertOCR_UsesOCRJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeHealthCert: {
			Provider:   stubHealthCertOCRProvider{},
			Capability: ocr.CapabilityWechatPrintedText,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{
		store:      store,
		ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{}),
	}

	createdAt := time.Date(2026, 3, 25, 14, 40, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 14, 40, 10, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           131,
		DocumentType: string(ocr.DocumentTypeHealthCert),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 231,
		OwnerType:    string(ocr.OwnerTypeRiderApplication),
		OwnerID:      82,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}
	app := db.RiderApplication{
		ID:                     82,
		HealthCertMediaAssetID: pgtype.Int8{Int64: 231, Valid: true},
		HealthCertOcr:          []byte(`{"name":"张三"}`),
		Status:                 "draft",
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(131)).Return(baseJob, nil),
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
		store.EXPECT().GetRiderApplication(gomock.Any(), int64(82)).Return(app, nil),
		store.EXPECT().UpdateRiderApplicationHealthCert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateRiderApplicationHealthCertParams) (db.RiderApplication, error) {
			require.Equal(t, int64(82), arg.ID)
			var payload riderHealthCertOCRData
			require.NoError(t, json.Unmarshal(arg.HealthCertOcr, &payload))
			require.Equal(t, "done", payload.Status)
			require.Equal(t, "张三", payload.Name)
			require.Equal(t, "JK20260001", payload.CertNumber)
			require.Equal(t, "2030年12月31日", payload.ValidEnd)
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, int64(131), *payload.OCRJobID)
			return db.RiderApplication{ID: 82}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			return db.AuditLog{ID: 1}, nil
		}),
	)

	payload, err := json.Marshal(riderApplicationOCRPayload{ApplicationID: 82, MediaAssetID: 231, OCRJobID: 131})
	require.NoError(t, err)
	task := asynq.NewTask(TaskRiderApplicationHealthCertOCR, payload)
	err = processor.ProcessTaskRiderApplicationHealthCertOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskRiderApplicationHealthCertOCR_PrefersParsedRawTextOverNoisyStructuredFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeHealthCert: {
			Provider:   noisyStructuredHealthCertOCRProvider{},
			Capability: ocr.CapabilityAliyunHealthCert,
		},
	})
	require.NoError(t, err)

	processor := &RedisTaskProcessor{
		store:      store,
		ocrService: ocr.NewService(store, router, stubFoodPermitBinaryReader{}),
	}

	createdAt := time.Date(2026, 4, 13, 4, 56, 0, 0, time.UTC)
	startedAt := time.Date(2026, 4, 13, 4, 56, 10, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           117,
		DocumentType: string(ocr.DocumentTypeHealthCert),
		Provider:     string(ocr.ProviderNameAliyun),
		MediaAssetID: 208,
		OwnerType:    string(ocr.OwnerTypeRiderApplication),
		OwnerID:      12,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}
	app := db.RiderApplication{
		ID:                     12,
		HealthCertMediaAssetID: pgtype.Int8{Int64: 208, Valid: true},
		HealthCertOcr:          []byte(`{"name":"人员健康合格证明安康姓全名周松涛"}`),
		Status:                 "draft",
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(117)).Return(baseJob, nil),
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
		store.EXPECT().GetRiderApplication(gomock.Any(), int64(12)).Return(app, nil),
		store.EXPECT().UpdateRiderApplicationHealthCert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateRiderApplicationHealthCertParams) (db.RiderApplication, error) {
			var payload riderHealthCertOCRData
			require.NoError(t, json.Unmarshal(arg.HealthCertOcr, &payload))
			require.Equal(t, "周松涛", payload.Name)
			require.Equal(t, "1305282025D590", payload.CertNumber)
			require.Equal(t, "2026.12.06", payload.ValidEnd)
			require.NotNil(t, payload.Readiness)
			require.Equal(t, ocrReadinessStateReady, payload.Readiness.State)
			return db.RiderApplication{ID: 12}, nil
		}),
		store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).Return(db.AuditLog{ID: 1}, nil),
	)

	payload, err := json.Marshal(riderApplicationOCRPayload{ApplicationID: 12, MediaAssetID: 208, OCRJobID: 117})
	require.NoError(t, err)
	task := asynq.NewTask(TaskRiderApplicationHealthCertOCR, payload)
	err = processor.ProcessTaskRiderApplicationHealthCertOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestParseRiderHealthCertOCRText_ExtractsFlexibleValidEnd(t *testing.T) {
	testCases := []struct {
		name            string
		text            string
		expectName      string
		expectCertNo    string
		expectValidFrom string
		expectValidEnd  string
	}{
		{
			name:           "ExplicitDeadlineWithDash",
			text:           "姓名：张三\n健康证号：JK20260001\n有效截止日期：2030-12-31",
			expectName:     "张三",
			expectCertNo:   "JK20260001",
			expectValidEnd: "2030-12-31",
		},
		{
			name:            "DateRangeWithDots",
			text:            "有效期：2024.01.01-2030.12.31",
			expectValidFrom: "2024.01.01",
			expectValidEnd:  "2030.12.31",
		},
		{
			name:           "DeadlineWithSpacesAndSlash",
			text:           "有效期限至: 2030 / 12 / 31",
			expectValidEnd: "2030/12/31",
		},
		{
			name:           "RealHealthCertLayoutWithGenderAndCertNo",
			text:           "姓名：周松涛性别：男\n从业类别：食品\n证书号：1305282025D590\n有效期至：2026.12.06",
			expectName:     "周松涛",
			expectCertNo:   "1305282025D590",
			expectValidEnd: "2026.12.06",
		},
		{
			name:           "FallbackToLastDateWhenLabelIsMissing",
			text:           "周松涛\n男\n食品\n1305282025D590\n2026.12.06",
			expectName:     "周松涛",
			expectValidEnd: "2026.12.06",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data riderHealthCertOCRData
			parseRiderHealthCertOCRText(&data, tc.text)
			require.Equal(t, tc.expectName, data.Name)
			require.Equal(t, tc.expectCertNo, data.CertNumber)
			require.Equal(t, tc.expectValidFrom, data.ValidStart)
			require.Equal(t, tc.expectValidEnd, data.ValidEnd)
		})
	}
}

func TestReadRiderIDCardOCR_DecodesStringifiedPayload(t *testing.T) {
	data := []byte(`"{\"name\":\"张三\",\"id_number\":\"110101199001011234\"}"`)
	payload := readRiderIDCardOCR(data)
	require.Equal(t, "张三", payload.Name)
	require.Equal(t, "110101199001011234", payload.IDNumber)
}
