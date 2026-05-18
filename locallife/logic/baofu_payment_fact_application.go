package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
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
		return result, fmt.Errorf("unsupported baofu payment terminal status %q", fact.TerminalStatus)
	}
	if err == nil {
		result.PaymentOrder = paymentOrder
		return result, nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return result, fmt.Errorf("mark baofu payment order terminal %s: %w", fact.TerminalStatus, err)
	}
	current, getErr := svc.store.GetPaymentOrder(ctx, application.BusinessObjectID)
	if getErr != nil {
		return result, fmt.Errorf("get baofu payment order after terminal update conflict: %w", getErr)
	}
	if current.Status == "closed" || current.Status == "failed" || current.Status == "paid" || current.Status == "refunded" {
		result.PaymentOrder = current
		return result, nil
	}
	return result, fmt.Errorf("baofu payment order %d is not terminal after %s fact: status=%s", current.ID, fact.TerminalStatus, current.Status)
}

func isBaofuMainBusinessPaymentFact(fact db.ExternalPaymentFact) bool {
	return fact.Provider == db.ExternalPaymentProviderBaofu &&
		fact.Channel == db.PaymentChannelBaofuAggregate &&
		fact.Capability == db.ExternalPaymentCapabilityBaofuPayment
}

func isSupportedMainBusinessPaymentFact(fact db.ExternalPaymentFact) bool {
	return isWechatMainBusinessPaymentFact(fact) || isBaofuMainBusinessPaymentFact(fact)
}

func isBaofuMainBusinessProfitSharingFact(fact db.ExternalPaymentFact) bool {
	return fact.Provider == db.ExternalPaymentProviderBaofu &&
		fact.Channel == db.PaymentChannelBaofuAggregate &&
		fact.Capability == db.ExternalPaymentCapabilityBaofuProfitSharing
}

func isSupportedMainBusinessProfitSharingFact(fact db.ExternalPaymentFact) bool {
	return isWechatMainBusinessProfitSharingFact(fact) || isBaofuMainBusinessProfitSharingFact(fact)
}
