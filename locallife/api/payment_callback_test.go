package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/combine-notify", bytes.NewReader(bodyBytes))
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
			store.EXPECT().CreatePlatformAlertEvent(gomock.Any(), gomock.Any()).AnyTimes().Return(db.PlatformAlertEvent{}, nil)
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

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 10000), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 201, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:                11,
			OutTradeNo:        outTradeNo,
			Amount:            10000,
			Status:            PaymentStatusClosed,
			CombinedPaymentID: pgtype.Int8{Int64: 201, Valid: true},
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

	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleEcommercePaymentNotify_DelegatesToPartnerHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	outTradeNo := "PS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptPartnerPaymentNotification(gomock.Any()).
		Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_expected", 8800), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:             88,
			OutTradeNo:     outTradeNo,
			Status:         PaymentStatusPaid,
			PaymentType:    "profit_sharing",
			PaymentChannel: db.PaymentChannelEcommerce,
			OrderID:        pgtype.Int8{Int64: 66, Valid: true},
			ProcessedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(66)).
		Return(db.Order{ID: 66, MerchantID: 99}, nil)

	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(99)).
		Return(db.MerchantPaymentConfig{MerchantID: 99, SubMchID: "sub_expected"}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommercePaymentNotify_ContractValidationFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptPartnerPaymentNotification(gomock.Any()).
		Return(&wechatcontracts.PartnerPaymentNotificationResource{
			SpMchID:        "sp_expected",
			SpAppID:        "app_expected",
			SubMchID:       "sub_expected",
			OutTradeNo:     "order-1",
			TradeType:      "JSAPI",
			TradeState:     "SUCCESS",
			TradeStateDesc: "success",
			BankType:       "OTHERS",
			SuccessTime:    "2026-04-16T10:00:00+08:00",
			Amount: wechatcontracts.PartnerOrderQueryAmount{
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "notification contract validation failed")
}

func TestHandleEcommercePaymentNotify_UsesPersistedSubMchIDWhenConfigDrifts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	outTradeNo := "PS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptPartnerPaymentNotification(gomock.Any()).
		Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_original", 8800), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:             90,
			OutTradeNo:     outTradeNo,
			Status:         PaymentStatusPaid,
			PaymentType:    "profit_sharing",
			PaymentChannel: db.PaymentChannelEcommerce,
			Attach:         pgtype.Text{String: "order_id:66;sub_mchid:sub_original", Valid: true},
			ProcessedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommercePaymentNotify_PaidUnprocessedCreatesOrderPaymentFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &ecommerceOrderPaymentFactApplicationRecorder{}

	notificationID := util.RandomString(32)
	outTradeNo := "PS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptPartnerPaymentNotification(gomock.Any()).
		Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_expected", 8800), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:             89,
			OutTradeNo:     outTradeNo,
			Status:         PaymentStatusPaid,
			PaymentType:    "profit_sharing",
			PaymentChannel: db.PaymentChannelEcommerce,
			BusinessType:   "order",
			OrderID:        pgtype.Int8{Int64: 67, Valid: true},
			ProcessedAt:    pgtype.Timestamptz{Valid: false},
		}, nil)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(67)).
		Return(db.Order{ID: 67, MerchantID: 100}, nil)

	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(100)).
		Return(db.MerchantPaymentConfig{MerchantID: 100, SubMchID: "sub_expected"}, nil)

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
		require.Equal(t, outTradeNo, arg.ExternalObjectKey)
		require.Equal(t, transactionID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.Equal(t, int64(89), arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 5001, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(5001), arg.FactID)
		require.Equal(t, "order_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, int64(89), arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 6001, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	taskDistributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(6001), payload.ApplicationID)
		return nil
	}

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommercePaymentNotify_PaidUnprocessedCreatesReservationPaymentFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &ecommerceOrderPaymentFactApplicationRecorder{}

	notificationID := util.RandomString(32)
	outTradeNo := "RS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptPartnerPaymentNotification(gomock.Any()).
		Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_expected", 6800), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:             189,
			OutTradeNo:     outTradeNo,
			Status:         PaymentStatusPaid,
			PaymentType:    "profit_sharing",
			PaymentChannel: db.PaymentChannelEcommerce,
			BusinessType:   db.ExternalPaymentBusinessOwnerReservation,
			ReservationID:  pgtype.Int8{Int64: 167, Valid: true},
			Attach:         pgtype.Text{String: "sub_mchid:sub_expected", Valid: true},
			ProcessedAt:    pgtype.Timestamptz{Valid: false},
		}, nil)

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
		require.Equal(t, outTradeNo, arg.ExternalObjectKey)
		require.Equal(t, transactionID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.Equal(t, int64(189), arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 5101, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(5101), arg.FactID)
		require.Equal(t, "reservation_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, int64(189), arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 6101, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	taskDistributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(6101), payload.ApplicationID)
		return nil
	}

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommercePaymentNotify_ClosedOrderEnqueueFailureEmitsAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	outTradeNo := "PS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	store.EXPECT().CreatePlatformAlertEvent(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
		require.Contains(t, arg.Title, "退款任务入队失败")
		return db.PlatformAlertEvent{}, nil
	})

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptPartnerPaymentNotification(gomock.Any()).Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_expected", 8800), nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).Return(db.PaymentOrder{
		ID:             88,
		OutTradeNo:     outTradeNo,
		Status:         PaymentStatusClosed,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 66, Valid: true},
	}, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(66)).Return(db.Order{ID: 66, MerchantID: 99}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(99)).Return(db.MerchantPaymentConfig{MerchantID: 99, SubMchID: "sub_expected"}, nil)
	taskDistributor.EXPECT().DistributeTaskProcessAnomalyRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessAnomalyRefund{}), gomock.Any(), gomock.Any()).Return(errors.New("enqueue failed"))

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommercePaymentNotify_AmountMismatchEnqueueFailureEmitsAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	outTradeNo := "PS_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	store.EXPECT().CreatePlatformAlertEvent(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
		require.Contains(t, arg.Title, "金额异常退款任务入队失败")
		return db.PlatformAlertEvent{}, nil
	})

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptPartnerPaymentNotification(gomock.Any()).Return(newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, "sub_expected", 9900), nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).Return(db.PaymentOrder{
		ID:             88,
		OutTradeNo:     outTradeNo,
		Status:         PaymentStatusPending,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         8800,
		OrderID:        pgtype.Int8{Int64: 66, Valid: true},
	}, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(66)).Return(db.Order{ID: 66, MerchantID: 99}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(99)).Return(db.MerchantPaymentConfig{MerchantID: 99, SubMchID: "sub_expected"}, nil)
	store.EXPECT().CreateRefundOrder(gomock.Any(), gomock.Any()).Return(db.RefundOrder{ID: 501}, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            int64(88),
		TransactionID: pgtype.Text{String: transactionID, Valid: true},
	}).Return(db.PaymentOrder{ID: 88, Status: PaymentStatusPaid}, nil)
	taskDistributor.EXPECT().DistributeTaskProcessRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessRefund{}), gomock.Any(), gomock.Any()).Return(errors.New("enqueue failed"))
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), int64(501)).Return(db.RefundOrder{ID: 501, Status: "failed"}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
	recorder := httptest.NewRecorder()
	request := newEcommercePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleCombinePaymentNotify_OwnershipMismatchReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	resource := newValidCombinePaymentNotification(combineOutTradeNo, "SUB_"+util.RandomString(18), "WX_"+util.RandomString(18), 10000)
	resource.CombineMchID = "sp_wrong"

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(resource, nil)

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

func TestHandleCombinePaymentNotify_ContractValidationFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(&wechatcontracts.CombinePaymentNotification{
			CombineOutTradeNo: "combine-1",
			CombineMchID:      "sp_expected",
			CombineAppID:      "app_expected",
		}, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "notification contract validation failed")
}

func TestHandleCombinePaymentNotify_SubOrderNotFoundReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 10000), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 301, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)

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

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 12000), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 401, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:                12,
			OutTradeNo:        outTradeNo,
			Amount:            10000,
			Status:            PaymentStatusPending,
			BusinessType:      "order",
			OrderID:           pgtype.Int8{Int64: 88, Valid: true},
			CombinedPaymentID: pgtype.Int8{Int64: 401, Valid: true},
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

	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleCombinePaymentNotify_MainOrderNotFoundReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 10000), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{}, db.ErrRecordNotFound)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "get combined payment order failed")
}

func TestHandleCombinePaymentNotify_PaymentSuccessEnqueueFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 10000), nil)

	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")

	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 501, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)

	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:                13,
			OutTradeNo:        outTradeNo,
			Amount:            10000,
			Status:            PaymentStatusPending,
			BusinessType:      "order",
			CombinedPaymentID: pgtype.Int8{Int64: 501, Valid: true},
		}, nil)

	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{ID: 13, Status: PaymentStatusPaid}, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "1 orders failed")
}

func TestHandleCombinePaymentNotify_ClosedOrderEnqueueFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	store.EXPECT().CreatePlatformAlertEvent(gomock.Any(), gomock.Any()).Return(db.PlatformAlertEvent{}, nil)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptCombinePaymentNotification(gomock.Any()).Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 10000), nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")
	store.EXPECT().GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).Return(db.CombinedPaymentOrder{ID: 601, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).Return(db.PaymentOrder{
		ID:                61,
		OutTradeNo:        outTradeNo,
		Amount:            10000,
		Status:            PaymentStatusClosed,
		CombinedPaymentID: pgtype.Int8{Int64: 601, Valid: true},
	}, nil)
	taskDistributor.EXPECT().DistributeTaskProcessAnomalyRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessAnomalyRefund{}), gomock.Any(), gomock.Any()).Return(errors.New("enqueue failed"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "1 orders failed")
}

func TestHandleCombinePaymentNotify_ReservationPaymentCreatesFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &ecommerceOrderPaymentFactApplicationRecorder{}

	notificationID := util.RandomString(32)
	combineOutTradeNo := "COMB_" + util.RandomString(18)
	outTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(true, nil)
	ecommerceClient.EXPECT().
		DecryptCombinePaymentNotification(gomock.Any()).
		Return(newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID, 6800), nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	ecommerceClient.EXPECT().GetSpAppID().Return("app_expected")
	store.EXPECT().
		GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combineOutTradeNo).
		Return(db.CombinedPaymentOrder{ID: 701, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPending}, nil)
	store.EXPECT().
		GetPaymentOrderByOutTradeNo(gomock.Any(), outTradeNo).
		Return(db.PaymentOrder{
			ID:                171,
			OutTradeNo:        outTradeNo,
			Amount:            6800,
			Status:            PaymentStatusPending,
			PaymentChannel:    db.PaymentChannelEcommerce,
			BusinessType:      db.ExternalPaymentBusinessOwnerReservation,
			ReservationID:     pgtype.Int8{Int64: 271, Valid: true},
			CombinedPaymentID: pgtype.Int8{Int64: 701, Valid: true},
			Attach:            pgtype.Text{String: "sub_mchid:sub_expected", Valid: true},
		}, nil)
	store.EXPECT().
		UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{ID: int64(171), TransactionID: pgtype.Text{String: transactionID, Valid: true}}).
		Return(db.PaymentOrder{
			ID:                171,
			OutTradeNo:        outTradeNo,
			Amount:            6800,
			Status:            PaymentStatusPaid,
			PaymentChannel:    db.PaymentChannelEcommerce,
			BusinessType:      db.ExternalPaymentBusinessOwnerReservation,
			ReservationID:     pgtype.Int8{Int64: 271, Valid: true},
			CombinedPaymentID: pgtype.Int8{Int64: 701, Valid: true},
			Attach:            pgtype.Text{String: "sub_mchid:sub_expected", Valid: true},
		}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectCombinedPayment, arg.ExternalObjectType)
		require.Equal(t, combineOutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, outTradeNo, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.Equal(t, int64(171), arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 5701, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(5701), arg.FactID)
		require.Equal(t, "reservation_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, int64(171), arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 6701, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	taskDistributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(6701), payload.ApplicationID)
		return nil
	}
	store.EXPECT().
		UpdateCombinedPaymentOrderToPaid(gomock.Any(), db.UpdateCombinedPaymentOrderToPaidParams{ID: int64(701), TransactionID: pgtype.Text{Valid: false}}).
		Return(db.CombinedPaymentOrder{ID: 701, CombineOutTradeNo: combineOutTradeNo, Status: PaymentStatusPaid}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newCombinePaymentNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	assertWechatSuccessResponse(t, recorder, "OK")
}

func TestHandleOrderSettlementNotify_ProfitSharingEnqueueFailureReturnsFailAndReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)
	merchantTradeNo := "SUB_" + util.RandomString(18)
	transactionID := "WX_" + util.RandomString(18)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			ID:                    21,
			OutTradeNo:            merchantTradeNo,
			Status:                PaymentStatusPaid,
			PaymentType:           "profit_sharing",
			PaymentChannel:        db.PaymentChannelEcommerce,
			RequiresProfitSharing: true,
		}, nil)

	taskDistributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(21), payload.PaymentOrderID)
			require.Equal(t, int64(77), payload.OrderID)
			return errors.New("queue down")
		})

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	recorder := httptest.NewRecorder()
	request := newSettlementNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "enqueue failed, please retry")
}

func TestHandleEcommerceRefundNotify_OwnershipMismatchReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

func TestHandleEcommerceRefundNotify_SuccessReturnsNoContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			SpMchID:       "sp_expected",
			SubMchID:      "sub-001",
			OutTradeNo:    "ORDER_1",
			TransactionID: "WX_TX_1",
			OutRefundNo:   "REFUND_1",
			RefundID:      "WX_REFUND_1",
			RefundStatus:  wechat.RefundStatusSuccess,
			Amount: wechat.EcommerceRefundAmount{
				Refund: 88,
			},
		}, nil)

	ecommerceClient.EXPECT().
		GetSpMchID().
		Times(1).
		Return("sp_expected")
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), "REFUND_1").Times(2).Return(db.RefundOrder{ID: 401, PaymentOrderID: 91, OutRefundNo: "REFUND_1", RefundAmount: 88}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(91)).Times(2).Return(db.PaymentOrder{ID: 91, OutTradeNo: "ORDER_1", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder, OrderID: pgtype.Int8{Int64: 701, Valid: true}}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityEcommerceRefund, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
		require.Equal(t, "REFUND_1", arg.ExternalObjectKey)
		require.Equal(t, "WX_REFUND_1", arg.ExternalSecondaryKey.String)
		require.Equal(t, int64(401), arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, "wechat:callback:ecommerce_refund:"+notificationID, arg.DedupeKey)
		return db.ExternalPaymentFact{ID: 121, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             121,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   int64(401),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 221, FactID: 121, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: 401, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
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

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommerceRefundNotify_ReservationRefundRecordsFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}

	notificationID := util.RandomString(32)
	outRefundNo := "RES_REFUND_1"
	refundID := "WX_RES_REFUND_1"

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptEcommerceRefundNotification(gomock.Any()).Return(&wechat.EcommerceRefundNotification{
		SpMchID:       "sp_expected",
		SubMchID:      "sub-001",
		OutTradeNo:    "RES_ORDER_1",
		TransactionID: "WX_TX_RES_1",
		OutRefundNo:   outRefundNo,
		RefundID:      refundID,
		RefundStatus:  wechat.RefundStatusClosed,
		Amount:        wechat.EcommerceRefundAmount{Refund: 188},
	}, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), outRefundNo).Return(db.RefundOrder{ID: 321, PaymentOrderID: 92, OutRefundNo: outRefundNo, RefundAmount: 188}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(92)).Return(db.PaymentOrder{ID: 92, OutTradeNo: "RES_ORDER_1", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: "reservation_addon", ReservationID: pgtype.Int8{Int64: 802, Valid: true}}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityEcommerceRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.Equal(t, outRefundNo, arg.ExternalObjectKey)
			require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
			require.Equal(t, int64(321), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusClosed, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:ecommerce_refund:"+notificationID, arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 111, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             111,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   int64(321),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 211, FactID: 111, Consumer: paymentFactConsumerReservationDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: 321, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
	recorder := httptest.NewRecorder()

	requestBody := map[string]interface{}{
		"id":            notificationID,
		"event_type":    "REFUND.CLOSED",
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

	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{211}, taskDistributor.applicationIDs)
}

func TestRecordOrderOrdinaryRefundCallbackFactCreatesPartnerRefundFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	notification := wechat.PaymentNotification{ID: "ordinary-refund-notify-1", EventType: "REFUND.SUCCESS"}
	refundOrder := db.RefundOrder{ID: 501, PaymentOrderID: 901, OutRefundNo: "ORD_REFUND_1", RefundAmount: 288}
	paymentOrder := db.PaymentOrder{
		ID:             901,
		OutTradeNo:     "ORD_ORDER_1",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 1001, Valid: true},
	}
	resource := &ordinaryRefundNotificationResource{
		SpMchID:       "sp_expected",
		SubMchID:      "sub-001",
		OutTradeNo:    "ORD_ORDER_1",
		TransactionID: "WX_TX_ORD_1",
		OutRefundNo:   "ORD_REFUND_1",
		RefundID:      "WX_ORD_REFUND_1",
		RefundStatus:  wechat.RefundStatusSuccess,
		Amount:        ordinaryRefundNotificationAmount{Refund: 288},
	}

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityPartnerRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.Equal(t, "ORD_REFUND_1", arg.ExternalObjectKey)
			require.Equal(t, "WX_ORD_REFUND_1", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
			require.Equal(t, int64(501), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, int64(288), arg.Amount.Int64)
			require.Equal(t, "wechat:callback:ordinary_service_provider_refund:ordinary-refund-notify-1", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 151, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             151,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   int64(501),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 251, FactID: 151, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: 501, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	application, err := server.recordOrderOrdinaryRefundCallbackFact(context.Background(), notification, refundOrder, paymentOrder, resource)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Equal(t, int64(251), application.ID)
}

func TestRecordReservationOrdinaryRefundCallbackFactCreatesPartnerRefundFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	notification := wechat.PaymentNotification{ID: "ordinary-reservation-refund-notify-1", EventType: "REFUND.CLOSED"}
	refundOrder := db.RefundOrder{ID: 502, PaymentOrderID: 902, OutRefundNo: "ORD_RES_REFUND_1", RefundAmount: 388}
	paymentOrder := db.PaymentOrder{
		ID:             902,
		OutTradeNo:     "ORD_RES_ORDER_1",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   "reservation_addon",
		ReservationID:  pgtype.Int8{Int64: 1002, Valid: true},
	}
	resource := &ordinaryRefundNotificationResource{
		SpMchID:       "sp_expected",
		SubMchID:      "sub-001",
		OutTradeNo:    "ORD_RES_ORDER_1",
		TransactionID: "WX_TX_ORD_RES_1",
		OutRefundNo:   "ORD_RES_REFUND_1",
		RefundID:      "WX_ORD_RES_REFUND_1",
		RefundStatus:  wechat.RefundStatusClosed,
		Amount:        ordinaryRefundNotificationAmount{Refund: 388},
	}

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityPartnerRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusClosed, arg.TerminalStatus)
			require.Equal(t, "ORD_RES_REFUND_1", arg.ExternalObjectKey)
			require.Equal(t, "wechat:callback:ordinary_service_provider_refund:ordinary-reservation-refund-notify-1", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 152, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             152,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   int64(502),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 252, FactID: 152, Consumer: paymentFactConsumerReservationDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: 502, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	application, err := server.recordReservationOrdinaryRefundCallbackFact(context.Background(), notification, refundOrder, paymentOrder, resource)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Equal(t, int64(252), application.ID)
}

func TestRecordOrderPaymentCallbackFact_OrdinaryServiceProviderCreatesOrdinaryFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	notification := wechat.PaymentNotification{ID: "ordinary-payment-notify-1", EventType: "TRANSACTION.SUCCESS"}
	paymentOrder := db.PaymentOrder{
		ID:             601,
		OutTradeNo:     "ORD_PAY_1",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 1601, Valid: true},
	}
	resource := &wechatcontracts.PartnerPaymentNotificationResource{
		SpAppID:       "wxsp_app",
		SpMchID:       "1900000109",
		SubMchID:      "sub-001",
		OutTradeNo:    "ORD_PAY_1",
		TransactionID: "WX_ORD_PAY_1",
		TradeType:     "JSAPI",
		TradeState:    "SUCCESS",
		SuccessTime:   time.Now().UTC().Format(time.RFC3339),
		Amount:        wechatcontracts.PartnerOrderQueryAmount{Total: 688, PayerTotal: 688, Currency: "CNY", PayerCurrency: "CNY"},
	}

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, "ORD_PAY_1", arg.ExternalObjectKey)
			require.Equal(t, "WX_ORD_PAY_1", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:ordinary_service_provider:order_payment:ordinary-payment-notify-1", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 161, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             161,
		Consumer:           orderPaymentFactConsumerDomain,
		BusinessObjectType: orderPaymentFactBusinessObjectOrder,
		BusinessObjectID:   int64(601),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 261, FactID: 161, Consumer: orderPaymentFactConsumerDomain, BusinessObjectType: orderPaymentFactBusinessObjectOrder, BusinessObjectID: 601, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	application, err := server.recordOrderPaymentCallbackFact(context.Background(), notification, paymentOrder, resource)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Equal(t, int64(261), application.ID)
}

func TestRecordCombinedReservationPaymentCallbackFact_OrdinaryServiceProviderCreatesOrdinaryFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	notification := wechat.PaymentNotification{ID: "ordinary-combine-notify-1", EventType: "TRANSACTION.SUCCESS"}
	combinedOrder := db.CombinedPaymentOrder{ID: 701, CombineOutTradeNo: "ORD_COMB_1"}
	paymentOrder := db.PaymentOrder{
		ID:                602,
		OutTradeNo:        "ORD_COMB_SUB_1",
		PaymentChannel:    db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:      db.ExternalPaymentBusinessOwnerReservation,
		ReservationID:     pgtype.Int8{Int64: 2602, Valid: true},
		CombinedPaymentID: pgtype.Int8{Int64: 701, Valid: true},
	}
	subOrder := wechatcontracts.CombinePaymentNotificationSubOrder{
		SubMchID:      "sub-001",
		OutTradeNo:    "ORD_COMB_SUB_1",
		TransactionID: "WX_ORD_COMB_SUB_1",
		TradeState:    "SUCCESS",
		SuccessTime:   time.Now().UTC().Format(time.RFC3339),
	}
	subOrder.Amount.TotalAmount = 788
	subOrder.Amount.Currency = "CNY"

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentObjectCombinedPayment, arg.ExternalObjectType)
			require.Equal(t, "ORD_COMB_1", arg.ExternalObjectKey)
			require.Equal(t, "ORD_COMB_SUB_1", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, "wechat:callback:ordinary_service_provider:combine_reservation_payment:ordinary-combine-notify-1:ORD_COMB_SUB_1", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 162, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             162,
		Consumer:           reservationPaymentFactConsumerDomain,
		BusinessObjectType: reservationPaymentFactBusinessObjectOrder,
		BusinessObjectID:   int64(602),
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 262, FactID: 162, Consumer: reservationPaymentFactConsumerDomain, BusinessObjectType: reservationPaymentFactBusinessObjectOrder, BusinessObjectID: 602, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	application, err := server.recordCombinedReservationPaymentCallbackFact(context.Background(), &notification, combinedOrder, paymentOrder, subOrder)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Equal(t, int64(262), application.ID)
}

func TestHandleEcommerceRefundNotify_OrderRefundPaymentLookupFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}

	notificationID := util.RandomString(32)
	outRefundNo := "ORDER_REFUND_1"

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptEcommerceRefundNotification(gomock.Any()).Return(&wechat.EcommerceRefundNotification{
		SpMchID:       "sp_expected",
		SubMchID:      "sub-001",
		OutTradeNo:    "ORDER_1",
		TransactionID: "WX_TX_ORDER_1",
		OutRefundNo:   outRefundNo,
		RefundID:      "WX_ORDER_REFUND_1",
		RefundStatus:  wechat.RefundStatusSuccess,
		Amount:        wechat.EcommerceRefundAmount{Refund: 188},
	}, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_expected")
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), outRefundNo).Times(2).Return(db.RefundOrder{ID: 421, PaymentOrderID: 93, OutRefundNo: outRefundNo, RefundAmount: 188}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(93)).Times(2).Return(db.PaymentOrder{}, errors.New("db unavailable"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)
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

	assertWechatFailResponse(t, recorder, "refund fact object resolution failed, please retry")
	require.Empty(t, taskDistributor.applicationIDs)
}

func TestHandleEcommerceRefundNotify_WithoutTaskDistributorReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			SpMchID:       "sp_expected",
			SubMchID:      "sub-001",
			OutTradeNo:    "ORDER_1",
			TransactionID: "WX_TX_1",
			OutRefundNo:   "REFUND_1",
			RefundID:      "WX_REFUND_1",
			RefundStatus:  wechat.RefundStatusSuccess,
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
	server.SetTaskDistributorForTest(nil)
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
	assertWechatFailResponse(t, recorder, "refund result processing unavailable, please retry")
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

				request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/refund-notify", bytes.NewReader(bodyBytes))
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

// newTestServerWithEcommerceClient 创建带Mock EcommerceClient的测试服务器
func newTestServerWithEcommerceClient(t *testing.T, store db.Store, ecommerceClient *mockwechat.MockEcommerceClientInterface) *Server {
	server := newTestServer(t, store)

	// 替换为Mock客户端
	server.ecommerceClient = ecommerceClient

	return server
}

type ecommerceOrderPaymentFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	processPaymentFactApplication func(context.Context, *worker.PaymentFactApplicationPayload, ...asynq.Option) error
}

func (d *ecommerceOrderPaymentFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	if d.processPaymentFactApplication == nil {
		return nil
	}
	return d.processPaymentFactApplication(ctx, payload, opts...)
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

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/combine-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	return request
}

func newValidCombinePaymentNotification(combineOutTradeNo, outTradeNo, transactionID string, totalAmount int64) *wechatcontracts.CombinePaymentNotification {
	return &wechatcontracts.CombinePaymentNotification{
		CombineAppID:      "app_expected",
		CombineMchID:      "sp_expected",
		CombineOutTradeNo: combineOutTradeNo,
		CombinePayerInfo: &wechatcontracts.CombinePaymentNotificationPayerInfo{
			OpenID: "payer-openid",
		},
		SubOrders: []wechatcontracts.CombinePaymentNotificationSubOrder{{
			MchID:         "sub_expected",
			OutTradeNo:    outTradeNo,
			TransactionID: transactionID,
			TradeType:     "JSAPI",
			TradeState:    "SUCCESS",
			BankType:      "OTHERS",
			SuccessTime:   "2026-04-16T10:00:00+08:00",
			Amount: struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{
				TotalAmount:   totalAmount,
				PayerAmount:   totalAmount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}},
	}
}

func newEcommercePaymentNotifyRequest(t *testing.T, notificationID string) *http.Request {
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

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/payment-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	return request
}

func newValidPartnerPaymentNotificationResource(outTradeNo, transactionID, subMchID string, totalAmount int64) *wechatcontracts.PartnerPaymentNotificationResource {
	return &wechatcontracts.PartnerPaymentNotificationResource{
		SpMchID:        "sp_expected",
		SpAppID:        "app_expected",
		SubMchID:       subMchID,
		OutTradeNo:     outTradeNo,
		TransactionID:  transactionID,
		TradeType:      "JSAPI",
		TradeState:     "SUCCESS",
		TradeStateDesc: "success",
		BankType:       "OTHERS",
		SuccessTime:    "2026-04-16T10:00:00+08:00",
		Payer: wechatcontracts.PartnerOrderPayerInfo{
			SpOpenID: "payer-openid",
		},
		Amount: wechatcontracts.PartnerOrderQueryAmount{
			Total:         totalAmount,
			PayerTotal:    totalAmount,
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	}
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

func newEcommerceWithdrawNotifyRequest(t *testing.T, notificationID string) *http.Request {
	t.Helper()

	requestBody := map[string]interface{}{
		"id":            notificationID,
		"event_type":    "MCHWITHDRAW.CHANGE",
		"resource_type": "encrypt-resource",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_encrypted_data",
			"nonce":           "mock_nonce",
			"associated_data": "mch_withdraw",
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/withdraw-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	request.Header.Set("Wechatpay-Timestamp", "1234567890")
	request.Header.Set("Wechatpay-Nonce", "test_nonce")
	request.Header.Set("Wechatpay-Signature", "test_signature")
	request.Header.Set("Wechatpay-Serial", "test_serial")

	return request
}

func validWithdrawNotificationResource(t *testing.T, outRequestNo, withdrawID, status, reason string) []byte {
	t.Helper()

	payload := map[string]string{
		"sub_mchid":      "sub-mch-001",
		"withdraw_id":    withdrawID,
		"out_request_no": outRequestNo,
		"status":         status,
		"reason":         reason,
		"create_time":    "2026-04-06T10:00:00+08:00",
		"update_time":    "2026-04-06T10:05:00+08:00",
		"account_type":   "BASIC",
		"account_number": "6222020202020202",
		"account_bank":   "招商银行",
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func expectWithdrawCallbackFact(t *testing.T, store *mockdb.MockStore, notificationID string, record db.WithdrawalRecord, status string, outRequestNo string, withdrawID string, reason string) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
		require.Equal(t, notificationID, arg.SourceEventID.String)
		require.Equal(t, wechatcontracts.FundManagementNotificationEventType, arg.SourceEventType.String)
		require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
		require.Equal(t, outRequestNo, arg.ExternalObjectKey)
		require.Equal(t, withdrawID, arg.ExternalSecondaryKey.String)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.Equal(t, merchantWithdrawFactBusinessObject, arg.BusinessObjectType.String)
		require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, status, arg.UpstreamState)
		require.Equal(t, record.Amount, arg.Amount.Int64)
		require.Equal(t, fmt.Sprintf("wechat:callback:ecommerce:withdraw:%s", notificationID), arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		require.EqualValues(t, record.ID, payload["withdrawal_record_id"])
		require.Equal(t, outRequestNo, payload["out_request_no"])
		require.Equal(t, withdrawID, payload["withdraw_id"])
		require.Equal(t, status, payload["wechat_status"])
		require.Equal(t, reason, payload["reason"])
		return db.ExternalPaymentFact{ID: 9201, DedupeKey: arg.DedupeKey, IsTerminal: arg.IsTerminal, TerminalStatus: arg.TerminalStatus}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             9201,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 9202,
		FactID:             9201,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
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

func TestHandleEcommerceWithdrawNotify_SuccessUpdatesWithdrawal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-001",
		OutRequestNo: "MW202604060001",
	})
	require.NoError(t, err)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Return(validWithdrawNotificationResource(t, "MW202604060001", "wd_001", "SUCCESS", ""), nil)
	record := db.WithdrawalRecord{
		ID:          66,
		UserID:      99,
		Amount:      1200,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		Reason:      pgtype.Text{String: "query withdraw result failed: timeout", Valid: true},
		AccountInfo: accountInfoBytes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "MW202604060001", Valid: true}).Return(record, nil)
	expectWithdrawCallbackFact(t, store, notificationID, record, "SUCCESS", "MW202604060001", "wd_001", "")
	store.EXPECT().UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
		info := parseMerchantWithdrawAccountInfo(arg.AccountInfo)
		require.Equal(t, "wd_001", info.WithdrawID)
		return db.WithdrawalRecord{ID: 66, UserID: 99, Amount: 1200, Status: "pending", Channel: merchantWithdrawChannel, Reason: pgtype.Text{String: "query withdraw result failed: timeout", Valid: true}, AccountInfo: arg.AccountInfo, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	})
	expectMerchantWithdrawFactApplySuccess(t, store, 9202, 9201, db.WithdrawalRecord{ID: 66, UserID: 99, Amount: 1200, Status: "pending", Channel: merchantWithdrawChannel, Reason: pgtype.Text{String: "query withdraw result failed: timeout", Valid: true}, AccountInfo: accountInfoBytes, CreatedAt: time.Now(), UpdatedAt: time.Now()}, "MW202604060001", "wd_001", "SUCCESS", "", &db.WithdrawalRecord{ID: 66, UserID: 99, Amount: 1200, Status: "success", Channel: merchantWithdrawChannel, AccountInfo: accountInfoBytes, CreatedAt: time.Now(), UpdatedAt: time.Now()})

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommerceWithdrawNotify_AccountInfoPersistFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-001",
		OutRequestNo: "MW202604060001",
	})
	require.NoError(t, err)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Return(validWithdrawNotificationResource(t, "MW202604060001", "wd_001", "SUCCESS", ""), nil)
	record := db.WithdrawalRecord{
		ID:          66,
		UserID:      99,
		Amount:      1200,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "MW202604060001", Valid: true}).Return(record, nil)
	expectWithdrawCallbackFact(t, store, notificationID, record, "SUCCESS", "MW202604060001", "wd_001", "")
	store.EXPECT().UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).Return(db.WithdrawalRecord{}, errors.New("db unavailable"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "withdrawal sync failed")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestHandleEcommerceWithdrawNotify_StatusPersistFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-001",
		OutRequestNo: "MW202604060001",
	})
	require.NoError(t, err)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Return(validWithdrawNotificationResource(t, "MW202604060001", "wd_001", "SUCCESS", ""), nil)
	record := db.WithdrawalRecord{
		ID:          66,
		UserID:      99,
		Amount:      1200,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	updatedRecord := db.WithdrawalRecord{ID: 66, UserID: 99, Amount: 1200, Status: "pending", Channel: merchantWithdrawChannel, AccountInfo: accountInfoBytes, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	store.EXPECT().GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "MW202604060001", Valid: true}).Return(record, nil)
	expectWithdrawCallbackFact(t, store, notificationID, record, "SUCCESS", "MW202604060001", "wd_001", "")
	store.EXPECT().UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).Return(updatedRecord, nil)
	expectMerchantWithdrawFactApplyFailure(t, store, 9202, 9201, updatedRecord, "MW202604060001", "wd_001", "SUCCESS", "", errors.New("db unavailable"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "withdrawal sync failed")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestHandleEcommerceWithdrawNotify_WithdrawalNotFoundReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Return(validWithdrawNotificationResource(t, "MW404", "wd_404", "FAIL", "账户异常"), nil)
	store.EXPECT().GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "MW404", Valid: true}).Return(db.WithdrawalRecord{}, db.ErrRecordNotFound)
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "withdrawal record not found, please retry")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestHandleEcommerceWithdrawNotify_FactPersistFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-001",
		OutRequestNo: "MW202604060001",
	})
	require.NoError(t, err)

	record := db.WithdrawalRecord{
		ID:          66,
		UserID:      99,
		Amount:      1200,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Return(validWithdrawNotificationResource(t, "MW202604060001", "wd_001", "SUCCESS", ""), nil)
	store.EXPECT().GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "MW202604060001", Valid: true}).Return(record, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).Return(db.ExternalPaymentFact{}, errors.New("fact db unavailable"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)
	store.EXPECT().UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "withdrawal fact record failed")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestHandleEcommerceWithdrawNotifyIdempotency(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(false, nil)
	store.EXPECT().GetWechatNotification(gomock.Any(), notificationID).Return(db.WechatNotification{ID: notificationID, ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true}}, nil)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleEcommerceWithdrawNotify_StaleClaimReturnsFailAndReleases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(false, nil)
	store.EXPECT().GetWechatNotification(gomock.Any(), notificationID).Return(db.WechatNotification{
		ID:        notificationID,
		CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-notificationClaimStaleWindow - time.Second), Valid: true},
	}, nil)
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)
	ecommerceClient.EXPECT().DecryptNotificationRaw(gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req := newEcommerceWithdrawNotifyRequest(t, notificationID)
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "stale claim, please retry")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "非进件事件类型_忽略处理",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

func TestHandleApplymentStateNotify_AccountNeedVerifyRoutesToPendingFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:                 88,
		SubjectType:        "merchant",
		SubjectID:          200,
		OutRequestNo:       "APPLY_M_88_1234567890",
		SignUrl:            pgtype.Text{String: "https://sign.example.com/keep", Valid: true},
		SignState:          pgtype.Text{String: "UNSIGNED", Valid: true},
		LegalValidationUrl: pgtype.Text{String: "https://wx.example.com/legal-keep", Valid: true},
		AccountValidation:  wechat.MarshalEcommerceApplymentAccountValidation(&wechatcontracts.EcommerceApplymentAccountValidation{Remark: "keep-existing-validation"}),
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":88,"out_request_no":"APPLY_M_88_1234567890","applyment_state":"ACCOUNT_NEED_VERIFY"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_M_88_1234567890").
		Times(1).
		Return(applyment, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, notificationID, arg.SourceEventID.String)
			require.Equal(t, "APPLYMENT_STATE.CHANGE", arg.SourceEventType.String)
			require.Equal(t, db.ExternalPaymentObjectApplyment, arg.ExternalObjectType)
			require.Equal(t, applyment.OutRequestNo, arg.ExternalObjectKey)
			require.Equal(t, db.ExternalPaymentBusinessOwnerApplyment, arg.BusinessOwner.String)
			require.Equal(t, "ordinary_service_provider_applyment", arg.BusinessObjectType.String)
			require.Equal(t, applyment.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "ACCOUNT_NEED_VERIFY", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, arg.TerminalStatus)
			require.False(t, arg.IsTerminal)
			require.Contains(t, arg.DedupeKey, "wechat:callback:applyment:")
			return db.ExternalPaymentFact{ID: 7088, IsTerminal: false}, nil
		})

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             7088,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ordinary_service_provider_applyment",
			BusinessObjectID:   applyment.ID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 8088, FactID: 7088, Consumer: "applyment_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: applyment.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, []int64{8088}, taskDistributor.applicationIDs)
	assertWechatNoContentResponse(t, recorder)
}

func TestHandleApplymentStateNotify_WithoutTaskDistributorReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(nil)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:                 88,
		SubjectType:        "merchant",
		SubjectID:          200,
		OutRequestNo:       "APPLY_M_88_1234567890",
		SignUrl:            pgtype.Text{String: "https://sign.example.com/keep", Valid: true},
		SignState:          pgtype.Text{String: "UNSIGNED", Valid: true},
		LegalValidationUrl: pgtype.Text{String: "https://wx.example.com/legal-keep", Valid: true},
		AccountValidation:  wechat.MarshalEcommerceApplymentAccountValidation(&wechatcontracts.EcommerceApplymentAccountValidation{Remark: "keep-existing-validation"}),
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":88,"out_request_no":"APPLY_M_88_1234567890","applyment_state":"ACCOUNT_NEED_VERIFY"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_M_88_1234567890").
		Times(1).
		Return(applyment, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Times(1).
		Return(nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "applyment result processing unavailable, please retry")
}

func TestHandleApplymentStateNotify_EnqueueFailureReturnsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{err: errors.New("queue down")}
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:                 89,
		SubjectType:        "merchant",
		SubjectID:          201,
		OutRequestNo:       "APPLY_M_89_1234567890",
		SignUrl:            pgtype.Text{String: "https://sign.example.com/keep", Valid: true},
		SignState:          pgtype.Text{String: "UNSIGNED", Valid: true},
		LegalValidationUrl: pgtype.Text{String: "https://wx.example.com/legal-keep", Valid: true},
		AccountValidation:  wechat.MarshalEcommerceApplymentAccountValidation(&wechatcontracts.EcommerceApplymentAccountValidation{Remark: "keep-existing-validation"}),
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":89,"out_request_no":"APPLY_M_89_1234567890","applyment_state":"ACCOUNT_NEED_VERIFY"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_M_89_1234567890").
		Times(1).
		Return(applyment, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "ACCOUNT_NEED_VERIFY", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, arg.TerminalStatus)
			require.False(t, arg.IsTerminal)
			return db.ExternalPaymentFact{ID: 7089, IsTerminal: false}, nil
		})

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             7089,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ordinary_service_provider_applyment",
			BusinessObjectID:   applyment.ID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 8089, FactID: 7089, Consumer: "applyment_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: applyment.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	store.EXPECT().
		ReleaseWechatNotificationClaim(gomock.Any(), notificationID).
		Times(1).
		Return(nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, []int64{8089}, taskDistributor.applicationIDs)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assertWechatFailResponse(t, recorder, "enqueue failed, please retry")
}

func TestHandleApplymentStateNotify_FinishRoutesToFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:           90,
		SubjectType:  "merchant",
		SubjectID:    202,
		OutRequestNo: "APPLY_M_90_1234567890",
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":90,"out_request_no":"APPLY_M_90_1234567890","applyment_state":"FINISH","sub_mchid":"sub_mch_90"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_M_90_1234567890").
		Times(1).
		Return(applyment, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, notificationID, arg.SourceEventID.String)
			require.Equal(t, "APPLYMENT_STATE.CHANGE", arg.SourceEventType.String)
			require.Equal(t, db.ExternalPaymentObjectApplyment, arg.ExternalObjectType)
			require.Equal(t, applyment.OutRequestNo, arg.ExternalObjectKey)
			require.Equal(t, db.ExternalPaymentBusinessOwnerApplyment, arg.BusinessOwner.String)
			require.Equal(t, "ordinary_service_provider_applyment", arg.BusinessObjectType.String)
			require.Equal(t, applyment.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "FINISH", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Contains(t, arg.DedupeKey, "wechat:callback:applyment:")
			return db.ExternalPaymentFact{ID: 7090, IsTerminal: true}, nil
		})

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             7090,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ordinary_service_provider_applyment",
			BusinessObjectID:   applyment.ID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 8090, FactID: 7090, Consumer: "applyment_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: applyment.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, []int64{8090}, taskDistributor.applicationIDs)
	assertWechatNoContentResponse(t, recorder)
}

func TestHandleApplymentStateNotify_RejectedRoutesToFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	taskDistributor := &refundFactApplicationEnqueueRecorder{}
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(taskDistributor)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:           91,
		SubjectType:  "merchant",
		SubjectID:    303,
		OutRequestNo: "APPLY_M_91_1234567890",
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":91,"out_request_no":"APPLY_M_91_1234567890","applyment_state":"REJECTED"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_M_91_1234567890").
		Times(1).
		Return(applyment, nil)

	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, "REJECTED", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusFailed, arg.TerminalStatus)
			return db.ExternalPaymentFact{ID: 7091, IsTerminal: true}, nil
		})

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             7091,
			Consumer:           "applyment_domain",
			BusinessObjectType: "ordinary_service_provider_applyment",
			BusinessObjectID:   applyment.ID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{ID: 8091, FactID: 7091, Consumer: "applyment_domain", BusinessObjectType: "ordinary_service_provider_applyment", BusinessObjectID: applyment.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, []int64{8091}, taskDistributor.applicationIDs)
	assertWechatNoContentResponse(t, recorder)
}

func TestHandleApplymentStateNotify_IgnoresOperatorApplymentAfterRemoval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)

	notificationID := util.RandomString(32)
	applyment := db.EcommerceApplyment{
		ID:           108,
		SubjectType:  "operator",
		SubjectID:    320,
		OutRequestNo: "APPLY_OP_108_1234567890",
	}

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptNotificationRaw(gomock.Any()).
		Times(1).
		Return([]byte(`{"applyment_id":108,"out_request_no":"APPLY_OP_108_1234567890","applyment_state":"ACCOUNT_NEED_VERIFY","sub_mchid":"sub_mch_op_108"}`), nil)

	store.EXPECT().
		GetEcommerceApplymentByOutRequestNo(gomock.Any(), "APPLY_OP_108_1234567890").
		Times(1).
		Return(applyment, nil)

	recorder := httptest.NewRecorder()
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

	server.router.ServeHTTP(recorder, request)
	assertWechatNoContentResponse(t, recorder)
}

func TestResolveApplymentCallbackStatus(t *testing.T) {
	require.Equal(t, "auditing", resolveApplymentCallbackStatus("auditing", "NEW_UPSTREAM_STATE"))
	require.Equal(t, "account_need_verify", resolveApplymentCallbackStatus("auditing", "ACCOUNT_NEED_VERIFY"))
	require.Equal(t, "to_be_signed", resolveApplymentCallbackStatus("to_be_signed", "NEED_SIGN"))
}

func TestHandleProfitSharingNotify_WithoutTaskDistributorReturnsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(nil)

	notificationID := util.RandomString(32)
	outOrderNo := util.RandomString(16)

	ecommerceClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(true, nil)

	ecommerceClient.EXPECT().
		DecryptProfitSharingNotification(gomock.Any()).
		Times(1).
		Return(&wechatcontracts.ProfitSharingNotification{
			SPMchID:       "sp_mch_id",
			SubMchID:      "sub_mch_id",
			OutOrderNo:    outOrderNo,
			OrderID:       "wx_order_id",
			TransactionID: "wx_transaction_id",
		}, nil)

	ecommerceClient.EXPECT().
		GetSpMchID().
		Times(1).
		Return("sp_mch_id")

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

	ecommerceClient.EXPECT().
		QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Eq("wx_transaction_id"), gomock.Eq(outOrderNo)).
		Times(1).
		Return(&wechatcontracts.ProfitSharingQueryResponse{
			Status: wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{
				{Result: wechatcontracts.ProfitSharingResultSuccess, Amount: 1 * fenPerYuan},
			},
		}, nil)

	expectProfitSharingCallbackFact(t, store, notificationID, outOrderNo, 1, 1*fenPerYuan, "SUCCESS", db.ExternalPaymentTerminalStatusSuccess, true)

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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func expectProfitSharingCallbackFact(t *testing.T, store *mockdb.MockStore, notificationID, outOrderNo string, profitSharingOrderID, amount int64, upstreamState, terminalStatus string, isTerminal bool) {
	t.Helper()

	factID := int64(7000) + profitSharingOrderID
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, notificationID, arg.SourceEventID.String)
			require.Equal(t, "TRANSACTION.SUCCESS", arg.SourceEventType.String)
			require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
			require.Equal(t, outOrderNo, arg.ExternalObjectKey)
			require.Equal(t, "wx_order_id", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectProfitSharingOrder, arg.BusinessObjectType.String)
			require.Equal(t, profitSharingOrderID, arg.BusinessObjectID.Int64)
			require.Equal(t, upstreamState, arg.UpstreamState)
			require.Equal(t, terminalStatus, arg.TerminalStatus)
			require.Equal(t, amount, arg.Amount.Int64)
			require.Equal(t, "CNY", arg.Currency)
			require.Equal(t, "wechat:callback:ecommerce:profit_sharing:"+notificationID, arg.DedupeKey)
			raw := string(arg.RawResource)
			require.Contains(t, raw, "receiver_results")
			require.NotContains(t, raw, "receiver_account")
			require.NotContains(t, raw, "account\":")
			return db.ExternalPaymentFact{ID: factID, DedupeKey: arg.DedupeKey, IsTerminal: isTerminal}, nil
		})

	if !isTerminal {
		return
	}

	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             factID,
			Consumer:           paymentFactConsumerProfitSharingDomain,
			BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
			BusinessObjectID:   profitSharingOrderID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).
		Return(db.ExternalPaymentFactApplication{
			ID:                 factID + 1000,
			FactID:             factID,
			Consumer:           paymentFactConsumerProfitSharingDomain,
			BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
			BusinessObjectID:   profitSharingOrderID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}, nil)
}

func TestHandleProfitSharingNotify_UnsupportedReceiverResultFallsBackToProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerceClient(t, store, ecommerceClient)
	server.SetTaskDistributorForTest(nil)

	notificationID := util.RandomString(32)
	outOrderNo := util.RandomString(16)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptProfitSharingNotification(gomock.Any()).Return(&wechatcontracts.ProfitSharingNotification{
		SPMchID:       "sp_mch_id",
		SubMchID:      "sub_mch_id",
		OutOrderNo:    outOrderNo,
		OrderID:       "wx_order_id",
		TransactionID: "wx_transaction_id",
	}, nil)
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_mch_id")
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).Return(db.ProfitSharingOrder{
		ID:         4,
		MerchantID: 24,
		OutOrderNo: outOrderNo,
		Status:     "processing",
	}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), gomock.Eq(int64(24))).Return(db.MerchantPaymentConfig{MerchantID: 24, SubMchID: "sub_mch_id", Status: "active"}, nil)
	ecommerceClient.EXPECT().QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Eq("wx_transaction_id"), gomock.Eq(outOrderNo)).Return(&wechatcontracts.ProfitSharingQueryResponse{
		Status: wechatcontracts.ProfitSharingStatusFinished,
		Receivers: []wechatcontracts.ProfitSharingReceiverResult{
			{Result: "UNSUPPORTED_RESULT", Amount: 2 * fenPerYuan, DetailID: "detail_unsupported"},
		},
	}, nil)
	expectProfitSharingCallbackFact(t, store, notificationID, outOrderNo, 4, 2*fenPerYuan, "PROCESSING", db.ExternalPaymentTerminalStatusProcessing, false)

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

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

// TestHandleProfitSharingNotifyIdempotency 测试分账回调的幂等性检查
func TestHandleProfitSharingNotifyIdempotency(t *testing.T) {
	notificationID := util.RandomString(32)
	outOrderNo := util.RandomString(16)

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor)
		setupRequest  func(t *testing.T) *http.Request
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "重复分账通知_直接返回SUCCESS",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				// 先验证签名
				ecommerceClient.EXPECT().
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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "分账成功通知_更新订单状态为finished",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
					Return(&wechatcontracts.ProfitSharingNotification{
						SPMchID:       "sp_mch_id",
						SubMchID:      "sub_mch_id",
						OutOrderNo:    outOrderNo,
						OrderID:       "wx_order_id",
						TransactionID: "wx_transaction_id",
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				ecommerceClient.EXPECT().
					QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Eq("wx_transaction_id"), gomock.Eq(outOrderNo)).
					Times(1).
					Return(&wechatcontracts.ProfitSharingQueryResponse{
						Status: wechatcontracts.ProfitSharingStatusFinished,
						Receivers: []wechatcontracts.ProfitSharingReceiverResult{
							{Result: wechatcontracts.ProfitSharingResultSuccess, Amount: 1 * fenPerYuan},
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

				expectProfitSharingCallbackFact(t, store, notificationID, outOrderNo, 1, 1*fenPerYuan, "SUCCESS", db.ExternalPaymentTerminalStatusSuccess, true)

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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "分账失败通知_更新订单状态为failed",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.ProfitSharingNotification{
						SPMchID:       "sp_mch_id",
						SubMchID:      "sub_mch_id",
						OutOrderNo:    outOrderNo,
						OrderID:       "wx_order_id",
						TransactionID: "wx_transaction_id",
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				ecommerceClient.EXPECT().
					QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Eq("wx_transaction_id"), gomock.Eq(outOrderNo)).
					Times(1).
					Return(&wechatcontracts.ProfitSharingQueryResponse{
						Status: wechatcontracts.ProfitSharingStatusFinished,
						Receivers: []wechatcontracts.ProfitSharingReceiverResult{
							{Result: wechatcontracts.ProfitSharingResultClosed, FailReason: wechatcontracts.ProfitSharingFailReasonNoRelation, Amount: 1 * fenPerYuan},
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

				expectProfitSharingCallbackFact(t, store, notificationID, outOrderNo, 1, 1*fenPerYuan, "FAILED", db.ExternalPaymentTerminalStatusFailed, true)

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
				assertWechatNoContentResponse(t, recorder)
			},
		},
		{
			name: "分账查询失败_返回FAIL等待重试",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.ProfitSharingNotification{
						SPMchID:       "sp_mch_id",
						SubMchID:      "sub_mch_id",
						OutOrderNo:    outOrderNo,
						OrderID:       "wx_order_id",
						TransactionID: "wx_transaction_id",
					}, nil)

				ecommerceClient.EXPECT().
					GetSpMchID().
					Times(1).
					Return("sp_mch_id")

				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), gomock.Eq(outOrderNo)).
					Times(1).
					Return(db.ProfitSharingOrder{
						ID:         3,
						MerchantID: 23,
						OutOrderNo: outOrderNo,
						Status:     "processing",
					}, nil)

				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), gomock.Eq(int64(23))).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 23, SubMchID: "sub_mch_id", Status: "active"}, nil)

				ecommerceClient.EXPECT().
					QueryProfitSharing(gomock.Any(), gomock.Eq("sub_mch_id"), gomock.Eq("wx_transaction_id"), gomock.Eq(outOrderNo)).
					Times(1).
					Return(nil, errors.New("query failed"))

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

				var response map[string]string
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "FAIL", response["code"])
				require.Equal(t, "query profit sharing failed", response["message"])
			},
		},
		{
			name: "无EcommerceClient_返回500",
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
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
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, taskDistributor *mockwk.MockTaskDistributor) {
				ecommerceClient.EXPECT().
					VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					TryClaimWechatNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(true, nil)

				ecommerceClient.EXPECT().
					DecryptProfitSharingNotification(gomock.Any()).
					Times(1).
					Return(&wechatcontracts.ProfitSharingNotification{
						SPMchID:       "sp_mch_id",
						SubMchID:      "sub_mch_wrong",
						OutOrderNo:    outOrderNo,
						OrderID:       "wx_order_id",
						TransactionID: "wx_transaction_id",
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

			store := newMockStoreWithAlertSink(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, ecommerceClient, taskDistributor)

			var server *Server
			if tc.name == "无EcommerceClient_返回500" {
				server = newTestServer(t, store)
			} else {
				server = newTestServerWithEcommerceClient(t, store, ecommerceClient)
				server.SetTaskDistributorForTest(taskDistributor)
			}
			recorder := httptest.NewRecorder()

			request := tc.setupRequest(t)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
