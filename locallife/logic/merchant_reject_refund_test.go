package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessMerchantRejectRefund_NoPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)

	result, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		MerchantRejectRefundInput{OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.Nil(t, result.PaymentOrder)
	require.Nil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_NotPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{Status: "failed"}, nil)
	store.EXPECT().
		GetPaymentOrdersByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.PaymentOrder{{Status: "failed", BusinessType: "order"}}, nil)

	result, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		MerchantRejectRefundInput{OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.Nil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_WechatSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 1, Status: "paid", OutTradeNo: "out_1", Amount: 1000}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(1000), arg.RefundAmount)
			require.Equal(t, int64(1), arg.PaymentOrderID)
			return db.RefundOrder{ID: 99}, nil
		})
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.RefundResponse{RefundID: "refund_1", Status: wechat.RefundStatusSuccess}, nil)
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), int64(99)).
		Times(1).
		Return(db.RefundOrder{}, nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), int64(1)).
		Times(1).
		Return(db.PaymentOrder{}, nil)

	result, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		paymentClient,
		MerchantRejectRefundInput{OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.NotNil(t, result.PaymentOrder)
	require.NotNil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_WechatProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 2, Status: "paid", OutTradeNo: "out_2", Amount: 800}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.RefundOrder{ID: 100}, nil)
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.RefundResponse{RefundID: "refund_2", Status: wechat.RefundStatusProcessing}, nil)
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       100,
			RefundID: pgtype.Text{String: "refund_2", Valid: true},
		}).
		Times(1).
		Return(db.RefundOrder{}, nil)

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		paymentClient,
		MerchantRejectRefundInput{OrderID: 10, Reason: "late"},
	)
	require.NoError(t, err)
}

func TestProcessMerchantRejectRefund_WechatFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 3, Status: "paid", OutTradeNo: "out_3", Amount: 700}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.RefundOrder{ID: 101}, nil)
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, errors.New("wechat down"))
	// R-05: 不再调用 UpdateRefundOrderToFailed，保持 pending 让恢复调度器补偿

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		paymentClient,
		MerchantRejectRefundInput{OrderID: 10, Reason: "sold out"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wechat refund api")
}
