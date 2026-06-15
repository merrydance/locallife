package baofuevidence

import (
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const withdrawalFactBusinessObjectWithdrawalOrder = "baofu_withdrawal_order"

type WithdrawalInput struct {
	Fact    db.ExternalPaymentFact
	Order   db.BaofuWithdrawalOrder
	Command *db.ExternalPaymentCommand
}

type WithdrawalSummary struct {
	Status                string   `json:"status"`
	FactID                int64    `json:"fact_id"`
	WithdrawalOrderID     int64    `json:"withdrawal_order_id"`
	CommandID             int64    `json:"command_id,omitempty"`
	Provider              string   `json:"provider"`
	Channel               string   `json:"channel"`
	Capability            string   `json:"capability"`
	FactSource            string   `json:"fact_source"`
	SourceEventType       string   `json:"source_event_type,omitempty"`
	TerminalStatus        string   `json:"terminal_status"`
	FactProcessingStatus  string   `json:"fact_processing_status"`
	WithdrawalOrderStatus string   `json:"withdrawal_order_status"`
	CommandStatus         string   `json:"command_status,omitempty"`
	OwnerType             string   `json:"owner_type"`
	OwnerID               int64    `json:"owner_id"`
	AccountBindingID      int64    `json:"account_binding_id,omitempty"`
	BusinessOwner         string   `json:"business_owner,omitempty"`
	AmountFen             int64    `json:"amount_fen"`
	OutRequestNoMasked    string   `json:"out_request_no_masked,omitempty"`
	BaofuWithdrawNoMasked string   `json:"baofu_withdraw_no_masked,omitempty"`
	Findings              []string `json:"findings,omitempty"`
}

func BuildWithdrawalEvidence(input WithdrawalInput) WithdrawalSummary {
	expectedOwner := withdrawalExpectedBusinessOwner(input.Order.OwnerType)
	summary := WithdrawalSummary{
		Status:                StatusPass,
		FactID:                input.Fact.ID,
		WithdrawalOrderID:     input.Order.ID,
		Provider:              input.Fact.Provider,
		Channel:               input.Fact.Channel,
		Capability:            input.Fact.Capability,
		FactSource:            input.Fact.FactSource,
		TerminalStatus:        input.Fact.TerminalStatus,
		FactProcessingStatus:  input.Fact.ProcessingStatus,
		WithdrawalOrderStatus: input.Order.Status,
		OwnerType:             input.Order.OwnerType,
		OwnerID:               input.Order.OwnerID,
		AccountBindingID:      input.Order.AccountBindingID,
		AmountFen:             input.Order.Amount,
		OutRequestNoMasked:    maskIdentifier(firstNonEmpty(input.Order.OutRequestNo, input.Fact.ExternalObjectKey)),
	}
	if input.Fact.BusinessOwner.Valid {
		summary.BusinessOwner = input.Fact.BusinessOwner.String
	}
	if input.Fact.SourceEventType.Valid {
		summary.SourceEventType = input.Fact.SourceEventType.String
	}
	if input.Order.BaofuWithdrawNo.Valid {
		summary.BaofuWithdrawNoMasked = maskIdentifier(input.Order.BaofuWithdrawNo.String)
	} else if input.Fact.ExternalSecondaryKey.Valid {
		summary.BaofuWithdrawNoMasked = maskIdentifier(input.Fact.ExternalSecondaryKey.String)
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
		addFinding("withdrawal fact provider is not baofu")
	}
	if input.Fact.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("withdrawal fact channel is not baofu_aggregate")
	}
	if input.Fact.Capability != db.ExternalPaymentCapabilityBaofuWithdraw {
		addFinding("withdrawal fact capability is not baofu withdraw")
	}
	if !isAcceptedPaymentFactSource(input.Fact.FactSource) {
		addFinding("withdrawal fact source is not callback/query/manual_reconciliation")
	}
	if input.Fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		addFinding("withdrawal fact terminal status is not success")
	}
	if !input.Fact.IsTerminal {
		addFinding("withdrawal fact is not terminal")
	}
	if !input.Fact.BusinessOwner.Valid || !isWithdrawalBusinessOwner(input.Fact.BusinessOwner.String) {
		addFinding("withdrawal fact business owner is not a withdrawal funds owner")
	} else if expectedOwner != "" && input.Fact.BusinessOwner.String != expectedOwner {
		addFinding("withdrawal fact business owner does not match withdrawal order owner")
	}
	if !input.Fact.BusinessObjectType.Valid || input.Fact.BusinessObjectType.String != withdrawalFactBusinessObjectWithdrawalOrder {
		addFinding("withdrawal fact business object is not baofu_withdrawal_order")
	}
	if !input.Fact.BusinessObjectID.Valid || input.Fact.BusinessObjectID.Int64 != input.Order.ID {
		addFinding("withdrawal fact business object does not match withdrawal order")
	}
	if input.Fact.ExternalObjectType != db.ExternalPaymentObjectWithdraw {
		addFinding("withdrawal fact external object is not withdraw")
	}
	if strings.TrimSpace(input.Fact.ExternalObjectKey) == "" {
		addFinding("withdrawal fact external object key is missing")
	} else if strings.TrimSpace(input.Fact.ExternalObjectKey) != strings.TrimSpace(input.Order.OutRequestNo) {
		addFinding("withdrawal fact external object key does not match out_request_no")
	}
	if !input.Fact.Amount.Valid {
		addFinding("withdrawal fact amount is missing")
	} else if input.Fact.Amount.Int64 != input.Order.Amount {
		addFinding("withdrawal fact amount does not match withdrawal order amount")
	}
	if expectedOwner == "" {
		addFinding("withdrawal order owner type is not supported")
	}
	if input.Order.Status != db.BaofuWithdrawalStatusSucceeded {
		addFinding("withdrawal order is not succeeded")
	}
	if !input.Order.FinishedAt.Valid {
		addFinding("withdrawal order is not finished_at stamped")
	}
	if strings.TrimSpace(input.Order.OutRequestNo) == "" {
		addFinding("withdrawal order out_request_no is missing")
	}
	if input.Order.Amount <= 0 {
		addFinding("withdrawal order amount is not positive")
	}
	if !input.Order.BaofuWithdrawNo.Valid || strings.TrimSpace(input.Order.BaofuWithdrawNo.String) == "" {
		addFinding("withdrawal order baofu_withdraw_no is missing")
	} else if input.Fact.ExternalSecondaryKey.Valid &&
		strings.TrimSpace(input.Fact.ExternalSecondaryKey.String) != strings.TrimSpace(input.Order.BaofuWithdrawNo.String) {
		addFinding("withdrawal fact baofu withdraw no does not match withdrawal order")
	}
	if input.Command == nil {
		addFinding("withdrawal command is missing")
	} else {
		validateWithdrawalCommand(input, expectedOwner, addFinding)
	}

	return summary
}

func validateWithdrawalCommand(input WithdrawalInput, expectedOwner string, addFinding func(string)) {
	command := input.Command
	if command.Provider != db.ExternalPaymentProviderBaofu {
		addFinding("withdrawal command provider is not baofu")
	}
	if command.Channel != db.PaymentChannelBaofuAggregate {
		addFinding("withdrawal command channel is not baofu_aggregate")
	}
	if command.Capability != db.ExternalPaymentCapabilityBaofuWithdraw {
		addFinding("withdrawal command capability is not baofu withdraw")
	}
	if command.CommandType != db.ExternalPaymentCommandTypeCreateBaofuWithdraw {
		addFinding("withdrawal command type is not create_baofu_withdraw")
	}
	if !isWithdrawalBusinessOwner(command.BusinessOwner) {
		addFinding("withdrawal command business owner is not a withdrawal funds owner")
	} else if expectedOwner != "" && command.BusinessOwner != expectedOwner {
		addFinding("withdrawal command business owner does not match withdrawal order owner")
	}
	if !command.BusinessObjectType.Valid || command.BusinessObjectType.String != withdrawalFactBusinessObjectWithdrawalOrder {
		addFinding("withdrawal command business object is not baofu_withdrawal_order")
	}
	if !command.BusinessObjectID.Valid || command.BusinessObjectID.Int64 != input.Order.ID {
		addFinding("withdrawal command business object does not match withdrawal order")
	}
	if command.ExternalObjectType != db.ExternalPaymentObjectWithdraw {
		addFinding("withdrawal command external object is not withdraw")
	}
	if strings.TrimSpace(command.ExternalObjectKey) == "" {
		addFinding("withdrawal command external object key is missing")
	} else if strings.TrimSpace(command.ExternalObjectKey) != strings.TrimSpace(input.Order.OutRequestNo) {
		addFinding("withdrawal command external object key does not match out_request_no")
	}
	if command.CommandStatus != db.ExternalPaymentCommandStatusAccepted {
		addFinding("withdrawal command is not accepted")
	}
}

func withdrawalExpectedBusinessOwner(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	case db.BaofuAccountOwnerTypeRider:
		return db.ExternalPaymentBusinessOwnerRiderIncome
	case db.BaofuAccountOwnerTypeOperator:
		return db.ExternalPaymentBusinessOwnerOperatorFunds
	case db.BaofuAccountOwnerTypePlatform:
		return db.ExternalPaymentBusinessOwnerPlatformFunds
	default:
		return ""
	}
}

func isWithdrawalBusinessOwner(owner string) bool {
	switch strings.TrimSpace(owner) {
	case db.ExternalPaymentBusinessOwnerMerchantFunds,
		db.ExternalPaymentBusinessOwnerRiderIncome,
		db.ExternalPaymentBusinessOwnerOperatorFunds,
		db.ExternalPaymentBusinessOwnerPlatformFunds:
		return true
	default:
		return false
	}
}
