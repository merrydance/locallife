package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ClaimVoucherTxParams contains the input parameters for claiming a voucher
type ClaimVoucherTxParams struct {
	VoucherID int64
	UserID    int64
}

// ClaimVoucherTxResult contains the result of claim voucher transaction
type ClaimVoucherTxResult struct {
	UserVoucher UserVoucher
	Voucher     Voucher
}

// ClaimVoucherTx claims a voucher for a user in a single transaction
func (store *SQLStore) ClaimVoucherTx(ctx context.Context, arg ClaimVoucherTxParams) (ClaimVoucherTxResult, error) {
	var result ClaimVoucherTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Lock voucher for update
		voucher, err := q.GetVoucherForUpdate(ctx, arg.VoucherID)
		if err != nil {
			return fmt.Errorf("get voucher: %w", err)
		}

		// 2. Check if voucher is active and valid
		now := time.Now()
		if !voucher.IsActive {
			return fmt.Errorf("voucher is not active")
		}
		if now.Before(voucher.ValidFrom) {
			return fmt.Errorf("voucher not yet valid")
		}
		if now.After(voucher.ValidUntil) {
			return fmt.Errorf("voucher has expired")
		}

		// 3. Check if user already claimed this voucher
		exists, err := q.CheckUserVoucherExists(ctx, CheckUserVoucherExistsParams{
			VoucherID: arg.VoucherID,
			UserID:    arg.UserID,
		})
		if err != nil {
			return fmt.Errorf("check user voucher: %w", err)
		}
		if exists {
			return fmt.Errorf("voucher already claimed by user")
		}

		// 4. Increment claimed quantity
		result.Voucher, err = q.IncrementVoucherClaimedQuantity(ctx, arg.VoucherID)
		if err != nil {
			return fmt.Errorf("increment claimed quantity: %w", err)
		}

		// 5. Create user voucher
		expiresAt := voucher.ValidUntil
		result.UserVoucher, err = q.CreateUserVoucher(ctx, CreateUserVoucherParams{
			VoucherID: arg.VoucherID,
			UserID:    arg.UserID,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			return fmt.Errorf("create user voucher: %w", err)
		}

		return nil
	})

	return result, err
}

// UseVoucherTxParams contains the input parameters for using a voucher
type UseVoucherTxParams struct {
	UserVoucherID int64
	OrderID       int64
}

// UseVoucherTxResult contains the result of use voucher transaction
type UseVoucherTxResult struct {
	UserVoucher UserVoucher
	Voucher     Voucher
}

// UseVoucherTx marks a user voucher as used in a single transaction
func (store *SQLStore) UseVoucherTx(ctx context.Context, arg UseVoucherTxParams) (UseVoucherTxResult, error) {
	var result UseVoucherTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. Lock user voucher for update
		userVoucher, err := q.GetUserVoucherForUpdate(ctx, arg.UserVoucherID)
		if err != nil {
			return fmt.Errorf("get user voucher: %w", err)
		}

		// 2. Check if voucher is unused
		if userVoucher.Status != "unused" {
			return fmt.Errorf("voucher is not unused: status=%s", userVoucher.Status)
		}

		// 3. Check if voucher has expired
		if time.Now().After(userVoucher.ExpiresAt) {
			return fmt.Errorf("voucher has expired")
		}

		// 4. Mark user voucher as used
		result.UserVoucher, err = q.MarkUserVoucherAsUsed(ctx, MarkUserVoucherAsUsedParams{
			ID:      arg.UserVoucherID,
			OrderID: pgtype.Int8{Int64: arg.OrderID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("mark voucher as used: %w", err)
		}

		// 5. Increment voucher used quantity
		result.Voucher, err = q.IncrementVoucherUsedQuantity(ctx, userVoucher.VoucherID)
		if err != nil {
			return fmt.Errorf("increment used quantity: %w", err)
		}

		return nil
	})

	return result, err
}
