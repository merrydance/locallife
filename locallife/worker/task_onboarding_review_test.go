package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type onboardingWorkerCredentialGovernanceStoreStub struct {
	activateMerchantFn func(context.Context, db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	getMerchantFn      func(context.Context, pgtype.Int8) ([]db.CredentialLedger, error)
	restoreMerchantFn  func(context.Context, db.RestoreMerchantCredentialGovernanceTxParams) (int64, error)
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) ActivateMerchantCredentialLedgersTx(ctx context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	if stub.activateMerchantFn == nil {
		return nil, fmt.Errorf("unexpected ActivateMerchantCredentialLedgersTx call")
	}
	return stub.activateMerchantFn(ctx, arg)
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) ActivateRiderCredentialLedgersTx(context.Context, db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	return nil, fmt.Errorf("unexpected ActivateRiderCredentialLedgersTx call")
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) GetActiveMerchantCredentialLedgers(ctx context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
	if stub.getMerchantFn == nil {
		return nil, fmt.Errorf("unexpected GetActiveMerchantCredentialLedgers call")
	}
	return stub.getMerchantFn(ctx, merchantID)
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) GetActiveRiderCredentialLedgers(context.Context, pgtype.Int8) ([]db.CredentialLedger, error) {
	return nil, fmt.Errorf("unexpected GetActiveRiderCredentialLedgers call")
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) RestoreMerchantCredentialGovernanceTx(ctx context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
	if stub.restoreMerchantFn == nil {
		return 0, fmt.Errorf("unexpected RestoreMerchantCredentialGovernanceTx call")
	}
	return stub.restoreMerchantFn(ctx, arg)
}

func (stub onboardingWorkerCredentialGovernanceStoreStub) RestoreRiderCredentialGovernanceTx(context.Context, db.RestoreRiderCredentialGovernanceTxParams) (int64, error) {
	return 0, fmt.Errorf("unexpected RestoreRiderCredentialGovernanceTx call")
}

func TestProcessTaskOnboardingReview_RiderApproved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	application := db.RiderApplication{
		ID:                     41,
		UserID:                 9,
		Status:                 db.RiderApplicationStatusSubmitted,
		RealName:               pgtype.Text{String: "张三", Valid: true},
		Phone:                  pgtype.Text{String: "13812345678", Valid: true},
		IDCardOcr:              []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"20350101"}`),
		HealthCertMediaAssetID: pgtype.Int8{Int64: 5, Valid: true},
		HealthCertOcr:          []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030年12月31日"}`),
	}

	store.EXPECT().
		GetOnboardingReviewRun(gomock.Any(), int64(801)).
		Return(db.OnboardingReviewRun{
			ID:                 801,
			ApplicationType:    onboardingReviewApplicationTypeRider,
			RiderApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
			RunStatus:          "queued",
		}, nil)

	store.EXPECT().
		GetRiderApplication(gomock.Any(), application.ID).
		Return(application, nil)

	store.EXPECT().
		ApproveRiderApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveRiderApplicationTxParams) (db.ApproveRiderApplicationTxResult, error) {
			require.Equal(t, application.ID, arg.ApplicationID)
			require.Equal(t, application.RealName.String, arg.RiderRealName)
			approvedApplication := application
			approvedApplication.Status = db.RiderApplicationStatusApproved
			return db.ApproveRiderApplicationTxResult{
				Application: approvedApplication,
				Rider: db.Rider{
					ID:     88,
					UserID: application.UserID,
				},
			}, nil
		})

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.onboardingReviewSvc = nil
	processor.credentialGovSvc = nil
	processor.riderReviewSvc = logic.NewRiderOnboardingReviewService(store, nil, nil)

	payloadBytes, err := json.Marshal(OnboardingReviewPayload{
		ReviewRunID:     801,
		ApplicationID:   application.ID,
		ApplicationType: onboardingReviewApplicationTypeRider,
		RequestedBy:     application.UserID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(TaskOnboardingReview, payloadBytes, asynq.ProcessIn(1*time.Second))
	err = processor.ProcessTaskOnboardingReview(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOnboardingReview_SkipsCompletedRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOnboardingReviewRun(gomock.Any(), int64(999)).
		Return(db.OnboardingReviewRun{
			ID:                    999,
			ApplicationType:       onboardingReviewApplicationTypeMerchant,
			MerchantApplicationID: pgtype.Int8{Int64: 77, Valid: true},
			RunStatus:             "completed",
		}, nil)

	processor := NewTestTaskProcessor(store, nil, nil, nil)

	payloadBytes, err := json.Marshal(OnboardingReviewPayload{
		ReviewRunID:     999,
		ApplicationID:   77,
		ApplicationType: onboardingReviewApplicationTypeMerchant,
		RequestedBy:     12,
	})
	require.NoError(t, err)

	task := asynq.NewTask(TaskOnboardingReview, payloadBytes, asynq.ProcessIn(1*time.Second))
	err = processor.ProcessTaskOnboardingReview(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOnboardingReview_MerchantApproved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	application := db.MerchantApplication{
		ID:                          51,
		UserID:                      19,
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
	}

	store.EXPECT().
		GetOnboardingReviewRun(gomock.Any(), int64(901)).
		Return(db.OnboardingReviewRun{
			ID:                    901,
			ApplicationType:       onboardingReviewApplicationTypeMerchant,
			MerchantApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
			RunStatus:             "queued",
		}, nil)

	store.EXPECT().
		GetMerchantApplication(gomock.Any(), application.ID).
		Return(application, nil)

	store.EXPECT().
		ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
			require.Equal(t, application.ID, arg.ApplicationID)
			require.Equal(t, application.MerchantName, arg.MerchantName)
			approvedApplication := application
			approvedApplication.Status = "approved"
			return db.ApproveMerchantApplicationTxResult{
				Application: approvedApplication,
				Merchant: db.Merchant{
					ID:          66,
					OwnerUserID: application.UserID,
				},
			}, nil
		})

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.onboardingReviewSvc = nil
	processor.credentialGovSvc = nil
	processor.merchantReviewSvc = logic.NewMerchantOnboardingReviewService(store, nil, nil)

	payloadBytes, err := json.Marshal(OnboardingReviewPayload{
		ReviewRunID:     901,
		ApplicationID:   application.ID,
		ApplicationType: onboardingReviewApplicationTypeMerchant,
		RequestedBy:     application.UserID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(TaskOnboardingReview, payloadBytes, asynq.ProcessIn(1*time.Second))
	err = processor.ProcessTaskOnboardingReview(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskOnboardingReview_MerchantApprovedActivatesCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	application := db.MerchantApplication{
		ID:                          52,
		UserID:                      20,
		Status:                      "submitted",
		MerchantName:                "测试轻食店",
		ContactPhone:                "13912345678",
		BusinessAddress:             "北京市朝阳区测试路200号",
		RegionID:                    pgtype.Int8{Int64: 9, Valid: true},
		BusinessLicenseNumber:       "91110000MA22345678",
		LegalPersonName:             "李四",
		LegalPersonIDNumber:         "110101199201011234",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 21, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 22, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 23, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 24, Valid: true},
		BusinessLicenseOcr:          []byte(`{"ocr_job_id":201,"reg_num":"91110000MA22345678","enterprise_name":"测试轻食店","legal_representative":"李四","address":"北京市朝阳区测试路200号","business_scope":"餐饮服务","valid_period":"2020年01月01日至2040年01月01日","credit_code":"91110000MA22345678"}`),
		FoodPermitOcr:               []byte(`{"ocr_job_id":202,"permit_no":"JY11105000000002","company_name":"测试轻食店","operator_name":"李四","valid_to":"2031年12月31日"}`),
	}

	store.EXPECT().
		GetOnboardingReviewRun(gomock.Any(), int64(902)).
		Return(db.OnboardingReviewRun{
			ID:                    902,
			ApplicationType:       onboardingReviewApplicationTypeMerchant,
			MerchantApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
			RunStatus:             "queued",
		}, nil)

	store.EXPECT().
		GetMerchantApplication(gomock.Any(), application.ID).
		Return(application, nil)

	store.EXPECT().
		ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
			require.Equal(t, application.ID, arg.ApplicationID)
			approvedApplication := application
			approvedApplication.Status = "approved"
			return db.ApproveMerchantApplicationTxResult{
				Application: approvedApplication,
				Merchant:    db.Merchant{ID: 67, OwnerUserID: application.UserID},
			}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(902)).
		Return(db.OnboardingReviewRun{ID: 902, ApplicationType: "merchant", RunStatus: "processing", Stage: "review"}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, int64(902), arg.ID)
			return db.OnboardingReviewRun{
				ID:         902,
				Stage:      "review",
				RunStatus:  "completed",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				OcrJobRefs: []int64{201, 202},
			}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			return db.MerchantApplication{ID: application.ID, ReviewSummary: arg.ReviewSummary}, nil
		})

	governanceSvc := logic.NewCredentialGovernanceService(onboardingWorkerCredentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(67), arg.MerchantID)
			require.Len(t, arg.Entries, 2)
			require.Equal(t, int64(902), arg.Entries[0].ReviewRunID.Int64)
			require.Equal(t, db.CredentialDocumentTypeBusinessLicense, arg.Entries[0].DocumentType)
			require.Equal(t, db.CredentialDocumentTypeFoodPermit, arg.Entries[1].DocumentType)
			return []db.CredentialLedger{{ID: 301}, {ID: 302}}, nil
		},
		getMerchantFn: func(_ context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(67), merchantID.Int64)
			return []db.CredentialLedger{
				{ID: 301, DocumentType: db.CredentialDocumentTypeBusinessLicense, ExpiresAt: pgtype.Timestamptz{Time: time.Now().AddDate(1, 0, 0), Valid: true}},
				{ID: 302, DocumentType: db.CredentialDocumentTypeFoodPermit, ExpiresAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 6, 0), Valid: true}},
			}, nil
		},
		restoreMerchantFn: func(_ context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			require.Equal(t, int64(67), arg.MerchantID)
			require.Equal(t, []int64{301, 302}, arg.CredentialLedgerIDs)
			return 1, nil
		},
	})
	require.NotNil(t, governanceSvc)

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.onboardingReviewSvc = logic.NewOnboardingReviewService(store)
	processor.credentialGovSvc = governanceSvc
	processor.merchantReviewSvc = logic.NewMerchantOnboardingReviewService(store, processor.onboardingReviewSvc, governanceSvc)

	payloadBytes, err := json.Marshal(OnboardingReviewPayload{
		ReviewRunID:     902,
		ApplicationID:   application.ID,
		ApplicationType: onboardingReviewApplicationTypeMerchant,
		RequestedBy:     application.UserID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(TaskOnboardingReview, payloadBytes, asynq.ProcessIn(1*time.Second))
	err = processor.ProcessTaskOnboardingReview(context.Background(), task)
	require.NoError(t, err)
}
