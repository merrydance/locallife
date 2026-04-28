package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentFactServiceRecordExternalPaymentFact_ProcessingDoesNotCreateApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	input := buildRecordExternalPaymentFactInput(now)
	input.TerminalStatus = db.ExternalPaymentTerminalStatusProcessing
	input.UpstreamState = "PROCESSING"
	input.DedupeKey = "wechat:query:direct:refund:RF202604250001:processing:202604251000"
	input.Application = &ExternalPaymentFactApplicationTarget{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   9001,
	}

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), buildCreateExternalPaymentFactParams(input, now, false)).
		Return(db.ExternalPaymentFact{
			ID:                 101,
			Provider:           input.Provider,
			Channel:            input.Channel,
			Capability:         input.Capability,
			FactSource:         input.FactSource,
			ExternalObjectType: input.ExternalObjectType,
			ExternalObjectKey:  input.ExternalObjectKey,
			UpstreamState:      input.UpstreamState,
			TerminalStatus:     input.TerminalStatus,
			IsTerminal:         false,
			DedupeKey:          input.DedupeKey,
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		}, nil)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(101), result.Fact.ID)
	require.False(t, result.Fact.IsTerminal)
	require.Nil(t, result.Application)
}

func TestPaymentFactServiceRecordExternalPaymentFact_TerminalCreatesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 10, 15, 0, 0, time.UTC)
	input := buildRecordExternalPaymentFactInput(now)
	input.Application = &ExternalPaymentFactApplicationTarget{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   9001,
	}
	fact := db.ExternalPaymentFact{
		ID:                 102,
		Provider:           input.Provider,
		Channel:            input.Channel,
		Capability:         input.Capability,
		FactSource:         input.FactSource,
		ExternalObjectType: input.ExternalObjectType,
		ExternalObjectKey:  input.ExternalObjectKey,
		BusinessObjectID:   pgtype.Int8{Int64: 9001, Valid: true},
		UpstreamState:      input.UpstreamState,
		TerminalStatus:     input.TerminalStatus,
		IsTerminal:         true,
		DedupeKey:          input.DedupeKey,
		ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
	}
	application := db.ExternalPaymentFactApplication{
		ID:                 201,
		FactID:             fact.ID,
		Consumer:           input.Application.Consumer,
		BusinessObjectType: input.Application.BusinessObjectType,
		BusinessObjectID:   input.Application.BusinessObjectID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), buildCreateExternalPaymentFactParams(input, now, true)).
		Return(fact, nil)
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             fact.ID,
			Consumer:           input.Application.Consumer,
			BusinessObjectType: input.Application.BusinessObjectType,
			BusinessObjectID:   input.Application.BusinessObjectID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(application, nil)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, fact.ID, result.Fact.ID)
	require.NotNil(t, result.Application)
	require.Equal(t, application.ID, result.Application.ID)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusPending, result.Application.Status)
}

func TestPaymentFactServiceRecordExternalPaymentFact_DuplicateTerminalReturnsExistingApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC)
	input := buildRecordExternalPaymentFactInput(now)
	input.Application = &ExternalPaymentFactApplicationTarget{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   9001,
	}
	fact := db.ExternalPaymentFact{
		ID:                 103,
		ExternalObjectType: input.ExternalObjectType,
		ExternalObjectKey:  input.ExternalObjectKey,
		TerminalStatus:     input.TerminalStatus,
		IsTerminal:         true,
		DedupeKey:          input.DedupeKey,
		ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
	}
	existingApplication := db.ExternalPaymentFactApplication{
		ID:                 202,
		FactID:             fact.ID,
		Consumer:           input.Application.Consumer,
		BusinessObjectType: input.Application.BusinessObjectType,
		BusinessObjectID:   input.Application.BusinessObjectID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
		AttemptCount:       1,
	}

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), buildCreateExternalPaymentFactParams(input, now, true)).
		Return(fact, nil)
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             fact.ID,
			Consumer:           input.Application.Consumer,
			BusinessObjectType: input.Application.BusinessObjectType,
			BusinessObjectID:   input.Application.BusinessObjectID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(existingApplication, nil)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, result.Application)
	require.Equal(t, db.ExternalPaymentFactApplicationStatusApplied, result.Application.Status)
	require.Equal(t, int32(1), result.Application.AttemptCount)
}

func TestPaymentFactServiceRecordExternalPaymentFact_InvalidStatusDoesNotWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentFactInput(time.Date(2026, 4, 25, 10, 45, 0, 0, time.UTC))
	input.TerminalStatus = "settled"

	svc := NewPaymentFactService(store)
	_, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported terminal status")
}

func TestPaymentFactServiceRecordExternalPaymentFact_CommandResponseSourceAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	input := buildRecordExternalPaymentFactInput(now)
	input.FactSource = db.ExternalPaymentFactSourceCommandResponse
	input.TerminalStatus = db.ExternalPaymentTerminalStatusUnknown
	input.UpstreamState = "SUCCESS"

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), buildCreateExternalPaymentFactParams(input, now, false)).
		Return(db.ExternalPaymentFact{
			ID:               104,
			FactSource:       input.FactSource,
			TerminalStatus:   input.TerminalStatus,
			IsTerminal:       false,
			DedupeKey:        input.DedupeKey,
			ProcessingStatus: db.ExternalPaymentFactProcessingStatusReceived,
		}, nil)

	svc := NewPaymentFactService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, result.Fact.FactSource)
	require.False(t, result.Fact.IsTerminal)
	require.Nil(t, result.Application)
}

func TestPaymentFactServiceRecordExternalPaymentFact_CommandResponseRejectsTerminalStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentFactInput(time.Date(2026, 4, 25, 11, 5, 0, 0, time.UTC))
	input.FactSource = db.ExternalPaymentFactSourceCommandResponse

	svc := NewPaymentFactService(store)
	_, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "command response facts must use terminal status")
}

func TestPaymentFactServiceRecordExternalPaymentFact_CommandResponseRejectsApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentFactInput(time.Date(2026, 4, 25, 11, 10, 0, 0, time.UTC))
	input.FactSource = db.ExternalPaymentFactSourceCommandResponse
	input.TerminalStatus = db.ExternalPaymentTerminalStatusUnknown
	input.Application = &ExternalPaymentFactApplicationTarget{
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   9001,
	}

	svc := NewPaymentFactService(store)
	_, err := svc.RecordExternalPaymentFact(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "command response facts cannot create applications")
}

func TestNormalizeProfitSharingTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizeProfitSharingTerminalStatus("SUCCESS"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizeProfitSharingTerminalStatus("FINISHED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizeProfitSharingTerminalStatus("FAILED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusClosed, NormalizeProfitSharingTerminalStatus("CLOSED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizeProfitSharingTerminalStatus("PROCESSING"))
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizeProfitSharingTerminalStatus("pending"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeProfitSharingTerminalStatus("NEW_STATE"))
}

func TestResolveProfitSharingQueryFinalResult(t *testing.T) {
	t.Run("success when finished and all receivers success", func(t *testing.T) {
		result, failReason := ResolveProfitSharingQueryFinalResult(&wechatcontracts.ProfitSharingQueryResponse{
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Result: wechatcontracts.ProfitSharingResultSuccess}},
		})
		require.Equal(t, "SUCCESS", result)
		require.Empty(t, failReason)
	})

	t.Run("failed when any receiver failed", func(t *testing.T) {
		result, failReason := ResolveProfitSharingQueryFinalResult(&wechatcontracts.ProfitSharingQueryResponse{
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Result: "FAILED", FailReason: "NO_RELATION"}},
		})
		require.Equal(t, "FAILED", result)
		require.Equal(t, "NO_RELATION", failReason)
	})

	t.Run("processing when receiver result is unsupported", func(t *testing.T) {
		result, failReason := ResolveProfitSharingQueryFinalResult(&wechatcontracts.ProfitSharingQueryResponse{
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Result: "UNSUPPORTED_RESULT"}},
		})
		require.Equal(t, "PROCESSING", result)
		require.Empty(t, failReason)
	})
}

func buildRecordExternalPaymentFactInput(now time.Time) RecordExternalPaymentFactInput {
	sourceEventID := "notification-20260425-001"
	sourceEventType := "REFUND.SUCCESS"
	externalSecondaryKey := "wxrefund_202604250001"
	businessOwner := db.ExternalPaymentBusinessOwnerRiderDeposit
	businessObjectType := "refund_order"
	businessObjectID := int64(9001)
	amount := int64(10000)
	return RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        &sourceEventID,
		SourceEventType:      &sourceEventType,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    "RF202604250001",
		ExternalSecondaryKey: &externalSecondaryKey,
		BusinessOwner:        &businessOwner,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               &amount,
		Currency:             "CNY",
		OccurredAt:           &now,
		UpstreamUpdatedAt:    &now,
		RawResource:          []byte(`{"refund_status":"SUCCESS"}`),
		DedupeKey:            "wechat:callback:direct_refund:notification-20260425-001",
	}
}

func buildCreateExternalPaymentFactParams(input RecordExternalPaymentFactInput, observedAt time.Time, isTerminal bool) db.CreateExternalPaymentFactParams {
	return db.CreateExternalPaymentFactParams{
		Provider:             input.Provider,
		Channel:              input.Channel,
		Capability:           input.Capability,
		FactSource:           input.FactSource,
		SourceEventID:        textFromStringPtr(input.SourceEventID),
		SourceEventType:      textFromStringPtr(input.SourceEventType),
		ExternalObjectType:   input.ExternalObjectType,
		ExternalObjectKey:    input.ExternalObjectKey,
		ExternalSecondaryKey: textFromStringPtr(input.ExternalSecondaryKey),
		BusinessOwner:        textFromStringPtr(input.BusinessOwner),
		BusinessObjectType:   textFromStringPtr(input.BusinessObjectType),
		BusinessObjectID:     int8FromInt64Ptr(input.BusinessObjectID),
		UpstreamState:        input.UpstreamState,
		TerminalStatus:       input.TerminalStatus,
		IsTerminal:           isTerminal,
		Amount:               int8FromInt64Ptr(input.Amount),
		Currency:             input.Currency,
		OccurredAt:           timestamptzFromTimePtr(input.OccurredAt),
		UpstreamUpdatedAt:    timestamptzFromTimePtr(input.UpstreamUpdatedAt),
		ObservedAt:           observedAt,
		RawResource:          input.RawResource,
		DedupeKey:            input.DedupeKey,
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	}
}
