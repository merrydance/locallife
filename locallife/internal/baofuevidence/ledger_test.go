package baofuevidence

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestRenderAggregatePaymentLedgerRowForCallbackEvidence(t *testing.T) {
	row, err := RenderAggregatePaymentLedgerRow(AggregatePaymentSummary{
		Status:               StatusPass,
		FactID:               11,
		ApplicationID:        21,
		PaymentOrderID:       31,
		OrderID:              41,
		ProfitSharingOrderID: 61,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventType:      "PAYMENT",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:    db.ExternalPaymentFactApplicationStatusApplied,
		PaymentOrderStatus:   "paid",
		OutTradeNoMasked:     "BAOF***0001",
		TradeNoMasked:        "2605***1965",
	}, AggregatePaymentLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/payment",
		ACK:      "OK",
		Commit:   "b6507961",
		Notes:    "controlled first-order callback",
	})

	require.NoError(t, err)
	require.Equal(t, "Payment Callback", row.Section)
	require.Equal(t, "| 2026-06-15 | production | `https://llapi.merrydance.cn/v1/webhooks/baofu/payment` | `BAOF***0001` | `2605***1965` | success; payment_status=paid | fact_id=11; source=callback; event=PAYMENT | applied application_id=21 | OK | `b6507961` | controlled first-order callback; local_row_ids: payment_order_id=31, order_id=41, profit_sharing_order_id=61 |", row.Row)
}

func TestRenderAggregatePaymentLedgerRowForQueryEvidence(t *testing.T) {
	row, err := RenderAggregatePaymentLedgerRow(AggregatePaymentSummary{
		Status:             StatusPass,
		FactID:             12,
		ApplicationID:      22,
		PaymentOrderID:     32,
		FactSource:         db.ExternalPaymentFactSourceManualReconciliation,
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:  db.ExternalPaymentFactApplicationStatusApplied,
		PaymentOrderStatus: "paid",
		OutTradeNoMasked:   "BAOF***0002",
		TradeNoMasked:      "2605***1966",
	}, AggregatePaymentLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://mch-juhe.baofoo.com/api order_query",
		Commit:   "b6507961",
		Notes:    "controlled recovery query",
	})

	require.NoError(t, err)
	require.Equal(t, "Payment Query", row.Section)
	require.Equal(t, "| 2026-06-15 | production | `https://mch-juhe.baofoo.com/api order_query` | `BAOF***0002` / `2605***1966` | success; fact_source=manual_reconciliation; payment_status=paid | fact_id=12; terminal_status=success | applied application_id=22 | `b6507961` | controlled recovery query; local_row_ids: payment_order_id=32 |", row.Row)
}

func TestRenderAggregatePaymentLedgerRowRejectsIncompleteContext(t *testing.T) {
	_, err := RenderAggregatePaymentLedgerRow(AggregatePaymentSummary{
		Status:             StatusPass,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		FactID:             11,
		ApplicationID:      21,
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		PaymentOrderStatus: "paid",
		OutTradeNoMasked:   "BAOF***0001",
	}, AggregatePaymentLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/payment",
		Commit:   "b6507961",
		Notes:    "missing callback ack",
	})

	require.ErrorContains(t, err, "callback ack is required")
}

func TestRenderAggregatePaymentLedgerRowRejectsFailingSummary(t *testing.T) {
	_, err := RenderAggregatePaymentLedgerRow(AggregatePaymentSummary{
		Status:   StatusFail,
		Findings: []string{"payment fact terminal status is not success"},
	}, AggregatePaymentLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://mch-juhe.baofoo.com/api",
		Commit:   "b6507961",
		Notes:    "should not render failed summary",
	})

	require.ErrorContains(t, err, "cannot render failing evidence summary")
}

func TestRenderAggregatePaymentLedgerRowRejectsIncompleteSummary(t *testing.T) {
	_, err := RenderAggregatePaymentLedgerRow(AggregatePaymentSummary{
		Status:             StatusPass,
		FactID:             11,
		ApplicationID:      21,
		FactSource:         "command_response",
		TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
		PaymentOrderStatus: "paid",
	}, AggregatePaymentLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://mch-juhe.baofoo.com/api",
		Commit:   "b6507961",
		Notes:    "incomplete summary",
	})

	require.ErrorContains(t, err, "ledger summary fact source is not callback/query/manual_reconciliation")
}
