package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 骑手押金提现事务 ====================

// WithdrawDepositTxParams contains the input parameters for withdrawing deposit
type WithdrawDepositTxParams struct {
	RiderID int64
	Amount  int64  // 提现金额
	Remark  string // 备注
}

// WithdrawDepositTxResult contains the result of the withdraw deposit transaction
type WithdrawDepositTxResult struct {
	Rider      Rider
	DepositLog RiderDeposit
}

// WithdrawDepositTx executes the deposit withdrawal in a single transaction:
// 1. Lock rider row with FOR UPDATE to prevent concurrent modifications
// 2. Verify available balance is sufficient
// 3. Update rider's deposit balance
// 4. Create deposit log
// This transaction ensures data consistency before calling external WeChat API.
// The caller should handle WeChat API call and potential rollback logic.
func (store *SQLStore) WithdrawDepositTx(ctx context.Context, arg WithdrawDepositTxParams) (WithdrawDepositTxResult, error) {
	var result WithdrawDepositTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 使用 FOR UPDATE 锁定骑手行，获取最新押金数据
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		// 2. 再次检查可用余额是否充足（在事务内检查，确保并发安全）
		availableBalance := rider.DepositAmount - rider.FrozenDeposit
		if arg.Amount > availableBalance {
			return fmt.Errorf("可用余额不足")
		}

		newBalance := rider.DepositAmount - arg.Amount

		// 3. 更新骑手押金余额
		result.Rider, err = q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: newBalance,
			FrozenDeposit: rider.FrozenDeposit,
		})
		if err != nil {
			return fmt.Errorf("update rider deposit: %w", err)
		}

		// 4. 创建押金流水
		var remark pgtype.Text
		if arg.Remark != "" {
			remark = pgtype.Text{String: arg.Remark, Valid: true}
		}

		result.DepositLog, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:      arg.RiderID,
			Amount:       arg.Amount,
			Type:         "withdraw",
			BalanceAfter: newBalance,
			Remark:       remark,
		})
		if err != nil {
			return fmt.Errorf("create deposit log: %w", err)
		}

		return nil
	})

	return result, err
}

// RollbackWithdrawTxParams contains the parameters for rolling back a failed withdrawal
type RollbackWithdrawTxParams struct {
	RiderID int64
	Amount  int64 // 恢复的金额
}

// RollbackWithdrawTx restores the deposit balance after a failed WeChat transfer
// This should be called when the WeChat API call fails
func (store *SQLStore) RollbackWithdrawTx(ctx context.Context, arg RollbackWithdrawTxParams) error {
	return store.execTx(ctx, func(q *Queries) error {
		// 1. 使用 FOR UPDATE 锁定骑手行，获取最新押金数据
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		newBalance := rider.DepositAmount + arg.Amount

		// 2. 恢复原余额
		_, err = q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: newBalance,
			FrozenDeposit: rider.FrozenDeposit,
		})
		if err != nil {
			return fmt.Errorf("rollback rider deposit: %w", err)
		}

		// 3. 创建回滚流水记录
		_, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:      arg.RiderID,
			Amount:       arg.Amount,
			Type:         "withdraw_rollback",
			BalanceAfter: newBalance,
			Remark:       pgtype.Text{String: "提现失败自动回滚", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create rollback deposit log: %w", err)
		}

		return nil
	})
}

// ==================== 骑手押金扣款事务 ====================

// DeductRiderDepositTxParams 骑手押金扣款事务参数
type DeductRiderDepositTxParams struct {
	RiderID int64
	Amount  int64  // 扣款金额
	Reason  string // 扣款原因
}

// DeductRiderDepositTxResult 骑手押金扣款事务结果
type DeductRiderDepositTxResult struct {
	Rider      Rider        // 更新后的骑手信息
	DepositLog RiderDeposit // 扣款记录
}

// DeductRiderDepositTx 在事务中执行骑手押金扣款
// 1. 检查余额并扣款
// 2. 记录押金变动日志
func (store *SQLStore) DeductRiderDepositTx(ctx context.Context, arg DeductRiderDepositTxParams) (DeductRiderDepositTxResult, error) {
	var result DeductRiderDepositTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 原子扣款（检查余额后扣款）
		result.Rider, err = q.DeductRiderDeposit(ctx, DeductRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("deduct rider deposit: %w", err)
		}

		// 2. 记录押金变动
		result.DepositLog, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:      arg.RiderID,
			Type:         "deduct",
			Amount:       arg.Amount,
			BalanceAfter: result.Rider.DepositAmount,
			Remark:       pgtype.Text{String: arg.Reason, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create deduct deposit log: %w", err)
		}

		return nil
	})

	return result, err
}
