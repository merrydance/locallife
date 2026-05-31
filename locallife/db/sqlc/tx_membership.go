package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// JoinMembershipTxParams contains the input parameters for joining a membership
type JoinMembershipTxParams struct {
	MerchantID int64
	UserID     int64
}

// JoinMembershipTxResult contains the result of joining membership transaction
type JoinMembershipTxResult struct {
	Membership MerchantMembership
}

// JoinMembershipTx creates a new membership for a user at a merchant
func (store *SQLStore) JoinMembershipTx(ctx context.Context, arg JoinMembershipTxParams) (JoinMembershipTxResult, error) {
	var result JoinMembershipTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Check if membership already exists
		existingMembership, err := q.GetMembershipByMerchantAndUser(ctx, GetMembershipByMerchantAndUserParams{
			MerchantID: arg.MerchantID,
			UserID:     arg.UserID,
		})
		if err == nil {
			// Membership already exists
			result.Membership = existingMembership
			return nil
		}

		// Create new membership
		result.Membership, err = q.CreateMerchantMembership(ctx, CreateMerchantMembershipParams{
			MerchantID: arg.MerchantID,
			UserID:     arg.UserID,
		})
		if err != nil {
			return fmt.Errorf("create membership: %w", err)
		}

		return nil
	})

	return result, err
}

// RechargeTxParams contains the input parameters for recharging membership
type RechargeTxParams struct {
	MembershipID   int64
	RechargeAmount int64
	BonusAmount    int64
	RechargeRuleID *int64
	Notes          string
	IdempotencyKey string
}

// RechargeTxResult contains the result of recharge transaction
type RechargeTxResult struct {
	Membership  MerchantMembership
	Transaction MembershipTransaction
}

// RechargeTx processes a membership recharge with optional bonus in a single transaction
func (store *SQLStore) RechargeTx(ctx context.Context, arg RechargeTxParams) (RechargeTxResult, error) {
	var result RechargeTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Lock membership for update
		membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
		if err != nil {
			return fmt.Errorf("get membership: %w", err)
		}

		// 2. Calculate total amount (recharge + bonus)
		principalAmount := arg.RechargeAmount
		bonusAmount := arg.BonusAmount
		newPrincipal := membership.PrincipalBalance + principalAmount
		newBonus := membership.BonusBalance + bonusAmount
		totalAmount := principalAmount + bonusAmount
		newBalance := newPrincipal + newBonus

		// 3. Update membership balance
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:               arg.MembershipID,
			Balance:          newBalance,
			PrincipalBalance: newPrincipal,
			BonusBalance:     newBonus,
			TotalRecharged:   membership.TotalRecharged + totalAmount,
			TotalConsumed:    membership.TotalConsumed,
		})
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 4. Create transaction record
		var rechargeRuleIDPg pgtype.Int8
		if arg.RechargeRuleID != nil {
			rechargeRuleIDPg = pgtype.Int8{Int64: *arg.RechargeRuleID, Valid: true}
		}

		notesPg := pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""}

		if arg.IdempotencyKey != "" {
			result.Transaction, err = q.CreateMembershipRechargeTransaction(ctx, CreateMembershipRechargeTransactionParams{
				MembershipID:    arg.MembershipID,
				Amount:          totalAmount,
				PrincipalAmount: principalAmount,
				BonusAmount:     bonusAmount,
				BalanceAfter:    newBalance,
				RelatedOrderID:  pgtype.Int8{},
				RechargeRuleID:  rechargeRuleIDPg,
				Notes:           notesPg,
				IdempotencyKey:  pgtype.Text{String: arg.IdempotencyKey, Valid: true},
			})
		} else {
			result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
				MembershipID:    arg.MembershipID,
				Type:            "recharge",
				Amount:          totalAmount,
				PrincipalAmount: principalAmount,
				BonusAmount:     bonusAmount,
				BalanceAfter:    newBalance,
				RelatedOrderID:  pgtype.Int8{},
				RechargeRuleID:  rechargeRuleIDPg,
				Notes:           notesPg,
			})
		}
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		return nil
	})

	return result, err
}

// ConsumeTxParams contains the input parameters for consuming membership balance
type ConsumeTxParams struct {
	MembershipID   int64
	Amount         int64
	RelatedOrderID int64
	Notes          string
}

// ConsumeTxResult contains the result of consume transaction
type ConsumeTxResult struct {
	Membership  MerchantMembership
	Transaction MembershipTransaction
}

// ConsumeTx deducts amount from membership balance in a single transaction
func (store *SQLStore) ConsumeTx(ctx context.Context, arg ConsumeTxParams) (ConsumeTxResult, error) {
	var result ConsumeTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Lock membership for update
		membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
		if err != nil {
			return fmt.Errorf("get membership: %w", err)
		}

		// 2. Check balance
		if membership.Balance < arg.Amount {
			return fmt.Errorf("insufficient balance: have %d, need %d", membership.Balance, arg.Amount)
		}

		// 3. Update membership balance (bonus first, then principal)
		bonusUsed := membership.BonusBalance
		if bonusUsed > arg.Amount {
			bonusUsed = arg.Amount
		}
		principalUsed := arg.Amount - bonusUsed
		newBonus := membership.BonusBalance - bonusUsed
		newPrincipal := membership.PrincipalBalance - principalUsed
		newBalance := newPrincipal + newBonus
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:               arg.MembershipID,
			Balance:          newBalance,
			PrincipalBalance: newPrincipal,
			BonusBalance:     newBonus,
			TotalRecharged:   membership.TotalRecharged,
			TotalConsumed:    membership.TotalConsumed + arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 4. Create transaction record
		notesPg := pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""}

		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:    arg.MembershipID,
			Type:            "consume",
			Amount:          -arg.Amount, // Negative for consumption
			PrincipalAmount: -principalUsed,
			BonusAmount:     -bonusUsed,
			BalanceAfter:    newBalance,
			RelatedOrderID:  pgtype.Int8{Int64: arg.RelatedOrderID, Valid: true},
			RechargeRuleID:  pgtype.Int8{},
			Notes:           notesPg,
		})
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		return nil
	})

	return result, err
}

// RefundTxParams contains the input parameters for refunding to membership balance
type RefundTxParams struct {
	MembershipID   int64
	Amount         int64
	RelatedOrderID int64
	Notes          string
}

// RefundTxResult contains the result of refund transaction
type RefundTxResult struct {
	Membership  MerchantMembership
	Transaction MembershipTransaction
}

// RefundTx refunds amount to membership balance in a single transaction
func (store *SQLStore) RefundTx(ctx context.Context, arg RefundTxParams) (RefundTxResult, error) {
	var result RefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Lock membership for update
		membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
		if err != nil {
			return fmt.Errorf("get membership: %w", err)
		}

		// 2. Update membership balance (refund to principal by default)
		newPrincipal := membership.PrincipalBalance + arg.Amount
		newBonus := membership.BonusBalance
		newBalance := newPrincipal + newBonus
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:               arg.MembershipID,
			Balance:          newBalance,
			PrincipalBalance: newPrincipal,
			BonusBalance:     newBonus,
			TotalRecharged:   membership.TotalRecharged,
			TotalConsumed:    membership.TotalConsumed,
		})
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 3. Create transaction record
		notesPg := pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""}

		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:    arg.MembershipID,
			Type:            "refund",
			Amount:          arg.Amount,
			PrincipalAmount: arg.Amount,
			BonusAmount:     0,
			BalanceAfter:    newBalance,
			RelatedOrderID:  pgtype.Int8{Int64: arg.RelatedOrderID, Valid: true},
			RechargeRuleID:  pgtype.Int8{},
			Notes:           notesPg,
		})
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 商户调整余额事务 ====================

// AdjustMemberBalanceTxParams 商户调整会员余额事务参数
type AdjustMemberBalanceTxParams struct {
	MembershipID   int64
	Amount         int64  // 正数增加，负数减少
	Notes          string // 调整备注
	IdempotencyKey string
}

// AdjustMemberBalanceTxResult 商户调整会员余额事务结果
type AdjustMemberBalanceTxResult struct {
	Membership  MerchantMembership
	Transaction MembershipTransaction
}

// AdjustMemberBalanceTx 在单一事务内完成余额调整和流水记录
// 解决 P1-007: 余额变动与流水记录非原子问题
func (store *SQLStore) AdjustMemberBalanceTx(ctx context.Context, arg AdjustMemberBalanceTxParams) (AdjustMemberBalanceTxResult, error) {
	var result AdjustMemberBalanceTxResult
	idempotencyKey := strings.TrimSpace(arg.IdempotencyKey)
	if idempotencyKey == "" {
		return result, fmt.Errorf("idempotency key is required")
	}
	notes := strings.TrimSpace(arg.Notes)

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 使用 FOR UPDATE 锁定会员记录，获取最新余额
		membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
		if err != nil {
			return fmt.Errorf("get membership for update: %w", err)
		}

		existingTransaction, err := q.GetMembershipAdjustmentTransactionByIdempotencyKey(ctx, GetMembershipAdjustmentTransactionByIdempotencyKeyParams{
			MembershipID:   arg.MembershipID,
			IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
		})
		if err == nil {
			if existingTransaction.Amount != arg.Amount || strings.TrimSpace(existingTransaction.Notes.String) != notes {
				return fmt.Errorf("%w: idempotency key already used by a different membership adjustment request", ErrMembershipAdjustmentIdempotencyConflict)
			}
			result.Transaction = existingTransaction
			result.Membership = membership
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get membership adjustment by idempotency key: %w", err)
		}

		// 2. 计算新余额并验证（调整默认作用于本金）
		newPrincipal := membership.PrincipalBalance + arg.Amount
		newBonus := membership.BonusBalance
		newBalance := newPrincipal + newBonus
		if newBalance < 0 {
			return fmt.Errorf("%w: current balance %d, debit %d", ErrMembershipBalanceInsufficient, membership.Balance, -arg.Amount)
		}

		// 3. 计算新的累计充值/消费
		var newTotalRecharged, newTotalConsumed int64
		var txType string
		if arg.Amount > 0 {
			newTotalRecharged = membership.TotalRecharged + arg.Amount
			newTotalConsumed = membership.TotalConsumed
			txType = "adjustment_credit"
		} else {
			newTotalRecharged = membership.TotalRecharged
			newTotalConsumed = membership.TotalConsumed + (-arg.Amount)
			txType = "adjustment_debit"
		}

		// 4. 更新余额（原子操作）
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:               arg.MembershipID,
			Balance:          newBalance,
			PrincipalBalance: newPrincipal,
			BonusBalance:     newBonus,
			TotalRecharged:   newTotalRecharged,
			TotalConsumed:    newTotalConsumed,
		})
		if err != nil {
			return fmt.Errorf("update membership balance: %w", err)
		}

		// 5. 创建流水记录（同一事务内，保证原子性）
		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:    arg.MembershipID,
			Type:            txType,
			Amount:          arg.Amount,
			PrincipalAmount: arg.Amount,
			BonusAmount:     0,
			BalanceAfter:    newBalance,
			RelatedOrderID:  pgtype.Int8{},
			RechargeRuleID:  pgtype.Int8{},
			Notes:           pgtype.Text{String: notes, Valid: notes != ""},
			IdempotencyKey:  pgtype.Text{String: idempotencyKey, Valid: true},
		})
		if err != nil {
			if ErrorCode(err) == UniqueViolation {
				existingTransaction, existingErr := q.GetMembershipAdjustmentTransactionByIdempotencyKey(ctx, GetMembershipAdjustmentTransactionByIdempotencyKeyParams{
					MembershipID:   arg.MembershipID,
					IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
				})
				if existingErr == nil && existingTransaction.Amount == arg.Amount && strings.TrimSpace(existingTransaction.Notes.String) == notes {
					result.Transaction = existingTransaction
					result.Membership = membership
					return nil
				}
			}
			return fmt.Errorf("create membership transaction: %w", err)
		}

		return nil
	})

	return result, err
}
