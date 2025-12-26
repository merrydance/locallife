package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 订单状态变更事务 ====================

// UpdateOrderStatusTxParams contains the input parameters for updating order status with log
type UpdateOrderStatusTxParams struct {
	OrderID      int64
	NewStatus    string
	OldStatus    string // 用于日志记录
	OperatorID   int64
	OperatorType string // "user", "merchant", "system"
	Notes        string // 可选备注
}

// UpdateOrderStatusTxResult contains the result of the update order status transaction
type UpdateOrderStatusTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// UpdateOrderStatusTx updates order status and creates a status log in a single transaction
func (store *SQLStore) UpdateOrderStatusTx(ctx context.Context, arg UpdateOrderStatusTxParams) (UpdateOrderStatusTxResult, error) {
	var result UpdateOrderStatusTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新订单状态
		result.Order, err = q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
			ID:     arg.OrderID,
			Status: arg.NewStatus,
		})
		if err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		// 2. 创建状态变更日志
		var notes pgtype.Text
		if arg.Notes != "" {
			notes = pgtype.Text{String: arg.Notes, Valid: true}
		}

		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     arg.NewStatus,
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
			Notes:        notes,
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 订单完成事务 ====================

// CompleteOrderTxParams contains the input parameters for completing an order
type CompleteOrderTxParams struct {
	OrderID      int64
	OldStatus    string
	OperatorID   int64
	OperatorType string
}

// CompleteOrderTxResult contains the result of the complete order transaction
type CompleteOrderTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// CompleteOrderTx completes an order and creates a status log in a single transaction
func (store *SQLStore) CompleteOrderTx(ctx context.Context, arg CompleteOrderTxParams) (CompleteOrderTxResult, error) {
	var result CompleteOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新订单为已完成
		result.Order, err = q.UpdateOrderToCompleted(ctx, arg.OrderID)
		if err != nil {
			return fmt.Errorf("update order to completed: %w", err)
		}

		// 2. 创建状态变更日志
		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     "completed",
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 订单取消事务 ====================

// CancelOrderTxParams contains the input parameters for cancelling an order
type CancelOrderTxParams struct {
	OrderID      int64
	OldStatus    string
	CancelReason string
	OperatorID   int64
	OperatorType string // "user", "merchant", "system"
}

// CancelOrderTxResult contains the result of the cancel order transaction
type CancelOrderTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// CancelOrderTx cancels an order and creates a status log in a single transaction
func (store *SQLStore) CancelOrderTx(ctx context.Context, arg CancelOrderTxParams) (CancelOrderTxResult, error) {
	var result CancelOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新订单为已取消
		var cancelReason pgtype.Text
		if arg.CancelReason != "" {
			cancelReason = pgtype.Text{String: arg.CancelReason, Valid: true}
		}

		result.Order, err = q.UpdateOrderToCancelled(ctx, UpdateOrderToCancelledParams{
			ID:           arg.OrderID,
			CancelReason: cancelReason,
		})
		if err != nil {
			return fmt.Errorf("update order to cancelled: %w", err)
		}

		// 2. 创建状态变更日志
		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     "cancelled",
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
			Notes:        cancelReason,
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		return nil
	})

	return result, err
}
