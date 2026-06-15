package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/internal/baofuevidence"
	"github.com/stretchr/testify/require"
)

func TestLoadProfitSharingEvidenceUsesExplicitRows(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 12, 10, 0, 0, time.UTC)
	reader := &fakeProfitSharingEvidenceReader{
		fact: db.ExternalPaymentFact{
			ID:                 101,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:         db.ExternalPaymentFactSourceCallback,
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
		application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		order: db.ProfitSharingOrder{
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
		command: db.ExternalPaymentCommand{
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
	}

	summary, err := loadProfitSharingEvidence(context.Background(), reader, 101, 201, 61, 301)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(301), summary.CommandID)
	require.Equal(t, []int64{101}, reader.factIDs)
	require.Equal(t, []int64{201}, reader.applicationIDs)
	require.Equal(t, []int64{61}, reader.profitSharingOrderIDs)
	require.Equal(t, []int64{301}, reader.commandIDs)
	require.Empty(t, reader.commandLookupParams)
}

func TestLoadProfitSharingEvidenceFindsCommandByExternalObject(t *testing.T) {
	reader := &fakeProfitSharingEvidenceReader{
		fact: db.ExternalPaymentFact{
			ID:                 101,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
			FactSource:         db.ExternalPaymentFactSourceManualReconciliation,
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
		application: db.ExternalPaymentFactApplication{
			ID:                 201,
			FactID:             101,
			BusinessObjectType: "profit_sharing_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		order: db.ProfitSharingOrder{
			ID:                 61,
			PaymentOrderID:     31,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			OutOrderNo:         "BFPS31O41",
			Status:             db.ProfitSharingOrderStatusFinished,
			FinishedAt:         pgtype.Timestamptz{Time: time.Date(2026, 6, 15, 12, 10, 0, 0, time.UTC), Valid: true},
			MerchantAmount:     8000,
			RiderAmount:        500,
			OperatorCommission: 300,
			PlatformCommission: 100,
		},
		command: db.ExternalPaymentCommand{
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
	}

	summary, err := loadProfitSharingEvidence(context.Background(), reader, 101, 201, 61, 0)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(301), summary.CommandID)
	require.Empty(t, reader.commandIDs)
	require.Equal(t, []db.GetExternalPaymentCommandByExternalObjectParams{{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
		CommandType:        db.ExternalPaymentCommandTypeCreateProfitSharing,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  "BFPS31O41",
	}}, reader.commandLookupParams)
}

func TestRenderProfitSharingCommandOutputKeepsSummaryShape(t *testing.T) {
	output, exitCode, err := renderCommandOutput(baofuevidence.ProfitSharingSummary{
		Status: baofuevidence.StatusPass,
		FactID: 101,
	})

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Contains(t, output, "\"status\": \"pass\"")
	require.NotContains(t, output, "\"summary\"")
}

func TestProfitSharingEvidenceExitCodeFailsOnFindings(t *testing.T) {
	_, exitCode, err := renderCommandOutput(baofuevidence.ProfitSharingSummary{
		Status:   baofuevidence.StatusFail,
		FactID:   101,
		Findings: []string{"profit sharing order is not finished"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, exitCode)
}

type fakeProfitSharingEvidenceReader struct {
	fact                  db.ExternalPaymentFact
	application           db.ExternalPaymentFactApplication
	order                 db.ProfitSharingOrder
	command               db.ExternalPaymentCommand
	factIDs               []int64
	applicationIDs        []int64
	profitSharingOrderIDs []int64
	commandIDs            []int64
	commandLookupParams   []db.GetExternalPaymentCommandByExternalObjectParams
}

func (reader *fakeProfitSharingEvidenceReader) GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error) {
	reader.factIDs = append(reader.factIDs, id)
	return reader.fact, nil
}

func (reader *fakeProfitSharingEvidenceReader) GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error) {
	reader.applicationIDs = append(reader.applicationIDs, id)
	return reader.application, nil
}

func (reader *fakeProfitSharingEvidenceReader) GetProfitSharingOrder(ctx context.Context, id int64) (db.ProfitSharingOrder, error) {
	reader.profitSharingOrderIDs = append(reader.profitSharingOrderIDs, id)
	return reader.order, nil
}

func (reader *fakeProfitSharingEvidenceReader) GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error) {
	reader.commandIDs = append(reader.commandIDs, id)
	return reader.command, nil
}

func (reader *fakeProfitSharingEvidenceReader) GetExternalPaymentCommandByExternalObject(ctx context.Context, params db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error) {
	reader.commandLookupParams = append(reader.commandLookupParams, params)
	return reader.command, nil
}
