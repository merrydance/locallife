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
		nil,
		MerchantRejectRefundInput{MerchantID: 1, OrderID: 10, Reason: "sold out"},
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
		nil,
		MerchantRejectRefundInput{MerchantID: 1, OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.Nil(t, result.RefundOrder)
}

// --- 直连支付路径（paymentTypeNative / paymentTypeMiniProgram）---

func TestProcessMerchantRejectRefund_DirectPay_WechatSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 1, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "miniprogram"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
			require.Equal(t, int64(1000), arg.RefundAmount)
			require.Equal(t, int64(1), arg.PaymentOrderID)
			return db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 99}}, nil
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
		nil,
		MerchantRejectRefundInput{MerchantID: 5, OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.NotNil(t, result.PaymentOrder)
	require.NotNil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_DirectPay_WechatProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 2, Status: "paid", OutTradeNo: "out_2", Amount: 800, PaymentType: "miniprogram"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 100}}, nil)
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
		nil,
		MerchantRejectRefundInput{MerchantID: 5, OrderID: 10, Reason: "late"},
	)
	require.NoError(t, err)
}

func TestProcessMerchantRejectRefund_DirectPay_WechatFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 3, Status: "paid", OutTradeNo: "out_3", Amount: 700, PaymentType: "miniprogram"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 101}}, nil)
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, errors.New("wechat down"))
	// R-05: 不再调用 UpdateRefundOrderToFailed，保持 pending 让恢复调度器补偿

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		paymentClient,
		nil,
		MerchantRejectRefundInput{MerchantID: 5, OrderID: 10, Reason: "sold out"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wechat refund api")
}

// --- 收付通合单支付路径（paymentTypeProfitSharing）---

func TestProcessMerchantRejectRefund_ProfitSharing_EcommerceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 10, Status: "paid", OutTradeNo: "combine_1", Amount: 2000, PaymentType: "profit_sharing"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
			require.Equal(t, int64(2000), arg.RefundAmount)
			return db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 200}}, nil
		})
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(7)).
		Times(1).
		Return(db.MerchantPaymentConfig{SubMchID: "sub_mch_007"}, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Equal(t, "sub_mch_007", req.SubMchID)
			require.Equal(t, "combine_1", req.OutTradeNo)
			require.Equal(t, int64(2000), req.RefundAmount)
			return &wechat.EcommerceRefundResponse{RefundID: "erefund_1", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), int64(200)).
		Times(1).
		Return(db.RefundOrder{}, nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), int64(10)).
		Times(1).
		Return(db.PaymentOrder{}, nil)

	result, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		ecommerceClient,
		MerchantRejectRefundInput{MerchantID: 7, OrderID: 20, Reason: "out of stock"},
	)
	require.NoError(t, err)
	require.NotNil(t, result.PaymentOrder)
	require.NotNil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_ProfitSharing_EcommerceProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 11, Status: "paid", OutTradeNo: "combine_2", Amount: 900, PaymentType: "profit_sharing"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 201}}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(7)).
		Times(1).
		Return(db.MerchantPaymentConfig{SubMchID: "sub_mch_007"}, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.EcommerceRefundResponse{RefundID: "erefund_2", Status: wechat.RefundStatusProcessing}, nil)
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       201,
			RefundID: pgtype.Text{String: "erefund_2", Valid: true},
		}).
		Times(1).
		Return(db.RefundOrder{}, nil)

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		ecommerceClient,
		MerchantRejectRefundInput{MerchantID: 7, OrderID: 21, Reason: "late"},
	)
	require.NoError(t, err)
}

func TestProcessMerchantRejectRefund_ProfitSharing_EcommerceAPIFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 12, Status: "paid", OutTradeNo: "combine_3", Amount: 1500, PaymentType: "profit_sharing"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 202}}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(7)).
		Times(1).
		Return(db.MerchantPaymentConfig{SubMchID: "sub_mch_007"}, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, errors.New("ecommerce api down"))
	// 保持 pending，由恢复调度器补偿

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		ecommerceClient,
		MerchantRejectRefundInput{MerchantID: 7, OrderID: 22, Reason: "sold out"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wechat ecommerce refund api")
}

func TestProcessMerchantRejectRefund_ProfitSharing_NoPaymentConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 13, Status: "paid", OutTradeNo: "combine_4", Amount: 600, PaymentType: "profit_sharing"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 203}}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(7)).
		Times(1).
		Return(db.MerchantPaymentConfig{}, db.ErrRecordNotFound)

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		ecommerceClient,
		MerchantRejectRefundInput{MerchantID: 7, OrderID: 23, Reason: "sold out"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "get merchant payment config")
}
