package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomExternalPaymentFact(t *testing.T, terminalStatus string, isTerminal bool) ExternalPaymentFact {
	dedupeKey := "wechat:callback:refund:" + util.RandomString(18)
	outRefundNo := "RF" + util.RandomString(24)
	now := time.Now().UTC()

	fact, err := testStore.CreateExternalPaymentFact(context.Background(), CreateExternalPaymentFactParams{
		Provider:             ExternalPaymentProviderWechat,
		Channel:              PaymentChannelDirect,
		Capability:           ExternalPaymentCapabilityDirectRefund,
		FactSource:           ExternalPaymentFactSourceCallback,
		SourceEventID:        pgtype.Text{String: util.RandomString(32), Valid: true},
		SourceEventType:      pgtype.Text{String: "REFUND.SUCCESS", Valid: true},
		ExternalObjectType:   ExternalPaymentObjectRefund,
		ExternalObjectKey:    outRefundNo,
		ExternalSecondaryKey: pgtype.Text{String: "wxrefund_" + util.RandomString(16), Valid: true},
		BusinessOwner:        pgtype.Text{String: ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "refund_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       terminalStatus,
		IsTerminal:           isTerminal,
		Amount:               pgtype.Int8{Int64: 10000, Valid: true},
		Currency:             "CNY",
		OccurredAt:           pgtype.Timestamptz{Time: now, Valid: true},
		UpstreamUpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		ObservedAt:           now,
		RawResource:          []byte(`{"refund_status":"SUCCESS"}`),
		DedupeKey:            dedupeKey,
		ProcessingStatus:     ExternalPaymentFactProcessingStatusReceived,
	})
	require.NoError(t, err)
	require.NotZero(t, fact.ID)
	require.Equal(t, dedupeKey, fact.DedupeKey)
	require.Equal(t, terminalStatus, fact.TerminalStatus)
	require.Equal(t, isTerminal, fact.IsTerminal)

	return fact
}

func TestCreateExternalPaymentCommand_DedupesByExternalObject(t *testing.T) {
	now := time.Now().UTC()
	externalKey := "PAY" + util.RandomString(24)

	arg := CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderWechat,
		Channel:            PaymentChannelDirect,
		Capability:         ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:        ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectPayment,
		ExternalObjectKey:  externalKey,
		CommandStatus:      ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        now,
		ResponseSnapshot:   []byte(`{"prepay_id":"wx_secret_redacted"}`),
	}

	command1, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	arg.CommandStatus = ExternalPaymentCommandStatusAccepted
	arg.AcceptedAt = pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true}
	arg.ResponseSnapshot = []byte(`{"prepay_id":"wx_secret_redacted","status":"accepted"}`)
	command2, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	require.Equal(t, command1.ID, command2.ID)
	require.Equal(t, ExternalPaymentCommandStatusAccepted, command2.CommandStatus)
	require.True(t, command2.AcceptedAt.Valid)
	require.JSONEq(t, `{"prepay_id":"wx_secret_redacted","status":"accepted"}`, string(command2.ResponseSnapshot))
}

func TestCreateExternalPaymentCommand_DoesNotDowngradeAcceptedToSubmitted(t *testing.T) {
	now := time.Now().UTC()
	externalKey := "PAY" + util.RandomString(24)

	arg := CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderWechat,
		Channel:            PaymentChannelDirect,
		Capability:         ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:        ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectPayment,
		ExternalObjectKey:  externalKey,
		CommandStatus:      ExternalPaymentCommandStatusAccepted,
		SubmittedAt:        now,
		AcceptedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		ResponseSnapshot:   []byte(`{"status":"accepted"}`),
	}

	command1, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	arg.CommandStatus = ExternalPaymentCommandStatusSubmitted
	arg.AcceptedAt = pgtype.Timestamptz{}
	arg.ResponseSnapshot = []byte(`{"status":"submitted"}`)
	command2, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	require.Equal(t, command1.ID, command2.ID)
	require.Equal(t, ExternalPaymentCommandStatusAccepted, command2.CommandStatus)
	require.True(t, command2.AcceptedAt.Valid)
	require.JSONEq(t, `{"status":"accepted"}`, string(command2.ResponseSnapshot))
}

func TestCreateExternalPaymentCommand_DoesNotAttachRejectedTimeToAcceptedCommand(t *testing.T) {
	now := time.Now().UTC()
	externalKey := "PAY" + util.RandomString(24)

	arg := CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderWechat,
		Channel:            PaymentChannelDirect,
		Capability:         ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:        ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectPayment,
		ExternalObjectKey:  externalKey,
		CommandStatus:      ExternalPaymentCommandStatusAccepted,
		SubmittedAt:        now,
		AcceptedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		ResponseSnapshot:   []byte(`{"status":"accepted"}`),
	}

	command1, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	arg.CommandStatus = ExternalPaymentCommandStatusRejected
	arg.AcceptedAt = pgtype.Timestamptz{}
	arg.RejectedAt = pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true}
	arg.LastErrorCode = pgtype.Text{String: "provider_rejected", Valid: true}
	arg.LastErrorMessage = pgtype.Text{String: "provider rejected", Valid: true}
	arg.ResponseSnapshot = []byte(`{"status":"rejected"}`)
	command2, err := testStore.CreateExternalPaymentCommand(context.Background(), arg)
	require.NoError(t, err)

	require.Equal(t, command1.ID, command2.ID)
	require.Equal(t, ExternalPaymentCommandStatusAccepted, command2.CommandStatus)
	require.True(t, command2.AcceptedAt.Valid)
	require.False(t, command2.RejectedAt.Valid)
	require.False(t, command2.LastErrorCode.Valid)
	require.False(t, command2.LastErrorMessage.Valid)
	require.JSONEq(t, `{"status":"accepted"}`, string(command2.ResponseSnapshot))
}

func TestUpdateExternalPaymentCommandOutcome_AcceptsSubmittedCommand(t *testing.T) {
	now := time.Now().UTC()
	command, err := testStore.CreateExternalPaymentCommand(context.Background(), CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuProfitSharing,
		CommandType:        ExternalPaymentCommandTypeCreateProfitSharing,
		BusinessOwner:      ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  "BFPS" + util.RandomString(24),
		CommandStatus:      ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        now,
		ResponseSnapshot:   []byte(`{"operation":"share_after_pay"}`),
	})
	require.NoError(t, err)

	updated, err := testStore.UpdateExternalPaymentCommandOutcome(context.Background(), UpdateExternalPaymentCommandOutcomeParams{
		ID:               command.ID,
		CommandStatus:    ExternalPaymentCommandStatusAccepted,
		AcceptedAt:       pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
		ResponseSnapshot: []byte(`{"operation":"share_after_pay","outcome":"accepted"}`),
	})
	require.NoError(t, err)

	require.Equal(t, ExternalPaymentCommandStatusAccepted, updated.CommandStatus)
	require.True(t, updated.AcceptedAt.Valid)
	require.False(t, updated.RejectedAt.Valid)
	require.False(t, updated.LastErrorCode.Valid)
	require.False(t, updated.LastErrorMessage.Valid)
	require.JSONEq(t, `{"operation":"share_after_pay","outcome":"accepted"}`, string(updated.ResponseSnapshot))
}

func TestUpdateExternalPaymentCommandOutcome_DoesNotRewriteAcceptedCommand(t *testing.T) {
	now := time.Now().UTC()
	command, err := testStore.CreateExternalPaymentCommand(context.Background(), CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuProfitSharing,
		CommandType:        ExternalPaymentCommandTypeCreateProfitSharing,
		BusinessOwner:      ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  "BFPS" + util.RandomString(24),
		CommandStatus:      ExternalPaymentCommandStatusAccepted,
		SubmittedAt:        now,
		AcceptedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		ResponseSnapshot:   []byte(`{"operation":"share_after_pay","outcome":"accepted","first":"true"}`),
	})
	require.NoError(t, err)

	updated, err := testStore.UpdateExternalPaymentCommandOutcome(context.Background(), UpdateExternalPaymentCommandOutcomeParams{
		ID:               command.ID,
		CommandStatus:    ExternalPaymentCommandStatusAccepted,
		AcceptedAt:       pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
		ResponseSnapshot: []byte(`{"operation":"share_after_pay","outcome":"accepted","first":"false"}`),
	})
	require.NoError(t, err)

	require.Equal(t, ExternalPaymentCommandStatusAccepted, updated.CommandStatus)
	require.True(t, command.AcceptedAt.Time.Equal(updated.AcceptedAt.Time))
	require.False(t, updated.RejectedAt.Valid)
	require.JSONEq(t, `{"operation":"share_after_pay","outcome":"accepted","first":"true"}`, string(updated.ResponseSnapshot))
}

func TestCreateExternalPaymentCommand_AcceptsBaofuAggregateChannel(t *testing.T) {
	externalKey := "BF" + util.RandomString(24)

	command, err := testStore.CreateExternalPaymentCommand(context.Background(), CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuPayment,
		CommandType:        ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      ExternalPaymentBusinessOwnerOrder,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		ExternalObjectType: ExternalPaymentObjectPayment,
		ExternalObjectKey:  externalKey,
		CommandStatus:      ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        time.Now().UTC(),
		ResponseSnapshot:   []byte(`{"provider":"baofu"}`),
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentProviderBaofu, command.Provider)
	require.Equal(t, PaymentChannelBaofuAggregate, command.Channel)
	require.Equal(t, ExternalPaymentCapabilityBaofuPayment, command.Capability)
}

func TestCreateExternalPaymentFact_DedupesIdenticalFactByDedupeKey(t *testing.T) {
	fact1 := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)

	arg := CreateExternalPaymentFactParams{
		Provider:             fact1.Provider,
		Channel:              fact1.Channel,
		Capability:           fact1.Capability,
		FactSource:           fact1.FactSource,
		SourceEventID:        fact1.SourceEventID,
		SourceEventType:      fact1.SourceEventType,
		ExternalObjectType:   fact1.ExternalObjectType,
		ExternalObjectKey:    fact1.ExternalObjectKey,
		ExternalSecondaryKey: fact1.ExternalSecondaryKey,
		BusinessOwner:        fact1.BusinessOwner,
		BusinessObjectType:   fact1.BusinessObjectType,
		BusinessObjectID:     fact1.BusinessObjectID,
		UpstreamState:        fact1.UpstreamState,
		TerminalStatus:       fact1.TerminalStatus,
		IsTerminal:           fact1.IsTerminal,
		Amount:               fact1.Amount,
		Currency:             fact1.Currency,
		OccurredAt:           fact1.OccurredAt,
		UpstreamUpdatedAt:    fact1.UpstreamUpdatedAt,
		ObservedAt:           time.Now().UTC(),
		RawResource:          fact1.RawResource,
		DedupeKey:            fact1.DedupeKey,
		ProcessingStatus:     fact1.ProcessingStatus,
	}

	fact2, err := testStore.CreateExternalPaymentFact(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, fact1.ID, fact2.ID)
	require.Equal(t, ExternalPaymentTerminalStatusSuccess, fact2.TerminalStatus)
	require.True(t, fact2.IsTerminal)
}

func TestCreateExternalPaymentFact_DedupesSameSemanticFactWhenSnapshotDiffers(t *testing.T) {
	fact1 := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)

	fact2, err := testStore.CreateExternalPaymentFact(context.Background(), CreateExternalPaymentFactParams{
		Provider:             fact1.Provider,
		Channel:              fact1.Channel,
		Capability:           fact1.Capability,
		FactSource:           fact1.FactSource,
		SourceEventID:        fact1.SourceEventID,
		SourceEventType:      fact1.SourceEventType,
		ExternalObjectType:   fact1.ExternalObjectType,
		ExternalObjectKey:    fact1.ExternalObjectKey,
		ExternalSecondaryKey: fact1.ExternalSecondaryKey,
		BusinessOwner:        fact1.BusinessOwner,
		BusinessObjectType:   fact1.BusinessObjectType,
		BusinessObjectID:     fact1.BusinessObjectID,
		UpstreamState:        fact1.UpstreamState,
		TerminalStatus:       fact1.TerminalStatus,
		IsTerminal:           fact1.IsTerminal,
		Amount:               fact1.Amount,
		Currency:             fact1.Currency,
		OccurredAt:           pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Minute), Valid: true},
		UpstreamUpdatedAt:    pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Minute), Valid: true},
		ObservedAt:           time.Now().UTC(),
		RawResource:          []byte(`{"refund_status":"SUCCESS","retry":true}`),
		DedupeKey:            fact1.DedupeKey,
		ProcessingStatus:     ExternalPaymentFactProcessingStatusReceived,
	})

	require.NoError(t, err)
	require.Equal(t, fact1.ID, fact2.ID)
	require.JSONEq(t, string(fact1.RawResource), string(fact2.RawResource))
	require.Equal(t, fact1.OccurredAt, fact2.OccurredAt)
}

func TestCreateExternalPaymentFact_AcceptsBaofuAggregateChannel(t *testing.T) {
	now := time.Now().UTC()
	dedupeKey := "baofu:callback:payment:" + util.RandomString(18)

	fact, err := testStore.CreateExternalPaymentFact(context.Background(), CreateExternalPaymentFactParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuPayment,
		FactSource:         ExternalPaymentFactSourceCallback,
		SourceEventID:      pgtype.Text{String: "BF_EVT_" + util.RandomString(18), Valid: true},
		SourceEventType:    pgtype.Text{String: "PAYMENT.SUCCESS", Valid: true},
		ExternalObjectType: ExternalPaymentObjectPayment,
		ExternalObjectKey:  "BF" + util.RandomString(24),
		BusinessOwner:      pgtype.Text{String: ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: time.Now().UnixNano(), Valid: true},
		UpstreamState:      "SUCCESS",
		TerminalStatus:     ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
		Amount:             pgtype.Int8{Int64: 10000, Valid: true},
		Currency:           "CNY",
		OccurredAt:         pgtype.Timestamptz{Time: now, Valid: true},
		ObservedAt:         now,
		RawResource:        []byte(`{"provider":"baofu","status":"SUCCESS"}`),
		DedupeKey:          dedupeKey,
		ProcessingStatus:   ExternalPaymentFactProcessingStatusReceived,
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentProviderBaofu, fact.Provider)
	require.Equal(t, PaymentChannelBaofuAggregate, fact.Channel)
	require.Equal(t, dedupeKey, fact.DedupeKey)
}

func TestCreateExternalPaymentFact_RejectsDedupeKeyWithDifferentPayload(t *testing.T) {
	fact1 := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)

	_, err := testStore.CreateExternalPaymentFact(context.Background(), CreateExternalPaymentFactParams{
		Provider:           ExternalPaymentProviderWechat,
		Channel:            PaymentChannelDirect,
		Capability:         ExternalPaymentCapabilityDirectRefund,
		FactSource:         ExternalPaymentFactSourceQuery,
		ExternalObjectType: ExternalPaymentObjectRefund,
		ExternalObjectKey:  fact1.ExternalObjectKey,
		UpstreamState:      "PROCESSING",
		TerminalStatus:     ExternalPaymentTerminalStatusProcessing,
		IsTerminal:         false,
		Currency:           "CNY",
		ObservedAt:         time.Now().UTC(),
		RawResource:        []byte(`{"refund_status":"PROCESSING"}`),
		DedupeKey:          fact1.DedupeKey,
		ProcessingStatus:   ExternalPaymentFactProcessingStatusReceived,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestCreateExternalPaymentFact_RejectsInconsistentTerminalFlag(t *testing.T) {
	now := time.Now().UTC()
	_, err := testStore.CreateExternalPaymentFact(context.Background(), CreateExternalPaymentFactParams{
		Provider:           ExternalPaymentProviderWechat,
		Channel:            PaymentChannelDirect,
		Capability:         ExternalPaymentCapabilityDirectRefund,
		FactSource:         ExternalPaymentFactSourceQuery,
		ExternalObjectType: ExternalPaymentObjectRefund,
		ExternalObjectKey:  "RF" + util.RandomString(24),
		UpstreamState:      "PROCESSING",
		TerminalStatus:     ExternalPaymentTerminalStatusProcessing,
		IsTerminal:         true,
		Currency:           "CNY",
		ObservedAt:         now,
		RawResource:        []byte(`{"refund_status":"PROCESSING"}`),
		DedupeKey:          "wechat:query:refund:" + util.RandomString(18),
		ProcessingStatus:   ExternalPaymentFactProcessingStatusReceived,
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))
}

func TestExternalPaymentFactApplication_StateTransitions(t *testing.T) {
	fact := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)
	now := time.Now().UTC()

	application, err := testStore.CreateExternalPaymentFactApplication(context.Background(), CreateExternalPaymentFactApplicationParams{
		FactID:             fact.ID,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   fact.BusinessObjectID.Int64,
		Status:             ExternalPaymentFactApplicationStatusPending,
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusPending, application.Status)
	require.Equal(t, int32(0), application.AttemptCount)

	duplicate, err := testStore.CreateExternalPaymentFactApplication(context.Background(), CreateExternalPaymentFactApplicationParams{
		FactID:             fact.ID,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   fact.BusinessObjectID.Int64,
		Status:             ExternalPaymentFactApplicationStatusPending,
	})
	require.NoError(t, err)
	require.Equal(t, application.ID, duplicate.ID)

	claimed, err := testStore.ClaimExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusProcessing, claimed.Status)
	require.Equal(t, int32(1), claimed.AttemptCount)

	_, err = testStore.ClaimExternalPaymentFactApplication(context.Background(), application.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	retryAt := now.Add(time.Minute)
	failed, err := testStore.MarkExternalPaymentFactApplicationFailed(context.Background(), MarkExternalPaymentFactApplicationFailedParams{
		ID:          application.ID,
		LastError:   pgtype.Text{String: "temporary domain failure", Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: retryAt, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusFailed, failed.Status)
	require.True(t, failed.NextRetryAt.Valid)

	retryableBefore, err := testStore.ListRetryableExternalPaymentFactApplications(context.Background(), ListRetryableExternalPaymentFactApplicationsParams{
		NowAt:      pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount: 10,
	})
	require.NoError(t, err)
	require.NotContains(t, externalPaymentFactApplicationIDs(retryableBefore), application.ID)

	retryableAfter, err := testStore.ListRetryableExternalPaymentFactApplications(context.Background(), ListRetryableExternalPaymentFactApplicationsParams{
		NowAt:      pgtype.Timestamptz{Time: retryAt.Add(time.Second), Valid: true},
		LimitCount: 10,
	})
	require.NoError(t, err)
	require.Contains(t, externalPaymentFactApplicationIDs(retryableAfter), application.ID)

	claimedAgain, err := testStore.ClaimExternalPaymentFactApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.Equal(t, int32(2), claimedAgain.AttemptCount)

	applied, err := testStore.MarkExternalPaymentFactApplicationApplied(context.Background(), MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusApplied, applied.Status)
	require.False(t, applied.NextRetryAt.Valid)
	require.False(t, applied.LastError.Valid)
	require.True(t, applied.AppliedAt.Valid)
}

func TestReclaimStaleExternalPaymentFactApplicationsByTarget(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	staleUpdatedAt := now.Add(-20 * time.Minute)

	staleFact := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)
	staleApplication, err := testStore.CreateExternalPaymentFactApplication(ctx, CreateExternalPaymentFactApplicationParams{
		FactID:             staleFact.ID,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   staleFact.BusinessObjectID.Int64,
		Status:             ExternalPaymentFactApplicationStatusPending,
	})
	require.NoError(t, err)
	staleApplication, err = testStore.ClaimExternalPaymentFactApplication(ctx, staleApplication.ID)
	require.NoError(t, err)
	setExternalPaymentFactApplicationUpdatedAt(t, staleApplication.ID, staleUpdatedAt)

	recentFact := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)
	recentApplication, err := testStore.CreateExternalPaymentFactApplication(ctx, CreateExternalPaymentFactApplicationParams{
		FactID:             recentFact.ID,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   recentFact.BusinessObjectID.Int64,
		Status:             ExternalPaymentFactApplicationStatusPending,
	})
	require.NoError(t, err)
	recentApplication, err = testStore.ClaimExternalPaymentFactApplication(ctx, recentApplication.ID)
	require.NoError(t, err)

	otherTargetFact := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)
	otherTargetApplication, err := testStore.CreateExternalPaymentFactApplication(ctx, CreateExternalPaymentFactApplicationParams{
		FactID:             otherTargetFact.ID,
		Consumer:           "order_domain",
		BusinessObjectType: "payment_order",
		BusinessObjectID:   otherTargetFact.BusinessObjectID.Int64,
		Status:             ExternalPaymentFactApplicationStatusPending,
	})
	require.NoError(t, err)
	otherTargetApplication, err = testStore.ClaimExternalPaymentFactApplication(ctx, otherTargetApplication.ID)
	require.NoError(t, err)
	setExternalPaymentFactApplicationUpdatedAt(t, otherTargetApplication.ID, staleUpdatedAt)

	reclaimed, err := testStore.ReclaimStaleExternalPaymentFactApplicationsByTarget(ctx, ReclaimStaleExternalPaymentFactApplicationsByTargetParams{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		StaleBefore:        now.Add(-15 * time.Minute),
		LastError:          pgtype.Text{String: "stale processing payment fact application reclaimed by scheduler", Valid: true},
		NextRetryAt:        pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount:         10,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{staleApplication.ID}, externalPaymentFactApplicationIDs(reclaimed))
	require.Equal(t, ExternalPaymentFactApplicationStatusFailed, reclaimed[0].Status)
	require.Equal(t, int32(1), reclaimed[0].AttemptCount)
	require.Equal(t, "stale processing payment fact application reclaimed by scheduler", reclaimed[0].LastError.String)
	require.True(t, reclaimed[0].NextRetryAt.Valid)

	recentAfter, err := testStore.GetExternalPaymentFactApplication(ctx, recentApplication.ID)
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusProcessing, recentAfter.Status)

	otherTargetAfter, err := testStore.GetExternalPaymentFactApplication(ctx, otherTargetApplication.ID)
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentFactApplicationStatusProcessing, otherTargetAfter.Status)

	retryable, err := testStore.ListRetryableExternalPaymentFactApplicationsByTarget(ctx, ListRetryableExternalPaymentFactApplicationsByTargetParams{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		NowAt:              pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
		LimitCount:         10,
	})
	require.NoError(t, err)
	require.Contains(t, externalPaymentFactApplicationIDs(retryable), staleApplication.ID)
	require.NotContains(t, externalPaymentFactApplicationIDs(retryable), recentApplication.ID)
}

func TestPaymentDomainOutbox_PendingList(t *testing.T) {
	now := time.Now().UTC()
	eventType := "rider_deposit_activated_" + util.RandomString(12)
	entry, err := testStore.CreatePaymentDomainOutbox(context.Background(), CreatePaymentDomainOutboxParams{
		EventType:     eventType,
		AggregateType: "rider",
		AggregateID:   time.Now().UnixNano(),
		Payload:       []byte(`{"rider_id":1}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)

	entries, err := testStore.ListPendingPaymentDomainOutboxByEventType(context.Background(), ListPendingPaymentDomainOutboxByEventTypeParams{
		EventType:  eventType,
		NowAt:      pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount: 10,
	})
	require.NoError(t, err)
	require.Contains(t, paymentDomainOutboxIDs(entries), entry.ID)
}

func TestCreatePaymentDomainOutboxOnce_DedupesIdenticalEventAndAggregate(t *testing.T) {
	aggregateID := time.Now().UnixNano()
	payload := []byte(`{"profit_sharing_order_id":1,"result":"SUCCESS"}`)
	first, err := testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       payload,
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)
	require.NotZero(t, first.ID)

	duplicate, err := testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       payload,
		Status:        PaymentDomainOutboxStatusFailed,
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, duplicate.ID)
	require.Equal(t, PaymentDomainOutboxStatusPending, duplicate.Status)
	require.JSONEq(t, string(first.Payload), string(duplicate.Payload))
}

func TestCreatePaymentDomainOutboxOnce_DedupesPayloadWithDifferentAuditFactIDs(t *testing.T) {
	aggregateID := time.Now().UnixNano()
	first, err := testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       []byte(`{"profit_sharing_order_id":1,"out_order_no":"BFPS20O8","result":"SUCCESS","merchant_id":2,"external_payment_fact_id":62,"payment_fact_application_id":44}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)

	duplicate, err := testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       []byte(`{"profit_sharing_order_id":1,"out_order_no":"BFPS20O8","result":"SUCCESS","merchant_id":2,"external_payment_fact_id":63,"payment_fact_application_id":45}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, duplicate.ID)
	require.JSONEq(t, string(first.Payload), string(duplicate.Payload))
}

func TestCreatePaymentDomainOutboxOnce_RejectsDifferentPayloadForSameEventAndAggregate(t *testing.T) {
	aggregateID := time.Now().UnixNano()
	_, err := testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       []byte(`{"profit_sharing_order_id":1,"result":"SUCCESS"}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)

	_, err = testStore.CreatePaymentDomainOutboxOnce(context.Background(), CreatePaymentDomainOutboxOnceParams{
		EventType:     PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   aggregateID,
		Payload:       []byte(`{"profit_sharing_order_id":1,"result":"FAILED"}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestPaymentDomainOutbox_ClaimAndMarkLifecycle(t *testing.T) {
	now := time.Now().UTC()
	entry, err := testStore.CreatePaymentDomainOutbox(context.Background(), CreatePaymentDomainOutboxParams{
		EventType:     "profit_sharing_result_ready",
		AggregateType: "profit_sharing_order",
		AggregateID:   time.Now().UnixNano(),
		Payload:       []byte(`{"profit_sharing_order_id":1}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)

	claimed, err := testStore.ClaimPaymentDomainOutbox(context.Background(), ClaimPaymentDomainOutboxParams{
		ID:    entry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, PaymentDomainOutboxStatusProcessing, claimed.Status)
	require.Equal(t, int32(1), claimed.AttemptCount)

	_, err = testStore.ClaimPaymentDomainOutbox(context.Background(), ClaimPaymentDomainOutboxParams{
		ID:    entry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	failed, err := testStore.MarkPaymentDomainOutboxFailed(context.Background(), MarkPaymentDomainOutboxFailedParams{
		ID:          entry.ID,
		LastError:   pgtype.Text{String: "queue unavailable", Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, PaymentDomainOutboxStatusFailed, failed.Status)
	require.Equal(t, "queue unavailable", failed.LastError.String)
	require.True(t, failed.NextRetryAt.Valid)

	_, err = testStore.ClaimPaymentDomainOutbox(context.Background(), ClaimPaymentDomainOutboxParams{
		ID:    entry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	reclaimed, err := testStore.ClaimPaymentDomainOutbox(context.Background(), ClaimPaymentDomainOutboxParams{
		ID:    entry.ID,
		NowAt: pgtype.Timestamptz{Time: now.Add(2 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, PaymentDomainOutboxStatusProcessing, reclaimed.Status)
	require.Equal(t, int32(2), reclaimed.AttemptCount)

	published, err := testStore.MarkPaymentDomainOutboxPublished(context.Background(), entry.ID)
	require.NoError(t, err)
	require.Equal(t, PaymentDomainOutboxStatusPublished, published.Status)
	require.False(t, published.NextRetryAt.Valid)
	require.False(t, published.LastError.Valid)

	_, err = testStore.MarkPaymentDomainOutboxFailed(context.Background(), MarkPaymentDomainOutboxFailedParams{
		ID:          entry.ID,
		LastError:   pgtype.Text{String: "late failure", Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: now.Add(3 * time.Minute), Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestReclaimStalePaymentDomainOutboxByEventType(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	eventType := "payment_domain_outbox_reclaim_" + util.RandomString(12)
	otherEventType := "payment_domain_outbox_reclaim_other_" + util.RandomString(12)

	staleEntry, err := testStore.CreatePaymentDomainOutbox(ctx, CreatePaymentDomainOutboxParams{
		EventType:     eventType,
		AggregateType: "profit_sharing_order",
		AggregateID:   time.Now().UnixNano(),
		Payload:       []byte(`{"profit_sharing_order_id":1}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)
	staleEntry, err = testStore.ClaimPaymentDomainOutbox(ctx, ClaimPaymentDomainOutboxParams{
		ID:    staleEntry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.NoError(t, err)
	setPaymentDomainOutboxUpdatedAt(t, staleEntry.ID, now.Add(-20*time.Minute))

	recentEntry, err := testStore.CreatePaymentDomainOutbox(ctx, CreatePaymentDomainOutboxParams{
		EventType:     eventType,
		AggregateType: "profit_sharing_order",
		AggregateID:   time.Now().UnixNano(),
		Payload:       []byte(`{"profit_sharing_order_id":2}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)
	recentEntry, err = testStore.ClaimPaymentDomainOutbox(ctx, ClaimPaymentDomainOutboxParams{
		ID:    recentEntry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.NoError(t, err)

	otherEventEntry, err := testStore.CreatePaymentDomainOutbox(ctx, CreatePaymentDomainOutboxParams{
		EventType:     otherEventType,
		AggregateType: "profit_sharing_order",
		AggregateID:   time.Now().UnixNano(),
		Payload:       []byte(`{"profit_sharing_order_id":3}`),
		Status:        PaymentDomainOutboxStatusPending,
	})
	require.NoError(t, err)
	otherEventEntry, err = testStore.ClaimPaymentDomainOutbox(ctx, ClaimPaymentDomainOutboxParams{
		ID:    otherEventEntry.ID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.NoError(t, err)
	setPaymentDomainOutboxUpdatedAt(t, otherEventEntry.ID, now.Add(-20*time.Minute))

	reclaimed, err := testStore.ReclaimStalePaymentDomainOutboxByEventType(ctx, ReclaimStalePaymentDomainOutboxByEventTypeParams{
		EventType:   eventType,
		StaleBefore: now.Add(-15 * time.Minute),
		LastError:   pgtype.Text{String: "stale processing payment domain outbox reclaimed by scheduler", Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount:  10,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{staleEntry.ID}, paymentDomainOutboxIDs(reclaimed))
	require.Equal(t, PaymentDomainOutboxStatusFailed, reclaimed[0].Status)
	require.Equal(t, int32(1), reclaimed[0].AttemptCount)
	require.Equal(t, "stale processing payment domain outbox reclaimed by scheduler", reclaimed[0].LastError.String)
	require.True(t, reclaimed[0].NextRetryAt.Valid)

	require.Equal(t, PaymentDomainOutboxStatusProcessing, getPaymentDomainOutboxStatus(t, recentEntry.ID))

	require.Equal(t, PaymentDomainOutboxStatusProcessing, getPaymentDomainOutboxStatus(t, otherEventEntry.ID))

	pendingEntries, err := testStore.ListPendingPaymentDomainOutboxByEventType(ctx, ListPendingPaymentDomainOutboxByEventTypeParams{
		EventType:  eventType,
		NowAt:      pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
		LimitCount: 10,
	})
	require.NoError(t, err)
	require.Contains(t, paymentDomainOutboxIDs(pendingEntries), staleEntry.ID)
	require.NotContains(t, paymentDomainOutboxIDs(pendingEntries), recentEntry.ID)
}

func setExternalPaymentFactApplicationUpdatedAt(t *testing.T, id int64, updatedAt time.Time) {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)
	_, err := store.connPool.Exec(context.Background(), "UPDATE external_payment_fact_applications SET updated_at = $1 WHERE id = $2", updatedAt, id)
	require.NoError(t, err)
}

func setPaymentDomainOutboxUpdatedAt(t *testing.T, id int64, updatedAt time.Time) {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)
	_, err := store.connPool.Exec(context.Background(), "UPDATE payment_domain_outbox SET updated_at = $1 WHERE id = $2", updatedAt, id)
	require.NoError(t, err)
}

func getPaymentDomainOutboxStatus(t *testing.T, id int64) string {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)
	var status string
	err := store.connPool.QueryRow(context.Background(), "SELECT status FROM payment_domain_outbox WHERE id = $1", id).Scan(&status)
	require.NoError(t, err)
	return status
}

func externalPaymentFactApplicationIDs(applications []ExternalPaymentFactApplication) []int64 {
	ids := make([]int64, 0, len(applications))
	for _, application := range applications {
		ids = append(ids, application.ID)
	}
	return ids
}

func paymentDomainOutboxIDs(entries []PaymentDomainOutbox) []int64 {
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}
