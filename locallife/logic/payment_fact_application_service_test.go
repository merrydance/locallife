package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingSuccessFinishesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(701, 601, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(601, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.False(t, result.Skipped)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuProfitSharingSuccessFinishesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 3, 12, 10, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(2701, 2601, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(2601, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Provider = db.ExternalPaymentProviderBaofu
	fact.Channel = db.PaymentChannelBaofuAggregate
	fact.Capability = db.ExternalPaymentCapabilityBaofuProfitSharing
	fact.ExternalObjectType = db.ExternalPaymentObjectProfitSharing
	fact.ExternalObjectKey = "BFSHARE202605030001"
	fact.ExternalSecondaryKey = pgtype.Text{String: "BFSHAREUP202605030001", Valid: true}
	fact.UpstreamState = "SUCCESS"
	fact.RawResource = []byte(`{"txnState":"SUCCESS","succAmt":10000}`)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingFailedFactDoesNotRegressFinishedOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(2702, 2602, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(2602, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.Provider = db.ExternalPaymentProviderBaofu
	fact.Channel = db.PaymentChannelBaofuAggregate
	fact.Capability = db.ExternalPaymentCapabilityBaofuProfitSharing
	fact.RawResource = []byte(`{"fail_reason":"STALE_CALLBACK"}`)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingFailedFactTreatsFinishedUpdateConflictAsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 10, 1, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(2703, 2603, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(2603, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.Provider = db.ExternalPaymentProviderBaofu
	fact.Channel = db.PaymentChannelBaofuAggregate
	fact.Capability = db.ExternalPaymentCapabilityBaofuProfitSharing
	fact.RawResource = []byte(`{"fail_reason":"STALE_CALLBACK"}`)
	processing := buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing)
	finished := buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(processing, nil)
	store.EXPECT().UpdateProfitSharingOrderToFailed(gomock.Any(), application.BusinessObjectID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(finished, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingSuccessRejectsPendingOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 5, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(702, 602, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(602, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(db.ProfitSharingOrder{ID: application.BusinessObjectID, Status: "pending"}, nil)
	expectApplicationFailed(t, store, application, now, "cannot apply success fact")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot apply success fact")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingFailedAlreadyFailedIsApplied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 10, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(703, 603, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(603, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.RawResource = []byte(`{"fail_reason":"NO_RELATION"}`)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFailed), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "FAILED", "NO_RELATION")
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingReturnFailedFailsRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 10, 45, 0, time.UTC)
	application := buildProfitSharingReturnFactApplication(705, 605, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingReturnFact(605, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed, "FAILED", "")
	fact.RawResource = []byte(`{"fail_reason":"PAYER_ACCOUNT_ABNORMAL"}`)
	returnRecord := db.ProfitSharingReturn{ID: application.BusinessObjectID, RefundOrderID: 4102, PaymentOrderID: 5102, OutReturnNo: "PR3002", Status: "processing"}
	failedReturn := returnRecord
	failedReturn.Status = "failed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingReturn(gomock.Any(), application.BusinessObjectID).Return(returnRecord, nil)
	store.EXPECT().UpdateProfitSharingReturnToFailed(gomock.Any(), db.UpdateProfitSharingReturnToFailedParams{
		ID:         returnRecord.ID,
		FailReason: pgtype.Text{String: "PAYER_ACCOUNT_ABNORMAL", Valid: true},
	}).Return(failedReturn, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), returnRecord.RefundOrderID).Return(db.RefundOrder{ID: returnRecord.RefundOrderID, Status: "failed"}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositPaymentSuccessDoesNotWriteReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 0, 0, time.UTC)
	application := buildRiderDepositPaymentFactApplication(801, 701, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositPaymentFact(701, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, UserID: 77, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositPaymentRetryDoesNotWriteReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 45, 0, time.UTC)
	application := buildRiderDepositPaymentFactApplication(8021, 7021, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositPaymentFact(7021, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{
		ID:           application.BusinessObjectID,
		UserID:       79,
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
		ProcessedAt:  pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
	}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    false,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ClaimRecoveryPaymentSuccessMarksRecoveryPaidWithoutOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 803,
		FactID:             703,
		Consumer:           paymentFactConsumerClaimRecoveryDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   903,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 703,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelDirect,
		Capability:         db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		ExternalObjectType: db.ExternalPaymentObjectPayment,
		ExternalObjectKey:  "CR_PAY_903",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerClaimRecovery, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
	}
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, UserID: 88, BusinessType: db.ExternalPaymentBusinessOwnerClaimRecovery}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.ClaimRecoveryPayment)
	require.Equal(t, paymentOrder.ID, result.ClaimRecoveryPayment.PaymentOrder.ID)
	require.True(t, result.ClaimRecoveryPayment.Processed)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeSuccessContinuesOpening(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1803, 1703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "paid",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1001;purpose:initial_open", Valid: true},
	}
	processedPaymentOrder := paymentOrder
	processedPaymentOrder.ProcessedAt = pgtype.Timestamptz{Time: now, Valid: true}
	continuation := &testBaofuVerifyFeeContinuation{}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderProcessedAt(gomock.Any(), application.BusinessObjectID).Return(processedPaymentOrder, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, processedPaymentOrder.ID, result.BaofuVerifyFeePayment.PaymentOrder.ID)
	require.True(t, continuation.called)
	require.Equal(t, processedPaymentOrder.ID, continuation.paymentOrder.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeRejectsAmountMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 32, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1807, 1707, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1707, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	fact.Amount = pgtype.Int8{Int64: 999, Valid: true}
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "paid",
		Amount:         200,
	}
	continuation := &testBaofuVerifyFeeContinuation{}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	expectApplicationFailed(t, store, application, now, "amount")

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "amount")
	require.False(t, result.Applied)
	require.Nil(t, result.BaofuVerifyFeePayment)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeClosedReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 35, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1804, 1704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1001;purpose:initial_open", Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7101,
		OwnerType:               db.BaofuAccountOwnerTypeRider,
		OwnerID:                 1001,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7001, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}
	pendingFlow := flow
	pendingFlow.State = db.BaofuAccountOpeningStateVerifyFeePending

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.Equal(t, flow.ProfileID, arg.ProfileID)
			require.Equal(t, flow.VerifyFeeAmount, arg.VerifyFeeAmount)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"state":"verify_fee_pending"`)
			require.Contains(t, string(arg.RawSnapshot), `"payment_status":"closed"`)
			return pendingFlow, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "closed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, result.BaofuVerifyFeePayment.Processed)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeFailedReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 36, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1805, 1705, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1705, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.UpstreamState = "PAYERROR"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:operator;owner_id:1002;purpose:initial_open", Valid: true},
	}
	failedPaymentOrder := paymentOrder
	failedPaymentOrder.Status = "failed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7102,
		OwnerType:               db.BaofuAccountOwnerTypeOperator,
		OwnerID:                 1002,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7002, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), paymentOrder.ID).Return(failedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"payment_status":"failed"`)
			return db.BaofuAccountOpeningFlow{ID: flow.ID, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, State: db.BaofuAccountOpeningStateVerifyFeePending}, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "failed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFeeExpiredReturnsFlowToRetryablePending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 8, 10, 37, 0, 0, time.UTC)
	application := buildBaofuVerifyFeePaymentFactApplication(1806, 1706, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuVerifyFeePaymentFact(1706, application.BusinessObjectID, db.ExternalPaymentTerminalStatusExpired)
	fact.UpstreamState = "EXPIRED"
	paymentOrder := db.PaymentOrder{
		ID:             application.BusinessObjectID,
		UserID:         88,
		BusinessType:   db.PaymentBusinessTypeBaofuAccountVerifyFee,
		PaymentChannel: db.PaymentChannelDirect,
		Status:         "pending",
		Attach:         pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:1003;purpose:initial_open", Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	flow := db.BaofuAccountOpeningFlow{
		ID:                      7103,
		OwnerType:               db.BaofuAccountOwnerTypeRider,
		OwnerID:                 1003,
		AccountType:             db.BaofuAccountTypePersonal,
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		ProfileID:               pgtype.Int8{Int64: 7003, Valid: true},
		VerifyFeeAmount:         200,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
	}

	continuation := &testBaofuVerifyFeeContinuation{}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil)
	store.EXPECT().GetBaofuAccountOpeningFlowByPaymentOrder(gomock.Any(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true}).Return(flow, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.AssignableToTypeOf(db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.False(t, arg.VerifyFeePaymentOrderID.Valid)
			require.Contains(t, string(arg.RawSnapshot), `"payment_terminal_status":"expired"`)
			require.Contains(t, string(arg.RawSnapshot), "支付未完成，请重新支付开户核验费。")
			return db.BaofuAccountOpeningFlow{ID: flow.ID, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, State: db.BaofuAccountOpeningStateVerifyFeePending}, nil
		})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithBaofuVerifyFeeContinuation(continuation)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.BaofuVerifyFeePayment)
	require.Equal(t, "closed", result.BaofuVerifyFeePayment.PaymentOrder.Status)
	require.False(t, result.BaofuVerifyFeePayment.Processed)
	require.False(t, continuation.called)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderPaymentSuccessProcessesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 40, 0, time.UTC)
	application := buildOrderPaymentFactApplication(803, 703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildOrderPaymentFact(703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6201, MerchantID: 7101, OrderNo: "ORD6201"}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6001", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8201, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, result.Outbox.EventType)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.OrderPayment.OrderResult)
	require.Equal(t, orderResult.Order.ID, result.OrderPayment.OrderResult.Order.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentSuccessMarksPaidAndProcessesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(823, 723, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuOrderPaymentFact(723, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, RequiresProfitSharing: true, Amount: 12900, Status: paymentStatusPaid}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6401, MerchantID: 7301, OrderNo: "ORD6401", OrderType: db.OrderTypeTakeout, DeliveryFee: 500}}
	merchant := db.Merchant{ID: orderResult.Order.MerchantID, RegionID: 8301}
	operator := db.Operator{ID: 9301}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6401", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	gomock.InOrder(
		store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil),
		store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
			OrderSource: db.OrderTypeTakeout,
			MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
			RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
		}).Return(db.ProfitSharingConfig{PlatformRate: 200, OperatorRate: 300, RiderEnabled: true}, nil),
		store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil),
		store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypeMerchant, OwnerID: merchant.ID}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID, "MER_CONTRACT", "MER_SHARE"), nil),
		store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypeOperator, OwnerID: operator.ID}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeOperator, operator.ID, "OP_CONTRACT", "OP_SHARE"), nil),
		store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypePlatform, OwnerID: int64(0)}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"), nil),
		store.EXPECT().EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, paymentOrder.ID, arg.ProfitSharingOrder.PaymentOrderID)
			require.Equal(t, merchant.ID, arg.ProfitSharingOrder.MerchantID)
			require.False(t, arg.ProfitSharingOrder.RiderID.Valid)
			require.Equal(t, "BFPS6001O6401", arg.ProfitSharingOrder.OutOrderNo)
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.ProfitSharingOrder.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.ProfitSharingOrder.Channel)
			require.Equal(t, db.ProfitSharingOrderStatusPending, arg.ProfitSharingOrder.Status)
			require.Equal(t, int64(0), arg.FeeBreakdown.RiderGrossAmount)
			return db.CreateBaofuProfitSharingOrderTxResult{ProfitSharingOrder: db.ProfitSharingOrder{ID: 9901, PaymentOrderID: paymentOrder.ID}}, nil
		}),
		store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
			require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
			require.Equal(t, paymentOrder.ID, arg.AggregateID)
			return db.PaymentDomainOutbox{ID: 8401, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
		}),
	)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentBillFailureBlocksOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 3, 11, 10, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(826, 726, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuOrderPaymentFact(726, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, RequiresProfitSharing: true, Amount: 12900, Status: paymentStatusPaid}
	orderResult := db.ProcessOrderPaymentTxResult{Order: db.Order{ID: 6402, MerchantID: 7302, OrderNo: "ORD6402", OrderType: db.OrderTypeTakeout, DeliveryFee: 500}}
	merchant := db.Merchant{ID: orderResult.Order.MerchantID, RegionID: 8302}
	operator := db.Operator{ID: 9302}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6401", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).Return(db.ProfitSharingConfig{PlatformRate: 200, OperatorRate: 300, RiderEnabled: true}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypeMerchant, OwnerID: merchant.ID}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID, "MER_CONTRACT", "MER_SHARE"), nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypeOperator, OwnerID: operator.ID}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeOperator, operator.ID, "OP_CONTRACT", "OP_SHARE"), nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypePlatform, OwnerID: int64(0)}).Return(activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"), nil)
	store.EXPECT().EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).Return(db.CreateBaofuProfitSharingOrderTxResult{}, db.ErrRecordNotFound)
	expectApplicationFailed(t, store, application, now, "ensure baofu profit sharing bill")

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ensure baofu profit sharing bill")
	require.False(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentClosedMarksPaymentClosedWithoutSuccessOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(824, 724, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuOrderPaymentFact(724, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	pendingPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: "pending"}
	closedPayment := pendingPayment
	closedPayment.Status = "closed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), pendingPayment.ID).Return(closedPayment, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.OrderPayment)
	require.False(t, result.OrderPayment.Processed)
	require.Equal(t, "closed", result.OrderPayment.PaymentOrder.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentFailedMarksPaymentFailedWithoutSuccessOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 11, 5, 0, 0, time.UTC)
	application := buildOrderPaymentFactApplication(825, 725, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildBaofuOrderPaymentFact(725, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.UpstreamState = "PAY_ERROR"
	pendingPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: "pending"}
	failedPayment := pendingPayment
	failedPayment.Status = "failed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), pendingPayment.ID).Return(failedPayment, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.OrderPayment)
	require.False(t, result.OrderPayment.Processed)
	require.Equal(t, "failed", result.OrderPayment.PaymentOrder.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderPaymentOutboxRetryAfterProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 50, 0, time.UTC)
	application := buildOrderPaymentFactApplication(805, 705, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildOrderPaymentFact(705, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	processedAt := pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, OrderID: pgtype.Int8{Int64: 6202, Valid: true}, ProcessedAt: processedAt}
	order := db.Order{ID: paymentOrder.OrderID.Int64, MerchantID: 7102, OrderNo: "ORD6202"}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6001", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	}).Return(db.ProcessPaymentSuccessTxResult{Processed: false, PaymentOrder: paymentOrder}, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8203, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store).WithPaymentSuccessConfig(15000, 20)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.NotNil(t, result.OrderPayment)
	require.True(t, result.OrderPayment.Processed)
	require.NotNil(t, result.OrderPayment.OrderResult)
	require.Equal(t, order.ID, result.OrderPayment.OrderResult.Order.ID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationPaymentSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(804, 704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: paymentStatusPaid}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6201", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8202, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, result.Outbox.EventType)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, int64(6201), result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentSuccessMarksPaidBeforeProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(834, 734, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(734, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: paymentStatusPaid}

	claimCall := store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	getFactCall := store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	markPaidCall := store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6201", Valid: true},
	}).Return(paymentOrder, nil)
	processCall := store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	gomock.InOrder(claimCall, getFactCall, markPaidCall, processCall)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8304, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, result.Outbox.EventType)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, paymentStatusPaid, result.ReservationPayment.PaymentOrder.Status)
	require.Equal(t, int64(6201), result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentSuccessAlreadyPaidStillProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 5, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(835, 735, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(735, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: paymentStatusPaid}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6201", Valid: true},
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetPaymentOrder(gomock.Any(), application.BusinessObjectID).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		Processed:    true,
		PaymentOrder: paymentOrder,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8305, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, paymentStatusPaid, result.ReservationPayment.PaymentOrder.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentClosedMarksPaymentClosedWithoutSuccessOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 10, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(836, 736, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(736, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	pendingPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: "pending"}
	closedPayment := pendingPayment
	closedPayment.Status = "closed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), pendingPayment.ID).Return(closedPayment, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.ReservationPayment)
	require.False(t, result.ReservationPayment.Processed)
	require.Equal(t, "closed", result.ReservationPayment.PaymentOrder.Status)
	require.Equal(t, int64(6201), result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentClosedAlreadyClosedIsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 12, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(838, 738, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(738, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	closedPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: "closed"}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), closedPayment.ID).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetPaymentOrder(gomock.Any(), closedPayment.ID).Return(closedPayment, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.ReservationPayment)
	require.False(t, result.ReservationPayment.Processed)
	require.Equal(t, "closed", result.ReservationPayment.PaymentOrder.Status)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentClosedConflictingPaidFailsApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 13, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(839, 739, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(739, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed)
	fact.UpstreamState = "CLOSED"
	paidPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: paymentStatusPaid}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paidPayment.ID).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paidPayment.ID).Return(paidPayment, nil)
	expectApplicationFailed(t, store, application, now, "status=paid")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status=paid")
	require.False(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentFailedMarksPaymentFailedWithoutSuccessOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 23, 10, 15, 0, 0, time.UTC)
	application := buildReservationPaymentFactApplication(837, 737, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(737, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed)
	fact.UpstreamState = "PAY_ERROR"
	pendingPayment := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6201, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, Status: "pending"}
	failedPayment := pendingPayment
	failedPayment.Status = "failed"

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), pendingPayment.ID).Return(failedPayment, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
	require.NotNil(t, result.ReservationPayment)
	require.False(t, result.ReservationPayment.Processed)
	require.Equal(t, "failed", result.ReservationPayment.PaymentOrder.Status)
	require.Equal(t, int64(6201), result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationPaymentOutboxRetryAfterProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 12, 0, 30, 0, time.UTC)
	application := buildReservationPaymentFactApplication(806, 706, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildReservationPaymentFact(706, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)
	paymentOrder := db.PaymentOrder{ID: application.BusinessObjectID, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 6203, Valid: true}, PaymentChannel: db.PaymentChannelBaofuAggregate, ProcessedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: "BFPAY_6201", Valid: true},
	}).Return(paymentOrder, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{Processed: false, PaymentOrder: paymentOrder}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 8204, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.NotNil(t, result.ReservationPayment)
	require.True(t, result.ReservationPayment.Processed)
	require.Equal(t, paymentOrder.ReservationID.Int64, result.ReservationPayment.ReservationID)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundSuccessUpdatesPrepaidAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 11, 45, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 811,
		FactID:             711,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4101,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 711,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		ExternalObjectKey:  "RFD4101",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: 4101, Valid: true},
		UpstreamState:      riderDepositRefundStatusSuccess,
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
	}
	refundOrder := db.RefundOrder{ID: 4101, PaymentOrderID: 5101, RefundAmount: 400, OutRefundNo: "RFD4101", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5101, ReservationID: pgtype.Int8{Int64: 6101, Valid: true}, Amount: 400, BusinessType: reservationAddonBusiness}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(400), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().AddReservationPrepaidAmount(gomock.Any(), db.AddReservationPrepaidAmountParams{ID: 6101, PrepaidAmount: -400}).Return(db.TableReservation{ID: 6101}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundClosedUpdatesRefundOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 15, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 812,
		FactID:             712,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4102,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                 712,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: 4102, Valid: true},
		UpstreamState:      riderDepositRefundStatusClosed,
		TerminalStatus:     db.ExternalPaymentTerminalStatusClosed,
		IsTerminal:         true,
	}
	refundOrder := db.RefundOrder{ID: 4102, PaymentOrderID: 5102, RefundAmount: 280, OutRefundNo: "RFD4102", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5102, ReservationID: pgtype.Int8{Int64: 6102, Valid: true}, Amount: 400, BusinessType: reservationAddonBusiness}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToClosed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusClosed}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 30, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 813,
		FactID:             713,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4103,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   713,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_4103", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4103, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4103, PaymentOrderID: 5103, RefundAmount: 280, OutRefundNo: "RFD4103", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5103, ReservationID: pgtype.Int8{Int64: 6103, Valid: true}, Amount: 400, BusinessType: db.ExternalPaymentBusinessOwnerReservation}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusFailed}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8102, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_ReservationRefundFailedFactDoesNotRegressSuccessfulRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 10, 3, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 1813,
		FactID:             1713,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4104,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   1713,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_4104", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4104, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4104, PaymentOrderID: 5104, RefundAmount: 280, OutRefundNo: "BFRFD4104", Status: refundOrderStatusSuccess}
	paymentOrder := db.PaymentOrder{ID: 5104, ReservationID: pgtype.Int8{Int64: 6104, Valid: true}, Amount: 400, BusinessType: db.ExternalPaymentBusinessOwnerReservation, PaymentChannel: db.PaymentChannelBaofuAggregate}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 45, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 814,
		FactID:             714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   714,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_14201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4201, Valid: true},
		UpstreamState:        riderDepositRefundStatusSuccess,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4201, PaymentOrderID: 5201, RefundAmount: 500, OutRefundNo: "RFD4201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5201, OrderID: pgtype.Int8{Int64: 6201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundSuccessDoesNotMarkPartialRefundedPaymentAsRefunded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2816,
		FactID:             2716,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24206,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2716,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24206", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24206, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24206, PaymentOrderID: 25206, RefundAmount: 500, OutRefundNo: "BFRFD24206", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 25206, OrderID: pgtype.Int8{Int64: 26206, Valid: true}, Amount: 1000, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: refundOrderStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		return db.PaymentDomainOutbox{ID: 28106, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderBaofuRefundSuccessResultCodeCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 50, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 1814,
		FactID:             1714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   14201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   1714,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_4201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 14201, Valid: true},
		UpstreamState:        riderDepositRefundStatusSuccess,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 14201, PaymentOrderID: 15201, RefundAmount: 500, OutRefundNo: "RFD14201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 15201, OrderID: pgtype.Int8{Int64: 16201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 177}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 18103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderBaofuRefundSuccessUsesBaofuChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2814,
		FactID:             2714,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24201,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2714,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24201, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24201, PaymentOrderID: 25201, RefundAmount: 500, OutRefundNo: "BFRFD24201", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 25201, OrderID: pgtype.Int8{Int64: 26201, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 28103, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderBaofuRefundResultCodeSuccessCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 4, 12, 11, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2817,
		FactID:             2717,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24207,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2717,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24207", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24207, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24207, PaymentOrderID: 25207, RefundAmount: 500, OutRefundNo: "BFRFD24207", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 25207, OrderID: pgtype.Int8{Int64: 26207, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusSuccess}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 28107, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundFailedFactDoesNotRegressSuccessfulRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 10, 5, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2815,
		FactID:             2715,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24202,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2715,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24202", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24202, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24202, PaymentOrderID: 25202, RefundAmount: 500, OutRefundNo: "BFRFD24202", Status: refundOrderStatusSuccess}
	paymentOrder := db.PaymentOrder{ID: 25202, OrderID: pgtype.Int8{Int64: 26202, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundFailedFactTreatsSuccessUpdateConflictAsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 5, 5, 10, 6, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 2816,
		FactID:             2716,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   24203,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   2716,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_24203", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 24203, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 24203, PaymentOrderID: 25203, RefundAmount: 500, OutRefundNo: "BFRFD24203", Status: "processing"}
	successRefundOrder := refundOrder
	successRefundOrder.Status = refundOrderStatusSuccess
	paymentOrder := db.PaymentOrder{ID: 25203, OrderID: pgtype.Int8{Int64: 26203, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, PaymentChannel: db.PaymentChannelBaofuAggregate, UserID: 77}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(successRefundOrder, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 13, 0, 0, time.UTC)
	application := db.ExternalPaymentFactApplication{
		ID:                 815,
		FactID:             715,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4202,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	fact := db.ExternalPaymentFact{
		ID:                   715,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalSecondaryKey: pgtype.Text{String: "BF_REFUND_4202", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 4202, Valid: true},
		UpstreamState:        riderDepositRefundStatusAbnormal,
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
	}
	refundOrder := db.RefundOrder{ID: 4202, PaymentOrderID: 5202, RefundAmount: 500, OutRefundNo: "RFD4202", Status: "processing"}
	paymentOrder := db.PaymentOrder{ID: 5202, OrderID: pgtype.Int8{Int64: 6202, Valid: true}, Amount: 500, BusinessType: db.ExternalPaymentBusinessOwnerOrder, UserID: 78}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, OutRefundNo: refundOrder.OutRefundNo, Status: riderDepositRefundStatusFailed}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)
		return db.PaymentDomainOutbox{ID: 8104, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositClosedResolvesRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 12, 0, 0, time.UTC)
	application := buildRiderDepositRefundFactApplication(803, 703, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositRefundFact(703, application.BusinessObjectID, db.ExternalPaymentTerminalStatusClosed, riderDepositRefundStatusClosed)
	refundOrder := db.RefundOrder{ID: application.BusinessObjectID, PaymentOrderID: 5101, OutRefundNo: "RFD3001"}
	paymentOrder := db.PaymentOrder{ID: 5101, UserID: 811, Amount: 30000, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}
	fact.ExternalObjectKey = refundOrder.OutRefundNo
	fact.ExternalSecondaryKey = pgtype.Text{String: "WX_REFUND_RFD3001", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
		RefundOrderID: application.BusinessObjectID,
		RefundStatus:  riderDepositRefundStatusClosed,
		RefundID:      "WX_REFUND_RFD3001",
	}).Return(db.ResolveRiderDepositRefundTxResult{}, nil)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Nil(t, result.Outbox)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RiderDepositAbnormalCreatesOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 13, 0, 0, time.UTC)
	application := buildRiderDepositRefundFactApplication(804, 704, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildRiderDepositRefundFact(704, application.BusinessObjectID, db.ExternalPaymentTerminalStatusFailed, riderDepositRefundStatusAbnormal)
	refundOrder := db.RefundOrder{ID: application.BusinessObjectID, PaymentOrderID: 5102, OutRefundNo: "RFD3002"}
	paymentOrder := db.PaymentOrder{ID: 5102, UserID: 812, Amount: 30000, BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit}
	fact.ExternalObjectKey = refundOrder.OutRefundNo
	fact.ExternalSecondaryKey = pgtype.Text{String: "WX_REFUND_RFD3002", Valid: true}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), application.BusinessObjectID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
		RefundOrderID: application.BusinessObjectID,
		RefundStatus:  riderDepositRefundStatusAbnormal,
		RefundID:      "WX_REFUND_RFD3002",
	}).Return(db.ResolveRiderDepositRefundTxResult{}, nil)
	expectRiderDepositRefundAbnormalOutbox(t, store, application, fact, refundOrder, paymentOrder)
	expectFactTerminalized(t, store, fact.ID, now)
	expectApplicationApplied(t, store, application, now)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Outbox)
	require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, result.Outbox.EventType)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_MarkAppliedFailureSchedulesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 15, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(705, 605, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(605, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	expectProfitSharingResultOutbox(t, store, application, fact, "SUCCESS", "")
	expectFactTerminalized(t, store, fact.ID, now)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)
	expectApplicationFailed(t, store, application, now, "mark external payment fact application applied")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mark external payment fact application applied")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_OutboxFailureSchedulesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 17, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(707, 607, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(607, application.BusinessObjectID, db.ExternalPaymentTerminalStatusSuccess)

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().GetProfitSharingOrder(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusProcessing), nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), application.BusinessObjectID).Return(buildProfitSharingOrderForApplication(application, db.ProfitSharingOrderStatusFinished), nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).Return(db.PaymentDomainOutbox{}, db.ErrRecordNotFound)
	expectApplicationFailed(t, store, application, now, "create profit sharing result outbox")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create profit sharing result outbox")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_RejectsNonTerminalFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 26, 11, 20, 0, 0, time.UTC)
	application := buildProfitSharingFactApplication(706, 606, db.ExternalPaymentFactApplicationStatusProcessing)
	fact := buildProfitSharingFact(606, application.BusinessObjectID, db.ExternalPaymentTerminalStatusProcessing)
	fact.IsTerminal = false

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	expectApplicationFailed(t, store, application, now, "is not terminal")

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), application.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not terminal")
	require.False(t, result.Applied)
}

func TestPaymentFactServiceApplyExternalPaymentFactApplication_SkipsUnclaimableApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(704)).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)

	svc := NewPaymentFactService(store)
	result, err := svc.ApplyExternalPaymentFactApplication(context.Background(), 704)
	require.NoError(t, err)
	require.True(t, result.Skipped)
}

func buildProfitSharingFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   3001,
		Status:             status,
	}
}

func buildProfitSharingReturnFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingReturn,
		BusinessObjectID:   3002,
		Status:             status,
	}
}

func buildRiderDepositRefundFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   4001,
		Status:             status,
	}
}

func buildRiderDepositPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   5001,
		Status:             status,
	}
}

func buildBaofuVerifyFeePaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerBaofuAccountVerifyFeeDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   5101,
		Status:             status,
	}
}

func buildOrderPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   6001,
		Status:             status,
	}
}

func buildReservationPaymentFactApplication(applicationID, factID int64, status string) db.ExternalPaymentFactApplication {
	return db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   6201,
		Status:             status,
	}
}

func buildProfitSharingFact(factID, profitSharingOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                 factID,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  "PS3001",
		BusinessObjectType: pgtype.Text{String: paymentFactBusinessObjectProfitSharingOrder, Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: profitSharingOrderID, Valid: true},
		TerminalStatus:     terminalStatus,
		IsTerminal:         true,
		RawResource:        []byte(`{}`),
	}
}

func buildProfitSharingReturnFact(factID, profitSharingReturnID int64, terminalStatus string, upstreamState string, returnID string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuProfitSharing,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    "PR3001",
		ExternalSecondaryKey: pgtype.Text{String: returnID, Valid: returnID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectProfitSharingReturn, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: profitSharingReturnID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildRiderDepositRefundFact(factID, refundOrderID int64, terminalStatus, upstreamState string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    "RFD3001",
		ExternalSecondaryKey: pgtype.Text{String: "WX_REFUND_RFD3001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectRefundOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: refundOrderID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildRiderDepositPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "RD5001",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_5001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildBaofuVerifyFeePaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    "BFVF5101",
		ExternalSecondaryKey: pgtype.Text{String: "WX_PAYMENT_5101", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerBaofuVerifyFee, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildOrderPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    "PO6001",
		ExternalSecondaryKey: pgtype.Text{String: "BFPAY_6001", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildBaofuOrderPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    "PO6401",
		ExternalSecondaryKey: pgtype.Text{String: "BFPAY_6401", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{"tradeNo":"BFPAY_6401","txnState":"SUCCESS"}`),
	}
}

func buildReservationPaymentFact(factID, paymentOrderID int64, terminalStatus string) db.ExternalPaymentFact {
	return db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    "RES6201",
		ExternalSecondaryKey: pgtype.Text{String: "BFPAY_6201", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrderID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}
}

func buildProfitSharingOrderForApplication(application db.ExternalPaymentFactApplication, status string) db.ProfitSharingOrder {
	return db.ProfitSharingOrder{
		ID:                 application.BusinessObjectID,
		PaymentOrderID:     9001,
		MerchantID:         801,
		OutOrderNo:         "PS3001",
		Status:             status,
		MerchantAmount:     1200,
		PlatformCommission: 100,
		OperatorCommission: 60,
	}
}

func expectProfitSharingResultOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, result string, failReason string) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateProfitSharingOrder, arg.AggregateType)
		require.Equal(t, application.BusinessObjectID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(application.BusinessObjectID), payload["profit_sharing_order_id"])
		require.Equal(t, "PS3001", payload["out_order_no"])
		require.Equal(t, result, payload["result"])
		if failReason == "" {
			require.NotContains(t, payload, "fail_reason")
		} else {
			require.Equal(t, failReason, payload["fail_reason"])
		}
		require.Equal(t, float64(801), payload["merchant_id"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{
			ID:            8001,
			EventType:     arg.EventType,
			AggregateType: arg.AggregateType,
			AggregateID:   arg.AggregateID,
			Payload:       arg.Payload,
			Status:        arg.Status,
		}, nil
	})
}

func expectRiderDepositRefundAbnormalOutbox(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder) {
	t.Helper()
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregateRefundOrder, arg.AggregateType)
		require.Equal(t, refundOrder.ID, arg.AggregateID)
		require.Equal(t, db.PaymentDomainOutboxStatusPending, arg.Status)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.Payload, &payload))
		require.Equal(t, float64(refundOrder.ID), payload["refund_order_id"])
		require.Equal(t, float64(paymentOrder.ID), payload["payment_order_id"])
		require.Equal(t, refundOrder.OutRefundNo, payload["out_refund_no"])
		require.Equal(t, riderDepositRefundStatusAbnormal, payload["refund_status"])
		require.Equal(t, "WX_REFUND_RFD3002", payload["refund_id"])
		require.Equal(t, float64(fact.ID), payload["external_payment_fact_id"])
		require.Equal(t, float64(application.ID), payload["payment_fact_application_id"])

		return db.PaymentDomainOutbox{
			ID:            8101,
			EventType:     arg.EventType,
			AggregateType: arg.AggregateType,
			AggregateID:   arg.AggregateID,
			Payload:       arg.Payload,
			Status:        arg.Status,
		}, nil
	})
}

func expectFactTerminalized(t *testing.T, store *mockdb.MockStore, factID int64, now time.Time) {
	t.Helper()
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), db.UpdateExternalPaymentFactProcessingStatusParams{
		ID:               factID,
		ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized,
		ProcessedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFact{ID: factID, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
}

func expectApplicationApplied(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, now time.Time) {
	t.Helper()
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 application.ID,
		FactID:             application.FactID,
		Consumer:           application.Consumer,
		BusinessObjectType: application.BusinessObjectType,
		BusinessObjectID:   application.BusinessObjectID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
	}, nil)
}

func expectApplicationFailed(t *testing.T, store *mockdb.MockStore, application db.ExternalPaymentFactApplication, now time.Time, errorSubstring string) {
	t.Helper()
	store.EXPECT().MarkExternalPaymentFactApplicationFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkExternalPaymentFactApplicationFailedParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, application.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.True(t, strings.Contains(arg.LastError.String, errorSubstring), "last_error %q does not contain %q", arg.LastError.String, errorSubstring)
		require.True(t, arg.NextRetryAt.Valid)
		require.Equal(t, now.Add(paymentFactApplicationRetryDelay), arg.NextRetryAt.Time)
		application.Status = db.ExternalPaymentFactApplicationStatusFailed
		application.LastError = arg.LastError
		application.NextRetryAt = arg.NextRetryAt
		return application, nil
	})
}

type testBaofuVerifyFeeContinuation struct {
	called       bool
	paymentOrder db.PaymentOrder
}

func (c *testBaofuVerifyFeeContinuation) ContinueAfterVerifyFeePaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	c.called = true
	c.paymentOrder = paymentOrder
	return nil
}
