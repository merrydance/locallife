package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
)

const (
	riderDepositRefundType      = "rider_deposit"
	riderDepositFreezeRemark    = "押金提现冻结"
	riderDepositWithdrawRemark  = "押金退款提现成功"
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
	RiderID int64
	Amount  int64
	Remark  string
}

type PrepareRiderDepositRefundTxResult struct {
	Rider        Rider
	RefundPlans  []RiderDepositRefundPlan
	FrozenAmount int64
}

type ResolveRiderDepositRefundTxParams struct {
	RefundOrderID int64
	RefundStatus  string
	RefundID      string
}

type ResolveRiderDepositRefundTxResult struct {
	RefundOrder RefundOrder
	Rider       Rider
	DepositLog  RiderDeposit
	Credit      RiderDepositCredit
	Applied     bool
}

func (store *SQLStore) PrepareRiderDepositRefundTx(ctx context.Context, arg PrepareRiderDepositRefundTxParams) (PrepareRiderDepositRefundTxResult, error) {
	var result PrepareRiderDepositRefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}
		if rider.FrozenDeposit > 0 {
			return ErrRiderDepositFrozen
		}

		availableBalance := rider.DepositAmount - rider.FrozenDeposit
		if arg.Amount > availableBalance {
			return ErrInsufficientDeposit
		}

		credits, err := q.ListActiveRiderDepositCreditsByRiderID(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("list rider deposit credits: %w", err)
		}

		remaining := arg.Amount
		availableAfter := availableBalance
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
		return nil
	})

	return result, err
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

			if lockedRider.DepositAmount < refundOrder.RefundAmount || lockedRider.FrozenDeposit < refundOrder.RefundAmount {
				return fmt.Errorf("rider deposit state invalid for refund settlement")
			}

			updatedRider, err := q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
				ID:            lockedRider.ID,
				DepositAmount: lockedRider.DepositAmount - refundOrder.RefundAmount,
				FrozenDeposit: lockedRider.FrozenDeposit - refundOrder.RefundAmount,
			})
			if err != nil {
				return fmt.Errorf("settle rider deposit refund success: %w", err)
			}

			depositLog, err := q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
				RiderID:        lockedRider.ID,
				Amount:         refundOrder.RefundAmount,
				Type:           "withdraw",
				RelatedOrderID: pgtype.Int8{},
				PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
				BalanceAfter:   updatedRider.DepositAmount - updatedRider.FrozenDeposit,
				Remark:         pgtype.Text{String: riderDepositWithdrawRemark, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create rider withdraw log: %w", err)
			}

			credit, err := q.GetRiderDepositCreditByPaymentOrderID(ctx, paymentOrder.ID)
			if err != nil {
				return fmt.Errorf("get rider deposit credit: %w", err)
			}

			result.RefundOrder = refundOrder
			result.Rider = updatedRider
			result.DepositLog = depositLog
			result.Credit = credit
			result.Applied = true
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
