package db

import (
	"context"
	"errors"
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

type ReplaceOrderWithRefundOrdersTxParams struct {
	ReplaceOrderTxParams
	RefundOrders []CreateRefundOrderTxParams
}

type ReplaceOrderWithRefundOrdersTxResult struct {
	ReplaceOrderTxResult
	RefundOrders []RefundOrder
}

// ReplaceOrderTx atomically creates a new order and marks the old order as replaced.
// P1-020 修复：确保创建新订单和标记旧订单在同一事务中完成
func (store *SQLStore) ReplaceOrderTx(ctx context.Context, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error) {
	var result ReplaceOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		result, err = replaceOrderWithQueries(ctx, q, arg)
		return err
	})

	return result, err
}

func (store *SQLStore) ReplaceOrderWithRefundOrdersTx(ctx context.Context, arg ReplaceOrderWithRefundOrdersTxParams) (ReplaceOrderWithRefundOrdersTxResult, error) {
	var result ReplaceOrderWithRefundOrdersTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		replaceResult, err := replaceOrderWithQueries(ctx, q, arg.ReplaceOrderTxParams)
		if err != nil {
			return err
		}
		result.ReplaceOrderTxResult = replaceResult

		result.RefundOrders = make([]RefundOrder, 0, len(arg.RefundOrders))
		for _, refundArg := range arg.RefundOrders {
			refundResult, err := createRefundOrderWithGuard(ctx, q, refundArg)
			if err != nil {
				return err
			}
			result.RefundOrders = append(result.RefundOrders, refundResult.RefundOrder)
		}

		return nil
	})

	return result, err
}

func replaceOrderWithQueries(ctx context.Context, q *Queries, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error) {
	var result ReplaceOrderTxResult

	oldOrder, err := q.GetOrderForUpdate(ctx, arg.OldOrderID)
	if err != nil {
		return result, fmt.Errorf("lock old order: %w", err)
	}
	if oldOrder.Status != OrderStatusPaid || oldOrder.ReplacedByOrderID.Valid {
		return result, &requestError{statusCode: 409, err: errors.New("order is no longer replaceable")}
	}

	// 1. 创建新订单
	result.NewOrder, err = q.CreateOrder(ctx, arg.CreateOrderParams)
	if err != nil {
		return result, fmt.Errorf("create replacement order: %w", err)
	}

	// 2. 创建订单项
	result.Items = make([]OrderItem, len(arg.Items))
	for i, item := range arg.Items {
		item.OrderID = result.NewOrder.ID
		result.Items[i], err = q.CreateOrderItem(ctx, item)
		if err != nil {
			return result, fmt.Errorf("create order item %d: %w", i, err)
		}
	}

	// 3. 创建状态日志
	_, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
		OrderID:    result.NewOrder.ID,
		FromStatus: pgtype.Text{String: "", Valid: false},
		ToStatus:   result.NewOrder.Status,
	})
	if err != nil {
		return result, fmt.Errorf("create status log: %w", err)
	}

	// 4. 标记旧订单为被替换
	result.OldOrder, err = q.MarkOrderReplaced(ctx, MarkOrderReplacedParams{
		ID:                arg.OldOrderID,
		ReplacedByOrderID: pgtype.Int8{Int64: result.NewOrder.ID, Valid: true},
		CancelReason:      pgtype.Text{String: arg.CancelReason, Valid: arg.CancelReason != ""},
	})
	if err != nil {
		return result, fmt.Errorf("mark old order replaced: %w", err)
	}

	oldGroupLinks, err := q.db.Query(ctx, `
		SELECT billing_group_id
		FROM billing_group_orders
		WHERE order_id = $1
		ORDER BY id ASC`, arg.OldOrderID)
	if err != nil {
		return result, fmt.Errorf("list old billing group links: %w", err)
	}

	groupIDs := make([]int64, 0, 1)

	for oldGroupLinks.Next() {
		var billingGroupID int64
		if err := oldGroupLinks.Scan(&billingGroupID); err != nil {
			oldGroupLinks.Close()
			return result, fmt.Errorf("scan old billing group link: %w", err)
		}
		groupIDs = append(groupIDs, billingGroupID)
	}
	if err := oldGroupLinks.Err(); err != nil {
		oldGroupLinks.Close()
		return result, fmt.Errorf("iterate old billing group links: %w", err)
	}
	oldGroupLinks.Close()

	for _, billingGroupID := range groupIDs {
		if _, err := q.CreateBillingGroupOrder(ctx, CreateBillingGroupOrderParams{
			BillingGroupID: billingGroupID,
			OrderID:        result.NewOrder.ID,
			Amount:         result.NewOrder.TotalAmount,
			Status:         "linked",
		}); err != nil {
			return result, fmt.Errorf("create replacement billing group order: %w", err)
		}
	}

	return result, nil
}
