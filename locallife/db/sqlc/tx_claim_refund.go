package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==========================================
// 平台索赔赔付事务
// ==========================================

// ClaimPayoutTxParams 平台索赔赔付事务参数
type ClaimPayoutTxParams struct {
	ClaimID    int64  // 索赔ID
	UserID     int64  // 用户ID
	Amount     int64  // 赔付金额（分）
	SourceType string // 资金来源：rider_deposit, merchant_refund, platform
	SourceID   int64  // 来源ID（骑手ID、商户ID，平台为0）
	Remark     string // 备注
}

// ClaimPayoutTxResult 平台索赔赔付事务结果
type ClaimPayoutTxResult struct {
	UserBalance    UserBalance    // 更新后的用户余额
	BalanceLog     UserBalanceLog // 余额变动日志
	BalanceCreated bool           // 是否新创建了余额账户
}

// ClaimPayoutTx 平台索赔赔付事务
// 将平台赔付金额转入用户余额账户，责任追偿由独立 recovery 链路处理。
func (store *SQLStore) ClaimPayoutTx(ctx context.Context, arg ClaimPayoutTxParams) (ClaimPayoutTxResult, error) {
	var result ClaimPayoutTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 幂等检查：是否已经赔付过
		existingLog, err := q.GetUserBalanceLogByRelated(ctx, GetUserBalanceLogByRelatedParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
		})
		if err == nil && existingLog.ID > 0 {
			// 已经赔付过，返回成功（幂等）
			balance, _ := q.GetUserBalance(ctx, arg.UserID)
			result.UserBalance = balance
			result.BalanceLog = existingLog
			if err := q.MarkClaimPaid(ctx, MarkClaimPaidParams{ID: arg.ClaimID, PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
				return fmt.Errorf("mark claim paid: %w", err)
			}
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
			Type:          "claim_payout",
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

		if err := q.MarkClaimPaid(ctx, MarkClaimPaidParams{ID: arg.ClaimID, PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
			return fmt.Errorf("mark claim paid: %w", err)
		}

		return nil
	})

	return result, err
}

// ==========================================
// 平台索赔赔付回滚事务（申诉成立）
// ==========================================

// ClaimPayoutRollbackTxParams 平台索赔赔付回滚参数
type ClaimPayoutRollbackTxParams struct {
	ClaimID int64  // 索赔ID
	UserID  int64  // 用户ID
	Remark  string // 备注
}

// ClaimPayoutRollbackTxResult 平台索赔赔付回滚结果
type ClaimPayoutRollbackTxResult struct {
	UserBalance UserBalance
	BalanceLog  UserBalanceLog
}

// ClaimPayoutRollbackTx 平台索赔赔付回滚事务
// 将平台赔付款从用户余额中扣回（幂等）。
func (store *SQLStore) ClaimPayoutRollbackTx(ctx context.Context, arg ClaimPayoutRollbackTxParams) (ClaimPayoutRollbackTxResult, error) {
	var result ClaimPayoutRollbackTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		payoutLog, err := q.GetUserBalanceLogByRelatedAndType(ctx, GetUserBalanceLogByRelatedAndTypeParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			Type:        "claim_payout",
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return fmt.Errorf("get payout log: %w", err)
		}

		reversalLog, err := q.GetUserBalanceLogByRelatedAndType(ctx, GetUserBalanceLogByRelatedAndTypeParams{
			RelatedType: pgtype.Text{String: "claim", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.ClaimID, Valid: true},
			Type:        "claim_payout_reversal",
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

		if payoutLog.Amount <= 0 {
			return nil
		}

		balance, err := q.GetUserBalanceForUpdate(ctx, arg.UserID)
		if err != nil {
			return fmt.Errorf("get user balance: %w", err)
		}

		balanceBefore := balance.Balance
		balance, err = q.DeductUserBalance(ctx, DeductUserBalanceParams{
			UserID:  arg.UserID,
			Balance: payoutLog.Amount,
		})
		if err != nil {
			return fmt.Errorf("deduct user balance: %w", err)
		}
		result.UserBalance = balance

		remark := arg.Remark
		if remark == "" {
			remark = "claim payout rollback"
		}

		sourceType := payoutLog.SourceType.String
		sourceTypeValid := payoutLog.SourceType.Valid && sourceType != ""
		sourceID := payoutLog.SourceID.Int64
		sourceIDValid := payoutLog.SourceID.Valid && sourceID > 0

		result.BalanceLog, err = q.CreateUserBalanceLog(ctx, CreateUserBalanceLogParams{
			UserID:        arg.UserID,
			Type:          "claim_payout_reversal",
			Amount:        payoutLog.Amount,
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

// ==========================================
// 申诉补偿事务
// ==========================================

type AppealCompensationTxParams struct {
	AppealID int64
	UserID   int64
	Amount   int64
	Remark   string
}

type AppealCompensationTxResult struct {
	UserBalance    UserBalance
	BalanceLog     UserBalanceLog
	BalanceCreated bool
}

func (store *SQLStore) AppealCompensationTx(ctx context.Context, arg AppealCompensationTxParams) (AppealCompensationTxResult, error) {
	var result AppealCompensationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		existingLog, err := q.GetUserBalanceLogByRelatedAndType(ctx, GetUserBalanceLogByRelatedAndTypeParams{
			RelatedType: pgtype.Text{String: "appeal", Valid: true},
			RelatedID:   pgtype.Int8{Int64: arg.AppealID, Valid: true},
			Type:        "appeal_compensation",
		})
		if err == nil && existingLog.ID > 0 {
			balance, _ := q.GetUserBalance(ctx, arg.UserID)
			result.UserBalance = balance
			result.BalanceLog = existingLog
			if err := q.MarkAppealCompensated(ctx, MarkAppealCompensatedParams{ID: arg.AppealID, CompensatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
				return fmt.Errorf("mark appeal compensated: %w", err)
			}
			return nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("get appeal compensation log: %w", err)
		}

		balance, err := q.GetUserBalanceForUpdate(ctx, arg.UserID)
		if err != nil {
			if _, createErr := q.GetOrCreateUserBalance(ctx, arg.UserID); createErr != nil {
				return fmt.Errorf("get or create user balance: %w", createErr)
			}
			balance, err = q.GetUserBalanceForUpdate(ctx, arg.UserID)
			if err != nil {
				return fmt.Errorf("get user balance for update after create: %w", err)
			}
			result.BalanceCreated = true
		}

		balanceBefore := balance.Balance
		balance, err = q.AddUserBalance(ctx, AddUserBalanceParams{
			UserID:  arg.UserID,
			Balance: arg.Amount,
		})
		if err != nil {
			return fmt.Errorf("add user balance: %w", err)
		}
		result.UserBalance = balance

		remark := arg.Remark
		if remark == "" {
			remark = "appeal compensation"
		}

		result.BalanceLog, err = q.CreateUserBalanceLog(ctx, CreateUserBalanceLogParams{
			UserID:        arg.UserID,
			Type:          "appeal_compensation",
			Amount:        arg.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balance.Balance,
			RelatedType:   pgtype.Text{String: "appeal", Valid: true},
			RelatedID:     pgtype.Int8{Int64: arg.AppealID, Valid: true},
			SourceType:    pgtype.Text{String: "platform", Valid: true},
			SourceID:      pgtype.Int8{Valid: false},
			Remark:        pgtype.Text{String: remark, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create appeal compensation balance log: %w", err)
		}

		if err := q.MarkAppealCompensated(ctx, MarkAppealCompensatedParams{ID: arg.AppealID, CompensatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
			return fmt.Errorf("mark appeal compensated: %w", err)
		}

		return nil
	})

	return result, err
}
