package baofuevidence

import (
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	refundFactBusinessObjectRefundOrder = "refund_order"
	refundOrderStatusSuccess            = "success"
	reservationAddonBusinessType        = "reservation_addon"
)

type RefundInput struct {
	Fact         db.ExternalPaymentFact
	Application  db.ExternalPaymentFactApplication
	RefundOrder  db.RefundOrder
	PaymentOrder db.PaymentOrder
	Command      *db.ExternalPaymentCommand
}

type RefundSummary struct {
	Status                  string   `json:"status"`
	FactID                  int64    `json:"fact_id"`
	ApplicationID           int64    `json:"application_id"`
	RefundOrderID           int64    `json:"refund_order_id"`
	PaymentOrderID          int64    `json:"payment_order_id"`
	OrderID                 int64    `json:"order_id,omitempty"`
	ReservationID           int64    `json:"reservation_id,omitempty"`
	CommandID               int64    `json:"command_id,omitempty"`
	Provider                string   `json:"provider"`
	Channel                 string   `json:"channel"`
	Capability              string   `json:"capability"`
	FactSource              string   `json:"fact_source"`
	SourceEventType         string   `json:"source_event_type,omitempty"`
	TerminalStatus          string   `json:"terminal_status"`
	FactProcessingStatus    string   `json:"fact_processing_status"`
	ApplicationStatus       string   `json:"application_status"`
	RefundOrderStatus       string   `json:"refund_order_status"`
	PaymentOrderStatus      string   `json:"payment_order_status"`
	CommandStatus           string   `json:"command_status,omitempty"`
	BusinessOwner           string   `json:"business_owner,omitempty"`
	AmountFen               int64    `json:"amount_fen"`
	RefundAmountFen         int64    `json:"refund_amount_fen"`
	OutRefundNoMasked       string   `json:"out_refund_no_masked,omitempty"`
	RefundIDMasked          string   `json:"refund_id_masked,omitempty"`
	PaymentOutTradeNoMasked string   `json:"payment_out_trade_no_masked,omitempty"`
	PaymentTradeNoMasked    string   `json:"payment_trade_no_masked,omitempty"`
	Findings                []string `json:"findings,omitempty"`
}

func BuildRefundEvidence(input RefundInput) RefundSummary {
	summary := RefundSummary{
		Status:                  StatusPass,
		FactID:                  input.Fact.ID,
		ApplicationID:           input.Application.ID,
		RefundOrderID:           input.RefundOrder.ID,
		PaymentOrderID:          input.PaymentOrder.ID,
		Provider:                input.Fact.Provider,
		Channel:                 input.Fact.Channel,
		Capability:              input.Fact.Capability,
		FactSource:              input.Fact.FactSource,
		TerminalStatus:          input.Fact.TerminalStatus,
		FactProcessingStatus:    input.Fact.ProcessingStatus,
		ApplicationStatus:       input.Application.Status,
		RefundOrderStatus:       input.RefundOrder.Status,
		PaymentOrderStatus:      input.PaymentOrder.Status,
		RefundAmountFen:         input.RefundOrder.RefundAmount,
		OutRefundNoMasked:       maskIdentifier(firstNonEmpty(input.RefundOrder.OutRefundNo, input.Fact.ExternalObjectKey)),
		PaymentOutTradeNoMasked: maskIdentifier(input.PaymentOrder.OutTradeNo),
	}
	if input.PaymentOrder.OrderID.Valid {
		summary.OrderID = input.PaymentOrder.OrderID.Int64
	}
	if input.PaymentOrder.ReservationID.Valid {
		summary.ReservationID = input.PaymentOrder.ReservationID.Int64
	}
	if input.Fact.BusinessOwner.Valid {
		summary.BusinessOwner = input.Fact.BusinessOwner.String
	}
	if input.Fact.SourceEventType.Valid {
		summary.SourceEventType = input.Fact.SourceEventType.String
	}
	if input.Fact.Amount.Valid {
		summary.AmountFen = input.Fact.Amount.Int64
	}
	if input.RefundOrder.RefundID.Valid {
		summary.RefundIDMasked = maskIdentifier(input.RefundOrder.RefundID.String)
	} else if input.Fact.ExternalSecondaryKey.Valid {
		summary.RefundIDMasked = maskIdentifier(input.Fact.ExternalSecondaryKey.String)
	}
	if input.PaymentOrder.TransactionID.Valid {
		summary.PaymentTradeNoMasked = maskIdentifier(input.PaymentOrder.TransactionID.String)
	}
	if input.Command != nil {
		summary.CommandID = input.Command.ID
		summary.CommandStatus = input.Command.CommandStatus
	}

	addFinding := func(finding string) {
		summary.Status = StatusFail
		summary.Findings = append(summary.Findings, finding)
	}

	expectedOwner := refundExpectedBusinessOwner(input.PaymentOrder)

	if input.Fact.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("refund fact provider is not baofu")
	}
	if input.Fact.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("refund fact channel is not baofu_aggregate")
	}
	if input.Fact.Capability != db.ExternalPaymentCapabilityBaofuRefund {
		addFinding("refund fact capability is not baofu refund")
	}
	if !isAcceptedPaymentFactSource(input.Fact.FactSource) {
		addFinding("refund fact source is not callback/query/manual_reconciliation")
	}
	if input.Fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		addFinding("refund fact terminal status is not success")
	}
	if !input.Fact.IsTerminal || input.Fact.ProcessingStatus != db.ExternalPaymentFactProcessingStatusTerminalized {
		addFinding("refund fact is not terminalized")
	}
	if !input.Fact.BusinessOwner.Valid || !isRefundBusinessOwner(input.Fact.BusinessOwner.String) {
		addFinding("refund fact business owner is not order/reservation")
	} else if expectedOwner != "" && input.Fact.BusinessOwner.String != expectedOwner {
		addFinding("refund fact business owner does not match payment order")
	}
	if !input.Fact.BusinessObjectType.Valid || input.Fact.BusinessObjectType.String != refundFactBusinessObjectRefundOrder {
		addFinding("refund fact business object is not refund_order")
	}
	if !input.Fact.BusinessObjectID.Valid || input.Fact.BusinessObjectID.Int64 != input.RefundOrder.ID {
		addFinding("refund fact business object does not match refund order")
	}
	if input.Fact.ExternalObjectType != db.ExternalPaymentObjectRefund {
		addFinding("refund fact external object is not refund")
	}
	if strings.TrimSpace(input.Fact.ExternalObjectKey) == "" {
		addFinding("refund fact external object key is missing")
	} else if strings.TrimSpace(input.Fact.ExternalObjectKey) != strings.TrimSpace(input.RefundOrder.OutRefundNo) {
		addFinding("refund fact external object key does not match out_refund_no")
	}
	if !input.Fact.Amount.Valid {
		addFinding("refund fact amount is missing")
	} else if input.Fact.Amount.Int64 != input.RefundOrder.RefundAmount {
		addFinding("refund fact amount does not match refund order amount")
	}
	if input.Application.FactID != input.Fact.ID {
		addFinding("refund application does not reference the fact")
	}
	if input.Application.BusinessObjectID != input.RefundOrder.ID {
		addFinding("refund application does not reference the refund order")
	}
	if input.Application.BusinessObjectType != refundFactBusinessObjectRefundOrder {
		addFinding("refund application business object is not refund_order")
	}
	if input.Application.Status != db.ExternalPaymentFactApplicationStatusApplied {
		addFinding("refund application is not applied")
	}
	if input.RefundOrder.PaymentOrderID != input.PaymentOrder.ID {
		addFinding("refund order does not reference the payment order")
	}
	if input.RefundOrder.Status != refundOrderStatusSuccess {
		addFinding("refund order is not success")
	}
	if !input.RefundOrder.RefundedAt.Valid {
		addFinding("refund order is not refunded_at stamped")
	}
	if strings.TrimSpace(input.RefundOrder.OutRefundNo) == "" {
		addFinding("refund order out_refund_no is missing")
	}
	if input.RefundOrder.RefundAmount <= 0 {
		addFinding("refund order amount is not positive")
	}
	if input.PaymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate {
		addFinding("payment order channel is not baofu_aggregate")
	}
	if expectedOwner == "" {
		addFinding("payment order business type is not order/reservation")
	}
	if input.Command == nil {
		addFinding("refund command is missing")
	} else {
		validateRefundCommand(input, expectedOwner, addFinding)
	}

	return summary
}

func validateRefundCommand(input RefundInput, expectedOwner string, addFinding func(string)) {
	command := input.Command
	if command.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("refund command provider is not baofu")
	}
	if command.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("refund command channel is not baofu_aggregate")
	}
	if command.Capability != db.ExternalPaymentCapabilityBaofuRefund {
		addFinding("refund command capability is not baofu refund")
	}
	if command.CommandType != db.ExternalPaymentCommandTypeCreateRefund {
		addFinding("refund command type is not create_refund")
	}
	if !isRefundBusinessOwner(command.BusinessOwner) {
		addFinding("refund command business owner is not order/reservation")
	} else if expectedOwner != "" && command.BusinessOwner != expectedOwner {
		addFinding("refund command business owner does not match payment order")
	}
	if !command.BusinessObjectType.Valid || command.BusinessObjectType.String != refundFactBusinessObjectRefundOrder {
		addFinding("refund command business object is not refund_order")
	}
	if !command.BusinessObjectID.Valid || command.BusinessObjectID.Int64 != input.RefundOrder.ID {
		addFinding("refund command business object does not match refund order")
	}
	if command.ExternalObjectType != db.ExternalPaymentObjectRefund {
		addFinding("refund command external object is not refund")
	}
	if strings.TrimSpace(command.ExternalObjectKey) == "" {
		addFinding("refund command external object key is missing")
	} else if strings.TrimSpace(command.ExternalObjectKey) != strings.TrimSpace(input.RefundOrder.OutRefundNo) {
		addFinding("refund command external object key does not match out_refund_no")
	}
	if command.CommandStatus != db.ExternalPaymentCommandStatusAccepted {
		addFinding("refund command is not accepted")
	}
}

func refundExpectedBusinessOwner(paymentOrder db.PaymentOrder) string {
	if paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder {
		return db.ExternalPaymentBusinessOwnerOrder
	}
	if paymentOrder.ReservationID.Valid &&
		(paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == reservationAddonBusinessType) {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return ""
}

func isRefundBusinessOwner(owner string) bool {
	switch strings.TrimSpace(owner) {
	case db.ExternalPaymentBusinessOwnerOrder, db.ExternalPaymentBusinessOwnerReservation:
		return true
	default:
		return false
	}
}
