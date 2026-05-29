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

func TestStartCookingReservationRejectsActiveAdjustment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(12)
	reservationID := int64(22)
	merchantID := int64(32)
	reservation := db.TableReservation{
		ID:         reservationID,
		UserID:     userID,
		MerchantID: merchantID,
		Status:     reservationStatusConfirmed,
	}
	adjustment := db.ReservationAdjustment{
		ID:             912,
		ReservationID:  reservationID,
		Status:         db.ReservationAdjustmentStatusPendingPayment,
		PaymentOrderID: pgtype.Int8{Int64: 1012, Valid: true},
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveReservationAdjustmentByReservation(gomock.Any(), reservationID).Return(adjustment, nil)

	_, err := StartCookingReservation(context.Background(), store, userID, reservationID)

	require.Error(t, err)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, 409, requestErr.Status)
	require.Contains(t, requestErr.Error(), "待支付改菜补差单")
}
