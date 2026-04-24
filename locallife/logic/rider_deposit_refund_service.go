package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	riderDepositWithdrawStatusSuccess    = "success"
	riderDepositWithdrawStatusProcessing = "processing"
	riderDepositRefundStatusSuccess      = "SUCCESS"
	riderDepositRefundStatusFailed       = "FAILED"
	riderDepositRefundStatusAbnormal     = "ABNORMAL"
	riderDepositRefundStatusClosed       = "CLOSED"
)

type RiderDepositRefundService struct {
	store         db.Store
	paymentClient wechat.DirectPaymentClientInterface
	receiverSync  *ProfitSharingReceiverSyncService
}

type SubmitRiderDepositWithdrawalInput struct {
	UserID int64
	Amount int64
	Remark string
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
	ecommerceClients ...wechat.EcommerceClientInterface,
) *RiderDepositRefundService {
	var receiverSync *ProfitSharingReceiverSyncService
	if len(ecommerceClients) > 0 && ecommerceClients[0] != nil {
		receiverSync = NewProfitSharingReceiverService(store, ecommerceClients[0])
	}

	return &RiderDepositRefundService{
		store:         store,
		paymentClient: paymentClient,
		receiverSync:  receiverSync,
	}
}

func (s *RiderDepositRefundService) SubmitWithdrawal(ctx context.Context, input SubmitRiderDepositWithdrawalInput) (SubmitRiderDepositWithdrawalResult, error) {
	var result SubmitRiderDepositWithdrawalResult

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
	if rider.FrozenDeposit > 0 {
		return result, NewRequestError(http.StatusConflict, ErrRiderDepositFrozen)
	}

	availableBalance := rider.DepositAmount - rider.FrozenDeposit
	if input.Amount > availableBalance {
		return result, NewRequestError(http.StatusBadRequest, ErrRiderAvailableDepositInsufficient)
	}

	activeDeliveries, err := s.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		return result, fmt.Errorf("list rider active deliveries: %w", err)
	}
	if len(activeDeliveries) > 0 {
		return result, NewRequestError(http.StatusBadRequest, ErrRiderHasActiveDeliveries)
	}

	prepareResult, err := s.store.PrepareRiderDepositRefundTx(ctx, db.PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  input.Amount,
		Remark:  input.Remark,
	})
	if err != nil {
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
			if firstRefundSubmissionErr == nil {
				firstRefundSubmissionErr = mapDirectRefundCreateError(refundErr)
			}
			log.Warn().Err(LoggableError(refundErr)).Int64("refund_order_id", plan.RefundOrder.ID).Msg("rider deposit refund request failed, compensation applied")
			continue
		}

		itemStatus := riderDepositWithdrawStatusProcessing
		switch wxRefund.Status {
		case wechatcontracts.DirectRefundStatusSuccess:
			err = s.ResolveRefund(ctx, plan.RefundOrder.ID, plan.SourcePaymentOrder, riderDepositRefundStatusSuccess, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("settle rider refund success: %w", err)
			}
			itemStatus = riderDepositWithdrawStatusSuccess
		case wechatcontracts.DirectRefundStatusProcessing:
			err = s.MarkRefundProcessing(ctx, plan.RefundOrder.ID, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("mark rider refund processing: %w", err)
			}
		case wechatcontracts.DirectRefundStatusClosed:
			err = s.ResolveRefund(ctx, plan.RefundOrder.ID, plan.SourcePaymentOrder, riderDepositRefundStatusClosed, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("close rider refund: %w", err)
			}
			continue
		case wechatcontracts.DirectRefundStatusAbnormal:
			err = s.ResolveRefund(ctx, plan.RefundOrder.ID, plan.SourcePaymentOrder, riderDepositRefundStatusAbnormal, wxRefund.RefundID)
			if err != nil {
				return result, fmt.Errorf("fail rider refund: %w", err)
			}
			continue
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
		if err := s.maybeDeleteRiderReceiver(ctx, paymentOrder.UserID); err != nil {
			return err
		}
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

func (s *RiderDepositRefundService) ResolveRefund(ctx context.Context, refundOrderID int64, paymentOrder db.PaymentOrder, refundStatus string, refundID string) error {
	return s.resolveRefund(ctx, refundOrderID, paymentOrder, refundStatus, refundID, false)
}

func (s *RiderDepositRefundService) maybeDeleteRiderReceiver(ctx context.Context, userID int64) error {
	if s.receiverSync == nil {
		return nil
	}

	rider, err := s.store.GetRiderByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get rider for receiver cleanup: %w", err)
	}
	if rider.DepositAmount > 0 || rider.FrozenDeposit > 0 {
		return nil
	}

	if err := s.receiverSync.DeleteRiderReceiver(ctx, rider); err != nil {
		return fmt.Errorf("delete rider profit sharing receiver: %w", err)
	}

	return nil
}

func (s *RiderDepositRefundService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) {
	totalRefunded, err := s.store.GetTotalRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg("failed to get total refunded amount")
		return
	}
	if totalRefunded >= paymentAmount {
		if _, dbErr := s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrderID).Msg("failed to mark payment order as refunded")
		}
	}
}
