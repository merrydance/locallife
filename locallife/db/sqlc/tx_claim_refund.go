package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==========================================
// 索赔退款事务
// ==========================================

// ClaimRefundTxParams 索赔退款事务参数
type ClaimRefundTxParams struct {
	ClaimID    int64  // 索赔ID
	UserID     int64  // 用户ID
	Amount     int64  // 退款金额（分）
	SourceType string // 资金来源：rider_deposit, merchant_refund, platform
	SourceID   int64  // 来源ID（骑手ID、商户ID，平台为0）
	Remark     string // 备注
}

// ClaimRefundTxResult 索赔退款事务结果
type ClaimRefundTxResult struct {
	UserBalance    UserBalance    // 更新后的用户余额
	BalanceLog     UserBalanceLog // 余额变动日志
	BalanceCreated bool           // 是否新创建了余额账户
}

// ClaimRefundTx 索赔退款事务
// 将退款金额从来源（骑手押金/商户/平台）转入用户余额账户
func (store *SQLStore) ClaimRefundTx(ctx context.Context, arg ClaimRefundTxParams) (ClaimRefundTxResult, error) {
	var result ClaimRefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 幂等检查：是否已经退款过
		existingLog, err := q.GetUserBalanceLogByRelated(ctx, GetUserBalanceLogByRelatedParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
		})
		if err == nil && existingLog.ID > 0 {
			// 已经退款过，返回成功（幂等）
			balance, _ := q.GetUserBalance(ctx, arg.UserID)
			result.UserBalance = balance
			result.BalanceLog = existingLog
			return nil
		}

		// 2. 获取或创建用户余额账户
		balance, err := q.GetUserBalanceForUpdate(ctx, arg.UserID)
		if err != nil {
			// 不存在，创建新账户
			balance, err = q.CreateUserBalance(ctx, arg.UserID)
			if err != nil {
				return fmt.Errorf("create user balance: %w", err)
			}
			result.BalanceCreated = true
		}

		balanceBefore := balance.Balance

		// 3. 增加用户余额
		balance, err = q.AddUserBalance(ctx, AddUserBalanceParams{
			UserID:  arg.UserID,
			Balance: arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("add user balance: %w", err)
		}

		result.UserBalance = balance

		// 4. 记录余额变动日志
		result.BalanceLog, err = q.CreateUserBalanceLog(ctx, CreateUserBalanceLogParams{
			UserID:        arg.UserID,
			Type:          "claim_refund",
			Amount:        arg.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balance.Balance,
			RelatedType:   pgtype.Text{String: "claim", Valid: true},
			RelatedID:     pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			SourceType:    pgtype.Text{String: arg.SourceType, Valid: true},
			SourceID:      pgtype.Int8{Int64: arg.SourceID, Valid: arg.SourceID > 0},
			Remark:        pgtype.Text{String: arg.Remark, Valid: arg.Remark != ""},
		})
		if err != nil {
			return fmt.Errorf("create balance log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==========================================
// 骑手押金扣款并退款给用户的完整事务
// ==========================================

// DeductRiderDepositAndRefundTxParams 骑手押金扣款并退款参数
type DeductRiderDepositAndRefundTxParams struct {
	RiderID    int64  // 骑手ID
	UserID     int64  // 用户ID（退款接收方）
	ClaimID    int64  // 索赔ID
	Amount     int64  // 扣款/退款金额（分）
	ClaimType  string // 索赔类型
}

// DeductRiderDepositAndRefundTxResult 骑手押金扣款并退款结果
type DeductRiderDepositAndRefundTxResult struct {
	Rider          Rider          // 更新后的骑手信息
	DepositLog     RiderDeposit   // 押金扣款记录
	UserBalance    UserBalance    // 更新后的用户余额
	BalanceLog     UserBalanceLog // 余额变动日志
}

// DeductRiderDepositAndRefundTx 骑手押金扣款并退款给用户
// 原子操作：骑手押金扣款 + 用户余额入账 + 双方日志记录
func (store *SQLStore) DeductRiderDepositAndRefundTx(ctx context.Context, arg DeductRiderDepositAndRefundTxParams) (DeductRiderDepositAndRefundTxResult, error) {
	var result DeductRiderDepositAndRefundTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 幂等检查：是否已经处理过
		existingLog, err := q.GetUserBalanceLogByRelated(ctx, GetUserBalanceLogByRelatedParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
		})
		if err == nil && existingLog.ID > 0 {
			// 已经处理过，返回成功（幂等）
			rider, _ := q.GetRider(ctx, arg.RiderID)
			balance, _ := q.GetUserBalance(ctx, arg.UserID)
			result.Rider = rider
			result.UserBalance = balance
			result.BalanceLog = existingLog
			return nil
		}

		// 2. 扣除骑手押金
		result.Rider, err = q.DeductRiderDeposit(ctx, DeductRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("deduct rider deposit: %w", err)
		}

		// 3. 记录骑手押金变动
		result.DepositLog, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:      arg.RiderID,
			Type:         "deduct",
			Amount:       arg.Amount,
			BalanceAfter: result.Rider.DepositAmount,
			Remark:       pgtype.Text{String: fmt.Sprintf("%s索赔扣款（索赔ID: %d）", arg.ClaimType, arg.ClaimID), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create rider deposit log: %w", err)
		}

		// 4. 获取或创建用户余额账户
		balance, err := q.GetUserBalanceForUpdate(ctx, arg.UserID)
		if err != nil {
			// 不存在，创建新账户
			balance, err = q.CreateUserBalance(ctx, arg.UserID)
			if err != nil {
				return fmt.Errorf("create user balance: %w", err)
			}
		}

		balanceBefore := balance.Balance

		// 5. 增加用户余额
		result.UserBalance, err = q.AddUserBalance(ctx, AddUserBalanceParams{
			UserID:  arg.UserID,
			Balance: arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("add user balance: %w", err)
		}

		// 6. 记录用户余额变动日志
		result.BalanceLog, err = q.CreateUserBalanceLog(ctx, CreateUserBalanceLogParams{
			UserID:        arg.UserID,
			Type:          "claim_refund",
			Amount:        arg.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  result.UserBalance.Balance,
			RelatedType:   pgtype.Text{String: "claim", Valid: true},
			RelatedID:     pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			SourceType:    pgtype.Text{String: "rider_deposit", Valid: true},
			SourceID:      pgtype.Int8{Int64: arg.RiderID, Valid: true},
			Remark:        pgtype.Text{String: fmt.Sprintf("%s索赔退款，来自骑手押金", arg.ClaimType), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create user balance log: %w", err)
		}

		return nil
	})

	return result, err
}
