package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
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
			// 不存在，原子性创建（ON CONFLICT 处理并发竞态）
			if _, createErr := q.GetOrCreateUserBalance(ctx, arg.UserID); createErr != nil {
				return fmt.Errorf("get or create user balance: %w", createErr)
			}
			// 重新获取并加锁
			balance, err = q.GetUserBalanceForUpdate(ctx, arg.UserID)
			if err != nil {
				return fmt.Errorf("get user balance for update after create: %w", err)
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
// 索赔退款回滚事务（申诉成立）
// ==========================================

// ClaimRefundRollbackTxParams 索赔退款回滚参数
type ClaimRefundRollbackTxParams struct {
	ClaimID int64  // 索赔ID
	UserID  int64  // 用户ID
	Remark  string // 备注
}

// ClaimRefundRollbackTxResult 索赔退款回滚结果
type ClaimRefundRollbackTxResult struct {
	UserBalance UserBalance
	BalanceLog  UserBalanceLog
}

// ClaimRefundRollbackTx 索赔退款回滚事务
// 将索赔退款从用户余额中扣回（幂等）
func (store *SQLStore) ClaimRefundRollbackTx(ctx context.Context, arg ClaimRefundRollbackTxParams) (ClaimRefundRollbackTxResult, error) {
	var result ClaimRefundRollbackTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		refundLog, err := q.GetUserBalanceLogByRelatedAndType(ctx, GetUserBalanceLogByRelatedAndTypeParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			Type:        "claim_refund",
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return fmt.Errorf("get refund log: %w", err)
		}

		reversalLog, err := q.GetUserBalanceLogByRelatedAndType(ctx, GetUserBalanceLogByRelatedAndTypeParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			Type:        "claim_refund_reversal",
		})
		if err == nil && reversalLog.ID > 0 {
			result.BalanceLog = reversalLog
			balance, _ := q.GetUserBalance(ctx, arg.UserID)
			result.UserBalance = balance
			return nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("get reversal log: %w", err)
		}

		if refundLog.Amount <= 0 {
			return nil
		}

		balance, err := q.GetUserBalanceForUpdate(ctx, arg.UserID)
		if err != nil {
			return fmt.Errorf("get user balance: %w", err)
		}

		balanceBefore := balance.Balance
		balance, err = q.DeductUserBalance(ctx, DeductUserBalanceParams{
			UserID:  arg.UserID,
			Balance: refundLog.Amount,
		})
		if err != nil {
			return fmt.Errorf("deduct user balance: %w", err)
		}
		result.UserBalance = balance

		remark := arg.Remark
		if remark == "" {
			remark = "claim refund rollback"
		}

		sourceType := refundLog.SourceType.String
		sourceTypeValid := refundLog.SourceType.Valid && sourceType != ""
		sourceID := refundLog.SourceID.Int64
		sourceIDValid := refundLog.SourceID.Valid && sourceID > 0

		result.BalanceLog, err = q.CreateUserBalanceLog(ctx, CreateUserBalanceLogParams{
			UserID:        arg.UserID,
			Type:          "claim_refund_reversal",
			Amount:        refundLog.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balance.Balance,
			RelatedType:   pgtype.Text{String: "claim", Valid: true},
			RelatedID:     pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			SourceType:    pgtype.Text{String: sourceType, Valid: sourceTypeValid},
			SourceID:      pgtype.Int8{Int64: sourceID, Valid: sourceIDValid},
			Remark:        pgtype.Text{String: remark, Valid: remark != ""},
		})
		if err != nil {
			return fmt.Errorf("create balance log: %w", err)
		}

		return nil
	})

	return result, err
}
