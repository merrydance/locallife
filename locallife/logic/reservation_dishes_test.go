package logic

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func TestCreateReservationAddonPaymentOrder_SuccessRecordsAcceptedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createCombinePaymentResponse: &ospcontracts.CombinePrepayResponse{PrepayID: "addon-prepay-1"}}
	reservation := db.TableReservation{ID: 88, MerchantID: 501}
	paymentOrder := db.PaymentOrder{ID: 7101, Amount: 3600, OutTradeNo: "RA-7101", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	updatedPayment := paymentOrder
	updatedPayment.PrepayID = pgtype.Text{String: "addon-prepay-1", Valid: true}
	combinedPayment := db.CombinedPaymentOrder{ID: 9101, UserID: 1001, CombineOutTradeNo: "CPRA20260425120000", Status: paymentStatusPending}
	capturedCombineOutTradeNo := ""

	store.EXPECT().GetUser(gomock.Any(), int64(1001)).Return(db.User{ID: 1001, WechatOpenid: "openid-addon"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, reservation.MerchantID, arg.MerchantID)
		require.Equal(t, reservationAddonBusiness, arg.BusinessType)
		require.Equal(t, reservation.ID, arg.ReservationID)
		require.Equal(t, int64(3600), arg.Amount)
		require.NotEmpty(t, arg.CombineOutTradeNo)
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-ordinary-addon"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{ID: paymentOrder.ID, PrepayID: pgtype.Text{String: "addon-prepay-1", Valid: true}}).Return(updatedPayment, nil)
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{ID: combinedPayment.ID, PrepayID: pgtype.Text{String: "addon-prepay-1", Valid: true}}).Return(combinedPayment, nil)
	expectReservationAddonCombineCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, &capturedCombineOutTradeNo, "addon-prepay-1", db.ExternalPaymentCommandStatusAccepted, "", db.PaymentChannelOrdinaryServiceProvider, 9911)

	resultPayment, payParams, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1001, 3600, time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC), "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, updatedPayment.ID, resultPayment.ID)
	require.NotNil(t, payParams)
	require.Equal(t, "prepay_id=addon-prepay-1", payParams.Package)
	require.NotNil(t, ordinaryClient.createCombinePaymentRequest)
	require.Equal(t, capturedCombineOutTradeNo, ordinaryClient.createCombinePaymentRequest.CombineOutTradeNo)
	require.Len(t, ordinaryClient.createCombinePaymentRequest.SubOrders, 1)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createCombinePaymentRequest.SubOrders[0].OutTradeNo)
}

func TestCreateReservationAddonPaymentOrder_OrdinaryAcceptedRecordsOrdinaryCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	reservation := db.TableReservation{ID: 89, MerchantID: 502}
	paymentOrder := db.PaymentOrder{ID: 7111, Amount: 4200, OutTradeNo: "RA-7111", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	updatedPayment := paymentOrder
	updatedPayment.PrepayID = pgtype.Text{String: "prepay-combine-ordinary", Valid: true}
	combinedPayment := db.CombinedPaymentOrder{ID: 9111, UserID: 1002, CombineOutTradeNo: "CPRA20260425121000", Status: paymentStatusPending}
	capturedCombineOutTradeNo := ""

	store.EXPECT().GetUser(gomock.Any(), int64(1002)).Return(db.User{ID: 1002, WechatOpenid: "openid-ordinary-addon"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, reservation.MerchantID, arg.MerchantID)
		require.Equal(t, reservationAddonBusiness, arg.BusinessType)
		require.Equal(t, reservation.ID, arg.ReservationID)
		require.Equal(t, int64(4200), arg.Amount)
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		require.NotEmpty(t, arg.CombineOutTradeNo)
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-ordinary"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{ID: paymentOrder.ID, PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true}}).Return(updatedPayment, nil)
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{ID: combinedPayment.ID, PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true}}).Return(combinedPayment, nil)
	expectReservationAddonCombineCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, &capturedCombineOutTradeNo, "prepay-combine-ordinary", db.ExternalPaymentCommandStatusAccepted, "", db.PaymentChannelOrdinaryServiceProvider, 9914)

	resultPayment, payParams, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1002, 4200, time.Date(2026, 4, 25, 12, 10, 0, 0, time.UTC), "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, updatedPayment.ID, resultPayment.ID)
	require.NotNil(t, payParams)
	require.Equal(t, "prepay_id=prepay-combine-ordinary", payParams.Package)
	require.NotNil(t, ordinaryClient.createCombinePaymentRequest)
	require.Equal(t, ordinaryClient.ServiceProviderAppID(), ordinaryClient.createCombinePaymentRequest.CombineAppID)
	require.Equal(t, ordinaryClient.ServiceProviderMchID(), ordinaryClient.createCombinePaymentRequest.CombineMchID)
	require.Equal(t, capturedCombineOutTradeNo, ordinaryClient.createCombinePaymentRequest.CombineOutTradeNo)
	require.Equal(t, "openid-ordinary-addon", ordinaryClient.createCombinePaymentRequest.CombinePayerInfo.OpenID)
	require.Len(t, ordinaryClient.createCombinePaymentRequest.SubOrders, 1)
	require.Equal(t, ordinaryClient.ServiceProviderMchID(), ordinaryClient.createCombinePaymentRequest.SubOrders[0].MchID)
	require.Equal(t, "sub-ordinary", ordinaryClient.createCombinePaymentRequest.SubOrders[0].SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createCombinePaymentRequest.SubOrders[0].OutTradeNo)
	require.Equal(t, int64(4200), ordinaryClient.createCombinePaymentRequest.SubOrders[0].Amount.TotalAmount)
	require.Equal(t, ordinaryClient.CombineNotifyURL(), ordinaryClient.createCombinePaymentRequest.NotifyURL)
}

func TestCreateReservationAddonPaymentOrder_LogsCleanupFailuresAfterPrepayUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	reservation := db.TableReservation{ID: 91, MerchantID: 504}
	paymentOrder := db.PaymentOrder{ID: 7121, Amount: 4300, OutTradeNo: "RA-7121", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	combinedPayment := db.CombinedPaymentOrder{ID: 9121, UserID: 1004, CombineOutTradeNo: "CPRA20260425123000", Status: paymentStatusPending}

	store.EXPECT().GetUser(gomock.Any(), int64(1004)).Return(db.User{ID: 1004, WechatOpenid: "openid-cleanup"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-cleanup"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
	}).Return(db.PaymentOrder{}, errors.New("update prepay failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, errors.New("mark payment failed"))
	store.EXPECT().UpdateCombinedPaymentOrderToFailed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, errors.New("mark combined failed"))

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1004, 4300, time.Date(2026, 4, 25, 12, 30, 0, 0, time.UTC), "127.0.0.1")

	require.Error(t, err)
	require.Contains(t, err.Error(), "update prepay id")
	require.Contains(t, logs.String(), "failed to mark reservation addon payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark payment failed")
	require.Contains(t, logs.String(), "failed to mark reservation addon combined payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark combined failed")
	require.NotNil(t, ordinaryClient.closeCombinePaymentRequest)
}

func TestCreateReservationAddonPaymentOrder_CombinedPrepayUpdateErrorClosesRemoteAndFailsLocal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	reservation := db.TableReservation{ID: 92, MerchantID: 505}
	paymentOrder := db.PaymentOrder{ID: 7131, Amount: 4400, OutTradeNo: "RA-7131", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	updatedPayment := paymentOrder
	updatedPayment.PrepayID = pgtype.Text{String: "prepay-combine-ordinary", Valid: true}
	combinedPayment := db.CombinedPaymentOrder{ID: 9131, UserID: 1005, CombineOutTradeNo: "CPRA20260425124000", Status: paymentStatusPending}
	capturedCombineOutTradeNo := ""

	store.EXPECT().GetUser(gomock.Any(), int64(1005)).Return(db.User{ID: 1005, WechatOpenid: "openid-combined-cleanup"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-combined-cleanup"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
	}).Return(updatedPayment, nil)
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       combinedPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
	}).Return(db.CombinedPaymentOrder{}, errors.New("update combined prepay failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, nil)
	store.EXPECT().UpdateCombinedPaymentOrderToFailed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, nil)

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1005, 4400, time.Date(2026, 4, 25, 12, 40, 0, 0, time.UTC), "127.0.0.1")

	require.Error(t, err)
	require.Contains(t, err.Error(), "update combined payment prepay")
	require.NotNil(t, ordinaryClient.closeCombinePaymentRequest)
	require.Equal(t, capturedCombineOutTradeNo, ordinaryClient.closeCombinePaymentRequest.CombineOutTradeNo)
	require.Len(t, ordinaryClient.closeCombinePaymentRequest.SubOrders, 1)
	require.Equal(t, "sub-combined-cleanup", ordinaryClient.closeCombinePaymentRequest.SubOrders[0].SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.closeCombinePaymentRequest.SubOrders[0].OutTradeNo)
}

func TestCreateReservationAddonPaymentOrder_MissingOrdinaryClientDoesNotFallbackToEcommerce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	reservation := db.TableReservation{ID: 90, MerchantID: 503}

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, ecommerceClient, nil, reservation, 1003, 4500, time.Date(2026, 4, 25, 12, 20, 0, 0, time.UTC), "127.0.0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ordinary service provider client")
	require.Contains(t, err.Error(), "not configured")
}

func TestCreateReservationAddonPaymentOrder_CreateErrorRecordsRejectedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createCombinePaymentErr: errors.New("ordinary create combine failed")}
	reservation := db.TableReservation{ID: 88, MerchantID: 501}
	paymentOrder := db.PaymentOrder{ID: 7102, Amount: 3600, OutTradeNo: "RA-7102", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	combinedPayment := db.CombinedPaymentOrder{ID: 9102, UserID: 1001, CombineOutTradeNo: "CPRA20260425120100", Status: paymentStatusPending}
	capturedCombineOutTradeNo := ""

	store.EXPECT().GetUser(gomock.Any(), int64(1001)).Return(db.User{ID: 1001, WechatOpenid: "openid-addon"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-ordinary-addon"}, nil
	})
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, nil)
	store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, nil)
	expectReservationAddonCombineCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, &capturedCombineOutTradeNo, "", db.ExternalPaymentCommandStatusRejected, "", db.PaymentChannelOrdinaryServiceProvider, 9912)

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1001, 3600, time.Date(2026, 4, 25, 12, 1, 0, 0, time.UTC), "127.0.0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "create combine order")
}

func TestCreateReservationAddonPaymentOrder_EmptyPrepayRecordsRejectedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createCombinePaymentResponse: &ospcontracts.CombinePrepayResponse{}}
	reservation := db.TableReservation{ID: 88, MerchantID: 501}
	paymentOrder := db.PaymentOrder{ID: 7103, Amount: 3600, OutTradeNo: "RA-7103", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	combinedPayment := db.CombinedPaymentOrder{ID: 9103, UserID: 1001, CombineOutTradeNo: "CPRA20260425120200", Status: paymentStatusPending}
	capturedCombineOutTradeNo := ""

	store.EXPECT().GetUser(gomock.Any(), int64(1001)).Return(db.User{ID: 1001, WechatOpenid: "openid-addon"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateEcommercePaymentTxParams) (db.CreateEcommercePaymentTxResult, error) {
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.PaymentChannel)
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		combinedPayment.CombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-ordinary-addon"}, nil
	})
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, nil)
	store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, nil)
	expectReservationAddonCombineCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, &capturedCombineOutTradeNo, "", db.ExternalPaymentCommandStatusRejected, "", db.PaymentChannelOrdinaryServiceProvider, 9913)

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1001, 3600, time.Date(2026, 4, 25, 12, 2, 0, 0, time.UTC), "127.0.0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty prepay id")
}

func TestCreateReservationAddonPaymentOrder_CreateRejectedSkipsCommandWhenCloseFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createCombinePaymentErr: errors.New("ordinary create combine failed")}
	reservation := db.TableReservation{ID: 88, MerchantID: 501}
	paymentOrder := db.PaymentOrder{ID: 7104, Amount: 3600, OutTradeNo: "RA-7104", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider}
	combinedPayment := db.CombinedPaymentOrder{ID: 9104, UserID: 1001, CombineOutTradeNo: "CPRA20260425120300", Status: paymentStatusPending}

	store.EXPECT().GetUser(gomock.Any(), int64(1001)).Return(db.User{ID: 1001, WechatOpenid: "openid-addon"}, nil)
	store.EXPECT().CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).Return(db.CreateEcommercePaymentTxResult{PaymentOrder: paymentOrder, CombinedPaymentOrder: combinedPayment, SubMchID: "sub-ordinary-addon"}, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, errors.New("close payment failed"))
	store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, nil)

	_, _, err := createReservationAddonPaymentOrder(context.Background(), store, nil, ordinaryClient, reservation, 1001, 3600, time.Date(2026, 4, 25, 12, 3, 0, 0, time.UTC), "127.0.0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "create combine order")
}

func expectReservationAddonCombineCommand(t *testing.T, store *mockdb.MockStore, paymentOrderID int64, outTradeNo string, combineOutTradeNo *string, secondaryKey string, status string, errorCode string, expectedChannel string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, expectedChannel, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, paymentOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectCombinedPayment, arg.ExternalObjectType)
		require.NotNil(t, combineOutTradeNo)
		require.NotEmpty(t, *combineOutTradeNo)
		require.Equal(t, *combineOutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), *combineOutTradeNo)
		require.Contains(t, string(arg.ResponseSnapshot), outTradeNo)
		if secondaryKey != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
			require.Contains(t, string(arg.ResponseSnapshot), secondaryKey)
		} else {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		}
		if errorCode != "" {
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), errorCode)
		} else {
			require.False(t, arg.LastErrorCode.Valid)
		}
		if status == db.ExternalPaymentCommandStatusRejected {
			require.True(t, arg.LastErrorMessage.Valid)
			require.NotEmpty(t, arg.LastErrorMessage.String)
			require.Contains(t, string(arg.ResponseSnapshot), arg.LastErrorMessage.String)
		} else {
			require.False(t, arg.LastErrorMessage.Valid)
		}
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}
