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

func TestCancelReservation_BaofuRefundCreatesPendingRefundOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	userID := int64(10)
	reservationID := int64(20)
	merchantID := int64(30)
	tableID := int64(40)
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	reservation := db.TableReservation{
		ID:             reservationID,
		UserID:         userID,
		MerchantID:     merchantID,
		TableID:        tableID,
		Status:         reservationStatusPaid,
		RefundDeadline: now.Add(time.Hour),
	}
	cancelledReservation := reservation
	cancelledReservation.Status = reservationStatusCancelled
	paymentOrder := db.PaymentOrder{
		ID:             501,
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:   businessTypeReservation,
		Status:         paymentStatusPaid,
		OutTradeNo:     "reservation_paid_001",
		Amount:         2000,
		PaymentType:    paymentTypeProfitSharing,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetLatestPaymentOrderByReservation(gomock.Any(), db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:  businessTypeReservation,
	}).Return(paymentOrder, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
		require.Equal(t, paymentOrder.ID, arg.PaymentOrderID)
		require.Equal(t, paymentTypeProfitSharing, arg.RefundType)
		require.Equal(t, int64(2000), arg.RefundAmount)
		require.Equal(t, "预定取消退款", arg.RefundReason)
		require.NotEmpty(t, arg.OutRefundNo)
		return db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 601, OutRefundNo: arg.OutRefundNo}}, nil
	})
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID, CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}, nil)
	store.EXPECT().CancelReservationTx(gomock.Any(), db.CancelReservationTxParams{
		ReservationID:        reservationID,
		TableID:              tableID,
		CancelReason:         "change of plan",
		CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		ReleaseInventory:     true,
	}).Return(db.CancelReservationTxResult{Reservation: cancelledReservation}, nil)

	result, err := CancelReservation(
		context.Background(),
		store,
		userID,
		reservationID,
		"change of plan",
		ReservationRefundPolicy{UserBeforeDeadlinePercent: 100},
		now,
	)
	require.NoError(t, err)
	require.Equal(t, reservationStatusCancelled, result.Reservation.Status)
	require.True(t, result.RefundEligible)
}
