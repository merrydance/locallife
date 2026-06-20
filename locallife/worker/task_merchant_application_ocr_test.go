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
	"github.com/merrydance/locallife/logic"
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

type stubSmallCateringQRCodeFoodPermitOCRProvider struct{}

func (stubSmallCateringQRCodeFoodPermitOCRProvider) Name() ocr.ProviderName {
	return ocr.ProviderNameAliyun
}

func (stubSmallCateringQRCodeFoodPermitOCRProvider) Recognize(_ context.Context, capability ocr.Capability, req ocr.RecognizeRequest) (ocr.RecognizeResponse, error) {
	if capability != ocr.CapabilityAliyunFoodPermit {
		return ocr.RecognizeResponse{}, nil
	}
	recognizedAt := time.Date(2026, 3, 25, 10, 8, 0, 0, time.UTC)
	return ocr.RecognizeResponse{
		RawResult: json.RawMessage(`{"data":{"qrCode":"http://121.28.87.7:8081/OrcodeXcyXzf.jsp?flowId=2130528020270&zsId=2130528020270"}}`),
		Normalized: ocr.NormalizedResult{
			DocumentType: ocr.DocumentTypeFoodPermit,
			RecognizedAt: recognizedAt,
			FoodPermit: &ocr.FoodPermitResult{
				BusinessName: "食品小作坊小餐饮登记证2130528020270",
				RawText:      "食品小作坊小餐饮登记证\n二维码：http://121.28.87.7:8081/OrcodeXcyXzf.jsp?flowId=2130528020270&zsId=2130528020270",
			},
		},
	}, nil
}

type stubMerchantFoodPermitOfficialVerifier struct {
	result logic.MerchantFoodPermitOfficialVerification
	err    error
	calls  *int
}

func (stub stubMerchantFoodPermitOfficialVerifier) VerifyMerchantFoodPermit(ctx context.Context, rawResult []byte) (logic.MerchantFoodPermitOfficialVerification, error) {
	if stub.calls != nil {
		*stub.calls = *stub.calls + 1
	}
	return stub.result, stub.err
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

func expectOCRSuccessAuditLog(t *testing.T, store *mockdb.MockStore, job db.OcrJob) *gomock.Call {
	t.Helper()
	return store.EXPECT().
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

func expectMerchantSubjectProfileSyncFromWorker(
	t *testing.T,
	store *mockdb.MockStore,
	applicationID int64,
	assertUpsert func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams),
) (*gomock.Call, *gomock.Call) {
	t.Helper()
	upsertCall := store.EXPECT().
		UpsertMerchantSubjectProfile(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertMerchantSubjectProfileParams) (db.MerchantSubjectProfile, error) {
			require.Equal(t, applicationID, arg.MerchantApplicationID)
			require.NotEmpty(t, arg.SourceSnapshot)
			if assertUpsert != nil {
				assertUpsert(t, arg)
			}
			return db.MerchantSubjectProfile{
				ID:                          applicationID + 500,
				MerchantApplicationID:       arg.MerchantApplicationID,
				MerchantID:                  arg.MerchantID,
				UserID:                      arg.UserID,
				BusinessLicenseNumber:       arg.BusinessLicenseNumber,
				BusinessLicenseName:         arg.BusinessLicenseName,
				BusinessLicenseAddress:      arg.BusinessLicenseAddress,
				LegalPersonName:             arg.LegalPersonName,
				LegalPersonIDNumber:         arg.LegalPersonIDNumber,
				FoodPermitNumber:            arg.FoodPermitNumber,
				FoodPermitCompanyName:       arg.FoodPermitCompanyName,
				BusinessLicenseMediaAssetID: arg.BusinessLicenseMediaAssetID,
				FoodPermitMediaAssetID:      arg.FoodPermitMediaAssetID,
				IDCardFrontMediaAssetID:     arg.IDCardFrontMediaAssetID,
				IDCardBackMediaAssetID:      arg.IDCardBackMediaAssetID,
				BusinessLicensePayload:      arg.BusinessLicensePayload,
				FoodPermitPayload:           arg.FoodPermitPayload,
				LegalPersonPayload:          arg.LegalPersonPayload,
				SourceSnapshot:              arg.SourceSnapshot,
				Version:                     1,
				CreatedAt:                   time.Now(),
				UpdatedAt:                   time.Now(),
			}, nil
		})
	versionCall := store.EXPECT().
		CreateMerchantSubjectProfileVersion(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantSubjectProfileVersionParams) (db.MerchantSubjectProfileVersion, error) {
			require.Equal(t, applicationID, arg.MerchantApplicationID)
			require.Equal(t, applicationID+500, arg.ProfileID)
			require.Equal(t, int32(1), arg.Version)
			require.NotEmpty(t, arg.Snapshot)
			return db.MerchantSubjectProfileVersion{
				ID:                    applicationID + 600,
				ProfileID:             arg.ProfileID,
				MerchantApplicationID: arg.MerchantApplicationID,
				MerchantID:            arg.MerchantID,
				UserID:                arg.UserID,
				Version:               arg.Version,
				Snapshot:              arg.Snapshot,
				CreatedAt:             time.Now(),
			}, nil
		})
	return upsertCall, versionCall
}

func expectMerchantSubjectProfileProjectionFailureFromWorker(store *mockdb.MockStore) *gomock.Call {
	return store.EXPECT().
		UpsertMerchantSubjectProfile(gomock.Any(), gomock.Any()).
		Return(db.MerchantSubjectProfile{}, errors.New("profile projection unavailable"))
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
	syncUpsert := expectMerchantSubjectProfileProjectionFailureFromWorker(store)
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{
		ID:        77,
		Status:    string(ocr.JobStatusSucceeded),
		OwnerType: string(ocr.OwnerTypeMerchantApplication),
		Provider:  string(ocr.ProviderNameWechat),
	})

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
				updatedApp := app
				updatedApp.FoodPermitOcr = arg.FoodPermitOcr
				return updatedApp, nil
			}),
		syncUpsert,
		auditCall,
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

	verifierCalls := 0
	processor := &RedisTaskProcessor{
		store: store,
		ocrService: ocr.NewService(
			store,
			router,
			stubFoodPermitBinaryReader{},
		),
		foodPermitVerifier: stubMerchantFoodPermitOfficialVerifier{calls: &verifierCalls},
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
	syncUpsert, syncVersion := expectMerchantSubjectProfileSyncFromWorker(t, store, app.ID, func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams) {
		require.Equal(t, "JY99887766554433", arg.FoodPermitNumber)
		require.Equal(t, "本地生活轻食店", arg.FoodPermitCompanyName)
		require.Equal(t, app.FoodPermitMediaAssetID, arg.FoodPermitMediaAssetID)
	})
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{
		ID:        88,
		Status:    string(ocr.JobStatusSucceeded),
		OwnerType: string(ocr.OwnerTypeMerchantApplication),
		Provider:  string(ocr.ProviderNameAliyun),
	})

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
			updatedApp := app
			updatedApp.FoodPermitOcr = arg.FoodPermitOcr
			return updatedApp, nil
		}),
		syncUpsert,
		syncVersion,
		auditCall,
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 77, MediaAssetID: 99, OCRJobID: 88})
	require.NoError(t, err)

	task := asynq.NewTask(TaskMerchantApplicationFoodPermitOCR, payload)
	err = processor.ProcessTaskMerchantApplicationFoodPermitOCR(context.Background(), task)
	require.NoError(t, err)
	require.Zero(t, verifierCalls)
}

func TestProcessTaskMerchantApplicationFoodPermitOCR_BackfillsSmallCateringFromOfficialVerification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	router, err := ocr.NewStaticRouter(map[ocr.DocumentType]ocr.Route{
		ocr.DocumentTypeFoodPermit: {
			Provider:   stubSmallCateringQRCodeFoodPermitOCRProvider{},
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
		foodPermitVerifier: stubMerchantFoodPermitOfficialVerifier{
			result: logic.MerchantFoodPermitOfficialVerification{
				CompanyName:  "宁晋县周鹏饭店",
				OperatorName: "周松涛",
				PermitNo:     "2130528020270",
				CreditCode:   "92130528MA0A5XB46A",
				Address:      "河北省邢台市宁晋县测试路1号",
				ValidTo:      "2028年12月21日",
			},
		},
	}
	app := db.MerchantApplication{
		ID:                     78,
		Status:                 "draft",
		FoodPermitMediaAssetID: pgtype.Int8{Int64: 100, Valid: true},
		FoodPermitOcr:          []byte(`{"status":"pending","ocr_job_id":89}`),
	}

	createdAt := time.Date(2026, 3, 25, 10, 7, 0, 0, time.UTC)
	startedAt := time.Date(2026, 3, 25, 10, 7, 30, 0, time.UTC)
	baseJob := db.OcrJob{
		ID:           89,
		DocumentType: string(ocr.DocumentTypeFoodPermit),
		Provider:     string(ocr.ProviderNameAliyun),
		MediaAssetID: 100,
		OwnerType:    string(ocr.OwnerTypeMerchantApplication),
		OwnerID:      78,
		Status:       string(ocr.JobStatusPending),
		CreatedAt:    createdAt,
	}
	syncUpsert, syncVersion := expectMerchantSubjectProfileSyncFromWorker(t, store, app.ID, func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams) {
		require.Equal(t, "2130528020270", arg.FoodPermitNumber)
		require.Equal(t, "宁晋县周鹏饭店", arg.FoodPermitCompanyName)
		require.Equal(t, app.FoodPermitMediaAssetID, arg.FoodPermitMediaAssetID)
	})
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{
		ID:        89,
		Status:    string(ocr.JobStatusSucceeded),
		OwnerType: string(ocr.OwnerTypeMerchantApplication),
		Provider:  string(ocr.ProviderNameAliyun),
	})

	gomock.InOrder(
		store.EXPECT().GetOCRJob(gomock.Any(), int64(89)).Return(baseJob, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(78)).Return(app, nil),
		store.EXPECT().GetOCRJob(gomock.Any(), int64(89)).Return(baseJob, nil),
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
		store.EXPECT().GetMerchantApplication(gomock.Any(), int64(78)).Return(app, nil),
		store.EXPECT().UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
			var payload foodPermitOCRData
			require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &payload))
			require.Equal(t, "2130528020270", payload.PermitNo)
			require.Equal(t, "宁晋县周鹏饭店", payload.CompanyName)
			require.Equal(t, "周松涛", payload.OperatorName)
			require.Equal(t, "2028年12月21日", payload.ValidTo)
			require.NotNil(t, payload.Readiness)
			require.Equal(t, ocrReadinessStateReady, payload.Readiness.State)
			require.Contains(t, payload.RawText, "官方核验主体名称：宁晋县周鹏饭店")
			updatedApp := app
			updatedApp.FoodPermitOcr = arg.FoodPermitOcr
			return updatedApp, nil
		}),
		syncUpsert,
		syncVersion,
		auditCall,
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 78, MediaAssetID: 100, OCRJobID: 89})
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

func TestRepairMerchantFoodPermitOCRDataFromOfficialVerification_FailsOpen(t *testing.T) {
	t.Parallel()

	data := foodPermitOCRData{
		RawText: "食品小作坊小餐饮登记证",
	}
	processor := &RedisTaskProcessor{
		foodPermitVerifier: stubMerchantFoodPermitOfficialVerifier{err: errors.New("official endpoint timeout")},
	}
	changed := processor.repairMerchantFoodPermitOCRDataFromOfficialVerification(context.Background(), &data, db.OcrJob{
		ID:        90,
		Provider:  string(ocr.ProviderNameAliyun),
		RawResult: []byte(`{"data":{"qrCode":"http://121.28.87.7:8081/OrcodeXcyXzf.jsp?flowId=2130528020270&zsId=2130528020270"}}`),
	}, 79)

	require.False(t, changed)
	require.Equal(t, "食品小作坊小餐饮登记证", data.RawText)
	require.Empty(t, data.PermitNo)
	require.Empty(t, data.CompanyName)
	require.Empty(t, data.OperatorName)
	require.Empty(t, data.ValidTo)
}

func TestRepairFoodPermitOCRDataFromOfficialVerification_DoesNotReplaceRecognizedCompanyName(t *testing.T) {
	t.Parallel()

	data := foodPermitOCRData{
		PermitNo:    "JY11105000000001",
		CompanyName: "测试餐饮有限公司",
		ValidTo:     "2030年12月31日",
	}

	changed := repairFoodPermitOCRDataFromOfficialVerification(&data, logic.MerchantFoodPermitOfficialVerification{
		CompanyName:  "另一家餐饮有限公司",
		OperatorName: "张三",
		PermitNo:     "2130528020270",
		ValidTo:      "2028年12月21日",
	})

	require.False(t, changed)
	require.Equal(t, "测试餐饮有限公司", data.CompanyName)
	require.Equal(t, "JY11105000000001", data.PermitNo)
	require.Empty(t, data.OperatorName)
	require.Equal(t, "2030年12月31日", data.ValidTo)
	require.Empty(t, data.RawText)
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
	syncUpsert, syncVersion := expectMerchantSubjectProfileSyncFromWorker(t, store, app.ID, func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams) {
		require.Equal(t, "91310000123456789A", arg.BusinessLicenseNumber)
		require.Equal(t, "本地生活科技有限公司", arg.BusinessLicenseName)
		require.Equal(t, "测试路1号", arg.BusinessLicenseAddress)
		require.Equal(t, "张三", arg.LegalPersonName)
		require.Equal(t, app.BusinessLicenseMediaAssetID, arg.BusinessLicenseMediaAssetID)
	})
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 78, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

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
			UpdateMerchantApplicationBusinessLicenseOCRResult(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseOCRResultParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(67), arg.ID)
				require.Equal(t, "91310000123456789A", arg.BusinessLicenseNumber.String)
				require.Equal(t, "餐饮服务", arg.BusinessScope.String)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
				require.Equal(t, "done", payload["status"])
				require.Equal(t, float64(78), payload["ocr_job_id"])
				require.Equal(t, "本地生活科技有限公司", payload["enterprise_name"])
				updatedApp := app
				updatedApp.BusinessLicenseNumber = arg.BusinessLicenseNumber.String
				updatedApp.BusinessScope = arg.BusinessScope
				updatedApp.BusinessLicenseOcr = arg.BusinessLicenseOcr
				return updatedApp, nil
			}),
		syncUpsert,
		syncVersion,
		auditCall,
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 67, MediaAssetID: 98, OCRJobID: 78})
	require.NoError(t, err)

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
	syncUpsert, syncVersion := expectMerchantSubjectProfileSyncFromWorker(t, store, app.ID, func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams) {
		require.Equal(t, "92310000123456789B", arg.BusinessLicenseNumber)
		require.Equal(t, "本地生活小馆", arg.BusinessLicenseName)
		require.Equal(t, "测试路2号", arg.BusinessLicenseAddress)
		require.Equal(t, "李四", arg.LegalPersonName)
		require.Equal(t, app.BusinessLicenseMediaAssetID, arg.BusinessLicenseMediaAssetID)
	})
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 178, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

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
			UpdateMerchantApplicationBusinessLicenseOCRResult(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseOCRResultParams) (db.MerchantApplication, error) {
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
				updatedApp := app
				updatedApp.BusinessLicenseNumber = arg.BusinessLicenseNumber.String
				updatedApp.BusinessScope = arg.BusinessScope
				updatedApp.BusinessLicenseOcr = arg.BusinessLicenseOcr
				return updatedApp, nil
			}),
		syncUpsert,
		syncVersion,
		auditCall,
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 167, MediaAssetID: 198, OCRJobID: 178})
	require.NoError(t, err)

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
	syncUpsert, syncVersion := expectMerchantSubjectProfileSyncFromWorker(t, store, app.ID, func(t *testing.T, arg db.UpsertMerchantSubjectProfileParams) {
		require.Equal(t, "张三", arg.LegalPersonName)
		require.Equal(t, "110101199001011234", arg.LegalPersonIDNumber)
		require.Equal(t, app.IDCardFrontMediaAssetID, arg.IDCardFrontMediaAssetID)
	})
	auditCall := expectOCRSuccessAuditLog(t, store, db.OcrJob{ID: 79, OwnerType: string(ocr.OwnerTypeMerchantApplication)})

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
			UpdateMerchantApplicationIDCardFrontOCRResult(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationIDCardFrontOCRResultParams) (db.MerchantApplication, error) {
				require.Equal(t, int64(68), arg.ID)
				require.Equal(t, "张三", arg.LegalPersonName.String)
				require.Equal(t, "110101199001011234", arg.LegalPersonIDNumber.String)
				var payload merchantIDCardOCRData
				require.NoError(t, json.Unmarshal(arg.IDCardFrontOcr, &payload))
				require.Equal(t, "done", payload.Status)
				require.NotNil(t, payload.OCRJobID)
				require.Equal(t, int64(79), *payload.OCRJobID)
				require.Equal(t, "张三", payload.Name)
				updatedApp := app
				updatedApp.LegalPersonName = arg.LegalPersonName.String
				updatedApp.LegalPersonIDNumber = arg.LegalPersonIDNumber.String
				updatedApp.IDCardFrontOcr = arg.IDCardFrontOcr
				return updatedApp, nil
			}),
		syncUpsert,
		syncVersion,
		auditCall,
	)

	payload, err := json.Marshal(merchantApplicationOCRPayload{ApplicationID: 68, MediaAssetID: 99, OCRJobID: 79, Side: "Front"})
	require.NoError(t, err)

	task := asynq.NewTask(TaskMerchantApplicationIDCardOCR, payload)
	err = processor.ProcessTaskMerchantApplicationIDCardOCR(context.Background(), task)
	require.NoError(t, err)
}
