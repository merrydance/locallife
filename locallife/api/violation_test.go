package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleViolationNotify_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1712808000"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptViolationNotification(gomock.Any()).Return(&wechat.ViolationNotificationResource{
		SubMchID:        "sub-mch-001",
		CompanyName:     "测试商户",
		RecordID:        "record-001",
		RiskType:        "ONE_YUAN_PURCHASES",
		RiskDescription: "涉嫌一元购",
	}, nil)
	store.EXPECT().GetMerchantPaymentConfigBySubMchID(gomock.Any(), "sub-mch-001").Return(db.MerchantPaymentConfig{
		MerchantID: 42,
		SubMchID:   "sub-mch-001",
	}, nil)
	store.EXPECT().UpsertWechatMerchantViolation(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertWechatMerchantViolationParams) (db.WechatMerchantViolation, error) {
		require.Equal(t, "record-001", arg.RecordID)
		require.True(t, arg.MerchantID.Valid)
		require.Equal(t, int64(42), arg.MerchantID.Int64)
		return db.WechatMerchantViolation{RecordID: arg.RecordID}, nil
	})

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, notificationID, "VIOLATION.PUNISH")
	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleViolationNotify_InvalidSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(wechat.ErrInvalidSignature)

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, util.RandomString(32), "VIOLATION.PUNISH")
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "signature verification failed")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestHandleViolationNotify_DuplicateProcessedReturnsSuccess(t *testing.T) {
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
		ID:          notificationID,
		ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
	}, nil)

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, notificationID, "VIOLATION.PUNISH")
	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleViolationNotify_UnsupportedEventMarksProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptViolationNotification(gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, util.RandomString(32), "VIOLATION.UNKNOWN")
	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleViolationNotify_DecryptFailureReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptViolationNotification(gomock.Any()).Return(nil, errors.New("decrypt failed"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, notificationID, "VIOLATION.INTERCEPT")
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "decrypt failed")
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestHandleViolationNotify_PersistFailureReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	notificationID := util.RandomString(32)
	ecommerceClient.EXPECT().VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	ecommerceClient.EXPECT().DecryptViolationNotification(gomock.Any()).Return(&wechat.ViolationNotificationResource{
		SubMchID: "sub-mch-001",
		RecordID: "record-001",
	}, nil)
	store.EXPECT().GetMerchantPaymentConfigBySubMchID(gomock.Any(), "sub-mch-001").Return(db.MerchantPaymentConfig{MerchantID: 42, SubMchID: "sub-mch-001"}, nil)
	store.EXPECT().UpsertWechatMerchantViolation(gomock.Any(), gomock.Any()).Return(db.WechatMerchantViolation{}, errors.New("db down"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req := newViolationNotifyRequest(t, notificationID, "VIOLATION.INTERCEPT")
	server.router.ServeHTTP(recorder, req)

	assertWechatFailResponse(t, recorder, "persist violation failed")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestGetPlatformViolationNotificationConfig_NotConfigured(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ecommerceClient.EXPECT().QueryViolationNotification(gomock.Any()).Return(nil, fmt.Errorf("query violation notification: %w", &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Message: "not found"}))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/violation-notification", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp platformViolationNotificationConfigResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.False(t, resp.Configured)
	require.Equal(t, server.config.EffectiveWechatEcommerceViolationNotifyURL(), resp.EffectiveNotifyURL)
}

func TestCreatePlatformViolationNotificationConfig(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	expectedURL := "https://api.example.com/v1/webhooks/wechat-ecommerce/violation-notify"
	ecommerceClient.EXPECT().CreateViolationNotification(gomock.Any(), &wechat.ViolationNotificationConfigRequest{NotifyURL: &expectedURL}).Return(&wechat.ViolationNotificationConfigResponse{NotifyURL: &expectedURL}, nil)

	body, err := json.Marshal(platformViolationNotificationConfigRequest{NotifyURL: &expectedURL})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ecommerce/violation-notification", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp platformViolationNotificationConfigResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.Configured)
	require.NotNil(t, resp.NotifyURL)
	require.Equal(t, expectedURL, *resp.NotifyURL)
}

func TestListPlatformWechatMerchantViolations(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().CountWechatMerchantViolations(gomock.Any(), gomock.Any()).Return(int64(1), nil)
	store.EXPECT().ListWechatMerchantViolations(gomock.Any(), gomock.Any()).Return([]db.WechatMerchantViolation{{
		ID:                   1,
		RecordID:             "record-001",
		SubMchID:             "sub-mch-001",
		MerchantID:           pgtype.Int8{Int64: 42, Valid: true},
		CompanyName:          "测试商户",
		EventType:            "VIOLATION.PUNISH",
		RiskType:             "ONE_YUAN_PURCHASES",
		RiskDescription:      "涉嫌一元购",
		PunishPlan:           "冻结收款",
		LatestNotificationID: "notify-001",
		LatestNotifyTime:     time.Now(),
		LastReceivedAt:       time.Now(),
		CreatedAt:            time.Now(),
		UpdatedAt:            pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/violations?page=1&limit=20&event_type=violation.punish", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp listPlatformWechatMerchantViolationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Violations, 1)
	require.Equal(t, "VIOLATION.PUNISH", resp.Violations[0].EventType)
	if resp.Violations[0].MerchantID == nil {
		t.Fatal("expected merchant_id")
	}
	require.Equal(t, int64(42), *resp.Violations[0].MerchantID)
}

func TestGetPlatformWechatMerchantViolation(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetWechatMerchantViolationByRecordID(gomock.Any(), "record-001").Return(db.WechatMerchantViolation{
		ID:                   1,
		RecordID:             "record-001",
		SubMchID:             "sub-mch-001",
		CompanyName:          "测试商户",
		EventType:            "VIOLATION.APPEAL",
		RiskType:             "ONE_YUAN_PURCHASES",
		LatestNotificationID: "notify-001",
		LatestNotifyTime:     time.Now(),
		LastReceivedAt:       time.Now(),
		CreatedAt:            time.Now(),
	}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/violations/record-001", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp platformWechatMerchantViolationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "record-001", resp.RecordID)
	require.Equal(t, "VIOLATION.APPEAL", resp.EventType)
}

func newViolationNotifyRequest(t *testing.T, notificationID, eventType string) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"id":            notificationID,
		"create_time":   "2026-04-11T12:00:00+08:00",
		"event_type":    eventType,
		"resource_type": "encrypt-resource",
		"summary":       "merchant violation notify",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "ciphertext",
			"nonce":           "nonce-value",
			"associated_data": "violation",
			"original_type":   "violation",
		},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/violation-notify", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")
	req.Header.Set("Wechatpay-Timestamp", "1712808000")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	return req
}
