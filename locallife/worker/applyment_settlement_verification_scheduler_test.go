package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type applymentSettlementVerificationTestDistributor struct {
	worker.NoopTaskDistributor
	sendNotification func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error
}

func (d applymentSettlementVerificationTestDistributor) DistributeTaskSendNotification(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
	if d.sendNotification != nil {
		return d.sendNotification(ctx, payload, opts...)
	}
	return nil
}

func expectSettlementVerificationFactApplied(t *testing.T, store *mockdb.MockStore, applicationID int64, item db.ListMerchantApplymentsPendingSettlementVerificationRow, verifyResult string, verifyFailReason string, runAt time.Time, firstTradeAt time.Time, checkCount int32, expectedStatus string) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilitySettlement, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectSettlement, arg.ExternalObjectType)
		require.Equal(t, item.SubMchID.String, arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.Equal(t, "ordinary_service_provider_applyment", arg.BusinessObjectType.String)
		require.Equal(t, item.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, verifyResult, arg.UpstreamState)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		require.Equal(t, float64(item.ID), payload["applyment_id"])
		require.Equal(t, float64(item.SubjectID), payload["merchant_id"])
		require.Equal(t, item.SubMchID.String, payload["sub_mch_id"])
		require.Equal(t, verifyResult, payload["verify_result"])
		require.Equal(t, verifyFailReason, payload["verify_fail_reason"])
		require.Equal(t, float64(checkCount), payload["settlement_verify_check_count"])
		return db.ExternalPaymentFact{ID: 9031, IsTerminal: arg.IsTerminal, TerminalStatus: arg.TerminalStatus}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             9031,
		Consumer:           "settlement_domain",
		BusinessObjectType: "ordinary_service_provider_applyment",
		BusinessObjectID:   item.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: applicationID, FactID: 9031, Consumer: "settlement_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: item.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), applicationID).Return(db.ExternalPaymentFactApplication{ID: applicationID, FactID: 9031, Consumer: "settlement_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: item.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)
	rawResource, err := json.Marshal(map[string]any{
		"applyment_id":                      item.ID,
		"merchant_id":                       item.SubjectID,
		"sub_mch_id":                        item.SubMchID.String,
		"verify_result":                     verifyResult,
		"verify_fail_reason":                verifyFailReason,
		"settlement_verify_first_trade_at":  firstTradeAt.UTC().Format(time.RFC3339Nano),
		"settlement_verify_last_checked_at": runAt.UTC().Format(time.RFC3339Nano),
		"settlement_verify_check_count":     checkCount,
	})
	require.NoError(t, err)
	terminalStatus := db.ExternalPaymentTerminalStatusProcessing
	if verifyResult == "VERIFY_SUCCESS" {
		terminalStatus = db.ExternalPaymentTerminalStatusSuccess
	}
	if verifyResult == "VERIFY_FAIL" {
		terminalStatus = db.ExternalPaymentTerminalStatusFailed
	}
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(9031)).Return(db.ExternalPaymentFact{
		ID:                 9031,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelOrdinaryServiceProvider,
		Capability:         db.ExternalPaymentCapabilitySettlement,
		ExternalObjectType: db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:  item.SubMchID.String,
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: item.ID, Valid: true},
		UpstreamState:      verifyResult,
		TerminalStatus:     terminalStatus,
		IsTerminal:         terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:        rawResource,
	}, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), item.ID).Return(db.EcommerceApplyment{ID: item.ID, SubjectType: "merchant", SubjectID: item.SubjectID, SubMchID: item.SubMchID}, nil)
	store.EXPECT().UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
		require.Equal(t, item.ID, arg.ID)
		require.True(t, arg.SettlementVerifyFirstTradeAt.Valid)
		require.Equal(t, firstTradeAt, arg.SettlementVerifyFirstTradeAt.Time)
		require.True(t, arg.SettlementVerifyLastCheckedAt.Valid)
		require.Equal(t, runAt, arg.SettlementVerifyLastCheckedAt.Time)
		require.True(t, arg.SettlementVerifyCheckCount.Valid)
		require.Equal(t, checkCount, arg.SettlementVerifyCheckCount.Int32)
		require.True(t, arg.SettlementVerifyStatus.Valid)
		require.Equal(t, expectedStatus, arg.SettlementVerifyStatus.String)
		require.True(t, arg.SettlementVerifyFailReason.Valid)
		require.Equal(t, verifyFailReason, arg.SettlementVerifyFailReason.String)
		return db.EcommerceApplyment{ID: item.ID, SubjectType: "merchant", SubjectID: item.SubjectID, SubMchID: item.SubMchID}, nil
	})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 9031, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: applicationID, FactID: 9031, Consumer: "settlement_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: item.ID, Status: db.ExternalPaymentFactApplicationStatusApplied}, nil)
}

func TestApplymentSettlementVerificationSchedulerMarksVerifyingCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := applymentSettlementVerificationTestDistributor{}
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	runAt := time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC)
	firstTradeAt := runAt.Add(-2 * time.Hour)
	item := db.ListMerchantApplymentsPendingSettlementVerificationRow{
		ID:                         11,
		SubjectID:                  21,
		SubMchID:                   pgtype.Text{String: "sub_mch_21", Valid: true},
		SettlementVerifyCheckCount: 0,
		FirstPaidAt:                firstTradeAt,
	}

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{item}, nil)

	ordinaryClient.EXPECT().
		QuerySettlement(gomock.Any(), ospcontracts.SettlementQueryRequest{SubMchID: "sub_mch_21"}).
		Return(&ospcontracts.SettlementQueryResponse{VerifyResult: ospcontracts.SettlementVerifyResultIng}, nil)
	expectSettlementVerificationFactApplied(t, store, 9131, item, "VERIFYING", "", runAt, firstTradeAt, 1, "verifying")

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ordinaryClient)
	scheduler.SetNowFuncForTest(func() time.Time { return runAt })
	scheduler.RunOnce()
}

func TestApplymentSettlementVerificationSchedulerNotifiesOperatorOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := applymentSettlementVerificationTestDistributor{
		sendNotification: func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(71), payload.UserID)
			require.Equal(t, "system", payload.Type)
			require.Equal(t, "商户结算卡校验失败", payload.Title)
			require.Contains(t, payload.Content, "南山店")
			require.Contains(t, payload.Content, "银行卡户名不一致")
			require.Equal(t, "merchant", payload.RelatedType)
			require.Equal(t, int64(41), payload.RelatedID)
			require.True(t, payload.IgnorePreferences)
			return nil
		},
	}
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	runAt := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
	firstTradeAt := runAt.Add(-24 * time.Hour)
	item := db.ListMerchantApplymentsPendingSettlementVerificationRow{
		ID:                         31,
		SubjectID:                  41,
		SubMchID:                   pgtype.Text{String: "sub_mch_41", Valid: true},
		SettlementVerifyCheckCount: 1,
		FirstPaidAt:                firstTradeAt,
	}

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{item}, nil)

	ordinaryClient.EXPECT().
		QuerySettlement(gomock.Any(), ospcontracts.SettlementQueryRequest{SubMchID: "sub_mch_41"}).
		Return(&ospcontracts.SettlementQueryResponse{
			VerifyResult:     ospcontracts.SettlementVerifyResultFail,
			VerifyFailReason: "银行卡户名不一致",
		}, nil)
	expectSettlementVerificationFactApplied(t, store, 9132, item, "VERIFY_FAIL", "银行卡户名不一致", runAt, firstTradeAt, 2, "fail")

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(41)).
		Return(db.Merchant{ID: 41, Name: "南山店", RegionID: 51}, nil)

	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), int64(51)).
		Return(db.Operator{ID: 61, UserID: 71, Name: "南山区运营商", Status: "active"}, nil)

	store.EXPECT().
		MarkEcommerceApplymentSettlementVerifyFailedNotified(gomock.Any(), int64(31)).
		Return(db.EcommerceApplyment{ID: 31}, nil)

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ordinaryClient)
	scheduler.SetNowFuncForTest(func() time.Time { return runAt })
	scheduler.RunOnce()
}

func TestApplymentSettlementVerificationSchedulerMarksTerminalFailureOnInvalidSettlementQueryInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := applymentSettlementVerificationTestDistributor{}
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{{
			ID:                         51,
			SubjectID:                  61,
			SubMchID:                   pgtype.Text{String: "sub_mch_61", Valid: true},
			SettlementVerifyCheckCount: 0,
			FirstPaidAt:                time.Now().Add(-4 * time.Hour),
		}}, nil)

	ordinaryClient.EXPECT().
		QuerySettlement(gomock.Any(), ospcontracts.SettlementQueryRequest{SubMchID: "sub_mch_61"}).
		Return(nil, &ordinaryserviceprovider.ProviderError{
			Operation:    "query ordinary service provider settlement",
			Category:     ordinaryserviceprovider.ErrorCategoryValidation,
			ProviderCode: "LOCAL_VALIDATION_ERROR",
		})

	store.EXPECT().
		UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
			require.Equal(t, int64(51), arg.ID)
			require.Equal(t, int32(1), arg.SettlementVerifyCheckCount.Int32)
			require.Equal(t, "fail", arg.SettlementVerifyStatus.String)
			require.Equal(t, "结算卡验卡巡检请求无效，请联系平台核验微信二级商户号和请求参数", arg.SettlementVerifyFailReason.String)
			return db.EcommerceApplyment{ID: 51}, nil
		})

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ordinaryClient)
	scheduler.RunOnce()
}

func TestApplymentSettlementVerificationSchedulerMarksTerminalFailureOnSettlementContractDrift(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := applymentSettlementVerificationTestDistributor{}
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{{
			ID:                         71,
			SubjectID:                  81,
			SubMchID:                   pgtype.Text{String: "1900000081", Valid: true},
			SettlementVerifyCheckCount: 1,
			FirstPaidAt:                time.Now().Add(-6 * time.Hour),
		}}, nil)

	ordinaryClient.EXPECT().
		QuerySettlement(gomock.Any(), ospcontracts.SettlementQueryRequest{SubMchID: "1900000081"}).
		Return(nil, &ordinaryserviceprovider.ProviderError{
			Operation:    "query ordinary service provider settlement",
			Category:     ordinaryserviceprovider.ErrorCategoryProvider,
			ProviderCode: "LOCAL_RESPONSE_DECODE_ERROR",
		})

	store.EXPECT().
		UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
			require.Equal(t, int64(71), arg.ID)
			require.Equal(t, int32(2), arg.SettlementVerifyCheckCount.Int32)
			require.Equal(t, "fail", arg.SettlementVerifyStatus.String)
			require.Equal(t, "微信结算卡查询响应不符合预期，请联系平台处理", arg.SettlementVerifyFailReason.String)
			return db.EcommerceApplyment{ID: 71}, nil
		})

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ordinaryClient)
	scheduler.RunOnce()
}

func TestApplymentSettlementVerificationSchedulerRunOnceSkipsWithoutOrdinaryServiceProviderClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := applymentSettlementVerificationTestDistributor{}

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, nil)
	scheduler.RunOnce()
}
