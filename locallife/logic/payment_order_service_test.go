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
		return db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-baofu"}, nil
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

func TestPaymentOrderServiceCreatePaymentOrder_ReservationAddonRejectedByGenericPath(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1007,
		OrderID:      2007,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: reservationAddonBusiness,
		ClientIP:     "203.0.113.9",
		Amount:       3600,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	svc := NewPaymentOrderServiceWithBaofu(store, nil, nil)

	_, err := svc.CreatePaymentOrder(context.Background(), input)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "reservation_addon")
}

func TestPaymentOrderServiceCreateReservationAdjustmentPaymentCreatesPendingAdjustment(t *testing.T) {
	input := CreateReservationAdjustmentPaymentInput{
		UserID:        1008,
		ReservationID: 2008,
		MerchantID:    3008,
		CurrentTotal:  5000,
		TargetTotal:   8600,
		DeltaAmount:   3600,
		ClientIP:      "203.0.113.10",
		ExpiresAt:     time.Now().Add(30 * time.Minute),
		Items: []db.CreateReservationItemParams{{
			ReservationID: 2008,
			DishID:        pgtype.Int8{Int64: 7008, Valid: true},
			Quantity:      2,
			UnitPrice:     4300,
			TotalPrice:    8600,
		}},
	}
	txPayment := db.PaymentOrder{
		ID:                    4008,
		UserID:                input.UserID,
		Status:                paymentStatusPending,
		PaymentType:           paymentTypeMiniProgram,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          reservationAddonBusiness,
		Amount:                input.DeltaAmount,
		OutTradeNo:            "BFRA4008",
		ReservationID:         pgtype.Int8{Int64: input.ReservationID, Valid: true},
		Attach:                pgtype.Text{String: "reservation_id:2008;payment_mode:full;addon:true;sub_mchid:sub-baofu", Valid: true},
	}
	adjustment := db.ReservationAdjustment{ID: 9008, ReservationID: input.ReservationID, PaymentOrderID: pgtype.Int8{Int64: txPayment.ID, Valid: true}}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{
		unifiedResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo: "BFPAY_ADJ_4008",
			ChannelReturn: aggregatecontracts.ChannelReturn{
				WechatPayData: json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"adjustment-nonce","package":"prepay_id=adjustment","signType":"RSA","paySign":"adjustment-pay-sign"}`),
			},
		},
	}
	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	expectActiveMerchantBaofuBindingForPayment(store, input.MerchantID)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-adjustment"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), input.MerchantID).Return(db.Merchant{ID: input.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreateReservationPositiveAdjustmentPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateReservationPositiveAdjustmentPaymentTxParams) (db.CreateReservationPositiveAdjustmentPaymentTxResult, error) {
		require.Equal(t, input.ReservationID, arg.ReservationID)
		require.Equal(t, input.UserID, arg.UserID)
		require.Equal(t, input.MerchantID, arg.MerchantID)
		require.Equal(t, input.CurrentTotal, arg.ExpectedCurrentAmount)
		require.Equal(t, input.TargetTotal, arg.TargetTotal)
		require.Equal(t, input.DeltaAmount, arg.DeltaAmount)
		require.Equal(t, input.Items, arg.Items)
		require.Equal(t, "reservation_id:2008;payment_mode:full;addon:true", arg.Attach)
		return db.CreateReservationPositiveAdjustmentPaymentTxResult{
			PaymentOrder: txPayment,
			Adjustment:   adjustment,
			SubMchID:     "sub-baofu-tx",
		}, nil
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 9708}, nil)
	store.EXPECT().MarkReservationAdjustmentPendingPayment(gomock.Any(), adjustment.ID).Return(db.ReservationAdjustment{ID: adjustment.ID, Status: db.ReservationAdjustmentStatusPendingPayment}, nil)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.CreateReservationAdjustmentPayment(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
	require.True(t, baofuClient.called)
	require.Equal(t, "sub-baofu-tx", baofuClient.lastRequest.SubMchID)
	require.Equal(t, "openid-adjustment", baofuClient.lastRequest.PayExtend.SubOpenID)
	require.Equal(t, "Merchant A - Reservation Add-on", baofuClient.lastRequest.PayExtend.Body)
	require.Equal(t, input.ClientIP, baofuClient.lastRequest.RiskInfo.ClientIP)
}

func TestPaymentOrderServiceCreateReservationAdjustmentPaymentMapsTxConflict(t *testing.T) {
	input := CreateReservationAdjustmentPaymentInput{
		UserID:        1009,
		ReservationID: 2009,
		MerchantID:    3009,
		CurrentTotal:  5000,
		TargetTotal:   8600,
		DeltaAmount:   3600,
		ClientIP:      "203.0.113.11",
		ExpiresAt:     time.Now().Add(30 * time.Minute),
		Items: []db.CreateReservationItemParams{{
			ReservationID: 2009,
			DishID:        pgtype.Int8{Int64: 7009, Valid: true},
			Quantity:      2,
			UnitPrice:     4300,
			TotalPrice:    8600,
		}},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuPayment := NewBaofuPaymentService(store, &fakeBaofuAggregatePaymentClient{}, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	expectActiveMerchantBaofuBindingForPayment(store, input.MerchantID)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-adjustment"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), input.MerchantID).Return(db.Merchant{ID: input.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreateReservationPositiveAdjustmentPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreateReservationPositiveAdjustmentPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	_, err := svc.CreateReservationAdjustmentPayment(context.Background(), input)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "支付订单状态已变化")
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

func TestPaymentOrderServiceQueryPaymentOrder_BaofuAggregateSuccessAmountMismatchDoesNotApplyPaymentFact(t *testing.T) {
	input := QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}
	paymentOrder := db.PaymentOrder{
		ID:             input.PaymentOrderID,
		UserID:         input.UserID,
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Status:         paymentStatusPending,
		Amount:         8900,
		OutTradeNo:     "BF_QUERY_SERVICE_MISMATCH_1",
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
			TradeNo:          "BFTX_QUERY_SERVICE_MISMATCH_1",
			TxnState:         aggregatecontracts.PaymentStateSuccess,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
			SuccessAmountFen: paymentOrder.Amount + 1,
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), gomock.Any()).Times(0)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.QueryPaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, paymentStatusPending, result.PaymentOrder.Status)
	require.NotNil(t, result.WechatOrder)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, result.WechatOrder.TradeState)
	require.Equal(t, paymentOrder.Amount+1, result.WechatOrder.Amount.Total)
	require.True(t, baofuClient.queryCalled)
}

func TestPaymentOrderServiceQueryPaymentOrder_BaofuAggregateSuccessAppliesPaymentFact(t *testing.T) {
	input := QueryPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}
	orderID := int64(3001)
	paymentOrder := db.PaymentOrder{
		ID:             input.PaymentOrderID,
		UserID:         input.UserID,
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		PaymentType:    paymentTypeMiniProgram,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Status:         paymentStatusPending,
		Amount:         8900,
		OutTradeNo:     "BF_QUERY_SERVICE_SUCCESS_1",
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
	}
	paidPayment := paymentOrder
	paidPayment.Status = paymentStatusPaid
	paidPayment.TransactionID = pgtype.Text{String: "BFTX_QUERY_SERVICE_SUCCESS_1", Valid: true}
	processedPayment := paidPayment
	processedPayment.ProcessedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	orderResult := db.ProcessOrderPaymentTxResult{
		Order: db.Order{
			ID:         orderID,
			MerchantID: 7301,
			OrderNo:    "ORD3001",
			OrderType:  db.OrderTypeTakeaway,
			Status:     db.OrderStatusPaid,
		},
	}
	queryResult := &aggregatecontracts.UnifiedOrderResult{
		MerchantID:       "COLLECT_MER",
		TerminalID:       "COLLECT_TER",
		OutTradeNo:       paymentOrder.OutTradeNo,
		TradeNo:          "BFTX_QUERY_SERVICE_SUCCESS_1",
		TxnState:         aggregatecontracts.PaymentStateSuccess,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		SuccessAmountFen: paymentOrder.Amount,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeBaofuAggregatePaymentClient{queryResult: queryResult}
	baofuPayment := NewBaofuPaymentService(store, baofuClient, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
		Return(paymentOrder, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
			require.Equal(t, queryResult.TradeNo, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectPaymentOrder, arg.BusinessObjectType.String)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, aggregatecontracts.PaymentStateSuccess, arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, paymentOrder.Amount, arg.Amount.Int64)
			return db.ExternalPaymentFact{
				ID:                   701,
				Provider:             arg.Provider,
				Channel:              arg.Channel,
				Capability:           arg.Capability,
				FactSource:           arg.FactSource,
				ExternalObjectType:   arg.ExternalObjectType,
				ExternalObjectKey:    arg.ExternalObjectKey,
				ExternalSecondaryKey: arg.ExternalSecondaryKey,
				BusinessOwner:        arg.BusinessOwner,
				BusinessObjectType:   arg.BusinessObjectType,
				BusinessObjectID:     arg.BusinessObjectID,
				UpstreamState:        arg.UpstreamState,
				TerminalStatus:       arg.TerminalStatus,
				IsTerminal:           arg.IsTerminal,
				RawResource:          []byte(`{}`),
			}, nil
		})
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
			require.Equal(t, int64(701), arg.FactID)
			require.Equal(t, paymentFactConsumerOrderDomain, arg.Consumer)
			require.Equal(t, paymentFactBusinessObjectPaymentOrder, arg.BusinessObjectType)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID)
			return db.ExternalPaymentFactApplication{
				ID:                 801,
				FactID:             arg.FactID,
				Consumer:           arg.Consumer,
				BusinessObjectType: arg.BusinessObjectType,
				BusinessObjectID:   arg.BusinessObjectID,
				Status:             db.ExternalPaymentFactApplicationStatusPending,
			}, nil
		})
	application := db.ExternalPaymentFactApplication{
		ID:                 801,
		FactID:             701,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusProcessing,
	}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(db.ExternalPaymentFact{
		ID:                   application.FactID,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    paymentOrder.OutTradeNo,
		ExternalSecondaryKey: pgtype.Text{String: queryResult.TradeNo, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerOrder, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectPaymentOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
		UpstreamState:        aggregatecontracts.PaymentStateSuccess,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		RawResource:          []byte(`{}`),
	}, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: queryResult.TradeNo, Valid: true},
	}).Return(paidPayment, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	}).Return(db.ProcessPaymentSuccessTxResult{
		PaymentOrder: processedPayment,
		Processed:    true,
		OrderResult:  &orderResult,
	}, nil)
	store.EXPECT().CreatePaymentDomainOutboxOnce(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentDomainOutboxOnceParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
		require.Equal(t, db.PaymentDomainOutboxAggregatePaymentOrder, arg.AggregateType)
		require.Equal(t, paymentOrder.ID, arg.AggregateID)
		return db.PaymentDomainOutbox{ID: 901, EventType: arg.EventType, AggregateType: arg.AggregateType, AggregateID: arg.AggregateID, Payload: arg.Payload, Status: arg.Status}, nil
	})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{ID: application.FactID}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFactApplication{ID: application.ID, Status: db.ExternalPaymentFactApplicationStatusApplied}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), input.PaymentOrderID).Return(processedPayment, nil)

	svc := NewPaymentOrderServiceWithBaofu(store, nil, baofuPayment)
	result, err := svc.QueryPaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, paymentStatusPaid, result.PaymentOrder.Status)
	require.NotNil(t, result.WechatOrder)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, result.WechatOrder.TradeState)
	require.True(t, baofuClient.queryCalled)
	require.Equal(t, paymentOrder.OutTradeNo, baofuClient.lastQuery.OutTradeNo)
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
