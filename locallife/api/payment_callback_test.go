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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type refundFactApplicationEnqueueRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
	err            error
}

func (r *refundFactApplicationEnqueueRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return r.err
}

type directPaymentCallbackTaskRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
	refundPayloads []worker.PayloadProcessRefund
}

func (r *directPaymentCallbackTaskRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

func (r *directPaymentCallbackTaskRecorder) DistributeTaskProcessRefund(_ context.Context, payload *worker.PayloadProcessRefund, _ ...asynq.Option) error {
	r.refundPayloads = append(r.refundPayloads, *payload)
	return nil
}

// TestHandlePaymentNotifyIdempotency 测试支付回调的幂等性检查
func TestHandlePaymentNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				// 先验证签名（必须通过）
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "首次通知_验证签名失败",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				// 签名验证失败（先于幂等性检查）
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Eq("invalid_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

			store := newMockStoreWithAlertSink(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
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
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复退款通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface) {
				// 先验证签名
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := newMockStoreWithAlertSink(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
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

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	paymentClient.EXPECT().
		DecryptRefundNotification(gomock.Any()).
		Times(1).
		Return(&wechatcontracts.DirectRefundNotificationResource{
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

// newTestServerWithPaymentClient 创建带Mock PaymentClient的测试服务器
func newTestServerWithPaymentClient(t *testing.T, store db.Store, paymentClient *mockwechat.MockDirectPaymentClientInterface) *Server {
	server := newTestServer(t, store)

	// 替换为Mock客户端
	server.directPaymentClient = paymentClient

	return server
}

// newTestServerWithTransferClient 创建带Mock TransferClient的测试服务器
func newTestServerWithTransferClient(t *testing.T, store db.Store, transferClient *mockwechat.MockTransferClientInterface) *Server {
	server := newTestServer(t, store)
	server.transferClient = transferClient
	return server
}

// TestHandlePaymentNotifyFullFlow 测试支付回调完整业务流程
func TestHandlePaymentNotifyFullFlow(t *testing.T) {
	notificationID := util.RandomString(32)
	outTradeNo := "TEST_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)
	amount := int64(10000) // 100元

	testCases := []struct {
		name               string
		buildStubs         func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor)
		setupRequest       func(t *testing.T) *http.Request
		checkResponse      func(t *testing.T, recorder *httptest.ResponseRecorder)
		useTaskDistributor bool
	}{
		{
			name: "直连订单支付回调_拒绝旧success任务",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				// 1. 签名验证通过
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: wechatcontracts.DirectOrderQueryAmount{
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

				taskDistributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				store.EXPECT().
					ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
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
				assertWechatFailResponse(t, recorder, "unsupported payment business type, please retry")
			},
			useTaskDistributor: true,
		},
		{
			name: "金额不匹配_返回SUCCESS避免重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: wechatcontracts.DirectOrderQueryAmount{
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "归属校验失败_返回FAIL触发重试",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						AppID:         "wx_wrong_app",
						MchID:         "mch_wrong",
						Amount: wechatcontracts.DirectOrderQueryAmount{
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: wechatcontracts.DirectOrderQueryAmount{
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: wechatcontracts.DirectOrderQueryAmount{
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
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				paymentClient.EXPECT().
					DecryptPaymentNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.DirectPaymentNotificationResource{
						OutTradeNo:    outTradeNo,
						TransactionID: transactionID,
						Amount: wechatcontracts.DirectOrderQueryAmount{
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
						ProcessedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "非SUCCESS事件类型_忽略处理",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				paymentClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "无PaymentClient_返回500",
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
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

			store := newMockStoreWithAlertSink(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, paymentClient, taskDistributor)

			var server *Server
			if tc.name == "无PaymentClient_返回500" {
				// 特殊处理：不设置paymentClient
				server = newTestServer(t, store)
			} else {
				server = newTestServerWithPaymentClient(t, store, paymentClient)
				if tc.useTaskDistributor {
					server.SetTaskDistributorForTest(taskDistributor)
				}
			}
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestHandlePaymentNotify_RiderDepositRecordsPaymentFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	outTradeNo := "RD_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)
	amount := int64(10000)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)
	paymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Return(&wechatcontracts.DirectPaymentNotificationResource{
			OutTradeNo:    outTradeNo,
			TransactionID: transactionID,
			TradeState:    "SUCCESS",
			SuccessTime:   "2026-04-16T10:00:00+08:00",
			Amount: wechatcontracts.DirectOrderQueryAmount{
				Total: amount,
			},
		}, nil)
	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           91,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPending,
			UserID:       1001,
			BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
		}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, outTradeNo, arg.ExternalObjectKey)
			require.Equal(t, transactionID, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerRiderDeposit, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectPaymentOrder, arg.BusinessObjectType.String)
			require.Equal(t, int64(91), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:direct_payment:"+notificationID, arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 100, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             100,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   int64(91),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 200,
		FactID:             100,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   91,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{
			ID:           91,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPaid,
			UserID:       1001,
			BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
		}, nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{200}, taskDistributor.applicationIDs)
}

func TestHandlePaymentNotify_ClaimRecoveryRecordsPaymentFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	outTradeNo := "CR_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)
	amount := int64(8800)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)
	paymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Return(&wechatcontracts.DirectPaymentNotificationResource{
			OutTradeNo:    outTradeNo,
			TransactionID: transactionID,
			TradeState:    "SUCCESS",
			SuccessTime:   "2026-04-26T10:00:00+08:00",
			Amount: wechatcontracts.DirectOrderQueryAmount{
				Total: amount,
			},
		}, nil)
	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           92,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPending,
			UserID:       1002,
			BusinessType: db.ExternalPaymentBusinessOwnerClaimRecovery,
		}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, outTradeNo, arg.ExternalObjectKey)
			require.Equal(t, transactionID, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectPaymentOrder, arg.BusinessObjectType.String)
			require.Equal(t, int64(92), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:direct_payment:"+notificationID, arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 101, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             101,
		Consumer:           paymentFactConsumerClaimRecoveryDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   int64(92),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 201,
		FactID:             101,
		Consumer:           paymentFactConsumerClaimRecoveryDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   92,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{
			ID:           92,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPaid,
			UserID:       1002,
			BusinessType: db.ExternalPaymentBusinessOwnerClaimRecovery,
		}, nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{201}, taskDistributor.applicationIDs)
}

func TestHandlePaymentNotify_BaofuVerifyFeeRecordsPaymentFactAndDedupesReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	outTradeNo := "BFVF_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)
	amount := int64(200)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(2).
		Return(nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)
	paymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Return(&wechatcontracts.DirectPaymentNotificationResource{
			OutTradeNo:    outTradeNo,
			TransactionID: transactionID,
			TradeState:    "SUCCESS",
			SuccessTime:   "2026-05-08T10:00:00+08:00",
			Amount: wechatcontracts.DirectOrderQueryAmount{
				Total: amount,
			},
		}, nil)
	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           93,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPending,
			UserID:       1003,
			BusinessType: db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, outTradeNo, arg.ExternalObjectKey)
			require.Equal(t, transactionID, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerBaofuVerifyFee, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectPaymentOrder, arg.BusinessObjectType.String)
			require.Equal(t, int64(93), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:direct_payment:"+notificationID, arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 102, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             102,
		Consumer:           paymentFactConsumerBaofuVerifyFeeDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   int64(93),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 202,
		FactID:             102,
		Consumer:           paymentFactConsumerBaofuVerifyFeeDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   93,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{
			ID:           93,
			OutTradeNo:   outTradeNo,
			Amount:       amount,
			Status:       PaymentStatusPaid,
			UserID:       1003,
			BusinessType: db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		}, nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(false, nil)
	store.EXPECT().
		GetWechatNotification(gomock.Any(), notificationID).
		Return(db.WechatNotification{
			ID:          notificationID,
			ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
		}, nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{202}, taskDistributor.applicationIDs)

	replay, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	replay.Header.Set("Wechatpay-Timestamp", "1234567890")
	replay.Header.Set("Wechatpay-Nonce", "test_nonce")
	replay.Header.Set("Wechatpay-Signature", "test_signature")
	replay.Header.Set("Wechatpay-Serial", "test_serial")

	replayRecorder := httptest.NewRecorder()
	server.router.ServeHTTP(replayRecorder, replay)
	assertWechatNoContentResponse(t, replayRecorder)
	require.Equal(t, []int64{202}, taskDistributor.applicationIDs)
}

func TestHandlePaymentNotify_BaofuVerifyFeeAmountMismatchDoesNotRecordPaymentFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	taskDistributor := &directPaymentCallbackTaskRecorder{}
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	outTradeNo := "BFVF_" + util.RandomString(20)
	transactionID := "WX_" + util.RandomString(20)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)
	paymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Return(&wechatcontracts.DirectPaymentNotificationResource{
			OutTradeNo:    outTradeNo,
			TransactionID: transactionID,
			TradeState:    "SUCCESS",
			SuccessTime:   "2026-05-08T10:00:00+08:00",
			Amount: wechatcontracts.DirectOrderQueryAmount{
				Total: 999,
			},
		}, nil)
	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:           94,
			OutTradeNo:   outTradeNo,
			Amount:       200,
			Status:       PaymentStatusPending,
			UserID:       1004,
			BusinessType: db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		}, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.AssignableToTypeOf(db.CreateRefundOrderParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(94), arg.PaymentOrderID)
			require.Equal(t, "amount_mismatch", arg.RefundType)
			require.Equal(t, int64(999), arg.RefundAmount)
			return db.RefundOrder{ID: 502, PaymentOrderID: 94, RefundAmount: 999, OutRefundNo: arg.OutRefundNo, Status: "pending"}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
			ID:            int64(94),
			TransactionID: pgtype.Text{String: transactionID, Valid: true},
		}).
		Return(db.PaymentOrder{
			ID:           94,
			OutTradeNo:   outTradeNo,
			Amount:       200,
			Status:       PaymentStatusPaid,
			UserID:       1004,
			BusinessType: db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		}, nil)
	store.EXPECT().
		UpdateRefundOrderToFailed(gomock.Any(), int64(502)).
		Return(db.RefundOrder{ID: 502, PaymentOrderID: 94, RefundAmount: 999, Status: "failed"}, nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
	require.Empty(t, taskDistributor.applicationIDs)
	require.Empty(t, taskDistributor.refundPayloads)
}

func TestHandleRefundNotify_RiderDepositRecordsRefundFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	outRefundNo := "RFD_" + util.RandomString(20)
	outTradeNo := "RD_" + util.RandomString(20)
	refundID := "WX_REFUND_" + util.RandomString(12)
	transactionID := "WX_" + util.RandomString(12)

	paymentClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	paymentClient.EXPECT().DecryptRefundNotification(gomock.Any()).Return(&wechatcontracts.DirectRefundNotificationResource{
		MchID:         "mch_expected",
		OutTradeNo:    outTradeNo,
		TransactionID: transactionID,
		OutRefundNo:   outRefundNo,
		RefundID:      refundID,
		RefundStatus:  wechatcontracts.DirectRefundStatusSuccess,
		SuccessTime:   "2026-04-16T10:00:00+08:00",
		Amount: wechatcontracts.DirectRefundNotificationAmount{
			Total:  10000,
			Refund: 10000,
		},
	}, nil)
	paymentClient.EXPECT().GetMchID().Return("mch_expected")
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), outRefundNo).Return(db.RefundOrder{
		ID:             301,
		PaymentOrderID: 91,
		OutRefundNo:    outRefundNo,
		RefundAmount:   10000,
	}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(91)).Return(db.PaymentOrder{
		ID:           91,
		OutTradeNo:   outTradeNo,
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
	}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.Equal(t, outRefundNo, arg.ExternalObjectKey)
			require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
			require.Equal(t, int64(301), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:direct_refund:"+notificationID, arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 101, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             101,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   int64(301),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 201,
		FactID:             101,
		Consumer:           paymentFactConsumerRiderDepositDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   301,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "REFUND.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{201}, taskDistributor.applicationIDs)
}

func TestHandleOrderSettlementNotify_DirectShippingSettlementRecordsNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	server := newTestServerWithPaymentClient(t, store, paymentClient)

	notificationID := util.RandomString(32)
	merchantTradeNo := "WXSHIP_" + util.RandomString(16)
	transactionID := "WX_" + util.RandomString(20)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateWechatNotificationParams) (bool, error) {
			require.Equal(t, notificationID, arg.ID)
			require.Equal(t, "trade_manage_order_settlement", arg.EventType)
			return true, nil
		})

	rawResource, err := json.Marshal(map[string]any{
		"appid":                  "wx-mini-app",
		"transaction_id":         transactionID,
		"merchant_trade_no":      merchantTradeNo,
		"confirm_receive_method": 1,
		"settlement_time":        "2026-05-22T12:00:00+08:00",
		"success_time":           "2026-05-22T11:58:00+08:00",
	})
	require.NoError(t, err)
	paymentClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Return(rawResource, nil)

	recorder := httptest.NewRecorder()
	request := newSettlementNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
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
	_ = expectedMessage
	assertWechatNoContentResponse(t, recorder)
}

func assertWechatNoContentResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Empty(t, recorder.Body.String())
}

func assertWechatFailResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedMessage string) {
	t.Helper()

	var response map[string]string
	err := json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "FAIL", response["code"])
	require.Equal(t, expectedMessage, response["message"])
}

func TestHandleMerchantTransferNotify_SuccessMarksClaimPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	server := newTestServerWithTransferClient(t, store, transferClient)

	notificationID := util.RandomString(32)
	actionID := int64(42)
	detailBytes, err := json.Marshal(map[string]any{
		"claim_id":    int64(88),
		"user_id":     int64(9),
		"amount":      int64(1200),
		"source_type": "claim",
		"source_id":   int64(88),
		"out_bill_no": "claimpayout42",
	})
	require.NoError(t, err)

	transferClient.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	transferClient.EXPECT().
		DecryptMerchantTransferNotification(gomock.Any()).
		Return(&wechatcontracts.DirectMerchantTransferNotificationResource{
			OutBillNo:      "claimpayout42",
			TransferBillNo: "transfer-001",
			State:          wechatcontracts.DirectMerchantTransferStateSuccess,
		}, nil)

	store.EXPECT().
		GetBehaviorAction(gomock.Any(), actionID).
		Return(db.BehaviorAction{
			ID:           actionID,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "running",
			Detail:       detailBytes,
		}, nil)

	store.EXPECT().
		MarkClaimPaid(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkClaimPaidParams) error {
			require.Equal(t, int64(88), arg.ID)
			require.True(t, arg.PaidAt.Valid)
			return nil
		})

	store.EXPECT().
		FinalizeClaimCompensationAfterPayoutTx(gomock.Any(), db.FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: 88}).
		Return(db.FinalizeClaimCompensationAfterPayoutTxResult{Claim: db.Claim{ID: 88}}, nil)

	store.EXPECT().
		UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params db.UpdateBehaviorActionExecutionParams) error {
			var detail map[string]any
			err := json.Unmarshal(params.Detail, &detail)
			require.NoError(t, err)
			require.Equal(t, "success", params.Status)
			require.Equal(t, "claimpayout42", detail["out_bill_no"])
			require.Equal(t, "transfer-001", detail["transfer_bill_no"])
			require.Equal(t, wechatcontracts.DirectMerchantTransferStateSuccess, detail["transfer_state"])
			return nil
		})

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    wechatcontracts.DirectMerchantTransferNotifyEventTypeBillFinished,
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_encrypted_data",
			"nonce":           "mock_nonce",
			"associated_data": "merchant_transfer",
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/merchant-transfer-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
}

func TestHandleMerchantTransferNotify_InvalidOutBillNoReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	server := newTestServerWithTransferClient(t, store, transferClient)

	notificationID := util.RandomString(32)

	transferClient.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	transferClient.EXPECT().
		DecryptMerchantTransferNotification(gomock.Any()).
		Return(&wechatcontracts.DirectMerchantTransferNotificationResource{
			OutBillNo:      "merchant-transfer-raw-001",
			TransferBillNo: "transfer-001",
			State:          wechatcontracts.DirectMerchantTransferStateSuccess,
		}, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    wechatcontracts.DirectMerchantTransferNotifyEventTypeBillFinished,
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_encrypted_data",
			"nonce":           "mock_nonce",
			"associated_data": "merchant_transfer",
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/merchant-transfer-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assertWechatFailResponse(t, recorder, "invalid out_bill_no")
}

func TestHandleMerchantTransferNotify_OwnershipMismatchReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	server := newTestServerWithTransferClient(t, store, transferClient)

	notificationID := util.RandomString(32)

	transferClient.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	transferClient.EXPECT().
		DecryptMerchantTransferNotification(gomock.Any()).
		Return(&wechatcontracts.DirectMerchantTransferNotificationResource{
			OutBillNo:      "claimpayout42",
			TransferBillNo: "transfer-001",
			State:          wechatcontracts.DirectMerchantTransferStateSuccess,
			MchID:          "mch_wrong",
		}, nil)

	transferClient.EXPECT().
		GetMchID().
		Return("mch_expected")

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    wechatcontracts.DirectMerchantTransferNotifyEventTypeBillFinished,
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_encrypted_data",
			"nonce":           "mock_nonce",
			"associated_data": "merchant_transfer",
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/merchant-transfer-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "ownership validation failed")
}

func TestHandleRefundNotify_WithoutTaskDistributorReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	server := newTestServerWithPaymentClient(t, store, paymentClient)
	server.taskDistributor = nil

	notificationID := util.RandomString(32)
	outRefundNo := "RF_" + util.RandomString(10)

	paymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1234567890"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	paymentClient.EXPECT().
		DecryptRefundNotification(gomock.Any()).
		Return(&wechatcontracts.DirectRefundNotificationResource{
			OutRefundNo:  outRefundNo,
			OutTradeNo:   "out_trade_no_001",
			RefundID:     "refund_id_001",
			RefundStatus: "SUCCESS",
			MchID:        "mch_expected",
			Amount: wechatcontracts.DirectRefundNotificationAmount{
				Refund: 100,
			},
		}, nil)

	paymentClient.EXPECT().
		GetMchID().
		Return("mch_expected")

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	requestBody := map[string]any{
		"id":            notificationID,
		"event_type":    "REFUND.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "refund result processing unavailable, please retry")
}

func newMockStoreWithAlertSink(ctrl *gomock.Controller) *mockdb.MockStore {
	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().CreatePlatformAlertEvent(gomock.Any(), gomock.Any()).AnyTimes().Return(db.PlatformAlertEvent{}, nil)
	return store
}
