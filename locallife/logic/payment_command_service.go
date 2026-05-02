package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

type PaymentCommandService struct {
	store externalPaymentCommandStore
	now   func() time.Time
}

type externalPaymentCommandStore interface {
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
}

func NewPaymentCommandService(store externalPaymentCommandStore) *PaymentCommandService {
	return &PaymentCommandService{
		store: store,
		now:   time.Now,
	}
}

type RecordExternalPaymentCommandInput struct {
	Provider             string
	Channel              string
	Capability           string
	CommandType          string
	BusinessOwner        string
	BusinessObjectType   *string
	BusinessObjectID     *int64
	ExternalObjectType   string
	ExternalObjectKey    string
	ExternalSecondaryKey *string
	CommandStatus        string
	SubmittedAt          *time.Time
	AcceptedAt           *time.Time
	RejectedAt           *time.Time
	LastErrorCode        *string
	LastErrorMessage     *string
	RequestFingerprint   *string
	ResponseSnapshot     []byte
}

type RecordExternalPaymentCommandResult struct {
	Command db.ExternalPaymentCommand
}

func (svc *PaymentCommandService) RecordExternalPaymentCommand(ctx context.Context, input RecordExternalPaymentCommandInput) (RecordExternalPaymentCommandResult, error) {
	var result RecordExternalPaymentCommandResult

	if err := validateRecordExternalPaymentCommandInput(input); err != nil {
		return result, err
	}

	now := svc.now().UTC()
	submittedAt := now
	if input.SubmittedAt != nil {
		submittedAt = input.SubmittedAt.UTC()
	}

	acceptedAt := input.AcceptedAt
	if acceptedAt == nil && input.CommandStatus == db.ExternalPaymentCommandStatusAccepted {
		acceptedAt = &now
	}

	rejectedAt := input.RejectedAt
	if rejectedAt == nil && input.CommandStatus == db.ExternalPaymentCommandStatusRejected {
		rejectedAt = &now
	}

	responseSnapshot := input.ResponseSnapshot
	if len(responseSnapshot) == 0 {
		responseSnapshot = []byte(`{}`)
	}

	command, err := svc.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
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
		SubmittedAt:          submittedAt,
		AcceptedAt:           timestamptzFromTimePtr(acceptedAt),
		RejectedAt:           timestamptzFromTimePtr(rejectedAt),
		LastErrorCode:        textFromStringPtr(input.LastErrorCode),
		LastErrorMessage:     textFromStringPtr(input.LastErrorMessage),
		RequestFingerprint:   textFromStringPtr(input.RequestFingerprint),
		ResponseSnapshot:     responseSnapshot,
	})
	if err != nil {
		return result, err
	}

	result.Command = command
	return result, nil
}

func validateRecordExternalPaymentCommandInput(input RecordExternalPaymentCommandInput) error {
	if !isExternalPaymentProvider(input.Provider) {
		return fmt.Errorf("unsupported provider %q", input.Provider)
	}
	if !isExternalPaymentChannel(input.Channel) {
		return fmt.Errorf("unsupported channel %q", input.Channel)
	}
	if !isExternalPaymentCapability(input.Capability) {
		return fmt.Errorf("unsupported capability %q", input.Capability)
	}
	if !isExternalPaymentCommandType(input.CommandType) {
		return fmt.Errorf("unsupported command type %q", input.CommandType)
	}
	if !isExternalPaymentBusinessOwner(input.BusinessOwner) {
		return fmt.Errorf("unsupported business owner %q", input.BusinessOwner)
	}
	if (input.BusinessObjectType == nil) != (input.BusinessObjectID == nil) {
		return fmt.Errorf("business object type and id must be provided together")
	}
	if input.BusinessObjectType != nil && strings.TrimSpace(*input.BusinessObjectType) == "" {
		return fmt.Errorf("business object type is required")
	}
	if input.BusinessObjectID != nil && *input.BusinessObjectID == 0 {
		return fmt.Errorf("business object id is required")
	}
	if !isExternalPaymentObjectType(input.ExternalObjectType) {
		return fmt.Errorf("unsupported external object type %q", input.ExternalObjectType)
	}
	if strings.TrimSpace(input.ExternalObjectKey) == "" {
		return fmt.Errorf("external object key is required")
	}
	if !isExternalPaymentCommandStatus(input.CommandStatus) {
		return fmt.Errorf("unsupported command status %q", input.CommandStatus)
	}
	if input.AcceptedAt != nil && input.CommandStatus != db.ExternalPaymentCommandStatusAccepted {
		return fmt.Errorf("accepted at is only valid for accepted command")
	}
	if input.RejectedAt != nil && input.CommandStatus != db.ExternalPaymentCommandStatusRejected {
		return fmt.Errorf("rejected at is only valid for rejected command")
	}
	return nil
}

func isExternalPaymentProvider(provider string) bool {
	switch provider {
	case db.ExternalPaymentProviderWechat:
		return true
	default:
		return false
	}
}

func isExternalPaymentChannel(channel string) bool {
	switch channel {
	case db.PaymentChannelDirect,
		db.PaymentChannelEcommerce,
		db.PaymentChannelOrdinaryServiceProvider:
		return true
	default:
		return false
	}
}

func isExternalPaymentCapability(capability string) bool {
	switch capability {
	case db.ExternalPaymentCapabilityDirectJSAPIPayment,
		db.ExternalPaymentCapabilityDirectRefund,
		db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		db.ExternalPaymentCapabilityPartnerRefund,
		db.ExternalPaymentCapabilityCombinePayment,
		db.ExternalPaymentCapabilityEcommerceRefund,
		db.ExternalPaymentCapabilityProfitSharing,
		db.ExternalPaymentCapabilitySubsidy,
		db.ExternalPaymentCapabilityApplyment,
		db.ExternalPaymentCapabilitySettlement,
		db.ExternalPaymentCapabilityWithdraw,
		db.ExternalPaymentCapabilityCancelWithdraw,
		db.ExternalPaymentCapabilityMerchantTransfer:
		return true
	default:
		return false
	}
}

func isExternalPaymentCommandType(commandType string) bool {
	switch commandType {
	case db.ExternalPaymentCommandTypeCreatePayment,
		db.ExternalPaymentCommandTypeClosePayment,
		db.ExternalPaymentCommandTypeCreateRefund,
		db.ExternalPaymentCommandTypeCreateProfitSharing,
		db.ExternalPaymentCommandTypeCreateProfitSharingReturn,
		db.ExternalPaymentCommandTypeFinishProfitSharing,
		db.ExternalPaymentCommandTypeCreateSubsidy,
		db.ExternalPaymentCommandTypeReturnSubsidy,
		db.ExternalPaymentCommandTypeCancelSubsidy,
		db.ExternalPaymentCommandTypeCreateApplyment,
		db.ExternalPaymentCommandTypeCreateSettlement,
		db.ExternalPaymentCommandTypeCreateWithdraw,
		db.ExternalPaymentCommandTypeCreateCancelWithdraw,
		db.ExternalPaymentCommandTypeCreateTransfer:
		return true
	default:
		return false
	}
}

func isExternalPaymentBusinessOwner(owner string) bool {
	switch owner {
	case db.ExternalPaymentBusinessOwnerRiderDeposit,
		db.ExternalPaymentBusinessOwnerOrder,
		db.ExternalPaymentBusinessOwnerReservation,
		db.ExternalPaymentBusinessOwnerClaimRecovery,
		db.ExternalPaymentBusinessOwnerProfitSharing,
		db.ExternalPaymentBusinessOwnerSubsidy,
		db.ExternalPaymentBusinessOwnerApplyment,
		db.ExternalPaymentBusinessOwnerMerchantFunds:
		return true
	default:
		return false
	}
}

func isExternalPaymentObjectType(objectType string) bool {
	switch objectType {
	case db.ExternalPaymentObjectPayment,
		db.ExternalPaymentObjectCombinedPayment,
		db.ExternalPaymentObjectRefund,
		db.ExternalPaymentObjectProfitSharing,
		db.ExternalPaymentObjectProfitSharingReturn,
		db.ExternalPaymentObjectSubsidy,
		db.ExternalPaymentObjectSubsidyReturn,
		db.ExternalPaymentObjectApplyment,
		db.ExternalPaymentObjectWithdraw,
		db.ExternalPaymentObjectCancelWithdraw,
		db.ExternalPaymentObjectMerchantTransfer,
		db.ExternalPaymentObjectComplaint,
		db.ExternalPaymentObjectViolation,
		db.ExternalPaymentObjectSettlement:
		return true
	default:
		return false
	}
}

func isExternalPaymentCommandStatus(status string) bool {
	switch status {
	case db.ExternalPaymentCommandStatusSubmitted,
		db.ExternalPaymentCommandStatusAccepted,
		db.ExternalPaymentCommandStatusRejected,
		db.ExternalPaymentCommandStatusUnknown:
		return true
	default:
		return false
	}
}
