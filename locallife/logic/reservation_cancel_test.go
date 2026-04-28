package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCancelReservation_EcommerceRefundAcceptedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

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
		PaymentChannel: db.PaymentChannelEcommerce,
	}
	capturedOutRefundNo := ""

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID, CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}, nil)
	store.EXPECT().CancelReservationTx(gomock.Any(), db.CancelReservationTxParams{
		ReservationID:        reservationID,
		TableID:              tableID,
		CancelReason:         "change of plan",
		CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		ReleaseInventory:     true,
	}).Return(db.CancelReservationTxResult{Reservation: cancelledReservation}, nil)
	store.EXPECT().GetLatestPaymentOrderByReservation(gomock.Any(), db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:  businessTypeReservation,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchantID).Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
		require.Equal(t, paymentOrder.ID, arg.PaymentOrderID)
		require.Equal(t, paymentTypeProfitSharing, arg.RefundType)
		require.Equal(t, int64(2000), arg.RefundAmount)
		require.Equal(t, "预定取消退款", arg.RefundReason)
		require.NotEmpty(t, arg.OutRefundNo)
		capturedOutRefundNo = arg.OutRefundNo
		return db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 601, OutRefundNo: arg.OutRefundNo}}, nil
	})
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
		require.Equal(t, "1900000109", req.SubMchID)
		require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
		require.Equal(t, capturedOutRefundNo, req.OutRefundNo)
		require.Equal(t, int64(2000), req.RefundAmount)
		return &wechat.EcommerceRefundResponse{RefundID: "erefund_cancel_1"}, nil
	})
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       601,
		RefundID: pgtype.Text{String: "erefund_cancel_1", Valid: true},
	}).Return(db.RefundOrder{ID: 601, Status: "processing"}, nil)
	expectCancelReservationRefundAcceptedCommand(t, store, 601, &capturedOutRefundNo, "erefund_cancel_1", 9401)

	result, err := CancelReservation(
		context.Background(),
		store,
		ecommerceClient,
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

func TestCancelReservation_EcommerceRefundAPIFailureKeepsPendingWithoutCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	userID := int64(11)
	reservationID := int64(21)
	merchantID := int64(31)
	tableID := int64(41)
	now := time.Date(2026, 4, 25, 12, 30, 0, 0, time.UTC)
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
		ID:             502,
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:   businessTypeReservation,
		Status:         paymentStatusPaid,
		OutTradeNo:     "reservation_paid_002",
		Amount:         2000,
		PaymentType:    paymentTypeProfitSharing,
		PaymentChannel: db.PaymentChannelEcommerce,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID}, nil)
	store.EXPECT().CancelReservationTx(gomock.Any(), gomock.Any()).Return(db.CancelReservationTxResult{Reservation: cancelledReservation}, nil)
	store.EXPECT().GetLatestPaymentOrderByReservation(gomock.Any(), gomock.Any()).Return(paymentOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchantID).Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"}, nil)
	store.EXPECT().CreateRefundOrderTx(gomock.Any(), gomock.Any()).Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 602, OutRefundNo: "cancel_refund_pending"}}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, errors.New("ecommerce api down"))

	result, err := CancelReservation(
		context.Background(),
		store,
		ecommerceClient,
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

func expectCancelReservationRefundAcceptedCommand(t *testing.T, store *mockdb.MockStore, refundOrderID int64, outRefundNo *string, refundID string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityEcommerceRefund, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "refund_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, refundOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
		require.NotNil(t, outRefundNo)
		require.NotEmpty(t, *outRefundNo)
		require.Equal(t, *outRefundNo, arg.ExternalObjectKey)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), *outRefundNo)
		require.Contains(t, string(arg.ResponseSnapshot), refundID)
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}
