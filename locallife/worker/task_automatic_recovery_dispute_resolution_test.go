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
	"github.com/merrydance/locallife/logic"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type automaticRecoveryDisputeResolutionTestDistributor struct {
	sendNotificationCalls int
	lastNotification      *SendNotificationPayload
}

func reviewedRecoveryDisputeForResolution(base db.RecoveryDispute, status, notes string) db.RecoveryDispute {
	return db.RecoveryDispute{
		ID:                 base.ID,
		ClaimID:            base.ClaimID,
		AppellantType:      base.AppellantType,
		AppellantID:        base.AppellantID,
		Reason:             base.Reason,
		Status:             status,
		ReviewerID:         base.ReviewerID,
		ReviewNotes:        pgtype.Text{String: notes, Valid: true},
		ReviewedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CompensationAmount: base.CompensationAmount,
		CompensatedAt:      base.CompensatedAt,
		RegionID:           base.RegionID,
		CreatedAt:          base.CreatedAt,
	}
}

func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskSendNotification(ctx context.Context, payload *SendNotificationPayload, opts ...asynq.Option) error {
	d.sendNotificationCalls++
	clone := *payload
	d.lastNotification = &clone
	return nil
}

func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskPaymentOrderTimeout(context.Context, *PayloadPaymentOrderTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskCombinedPaymentOrderTimeout(context.Context, *PayloadCombinedPaymentOrderTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskReservationPaymentTimeout(context.Context, *PayloadReservationPaymentTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskOrderPaymentTimeout(context.Context, *PayloadOrderPaymentTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskReservationNoShowAlert(context.Context, *PayloadReservationNoShowAlert, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskReservationFoodSafetyAlert(context.Context, *PayloadReservationFoodSafetyAlert, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessPaymentSuccess(context.Context, *PaymentSuccessPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessRefund(context.Context, *PayloadProcessRefund, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessRefundResult(context.Context, *RefundResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessProfitSharing(context.Context, *ProfitSharingPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessApplymentResult(context.Context, *ApplymentResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessProfitSharingResult(context.Context, *ProfitSharingResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessProfitSharingReturnResult(context.Context, *ProfitSharingReturnResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessMerchantWithdrawResult(context.Context, *MerchantWithdrawResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessMerchantCancelWithdrawResult(context.Context, *MerchantCancelWithdrawResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskCheckMerchantForeignObject(context.Context, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskCheckRiderDamage(context.Context, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessRecoveryDisputeResult(context.Context, *ProcessRecoveryDisputeResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskAutomaticRecoveryDisputeResolution(context.Context, *AutomaticRecoveryDisputeResolutionPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskClaimPayout(context.Context, *ClaimPayoutPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskClaimBehaviorAction(context.Context, *ClaimBehaviorActionPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskMerchantApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskOperatorApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskOperatorApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskRiderApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskRiderApplicationHealthCertOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskGroupApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskGroupApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskUploadShippingInfo(context.Context, *UploadShippingInfoPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskSyncComplaints(context.Context, *SyncComplaintsPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskProcessAnomalyRefund(context.Context, *PayloadProcessAnomalyRefund, ...asynq.Option) error {
	return nil
}
func (d *automaticRecoveryDisputeResolutionTestDistributor) DistributeTaskPrintOrder(context.Context, *PrintOrderPayload, ...asynq.Option) error {
	return nil
}

func TestProcessTaskAutomaticRecoveryDisputeResolution_ResolvesSubmittedRecoveryDispute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &automaticRecoveryDisputeResolutionTestDistributor{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)

	recoveryDispute := db.RecoveryDispute{
		ID:            31,
		ClaimID:       81,
		AppellantType: "rider",
		AppellantID:   91,
		Reason:        "天气原因导致延迟",
		Status:        "submitted",
		RegionID:      101,
		CreatedAt:     time.Now(),
	}
	claim := struct {
		ID          int64
		OrderID     int64
		ClaimType   string
		ClaimAmount int64
		MerchantID  int64
		RegionID    int64
		RiderID     pgtype.Int8
		CreatedAt   time.Time
	}{
		ID:          81,
		OrderID:     701,
		ClaimType:   "delay",
		ClaimAmount: 300,
		MerchantID:  55,
		RegionID:    101,
		RiderID:     pgtype.Int8{Int64: 91, Valid: true},
		CreatedAt:   time.Now(),
	}
	decision := db.BehaviorDecision{
		ID:                 501,
		ClaimID:            pgtype.Int8{Int64: recoveryDispute.ClaimID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	resolvedRecoveryDispute := reviewedRecoveryDisputeForResolution(recoveryDispute, "rejected", "系统复核确认最新行为判责仍指向当前申诉方，维持原判。")
	postProcess := db.GetRecoveryDisputeForPostProcessRow{
		RecoveryDisputeID: recoveryDispute.ID,
		ClaimID:           recoveryDispute.ClaimID,
		AppellantType:     recoveryDispute.AppellantType,
		AppellantID:       recoveryDispute.AppellantID,
		ClaimantUserID:    201,
		ClaimType:         claim.ClaimType,
		ClaimAmount:       claim.ClaimAmount,
		OrderID:           claim.OrderID,
		OrderNo:           "20240101120000123456",
		MerchantID:        claim.MerchantID,
		RiderID:           pgtype.Int8{Int64: recoveryDispute.AppellantID, Valid: true},
	}

	payloadBytes, err := json.Marshal(&AutomaticRecoveryDisputeResolutionPayload{RecoveryDisputeID: recoveryDispute.ID})
	require.NoError(t, err)
	task := asynq.NewTask(TaskAutomaticRecoveryDisputeResolution, payloadBytes)

	store.EXPECT().GetRecoveryDispute(gomock.Any(), recoveryDispute.ID).Return(recoveryDispute, nil)
	store.EXPECT().GetClaimRecoveryContextByClaimID(gomock.Any(), recoveryDispute.ClaimID).Return(db.GetClaimRecoveryContextByClaimIDRow{
		ClaimID:        claim.ID,
		OrderID:        claim.OrderID,
		MerchantID:     claim.MerchantID,
		RegionID:       claim.RegionID,
		RiderID:        claim.RiderID,
		ClaimCreatedAt: claim.CreatedAt,
	}, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().ReviewRecoveryDisputeWithCompensationTx(gomock.Any(), db.ReviewRecoveryDisputeWithCompensationTxParams{
		ID:                 recoveryDispute.ID,
		Status:             "rejected",
		DecisionID:         pgtype.Int8{Int64: decision.ID, Valid: true},
		ReviewerID:         pgtype.Int8{},
		ReviewNotes:        pgtype.Text{String: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", Valid: true},
		CompensationAmount: pgtype.Int8{},
	}).Return(db.ReviewRecoveryDisputeWithCompensationTxResult{
		RecoveryDispute: resolvedRecoveryDispute,
		PostProcess:     postProcess,
	}, nil)
	store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).Return(db.AuditLog{ID: 1}, nil)
	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), recoveryDispute.ClaimID).Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
	store.EXPECT().GetRider(gomock.Any(), recoveryDispute.AppellantID).Return(db.Rider{ID: recoveryDispute.AppellantID, UserID: 301}, nil)

	err = processor.ProcessTaskAutomaticRecoveryDisputeResolution(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 2, distributor.sendNotificationCalls)
}

func TestProcessTaskAutomaticRecoveryDisputeResolution_ReplaysPostProcessForResolvedRecoveryDispute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &automaticRecoveryDisputeResolutionTestDistributor{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)

	recoveryDispute := db.RecoveryDispute{
		ID:            41,
		ClaimID:       82,
		AppellantType: "merchant",
		AppellantID:   92,
		Reason:        "顾客签收时已核对",
		Status:        "approved",
		RegionID:      102,
		CreatedAt:     time.Now(),
	}
	claim := struct {
		ID          int64
		OrderID     int64
		ClaimType   string
		ClaimAmount int64
		MerchantID  int64
		RegionID    int64
		CreatedAt   time.Time
	}{
		ID:          82,
		OrderID:     702,
		ClaimType:   "missing-item",
		ClaimAmount: 500,
		MerchantID:  92,
		RegionID:    102,
		CreatedAt:   time.Now(),
	}
	decision := db.BehaviorDecision{
		ID:                 502,
		ClaimID:            pgtype.Int8{Int64: recoveryDispute.ClaimID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	postProcess := db.GetRecoveryDisputeForPostProcessRow{
		RecoveryDisputeID: recoveryDispute.ID,
		ClaimID:           recoveryDispute.ClaimID,
		AppellantType:     recoveryDispute.AppellantType,
		AppellantID:       recoveryDispute.AppellantID,
		ClaimantUserID:    202,
		ClaimType:         claim.ClaimType,
		ClaimAmount:       claim.ClaimAmount,
		OrderID:           claim.OrderID,
		OrderNo:           "20240101120000999999",
		MerchantID:        claim.MerchantID,
	}

	payloadBytes, err := json.Marshal(&AutomaticRecoveryDisputeResolutionPayload{RecoveryDisputeID: recoveryDispute.ID})
	require.NoError(t, err)
	task := asynq.NewTask(TaskAutomaticRecoveryDisputeResolution, payloadBytes)

	store.EXPECT().GetRecoveryDispute(gomock.Any(), recoveryDispute.ID).Return(recoveryDispute, nil)
	store.EXPECT().GetRecoveryDisputeForPostProcess(gomock.Any(), recoveryDispute.ID).Return(postProcess, nil)
	store.EXPECT().GetClaimRecoveryContextByClaimID(gomock.Any(), recoveryDispute.ClaimID).Return(db.GetClaimRecoveryContextByClaimIDRow{
		ClaimID:        claim.ID,
		OrderID:        claim.OrderID,
		MerchantID:     claim.MerchantID,
		RegionID:       claim.RegionID,
		ClaimCreatedAt: claim.CreatedAt,
	}, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().ListBehaviorActionsByDecision(gomock.Any(), decision.ID).Return(nil, nil)
	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), recoveryDispute.ClaimID).Return(db.ClaimRecovery{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: postProcess.ClaimantUserID}).Return(db.BehaviorBlocklist{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), recoveryDispute.AppellantID).Return(db.Merchant{ID: recoveryDispute.AppellantID, OwnerUserID: 302}, nil)

	err = processor.ProcessTaskAutomaticRecoveryDisputeResolution(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 2, distributor.sendNotificationCalls)
}

func TestBuildProcessRecoveryDisputeResultPayloadFindsReleaseActionAcrossRecoveryAndResolutionDecisionAnchors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	recoveryDispute := db.RecoveryDispute{
		ID:            51,
		ClaimID:       91,
		AppellantType: "merchant",
		AppellantID:   101,
		Status:        "approved",
		ReviewerID:    pgtype.Int8{Int64: 701, Valid: true},
	}
	postProcess := db.GetRecoveryDisputeForPostProcessRow{
		ClaimID:        recoveryDispute.ClaimID,
		AppellantType:  recoveryDispute.AppellantType,
		AppellantID:    recoveryDispute.AppellantID,
		ClaimantUserID: 202,
		ClaimType:      "damage",
		ClaimAmount:    500,
		OrderNo:        "20240101120000777777",
	}
	resolution := logic.AutomaticRecoveryDisputeResolution{
		Status:     "approved",
		DecisionID: pgtype.Int8{Int64: 702, Valid: true},
	}
	recovery := db.ClaimRecovery{
		ID:         901,
		ClaimID:    recoveryDispute.ClaimID,
		DecisionID: pgtype.Int8{Int64: 701, Valid: true},
	}
	releaseDetailBytes, err := json.Marshal(claimRecoveryReleaseActionDetail{
		Action:       "release_recovery_suspension",
		ClaimID:      recoveryDispute.ClaimID,
		RecoveryID:   recovery.ID,
		OrderID:      1201,
		TargetEntity: "merchant",
		Remark:       "approved recovery dispute release action created",
	})
	require.NoError(t, err)
	payoutDetailBytes, err := json.Marshal(claimPayoutActionDetail{
		RecoveryDisputeID: recoveryDispute.ID,
		ClaimID:           recoveryDispute.ClaimID,
		UserID:            postProcess.ClaimantUserID,
		Amount:            300,
		SourceType:        "platform",
	})
	require.NoError(t, err)

	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), recoveryDispute.ClaimID).Return(recovery, nil)
	store.EXPECT().ListBehaviorActionsByDecision(gomock.Any(), int64(701)).Return([]db.BehaviorAction{{
		ID:           801,
		DecisionID:   701,
		ActionType:   "release",
		TargetEntity: "merchant",
		Detail:       releaseDetailBytes,
	}}, nil)
	store.EXPECT().ListBehaviorActionsByDecision(gomock.Any(), int64(702)).Return([]db.BehaviorAction{{
		ID:           802,
		DecisionID:   702,
		ActionType:   "payout",
		TargetEntity: "user",
		Detail:       payoutDetailBytes,
	}}, nil)

	payload, err := processor.buildProcessRecoveryDisputeResultPayload(context.Background(), recoveryDispute, postProcess, resolution)
	require.NoError(t, err)
	require.Equal(t, int64(801), payload.ReleaseActionID)
	require.Equal(t, int64(802), payload.CompensationActionID)
}
