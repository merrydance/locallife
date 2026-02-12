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

func TestCancelOrder_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), int64(10)).
		Times(1).
		Return(db.Order{}, db.ErrRecordNotFound)

	_, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: 1, OrderID: 10})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "order not found", reqErr.Err.Error())
}

func TestCancelOrder_NotBelong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), int64(10)).
		Times(1).
		Return(db.Order{ID: 10, UserID: 2}, nil)

	_, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: 1, OrderID: 10})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "order does not belong to you", reqErr.Err.Error())
}

func TestCancelOrder_LateStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 11, UserID: 3, Status: "preparing"}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		UpdateOrderExceptionState(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Order{}, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)

	_, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: order.UserID, OrderID: order.ID})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "订单已制作/配送，已记录取消诉求，请联系商户或客服处理", reqErr.Err.Error())
}

func TestCancelOrder_InvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 12, UserID: 4, Status: "cancelled"}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)

	_, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: order.UserID, OrderID: order.ID})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "订单当前状态无法取消，商户已接单后请联系商户处理", reqErr.Err.Error())
}

func TestCancelOrder_PendingNoRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 13, UserID: 5, Status: "pending"}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		CancelOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CancelOrderTxParams) (db.CancelOrderTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, "pending", arg.OldStatus)
			return db.CancelOrderTxResult{Order: db.Order{ID: order.ID, Status: "cancelled"}}, nil
		})

	result, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: order.UserID, OrderID: order.ID})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Order.Status)
	require.Nil(t, result.Refund)
}

func TestCancelOrder_PaidWithRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	order := db.Order{ID: 14, UserID: 6, Status: "paid"}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		CancelOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CancelOrderTxResult{Order: db.Order{ID: order.ID, Status: "cancelled"}}, nil)
	store.EXPECT().
		GetPaymentOrdersByOrder(gomock.Any(), pgtype.Int8{Int64: order.ID, Valid: true}).
		Times(1).
		Return([]db.PaymentOrder{
			{ID: 101, Status: "failed"},
			{ID: 202, Status: "paid", Amount: 900},
		}, nil)

	result, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: order.UserID, OrderID: order.ID, Reason: "change mind"})
	require.NoError(t, err)
	require.NotNil(t, result.Refund)
	require.Equal(t, int64(202), result.Refund.PaymentOrderID)
	require.Equal(t, int64(900), result.Refund.Amount)
	require.Equal(t, "change mind", result.Refund.Reason)
}
