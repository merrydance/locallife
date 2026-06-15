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

func TestLoadRefundEvidenceUsesExplicitRows(t *testing.T) {
	refundedAt := time.Date(2026, 6, 15, 14, 20, 0, 0, time.UTC)
	reader := &fakeRefundEvidenceReader{
		fact: db.ExternalPaymentFact{
			ID:                 401,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			FactSource:         db.ExternalPaymentFactSourceCallback,
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31O41",
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		refundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundAmount:   8800,
			OutRefundNo:    "BFRF31O41",
			Status:         "success",
			RefundedAt:     pgtype.Timestamptz{Time: refundedAt, Valid: true},
		},
		paymentOrder: db.PaymentOrder{
			ID:             31,
			OrderID:        pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			OutTradeNo:     "BFPAY31O41",
			Status:         "refunded",
		},
		command: db.ExternalPaymentCommand{
			ID:                 601,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			CommandType:        db.ExternalPaymentCommandTypeCreateRefund,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerOrder,
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31O41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	}

	summary, err := loadRefundEvidence(context.Background(), reader, 401, 501, 71, 31, 601)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(601), summary.CommandID)
	require.Equal(t, []int64{401}, reader.factIDs)
	require.Equal(t, []int64{501}, reader.applicationIDs)
	require.Equal(t, []int64{71}, reader.refundOrderIDs)
	require.Equal(t, []int64{31}, reader.paymentOrderIDs)
	require.Equal(t, []int64{601}, reader.commandIDs)
	require.Empty(t, reader.commandLookupParams)
}

func TestLoadRefundEvidenceFindsCommandByExternalObject(t *testing.T) {
	reader := &fakeRefundEvidenceReader{
		fact: db.ExternalPaymentFact{
			ID:                 401,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			FactSource:         db.ExternalPaymentFactSourceQuery,
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31O41",
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		refundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundAmount:   8800,
			OutRefundNo:    "BFRF31O41",
			Status:         "success",
			RefundedAt:     pgtype.Timestamptz{Time: time.Date(2026, 6, 15, 14, 20, 0, 0, time.UTC), Valid: true},
		},
		paymentOrder: db.PaymentOrder{
			ID:             31,
			OrderID:        pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			Status:         "refunded",
		},
		command: db.ExternalPaymentCommand{
			ID:                 601,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			CommandType:        db.ExternalPaymentCommandTypeCreateRefund,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerOrder,
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31O41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	}

	summary, err := loadRefundEvidence(context.Background(), reader, 401, 501, 71, 31, 0)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, int64(601), summary.CommandID)
	require.Empty(t, reader.commandIDs)
	require.Equal(t, []db.GetExternalPaymentCommandByExternalObjectParams{{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:        db.ExternalPaymentCommandTypeCreateRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		ExternalObjectKey:  "BFRF31O41",
	}}, reader.commandLookupParams)
}

func TestRenderRefundCommandOutputKeepsSummaryShape(t *testing.T) {
	output, exitCode, err := renderCommandOutput(baofuevidence.RefundSummary{
		Status: baofuevidence.StatusPass,
		FactID: 401,
	}, commandOutputOptions{})

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Contains(t, output, "\"status\": \"pass\"")
	require.NotContains(t, output, "\"summary\"")
}

func TestRenderRefundCommandOutputIncludesLedgerRowWhenRequested(t *testing.T) {
	output, exitCode, err := renderCommandOutput(baofuevidence.RefundSummary{
		Status:             baofuevidence.StatusPass,
		FactID:             401,
		ApplicationID:      501,
		RefundOrderID:      71,
		PaymentOrderID:     31,
		OrderID:            41,
		CommandID:          601,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		SourceEventType:    "REFUND",
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:  db.ExternalPaymentFactApplicationStatusApplied,
		RefundOrderStatus:  "success",
		PaymentOrderStatus: "refunded",
		OutRefundNoMasked:  "BFRF***1O41",
	}, commandOutputOptions{
		LedgerRow: true,
		LedgerContext: baofuevidence.EvidenceLedgerRowContext{
			Date:     "2026-06-15",
			Env:      "production",
			Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/refund",
			ACK:      "OK",
			Commit:   "7c325e4d",
			Notes:    "controlled refund callback",
		},
	})

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Contains(t, output, "\"status\": \"pass\"")
	require.Contains(t, output, "\"section\": \"Refund Callback\"")
	require.Contains(t, output, "| 2026-06-15 | production |")
}

func TestRenderRefundCommandOutputRejectsLedgerRowMissingContext(t *testing.T) {
	_, _, err := renderCommandOutput(baofuevidence.RefundSummary{
		Status:            baofuevidence.StatusPass,
		FactID:            401,
		ApplicationID:     501,
		FactSource:        db.ExternalPaymentFactSourceCallback,
		TerminalStatus:    db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus: db.ExternalPaymentFactApplicationStatusApplied,
		RefundOrderStatus: "success",
		OutRefundNoMasked: "BFRF***1O41",
	}, commandOutputOptions{
		LedgerRow: true,
		LedgerContext: baofuevidence.EvidenceLedgerRowContext{
			Date:     "2026-06-15",
			Env:      "production",
			Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/refund",
			Commit:   "7c325e4d",
			Notes:    "missing ack",
		},
	})

	require.ErrorContains(t, err, "callback ack is required")
}

func TestRefundEvidenceExitCodeFailsOnFindings(t *testing.T) {
	_, exitCode, err := renderCommandOutput(baofuevidence.RefundSummary{
		Status:   baofuevidence.StatusFail,
		FactID:   401,
		Findings: []string{"refund order is not success"},
	}, commandOutputOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, exitCode)
}

type fakeRefundEvidenceReader struct {
	fact                db.ExternalPaymentFact
	application         db.ExternalPaymentFactApplication
	refundOrder         db.RefundOrder
	paymentOrder        db.PaymentOrder
	command             db.ExternalPaymentCommand
	factIDs             []int64
	applicationIDs      []int64
	refundOrderIDs      []int64
	paymentOrderIDs     []int64
	commandIDs          []int64
	commandLookupParams []db.GetExternalPaymentCommandByExternalObjectParams
}

func (reader *fakeRefundEvidenceReader) GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error) {
	reader.factIDs = append(reader.factIDs, id)
	return reader.fact, nil
}

func (reader *fakeRefundEvidenceReader) GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error) {
	reader.applicationIDs = append(reader.applicationIDs, id)
	return reader.application, nil
}

func (reader *fakeRefundEvidenceReader) GetRefundOrder(ctx context.Context, id int64) (db.RefundOrder, error) {
	reader.refundOrderIDs = append(reader.refundOrderIDs, id)
	return reader.refundOrder, nil
}

func (reader *fakeRefundEvidenceReader) GetPaymentOrder(ctx context.Context, id int64) (db.PaymentOrder, error) {
	reader.paymentOrderIDs = append(reader.paymentOrderIDs, id)
	return reader.paymentOrder, nil
}

func (reader *fakeRefundEvidenceReader) GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error) {
	reader.commandIDs = append(reader.commandIDs, id)
	return reader.command, nil
}

func (reader *fakeRefundEvidenceReader) GetExternalPaymentCommandByExternalObject(ctx context.Context, params db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error) {
	reader.commandLookupParams = append(reader.commandLookupParams, params)
	return reader.command, nil
}
