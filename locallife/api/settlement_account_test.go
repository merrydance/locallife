package api

import (
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

// ==================== 商户结算账户测试 ====================

func TestGetMerchantSettlementAccountNotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.MerchantPaymentConfig{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "not_configured", resp.AccountStatus)
	require.Nil(t, resp.Account)
}

func TestGetMerchantSettlementAccountInactive(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	paymentConfig := db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_123",
		Status:     "pending",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(paymentConfig, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "inactive", resp.AccountStatus)
	require.Nil(t, resp.Account)
}

func TestGetMerchantSettlementAccountOK(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	paymentConfig := db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_123",
		Status:     "active",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(paymentConfig, nil)

	ecommerce.EXPECT().
		QuerySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, "").
		Return(&wechat.SubMerchantSettlementResponse{
			AccountType:   "BASIC",
			AccountBank:   "工商银行",
			BankName:      "工商银行北京分行",
			AccountNumber: "6222****8888",
			VerifyResult:  "PASSED",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "active", resp.AccountStatus)
	require.NotNil(t, resp.Account)
	require.Equal(t, "BASIC", resp.Account.AccountType)
	require.Equal(t, "工商银行", resp.Account.AccountBank)
	require.Equal(t, "6222****8888", resp.Account.AccountNumber)
	require.Equal(t, "PASSED", resp.Account.VerifyResult)
}

// ==================== 运营商结算账户测试 ====================

func TestGetOperatorSettlementAccountNotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		// SubMchID not set → not_configured
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "not_configured", resp.AccountStatus)
	require.Nil(t, resp.Account)
}

func TestGetOperatorSettlementAccountOK(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		SubMchID: pgtype.Text{String: "sub_mch_op_001", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	ecommerce.EXPECT().
		QuerySubMerchantSettlement(gomock.Any(), operator.SubMchID.String, "").
		Return(&wechat.SubMerchantSettlementResponse{
			AccountType:   "BASIC",
			AccountBank:   "建设银行",
			BankName:      "建设银行上海分行",
			AccountNumber: "6217****5678",
			VerifyResult:  "PASSED",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "active", resp.AccountStatus)
	require.NotNil(t, resp.Account)
	require.Equal(t, "BASIC", resp.Account.AccountType)
	require.Equal(t, "建设银行", resp.Account.AccountBank)
	require.Equal(t, "6217****5678", resp.Account.AccountNumber)
	require.Equal(t, "PASSED", resp.Account.VerifyResult)
}

// TestGetOperatorSettlementAccountBindbankSubmitted 验证 bindbank_submitted 状态运营商被拒
// findign-1 修复：与 getOperatorAccountBalance 保持一致，仅 active 状态允许查账
func TestGetOperatorSettlementAccountBindbankSubmitted(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "bindbank_submitted",
		SubMchID: pgtype.Text{String: "sub_mch_op_001", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	// ecommerce mock 不应被调用，断言通过 gomock 隐式验证（未注册 = 不允许调用）

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
