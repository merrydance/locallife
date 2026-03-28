package logic

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func numericFromFloatStatus(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestStartPickup_NotRider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(db.Rider{}, db.ErrRecordNotFound)

	_, err := StartPickup(context.Background(), store, DeliveryStatusInput{UserID: 1, DeliveryID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
}

func TestStartPickup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "assigned", RiderID: pgtype.Int8{Int64: 10, Valid: true}}
	order := db.Order{ID: 2, Status: "courier_accepted"}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		UpdateDeliveryToPickupTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.UpdateDeliveryToPickupTxResult{Delivery: delivery}, nil)

	result, err := StartPickup(context.Background(), store, DeliveryStatusInput{UserID: 1, DeliveryID: 2})
	require.NoError(t, err)
	require.Equal(t, delivery.ID, result.Delivery.ID)
}

func TestConfirmPickup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "picking", RiderID: pgtype.Int8{Int64: 10, Valid: true}}
	order := db.Order{ID: 2, Status: "courier_accepted"}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
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

	result, err := ConfirmPickup(context.Background(), store, DeliveryStatusInput{UserID: 1, DeliveryID: 2})
	require.NoError(t, err)
	require.Equal(t, "picked", result.Order.Status)
}

func TestStartDelivery_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "picked", RiderID: pgtype.Int8{Int64: 10, Valid: true}}
	order := db.Order{ID: 2, Status: "picked"}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		UpdateDeliveryToDeliveringTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.UpdateDeliveryToDeliveringTxResult{Delivery: delivery}, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)

	result, err := StartDelivery(context.Background(), store, DeliveryStatusInput{UserID: 1, DeliveryID: 2})
	require.NoError(t, err)
	require.Equal(t, "delivering", result.Order.Status)
}

func TestConfirmDelivery_DistanceTooFar(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "delivering", RiderID: pgtype.Int8{Int64: 10, Valid: true},
		DeliveryLongitude: numericFromFloatStatus(121.0),
		DeliveryLatitude:  numericFromFloatStatus(31.0),
	}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)

	_, err := ConfirmDelivery(context.Background(), store, ConfirmDeliveryInput{UserID: 1, DeliveryID: 2, ConfirmRadiusMeters: 100, LocationMaxAgeSec: 120})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestConfirmDelivery_RiderLocationMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "delivering", RiderID: pgtype.Int8{Int64: 10, Valid: true},
		DeliveryLongitude: numericFromFloatStatus(121.0),
		DeliveryLatitude:  numericFromFloatStatus(31.0),
	}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)

	_, err := ConfirmDelivery(context.Background(), store, ConfirmDeliveryInput{UserID: 1, DeliveryID: 2, ConfirmRadiusMeters: 100, LocationMaxAgeSec: 120})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "骑手定位缺失，无法确认送达，请先刷新定位", reqErr.Err.Error())
}

func TestConfirmDelivery_DropoffLocationMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "delivering", RiderID: pgtype.Int8{Int64: 10, Valid: true}}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)

	_, err := ConfirmDelivery(context.Background(), store, ConfirmDeliveryInput{UserID: 1, DeliveryID: 2, ConfirmRadiusMeters: 100, LocationMaxAgeSec: 120})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "收货位置缺失，无法确认送达，请联系平台处理", reqErr.Err.Error())
}

func TestConfirmDelivery_RiderLocationStale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now().Add(-3 * time.Minute), Valid: true},
	}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "delivering", RiderID: pgtype.Int8{Int64: 10, Valid: true},
		DeliveryLongitude: numericFromFloatStatus(120.0),
		DeliveryLatitude:  numericFromFloatStatus(30.0),
	}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)

	_, err := ConfirmDelivery(context.Background(), store, ConfirmDeliveryInput{UserID: 1, DeliveryID: 2, ConfirmRadiusMeters: 100, LocationMaxAgeSec: 120})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "骑手定位已过期，无法确认送达，请刷新定位后重试", reqErr.Err.Error())
}

func TestConfirmDelivery_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1,
		CurrentLongitude:  numericFromFloatStatus(120.0),
		CurrentLatitude:   numericFromFloatStatus(30.0),
		LocationUpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	delivery := db.Delivery{ID: 20, OrderID: 2, Status: "delivering", RiderID: pgtype.Int8{Int64: 10, Valid: true},
		DeliveryLongitude: numericFromFloatStatus(120.0),
		DeliveryLatitude:  numericFromFloatStatus(30.0),
		DeliveryFee:       500,
	}
	order := db.Order{ID: 2, Status: "delivering", TotalAmount: 1000}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetDelivery(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
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

	result, err := ConfirmDelivery(context.Background(), store, ConfirmDeliveryInput{UserID: 1, DeliveryID: 2, ConfirmRadiusMeters: 1000, LocationMaxAgeSec: 120})
	require.NoError(t, err)
	require.Equal(t, "rider_delivered", result.Order.Status)
}
