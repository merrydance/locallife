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
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockosp "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"

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

func TestHandleOrdinaryServiceProviderViolationNotify_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	notificationID := util.RandomString(32)
	createTime := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	plaintext := `{"sub_mchid":"sub-mch-ordinary","company_name":"普通服务商商户","record_id":"ordinary-record-001","risk_type":"ABNORMAL_TRANSACTION","risk_description":"交易异常","punish_plan":"限制退款"}`
	ordinaryClient.EXPECT().
		ParseNotification(gomock.Any(), gomock.Any(), ordinaryserviceprovider.NotificationTargetMerchantViolation).
		Return(&ordinaryserviceprovider.NotificationEnvelope{
			ID:           notificationID,
			CreateTime:   &createTime,
			EventType:    "VIOLATION.PUNISH",
			ResourceType: "encrypt-resource",
			Summary:      "ordinary merchant violation notify",
			Plaintext:    plaintext,
		}, nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	store.EXPECT().GetMerchantPaymentConfigBySubMchID(gomock.Any(), "sub-mch-ordinary").Return(db.MerchantPaymentConfig{
		MerchantID: 52,
		SubMchID:   "sub-mch-ordinary",
	}, nil)
	store.EXPECT().UpsertWechatMerchantViolation(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpsertWechatMerchantViolationParams) (db.WechatMerchantViolation, error) {
		require.Equal(t, "ordinary-record-001", arg.RecordID)
		require.Equal(t, "sub-mch-ordinary", arg.SubMchID)
		require.True(t, arg.MerchantID.Valid)
		require.Equal(t, int64(52), arg.MerchantID.Int64)
		require.Equal(t, "VIOLATION.PUNISH", arg.EventType)
		require.Equal(t, "ABNORMAL_TRANSACTION", arg.RiskType)
		return db.WechatMerchantViolation{RecordID: arg.RecordID}, nil
	})

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ordinary/violation-notify", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)

	server.router.ServeHTTP(recorder, req)

	assertWechatNoContentResponse(t, recorder)
}

func TestHandleOrdinaryServiceProviderViolationNotify_MissingSubMchIDReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	notificationID := util.RandomString(32)
	plaintext := `{"company_name":"普通服务商商户","record_id":"ordinary-record-001","risk_type":"ABNORMAL_TRANSACTION","risk_description":"交易异常","punish_plan":"限制退款"}`
	ordinaryClient.EXPECT().
		ParseNotification(gomock.Any(), gomock.Any(), ordinaryserviceprovider.NotificationTargetMerchantViolation).
		Return(&ordinaryserviceprovider.NotificationEnvelope{
			ID:           notificationID,
			EventType:    "VIOLATION.PUNISH",
			ResourceType: "encrypt-resource",
			Summary:      "ordinary merchant violation notify",
			Plaintext:    plaintext,
		}, nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ordinary/violation-notify", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assertWechatFailResponse(t, recorder, "parse resource failed")
}

func TestHandleOrdinaryServiceProviderViolationNotify_InvalidPunishTimeReleasesClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	notificationID := util.RandomString(32)
	plaintext := `{"sub_mchid":"sub-mch-ordinary","company_name":"普通服务商商户","record_id":"ordinary-record-001","risk_type":"ABNORMAL_TRANSACTION","risk_description":"交易异常","punish_plan":"限制退款","punish_time":"2026/04/11 12:00:00"}`
	ordinaryClient.EXPECT().
		ParseNotification(gomock.Any(), gomock.Any(), ordinaryserviceprovider.NotificationTargetMerchantViolation).
		Return(&ordinaryserviceprovider.NotificationEnvelope{
			ID:           notificationID,
			EventType:    "VIOLATION.PUNISH",
			ResourceType: "encrypt-resource",
			Summary:      "ordinary merchant violation notify",
			Plaintext:    plaintext,
		}, nil)
	store.EXPECT().TryClaimWechatNotification(gomock.Any(), gomock.Any()).Return(true, nil)
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), notificationID).Return(nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ordinary/violation-notify", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assertWechatFailResponse(t, recorder, "parse resource failed")
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

func TestGetPlatformViolationNotificationConfig_EcommerceClientUnavailable(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/violation-notification", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, platformViolationServiceUnavailableMessage, resp.Message)
}

func TestCreatePlatformViolationNotificationConfig_EcommerceRouteIgnoresOrdinaryClient(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerceClient)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	expectedURL := "https://api.example.com/v1/webhooks/wechat-ecommerce/violation-notify"
	ecommerceClient.EXPECT().CreateViolationNotification(gomock.Any(), &wechat.ViolationNotificationConfigRequest{NotifyURL: &expectedURL}).Return(&wechat.ViolationNotificationConfigResponse{NotifyURL: &expectedURL}, nil)
	ordinaryClient.EXPECT().CreateViolationNotificationConfig(gomock.Any(), gomock.Any()).Times(0)

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

func TestCreatePlatformViolationNotificationConfig_OrdinaryServiceProvider(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	expectedURL := "https://api.example.com/v1/webhooks/wechat-ordinary/violation-notify"
	ordinaryClient.EXPECT().
		CreateViolationNotificationConfig(gomock.Any(), ospcontracts.ViolationNotificationConfigRequest{NotifyURL: expectedURL}).
		Return(&ospcontracts.ViolationNotificationConfigResponse{NotifyURL: expectedURL}, nil)

	body, err := json.Marshal(platformViolationNotificationConfigRequest{NotifyURL: &expectedURL})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/violation-notification", bytes.NewReader(body))
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

func TestCreatePlatformViolationNotificationConfig_OrdinaryProviderFailureReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	expectedURL := "https://api.example.com/v1/webhooks/wechat-ordinary/violation-notify"
	ordinaryClient.EXPECT().
		CreateViolationNotificationConfig(gomock.Any(), ospcontracts.ViolationNotificationConfigRequest{NotifyURL: expectedURL}).
		Return(nil, errors.New("provider down"))

	body, err := json.Marshal(platformViolationNotificationConfigRequest{NotifyURL: &expectedURL})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/violation-notification", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Contains(t, resp.Message, "普通服务商处置通知配置失败")
	require.Contains(t, resp.Message, "稍后重试")
}

func TestCreateOrdinaryViolationNotificationConfig_InvalidNotifyURLReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().CreateViolationNotificationConfig(gomock.Any(), gomock.Any()).Times(0)

	body := []byte(`{"notify_url":"not-a-url"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/violation-notification", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "处置通知回调地址格式无效，请填写 HTTPS 回调地址后重试", resp.Message)
	require.NotContains(t, resp.Message, "NotifyURL")
	require.NotContains(t, resp.Message, "binding")
	require.NotContains(t, resp.Message, "provider")
	require.NotContains(t, resp.Message, "request_id")
}

func TestGetPlatformViolationNotificationConfig_OrdinaryProviderFailureReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		QueryViolationNotificationConfig(gomock.Any()).
		Return(nil, errors.New("provider down"))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ordinary/violation-notification", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Contains(t, resp.Message, "普通服务商处置通知配置查询失败")
	require.Contains(t, resp.Message, "稍后重试")
}

func TestDeletePlatformViolationNotificationConfig_OrdinaryProviderFailureReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		DeleteViolationNotificationConfig(gomock.Any()).
		Return(errors.New("provider down"))

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodDelete, "/v1/platform/finance/wechat-ordinary/violation-notification", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Contains(t, resp.Message, "普通服务商处置通知配置删除失败")
	require.Contains(t, resp.Message, "稍后重试")
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

func TestListOrdinaryWechatMerchantViolations_InvalidLimitReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().CountWechatMerchantViolations(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ListWechatMerchantViolations(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ordinary/violations?limit=101", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "每页最多查询 100 条违规记录，请缩小分页范围后重试", resp.Message)
	require.NotContains(t, resp.Message, "limit must")
	require.NotContains(t, resp.Message, "sql")
	require.NotContains(t, resp.Message, "provider")
}

func TestGetOrdinaryMerchantLimitationDiagnostic(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		QueryMerchantLimitation(gomock.Any(), ospcontracts.MerchantLimitationQueryRequest{SubMchID: "sub-mch-ordinary"}).
		Return(&ospcontracts.MerchantLimitationQueryResponse{
			SubMchID:         "sub-mch-ordinary",
			MchID:            "1900000109",
			LimitedFunctions: []ospcontracts.MerchantLimitedFunction{ospcontracts.MerchantLimitedNoRefund},
			RecoverySpecifications: []ospcontracts.MerchantRecoverySpecification{{
				LimitationCaseID: "case-001",
				RecoverWay:       "VERIFY_INACTIVE_MERCHANT_IDENTITY",
				RecoverHelpURL:   "https://pay.weixin.qq.com/help",
			}},
		}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ordinary/merchant-limitations/sub-mch-ordinary", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ordinaryMerchantLimitationDiagnosticResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "sub-mch-ordinary", resp.SubMchID)
	require.Equal(t, "1900000109", resp.MchID)
	require.Equal(t, []string{"NO_REFUND"}, resp.LimitedFunctions)
	require.True(t, resp.CanVerifyInactiveMerchantIdentity)
	require.Contains(t, resp.InactiveMerchantIdentityActionGuide, "平台管理员")
	require.Contains(t, resp.MerchantControlActionGuide, "不要重复发起")
}

func TestCreateInactiveMerchantIdentityVerification_GuardedByRecoverWay(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		QueryMerchantLimitation(gomock.Any(), ospcontracts.MerchantLimitationQueryRequest{SubMchID: "sub-mch-ordinary"}).
		Return(&ospcontracts.MerchantLimitationQueryResponse{
			SubMchID: "sub-mch-ordinary",
			RecoverySpecifications: []ospcontracts.MerchantRecoverySpecification{{
				RecoverWay: "CONTACT_WECHAT_PAY",
			}},
		}, nil)
	ordinaryClient.EXPECT().CreateInactiveMerchantIdentityVerification(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/merchant-limitations/sub-mch-ordinary/inactive-identity-verifications", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Contains(t, resp.Message, "未返回不活跃商户身份核实解脱路径")
}

func TestCreateInactiveMerchantIdentityVerification_Success(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		QueryMerchantLimitation(gomock.Any(), ospcontracts.MerchantLimitationQueryRequest{SubMchID: "sub-mch-ordinary"}).
		Return(&ospcontracts.MerchantLimitationQueryResponse{
			SubMchID: "sub-mch-ordinary",
			RecoverySpecifications: []ospcontracts.MerchantRecoverySpecification{{
				RecoverWay: "VERIFY_INACTIVE_MERCHANT_IDENTITY",
			}},
		}, nil)
	ordinaryClient.EXPECT().
		CreateInactiveMerchantIdentityVerification(gomock.Any(), ospcontracts.InactiveMerchantIdentityVerificationCreateRequest{
			SubMchID:     "sub-mch-ordinary",
			BusinessCode: "biz-001",
		}).
		Return(&ospcontracts.InactiveMerchantIdentityVerificationCreateResponse{VerificationID: "verify-001"}, nil)

	body, err := json.Marshal(createInactiveMerchantIdentityVerificationRequest{BusinessCode: "biz-001"})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/merchant-limitations/sub-mch-ordinary/inactive-identity-verifications", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp inactiveMerchantIdentityVerificationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "sub-mch-ordinary", resp.SubMchID)
	require.Equal(t, "verify-001", resp.VerificationID)
	require.Contains(t, resp.ActionGuide, "等待微信支付处理")
}

func TestCreateInactiveMerchantIdentityVerification_InvalidJSONReturnsChineseGuidance(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().QueryMerchantLimitation(gomock.Any(), gomock.Any()).Times(0)
	ordinaryClient.EXPECT().CreateInactiveMerchantIdentityVerification(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/wechat-ordinary/merchant-limitations/sub-mch-ordinary/inactive-identity-verifications", bytes.NewReader([]byte(`{`)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "身份核实请求格式无效，请检查填写内容后重试", resp.Message)
	require.NotContains(t, resp.Message, "invalid character")
	require.NotContains(t, resp.Message, "provider")
	require.NotContains(t, resp.Message, "request_id")
}

func TestGetInactiveMerchantIdentityVerification(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ordinaryClient.EXPECT().
		QueryInactiveMerchantIdentityVerification(gomock.Any(), ospcontracts.InactiveMerchantIdentityVerificationQueryRequest{
			SubMchID:       "sub-mch-ordinary",
			VerificationID: "verify-001",
		}).
		Return(&ospcontracts.InactiveMerchantIdentityVerificationQueryResponse{
			SubMchID:       "sub-mch-ordinary",
			VerificationID: "verify-001",
			State:          ospcontracts.InactiveMerchantIdentityVerificationProcessing,
			Reason:         "waiting for wechat review",
		}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ordinary/merchant-limitations/sub-mch-ordinary/inactive-identity-verifications/verify-001", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp inactiveMerchantIdentityVerificationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "sub-mch-ordinary", resp.SubMchID)
	require.Equal(t, "verify-001", resp.VerificationID)
	require.Equal(t, "PROCESSING", resp.State)
	require.Contains(t, resp.ActionGuide, "处理中请等待")
	require.NotContains(t, resp.ActionGuide, "PROCESSING")
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
