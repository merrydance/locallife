package baofuevidence

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildWithdrawalEvidencePassesForSucceededLocalRows(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 15, 40, 0, 0, time.UTC)
	summary := BuildWithdrawalEvidence(WithdrawalInput{
		Fact: db.ExternalPaymentFact{
			ID:                   701,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuWithdraw,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			SourceEventType:      pgtype.Text{String: "BAOFU_WITHDRAW", Valid: true},
			ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
			ExternalObjectKey:    "BFWD31O41",
			ExternalSecondaryKey: pgtype.Text{String: "260500007777", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderIncome, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 81, Valid: true},
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Amount:               pgtype.Int8{Int64: 1200, Valid: true},
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
		},
		Order: db.BaofuWithdrawalOrder{
			ID:               81,
			OwnerType:        db.BaofuAccountOwnerTypeRider,
			OwnerID:          901,
			AccountBindingID: 801,
			OutRequestNo:     "BFWD31O41",
			BaofuWithdrawNo:  pgtype.Text{String: "260500007777", Valid: true},
			Amount:           1200,
			Status:           db.BaofuWithdrawalStatusSucceeded,
			FinishedAt:       pgtype.Timestamptz{Time: finishedAt, Valid: true},
		},
		Command: &db.ExternalPaymentCommand{
			ID:                 901,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerRiderIncome,
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectWithdraw,
			ExternalObjectKey:  "BFWD31O41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	})

	require.Equal(t, StatusPass, summary.Status)
	require.Equal(t, int64(81), summary.WithdrawalOrderID)
	require.Equal(t, int64(901), summary.CommandID)
	require.Equal(t, db.BaofuAccountOwnerTypeRider, summary.OwnerType)
	require.Equal(t, db.ExternalPaymentBusinessOwnerRiderIncome, summary.BusinessOwner)
	require.Equal(t, int64(1200), summary.AmountFen)
	require.Equal(t, db.ExternalPaymentFactProcessingStatusReceived, summary.FactProcessingStatus)
	require.Equal(t, "BFWD***1O41", summary.OutRequestNoMasked)
	require.Equal(t, "2605***7777", summary.BaofuWithdrawNoMasked)
	require.Empty(t, summary.Findings)
}

func TestBuildWithdrawalEvidenceFailsOnIncompleteConvergence(t *testing.T) {
	summary := BuildWithdrawalEvidence(WithdrawalInput{
		Fact: db.ExternalPaymentFact{
			ID:                 701,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			FactSource:         db.ExternalPaymentFactSourceCommandResponse,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusFailed,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 1100, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		},
		Order: db.BaofuWithdrawalOrder{
			ID:           81,
			OwnerType:    "unknown",
			OwnerID:      901,
			OutRequestNo: "BFWD31O41",
			Amount:       1200,
			Status:       db.BaofuWithdrawalStatusProcessing,
		},
		Command: &db.ExternalPaymentCommand{
			ID:            901,
			CommandStatus: db.ExternalPaymentCommandStatusSubmitted,
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "withdrawal fact source is not callback/query/manual_reconciliation")
	require.Contains(t, summary.Findings, "withdrawal fact terminal status is not success")
	require.Contains(t, summary.Findings, "withdrawal fact business owner is not a withdrawal funds owner")
	require.Contains(t, summary.Findings, "withdrawal fact business object is not baofu_withdrawal_order")
	require.Contains(t, summary.Findings, "withdrawal fact external object key is missing")
	require.Contains(t, summary.Findings, "withdrawal fact amount does not match withdrawal order amount")
	require.Contains(t, summary.Findings, "withdrawal order owner type is not supported")
	require.Contains(t, summary.Findings, "withdrawal order is not succeeded")
	require.Contains(t, summary.Findings, "withdrawal order is not finished_at stamped")
	require.Contains(t, summary.Findings, "withdrawal command is not accepted")
}

func TestBuildWithdrawalEvidenceRequiresCommandProof(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 15, 40, 0, 0, time.UTC)
	summary := BuildWithdrawalEvidence(WithdrawalInput{
		Fact: db.ExternalPaymentFact{
			ID:                 701,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			FactSource:         db.ExternalPaymentFactSourceQuery,
			ExternalObjectType: db.ExternalPaymentObjectWithdraw,
			ExternalObjectKey:  "BFWD31O41",
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 1200, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		},
		Order: db.BaofuWithdrawalOrder{
			ID:           81,
			OwnerType:    db.BaofuAccountOwnerTypeMerchant,
			OwnerID:      701,
			OutRequestNo: "BFWD31O41",
			Amount:       1200,
			Status:       db.BaofuWithdrawalStatusSucceeded,
			FinishedAt:   pgtype.Timestamptz{Time: finishedAt, Valid: true},
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "withdrawal command is missing")
}

func TestBuildWithdrawalEvidenceRequiresExternalObjectAndProviderKeys(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 15, 40, 0, 0, time.UTC)
	summary := BuildWithdrawalEvidence(WithdrawalInput{
		Fact: db.ExternalPaymentFact{
			ID:                 701,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			FactSource:         db.ExternalPaymentFactSourceCallback,
			ExternalObjectType: db.ExternalPaymentObjectWithdraw,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerPlatformFunds, Valid: true},
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 1200, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		},
		Order: db.BaofuWithdrawalOrder{
			ID:         81,
			OwnerType:  db.BaofuAccountOwnerTypePlatform,
			OwnerID:    1,
			Amount:     1200,
			Status:     db.BaofuWithdrawalStatusSucceeded,
			FinishedAt: pgtype.Timestamptz{Time: finishedAt, Valid: true},
		},
		Command: &db.ExternalPaymentCommand{
			ID:                 901,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerPlatformFunds,
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectWithdraw,
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "withdrawal order out_request_no is missing")
	require.Contains(t, summary.Findings, "withdrawal fact external object key is missing")
	require.Contains(t, summary.Findings, "withdrawal order baofu_withdraw_no is missing")
	require.Contains(t, summary.Findings, "withdrawal command external object key is missing")
}
