package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reservationDishesTaskSchedulerStub struct {
	refundInputs []ProcessRefundTaskInput
}

func (s *reservationDishesTaskSchedulerStub) ScheduleOrderPaymentTimeout(context.Context, int64, time.Time) error {
	return nil
}

func (s *reservationDishesTaskSchedulerStub) SchedulePaymentOrderTimeout(context.Context, string, time.Time) error {
	return nil
}

func (s *reservationDishesTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(context.Context, string, time.Time) error {
	return nil
}

func (s *reservationDishesTaskSchedulerStub) ScheduleProcessRefund(_ context.Context, input ProcessRefundTaskInput) error {
	s.refundInputs = append(s.refundInputs, input)
	return nil
}

func (s *reservationDishesTaskSchedulerStub) ScheduleProfitSharing(context.Context, int64) error {
	return nil
}

func (s *reservationDishesTaskSchedulerStub) ScheduleProfitSharingReturnResult(context.Context, ProfitSharingReturnResultTaskInput) error {
	return nil
}

func (s *reservationDishesTaskSchedulerStub) ScheduleOrderPrint(context.Context, OrderPrintTaskInput) error {
	return nil
}

type reservationDishesPaymentFacade struct {
	createPaymentCalled bool
	lastCreatePayment   CreatePaymentOrderInput
	paymentResult       CreatePaymentOrderResult
	paymentErr          error
}

func (f *reservationDishesPaymentFacade) CreatePaymentOrder(_ context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	f.createPaymentCalled = true
	f.lastCreatePayment = input
	return f.paymentResult, f.paymentErr
}

func (f *reservationDishesPaymentFacade) CreateReservationAdjustmentPaymentOrder(_ context.Context, input CreateReservationAdjustmentPaymentInput) (CreatePaymentOrderResult, error) {
	f.createPaymentCalled = true
	f.lastCreatePayment = CreatePaymentOrderInput{
		UserID:       input.UserID,
		OrderID:      input.ReservationID,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: reservationAddonBusiness,
		ClientIP:     input.ClientIP,
		Amount:       input.DeltaAmount,
	}
	return f.paymentResult, f.paymentErr
}

func (f *reservationDishesPaymentFacade) CreateCombinedPaymentOrder(context.Context, CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	return CreateCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) GetCombinedPaymentOrder(context.Context, GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error) {
	return GetCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) QueryCombinedPaymentOrder(context.Context, QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	return QueryCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) CloseCombinedPaymentOrder(context.Context, CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	return CloseCombinedPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) GetPaymentOrder(context.Context, GetPaymentOrderInput) (GetPaymentOrderResult, error) {
	return GetPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) QueryPaymentOrder(context.Context, QueryPaymentOrderInput) (QueryPaymentOrderResult, error) {
	return QueryPaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) ListPaymentOrders(context.Context, ListPaymentOrdersInput) (ListPaymentOrdersResult, error) {
	return ListPaymentOrdersResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) ListPaymentLedger(context.Context, ListPaymentLedgerInput) (ListPaymentLedgerResult, error) {
	return ListPaymentLedgerResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) ClosePaymentOrder(context.Context, ClosePaymentOrderInput) (ClosePaymentOrderResult, error) {
	return ClosePaymentOrderResult{}, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) CreateRefund(context.Context, *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) CreateBaofuRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented")
}

func (f *reservationDishesPaymentFacade) BaofuRefundNotifyURL() string {
	return ""
}

func expectNoActiveReservationAdjustment(store *mockdb.MockStore, reservationID int64) *gomock.Call {
	return store.EXPECT().
		GetActiveReservationAdjustmentByReservation(gomock.Any(), reservationID).
		Return(db.ReservationAdjustment{}, db.ErrRecordNotFound)
}

func TestModifyReservationDishesPositiveDeltaCreatesPendingAdjustmentWithoutReplacingItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(10)
	merchantID := int64(20)
	reservationID := int64(30)
	dishID := int64(40)
	paymentOrder := db.PaymentOrder{ID: 501, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, Amount: 2000, Status: paymentStatusPending}
	facade := &reservationDishesPaymentFacade{
		paymentResult: CreatePaymentOrderResult{
			PaymentOrder: paymentOrder,
			PayParams:    &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "nonce", Package: "prepay_id=abc", SignType: "RSA", PaySign: "sign"},
		},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		Status:      reservationStatusPaid,
		PaymentMode: paymentModeFull,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	expectNoActiveReservationAdjustment(store, reservationID)
	store.EXPECT().SumReservationItemsTotal(gomock.Any(), reservationID).Return(int64(3000), nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Price: 5000, IsOnline: true}, nil)

	result, err := ModifyReservationDishes(context.Background(), store, ModifyReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
		PaymentFacade: facade,
		ClientIP:      "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, int64(2000), result.Delta)
	require.NotNil(t, result.Payment)
	require.True(t, facade.createPaymentCalled)
}

func TestAddReservationDishesPositiveDeltaCreatesPendingAdjustmentWithoutAppendingItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(11)
	merchantID := int64(21)
	reservationID := int64(31)
	dishID := int64(41)
	paymentOrder := db.PaymentOrder{ID: 502, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, Amount: 1800, Status: paymentStatusPending}
	facade := &reservationDishesPaymentFacade{
		paymentResult: CreatePaymentOrderResult{
			PaymentOrder: paymentOrder,
			PayParams:    &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "nonce", Package: "prepay_id=abc", SignType: "RSA", PaySign: "sign"},
		},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		Status:      reservationStatusPaid,
		PaymentMode: paymentModeFull,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	expectNoActiveReservationAdjustment(store, reservationID)
	store.EXPECT().SumReservationItemsTotal(gomock.Any(), reservationID).Return(int64(0), nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Price: 1800, IsOnline: true}, nil)
	store.EXPECT().GetReservationItemsByReservation(gomock.Any(), reservationID).Return([]db.ReservationItem{}, nil)

	result, err := AddReservationDishes(context.Background(), store, AddReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
		PaymentFacade: facade,
		ClientIP:      "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1800), result.AddedAmount)
	require.NotNil(t, result.Payment)
	require.True(t, facade.createPaymentCalled)
}

func TestModifyReservationDishesNegativeDeltaUsesCombinedRefundTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &reservationDishesTaskSchedulerStub{}

	userID := int64(10)
	merchantID := int64(20)
	reservationID := int64(30)
	dishID := int64(40)
	paymentOrder := db.PaymentOrder{
		ID:                    501,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                5000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	reservation := db.TableReservation{
		ID:            reservationID,
		UserID:        userID,
		MerchantID:    merchantID,
		Status:        reservationStatusPaid,
		PaymentMode:   paymentModeFull,
		PrepaidAmount: 5000,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	expectNoActiveReservationAdjustment(store, reservationID)
	store.EXPECT().SumReservationItemsTotal(gomock.Any(), reservationID).Return(int64(5000), nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Price: 3000, IsAvailable: true, IsOnline: true}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)
	store.EXPECT().ReplaceReservationItemsWithRefundOrdersTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ReplaceReservationItemsWithRefundOrdersTxParams) (db.ReplaceReservationItemsWithRefundOrdersTxResult, error) {
			require.Equal(t, reservationID, arg.ReservationID)
			require.Equal(t, int64(5000), arg.ExpectedCurrentAmount)
			require.Len(t, arg.Items, 1)
			require.Equal(t, dishID, arg.Items[0].DishID.Int64)
			require.Equal(t, int64(3000), arg.Items[0].TotalPrice)
			require.Len(t, arg.RefundOrders, 1)
			require.Equal(t, paymentOrder.ID, arg.RefundOrders[0].PaymentOrderID)
			require.Equal(t, int64(2000), arg.RefundOrders[0].RefundAmount)
			require.NotEmpty(t, arg.RefundOrders[0].OutRefundNo)
			refundOrder := db.RefundOrder{ID: 601, PaymentOrderID: paymentOrder.ID, RefundAmount: 2000, OutRefundNo: arg.RefundOrders[0].OutRefundNo}
			return db.ReplaceReservationItemsWithRefundOrdersTxResult{RefundOrders: []db.RefundOrder{refundOrder}}, nil
		})

	result, err := ModifyReservationDishes(context.Background(), store, ModifyReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
		TaskScheduler: scheduler,
	})

	require.NoError(t, err)
	require.Equal(t, int64(-2000), result.Delta)
	require.Equal(t, int64(2000), result.RefundAmount)
	require.True(t, result.RefundInitiated)
	require.Len(t, scheduler.refundInputs, 1)
	require.Equal(t, paymentOrder.ID, scheduler.refundInputs[0].PaymentOrderID)
	require.Equal(t, reservationID, scheduler.refundInputs[0].ReservationID)
	require.Equal(t, int64(2000), scheduler.refundInputs[0].RefundAmount)
}

func TestModifyReservationDishesNegativeDeltaRejectsWhenRefundAllocationIsIncomplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	userID := int64(11)
	merchantID := int64(21)
	reservationID := int64(31)
	dishID := int64(41)
	paymentOrder := db.PaymentOrder{
		ID:                    502,
		ReservationID:         pgtype.Int8{Int64: reservationID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                5000,
		Status:                paymentStatusPaid,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	reservation := db.TableReservation{
		ID:            reservationID,
		UserID:        userID,
		MerchantID:    merchantID,
		Status:        reservationStatusPaid,
		PaymentMode:   paymentModeFull,
		PrepaidAmount: 5000,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	expectNoActiveReservationAdjustment(store, reservationID)
	store.EXPECT().SumReservationItemsTotal(gomock.Any(), reservationID).Return(int64(5000), nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Price: 3000, IsAvailable: true, IsOnline: true}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(4000), nil)

	_, err := ModifyReservationDishes(context.Background(), store, ModifyReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	require.Error(t, err)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, 409, requestErr.Status)
	require.Contains(t, requestErr.Error(), "预订退款资金链路已变化")
}

func TestModifyReservationDishesRejectsActiveAdjustment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(10)
	merchantID := int64(20)
	reservationID := int64(30)
	dishID := int64(40)
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		Status:      reservationStatusPaid,
		PaymentMode: paymentModeFull,
	}
	adjustment := db.ReservationAdjustment{
		ID:             701,
		ReservationID:  reservationID,
		Status:         db.ReservationAdjustmentStatusPendingPayment,
		PaymentOrderID: pgtype.Int8{Int64: 801, Valid: true},
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveReservationAdjustmentByReservation(gomock.Any(), reservationID).Return(adjustment, nil)

	_, err := ModifyReservationDishes(context.Background(), store, ModifyReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	require.Error(t, err)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, 409, requestErr.Status)
	require.Contains(t, requestErr.Error(), "待支付改菜补差单")
}

func TestAddReservationDishesRejectsActiveAdjustment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(11)
	merchantID := int64(21)
	reservationID := int64(31)
	dishID := int64(41)
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		Status:      reservationStatusPaid,
		PaymentMode: paymentModeFull,
	}
	adjustment := db.ReservationAdjustment{
		ID:            702,
		ReservationID: reservationID,
		Status:        db.ReservationAdjustmentStatusCreatingPayment,
	}

	store.EXPECT().GetTableReservationForUpdate(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveReservationAdjustmentByReservation(gomock.Any(), reservationID).Return(adjustment, nil)

	_, err := AddReservationDishes(context.Background(), store, AddReservationDishesInput{
		ReservationID: reservationID,
		UserID:        userID,
		Items: []ReservationItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	require.Error(t, err)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, 409, requestErr.Status)
	require.Contains(t, requestErr.Error(), "进行中的改菜补差单")
}
