package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	db "github.com/merrydance/locallife/db/sqlc"
)

func (server *Server) recordBaofuWithdrawCallbackFact(ctx context.Context, notification *baofunotification.WithdrawNotification, withdrawalOrder db.BaofuWithdrawalOrder) (db.ExternalPaymentFact, error) {
	if notification == nil {
		return db.ExternalPaymentFact{}, fmt.Errorf("baofu withdraw callback notification is required")
	}
	outRequestNo := strings.TrimSpace(notification.TransSerialNo)
	if outRequestNo == "" {
		return db.ExternalPaymentFact{}, fmt.Errorf("baofu withdraw callback trans serial no is required")
	}
	upstreamState := strings.TrimSpace(notification.UpstreamState)
	terminalStatus, isTerminal := baofuWithdrawalTerminalStatus(upstreamState)
	occurredAt := notification.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	createParams := db.CreateExternalPaymentFactParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuWithdraw,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        pgtype.Text{String: outRequestNo, Valid: true},
		SourceEventType:      pgtype.Text{String: "BAOFU_WITHDRAW", Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: baofuText(strings.TrimSpace(notification.BaofuWithdrawNo)),
		BusinessOwner:        pgtype.Text{String: businessOwnerForBaofuWithdrawalFact(withdrawalOrder.OwnerType), Valid: true},
		BusinessObjectType:   pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: withdrawalOrder.ID, Valid: withdrawalOrder.ID > 0},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           isTerminal,
		Amount:               pgtype.Int8{Int64: notification.AmountFen, Valid: notification.AmountFen > 0},
		Currency:             "CNY",
		OccurredAt:           pgtype.Timestamptz{Time: occurredAt, Valid: true},
		UpstreamUpdatedAt:    pgtype.Timestamptz{Time: occurredAt, Valid: true},
		ObservedAt:           time.Now().UTC(),
		RawResource:          notification.Raw,
		DedupeKey:            baofuWithdrawCallbackDedupeKey(notification),
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	}
	return server.store.CreateExternalPaymentFact(ctx, createParams)
}

func baofuWithdrawalTerminalStatus(upstreamState string) (string, bool) {
	switch baofucontracts.WithdrawStatusFromUpstream(upstreamState) {
	case db.BaofuWithdrawalStatusSucceeded:
		return db.ExternalPaymentTerminalStatusSuccess, true
	case db.BaofuWithdrawalStatusFailed, db.BaofuWithdrawalStatusReturned:
		return db.ExternalPaymentTerminalStatusFailed, true
	default:
		return db.ExternalPaymentTerminalStatusProcessing, false
	}
}

func baofuWithdrawCallbackDedupeKey(notification *baofunotification.WithdrawNotification) string {
	outRequestNo := strings.TrimSpace(notification.TransSerialNo)
	sourceEventID := strings.TrimSpace(notification.BaofuWithdrawNo)
	if sourceEventID == "" {
		sourceEventID = strings.TrimSpace(notification.UpstreamState)
	}
	return fmt.Sprintf("baofu:callback:withdraw:%s:%s", outRequestNo, sourceEventID)
}

func businessOwnerForBaofuWithdrawalFact(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	case db.BaofuAccountOwnerTypeRider:
		return db.ExternalPaymentBusinessOwnerRiderIncome
	case db.BaofuAccountOwnerTypeOperator:
		return db.ExternalPaymentBusinessOwnerOperatorFunds
	case db.BaofuAccountOwnerTypePlatform:
		return db.ExternalPaymentBusinessOwnerPlatformFunds
	default:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	}
}
