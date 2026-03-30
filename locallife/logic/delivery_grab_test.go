package logic

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func numericFromFloatGrab(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestGrabDeliveryOrder_NotRider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(db.Rider{}, db.ErrRecordNotFound)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
}

func TestGrabDeliveryOrder_NoRegionStillChecksDistance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000,
		CurrentLongitude: numericFromFloatGrab(120.0), CurrentLatitude: numericFromFloatGrab(30.0)}
	merchant := db.Merchant{ID: 20, RegionID: 3, Longitude: numericFromFloatGrab(121.0), Latitude: numericFromFloatGrab(31.0)}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: 20, ExpiresAt: time.Now().Add(time.Hour)}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(20)).
		Times(1).
		Return(merchant, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2, MaxDistanceMeters: 100})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestGrabDeliveryOrder_DistanceTooFar(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true,
		CurrentLongitude: numericFromFloatGrab(120.0), CurrentLatitude: numericFromFloatGrab(30.0)}
	merchant := db.Merchant{ID: 20, RegionID: 9, Longitude: numericFromFloatGrab(121.0), Latitude: numericFromFloatGrab(31.0)}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: merchant.ID, ExpiresAt: time.Now().Add(time.Hour)}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2, MaxDistanceMeters: 100})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestGrabDeliveryOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}
	merchant := db.Merchant{ID: 20, RegionID: 9}
	delivery := db.Delivery{ID: 30, OrderID: 2}
	order := db.Order{ID: 2, Status: db.OrderStatusReady, TotalAmount: 500}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: merchant.ID, ExpiresAt: time.Now().Add(time.Hour), DeliveryFee: 500}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		GrabOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.GrabOrderTxResult{Delivery: delivery}, nil)
	store.EXPECT().
		UpdateOrderToCourierAccepted(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Order{ID: 2, Status: "courier_accepted"}, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)

	result, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2, MaxDistanceMeters: 5000})
	require.NoError(t, err)
	require.Equal(t, delivery.ID, result.Delivery.ID)
}

func TestGrabDeliveryOrder_PreparingOrderRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}
	merchant := db.Merchant{ID: 20, RegionID: 9}
	delivery := db.Delivery{ID: 30, OrderID: 2}
	order := db.Order{ID: 2, Status: db.OrderStatusPreparing, TotalAmount: 500}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: merchant.ID, ExpiresAt: time.Now().Add(time.Hour), DeliveryFee: 500}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "商户未出餐，暂不可抢单", reqErr.Err.Error())
}

func TestGrabDeliveryOrder_PaidOrderRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}
	merchant := db.Merchant{ID: 20, RegionID: 9}
	delivery := db.Delivery{ID: 30, OrderID: 2}
	order := db.Order{ID: 2, Status: db.OrderStatusPaid, TotalAmount: 500}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: merchant.ID, ExpiresAt: time.Now().Add(time.Hour), DeliveryFee: 500}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "商户未接单，暂不可抢单", reqErr.Err.Error())
}

func TestGrabDeliveryOrder_Suspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{
			RiderID:       rider.ID,
			IsSuspended:   true,
			SuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
		}, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "骑手接单已暂停", reqErr.Err.Error())
}

func TestGrabDeliveryOrder_NonActiveOnlineRiderRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusApproved, IsOnline: true, DepositAmount: 1000}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "押金不足或账号未激活，暂不可接单", reqErr.Err.Error())
}

func TestGrabDeliveryOrder_HighValueScoreDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: 20, ExpiresAt: time.Now().Add(time.Hour), DeliveryFee: 2000}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(-1), nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
}

func TestGrabDeliveryOrder_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, RegionID: pgtype.Int8{Int64: 9, Valid: true}, DepositAmount: 1000}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: 20, ExpiresAt: time.Now().Add(-time.Hour)}, nil)

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestGrabDeliveryOrder_GrabTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rider := db.Rider{ID: 10, UserID: 1, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 1000}
	merchant := db.Merchant{ID: 20, RegionID: 9}
	delivery := db.Delivery{ID: 30, OrderID: 2}
	order := db.Order{ID: 2, Status: db.OrderStatusReady, TotalAmount: 500}

	store.EXPECT().
		GetRiderByUserID(gomock.Any(), int64(1)).
		Times(1).
		Return(rider, nil)
	store.EXPECT().
		GetRiderProfile(gomock.Any(), rider.ID).
		Times(1).
		Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
	store.EXPECT().
		GetDeliveryPoolByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(db.DeliveryPool{OrderID: 2, MerchantID: merchant.ID, ExpiresAt: time.Now().Add(time.Hour), DeliveryFee: 500}, nil)
	store.EXPECT().
		GetRiderPremiumScore(gomock.Any(), rider.ID).
		Times(1).
		Return(int16(0), db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), int64(2)).
		Times(1).
		Return(delivery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(2)).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		GrabOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.GrabOrderTxResult{}, errors.New("boom"))

	_, err := GrabDeliveryOrder(context.Background(), store, GrabOrderInput{UserID: 1, OrderID: 2})
	require.Error(t, err)
}
