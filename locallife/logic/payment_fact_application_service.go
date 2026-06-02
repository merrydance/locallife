package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	paymentFactConsumerProfitSharingDomain         = "profit_sharing_domain"
	paymentFactConsumerOrderDomain                 = "order_domain"
	paymentFactConsumerClaimRecoveryDomain         = "claim_recovery_domain"
	paymentFactConsumerRiderDepositDomain          = "rider_deposit_domain"
	paymentFactConsumerReservationDomain           = "reservation_domain"
	paymentFactConsumerBaofuAccountVerifyFeeDomain = "baofu_account_verify_fee_domain"
	paymentFactBusinessObjectPaymentOrder          = "payment_order"
	paymentFactBusinessObjectRefundOrder           = "refund_order"
	paymentFactBusinessObjectProfitSharingOrder    = "profit_sharing_order"
	paymentFactBusinessObjectProfitSharingReturn   = "profit_sharing_return"
	paymentFactApplicationRetryDelay               = 5 * time.Minute
)

type ApplyExternalPaymentFactApplicationResult struct {
	Application                db.ExternalPaymentFactApplication
	Fact                       db.ExternalPaymentFact
	Outbox                     *db.PaymentDomainOutbox
	OrderPayment               *ApplyOrderPaymentFactResult
	ClaimRecoveryPayment       *claimRecoveryPaymentDomainResult
	ClaimRecoveryReleaseAction *db.BehaviorAction
	ReservationPayment         *ApplyReservationPaymentFactResult
	BaofuVerifyFeePayment      *baofuVerifyFeePaymentDomainResult
	Applied                    bool
	Skipped                    bool
}

type appliedPaymentFactDomainResult struct {
	ProfitSharingOrder         *db.ProfitSharingOrder
	ProfitSharingReturn        *profitSharingReturnDomainResult
	OrderPayment               *ApplyOrderPaymentFactResult
	ClaimRecoveryPayment       *claimRecoveryPaymentDomainResult
	ClaimRecoveryReleaseAction *db.BehaviorAction
	ReservationPayment         *ApplyReservationPaymentFactResult
	RiderDepositPayment        *riderDepositPaymentDomainResult
	BaofuVerifyFeePayment      *baofuVerifyFeePaymentDomainResult
	RiderDepositRefund         *riderDepositRefundDomainResult
	OrderRefund                *orderRefundDomainResult
	ReservationRefund          *reservationRefundDomainResult
}

type riderDepositPaymentDomainResult struct {
	PaymentOrder db.PaymentOrder
	Processed    bool
}

type baofuVerifyFeePaymentDomainResult struct {
	PaymentOrder db.PaymentOrder
	Processed    bool
}

type claimRecoveryPaymentDomainResult struct {
	PaymentOrder  db.PaymentOrder
	Processed     bool
	ReleaseAction *db.BehaviorAction
}

type ApplyOrderPaymentFactResult struct {
	PaymentOrder db.PaymentOrder
	Processed    bool
	OrderResult  *db.ProcessOrderPaymentTxResult
}

type ApplyReservationPaymentFactResult struct {
	PaymentOrder  db.PaymentOrder
	Processed     bool
	ReservationID int64
}

type riderDepositRefundDomainResult struct {
	RefundOrderID  int64
	PaymentOrderID int64
	OutRefundNo    string
	RefundStatus   string
	RefundID       string
}

type orderRefundDomainResult struct {
	RefundOrderID  int64
	PaymentOrderID int64
	OrderID        int64
	UserID         int64
	OutRefundNo    string
	RefundAmount   int64
	RefundStatus   string
	RefundID       string
}

type reservationRefundDomainResult struct {
	RefundOrderID  int64
	PaymentOrderID int64
	ReservationID  int64
	OutRefundNo    string
	RefundStatus   string
	RefundID       string
}

type profitSharingReturnDomainResult struct {
	ProfitSharingReturnID int64
	RefundOrderID         int64
	Status                string
	ReturnID              string
}

type profitSharingResultReadyOutboxPayload struct {
	ProfitSharingOrderID     int64  `json:"profit_sharing_order_id"`
	OutOrderNo               string `json:"out_order_no"`
	Result                   string `json:"result"`
	FailReason               string `json:"fail_reason,omitempty"`
	MerchantID               int64  `json:"merchant_id"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type orderPaymentSucceededOutboxPayload struct {
	PaymentOrderID           int64  `json:"payment_order_id"`
	OrderID                  int64  `json:"order_id"`
	MerchantID               int64  `json:"merchant_id"`
	OrderNo                  string `json:"order_no"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type reservationPaymentSucceededOutboxPayload struct {
	PaymentOrderID           int64  `json:"payment_order_id"`
	ReservationID            int64  `json:"reservation_id"`
	BusinessType             string `json:"business_type"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type riderDepositRefundAbnormalOutboxPayload struct {
	RefundOrderID            int64  `json:"refund_order_id"`
	PaymentOrderID           int64  `json:"payment_order_id"`
	OutRefundNo              string `json:"out_refund_no"`
	RefundStatus             string `json:"refund_status"`
	RefundID                 string `json:"refund_id,omitempty"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type reservationRefundAbnormalOutboxPayload struct {
	RefundOrderID            int64  `json:"refund_order_id"`
	PaymentOrderID           int64  `json:"payment_order_id"`
	ReservationID            int64  `json:"reservation_id"`
	OutRefundNo              string `json:"out_refund_no"`
	RefundStatus             string `json:"refund_status"`
	RefundID                 string `json:"refund_id,omitempty"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type orderRefundSucceededOutboxPayload struct {
	RefundOrderID            int64  `json:"refund_order_id"`
	PaymentOrderID           int64  `json:"payment_order_id"`
	OrderID                  int64  `json:"order_id"`
	UserID                   int64  `json:"user_id"`
	OutRefundNo              string `json:"out_refund_no"`
	RefundAmount             int64  `json:"refund_amount"`
	RefundStatus             string `json:"refund_status"`
	RefundID                 string `json:"refund_id,omitempty"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type orderRefundAbnormalOutboxPayload struct {
	RefundOrderID            int64  `json:"refund_order_id"`
	PaymentOrderID           int64  `json:"payment_order_id"`
	OrderID                  int64  `json:"order_id"`
	OutRefundNo              string `json:"out_refund_no"`
	RefundStatus             string `json:"refund_status"`
	RefundID                 string `json:"refund_id,omitempty"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

func (svc *PaymentFactService) ApplyExternalPaymentFactApplication(ctx context.Context, applicationID int64) (ApplyExternalPaymentFactApplicationResult, error) {
	var result ApplyExternalPaymentFactApplicationResult
	if applicationID == 0 {
		return result, fmt.Errorf("application id is required")
	}

	application, err := svc.store.ClaimExternalPaymentFactApplication(ctx, applicationID)
	if errors.Is(err, db.ErrRecordNotFound) {
		result.Skipped = true
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("claim external payment fact application: %w", err)
	}
	result.Application = application

	fact, err := svc.store.GetExternalPaymentFact(ctx, application.FactID)
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("get external payment fact: %w", err))
	}
	if len(fact.RawResource) > 0 && !json.Valid(fact.RawResource) {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("external payment fact %d raw_resource is not valid JSON", fact.ID), fact)
	}
	result.Fact = fact

	domainResult, err := svc.applyExternalPaymentFactToDomain(ctx, application, fact)
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, err, fact)
	}
	result.OrderPayment = domainResult.OrderPayment
	result.ClaimRecoveryPayment = domainResult.ClaimRecoveryPayment
	result.ClaimRecoveryReleaseAction = domainResult.ClaimRecoveryReleaseAction
	result.ReservationPayment = domainResult.ReservationPayment
	result.BaofuVerifyFeePayment = domainResult.BaofuVerifyFeePayment

	outbox, err := svc.createPaymentDomainOutboxForAppliedFact(ctx, application, fact, domainResult)
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, err, fact)
	}
	result.Outbox = outbox

	processedAt := svc.now().UTC()
	if _, err := svc.store.UpdateExternalPaymentFactProcessingStatus(ctx, db.UpdateExternalPaymentFactProcessingStatusParams{
		ID:               fact.ID,
		ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized,
		ProcessedAt:      pgtype.Timestamptz{Time: processedAt, Valid: true},
	}); err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("mark external payment fact terminalized: %w", err), fact)
	}

	applied, err := svc.store.MarkExternalPaymentFactApplicationApplied(ctx, db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: processedAt, Valid: true},
	})
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("mark external payment fact application applied: %w", err), fact)
	}
	result.Application = applied
	result.Applied = true
	return result, nil
}

func (svc *PaymentFactService) applyExternalPaymentFactToDomain(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (appliedPaymentFactDomainResult, error) {
	var result appliedPaymentFactDomainResult
	switch {
	case application.Consumer == paymentFactConsumerProfitSharingDomain && application.BusinessObjectType == paymentFactBusinessObjectProfitSharingOrder:
		profitSharingOrder, err := svc.applyProfitSharingFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		if profitSharingOrder.ID != 0 {
			result.ProfitSharingOrder = &profitSharingOrder
		}
		return result, nil
	case application.Consumer == paymentFactConsumerProfitSharingDomain && application.BusinessObjectType == paymentFactBusinessObjectProfitSharingReturn:
		profitSharingReturn, err := svc.applyProfitSharingReturnFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.ProfitSharingReturn = &profitSharingReturn
		return result, nil
	case application.Consumer == paymentFactConsumerOrderDomain && application.BusinessObjectType == paymentFactBusinessObjectPaymentOrder:
		orderPayment, err := svc.applyOrderPaymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.OrderPayment = &orderPayment
		return result, nil
	case application.Consumer == paymentFactConsumerClaimRecoveryDomain && application.BusinessObjectType == paymentFactBusinessObjectPaymentOrder:
		claimRecoveryPayment, err := svc.applyClaimRecoveryPaymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.ClaimRecoveryPayment = &claimRecoveryPayment
		result.ClaimRecoveryReleaseAction = claimRecoveryPayment.ReleaseAction
		return result, nil
	case application.Consumer == paymentFactConsumerReservationDomain && application.BusinessObjectType == paymentFactBusinessObjectPaymentOrder:
		reservationPayment, err := svc.applyReservationPaymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.ReservationPayment = &reservationPayment
		return result, nil
	case application.Consumer == paymentFactConsumerRiderDepositDomain && application.BusinessObjectType == paymentFactBusinessObjectPaymentOrder:
		riderDepositPayment, err := svc.applyRiderDepositPaymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.RiderDepositPayment = &riderDepositPayment
		return result, nil
	case application.Consumer == paymentFactConsumerBaofuAccountVerifyFeeDomain && application.BusinessObjectType == paymentFactBusinessObjectPaymentOrder:
		baofuVerifyFeePayment, err := svc.applyBaofuVerifyFeePaymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.BaofuVerifyFeePayment = &baofuVerifyFeePayment
		return result, nil
	case application.Consumer == paymentFactConsumerRiderDepositDomain && application.BusinessObjectType == paymentFactBusinessObjectRefundOrder:
		riderDepositRefund, err := svc.applyRiderDepositRefundFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.RiderDepositRefund = &riderDepositRefund
		return result, nil
	case application.Consumer == paymentFactConsumerOrderDomain && application.BusinessObjectType == paymentFactBusinessObjectRefundOrder:
		orderRefund, err := svc.applyOrderRefundFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.OrderRefund = &orderRefund
		return result, nil
	case application.Consumer == paymentFactConsumerReservationDomain && application.BusinessObjectType == paymentFactBusinessObjectRefundOrder:
		reservationRefund, err := svc.applyReservationRefundFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.ReservationRefund = &reservationRefund
		return result, nil
	default:
		return result, fmt.Errorf("unsupported payment fact application target %s/%s", application.Consumer, application.BusinessObjectType)
	}
}

func (svc *PaymentFactService) applyOrderPaymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (ApplyOrderPaymentFactResult, error) {
	var result ApplyOrderPaymentFactResult
	if err := validateOrderPaymentFactApplication(application, fact); err != nil {
		return result, err
	}
	if isBaofuMainBusinessPaymentFact(fact) {
		switch fact.TerminalStatus {
		case db.ExternalPaymentTerminalStatusClosed, db.ExternalPaymentTerminalStatusFailed:
			return svc.applyBaofuOrderPaymentTerminalFailure(ctx, application, fact)
		}
		if _, err := svc.markBaofuPaymentOrderPaid(ctx, application, fact); err != nil {
			return result, err
		}
	}

	paymentResult, err := svc.store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     application.BusinessObjectID,
		RiderAverageSpeed:  svc.riderAverageSpeed,
		DefaultPrepareTime: svc.defaultPrepareTime,
	})
	if err != nil {
		return result, fmt.Errorf("process order payment success: %w", err)
	}

	result = ApplyOrderPaymentFactResult{
		PaymentOrder: paymentResult.PaymentOrder,
		Processed:    paymentResult.Processed,
		OrderResult:  paymentResult.OrderResult,
	}
	if !result.Processed && result.PaymentOrder.ProcessedAt.Valid && result.PaymentOrder.OrderID.Valid {
		order, err := svc.store.GetOrder(ctx, result.PaymentOrder.OrderID.Int64)
		if err != nil {
			return result, fmt.Errorf("load processed order for payment outbox retry: %w", err)
		}
		result.Processed = true
		result.OrderResult = &db.ProcessOrderPaymentTxResult{Order: order}
	}
	return result, nil
}

func validateOrderPaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess &&
		!(isBaofuMainBusinessPaymentFact(fact) &&
			(fact.TerminalStatus == db.ExternalPaymentTerminalStatusClosed ||
				fact.TerminalStatus == db.ExternalPaymentTerminalStatusFailed)) {
		return fmt.Errorf("payment fact %d terminal status %q is not success", fact.ID, fact.TerminalStatus)
	}
	if !isSupportedMainBusinessPaymentFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported main business payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectBaofuPaymentOrder {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment fact %d business owner %q is not order", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		return fmt.Errorf("payment fact %d business object type %q is not payment_order", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) applyReservationPaymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (ApplyReservationPaymentFactResult, error) {
	var result ApplyReservationPaymentFactResult
	if err := validateReservationPaymentFactApplication(application, fact); err != nil {
		return result, err
	}
	if isBaofuMainBusinessPaymentFact(fact) {
		switch fact.TerminalStatus {
		case db.ExternalPaymentTerminalStatusClosed, db.ExternalPaymentTerminalStatusFailed:
			return svc.applyBaofuReservationPaymentTerminalFailure(ctx, application, fact)
		}
		if _, err := svc.markBaofuPaymentOrderPaid(ctx, application, fact); err != nil {
			return result, err
		}
	}

	paymentResult, err := svc.store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID: application.BusinessObjectID,
	})
	if err != nil {
		return result, fmt.Errorf("process reservation payment success: %w", err)
	}

	result = ApplyReservationPaymentFactResult{
		PaymentOrder: paymentResult.PaymentOrder,
		Processed:    paymentResult.Processed,
	}
	if !result.Processed && paymentResult.PaymentOrder.ProcessedAt.Valid {
		result.Processed = true
	}
	if paymentResult.PaymentOrder.ReservationID.Valid {
		result.ReservationID = paymentResult.PaymentOrder.ReservationID.Int64
	}
	return result, nil
}

func validateReservationPaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess &&
		!(isBaofuMainBusinessPaymentFact(fact) &&
			(fact.TerminalStatus == db.ExternalPaymentTerminalStatusClosed ||
				fact.TerminalStatus == db.ExternalPaymentTerminalStatusFailed)) {
		return fmt.Errorf("payment fact %d terminal status %q is not success", fact.ID, fact.TerminalStatus)
	}
	if !isSupportedMainBusinessPaymentFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported baofu main business payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectBaofuPaymentOrder {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerReservation {
		return fmt.Errorf("payment fact %d business owner %q is not reservation", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectPaymentOrder {
		return fmt.Errorf("payment fact %d business object type %q is not payment_order", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) applyRiderDepositRefundFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (riderDepositRefundDomainResult, error) {
	var result riderDepositRefundDomainResult
	if err := validateRiderDepositRefundFactApplication(application, fact); err != nil {
		return result, err
	}

	refundOrder, err := svc.store.GetRefundOrder(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := svc.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return result, fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerRiderDeposit {
		return result, fmt.Errorf("payment order %d business type %q is not rider deposit", paymentOrder.ID, paymentOrder.BusinessType)
	}

	refundService := NewRiderDepositRefundService(svc.store, nil)
	refundID := ""
	if fact.ExternalSecondaryKey.Valid {
		refundID = fact.ExternalSecondaryKey.String
	}
	if err := refundService.ResolveRefund(ctx, refundOrder.ID, paymentOrder, fact.UpstreamState, refundID); err != nil {
		return result, fmt.Errorf("apply rider deposit refund fact: %w", err)
	}

	result = riderDepositRefundDomainResult{
		RefundOrderID:  refundOrder.ID,
		PaymentOrderID: paymentOrder.ID,
		OutRefundNo:    refundOrder.OutRefundNo,
		RefundStatus:   fact.UpstreamState,
		RefundID:       refundID,
	}
	return result, nil
}

func validateRiderDepositRefundFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelDirect || fact.Capability != db.ExternalPaymentCapabilityDirectRefund {
		return fmt.Errorf("payment fact %d is not a wechat direct refund fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectRefund {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerRiderDeposit {
		return fmt.Errorf("payment fact %d business owner %q is not rider deposit", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectRefundOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) applyOrderRefundFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (orderRefundDomainResult, error) {
	var result orderRefundDomainResult
	if err := validateOrderRefundFactApplication(application, fact); err != nil {
		return result, err
	}

	refundOrder, err := svc.store.GetRefundOrder(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get order refund order: %w", err)
	}
	paymentOrder, err := svc.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return result, fmt.Errorf("get order payment order: %w", err)
	}
	if !isOrderRefundPaymentOrder(paymentOrder) {
		return result, fmt.Errorf("payment order %d business type %q is not order refund", paymentOrder.ID, paymentOrder.BusinessType)
	}

	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		if refundOrder.Status != refundOrderStatusSuccess {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
			if err != nil {
				return result, fmt.Errorf("update order refund order to success: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
		if err := svc.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount); err != nil {
			return result, fmt.Errorf("maybe mark order payment order refunded: %w", err)
		}
	case db.ExternalPaymentTerminalStatusClosed:
		if isTerminalRefundOrderStatus(refundOrder.Status) && refundOrder.Status != refundOrderStatusClosed {
			return result, nil
		}
		if refundOrder.Status != refundOrderStatusClosed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
			if err != nil {
				if terminal, ok, reloadErr := svc.refundOrderAfterTerminalUpdateConflict(ctx, refundOrder.ID); reloadErr != nil {
					return result, fmt.Errorf("reload order refund order after closed conflict: %w", reloadErr)
				} else if ok {
					if terminal.Status != refundOrderStatusClosed {
						return result, nil
					}
					refundOrder = terminal
					break
				}
				return result, fmt.Errorf("update order refund order to closed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	case db.ExternalPaymentTerminalStatusFailed:
		if isTerminalRefundOrderStatus(refundOrder.Status) && refundOrder.Status != refundOrderStatusFailed {
			return result, nil
		}
		if refundOrder.Status != refundOrderStatusFailed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			if err != nil {
				if terminal, ok, reloadErr := svc.refundOrderAfterTerminalUpdateConflict(ctx, refundOrder.ID); reloadErr != nil {
					return result, fmt.Errorf("reload order refund order after failed conflict: %w", reloadErr)
				} else if ok {
					if terminal.Status != refundOrderStatusFailed {
						return result, nil
					}
					refundOrder = terminal
					break
				}
				return result, fmt.Errorf("update order refund order to failed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	default:
		return result, fmt.Errorf("unsupported order refund terminal status %q", fact.TerminalStatus)
	}

	refundID := ""
	if fact.ExternalSecondaryKey.Valid {
		refundID = fact.ExternalSecondaryKey.String
	}
	result = orderRefundDomainResult{
		RefundOrderID:  refundOrder.ID,
		PaymentOrderID: paymentOrder.ID,
		UserID:         paymentOrder.UserID,
		OutRefundNo:    refundOrder.OutRefundNo,
		RefundAmount:   refundOrder.RefundAmount,
		RefundStatus:   fact.UpstreamState,
		RefundID:       refundID,
	}
	if paymentOrder.OrderID.Valid {
		result.OrderID = paymentOrder.OrderID.Int64
	}
	return result, nil
}

func validateOrderRefundFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if !isSupportedMainBusinessRefundFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported main-business refund fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectRefund {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment fact %d business owner %q is not order", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectRefundOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) applyReservationRefundFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (reservationRefundDomainResult, error) {
	var result reservationRefundDomainResult
	if err := validateReservationRefundFactApplication(application, fact); err != nil {
		return result, err
	}

	refundOrder, err := svc.store.GetRefundOrder(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get reservation refund order: %w", err)
	}
	paymentOrder, err := svc.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return result, fmt.Errorf("get reservation payment order: %w", err)
	}
	if !isReservationRefundPaymentOrder(paymentOrder) {
		return result, fmt.Errorf("payment order %d business type %q is not reservation refund", paymentOrder.ID, paymentOrder.BusinessType)
	}

	transitionedToSuccess := false
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		if refundOrder.Status != refundOrderStatusSuccess {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
			if err != nil {
				return result, fmt.Errorf("update reservation refund order to success: %w", err)
			}
			refundOrder = updatedRefundOrder
			transitionedToSuccess = true
		}
		if err := svc.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount); err != nil {
			return result, fmt.Errorf("maybe mark reservation payment order refunded: %w", err)
		}
		if transitionedToSuccess && paymentOrder.ReservationID.Valid && refundOrder.RefundAmount > 0 {
			if _, err := svc.store.AddReservationPrepaidAmount(ctx, db.AddReservationPrepaidAmountParams{
				ID:            paymentOrder.ReservationID.Int64,
				PrepaidAmount: -refundOrder.RefundAmount,
			}); err != nil {
				return result, fmt.Errorf("update reservation prepaid amount: %w", err)
			}
		}
	case db.ExternalPaymentTerminalStatusClosed:
		if isTerminalRefundOrderStatus(refundOrder.Status) && refundOrder.Status != refundOrderStatusClosed {
			return result, nil
		}
		if refundOrder.Status != refundOrderStatusClosed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
			if err != nil {
				if terminal, ok, reloadErr := svc.refundOrderAfterTerminalUpdateConflict(ctx, refundOrder.ID); reloadErr != nil {
					return result, fmt.Errorf("reload reservation refund order after closed conflict: %w", reloadErr)
				} else if ok {
					if terminal.Status != refundOrderStatusClosed {
						return result, nil
					}
					refundOrder = terminal
					break
				}
				return result, fmt.Errorf("update reservation refund order to closed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	case db.ExternalPaymentTerminalStatusFailed:
		if isTerminalRefundOrderStatus(refundOrder.Status) && refundOrder.Status != refundOrderStatusFailed {
			return result, nil
		}
		if refundOrder.Status != refundOrderStatusFailed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			if err != nil {
				if terminal, ok, reloadErr := svc.refundOrderAfterTerminalUpdateConflict(ctx, refundOrder.ID); reloadErr != nil {
					return result, fmt.Errorf("reload reservation refund order after failed conflict: %w", reloadErr)
				} else if ok {
					if terminal.Status != refundOrderStatusFailed {
						return result, nil
					}
					refundOrder = terminal
					break
				}
				return result, fmt.Errorf("update reservation refund order to failed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	default:
		return result, fmt.Errorf("unsupported reservation refund terminal status %q", fact.TerminalStatus)
	}

	refundID := ""
	if fact.ExternalSecondaryKey.Valid {
		refundID = fact.ExternalSecondaryKey.String
	}
	result = reservationRefundDomainResult{
		RefundOrderID:  refundOrder.ID,
		PaymentOrderID: paymentOrder.ID,
		OutRefundNo:    refundOrder.OutRefundNo,
		RefundStatus:   fact.UpstreamState,
		RefundID:       refundID,
	}
	if paymentOrder.ReservationID.Valid {
		result.ReservationID = paymentOrder.ReservationID.Int64
	}
	return result, nil
}

func validateReservationRefundFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if !isSupportedMainBusinessRefundFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported main-business refund fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectRefund {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerReservation {
		return fmt.Errorf("payment fact %d business owner %q is not reservation", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectRefundOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func isReservationRefundPaymentOrder(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.ReservationID.Valid &&
		(paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == reservationAddonBusiness)
}

func isSupportedMainBusinessRefundFact(fact db.ExternalPaymentFact) bool {
	return fact.Provider == db.ExternalPaymentProviderBaofu &&
		fact.Channel == db.PaymentChannelBaofuAggregate &&
		fact.Capability == db.ExternalPaymentCapabilityBaofuRefund
}

func isOrderRefundPaymentOrder(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder
}

const (
	refundOrderStatusSuccess = "success"
	refundOrderStatusFailed  = "failed"
	refundOrderStatusClosed  = "closed"
)

func isTerminalRefundOrderStatus(status string) bool {
	switch status {
	case refundOrderStatusSuccess, refundOrderStatusFailed, refundOrderStatusClosed:
		return true
	default:
		return false
	}
}

func (svc *PaymentFactService) refundOrderAfterTerminalUpdateConflict(ctx context.Context, refundOrderID int64) (db.RefundOrder, bool, error) {
	current, err := svc.store.GetRefundOrder(ctx, refundOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.RefundOrder{}, false, nil
		}
		return db.RefundOrder{}, false, err
	}
	if !isTerminalRefundOrderStatus(current.Status) {
		return db.RefundOrder{}, false, nil
	}
	return current, true, nil
}

func (svc *PaymentFactService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) error {
	totalRefunded, err := svc.store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return err
	}
	if totalRefunded < paymentAmount {
		return nil
	}
	_, err = svc.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID)
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil
	}
	return err
}

func (svc *PaymentFactService) applyProfitSharingFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (db.ProfitSharingOrder, error) {
	if err := validateProfitSharingFactApplication(application, fact); err != nil {
		return db.ProfitSharingOrder{}, err
	}

	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		return svc.applyProfitSharingSuccessFact(ctx, application.BusinessObjectID, fact)
	case db.ExternalPaymentTerminalStatusFailed, db.ExternalPaymentTerminalStatusClosed:
		return svc.applyProfitSharingFailedFact(ctx, application.BusinessObjectID)
	default:
		return db.ProfitSharingOrder{}, fmt.Errorf("unsupported profit sharing terminal status %q", fact.TerminalStatus)
	}
}

func validateProfitSharingFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if !isSupportedMainBusinessProfitSharingFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported main business profit sharing fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectProfitSharing {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectProfitSharingOrder {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) applyProfitSharingSuccessFact(ctx context.Context, profitSharingOrderID int64, fact db.ExternalPaymentFact) (db.ProfitSharingOrder, error) {
	order, err := svc.store.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("get profit sharing order: %w", err)
	}
	if order.Status == db.ProfitSharingOrderStatusFinished {
		if err := validateBaofuProfitSharingSuccessAmount(fact, order); err != nil {
			return db.ProfitSharingOrder{}, err
		}
		return order, nil
	}
	if order.Status != db.ProfitSharingOrderStatusProcessing {
		return db.ProfitSharingOrder{}, fmt.Errorf("profit sharing order %d status %q cannot apply success fact", profitSharingOrderID, order.Status)
	}
	if err := validateBaofuProfitSharingSuccessAmount(fact, order); err != nil {
		return db.ProfitSharingOrder{}, err
	}
	updated, err := svc.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrderID)
	if err != nil {
		if terminal, ok, reloadErr := svc.profitSharingOrderAfterTerminalUpdateConflict(ctx, profitSharingOrderID); reloadErr != nil {
			return db.ProfitSharingOrder{}, fmt.Errorf("reload profit sharing order after finished conflict: %w", reloadErr)
		} else if ok {
			if terminal.Status == db.ProfitSharingOrderStatusFinished {
				return terminal, nil
			}
			return db.ProfitSharingOrder{}, fmt.Errorf("profit sharing order %d already reached terminal status %q before success fact", profitSharingOrderID, terminal.Status)
		}
		return db.ProfitSharingOrder{}, fmt.Errorf("update profit sharing order to finished: %w", err)
	}
	return updated, nil
}

func validateBaofuProfitSharingSuccessAmount(fact db.ExternalPaymentFact, order db.ProfitSharingOrder) error {
	if !isBaofuMainBusinessProfitSharingFact(fact) || fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return nil
	}
	expected := baofuProfitSharingOrderExpectedShareAmount(order)
	if !fact.Amount.Valid || fact.Amount.Int64 <= 0 {
		return fmt.Errorf("宝付分账结果金额与本地账单不一致，请等待系统对账或联系平台处理: profit_sharing_order_id=%d reason=missing_success_amount", order.ID)
	}
	if fact.Amount.Int64 != expected {
		return fmt.Errorf("宝付分账结果金额与本地账单不一致，请等待系统对账或联系平台处理: profit_sharing_order_id=%d fact_amount=%d expected_amount=%d", order.ID, fact.Amount.Int64, expected)
	}
	return nil
}

func (svc *PaymentFactService) applyProfitSharingFailedFact(ctx context.Context, profitSharingOrderID int64) (db.ProfitSharingOrder, error) {
	order, err := svc.store.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("get profit sharing order: %w", err)
	}
	if order.Status == db.ProfitSharingOrderStatusFailed {
		return order, nil
	}
	if order.Status == db.ProfitSharingOrderStatusFinished {
		return db.ProfitSharingOrder{}, nil
	}
	if order.Status != db.ProfitSharingOrderStatusProcessing {
		return db.ProfitSharingOrder{}, fmt.Errorf("profit sharing order %d status %q cannot apply failed fact", profitSharingOrderID, order.Status)
	}
	updated, err := svc.store.UpdateProfitSharingOrderToFailed(ctx, profitSharingOrderID)
	if err != nil {
		if terminal, ok, reloadErr := svc.profitSharingOrderAfterTerminalUpdateConflict(ctx, profitSharingOrderID); reloadErr != nil {
			return db.ProfitSharingOrder{}, fmt.Errorf("reload profit sharing order after failed conflict: %w", reloadErr)
		} else if ok {
			if terminal.Status == db.ProfitSharingOrderStatusFailed {
				return terminal, nil
			}
			return db.ProfitSharingOrder{}, nil
		}
		return db.ProfitSharingOrder{}, fmt.Errorf("update profit sharing order to failed: %w", err)
	}
	return updated, nil
}

func (svc *PaymentFactService) profitSharingOrderAfterTerminalUpdateConflict(ctx context.Context, profitSharingOrderID int64) (db.ProfitSharingOrder, bool, error) {
	current, err := svc.store.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ProfitSharingOrder{}, false, nil
		}
		return db.ProfitSharingOrder{}, false, err
	}
	switch current.Status {
	case db.ProfitSharingOrderStatusFinished, db.ProfitSharingOrderStatusFailed:
		return current, true, nil
	default:
		return db.ProfitSharingOrder{}, false, nil
	}
}

func (svc *PaymentFactService) applyProfitSharingReturnFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (profitSharingReturnDomainResult, error) {
	var result profitSharingReturnDomainResult
	if err := validateProfitSharingReturnFactApplication(application, fact); err != nil {
		return result, err
	}

	returnRecord, err := svc.store.GetProfitSharingReturn(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get profit sharing return: %w", err)
	}

	returnID := ""
	if fact.ExternalSecondaryKey.Valid {
		returnID = fact.ExternalSecondaryKey.String
	}
	if returnID != "" && returnRecord.Status != "success" && returnRecord.Status != "failed" {
		updatedReturn, err := svc.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
			ID:       returnRecord.ID,
			ReturnID: pgtype.Text{String: returnID, Valid: true},
		})
		if err != nil {
			return result, fmt.Errorf("update profit sharing return to processing: %w", err)
		}
		returnRecord = updatedReturn
	}

	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		if returnRecord.Status != "success" {
			updatedReturn, err := svc.store.UpdateProfitSharingReturnToSuccess(ctx, returnRecord.ID)
			if err != nil {
				return result, fmt.Errorf("update profit sharing return to success: %w", err)
			}
			returnRecord = updatedReturn
		}
		if err := svc.tryInitiateRefundAfterProfitSharingReturns(ctx, returnRecord.RefundOrderID); err != nil {
			return result, fmt.Errorf("continue refund after profit sharing return success: %w", err)
		}
	case db.ExternalPaymentTerminalStatusFailed, db.ExternalPaymentTerminalStatusClosed:
		failReason, err := profitSharingReturnFactFailReason(fact.RawResource)
		if err != nil {
			return result, err
		}
		if failReason == "" {
			failReason = fact.UpstreamState
		}
		if returnRecord.Status != "failed" {
			updatedReturn, err := svc.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: failReason, Valid: failReason != ""},
			})
			if err != nil {
				return result, fmt.Errorf("update profit sharing return to failed: %w", err)
			}
			returnRecord = updatedReturn
		}
		if _, err := svc.store.UpdateRefundOrderToFailed(ctx, returnRecord.RefundOrderID); err != nil {
			return result, fmt.Errorf("update refund order to failed: %w", err)
		}
	default:
		return result, fmt.Errorf("unsupported profit sharing return terminal status %q", fact.TerminalStatus)
	}

	result = profitSharingReturnDomainResult{
		ProfitSharingReturnID: returnRecord.ID,
		RefundOrderID:         returnRecord.RefundOrderID,
		Status:                returnRecord.Status,
		ReturnID:              returnID,
	}
	return result, nil
}

func validateProfitSharingReturnFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if !isSupportedMainBusinessProfitSharingFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported baofu profit sharing return fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectProfitSharingReturn {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerProfitSharing {
		return fmt.Errorf("payment fact %d business owner %q is not profit sharing", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectProfitSharingReturn {
		return fmt.Errorf("payment fact %d has unsupported business object type %q", fact.ID, fact.BusinessObjectType.String)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	return nil
}

func (svc *PaymentFactService) tryInitiateRefundAfterProfitSharingReturns(ctx context.Context, refundOrderID int64) error {
	refundOrder, err := svc.store.GetRefundOrder(ctx, refundOrderID)
	if err != nil {
		return fmt.Errorf("get refund order: %w", err)
	}
	if refundOrder.Status != "pending" {
		return nil
	}

	totalCount, err := svc.store.CountProfitSharingReturnsByRefundOrder(ctx, refundOrderID)
	if err != nil {
		return fmt.Errorf("count profit sharing returns: %w", err)
	}
	if totalCount == 0 {
		return nil
	}

	successCount, err := svc.store.CountProfitSharingReturnsByRefundOrderStatus(ctx, db.CountProfitSharingReturnsByRefundOrderStatusParams{
		RefundOrderID: refundOrderID,
		Status:        "success",
	})
	if err != nil {
		return fmt.Errorf("count profit sharing returns success: %w", err)
	}
	failedCount, err := svc.store.CountProfitSharingReturnsByRefundOrderStatus(ctx, db.CountProfitSharingReturnsByRefundOrderStatusParams{
		RefundOrderID: refundOrderID,
		Status:        "failed",
	})
	if err != nil {
		return fmt.Errorf("count profit sharing returns failed: %w", err)
	}
	if failedCount > 0 {
		if _, err := svc.store.UpdateRefundOrderToFailed(ctx, refundOrderID); err != nil {
			return fmt.Errorf("update refund order to failed: %w", err)
		}
		return nil
	}
	if successCount < totalCount {
		return nil
	}

	paymentOrder, err := svc.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.Status != "paid" {
		return nil
	}
	if !paymentOrder.OrderID.Valid {
		return fmt.Errorf("payment order has no order id")
	}

	order, err := svc.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	paymentConfig, err := svc.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
	}
	if paymentConfig.SubMchID == "" {
		return fmt.Errorf("merchant sub mchid not configured")
	}

	if _, err := svc.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{},
	}); err != nil {
		return fmt.Errorf("update refund order to processing: %w", err)
	}
	return nil
}

func (svc *PaymentFactService) createPaymentDomainOutboxForAppliedFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, domainResult appliedPaymentFactDomainResult) (*db.PaymentDomainOutbox, error) {
	if domainResult.ProfitSharingReturn != nil {
		return nil, nil
	}
	if domainResult.OrderPayment != nil {
		return svc.createOrderPaymentOutbox(ctx, application, fact, *domainResult.OrderPayment)
	}
	if domainResult.ClaimRecoveryPayment != nil {
		return nil, nil
	}
	if domainResult.BaofuVerifyFeePayment != nil {
		return nil, nil
	}
	if domainResult.ReservationPayment != nil {
		return svc.createReservationPaymentOutbox(ctx, application, fact, *domainResult.ReservationPayment)
	}
	if domainResult.ProfitSharingOrder == nil {
		outbox, err := svc.createOrderRefundOutbox(ctx, application, fact, domainResult)
		if err != nil || outbox != nil {
			return outbox, err
		}
		outbox, err = svc.createRiderDepositRefundOutbox(ctx, application, fact, domainResult)
		if err != nil || outbox != nil {
			return outbox, err
		}
		return svc.createReservationRefundOutbox(ctx, application, fact, domainResult)
	}

	result, err := profitSharingResultForOutbox(fact.TerminalStatus)
	if err != nil {
		return nil, err
	}
	failReason, err := profitSharingFactFailReason(fact.RawResource)
	if err != nil {
		return nil, err
	}

	order := *domainResult.ProfitSharingOrder
	payload, err := json.Marshal(profitSharingResultReadyOutboxPayload{
		ProfitSharingOrderID:     order.ID,
		OutOrderNo:               order.OutOrderNo,
		Result:                   result,
		FailReason:               failReason,
		MerchantID:               order.MerchantID,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal profit sharing result outbox payload: %w", err)
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: db.PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   order.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create profit sharing result outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) createOrderPaymentOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, orderPayment ApplyOrderPaymentFactResult) (*db.PaymentDomainOutbox, error) {
	if !orderPayment.Processed || orderPayment.OrderResult == nil {
		return nil, nil
	}

	if err := svc.ensureBaofuOrderPaymentBill(ctx, orderPayment.PaymentOrder, orderPayment.OrderResult.Order); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(orderPaymentSucceededOutboxPayload{
		PaymentOrderID:           orderPayment.PaymentOrder.ID,
		OrderID:                  orderPayment.OrderResult.Order.ID,
		MerchantID:               orderPayment.OrderResult.Order.MerchantID,
		OrderNo:                  orderPayment.OrderResult.Order.OrderNo,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal order payment outbox payload: %w", err)
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventOrderPaymentSucceeded,
		AggregateType: db.PaymentDomainOutboxAggregatePaymentOrder,
		AggregateID:   orderPayment.PaymentOrder.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create order payment outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) ensureBaofuOrderPaymentBill(ctx context.Context, paymentOrder db.PaymentOrder, order db.Order) error {
	if !db.PaymentOrderRequiresProfitSharing(paymentOrder) {
		return nil
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment order %d business type %q is not order for baofu profit sharing bill", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if paymentOrder.Status != paymentStatusPaid {
		return fmt.Errorf("payment order %d status %q is not paid for baofu profit sharing bill", paymentOrder.ID, paymentOrder.Status)
	}
	if paymentOrder.Amount <= 0 {
		return fmt.Errorf("payment order %d amount is required for baofu profit sharing bill", paymentOrder.ID)
	}
	if order.ID <= 0 || order.MerchantID <= 0 {
		return fmt.Errorf("order is required for baofu profit sharing bill: payment_order_id=%d order_id=%d merchant_id=%d", paymentOrder.ID, order.ID, order.MerchantID)
	}
	if paymentOrder.OrderID.Valid && paymentOrder.OrderID.Int64 != order.ID {
		return fmt.Errorf("payment order %d order id %d does not match applied order %d", paymentOrder.ID, paymentOrder.OrderID.Int64, order.ID)
	}
	refundedAmount, err := svc.store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		return fmt.Errorf("get successful refund amount for baofu profit sharing bill: %w", err)
	}
	if refundedAmount > 0 {
		return nil
	}

	merchant, err := svc.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant for baofu profit sharing bill: %w", err)
	}
	orderSource := baofuProfitSharingOrderSource(order)
	config, err := ResolveBaofuProfitSharingConfigStrict(ctx, svc.store, orderSource, merchant)
	if err != nil {
		return fmt.Errorf("resolve baofu profit sharing config: %w", err)
	}

	_, err = NewBaofuProfitSharingService(svc.store).CreatePendingOrder(ctx, BaofuProfitSharingOrderInput{
		PaymentOrderID:  paymentOrder.ID,
		MerchantID:      order.MerchantID,
		OperatorID:      config.OperatorID,
		PlatformOwnerID: 0,
		OrderSource:     orderSource,
		TotalAmountFen:  paymentOrder.Amount,
		DeliveryFeeFen:  order.DeliveryFee,
		PlatformRateBps: config.PlatformRateBps,
		OperatorRateBps: config.OperatorRateBps,
		OutOrderNo:      fmt.Sprintf("BFPS%dO%d", paymentOrder.ID, order.ID),
	})
	if err != nil {
		return fmt.Errorf("ensure baofu profit sharing bill: %w", err)
	}
	return nil
}

func baofuProfitSharingOrderSource(order db.Order) string {
	return BaofuProfitSharingOrderSource(order)
}

func profitSharingPercentToBps(rate int32) int32 {
	if rate <= 0 {
		return 0
	}
	return rate * 100
}

func (svc *PaymentFactService) createReservationPaymentOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, reservationPayment ApplyReservationPaymentFactResult) (*db.PaymentDomainOutbox, error) {
	if !reservationPayment.Processed || reservationPayment.ReservationID == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(reservationPaymentSucceededOutboxPayload{
		PaymentOrderID:           reservationPayment.PaymentOrder.ID,
		ReservationID:            reservationPayment.ReservationID,
		BusinessType:             reservationPayment.PaymentOrder.BusinessType,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal reservation payment outbox payload: %w", err)
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventReservationPaymentSucceeded,
		AggregateType: db.PaymentDomainOutboxAggregatePaymentOrder,
		AggregateID:   reservationPayment.PaymentOrder.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create reservation payment outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) createOrderRefundOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, domainResult appliedPaymentFactDomainResult) (*db.PaymentDomainOutbox, error) {
	if domainResult.OrderRefund == nil {
		return nil, nil
	}

	refundResult := *domainResult.OrderRefund
	var (
		eventType string
		payload   []byte
		err       error
	)
	switch refundResult.RefundStatus {
	case riderDepositRefundStatusSuccess:
		eventType = db.PaymentDomainOutboxEventOrderRefundSucceeded
		payload, err = json.Marshal(orderRefundSucceededOutboxPayload{
			RefundOrderID:            refundResult.RefundOrderID,
			PaymentOrderID:           refundResult.PaymentOrderID,
			OrderID:                  refundResult.OrderID,
			UserID:                   refundResult.UserID,
			OutRefundNo:              refundResult.OutRefundNo,
			RefundAmount:             refundResult.RefundAmount,
			RefundStatus:             refundResult.RefundStatus,
			RefundID:                 refundResult.RefundID,
			ExternalPaymentFactID:    fact.ID,
			PaymentFactApplicationID: application.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal order refund succeeded outbox payload: %w", err)
		}
	case riderDepositRefundStatusAbnormal:
		eventType = db.PaymentDomainOutboxEventOrderRefundAbnormal
		payload, err = json.Marshal(orderRefundAbnormalOutboxPayload{
			RefundOrderID:            refundResult.RefundOrderID,
			PaymentOrderID:           refundResult.PaymentOrderID,
			OrderID:                  refundResult.OrderID,
			OutRefundNo:              refundResult.OutRefundNo,
			RefundStatus:             refundResult.RefundStatus,
			RefundID:                 refundResult.RefundID,
			ExternalPaymentFactID:    fact.ID,
			PaymentFactApplicationID: application.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal order refund abnormal outbox payload: %w", err)
		}
	default:
		return nil, nil
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     eventType,
		AggregateType: db.PaymentDomainOutboxAggregateRefundOrder,
		AggregateID:   refundResult.RefundOrderID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create order refund outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) maybeRequestRiderReceiverPresent(_ context.Context, _ db.PaymentOrder) error {
	return nil
}

func (svc *PaymentFactService) createRiderDepositRefundOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, domainResult appliedPaymentFactDomainResult) (*db.PaymentDomainOutbox, error) {
	if domainResult.RiderDepositRefund == nil || domainResult.RiderDepositRefund.RefundStatus != riderDepositRefundStatusAbnormal {
		return nil, nil
	}

	refundResult := *domainResult.RiderDepositRefund
	payload, err := json.Marshal(riderDepositRefundAbnormalOutboxPayload{
		RefundOrderID:            refundResult.RefundOrderID,
		PaymentOrderID:           refundResult.PaymentOrderID,
		OutRefundNo:              refundResult.OutRefundNo,
		RefundStatus:             refundResult.RefundStatus,
		RefundID:                 refundResult.RefundID,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal rider deposit refund abnormal outbox payload: %w", err)
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventRiderDepositRefundAbnormal,
		AggregateType: db.PaymentDomainOutboxAggregateRefundOrder,
		AggregateID:   refundResult.RefundOrderID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create rider deposit refund abnormal outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) createReservationRefundOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, domainResult appliedPaymentFactDomainResult) (*db.PaymentDomainOutbox, error) {
	if domainResult.ReservationRefund == nil || domainResult.ReservationRefund.RefundStatus != riderDepositRefundStatusAbnormal {
		return nil, nil
	}

	refundResult := *domainResult.ReservationRefund
	payload, err := json.Marshal(reservationRefundAbnormalOutboxPayload{
		RefundOrderID:            refundResult.RefundOrderID,
		PaymentOrderID:           refundResult.PaymentOrderID,
		ReservationID:            refundResult.ReservationID,
		OutRefundNo:              refundResult.OutRefundNo,
		RefundStatus:             refundResult.RefundStatus,
		RefundID:                 refundResult.RefundID,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal reservation refund abnormal outbox payload: %w", err)
	}

	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventReservationRefundAbnormal,
		AggregateType: db.PaymentDomainOutboxAggregateRefundOrder,
		AggregateID:   refundResult.RefundOrderID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create reservation refund abnormal outbox: %w", err)
	}
	return &outbox, nil
}

func profitSharingResultForOutbox(terminalStatus string) (string, error) {
	switch terminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		return "SUCCESS", nil
	case db.ExternalPaymentTerminalStatusClosed:
		return "CLOSED", nil
	case db.ExternalPaymentTerminalStatusFailed:
		return "FAILED", nil
	default:
		return "", fmt.Errorf("unsupported profit sharing terminal status %q for outbox", terminalStatus)
	}
}

func profitSharingFactFailReason(rawResource []byte) (string, error) {
	if len(rawResource) == 0 {
		return "", nil
	}
	var payload struct {
		FailReason string `json:"fail_reason"`
	}
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return "", fmt.Errorf("decode profit sharing fact resource: %w", err)
	}
	return payload.FailReason, nil
}

func profitSharingReturnFactFailReason(rawResource []byte) (string, error) {
	if len(rawResource) == 0 {
		return "", nil
	}
	var payload struct {
		FailReason string `json:"fail_reason"`
	}
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return "", fmt.Errorf("decode profit sharing return fact resource: %w", err)
	}
	return payload.FailReason, nil
}
