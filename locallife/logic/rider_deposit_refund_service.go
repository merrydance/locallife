package logic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

var (
	ErrRiderProfileNotFound              = errors.New("rider profile not found")
	ErrRiderAccountNotActivated          = errors.New("rider account has not been activated")
	ErrRiderDepositFrozen                = errors.New("rider deposit is currently frozen")
	ErrRiderAvailableDepositInsufficient = errors.New("insufficient available balance")
	ErrRiderHasActiveDeliveries          = errors.New("rider has active delivery orders")
)

const (
	riderDepositWithdrawalIdempotencyScope = "rider_deposit_withdrawal"
	riderDepositWithdrawStatusSuccess      = "success"
	riderDepositWithdrawStatusProcessing   = "processing"
	riderDepositRefundStatusSuccess        = "SUCCESS"
	riderDepositRefundStatusFailed         = "FAILED"
	riderDepositRefundStatusAbnormal       = "ABNORMAL"
	riderDepositRefundStatusClosed         = "CLOSED"
	riderDepositRefundOrderObjectType      = "refund_order"
)

type RiderDepositRefundService struct {
	store             db.Store
	paymentClient     wechat.DirectPaymentClientInterface
	paymentCommandSvc *PaymentCommandService
}

type SubmitRiderDepositWithdrawalInput struct {
	UserID         int64
	Amount         int64
	Remark         string
	IdempotencyKey string
}

type RiderDepositWithdrawalRefundItem struct {
	RefundOrder  db.RefundOrder
	PaymentOrder db.PaymentOrder
	Status       string
}

type SubmitRiderDepositWithdrawalResult struct {
	RequestedAmount int64
	AcceptedAmount  int64
	Status          string
	Refunds         []RiderDepositWithdrawalRefundItem
}

func NewRiderDepositRefundService(
	store db.Store,
	paymentClient wechat.DirectPaymentClientInterface,
) *RiderDepositRefundService {
	return &RiderDepositRefundService{
		store:             store,
		paymentClient:     paymentClient,
		paymentCommandSvc: NewPaymentCommandService(store),
	}
}

func (s *RiderDepositRefundService) SubmitWithdrawal(ctx context.Context, input SubmitRiderDepositWithdrawalInput) (SubmitRiderDepositWithdrawalResult, error) {
	var result SubmitRiderDepositWithdrawalResult

	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("Idempotency-Key header is required"))
	}
	requestHash := riderDepositWithdrawalRequestHash(input)

	if s.paymentClient == nil {
		return result, fmt.Errorf("payment client not configured")
	}

	rider, err := s.store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, ErrRiderProfileNotFound)
		}
		return result, err
	}

	if rider.Status != db.RiderStatusApproved && rider.Status != db.RiderStatusActive {
		return result, NewRequestError(http.StatusBadRequest, ErrRiderAccountNotActivated)
	}
	withdrawalProcessingAmount, err := s.store.GetPendingRiderDepositRefundAmountByUserID(ctx, input.UserID)
	if err != nil {
		return result, fmt.Errorf("get pending rider deposit refund amount: %w", err)
	}
	availability := db.CalculateRiderDepositAvailability(rider, withdrawalProcessingAmount)
	if rider.FrozenDeposit == 0 && input.Amount > availability.AvailableDeposit {
		return result, NewRequestError(http.StatusBadRequest, ErrRiderAvailableDepositInsufficient)
	}

	if rider.FrozenDeposit == 0 {
		activeDeliveries, err := s.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
		if err != nil {
			return result, fmt.Errorf("list rider active deliveries: %w", err)
		}
		if len(activeDeliveries) > 0 {
			return result, NewRequestError(http.StatusBadRequest, ErrRiderHasActiveDeliveries)
		}
	}

	prepareResult, err := s.store.PrepareRiderDepositRefundTx(ctx, db.PrepareRiderDepositRefundTxParams{
		RiderID:                rider.ID,
		UserID:                 input.UserID,
		Amount:                 input.Amount,
		Remark:                 input.Remark,
		IdempotencyKey:         idempotencyKey,
		IdempotencyRequestHash: requestHash,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			unwrapped := errors.Unwrap(err)
			if unwrapped == nil {
				unwrapped = err
			}
			return result, NewRequestError(statusCode, unwrapped)
		}
		if errors.Is(err, db.ErrRiderDepositFrozen) {
			return result, NewRequestError(http.StatusConflict, ErrRiderDepositFrozen)
		}
		if errors.Is(err, db.ErrInsufficientDeposit) {
			return result, NewRequestError(http.StatusBadRequest, ErrRiderAvailableDepositInsufficient)
		}
		return result, fmt.Errorf("prepare rider deposit refund: %w", err)
	}

	result = SubmitRiderDepositWithdrawalResult{
		RequestedAmount: input.Amount,
		Status:          riderDepositWithdrawStatusProcessing,
		Refunds:         make([]RiderDepositWithdrawalRefundItem, 0, len(prepareResult.RefundPlans)),
	}
	var firstRefundSubmissionErr error

	for _, plan := range prepareResult.RefundPlans {
		if prepareResult.IdempotencyReplayed && plan.RefundOrder.Status != "pending" {
			itemStatus := riderDepositWithdrawStatusProcessing
			if plan.RefundOrder.Status == "success" {
				itemStatus = riderDepositWithdrawStatusSuccess
			}
			result.AcceptedAmount += plan.RefundOrder.RefundAmount
			result.Refunds = append(result.Refunds, RiderDepositWithdrawalRefundItem{
				RefundOrder:  plan.RefundOrder,
				PaymentOrder: plan.SourcePaymentOrder,
				Status:       itemStatus,
			})
			continue
		}

		wxRefund, refundErr := createDirectRefundContract(ctx, s.paymentClient, &wechatcontracts.DirectRefundRequest{
			OutTradeNo:  plan.SourcePaymentOrder.OutTradeNo,
			OutRefundNo: plan.RefundOrder.OutRefundNo,
			Reason:      input.Remark,
			Amount: &wechatcontracts.DirectRefundRequestAmount{
				Refund:   plan.RefundOrder.RefundAmount,
				Total:    plan.SourcePaymentOrder.Amount,
				Currency: wechatcontracts.DirectRefundCurrencyCNY,
			},
		})
		if refundErr != nil {
			if isDirectRefundAlreadyFullyRefundedError(refundErr) {
				resolveErr := s.resolveRefund(ctx, plan.RefundOrder.ID, plan.SourcePaymentOrder, riderDepositRefundStatusSuccess, "", true)
				if resolveErr != nil {
					return result, fmt.Errorf("reconcile already refunded rider deposit credit: %w; original error: %v", resolveErr, LoggableError(refundErr))
				}
				s.recordRiderDepositRefundCommandRejected(ctx, plan, refundErr)

				result.AcceptedAmount += plan.RefundOrder.RefundAmount
				result.Refunds = append(result.Refunds, RiderDepositWithdrawalRefundItem{
					RefundOrder:  plan.RefundOrder,
					PaymentOrder: plan.SourcePaymentOrder,
					Status:       riderDepositWithdrawStatusSuccess,
				})

				log.Warn().Err(LoggableError(refundErr)).Int64("refund_order_id", plan.RefundOrder.ID).Int64("payment_order_id", plan.SourcePaymentOrder.ID).Msg("rider deposit source payment already fully refunded upstream, reconciled stale local credit")
				continue
			}

			resolveErr := s.ResolveRefund(ctx, plan.RefundOrder.ID, plan.SourcePaymentOrder, riderDepositRefundStatusFailed, "")
			if resolveErr != nil {
				return result, fmt.Errorf("request rider deposit refund failed: %w; compensation failed: %v", LoggableError(refundErr), resolveErr)
			}
			s.recordRiderDepositRefundCommandRejected(ctx, plan, refundErr)
			if firstRefundSubmissionErr == nil {
				firstRefundSubmissionErr = mapDirectRefundCreateError(refundErr)
			}
			log.Warn().Err(LoggableError(refundErr)).Int64("refund_order_id", plan.RefundOrder.ID).Msg("rider deposit refund request failed, compensation applied")
			continue
		}

		itemStatus := riderDepositWithdrawStatusProcessing
		switch wxRefund.Status {
		case wechatcontracts.DirectRefundStatusSuccess:
			err = s.MarkRefundProcessing(ctx, plan.RefundOrder.ID, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("mark rider refund accepted: %w", err)
			}
			s.recordRiderDepositRefundCommandAccepted(ctx, plan, wxRefund)
		case wechatcontracts.DirectRefundStatusProcessing:
			err = s.MarkRefundProcessing(ctx, plan.RefundOrder.ID, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("mark rider refund processing: %w", err)
			}
			s.recordRiderDepositRefundCommandAccepted(ctx, plan, wxRefund)
		case wechatcontracts.DirectRefundStatusClosed:
			err = s.MarkRefundProcessing(ctx, plan.RefundOrder.ID, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("mark rider refund closed response accepted: %w", err)
			}
			s.recordRiderDepositRefundCommandAccepted(ctx, plan, wxRefund)
		case wechatcontracts.DirectRefundStatusAbnormal:
			err = s.MarkRefundProcessing(ctx, plan.RefundOrder.ID, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("mark rider refund abnormal response accepted: %w", err)
			}
			s.recordRiderDepositRefundCommandAccepted(ctx, plan, wxRefund)
		default:
			return result, fmt.Errorf("unexpected rider refund status: %s", wxRefund.Status)
		}

		result.AcceptedAmount += plan.RefundOrder.RefundAmount
		result.Refunds = append(result.Refunds, RiderDepositWithdrawalRefundItem{
			RefundOrder:  plan.RefundOrder,
			PaymentOrder: plan.SourcePaymentOrder,
			Status:       itemStatus,
		})
	}

	if result.AcceptedAmount == 0 {
		if firstRefundSubmissionErr != nil {
			return result, firstRefundSubmissionErr
		}
		return result, fmt.Errorf("rider withdrawal refund submission failed")
	}

	if result.AcceptedAmount == input.Amount {
		allSuccess := true
		for _, item := range result.Refunds {
			if item.Status != riderDepositWithdrawStatusSuccess {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			result.Status = riderDepositWithdrawStatusSuccess
		}
	}

	return result, nil
}

func riderDepositWithdrawalRequestHash(input SubmitRiderDepositWithdrawalInput) string {
	parts := []string{
		"v1",
		riderDepositWithdrawalIdempotencyScope,
		strconv.FormatInt(input.UserID, 10),
		strconv.FormatInt(input.Amount, 10),
		strings.TrimSpace(input.Remark),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func (s *RiderDepositRefundService) resolveRefund(ctx context.Context, refundOrderID int64, paymentOrder db.PaymentOrder, refundStatus string, refundID string, drainRemainingCredit bool) error {
	switch refundStatus {
	case riderDepositRefundStatusSuccess, riderDepositRefundStatusFailed, riderDepositRefundStatusAbnormal, riderDepositRefundStatusClosed:
	default:
		return fmt.Errorf("unsupported rider deposit refund status: %s", refundStatus)
	}

	result, err := s.store.ResolveRiderDepositRefundTx(ctx, db.ResolveRiderDepositRefundTxParams{
		RefundOrderID:        refundOrderID,
		RefundStatus:         refundStatus,
		RefundID:             refundID,
		DrainRemainingCredit: drainRemainingCredit,
	})
	if err != nil {
		return fmt.Errorf("resolve rider deposit refund tx: %w", err)
	}

	if refundStatus == riderDepositRefundStatusSuccess {
		if result.ReconciledAmount > 0 {
			if _, err := s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID); err != nil {
				return fmt.Errorf("mark payment order refunded after stale credit reconciliation: %w", err)
			}
		} else {
			s.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
		}
		s.maybeRequestRiderReceiverAbsent(ctx, paymentOrder.UserID)
	}

	return nil
}

func (s *RiderDepositRefundService) MarkRefundProcessing(ctx context.Context, refundOrderID int64, refundID string) error {
	_, err := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrderID,
		RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
	})
	if err != nil {
		return fmt.Errorf("update refund order to processing: %w", err)
	}
	return nil
}

func (s *RiderDepositRefundService) recordRiderDepositRefundCommandAccepted(ctx context.Context, plan db.RiderDepositRefundPlan, wxRefund *wechatcontracts.DirectRefundResponse) {
	if s.paymentCommandSvc == nil || wxRefund == nil {
		return
	}

	refundID := wxRefund.RefundID
	_, err := s.paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRiderDepositRefundCommandInput(plan, db.ExternalPaymentCommandStatusAccepted, &refundID, nil, nil, riderDepositRefundCommandSnapshot(map[string]string{
		"out_refund_no": plan.RefundOrder.OutRefundNo,
		"refund_id":     wxRefund.RefundID,
		"status":        wxRefund.Status,
	})))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", plan.RefundOrder.ID).
			Str("out_refund_no", plan.RefundOrder.OutRefundNo).
			Msg("record rider deposit refund command accepted failed")
	}
}

func (s *RiderDepositRefundService) recordRiderDepositRefundCommandRejected(ctx context.Context, plan db.RiderDepositRefundPlan, refundErr error) {
	if s.paymentCommandSvc == nil {
		return
	}

	errorCode, errorMessage := directRefundCommandErrorFields(refundErr)
	_, err := s.paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRiderDepositRefundCommandInput(plan, db.ExternalPaymentCommandStatusRejected, nil, errorCode, errorMessage, riderDepositRefundCommandSnapshot(map[string]string{
		"out_refund_no": plan.RefundOrder.OutRefundNo,
		"error_code":    stringValue(errorCode),
		"error_message": stringValue(errorMessage),
	})))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", plan.RefundOrder.ID).
			Str("out_refund_no", plan.RefundOrder.OutRefundNo).
			Msg("record rider deposit refund command rejected failed")
	}
}

func dbRiderDepositRefundCommandInput(
	plan db.RiderDepositRefundPlan,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := riderDepositRefundOrderObjectType
	businessObjectID := plan.RefundOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    plan.RefundOrder.OutRefundNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func directRefundCommandErrorFields(err error) (*string, *string) {
	loggableErr := LoggableError(err)
	var wxErr *wechat.WechatPayError
	if errors.As(loggableErr, &wxErr) {
		return stringPtrIfNotEmpty(wxErr.Code), stringPtrIfNotEmpty(wxErr.Message)
	}
	if loggableErr == nil {
		return nil, nil
	}
	return nil, stringPtrIfNotEmpty(loggableErr.Error())
}

func riderDepositRefundCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func stringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *RiderDepositRefundService) ResolveRefund(ctx context.Context, refundOrderID int64, paymentOrder db.PaymentOrder, refundStatus string, refundID string) error {
	return s.resolveRefund(ctx, refundOrderID, paymentOrder, refundStatus, refundID, false)
}

func (s *RiderDepositRefundService) maybeRequestRiderReceiverAbsent(ctx context.Context, userID int64) {
	_ = ctx
	_ = userID
}

func (s *RiderDepositRefundService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) {
	totalSuccessfulRefunded, err := s.store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg("failed to get total successful refunded amount")
		return
	}
	if totalSuccessfulRefunded >= paymentAmount {
		if _, dbErr := s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrderID).Msg("failed to mark payment order as refunded")
		}
	}
}
