package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentCommandServiceRecordExternalPaymentCommand_AcceptedWritesCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	input := buildRecordExternalPaymentCommandInput()
	command := db.ExternalPaymentCommand{
		ID:                 501,
		Provider:           input.Provider,
		Channel:            input.Channel,
		Capability:         input.Capability,
		CommandType:        input.CommandType,
		BusinessOwner:      input.BusinessOwner,
		ExternalObjectType: input.ExternalObjectType,
		ExternalObjectKey:  input.ExternalObjectKey,
		CommandStatus:      input.CommandStatus,
		SubmittedAt:        now,
	}

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), buildCreateExternalPaymentCommandParams(input, now)).
		Return(command, nil)

	svc := NewPaymentCommandService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, command.ID, result.Command.ID)
	require.Equal(t, db.ExternalPaymentCommandStatusAccepted, result.Command.CommandStatus)
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_RejectedDefaultsRejectedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 12, 15, 0, 0, time.UTC)
	input := buildRecordExternalPaymentCommandInput()
	input.CommandStatus = db.ExternalPaymentCommandStatusRejected
	errorCode := "PARAM_ERROR"
	errorMessage := "invalid amount"
	input.LastErrorCode = &errorCode
	input.LastErrorMessage = &errorMessage
	input.ResponseSnapshot = nil

	params := buildCreateExternalPaymentCommandParams(input, now)
	params.AcceptedAt = pgtype.Timestamptz{}
	params.RejectedAt = pgtype.Timestamptz{Time: now, Valid: true}
	params.LastErrorCode = pgtype.Text{String: errorCode, Valid: true}
	params.LastErrorMessage = pgtype.Text{String: errorMessage, Valid: true}
	params.ResponseSnapshot = []byte(`{}`)

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), params).
		Return(db.ExternalPaymentCommand{ID: 502, CommandStatus: input.CommandStatus}, nil)

	svc := NewPaymentCommandService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(502), result.Command.ID)
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_DuplicateReturnsExistingCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Date(2026, 4, 25, 12, 30, 0, 0, time.UTC)
	input := buildRecordExternalPaymentCommandInput()
	existingCommand := db.ExternalPaymentCommand{
		ID:            503,
		CommandStatus: db.ExternalPaymentCommandStatusSubmitted,
	}

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), buildCreateExternalPaymentCommandParams(input, now)).
		Return(existingCommand, nil)

	svc := NewPaymentCommandService(store)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, existingCommand.ID, result.Command.ID)
	require.Equal(t, db.ExternalPaymentCommandStatusSubmitted, result.Command.CommandStatus)
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_InvalidStatusDoesNotWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentCommandInput()
	input.CommandStatus = "success"

	svc := NewPaymentCommandService(store)
	_, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported command status")
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_InvalidCapabilityDoesNotWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentCommandInput()
	input.Capability = "direct_refund_success"

	svc := NewPaymentCommandService(store)
	_, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported capability")
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_AcceptedAtRequiresAcceptedStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentCommandInput()
	input.CommandStatus = db.ExternalPaymentCommandStatusSubmitted
	acceptedAt := time.Date(2026, 4, 25, 12, 45, 0, 0, time.UTC)
	input.AcceptedAt = &acceptedAt

	svc := NewPaymentCommandService(store)
	_, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepted at is only valid")
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_BusinessObjectPairRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	input := buildRecordExternalPaymentCommandInput()
	input.BusinessObjectID = nil

	svc := NewPaymentCommandService(store)
	_, err := svc.RecordExternalPaymentCommand(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "business object type and id must be provided together")
}

func buildRecordExternalPaymentCommandInput() RecordExternalPaymentCommandInput {
	businessObjectType := "refund_order"
	businessObjectID := int64(7001)
	externalSecondaryKey := "wxrefund_202604250001"
	requestFingerprint := "sha256:direct-refund-request-001"
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    "RF202604250001",
		ExternalSecondaryKey: &externalSecondaryKey,
		CommandStatus:        db.ExternalPaymentCommandStatusAccepted,
		RequestFingerprint:   &requestFingerprint,
		ResponseSnapshot:     []byte(`{"refund_id":"wxrefund_202604250001","status":"PROCESSING"}`),
	}
}

func buildCreateExternalPaymentCommandParams(input RecordExternalPaymentCommandInput, now time.Time) db.CreateExternalPaymentCommandParams {
	return db.CreateExternalPaymentCommandParams{
		Provider:             input.Provider,
		Channel:              input.Channel,
		Capability:           input.Capability,
		CommandType:          input.CommandType,
		BusinessOwner:        input.BusinessOwner,
		BusinessObjectType:   textFromStringPtr(input.BusinessObjectType),
		BusinessObjectID:     int8FromInt64Ptr(input.BusinessObjectID),
		ExternalObjectType:   input.ExternalObjectType,
		ExternalObjectKey:    input.ExternalObjectKey,
		ExternalSecondaryKey: textFromStringPtr(input.ExternalSecondaryKey),
		CommandStatus:        input.CommandStatus,
		SubmittedAt:          now,
		AcceptedAt:           pgtype.Timestamptz{Time: now, Valid: true},
		RejectedAt:           pgtype.Timestamptz{},
		LastErrorCode:        textFromStringPtr(input.LastErrorCode),
		LastErrorMessage:     textFromStringPtr(input.LastErrorMessage),
		RequestFingerprint:   textFromStringPtr(input.RequestFingerprint),
		ResponseSnapshot:     input.ResponseSnapshot,
	}
}
