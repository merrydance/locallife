package worker_test

import (
	"context"
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
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestApplymentRecoverySchedulerRunOnceRequeuesLocalUnprocessedFollowUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

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

	distributor.EXPECT().
		DistributeTaskProcessApplymentResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ApplymentResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(11), payload.ApplymentID)
			require.Equal(t, "finish", payload.ApplymentStatus)
			require.Equal(t, "sub_mch_001", payload.SubMchID)
			return nil
		})

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceQueriesSubmittedMerchantAndReconcilesSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
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
		ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
			ApplymentID: 31,
			SubjectType: "merchant",
			SubjectID:   41,
			SubMchID:    "sub_mch_991",
		}).
		Return(nil)

	distributor.EXPECT().
		DistributeTaskProcessApplymentResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ApplymentResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
			require.Equal(t, "FINISH", payload.ApplymentState)
			require.Equal(t, "finish", payload.ApplymentStatus)
			require.Equal(t, "sub_mch_991", payload.SubMchID)
			return nil
		})

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceIgnoresOperatorRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

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
	distributor := mockwk.NewMockTaskDistributor(ctrl)

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
	distributor := mockwk.NewMockTaskDistributor(ctrl)
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

	distributor.EXPECT().
		DistributeTaskProcessApplymentResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ApplymentResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
			require.Equal(t, "AUDITING", payload.ApplymentState)
			require.Equal(t, "UNSIGNED", payload.SignState)
			require.Equal(t, "auditing", payload.ApplymentStatus)
			return nil
		})

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentRecoverySchedulerRunOnceEnqueuesFrozenFollowUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
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
		UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentStatusParams) (db.EcommerceApplyment, error) {
			require.Equal(t, int64(111), arg.ID)
			require.Equal(t, "frozen", arg.Status)
			require.Equal(t, pgtype.Text{String: "id_card_copy: 身份证图片存在问题", Valid: true}, arg.RejectReason)
			return db.EcommerceApplyment{ID: arg.ID, Status: arg.Status}, nil
		})

	distributor.EXPECT().
		DistributeTaskProcessApplymentResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ApplymentResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ApplymentResultPayload, _ ...asynq.Option) error {
			require.Equal(t, "FROZEN", payload.ApplymentState)
			require.Equal(t, "frozen", payload.ApplymentStatus)
			return nil
		})

	scheduler := worker.NewApplymentRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}
