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

func (svc *PaymentFactService) markBaofuPaymentOrderPaid(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (db.PaymentOrder, error) {
	transactionID := ""
	if fact.ExternalSecondaryKey.Valid {
		transactionID = strings.TrimSpace(fact.ExternalSecondaryKey.String)
	}
	if transactionID == "" {
		transactionID = strings.TrimSpace(fact.ExternalObjectKey)
	}
	paid, err := svc.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            application.BusinessObjectID,
		TransactionID: pgtype.Text{String: transactionID, Valid: transactionID != ""},
	})
	if err == nil {
		return paid, nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return db.PaymentOrder{}, fmt.Errorf("mark baofu payment order paid: %w", err)
	}
	current, getErr := svc.store.GetPaymentOrder(ctx, application.BusinessObjectID)
	if getErr != nil {
		return db.PaymentOrder{}, fmt.Errorf("get baofu payment order after paid update conflict: %w", getErr)
	}
	if current.Status != "paid" {
		return db.PaymentOrder{}, fmt.Errorf("baofu payment order %d is not payable after success fact: status=%s", current.ID, current.Status)
	}
	return current, nil
}

func (svc *PaymentFactService) applyBaofuOrderPaymentTerminalFailure(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (ApplyOrderPaymentFactResult, error) {
	var result ApplyOrderPaymentFactResult
	paymentOrder, err := svc.applyBaofuPaymentTerminalFailure(ctx, application, fact)
	if err != nil {
		return result, err
	}
	result.PaymentOrder = paymentOrder
	return result, nil
}

func (svc *PaymentFactService) applyBaofuReservationPaymentTerminalFailure(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (ApplyReservationPaymentFactResult, error) {
	var result ApplyReservationPaymentFactResult
	paymentOrder, err := svc.applyBaofuPaymentTerminalFailure(ctx, application, fact)
	if err != nil {
		return result, err
	}
	result.PaymentOrder = paymentOrder
	if paymentOrder.ReservationID.Valid {
		result.ReservationID = paymentOrder.ReservationID.Int64
	}
	return result, nil
}

func (svc *PaymentFactService) applyBaofuPaymentTerminalFailure(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (db.PaymentOrder, error) {
	var (
		paymentOrder db.PaymentOrder
		err          error
	)
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusClosed:
		paymentOrder, err = svc.store.UpdatePaymentOrderToClosed(ctx, application.BusinessObjectID)
	case db.ExternalPaymentTerminalStatusFailed:
		paymentOrder, err = svc.store.UpdatePaymentOrderToFailed(ctx, application.BusinessObjectID)
	default:
		return db.PaymentOrder{}, fmt.Errorf("unsupported baofu payment terminal status %q", fact.TerminalStatus)
	}
	if err == nil {
		return paymentOrder, nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return db.PaymentOrder{}, fmt.Errorf("mark baofu payment order terminal %s: %w", fact.TerminalStatus, err)
	}
	current, getErr := svc.store.GetPaymentOrder(ctx, application.BusinessObjectID)
	if getErr != nil {
		return db.PaymentOrder{}, fmt.Errorf("get baofu payment order after terminal update conflict: %w", getErr)
	}
	if baofuPaymentTerminalStatusMatchesLocal(fact.TerminalStatus, current.Status) {
		return current, nil
	}
	logBaofuPaymentTerminalConflict(application, fact, current)
	return db.PaymentOrder{}, fmt.Errorf("baofu payment order %d is not terminal after %s fact: status=%s", current.ID, fact.TerminalStatus, current.Status)
}

func baofuPaymentTerminalStatusMatchesLocal(terminalStatus string, localStatus string) bool {
	switch terminalStatus {
	case db.ExternalPaymentTerminalStatusClosed:
		return localStatus == "closed"
	case db.ExternalPaymentTerminalStatusFailed:
		return localStatus == "failed"
	default:
		return false
	}
}

func logBaofuPaymentTerminalConflict(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, current db.PaymentOrder) {
	logger := log.Error().
		Int64("payment_order_id", application.BusinessObjectID).
		Int64("payment_fact_application_id", application.ID).
		Int64("fact_id", fact.ID).
		Str("terminal_status", fact.TerminalStatus).
		Str("current_payment_status", current.Status).
		Str("business_type", current.BusinessType).
		Str("operation", "apply_baofu_payment_terminal_failure")
	if current.ReservationID.Valid {
		logger = logger.Int64("reservation_id", current.ReservationID.Int64)
	}
	if current.OrderID.Valid {
		logger = logger.Int64("order_id", current.OrderID.Int64)
	}
	logger.Msg("baofu payment terminal fact conflicts with local payment status")
}

func isBaofuMainBusinessPaymentFact(fact db.ExternalPaymentFact) bool {
	return fact.Provider == db.ExternalPaymentProviderBaofu &&
		fact.Channel == db.PaymentChannelBaofuAggregate &&
		fact.Capability == db.ExternalPaymentCapabilityBaofuPayment
}

func isSupportedMainBusinessPaymentFact(fact db.ExternalPaymentFact) bool {
	return isBaofuMainBusinessPaymentFact(fact)
}

func isBaofuMainBusinessProfitSharingFact(fact db.ExternalPaymentFact) bool {
	return fact.Provider == db.ExternalPaymentProviderBaofu &&
		fact.Channel == db.PaymentChannelBaofuAggregate &&
		fact.Capability == db.ExternalPaymentCapabilityBaofuProfitSharing
}

func isSupportedMainBusinessProfitSharingFact(fact db.ExternalPaymentFact) bool {
	return isBaofuMainBusinessProfitSharingFact(fact)
}
