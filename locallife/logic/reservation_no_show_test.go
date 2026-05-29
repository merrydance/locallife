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

func TestMarkReservationNoShowRejectsActiveAdjustment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(13)
	reservationID := int64(23)
	merchantID := int64(33)
	reservation := db.TableReservation{
		ID:         reservationID,
		UserID:     userID + 1,
		MerchantID: merchantID,
		Status:     reservationStatusConfirmed,
	}
	merchant := db.Merchant{ID: merchantID, OwnerUserID: userID}
	adjustment := db.ReservationAdjustment{
		ID:             913,
		ReservationID:  reservationID,
		Status:         db.ReservationAdjustmentStatusPendingPayment,
		PaymentOrderID: pgtype.Int8{Int64: 1013, Valid: true},
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), userID).Return(merchant, nil)
	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveReservationAdjustmentByReservation(gomock.Any(), reservationID).Return(adjustment, nil)

	_, err := MarkReservationNoShow(context.Background(), store, userID, reservationID)

	require.Error(t, err)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, 409, requestErr.Status)
	require.Contains(t, requestErr.Error(), "待支付改菜补差单")
}
