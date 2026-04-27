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
		MerchantRejectRefundInput{MerchantID: 1, OrderID: 10, Reason: "sold out"},
	)
	require.NoError(t, err)
	require.Nil(t, result.RefundOrder)
}

func TestProcessMerchantRejectRefund_RejectsNonEcommercePaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	paymentOrder := db.PaymentOrder{ID: 3, Status: "paid", OutTradeNo: "out_3", Amount: 700, PaymentType: "miniprogram"}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
		nil,
		MerchantRejectRefundInput{MerchantID: 5, OrderID: 10, Reason: "sold out"},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "当前主营业务支付单不属于收付通链路，无法处理商户拒单退款，请联系平台处理", reqErr.Err.Error())
}

// --- 收付通合单支付路径（paymentTypeProfitSharing）---

func TestProcessMerchantRejectRefund_ProfitSharing_EcommerceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{ID: 10, Status: "paid", OutTradeNo: "combine_1", Amount: 2000, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce}
	capturedOutRefundNo := ""

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
			require.Equal(t, int64(2000), arg.RefundAmount)
			require.NotEmpty(t, arg.OutRefundNo)
			capturedOutRefundNo = arg.OutRefundNo
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
			return &wechat.EcommerceRefundResponse{RefundID: "erefund_1"}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       200,
			RefundID: pgtype.Text{String: "erefund_1", Valid: true},
		}).
		Times(1).
		Return(db.RefundOrder{}, nil)
	expectMerchantRejectRefundAcceptedCommand(t, store, 200, &capturedOutRefundNo, "erefund_1", 9201)

	result, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
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

	paymentOrder := db.PaymentOrder{ID: 11, Status: "paid", OutTradeNo: "combine_2", Amount: 900, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce}
	capturedOutRefundNo := ""

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateRefundOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderTxParams) (db.CreateRefundOrderTxResult, error) {
			require.NotEmpty(t, arg.OutRefundNo)
			capturedOutRefundNo = arg.OutRefundNo
			return db.CreateRefundOrderTxResult{RefundOrder: db.RefundOrder{ID: 201}}, nil
		})
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(7)).
		Times(1).
		Return(db.MerchantPaymentConfig{SubMchID: "sub_mch_007"}, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.EcommerceRefundResponse{RefundID: "erefund_2"}, nil)
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       201,
			RefundID: pgtype.Text{String: "erefund_2", Valid: true},
		}).
		Times(1).
		Return(db.RefundOrder{}, nil)
	expectMerchantRejectRefundAcceptedCommand(t, store, 201, &capturedOutRefundNo, "erefund_2", 9202)

	_, err := ProcessMerchantRejectRefund(
		context.Background(),
		store,
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

	paymentOrder := db.PaymentOrder{ID: 12, Status: "paid", OutTradeNo: "combine_3", Amount: 1500, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce}

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

	paymentOrder := db.PaymentOrder{ID: 13, Status: "paid", OutTradeNo: "combine_4", Amount: 600, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce}

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
		ecommerceClient,
		MerchantRejectRefundInput{MerchantID: 7, OrderID: 23, Reason: "sold out"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "get merchant payment config")
}

func expectMerchantRejectRefundAcceptedCommand(t *testing.T, store *mockdb.MockStore, refundOrderID int64, outRefundNo *string, refundID string, commandID int64) {
	t.Helper()

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityEcommerceRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
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
