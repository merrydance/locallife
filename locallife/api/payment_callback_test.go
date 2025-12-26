package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var ErrInvalidSignature = wechat.ErrInvalidSignature

// TestHandlePaymentNotifyIdempotency 测试支付回调的幂等性检查
func TestHandlePaymentNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				// 先验证签名（必须通过）
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)

				// 不应该调用解密方法（因为幂等性检查发现已处理）
				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "首次通知_验证签名失败",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				// 签名验证失败（先于幂等性检查）
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(ErrInvalidSignature)

				// 签名失败直接返回，不会检查幂等性
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "invalid_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			// 创建测试服务器
			server := newTestServerWithPaymentClient(t, store, paymentClient)
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestHandleRefundNotifyIdempotency 测试退款回调的幂等性检查
func TestHandleRefundNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复退款通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				// 先验证签名
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "REFUND.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "refund",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/refund-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			server := newTestServerWithPaymentClient(t, store, paymentClient)
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestHandleCombinePaymentNotifyIdempotency 测试平台收付通合单支付回调的幂等性检查
func TestHandleCombinePaymentNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复合单支付通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 先验证签名
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestHandleEcommerceRefundNotifyIdempotency 测试平台收付通退款回调的幂等性检查
func TestHandleEcommerceRefundNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复平台收付通退款通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 先验证签名
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "REFUND.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "refund",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/refund-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// newTestServerWithPaymentClient 创建带Mock PaymentClient的测试服务器
func newTestServerWithPaymentClient(t *testing.T, store db.Store, paymentClient *mockwechat.MockPaymentClientInterface) *Server {
	server := newTestServer(t, store)

	// 替换为Mock客户端
	server.paymentClient = paymentClient

	return server
}

// newTestServerWithEcommerceClient 创建带Mock EcommerceClient的测试服务器
func newTestServerWithEcommerceClient(t *testing.T, store db.Store, ecommerceClient *mockwechat.MockEcommerceClientInterface) *Server {
	server := newTestServer(t, store)

	// 替换为Mock客户端
	server.ecommerceClient = ecommerceClient

	return server
}

// TestHandlePaymentNotifyFullFlow 测试支付回调完整业务流程
func TestHandlePaymentNotifyFullFlow(t *testing.T) {
	notificationID := util.RandomString(32)
	outTradeNo := "TEST_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)
	amount := int64(10000) // 100元

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "支付成功_完整流程",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				// 1. 签名验证通过
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 2. 幂等性检查：首次处理
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(false, nil)

				// 3. 解密通知
				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechat.PaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: struct {
							Total         int64  `json:"total"`
							PayerTotal    int64  `json:"payer_total"`
							Currency      string `json:"currency"`
							PayerCurrency string `json:"payer_currency"`
						}{
							Total: amount,
						},
					}, nil)

				// 4. 查询支付订单
				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       amount,
						Status:       "pending",
						UserID:       100,
						BusinessType: "order",
					}, nil)

				// 5. 更新支付订单状态为已支付
				store.EXPECT().
					UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       amount,
						Status:       "paid",
						UserID:       100,
						BusinessType: "order",
					}, nil)

				// 6. 记录通知ID
				store.EXPECT().
					CreateWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.WechatNotification{}, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "金额不匹配_返回SUCCESS避免重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				// 解密后金额为20000（200元）
				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechat.PaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: struct {
							Total         int64  `json:"total"`
							PayerTotal    int64  `json:"payer_total"`
							Currency      string `json:"currency"`
							PayerCurrency string `json:"payer_currency"`
						}{
							Total: 20000, // 200元，与订单金额不匹配
						},
					}, nil)

				// 订单金额为10000（100元）
				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       10000, // 100元
						Status:       "pending",
						UserID:       100,
						BusinessType: "order",
					}, nil)

				// 金额不匹配，不应更新状态
				store.EXPECT().
					UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// 金额不匹配返回SUCCESS避免微信重试，但需要人工审核
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
				require.Contains(t, response["message"], "amount mismatch")
			},
		},
		{
			name: "订单已支付_幂等返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechat.PaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: struct {
							Total         int64  `json:"total"`
							PayerTotal    int64  `json:"payer_total"`
							Currency      string `json:"currency"`
							PayerCurrency string `json:"payer_currency"`
						}{
							Total: amount,
						},
					}, nil)

				// 订单状态已经是paid
				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       amount,
						Status:       "paid", // 已支付
						UserID:       100,
						BusinessType: "order",
					}, nil)

				// 不应再次更新
				store.EXPECT().
					UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "非SUCCESS事件类型_忽略处理",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 非SUCCESS事件不检查幂等性
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.CLOSED", // 非SUCCESS
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "无PaymentClient_返回500",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				// 不设置任何mock，测试nil client场景
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":         notificationID,
					"event_type": "TRANSACTION.SUCCESS",
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)

			var server *Server
			if tc.name == "无PaymentClient_返回500" {
				// 特殊处理：不设置paymentClient
				server = newTestServer(t, store)
			} else {
				server = newTestServerWithPaymentClient(t, store, paymentClient)
			}
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestHandleApplymentStateNotifyIdempotency 测试进件回调的幂等性检查
func TestHandleApplymentStateNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复进件通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 先验证签名
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "APPLYMENT_STATE.CHANGE",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "applyment",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/applyment-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "非进件事件类型_忽略处理",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 非进件事件不检查幂等性
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Any()).
					Times(0)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS", // 非进件事件
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "transaction",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/applyment-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestHandleProfitSharingNotifyIdempotency 测试分账回调的幂等性检查
func TestHandleProfitSharingNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)
	outOrderNo := util.RandomString(16)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复分账通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 先验证签名
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：通知ID已存在
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(true, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "profit_sharing",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/profit-sharing-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "分账成功通知_更新订单状态为finished",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// 幂等性检查：新通知
				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(false, nil)

				// 解密通知
				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechat.ProfitSharingNotification{
						MchID:      "sp_mch_id",
						SubMchID:   "sub_mch_id",
						OutOrderNo: outOrderNo,
						OrderID:    "wx_order_id",
						Receiver: struct {
							Type            string `json:"type"`
							ReceiverAccount string `json:"receiver_account"`
							Amount          int64  `json:"amount"`
							Description     string `json:"description"`
							Result          string `json:"result"`
							DetailID        string `json:"detail_id"`
							FinishTime      string `json:"finish_time"`
							FailReason      string `json:"fail_reason"`
						}{
							Result:     "SUCCESS",
							FailReason: "",
							Amount:     100,
						},
					}, nil)

				// 查询分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         1,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				// 更新为成功
				store.EXPECT().
					UpdateProfitSharingOrderToFinished(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(db.ProfitSharingOrder{ID: 1, Status: "finished"}, nil)

				// 记录通知
				store.EXPECT().
					CreateWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.WechatNotification{ID: notificationID}, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "profit_sharing",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/profit-sharing-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "分账失败通知_更新订单状态为failed",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					CheckNotificationExists(gomock.Any(), gomock.Eq(notificationID)).
					Times(1).
					Return(false, nil)

				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechat.ProfitSharingNotification{
						MchID:      "sp_mch_id",
						SubMchID:   "sub_mch_id",
						OutOrderNo: outOrderNo,
						OrderID:    "wx_order_id",
						Receiver: struct {
							Type            string `json:"type"`
							ReceiverAccount string `json:"receiver_account"`
							Amount          int64  `json:"amount"`
							Description     string `json:"description"`
							Result          string `json:"result"`
							DetailID        string `json:"detail_id"`
							FinishTime      string `json:"finish_time"`
							FailReason      string `json:"fail_reason"`
						}{
							Result:     "CLOSED",
							FailReason: "NO_RELATION",
							Amount:     100,
						},
					}, nil)

				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         1,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				// 更新为失败
				store.EXPECT().
					UpdateProfitSharingOrderToFailed(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(db.ProfitSharingOrder{ID: 1, Status: "failed"}, nil)

				store.EXPECT().
					CreateWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.WechatNotification{ID: notificationID}, nil)
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":            notificationID,
					"event_type":    "TRANSACTION.SUCCESS",
					"resource_type": "encrypt-resource",
					"resource": map[string]interface{}{
						"algorithm":       "AEAD_AES_256_GCM",
						"ciphertext":      "mock_encrypted_data",
						"nonce":           "mock_nonce",
						"associated_data": "profit_sharing",
					},
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/profit-sharing-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				request.Header.Set("Wechatpay-Timestamp", "1234567890")
				request.Header.Set("Wechatpay-Nonce", "test_nonce")
				request.Header.Set("Wechatpay-Signature", "test_signature")
				request.Header.Set("Wechatpay-Serial", "test_serial")

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "SUCCESS", response["code"])
			},
		},
		{
			name: "无EcommerceClient_返回500",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 不设置任何mock
			},
			setupRequest: func(t *testing.T) *http.Request {
				requestBody := map[string]interface{}{
					"id":         notificationID,
					"event_type": "TRANSACTION.SUCCESS",
				}
				bodyBytes, err := json.Marshal(requestBody)
				require.NoError(t, err)

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/profit-sharing-notify", bytes.NewReader(bodyBytes))
				require.NoError(t, err)

				return request
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			var server *Server
			if tc.name == "无EcommerceClient_返回500" {
				server = newTestServer(t, store)
			} else {
				server = newTestServerWithEcommerceClient(t, store, ecommerceClient)
			}
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}