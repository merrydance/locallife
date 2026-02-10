package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 替换订单事务 ====================

// ReplaceOrderTxParams contains the input parameters for replacing an order atomically
type ReplaceOrderTxParams struct {
	// 新订单参数
	CreateOrderParams CreateOrderParams
	Items             []CreateOrderItemParams
	// 旧订单信息
	OldOrderID   int64
	CancelReason string
}

// ReplaceOrderTxResult contains the result of the replace order transaction
type ReplaceOrderTxResult struct {
	NewOrder Order
	OldOrder Order
	Items    []OrderItem
}

// ReplaceOrderTx atomically creates a new order and marks the old order as replaced.
// P1-020 修复：确保创建新订单和标记旧订单在同一事务中完成
func (store *SQLStore) ReplaceOrderTx(ctx context.Context, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error) {
	var result ReplaceOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 创建新订单
		result.NewOrder, err = q.CreateOrder(ctx, arg.CreateOrderParams)
		if err != nil {
			return fmt.Errorf("create replacement order: %w", err)
		}

		// 2. 创建订单项
		result.Items = make([]OrderItem, len(arg.Items))
		for i, item := range arg.Items {
			item.OrderID = result.NewOrder.ID
			result.Items[i], err = q.CreateOrderItem(ctx, item)
			if err != nil {
				return fmt.Errorf("create order item %d: %w", i, err)
			}
		}

		// 3. 创建状态日志
		_, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:    result.NewOrder.ID,
			FromStatus: pgtype.Text{String: "", Valid: false},
			ToStatus:   result.NewOrder.Status,
		})
		if err != nil {
			return fmt.Errorf("create status log: %w", err)
		}

		// 4. 标记旧订单为被替换
		result.OldOrder, err = q.MarkOrderReplaced(ctx, MarkOrderReplacedParams{
			ID:                arg.OldOrderID,
			ReplacedByOrderID: pgtype.Int8{Int64: result.NewOrder.ID, Valid: true},
			CancelReason:      pgtype.Text{String: arg.CancelReason, Valid: arg.CancelReason != ""},
		})
		if err != nil {
			return fmt.Errorf("mark old order replaced: %w", err)
		}

		return nil
	})

	return result, err
}
