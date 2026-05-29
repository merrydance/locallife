package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reservationCompleteTaskSchedulerStub struct {
	profitSharingOrderIDs []int64
	err                   error
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleOrderPaymentTimeout(context.Context, int64, time.Time) error {
	return nil
}

func (s *reservationCompleteTaskSchedulerStub) SchedulePaymentOrderTimeout(context.Context, string, time.Time) error {
	return nil
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(context.Context, string, time.Time) error {
	return nil
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleProcessRefund(context.Context, ProcessRefundTaskInput) error {
	return nil
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleProfitSharing(_ context.Context, profitSharingOrderID int64) error {
	s.profitSharingOrderIDs = append(s.profitSharingOrderIDs, profitSharingOrderID)
	return s.err
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleProfitSharingReturnResult(context.Context, ProfitSharingReturnResultTaskInput) error {
	return nil
}

func (s *reservationCompleteTaskSchedulerStub) ScheduleOrderPrint(context.Context, OrderPrintTaskInput) error {
	return nil
}

func TestCompleteReservationSchedulesBaofuProfitSharingForCompletedReservationPayments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &reservationCompleteTaskSchedulerStub{}

	userID := int64(10)
	reservationID := int64(20)
	merchantID := int64(30)
	tableID := int64(40)
	operatorID := int64(50)
	reservation := db.TableReservation{
		ID:         reservationID,
		UserID:     userID + 1,
		MerchantID: merchantID,
		TableID:    tableID,
		Status:     reservationStatusCheckedIn,
	}
	completedReservation := reservation
	completedReservation.Status = reservationStatusCompleted
	completedReservation.CompletedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	merchant := db.Merchant{ID: merchantID, OwnerUserID: userID, RegionID: 60}
	operator := db.Operator{ID: operatorID, RegionID: merchant.RegionID}
	profitSharingConfig := db.ProfitSharingConfig{PlatformRate: 4, OperatorRate: 1}
	primaryPayment := db.PaymentOrder{
		ID:                    501,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	addonPayment := db.PaymentOrder{
		ID:                    502,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          reservationAddonBusiness,
		Amount:                3000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	wechatPayment := db.PaymentOrder{
		ID:                    503,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                2000,
		Status:                paymentStatusPaid,
		PaymentChannel:        "wechat",
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), userID).Return(merchant, nil)
	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID, CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}, nil)
	store.EXPECT().CompleteReservationTx(gomock.Any(), db.CompleteReservationTxParams{
		ReservationID:        reservationID,
		TableID:              tableID,
		CurrentReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}).Return(db.CompleteReservationTxResult{Reservation: completedReservation, TableUpdated: true}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{
		primaryPayment,
		addonPayment,
		wechatPayment,
	}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeReservation,
		MerchantID:  pgtype.Int8{Int64: merchantID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(profitSharingConfig, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchantID, "MER_SHARE").Times(2)
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeOperator, operatorID, "OP_SHARE").Times(2)
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE").Times(2)
	store.EXPECT().EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, db.OrderTypeReservation, arg.ProfitSharingOrder.OrderSource)
			require.Equal(t, merchantID, arg.ProfitSharingOrder.MerchantID)
			require.Equal(t, operatorID, arg.ProfitSharingOrder.OperatorID.Int64)
			require.Equal(t, int32(400), arg.ProfitSharingOrder.PlatformRate)
			require.Equal(t, int32(100), arg.ProfitSharingOrder.OperatorRate)
			require.Contains(t, []int64{primaryPayment.ID, addonPayment.ID}, arg.ProfitSharingOrder.PaymentOrderID)
			return db.CreateBaofuProfitSharingOrderTxResult{
				ProfitSharingOrder: db.ProfitSharingOrder{ID: arg.ProfitSharingOrder.PaymentOrderID + 1000, PaymentOrderID: arg.ProfitSharingOrder.PaymentOrderID},
			}, nil
		}).Times(2)

	result, err := CompleteReservation(context.Background(), store, scheduler, userID, reservationID)

	require.NoError(t, err)
	require.Equal(t, reservationStatusCompleted, result.Reservation.Status)
	require.Equal(t, []int64{1501, 1502}, scheduler.profitSharingOrderIDs)
}

func TestCompleteReservationReturnsSuccessWhenProfitSharingScheduleFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &reservationCompleteTaskSchedulerStub{err: errors.New("queue unavailable")}

	userID := int64(11)
	reservationID := int64(21)
	merchantID := int64(31)
	tableID := int64(41)
	reservation := db.TableReservation{
		ID:         reservationID,
		UserID:     userID + 1,
		MerchantID: merchantID,
		TableID:    tableID,
		Status:     reservationStatusConfirmed,
	}
	completedReservation := reservation
	completedReservation.Status = reservationStatusCompleted
	completedReservation.CompletedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	merchant := db.Merchant{ID: merchantID, OwnerUserID: userID}
	profitSharingConfig := db.ProfitSharingConfig{PlatformRate: 4, OperatorRate: 0}
	payment := db.PaymentOrder{
		ID:                    601,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), userID).Return(merchant, nil)
	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID}, nil)
	store.EXPECT().CompleteReservationTx(gomock.Any(), gomock.Any()).Return(db.CompleteReservationTxResult{Reservation: completedReservation}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{payment}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeReservation,
		MerchantID:  pgtype.Int8{Int64: merchantID, Valid: true},
		RegionID:    pgtype.Int8{},
	}).Return(profitSharingConfig, nil)
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchantID, "MER_SHARE")
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE")
	store.EXPECT().EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).Return(db.CreateBaofuProfitSharingOrderTxResult{
		ProfitSharingOrder: db.ProfitSharingOrder{ID: 1601, PaymentOrderID: payment.ID},
	}, nil)

	result, err := CompleteReservation(context.Background(), store, scheduler, userID, reservationID)

	require.NoError(t, err)
	require.Equal(t, reservationStatusCompleted, result.Reservation.Status)
	require.Equal(t, []int64{1601}, scheduler.profitSharingOrderIDs)
}

func TestCompleteReservationCreatesBaofuProfitSharingBillWithoutScheduler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	userID := int64(12)
	reservationID := int64(22)
	merchantID := int64(32)
	tableID := int64(42)
	reservation := db.TableReservation{
		ID:         reservationID,
		UserID:     userID + 1,
		MerchantID: merchantID,
		TableID:    tableID,
		Status:     reservationStatusCheckedIn,
	}
	completedReservation := reservation
	completedReservation.Status = reservationStatusCompleted
	completedReservation.CompletedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	merchant := db.Merchant{ID: merchantID, OwnerUserID: userID}
	profitSharingConfig := db.ProfitSharingConfig{PlatformRate: 4, OperatorRate: 0}
	payment := db.PaymentOrder{
		ID:                    701,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchantByOwner(gomock.Any(), userID).Return(merchant, nil)
	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetTable(gomock.Any(), tableID).Return(db.Table{ID: tableID}, nil)
	store.EXPECT().CompleteReservationTx(gomock.Any(), gomock.Any()).Return(db.CompleteReservationTxResult{Reservation: completedReservation}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{payment}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeReservation,
		MerchantID:  pgtype.Int8{Int64: merchantID, Valid: true},
		RegionID:    pgtype.Int8{},
	}).Return(profitSharingConfig, nil)
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchantID, "MER_SHARE")
	expectLogicBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE")
	store.EXPECT().EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, payment.ID, arg.ProfitSharingOrder.PaymentOrderID)
			require.Equal(t, merchantID, arg.ProfitSharingOrder.MerchantID)
			require.Equal(t, db.OrderTypeReservation, arg.ProfitSharingOrder.OrderSource)
			require.Equal(t, int32(400), arg.ProfitSharingOrder.PlatformRate)
			require.Equal(t, int32(0), arg.ProfitSharingOrder.OperatorRate)
			return db.CreateBaofuProfitSharingOrderTxResult{
				ProfitSharingOrder: db.ProfitSharingOrder{ID: 1701, PaymentOrderID: payment.ID},
			}, nil
		})

	result, err := CompleteReservation(context.Background(), store, nil, userID, reservationID)

	require.NoError(t, err)
	require.Equal(t, reservationStatusCompleted, result.Reservation.Status)
}

func expectLogicBaofuReceiverLookup(store *mockdb.MockStore, ownerType string, ownerID int64, sharingMerID string) *gomock.Call {
	return store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: ownerType, OwnerID: ownerID}).
		Return(activeBaofuReceiverBinding(ownerType, ownerID, ownerType+"_CONTRACT", sharingMerID), nil)
}
