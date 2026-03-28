package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAutoAdvancePickup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 1, UserID: 10}
	delivery := db.Delivery{ID: 2, OrderID: 3, Status: "assigned"}
	order := db.Order{ID: 3, Status: "courier_accepted"}

	store.EXPECT().
		GetOrder(gomock.Any(), int64(3)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		UpdateDeliveryToPickupTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.UpdateDeliveryToPickupTxResult{Delivery: delivery}, nil)

	result, err := AutoAdvancePickup(context.Background(), store, delivery, rider)
	require.NoError(t, err)
	require.True(t, result.Updated)
}

func TestAutoConfirmPickup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 1, UserID: 10}
	delivery := db.Delivery{ID: 2, OrderID: 3, Status: "picking", RiderID: pgtype.Int8{Int64: 1, Valid: true}}
	order := db.Order{ID: 3, Status: "courier_accepted"}

	store.EXPECT().
		GetOrder(gomock.Any(), int64(3)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		UpdateDeliveryToPickedTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.UpdateDeliveryToPickedTxResult{Delivery: delivery}, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)

	result, err := AutoConfirmPickup(context.Background(), store, delivery, rider)
	require.NoError(t, err)
	require.True(t, result.Updated)
	require.Equal(t, "picked", result.Order.Status)
}

func TestAutoConfirmDelivery_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 1, UserID: 10,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	delivery := db.Delivery{ID: 2, OrderID: 3, Status: "delivering", DeliveryFee: 500,
		DeliveryLongitude: numericFromFloatStatus(120.0),
		DeliveryLatitude:  numericFromFloatStatus(30.0),
	}
	order := db.Order{ID: 3, Status: "delivering"}

	store.EXPECT().
		GetOrder(gomock.Any(), int64(3)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		CompleteDeliveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CompleteDeliveryTxResult{Delivery: delivery}, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)

	result, err := AutoConfirmDelivery(context.Background(), store, delivery, rider, 500, 120, 5000)
	require.NoError(t, err)
	require.True(t, result.Updated)
	require.Equal(t, "rider_delivered", result.Order.Status)
}

func TestAutoConfirmDelivery_RiderLocationMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 1, UserID: 10}
	delivery := db.Delivery{ID: 2, OrderID: 3, Status: "delivering", DeliveryFee: 500,
		DeliveryLongitude: numericFromFloatStatus(120.0),
		DeliveryLatitude:  numericFromFloatStatus(30.0),
	}

	result, err := AutoConfirmDelivery(context.Background(), store, delivery, rider, 500, 120, 5000)
	require.Error(t, err)
	require.Equal(t, "骑手定位缺失，无法确认送达，请先刷新定位", err.Error())
	require.False(t, result.Updated)
}

func TestAutoConfirmDelivery_RiderLocationStale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 1, UserID: 10,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now().Add(-3 * time.Minute), Valid: true},
	}
	delivery := db.Delivery{ID: 2, OrderID: 3, Status: "delivering", DeliveryFee: 500,
		DeliveryLongitude: numericFromFloatStatus(120.0),
		DeliveryLatitude:  numericFromFloatStatus(30.0),
	}

	result, err := AutoConfirmDelivery(context.Background(), store, delivery, rider, 500, 120, 5000)
	require.Error(t, err)
	require.Equal(t, "骑手定位已过期，无法确认送达，请刷新定位后重试", err.Error())
	require.False(t, result.Updated)
}
