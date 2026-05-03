package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

const (
	paymentFactConsumerProfitSharingDomain          = "profit_sharing_domain"
	paymentFactConsumerOrderDomain                  = "order_domain"
	paymentFactConsumerClaimRecoveryDomain          = "claim_recovery_domain"
	paymentFactConsumerRiderDepositDomain           = "rider_deposit_domain"
	paymentFactConsumerReservationDomain            = "reservation_domain"
	paymentFactConsumerApplymentDomain              = "applyment_domain"
	paymentFactConsumerSettlementDomain             = "settlement_domain"
	paymentFactConsumerMerchantFundsDomain          = "merchant_funds_domain"
	paymentFactBusinessObjectPaymentOrder           = "payment_order"
	paymentFactBusinessObjectRefundOrder            = "refund_order"
	paymentFactBusinessObjectProfitSharingOrder     = "profit_sharing_order"
	paymentFactBusinessObjectProfitSharingReturn    = "profit_sharing_return"
	paymentFactBusinessObjectApplyment              = "ordinary_service_provider_applyment"
	paymentFactBusinessObjectMerchantPaymentConfig  = "merchant_payment_config"
	paymentFactBusinessObjectMerchantCancelWithdraw = "merchant_cancel_withdraw_application"
	paymentFactBusinessObjectWithdrawalRecord       = "withdrawal_record"
	paymentFactApplicationRetryDelay                = 5 * time.Minute
)

type ApplyExternalPaymentFactApplicationResult struct {
	Application                   db.ExternalPaymentFactApplication
	Fact                          db.ExternalPaymentFact
	Outbox                        *db.PaymentDomainOutbox
	OrderPayment                  *ApplyOrderPaymentFactResult
	ClaimRecoveryPayment          *claimRecoveryPaymentDomainResult
	SettlementApplicationTracking *settlementApplicationTrackingDomainResult
	SettlementVerification        *settlementVerificationDomainResult
	MerchantCancelWithdraw        *merchantCancelWithdrawDomainResult
	MerchantWithdraw              *merchantWithdrawDomainResult
	ReservationPayment            *ApplyReservationPaymentFactResult
	Applied                       bool
	Skipped                       bool
}

type appliedPaymentFactDomainResult struct {
	ProfitSharingOrder            *db.ProfitSharingOrder
	ProfitSharingReturn           *profitSharingReturnDomainResult
	Applyment                     *applymentDomainResult
	OrderPayment                  *ApplyOrderPaymentFactResult
	ClaimRecoveryPayment          *claimRecoveryPaymentDomainResult
	SettlementApplicationTracking *settlementApplicationTrackingDomainResult
	SettlementVerification        *settlementVerificationDomainResult
	MerchantCancelWithdraw        *merchantCancelWithdrawDomainResult
	MerchantWithdraw              *merchantWithdrawDomainResult
	ReservationPayment            *ApplyReservationPaymentFactResult
	RiderDepositPayment           *riderDepositPaymentDomainResult
	RiderDepositRefund            *riderDepositRefundDomainResult
	OrderRefund                   *orderRefundDomainResult
	ReservationRefund             *reservationRefundDomainResult
}

type profitSharingReturnDomainResult struct {
	ProfitSharingReturnID int64
	RefundOrderID         int64
	Status                string
	ReturnID              string
}

type riderDepositPaymentDomainResult struct {
	PaymentOrder db.PaymentOrder
	Processed    bool
}

type claimRecoveryPaymentDomainResult struct {
	PaymentOrder db.PaymentOrder
	Processed    bool
}

type settlementVerificationDomainResult struct {
	Applyment        db.EcommerceApplyment
	MerchantID       int64
	Status           string
	VerifyFailReason string
}

type settlementApplicationTrackingDomainResult struct {
	PaymentConfig db.MerchantPaymentConfig
	ApplicationNo string
	VerifyResult  string
}

type merchantCancelWithdrawDomainResult struct {
	Application db.MerchantCancelWithdrawApplication
}

type merchantWithdrawDomainResult struct {
	WithdrawalRecord db.WithdrawalRecord
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

type applymentDomainResult struct {
	Applyment      db.EcommerceApplyment
	MerchantID     int64
	Activated      bool
	PendingStatus  string
	TerminalStatus string
}

type applymentActivatedOutboxPayload struct {
	ApplymentID              int64  `json:"applyment_id"`
	MerchantID               int64  `json:"merchant_id"`
	OutRequestNo             string `json:"out_request_no"`
	SubMchID                 string `json:"sub_mch_id"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type applymentTerminalStateOutboxPayload struct {
	ApplymentID              int64  `json:"applyment_id"`
	MerchantID               int64  `json:"merchant_id"`
	OutRequestNo             string `json:"out_request_no"`
	ApplymentStatus          string `json:"applyment_status"`
	RejectReason             string `json:"reject_reason,omitempty"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
}

type applymentPendingStateOutboxPayload struct {
	ApplymentID              int64  `json:"applyment_id"`
	MerchantID               int64  `json:"merchant_id"`
	OutRequestNo             string `json:"out_request_no"`
	ApplymentStatus          string `json:"applyment_status"`
	ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
	PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
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
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("external payment fact %d raw_resource is not valid JSON", fact.ID))
	}
	result.Fact = fact

	domainResult, err := svc.applyExternalPaymentFactToDomain(ctx, application, fact)
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, err)
	}
	result.OrderPayment = domainResult.OrderPayment
	result.ClaimRecoveryPayment = domainResult.ClaimRecoveryPayment
	result.SettlementApplicationTracking = domainResult.SettlementApplicationTracking
	result.SettlementVerification = domainResult.SettlementVerification
	result.MerchantCancelWithdraw = domainResult.MerchantCancelWithdraw
	result.MerchantWithdraw = domainResult.MerchantWithdraw
	result.ReservationPayment = domainResult.ReservationPayment

	outbox, err := svc.createPaymentDomainOutboxForAppliedFact(ctx, application, fact, domainResult)
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, err)
	}
	result.Outbox = outbox

	processedAt := svc.now().UTC()
	if _, err := svc.store.UpdateExternalPaymentFactProcessingStatus(ctx, db.UpdateExternalPaymentFactProcessingStatusParams{
		ID:               fact.ID,
		ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized,
		ProcessedAt:      pgtype.Timestamptz{Time: processedAt, Valid: true},
	}); err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("mark external payment fact terminalized: %w", err))
	}

	applied, err := svc.store.MarkExternalPaymentFactApplicationApplied(ctx, db.MarkExternalPaymentFactApplicationAppliedParams{
		ID:        application.ID,
		AppliedAt: pgtype.Timestamptz{Time: processedAt, Valid: true},
	})
	if err != nil {
		return result, svc.markExternalPaymentFactApplicationFailed(ctx, application, fmt.Errorf("mark external payment fact application applied: %w", err))
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
		result.ProfitSharingOrder = &profitSharingOrder
		return result, nil
	case application.Consumer == paymentFactConsumerApplymentDomain && application.BusinessObjectType == paymentFactBusinessObjectApplyment:
		applymentResult, err := svc.applyApplymentFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.Applyment = &applymentResult
		return result, nil
	case application.Consumer == paymentFactConsumerSettlementDomain && application.BusinessObjectType == paymentFactBusinessObjectMerchantPaymentConfig:
		settlementApplicationTracking, err := svc.applySettlementApplicationFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.SettlementApplicationTracking = &settlementApplicationTracking
		return result, nil
	case application.Consumer == paymentFactConsumerSettlementDomain && application.BusinessObjectType == paymentFactBusinessObjectApplyment:
		settlementVerification, err := svc.applySettlementVerificationFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.SettlementVerification = &settlementVerification
		return result, nil
	case application.Consumer == paymentFactConsumerMerchantFundsDomain && application.BusinessObjectType == paymentFactBusinessObjectMerchantCancelWithdraw:
		merchantCancelWithdraw, err := svc.applyMerchantCancelWithdrawFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.MerchantCancelWithdraw = &merchantCancelWithdraw
		return result, nil
	case application.Consumer == paymentFactConsumerMerchantFundsDomain && application.BusinessObjectType == paymentFactBusinessObjectWithdrawalRecord:
		merchantWithdraw, err := svc.applyMerchantWithdrawFact(ctx, application, fact)
		if err != nil {
			return result, err
		}
		result.MerchantWithdraw = &merchantWithdraw
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

func (svc *PaymentFactService) applyApplymentFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (applymentDomainResult, error) {
	var result applymentDomainResult
	if err := validateApplymentFactApplication(application, fact); err != nil {
		return result, err
	}

	applyment, err := svc.store.GetEcommerceApplyment(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get applyment: %w", err)
	}
	if applyment.SubjectType != "merchant" {
		return result, fmt.Errorf("applyment %d subject type %q is not merchant", applyment.ID, applyment.SubjectType)
	}

	subMchID := applymentFactPayloadSubMchID(fact.RawResource)
	resolvedWechatApplymentID := applyment.ApplymentID
	if upstreamApplymentID := applymentFactPayloadApplymentID(fact.RawResource); upstreamApplymentID.Valid && !resolvedWechatApplymentID.Valid {
		resolvedWechatApplymentID = upstreamApplymentID
	}
	resolvedStatus := NormalizeResolvedApplymentStatus(MapWechatApplymentStateToStatus(fact.UpstreamState), strings.TrimSpace(subMchID) != "")

	switch resolvedStatus {
	case "finish":
		if subMchID == "" {
			return result, fmt.Errorf("applyment fact %d missing sub_mch_id", fact.ID)
		}
		accountAuthorizeState := applymentFactPayloadAccountAuthorizeState(fact.RawResource)
		if accountAuthorizeState == "" {
			accountAuthorizeState = strings.TrimSpace(applyment.AccountAuthorizeState.String)
		}
		if err := svc.store.ApplymentSubMchActivationTx(ctx, db.ApplymentSubMchActivationTxParams{
			ApplymentID:           applyment.ID,
			WechatApplymentID:     resolvedWechatApplymentID,
			SubjectType:           applyment.SubjectType,
			SubjectID:             applyment.SubjectID,
			SubMchID:              subMchID,
			AccountAuthorizeState: accountAuthorizeState,
		}); err != nil {
			return result, fmt.Errorf("activate applyment: %w", err)
		}

		updatedApplyment, err := svc.store.GetEcommerceApplyment(ctx, application.BusinessObjectID)
		if err != nil {
			return result, fmt.Errorf("reload applyment after activation: %w", err)
		}
		result = applymentDomainResult{
			Applyment:  updatedApplyment,
			MerchantID: updatedApplyment.SubjectID,
			Activated:  true,
		}
		return result, nil
	case "rejected", "frozen", "canceled":
		rejectReason := applymentFactRejectReason(fact.RawResource)
		if !rejectReason.Valid {
			rejectReason = applyment.RejectReason
		}
		updatedApplyment, err := svc.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
			ID:                 applyment.ID,
			ApplymentID:        resolvedWechatApplymentID,
			Status:             resolvedStatus,
			RejectReason:       rejectReason,
			SignUrl:            applyment.SignUrl,
			SignState:          applyment.SignState,
			LegalValidationUrl: applyment.LegalValidationUrl,
			AccountValidation:  applyment.AccountValidation,
			SubMchID:           pgtype.Text{String: subMchID, Valid: strings.TrimSpace(subMchID) != ""},
		})
		if err != nil {
			return result, fmt.Errorf("update applyment terminal status: %w", err)
		}
		result = applymentDomainResult{
			Applyment:      updatedApplyment,
			MerchantID:     updatedApplyment.SubjectID,
			TerminalStatus: resolvedStatus,
		}
		return result, nil
	case "account_need_verify", "to_be_confirmed", "to_be_signed":
		signURL := applymentFactPayloadSignURL(fact.RawResource)
		if !signURL.Valid {
			signURL = applyment.SignUrl
		}
		signState := applymentFactPayloadSignState(fact.RawResource)
		if !signState.Valid {
			signState = applyment.SignState
		}
		legalValidationURL := applymentFactPayloadLegalValidationURL(fact.RawResource)
		if !legalValidationURL.Valid {
			legalValidationURL = applyment.LegalValidationUrl
		}
		accountValidation := applymentFactPayloadAccountValidation(fact.RawResource)
		if len(accountValidation) == 0 {
			accountValidation = applyment.AccountValidation
		}
		updatedApplyment, err := svc.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
			ID:                 applyment.ID,
			ApplymentID:        resolvedWechatApplymentID,
			Status:             resolvedStatus,
			RejectReason:       pgtype.Text{},
			SignUrl:            signURL,
			SignState:          signState,
			LegalValidationUrl: legalValidationURL,
			AccountValidation:  accountValidation,
			SubMchID:           pgtype.Text{String: subMchID, Valid: strings.TrimSpace(subMchID) != ""},
		})
		if err != nil {
			return result, fmt.Errorf("update applyment pending status: %w", err)
		}
		result = applymentDomainResult{
			Applyment:     updatedApplyment,
			MerchantID:    updatedApplyment.SubjectID,
			PendingStatus: resolvedStatus,
		}
		return result, nil
	default:
		return result, fmt.Errorf("unsupported applyment upstream state %q", fact.UpstreamState)
	}
}

func validateApplymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Capability != db.ExternalPaymentCapabilityApplyment {
		return fmt.Errorf("payment fact %d is not a wechat applyment fact", fact.ID)
	}
	if fact.Channel != db.PaymentChannelEcommerce && fact.Channel != db.PaymentChannelOrdinaryServiceProvider {
		return fmt.Errorf("payment fact %d applyment channel %q is not supported", fact.ID, fact.Channel)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectApplyment {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerApplyment {
		return fmt.Errorf("payment fact %d business owner %q is not applyment", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectApplyment {
		return fmt.Errorf("payment fact %d business object type %q is not %s", fact.ID, fact.BusinessObjectType.String, paymentFactBusinessObjectApplyment)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	resolvedStatus := NormalizeResolvedApplymentStatus(MapWechatApplymentStateToStatus(fact.UpstreamState), strings.TrimSpace(applymentFactPayloadSubMchID(fact.RawResource)) != "")
	switch resolvedStatus {
	case "finish", "rejected", "frozen", "canceled":
		if !fact.IsTerminal {
			return fmt.Errorf("payment fact %d is not terminal", fact.ID)
		}
		if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess && fact.TerminalStatus != db.ExternalPaymentTerminalStatusFailed && fact.TerminalStatus != db.ExternalPaymentTerminalStatusClosed {
			return fmt.Errorf("payment fact %d terminal status %q is not supported for applyment", fact.ID, fact.TerminalStatus)
		}
		return nil
	case "account_need_verify", "to_be_confirmed", "to_be_signed":
		if fact.IsTerminal {
			return fmt.Errorf("payment fact %d pending applyment state %q must not be terminal", fact.ID, fact.UpstreamState)
		}
		if fact.TerminalStatus != db.ExternalPaymentTerminalStatusProcessing {
			return fmt.Errorf("payment fact %d terminal status %q is not supported for applyment pending state", fact.ID, fact.TerminalStatus)
		}
		return nil
	default:
		return fmt.Errorf("payment fact %d applyment upstream state %q does not resolve to supported terminal status", fact.ID, fact.UpstreamState)
	}
}

func (svc *PaymentFactService) applySettlementApplicationFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (settlementApplicationTrackingDomainResult, error) {
	var result settlementApplicationTrackingDomainResult
	if err := validateSettlementApplicationFactApplication(application, fact); err != nil {
		return result, err
	}

	paymentConfig, err := svc.store.GetMerchantPaymentConfigBySubMchID(ctx, strings.TrimSpace(fact.ExternalObjectKey))
	if err != nil {
		return result, fmt.Errorf("get merchant payment config by sub mch id: %w", err)
	}
	if paymentConfig.ID != application.BusinessObjectID {
		return result, fmt.Errorf("payment config %d does not match application object id %d", paymentConfig.ID, application.BusinessObjectID)
	}

	applicationNo := settlementApplicationFactPayloadApplicationNo(fact)
	if applicationNo == "" {
		return result, fmt.Errorf("payment fact %d missing settlement application_no", fact.ID)
	}

	updatedPaymentConfig, err := svc.store.UpdateMerchantPaymentConfigSettlementApplication(ctx, db.UpdateMerchantPaymentConfigSettlementApplicationParams{
		MerchantID:                             paymentConfig.MerchantID,
		LatestSettlementApplicationNo:          pgtype.Text{String: applicationNo, Valid: true},
		LatestSettlementApplicationSubmittedAt: pgtype.Timestamptz{},
	})
	if err != nil {
		return result, fmt.Errorf("update merchant settlement application tracking: %w", err)
	}

	result = settlementApplicationTrackingDomainResult{
		PaymentConfig: updatedPaymentConfig,
		ApplicationNo: applicationNo,
		VerifyResult:  strings.TrimSpace(fact.UpstreamState),
	}
	return result, nil
}

func validateSettlementApplicationFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelOrdinaryServiceProvider || fact.Capability != db.ExternalPaymentCapabilitySettlement {
		return fmt.Errorf("payment fact %d is not a wechat ordinary service provider settlement fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectSettlement {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerMerchantFunds {
		return fmt.Errorf("payment fact %d business owner %q is not merchant_funds", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectMerchantPaymentConfig {
		return fmt.Errorf("payment fact %d business object type %q is not %s", fact.ID, fact.BusinessObjectType.String, paymentFactBusinessObjectMerchantPaymentConfig)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	if strings.TrimSpace(fact.ExternalObjectKey) == "" {
		return fmt.Errorf("payment fact %d missing settlement sub_mch_id", fact.ID)
	}
	if settlementApplicationFactPayloadApplicationNo(fact) == "" {
		return fmt.Errorf("payment fact %d missing settlement application_no", fact.ID)
	}
	switch strings.TrimSpace(fact.UpstreamState) {
	case wechatcontracts.SubMerchantSettlementApplicationAuditSuccess, wechatcontracts.SubMerchantSettlementApplicationAuditFail:
		if !fact.IsTerminal {
			return fmt.Errorf("payment fact %d terminal settlement application fact must be terminal", fact.ID)
		}
		if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess && fact.TerminalStatus != db.ExternalPaymentTerminalStatusFailed {
			return fmt.Errorf("payment fact %d terminal status %q is not supported for settlement application terminal state", fact.ID, fact.TerminalStatus)
		}
	case wechatcontracts.SubMerchantSettlementApplicationAuditing:
		if fact.IsTerminal {
			return fmt.Errorf("payment fact %d auditing settlement application fact must not be terminal", fact.ID)
		}
		if fact.TerminalStatus != db.ExternalPaymentTerminalStatusProcessing {
			return fmt.Errorf("payment fact %d terminal status %q is not supported for settlement application auditing state", fact.ID, fact.TerminalStatus)
		}
	default:
		return fmt.Errorf("payment fact %d settlement application upstream state %q is not supported", fact.ID, fact.UpstreamState)
	}
	return nil
}

func settlementApplicationFactPayloadApplicationNo(fact db.ExternalPaymentFact) string {
	if fact.ExternalSecondaryKey.Valid {
		if applicationNo := strings.TrimSpace(fact.ExternalSecondaryKey.String); applicationNo != "" {
			return applicationNo
		}
	}
	if len(fact.RawResource) == 0 {
		return ""
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(fact.RawResource, &payload); err != nil {
		return ""
	}
	var applicationNo string
	if err := json.Unmarshal(payload["application_no"], &applicationNo); err != nil {
		return ""
	}
	return strings.TrimSpace(applicationNo)
}

func (svc *PaymentFactService) applySettlementVerificationFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (settlementVerificationDomainResult, error) {
	var result settlementVerificationDomainResult
	if err := validateSettlementVerificationFactApplication(application, fact); err != nil {
		return result, err
	}

	applyment, err := svc.store.GetEcommerceApplyment(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get applyment for settlement verification: %w", err)
	}
	if applyment.SubjectType != "merchant" {
		return result, fmt.Errorf("applyment %d subject type %q is not merchant", applyment.ID, applyment.SubjectType)
	}
	if !applyment.SubMchID.Valid || strings.TrimSpace(applyment.SubMchID.String) == "" {
		return result, fmt.Errorf("applyment %d missing sub_mch_id", applyment.ID)
	}
	if externalObjectKey := strings.TrimSpace(fact.ExternalObjectKey); externalObjectKey != "" && externalObjectKey != strings.TrimSpace(applyment.SubMchID.String) {
		return result, fmt.Errorf("settlement fact %d sub_mch_id %q does not match applyment %d sub_mch_id %q", fact.ID, externalObjectKey, applyment.ID, strings.TrimSpace(applyment.SubMchID.String))
	}

	checkCount, hasCheckCount, err := settlementVerificationFactPayloadCheckCount(fact.RawResource)
	if err != nil {
		return result, err
	}
	status, failReason, err := settlementVerificationStateForApplication(fact, checkCount, hasCheckCount)
	if err != nil {
		return result, err
	}

	params := db.UpdateEcommerceApplymentSettlementVerificationParams{
		ID:                         applyment.ID,
		SettlementVerifyStatus:     pgtype.Text{String: status, Valid: status != ""},
		SettlementVerifyFailReason: pgtype.Text{String: failReason, Valid: true},
	}
	if firstTradeAt, ok, err := settlementVerificationFactPayloadTime(fact.RawResource, "settlement_verify_first_trade_at"); err != nil {
		return result, err
	} else if ok {
		params.SettlementVerifyFirstTradeAt = pgtype.Timestamptz{Time: firstTradeAt, Valid: true}
	}
	if checkedAt, ok, err := settlementVerificationFactPayloadTime(fact.RawResource, "settlement_verify_last_checked_at"); err != nil {
		return result, err
	} else if ok {
		params.SettlementVerifyLastCheckedAt = pgtype.Timestamptz{Time: checkedAt, Valid: true}
	}
	if hasCheckCount {
		params.SettlementVerifyCheckCount = pgtype.Int4{Int32: checkCount, Valid: true}
	}

	updatedApplyment, err := svc.store.UpdateEcommerceApplymentSettlementVerification(ctx, params)
	if err != nil {
		return result, fmt.Errorf("update settlement verification state: %w", err)
	}

	result = settlementVerificationDomainResult{
		Applyment:        updatedApplyment,
		MerchantID:       updatedApplyment.SubjectID,
		Status:           status,
		VerifyFailReason: failReason,
	}
	return result, nil
}

func validateSettlementVerificationFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelOrdinaryServiceProvider || fact.Capability != db.ExternalPaymentCapabilitySettlement {
		return fmt.Errorf("payment fact %d is not a wechat ordinary service provider settlement fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectSettlement {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerMerchantFunds {
		return fmt.Errorf("payment fact %d business owner %q is not merchant_funds", fact.ID, fact.BusinessOwner.String)
	}
	if application.BusinessObjectID == 0 {
		return fmt.Errorf("payment fact application %d missing applyment id", application.ID)
	}
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess, db.ExternalPaymentTerminalStatusFailed:
		if !fact.IsTerminal {
			return fmt.Errorf("payment fact %d terminal settlement fact must be terminal", fact.ID)
		}
	case db.ExternalPaymentTerminalStatusProcessing:
		if fact.IsTerminal {
			return fmt.Errorf("payment fact %d processing settlement fact must not be terminal", fact.ID)
		}
	default:
		return fmt.Errorf("payment fact %d terminal status %q is not supported for settlement verification", fact.ID, fact.TerminalStatus)
	}
	return nil
}

func (svc *PaymentFactService) applyMerchantCancelWithdrawFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (merchantCancelWithdrawDomainResult, error) {
	var result merchantCancelWithdrawDomainResult
	if err := validateMerchantCancelWithdrawFactApplication(application, fact); err != nil {
		return result, err
	}

	current, err := svc.store.GetMerchantCancelWithdrawApplication(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get merchant cancel withdraw application: %w", err)
	}

	queryResp, err := merchantCancelWithdrawFactPayloadQueryResponse(fact)
	if err != nil {
		return result, err
	}

	syncParams, err := buildMerchantCancelWithdrawSyncParams(current, queryResp, svc.now())
	if err != nil {
		return result, err
	}
	updated, err := svc.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, syncParams)
	if err != nil {
		return result, fmt.Errorf("update merchant cancel withdraw application sync: %w", err)
	}

	result = merchantCancelWithdrawDomainResult{Application: updated}
	return result, nil
}

func validateMerchantCancelWithdrawFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelEcommerce || fact.Capability != db.ExternalPaymentCapabilityCancelWithdraw {
		return fmt.Errorf("payment fact %d is not a wechat ecommerce cancel withdraw fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectCancelWithdraw {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerMerchantFunds {
		return fmt.Errorf("payment fact %d business owner %q is not merchant_funds", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectMerchantCancelWithdraw {
		return fmt.Errorf("payment fact %d business object type %q is not %s", fact.ID, fact.BusinessObjectType.String, paymentFactBusinessObjectMerchantCancelWithdraw)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess, db.ExternalPaymentTerminalStatusFailed:
		if !fact.IsTerminal {
			return fmt.Errorf("payment fact %d terminal cancel withdraw fact must be terminal", fact.ID)
		}
	case db.ExternalPaymentTerminalStatusProcessing:
		if fact.IsTerminal {
			return fmt.Errorf("payment fact %d processing cancel withdraw fact must not be terminal", fact.ID)
		}
	default:
		return fmt.Errorf("payment fact %d terminal status %q is not supported for merchant cancel withdraw", fact.ID, fact.TerminalStatus)
	}
	return nil
}

func merchantCancelWithdrawFactPayloadQueryResponse(fact db.ExternalPaymentFact) (*wechatcontracts.CancelWithdrawQueryResponse, error) {
	if len(fact.RawResource) == 0 {
		return nil, fmt.Errorf("payment fact %d missing cancel withdraw raw resource", fact.ID)
	}

	type merchantCancelWithdrawFactPayload struct {
		ApplymentID              string                                                `json:"applyment_id"`
		OutRequestNo             string                                                `json:"out_request_no"`
		SubMchID                 string                                                `json:"sub_mch_id"`
		CancelState              string                                                `json:"cancel_state"`
		CancelStateDescription   string                                                `json:"cancel_state_description"`
		Withdraw                 string                                                `json:"withdraw"`
		WithdrawState            string                                                `json:"withdraw_state"`
		WithdrawStateDescription string                                                `json:"withdraw_state_description"`
		ModifyTime               string                                                `json:"modify_time"`
		ConfirmCancelURL         string                                                `json:"confirm_cancel_url"`
		AccountInfo              []wechatcontracts.CancelWithdrawAccountInfo           `json:"account_info"`
		AccountWithdrawResult    []wechatcontracts.CancelWithdrawAccountWithdrawResult `json:"account_withdraw_result"`
	}

	var payload merchantCancelWithdrawFactPayload
	if err := json.Unmarshal(fact.RawResource, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal cancel withdraw fact payload: %w", err)
	}

	queryResp := &wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:              strings.TrimSpace(payload.ApplymentID),
		OutRequestNo:             strings.TrimSpace(payload.OutRequestNo),
		SubMchID:                 strings.TrimSpace(payload.SubMchID),
		CancelState:              strings.TrimSpace(payload.CancelState),
		CancelStateDescription:   strings.TrimSpace(payload.CancelStateDescription),
		Withdraw:                 strings.TrimSpace(payload.Withdraw),
		WithdrawState:            strings.TrimSpace(payload.WithdrawState),
		WithdrawStateDescription: strings.TrimSpace(payload.WithdrawStateDescription),
		ModifyTime:               strings.TrimSpace(payload.ModifyTime),
		AccountInfo:              payload.AccountInfo,
		AccountWithdrawResult:    payload.AccountWithdrawResult,
	}
	if queryResp.ApplymentID == "" && fact.ExternalSecondaryKey.Valid {
		queryResp.ApplymentID = strings.TrimSpace(fact.ExternalSecondaryKey.String)
	}
	if queryResp.OutRequestNo == "" {
		queryResp.OutRequestNo = strings.TrimSpace(fact.ExternalObjectKey)
	}
	if queryResp.CancelState == "" {
		queryResp.CancelState = strings.TrimSpace(fact.UpstreamState)
	}
	if confirmCancelURL := strings.TrimSpace(payload.ConfirmCancelURL); confirmCancelURL != "" {
		queryResp.ConfirmCancel = &wechatcontracts.CancelWithdrawConfirmCancel{ConfirmCancelURL: confirmCancelURL}
	}
	return queryResp, nil
}

func buildMerchantCancelWithdrawSyncParams(current db.MerchantCancelWithdrawApplication, query *wechatcontracts.CancelWithdrawQueryResponse, now time.Time) (db.UpdateMerchantCancelWithdrawApplicationSyncParams, error) {
	params, err := BuildMerchantCancelWithdrawSyncParams(current, query, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, "", false, now)
	if err != nil {
		return db.UpdateMerchantCancelWithdrawApplicationSyncParams{}, fmt.Errorf("build merchant cancel withdraw sync params: %w", err)
	}
	return params, nil
}

func (svc *PaymentFactService) applyMerchantWithdrawFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) (merchantWithdrawDomainResult, error) {
	var result merchantWithdrawDomainResult
	if err := validateMerchantWithdrawFactApplication(application, fact); err != nil {
		return result, err
	}

	withdrawalRecord, err := svc.store.GetWithdrawalRecord(ctx, application.BusinessObjectID)
	if err != nil {
		return result, fmt.Errorf("get withdrawal record: %w", err)
	}

	status := merchantWithdrawStatusForApplication(fact.UpstreamState)
	reason := merchantWithdrawFactPayloadReason(fact.RawResource)
	reasonArg := pgtype.Text{}
	if reason != "" {
		reasonArg = pgtype.Text{String: reason, Valid: true}
	}
	clearReason := reason == "" && withdrawalRecord.Reason.Valid
	if status == withdrawalRecord.Status && !reasonArg.Valid && !clearReason {
		result = merchantWithdrawDomainResult{WithdrawalRecord: withdrawalRecord}
		return result, nil
	}

	updatedWithdrawalRecord, err := svc.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
		ID:          withdrawalRecord.ID,
		Status:      status,
		Reason:      reasonArg,
		ClearReason: clearReason,
	})
	if err != nil {
		return result, fmt.Errorf("update withdrawal status: %w", err)
	}

	result = merchantWithdrawDomainResult{WithdrawalRecord: updatedWithdrawalRecord}
	return result, nil
}

func validateMerchantWithdrawFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Channel != db.PaymentChannelEcommerce || fact.Capability != db.ExternalPaymentCapabilityWithdraw {
		return fmt.Errorf("payment fact %d is not a wechat ecommerce withdraw fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectWithdraw {
		return fmt.Errorf("payment fact %d has unsupported external object type %q", fact.ID, fact.ExternalObjectType)
	}
	if fact.BusinessOwner.Valid && fact.BusinessOwner.String != db.ExternalPaymentBusinessOwnerMerchantFunds {
		return fmt.Errorf("payment fact %d business owner %q is not merchant_funds", fact.ID, fact.BusinessOwner.String)
	}
	if fact.BusinessObjectType.Valid && fact.BusinessObjectType.String != paymentFactBusinessObjectWithdrawalRecord {
		return fmt.Errorf("payment fact %d business object type %q is not %s", fact.ID, fact.BusinessObjectType.String, paymentFactBusinessObjectWithdrawalRecord)
	}
	if fact.BusinessObjectID.Valid && fact.BusinessObjectID.Int64 != application.BusinessObjectID {
		return fmt.Errorf("payment fact %d business object id %d does not match application object id %d", fact.ID, fact.BusinessObjectID.Int64, application.BusinessObjectID)
	}
	switch fact.TerminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess, db.ExternalPaymentTerminalStatusFailed:
		if !fact.IsTerminal {
			return fmt.Errorf("payment fact %d terminal withdraw fact must be terminal", fact.ID)
		}
	case db.ExternalPaymentTerminalStatusProcessing:
		if fact.IsTerminal {
			return fmt.Errorf("payment fact %d processing withdraw fact must not be terminal", fact.ID)
		}
	default:
		return fmt.Errorf("payment fact %d terminal status %q is not supported for merchant withdraw", fact.ID, fact.TerminalStatus)
	}
	return nil
}

func merchantWithdrawFactPayloadReason(rawResource []byte) string {
	if len(rawResource) == 0 {
		return ""
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return ""
	}
	var reason string
	if err := json.Unmarshal(payload["reason"], &reason); err != nil {
		return ""
	}
	return strings.TrimSpace(reason)
}

func merchantWithdrawStatusForApplication(status string) string {
	switch strings.ToUpper(status) {
	case wechatcontracts.FundManagementWithdrawStatusSuccess:
		return "success"
	case wechatcontracts.FundManagementWithdrawStatusFail, wechatcontracts.FundManagementWithdrawStatusRefund, wechatcontracts.FundManagementWithdrawStatusClose:
		return "failed"
	default:
		return "pending"
	}
}

func settlementVerificationStateForApplication(fact db.ExternalPaymentFact, checkCount int32, hasCheckCount bool) (string, string, error) {
	switch strings.TrimSpace(fact.UpstreamState) {
	case wechatcontracts.SubMerchantSettlementVerifyResultFail:
		return "fail", settlementVerificationFactPayloadFailReason(fact.RawResource), nil
	case wechatcontracts.SubMerchantSettlementVerifyResultSuccess:
		return "success", "", nil
	case wechatcontracts.SubMerchantSettlementVerifyResultVerifying:
		if hasCheckCount && checkCount >= 3 {
			return "success", "", nil
		}
		return "verifying", "", nil
	default:
		if hasCheckCount && checkCount >= 3 {
			return "success", "", nil
		}
		return "", "", fmt.Errorf("payment fact %d settlement upstream state %q is not supported", fact.ID, fact.UpstreamState)
	}
}

func settlementVerificationFactPayloadFailReason(rawResource []byte) string {
	if len(rawResource) == 0 {
		return ""
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return ""
	}
	var reason string
	if err := json.Unmarshal(payload["verify_fail_reason"], &reason); err != nil {
		return ""
	}
	return strings.TrimSpace(reason)
}

func settlementVerificationFactPayloadCheckCount(rawResource []byte) (int32, bool, error) {
	if len(rawResource) == 0 {
		return 0, false, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return 0, false, fmt.Errorf("unmarshal settlement verification payload: %w", err)
	}
	rawValue, ok := payload["settlement_verify_check_count"]
	if !ok {
		return 0, false, nil
	}
	var checkCount int32
	if err := json.Unmarshal(rawValue, &checkCount); err != nil {
		return 0, false, fmt.Errorf("unmarshal settlement verification check count: %w", err)
	}
	return checkCount, true, nil
}

func settlementVerificationFactPayloadTime(rawResource []byte, field string) (time.Time, bool, error) {
	if len(rawResource) == 0 {
		return time.Time{}, false, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawResource, &payload); err != nil {
		return time.Time{}, false, fmt.Errorf("unmarshal settlement verification payload: %w", err)
	}
	rawValue, ok := payload[field]
	if !ok {
		return time.Time{}, false, nil
	}
	var value string
	if err := json.Unmarshal(rawValue, &value); err != nil {
		return time.Time{}, false, fmt.Errorf("unmarshal settlement verification time %s: %w", field, err)
	}
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return time.Time{}, false, nil
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, trimmedValue)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse settlement verification time %s: %w", field, err)
	}
	return parsedTime, true, nil
}

func validateOrderPaymentFactApplication(application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact) error {
	if !fact.IsTerminal {
		return fmt.Errorf("payment fact %d is not terminal", fact.ID)
	}
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("payment fact %d terminal status %q is not success", fact.ID, fact.TerminalStatus)
	}
	if !isWechatMainBusinessPaymentFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported wechat main business payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectPayment && fact.ExternalObjectType != db.ExternalPaymentObjectCombinedPayment {
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
	if fact.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return fmt.Errorf("payment fact %d terminal status %q is not success", fact.ID, fact.TerminalStatus)
	}
	if !isWechatMainBusinessPaymentFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported wechat main business payment fact", fact.ID)
	}
	if fact.ExternalObjectType != db.ExternalPaymentObjectPayment && fact.ExternalObjectType != db.ExternalPaymentObjectCombinedPayment {
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

func isWechatMainBusinessPaymentFact(fact db.ExternalPaymentFact) bool {
	if fact.Provider != db.ExternalPaymentProviderWechat {
		return false
	}
	if fact.Capability != db.ExternalPaymentCapabilityPartnerJSAPIPayment && fact.Capability != db.ExternalPaymentCapabilityCombinePayment {
		return false
	}
	return fact.Channel == db.PaymentChannelEcommerce || fact.Channel == db.PaymentChannelOrdinaryServiceProvider
}

func isWechatMainBusinessProfitSharingFact(fact db.ExternalPaymentFact) bool {
	if fact.Provider != db.ExternalPaymentProviderWechat || fact.Capability != db.ExternalPaymentCapabilityProfitSharing {
		return false
	}
	return fact.Channel == db.PaymentChannelEcommerce || fact.Channel == db.PaymentChannelOrdinaryServiceProvider
}

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

	refundService := NewRiderDepositRefundService(svc.store, nil, svc.ecommerceClient)
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
		if refundOrder.Status != riderDepositRefundStatusSuccess {
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
		if refundOrder.Status != riderDepositRefundStatusClosed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
			if err != nil {
				return result, fmt.Errorf("update order refund order to closed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	case db.ExternalPaymentTerminalStatusFailed:
		if refundOrder.Status != riderDepositRefundStatusFailed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			if err != nil {
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
	if !isWechatMainBusinessRefundFact(fact) {
		return fmt.Errorf("payment fact %d is not a wechat main-business refund fact", fact.ID)
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
		if refundOrder.Status != riderDepositRefundStatusSuccess {
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
		if refundOrder.Status != riderDepositRefundStatusClosed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
			if err != nil {
				return result, fmt.Errorf("update reservation refund order to closed: %w", err)
			}
			refundOrder = updatedRefundOrder
		}
	case db.ExternalPaymentTerminalStatusFailed:
		if refundOrder.Status != riderDepositRefundStatusFailed {
			updatedRefundOrder, err := svc.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			if err != nil {
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
	if !isWechatMainBusinessRefundFact(fact) {
		return fmt.Errorf("payment fact %d is not a wechat main-business refund fact", fact.ID)
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

func isWechatMainBusinessRefundFact(fact db.ExternalPaymentFact) bool {
	if fact.Provider != db.ExternalPaymentProviderWechat {
		return false
	}
	if fact.Channel == db.PaymentChannelEcommerce && fact.Capability == db.ExternalPaymentCapabilityEcommerceRefund {
		return true
	}
	return fact.Channel == db.PaymentChannelOrdinaryServiceProvider && fact.Capability == db.ExternalPaymentCapabilityPartnerRefund
}

func isOrderRefundPaymentOrder(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder
}

func (svc *PaymentFactService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) error {
	totalRefunded, err := svc.store.GetTotalRefundedByPaymentOrder(ctx, paymentOrderID)
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
		return svc.applyProfitSharingSuccessFact(ctx, application.BusinessObjectID)
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
	if !isWechatMainBusinessProfitSharingFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported wechat profit sharing fact", fact.ID)
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

func (svc *PaymentFactService) applyProfitSharingSuccessFact(ctx context.Context, profitSharingOrderID int64) (db.ProfitSharingOrder, error) {
	order, err := svc.store.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("get profit sharing order: %w", err)
	}
	if order.Status == db.ProfitSharingOrderStatusFinished {
		return order, nil
	}
	if order.Status != db.ProfitSharingOrderStatusProcessing {
		return db.ProfitSharingOrder{}, fmt.Errorf("profit sharing order %d status %q cannot apply success fact", profitSharingOrderID, order.Status)
	}
	updated, err := svc.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("update profit sharing order to finished: %w", err)
	}
	return updated, nil
}

func (svc *PaymentFactService) applyProfitSharingFailedFact(ctx context.Context, profitSharingOrderID int64) (db.ProfitSharingOrder, error) {
	order, err := svc.store.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("get profit sharing order: %w", err)
	}
	if order.Status == db.ProfitSharingOrderStatusFailed {
		return order, nil
	}
	updated, err := svc.store.UpdateProfitSharingOrderToFailed(ctx, profitSharingOrderID)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("update profit sharing order to failed: %w", err)
	}
	return updated, nil
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
	if !isWechatMainBusinessProfitSharingFact(fact) {
		return fmt.Errorf("payment fact %d is not a supported wechat profit sharing return fact", fact.ID)
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
	if svc.refundCreator == nil {
		return fmt.Errorf("wechat refund client not configured for payment channel %q", paymentOrder.PaymentChannel)
	}

	reason := ""
	if refundOrder.RefundReason.Valid {
		reason = refundOrder.RefundReason.String
	}

	refundID, err := svc.createRefundAfterProfitSharingReturns(ctx, paymentOrder, refundOrder, paymentConfig.SubMchID, reason)
	if err != nil {
		if _, dbErr := svc.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("call wechat refund API: %w; update refund order to failed: %v", err, dbErr)
		}
		return fmt.Errorf("call wechat refund API: %w", err)
	}

	if _, err := svc.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
	}); err != nil {
		return fmt.Errorf("update refund order to processing: %w", err)
	}
	return nil
}

func (svc *PaymentFactService) createRefundAfterProfitSharingReturns(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	subMchID string,
	reason string,
) (string, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		ordinaryCreator, ok := svc.refundCreator.(paymentFactOrdinaryRefundCreator)
		if !ok {
			return "", fmt.Errorf("ordinary service provider refund client not configured")
		}
		refundResp, err := ordinaryCreator.CreateOrdinaryServiceProviderRefund(ctx, ospcontracts.RefundCreateRequest{
			SubMchID:    subMchID,
			OutTradeNo:  paymentOrder.OutTradeNo,
			OutRefundNo: refundOrder.OutRefundNo,
			Reason:      reason,
			NotifyURL:   ordinaryCreator.OrdinaryServiceProviderRefundNotifyURL(),
			Amount: ospcontracts.RefundAmountRequest{
				Refund:   refundOrder.RefundAmount,
				Total:    paymentOrder.Amount,
				Currency: ospcontracts.CurrencyCNY,
			},
		})
		if err != nil {
			return "", mapOrdinaryServiceProviderRefundCreateError(err)
		}
		if refundResp == nil {
			return "", nil
		}
		return refundResp.RefundID, nil
	}
	if !paymentOrderUsesEcommerceChannel(paymentOrder) {
		return "", fmt.Errorf("payment channel %q does not support refund after profit sharing return", paymentOrder.PaymentChannel)
	}
	refundResp, err := svc.refundCreator.CreateEcommerceRefund(ctx, &wechatcontracts.EcommerceRefundRequest{
		SubMchID:    subMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: refundOrder.OutRefundNo,
		Reason:      reason,
		Amount: &wechatcontracts.EcommerceRefundRequestAmount{
			Refund:   refundOrder.RefundAmount,
			Total:    paymentOrder.Amount,
			Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
		},
	})
	if err != nil {
		return "", err
	}
	if refundResp == nil {
		return "", nil
	}
	return refundResp.RefundID, nil
}

func (svc *PaymentFactService) createPaymentDomainOutboxForAppliedFact(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, domainResult appliedPaymentFactDomainResult) (*db.PaymentDomainOutbox, error) {
	if domainResult.ProfitSharingReturn != nil {
		return nil, nil
	}
	if domainResult.Applyment != nil {
		if domainResult.Applyment.Activated {
			return svc.createApplymentActivatedOutbox(ctx, application, fact, *domainResult.Applyment)
		}
		if domainResult.Applyment.PendingStatus != "" {
			return svc.createApplymentPendingStateOutbox(ctx, application, fact, *domainResult.Applyment)
		}
		return svc.createApplymentTerminalStateOutbox(ctx, application, fact, *domainResult.Applyment)
	}
	if domainResult.OrderPayment != nil {
		return svc.createOrderPaymentOutbox(ctx, application, fact, *domainResult.OrderPayment)
	}
	if domainResult.ClaimRecoveryPayment != nil {
		return nil, nil
	}
	if domainResult.SettlementVerification != nil {
		return nil, nil
	}
	if domainResult.SettlementApplicationTracking != nil {
		return nil, nil
	}
	if domainResult.MerchantWithdraw != nil {
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

func (svc *PaymentFactService) createApplymentActivatedOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, result applymentDomainResult) (*db.PaymentDomainOutbox, error) {
	if !result.Activated {
		return nil, nil
	}
	payload, err := json.Marshal(applymentActivatedOutboxPayload{
		ApplymentID:              result.Applyment.ID,
		MerchantID:               result.MerchantID,
		OutRequestNo:             result.Applyment.OutRequestNo,
		SubMchID:                 paymentFactTextValue(result.Applyment.SubMchID),
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal applyment activated outbox payload: %w", err)
	}
	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventApplymentActivated,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   result.Applyment.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create applyment activated outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) createApplymentTerminalStateOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, result applymentDomainResult) (*db.PaymentDomainOutbox, error) {
	if result.TerminalStatus == "" {
		return nil, nil
	}
	payload, err := json.Marshal(applymentTerminalStateOutboxPayload{
		ApplymentID:              result.Applyment.ID,
		MerchantID:               result.MerchantID,
		OutRequestNo:             result.Applyment.OutRequestNo,
		ApplymentStatus:          result.TerminalStatus,
		RejectReason:             paymentFactTextValue(result.Applyment.RejectReason),
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal applyment terminal outbox payload: %w", err)
	}
	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventApplymentTerminalStateReady,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   result.Applyment.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create applyment terminal outbox: %w", err)
	}
	return &outbox, nil
}

func (svc *PaymentFactService) createApplymentPendingStateOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, result applymentDomainResult) (*db.PaymentDomainOutbox, error) {
	if result.PendingStatus == "" {
		return nil, nil
	}
	payload, err := json.Marshal(applymentPendingStateOutboxPayload{
		ApplymentID:              result.Applyment.ID,
		MerchantID:               result.MerchantID,
		OutRequestNo:             result.Applyment.OutRequestNo,
		ApplymentStatus:          result.PendingStatus,
		ExternalPaymentFactID:    fact.ID,
		PaymentFactApplicationID: application.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal applyment pending outbox payload: %w", err)
	}
	outbox, err := svc.store.CreatePaymentDomainOutboxOnce(ctx, db.CreatePaymentDomainOutboxOnceParams{
		EventType:     db.PaymentDomainOutboxEventApplymentPendingStateReady,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   result.Applyment.ID,
		Payload:       payload,
		Status:        db.PaymentDomainOutboxStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create applyment pending outbox: %w", err)
	}
	return &outbox, nil
}

func paymentFactTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func applymentFactPayloadSubMchID(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		SubMchID string `json:"sub_mch_id"`
		Resource struct {
			SubMchID string `json:"sub_mchid"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if strings.TrimSpace(payload.SubMchID) != "" {
		return strings.TrimSpace(payload.SubMchID)
	}
	return strings.TrimSpace(payload.Resource.SubMchID)
}

func applymentFactPayloadApplymentID(raw []byte) pgtype.Int8 {
	if len(raw) == 0 {
		return pgtype.Int8{}
	}
	var payload struct {
		ApplymentID any `json:"applyment_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pgtype.Int8{}
	}
	switch value := payload.ApplymentID.(type) {
	case float64:
		return pgtype.Int8{Int64: int64(value), Valid: value > 0}
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return pgtype.Int8{}
		}
		var parsed int64
		_, err := fmt.Sscanf(trimmed, "%d", &parsed)
		if err != nil || parsed <= 0 {
			return pgtype.Int8{}
		}
		return pgtype.Int8{Int64: parsed, Valid: true}
	default:
		return pgtype.Int8{}
	}
}

func applymentFactRejectReason(raw []byte) pgtype.Text {
	if len(raw) == 0 {
		return pgtype.Text{}
	}
	var payload struct {
		RejectReason string `json:"reject_reason"`
		StateDesc    string `json:"applyment_state_desc"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pgtype.Text{}
	}
	rejectReason := strings.TrimSpace(payload.RejectReason)
	if rejectReason == "" {
		rejectReason = strings.TrimSpace(payload.StateDesc)
	}
	return pgtype.Text{String: rejectReason, Valid: rejectReason != ""}
}

func applymentFactPayloadSignURL(raw []byte) pgtype.Text {
	if len(raw) == 0 {
		return pgtype.Text{}
	}
	var payload struct {
		SignURL string `json:"sign_url"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pgtype.Text{}
	}
	value := strings.TrimSpace(payload.SignURL)
	return pgtype.Text{String: value, Valid: value != ""}
}

func applymentFactPayloadSignState(raw []byte) pgtype.Text {
	if len(raw) == 0 {
		return pgtype.Text{}
	}
	var payload struct {
		SignState string `json:"sign_state"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pgtype.Text{}
	}
	value := strings.TrimSpace(payload.SignState)
	return pgtype.Text{String: value, Valid: value != ""}
}

func applymentFactPayloadLegalValidationURL(raw []byte) pgtype.Text {
	if len(raw) == 0 {
		return pgtype.Text{}
	}
	var payload struct {
		LegalValidationURL string `json:"legal_validation_url"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pgtype.Text{}
	}
	value := strings.TrimSpace(payload.LegalValidationURL)
	return pgtype.Text{String: value, Valid: value != ""}
}

func applymentFactPayloadAccountValidation(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		AccountValidation json.RawMessage `json:"account_validation"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.AccountValidation) == 0 {
		return nil
	}
	validation, err := wechat.UnmarshalEcommerceApplymentAccountValidation(payload.AccountValidation)
	if err != nil {
		return nil
	}
	return wechat.MarshalEcommerceApplymentAccountValidation(validation)
}

func applymentFactPayloadAccountAuthorizeState(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		AccountAuthorizeState string `json:"account_authorize_state"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.AccountAuthorizeState)
}

func (svc *PaymentFactService) createOrderPaymentOutbox(ctx context.Context, application db.ExternalPaymentFactApplication, fact db.ExternalPaymentFact, orderPayment ApplyOrderPaymentFactResult) (*db.PaymentDomainOutbox, error) {
	if !orderPayment.Processed || orderPayment.OrderResult == nil {
		return nil, nil
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

func (svc *PaymentFactService) markExternalPaymentFactApplicationFailed(ctx context.Context, application db.ExternalPaymentFactApplication, applyErr error) error {
	nextRetryAt := svc.now().UTC().Add(paymentFactApplicationRetryDelay)
	_, markErr := svc.store.MarkExternalPaymentFactApplicationFailed(ctx, db.MarkExternalPaymentFactApplicationFailedParams{
		ID:          application.ID,
		LastError:   pgtype.Text{String: applyErr.Error(), Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
	})
	if markErr != nil {
		return fmt.Errorf("%w; mark external payment fact application failed: %v", applyErr, markErr)
	}
	return applyErr
}
