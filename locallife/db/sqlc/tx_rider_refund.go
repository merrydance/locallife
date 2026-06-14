package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
)

const (
	riderDepositRefundType      = "rider_deposit"
	riderDepositFreezeRemark    = "押金提现冻结"
	riderDepositWithdrawRemark  = "押金退款提现成功"
	riderDepositDeductRemark    = "押金原单已退款自动对账扣减"
	riderDepositUnfreezeRemark  = "押金退款失败解冻"
	riderDepositRefundSucceeded = "SUCCESS"
	riderDepositRefundFailed    = "FAILED"
	riderDepositRefundAbnormal  = "ABNORMAL"
	riderDepositRefundClosed    = "CLOSED"
)

type RiderDepositRefundPlan struct {
	RefundOrder         RefundOrder
	SourcePaymentOrder  PaymentOrder
	ReservedCredit      RiderDepositCredit
	FreezeDepositRecord RiderDeposit
}

type PrepareRiderDepositRefundTxParams struct {
	RiderID                int64
	UserID                 int64
	Amount                 int64
	Remark                 string
	IdempotencyKey         string
	IdempotencyRequestHash string
}

type PrepareRiderDepositRefundTxResult struct {
	Rider               Rider
	RefundPlans         []RiderDepositRefundPlan
	FrozenAmount        int64
	IdempotencyReplayed bool
	WithdrawalRequestID int64
}

type ResolveRiderDepositRefundTxParams struct {
	RefundOrderID        int64
	RefundStatus         string
	RefundID             string
	DrainRemainingCredit bool
}

type ResolveRiderDepositRefundTxResult struct {
	RefundOrder      RefundOrder
	Rider            Rider
	DepositLog       RiderDeposit
	Credit           RiderDepositCredit
	Applied          bool
	ReconciledAmount int64
}

func (store *SQLStore) PrepareRiderDepositRefundTx(ctx context.Context, arg PrepareRiderDepositRefundTxParams) (PrepareRiderDepositRefundTxResult, error) {
	var result PrepareRiderDepositRefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		idempotencyKey := strings.TrimSpace(arg.IdempotencyKey)
		idempotencyRequestHash := strings.TrimSpace(arg.IdempotencyRequestHash)
		if arg.UserID == 0 || idempotencyKey == "" || idempotencyRequestHash == "" {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("rider deposit withdrawal idempotency metadata is incomplete")}
		}
		if rider.UserID != arg.UserID {
			return &requestError{statusCode: http.StatusForbidden, err: errors.New("rider does not belong to user")}
		}

		withdrawalRequest, err := q.GetRiderDepositWithdrawalRequestForUpdate(ctx, GetRiderDepositWithdrawalRequestForUpdateParams{
			UserID:         arg.UserID,
			IdempotencyKey: idempotencyKey,
		})
		if err == nil {
			if withdrawalRequest.RequestHash != idempotencyRequestHash {
				return &requestError{statusCode: http.StatusConflict, err: errors.New("idempotency key already used by a different rider deposit withdrawal request")}
			}
			plans, replayErr := loadRiderDepositWithdrawalRefundPlans(ctx, q, withdrawalRequest)
			if replayErr != nil {
				return replayErr
			}
			result.Rider = rider
			result.RefundPlans = plans
			result.FrozenAmount = withdrawalRequest.AcceptedAmount
			result.IdempotencyReplayed = true
			result.WithdrawalRequestID = withdrawalRequest.ID
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get rider deposit withdrawal request: %w", err)
		}

		if rider.Status != RiderStatusApproved && rider.Status != RiderStatusActive {
			return ErrRiderAccountNotActivated
		}

		if rider.FrozenDeposit > 0 {
			return ErrRiderDepositFrozen
		}

		activeDeliveries, err := q.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
		if err != nil {
			return fmt.Errorf("list rider active deliveries: %w", err)
		}
		if len(activeDeliveries) > 0 {
			return ErrRiderHasActiveDeliveries
		}

		withdrawalRequest, err = q.CreateRiderDepositWithdrawalRequest(ctx, CreateRiderDepositWithdrawalRequestParams{
			UserID:          arg.UserID,
			IdempotencyKey:  idempotencyKey,
			RequestHash:     idempotencyRequestHash,
			RequestedAmount: arg.Amount,
			AcceptedAmount:  nil,
			RefundOrderIds:  []byte("[]"),
		})
		if err != nil {
			return fmt.Errorf("create rider deposit withdrawal request: %w", err)
		}

		withdrawalProcessingAmount, err := q.GetPendingRiderDepositRefundAmountByUserID(ctx, rider.UserID)
		if err != nil {
			return fmt.Errorf("get pending rider deposit refund amount: %w", err)
		}
		availability := CalculateRiderDepositAvailability(rider, withdrawalProcessingAmount)
		if arg.Amount > availability.AvailableDeposit {
			return ErrInsufficientDeposit
		}

		credits, err := q.ListActiveRiderDepositCreditsByRiderID(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("list rider deposit credits: %w", err)
		}

		remaining := arg.Amount
		availableAfter := availability.AvailableDeposit
		plans := make([]RiderDepositRefundPlan, 0)

		for _, credit := range credits {
			if remaining == 0 {
				break
			}

			lockedCredit, err := q.GetRiderDepositCreditForUpdate(ctx, credit.ID)
			if err != nil {
				return fmt.Errorf("lock rider deposit credit: %w", err)
			}
			if lockedCredit.RefundableAmount <= 0 {
				continue
			}
			if lockedCredit.Status != riderDepositCreditStatusActive && lockedCredit.Status != riderDepositCreditStatusPartial {
				continue
			}

			refundAmount := remaining
			if refundAmount > lockedCredit.RefundableAmount {
				refundAmount = lockedCredit.RefundableAmount
			}

			reservedCredit, err := q.ConsumeRiderDepositCredit(ctx, ConsumeRiderDepositCreditParams{
				ID:               lockedCredit.ID,
				RefundableAmount: refundAmount,
			})
			if err != nil {
				return fmt.Errorf("reserve rider deposit credit: %w", err)
			}

			sourcePaymentOrder, err := q.GetPaymentOrderForUpdate(ctx, lockedCredit.PaymentOrderID)
			if err != nil {
				return fmt.Errorf("get source payment order for update: %w", err)
			}
			if sourcePaymentOrder.Status != "paid" {
				return fmt.Errorf("source payment order %d is not paid", sourcePaymentOrder.ID)
			}

			outRefundNo, err := util.GenerateOutRefundNo()
			if err != nil {
				return fmt.Errorf("generate out refund no: %w", err)
			}

			refundOrder, err := q.CreateRefundOrder(ctx, CreateRefundOrderParams{
				PaymentOrderID: sourcePaymentOrder.ID,
				RefundType:     riderDepositRefundType,
				RefundAmount:   refundAmount,
				RefundReason:   pgtype.Text{String: arg.Remark, Valid: arg.Remark != ""},
				OutRefundNo:    outRefundNo,
				Status:         "pending",
			})
			if err != nil {
				return fmt.Errorf("create rider deposit refund order: %w", err)
			}

			availableAfter -= refundAmount
			freezeLog, err := q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
				RiderID:        arg.RiderID,
				Amount:         refundAmount,
				Type:           "freeze",
				RelatedOrderID: pgtype.Int8{},
				PaymentOrderID: pgtype.Int8{Int64: sourcePaymentOrder.ID, Valid: true},
				BalanceAfter:   availableAfter,
				Remark:         pgtype.Text{String: riderDepositFreezeRemark, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create rider deposit freeze log: %w", err)
			}

			plans = append(plans, RiderDepositRefundPlan{
				RefundOrder:         refundOrder,
				SourcePaymentOrder:  sourcePaymentOrder,
				ReservedCredit:      reservedCredit,
				FreezeDepositRecord: freezeLog,
			})
			remaining -= refundAmount
		}

		if remaining > 0 {
			return ErrInsufficientDeposit
		}

		refundOrderIDs := make([]int64, 0, len(plans))
		acceptedAmount := int64(0)
		for _, plan := range plans {
			refundOrderIDs = append(refundOrderIDs, plan.RefundOrder.ID)
			acceptedAmount += plan.RefundOrder.RefundAmount
		}
		refundOrderIDPayload, err := json.Marshal(refundOrderIDs)
		if err != nil {
			return fmt.Errorf("marshal rider deposit withdrawal refund order ids: %w", err)
		}
		withdrawalRequest, err = q.UpdateRiderDepositWithdrawalRequestRefundOrders(ctx, UpdateRiderDepositWithdrawalRequestRefundOrdersParams{
			ID:             withdrawalRequest.ID,
			AcceptedAmount: acceptedAmount,
			RefundOrderIds: refundOrderIDPayload,
		})
		if err != nil {
			return fmt.Errorf("update rider deposit withdrawal request refund orders: %w", err)
		}

		updatedRider, err := q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
			ID:            rider.ID,
			DepositAmount: rider.DepositAmount,
			FrozenDeposit: rider.FrozenDeposit + arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("freeze rider deposit: %w", err)
		}

		result.Rider = updatedRider
		result.RefundPlans = plans
		result.FrozenAmount = arg.Amount
		result.WithdrawalRequestID = withdrawalRequest.ID
		return nil
	})

	return result, err
}

func loadRiderDepositWithdrawalRefundPlans(ctx context.Context, q *Queries, request RiderDepositWithdrawalRequest) ([]RiderDepositRefundPlan, error) {
	refundOrderIDs, err := decodeRiderDepositWithdrawalRefundOrderIDs(request.RefundOrderIds)
	if err != nil {
		return nil, fmt.Errorf("decode rider deposit withdrawal refund order ids: %w", err)
	}
	if len(refundOrderIDs) == 0 {
		return nil, fmt.Errorf("rider deposit withdrawal request %d has no refund orders", request.ID)
	}

	rows, err := q.ListRiderDepositWithdrawalRefundOrdersByIDs(ctx, ListRiderDepositWithdrawalRefundOrdersByIDsParams{
		UserID:         request.UserID,
		RefundOrderIds: refundOrderIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list rider deposit withdrawal refund orders: %w", err)
	}
	rowsByRefundOrderID := make(map[int64]ListRiderDepositWithdrawalRefundOrdersByIDsRow, len(rows))
	for _, row := range rows {
		rowsByRefundOrderID[row.RefundOrderID] = row
	}
	if len(rowsByRefundOrderID) != len(refundOrderIDs) {
		return nil, fmt.Errorf("rider deposit withdrawal request %d refund orders are incomplete", request.ID)
	}

	plans := make([]RiderDepositRefundPlan, 0, len(refundOrderIDs))
	for _, refundOrderID := range refundOrderIDs {
		row, ok := rowsByRefundOrderID[refundOrderID]
		if !ok {
			return nil, fmt.Errorf("rider deposit withdrawal request %d refund order %d is missing", request.ID, refundOrderID)
		}
		plans = append(plans, RiderDepositRefundPlan{
			RefundOrder: RefundOrder{
				ID:             row.RefundOrderID,
				PaymentOrderID: row.PaymentOrderID,
				RefundType:     riderDepositRefundType,
				RefundAmount:   row.RefundAmount,
				OutRefundNo:    row.OutRefundNo,
				RefundID:       row.RefundID,
				Status:         row.Status,
				RefundedAt:     row.RefundedAt,
				CreatedAt:      row.CreatedAt,
			},
			SourcePaymentOrder: PaymentOrder{
				ID:           row.PaymentOrderID,
				UserID:       request.UserID,
				BusinessType: "rider_deposit",
				Amount:       row.SourcePaymentAmount,
				OutTradeNo:   row.OutTradeNo,
			},
		})
	}
	return plans, nil
}

func decodeRiderDepositWithdrawalRefundOrderIDs(raw []byte) ([]int64, error) {
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid refund order id %d", id)
		}
	}
	return ids, nil
}

func (store *SQLStore) ResolveRiderDepositRefundTx(ctx context.Context, arg ResolveRiderDepositRefundTxParams) (ResolveRiderDepositRefundTxResult, error) {
	var result ResolveRiderDepositRefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		refundOrder, err := q.GetRefundOrderForUpdate(ctx, arg.RefundOrderID)
		if err != nil {
			return fmt.Errorf("get refund order for update: %w", err)
		}

		paymentOrder, err := q.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get payment order: %w", err)
		}
		if paymentOrder.BusinessType != "rider_deposit" {
			return fmt.Errorf("payment order %d is not rider_deposit", paymentOrder.ID)
		}

		rider, err := q.GetRiderByUserID(ctx, paymentOrder.UserID)
		if err != nil {
			return fmt.Errorf("get rider by user id: %w", err)
		}
		lockedRider, err := q.GetRiderForUpdate(ctx, rider.ID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		switch arg.RefundStatus {
		case riderDepositRefundSucceeded:
			if refundOrder.Status == "success" {
				result.RefundOrder = refundOrder
				result.Rider = lockedRider
				return nil
			}

			if refundOrder.Status == "pending" || refundOrder.Status == "processing" {
				processingRefundID := refundOrder.RefundID
				if arg.RefundID != "" {
					processingRefundID = pgtype.Text{String: arg.RefundID, Valid: true}
				}
				if refundOrder.Status == "pending" {
					refundOrder, err = q.UpdateRefundOrderToProcessing(ctx, UpdateRefundOrderToProcessingParams{
						ID:       refundOrder.ID,
						RefundID: processingRefundID,
					})
					if err != nil {
						return fmt.Errorf("mark rider refund order processing: %w", err)
					}
				}

				refundOrder, err = q.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
				if err != nil {
					return fmt.Errorf("mark rider refund order success: %w", err)
				}
			}

			credit, err := q.GetRiderDepositCreditByPaymentOrderID(ctx, paymentOrder.ID)
			if err != nil {
				return fmt.Errorf("get rider deposit credit: %w", err)
			}

			reconciledAmount := int64(0)
			if arg.DrainRemainingCredit && credit.RefundableAmount > 0 {
				reconciledAmount = credit.RefundableAmount
				credit, err = q.ConsumeRiderDepositCredit(ctx, ConsumeRiderDepositCreditParams{
					ID:               credit.ID,
					RefundableAmount: reconciledAmount,
				})
				if err != nil {
					return fmt.Errorf("consume remaining stale rider deposit credit: %w", err)
				}
			}

			if lockedRider.DepositAmount < refundOrder.RefundAmount+reconciledAmount || lockedRider.FrozenDeposit < refundOrder.RefundAmount {
				return fmt.Errorf("rider deposit state invalid for refund settlement")
			}

			withdrawBalanceAfter := lockedRider.DepositAmount - refundOrder.RefundAmount

			updatedRider, err := q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
				ID:            lockedRider.ID,
				DepositAmount: lockedRider.DepositAmount - refundOrder.RefundAmount - reconciledAmount,
				FrozenDeposit: lockedRider.FrozenDeposit - refundOrder.RefundAmount,
			})
			if err != nil {
				return fmt.Errorf("settle rider deposit refund success: %w", err)
			}

			updatedRider, err = ReconcileRiderOperationalStatus(ctx, q, updatedRider)
			if err != nil {
				return fmt.Errorf("reconcile rider status after refund: %w", err)
			}

			depositLog, err := q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
				RiderID:        lockedRider.ID,
				Amount:         refundOrder.RefundAmount,
				Type:           "withdraw",
				RelatedOrderID: pgtype.Int8{},
				PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
				BalanceAfter:   withdrawBalanceAfter,
				Remark:         pgtype.Text{String: riderDepositWithdrawRemark, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create rider withdraw log: %w", err)
			}

			if reconciledAmount > 0 {
				_, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
					RiderID:        lockedRider.ID,
					Amount:         reconciledAmount,
					Type:           "deduct",
					RelatedOrderID: pgtype.Int8{},
					PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
					BalanceAfter:   updatedRider.DepositAmount - updatedRider.FrozenDeposit,
					Remark:         pgtype.Text{String: riderDepositDeductRemark, Valid: true},
				})
				if err != nil {
					return fmt.Errorf("create rider stale credit deduct log: %w", err)
				}
			}

			result.RefundOrder = refundOrder
			result.Rider = updatedRider
			result.DepositLog = depositLog
			result.Credit = credit
			result.Applied = true
			result.ReconciledAmount = reconciledAmount
			return nil

		case riderDepositRefundFailed, riderDepositRefundAbnormal, riderDepositRefundClosed:
			if refundOrder.Status == "failed" || refundOrder.Status == "closed" {
				result.RefundOrder = refundOrder
				result.Rider = lockedRider
				return nil
			}

			credit, err := q.RestoreRiderDepositCreditByPaymentOrderID(ctx, RestoreRiderDepositCreditByPaymentOrderIDParams{
				PaymentOrderID:   paymentOrder.ID,
				RefundableAmount: refundOrder.RefundAmount,
			})
			if err != nil {
				return fmt.Errorf("restore rider deposit credit: %w", err)
			}

			if lockedRider.FrozenDeposit < refundOrder.RefundAmount {
				return fmt.Errorf("rider frozen deposit insufficient for refund rollback")
			}

			updatedRider, err := q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
				ID:            lockedRider.ID,
				DepositAmount: lockedRider.DepositAmount,
				FrozenDeposit: lockedRider.FrozenDeposit - refundOrder.RefundAmount,
			})
			if err != nil {
				return fmt.Errorf("rollback rider deposit refund freeze: %w", err)
			}

			if arg.RefundStatus == riderDepositRefundClosed {
				refundOrder, err = q.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
				if err != nil {
					return fmt.Errorf("mark rider refund order closed: %w", err)
				}
			} else {
				refundOrder, err = q.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
				if err != nil {
					return fmt.Errorf("mark rider refund order failed: %w", err)
				}
			}

			depositLog, err := q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
				RiderID:        lockedRider.ID,
				Amount:         refundOrder.RefundAmount,
				Type:           "unfreeze",
				RelatedOrderID: pgtype.Int8{},
				PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
				BalanceAfter:   updatedRider.DepositAmount - updatedRider.FrozenDeposit,
				Remark:         pgtype.Text{String: riderDepositUnfreezeRemark, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create rider unfreeze log: %w", err)
			}

			result.RefundOrder = refundOrder
			result.Rider = updatedRider
			result.DepositLog = depositLog
			result.Credit = credit
			result.Applied = true
			return nil
		}

		return errors.New("unsupported rider deposit refund status")
	})

	return result, err
}
