package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrDeliveryStateTransitionConflict = errors.New("delivery state changed concurrently")
var ErrTakeoutOrderPausedByFoodSafety = errors.New("takeout order is paused due to food safety suspension")
var ErrBaofuProfitSharingBillNotPending = errors.New("baofu profit sharing bill is not pending")

// ==================== 骑手抢单事务 ====================

// GrabOrderTxParams contains the input parameters for grabbing an order
type GrabOrderTxParams struct {
	DeliveryID             int64
	RiderID                int64
	RiderUserID            int64
	OrderID                int64
	FreezeAmount           int64 // 需要冻结的押金金额
	ProfitSharingRiderBill *UpdateProfitSharingOrderRiderBillByPaymentOrderParams
}

// GrabOrderTxResult contains the result of the grab order transaction
type GrabOrderTxResult struct {
	Delivery   Delivery
	Order      Order
	DepositLog RiderDeposit
	StatusLog  OrderStatusLog
}

// ==================== 配送状态同步事务 ====================

// UpdateDeliveryToPickedTxParams contains the input parameters for updating delivery to picked
type UpdateDeliveryToPickedTxParams struct {
	DeliveryID int64
	RiderID    int64
	OrderID    int64
}

// UpdateDeliveryToPickedTxResult contains the result of the picked transaction
type UpdateDeliveryToPickedTxResult struct {
	Delivery Delivery
	Order    Order
}

// UpdateDeliveryToPickupTxParams contains the input parameters for updating delivery to picking
type UpdateDeliveryToPickupTxParams struct {
	DeliveryID int64
	RiderID    int64
	OrderID    int64
}

// UpdateDeliveryToPickupTxResult contains the result of the picking transaction
type UpdateDeliveryToPickupTxResult struct {
	Delivery Delivery
	Order    Order
}

// UpdateDeliveryToPickupTx updates delivery to picking and syncs order status in a single transaction
func (store *SQLStore) UpdateDeliveryToPickupTx(ctx context.Context, arg UpdateDeliveryToPickupTxParams) (UpdateDeliveryToPickupTxResult, error) {
	var result UpdateDeliveryToPickupTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		if err := ensureFoodSafetyTakeoutProgressAllowed(ctx, q, arg.OrderID); err != nil {
			return err
		}

		result.Delivery, err = q.UpdateDeliveryToPickup(ctx, UpdateDeliveryToPickupParams{
			ID:      arg.DeliveryID,
			RiderID: pgtype.Int8{Int64: arg.RiderID, Valid: true},
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update delivery to picking: %w", err)
		}

		result.Order, err = q.UpdateOrderToCourierAccepted(ctx, arg.OrderID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update order to courier_accepted: %w", err)
		}

		return nil
	})

	return result, err
}

// UpdateDeliveryToPickedTx updates delivery to picked and syncs order status in a single transaction
func (store *SQLStore) UpdateDeliveryToPickedTx(ctx context.Context, arg UpdateDeliveryToPickedTxParams) (UpdateDeliveryToPickedTxResult, error) {
	var result UpdateDeliveryToPickedTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		if err := ensureFoodSafetyTakeoutProgressAllowed(ctx, q, arg.OrderID); err != nil {
			return err
		}

		result.Delivery, err = q.UpdateDeliveryToPicked(ctx, UpdateDeliveryToPickedParams{
			ID:      arg.DeliveryID,
			RiderID: pgtype.Int8{Int64: arg.RiderID, Valid: true},
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update delivery to picked: %w", err)
		}

		result.Order, err = q.UpdateOrderToPicked(ctx, arg.OrderID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update order to picked: %w", err)
		}

		return nil
	})

	return result, err
}

// UpdateDeliveryToDeliveringTxParams contains the input parameters for updating delivery to delivering
type UpdateDeliveryToDeliveringTxParams struct {
	DeliveryID int64
	RiderID    int64
	OrderID    int64
}

// UpdateDeliveryToDeliveringTxResult contains the result of the delivering transaction
type UpdateDeliveryToDeliveringTxResult struct {
	Delivery Delivery
	Order    Order
}

// UpdateDeliveryToDeliveringTx updates delivery to delivering and syncs order status in a single transaction
func (store *SQLStore) UpdateDeliveryToDeliveringTx(ctx context.Context, arg UpdateDeliveryToDeliveringTxParams) (UpdateDeliveryToDeliveringTxResult, error) {
	var result UpdateDeliveryToDeliveringTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		if err := ensureFoodSafetyTakeoutProgressAllowed(ctx, q, arg.OrderID); err != nil {
			return err
		}

		result.Delivery, err = q.UpdateDeliveryToDelivering(ctx, UpdateDeliveryToDeliveringParams{
			ID:      arg.DeliveryID,
			RiderID: pgtype.Int8{Int64: arg.RiderID, Valid: true},
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update delivery to delivering: %w", err)
		}

		result.Order, err = q.UpdateOrderToDelivering(ctx, arg.OrderID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDeliveryStateTransitionConflict
			}
			return fmt.Errorf("update order to delivering: %w", err)
		}

		return nil
	})

	return result, err
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

		if err := ensureFoodSafetyTakeoutProgressAllowed(ctx, q, arg.OrderID); err != nil {
			return err
		}

		order, err := q.GetOrderForUpdate(ctx, arg.OrderID)
		if err != nil {
			return fmt.Errorf("get order for update: %w", err)
		}

		// 1. 使用 FOR UPDATE 锁定骑手行，获取最新押金数据
		rider, err := q.GetRiderForUpdate(ctx, arg.RiderID)
		if err != nil {
			return fmt.Errorf("get rider for update: %w", err)
		}

		// 2. 锁定并检查订单池（关键修复 P0-001：防止并发抢单）
		_, err = q.GetDeliveryPoolByOrderIDForUpdate(ctx, arg.OrderID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("手慢了，订单已被抢走")
			}
			return fmt.Errorf("lock delivery pool: %w", err)
		}

		// 3. 再次检查押金是否充足（在事务内检查，确保并发安全）
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

		if arg.ProfitSharingRiderBill != nil {
			if _, err := q.UpdateProfitSharingOrderRiderBillByPaymentOrder(ctx, *arg.ProfitSharingRiderBill); err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrBaofuProfitSharingBillNotPending
				}
				return fmt.Errorf("update baofu rider profit sharing bill: %w", err)
			}
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

		result.Order, err = q.UpdateOrderToCourierAccepted(ctx, arg.OrderID)
		if err != nil {
			return fmt.Errorf("update order to courier_accepted: %w", err)
		}

		if order.Status != OrderStatusCourierAccepted {
			result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
				OrderID:      arg.OrderID,
				FromStatus:   pgtype.Text{String: order.Status, Valid: true},
				ToStatus:     OrderStatusCourierAccepted,
				OperatorID:   pgtype.Int8{Int64: arg.RiderUserID, Valid: true},
				OperatorType: pgtype.Text{String: "rider", Valid: true},
				Notes:        pgtype.Text{String: "骑手接单", Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create order status log: %w", err)
			}
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
	DeliveryFee    int64 // 配送费（分）：用于更新收益
}

// CompleteDeliveryTxResult contains the result of the complete delivery transaction
type CompleteDeliveryTxResult struct {
	Delivery   Delivery
	DepositLog RiderDeposit
	Order      Order
}

// CompleteDeliveryTx executes all operations for completing a delivery in a single transaction:
// 1. Lock rider row with FOR UPDATE
// 2. Update delivery status to delivered
// 3. Unfreeze rider's deposit
// 4. Create deposit log
// 5. Update rider stats
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

		// 2.1 同步订单状态为 rider_delivered（允许幂等）
		result.Order, err = q.UpdateOrderToRiderDelivered(ctx, arg.OrderID)
		if err != nil {
			if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("update order to rider_delivered: %w", err)
			}
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

		if rider.Status != RiderStatusActive && rider.IsOnline {
			if _, err := maybeSetRiderOfflineWhenNotEligible(ctx, q, rider); err != nil {
				return err
			}
		}

		return nil
	})

	return result, err
}
