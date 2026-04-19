package algorithm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvaluateFoodSafetyReport_RequiresThreeDistinctUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	handler := NewFoodSafetyHandler(store, nil)

	currentOrder := db.Order{ID: 1001, UserID: 1, MerchantID: 88, AddressID: pgtype.Int8{Int64: 11, Valid: true}}
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), currentOrder.MerchantID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{
		{ID: 10, OrderID: 2001, UserID: 2},
		{ID: 11, OrderID: 2002, UserID: 2},
	}, nil)

	result, err := handler.EvaluateFoodSafetyReport(context.Background(), FoodSafetyReportInput{
		ReporterUserID: currentOrder.UserID,
		MerchantID:     currentOrder.MerchantID,
		Order:          currentOrder,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.ShouldCircuitBreak)
	require.False(t, result.IsMalicious)
	require.Equal(t, "insufficient-reports", result.ReasonCode)
}

func TestEvaluateFoodSafetyReport_FlagsSharedDeviceAsMalicious(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	handler := NewFoodSafetyHandler(store, nil)

	currentOrder := db.Order{ID: 1002, UserID: 1, MerchantID: 99, AddressID: pgtype.Int8{Int64: 21, Valid: true}}
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), currentOrder.MerchantID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{
		{ID: 20, OrderID: 3001, UserID: 2},
		{ID: 21, OrderID: 3002, UserID: 3},
	}, nil)
	store.EXPECT().CountUserOrders(gomock.Any(), int64(1)).Return(int32(2), nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(1)).Return([]db.UserDevice{{DeviceID: "device-a"}}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(2)).Return([]db.UserDevice{{DeviceFingerprint: pgtype.Text{String: "shared-device", Valid: true}}}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(3)).Return([]db.UserDevice{{DeviceFingerprint: pgtype.Text{String: "shared-device", Valid: true}}}, nil)

	result, err := handler.EvaluateFoodSafetyReport(context.Background(), FoodSafetyReportInput{
		ReporterUserID: currentOrder.UserID,
		MerchantID:     currentOrder.MerchantID,
		Order:          currentOrder,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.ShouldCircuitBreak)
	require.True(t, result.IsMalicious)
	require.Equal(t, "malicious-coordinated-reports", result.ReasonCode)
}

func TestEvaluateFoodSafetyReport_FlagsSharedAddressAsMalicious(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	handler := NewFoodSafetyHandler(store, nil)

	currentOrder := db.Order{ID: 1003, UserID: 1, MerchantID: 77, AddressID: pgtype.Int8{Int64: 31, Valid: true}}
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), currentOrder.MerchantID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{
		{ID: 30, OrderID: 4001, UserID: 2},
		{ID: 31, OrderID: 4002, UserID: 3},
	}, nil)
	store.EXPECT().CountUserOrders(gomock.Any(), int64(1)).Return(int32(2), nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(1)).Return([]db.UserDevice{{DeviceID: "device-a"}}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(2)).Return([]db.UserDevice{{DeviceID: "device-b"}}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), int64(3)).Return([]db.UserDevice{{DeviceID: "device-c"}}, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(4001)).Return(db.Order{ID: 4001, UserID: 2, AddressID: pgtype.Int8{Int64: 31, Valid: true}}, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(4002)).Return(db.Order{ID: 4002, UserID: 3, AddressID: pgtype.Int8{Int64: 91, Valid: true}}, nil)

	result, err := handler.EvaluateFoodSafetyReport(context.Background(), FoodSafetyReportInput{
		ReporterUserID: currentOrder.UserID,
		MerchantID:     currentOrder.MerchantID,
		Order:          currentOrder,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.ShouldCircuitBreak)
	require.True(t, result.IsMalicious)
	require.Equal(t, "malicious-coordinated-reports", result.ReasonCode)
}
