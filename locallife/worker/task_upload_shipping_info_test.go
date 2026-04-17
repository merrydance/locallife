package worker_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskUploadShippingInfo_ProfitSharingUsesCombineOutTradeNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)
	processor := worker.NewTestTaskProcessor(store, nil, wechatClient, nil)

	task, err := worker.NewUploadShippingInfoTask(&worker.UploadShippingInfoPayload{
		OrderID: 1001,
		UserID:  2001,
	})
	require.NoError(t, err)

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 1001, Valid: true},
			BusinessType: "order",
		}).
		Times(1).
		Return(db.PaymentOrder{
			ID:                3001,
			Status:            "paid",
			PaymentType:       "profit_sharing",
			PaymentChannel:    db.PaymentChannelEcommerce,
			OutTradeNo:        "SUB123",
			CombinedPaymentID: pgtype.Int8{Int64: 4001, Valid: true},
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), int64(2001)).
		Times(1).
		Return(db.User{ID: 2001, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().
		GetCombinedPaymentOrder(gomock.Any(), int64(4001)).
		Times(1).
		Return(db.CombinedPaymentOrder{ID: 4001, CombineOutTradeNo: "COMBINE123"}, nil)
	store.EXPECT().
		GetCombinedPaymentSubOrdersByOrder(gomock.Any(), int64(1001)).
		Times(1).
		Return([]db.CombinedPaymentSubOrder{{SubMchid: "1900000109", OutTradeNo: "SUB123"}}, nil)
	wechatClient.EXPECT().
		UploadCombinedShippingInfo(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.UploadCombinedShippingInfoRequest) error {
			require.Equal(t, "COMBINE123", req.CombineOutTradeNo)
			require.Equal(t, "openid-user", req.PayerOpenID)
			require.Len(t, req.SubOrders, 1)
			require.Equal(t, "1900000109", req.SubOrders[0].MchID)
			require.Equal(t, "SUB123", req.SubOrders[0].OutTradeNo)
			return nil
		})

	err = processor.ProcessTaskUploadShippingInfo(context.Background(), task)
	require.NoError(t, err)
}
