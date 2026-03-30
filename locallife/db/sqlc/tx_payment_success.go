package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

const (
	riderDepositCreditStatusActive  = "active"
	riderDepositCreditStatusPartial = "partially_refunded"
	riderDepositRefundWindow        = 365 * 24 * time.Hour
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
			hasDepositLog := false
			if _, err := q.GetRiderDepositByPaymentOrderID(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: true}); err == nil {
				hasDepositLog = true
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get rider deposit by payment order: %w", err)
			}

			hasDepositCredit := false
			if _, err := q.GetRiderDepositCreditByPaymentOrderID(ctx, paymentOrder.ID); err == nil {
				hasDepositCredit = true
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get rider deposit credit by payment order: %w", err)
			}

			if hasDepositLog && hasDepositCredit {
				break
			}

			rider, err := q.GetRiderByUserID(ctx, paymentOrder.UserID)
			if err != nil {
				return fmt.Errorf("get rider: %w", err)
			}

			if !hasDepositLog {
				lockedRider, err := q.GetRiderForUpdate(ctx, rider.ID)
				if err != nil {
					return fmt.Errorf("lock rider: %w", err)
				}

				newBalance := lockedRider.DepositAmount + paymentOrder.Amount

				updatedRider, err := q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
					ID:            lockedRider.ID,
					DepositAmount: newBalance,
					FrozenDeposit: lockedRider.FrozenDeposit,
				})
				if err != nil {
					return fmt.Errorf("update rider deposit: %w", err)
				}

				updatedRider, err = ReconcileRiderOperationalStatus(ctx, q, updatedRider)
				if err != nil {
					return fmt.Errorf("reconcile rider status after deposit: %w", err)
				}
				rider = updatedRider

				_, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
					RiderID:        rider.ID,
					Amount:         paymentOrder.Amount,
					Type:           "deposit",
					BalanceAfter:   newBalance,
					PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
					Remark:         pgtype.Text{String: "微信支付充值", Valid: true},
				})
				if err != nil {
					return fmt.Errorf("create rider deposit: %w", err)
				}
			}

			if !hasDepositCredit {
				if !paymentOrder.PaidAt.Valid {
					return fmt.Errorf("paid_at is required for rider deposit credit")
				}

				paidAt := paymentOrder.PaidAt.Time
				_, err = q.CreateRiderDepositCredit(ctx, CreateRiderDepositCreditParams{
					RiderID:          rider.ID,
					PaymentOrderID:   paymentOrder.ID,
					OriginalAmount:   paymentOrder.Amount,
					RefundableAmount: paymentOrder.Amount,
					RefundedAmount:   0,
					Status:           riderDepositCreditStatusActive,
					PaidAt:           paidAt,
					RefundableUntil:  paidAt.Add(riderDepositRefundWindow),
				})
				if err != nil {
					return fmt.Errorf("create rider deposit credit: %w", err)
				}
			}

		case "reservation":
			if !paymentOrder.ReservationID.Valid {
				return fmt.Errorf("reservation_id is required")
			}
			reservationID := paymentOrder.ReservationID.Int64
			// 幂等检查：payment_order_id 有唯一约束，重试时 INSERT 会触发 23505，
			// 而非 ErrRecordNotFound；必须先查后插，与 rider_deposit/membership_recharge 保持一致。
			if _, err := q.GetReservationPaymentByPaymentOrderID(ctx, paymentOrder.ID); err == nil {
				break // 已处理，幂等跳过
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get reservation payment by payment order: %w", err)
			}
			if _, err := q.CreateReservationPayment(ctx, CreateReservationPaymentParams{
				ReservationID:  reservationID,
				PaymentOrderID: paymentOrder.ID,
				Amount:         paymentOrder.Amount,
				Type:           "reservation",
			}); err != nil {
				return fmt.Errorf("create reservation payment: %w", err)
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
			// 同 reservation case：先查后插，防止重试时唯一约束冲突导致任务卡死。
			if _, err := q.GetReservationPaymentByPaymentOrderID(ctx, paymentOrder.ID); err == nil {
				break // 已处理，幂等跳过
			} else if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get reservation addon payment by payment order: %w", err)
			}
			if _, err := q.CreateReservationPayment(ctx, CreateReservationPaymentParams{
				ReservationID:  reservationID,
				PaymentOrderID: paymentOrder.ID,
				Amount:         paymentOrder.Amount,
				Type:           "addon",
			}); err != nil {
				return fmt.Errorf("create reservation addon payment: %w", err)
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
				// 用户已付款但订单 ID 丢失，订单永远不会被激活，需人工干预。
				// 返回 ErrPaymentMissingOrderID 以便 worker 层 SkipRetry 并保持 processed_at=NULL 让监控可见。
				log.Error().
					Str("alert_type", "PAYMENT_ORDER_MISSING_ORDER_ID").
					Str("level", "critical").
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", paymentOrder.OutTradeNo).
					Str("business_type", paymentOrder.BusinessType).
					Msg("⚠️ CRITICAL: payment_order.order_id is NULL for business_type=order — user charged but order will never be activated; manual intervention required")
				return ErrPaymentMissingOrderID
			}

			// 如果是预定关联的订单，需要先确保关联的会话信息正确
			// 某些情况下（如下单未支付时），会话可能未正确关联订单
			// 这里不做强校验，由 processOrderPaymentWithQueries 处理业务逻辑

			orderResult, err := processOrderPaymentWithQueries(ctx, q, ProcessOrderPaymentTxParams{
				OrderID:            paymentOrder.OrderID.Int64,
				PaymentMethod:      orderPaymentMethodWechat,
				RiderAverageSpeed:  arg.RiderAverageSpeed,
				DefaultPrepareTime: arg.DefaultPrepareTime,
			})
			if err != nil {
				return fmt.Errorf("process order payment: %w", err)
			}
			result.OrderResult = &orderResult

		case "claim_recovery":
			if !paymentOrder.Attach.Valid || paymentOrder.Attach.String == "" {
				return fmt.Errorf("claim recovery attach is required")
			}

			var attach struct {
				ClaimID        int64  `json:"claim_id"`
				RecoveryID     int64  `json:"recovery_id"`
				RecoveryTarget string `json:"recovery_target"`
			}
			if err := json.Unmarshal([]byte(paymentOrder.Attach.String), &attach); err != nil {
				return fmt.Errorf("parse claim recovery attach: %w", err)
			}
			if attach.ClaimID == 0 {
				return fmt.Errorf("claim recovery attach claim_id is required")
			}

			recovery, err := q.GetClaimRecoveryByClaimID(ctx, attach.ClaimID)
			if err != nil {
				return fmt.Errorf("get claim recovery by claim id: %w", err)
			}
			if attach.RecoveryID != 0 && recovery.ID != attach.RecoveryID {
				return fmt.Errorf("claim recovery id mismatch")
			}

			if recovery.Status == "paid" {
				break
			}
			if recovery.Status != "pending" && recovery.Status != "overdue" {
				log.Error().
					Int64("payment_order_id", paymentOrder.ID).
					Int64("claim_id", attach.ClaimID).
					Int64("recovery_id", recovery.ID).
					Str("recovery_status", recovery.Status).
					Msg("claim recovery payment succeeded but recovery status is no longer payable")
				break
			}

			if _, err := q.MarkClaimRecoveryPaid(ctx, recovery.ID); err != nil {
				return fmt.Errorf("mark claim recovery paid: %w", err)
			}

			switch attach.RecoveryTarget {
			case "merchant":
				order, err := q.GetOrder(ctx, recovery.OrderID)
				if err != nil {
					return fmt.Errorf("get order for merchant recovery: %w", err)
				}
				if err := q.UnsuspendMerchantTakeout(ctx, order.MerchantID); err != nil {
					return fmt.Errorf("unsuspend merchant takeout: %w", err)
				}
			case "rider":
				delivery, err := q.GetDeliveryByOrderID(ctx, recovery.OrderID)
				if err != nil {
					return fmt.Errorf("get delivery for rider recovery: %w", err)
				}
				if delivery.RiderID.Valid {
					if err := q.UnsuspendRider(ctx, delivery.RiderID.Int64); err != nil {
						return fmt.Errorf("unsuspend rider: %w", err)
					}
				}
			}

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
