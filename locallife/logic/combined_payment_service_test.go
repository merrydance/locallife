package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateCombinedPaymentOrder(t *testing.T) {
	baseInput := CreateCombinedPaymentOrderInput{
		UserID:   1001,
		OrderIDs: []int64{11, 22},
		ClientIP: "127.0.0.1",
	}

	testCases := []struct {
		name       string
		input      CreateCombinedPaymentOrderInput
		setupNow   func() time.Time
		buildStubs func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface)
		check      func(t *testing.T, result CreateCombinedPaymentOrderResult, err error)
	}{
		{
			name:  "ClientNotConfigured",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.EqualError(t, err, "ecommerce client: not configured")
			},
		},
		{
			name: "InvalidOrderIDs",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{0, -1},
				ClientIP: "127.0.0.1",
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "invalid order ids", reqErr.Err.Error())
			},
		},
		{
			name: "TooManyOrders",
			input: CreateCombinedPaymentOrderInput{
				UserID: 1001,
				OrderIDs: func() []int64 {
					orderIDs := make([]int64, 51)
					for index := range orderIDs {
						orderIDs[index] = int64(index + 1)
					}
					return orderIDs
				}(),
				ClientIP: "127.0.0.1",
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Contains(t, reqErr.Err.Error(), "too many orders")
			},
		},
		{
			name:  "UserNoOpenID",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: ""}, nil)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "wechat openid not found", reqErr.Err.Error())
			},
		},
		{
			name:  "GetUserError",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{}, errors.New("db get user failed"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "get user")
				require.Contains(t, err.Error(), "db get user failed")
			},
		},
		{
			name:  "CreateTxMappedForbidden",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, errors.New("order 11 does not belong to user"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "订单不属于当前用户", reqErr.Err.Error())
			},
		},
		{
			name:  "CreateTxMappedInvalidStatus",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, errors.New("order 11 status is paid, expect pending"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "订单已不在待支付状态，请刷新页面确认", reqErr.Err.Error())
			},
		},
		{
			name:  "CreateTxMappedInvalidPaymentConfig",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, errors.New("merchant 8 payment config invalid"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "商户支付配置无效，请联系平台处理", reqErr.Err.Error())
			},
		},
		{
			name:  "CreateTxMappedUnsupportedTakeaway",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, db.ErrCombinedPaymentUnsupportedOrderType)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "外带订单不支持合单支付，请使用普通支付入口", reqErr.Err.Error())
			},
		},
		{
			name:  "CreateTxMappedActivePaymentOrder",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, errors.New("order 11 has processing payment order"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "订单已有进行中的支付单，请先刷新支付结果", reqErr.Err.Error())
			},
		},
		{
			name:  "CreateTxUnmappedError",
			input: baseInput,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), baseInput.UserID).
					Times(1).
					Return(db.User{ID: baseInput.UserID, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{}, errors.New("db timeout"))
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "create combined payment")
				require.Contains(t, err.Error(), "db timeout")
			},
		},
		{
			name: "CreateCombineOrderError",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{11, 22, 11, 0},
				ClientIP: "127.0.0.1",
			},
			setupNow: func() time.Time {
				return time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				capturedCombineOutTradeNo := ""
				store.EXPECT().
					GetUser(gomock.Any(), int64(1001)).
					Times(1).
					Return(db.User{ID: 1001, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, arg db.CreateCombinedPaymentTxParams) (db.CreateCombinedPaymentTxResult, error) {
						require.Equal(t, []int64{11, 22}, arg.OrderIDs)
						require.True(t, strings.HasPrefix(arg.CombineOutTradeNo, "CP"))
						capturedCombineOutTradeNo = arg.CombineOutTradeNo
						return db.CreateCombinedPaymentTxResult{
							CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 3001, UserID: 1001, CombineOutTradeNo: arg.CombineOutTradeNo, Status: paymentStatusPending},
							OrderInfos: []db.CombinedPaymentOrderInfo{
								{
									Order:         db.Order{ID: 11, MerchantID: 501},
									PaymentOrder:  db.PaymentOrder{ID: 7001, Amount: 3200, OutTradeNo: "P11", Attach: pgtype.Text{String: "a11", Valid: true}},
									PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
									Merchant:      db.Merchant{Name: "m1"},
								},
								{
									Order:         db.Order{ID: 22, MerchantID: 502},
									PaymentOrder:  db.PaymentOrder{ID: 7002, Amount: 4800, OutTradeNo: "P22", Attach: pgtype.Text{String: "a22", Valid: true}},
									PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190002"},
									Merchant:      db.Merchant{Name: "m2"},
								},
							},
						}, nil
					})

				client.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, req *wechatcontracts.CombineOrderRequest) (*wechatcontracts.CombineOrderResponse, *wechat.JSAPIPayParams, error) {
						require.Equal(t, "openid-1", req.PayerOpenID)
						require.Equal(t, "127.0.0.1", req.SceneInfo.PayerClientIP)
						require.Len(t, req.SubOrders, 2)
						return nil, nil, errors.New("wechat create combine failed")
					})

				// Cleanup: payment orders and combined order should be marked as closed
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), int64(7001)).
					Times(1).
					Return(db.PaymentOrder{}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), int64(7002)).
					Times(1).
					Return(db.PaymentOrder{}, nil)
				store.EXPECT().
					UpdateCombinedPaymentOrderToClosed(gomock.Any(), int64(3001)).
					Times(1).
					Return(db.CombinedPaymentOrder{}, nil)
				expectCombinePaymentCommand(t, store, 3001, &capturedCombineOutTradeNo, "", db.ExternalPaymentCommandStatusRejected, "", 9901)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "create combine order")
			},
		},
		{
			name: "CreateCombineOrderEmptyPrepayID",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{11, 22},
				ClientIP: "127.0.0.1",
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), int64(1001)).
					Times(1).
					Return(db.User{ID: 1001, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 3051, UserID: 1001, CombineOutTradeNo: "CP20260301121212", Status: paymentStatusPending},
						OrderInfos: []db.CombinedPaymentOrderInfo{
							{
								Order:         db.Order{ID: 11, MerchantID: 501},
								PaymentOrder:  db.PaymentOrder{ID: 7051, Amount: 3200, OutTradeNo: "P11", Attach: pgtype.Text{String: "a11", Valid: true}},
								PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
								Merchant:      db.Merchant{Name: "m1"},
							},
						},
					}, nil)

				client.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechatcontracts.CombineOrderResponse{PrepayID: "   "}, &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "n", Package: "p", SignType: "RSA", PaySign: "s"}, nil)
				// Cleanup: payment order and combined order should be marked as closed
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), int64(7051)).
					Times(1).
					Return(db.PaymentOrder{}, nil)
				store.EXPECT().
					UpdateCombinedPaymentOrderToClosed(gomock.Any(), int64(3051)).
					Times(1).
					Return(db.CombinedPaymentOrder{}, nil)
				expectCombinePaymentCommand(t, store, 3051, stringPtrIfNotEmpty("CP20260301121212"), "", db.ExternalPaymentCommandStatusRejected, "", 9902)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusBadGateway, reqErr.Status)
				require.Equal(t, "微信支付未返回可用预支付会话，请返回订单页重新发起支付", reqErr.Err.Error())
			},
		},
		{
			name: "CreateCombineOrderNilResponse",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{11, 22},
				ClientIP: "127.0.0.1",
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), int64(1001)).
					Times(1).
					Return(db.User{ID: 1001, WechatOpenid: "openid-1"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 3052, UserID: 1001, CombineOutTradeNo: "CP20260301121213", Status: paymentStatusPending},
						OrderInfos: []db.CombinedPaymentOrderInfo{
							{
								Order:         db.Order{ID: 11, MerchantID: 501},
								PaymentOrder:  db.PaymentOrder{ID: 7052, Amount: 3200, OutTradeNo: "P11", Attach: pgtype.Text{String: "a11", Valid: true}},
								PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
								Merchant:      db.Merchant{Name: "m1"},
							},
						},
					}, nil)

				client.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "n", Package: "p", SignType: "RSA", PaySign: "s"}, nil)
				// Cleanup: payment order and combined order should be marked as closed
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), int64(7052)).
					Times(1).
					Return(db.PaymentOrder{}, nil)
				store.EXPECT().
					UpdateCombinedPaymentOrderToClosed(gomock.Any(), int64(3052)).
					Times(1).
					Return(db.CombinedPaymentOrder{}, nil)
				expectCombinePaymentCommand(t, store, 3052, stringPtrIfNotEmpty("CP20260301121213"), "", db.ExternalPaymentCommandStatusRejected, "", 9903)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				reqErr := assertRequestError(t, err)
				require.Equal(t, http.StatusBadGateway, reqErr.Status)
				require.Equal(t, "微信支付未返回可用预支付会话，请返回订单页重新发起支付", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{11, 22, 11},
				ClientIP: "127.0.0.1",
			},
			setupNow: func() time.Time {
				return time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				capturedCombineOutTradeNo := ""
				store.EXPECT().
					GetUser(gomock.Any(), int64(1001)).
					Times(1).
					Return(db.User{ID: 1001, WechatOpenid: "openid-ok"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, arg db.CreateCombinedPaymentTxParams) (db.CreateCombinedPaymentTxResult, error) {
						require.Equal(t, []int64{11, 22}, arg.OrderIDs)
						require.True(t, strings.HasPrefix(arg.CombineOutTradeNo, "CP"))
						capturedCombineOutTradeNo = arg.CombineOutTradeNo
						return db.CreateCombinedPaymentTxResult{
							CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 9001, UserID: 1001, CombineOutTradeNo: arg.CombineOutTradeNo, Status: paymentStatusPending},
							OrderInfos: []db.CombinedPaymentOrderInfo{
								{
									Order:         db.Order{ID: 11, MerchantID: 501},
									PaymentOrder:  db.PaymentOrder{ID: 7101, Amount: 3200, OutTradeNo: "PO-11", Attach: pgtype.Text{String: "attach-11", Valid: true}},
									PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
									Merchant:      db.Merchant{Name: "merchant-1"},
								},
								{
									Order:         db.Order{ID: 22, MerchantID: 502},
									PaymentOrder:  db.PaymentOrder{ID: 7102, Amount: 4800, OutTradeNo: "PO-22", Attach: pgtype.Text{String: "attach-22", Valid: true}},
									PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190002"},
									Merchant:      db.Merchant{Name: "merchant-2"},
								},
							},
						}, nil
					})

				client.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, req *wechatcontracts.CombineOrderRequest) (*wechatcontracts.CombineOrderResponse, *wechat.JSAPIPayParams, error) {
						require.Equal(t, "openid-ok", req.PayerOpenID)
						require.Equal(t, "127.0.0.1", req.SceneInfo.PayerClientIP)
						require.Len(t, req.SubOrders, 2)
						require.Equal(t, "190001", req.SubOrders[0].SubMchID)
						require.Equal(t, "PO-11", req.SubOrders[0].OutTradeNo)
						require.Equal(t, int64(3200), req.SubOrders[0].Amount)
						return &wechatcontracts.CombineOrderResponse{PrepayID: "wx-prepay-1"}, &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "n", Package: "p", SignType: "RSA", PaySign: "s"}, nil
					})

				store.EXPECT().
					UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{
						ID:       9001,
						PrepayID: pgtype.Text{String: "wx-prepay-1", Valid: true},
					}).
					Times(1).
					Return(db.CombinedPaymentOrder{ID: 9001, Status: paymentStatusPending, PrepayID: pgtype.Text{String: "wx-prepay-1", Valid: true}}, nil)
				expectCombinePaymentCommand(t, store, 9001, &capturedCombineOutTradeNo, "wx-prepay-1", db.ExternalPaymentCommandStatusAccepted, "", 9904)
			},
			check: func(t *testing.T, result CreateCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(9001), result.CombinedPayment.ID)
				require.Equal(t, "wx-prepay-1", result.CombinedPayment.PrepayID.String)
				require.NotNil(t, result.PayParams)
				require.Len(t, result.SubOrders, 2)
				require.Equal(t, int64(11), result.SubOrders[0].OrderID)
				require.Equal(t, "PO-11", result.SubOrders[0].OutTradeNo)
			},
		},
		{
			name: "UpdateCombinedPaymentOrderPrepayError",
			input: CreateCombinedPaymentOrderInput{
				UserID:   1001,
				OrderIDs: []int64{11, 22},
				ClientIP: "127.0.0.1",
			},
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetUser(gomock.Any(), int64(1001)).
					Times(1).
					Return(db.User{ID: 1001, WechatOpenid: "openid-ok"}, nil)

				store.EXPECT().
					CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCombinedPaymentTxResult{
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 9101, UserID: 1001, CombineOutTradeNo: "CP20260301112233", Status: paymentStatusPending},
						OrderInfos: []db.CombinedPaymentOrderInfo{
							{
								Order:         db.Order{ID: 11, MerchantID: 501},
								PaymentOrder:  db.PaymentOrder{ID: 7201, Amount: 3200, OutTradeNo: "PO-11", Attach: pgtype.Text{String: "attach-11", Valid: true}},
								PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
								Merchant:      db.Merchant{Name: "merchant-1"},
							},
						},
					}, nil)

				client.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechatcontracts.CombineOrderResponse{PrepayID: "wx-prepay-update-fail"}, &wechat.JSAPIPayParams{TimeStamp: "1", NonceStr: "n", Package: "p", SignType: "RSA", PaySign: "s"}, nil)

				store.EXPECT().
					UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{
						ID:       9101,
						PrepayID: pgtype.Text{String: "wx-prepay-update-fail", Valid: true},
					}).
					Times(1).
					Return(db.CombinedPaymentOrder{}, errors.New("update combined prepay failed"))

				// 新增的补偿清理逻辑：标记子单和合单为 failed，尝试关闭微信合单
				store.EXPECT().
					UpdatePaymentOrderToFailed(gomock.Any(), int64(7201)).
					Times(1).
					Return(db.PaymentOrder{}, nil)
				store.EXPECT().
					UpdateCombinedPaymentOrderToFailed(gomock.Any(), int64(9101)).
					Times(1).
					Return(db.CombinedPaymentOrder{}, nil)
				client.EXPECT().
					CloseCombineOrder(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			check: func(t *testing.T, _ CreateCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "update combined payment prepay")
				require.Contains(t, err.Error(), "update combined prepay failed")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			client := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, client)

			var ecommerceClient wechat.EcommerceClientInterface
			if tc.name != "ClientNotConfigured" {
				ecommerceClient = client
			}

			svc := NewCombinedPaymentService(store, ecommerceClient)
			if tc.setupNow != nil {
				svc.now = tc.setupNow
			}

			result, err := svc.CreateCombinedPaymentOrder(context.Background(), tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestCreateCombinedPaymentOrder_ReusesConcurrentPendingCombinedPayment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := mockwechat.NewMockEcommerceClientInterface(ctrl)
	svc := NewCombinedPaymentService(store, client)

	input := CreateCombinedPaymentOrderInput{
		UserID:   1001,
		OrderIDs: []int64{22, 11},
		ClientIP: "127.0.0.1",
	}

	subOrders, err := json.Marshal([]combinedSubOrderPayload{
		{OrderID: 11, PaymentOrderID: 7001, MerchantID: 501, SubMchID: "190001", Amount: 3200, OutTradeNo: "P11", Description: "m1 - Order Payment"},
		{OrderID: 22, PaymentOrderID: 7002, MerchantID: 502, SubMchID: "190002", Amount: 4800, OutTradeNo: "P22", Description: "m2 - Order Payment"},
	})
	require.NoError(t, err)

	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().
		CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreateCombinedPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 11, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{ID: 7001, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 22, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{ID: 7002, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(9001)).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:                9001,
			UserID:            input.UserID,
			CombineOutTradeNo: "CP20260406000001",
			TotalAmount:       8000,
			PrepayID:          pgtype.Text{String: "combine-prepay-9001", Valid: true},
			Status:            paymentStatusPending,
			SubOrders:         subOrders,
		}, nil)
	client.EXPECT().
		GenerateJSAPIPayParams("combine-prepay-9001").
		Return(&wechat.JSAPIPayParams{Package: "prepay_id=combine-prepay-9001"}, nil)

	result, err := svc.CreateCombinedPaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(9001), result.CombinedPayment.ID)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=combine-prepay-9001", result.PayParams.Package)
	require.Len(t, result.SubOrders, 2)
	require.Equal(t, []int64{11, 22}, []int64{result.SubOrders[0].OrderID, result.SubOrders[1].OrderID})
}

func TestCreateCombinedPaymentOrder_CreateRejectedSkipsCommandWhenCloseFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := mockwechat.NewMockEcommerceClientInterface(ctrl)
	svc := NewCombinedPaymentService(store, client)

	input := CreateCombinedPaymentOrderInput{UserID: 1001, OrderIDs: []int64{11}, ClientIP: "127.0.0.1"}
	combinedPayment := db.CombinedPaymentOrder{ID: 9301, UserID: 1001, CombineOutTradeNo: "CP20260425101010", Status: paymentStatusPending}
	paymentOrder := db.PaymentOrder{ID: 7301, Amount: 3200, OutTradeNo: "PO-7301", Attach: pgtype.Text{String: "attach-7301", Valid: true}}

	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreateCombinedPaymentTxResult{
		CombinedPaymentOrder: combinedPayment,
		OrderInfos: []db.CombinedPaymentOrderInfo{{
			Order:         db.Order{ID: 11, MerchantID: 501},
			PaymentOrder:  paymentOrder,
			PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
			Merchant:      db.Merchant{Name: "m1"},
		}},
	}, nil)
	client.EXPECT().CreateCombineOrder(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("wechat create combine failed"))
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, nil)
	store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedPayment.ID).Return(db.CombinedPaymentOrder{}, errors.New("combined close failed"))

	_, err := svc.CreateCombinedPaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create combine order")
}

func expectCombinePaymentCommand(t *testing.T, store *mockdb.MockStore, combinedPaymentID int64, combineOutTradeNo *string, secondaryKey string, status string, errorCode string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "combined_payment_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, combinedPaymentID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectCombinedPayment, arg.ExternalObjectType)
		require.NotNil(t, combineOutTradeNo)
		require.NotEmpty(t, *combineOutTradeNo)
		require.Equal(t, *combineOutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), *combineOutTradeNo)
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

func expectCombinedPaymentMerchantBaofuReady(store *mockdb.MockStore, userID int64, orders []db.Order) {
	seenMerchantIDs := make(map[int64]struct{}, len(orders))
	for _, order := range orders {
		order := order
		store.EXPECT().
			GetOrder(gomock.Any(), order.ID).
			Times(1).
			Return(order, nil)
		if order.UserID != userID {
			continue
		}
		if _, exists := seenMerchantIDs[order.MerchantID]; exists {
			continue
		}
		seenMerchantIDs[order.MerchantID] = struct{}{}
		store.EXPECT().
			GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
				OwnerType: db.BaofuAccountOwnerTypeMerchant,
				OwnerID:   order.MerchantID,
			}).
			Times(1).
			Return(db.BaofuAccountBinding{
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				OwnerID:      order.MerchantID,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateActive,
				ContractNo:   pgtype.Text{String: "contract-ready", Valid: true},
				SharingMerID: pgtype.Text{String: "sharing-ready", Valid: true},
			}, nil)
		store.EXPECT().
			GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
				OwnerType:  db.BaofuAccountOwnerTypeMerchant,
				OwnerID:    order.MerchantID,
				ReportType: db.BaofuMerchantReportTypeWechat,
			}).
			Times(1).
			Return(db.BaofuMerchantReport{
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         order.MerchantID,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportState:     db.BaofuMerchantReportStateSucceeded,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
				SubMchID:        pgtype.Text{String: "sub-mch-ready", Valid: true},
			}, nil)
	}
}

func TestCreateCombinedPaymentOrder_UsesOrdinaryServiceProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	svc := NewCombinedPaymentServiceWithOrdinaryServiceProvider(store, ordinaryClient)
	svc.now = func() time.Time { return time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC) }

	input := CreateCombinedPaymentOrderInput{UserID: 1001, OrderIDs: []int64{22, 11, 11}, ClientIP: "127.0.0.1"}
	capturedCombineOutTradeNo := ""
	expectCombinedPaymentMerchantBaofuReady(store, input.UserID, []db.Order{
		{ID: 11, UserID: input.UserID, MerchantID: 501},
		{ID: 22, UserID: input.UserID, MerchantID: 502},
	})
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-ok"}, nil)
	store.EXPECT().CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateCombinedPaymentTxParams) (db.CreateCombinedPaymentTxResult, error) {
		require.Equal(t, []int64{11, 22}, arg.OrderIDs)
		require.True(t, strings.HasPrefix(arg.CombineOutTradeNo, "CP"))
		capturedCombineOutTradeNo = arg.CombineOutTradeNo
		return db.CreateCombinedPaymentTxResult{
			CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 9001, UserID: 1001, CombineOutTradeNo: arg.CombineOutTradeNo, Status: paymentStatusPending},
			OrderInfos: []db.CombinedPaymentOrderInfo{
				{
					Order:         db.Order{ID: 11, MerchantID: 501},
					PaymentOrder:  db.PaymentOrder{ID: 7101, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, RequiresProfitSharing: true, Amount: 3200, OutTradeNo: "PO-11", Attach: pgtype.Text{String: "attach-11", Valid: true}},
					PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
					Merchant:      db.Merchant{Name: "merchant-1"},
				},
				{
					Order:         db.Order{ID: 22, MerchantID: 502},
					PaymentOrder:  db.PaymentOrder{ID: 7102, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, RequiresProfitSharing: false, Amount: 4800, OutTradeNo: "PO-22", Attach: pgtype.Text{String: "attach-22", Valid: true}},
					PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190002"},
					Merchant:      db.Merchant{Name: "merchant-2"},
				},
			},
		}, nil
	})
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       9001,
		PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
	}).Return(db.CombinedPaymentOrder{ID: 9001, Status: paymentStatusPending, PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true}}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.Equal(t, capturedCombineOutTradeNo, arg.ExternalObjectKey)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, "prepay-combine-ordinary", arg.ExternalSecondaryKey.String)
		return db.ExternalPaymentCommand{ID: 9900}, nil
	})

	result, err := svc.CreateCombinedPaymentOrder(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(9001), result.CombinedPayment.ID)
	require.NotNil(t, result.PayParams)
	require.NotNil(t, ordinaryClient.createCombinePaymentRequest)
	require.Equal(t, "wxsp_app", ordinaryClient.createCombinePaymentRequest.CombineAppID)
	require.Equal(t, "1900000109", ordinaryClient.createCombinePaymentRequest.CombineMchID)
	require.Equal(t, capturedCombineOutTradeNo, ordinaryClient.createCombinePaymentRequest.CombineOutTradeNo)
	require.Equal(t, "openid-ok", ordinaryClient.createCombinePaymentRequest.CombinePayerInfo.OpenID)
	require.Equal(t, "127.0.0.1", ordinaryClient.createCombinePaymentRequest.SceneInfo.PayerClientIP)
	require.Equal(t, "https://api.example.com/v1/webhooks/wechat-ordinary/combine-notify", ordinaryClient.createCombinePaymentRequest.NotifyURL)
	require.Len(t, ordinaryClient.createCombinePaymentRequest.SubOrders, 2)
	require.Equal(t, "1900000109", ordinaryClient.createCombinePaymentRequest.SubOrders[0].MchID)
	require.Equal(t, "190001", ordinaryClient.createCombinePaymentRequest.SubOrders[0].SubMchID)
	require.Equal(t, "PO-11", ordinaryClient.createCombinePaymentRequest.SubOrders[0].OutTradeNo)
	require.Equal(t, int64(3200), ordinaryClient.createCombinePaymentRequest.SubOrders[0].Amount.TotalAmount)
	require.True(t, ordinaryClient.createCombinePaymentRequest.SubOrders[0].SettleInfo.ProfitSharing)
	require.False(t, ordinaryClient.createCombinePaymentRequest.SubOrders[1].SettleInfo.ProfitSharing)
}

func TestCreateCombinedPaymentOrder_OrdinaryServiceProviderRequiresMerchantBaofuReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	svc := NewCombinedPaymentServiceWithOrdinaryServiceProvider(store, ordinaryClient)

	input := CreateCombinedPaymentOrderInput{UserID: 1001, OrderIDs: []int64{11, 22, 11}, ClientIP: "127.0.0.1"}

	store.EXPECT().
		GetOrder(gomock.Any(), int64(11)).
		Times(1).
		Return(db.Order{ID: 11, UserID: input.UserID, MerchantID: 501}, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), int64(22)).
		Times(1).
		Return(db.Order{ID: 22, UserID: input.UserID, MerchantID: 501}, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   501,
		}).
		Times(1).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)

	result, err := svc.CreateCombinedPaymentOrder(context.Background(), input)

	require.Empty(t, result.CombinedPayment.ID)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Equal(t, "商户结算账户未开通，暂不能创建支付订单", reqErr.Err.Error())
	require.Nil(t, ordinaryClient.createCombinePaymentRequest)
}

func TestCreateCombinedPaymentOrder_LogsOrdinaryCleanupFailuresAfterPrepayUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	svc := NewCombinedPaymentServiceWithOrdinaryServiceProvider(store, ordinaryClient)

	input := CreateCombinedPaymentOrderInput{UserID: 1001, OrderIDs: []int64{11}, ClientIP: "127.0.0.1"}
	expectCombinedPaymentMerchantBaofuReady(store, input.UserID, []db.Order{
		{ID: 11, UserID: input.UserID, MerchantID: 501},
	})
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid-ok"}, nil)
	store.EXPECT().CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreateCombinedPaymentTxResult{
		CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 9101, UserID: 1001, CombineOutTradeNo: "CP20260301112233", Status: paymentStatusPending},
		OrderInfos: []db.CombinedPaymentOrderInfo{
			{
				Order:         db.Order{ID: 11, MerchantID: 501},
				PaymentOrder:  db.PaymentOrder{ID: 7201, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, RequiresProfitSharing: true, Amount: 3200, OutTradeNo: "PO-11", Attach: pgtype.Text{String: "attach-11", Valid: true}},
				PaymentConfig: db.MerchantPaymentConfig{SubMchID: "190001"},
				Merchant:      db.Merchant{Name: "merchant-1"},
			},
		},
	}, nil)
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       9101,
		PrepayID: pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
	}).Return(db.CombinedPaymentOrder{}, errors.New("update combined prepay failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), int64(7201)).Return(db.PaymentOrder{}, errors.New("mark child failed"))
	store.EXPECT().UpdateCombinedPaymentOrderToFailed(gomock.Any(), int64(9101)).Return(db.CombinedPaymentOrder{}, errors.New("mark combined failed"))

	_, err := svc.CreateCombinedPaymentOrder(context.Background(), input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "update combined payment prepay")
	require.Contains(t, logs.String(), "failed to mark child payment order failed after combine prepay update failure")
	require.Contains(t, logs.String(), "mark child failed")
	require.Contains(t, logs.String(), "failed to mark combined payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark combined failed")
	require.NotNil(t, ordinaryClient.closeCombinePaymentRequest)
}

func TestCreateCombinedPaymentOrder_ConcurrentPendingCombinedSigningFailureReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := mockwechat.NewMockEcommerceClientInterface(ctrl)
	svc := NewCombinedPaymentService(store, client)

	input := CreateCombinedPaymentOrderInput{
		UserID:   1001,
		OrderIDs: []int64{11, 22},
		ClientIP: "127.0.0.1",
	}

	subOrders, err := json.Marshal([]combinedSubOrderPayload{
		{OrderID: 11, PaymentOrderID: 7001, MerchantID: 501, SubMchID: "190001", Amount: 3200, OutTradeNo: "P11", Description: "m1 - Order Payment"},
		{OrderID: 22, PaymentOrderID: 7002, MerchantID: 502, SubMchID: "190002", Amount: 4800, OutTradeNo: "P22", Description: "m2 - Order Payment"},
	})
	require.NoError(t, err)

	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().
		CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreateCombinedPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 11, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(1).
		Return(db.PaymentOrder{ID: 7001, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 22, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(1).
		Return(db.PaymentOrder{ID: 7002, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(9001)).
		Times(1).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:                9001,
			UserID:            input.UserID,
			CombineOutTradeNo: "CP20260406000001",
			TotalAmount:       8000,
			PrepayID:          pgtype.Text{String: "combine-prepay-9001", Valid: true},
			Status:            paymentStatusPending,
			SubOrders:         subOrders,
		}, nil)
	client.EXPECT().
		GenerateJSAPIPayParams("combine-prepay-9001").
		Times(1).
		Return(nil, errors.New("signing unavailable"))

	_, err = svc.CreateCombinedPaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sign concurrent combined payment order")
	require.Contains(t, err.Error(), "signing unavailable")
}

func TestCreateCombinedPaymentOrder_ConcurrentPendingCombinedWithoutPrepayReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := mockwechat.NewMockEcommerceClientInterface(ctrl)
	svc := NewCombinedPaymentService(store, client)

	input := CreateCombinedPaymentOrderInput{
		UserID:   1001,
		OrderIDs: []int64{11, 22},
		ClientIP: "127.0.0.1",
	}

	subOrders, err := json.Marshal([]combinedSubOrderPayload{
		{OrderID: 11, PaymentOrderID: 7001, MerchantID: 501, SubMchID: "190001", Amount: 3200, OutTradeNo: "P11", Description: "m1 - Order Payment"},
		{OrderID: 22, PaymentOrderID: 7002, MerchantID: 502, SubMchID: "190002", Amount: 4800, OutTradeNo: "P22", Description: "m2 - Order Payment"},
	})
	require.NoError(t, err)

	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().
		CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreateCombinedPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 11, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(outTradeNoMaxRetry).
		Return(db.PaymentOrder{ID: 7001, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: 22, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(outTradeNoMaxRetry).
		Return(db.PaymentOrder{ID: 7002, Status: paymentStatusPending, CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true}}, nil)
	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(9001)).
		Times(outTradeNoMaxRetry).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:                9001,
			UserID:            input.UserID,
			CombineOutTradeNo: "CP20260406000001",
			TotalAmount:       8000,
			Status:            paymentStatusPending,
			SubOrders:         subOrders,
		}, nil)

	_, err = svc.CreateCombinedPaymentOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, "combined payment order is still preparing, please retry", reqErr.Err.Error())
}

func TestGetCombinedPaymentOrder(t *testing.T) {
	input := GetCombinedPaymentOrderInput{UserID: 1001, CombinedPaymentID: 2001}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result GetCombinedPaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ GetCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "combined payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID + 1}, nil)
			},
			check: func(t *testing.T, _ GetCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "combined payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				row := db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID, Status: paymentStatusPending}
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(row, nil)
			},
			check: func(t *testing.T, result GetCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, input.CombinedPaymentID, result.CombinedPayment.ID)
				require.Equal(t, input.UserID, result.CombinedPayment.UserID)
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

			svc := NewCombinedPaymentService(store, nil)
			result, err := svc.GetCombinedPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestGetCombinedPaymentOrder_DoesNotReturnPayParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetCombinedPaymentOrderWithSubOrders(gomock.Any(), int64(2001)).
		Times(1).
		Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
			ID:        2001,
			UserID:    1001,
			Status:    paymentStatusPending,
			PrepayID:  pgtype.Text{String: "combine-prepay-001", Valid: true},
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
		}, nil)

	svc := NewCombinedPaymentService(store, client)
	result, err := svc.GetCombinedPaymentOrder(context.Background(), GetCombinedPaymentOrderInput{
		UserID:            1001,
		CombinedPaymentID: 2001,
	})
	require.NoError(t, err)
	require.Nil(t, result.PayParams)
}

func TestQueryCombinedPaymentOrder(t *testing.T) {
	input := QueryCombinedPaymentOrderInput{UserID: 1001, CombinedPaymentID: 2001}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface)
		withClient bool
		check      func(t *testing.T, result QueryCombinedPaymentOrderResult, err error)
	}{
		{
			name:       "ClientNotConfigured",
			withClient: false,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {},
			check: func(t *testing.T, _ QueryCombinedPaymentOrderResult, err error) {
				require.EqualError(t, err, "ecommerce client: not configured")
			},
		},
		{
			name:       "NotFound",
			withClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ QueryCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
			},
		},
		{
			name:       "SuccessPendingRemoteStateReturnsPayParams",
			withClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
						ID:                input.CombinedPaymentID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						CombineOutTradeNo: "CP20260406000001",
						PrepayID:          pgtype.Text{String: "combine-prepay-2001", Valid: true},
						ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
					}, nil)
				client.EXPECT().
					GenerateJSAPIPayParams("combine-prepay-2001").
					Times(1).
					Return(&wechat.JSAPIPayParams{Package: "prepay_id=combine-prepay-2001"}, nil)
				client.EXPECT().
					QueryCombineOrder(gomock.Any(), "CP20260406000001").
					Times(1).
					Return(&wechatcontracts.CombineQueryResponse{
						CombineOutTradeNo: "CP20260406000001",
						SubOrders: []wechatcontracts.CombineSubOrderResult{
							{
								OutTradeNo:    "PO-11",
								TransactionID: "wx-txn-11",
								TradeType:     "JSAPI",
								TradeState:    "NOTPAY",
								Amount: struct {
									TotalAmount    int64  `json:"total_amount"`
									PayerAmount    int64  `json:"payer_amount"`
									Currency       string `json:"currency"`
									PayerCurrency  string `json:"payer_currency"`
									SettlementRate int64  `json:"settlement_rate"`
								}{TotalAmount: 100, PayerAmount: 100, Currency: "CNY", PayerCurrency: "CNY"},
							},
						},
					}, nil)
			},
			check: func(t *testing.T, result QueryCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.PayParams)
				require.Equal(t, "prepay_id=combine-prepay-2001", result.PayParams.Package)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "pending", result.WechatOrder.AggregateTradeState)
				require.Len(t, result.WechatOrder.SubOrders, 1)
				require.Equal(t, "PO-11", result.WechatOrder.SubOrders[0].OutTradeNo)
			},
		},
		{
			name:       "SignExistingPayParamsFailureReturnsError",
			withClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
						ID:                input.CombinedPaymentID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						CombineOutTradeNo: "CP20260406000001",
						PrepayID:          pgtype.Text{String: "combine-prepay-2001", Valid: true},
						ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
					}, nil)
				client.EXPECT().
					QueryCombineOrder(gomock.Any(), "CP20260406000001").
					Times(1).
					Return(&wechatcontracts.CombineQueryResponse{
						CombineOutTradeNo: "CP20260406000001",
						SubOrders: []wechatcontracts.CombineSubOrderResult{{
							OutTradeNo: "PO-11",
							TradeType:  "JSAPI",
							TradeState: "NOTPAY",
							Amount: struct {
								TotalAmount    int64  `json:"total_amount"`
								PayerAmount    int64  `json:"payer_amount"`
								Currency       string `json:"currency"`
								PayerCurrency  string `json:"payer_currency"`
								SettlementRate int64  `json:"settlement_rate"`
							}{TotalAmount: 100, PayerAmount: 100, Currency: "CNY", PayerCurrency: "CNY"},
						}},
					}, nil)
				client.EXPECT().
					GenerateJSAPIPayParams("combine-prepay-2001").
					Times(1).
					Return(nil, errors.New("sign failed"))
			},
			check: func(t *testing.T, result QueryCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "sign combined payment order")
				require.ErrorContains(t, err, "sign failed")
				require.Nil(t, result.PayParams)
			},
		},
		{
			name:       "RemotePaidSuppressesPayParams",
			withClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
						ID:                input.CombinedPaymentID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						CombineOutTradeNo: "CP20260406000001",
						PrepayID:          pgtype.Text{String: "combine-prepay-2001", Valid: true},
						ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
					}, nil)
				client.EXPECT().
					QueryCombineOrder(gomock.Any(), "CP20260406000001").
					Times(1).
					Return(&wechatcontracts.CombineQueryResponse{
						CombineOutTradeNo: "CP20260406000001",
						SubOrders: []wechatcontracts.CombineSubOrderResult{{
							OutTradeNo:    "PO-11",
							TransactionID: "wx-txn-11",
							TradeType:     "JSAPI",
							TradeState:    "SUCCESS",
							Amount: struct {
								TotalAmount    int64  `json:"total_amount"`
								PayerAmount    int64  `json:"payer_amount"`
								Currency       string `json:"currency"`
								PayerCurrency  string `json:"payer_currency"`
								SettlementRate int64  `json:"settlement_rate"`
							}{TotalAmount: 100, PayerAmount: 100, Currency: "CNY", PayerCurrency: "CNY"},
						}},
					}, nil)
			},
			check: func(t *testing.T, result QueryCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Nil(t, result.PayParams)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "paid", result.WechatOrder.AggregateTradeState)
			},
		},
		{
			name:       "MixedSuccessAndPendingIsPartial",
			withClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
						ID:                input.CombinedPaymentID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						CombineOutTradeNo: "CP20260406000001",
						PrepayID:          pgtype.Text{String: "combine-prepay-2001", Valid: true},
						ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
					}, nil)
				client.EXPECT().
					QueryCombineOrder(gomock.Any(), "CP20260406000001").
					Times(1).
					Return(&wechatcontracts.CombineQueryResponse{
						CombineOutTradeNo: "CP20260406000001",
						SubOrders: []wechatcontracts.CombineSubOrderResult{
							{
								OutTradeNo:    "PO-11",
								TransactionID: "wx-txn-11",
								TradeType:     "JSAPI",
								TradeState:    "SUCCESS",
								Amount: struct {
									TotalAmount    int64  `json:"total_amount"`
									PayerAmount    int64  `json:"payer_amount"`
									Currency       string `json:"currency"`
									PayerCurrency  string `json:"payer_currency"`
									SettlementRate int64  `json:"settlement_rate"`
								}{TotalAmount: 100, PayerAmount: 100, Currency: "CNY", PayerCurrency: "CNY"},
							},
							{
								OutTradeNo: "PO-12",
								TradeType:  "JSAPI",
								TradeState: "NOTPAY",
								Amount: struct {
									TotalAmount    int64  `json:"total_amount"`
									PayerAmount    int64  `json:"payer_amount"`
									Currency       string `json:"currency"`
									PayerCurrency  string `json:"payer_currency"`
									SettlementRate int64  `json:"settlement_rate"`
								}{TotalAmount: 100, PayerAmount: 0, Currency: "CNY", PayerCurrency: "CNY"},
							},
						},
					}, nil)
			},
			check: func(t *testing.T, result QueryCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Nil(t, result.PayParams)
				require.NotNil(t, result.WechatOrder)
				require.Equal(t, "partial", result.WechatOrder.AggregateTradeState)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			client := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, client)

			var ecommerceClient wechat.EcommerceClientInterface
			if tc.withClient {
				ecommerceClient = client
			}

			svc := NewCombinedPaymentService(store, ecommerceClient)
			result, err := svc.QueryCombinedPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestCloseCombinedPaymentOrder(t *testing.T) {
	input := CloseCombinedPaymentOrderInput{UserID: 1002, CombinedPaymentID: 2002}

	buildSubOrdersJSON := func() []byte {
		subs := []map[string]any{
			{
				"order_id":         int64(11),
				"payment_order_id": int64(22),
				"merchant_id":      int64(33),
				"sub_mch_id":       "1900001111",
				"amount":           int64(5000),
				"out_trade_no":     "P202001010000000001",
				"description":      "test-sub-order",
			},
		}
		payload, err := json.Marshal(subs)
		require.NoError(t, err)
		return payload
	}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface)
		check      func(t *testing.T, result CloseCombinedPaymentOrderResult, err error)
	}{
		{
			name:       "ClientNotConfigured",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				require.EqualError(t, err, "ecommerce client: not configured")
			},
		},
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "combined payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID + 10, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "combined payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID, Status: "paid"}, nil)
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only pending combined payment orders can be closed", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidSubOrdersJSON",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID, Status: paymentStatusPending, CombineOutTradeNo: "C202001010000000001", SubOrders: []byte("{")}, nil)
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unexpected end of JSON input")
			},
		},
		{
			name: "NoSubOrders",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: input.CombinedPaymentID, UserID: input.UserID, Status: paymentStatusPending, CombineOutTradeNo: "C202001010000000001", SubOrders: []byte(`[{"sub_mch_id":"","out_trade_no":""}]`)}, nil)
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "no sub orders available to close", reqErr.Err.Error())
			},
		},
		{
			name: "CloseCombineOrderError",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
					ID:                input.CombinedPaymentID,
					UserID:            input.UserID,
					Status:            paymentStatusPending,
					CombineOutTradeNo: "C202001010000000001",
					SubOrders:         buildSubOrdersJSON(),
				}
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(combinedRow, nil)

				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "C202001010000000001", []wechatcontracts.SubOrderClose{{SubMchID: "1900001111", OutTradeNo: "P202001010000000001"}}).
					Times(1).
					Return(errors.New("wechat close failed"))
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "wechat close failed")
			},
		},
		{
			name: "UpdateCombinedClosedError",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
					ID:                input.CombinedPaymentID,
					UserID:            input.UserID,
					Status:            paymentStatusPending,
					CombineOutTradeNo: "C202001010000000001",
					SubOrders:         buildSubOrdersJSON(),
				}
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(combinedRow, nil)

				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "C202001010000000001", []wechatcontracts.SubOrderClose{{SubMchID: "1900001111", OutTradeNo: "P202001010000000001"}}).
					Times(1).
					Return(nil)

				store.EXPECT().
					CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
						CombinedPaymentOrderID: input.CombinedPaymentID,
						SubOrderOutTradeNos:    []string{"P202001010000000001"},
					}).
					Times(1).
					Return(db.CloseCombinedPaymentOrderTxResult{}, errors.New("update combined failed"))
			},
			check: func(t *testing.T, _ CloseCombinedPaymentOrderResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "update combined failed")
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
					ID:                input.CombinedPaymentID,
					UserID:            input.UserID,
					Status:            paymentStatusPending,
					CombineOutTradeNo: "C202001010000000001",
					SubOrders:         buildSubOrdersJSON(),
				}
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(combinedRow, nil)

				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "C202001010000000001", []wechatcontracts.SubOrderClose{{SubMchID: "1900001111", OutTradeNo: "P202001010000000001"}}).
					Times(1).
					Return(nil)

				store.EXPECT().
					CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
						CombinedPaymentOrderID: input.CombinedPaymentID,
						SubOrderOutTradeNos:    []string{"P202001010000000001"},
					}).
					Times(1).
					Return(db.CloseCombinedPaymentOrderTxResult{
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: input.CombinedPaymentID, Status: "closed", PrepayID: pgtype.Text{}},
					}, nil)
			},
			check: func(t *testing.T, result CloseCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(input.CombinedPaymentID), result.CombinedPayment.ID)
				require.Equal(t, "closed", result.CombinedPayment.Status)
				require.Len(t, result.SubOrders, 1)
				require.Equal(t, "P202001010000000001", result.SubOrders[0].OutTradeNo)
			},
		},
		{
			name: "Success_IgnoreGetPaymentOrderError",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
					ID:                input.CombinedPaymentID,
					UserID:            input.UserID,
					Status:            paymentStatusPending,
					CombineOutTradeNo: "C202001010000000001",
					SubOrders:         buildSubOrdersJSON(),
				}
				store.EXPECT().
					GetCombinedPaymentOrderWithSubOrders(gomock.Any(), input.CombinedPaymentID).
					Times(1).
					Return(combinedRow, nil)

				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "C202001010000000001", []wechatcontracts.SubOrderClose{{SubMchID: "1900001111", OutTradeNo: "P202001010000000001"}}).
					Times(1).
					Return(nil)

				store.EXPECT().
					CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
						CombinedPaymentOrderID: input.CombinedPaymentID,
						SubOrderOutTradeNos:    []string{"P202001010000000001"},
					}).
					Times(1).
					Return(db.CloseCombinedPaymentOrderTxResult{
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: input.CombinedPaymentID, Status: "closed", PrepayID: pgtype.Text{}},
					}, nil)
			},
			check: func(t *testing.T, result CloseCombinedPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(input.CombinedPaymentID), result.CombinedPayment.ID)
				require.Equal(t, "closed", result.CombinedPayment.Status)
				require.Len(t, result.SubOrders, 1)
				require.Equal(t, "P202001010000000001", result.SubOrders[0].OutTradeNo)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			client := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, client)

			var ecommerceClient wechat.EcommerceClientInterface
			if tc.name != "ClientNotConfigured" {
				ecommerceClient = client
			}

			svc := NewCombinedPaymentService(store, ecommerceClient)
			result, err := svc.CloseCombinedPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestQueryCombinedPaymentOrder_UsesOrdinaryServiceProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{
		queryCombinePaymentResponse: &ospcontracts.CombineQueryResponse{
			CombineAppID:      "wxsp_app",
			CombineMchID:      "1900000109",
			CombineOutTradeNo: "C202605010001",
			TradeState:        ospcontracts.PaymentTradeStateNotPay,
			SubOrders: []ospcontracts.CombineOrderState{
				{SubMchID: "sub-1", OutTradeNo: "P-1", TradeState: ospcontracts.PaymentTradeStateNotPay},
			},
		},
	}
	svc := NewCombinedPaymentServiceWithOrdinaryServiceProvider(store, ordinaryClient)
	svc.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }
	combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
		ID:                2002,
		UserID:            1002,
		Status:            paymentStatusPending,
		CombineOutTradeNo: "C202605010001",
		PrepayID:          pgtype.Text{String: "prepay-combine-ordinary", Valid: true},
		ExpiresAt:         pgtype.Timestamptz{Time: time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC), Valid: true},
		SubOrders:         []byte(`[{}]`),
	}
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedRow.ID).Return(combinedRow, nil)

	result, err := svc.QueryCombinedPaymentOrder(context.Background(), QueryCombinedPaymentOrderInput{UserID: combinedRow.UserID, CombinedPaymentID: combinedRow.ID})

	require.NoError(t, err)
	require.NotNil(t, ordinaryClient.queryCombinePaymentRequest)
	require.Equal(t, "1900000109", ordinaryClient.queryCombinePaymentRequest.CombineMchID)
	require.Equal(t, "C202605010001", ordinaryClient.queryCombinePaymentRequest.CombineOutTradeNo)
	require.Equal(t, "pending", result.WechatOrder.AggregateTradeState)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay-combine-ordinary", result.PayParams.Package)
}

func TestCloseCombinedPaymentOrder_UsesOrdinaryServiceProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	svc := NewCombinedPaymentServiceWithOrdinaryServiceProvider(store, ordinaryClient)
	combinedRow := db.GetCombinedPaymentOrderWithSubOrdersRow{
		ID:                2002,
		UserID:            1002,
		Status:            paymentStatusPending,
		CombineOutTradeNo: "C202605010001",
		SubOrders:         []byte(`[{"order_id":11,"payment_order_id":22,"merchant_id":33,"sub_mch_id":"sub-1","amount":5000,"out_trade_no":"P-1","description":"test-sub-order"}]`),
	}
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedRow.ID).Return(combinedRow, nil)
	store.EXPECT().CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
		CombinedPaymentOrderID: combinedRow.ID,
		SubOrderOutTradeNos:    []string{"P-1"},
	}).Return(db.CloseCombinedPaymentOrderTxResult{
		CombinedPaymentOrder: db.CombinedPaymentOrder{ID: combinedRow.ID, Status: "closed"},
	}, nil)

	result, err := svc.CloseCombinedPaymentOrder(context.Background(), CloseCombinedPaymentOrderInput{UserID: combinedRow.UserID, CombinedPaymentID: combinedRow.ID})

	require.NoError(t, err)
	require.Equal(t, "closed", result.CombinedPayment.Status)
	require.NotNil(t, ordinaryClient.closeCombinePaymentRequest)
	require.Equal(t, "wxsp_app", ordinaryClient.closeCombinePaymentRequest.CombineAppID)
	require.Equal(t, "1900000109", ordinaryClient.closeCombinePaymentRequest.CombineMchID)
	require.Equal(t, "C202605010001", ordinaryClient.closeCombinePaymentRequest.CombineOutTradeNo)
	require.Len(t, ordinaryClient.closeCombinePaymentRequest.SubOrders, 1)
	require.Equal(t, "1900000109", ordinaryClient.closeCombinePaymentRequest.SubOrders[0].MchID)
	require.Equal(t, "sub-1", ordinaryClient.closeCombinePaymentRequest.SubOrders[0].SubMchID)
	require.Equal(t, "P-1", ordinaryClient.closeCombinePaymentRequest.SubOrders[0].OutTradeNo)
}

func TestDedupePositiveIDs(t *testing.T) {
	testCases := []struct {
		name     string
		input    []int64
		expected []int64
	}{
		{
			name:     "NilInput",
			input:    nil,
			expected: []int64{},
		},
		{
			name:     "FilterNonPositiveAndDedupe",
			input:    []int64{0, -1, 5, 5, 3, 0, 3, 2},
			expected: []int64{5, 3, 2},
		},
		{
			name:     "KeepFirstOccurrenceOrder",
			input:    []int64{9, 8, 9, 7, 8, 6},
			expected: []int64{9, 8, 7, 6},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := dedupePositiveIDs(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestMapCombinedPaymentError(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectReqError bool
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "BelongToUser",
			err:            errors.New("order 100 does not belong to user"),
			expectReqError: true,
			expectedStatus: 403,
			expectedMsg:    "订单不属于当前用户",
		},
		{
			name:           "InvalidStatus",
			err:            errors.New("order 100 status is paid, expect pending"),
			expectReqError: true,
			expectedStatus: 400,
			expectedMsg:    "订单已不在待支付状态，请刷新页面确认",
		},
		{
			name:           "InvalidPaymentConfig",
			err:            errors.New("merchant 9 payment config invalid"),
			expectReqError: true,
			expectedStatus: 400,
			expectedMsg:    "商户支付配置无效，请联系平台处理",
		},
		{
			name:           "ActivePaymentOrder",
			err:            errors.New("order 100 has processing payment order"),
			expectReqError: true,
			expectedStatus: 400,
			expectedMsg:    "订单已有进行中的支付单，请先刷新支付结果",
		},
		{
			name:           "Unmapped",
			err:            errors.New("database timeout"),
			expectReqError: false,
		},
		{
			name:           "Nil",
			err:            nil,
			expectReqError: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mapped := mapCombinedPaymentError(tc.err)
			if !tc.expectReqError {
				require.Nil(t, mapped)
				return
			}

			require.Error(t, mapped)
			reqErr := assertRequestError(t, mapped)
			require.Equal(t, tc.expectedStatus, reqErr.Status)
			require.Equal(t, tc.expectedMsg, reqErr.Err.Error())
		})
	}
}

func TestMapCombineOrderQueryError(t *testing.T) {
	err := mapCombineOrderQueryError(&wechat.WechatPayError{StatusCode: 404, Code: "ORDERNOTEXIST", Message: "订单不存在"})
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.Equal(t, "微信侧暂未确认该支付单，请保留当前订单并稍后刷新结果", reqErr.Err.Error())
}

func TestMapCombineOrderQueryError_ContractDrift(t *testing.T) {
	err := mapCombineOrderQueryError(&wechat.CombineOrderQueryContractError{Message: "query combine order: wechat response missing combine_mchid"})
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.Equal(t, "微信支付状态返回异常，请不要重复支付，返回订单页后重新查询", reqErr.Err.Error())
}

func TestMapCombineOrderCloseError(t *testing.T) {
	err := mapCombineOrderCloseError(&wechat.WechatPayError{StatusCode: 202, Code: "USERPAYING", Message: "用户支付中"})
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, "支付处理中，请先刷新支付结果确认后再决定是否关闭", reqErr.Err.Error())
}
