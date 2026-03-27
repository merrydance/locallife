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

func reservationTimeFrom(t time.Time) pgtype.Time {
	seconds := int64(t.Hour()*3600+t.Minute()*60) * 1000000
	return pgtype.Time{Microseconds: seconds, Valid: true}
}

func TestPrecheckDiningSession(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	dateOnly := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	reservationDate := pgtype.Date{Time: dateOnly, Valid: true}
	reservationTime := reservationTimeFrom(now)

	table := db.Table{ID: 10, MerchantID: 20}
	merchant := db.Merchant{ID: table.MerchantID, OwnerUserID: 99}
	reservation := db.TableReservation{
		ID:              30,
		UserID:          40,
		MerchantID:      merchant.ID,
		TableID:         table.ID,
		Status:          "paid",
		PaymentMode:     paymentModeDeposit,
		DepositAmount:   500,
		PrepaidAmount:   800,
		ReservationDate: reservationDate,
		ReservationTime: reservationTime,
	}

	testCases := []struct {
		name       string
		input      DiningSessionPrecheckInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result DiningSessionPrecheckResult, err error)
	}{
		{
			name:  "TableNotFound",
			input: DiningSessionPrecheckInput{UserID: 1, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ DiningSessionPrecheckResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "table not found", reqErr.Err.Error())
			},
		},
		{
			name:  "NoReservation",
			input: DiningSessionPrecheckInput{UserID: 1, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), db.ListReservationsByTableAndDateParams{
						TableID:         table.ID,
						ReservationDate: reservationDate,
					}).
					Times(1).
					Return([]db.TableReservation{}, nil)
			},
			check: func(t *testing.T, result DiningSessionPrecheckResult, err error) {
				require.NoError(t, err)
				require.False(t, result.Reserved)
				require.Nil(t, result.Reservation)
			},
		},
		{
			name:  "ReservationNotOwner",
			input: DiningSessionPrecheckInput{UserID: 50, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), db.ListReservationsByTableAndDateParams{
						TableID:         table.ID,
						ReservationDate: reservationDate,
					}).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{
						MerchantID: table.MerchantID,
						UserID:     int64(50),
					}).
					Times(1).
					Return(false, nil)
			},
			check: func(t *testing.T, _ DiningSessionPrecheckResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "桌位已被预约，暂时不可用", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationOwnerWithOrder",
			input: DiningSessionPrecheckInput{UserID: reservation.UserID, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), db.ListReservationsByTableAndDateParams{
						TableID:         table.ID,
						ReservationDate: reservationDate,
					}).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{
						MerchantID: table.MerchantID,
						UserID:     reservation.UserID,
					}).
					Times(1).
					Return(false, nil)
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
					Times(1).
					Return(db.Order{ID: 88, Status: "paid", FulfillmentStatus: "scheduled"}, nil)
			},
			check: func(t *testing.T, result DiningSessionPrecheckResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Reserved)
				require.NotNil(t, result.Reservation)
				require.NotNil(t, result.Order)
				require.True(t, result.IsReservationOwner)
				require.Equal(t, int64(500), *result.PaidAmount)
				require.Equal(t, paymentModeDeposit, *result.PaymentMode)
			},
		},
		{
			name:  "ReservationAllowedMerchant",
			input: DiningSessionPrecheckInput{UserID: merchant.OwnerUserID, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), db.ListReservationsByTableAndDateParams{
						TableID:         table.ID,
						ReservationDate: reservationDate,
					}).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result DiningSessionPrecheckResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Reserved)
				require.NotNil(t, result.Reservation)
				require.Nil(t, result.Order)
				require.False(t, result.IsReservationOwner)
			},
		},
		{
			name:  "ReservationAllowedMerchantStaff",
			input: DiningSessionPrecheckInput{UserID: 120, TableID: table.ID, Now: now},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), db.ListReservationsByTableAndDateParams{
						TableID:         table.ID,
						ReservationDate: reservationDate,
					}).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{
						MerchantID: table.MerchantID,
						UserID:     int64(120),
					}).
					Times(1).
					Return(true, nil)
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result DiningSessionPrecheckResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Reserved)
				require.NotNil(t, result.Reservation)
				require.False(t, result.IsReservationOwner)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			result, err := PrecheckDiningSession(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
