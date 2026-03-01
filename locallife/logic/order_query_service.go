package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

func (s *OrderService) GetUserOrder(ctx context.Context, input GetUserOrderQueryInput) (GetUserOrderQueryResult, error) {
	order, err := s.store.GetOrderWithDetails(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetUserOrderQueryResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return GetUserOrderQueryResult{}, err
	}

	if order.UserID != input.UserID {
		return GetUserOrderQueryResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	}

	items, err := s.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return GetUserOrderQueryResult{}, err
	}

	return GetUserOrderQueryResult{
		Order: order,
		Items: items,
	}, nil
}

func (s *OrderService) ListUserOrders(ctx context.Context, input ListUserOrdersQueryInput) (ListUserOrdersQueryResult, error) {
	status := pgtype.Text{}
	if input.Status != nil && *input.Status != "" {
		status = pgtype.Text{String: *input.Status, Valid: true}
	}

	orderType := pgtype.Text{}
	if input.OrderType != nil && *input.OrderType != "" {
		orderType = pgtype.Text{String: *input.OrderType, Valid: true}
	}

	reservationID := pgtype.Int8{}
	if input.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *input.ReservationID, Valid: true}
	}

	offset := int32((input.PageID - 1) * input.PageSize)
	orders, err := s.store.ListOrdersByUserWithFilters(ctx, db.ListOrdersByUserWithFiltersParams{
		UserID:        input.UserID,
		Status:        status,
		OrderType:     orderType,
		ReservationID: reservationID,
		Offset:        offset,
		Limit:         input.PageSize,
	})
	if err != nil {
		return ListUserOrdersQueryResult{}, err
	}

	return ListUserOrdersQueryResult{Orders: orders}, nil
}

func (s *OrderService) GetMerchantOrder(ctx context.Context, input GetMerchantOrderQueryInput) (GetMerchantOrderQueryResult, error) {
	order, err := s.store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetMerchantOrderQueryResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return GetMerchantOrderQueryResult{}, err
	}

	if order.MerchantID != input.MerchantID {
		return GetMerchantOrderQueryResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}

	items, err := s.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return GetMerchantOrderQueryResult{}, err
	}

	return GetMerchantOrderQueryResult{
		Order: order,
		Items: items,
	}, nil
}

func (s *OrderService) ListMerchantOrders(ctx context.Context, input ListMerchantOrdersQueryInput) (ListMerchantOrdersQueryResult, error) {
	offset := int32((input.PageID - 1) * input.PageSize)

	var (
		orders []db.Order
		err    error
		total  int64
	)
	if input.Status != nil && *input.Status != "" {
		orders, err = s.store.ListOrdersByMerchantAndStatus(ctx, db.ListOrdersByMerchantAndStatusParams{
			MerchantID: input.MerchantID,
			Status:     *input.Status,
			Limit:      input.PageSize,
			Offset:     offset,
		})
		if err == nil {
			total, err = s.store.CountOrdersByMerchantAndStatus(ctx, db.CountOrdersByMerchantAndStatusParams{
				MerchantID: input.MerchantID,
				Status:     *input.Status,
			})
		}
	} else {
		orders, err = s.store.ListOrdersByMerchant(ctx, db.ListOrdersByMerchantParams{
			MerchantID: input.MerchantID,
			Limit:      input.PageSize,
			Offset:     offset,
		})
		if err == nil {
			total, err = s.store.CountOrdersByMerchant(ctx, input.MerchantID)
		}
	}
	if err != nil {
		return ListMerchantOrdersQueryResult{}, err
	}

	return ListMerchantOrdersQueryResult{Orders: orders, TotalCount: total}, nil
}

func (s *OrderService) GetMerchantOrderStats(ctx context.Context, input GetMerchantOrderStatsQueryInput) (GetMerchantOrderStatsQueryResult, error) {
	stats, err := s.store.GetOrderStats(ctx, db.GetOrderStatsParams{
		MerchantID: input.MerchantID,
		StartAt:    input.StartDate,
		EndAt:      input.EndDate,
	})
	if err != nil {
		return GetMerchantOrderStatsQueryResult{}, err
	}

	return GetMerchantOrderStatsQueryResult{Stats: stats}, nil
}

func (s *OrderService) CalculateOrderPreview(ctx context.Context, input CalculateOrderPreviewInput) (OrderCalculationResult, error) {
	store := s.store
	if input.Store != nil {
		store = input.Store
	}

	mapClient := input.MapClient
	if mapClient == nil {
		mapClient = nil
	}

	return CalculateOrderPreview(
		ctx,
		store,
		mapClient,
		input.OrderCalculationInput,
		input.Normalize,
		input.CalculateFee,
	)
}
