package baofuevidence

import (
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	profitSharingFactBusinessObjectProfitSharingOrder = "profit_sharing_order"
	baofuFeeV2CalculationVersion                      = "baofu_fee_v2"
)

type ProfitSharingInput struct {
	Fact        db.ExternalPaymentFact
	Application db.ExternalPaymentFactApplication
	Order       db.ProfitSharingOrder
	Command     *db.ExternalPaymentCommand
}

type ProfitSharingSummary struct {
	Status                     string   `json:"status"`
	FactID                     int64    `json:"fact_id"`
	ApplicationID              int64    `json:"application_id"`
	ProfitSharingOrderID       int64    `json:"profit_sharing_order_id"`
	PaymentOrderID             int64    `json:"payment_order_id"`
	CommandID                  int64    `json:"command_id,omitempty"`
	Provider                   string   `json:"provider"`
	Channel                    string   `json:"channel"`
	Capability                 string   `json:"capability"`
	FactSource                 string   `json:"fact_source"`
	SourceEventType            string   `json:"source_event_type,omitempty"`
	TerminalStatus             string   `json:"terminal_status"`
	FactProcessingStatus       string   `json:"fact_processing_status"`
	ApplicationStatus          string   `json:"application_status"`
	ProfitSharingOrderStatus   string   `json:"profit_sharing_order_status"`
	CommandStatus              string   `json:"command_status,omitempty"`
	AmountFen                  int64    `json:"amount_fen"`
	ExpectedAmountFen          int64    `json:"expected_amount_fen"`
	OutOrderNoMasked           string   `json:"out_order_no_masked,omitempty"`
	TradeNoMasked              string   `json:"trade_no_masked,omitempty"`
	MerchantSharingMerIDMasked string   `json:"merchant_sharing_mer_id_masked,omitempty"`
	RiderSharingMerIDMasked    string   `json:"rider_sharing_mer_id_masked,omitempty"`
	OperatorSharingMerIDMasked string   `json:"operator_sharing_mer_id_masked,omitempty"`
	PlatformSharingMerIDMasked string   `json:"platform_sharing_mer_id_masked,omitempty"`
	Findings                   []string `json:"findings,omitempty"`
}

func BuildProfitSharingEvidence(input ProfitSharingInput) ProfitSharingSummary {
	expectedAmount := profitSharingOrderExpectedShareAmount(input.Order)
	summary := ProfitSharingSummary{
		Status:                   StatusPass,
		FactID:                   input.Fact.ID,
		ApplicationID:            input.Application.ID,
		ProfitSharingOrderID:     input.Order.ID,
		PaymentOrderID:           input.Order.PaymentOrderID,
		Provider:                 input.Fact.Provider,
		Channel:                  input.Fact.Channel,
		Capability:               input.Fact.Capability,
		FactSource:               input.Fact.FactSource,
		TerminalStatus:           input.Fact.TerminalStatus,
		FactProcessingStatus:     input.Fact.ProcessingStatus,
		ApplicationStatus:        input.Application.Status,
		ProfitSharingOrderStatus: input.Order.Status,
		ExpectedAmountFen:        expectedAmount,
		OutOrderNoMasked:         maskIdentifier(firstNonEmpty(input.Order.OutOrderNo, input.Fact.ExternalObjectKey)),
	}
	if input.Fact.SourceEventType.Valid {
		summary.SourceEventType = input.Fact.SourceEventType.String
	}
	if input.Fact.Amount.Valid {
		summary.AmountFen = input.Fact.Amount.Int64
	}
	if input.Order.SharingOrderID.Valid {
		summary.TradeNoMasked = maskIdentifier(input.Order.SharingOrderID.String)
	} else if input.Fact.ExternalSecondaryKey.Valid {
		summary.TradeNoMasked = maskIdentifier(input.Fact.ExternalSecondaryKey.String)
	}
	if input.Order.MerchantSharingMerID.Valid {
		summary.MerchantSharingMerIDMasked = maskIdentifier(input.Order.MerchantSharingMerID.String)
	}
	if input.Order.RiderSharingMerID.Valid {
		summary.RiderSharingMerIDMasked = maskIdentifier(input.Order.RiderSharingMerID.String)
	}
	if input.Order.OperatorSharingMerID.Valid {
		summary.OperatorSharingMerIDMasked = maskIdentifier(input.Order.OperatorSharingMerID.String)
	}
	if input.Order.PlatformSharingMerID.Valid {
		summary.PlatformSharingMerIDMasked = maskIdentifier(input.Order.PlatformSharingMerID.String)
	}
	if input.Command != nil {
		summary.CommandID = input.Command.ID
		summary.CommandStatus = input.Command.CommandStatus
	}

	addFinding := func(finding string) {
		summary.Status = StatusFail
		summary.Findings = append(summary.Findings, finding)
	}

	if input.Fact.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("profit sharing fact provider is not baofu")
	}
	if input.Fact.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("profit sharing fact channel is not baofu_aggregate")
	}
	if input.Fact.Capability != db.ExternalPaymentCapabilityBaofuProfitSharing {
		addFinding("profit sharing fact capability is not baofu profit sharing")
	}
	if !isAcceptedPaymentFactSource(input.Fact.FactSource) {
		addFinding("profit sharing fact source is not callback/query/manual_reconciliation")
	}
	if input.Fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		addFinding("profit sharing fact terminal status is not success")
	}
	if !input.Fact.IsTerminal || input.Fact.ProcessingStatus != db.ExternalPaymentFactProcessingStatusTerminalized {
		addFinding("profit sharing fact is not terminalized")
	}
	if !input.Fact.BusinessOwner.Valid || input.Fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerProfitSharing {
		addFinding("profit sharing fact business owner is not profit_sharing")
	}
	if !input.Fact.BusinessObjectType.Valid || input.Fact.BusinessObjectType.String != profitSharingFactBusinessObjectProfitSharingOrder {
		addFinding("profit sharing fact business object is not profit_sharing_order")
	}
	if !input.Fact.BusinessObjectID.Valid || input.Fact.BusinessObjectID.Int64 != input.Order.ID {
		addFinding("profit sharing fact business object does not match profit sharing order")
	}
	if input.Fact.ExternalObjectType != db.ExternalPaymentObjectProfitSharing {
		addFinding("profit sharing fact external object is not profit_sharing")
	}
	if strings.TrimSpace(input.Fact.ExternalObjectKey) == "" {
		addFinding("profit sharing fact external object key is missing")
	} else if strings.TrimSpace(input.Fact.ExternalObjectKey) != strings.TrimSpace(input.Order.OutOrderNo) {
		addFinding("profit sharing fact external object key does not match out_order_no")
	}
	if !input.Fact.Amount.Valid {
		addFinding("profit sharing fact amount is missing")
	} else if input.Fact.Amount.Int64 != expectedAmount {
		addFinding("profit sharing fact amount does not match expected share amount")
	}
	if input.Application.FactID != input.Fact.ID {
		addFinding("profit sharing application does not reference the fact")
	}
	if input.Application.BusinessObjectID != input.Order.ID {
		addFinding("profit sharing application does not reference the profit sharing order")
	}
	if input.Application.BusinessObjectType != profitSharingFactBusinessObjectProfitSharingOrder {
		addFinding("profit sharing application business object is not profit_sharing_order")
	}
	if input.Application.Status != db.ExternalPaymentFactApplicationStatusApplied {
		addFinding("profit sharing application is not applied")
	}
	if input.Order.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("profit sharing order provider is not baofu")
	}
	if input.Order.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("profit sharing order channel is not baofu_aggregate")
	}
	if strings.TrimSpace(input.Order.OutOrderNo) == "" {
		addFinding("profit sharing order out_order_no is missing")
	}
	if input.Order.Status != db.ProfitSharingOrderStatusFinished {
		addFinding("profit sharing order is not finished")
	}
	if !input.Order.FinishedAt.Valid {
		addFinding("profit sharing order is not finished_at stamped")
	}
	if input.Command == nil {
		addFinding("profit sharing command is missing")
	} else {
		validateProfitSharingCommand(input, addFinding)
	}

	return summary
}

func validateProfitSharingCommand(input ProfitSharingInput, addFinding func(string)) {
	command := input.Command
	if command.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("profit sharing command provider is not baofu")
	}
	if command.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("profit sharing command channel is not baofu_aggregate")
	}
	if command.Capability != db.ExternalPaymentCapabilityBaofuProfitSharing {
		addFinding("profit sharing command capability is not baofu profit sharing")
	}
	if command.CommandType != db.ExternalPaymentCommandTypeCreateProfitSharing {
		addFinding("profit sharing command type is not create_profit_sharing")
	}
	if command.BusinessOwner != db.ExternalPaymentBusinessOwnerProfitSharing {
		addFinding("profit sharing command business owner is not profit_sharing")
	}
	if !command.BusinessObjectType.Valid || command.BusinessObjectType.String != profitSharingFactBusinessObjectProfitSharingOrder {
		addFinding("profit sharing command business object is not profit_sharing_order")
	}
	if !command.BusinessObjectID.Valid || command.BusinessObjectID.Int64 != input.Order.ID {
		addFinding("profit sharing command business object does not match profit sharing order")
	}
	if command.ExternalObjectType != db.ExternalPaymentObjectProfitSharing {
		addFinding("profit sharing command external object is not profit_sharing")
	}
	if strings.TrimSpace(command.ExternalObjectKey) == "" {
		addFinding("profit sharing command external object key is missing")
	} else if strings.TrimSpace(command.ExternalObjectKey) != strings.TrimSpace(input.Order.OutOrderNo) {
		addFinding("profit sharing command external object key does not match out_order_no")
	}
	if command.CommandStatus != db.ExternalPaymentCommandStatusAccepted {
		addFinding("profit sharing command is not accepted")
	}
}

func profitSharingOrderExpectedShareAmount(order db.ProfitSharingOrder) int64 {
	if order.CalculationVersion == baofuFeeV2CalculationVersion || order.PlatformReceiverAmount > 0 {
		return order.MerchantAmount + order.RiderAmount + order.OperatorCommission + order.PlatformReceiverAmount
	}
	return order.MerchantAmount + order.RiderAmount + order.OperatorCommission + order.PlatformCommission
}
