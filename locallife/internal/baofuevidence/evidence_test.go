package baofuevidence

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildAggregatePaymentEvidencePassesForAppliedLocalRows(t *testing.T) {
	paidAt := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)
	input := AggregatePaymentInput{
		Fact: db.ExternalPaymentFact{
			ID:                11,
			Provider:          db.ExternalPaymentProviderBaofu,
			Channel:           db.PaymentChannelBaofuAggregate,
			Capability:        db.ExternalPaymentCapabilityBaofuPayment,
			FactSource:        db.ExternalPaymentFactSourceCallback,
			SourceEventType:   pgtype.Text{String: "PAYMENT", Valid: true},
			ExternalObjectKey: "BAOFU_OUT_TRADE_1234567890",
			ExternalSecondaryKey: pgtype.Text{
				String: "260500001234567890",
				Valid:  true,
			},
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 31, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			Consumer:           "order_domain",
			BusinessObjectType: "payment_order",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		PaymentOrder: db.PaymentOrder{
			ID:                    31,
			OrderID:               pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel:        db.PaymentChannelBaofuAggregate,
			BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
			Amount:                8800,
			OutTradeNo:            "BAOFU_OUT_TRADE_1234567890",
			TransactionID:         pgtype.Text{String: "260500001234567890", Valid: true},
			Status:                "paid",
			PaidAt:                pgtype.Timestamptz{Time: paidAt, Valid: true},
			ProcessedAt:           pgtype.Timestamptz{Time: paidAt.Add(time.Second), Valid: true},
			RequiresProfitSharing: true,
		},
		Outbox: &db.PaymentDomainOutbox{
			ID:            51,
			EventType:     db.PaymentDomainOutboxEventOrderPaymentSucceeded,
			AggregateType: db.PaymentDomainOutboxAggregatePaymentOrder,
			AggregateID:   31,
			Status:        db.PaymentDomainOutboxStatusPending,
		},
		ProfitSharingOrder: &db.ProfitSharingOrder{
			ID:                   61,
			PaymentOrderID:       31,
			OutOrderNo:           "BFPS31O41",
			Status:               db.ProfitSharingOrderStatusPending,
			MerchantSharingMerID: pgtype.Text{String: "CP61000000002938", Valid: true},
		},
	}

	summary := BuildAggregatePaymentEvidence(input)

	require.Equal(t, StatusPass, summary.Status)
	require.Equal(t, int64(11), summary.FactID)
	require.Equal(t, int64(21), summary.ApplicationID)
	require.Equal(t, int64(31), summary.PaymentOrderID)
	require.Equal(t, int64(51), summary.OutboxID)
	require.Equal(t, int64(61), summary.ProfitSharingOrderID)
	require.Equal(t, db.ExternalPaymentFactSourceCallback, summary.FactSource)
	require.Equal(t, "PAYMENT", summary.SourceEventType)
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, summary.TerminalStatus)
	require.Equal(t, "BAOF***7890", summary.OutTradeNoMasked)
	require.Equal(t, "2605***7890", summary.TradeNoMasked)
	require.Equal(t, "CP61***2938", summary.MerchantSharingMerIDMasked)
	require.Empty(t, summary.Findings)
}

func TestBuildAggregatePaymentEvidenceFailsOnIncompleteConvergence(t *testing.T) {
	summary := BuildAggregatePaymentEvidence(AggregatePaymentInput{
		Fact: db.ExternalPaymentFact{
			ID:               11,
			Provider:         db.ExternalPaymentProviderBaofu,
			Channel:          db.PaymentChannelBaofuAggregate,
			Capability:       db.ExternalPaymentCapabilityBaofuPayment,
			BusinessObjectID: pgtype.Int8{Int64: 31, Valid: true},
			Amount:           pgtype.Int8{Int64: 8800, Valid: true},
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			BusinessObjectID:   31,
			BusinessObjectType: "payment_order",
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			PaymentChannel: db.PaymentChannelDirect,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         9900,
			Status:         "pending",
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "payment fact is not terminalized")
	require.Contains(t, summary.Findings, "payment fact application is not applied")
	require.Contains(t, summary.Findings, "payment order is not paid")
	require.Contains(t, summary.Findings, "payment order channel is not baofu_aggregate")
	require.Contains(t, summary.Findings, "payment fact amount does not match payment order amount")
}

func TestBuildAggregatePaymentEvidenceRequiresFactBusinessLinkAndAmount(t *testing.T) {
	paidAt := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)
	summary := BuildAggregatePaymentEvidence(AggregatePaymentInput{
		Fact: db.ExternalPaymentFact{
			ID:                 11,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuPayment,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerReservation, Valid: true},
			BusinessObjectType: pgtype.Text{String: "reservation_order", Valid: true},
			IsTerminal:         true,
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			Status:         "paid",
			ProcessedAt:    pgtype.Timestamptz{Time: paidAt, Valid: true},
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Contains(t, summary.Findings, "payment fact business owner is not order")
	require.Contains(t, summary.Findings, "payment fact business object is not payment_order")
	require.Contains(t, summary.Findings, "payment fact business object does not match payment order")
	require.Contains(t, summary.Findings, "payment fact amount is missing")
}

func TestBuildAggregatePaymentEvidenceRequiresSuccessPaymentFactSource(t *testing.T) {
	paidAt := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)
	summary := BuildAggregatePaymentEvidence(AggregatePaymentInput{
		Fact: db.ExternalPaymentFact{
			ID:                 11,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuPayment,
			FactSource:         db.ExternalPaymentFactSourceCommandResponse,
			SourceEventType:    pgtype.Text{String: "PAYMENT", Valid: true},
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 31, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			Status:         "paid",
			ProcessedAt:    pgtype.Timestamptz{Time: paidAt, Valid: true},
		},
	})

	require.Equal(t, StatusFail, summary.Status)
	require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, summary.FactSource)
	require.Equal(t, "PAYMENT", summary.SourceEventType)
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, summary.TerminalStatus)
	require.Contains(t, summary.Findings, "payment fact source is not callback/query/manual_reconciliation")
	require.Contains(t, summary.Findings, "payment fact terminal status is not success")
}

func TestBuildAggregatePaymentEvidenceMasksShortProviderIdentifiers(t *testing.T) {
	paidAt := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)
	summary := BuildAggregatePaymentEvidence(AggregatePaymentInput{
		Fact: db.ExternalPaymentFact{
			ID:                   11,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuPayment,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			ExternalObjectKey:    "ABCD1234",
			ExternalSecondaryKey: pgtype.Text{String: "T123", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 31, Valid: true},
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Amount:               pgtype.Int8{Int64: 8800, Valid: true},
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		Application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		PaymentOrder: db.PaymentOrder{
			ID:             31,
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Amount:         8800,
			OutTradeNo:     "ABCD1234",
			TransactionID:  pgtype.Text{String: "T123", Valid: true},
			Status:         "paid",
			ProcessedAt:    pgtype.Timestamptz{Time: paidAt, Valid: true},
		},
	})

	require.Equal(t, StatusPass, summary.Status)
	require.Equal(t, "AB***34", summary.OutTradeNoMasked)
	require.Equal(t, "T***3", summary.TradeNoMasked)
}
