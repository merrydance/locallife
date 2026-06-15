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

func TestLoadAggregatePaymentEvidenceUsesExplicitRows(t *testing.T) {
	paidAt := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	reader := &fakeEvidenceReader{
		fact: db.ExternalPaymentFact{
			ID:                 11,
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuPayment,
			FactSource:         db.ExternalPaymentFactSourceCallback,
			BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
			BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: 31, Valid: true},
			TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:         true,
			Amount:             pgtype.Int8{Int64: 6600, Valid: true},
			ProcessingStatus:   db.ExternalPaymentFactProcessingStatusTerminalized,
		},
		application: db.ExternalPaymentFactApplication{
			ID:                 21,
			FactID:             11,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   31,
			Status:             db.ExternalPaymentFactApplicationStatusApplied,
		},
		paymentOrder: db.PaymentOrder{
			ID:                    31,
			OrderID:               pgtype.Int8{Int64: 41, Valid: true},
			PaymentChannel:        db.PaymentChannelBaofuAggregate,
			BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
			Amount:                6600,
			OutTradeNo:            "BAOFU_ORDER_202606150001",
			Status:                "paid",
			ProcessedAt:           pgtype.Timestamptz{Time: paidAt, Valid: true},
			RequiresProfitSharing: true,
		},
		profitSharingOrder: db.ProfitSharingOrder{
			ID:             51,
			PaymentOrderID: 31,
			Status:         db.ProfitSharingOrderStatusPending,
		},
	}

	summary, err := loadAggregatePaymentEvidence(context.Background(), reader, 11, 21, 31, 51)

	require.NoError(t, err)
	require.Equal(t, baofuevidence.StatusPass, summary.Status)
	require.Equal(t, []int64{11}, reader.factIDs)
	require.Equal(t, []int64{21}, reader.applicationIDs)
	require.Equal(t, []int64{31}, reader.paymentOrderIDs)
	require.Equal(t, []int64{51}, reader.profitSharingOrderIDs)
}

type fakeEvidenceReader struct {
	fact                  db.ExternalPaymentFact
	application           db.ExternalPaymentFactApplication
	paymentOrder          db.PaymentOrder
	profitSharingOrder    db.ProfitSharingOrder
	factIDs               []int64
	applicationIDs        []int64
	paymentOrderIDs       []int64
	profitSharingOrderIDs []int64
}

func (reader *fakeEvidenceReader) GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error) {
	reader.factIDs = append(reader.factIDs, id)
	return reader.fact, nil
}

func (reader *fakeEvidenceReader) GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error) {
	reader.applicationIDs = append(reader.applicationIDs, id)
	return reader.application, nil
}

func (reader *fakeEvidenceReader) GetPaymentOrder(ctx context.Context, id int64) (db.PaymentOrder, error) {
	reader.paymentOrderIDs = append(reader.paymentOrderIDs, id)
	return reader.paymentOrder, nil
}

func (reader *fakeEvidenceReader) GetProfitSharingOrder(ctx context.Context, id int64) (db.ProfitSharingOrder, error) {
	reader.profitSharingOrderIDs = append(reader.profitSharingOrderIDs, id)
	return reader.profitSharingOrder, nil
}
