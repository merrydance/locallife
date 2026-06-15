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

func TestRenderProfitSharingLedgerRowForCallbackEvidence(t *testing.T) {
	row, err := RenderProfitSharingLedgerRow(ProfitSharingSummary{
		Status:                   StatusPass,
		FactID:                   101,
		ApplicationID:            201,
		ProfitSharingOrderID:     61,
		PaymentOrderID:           31,
		CommandID:                301,
		FactSource:               db.ExternalPaymentFactSourceCallback,
		SourceEventType:          "SHARING",
		TerminalStatus:           db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:        db.ExternalPaymentFactApplicationStatusApplied,
		ProfitSharingOrderStatus: db.ProfitSharingOrderStatusFinished,
		CommandStatus:            db.ExternalPaymentCommandStatusAccepted,
		AmountFen:                8900,
		OutOrderNoMasked:         "BFPS***1O41",
		TradeNoMasked:            "2605***9999",
	}, EvidenceLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/profit-sharing",
		ACK:      "OK",
		Commit:   "2d6ebbdf",
		Notes:    "controlled share callback",
	})

	require.NoError(t, err)
	require.Equal(t, "Profit Sharing Callback", row.Section)
	require.Equal(t, "| 2026-06-15 | production | `https://llapi.merrydance.cn/v1/webhooks/baofu/profit-sharing` | `BFPS***1O41` | `2605***9999` | success; share_status=finished | fact_id=101; source=callback; event=SHARING | applied application_id=201 | OK | `2d6ebbdf` | controlled share callback; local_row_ids: profit_sharing_order_id=61, payment_order_id=31, command_id=301 |", row.Row)
}

func TestRenderProfitSharingLedgerRowForQueryEvidence(t *testing.T) {
	row, err := RenderProfitSharingLedgerRow(ProfitSharingSummary{
		Status:                   StatusPass,
		FactID:                   102,
		ApplicationID:            202,
		ProfitSharingOrderID:     62,
		PaymentOrderID:           32,
		CommandID:                302,
		FactSource:               db.ExternalPaymentFactSourceManualReconciliation,
		TerminalStatus:           db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:        db.ExternalPaymentFactApplicationStatusApplied,
		ProfitSharingOrderStatus: db.ProfitSharingOrderStatusFinished,
		CommandStatus:            db.ExternalPaymentCommandStatusAccepted,
		AmountFen:                6600,
		OutOrderNoMasked:         "BFPS***1O42",
		TradeNoMasked:            "2605***8888",
	}, EvidenceLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://mch-juhe.baofoo.com/api share_query",
		Commit:   "2d6ebbdf",
		Notes:    "controlled share recovery query",
	})

	require.NoError(t, err)
	require.Equal(t, "Profit Sharing Query", row.Section)
	require.Equal(t, "| 2026-06-15 | production | `https://mch-juhe.baofoo.com/api share_query` | `BFPS***1O42` / `2605***8888` | success; fact_source=manual_reconciliation; share_status=finished | fact_id=102; terminal_status=success | applied application_id=202 | `2d6ebbdf` | controlled share recovery query; local_row_ids: profit_sharing_order_id=62, payment_order_id=32, command_id=302 |", row.Row)
}

func TestRenderProfitSharingLedgerRowRejectsMissingCallbackACK(t *testing.T) {
	_, err := RenderProfitSharingLedgerRow(ProfitSharingSummary{
		Status:                   StatusPass,
		FactID:                   101,
		ApplicationID:            201,
		FactSource:               db.ExternalPaymentFactSourceCallback,
		TerminalStatus:           db.ExternalPaymentTerminalStatusSuccess,
		ApplicationStatus:        db.ExternalPaymentFactApplicationStatusApplied,
		ProfitSharingOrderStatus: db.ProfitSharingOrderStatusFinished,
		OutOrderNoMasked:         "BFPS***1O41",
	}, EvidenceLedgerRowContext{
		Date:     "2026-06-15",
		Env:      "production",
		Endpoint: "https://llapi.merrydance.cn/v1/webhooks/baofu/profit-sharing",
		Commit:   "2d6ebbdf",
		Notes:    "missing callback ack",
	})

	require.ErrorContains(t, err, "callback ack is required")
}
