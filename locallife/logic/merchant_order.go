package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

// MerchantOrderUpdateInput carries merchant order update parameters.
type MerchantOrderUpdateInput struct {
	MerchantID int64
	OrderID    int64
	OperatorID int64
	Reason     string
}

// MerchantOrderUpdateResult includes previous and updated order states.
type MerchantOrderUpdateResult struct {
	Order    db.Order
	Previous db.Order
	PoolItem *db.DeliveryPool
}

// AcceptMerchantOrder validates and accepts a paid order.
func AcceptMerchantOrder(ctx context.Context, store db.Store, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	order, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return MerchantOrderUpdateResult{}, err
	}
	if order.MerchantID != input.MerchantID {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}
	suspension, err := GetTakeoutSuspension(ctx, store, input.MerchantID)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}
	if suspension != nil {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("商户外卖接单已暂停"))
	}
	if order.Status != db.OrderStatusPaid {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only paid orders can be accepted"))
	}

	if order.OrderType == db.OrderTypeTakeout {
		result, err := store.AcceptTakeoutOrderTx(ctx, db.AcceptTakeoutOrderTxParams{
			OrderID:      input.OrderID,
			OldStatus:    order.Status,
			OperatorID:   input.OperatorID,
			OperatorType: "merchant",
		})
		if err != nil {
			if errors.Is(err, db.ErrTakeoutOrderPausedByFoodSafety) {
				return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("食安暂停期间不可继续处理外卖订单"))
			}
			return MerchantOrderUpdateResult{}, err
		}

		return MerchantOrderUpdateResult{Order: result.Order, Previous: order, PoolItem: &result.PoolItem}, nil
	}

	fulfillment := db.FulfillmentStatusPreparing
	result, err := store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:              input.OrderID,
		NewStatus:            db.OrderStatusPreparing,
		OldStatus:            order.Status,
		OperatorID:           input.OperatorID,
		OperatorType:         "merchant",
		NewFulfillmentStatus: &fulfillment,
	})
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	return MerchantOrderUpdateResult{Order: result.Order, Previous: order}, nil
}

// RejectMerchantOrder validates and rejects a paid order.
// Uses CancelOrderTx to atomically rollback vouchers, membership balance, and inventory.
func RejectMerchantOrder(ctx context.Context, store db.Store, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	order, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return MerchantOrderUpdateResult{}, err
	}
	if order.MerchantID != input.MerchantID {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}
	if order.Status != "paid" {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only paid orders can be rejected"))
	}

	cancelReason := fmt.Sprintf("商户拒单：%s", input.Reason)

	// R-06 修复：使用 CancelOrderTx 代替 UpdateOrderStatusTx
	// 确保优惠券回滚、会员余额退回、库存恢复等关联操作原子执行
	result, err := store.CancelOrderTx(ctx, db.CancelOrderTxParams{
		OrderID:      input.OrderID,
		OldStatus:    order.Status,
		CancelReason: cancelReason,
		OperatorID:   input.OperatorID,
		OperatorType: "merchant",
	})
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	return MerchantOrderUpdateResult{Order: result.Order, Previous: order}, nil
}

// MarkMerchantOrderReady marks a preparing order as ready.
func MarkMerchantOrderReady(ctx context.Context, store db.Store, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	order, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return MerchantOrderUpdateResult{}, err
	}
	if order.MerchantID != input.MerchantID {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}
	if order.OrderType == db.OrderTypeTakeout {
		if order.Status != db.OrderStatusPreparing && order.Status != db.OrderStatusCourierAccepted {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only preparing orders can be marked as ready"))
		}
		if order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus == db.FulfillmentStatusReady {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("order already marked as ready"))
		}
		result, err := store.MarkTakeoutOrderReadyTx(ctx, db.MarkTakeoutOrderReadyTxParams{
			OrderID:      input.OrderID,
			OldStatus:    order.Status,
			OperatorID:   input.OperatorID,
			OperatorType: "merchant",
		})
		if err != nil {
			if errors.Is(err, db.ErrTakeoutOrderPausedByFoodSafety) {
				return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("食安暂停期间不可继续处理外卖订单"))
			}
			return MerchantOrderUpdateResult{}, err
		}

		return MerchantOrderUpdateResult{Order: result.Order, Previous: order}, nil
	}
	if order.Status != db.OrderStatusPreparing {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only preparing orders can be marked as ready"))
	}

	fulfillment := db.FulfillmentStatusReady
	result, err := store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:              input.OrderID,
		NewStatus:            db.OrderStatusReady,
		OldStatus:            order.Status,
		OperatorID:           input.OperatorID,
		OperatorType:         "merchant",
		NewFulfillmentStatus: &fulfillment,
	})
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	return MerchantOrderUpdateResult{Order: result.Order, Previous: order}, nil
}

// CompleteMerchantOrder completes a ready non-takeout order.
func CompleteMerchantOrder(ctx context.Context, store db.Store, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	order, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantOrderUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return MerchantOrderUpdateResult{}, err
	}
	if order.MerchantID != input.MerchantID {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}
	if order.OrderType == "takeout" {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("takeout orders cannot be completed by merchant"))
	}
	if order.Status != db.OrderStatusReady {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only ready orders can be completed"))
	}

	result, err := store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      input.OrderID,
		OldStatus:    order.Status,
		OperatorID:   input.OperatorID,
		OperatorType: "merchant",
	})
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	return MerchantOrderUpdateResult{Order: result.Order, Previous: order}, nil
}
