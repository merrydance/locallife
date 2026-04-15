package logic

import (
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

func TestCreateCombinedPaymentOrder_ConcurrentPendingCombinedWithoutPayParamsReturnsConflict(t *testing.T) {
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
			PrepayID:          pgtype.Text{String: "combine-prepay-9001", Valid: true},
			Status:            paymentStatusPending,
			SubOrders:         subOrders,
		}, nil)
	client.EXPECT().
		GenerateJSAPIPayParams("combine-prepay-9001").
		Times(outTradeNoMaxRetry).
		Return(nil, errors.New("signing unavailable"))

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
