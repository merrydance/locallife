package logic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectActiveMerchantBaofuBindingForPayment(store *mockdb.MockStore, merchantID int64) {
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchantID,
		}).
		Return(db.BaofuAccountBinding{
			OwnerType:    db.BaofuAccountOwnerTypeMerchant,
			OwnerID:      merchantID,
			AccountType:  db.BaofuAccountTypeBusiness,
			OpenState:    db.BaofuAccountOpenStateActive,
			ContractNo:   pgtype.Text{String: "CM3001", Valid: true},
			SharingMerID: pgtype.Text{String: "CM3001", Valid: true},
		}, nil)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchantID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		}).
		Return(db.BaofuMerchantReport{
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchantID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
			SubMchID:        pgtype.Text{String: "sub-baofu", Valid: true},
		}, nil)
}

func TestPaymentOrderServiceCreatePaymentOrder_UsesBaofuForMainBusiness(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:                    4003,
		UserID:                input.UserID,
		Status:                paymentStatusPending,
		PaymentType:           paymentTypeMiniProgram,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          businessTypeOrder,
		Amount:                1000,
		OutTradeNo:            "baofu-out-trade-no",
		Attach:                pgtype.Text{String: "order_id:2001;sub_mchid:sub-baofu", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{
		unifiedResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo: "BFPAY_4003",
			ChannelReturn: aggregatecontracts.ChannelReturn{
				WechatPayData: json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"nonce-baofu","package":"prepay_id=baofu","signType":"RSA","paySign":"pay-sign-baofu"}`),
			},
		},
	}
	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	expectActiveMerchantBaofuBindingForPayment(store, order.MerchantID)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-baofu"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant B"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.PaymentChannel)
		require.True(t, arg.RequiresProfitSharing)
		require.Equal(t, "order_id:2001", arg.Attach)
		return db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment}, nil
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
		require.Equal(t, txPayment.OutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentCommandStatusSubmitted, arg.CommandStatus)
		require.NotContains(t, string(arg.ResponseSnapshot), "openid-baofu")
		return db.ExternalPaymentCommand{ID: 9701}, nil
	})

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.CreatePaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, db.PaymentChannelBaofuAggregate, result.PaymentOrder.PaymentChannel)
	require.True(t, result.PaymentOrder.RequiresProfitSharing)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "1767225600", result.PayParams.TimeStamp)
	require.Equal(t, "nonce-baofu", result.PayParams.NonceStr)
	require.Equal(t, "prepay_id=baofu", result.PayParams.Package)
	require.Equal(t, "RSA", result.PayParams.SignType)
	require.Equal(t, "pay-sign-baofu", result.PayParams.PaySign)
	require.True(t, baofuClient.called)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRequest.MerchantID)
	require.Equal(t, "sub-baofu", baofuClient.lastRequest.SubMchID)
	require.Equal(t, "openid-baofu", baofuClient.lastRequest.PayExtend.SubOpenID)
	require.Equal(t, "Merchant B - Order Payment", baofuClient.lastRequest.PayExtend.Body)
}

func TestPaymentOrderServiceCreatePaymentOrder_RequiresMerchantBaofuReadiness(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant B"}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   order.MerchantID,
	}).Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Times(0)

	baofuPayment := NewBaofuPaymentService(store, &fakeBaofuAggregatePaymentClient{}, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})
	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	_, err := svc.CreatePaymentOrder(context.Background(), input)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Equal(t, "商户结算账户未开通，暂不能创建支付订单", reqErr.Err.Error())
}

func TestPaymentOrderServiceCreatePaymentOrder_BaofuMissingClientFailsBeforeLocalPayment(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant B"}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Times(0)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, nil)
	_, err := svc.CreatePaymentOrder(context.Background(), input)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.Equal(t, "宝付支付通道未配置，请联系平台处理", reqErr.Err.Error())
}

func TestPaymentOrderServiceCreatePaymentOrder_BaofuWechatChannelNotReadyFailsBeforeClientCall(t *testing.T) {
	tests := []struct {
		name   string
		report db.BaofuMerchantReport
	}{
		{
			name: "missing merchant report sub mch id",
			report: db.BaofuMerchantReport{
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         3001,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportState:     db.BaofuMerchantReportStateSucceeded,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
			},
		},
		{
			name: "pending applet bind",
			report: db.BaofuMerchantReport{
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         3001,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportState:     db.BaofuMerchantReportStateSucceeded,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
				SubMchID:        pgtype.Text{String: "sub-baofu", Valid: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreatePaymentOrderInput{
				UserID:       1001,
				OrderID:      2001,
				PaymentType:  paymentTypeMiniProgram,
				BusinessType: businessTypeOrder,
				ClientIP:     "127.0.0.1",
			}
			order := db.Order{
				ID:          input.OrderID,
				UserID:      input.UserID,
				MerchantID:  3001,
				OrderType:   orderTypeTakeaway,
				Status:      "pending",
				TotalAmount: 1000,
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			baofuClient := &fakeBaofuAggregatePaymentClient{
				unifiedResult: &aggregatecontracts.UnifiedOrderResult{},
			}
			baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
				CollectMerchantID: "COLLECT_MER",
				CollectTerminalID: "COLLECT_TER",
				MiniProgramAppID:  "wxapp",
				PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
			})

			store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
			store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
				BusinessType: businessTypeOrder,
			}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant B"}, nil)
			store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
				OwnerType: db.BaofuAccountOwnerTypeMerchant,
				OwnerID:   order.MerchantID,
			}).Return(db.BaofuAccountBinding{
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				OwnerID:      order.MerchantID,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateActive,
				ContractNo:   pgtype.Text{String: "CM3001", Valid: true},
				SharingMerID: pgtype.Text{String: "CM3001", Valid: true},
			}, nil)
			store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
				OwnerType:  db.BaofuAccountOwnerTypeMerchant,
				OwnerID:    order.MerchantID,
				ReportType: db.BaofuMerchantReportTypeWechat,
			}).Return(tt.report, nil)
			store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Times(0)

			svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
			_, err := svc.CreatePaymentOrder(context.Background(), input)

			reqErr := assertRequestError(t, err)
			require.Equal(t, http.StatusBadRequest, reqErr.Status)
			require.Equal(t, "商户微信支付通道待开通，暂不能创建微信生态支付订单", reqErr.Err.Error())
			require.False(t, baofuClient.called)
		})
	}
}

func TestPaymentOrderServiceQueryPaymentOrder_DirectPaymentIgnoresBaofuMainBusinessClient(t *testing.T) {
	input := QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}
	paymentOrder := db.PaymentOrder{
		ID:             input.PaymentOrderID,
		UserID:         input.UserID,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
		Status:         paymentStatusPending,
		OutTradeNo:     "DP202605040001",
		PrepayID:       pgtype.Text{String: "prepay-direct-boundary", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{
		unifiedResult: &aggregatecontracts.UnifiedOrderResult{},
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
		Return(paymentOrder, nil)
	directClient.EXPECT().
		QueryOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).
		Return(&wechatcontracts.DirectOrderQueryResponse{
			OutTradeNo:     paymentOrder.OutTradeNo,
			TradeState:     "NOTPAY",
			TradeStateDesc: "待支付",
			Amount:         wechatcontracts.DirectOrderQueryAmount{Total: paymentOrder.Amount},
		}, nil)
	directClient.EXPECT().
		GenerateJSAPIPayParams("prepay-direct-boundary").
		Return(&wechat.JSAPIPayParams{NonceStr: "direct-boundary-nonce"}, nil)

	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})
	svc := NewPaymentOrderServiceWithBaofu(store, directClient, baofuPayment)
	result, err := svc.QueryPaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, "NOTPAY", result.WechatOrder.TradeState)
	require.NotNil(t, result.PayParams)
	require.False(t, baofuClient.called)
}

func TestPaymentOrderServiceQueryPaymentOrder_BaofuAggregateUsesRemoteQuery(t *testing.T) {
	input := QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}
	paymentOrder := db.PaymentOrder{
		ID:             input.PaymentOrderID,
		UserID:         input.UserID,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Status:         paymentStatusPending,
		Amount:         8900,
		OutTradeNo:     "BF_QUERY_SERVICE_1",
		TransactionID:  pgtype.Text{String: "BFTX_QUERY_SERVICE_1", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{
		queryResult: &aggregatecontracts.UnifiedOrderResult{
			MerchantID:       "COLLECT_MER",
			TerminalID:       "COLLECT_TER",
			OutTradeNo:       paymentOrder.OutTradeNo,
			TradeNo:          "BFTX_QUERY_SERVICE_1",
			TxnState:         aggregatecontracts.PaymentStateWaitPaying,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
			SuccessAmountFen: paymentOrder.Amount,
		},
	}
	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
		Return(paymentOrder, nil)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.QueryPaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, result.WechatOrder)
	require.Equal(t, aggregatecontracts.PaymentStateWaitPaying, result.WechatOrder.TradeState)
	require.Equal(t, "待支付", result.WechatOrder.TradeStateDesc)
	require.Equal(t, paymentOrder.Amount, result.WechatOrder.Amount.Total)
	require.True(t, baofuClient.queryCalled)
	require.Equal(t, "BFTX_QUERY_SERVICE_1", baofuClient.lastQuery.TradeNo)
	require.Empty(t, baofuClient.lastQuery.OutTradeNo)
	require.False(t, baofuClient.called)
}

func TestPaymentOrderServiceClosePaymentOrder_BaofuAggregateCallsRemoteCloseBeforeLocalClose(t *testing.T) {
	input := ClosePaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}
	paymentOrder := db.PaymentOrder{
		ID:             input.PaymentOrderID,
		UserID:         input.UserID,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Status:         paymentStatusPending,
		OutTradeNo:     "BF_CLOSE_SERVICE_1",
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{
		closeResult: &aggregatecontracts.OrderCloseResult{
			MerchantID: "COLLECT_MER",
			TerminalID: "COLLECT_TER",
			OutTradeNo: paymentOrder.OutTradeNo,
			ResultCode: aggregatecontracts.BusinessResultCodeSuccess,
		},
	}
	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	gomock.InOrder(
		store.EXPECT().GetPaymentOrder(gomock.Any(), input.PaymentOrderID).Return(paymentOrder, nil),
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeClosePayment, arg.CommandType)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
			return db.ExternalPaymentCommand{ID: 22001}, nil
		}),
		store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).Return(closedPaymentOrder, nil),
	)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.ClosePaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, "closed", result.PaymentOrder.Status)
	require.True(t, baofuClient.closeCalled)
	require.Equal(t, "COLLECT_MER", baofuClient.lastClose.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastClose.TerminalID)
	require.Equal(t, paymentOrder.OutTradeNo, baofuClient.lastClose.OutTradeNo)
}

func TestPaymentOrderServiceGetPaymentOrder(t *testing.T) {
	input := GetPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result GetPaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1}, nil)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, result GetPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, input.PaymentOrderID, result.PaymentOrder.ID)
				require.Equal(t, input.UserID, result.PaymentOrder.UserID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			svc := NewPaymentOrderServiceWithDirectPayment(store, nil)
			result, err := svc.GetPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceListPaymentOrders(t *testing.T) {
	baseInput := ListPaymentOrdersInput{UserID: 1002, PageID: 1, PageSize: 10}

	testCases := []struct {
		name       string
		input      ListPaymentOrdersInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ListPaymentOrdersResult, err error)
	}{
		{
			name: "ByOrderID_NotFound_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3001)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3001, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_OtherUser_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3002)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3002, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4001, UserID: baseInput.UserID + 10}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_Success",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3003)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3003, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4002, UserID: baseInput.UserID}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 1)
				require.Equal(t, int64(4002), result.PaymentOrders[0].ID)
				require.Equal(t, int64(1), result.TotalCount)
			},
		},
		{
			name:  "Paged_Success",
			input: ListPaymentOrdersInput{UserID: baseInput.UserID, PageID: 2, PageSize: 5},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: baseInput.UserID,
						Limit:  5,
						Offset: 5,
					}).
					Times(1).
					Return([]db.PaymentOrder{{ID: 5001}, {ID: 5002}}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 2)
				require.Equal(t, int64(2), result.TotalCount)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			svc := NewPaymentOrderServiceWithDirectPayment(store, nil)
			result, err := svc.ListPaymentOrders(context.Background(), tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceClosePaymentOrder(t *testing.T) {
	input := ClosePaymentOrderInput{UserID: 1003, PaymentOrderID: 2003}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface)
		check      func(t *testing.T, result ClosePaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "paid"}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only pending payment orders can be closed", reqErr.Err.Error())
			},
		},
		{
			name: "Success_WithoutClient",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockDirectPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, OutTradeNo: "P202001010000000001", PrepayID: pgtype.Text{Valid: true, String: "prepay"}}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			svc := NewPaymentOrderServiceWithDirectPayment(store, nil)
			result, err := svc.ClosePaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}
