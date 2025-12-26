package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 骑手抢单事务 ====================

// GrabOrderTxParams contains the input parameters for grabbing an order
type GrabOrderTxParams struct {
	DeliveryID   int64
	RiderID      int64
	OrderID      int64
	FreezeAmount int64 // 需要冻结的押金金额
}

// GrabOrderTxResult contains the result of the grab order transaction
type GrabOrderTxResult struct {
	Delivery   Delivery
	DepositLog RiderDeposit
}

// GrabOrderTx executes all operations for grabbing an order in a single transaction:
// 1. Lock rider row with FOR UPDATE
// 2. Check deposit is sufficient
// 3. Assign delivery to rider
// 4. Remove order from delivery pool
// 5. Freeze rider's deposit
// 6. Create deposit log
func (store *SQLStore) GrabOrderTx(ctx context.Context, arg GrabOrderTxParams) (GrabOrderTxResult, error) {
	var result GrabOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 使用 FOR UPDATE 锁定骑手行，获取最新押金数据
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		// 2. 再次检查押金是否充足（在事务内检查，确保并发安全）
		availableDeposit := rider.DepositAmount - rider.FrozenDeposit
		if availableDeposit < arg.FreezeAmount {
			return fmt.Errorf("押金余额不足，无法接单")
		}

		// 3. 分配配送单给骑手
		result.Delivery, err = q.AssignDelivery(ctx, AssignDeliveryParams{
			ID:      arg.DeliveryID,
			RiderID: pgtype.Int8{Int64: arg.RiderID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("assign delivery: %w", err)
		}

		// 4. 从订单池中移除
		err = q.RemoveFromDeliveryPool(ctx, arg.OrderID)
		if err != nil {
			return fmt.Errorf("remove from delivery pool: %w", err)
		}

		// 5. 冻结押金（使用事务内获取的最新值）
		_, err = q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: rider.DepositAmount,
			FrozenDeposit: rider.FrozenDeposit + arg.FreezeAmount,
		})
		if err != nil {
			return fmt.Errorf("update rider deposit: %w", err)
		}

		// 6. 创建押金流水
		result.DepositLog, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:        arg.RiderID,
			Amount:         arg.FreezeAmount,
			Type:           "freeze",
			RelatedOrderID: pgtype.Int8{Int64: arg.OrderID, Valid: true},
			BalanceAfter:   rider.DepositAmount - rider.FrozenDeposit - arg.FreezeAmount,
			Remark:         pgtype.Text{String: "接单冻结押金", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create deposit log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 确认送达事务 ====================

// CompleteDeliveryTxParams contains the input parameters for completing a delivery
type CompleteDeliveryTxParams struct {
	DeliveryID     int64
	RiderID        int64
	OrderID        int64
	UnfreezeAmount int64 // 需要解冻的押金金额
	DeliveryFee    int64 // 配送费（分）：用于判断高值单和更新收益
}

// CompleteDeliveryTxResult contains the result of the complete delivery transaction
type CompleteDeliveryTxResult struct {
	Delivery              Delivery
	DepositLog            RiderDeposit
	PremiumScoreLog       *RiderPremiumScoreLog // 高值单资格积分变更记录
	NewPremiumScore       int16                 // 更新后的高值单资格积分
}

// 高值单阈值：运费 >= 10 元（1000分）
const HighValueOrderThreshold = int64(1000)

// 积分变更规则
const (
	PremiumScoreNormalOrder  = int16(1)  // 完成普通单 +1
	PremiumScorePremiumOrder = int16(-3) // 完成高值单 -3
)

// CompleteDeliveryTx executes all operations for completing a delivery in a single transaction:
// 1. Lock rider row with FOR UPDATE
// 2. Update delivery status to delivered
// 3. Unfreeze rider's deposit
// 4. Create deposit log
// 5. Update rider stats
// 6. Update premium score and create log
func (store *SQLStore) CompleteDeliveryTx(ctx context.Context, arg CompleteDeliveryTxParams) (CompleteDeliveryTxResult, error) {
	var result CompleteDeliveryTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 使用 FOR UPDATE 锁定骑手行，获取最新押金数据
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		// 2. 更新配送状态为已送达
		result.Delivery, err = q.UpdateDeliveryToDelivered(ctx, UpdateDeliveryToDeliveredParams{
			ID:      arg.DeliveryID,
			RiderID: pgtype.Int8{Int64: arg.RiderID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("update delivery to delivered: %w", err)
		}

		// 3. 解冻押金（使用事务内获取的最新值）
		_, err = q.UpdateRiderDeposit(ctx, UpdateRiderDepositParams{
			ID:            arg.RiderID,
			DepositAmount: rider.DepositAmount,
			FrozenDeposit: rider.FrozenDeposit - arg.UnfreezeAmount,
		})
		if err != nil {
			return fmt.Errorf("update rider deposit: %w", err)
		}

		// 4. 创建押金流水
		result.DepositLog, err = q.CreateRiderDeposit(ctx, CreateRiderDepositParams{
			RiderID:        arg.RiderID,
			Amount:         arg.UnfreezeAmount,
			Type:           "unfreeze",
			RelatedOrderID: pgtype.Int8{Int64: arg.OrderID, Valid: true},
			BalanceAfter:   rider.DepositAmount - rider.FrozenDeposit + arg.UnfreezeAmount,
			Remark:         pgtype.Text{String: "配送完成解冻押金", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create deposit log: %w", err)
		}

		// 5. 更新骑手统计
		_, err = q.UpdateRiderStats(ctx, UpdateRiderStatsParams{
			ID:            arg.RiderID,
			TotalOrders:   1,
			TotalEarnings: arg.DeliveryFee,
		})
		if err != nil {
			return fmt.Errorf("update rider stats: %w", err)
		}

		// 6. 更新高值单资格积分
		// 获取当前积分
		oldScore, err := q.GetRiderPremiumScore(ctx, arg.RiderID)
		if err != nil {
			// 如果rider_profiles不存在，默认为0
			oldScore = 0
		}

		// 判断是高值单还是普通单，计算积分变更
		var changeAmount int16
		var changeType string
		var remark string
		if arg.DeliveryFee >= HighValueOrderThreshold {
			changeAmount = PremiumScorePremiumOrder
			changeType = "premium_order"
			remark = "完成高值单（运费≥10元）"
		} else {
			changeAmount = PremiumScoreNormalOrder
			changeType = "normal_order"
			remark = "完成普通单"
		}

		// 更新积分
		newScore, err := q.UpdateRiderPremiumScore(ctx, UpdateRiderPremiumScoreParams{
			RiderID:      arg.RiderID,
			PremiumScore: changeAmount,
		})
		if err != nil {
			return fmt.Errorf("update premium score: %w", err)
		}
		result.NewPremiumScore = newScore

		// 创建积分变更日志
		scoreLog, err := q.CreateRiderPremiumScoreLog(ctx, CreateRiderPremiumScoreLogParams{
			RiderID:           arg.RiderID,
			ChangeAmount:      changeAmount,
			OldScore:          oldScore,
			NewScore:          newScore,
			ChangeType:        changeType,
			RelatedOrderID:    pgtype.Int8{Int64: arg.OrderID, Valid: true},
			RelatedDeliveryID: pgtype.Int8{Int64: arg.DeliveryID, Valid: true},
			Remark:            pgtype.Text{String: remark, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create premium score log: %w", err)
		}
		result.PremiumScoreLog = &scoreLog

		return nil
	})

	return result, err
}
