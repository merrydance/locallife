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

type EvidenceLedgerRowContext = AggregatePaymentLedgerRowContext

type AggregatePaymentLedgerRow struct {
	Section string `json:"section"`
	Row     string `json:"row"`
}

type EvidenceLedgerRow = AggregatePaymentLedgerRow

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

func RenderProfitSharingLedgerRow(summary ProfitSharingSummary, context EvidenceLedgerRowContext) (EvidenceLedgerRow, error) {
	if summary.Status != StatusPass {
		return EvidenceLedgerRow{}, fmt.Errorf("cannot render failing evidence summary: %s", strings.Join(summary.Findings, "; "))
	}
	if err := validateProfitSharingLedgerSummary(summary); err != nil {
		return EvidenceLedgerRow{}, err
	}
	if err := validateProfitSharingLedgerRowContext(summary, context); err != nil {
		return EvidenceLedgerRow{}, err
	}

	localIDs := profitSharingLocalRowIDs(summary)
	notes := strings.TrimSpace(context.Notes)
	if localIDs != "" {
		notes = strings.TrimSpace(notes + "; local_row_ids: " + localIDs)
	}

	switch strings.TrimSpace(summary.FactSource) {
	case db.ExternalPaymentFactSourceCallback:
		return EvidenceLedgerRow{
			Section: "Profit Sharing Callback",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` | `%s` | success; share_status=%s | fact_id=%d; source=%s; event=%s | applied application_id=%d | %s | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutOrderNoMasked,
				emptyDash(summary.TradeNoMasked),
				summary.ProfitSharingOrderStatus,
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
		return EvidenceLedgerRow{
			Section: "Profit Sharing Query",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` / `%s` | success; fact_source=%s; share_status=%s | fact_id=%d; terminal_status=%s | applied application_id=%d | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutOrderNoMasked,
				emptyDash(summary.TradeNoMasked),
				summary.FactSource,
				summary.ProfitSharingOrderStatus,
				summary.FactID,
				summary.TerminalStatus,
				summary.ApplicationID,
				context.Commit,
				notes,
			),
		}, nil
	}
}

func RenderRefundLedgerRow(summary RefundSummary, context EvidenceLedgerRowContext) (EvidenceLedgerRow, error) {
	if summary.Status != StatusPass {
		return EvidenceLedgerRow{}, fmt.Errorf("cannot render failing evidence summary: %s", strings.Join(summary.Findings, "; "))
	}
	if err := validateRefundLedgerSummary(summary); err != nil {
		return EvidenceLedgerRow{}, err
	}
	if err := validateRefundLedgerRowContext(summary, context); err != nil {
		return EvidenceLedgerRow{}, err
	}

	localIDs := refundLocalRowIDs(summary)
	notes := strings.TrimSpace(context.Notes)
	if localIDs != "" {
		notes = strings.TrimSpace(notes + "; local_row_ids: " + localIDs)
	}

	switch strings.TrimSpace(summary.FactSource) {
	case db.ExternalPaymentFactSourceCallback:
		return EvidenceLedgerRow{
			Section: "Refund Callback",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` | success; refund_status=%s | fact_id=%d; source=%s; event=%s | applied application_id=%d | %s | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutRefundNoMasked,
				summary.RefundOrderStatus,
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
		return EvidenceLedgerRow{
			Section: "Refund Query",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` / `%s` | success; fact_source=%s; refund_status=%s | fact_id=%d; terminal_status=%s | applied application_id=%d | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutRefundNoMasked,
				emptyDash(summary.RefundIDMasked),
				summary.FactSource,
				summary.RefundOrderStatus,
				summary.FactID,
				summary.TerminalStatus,
				summary.ApplicationID,
				context.Commit,
				notes,
			),
		}, nil
	}
}

func RenderWithdrawalLedgerRow(summary WithdrawalSummary, context EvidenceLedgerRowContext) (EvidenceLedgerRow, error) {
	if summary.Status != StatusPass {
		return EvidenceLedgerRow{}, fmt.Errorf("cannot render failing evidence summary: %s", strings.Join(summary.Findings, "; "))
	}
	if err := validateWithdrawalLedgerSummary(summary); err != nil {
		return EvidenceLedgerRow{}, err
	}
	if err := validateWithdrawalLedgerRowContext(summary, context); err != nil {
		return EvidenceLedgerRow{}, err
	}

	localIDs := withdrawalLocalRowIDs(summary, strings.TrimSpace(summary.FactSource) == db.ExternalPaymentFactSourceCallback)
	notes := strings.TrimSpace(context.Notes)
	if localIDs != "" {
		notes = strings.TrimSpace(notes + "; local_row_ids: " + localIDs)
	}

	switch strings.TrimSpace(summary.FactSource) {
	case db.ExternalPaymentFactSourceCallback:
		return EvidenceLedgerRow{
			Section: "Withdrawal",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` | %s | %d | success; withdrawal_status=%s | `%s` | %s | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutRequestNoMasked,
				withdrawalLedgerOwner(summary),
				summary.AmountFen,
				summary.WithdrawalOrderStatus,
				emptyDash(summary.BaofuWithdrawNoMasked),
				context.ACK,
				context.Commit,
				notes,
			),
		}, nil
	default:
		return EvidenceLedgerRow{
			Section: "Withdrawal Query",
			Row: fmt.Sprintf("| %s | %s | `%s` | `%s` | - | success; fact_source=%s; withdrawal_status=%s | fact_id=%d; terminal_status=%s; baofu_withdraw_no=`%s` | `%s` | %s |",
				context.Date,
				context.Env,
				context.Endpoint,
				summary.OutRequestNoMasked,
				summary.FactSource,
				summary.WithdrawalOrderStatus,
				summary.FactID,
				summary.TerminalStatus,
				emptyDash(summary.BaofuWithdrawNoMasked),
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

func validateProfitSharingLedgerSummary(summary ProfitSharingSummary) error {
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
	if strings.TrimSpace(summary.ApplicationStatus) != db.ExternalPaymentFactApplicationStatusApplied {
		return fmt.Errorf("ledger summary application is not applied")
	}
	if strings.TrimSpace(summary.ProfitSharingOrderStatus) != db.ProfitSharingOrderStatusFinished {
		return fmt.Errorf("ledger summary profit sharing order is not finished")
	}
	if strings.TrimSpace(summary.OutOrderNoMasked) == "" {
		return fmt.Errorf("ledger summary masked out order no is required")
	}
	return nil
}

func validateRefundLedgerSummary(summary RefundSummary) error {
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
	if strings.TrimSpace(summary.ApplicationStatus) != db.ExternalPaymentFactApplicationStatusApplied {
		return fmt.Errorf("ledger summary application is not applied")
	}
	if strings.TrimSpace(summary.RefundOrderStatus) != refundOrderStatusSuccess {
		return fmt.Errorf("ledger summary refund order is not success")
	}
	if strings.TrimSpace(summary.OutRefundNoMasked) == "" {
		return fmt.Errorf("ledger summary masked out refund no is required")
	}
	return nil
}

func validateWithdrawalLedgerSummary(summary WithdrawalSummary) error {
	if summary.FactID <= 0 {
		return fmt.Errorf("ledger summary fact id is required")
	}
	if summary.WithdrawalOrderID <= 0 {
		return fmt.Errorf("ledger summary withdrawal order id is required")
	}
	if !isAcceptedPaymentFactSource(summary.FactSource) {
		return fmt.Errorf("ledger summary fact source is not callback/query/manual_reconciliation")
	}
	if summary.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("ledger summary terminal status is not success")
	}
	if strings.TrimSpace(summary.WithdrawalOrderStatus) != db.BaofuWithdrawalStatusSucceeded {
		return fmt.Errorf("ledger summary withdrawal order is not succeeded")
	}
	if strings.TrimSpace(summary.OutRequestNoMasked) == "" {
		return fmt.Errorf("ledger summary masked out request no is required")
	}
	if strings.TrimSpace(summary.BaofuWithdrawNoMasked) == "" {
		return fmt.Errorf("ledger summary masked baofu withdraw no is required")
	}
	if summary.AmountFen <= 0 {
		return fmt.Errorf("ledger summary withdrawal amount is not positive")
	}
	if strings.TrimSpace(summary.OwnerType) == "" || summary.OwnerID <= 0 {
		return fmt.Errorf("ledger summary withdrawal owner is required")
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

func validateProfitSharingLedgerRowContext(summary ProfitSharingSummary, context EvidenceLedgerRowContext) error {
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

func validateRefundLedgerRowContext(summary RefundSummary, context EvidenceLedgerRowContext) error {
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

func validateWithdrawalLedgerRowContext(summary WithdrawalSummary, context EvidenceLedgerRowContext) error {
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

func withdrawalLocalRowIDs(summary WithdrawalSummary, includeFactID bool) string {
	parts := []string{}
	if summary.WithdrawalOrderID > 0 {
		parts = append(parts, fmt.Sprintf("withdrawal_order_id=%d", summary.WithdrawalOrderID))
	}
	if summary.CommandID > 0 {
		parts = append(parts, fmt.Sprintf("command_id=%d", summary.CommandID))
	}
	if includeFactID && summary.FactID > 0 {
		parts = append(parts, fmt.Sprintf("fact_id=%d", summary.FactID))
	}
	return strings.Join(parts, ", ")
}

func withdrawalLedgerOwner(summary WithdrawalSummary) string {
	ownerType := emptyDash(summary.OwnerType)
	if summary.OwnerID <= 0 {
		return ownerType
	}
	return fmt.Sprintf("%s:%d", ownerType, summary.OwnerID)
}

func refundLocalRowIDs(summary RefundSummary) string {
	parts := []string{}
	if summary.RefundOrderID > 0 {
		parts = append(parts, fmt.Sprintf("refund_order_id=%d", summary.RefundOrderID))
	}
	if summary.PaymentOrderID > 0 {
		parts = append(parts, fmt.Sprintf("payment_order_id=%d", summary.PaymentOrderID))
	}
	if summary.OrderID > 0 {
		parts = append(parts, fmt.Sprintf("order_id=%d", summary.OrderID))
	}
	if summary.ReservationID > 0 {
		parts = append(parts, fmt.Sprintf("reservation_id=%d", summary.ReservationID))
	}
	if summary.CommandID > 0 {
		parts = append(parts, fmt.Sprintf("command_id=%d", summary.CommandID))
	}
	return strings.Join(parts, ", ")
}

func profitSharingLocalRowIDs(summary ProfitSharingSummary) string {
	parts := []string{}
	if summary.ProfitSharingOrderID > 0 {
		parts = append(parts, fmt.Sprintf("profit_sharing_order_id=%d", summary.ProfitSharingOrderID))
	}
	if summary.PaymentOrderID > 0 {
		parts = append(parts, fmt.Sprintf("payment_order_id=%d", summary.PaymentOrderID))
	}
	if summary.CommandID > 0 {
		parts = append(parts, fmt.Sprintf("command_id=%d", summary.CommandID))
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
