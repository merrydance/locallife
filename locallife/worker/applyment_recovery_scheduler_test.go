package worker_test

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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type applymentRecoveryTestDistributor struct {
	worker.NoopTaskDistributor
	onProcessApplymentResult func(context.Context, *worker.ApplymentResultPayload, ...asynq.Option) error
	onProcessFactApplication func(context.Context, *worker.PaymentFactApplicationPayload, ...asynq.Option) error
}

func (d *applymentRecoveryTestDistributor) DistributeTaskProcessApplymentResult(ctx context.Context, payload *worker.ApplymentResultPayload, opts ...asynq.Option) error {
	if d.onProcessApplymentResult != nil {
		return d.onProcessApplymentResult(ctx, payload, opts...)
	}
	return nil
}

func (d *applymentRecoveryTestDistributor) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	if d.onProcessFactApplication != nil {
		return d.onProcessFactApplication(ctx, payload, opts...)
	}
	return nil
}

func TestApplymentRecoverySchedulerRunOnceRequeuesLocalUnprocessedFollowUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           11,
			SubjectType:  "merchant",
			SubjectID:    22,
			OutRequestNo: "APPLY_RECOVERY_001",
			Status:       "finish",
			SubMchID:     pgtype.Text{String: "sub_mch_001", Valid: true},
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	distributor.onProcessApplymentResult = func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(11), payload.ApplymentID)
		require.Equal(t, "finish", payload.ApplymentStatus)
		require.Equal(t, "sub_mch_001", payload.SubMchID)
		return nil
	}

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceQueriesSubmittedMerchantAndReconcilesSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           31,
			SubjectType:  "merchant",
			SubjectID:    41,
			OutRequestNo: "APPLY_RECOVERY_002",
			ApplymentID:  pgtype.Int8{Int64: 991, Valid: true},
			Status:       "submitted",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(991)).
		Return(&wechatcontracts.EcommerceApplymentQueryResponse{
			ApplymentID:    991,
			OutRequestNo:   "APPLY_RECOVERY_002",
			ApplymentState: "FINISH",
			SubMchID:       "sub_mch_991",
		}, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectApplyment, arg.ExternalObjectType)
			require.Equal(t, "APPLY_RECOVERY_002", arg.ExternalObjectKey)
			require.Equal(t, db.ExternalPaymentBusinessOwnerApplyment, arg.BusinessOwner.String)
			require.Equal(t, "ecommerce_applyment", arg.BusinessObjectType.String)
			require.Equal(t, int64(31), arg.BusinessObjectID.Int64)
			require.Equal(t, "FINISH", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			return db.ExternalPaymentFact{ID: 9031, IsTerminal: true}, nil
		})
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             9031,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ecommerce_applyment",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 9131, FactID: 9031, Consumer: "applyment_domain", BusinessObjectType: "ecommerce_applyment", BusinessObjectID: 31, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	distributor.onProcessFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9131), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceQueriesPendingMerchantAndReconcilesFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	accountValidation := &wechatcontracts.EcommerceApplymentAccountValidation{AccountName: "测试商户有限公司", Remark: "请汇款 0.01 元"}

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           32,
			SubjectType:  "merchant",
			SubjectID:    42,
			OutRequestNo: "APPLY_RECOVERY_002_PENDING",
			ApplymentID:  pgtype.Int8{Int64: 992, Valid: true},
			Status:       "auditing",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(992)).
		Return(&wechatcontracts.EcommerceApplymentQueryResponse{
			ApplymentID:        992,
			OutRequestNo:       "APPLY_RECOVERY_002_PENDING",
			ApplymentState:     "ACCOUNT_NEED_VERIFY",
			SignURL:            "https://pay.weixin.qq.com/sign/992",
			SignState:          "UNSIGNED",
			LegalValidationURL: "https://wx.example.com/legal-992",
			AccountValidation:  accountValidation,
		}, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectApplyment, arg.ExternalObjectType)
			require.Equal(t, "APPLY_RECOVERY_002_PENDING", arg.ExternalObjectKey)
			require.Equal(t, db.ExternalPaymentBusinessOwnerApplyment, arg.BusinessOwner.String)
			require.Equal(t, "ecommerce_applyment", arg.BusinessObjectType.String)
			require.Equal(t, int64(32), arg.BusinessObjectID.Int64)
			require.Equal(t, "ACCOUNT_NEED_VERIFY", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, arg.TerminalStatus)
			require.False(t, arg.IsTerminal)
			require.Contains(t, string(arg.RawResource), "legal_validation_url")
			require.Contains(t, string(arg.RawResource), "account_validation")
			return db.ExternalPaymentFact{ID: 9032, IsTerminal: false}, nil
		})
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             9032,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ecommerce_applyment",
			BusinessObjectID:   32,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 9132, FactID: 9032, Consumer: "applyment_domain", BusinessObjectType: "ecommerce_applyment", BusinessObjectID: 32, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	distributor.onProcessFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9132), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceIgnoresOperatorRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           51,
			SubjectType:  "operator",
			SubjectID:    61,
			OutRequestNo: "APPLY_RECOVERY_003",
			Status:       "auditing",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceIgnoresOperatorSubmittedRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           71,
			SubjectType:  "operator",
			SubjectID:    81,
			OutRequestNo: "APPLY_RECOVERY_004",
			ApplymentID:  pgtype.Int8{Int64: 7711, Valid: true},
			Status:       "submitted",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceFallsBackToOutRequestNoAfterIDQueryFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           91,
			SubjectType:  "merchant",
			SubjectID:    92,
			OutRequestNo: "APPLY_RECOVERY_005",
			ApplymentID:  pgtype.Int8{Int64: 9901, Valid: true},
			Status:       "auditing",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(9901)).
		Return(nil, errors.New("wechat id lookup failed"))

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_RECOVERY_005").
		Return(&wechatcontracts.EcommerceApplymentQueryResponse{
			ApplymentID:    9901,
			OutRequestNo:   "APPLY_RECOVERY_005",
			ApplymentState: "AUDITING",
			SignState:      "UNSIGNED",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentStatusParams) (db.EcommerceApplyment, error) {
			require.Equal(t, int64(91), arg.ID)
			require.Equal(t, "auditing", arg.Status)
			require.Equal(t, pgtype.Text{String: "UNSIGNED", Valid: true}, arg.SignState)
			return db.EcommerceApplyment{ID: arg.ID, Status: arg.Status}, nil
		})

	distributor.onProcessApplymentResult = func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
		require.Equal(t, "AUDITING", payload.ApplymentState)
		require.Equal(t, "UNSIGNED", payload.SignState)
		require.Equal(t, "auditing", payload.ApplymentStatus)
		return nil
	}

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceAlertsUnsupportedStateWithoutLegacyStatusUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{
		onProcessApplymentResult: func(_ context.Context, _ *worker.ApplymentResultPayload, _ ...asynq.Option) error {
			require.FailNow(t, "unsupported applyment state must not enqueue legacy follow-up")
			return nil
		},
		onProcessFactApplication: func(_ context.Context, _ *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
			require.FailNow(t, "unsupported applyment state must not enqueue fact application")
			return nil
		},
	}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           93,
			SubjectType:  "merchant",
			SubjectID:    94,
			OutRequestNo: "APPLY_RECOVERY_UNSUPPORTED_001",
			ApplymentID:  pgtype.Int8{Int64: 9903, Valid: true},
			Status:       "auditing",
			SubMchID:     pgtype.Text{String: "sub_mch_existing", Valid: true},
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(9903)).
		Return(&wechatcontracts.EcommerceApplymentQueryResponse{
			ApplymentID:        9903,
			OutRequestNo:       "APPLY_RECOVERY_UNSUPPORTED_001",
			ApplymentState:     "NEW_UPSTREAM_STATE",
			ApplymentStateDesc: "new state from wechat",
			SignState:          "UNSIGNED",
			SubMchID:           "sub_mch_new",
		}, nil)

	store.EXPECT().
		CreatePlatformAlertEvent(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePlatformAlertEventParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
			require.Equal(t, string(worker.AlertTypeSystemError), arg.AlertType)
			require.Equal(t, string(worker.AlertLevelCritical), arg.Level)
			require.Equal(t, int64(93), arg.RelatedID)
			require.Equal(t, "ecommerce_applyment", arg.RelatedType)

			var extra map[string]any
			require.NoError(t, json.Unmarshal(arg.Extra, &extra))
			require.Equal(t, "NEW_UPSTREAM_STATE", extra["applyment_state"])
			require.Equal(t, "auditing", extra["local_status"])
			require.Equal(t, true, extra["requires_mapping"])
			return db.PlatformAlertEvent{ID: 9301}, nil
		})

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceSkipsRemoteQueryWithoutEcommerceClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           101,
			SubjectType:  "merchant",
			SubjectID:    102,
			OutRequestNo: "APPLY_RECOVERY_NO_ECOM_001",
			ApplymentID:  pgtype.Int8{Int64: 9101, Valid: true},
			Status:       "submitted",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceEnqueuesFrozenFollowUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &applymentRecoveryTestDistributor{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListEcommerceApplymentsPendingFollowUp(gomock.Any(), gomock.Any()).
		Return([]db.EcommerceApplymentPendingFollowUp{{
			ID:           111,
			SubjectType:  "merchant",
			SubjectID:    211,
			OutRequestNo: "APPLY_RECOVERY_006",
			ApplymentID:  pgtype.Int8{Int64: 6060, Valid: true},
			Status:       "auditing",
			UpdatedAt:    time.Now().Add(-5 * time.Minute),
		}}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(6060)).
		Return(&wechatcontracts.EcommerceApplymentQueryResponse{
			ApplymentID:        6060,
			OutRequestNo:       "APPLY_RECOVERY_006",
			ApplymentState:     "FROZEN",
			ApplymentStateDesc: "已冻结",
			AuditDetail: []wechatcontracts.ApplymentAuditDetail{{
				ParamName:    "id_card_copy",
				RejectReason: "身份证图片存在问题",
			}},
		}, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "FROZEN", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusClosed, arg.TerminalStatus)
			return db.ExternalPaymentFact{ID: 9111, IsTerminal: true}, nil
		})

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             9111,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ecommerce_applyment",
			BusinessObjectID:   111,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 9211, FactID: 9111, Consumer: "applyment_domain", BusinessObjectType: "ecommerce_applyment", BusinessObjectID: 111, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	distributor.onProcessFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9211), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}
