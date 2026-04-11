package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEnsureReservationSingleActiveOrder(t *testing.T) {
	reservationID := int64(900)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, err error)
	}{
		{
			name: "NoExistingOrder",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "StoreError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(db.Order{}, errors.New("boom"))
			},
			check: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "CancelledOrder",
			buildStubs: func(store *mockdb.MockStore) {
				order := db.Order{Status: "cancelled"}
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "ReplacedOrder",
			buildStubs: func(store *mockdb.MockStore) {
				order := db.Order{Status: "paid", ReplacedByOrderID: pgtype.Int8{Int64: 11, Valid: true}}
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "ActiveOrder",
			buildStubs: func(store *mockdb.MockStore) {
				order := db.Order{Status: "paid"}
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation already has an active order", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			err := EnsureReservationSingleActiveOrder(context.Background(), store, reservationID)
			tc.check(t, err)
		})
	}
}

func TestBindDiningSessionActiveOrder(t *testing.T) {
	sessionID := int64(120)
	orderID := int64(240)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		UpdateDiningSessionActiveOrder(gomock.Any(), db.UpdateDiningSessionActiveOrderParams{
			ID:            sessionID,
			ActiveOrderID: pgtype.Int8{Int64: orderID, Valid: true},
		}).
		Times(1).
		Return(db.DiningSession{}, nil)

	err := BindDiningSessionActiveOrder(context.Background(), store, sessionID, orderID)
	require.NoError(t, err)
}

func TestClearDiningOrderCart(t *testing.T) {
	userID := int64(10)
	merchantID := int64(20)
	orderType := "dine_in"
	tableID := int64(30)
	reservationID := int64(40)

	testCases := []struct {
		name       string
		input      ClearDiningOrderCartInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, err error)
	}{
		{
			name:  "NoCart",
			input: ClearDiningOrderCartInput{UserID: userID, MerchantID: merchantID, OrderType: orderType},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
						UserID:        userID,
						MerchantID:    merchantID,
						OrderType:     orderType,
						TableID:       pgtype.Int8{},
						ReservationID: pgtype.Int8{},
					}).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:  "StoreError",
			input: ClearDiningOrderCartInput{UserID: userID, MerchantID: merchantID, OrderType: orderType},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, errors.New("boom"))
			},
			check: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "ClearCart",
			input: ClearDiningOrderCartInput{
				UserID:        userID,
				MerchantID:    merchantID,
				OrderType:     orderType,
				TableID:       &tableID,
				ReservationID: &reservationID,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
						UserID:        userID,
						MerchantID:    merchantID,
						OrderType:     orderType,
						TableID:       pgtype.Int8{Int64: tableID, Valid: true},
						ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
					}).
					Times(1).
					Return(db.Cart{ID: 77}, nil)
				store.EXPECT().
					ClearCart(gomock.Any(), int64(77)).
					Times(1).
					Return(nil)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			err := ClearDiningOrderCart(context.Background(), store, tc.input)
			tc.check(t, err)
		})
	}
}
