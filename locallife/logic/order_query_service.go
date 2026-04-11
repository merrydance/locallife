package logic

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
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

	etaMinutes, estimatedDeliveryAt := s.buildGetUserOrderDeliveryETA(ctx, order)

	var wechatTransactionID *string
	if po, err := s.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: "order",
	}); err == nil && po.TransactionID.Valid {
		wechatTransactionID = &po.TransactionID.String
	}

	return GetUserOrderQueryResult{
		Order:               order,
		Items:               items,
		DeliveryEtaMinutes:  etaMinutes,
		EstimatedDeliveryAt: estimatedDeliveryAt,
		WechatTransactionID: wechatTransactionID,
	}, nil
}

func (s *OrderService) buildGetUserOrderDeliveryETA(ctx context.Context, order db.GetOrderWithDetailsRow) (*int32, *time.Time) {
	if order.OrderType != "takeout" || order.Status == db.OrderStatusCancelled {
		return nil, nil
	}

	delivery, err := s.store.GetDeliveryByOrderID(ctx, order.ID)
	if err == nil {
		if delivery.EstimatedDeliveryAt.Valid {
			estimatedAt := delivery.EstimatedDeliveryAt.Time
			delta := time.Until(estimatedAt)
			eta := int32(math.Ceil(delta.Minutes()))
			if eta < 0 {
				eta = 0
			}
			return &eta, &estimatedAt
		}

		distance := ExtractDistance(delivery.Distance, order.DeliveryDistance)
		eta := ComputeDeliveryETA(ctx, s.store, order.MerchantID, distance, EstimateDurationSecByDistance(distance))
		estimatedAt := time.Now().Add(time.Duration(eta.DeliveryEtaMinutes) * time.Minute)
		etaMinutes := eta.DeliveryEtaMinutes
		return &etaMinutes, &estimatedAt
	}

	distance := ExtractDistance(0, order.DeliveryDistance)
	if distance > 0 {
		eta := ComputeDeliveryETA(ctx, s.store, order.MerchantID, distance, EstimateDurationSecByDistance(distance))
		estimatedAt := time.Now().Add(time.Duration(eta.DeliveryEtaMinutes) * time.Minute)
		etaMinutes := eta.DeliveryEtaMinutes
		return &etaMinutes, &estimatedAt
	}

	if !errors.Is(err, db.ErrRecordNotFound) {
		log.Warn().Err(err).Int64("order_id", order.ID).Msg("get delivery by order failed")
	}

	return nil, nil
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

	total, err := s.store.CountOrdersByUserWithFilters(ctx, db.CountOrdersByUserWithFiltersParams{
		UserID:        input.UserID,
		Status:        status,
		OrderType:     orderType,
		ReservationID: reservationID,
	})
	if err != nil {
		return ListUserOrdersQueryResult{}, err
	}

	return ListUserOrdersQueryResult{Orders: orders, TotalCount: total}, nil
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
	status := pgtype.Text{}
	if input.Status != nil && *input.Status != "" {
		status = pgtype.Text{String: *input.Status, Valid: true}
	}
	orderType := pgtype.Text{}
	if input.OrderType != nil && *input.OrderType != "" {
		orderType = pgtype.Text{String: *input.OrderType, Valid: true}
	}

	orders, err := s.store.ListOrdersByMerchantWithFilters(ctx, db.ListOrdersByMerchantWithFiltersParams{
		MerchantID: input.MerchantID,
		Status:     status,
		OrderType:  orderType,
		Limit:      input.PageSize,
		Offset:     offset,
	})
	if err != nil {
		return ListMerchantOrdersQueryResult{}, err
	}

	total, err := s.store.CountOrdersByMerchantWithFilters(ctx, db.CountOrdersByMerchantWithFiltersParams{
		MerchantID: input.MerchantID,
		Status:     status,
		OrderType:  orderType,
	})
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
