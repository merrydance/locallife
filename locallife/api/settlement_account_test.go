package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		MerchantID:                    merchant.ID,
		SubMchID:                      "sub_mch_123",
		Status:                        "active",
		LatestSettlementApplicationNo: pgtype.Text{String: "APP_OLD_001", Valid: true},
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
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			BankName:      "工商银行北京分行",
			AccountNumber: "6222****8888",
			VerifyResult:  "VERIFY_SUCCESS",
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
	require.Equal(t, "ACCOUNT_TYPE_BUSINESS", resp.Account.AccountType)
	require.Equal(t, "工商银行", resp.Account.AccountBank)
	require.Equal(t, "6222****8888", resp.Account.AccountNumber)
	require.Equal(t, "VERIFY_SUCCESS", resp.Account.VerifyResult)
	require.Equal(t, "APP_OLD_001", resp.LatestApplicationNo)
	require.Equal(t, "微信提现卡已通过微信校验", resp.StatusDesc)
}

func TestGetMerchantSettlementAccountWithMaskRule(t *testing.T) {
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
		QuerySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, "ACCOUNT_NUMBER_RULE_MASK_V2").
		Times(1).
		Return(&wechat.SubMerchantSettlementResponse{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			AccountNumber: "622202******8888",
			VerifyResult:  "VERIFY_SUCCESS",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account?account_number_rule=ACCOUNT_NUMBER_RULE_MASK_V2", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetMerchantSettlementAccountInvalidMaskRule(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account?account_number_rule=MASK_V3", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMerchantSettlementAccountInvalidWechatResponse(t *testing.T) {
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
		Times(1).
		Return(nil, wechatcontracts.NewSubMerchantSettlementContractError("unsupported verify_result %q", ""))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadGateway, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementWechatInvalidResponse.Code, resp.Code)
	require.Equal(t, ErrSettlementWechatInvalidResponse.Message, resp.Error)
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
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "工商银行",
				BankAliasCode:  "1002",
				AccountBank:    "工商银行",
				NeedBankBranch: true,
			}},
		}, nil)

	ecommerce.EXPECT().
		EncryptSensitiveData("6222020202020202").
		Times(1).
		Return("wx_encrypted_account_number", nil)

	ecommerce.EXPECT().
		EncryptSensitiveData("测试商户有限公司").
		Times(1).
		Return("wx_encrypted_account_name", nil)

	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			BankName:      "中国工商银行北京分行",
			BankBranchID:  "402713354941",
			AccountNumber: "wx_encrypted_account_number",
			AccountName:   "wx_encrypted_account_name",
		}).
		Return(&wechat.ModifySubMerchantSettlementResponse{
			ApplicationNo: "102329389XXXX",
		}, nil)

	store.EXPECT().
		UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantPaymentConfigSettlementApplicationParams{})).
		DoAndReturn(func(_ any, arg db.UpdateMerchantPaymentConfigSettlementApplicationParams) (db.MerchantPaymentConfig, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, "102329389XXXX", arg.LatestSettlementApplicationNo.String)
			require.True(t, arg.LatestSettlementApplicationNo.Valid)
			require.True(t, arg.LatestSettlementApplicationSubmittedAt.Valid)
			return paymentConfig, nil
		})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		BankName:      "中国工商银行北京分行",
		BankBranchID:  "402713354941",
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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

	// account_number and account_name missing (required fields)
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

func TestModifyMerchantSettlementAccountInvalidAccountType(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_UNKNOWN",
		AccountBank:   "工商银行",
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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

func TestModifyMerchantSettlementAccountNonNumericAccountNumber(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "6222-0202-0202-0202",
		AccountName:   "测试商户有限公司",
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

func TestModifyMerchantSettlementAccountAllowsMissingAccountName(t *testing.T) {
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
	paymentConfig := db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_123",
		Status:     "active",
	}

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(paymentConfig, nil)
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "工商银行",
				BankAliasCode:  "1002",
				AccountBank:    "工商银行",
				NeedBankBranch: false,
			}},
		}, nil)
	ecommerce.EXPECT().
		EncryptSensitiveData("6222020202020202").
		Times(1).
		Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			AccountNumber: "wx_encrypted_account_number",
		}).
		Times(1).
		Return(&wechat.ModifySubMerchantSettlementResponse{ApplicationNo: "APP_NO_NAME_001"}, nil)
	store.EXPECT().
		UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantPaymentConfigSettlementApplicationParams{})).
		Times(1).
		Return(paymentConfig, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "6222020202020202",
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
	require.Equal(t, "APP_NO_NAME_001", resp.ApplicationNo)
}

func TestModifyMerchantSettlementAccountMissingBranchSelectionWhenRequired(t *testing.T) {
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
	paymentConfig := db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_123",
		Status:     "active",
	}

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(paymentConfig, nil)
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "工商银行",
				BankAliasCode:  "1002",
				AccountBank:    "工商银行",
				NeedBankBranch: true,
			}},
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, errSettlementBankBranchRequired.Error(), resp.Message)
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
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "工商银行",
				BankAliasCode:  "1002",
				AccountBank:    "工商银行",
				NeedBankBranch: false,
			}},
		}, nil)

	ecommerce.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(1).
		Return("", errors.New("encryption failed"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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

func TestModifyMerchantSettlementAccountIgnoresFrontendNeedBankBranchHint(t *testing.T) {
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
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "建设银行",
				BankAliasCode:  "1003",
				AccountBank:    "建设银行",
				NeedBankBranch: false,
			}},
		}, nil)
	ecommerce.EXPECT().
		EncryptSensitiveData("6222020202020202").
		Times(1).
		Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().
		EncryptSensitiveData("测试商户有限公司").
		Times(1).
		Return("wx_encrypted_account_name", nil)
	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "建设银行",
			AccountNumber: "wx_encrypted_account_number",
			AccountName:   "wx_encrypted_account_name",
		}).
		Times(1).
		Return(&wechat.ModifySubMerchantSettlementResponse{ApplicationNo: "APP_IGNORE_HINT_001"}, nil)
	store.EXPECT().
		UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantPaymentConfigSettlementApplicationParams{})).
		Times(1).
		Return(paymentConfig, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:    "ACCOUNT_TYPE_BUSINESS",
		AccountBank:    "建设银行",
		NeedBankBranch: true,
		AccountNumber:  "6222020202020202",
		AccountName:    "测试商户有限公司",
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
}

func TestModifyMerchantSettlementAccountWechatParamError(t *testing.T) {
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
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{
			SubjectType: "merchant",
			SubjectID:   merchant.ID,
		}).
		Times(1).
		Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechat.CapitalBank{{
				BankAlias:      "建设银行",
				BankAliasCode:  "1003",
				AccountBank:    "建设银行",
				NeedBankBranch: false,
			}},
		}, nil)
	ecommerce.EXPECT().
		EncryptSensitiveData("6222020202020202").
		Times(1).
		Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().
		EncryptSensitiveData("测试商户有限公司").
		Times(1).
		Return("wx_encrypted_account_name", nil)
	ecommerce.EXPECT().
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "建设银行",
			AccountNumber: "wx_encrypted_account_number",
			AccountName:   "wx_encrypted_account_name",
		}).
		Times(1).
		Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "参数错误", Detail: "invalid bank info"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	body := modifySettlementAccountRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "建设银行",
		AccountNumber: "6222020202020202",
		AccountName:   "测试商户有限公司",
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

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, errSettlementWechatParamError.Error(), resp.Message)
}

func TestModifyMerchantSettlementAccountWechatNoAuth(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Times(1).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechat.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechat.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "商户权限异常"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	bodyBytes, err := json.Marshal(modifySettlementAccountRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "6222020202020202"})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementWechatNoAuth.Code, resp.Code)
	require.Equal(t, ErrSettlementWechatNoAuth.Message, resp.Error)
}

func TestModifyMerchantSettlementAccountWechatFrequencyLimit(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Times(1).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechat.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechat.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQENCY_LIMIT", Message: "频率超限"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	bodyBytes, err := json.Marshal(modifySettlementAccountRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "6222020202020202"})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementWechatFrequencyLimit.Code, resp.Code)
	require.Equal(t, ErrSettlementWechatFrequencyLimit.Message, resp.Error)
}

func TestModifyMerchantSettlementAccountWechatNameMismatch(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Times(1).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechat.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechat.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().EncryptSensitiveData("新的开户名").Times(1).Return("wx_encrypted_account_name", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number", AccountName: "wx_encrypted_account_name"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "无效请求", Detail: "你的开户名称与主体名称不一致，请修改后重试"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	bodyBytes, err := json.Marshal(modifySettlementAccountRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "6222020202020202", AccountName: "新的开户名"})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/settlement-account", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, errSettlementWechatNameMismatch.Error(), resp.Message)
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

	store.EXPECT().
		UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantPaymentConfigSettlementApplicationParams{})).
		DoAndReturn(func(_ any, arg db.UpdateMerchantPaymentConfigSettlementApplicationParams) (db.MerchantPaymentConfig, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, "102329389XXXX", arg.LatestSettlementApplicationNo.String)
			require.True(t, arg.LatestSettlementApplicationNo.Valid)
			return paymentConfig, nil
		})

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
		Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "ORDER_NOT_EXIST", Message: "申请单不存在"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_404", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementApplicationNotFound.Code, resp.Code)
	require.Equal(t, ErrSettlementApplicationNotFound.Message, resp.Error)
}

func TestGetMerchantSettlementApplicationInvalidMaskRule(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_001?account_number_rule=MASK_V3", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMerchantSettlementApplicationTooLongApplicationNo(t *testing.T) {
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

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	applicationNo := strings.Repeat("A", 65)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/"+applicationNo, nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMerchantSettlementApplicationInvalidWechatResponse(t *testing.T) {
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
		QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_BAD", "").
		Times(1).
		Return(nil, wechatcontracts.NewSubMerchantSettlementApplicationContractError("account_name is required"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_BAD", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadGateway, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementWechatInvalidResponse.Code, resp.Code)
	require.Equal(t, ErrSettlementWechatInvalidResponse.Message, resp.Error)
}
