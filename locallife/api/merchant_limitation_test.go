package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantAccountLimitationsNotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/limitations", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp accountLimitationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "not_configured", resp.AccountStatus)
	require.Empty(t, resp.LimitedFunctions)
	require.Empty(t, resp.MchID)
}

func TestGetMerchantAccountLimitationsOK(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(paymentConfig, nil)
	ecommerce.EXPECT().QuerySubMerchantLimitations(gomock.Any(), paymentConfig.SubMchID).Return(&wechat.SubMerchantLimitationsResponse{
		MchID:                 paymentConfig.SubMchID,
		LimitedFunctions:      []string{"NO_TRANSACTION_AND_RECHARGE", "NO_REFUND"},
		OtherLimitedFunctions: "关闭相册扫码支付",
		RecoverySpecifications: []wechat.SubMerchantLimitationRecoverySpecification{{
			LimitationCaseID:         "A20250819155047774441874",
			LimitationReasonType:     "LICENSE_ABNORMAL",
			LimitationReason:         "入驻后180天无账户动账",
			LimitationReasonDescribe: "当前商户号入驻后长时间无账户动账",
			RelateLimitations:        []string{"NO_TRANSACTION_AND_RECHARGE"},
			RecoverWay:               "MODIFY_SUBJECT_INFORMATION",
			RecoverWayParam:          "100200300112233",
			RecoverHelpURL:           "https://kf.qq.com",
			LimitationActionType:     "LIMIT_ACTION_TYPE_IMMEDIATE_CONTROL",
			LimitationStartDate:      "2025-06-08T10:34:56+08:00",
			LimitationDate:           "2025-06-08T10:34:56+08:00",
		}},
	}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/limitations", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp accountLimitationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "active", resp.AccountStatus)
	require.Equal(t, paymentConfig.SubMchID, resp.MchID)
	require.Equal(t, []string{"NO_TRANSACTION_AND_RECHARGE", "NO_REFUND"}, resp.LimitedFunctions)
	require.Equal(t, "关闭相册扫码支付", resp.OtherLimitedFunctions)
	require.Len(t, resp.RecoverySpecifications, 1)
	require.Equal(t, "LICENSE_ABNORMAL", resp.RecoverySpecifications[0].LimitationReasonType)
	require.Equal(t, "MODIFY_SUBJECT_INFORMATION", resp.RecoverySpecifications[0].RecoverWay)
}

func TestGetOperatorAccountLimitationsOK(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 1001, UserID: user.ID, RegionID: 1, Status: "active", SubMchID: pgtype.Text{String: "sub_mch_op_001", Valid: true}}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().ListUserRoles(gomock.Any(), user.ID).Return([]db.UserRole{{
		UserID:          user.ID,
		Role:            "operator",
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
	}}, nil)
	store.EXPECT().GetOperatorByUser(gomock.Any(), user.ID).Return(operator, nil)
	ecommerce.EXPECT().QuerySubMerchantLimitations(gomock.Any(), operator.SubMchID.String).Return(&wechat.SubMerchantLimitationsResponse{
		MchID:            operator.SubMchID.String,
		LimitedFunctions: []string{"NO_PAYMENT"},
	}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/limitations", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp accountLimitationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "active", resp.AccountStatus)
	require.Equal(t, operator.SubMchID.String, resp.MchID)
	require.Equal(t, []string{"NO_PAYMENT"}, resp.LimitedFunctions)
}

func TestGetPlatformSubMerchantLimitations(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ecommerce.EXPECT().QuerySubMerchantLimitations(gomock.Any(), "1900000109").Return(&wechat.SubMerchantLimitationsResponse{
		MchID:                 "1900000109",
		LimitedFunctions:      []string{"NO_WITHDRAWAL"},
		OtherLimitedFunctions: "关闭长按识别支付",
	}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/merchant-limitations/1900000109", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformSubMerchantLimitationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "1900000109", resp.MchID)
	require.Equal(t, []string{"NO_WITHDRAWAL"}, resp.LimitedFunctions)
	require.Equal(t, "关闭长按识别支付", resp.OtherLimitedFunctions)
}

func TestGetPlatformSubMerchantLimitationsBadGateway(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().ListUserRoles(gomock.Any(), admin.ID).Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	ecommerce.EXPECT().QuerySubMerchantLimitations(gomock.Any(), "1900000109").Return(nil, fmt.Errorf("query sub merchant limitations: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid relationship"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/wechat-ecommerce/merchant-limitations/1900000109", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
}
