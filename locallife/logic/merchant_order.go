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
	if order.Status != "paid" {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only paid orders can be accepted"))
	}

	fulfillment := "preparing"
	result, err := store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:              input.OrderID,
		NewStatus:            "preparing",
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

	fulfillment := "cancelled"
	result, err := store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:              input.OrderID,
		NewStatus:            "cancelled",
		OldStatus:            order.Status,
		OperatorID:           input.OperatorID,
		OperatorType:         "merchant",
		Notes:                fmt.Sprintf("商户拒单：%s", input.Reason),
		NewFulfillmentStatus: &fulfillment,
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
	if order.Status != "preparing" {
		return MerchantOrderUpdateResult{}, NewRequestError(http.StatusBadRequest, errors.New("only preparing orders can be marked as ready"))
	}

	fulfillment := "ready"
	result, err := store.UpdateOrderStatusTx(ctx, db.UpdateOrderStatusTxParams{
		OrderID:              input.OrderID,
		NewStatus:            "ready",
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
	if order.Status != "ready" {
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
