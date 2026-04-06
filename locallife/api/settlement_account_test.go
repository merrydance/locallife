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

// ==================== 商户修改结算账户测试 ====================

func TestModifyMerchantSettlementAccountOK(t *testing.T) {
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
		EncryptSensitiveData("encrypted_account_number").
		Times(1).
		Return("wx_encrypted_account_number", nil)

	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			BankName:      "中国工商银行北京分行",
			BankBranchID:  "402713354941",
			AccountNumber: "wx_encrypted_account_number",
		}).
		Return(&wechat.ModifySubMerchantSettlementResponse{
			ApplicationNo: "102329389XXXX",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		BankName:      "中国工商银行北京分行",
		BankBranchID:  "402713354941",
		AccountNumber: "encrypted_account_number",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp modifySettlementAccountApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "102329389XXXX", resp.ApplicationNo)
}

func TestModifyMerchantSettlementAccountNotActive(t *testing.T) {
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

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "encrypted_account_number",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
}

func TestModifyMerchantSettlementAccountMissingFields(t *testing.T) {
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

	// middleware runs before binding check: expect merchant resolution
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	// account_number missing (required field)
	body := map[string]string{
		"account_type": "ACCOUNT_TYPE_BUSINESS",
		"account_bank": "工商银行",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestModifyMerchantSettlementAccountEncryptFails(t *testing.T) {
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
		EncryptSensitiveData(gomock.Any()).
		Times(1).
		Return("", errors.New("encryption failed"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "plain_account_number",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

// ==================== 运营商修改结算账户测试 ====================

func TestModifyOperatorSettlementAccountOK(t *testing.T) {
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
		EncryptSensitiveData("encrypted_account_number_op").
		Times(1).
		Return("wx_encrypted_account_number_op", nil)

	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), operator.SubMchID.String, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_PRIVATE",
			AccountBank:   "建设银行",
			AccountNumber: "wx_encrypted_account_number_op",
		}).
		Return(&wechat.ModifySubMerchantSettlementResponse{
			ApplicationNo: "OP_APP_001",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_PRIVATE",
		AccountBank:   "建设银行",
		AccountNumber: "encrypted_account_number_op",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/operators/me/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp modifySettlementAccountApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "OP_APP_001", resp.ApplicationNo)
}

func TestModifyOperatorSettlementAccountNotActive(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_PRIVATE",
		AccountBank:   "建设银行",
		AccountNumber: "encrypted_account_number_op",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/operators/me/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

// ==================== 商户查询结算账户修改申请状态 ====================

func TestGetMerchantSettlementApplicationOK(t *testing.T) {
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
		QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "102329389XXXX", "").
		Times(1).
		Return(&wechat.QuerySubMerchantSettlementApplicationResponse{
			AccountName:      "张*",
			AccountType:      "ACCOUNT_TYPE_BUSINESS",
			AccountBank:      "工商银行",
			AccountNumber:    "62***78",
			VerifyResult:     "AUDIT_SUCCESS",
			VerifyFinishTime: "2015-05-20T13:29:35+08:00",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/102329389XXXX", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp settlementApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "AUDIT_SUCCESS", resp.VerifyResult)
	require.Equal(t, "62***78", resp.AccountNumber)
	require.Equal(t, "工商银行", resp.AccountBank)
}

func TestGetMerchantSettlementApplicationNotActive(t *testing.T) {
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
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_001", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
}

func TestGetMerchantSettlementApplicationWechatNotFound(t *testing.T) {
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
		QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_404", "").
		Times(1).
		Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "RESOURCE_NOT_EXISTS", Message: "申请单不存在"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_404", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

// ==================== 运营商查询结算账户修改申请状态 ====================

func TestGetOperatorSettlementApplicationOK(t *testing.T) {
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
		QuerySubMerchantSettlementApplication(gomock.Any(), operator.SubMchID.String, "OP_APP_001", "ACCOUNT_NUMBER_RULE_MASK_V2").
		Times(1).
		Return(&wechat.QuerySubMerchantSettlementApplicationResponse{
			AccountName:   "王*",
			AccountType:   "ACCOUNT_TYPE_PRIVATE",
			AccountBank:   "建设银行",
			AccountNumber: "623456****78",
			VerifyResult:  "AUDITING",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account/applications/OP_APP_001?account_number_rule=ACCOUNT_NUMBER_RULE_MASK_V2", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp settlementApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "AUDITING", resp.VerifyResult)
	require.Equal(t, "623456****78", resp.AccountNumber)
}

func TestGetOperatorSettlementApplicationNotActive(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "suspended",
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account/applications/OP_APP_001", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetOperatorSettlementApplicationWechatNotFound(t *testing.T) {
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
		QuerySubMerchantSettlementApplication(gomock.Any(), operator.SubMchID.String, "OP_APP_404", "").
		Times(1).
		Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "RESOURCE_NOT_EXISTS", Message: "申请单不存在"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/settlement-account/applications/OP_APP_404", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
