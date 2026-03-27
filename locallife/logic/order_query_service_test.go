package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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

	result, err := service.ListMerchantOrders(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, expectedOrders, result.Orders)
	require.Equal(t, int64(21), result.TotalCount)
}
