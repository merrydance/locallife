package baofuevidence

import (
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	StatusPass = "pass"
	StatusFail = "fail"

	paymentFactBusinessObjectPaymentOrder = "payment_order"
	paymentOrderStatusPaid                = "paid"
)

type AggregatePaymentInput struct {
	Fact               db.ExternalPaymentFact
	Application        db.ExternalPaymentFactApplication
	PaymentOrder       db.PaymentOrder
	Outbox             *db.PaymentDomainOutbox
	ProfitSharingOrder *db.ProfitSharingOrder
}

type AggregatePaymentSummary struct {
	Status                     string   `json:"status"`
	FactID                     int64    `json:"fact_id"`
	ApplicationID              int64    `json:"application_id"`
	PaymentOrderID             int64    `json:"payment_order_id"`
	OrderID                    int64    `json:"order_id,omitempty"`
	OutboxID                   int64    `json:"outbox_id,omitempty"`
	ProfitSharingOrderID       int64    `json:"profit_sharing_order_id,omitempty"`
	Provider                   string   `json:"provider"`
	Channel                    string   `json:"channel"`
	Capability                 string   `json:"capability"`
	FactSource                 string   `json:"fact_source"`
	SourceEventType            string   `json:"source_event_type,omitempty"`
	TerminalStatus             string   `json:"terminal_status"`
	FactProcessingStatus       string   `json:"fact_processing_status"`
	ApplicationStatus          string   `json:"application_status"`
	PaymentOrderStatus         string   `json:"payment_order_status"`
	OutboxStatus               string   `json:"outbox_status,omitempty"`
	ProfitSharingOrderStatus   string   `json:"profit_sharing_order_status,omitempty"`
	AmountFen                  int64    `json:"amount_fen"`
	OutTradeNoMasked           string   `json:"out_trade_no_masked,omitempty"`
	TradeNoMasked              string   `json:"trade_no_masked,omitempty"`
	MerchantSharingMerIDMasked string   `json:"merchant_sharing_mer_id_masked,omitempty"`
	Findings                   []string `json:"findings,omitempty"`
}

func BuildAggregatePaymentEvidence(input AggregatePaymentInput) AggregatePaymentSummary {
	summary := AggregatePaymentSummary{
		Status:               StatusPass,
		FactID:               input.Fact.ID,
		ApplicationID:        input.Application.ID,
		PaymentOrderID:       input.PaymentOrder.ID,
		Provider:             input.Fact.Provider,
		Channel:              input.Fact.Channel,
		Capability:           input.Fact.Capability,
		FactSource:           input.Fact.FactSource,
		TerminalStatus:       input.Fact.TerminalStatus,
		FactProcessingStatus: input.Fact.ProcessingStatus,
		ApplicationStatus:    input.Application.Status,
		PaymentOrderStatus:   input.PaymentOrder.Status,
		AmountFen:            input.PaymentOrder.Amount,
		OutTradeNoMasked:     maskIdentifier(firstNonEmpty(input.PaymentOrder.OutTradeNo, input.Fact.ExternalObjectKey)),
	}
	if input.PaymentOrder.OrderID.Valid {
		summary.OrderID = input.PaymentOrder.OrderID.Int64
	}
	if input.Fact.SourceEventType.Valid {
		summary.SourceEventType = input.Fact.SourceEventType.String
	}
	if input.PaymentOrder.TransactionID.Valid {
		summary.TradeNoMasked = maskIdentifier(input.PaymentOrder.TransactionID.String)
	} else if input.Fact.ExternalSecondaryKey.Valid {
		summary.TradeNoMasked = maskIdentifier(input.Fact.ExternalSecondaryKey.String)
	}
	if input.Outbox != nil {
		summary.OutboxID = input.Outbox.ID
		summary.OutboxStatus = input.Outbox.Status
	}
	if input.ProfitSharingOrder != nil {
		summary.ProfitSharingOrderID = input.ProfitSharingOrder.ID
		summary.ProfitSharingOrderStatus = input.ProfitSharingOrder.Status
		if input.ProfitSharingOrder.MerchantSharingMerID.Valid {
			summary.MerchantSharingMerIDMasked = maskIdentifier(input.ProfitSharingOrder.MerchantSharingMerID.String)
		}
	}

	addFinding := func(finding string) {
		summary.Status = StatusFail
		summary.Findings = append(summary.Findings, finding)
	}

	if input.Fact.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("payment fact provider is not baofu")
	}
	if input.Fact.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("payment fact channel is not baofu_aggregate")
	}
	if input.Fact.Capability != db.ExternalPaymentCapabilityBaofuPayment {
		addFinding("payment fact capability is not baofu payment")
	}
	if !isAcceptedPaymentFactSource(input.Fact.FactSource) {
		addFinding("payment fact source is not callback/query/manual_reconciliation")
	}
	if input.Fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		addFinding("payment fact terminal status is not success")
	}
	if !input.Fact.IsTerminal || input.Fact.ProcessingStatus != db.ExternalPaymentFactProcessingStatusTerminalized {
		addFinding("payment fact is not terminalized")
	}
	if !input.Fact.BusinessOwner.Valid || input.Fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerOrder {
		addFinding("payment fact business owner is not order")
	}
	if !input.Fact.BusinessObjectType.Valid || input.Fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		addFinding("payment fact business object is not payment_order")
	}
	if !input.Fact.BusinessObjectID.Valid || input.Fact.BusinessObjectID.Int64 != input.PaymentOrder.ID {
		addFinding("payment fact business object does not match payment order")
	}
	if !input.Fact.Amount.Valid {
		addFinding("payment fact amount is missing")
	} else if input.Fact.Amount.Int64 != input.PaymentOrder.Amount {
		addFinding("payment fact amount does not match payment order amount")
	}
	if input.Application.FactID != input.Fact.ID {
		addFinding("payment fact application does not reference the fact")
	}
	if input.Application.BusinessObjectID != input.PaymentOrder.ID {
		addFinding("payment fact application does not reference the payment order")
	}
	if input.Application.BusinessObjectType != paymentFactBusinessObjectPaymentOrder {
		addFinding("payment fact application business object is not payment_order")
	}
	if input.Application.Status != db.ExternalPaymentFactApplicationStatusApplied {
		addFinding("payment fact application is not applied")
	}
	if input.PaymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate {
		addFinding("payment order channel is not baofu_aggregate")
	}
	if input.PaymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		addFinding("payment order business type is not order")
	}
	if input.PaymentOrder.Status != paymentOrderStatusPaid {
		addFinding("payment order is not paid")
	}
	if !input.PaymentOrder.ProcessedAt.Valid {
		addFinding("payment order is not processed")
	}
	if input.Outbox != nil {
		if input.Outbox.EventType != db.PaymentDomainOutboxEventOrderPaymentSucceeded {
			addFinding("payment outbox event type is not order_payment_succeeded")
		}
		if input.Outbox.AggregateType != db.PaymentDomainOutboxAggregatePaymentOrder || input.Outbox.AggregateID != input.PaymentOrder.ID {
			addFinding("payment outbox aggregate does not reference the payment order")
		}
	}
	if input.PaymentOrder.RequiresProfitSharing {
		if input.ProfitSharingOrder == nil {
			addFinding("profit sharing bill is missing")
		} else if input.ProfitSharingOrder.PaymentOrderID != input.PaymentOrder.ID {
			addFinding("profit sharing bill does not reference the payment order")
		}
	}

	return summary
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isAcceptedPaymentFactSource(source string) bool {
	switch strings.TrimSpace(source) {
	case db.ExternalPaymentFactSourceCallback,
		db.ExternalPaymentFactSourceQuery,
		db.ExternalPaymentFactSourceManualReconciliation:
		return true
	default:
		return false
	}
}

func maskIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		if len(value) <= 2 {
			return "***"
		}
		if len(value) <= 4 {
			return value[:1] + "***" + value[len(value)-1:]
		}
		return value[:2] + "***" + value[len(value)-2:]
	}
	return value[:4] + "***" + value[len(value)-4:]
}
