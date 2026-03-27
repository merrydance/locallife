package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{
						ID:          notificationID,
						ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
					}, nil)

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
					Return(wechat.ErrInvalidSignature)

				// 签名失败直接返回，不会检查幂等性
				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
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
		{
			name: "重复通知_查询状态失败返回FAIL",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{}, errors.New("lookup failed"))

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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				assertWechatFailResponse(t, recorder, "notification status lookup failed")
			},
		},
		{
			name: "重复通知_处理中返回FAIL",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{
						ID:        notificationID,
						CreatedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
					}, nil)

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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				assertWechatFailResponse(t, recorder, "notification in processing")
			},
		},
		{
			name: "重复通知_stale claim返回FAIL并释放占位",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{
						ID:        notificationID,
						CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-notificationClaimStaleWindow - time.Second), Valid: true},
					}, nil)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(nil)

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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				assertWechatFailResponse(t, recorder, "stale claim, please retry")
			},
		},
		{
			name: "重复通知_stale claim释放失败仍返回FAIL",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{
						ID:        notificationID,
						CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-notificationClaimStaleWindow - time.Second), Valid: true},
					}, nil)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(errors.New("release failed"))

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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				assertWechatFailResponse(t, recorder, "stale claim, please retry")
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)
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

func TestHandleRefundNotifyOwnershipMismatch(t *testing.T) {
	notificationID := util.RandomString(32)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	paymentClient.EXPECT().
		DecryptRefundNotification(gomock.Any()).
		Times(1).
		Return(&wechat.RefundNotificationResource{
			MchID:        "mch_wrong",
			OutTradeNo:   "ORDER_REFUND_1",
			OutRefundNo:  "REFUND_1",
			RefundID:     "WX_REFUND_1",
			RefundStatus: "SUCCESS",
		}, nil)

	paymentClient.EXPECT().
		GetMchID().
		Times(1).
		Return("mch_expected")

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Times(1).
		Return(nil)

	server := newTestServerWithPaymentClient(t, store, paymentClient)
	recorder := httptest.NewRecorder()

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

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	var response map[string]string
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "FAIL", response["code"])
	require.Contains(t, response["message"], "ownership validation failed")
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)
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

func TestHandleCombinePaymentNotify_ClosedOrderEnqueuesAnomalyRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			SubOrders: []wechat.CombineSubOrderResult{
				{
					OutTradeNo:    outTradeNo,
					TransactionID: transactionID,
					TradeState:    "SUCCESS",
					Amount: struct {
						TotalAmount int64  `json:"total_amount"`
						PayerAmount int64  `json:"payer_amount"`
						Currency    string `json:"currency"`
					}{
						TotalAmount: 10000,
						PayerAmount: 10000,
						Currency:    "CNY",
					},
				},
			},
		}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:         11,
			OutTradeNo: outTradeNo,
			Amount:     10000,
			Status:     PaymentStatusClosed,
		}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessAnomalyRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessAnomalyRefund{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PayloadProcessAnomalyRefund, _ ...asynq.Option) error {
			require.Equal(t, int64(11), payload.PaymentOrderID)
			require.Equal(t, transactionID, payload.TransactionID)
			require.Equal(t, int64(10000), payload.RefundAmount)
			require.Equal(t, "CRF11", payload.OutRefundNo)
			return nil
		})

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleCombinePaymentNotify_OwnershipMismatchReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			CombineMchID:      "sp_wrong",
			CombineAppID:      "app_wrong",
		}, nil)

	ecommerceClient.EXPECT().
		GetSpMchID().
		Times(1).
		Return("sp_expected")

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Times(1).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "ownership validation failed")
}

func TestHandleCombinePaymentNotify_SubOrderNotFoundReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			SubOrders: []wechat.CombineSubOrderResult{{
				OutTradeNo:    outTradeNo,
				TransactionID: transactionID,
				TradeState:    "SUCCESS",
				Amount: struct {
					TotalAmount int64  `json:"total_amount"`
					PayerAmount int64  `json:"payer_amount"`
					Currency    string `json:"currency"`
				}{
					TotalAmount: 10000,
					PayerAmount: 10000,
					Currency:    "CNY",
				},
			}},
		}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "1 orders failed")
}

func TestHandleCombinePaymentNotify_AmountMismatchEnqueuesRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			SubOrders: []wechat.CombineSubOrderResult{
				{
					OutTradeNo:    outTradeNo,
					TransactionID: transactionID,
					TradeState:    "SUCCESS",
					Amount: struct {
						TotalAmount int64  `json:"total_amount"`
						PayerAmount int64  `json:"payer_amount"`
						Currency    string `json:"currency"`
					}{
						TotalAmount: 12000,
						PayerAmount: 12000,
						Currency:    "CNY",
					},
				},
			},
		}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           12,
			OutTradeNo:   outTradeNo,
			Amount:       10000,
			Status:       PaymentStatusPending,
			BusinessType: "order",
			OrderID:      pgtype.Int8{Int64: 88, Valid: true},
		}, nil)

	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Return(db.RefundOrder{ID: 1201, PaymentOrderID: 12, Status: "pending", OutRefundNo: "RF12_88"}, nil)

	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{ID: 12, Status: PaymentStatusPaid}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessRefund{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PayloadProcessRefund, _ ...asynq.Option) error {
			require.Equal(t, int64(12), payload.PaymentOrderID)
			require.Equal(t, int64(88), payload.OrderID)
			require.Equal(t, int64(12000), payload.RefundAmount)
			require.Contains(t, payload.Reason, "金额异常")
			return nil
		})

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleCombinePaymentNotify_MainOrderNotFoundReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			SubOrders: []wechat.CombineSubOrderResult{{
				OutTradeNo:    outTradeNo,
				TransactionID: transactionID,
				TradeState:    "SUCCESS",
				Amount: struct {
					TotalAmount int64  `json:"total_amount"`
					PayerAmount int64  `json:"payer_amount"`
					Currency    string `json:"currency"`
				}{
					TotalAmount: 10000,
					PayerAmount: 10000,
					Currency:    "CNY",
				},
			}},
		}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           13,
			OutTradeNo:   outTradeNo,
			Amount:       10000,
			Status:       PaymentStatusPending,
			BusinessType: "order",
		}, nil)

	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{ID: 13, Status: PaymentStatusPaid}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(gomock.Any(), gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{}, db.ErrRecordNotFound)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "get combined payment order failed")
}

func TestHandleCombinePaymentNotify_PaymentSuccessEnqueueFailureStillReturnsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechat.CombinePaymentNotification{
			CombineOutTradeNo: combineOutTradeNo,
			SubOrders: []wechat.CombineSubOrderResult{
				{
					OutTradeNo:    outTradeNo,
					TransactionID: transactionID,
					TradeState:    "SUCCESS",
					Amount: struct {
						TotalAmount int64  `json:"total_amount"`
						PayerAmount int64  `json:"payer_amount"`
						Currency    string `json:"currency"`
					}{
						TotalAmount: 10000,
						PayerAmount: 10000,
						Currency:    "CNY",
					},
				},
			},
		}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           13,
			OutTradeNo:   outTradeNo,
			Amount:       10000,
			Status:       PaymentStatusPending,
			BusinessType: "order",
		}, nil)

	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{ID: 13, Status: PaymentStatusPaid}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(gomock.Any(), gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PaymentSuccessPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(13), payload.PaymentOrderID)
			require.Equal(t, transactionID, payload.TransactionID)
			require.Equal(t, "order", payload.BusinessType)
			return errors.New("queue down")
		})

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 101, CombineOutTradeNo: combineOutTradeNo}, nil)

	store.EXPECT().
		UpdateCombinedPaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.CombinedPaymentOrder{ID: 101, Status: PaymentStatusPaid}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleOrderSettlementNotify_ProfitSharingEnqueueFailureStillReturnsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	merchantTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptSettlementNotification(gomock.Any()).
		Return(&wechat.SettlementNotificationResource{
			TransactionID:        transactionID,
			MerchantTradeNo:      merchantTradeNo,
			ConfirmReceiveMethod: 1,
			SettlementTime:       "2026-03-24T12:00:00+08:00",
		}, nil)

	store.EXPECT().
		GetCombinedPaymentSubOrderByOutTradeNo(gomock.Any(), merchantTradeNo).
		Return(db.CombinedPaymentSubOrder{OutTradeNo: merchantTradeNo, OrderID: 77}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), merchantTradeNo).
		Return(db.PaymentOrder{
			ID:          21,
			OutTradeNo:  merchantTradeNo,
			Status:      PaymentStatusPaid,
			PaymentType: "profit_sharing",
		}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(21), payload.PaymentOrderID)
			require.Equal(t, int64(77), payload.OrderID)
			return errors.New("queue down")
		})

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newSettlementNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleEcommerceRefundNotify_OwnershipMismatchReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptEcommerceRefundNotification(gomock.Any()).
		Times(1).
		Return(&wechat.EcommerceRefundNotification{
			OutTradeNo:  "ORDER_1",
			OutRefundNo: "REFUND_1",
			SpMchID:     "sp_wrong",
		}, nil)

	ecommerceClient.EXPECT().
		GetSpMchID().
		Times(1).
		Return("sp_expected")

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Times(1).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()

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

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "ownership validation failed")
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				// 解密后金额为200元，与订单金额不匹配
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
							Total: 200 * fenPerYuan,
						},
					}, nil)

				// 订单金额为100元
				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       100 * fenPerYuan,
						Status:       "pending",
						UserID:       100,
						BusinessType: "order",
					}, nil)

				store.EXPECT().
					CreateRefundOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.RefundOrder{ID: 501, PaymentOrderID: 1, Status: "pending", OutRefundNo: "RFM1"}, nil)

				// 金额不匹配，先标记为 paid 再触发退款
				store.EXPECT().
					UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{
						ID:           1,
						OutTradeNo:   outTradeNo,
						Amount:       100 * fenPerYuan,
						Status:       "paid",
						UserID:       100,
						BusinessType: "order",
					}, nil)

				store.EXPECT().
					UpdateRefundOrderToFailed(gomock.Any(), int64(501)).
					Times(1).
					Return(db.RefundOrder{ID: 501, PaymentOrderID: 1, Status: "failed", OutRefundNo: "RFM1"}, nil)
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
			name: "归属校验失败_返回FAIL触发重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechat.PaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						AppID:         "wx_wrong_app",
						MchID:         "mch_wrong",
						Amount: struct {
							Total         int64  `json:"total"`
							PayerTotal    int64  `json:"payer_total"`
							Currency      string `json:"currency"`
							PayerCurrency string `json:"payer_currency"`
						}{
							Total: amount,
						},
					}, nil)

				paymentClient.EXPECT().
					GetMchID().
					Times(1).
					Return("mch_expected")

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(nil)
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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
				require.Contains(t, response["message"], "ownership validation failed")
			},
		},
		{
			name: "订单不存在_返回FAIL触发重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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

				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(nil)
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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
				require.Contains(t, response["message"], "payment order not found")
			},
		},
		{
			name: "订单不存在_release失败仍返回FAIL触发重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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

				store.EXPECT().
					GetPaymentOrderByOutTradeNo(gomock.Any(), gomock.Eq(outTradeNo)).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(errors.New("release failed"))
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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
				require.Contains(t, response["message"], "payment order not found")
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
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

func newCombinePaymentNotifyRequest(t *testing.T, notificationID string) *http.Request {
	t.Helper()

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
}

func newSettlementNotifyRequest(t *testing.T, notificationID string) *http.Request {
	t.Helper()

	requestBody := map[string]interface{}{
		"id":            notificationID,
		"event_type":    "trade_manage_order_settlement",
		"resource_type": "encrypt-resource",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_encrypted_data",
			"nonce":           "mock_nonce",
			"associated_data": "settlement",
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-miniprogram/settlement-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	return request
}

func assertWechatSuccessResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedMessage string) {
	t.Helper()

	var response map[string]string
	err := json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "SUCCESS", response["code"])
	require.Equal(t, expectedMessage, response["message"])
}

func assertWechatFailResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedMessage string) {
	t.Helper()

	var response map[string]string
	err := json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "FAIL", response["code"])
	require.Equal(t, expectedMessage, response["message"])
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					GetWechatNotification(gomock.Any(), notificationID).
					Times(1).
					Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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
							Amount:     1 * fenPerYuan,
						},
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				ecommerceClient.EXPECT().
					QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(&wechat.ProfitSharingQueryResponse{
						Status: "FINISHED",
						Receivers: []wechat.ProfitSharingReceiverResult{
							{Result: "SUCCESS", Amount: 1 * fenPerYuan},
						},
					}, nil)

				// 查询分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         1,
						MerchantID: 21,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), gomock.Eq(int64(21))).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 21, SubMchID: "sub_mch_id", Status: "active"}, nil)

				// 更新为成功
				store.EXPECT().
					UpdateProfitSharingOrderToFinished(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(db.ProfitSharingOrder{ID: 1, Status: "finished"}, nil)

				// 记录通知
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
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

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
							Amount:     1 * fenPerYuan,
						},
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				ecommerceClient.EXPECT().
					QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(&wechat.ProfitSharingQueryResponse{
						Status: "FINISHED",
						Receivers: []wechat.ProfitSharingReceiverResult{
							{Result: "CLOSED", FailReason: "NO_RELATION", Amount: 1 * fenPerYuan},
						},
					}, nil)

				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         1,
						MerchantID: 21,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), gomock.Eq(int64(21))).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 21, SubMchID: "sub_mch_id", Status: "active"}, nil)

				// 更新为失败
				store.EXPECT().
					UpdateProfitSharingOrderToFailed(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(db.ProfitSharingOrder{ID: 1, Status: "failed"}, nil)
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
		{
			name: "分账回调子商户归属不匹配_返回FAIL",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechat.ProfitSharingNotification{
						MchID:      "sp_mch_id",
						SubMchID:   "sub_mch_wrong",
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
							Result: "SUCCESS",
							Amount: 1 * fenPerYuan,
						},
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         2,
						MerchantID: 22,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), gomock.Eq(int64(22))).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 22, SubMchID: "sub_mch_expected", Status: "active"}, nil)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
					Times(1).
					Return(nil)
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
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				assertWechatFailResponse(t, recorder, "ownership validation failed")
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
