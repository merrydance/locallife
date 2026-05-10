package logic

import (
	"context"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

func (svc *PaymentFactService) applyRiderDepositPaymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (riderDepositPaymentDomainResult, error) {
	var result riderDepositPaymentDomainResult
	if err := validateRiderDepositPaymentFactApplication(application, fact); err != nil {
		return result, err
	}

	paymentResult, err := svc.store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID: application.BusinessObjectID,
	})
	if err != nil {
		return result, fmt.Errorf("process rider deposit payment success: %w", err)
	}

	result = riderDepositPaymentDomainResult{
		PaymentOrder: paymentResult.PaymentOrder,
		Processed:    paymentResult.Processed,
	}
	if paymentResult.Processed || paymentResult.PaymentOrder.ProcessedAt.Valid {
		if err := svc.maybeRequestRiderReceiverPresent(ctx, paymentResult.PaymentOrder); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (svc *PaymentFactService) applyClaimRecoveryPaymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (claimRecoveryPaymentDomainResult, error) {
	var result claimRecoveryPaymentDomainResult
	if err := validateClaimRecoveryPaymentFactApplication(application, fact); err != nil {
		return result, err
	}

	paymentResult, err := svc.store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID: application.BusinessObjectID,
	})
	if err != nil {
		return result, fmt.Errorf("process claim recovery payment success: %w", err)
	}

	result = claimRecoveryPaymentDomainResult{
		PaymentOrder: paymentResult.PaymentOrder,
		Processed:    paymentResult.Processed,
	}
	return result, nil
}

func validateRiderDepositPaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelDirect || fact.Capability != db.ExternalPaymentCapabilityDirectJSAPIPayment {
		return fmt.Errorf("payment fact %d is not a wechat direct payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectPayment {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerRiderDeposit {
		return fmt.Errorf("payment fact %d business owner %q is not rider deposit", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("unsupported rider deposit payment terminal status %q", fact.TerminalStatus)
	}
	return nil
}

func validateClaimRecoveryPaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelDirect || fact.Capability != db.ExternalPaymentCapabilityDirectJSAPIPayment {
		return fmt.Errorf("payment fact %d is not a wechat direct payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectPayment {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerClaimRecovery {
		return fmt.Errorf("payment fact %d business owner %q is not claim recovery", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("unsupported claim recovery payment terminal status %q", fact.TerminalStatus)
	}
	return nil
}
