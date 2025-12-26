package db

import (
	"context"
	"fmt"

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
		totalAmount := arg.RechargeAmount + arg.BonusAmount
		newBalance := membership.Balance + totalAmount

		// 3. Update membership balance
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:             arg.MembershipID,
			Balance:        newBalance,
			TotalRecharged: membership.TotalRecharged + totalAmount,
			TotalConsumed:  membership.TotalConsumed,
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

		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:   arg.MembershipID,
			Type:           "recharge",
			Amount:         totalAmount,
			BalanceAfter:   newBalance,
			RelatedOrderID: pgtype.Int8{},
			RechargeRuleID: rechargeRuleIDPg,
			Notes:          notesPg,
		})
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

		// 3. Update membership balance
		newBalance := membership.Balance - arg.Amount
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:             arg.MembershipID,
			Balance:        newBalance,
			TotalRecharged: membership.TotalRecharged,
			TotalConsumed:  membership.TotalConsumed + arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 4. Create transaction record
		notesPg := pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""}

		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:   arg.MembershipID,
			Type:           "consume",
			Amount:         -arg.Amount, // Negative for consumption
			BalanceAfter:   newBalance,
			RelatedOrderID: pgtype.Int8{Int64: arg.RelatedOrderID, Valid: true},
			RechargeRuleID: pgtype.Int8{},
			Notes:          notesPg,
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

		// 2. Update membership balance
		newBalance := membership.Balance + arg.Amount
		result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
			ID:             arg.MembershipID,
			Balance:        newBalance,
			TotalRecharged: membership.TotalRecharged,
			TotalConsumed:  membership.TotalConsumed,
		})
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 3. Create transaction record
		notesPg := pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""}

		result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
			MembershipID:   arg.MembershipID,
			Type:           "refund",
			Amount:         arg.Amount,
			BalanceAfter:   newBalance,
			RelatedOrderID: pgtype.Int8{Int64: arg.RelatedOrderID, Valid: true},
			RechargeRuleID: pgtype.Int8{},
			Notes:          notesPg,
		})
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		return nil
	})

	return result, err
}
