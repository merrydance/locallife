package baofuevidence

import (
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

type AggregatePaymentLedgerRowContext struct {
	Date     string
	Env      string
	Endpoint string
	ACK      string
	Commit   string
	Notes    string
}

type AggregatePaymentLedgerRow struct {
	Section string `json:"section"`
	Row     string `json:"row"`
}

func RenderAggregatePaymentLedgerRow(summary AggregatePaymentSummary, context AggregatePaymentLedgerRowContext) (AggregatePaymentLedgerRow, error) {
	if summary.Status != StatusPass {
		return AggregatePaymentLedgerRow{}, fmt.Errorf("cannot render failing evidence summary: %s", strings.Join(summary.Findings, "; "))
	}
	if err := validateLedgerSummary(summary); err != nil {
		return AggregatePaymentLedgerRow{}, err
	}
	if err := validateLedgerRowContext(summary, context); err != nil {
		return AggregatePaymentLedgerRow{}, err
	}

	localIDs := aggregatePaymentLocalRowIDs(summary)
	notes := strings.TrimSpace(context.Notes)
	if localIDs != "" {
		notes = strings.TrimSpace(notes + "; local_row_ids: " + localIDs)
	}

	switch strings.TrimSpace(summary.FactSource) {
	case db.ExternalPaymentFactSourceCallback:
		return AggregatePaymentLedgerRow{
			Section: "Payment Callback",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` | `%s` | success; payment_status=%s | fact_id=%d; source=%s; event=%s | applied application_id=%d | %s | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutTradeNoMasked,
				emptyDash(summary.TradeNoMasked),
				summary.PaymentOrderStatus,
				summary.FactID,
				summary.FactSource,
				emptyDash(summary.SourceEventType),
				summary.ApplicationID,
				context.ACK,
				context.Commit,
				notes,
			),
		}, nil
	default:
		return AggregatePaymentLedgerRow{
			Section: "Payment Query",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` / `%s` | success; fact_source=%s; payment_status=%s | fact_id=%d; terminal_status=%s | applied application_id=%d | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutTradeNoMasked,
				emptyDash(summary.TradeNoMasked),
				summary.FactSource,
				summary.PaymentOrderStatus,
				summary.FactID,
				summary.TerminalStatus,
				summary.ApplicationID,
				context.Commit,
				notes,
			),
		}, nil
	}
}

func validateLedgerSummary(summary AggregatePaymentSummary) error {
	if summary.FactID <= 0 {
		return fmt.Errorf("ledger summary fact id is required")
	}
	if summary.ApplicationID <= 0 {
		return fmt.Errorf("ledger summary application id is required")
	}
	if !isAcceptedPaymentFactSource(summary.FactSource) {
		return fmt.Errorf("ledger summary fact source is not callback/query/manual_reconciliation")
	}
	if summary.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("ledger summary terminal status is not success")
	}
	if strings.TrimSpace(summary.PaymentOrderStatus) != paymentOrderStatusPaid {
		return fmt.Errorf("ledger summary payment order is not paid")
	}
	if strings.TrimSpace(summary.OutTradeNoMasked) == "" {
		return fmt.Errorf("ledger summary masked out trade no is required")
	}
	return nil
}

func validateLedgerRowContext(summary AggregatePaymentSummary, context AggregatePaymentLedgerRowContext) error {
	if strings.TrimSpace(context.Date) == "" {
		return fmt.Errorf("ledger evidence date is required")
	}
	if strings.TrimSpace(context.Env) == "" {
		return fmt.Errorf("ledger evidence env is required")
	}
	if strings.TrimSpace(context.Endpoint) == "" {
		return fmt.Errorf("ledger evidence endpoint is required")
	}
	if strings.TrimSpace(context.Commit) == "" {
		return fmt.Errorf("ledger evidence commit is required")
	}
	if strings.TrimSpace(context.Notes) == "" {
		return fmt.Errorf("ledger evidence notes are required")
	}
	if strings.TrimSpace(summary.FactSource) == db.ExternalPaymentFactSourceCallback && strings.TrimSpace(context.ACK) == "" {
		return fmt.Errorf("callback ack is required")
	}
	return nil
}

func aggregatePaymentLocalRowIDs(summary AggregatePaymentSummary) string {
	parts := []string{}
	if summary.PaymentOrderID > 0 {
		parts = append(parts, fmt.Sprintf("payment_order_id=%d", summary.PaymentOrderID))
	}
	if summary.OrderID > 0 {
		parts = append(parts, fmt.Sprintf("order_id=%d", summary.OrderID))
	}
	if summary.ProfitSharingOrderID > 0 {
		parts = append(parts, fmt.Sprintf("profit_sharing_order_id=%d", summary.ProfitSharingOrderID))
	}
	return strings.Join(parts, ", ")
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
