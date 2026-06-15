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

func TestLoadWithdrawalEvidenceUsesExplicitRows(t *testing.T) {
	finishedAt := time.Date(2026, 6, 15, 15, 40, 0, 0, time.UTC)
	reader := &fakeWithdrawalEvidenceReader{
		fact: db.ExternalPaymentFact{
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
		withdrawalOrder: db.BaofuWithdrawalOrder{
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
		command: db.ExternalPaymentCommand{
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
	}

	summary, err := loadWithdrawalEvidence(context.Background(), reader, 701, 81, 901)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(901), summary.CommandID)
	require.Equal(t, []int64{701}, reader.factIDs)
	require.Equal(t, []int64{81}, reader.withdrawalOrderIDs)
	require.Equal(t, []int64{901}, reader.commandIDs)
	require.Empty(t, reader.commandLookupParams)
}

func TestLoadWithdrawalEvidenceFindsCommandByExternalObject(t *testing.T) {
	reader := &fakeWithdrawalEvidenceReader{
		fact: db.ExternalPaymentFact{
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
		withdrawalOrder: db.BaofuWithdrawalOrder{
			ID:               81,
			OwnerType:        db.BaofuAccountOwnerTypeMerchant,
			OwnerID:          701,
			AccountBindingID: 801,
			OutRequestNo:     "BFWD31O41",
			BaofuWithdrawNo:  pgtype.Text{String: "260500007777", Valid: true},
			Amount:           1200,
			Status:           db.BaofuWithdrawalStatusSucceeded,
			FinishedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 15, 15, 40, 0, 0, time.UTC), Valid: true},
		},
		command: db.ExternalPaymentCommand{
			ID:                 901,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
			CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerMerchantFunds,
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 81, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectWithdraw,
			ExternalObjectKey:  "BFWD31O41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	}

	summary, err := loadWithdrawalEvidence(context.Background(), reader, 701, 81, 0)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(901), summary.CommandID)
	require.Empty(t, reader.commandIDs)
	require.Equal(t, []db.GetExternalPaymentCommandByExternalObjectParams{{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
		ExternalObjectType: db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  "BFWD31O41",
	}}, reader.commandLookupParams)
}

func TestRenderWithdrawalCommandOutputKeepsSummaryShape(t *testing.T) {
	output, exitCode, err := renderCommandOutput(baofuevidence.WithdrawalSummary{
		Status: baofuevidence.StatusPass,
		FactID: 701,
	})

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Contains(t, output, "\"status\": \"pass\"")
	require.NotContains(t, output, "\"summary\"")
}

func TestWithdrawalEvidenceExitCodeFailsOnFindings(t *testing.T) {
	_, exitCode, err := renderCommandOutput(baofuevidence.WithdrawalSummary{
		Status:   baofuevidence.StatusFail,
		FactID:   701,
		Findings: []string{"withdrawal order is not succeeded"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, exitCode)
}

type fakeWithdrawalEvidenceReader struct {
	fact                db.ExternalPaymentFact
	withdrawalOrder     db.BaofuWithdrawalOrder
	command             db.ExternalPaymentCommand
	factIDs             []int64
	withdrawalOrderIDs  []int64
	commandIDs          []int64
	commandLookupParams []db.GetExternalPaymentCommandByExternalObjectParams
}

func (reader *fakeWithdrawalEvidenceReader) GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error) {
	reader.factIDs = append(reader.factIDs, id)
	return reader.fact, nil
}

func (reader *fakeWithdrawalEvidenceReader) GetBaofuWithdrawalOrder(ctx context.Context, id int64) (db.BaofuWithdrawalOrder, error) {
	reader.withdrawalOrderIDs = append(reader.withdrawalOrderIDs, id)
	return reader.withdrawalOrder, nil
}

func (reader *fakeWithdrawalEvidenceReader) GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error) {
	reader.commandIDs = append(reader.commandIDs, id)
	return reader.command, nil
}

func (reader *fakeWithdrawalEvidenceReader) GetExternalPaymentCommandByExternalObject(ctx context.Context, params db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error) {
	reader.commandLookupParams = append(reader.commandLookupParams, params)
	return reader.command, nil
}
