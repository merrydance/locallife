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

type stubPartialBusinessLicenseOCRProvider struct{}

func (stubPartialBusinessLicenseOCRProvider) Name() ocr.ProviderName {
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

func (stubPartialBusinessLicenseOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityWechatBusinessLicense {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 11, 5, 0, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"text":"partial"}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeBusinessLicense,
			RecognizedAt: recognizedAt,
			BusinessLicense: &ocr.BusinessLicenseResult{
				CreditCode:          "92310000123456789B",
				RegistrationNumber:  "92310000123456789B",
				EnterpriseName:      "本地生活小馆",
				LegalRepresentative: "李四",
				Address:             "测试路2号",
				BusinessScope:       "餐饮服务",
			},
		},
	}, nil
}

type failingBusinessLicenseOCRProvider struct{}

func (failingBusinessLicenseOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameWechat
}

func (failingBusinessLicenseOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	return ocr.RecognizeResponse{}, errors.New("provider failed")
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
	app := db.MerchantApplication{
		ID:                     66,
		Status:                 "draft",
		FoodPermitOcr:          []byte(`{"status":"pending","ocr_job_id":77}`),
		FoodPermitMediaAssetID: pgtype.Int8{Int64: 88, Valid: true},
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
			GetMerchantApplication(gomock.Any(), int64(66)).
			Return(app, nil),
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
			GetMerchantApplication(gomock.Any(), int64(66)).
			Return(app, nil),
		store.EXPECT().
			UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(66), arg.ID)
				var payload foodPermitOCRData
				require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &payload))
				require.Equal(t, "done", payload.Status)
				require.NotNil(t, payload.Readiness)
				require.Equal(t, ocrReadinessStateReady, payload.Readiness.State)
				require.NotNil(t, payload.OCRJobID)
				require.Equal(t, int64(77), *payload.OCRJobID)
				require.Equal(t, "JY12345678901234", payload.PermitNo)
				require.Equal(t, "本地生活餐饮店", payload.CompanyName)
				require.Equal(t, "2027年01月08日", payload.ValidTo)
				require.Contains(t, payload.RawText, "许可证编号")
				return app, nil
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
		MediaAssetID:  88,
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
	app := db.MerchantApplication{
		ID:                     77,
		Status:                 "draft",
		FoodPermitMediaAssetID: pgtype.Int8{Int64: 99, Valid: true},
		FoodPermitOcr:          []byte(`{"status":"pending","ocr_job_id":88}`),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(77)).Return(app, nil),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(77)).Return(app, nil),
		store.EXPECT().UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
			var payload foodPermitOCRData
			require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &payload))
			require.Equal(t, "JY99887766554433", payload.PermitNo)
			require.Equal(t, "本地生活轻食店", payload.CompanyName)
			require.Equal(t, "王五", payload.OperatorName)
			require.Equal(t, "2025年01月08日", payload.ValidFrom)
			require.Equal(t, "2030年12月31日", payload.ValidTo)
			require.Equal(t, "经营场所：北京市朝阳区测试路100号1楼", payload.RawText)
			return app, nil
		}),
	)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{
		ID:        88,
		Status:    string(ocr.JobStatusSucceeded),
		OwnerType: string(ocr.OwnerTypeMerchantApplication),
		Provider:  string(ocr.ProviderNameAliyun),
	})

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 77, MediaAssetID: 99, OCRJobID: 88})
	require.NoError(t, err)

	task := asynq.NewTask(TaskMerchantApplicationFoodPermitOCR, payload)
	err = processor.ProcessTaskMerchantApplicationFoodPermitOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestParseFoodPermitOCRTextFallback_ExtractsLikelyCompanyName(t *testing.T) {
	t.Parallel()

	var data foodPermitOCRData
	parseFoodPermitOCRTextFallback(&data, "经营者名称：测试餐饮有限公司\n许可证编号：JY12345678901234\n有效期至：2027年01月08日")

	require.Equal(t, "测试餐饮有限公司", data.CompanyName)
	require.Equal(t, "JY12345678901234", data.PermitNo)
	require.Equal(t, "2027年01月08日", data.ValidTo)
}

func TestParseFoodPermitOCRTextFallback_RejectsSuspiciousCompanyName(t *testing.T) {
	t.Parallel()

	var data foodPermitOCRData
	parseFoodPermitOCRTextFallback(&data, "经营者名称：地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东\n许可证编号：JY12345678901234\n有效期至：2027年01月08日")

	require.Empty(t, data.CompanyName)
	require.Equal(t, "JY12345678901234", data.PermitNo)
	require.Equal(t, "2027年01月08日", data.ValidTo)
}

func TestParseFoodPermitOCRTextFallback_UsesRegistrationBusinessName(t *testing.T) {
	t.Parallel()

	var data foodPermitOCRData
	parseFoodPermitOCRTextFallback(&data, "食品小作坊小餐饮登记证\n商号名称：宁晋县玉水轩鱼味馆\n经营者姓名：张三\n登记证编号：2130528020946\n有效期至：2030年12月31日")

	require.Equal(t, "宁晋县玉水轩鱼味馆", data.CompanyName)
	require.Equal(t, "2130528020946", data.PermitNo)
	require.Equal(t, "2030年12月31日", data.ValidTo)
}

func TestParseFoodPermitOCRTextFallback_DoesNotOverwriteStructuredFields(t *testing.T) {
	t.Parallel()

	data := foodPermitOCRData{
		CompanyName:  "结构化餐饮店",
		OperatorName: "李四",
		PermitNo:     "JY00000000000000",
		ValidTo:      "2035年01月01日",
	}
	parseFoodPermitOCRTextFallback(&data, "经营者名称：文本餐饮有限公司\n经营者姓名：张三\n许可证编号：JY12345678901234\n有效期至：2027年01月08日")

	require.Equal(t, "结构化餐饮店", data.CompanyName)
	require.Equal(t, "李四", data.OperatorName)
	require.Equal(t, "JY00000000000000", data.PermitNo)
	require.Equal(t, "2035年01月01日", data.ValidTo)
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
	app := db.MerchantApplication{
		ID:                          67,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 98, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":78}`),
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
			GetMerchantApplication(gomock.Any(), int64(67)).
			Return(app, nil),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(67)).Return(app, nil),
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
				return app, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 67, MediaAssetID: 98, OCRJobID: 78})
	require.NoError(t, err)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 78, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider(t *testing.T) {
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

	job := db.OcrJob{
		ID:           7801,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 9801,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      6701,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    time.Now(),
	}
	app := db.MerchantApplication{
		ID:                          6701,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 9802, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":7801}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7801)).Return(job, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6701)).Return(app, nil),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 6701, MediaAssetID: 9801, OCRJobID: 7801})
	require.NoError(t, err)
	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsStaleAssetAfterProvider(t *testing.T) {
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
	initialApp := db.MerchantApplication{
		ID:                          6801,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 9801, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":7801}`),
	}
	updatedApp := initialApp
	updatedApp.BusinessLicenseMediaAssetID = pgtype.Int8{Int64: 9802, Valid: true}

	createdAt := time.Date(2026, 3, 25, 10, 59, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 10, 59, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           7801,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 9801,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      6801,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7801)).Return(baseJob, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6801)).Return(initialApp, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7801)).Return(baseJob, nil),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6801)).Return(updatedApp, nil),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 6801, MediaAssetID: 9801, OCRJobID: 7801})
	require.NoError(t, err)
	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsStaleFailureAfterProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   failingBusinessLicenseOCRProvider{},
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
	initialApp := db.MerchantApplication{
		ID:                          6802,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 9803, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":7802}`),
	}
	updatedApp := initialApp
	updatedApp.BusinessLicenseOcr = []byte(`{"status":"pending","ocr_job_id":7803}`)

	createdAt := time.Date(2026, 3, 25, 11, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 11, 0, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           7802,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 9803,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      6802,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}
	failedJob := baseJob
	failedJob.Status = string(ocr.JobStatusFailed)
	failedJob.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7802)).Return(baseJob, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6802)).Return(initialApp, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7802)).Return(baseJob, nil),
		store.EXPECT().MarkOCRJobProcessing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
			job := baseJob
			job.Status = string(ocr.JobStatusProcessing)
			job.StartedAt = pgtype.Timestamptz{Time: startedAt, Valid: true}
			return job, nil
		}),
		store.EXPECT().FailOCRJob(gomock.Any(), gomock.Any()).Return(failedJob, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6802)).Return(updatedApp, nil),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 6802, MediaAssetID: 9803, OCRJobID: 7802})
	require.NoError(t, err)
	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsNonDraftApplication(t *testing.T) {
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

	job := db.OcrJob{
		ID:           7802,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 9803,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      6702,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    time.Now(),
	}
	app := db.MerchantApplication{
		ID:                          6702,
		Status:                      "submitted",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 9803, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":7802}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7802)).Return(job, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6702)).Return(app, nil),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 6702, MediaAssetID: 9803, OCRJobID: 7802})
	require.NoError(t, err)
	task := asynq.NewTask(TaskMerchantApplicationBusinessLicenseOCR, payload)
	err = processor.ProcessTaskMerchantApplicationBusinessLicenseOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationIDCardOCR_SkipsStaleAssetBeforeProvider(t *testing.T) {
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

	job := db.OcrJob{
		ID:           7901,
		DocumentType: string(ocr.DocumentTypeIDCard),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 9901,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      6801,
		Status:       string(ocr.JobStatusPending),
		Side:         string(ocr.DocumentSideFront),
		CreatedAt:    time.Now(),
	}
	app := db.MerchantApplication{
		ID:                      6801,
		Status:                  "draft",
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 9902, Valid: true},
		IDCardFrontOcr:          []byte(`{"status":"pending","ocr_job_id":7901}`),
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(7901)).Return(job, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(6801)).Return(app, nil),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 6801, MediaAssetID: 9901, OCRJobID: 7901, Side: "Front"})
	require.NoError(t, err)
	task := asynq.NewTask(TaskMerchantApplicationIDCardOCR, payload)
	err = processor.ProcessTaskMerchantApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantApplicationBusinessLicenseOCR_WritesPartialReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeBusinessLicense: {
			Provider:   stubPartialBusinessLicenseOCRProvider{},
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
	app := db.MerchantApplication{
		ID:                          167,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 198, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":178}`),
	}

	createdAt := time.Date(2026, 3, 25, 11, 4, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 11, 4, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           178,
		DocumentType: string(ocr.DocumentTypeBusinessLicense),
		Provider:     string(ocr.ProviderNameWechat),
		MediaAssetID: 198,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      167,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(178)).Return(baseJob, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(167)).Return(app, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(178)).Return(baseJob, nil),
		store.EXPECT().
			MarkOCRJobProcessing(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
				require.Equal(t, int64(178), arg.ID)
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
				job.FinishedAt = pgtype.Timestamptz{Time: startedAt.Add(12 * time.Second), Valid: true}
				return job, nil
			}),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(167)).Return(app, nil),
		store.EXPECT().
			UpdateMerchantApplicationBusinessLicense(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(167), arg.ID)
				require.Equal(t, "92310000123456789B", arg.BusinessLicenseNumber.String)
				require.Equal(t, "餐饮服务", arg.BusinessScope.String)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
				require.Equal(t, "done", payload["status"])
				readiness, ok := payload["readiness"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, ocrReadinessStatePartial, readiness["state"])
				require.Equal(t, ocrReadinessReasonRequiredFieldMissing, readiness["reason_code"])
				require.Equal(t, []any{"valid_period"}, readiness["missing_fields"])
				require.Equal(t, "", payload["valid_period"])
				return app, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 167, MediaAssetID: 198, OCRJobID: 178})
	require.NoError(t, err)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 178, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

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
	app := db.MerchantApplication{
		ID:                      68,
		Status:                  "draft",
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 99, Valid: true},
		IDCardFrontOcr:          []byte(`{"status":"pending","ocr_job_id":79}`),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(68)).Return(app, nil),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(68)).Return(app, nil),
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
				return app, nil
			}),
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 68, MediaAssetID: 99, OCRJobID: 79, Side: "Front"})
	require.NoError(t, err)
	expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 79, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

	task := asynq.NewTask(TaskMerchantApplicationIDCardOCR, payload)
	err = processor.ProcessTaskMerchantApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}
