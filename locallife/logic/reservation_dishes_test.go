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

func TestBuildReservationRefundAllocations_SplitsAcrossReservationPayments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reservationID := int64(88)
	basePayment := db.PaymentOrder{ID: 1, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, BusinessType: businessTypeReservation, Status: paymentStatusPaid, Amount: 1000}
	addonPaymentA := db.PaymentOrder{ID: 2, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, BusinessType: reservationAddonBusiness, Status: paymentStatusPaid, Amount: 500}
	addonPaymentB := db.PaymentOrder{ID: 3, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, BusinessType: reservationAddonBusiness, Status: paymentStatusPaid, Amount: 300}

	store.EXPECT().
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Return([]db.PaymentOrder{addonPaymentB, addonPaymentA, basePayment}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), addonPaymentB.ID).Return(int64(250), nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), addonPaymentA.ID).Return(int64(0), nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), basePayment.ID).Return(int64(100), nil)

	allocations, err := buildReservationRefundAllocations(context.Background(), store, reservationID, 800)
	require.NoError(t, err)
	require.Len(t, allocations, 3)
	require.Equal(t, addonPaymentB.ID, allocations[0].PaymentOrder.ID)
	require.Equal(t, int64(50), allocations[0].RefundAmount)
	require.Equal(t, addonPaymentA.ID, allocations[1].PaymentOrder.ID)
	require.Equal(t, int64(500), allocations[1].RefundAmount)
	require.Equal(t, basePayment.ID, allocations[2].PaymentOrder.ID)
	require.Equal(t, int64(250), allocations[2].RefundAmount)
	require.Equal(t, int64(800), sumReservationRefundAllocations(allocations))
}

func TestBuildReservationRefundAllocations_IncludesReservationLinkedOrderPayments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reservationID := int64(108)
	replacementPayment := db.PaymentOrder{ID: 11, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, BusinessType: businessTypeOrder, Status: paymentStatusPaid, Amount: 500}
	basePayment := db.PaymentOrder{ID: 12, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, BusinessType: businessTypeReservation, Status: paymentStatusPaid, Amount: 1000}

	store.EXPECT().
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Return([]db.PaymentOrder{replacementPayment, basePayment}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), replacementPayment.ID).Return(int64(0), nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), basePayment.ID).Return(int64(0), nil)

	allocations, err := buildReservationRefundAllocations(context.Background(), store, reservationID, 600)
	require.NoError(t, err)
	require.Len(t, allocations, 2)
	require.Equal(t, replacementPayment.ID, allocations[0].PaymentOrder.ID)
	require.Equal(t, int64(500), allocations[0].RefundAmount)
	require.Equal(t, basePayment.ID, allocations[1].PaymentOrder.ID)
	require.Equal(t, int64(100), allocations[1].RefundAmount)
}
