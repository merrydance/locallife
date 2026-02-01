package db

import (
	"context"
	"errors"
)

var ErrInsufficientBalance = errors.New("insufficient balance")

// WithdrawOperatorTxParams definition
type WithdrawOperatorTxParams struct {
	OperatorID int64
	Amount     int64
	Channel    string
}

// WithdrawOperatorTxResult definition
type WithdrawOperatorTxResult struct {
	WithdrawalRecord WithdrawalRecord
}

// WithdrawOperatorTx performs a money withdrawal for an operator
func (store *SQLStore) WithdrawOperatorTx(ctx context.Context, arg WithdrawOperatorTxParams) (WithdrawOperatorTxResult, error) {
	var result WithdrawOperatorTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Get operator for update (lock)
		operator, err := q.GetOperatorForUpdate(ctx, arg.OperatorID)
		if err != nil {
			return err
		}

		// 2. Check balance
		if operator.Balance < arg.Amount {
			return ErrInsufficientBalance
		}

		// 3. Deduct balance
		// We use UpdateOperatorBalance which adds the amount, so we pass negative amount
		_, err = q.UpdateOperatorBalance(ctx, UpdateOperatorBalanceParams{
			ID:     arg.OperatorID,
			Amount: -arg.Amount,
		})
		if err != nil {
			return err
		}

		// 4. Create withdrawal record
		// Validate AccountInfo existence? Assuming business logic checked it,
		// but safeguards here don't hurt.
		// However, SQL constraints or checks are better.
		// We simply store what is there.

		result.WithdrawalRecord, err = q.CreateWithdrawalRecord(ctx, CreateWithdrawalRecordParams{
			UserID:      operator.UserID, // Note: WithdrawalRecord uses UserID, not OperatorID directly usually, check schema
			Amount:      arg.Amount,
			Status:      "pending",
			Channel:     arg.Channel,
			AccountInfo: operator.WalletAccount, // pass the wallet info snapshot
		})
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}
