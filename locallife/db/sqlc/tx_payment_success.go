package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ProcessPaymentSuccessTxParams contains the input parameters for processing payment success idempotently
// across different business types.
type ProcessPaymentSuccessTxParams struct {
	PaymentOrderID     int64
	RiderAverageSpeed  int
	DefaultPrepareTime int
}

// ProcessPaymentSuccessTxResult contains the result of payment success processing
type ProcessPaymentSuccessTxResult struct {
	PaymentOrder PaymentOrder
	Processed    bool
	OrderResult  *ProcessOrderPaymentTxResult
}

// ProcessPaymentSuccessTx handles payment success in a single transaction with idempotency guard.
func (store *SQLStore) ProcessPaymentSuccessTx(ctx context.Context, arg ProcessPaymentSuccessTxParams) (ProcessPaymentSuccessTxResult, error) {
	var result ProcessPaymentSuccessTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get payment order: %w", err)
		}
		result.PaymentOrder = paymentOrder

		if paymentOrder.Status != "paid" {
			return nil
		}
		if paymentOrder.ProcessedAt.Valid {
			return nil
		}

		switch paymentOrder.BusinessType {
		case "rider_deposit":
			if _, err := q.GetRiderDepositByPaymentOrderID(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: true}); err == nil {
				break
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get rider deposit by payment order: %w", err)
			}

			rider, err := q.GetRiderByUserID(ctx, paymentOrder.UserID)
			if err != nil {
				return fmt.Errorf("get rider: %w", err)
			}
			lockedRider, err := q.GetRiderForUpdate(ctx, rider.ID)
			if err != nil {
				return fmt.Errorf("lock rider: %w", err)
			}

			newBalance := lockedRider.DepositAmount + paymentOrder.Amount

			_, err = q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
				ID:            lockedRider.ID,
				DepositAmount: newBalance,
				FrozenDeposit: lockedRider.FrozenDeposit,
			})
			if err != nil {
				return fmt.Errorf("update rider deposit: %w", err)
			}

			_, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
				RiderID:        lockedRider.ID,
				Amount:         paymentOrder.Amount,
				Type:           "deposit",
				BalanceAfter:   newBalance,
				PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
				Remark:         pgtype.Text{String: "微信支付充值", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create rider deposit: %w", err)
			}

		case "reservation":
			if !paymentOrder.ReservationID.Valid {
				return fmt.Errorf("reservation_id is required")
			}
			reservationID := paymentOrder.ReservationID.Int64
			if _, err := q.CreateReservationPayment(ctx, CreateReservationPaymentParams{
				ReservationID:  reservationID,
				PaymentOrderID: paymentOrder.ID,
				Amount:         paymentOrder.Amount,
				Type:           "reservation",
			}); err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return fmt.Errorf("create reservation payment: %w", err)
				}
				break
			}

			if _, err := q.UpdateReservationStatus(ctx, UpdateReservationStatusParams{
				ID:     reservationID,
				Status: "paid",
			}); err != nil {
				return fmt.Errorf("update reservation status: %w", err)
			}
			if _, err := syncReservationInventoryWithQueries(ctx, q, reservationID); err != nil {
				return fmt.Errorf("sync reservation inventory: %w", err)
			}

		case "reservation_addon":
			if !paymentOrder.ReservationID.Valid {
				return fmt.Errorf("reservation_id is required")
			}
			reservationID := paymentOrder.ReservationID.Int64
			if _, err := q.CreateReservationPayment(ctx, CreateReservationPaymentParams{
				ReservationID:  reservationID,
				PaymentOrderID: paymentOrder.ID,
				Amount:         paymentOrder.Amount,
				Type:           "addon",
			}); err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return fmt.Errorf("create reservation addon payment: %w", err)
				}
				break
			}

			if _, err := q.AddReservationPrepaidAmount(ctx, AddReservationPrepaidAmountParams{
				ID:            reservationID,
				PrepaidAmount: paymentOrder.Amount,
			}); err != nil {
				return fmt.Errorf("add reservation prepaid amount: %w", err)
			}
			if _, err := syncReservationInventoryWithQueries(ctx, q, reservationID); err != nil {
				return fmt.Errorf("sync reservation inventory: %w", err)
			}

		case "membership_recharge":
			if !paymentOrder.Attach.Valid || paymentOrder.Attach.String == "" {
				return fmt.Errorf("attach data is missing")
			}

			var attachData struct {
				MembershipID   int64  `json:"membership_id"`
				BonusAmount    int64  `json:"bonus_amount"`
				RechargeRuleID *int64 `json:"recharge_rule_id"`
			}
			if err := json.Unmarshal([]byte(paymentOrder.Attach.String), &attachData); err != nil {
				return fmt.Errorf("parse attach data: %w", err)
			}

			if _, err := q.GetMembershipTransactionByPaymentOrderID(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: true}); err == nil {
				break
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get membership transaction by payment order: %w", err)
			}

			membership, err := q.GetMembershipForUpdate(ctx, attachData.MembershipID)
			if err != nil {
				return fmt.Errorf("get membership: %w", err)
			}

			principalAmount := paymentOrder.Amount
			bonusAmount := attachData.BonusAmount
			newPrincipal := membership.PrincipalBalance + principalAmount
			newBonus := membership.BonusBalance + bonusAmount
			newBalance := newPrincipal + newBonus

			if _, err := q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
				ID:               attachData.MembershipID,
				Balance:          newBalance,
				PrincipalBalance: newPrincipal,
				BonusBalance:     newBonus,
				TotalRecharged:   membership.TotalRecharged + principalAmount + bonusAmount,
				TotalConsumed:    membership.TotalConsumed,
			}); err != nil {
				return fmt.Errorf("update balance: %w", err)
			}

			var rechargeRuleIDPg pgtype.Int8
			if attachData.RechargeRuleID != nil {
				rechargeRuleIDPg = pgtype.Int8{Int64: *attachData.RechargeRuleID, Valid: true}
			}

			notesPg := pgtype.Text{String: fmt.Sprintf("微信支付充值，订单号：%s", paymentOrder.OutTradeNo), Valid: true}
			if _, err := q.CreateMembershipTransactionWithPaymentOrderID(ctx, CreateMembershipTransactionWithPaymentOrderIDParams{
				MembershipID:    attachData.MembershipID,
				Type:            "recharge",
				Amount:          principalAmount + bonusAmount,
				PrincipalAmount: principalAmount,
				BonusAmount:     bonusAmount,
				BalanceAfter:    newBalance,
				RelatedOrderID:  pgtype.Int8{},
				RechargeRuleID:  rechargeRuleIDPg,
				Notes:           notesPg,
				PaymentOrderID:  pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
			}); err != nil {
				return fmt.Errorf("create membership transaction: %w", err)
			}

		case "order":
			if !paymentOrder.OrderID.Valid {
				// 容错处理：如果 order_id 缺失，不返回错误以避免无限重试，直接跳过处理标记为已完成
				return nil
			}

			// 如果是预定关联的订单，需要先确保关联的会话信息正确
			// 某些情况下（如下单未支付时），会话可能未正确关联订单
			// 这里不做强校验，由 processOrderPaymentWithQueries 处理业务逻辑

			orderResult, err := processOrderPaymentWithQueries(ctx, q, ProcessOrderPaymentTxParams{
				OrderID:            paymentOrder.OrderID.Int64,
				RiderAverageSpeed:  arg.RiderAverageSpeed,
				DefaultPrepareTime: arg.DefaultPrepareTime,
			})
			if err != nil {
				return fmt.Errorf("process order payment: %w", err)
			}
			result.OrderResult = &orderResult

		default:
			return fmt.Errorf("unknown business type: %s", paymentOrder.BusinessType)
		}

		processedOrder, err := q.UpdatePaymentOrderProcessedAt(ctx, paymentOrder.ID)
		if err != nil {
			return fmt.Errorf("mark payment order processed: %w", err)
		}
		result.PaymentOrder = processedOrder
		result.Processed = true

		return nil
	})

	return result, err
}
