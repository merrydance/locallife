package baofuevidence

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildRefundEvidencePassesForAppliedBaofuRefundRows(t *testing.T) {
	refundedAt := time.Date(2026, 6, 15, 14, 20, 0, 0, time.UTC)
	summary := BuildRefundEvidence(RefundInput{
		Fact: db.ExternalPaymentFact{
			ID:                   401,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuRefund,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			SourceEventType:      pgtype.Text{String: "REFUND", Valid: true},
			ExternalObjectType:   db.ExternalPaymentObjectRefund,
			ExternalObjectKey:    "BFRF31O41",
			ExternalSecondaryKey: pgtype.Text{String: "260500008888", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 71, Valid: true},
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Amount:               pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		RefundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundType:     "full",
			RefundAmount:   8800,
			OutRefundNo:    "BFRF31O41",
			RefundID:       pgtype.Text{String: "260500008888", Valid: true},
			Status:         refundOrderStatusSuccess,
			RefundedAt:     pgtype.Timestamptz{Time: refundedAt, Valid: true},
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			OrderID:        pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			OutTradeNo:     "BFPAY31O41",
			TransactionID:  pgtype.Text{String: "260500001234", Valid: true},
			Status:         "refunded",
		},
		Command: &db.ExternalPaymentCommand{
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
	})

	require.Equal(t, StatusPass, summary.Status)
	require.Equal(t, int64(71), summary.RefundOrderID)
	require.Equal(t, int64(31), summary.PaymentOrderID)
	require.Equal(t, int64(601), summary.CommandID)
	require.Equal(t, int64(8800), summary.AmountFen)
	require.Equal(t, "BFRF***1O41", summary.OutRefundNoMasked)
	require.Equal(t, "2605***8888", summary.RefundIDMasked)
	require.Equal(t, "BFPA***1O41", summary.PaymentOutTradeNoMasked)
	require.Empty(t, summary.Findings)
}

func TestBuildRefundEvidencePassesForReservationOwner(t *testing.T) {
	refundedAt := time.Date(2026, 6, 15, 14, 20, 0, 0, time.UTC)
	summary := BuildRefundEvidence(RefundInput{
		Fact: db.ExternalPaymentFact{
			ID:                 401,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			FactSource:         db.ExternalPaymentFactSourceQuery,
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31R41",
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 6600, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		RefundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundAmount:   6600,
			OutRefundNo:    "BFRF31R41",
			Status:         refundOrderStatusSuccess,
			RefundedAt:     pgtype.Timestamptz{Time: refundedAt, Valid: true},
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			ReservationID:  pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerReservation,
			Amount:         6600,
			OutTradeNo:     "BFPAY31R41",
			Status:         "refunded",
		},
		Command: &db.ExternalPaymentCommand{
			ID:                 601,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			CommandType:        db.ExternalPaymentCommandTypeCreateRefund,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerReservation,
			BusinessObjectType: pgtype.Text{String: "refund_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			ExternalObjectType: db.ExternalPaymentObjectRefund,
			ExternalObjectKey:  "BFRF31R41",
			CommandStatus:      db.ExternalPaymentCommandStatusAccepted,
		},
	})

	require.Equal(t, StatusPass, summary.Status)
	require.Empty(t, summary.Findings)
}

func TestBuildRefundEvidenceFailsOnIncompleteConvergence(t *testing.T) {
	summary := BuildRefundEvidence(RefundInput{
		Fact: db.ExternalPaymentFact{
			ID:                 401,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuRefund,
			FactSource:         db.ExternalPaymentFactSourceCommandResponse,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerRiderDeposit, Valid: true},
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 71, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusFailed,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 7700, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusReceived,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusFailed,
		},
		RefundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundAmount:   8800,
			Status:         "processing",
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			PaymentChannel: db.PaymentChannelDirect,
			BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
			Amount:         8800,
			Status:         "paid",
		},
		Command: &db.ExternalPaymentCommand{
			ID:            601,
			CommandStatus: db.ExternalPaymentCommandStatusSubmitted,
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "refund fact source is not callback/query/manual_reconciliation")
	require.Contains(t, summary.Findings, "refund fact terminal status is not success")
	require.Contains(t, summary.Findings, "refund fact is not terminalized")
	require.Contains(t, summary.Findings, "refund fact business owner is not order/reservation")
	require.Contains(t, summary.Findings, "refund fact business object is not refund_order")
	require.Contains(t, summary.Findings, "refund fact external object key is missing")
	require.Contains(t, summary.Findings, "refund application is not applied")
	require.Contains(t, summary.Findings, "refund order is not success")
	require.Contains(t, summary.Findings, "refund order is not refunded_at stamped")
	require.Contains(t, summary.Findings, "refund order out_refund_no is missing")
	require.Contains(t, summary.Findings, "payment order channel is not baofu_aggregate")
	require.Contains(t, summary.Findings, "payment order business type is not order/reservation")
	require.Contains(t, summary.Findings, "refund command is not accepted")
	require.Contains(t, summary.Findings, "refund fact amount does not match refund order amount")
}

func TestBuildRefundEvidenceRequiresCommandProof(t *testing.T) {
	refundedAt := time.Date(2026, 6, 15, 14, 20, 0, 0, time.UTC)
	summary := BuildRefundEvidence(RefundInput{
		Fact: db.ExternalPaymentFact{
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
		Application: db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			BusinessObjectType: "refund_order",
			BusinessObjectID:   71,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		RefundOrder: db.RefundOrder{
			ID:             71,
			PaymentOrderID: 31,
			RefundAmount:   8800,
			OutRefundNo:    "BFRF31O41",
			Status:         refundOrderStatusSuccess,
			RefundedAt:     pgtype.Timestamptz{Time: refundedAt, Valid: true},
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			OrderID:        pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			Status:         "refunded",
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "refund command is missing")
}
