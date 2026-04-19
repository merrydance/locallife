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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type automaticAppealResolutionTestDistributor struct {
	sendNotificationCalls int
	lastNotification      *SendNotificationPayload
}

func (d *automaticAppealResolutionTestDistributor) DistributeTaskSendNotification(ctx context.Context, payload *SendNotificationPayload, opts ...asynq.Option) error {
	d.sendNotificationCalls++
	clone := *payload
	d.lastNotification = &clone
	return nil
}

func (d *automaticAppealResolutionTestDistributor) DistributeTaskPaymentOrderTimeout(context.Context, *PayloadPaymentOrderTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskCombinedPaymentOrderTimeout(context.Context, *PayloadCombinedPaymentOrderTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskReservationPaymentTimeout(context.Context, *PayloadReservationPaymentTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskOrderPaymentTimeout(context.Context, *PayloadOrderPaymentTimeout, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskReservationNoShowAlert(context.Context, *PayloadReservationNoShowAlert, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskReservationFoodSafetyAlert(context.Context, *PayloadReservationFoodSafetyAlert, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessPaymentSuccess(context.Context, *PaymentSuccessPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessRefund(context.Context, *PayloadProcessRefund, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessRefundResult(context.Context, *RefundResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessProfitSharing(context.Context, *ProfitSharingPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessApplymentResult(context.Context, *ApplymentResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessProfitSharingResult(context.Context, *ProfitSharingResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessProfitSharingReturnResult(context.Context, *ProfitSharingReturnResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessMerchantWithdrawResult(context.Context, *MerchantWithdrawResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessMerchantCancelWithdrawResult(context.Context, *MerchantCancelWithdrawResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskCheckMerchantForeignObject(context.Context, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskCheckRiderDamage(context.Context, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessAppealResult(context.Context, *ProcessAppealResultPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskAutomaticAppealResolution(context.Context, *AutomaticAppealResolutionPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskClaimPayout(context.Context, *ClaimPayoutPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskMerchantApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskOperatorApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskOperatorApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskRiderApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskRiderApplicationHealthCertOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskGroupApplicationBusinessLicenseOCR(context.Context, int64, int64, int64, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskGroupApplicationIDCardOCR(context.Context, int64, int64, int64, string, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskUploadShippingInfo(context.Context, *UploadShippingInfoPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskSyncComplaints(context.Context, *SyncComplaintsPayload, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskProcessAnomalyRefund(context.Context, *PayloadProcessAnomalyRefund, ...asynq.Option) error {
	return nil
}
func (d *automaticAppealResolutionTestDistributor) DistributeTaskPrintOrder(context.Context, *PrintOrderPayload, ...asynq.Option) error {
	return nil
}

func TestProcessTaskAutomaticAppealResolution_ResolvesPendingAppeal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &automaticAppealResolutionTestDistributor{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)

	appeal := db.Appeal{
		ID:            31,
		ClaimID:       81,
		AppellantType: "rider",
		AppellantID:   91,
		Reason:        "天气原因导致延迟",
		Status:        "pending",
		RegionID:      101,
		CreatedAt:     time.Now(),
	}
	claim := db.GetClaimForAppealRow{
		ID:          81,
		OrderID:     701,
		ClaimType:   "delay",
		ClaimAmount: 300,
		Status:      "approved",
		MerchantID:  55,
		RegionID:    101,
		RiderID:     pgtype.Int8{Int64: 91, Valid: true},
		CreatedAt:   time.Now(),
	}
	decision := db.BehaviorDecision{
		ID:                 501,
		ClaimID:            pgtype.Int8{Int64: appeal.ClaimID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	resolvedAppeal := appeal
	resolvedAppeal.Status = "rejected"
	resolvedAppeal.ReviewNotes = pgtype.Text{String: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", Valid: true}
	resolvedAppeal.ReviewedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	postProcess := db.GetAppealForPostProcessRow{
		AppealID:       appeal.ID,
		ClaimID:        appeal.ClaimID,
		AppellantType:  appeal.AppellantType,
		AppellantID:    appeal.AppellantID,
		ClaimantUserID: 201,
		ClaimType:      claim.ClaimType,
		ClaimAmount:    claim.ClaimAmount,
		OrderID:        claim.OrderID,
		OrderNo:        "20240101120000123456",
		MerchantID:     claim.MerchantID,
		RiderID:        pgtype.Int8{Int64: appeal.AppellantID, Valid: true},
	}

	payloadBytes, err := json.Marshal(&AutomaticAppealResolutionPayload{AppealID: appeal.ID})
	require.NoError(t, err)
	task := asynq.NewTask(TaskAutomaticAppealResolution, payloadBytes)

	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).Return(appeal, nil)
	store.EXPECT().GetClaimForAppeal(gomock.Any(), appeal.ClaimID).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().ReviewAppealWithCompensationTx(gomock.Any(), db.ReviewAppealWithCompensationTxParams{
		ID:                 appeal.ID,
		Status:             "rejected",
		ReviewerID:         pgtype.Int8{},
		ReviewNotes:        pgtype.Text{String: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", Valid: true},
		CompensationAmount: pgtype.Int8{},
	}).Return(db.ReviewAppealWithCompensationTxResult{
		Appeal:      resolvedAppeal,
		PostProcess: postProcess,
	}, nil)
	store.EXPECT().ListBehaviorAppealsByEntity(gomock.Any(), db.ListBehaviorAppealsByEntityParams{EntityType: appeal.AppellantType, EntityID: appeal.AppellantID}).Return(nil, nil)
	store.EXPECT().CreateBehaviorAppeal(gomock.Any(), gomock.Any()).Return(db.BehaviorAppeal{ID: 99}, nil)
	store.EXPECT().CreateAuditLog(gomock.Any(), gomock.Any()).Return(db.AuditLog{ID: 1}, nil)
	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), appeal.ClaimID).Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
	store.EXPECT().GetRider(gomock.Any(), appeal.AppellantID).Return(db.Rider{ID: appeal.AppellantID, UserID: 301}, nil)

	err = processor.ProcessTaskAutomaticAppealResolution(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 2, distributor.sendNotificationCalls)
}

func TestProcessTaskAutomaticAppealResolution_ReplaysPostProcessForResolvedAppeal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &automaticAppealResolutionTestDistributor{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)

	appeal := db.Appeal{
		ID:            41,
		ClaimID:       82,
		AppellantType: "merchant",
		AppellantID:   92,
		Reason:        "顾客签收时已核对",
		Status:        "approved",
		RegionID:      102,
		CreatedAt:     time.Now(),
	}
	claim := db.GetClaimForAppealRow{
		ID:          82,
		OrderID:     702,
		ClaimType:   "missing-item",
		ClaimAmount: 500,
		Status:      "approved",
		MerchantID:  92,
		RegionID:    102,
		CreatedAt:   time.Now(),
	}
	decision := db.BehaviorDecision{
		ID:                 502,
		ClaimID:            pgtype.Int8{Int64: appeal.ClaimID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	postProcess := db.GetAppealForPostProcessRow{
		AppealID:       appeal.ID,
		ClaimID:        appeal.ClaimID,
		AppellantType:  appeal.AppellantType,
		AppellantID:    appeal.AppellantID,
		ClaimantUserID: 202,
		ClaimType:      claim.ClaimType,
		ClaimAmount:    claim.ClaimAmount,
		OrderID:        claim.OrderID,
		OrderNo:        "20240101120000999999",
		MerchantID:     claim.MerchantID,
	}

	payloadBytes, err := json.Marshal(&AutomaticAppealResolutionPayload{AppealID: appeal.ID})
	require.NoError(t, err)
	task := asynq.NewTask(TaskAutomaticAppealResolution, payloadBytes)

	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).Return(appeal, nil)
	store.EXPECT().GetAppealForPostProcess(gomock.Any(), appeal.ID).Return(postProcess, nil)
	store.EXPECT().GetClaimForAppeal(gomock.Any(), appeal.ClaimID).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().ListBehaviorAppealsByEntity(gomock.Any(), db.ListBehaviorAppealsByEntityParams{EntityType: appeal.AppellantType, EntityID: appeal.AppellantID}).Return([]db.BehaviorAppeal{{
		ID:       88,
		Evidence: pgtype.Text{String: "appeal_id=41,claim_id=82", Valid: true},
	}}, nil)
	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), appeal.ClaimID).Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: postProcess.ClaimantUserID}).Return(db.BehaviorBlocklist{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), appeal.AppellantID).Return(db.Merchant{ID: appeal.AppellantID, OwnerUserID: 302}, nil)

	err = processor.ProcessTaskAutomaticAppealResolution(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 2, distributor.sendNotificationCalls)
}
