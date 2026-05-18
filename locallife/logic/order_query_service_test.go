package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOrderServiceListMerchantOrders_RejectsPendingStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	status := db.OrderStatusPending
	_, err := service.ListMerchantOrders(context.Background(), ListMerchantOrdersQueryInput{
		MerchantID: 88,
		Status:     &status,
		PageID:     1,
		PageSize:   10,
	})

	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.EqualError(t, reqErr.Err, "订单尚未支付，暂不可处理")
}

func TestOrderServiceGetMerchantOrder_RejectsPendingBeforeLoadingItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	order := db.Order{ID: 901, MerchantID: 88, Status: db.OrderStatusPending}
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)

	_, err := service.GetMerchantOrder(context.Background(), GetMerchantOrderQueryInput{
		MerchantID: order.MerchantID,
		OrderID:    order.ID,
	})

	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.True(t, errors.Is(reqErr.Err, ErrMerchantOrderNotPaid))
	require.EqualError(t, reqErr.Err, "订单尚未支付，暂不可处理")
}

func TestOrderServiceListMerchantOrders_WithOrderTypeFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	status := "paid"
	orderType := "reservation"
	input := ListMerchantOrdersQueryInput{
		MerchantID: 88,
		Status:     &status,
		OrderType:  &orderType,
		PageID:     2,
		PageSize:   10,
	}

	expectedOrders := []db.Order{{ID: 901, MerchantID: input.MerchantID, Status: status, OrderType: orderType}}
	expectedItems := []db.ListOrderItemsWithDishByOrderIDsRow{{ID: 3001, OrderID: expectedOrders[0].ID, Name: "测试菜品", Quantity: 1}}
	store.EXPECT().
		ListOrdersByMerchantWithFilters(gomock.Any(), db.ListOrdersByMerchantWithFiltersParams{
			MerchantID: input.MerchantID,
			Status:     pgtype.Text{String: status, Valid: true},
			OrderType:  pgtype.Text{String: orderType, Valid: true},
			Limit:      input.PageSize,
			Offset:     10,
		}).
		Times(1).
		Return(expectedOrders, nil)
	store.EXPECT().
		CountOrdersByMerchantWithFilters(gomock.Any(), db.CountOrdersByMerchantWithFiltersParams{
			MerchantID: input.MerchantID,
			Status:     pgtype.Text{String: status, Valid: true},
			OrderType:  pgtype.Text{String: orderType, Valid: true},
		}).
		Times(1).
		Return(int64(21), nil)
	store.EXPECT().
		ListOrderItemsWithDishByOrderIDs(gomock.Any(), gomock.Eq([]int64{expectedOrders[0].ID})).
		Times(1).
		Return(expectedItems, nil)

	result, err := service.ListMerchantOrders(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, expectedOrders, result.Orders)
	require.Equal(t, expectedItems, result.ItemsByOrderID[expectedOrders[0].ID])
	require.Equal(t, int64(21), result.TotalCount)
}

func TestOrderServiceListUserOrders_ReturnsTotalCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	status := "paid"
	orderType := "takeout"
	reservationID := int64(0)
	input := ListUserOrdersQueryInput{
		UserID:        66,
		Status:        &status,
		OrderType:     &orderType,
		ReservationID: &reservationID,
		PageID:        3,
		PageSize:      5,
	}

	expectedOrders := []db.ListOrdersByUserWithFiltersRow{{ID: 501, UserID: input.UserID, Status: status, OrderType: orderType}}
	store.EXPECT().
		ListOrdersByUserWithFilters(gomock.Any(), db.ListOrdersByUserWithFiltersParams{
			UserID:        input.UserID,
			Status:        pgtype.Text{String: status, Valid: true},
			OrderType:     pgtype.Text{String: orderType, Valid: true},
			ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
			Limit:         input.PageSize,
			Offset:        10,
		}).
		Times(1).
		Return(expectedOrders, nil)
	store.EXPECT().
		CountOrdersByUserWithFilters(gomock.Any(), db.CountOrdersByUserWithFiltersParams{
			UserID:        input.UserID,
			Status:        pgtype.Text{String: status, Valid: true},
			OrderType:     pgtype.Text{String: orderType, Valid: true},
			ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		}).
		Times(1).
		Return(int64(17), nil)

	result, err := service.ListUserOrders(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, expectedOrders, result.Orders)
	require.Equal(t, int64(17), result.TotalCount)
}
