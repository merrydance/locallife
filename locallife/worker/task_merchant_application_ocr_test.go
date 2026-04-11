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

type stubFoodPermitOCRProvider struct{}

func (stubFoodPermitOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

func (stubFoodPermitOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityWechatPrintedText {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	rawText := "经营者名称：本地生活餐饮店\n许可证编号：JY12345678901234\n有效期至：2027年01月08日"
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"ok"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeFoodPermit,
			RecognizedAt: recognizedAt,
			FoodPermit: &ocr.FoodPermitResult{
				LicenseNumber: "JY12345678901234",
				BusinessName:  "本地生活餐饮店",
				ValidPeriod:   "2026年01月08日至2027年01月08日",
				RawText:       rawText,
			},
		},
	}, nil
}

type stubStructuredFoodPermitOCRProvider struct{}

func (stubStructuredFoodPermitOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameAliyun
}

func (stubStructuredFoodPermitOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityAliyunFoodPermit {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 10, 5, 0, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"structured":true}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeFoodPermit,
			RecognizedAt: recognizedAt,
			FoodPermit: &ocr.FoodPermitResult{
				LicenseNumber: "JY99887766554433",
				BusinessName:  "本地生活轻食店",
				OperatorName:  "王五",
				ValidPeriod:   "2025年01月08日至2030年12月31日",
				RawText:       "经营场所：北京市朝阳区测试路100号1楼",
			},
		},
	}, nil
}

type stubFoodPermitBinaryReader struct{}

func (stubFoodPermitBinaryReader) ReadMediaAsset(_ context.Context, mediaAssetID int64) ([]byte, string, error) {
	return []byte("fake-image"), "image/jpeg", nil
}

type stubBusinessLicenseOCRProvider struct{}

func (stubBusinessLicenseOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

type stubIDCardOCRProvider struct{}

func (stubIDCardOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

func (stubIDCardOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityWechatIDCard {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	result := &ocr.IDCardResult{}
	if req.Side == ocr.DocumentSideBack {
		result.ValidPeriod = "2020.01.01-2030.01.01"
	} else {
		result.Name = "张三"
		result.IDNumber = "110101199001011234"
		result.Gender = "男"
		result.Ethnicity = "汉"
		result.Address = "测试路1号"
	}
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"ok"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeIDCard,
			Side:         req.Side,
			RecognizedAt: recognizedAt,
			IDCard:       result,
		},
	}, nil
}

func expectOCRSuccessAuditLog(t *testing.T, store *mockdb.MockStore, job db.OcrJob) {
	t.Helper()
	store.EXPECT().
		CreateAuditLog(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			require.Equal(t, "system", arg.ActorRole)
			require.Equal(t, "ocr_job_succeeded", arg.Action)
			require.Equal(t, "ocr_job", arg.TargetType)
			require.True(t, arg.TargetID.Valid)
			require.Equal(t, job.ID, arg.TargetID.Int64)
			var metadata map[string]any
			require.NoError(t, json.Unmarshal(arg.Metadata, &metadata))
			require.Equal(t, float64(job.ID), metadata["ocr_job_id"])
			require.Equal(t, "succeeded", metadata["status"])
			require.Equal(t, job.OwnerType, metadata["owner_type"])
			return db.AuditLog{ID: 1}, nil
		})
}

func (stubBusinessLicenseOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityWechatBusinessLicense {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 11, 0, 0, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"ok"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeBusinessLicense,
			RecognizedAt: recognizedAt,
			BusinessLicense: &ocr.BusinessLicenseResult{
				CreditCode:          "91310000123456789A",
				RegistrationNumber:  "91310000123456789A",
				EnterpriseName:      "本地生活科技有限公司",
				LegalRepresentative: "张三",
				Address:             "测试路1号",
				BusinessScope:       "餐饮服务",
				ValidPeriod:         "2020-01-01 至 2040-01-01",
			},
		},
	}, nil
}

func TestProcessTaskMerchantApplicationFoodPermitOCR_UsesOCRJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeFoodPermit: {
			Provider:   stubFoodPermitOCRProvider{},
			Capability: ocr.CapabilityWechatPrintedText,
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

	createdAt := time.Date(2026, 3, 25, 9, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 9, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           77,
		DocumentType: string(ocr.DocumentTypeFoodPermit),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 88,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      66,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().
			GetOCRJob(gomock.Any(), int64(77)).
			Return(baseJob, nil),
		store.EXPECT().
			MarkOCRJobProcessing(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
				require.Equal(t, int64(77), arg.ID)
				require.True(t, arg.LeaseOwner.Valid)
				require.Equal(t, "worker:merchant_food_permit", arg.LeaseOwner.String)
				job := baseJob
				job.Status = string(ocr.JobStatusProcessing)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				return job, nil
			}),
		store.EXPECT().
			CompleteOCRJob(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
				require.Equal(t, int64(77), arg.ID)
				require.NotEmpty(t, arg.RawResult)
				require.NotEmpty(t, arg.NormalizedResult)
				job := baseJob
				job.Status = string(ocr.JobStatusSucceeded)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				job.NormalizedResult = arg.NormalizedResult
				job.RawResult = arg.RawResult
				job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(15 * time.Second), Valid: true}
				return job, nil
			}),
		store.EXPECT().
			UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(66), arg.ID)
				var payload foodPermitOCRData
				require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &payload))
				require.Equal(t, "done", payload.Status)
				require.NotNil(t, payload.OCRJobID)
				require.Equal(t, int64(77), *payload.OCRJobID)
				require.Equal(t, "JY12345678901234", payload.PermitNo)
				require.Equal(t, "本地生活餐饮店", payload.CompanyName)
				require.Equal(t, "2027年01月08日", payload.ValidTo)
				require.Contains(t, payload.RawText, "许可证编号")
				return db.MerchantApplication{ID: 66}, nil
			}),
		store.EXPECT().
			CreateAuditLog(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				require.Equal(t, "system", arg.ActorRole)
				require.Equal(t, "ocr_job_succeeded", arg.Action)
				require.Equal(t, "ocr_job", arg.TargetType)
				require.True(t, arg.TargetID.Valid)
				require.Equal(t, int64(77), arg.TargetID.Int64)
				var metadata map[string]any
				require.NoError(t, json.Unmarshal(arg.Metadata, &metadata))
				require.Equal(t, float64(77), metadata["ocr_job_id"])
				require.Equal(t, "succeeded", metadata["status"])
				require.Equal(t, "merchant_application", metadata["owner_type"])
				return db.AuditLog{ID: 1}, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: 66,
		OCRJobID:      77,
	})
	require.NoError(t, err)

	task := asynq.NewTask(TaskMerchantApplicationFoodPermitOCR, payload)
	err = processor.ProcessTaskMerchantApplicationFoodPermitOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationFoodPermitOCR_PrefersStructuredFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeFoodPermit: {
			Provider:   stubStructuredFoodPermitOCRProvider{},
			Capability: ocr.CapabilityAliyunFoodPermit,
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

	createdAt := time.Date(2026, 3, 25, 10, 4, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 10, 4, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           88,
		DocumentType: string(ocr.DocumentTypeFoodPermit),
		Provider:     string(ocr.ProviderNameAliyun),
		MediaAssetID: 99,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      77,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(88)).Return(baseJob, nil),
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
			job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(6 * time.Second), Valid: true}
			return job, nil
		}),
		store.EXPECT().UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
			var payload foodPermitOCRData
			require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &payload))
			require.Equal(t, "JY99887766554433", payload.PermitNo)
			require.Equal(t, "本地生活轻食店", payload.CompanyName)
			require.Equal(t, "王五", payload.OperatorName)
			require.Equal(t, "2025年01月08日", payload.ValidFrom)
			require.Equal(t, "2030年12月31日", payload.ValidTo)
			require.Equal(t, "经营场所：北京市朝阳区测试路100号1楼", payload.RawText)
			return db.MerchantApplication{ID: 77}, nil
		}),
	)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{
		ID:        88,
		Status:    string(ocr.JobStatusSucceeded),
		OwnerType: string(ocr.OwnerTypeMerchantApplication),
		Provider:  string(ocr.ProviderNameAliyun),
	})

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 77, OCRJobID: 88})
	require.NoError(t, err)

	task := asynq.NewTask(TaskMerchantApplicationFoodPermitOCR, payload)
	err = processor.ProcessTaskMerchantApplicationFoodPermitOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestParseFoodPermitOCRText_ExtractsLikelyCompanyName(t *testing.T) {
	t.Parallel()

	var data foodPermitOCRData
	parseFoodPermitOCRText(&data, "经营者名称：测试餐饮有限公司\n许可证编号：JY12345678901234\n有效期至：2027年01月08日")

	require.Equal(t, "测试餐饮有限公司", data.CompanyName)
	require.Equal(t, "JY12345678901234", data.PermitNo)
	require.Equal(t, "2027年01月08日", data.ValidTo)
}

func TestParseFoodPermitOCRText_RejectsSuspiciousCompanyName(t *testing.T) {
	t.Parallel()

	var data foodPermitOCRData
	parseFoodPermitOCRText(&data, "经营者名称：地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东\n许可证编号：JY12345678901234\n有效期至：2027年01月08日")

	require.Empty(t, data.CompanyName)
	require.Equal(t, "JY12345678901234", data.PermitNo)
	require.Equal(t, "2027年01月08日", data.ValidTo)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_UsesOCRJob(t *testing.T) {
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

	createdAt := time.Date(2026, 3, 25, 10, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 10, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           78,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 98,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      67,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().
			GetOCRJob(gomock.Any(), int64(78)).
			Return(baseJob, nil),
		store.EXPECT().
			MarkOCRJobProcessing(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
				require.Equal(t, int64(78), arg.ID)
				job := baseJob
				job.Status = string(ocr.JobStatusProcessing)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				return job, nil
			}),
		store.EXPECT().
			CompleteOCRJob(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
				job := baseJob
				job.Status = string(ocr.JobStatusSucceeded)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				job.NormalizedResult = arg.NormalizedResult
				job.RawResult = arg.RawResult
				job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(10 * time.Second), Valid: true}
				return job, nil
			}),
		store.EXPECT().
			UpdateMerchantApplicationBusinessLicense(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(67), arg.ID)
				require.Equal(t, "91310000123456789A", arg.BusinessLicenseNumber.String)
				require.Equal(t, "餐饮服务", arg.BusinessScope.String)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
				require.Equal(t, "done", payload["status"])
				require.Equal(t, float64(78), payload["ocr_job_id"])
				require.Equal(t, "本地生活科技有限公司", payload["enterprise_name"])
				return db.MerchantApplication{ID: 67}, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 67, OCRJobID: 78})
	require.NoError(t, err)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 78, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationIDCardOCR_UsesOCRJob(t *testing.T) {
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
		store: store,
		ocrService: ocr.NewService(
			store,
			router,
			stubFoodPermitBinaryReader{},
		),
	}

	createdAt := time.Date(2026, 3, 25, 11, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 11, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           79,
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 99,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      68,
		Status:       string(ocr.JobStatusPending),
		Side:         string(ocr.DocumentSideFront),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(79)).Return(baseJob, nil),
		store.EXPECT().
			MarkOCRJobProcessing(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
				job := baseJob
				job.Status = string(ocr.JobStatusProcessing)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				return job, nil
			}),
		store.EXPECT().
			CompleteOCRJob(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
				job := baseJob
				job.Status = string(ocr.JobStatusSucceeded)
				job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
				job.NormalizedResult = arg.NormalizedResult
				job.RawResult = arg.RawResult
				job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(10 * time.Second), Valid: true}
				return job, nil
			}),
		store.EXPECT().
			UpdateMerchantApplicationIDCardFront(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationIDCardFrontParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(68), arg.ID)
				require.Equal(t, "张三", arg.LegalPersonName.String)
				require.Equal(t, "110101199001011234", arg.LegalPersonIDNumber.String)
				var payload merchantIDCardOCRData
				require.NoError(t, json.Unmarshal(arg.IDCardFrontOcr, &payload))
				require.Equal(t, "done", payload.Status)
				require.NotNil(t, payload.OCRJobID)
				require.Equal(t, int64(79), *payload.OCRJobID)
				require.Equal(t, "张三", payload.Name)
				return db.MerchantApplication{ID: 68}, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 68, OCRJobID: 79, Side: "Front"})
	require.NoError(t, err)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 79, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

	task := asynq.NewTask(TaskMerchantApplicationIDCardOCR, payload)
	err = processor.ProcessTaskMerchantApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}
