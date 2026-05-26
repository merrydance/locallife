package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type merchantOnboardingReviewFlowStoreStub struct {
	approveMerchantApplicationTxFn      func(context.Context, db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error)
	markOnboardingReviewRunProcessingFn func(context.Context, int64) (db.OnboardingReviewRun, error)
	completeOnboardingReviewRunFn       func(context.Context, db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error)
	updateMerchantReviewSummaryFn       func(context.Context, db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error)
	activateMerchantCredentialsFn       func(context.Context, db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	getActiveMerchantCredentialsFn      func(context.Context, pgtype.Int8) ([]db.CredentialLedger, error)
	restoreMerchantCredentialGovFn      func(context.Context, db.RestoreMerchantCredentialGovernanceTxParams) (int64, error)
}

func (stub merchantOnboardingReviewFlowStoreStub) GetMerchantApplication(context.Context, int64) (db.MerchantApplication, error) {
	return db.MerchantApplication{}, fmt.Errorf("unexpected GetMerchantApplication call")
}

func (stub merchantOnboardingReviewFlowStoreStub) ApproveMerchantApplicationTx(ctx context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
	if stub.approveMerchantApplicationTxFn == nil {
		return db.ApproveMerchantApplicationTxResult{}, fmt.Errorf("unexpected ApproveMerchantApplicationTx call")
	}
	return stub.approveMerchantApplicationTxFn(ctx, arg)
}

func (stub merchantOnboardingReviewFlowStoreStub) CreateMerchantOnboardingReviewRun(context.Context, db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
	return db.OnboardingReviewRun{}, fmt.Errorf("unexpected CreateMerchantOnboardingReviewRun call")
}

func (stub merchantOnboardingReviewFlowStoreStub) CreateRiderOnboardingReviewRun(context.Context, db.CreateRiderOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
	return db.OnboardingReviewRun{}, fmt.Errorf("unexpected CreateRiderOnboardingReviewRun call")
}

func (stub merchantOnboardingReviewFlowStoreStub) CancelOnboardingReviewRun(context.Context, db.CancelOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
	return db.OnboardingReviewRun{}, fmt.Errorf("unexpected CancelOnboardingReviewRun call")
}

func (stub merchantOnboardingReviewFlowStoreStub) MarkOnboardingReviewRunProcessing(ctx context.Context, id int64) (db.OnboardingReviewRun, error) {
	if stub.markOnboardingReviewRunProcessingFn == nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("unexpected MarkOnboardingReviewRunProcessing call")
	}
	return stub.markOnboardingReviewRunProcessingFn(ctx, id)
}

func (stub merchantOnboardingReviewFlowStoreStub) CompleteOnboardingReviewRun(ctx context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
	if stub.completeOnboardingReviewRunFn == nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("unexpected CompleteOnboardingReviewRun call")
	}
	return stub.completeOnboardingReviewRunFn(ctx, arg)
}

func (stub merchantOnboardingReviewFlowStoreStub) UpdateMerchantApplicationReviewSummary(ctx context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
	if stub.updateMerchantReviewSummaryFn == nil {
		return db.MerchantApplication{}, fmt.Errorf("unexpected UpdateMerchantApplicationReviewSummary call")
	}
	return stub.updateMerchantReviewSummaryFn(ctx, arg)
}

func (stub merchantOnboardingReviewFlowStoreStub) UpdateRiderApplicationReviewSummary(context.Context, db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
	return db.RiderApplication{}, fmt.Errorf("unexpected UpdateRiderApplicationReviewSummary call")
}

func (stub merchantOnboardingReviewFlowStoreStub) ActivateMerchantCredentialLedgersTx(ctx context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	if stub.activateMerchantCredentialsFn == nil {
		return nil, fmt.Errorf("unexpected ActivateMerchantCredentialLedgersTx call")
	}
	return stub.activateMerchantCredentialsFn(ctx, arg)
}

func (stub merchantOnboardingReviewFlowStoreStub) ActivateRiderCredentialLedgersTx(context.Context, db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	return nil, fmt.Errorf("unexpected ActivateRiderCredentialLedgersTx call")
}

func (stub merchantOnboardingReviewFlowStoreStub) GetActiveMerchantCredentialLedgers(ctx context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
	if stub.getActiveMerchantCredentialsFn == nil {
		return nil, fmt.Errorf("unexpected GetActiveMerchantCredentialLedgers call")
	}
	return stub.getActiveMerchantCredentialsFn(ctx, merchantID)
}

func (stub merchantOnboardingReviewFlowStoreStub) GetActiveRiderCredentialLedgers(context.Context, pgtype.Int8) ([]db.CredentialLedger, error) {
	return nil, fmt.Errorf("unexpected GetActiveRiderCredentialLedgers call")
}

func (stub merchantOnboardingReviewFlowStoreStub) RestoreMerchantCredentialGovernanceTx(ctx context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
	if stub.restoreMerchantCredentialGovFn == nil {
		return 0, fmt.Errorf("unexpected RestoreMerchantCredentialGovernanceTx call")
	}
	return stub.restoreMerchantCredentialGovFn(ctx, arg)
}

func (stub merchantOnboardingReviewFlowStoreStub) RestoreRiderCredentialGovernanceTx(context.Context, db.RestoreRiderCredentialGovernanceTxParams) (int64, error) {
	return 0, fmt.Errorf("unexpected RestoreRiderCredentialGovernanceTx call")
}

func merchantReviewTestApplication() db.MerchantApplication {
	return db.MerchantApplication{
		ID:                          41,
		UserID:                      9,
		Status:                      "submitted",
		MerchantName:                "测试餐饮店",
		ContactPhone:                "13812345678",
		BusinessAddress:             "北京市朝阳区测试路100号",
		RegionID:                    pgtype.Int8{Int64: 8, Valid: true},
		BusinessLicenseNumber:       "91110000MA12345678",
		LegalPersonName:             "张三",
		LegalPersonIDNumber:         "110101199001011234",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 11, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 12, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 13, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 14, Valid: true},
		BusinessLicenseOcr:          []byte(`{"ocr_job_id":101,"reg_num":"91110000MA12345678","enterprise_name":"测试餐饮店","legal_representative":"张三","address":"北京市朝阳区测试路100号","business_scope":"餐饮服务","valid_period":"2020年01月01日至2040年01月01日","credit_code":"91110000MA12345678"}`),
		FoodPermitOcr:               []byte(`{"ocr_job_id":102,"permit_no":"JY11105000000001","company_name":"测试餐饮店","operator_name":"张三","valid_to":"2030年12月31日"}`),
		StorefrontImages:            []byte(`["storefront-a.jpg","storefront-b.jpg"]`),
		EnvironmentImages:           []byte(`["environment-a.jpg"]`),
	}
}

func merchantReviewFixedNow() time.Time {
	return time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
}

func merchantReviewTestInput() MerchantDocumentReviewInput {
	return MerchantDocumentReviewInput{
		BusinessLicense: MerchantReviewBusinessLicenseOCRData{
			EnterpriseName:      "测试餐饮有限公司",
			LegalRepresentative: "张三",
			Address:             "北京市朝阳区测试路100号",
			BusinessScope:       "餐饮服务",
			ValidPeriod:         "2020年01月01日至2040年01月01日",
		},
		FoodPermit: MerchantReviewFoodPermitOCRData{
			CompanyName:  "测试餐饮有限公司",
			OperatorName: "张三",
			ValidTo:      "2030年12月31日",
		},
		IDCardFront: MerchantReviewIDCardOCRData{
			Name:     "张三",
			IDNumber: "110101199001011234",
		},
		IDCardBack: MerchantReviewIDCardOCRData{
			ValidDate: "2020.01.01-2035.01.01",
		},
	}
}

func TestEvaluateMerchantDocumentReview_UsesBusinessLicenseReadiness(t *testing.T) {
	input := merchantReviewTestInput()
	input.BusinessLicense.Readiness = &MerchantReviewOCRReadiness{
		State:         "partial",
		ReasonCode:    "required_field_missing",
		MissingFields: []string{"valid_period"},
	}

	_, err := EvaluateMerchantDocumentReview(input, merchantReviewFixedNow())
	require.Error(t, err)
	reviewErr, ok := err.(*MerchantDocumentReviewError)
	require.True(t, ok)
	require.Equal(t, MerchantDocumentReviewCodeBusinessLicenseRequired, reviewErr.Code)
	require.Equal(t, "营业执照有效期未识别，请重新上传清晰完整的营业执照照片", reviewErr.Message)
}

func TestEvaluateMerchantDocumentReview_UsesFoodPermitProviderFailure(t *testing.T) {
	input := merchantReviewTestInput()
	input.FoodPermit.Readiness = &MerchantReviewOCRReadiness{
		State:      "provider_failed",
		ReasonCode: "provider_error",
	}

	_, err := EvaluateMerchantDocumentReview(input, merchantReviewFixedNow())
	require.Error(t, err)
	reviewErr, ok := err.(*MerchantDocumentReviewError)
	require.True(t, ok)
	require.Equal(t, MerchantDocumentReviewCodeFoodLicenseRequired, reviewErr.Code)
	require.Equal(t, "食品经营许可证OCR处理失败，请重新上传清晰完整的食品经营许可证照片", reviewErr.Message)
}

func TestRepairMerchantBusinessLicenseFromRawResult_UsesAliyunValidDates(t *testing.T) {
	ocrJobID := int64(778)
	businessLicense := MerchantReviewBusinessLicenseOCRData{
		OCRJobID:            &ocrJobID,
		EnterpriseName:      "测试餐饮有限公司",
		LegalRepresentative: "张三",
		Address:             "北京市朝阳区测试路100号",
		BusinessScope:       "餐饮服务",
		Readiness: &MerchantReviewOCRReadiness{
			State:         merchantOCRReadinessStatePartial,
			ReasonCode:    merchantOCRReadinessReasonRequiredFieldMissing,
			MissingFields: []string{"valid_period"},
		},
	}
	rawResult := []byte(`{"Data":"{\"data\":{\"validFromDate\":\"20170104\",\"validToDate\":\"29991231\"}}"}`)

	repairedPayload, changed, err := RepairMerchantBusinessLicenseFromRawResult(&businessLicense, rawResult)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, repairedPayload)
	require.Equal(t, "2017年01月04日至长期", businessLicense.ValidPeriod)
	require.Equal(t, merchantOCRReadinessStateReady, businessLicense.Readiness.State)
	require.Equal(t, "ok", businessLicense.Readiness.ReasonCode)
	require.Empty(t, businessLicense.Readiness.MissingFields)
}

func TestRepairMerchantBusinessLicenseFromRawResult_TreatsStartOnlyAsLongTerm(t *testing.T) {
	businessLicense := MerchantReviewBusinessLicenseOCRData{}
	rawResult := []byte(`{"Data":"{\"data\":{\"registrationDate\":\"20170104\"}}"}`)

	_, changed, err := RepairMerchantBusinessLicenseFromRawResult(&businessLicense, rawResult)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "长期", businessLicense.ValidPeriod)
}

func TestBuildMerchantDocumentReviewInputFromPayloads_RepairsFoodPermitFromRawText(t *testing.T) {
	payloadResult, err := BuildMerchantDocumentReviewInputFromPayloads(MerchantDocumentReviewPayloads{
		BusinessLicenseJSON: []byte(`{"enterprise_name":"测试餐饮有限公司","legal_representative":"张三","address":"北京市朝阳区测试路100号","business_scope":"餐饮服务","valid_period":"2020年01月01日至2040年01月01日"}`),
		FoodPermitJSON:      []byte(`{"raw_text":"商号名称：测试餐饮有限公司\n经营者姓名：张三\n登记证编号：2130528020946\n有效期至：2030年12月31日","company_name":"","operator_name":"","permit_no":"","valid_to":""}`),
		IDCardFrontJSON:     []byte(`{"name":"张三","id_number":"110101199001011234"}`),
		IDCardBackJSON:      []byte(`{"valid_date":"2020.01.01-2035.01.01"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "测试餐饮有限公司", payloadResult.Input.FoodPermit.CompanyName)
	require.Equal(t, "张三", payloadResult.Input.FoodPermit.OperatorName)
	require.Equal(t, "2130528020946", payloadResult.Input.FoodPermit.PermitNo)
	require.Equal(t, "2030年12月31日", payloadResult.Input.FoodPermit.ValidTo)
	require.NotEmpty(t, payloadResult.RepairedFoodPermitJSON)
	require.False(t, payloadResult.FoodPermitNeedsNormalizedRepair)
}

func TestRepairMerchantFoodPermitFromNormalized_UsesNormalizedResult(t *testing.T) {
	foodPermit := MerchantReviewFoodPermitOCRData{
		CompanyName: "地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东",
		RawText:     "经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001",
	}
	normalizedResult, err := ocr.MarshalNormalizedResult(ocr.NormalizedResult{
		DocumentType: ocr.DocumentTypeFoodPermit,
		RecognizedAt: time.Now(),
		FoodPermit: &ocr.FoodPermitResult{
			LicenseNumber: "JY11105000000001",
			BusinessName:  "测试餐饮有限公司",
			OperatorName:  "张三",
			ValidPeriod:   "2030年12月31日",
			RawText:       "主体名称：测试餐饮有限公司\n经营者姓名：张三\n许可证编号：JY11105000000001\n有效期至：2030年12月31日",
		},
	})
	require.NoError(t, err)

	repairedPayload, changed, err := RepairMerchantFoodPermitFromNormalized(&foodPermit, normalizedResult)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, repairedPayload)
	require.Equal(t, "测试餐饮有限公司", foodPermit.CompanyName)
	require.Equal(t, "张三", foodPermit.OperatorName)
	require.Equal(t, "JY11105000000001", foodPermit.PermitNo)
	require.Equal(t, "2030年12月31日", foodPermit.ValidTo)
	require.False(t, merchantFoodPermitNeedsRepair(foodPermit))
}

func TestMerchantOnboardingReviewServiceProcessSubmittedApplication_ActivatesCredentialsAndAttemptsRestore(t *testing.T) {
	application := merchantReviewTestApplication()
	existingRunID := int64(901)
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	approvedApplication := application
	approvedApplication.Status = "approved"

	store := merchantOnboardingReviewFlowStoreStub{
		approveMerchantApplicationTxFn: func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
			require.Equal(t, application.ID, arg.ApplicationID)
			return db.ApproveMerchantApplicationTxResult{
				Application: approvedApplication,
				Merchant:    db.Merchant{ID: 66, OwnerUserID: application.UserID},
			}, nil
		},
		markOnboardingReviewRunProcessingFn: func(_ context.Context, id int64) (db.OnboardingReviewRun, error) {
			require.Equal(t, existingRunID, id)
			return db.OnboardingReviewRun{ID: existingRunID, RunStatus: "processing", Stage: "review", CreatedAt: now}, nil
		},
		completeOnboardingReviewRunFn: func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, existingRunID, arg.ID)
			return db.OnboardingReviewRun{
				ID:         existingRunID,
				RunStatus:  "completed",
				Stage:      "review",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				OcrJobRefs: []int64{101, 102},
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
		updateMerchantReviewSummaryFn: func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			return db.MerchantApplication{ID: approvedApplication.ID, ReviewSummary: arg.ReviewSummary}, nil
		},
		activateMerchantCredentialsFn: func(_ context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(66), arg.MerchantID)
			require.Len(t, arg.Entries, 2)
			require.Equal(t, existingRunID, arg.Entries[0].ReviewRunID.Int64)
			require.Equal(t, application.ID, arg.Entries[0].MerchantApplicationID)
			require.Equal(t, db.CredentialDocumentTypeBusinessLicense, arg.Entries[0].DocumentType)
			require.Equal(t, db.CredentialDocumentTypeFoodPermit, arg.Entries[1].DocumentType)
			var businessPayload map[string]any
			require.NoError(t, json.Unmarshal(arg.Entries[0].NormalizedPayload, &businessPayload))
			require.Equal(t, "91110000MA12345678", businessPayload["credit_code"])
			return []db.CredentialLedger{{ID: 101}, {ID: 102}}, nil
		},
		getActiveMerchantCredentialsFn: func(_ context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(66), merchantID.Int64)
			return []db.CredentialLedger{
				{ID: 101, DocumentType: db.CredentialDocumentTypeBusinessLicense, ExpiresAt: pgtype.Timestamptz{Time: now.AddDate(1, 0, 0), Valid: true}},
				{ID: 102, DocumentType: db.CredentialDocumentTypeFoodPermit, ExpiresAt: pgtype.Timestamptz{Time: now.AddDate(0, 6, 0), Valid: true}},
			}, nil
		},
		restoreMerchantCredentialGovFn: func(_ context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			require.Equal(t, int64(66), arg.MerchantID)
			require.Equal(t, []int64{101, 102}, arg.CredentialLedgerIDs)
			require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, arg.TakeoutSuspendReason.String)
			return 1, nil
		},
	}

	onboardingSvc := NewOnboardingReviewService(store)
	onboardingSvc.now = func() time.Time { return now }
	governanceSvc := NewCredentialGovernanceService(store)
	require.NotNil(t, governanceSvc)
	governanceSvc.now = func() time.Time { return now }
	service := NewMerchantOnboardingReviewService(store, onboardingSvc, governanceSvc)

	result, err := service.ProcessSubmittedApplication(context.Background(), application, application.UserID, &existingRunID)
	require.NoError(t, err)
	require.Len(t, result.CredentialEntries, 2)
	require.True(t, result.RestoreReleased)
	require.NotNil(t, result.ReviewRun)
	require.Equal(t, existingRunID, result.ReviewRun.ID)
	var summary map[string]any
	require.NoError(t, json.Unmarshal(result.Application.ReviewSummary, &summary))
	require.Equal(t, "approved", summary["outcome"])
}

func TestMerchantOnboardingReviewServiceProcessSubmittedApplication_CompletesExistingRunAndReturnsSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewMerchantOnboardingReviewService(store, NewOnboardingReviewService(store), nil)
	application := merchantReviewTestApplication()
	existingRunID := int64(801)
	completedAt := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)

	approvedApplication := application
	approvedApplication.Status = "approved"

	store.EXPECT().
		ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
			require.Equal(t, application.ID, arg.ApplicationID)
			require.Equal(t, application.UserID, arg.UserID)
			require.Equal(t, application.MerchantName, arg.MerchantName)
			require.Equal(t, application.ContactPhone, arg.Phone)
			require.Equal(t, application.BusinessAddress, arg.Address)
			require.Equal(t, application.RegionID.Int64, arg.RegionID)
			return db.ApproveMerchantApplicationTxResult{
				Application: approvedApplication,
				Merchant: db.Merchant{
					ID:          66,
					OwnerUserID: application.UserID,
				},
			}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), existingRunID).
		Return(db.OnboardingReviewRun{ID: existingRunID, ApplicationType: "merchant", RunStatus: "processing", Stage: "review", CreatedAt: completedAt}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, existingRunID, arg.ID)
			require.Equal(t, "approved", arg.Outcome.String)
			require.Equal(t, "auto_approved", arg.ReasonCode.String)
			require.Equal(t, []int64{101, 102}, arg.OcrJobRefs)
			return db.OnboardingReviewRun{
				ID:         existingRunID,
				Stage:      "review",
				RunStatus:  "completed",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				RuleHits:   []string{"merchant.auto_approve"},
				OcrJobRefs: []int64{101, 102},
				CreatedAt:  completedAt,
				UpdatedAt:  completedAt,
			}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			require.Equal(t, approvedApplication.ID, arg.ID)
			var summary map[string]any
			require.NoError(t, json.Unmarshal(arg.ReviewSummary, &summary))
			require.Equal(t, float64(existingRunID), summary["run_id"])
			require.Equal(t, "approved", summary["outcome"])
			require.Equal(t, "auto_approved", summary["reason_code"])
			return db.MerchantApplication{ID: approvedApplication.ID, ReviewSummary: arg.ReviewSummary}, nil
		})

	result, err := service.ProcessSubmittedApplication(context.Background(), application, application.UserID, &existingRunID)
	require.NoError(t, err)
	require.NotNil(t, result.Merchant)
	require.Equal(t, int64(66), result.Merchant.ID)
	require.NotNil(t, result.ReviewRun)
	require.Equal(t, existingRunID, result.ReviewRun.ID)
	require.Equal(t, "approved", result.Application.Status)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(result.Application.ReviewSummary, &summary))
	require.Equal(t, float64(existingRunID), summary["run_id"])
	require.Equal(t, "approved", summary["outcome"])
	require.Equal(t, "auto_approved", summary["reason_code"])
}

func TestBuildMerchantApprovalTxParams_IncludesImagePayloads(t *testing.T) {
	application := merchantReviewTestApplication()

	arg, err := buildMerchantApprovalTxParams(application)
	require.NoError(t, err)
	require.Equal(t, application.ID, arg.ApplicationID)
	require.Equal(t, application.UserID, arg.UserID)
	require.Equal(t, application.RegionID.Int64, arg.RegionID)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(arg.AppData, &payload))
	require.Equal(t, application.BusinessLicenseNumber, payload["business_license_number"])
	require.Equal(t, application.LegalPersonName, payload["legal_person_name"])
	require.Equal(t, application.LegalPersonIDNumber, payload["legal_person_id_number"])
	require.Equal(t, float64(application.BusinessLicenseMediaAssetID.Int64), payload["business_license_media_asset_id"])
	require.Equal(t, float64(application.FoodPermitMediaAssetID.Int64), payload["food_permit_media_asset_id"])
	require.Equal(t, []any{"storefront-a.jpg", "storefront-b.jpg"}, payload["storefront_images"])
	require.Equal(t, []any{"environment-a.jpg"}, payload["environment_images"])
}

func TestBuildMerchantApprovalTxParams_RequiresRegionID(t *testing.T) {
	application := merchantReviewTestApplication()
	application.RegionID = pgtype.Int8{}

	_, err := buildMerchantApprovalTxParams(application)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing region_id")
}
