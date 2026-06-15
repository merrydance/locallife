package baofuevidence

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildProfitSharingEvidencePassesForAppliedShareRows(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 12, 10, 0, 0, time.UTC)
	summary := BuildProfitSharingEvidence(ProfitSharingInput{
		Fact: db.ExternalPaymentFact{
			ID:                   101,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			SourceEventType:      pgtype.Text{String: "SHARING", Valid: true},
			ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
			ExternalObjectKey:    "BFPS31O41",
			ExternalSecondaryKey: pgtype.Text{String: "260500009999", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "profit_sharing_order", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 61, Valid: true},
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Amount:               pgtype.Int8{Int64: 8900, Valid: true},
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		Order: db.ProfitSharingOrder{
			ID:                     61,
			PaymentOrderID:         31,
			Provider:               db.ExternalPaymentProviderBaofu,
			Channel:                db.PaymentChannelBaofuAggregate,
			OutOrderNo:             "BFPS31O41",
			SharingOrderID:         pgtype.Text{String: "260500009999", Valid: true},
			Status:                 db.ProfitSharingOrderStatusFinished,
			FinishedAt:             pgtype.Timestamptz{Time: finishedAt, Valid: true},
			MerchantAmount:         8000,
			RiderAmount:            500,
			OperatorCommission:     300,
			PlatformReceiverAmount: 100,
			CalculationVersion:     "baofu_fee_v2",
			MerchantSharingMerID:   pgtype.Text{String: "CP61000000002938", Valid: true},
		},
		Command: &db.ExternalPaymentCommand{
			ID:                 301,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			CommandType:        db.ExternalPaymentCommandTypeCreateProfitSharing,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerProfitSharing,
			BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 61, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
			ExternalObjectKey:  "BFPS31O41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	})

	require.Equal(t, StatusPass, summary.Status)
	require.Equal(t, int64(61), summary.ProfitSharingOrderID)
	require.Equal(t, int64(301), summary.CommandID)
	require.Equal(t, int64(8900), summary.AmountFen)
	require.Equal(t, "BFPS***1O41", summary.OutOrderNoMasked)
	require.Equal(t, "2605***9999", summary.TradeNoMasked)
	require.Equal(t, "CP61***2938", summary.MerchantSharingMerIDMasked)
	require.Empty(t, summary.Findings)
}

func TestBuildProfitSharingEvidenceFailsOnIncompleteConvergence(t *testing.T) {
	summary := BuildProfitSharingEvidence(ProfitSharingInput{
		Fact: db.ExternalPaymentFact{
			ID:                 101,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:         db.ExternalPaymentFactSourceCommandResponse,
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 61, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 7700, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusFailed,
		},
		Order: db.ProfitSharingOrder{
			ID:                 61,
			PaymentOrderID:     31,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			OutOrderNo:         "BFPS31O41",
			Status:             db.ProfitSharingOrderStatusProcessing,
			MerchantAmount:     8000,
			RiderAmount:        500,
			OperatorCommission: 300,
			PlatformCommission: 200,
		},
		Command: &db.ExternalPaymentCommand{
			ID:            301,
			CommandStatus: db.ExternalPaymentCommandStatusSubmitted,
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "profit sharing fact source is not callback/query/manual_reconciliation")
	require.Contains(t, summary.Findings, "profit sharing fact terminal status is not success")
	require.Contains(t, summary.Findings, "profit sharing fact is not terminalized")
	require.Contains(t, summary.Findings, "profit sharing fact business object is not profit_sharing_order")
	require.Contains(t, summary.Findings, "profit sharing application is not applied")
	require.Contains(t, summary.Findings, "profit sharing order is not finished")
	require.Contains(t, summary.Findings, "profit sharing order is not finished_at stamped")
	require.Contains(t, summary.Findings, "profit sharing command is not accepted")
	require.Contains(t, summary.Findings, "profit sharing fact amount does not match expected share amount")
}

func TestBuildProfitSharingEvidenceRequiresCommandProof(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 12, 10, 0, 0, time.UTC)
	summary := BuildProfitSharingEvidence(ProfitSharingInput{
		Fact: db.ExternalPaymentFact{
			ID:                 101,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:         db.ExternalPaymentFactSourceQuery,
			ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
			ExternalObjectKey:  "BFPS31O41",
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
			BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 61, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8900, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		Order: db.ProfitSharingOrder{
			ID:                     61,
			PaymentOrderID:         31,
			Provider:               db.ExternalPaymentProviderBaofu,
			Channel:                db.PaymentChannelBaofuAggregate,
			OutOrderNo:             "BFPS31O41",
			Status:                 db.ProfitSharingOrderStatusFinished,
			FinishedAt:             pgtype.Timestamptz{Time: finishedAt, Valid: true},
			MerchantAmount:         8000,
			RiderAmount:            500,
			OperatorCommission:     300,
			PlatformReceiverAmount: 100,
			CalculationVersion:     "baofu_fee_v2",
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "profit sharing command is missing")
}

func TestBuildProfitSharingEvidenceRequiresExternalObjectKeys(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 12, 10, 0, 0, time.UTC)
	summary := BuildProfitSharingEvidence(ProfitSharingInput{
		Fact: db.ExternalPaymentFact{
			ID:                 101,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:         db.ExternalPaymentFactSourceCallback,
			ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
			BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 61, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8900, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		Order: db.ProfitSharingOrder{
			ID:                     61,
			PaymentOrderID:         31,
			Provider:               db.ExternalPaymentProviderBaofu,
			Channel:                db.PaymentChannelBaofuAggregate,
			Status:                 db.ProfitSharingOrderStatusFinished,
			FinishedAt:             pgtype.Timestamptz{Time: finishedAt, Valid: true},
			MerchantAmount:         8000,
			RiderAmount:            500,
			OperatorCommission:     300,
			PlatformReceiverAmount: 100,
			CalculationVersion:     "baofu_fee_v2",
		},
		Command: &db.ExternalPaymentCommand{
			ID:                 301,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			CommandType:        db.ExternalPaymentCommandTypeCreateProfitSharing,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerProfitSharing,
			BusinessObjectType: pgtype.Text{String: "profit_sharing_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 61, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "profit sharing order out_order_no is missing")
	require.Contains(t, summary.Findings, "profit sharing fact external object key is missing")
	require.Contains(t, summary.Findings, "profit sharing command external object key is missing")
}
