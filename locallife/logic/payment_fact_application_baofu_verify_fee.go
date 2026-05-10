package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func (svc *PaymentFactService) applyBaofuVerifyFeePaymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (baofuVerifyFeePaymentDomainResult, error) {
	var result baofuVerifyFeePaymentDomainResult
	if err := validateBaofuVerifyFeePaymentFactApplication(application, fact); err != nil {
		return result, err
	}

	paymentOrder, err := svc.store.GetPaymentOrder(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get baofu verify fee payment order: %w", err)
	}
	if paymentOrder.BusinessType != db.PaymentBusinessTypeBaofuAccountVerifyFee {
		return result, fmt.Errorf("payment order %d business type %q is not baofu account verify fee", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if paymentOrder.PaymentChannel != db.PaymentChannelDirect {
		return result, fmt.Errorf("payment order %d channel %q is not direct", paymentOrder.ID, paymentOrder.PaymentChannel)
	}
	if fact.TerminalStatus == db.ExternalPaymentTerminalStatusSuccess && fact.Amount.Valid && fact.Amount.Int64 != paymentOrder.Amount {
		return result, fmt.Errorf("payment fact %d amount %d does not match baofu verify fee payment order %d amount %d", fact.ID, fact.Amount.Int64, paymentOrder.ID, paymentOrder.Amount)
	}
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusClosed, db.ExternalPaymentTerminalStatusFailed, db.ExternalPaymentTerminalStatusExpired:
		return svc.applyBaofuVerifyFeePaymentTerminalFailure(ctx, application, fact, paymentOrder)
	}
	if paymentOrder.Status != "paid" {
		return result, fmt.Errorf("payment order %d status %q is not paid", paymentOrder.ID, paymentOrder.Status)
	}

	processedPaymentOrder, err := svc.store.UpdatePaymentOrderProcessedAt(ctx, paymentOrder.ID)
	if errors.Is(err, db.ErrRecordNotFound) && paymentOrder.ProcessedAt.Valid {
		processedPaymentOrder = paymentOrder
		err = nil
	}
	if err != nil {
		return result, fmt.Errorf("mark baofu verify fee payment processed: %w", err)
	}

	result = baofuVerifyFeePaymentDomainResult{
		PaymentOrder: processedPaymentOrder,
		Processed:    processedPaymentOrder.ProcessedAt.Valid,
	}
	if svc.baofuContinuation != nil {
		if err := svc.baofuContinuation.ContinueAfterVerifyFeePaid(ctx, processedPaymentOrder); err != nil {
			return result, fmt.Errorf("continue baofu account opening after verify fee paid: %w", err)
		}
	}
	return result, nil
}

func (svc *PaymentFactService) applyBaofuVerifyFeePaymentTerminalFailure(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, paymentOrder db.PaymentOrder) (baofuVerifyFeePaymentDomainResult, error) {
	result := baofuVerifyFeePaymentDomainResult{PaymentOrder: paymentOrder}
	terminalStatus := strings.TrimSpace(fact.TerminalStatus)
	var (
		terminalPayment db.PaymentOrder
		err             error
	)
	switch terminalStatus {
	case db.ExternalPaymentTerminalStatusClosed, db.ExternalPaymentTerminalStatusExpired:
		terminalPayment, err = svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	case db.ExternalPaymentTerminalStatusFailed:
		terminalPayment, err = svc.store.UpdatePaymentOrderToFailed(ctx, paymentOrder.ID)
	default:
		return result, fmt.Errorf("unsupported baofu verify fee terminal failure status %q", terminalStatus)
	}
	if errors.Is(err, db.ErrRecordNotFound) && (paymentOrder.Status == "closed" || paymentOrder.Status == "failed") {
		terminalPayment = paymentOrder
		err = nil
	}
	if err != nil {
		return result, fmt.Errorf("mark baofu verify fee payment terminal %s: %w", terminalStatus, err)
	}
	result.PaymentOrder = terminalPayment
	result.Processed = false

	flow, err := svc.store.GetBaofuAccountOpeningFlowByPaymentOrder(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: paymentOrder.ID > 0})
	if errors.Is(err, db.ErrRecordNotFound) {
		log.Warn().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("consumer", application.Consumer).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", fact.ID).
			Str("terminal_status", terminalStatus).
			Msg("baofu verify fee terminal payment fact has no linked opening flow")
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("get baofu opening flow for terminal verify fee payment: %w", err)
	}
	updated, err := svc.store.MarkBaofuAccountOpeningFlowVerifyFeePending(ctx, db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{
		ID:                      flow.ID,
		ProfileID:               flow.ProfileID,
		VerifyFeeAmount:         flow.VerifyFeeAmount,
		VerifyFeePaymentOrderID: pgtype.Int8{},
		RawSnapshot: baofuOpeningSnapshot(map[string]any{
			"state":                       db.BaofuAccountOpeningStateVerifyFeePending,
			"payment_order_id":            paymentOrder.ID,
			"payment_status":              terminalPayment.Status,
			"payment_terminal_status":     terminalStatus,
			"payment_fact_id":             fact.ID,
			"payment_fact_application_id": application.ID,
			"retry_guidance":              "支付未完成，请重新支付开户核验费。",
		}),
	})
	if err != nil {
		return result, fmt.Errorf("reset baofu opening flow after terminal verify fee payment: %w", err)
	}
	log.Info().
		Int64("payment_order_id", paymentOrder.ID).
		Str("out_trade_no", paymentOrder.OutTradeNo).
		Str("consumer", application.Consumer).
		Int64("payment_fact_application_id", application.ID).
		Int64("payment_fact_id", fact.ID).
		Int64("flow_id", updated.ID).
		Str("owner_type", updated.OwnerType).
		Int64("owner_id", updated.OwnerID).
		Str("terminal_status", terminalStatus).
		Str("payment_status", terminalPayment.Status).
		Msg("baofu verify fee payment terminal failure reset opening flow to retryable pending")
	return result, nil
}

func validateBaofuVerifyFeePaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelDirect || fact.Capability != db.ExternalPaymentCapabilityDirectJSAPIPayment {
		return fmt.Errorf("payment fact %d is not a wechat direct payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectPayment {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerBaofuVerifyFee {
		return fmt.Errorf("payment fact %d business owner %q is not baofu account verify fee", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusClosed,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusExpired:
	default:
		return fmt.Errorf("unsupported baofu verify fee payment terminal status %q", fact.TerminalStatus)
	}
	return nil
}
