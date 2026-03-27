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

func TestConfirmReservation(t *testing.T) {
	userID := int64(10)
	merchantID := int64(20)
	reservationID := int64(30)
	tableID := int64(40)

	merchant := db.Merchant{ID: merchantID, OwnerUserID: userID}
	reservation := db.TableReservation{ID: reservationID, MerchantID: merchantID, TableID: tableID, Status: reservationStatusPaid}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ReservationStatusUpdateResult, err error)
	}{
		{
			name: "Suspended",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), userID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(db.GetMerchantProfileRow{
						MerchantID:           merchantID,
						IsTakeoutSuspended:   true,
						TakeoutSuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
					}, nil)
			},
			check: func(t *testing.T, _ ReservationStatusUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "商户预订接单已暂停", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				confirmedReservation := reservation
				confirmedReservation.Status = reservationStatusConfirmed
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), userID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchantID, IsTakeoutSuspended: false}, nil)
				store.EXPECT().
					ConfirmReservationTx(gomock.Any(), db.ConfirmReservationTxParams{ReservationID: reservationID, TableID: tableID}).
					Times(1).
					Return(db.ConfirmReservationTxResult{Reservation: confirmedReservation}, nil)
			},
			check: func(t *testing.T, result ReservationStatusUpdateResult, err error) {
				require.NoError(t, err)
				require.Equal(t, reservationStatusConfirmed, result.Reservation.Status)
				require.Equal(t, reservationStatusPaid, result.PreviousStatus)
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

			result, err := ConfirmReservation(context.Background(), store, userID, reservationID)
			tc.check(t, result, err)
		})
	}
}
