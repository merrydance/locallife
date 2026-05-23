package worker

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskUploadShippingInfoBuildsItemDescAndMerchantOrderKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const orderID int64 = 2001
	const userID int64 = 1001

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)
	processor := &RedisTaskProcessor{store: store, wechatClient: wxClient}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
			BusinessType: "order",
		}).
		Return(db.PaymentOrder{
			ID:                    4001,
			OrderID:               pgtype.Int8{Int64: orderID, Valid: true},
			UserID:                userID,
			PaymentType:           "miniprogram",
			PaymentChannel:        db.PaymentChannelBaofuAggregate,
			RequiresProfitSharing: true,
			BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
			Status:                "paid",
			OutTradeNo:            "BF202605230001",
			Attach:                pgtype.Text{String: "order_id:2001;sub_mchid:1900000118", Valid: true},
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Return(db.User{ID: userID, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().
		ListOrderItemsByOrder(gomock.Any(), orderID).
		Return([]db.OrderItem{
			{Name: "招牌牛肉饭", Quantity: 2},
			{Name: "柠檬茶", Quantity: 1},
		}, nil)
	wxClient.EXPECT().
		UploadShippingInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.UploadShippingInfoRequest) error {
			require.Empty(t, req.TransactionID)
			require.Equal(t, "BF202605230001", req.OutTradeNo)
			require.Equal(t, "1900000118", req.MchID)
			require.Equal(t, "openid-user", req.PayerOpenID)
			require.Equal(t, "招牌牛肉饭x2、柠檬茶x1", req.ItemDesc)
			require.Empty(t, req.NotifyURL)
			require.False(t, req.UploadTime.IsZero())
			return nil
		})

	task := mustUploadShippingInfoTask(t, orderID, userID)
	require.NoError(t, processor.ProcessTaskUploadShippingInfo(context.Background(), task))
}

func TestProcessTaskUploadShippingInfoBaofuAggregateIgnoresStoredBaofuTradeNoForWechatTransactionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const orderID int64 = 2004
	const userID int64 = 1004

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)
	processor := &RedisTaskProcessor{store: store, wechatClient: wxClient}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
			BusinessType: "order",
		}).
		Return(db.PaymentOrder{
			ID:             4004,
			OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
			UserID:         userID,
			PaymentType:    "miniprogram",
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Status:         "paid",
			OutTradeNo:     "BF202605230004",
			TransactionID:  pgtype.Text{String: "BFPAY_UP_202605230004", Valid: true},
			Attach:         pgtype.Text{String: "order_id:2004;sub_mchid:1900000120", Valid: true},
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Return(db.User{ID: userID, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().
		ListOrderItemsByOrder(gomock.Any(), orderID).
		Return([]db.OrderItem{{Name: "番茄鸡蛋面", Quantity: 1}}, nil)
	wxClient.EXPECT().
		UploadShippingInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.UploadShippingInfoRequest) error {
			require.Empty(t, req.TransactionID)
			require.Equal(t, "BF202605230004", req.OutTradeNo)
			require.Equal(t, "1900000120", req.MchID)
			require.Equal(t, "番茄鸡蛋面x1", req.ItemDesc)
			return nil
		})

	task := mustUploadShippingInfoTask(t, orderID, userID)
	require.NoError(t, processor.ProcessTaskUploadShippingInfo(context.Background(), task))
}

func TestProcessTaskUploadShippingInfoDirectChannelUsesWechatTransactionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const orderID int64 = 2005
	const userID int64 = 1005

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)
	processor := &RedisTaskProcessor{store: store, wechatClient: wxClient}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
			BusinessType: "order",
		}).
		Return(db.PaymentOrder{
			ID:             4005,
			OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
			UserID:         userID,
			PaymentType:    "miniprogram",
			PaymentChannel: db.PaymentChannelDirect,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Status:         "paid",
			OutTradeNo:     "WXOUT202605230005",
			TransactionID:  pgtype.Text{String: "420000000020260523000005", Valid: true},
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Return(db.User{ID: userID, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().
		ListOrderItemsByOrder(gomock.Any(), orderID).
		Return([]db.OrderItem{{Name: "鲜虾云吞面", Quantity: 1}}, nil)
	wxClient.EXPECT().
		UploadShippingInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.UploadShippingInfoRequest) error {
			require.Equal(t, "420000000020260523000005", req.TransactionID)
			require.Equal(t, "WXOUT202605230005", req.OutTradeNo)
			require.Empty(t, req.MchID)
			require.Equal(t, "鲜虾云吞面x1", req.ItemDesc)
			return nil
		})

	task := mustUploadShippingInfoTask(t, orderID, userID)
	require.NoError(t, processor.ProcessTaskUploadShippingInfo(context.Background(), task))
}

func TestProcessTaskUploadShippingInfoSkipsMerchantOrderWithoutMchID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const orderID int64 = 2002
	const userID int64 = 1002

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)
	processor := &RedisTaskProcessor{store: store, wechatClient: wxClient}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{
			ID:             4002,
			OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
			UserID:         userID,
			PaymentType:    "miniprogram",
			PaymentChannel: db.PaymentChannelBaofuAggregate,
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Status:         "paid",
			OutTradeNo:     "BF202605230002",
			Attach:         pgtype.Text{String: "order_id:2002", Valid: true},
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Return(db.User{ID: userID, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().ListOrderItemsByOrder(gomock.Any(), gomock.Any()).Times(0)
	wxClient.EXPECT().UploadShippingInfo(gomock.Any(), gomock.Any()).Times(0)

	task := mustUploadShippingInfoTask(t, orderID, userID)
	require.NoError(t, processor.ProcessTaskUploadShippingInfo(context.Background(), task))
}

func TestProcessTaskUploadShippingInfoSkipsUnsupportedPaymentChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const orderID int64 = 2003
	const userID int64 = 1003

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)
	processor := &RedisTaskProcessor{store: store, wechatClient: wxClient}

	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{
			ID:             4003,
			OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
			UserID:         userID,
			PaymentType:    "miniprogram",
			PaymentChannel: "legacy_channel",
			BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
			Status:         "paid",
			OutTradeNo:     "LEGACY202605230001",
		}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Return(db.User{ID: userID, WechatOpenid: "openid-user"}, nil)
	store.EXPECT().ListOrderItemsByOrder(gomock.Any(), gomock.Any()).Times(0)
	wxClient.EXPECT().UploadShippingInfo(gomock.Any(), gomock.Any()).Times(0)

	task := mustUploadShippingInfoTask(t, orderID, userID)
	require.NoError(t, processor.ProcessTaskUploadShippingInfo(context.Background(), task))
}

func mustUploadShippingInfoTask(t *testing.T, orderID int64, userID int64) *asynq.Task {
	t.Helper()
	task, err := NewUploadShippingInfoTask(&UploadShippingInfoPayload{OrderID: orderID, UserID: userID})
	require.NoError(t, err)
	return task
}
