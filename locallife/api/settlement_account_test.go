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

	"github.com/gin-gonic/gin"
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

func expectSettlementModifyCommand(t *testing.T, store *mockdb.MockStore, paymentConfig db.MerchantPaymentConfig, commandStatus string, externalSecondaryKey string, lastErrorCode string, lastErrorMessage string, checkPayload func(map[string]any)) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).DoAndReturn(func(_ any, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilitySettlement, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateSettlement, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
		require.Equal(t, "merchant_payment_config", arg.BusinessObjectType.String)
		require.Equal(t, paymentConfig.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectSettlement, arg.ExternalObjectType)
		require.Equal(t, paymentConfig.SubMchID, arg.ExternalObjectKey)
		require.Equal(t, commandStatus, arg.CommandStatus)
		if externalSecondaryKey == "" {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		} else {
			require.Equal(t, externalSecondaryKey, arg.ExternalSecondaryKey.String)
			require.True(t, arg.ExternalSecondaryKey.Valid)
		}
		if lastErrorCode == "" {
			require.False(t, arg.LastErrorCode.Valid)
		} else {
			require.Equal(t, lastErrorCode, arg.LastErrorCode.String)
			require.True(t, arg.LastErrorCode.Valid)
		}
		if lastErrorMessage == "" {
			require.False(t, arg.LastErrorMessage.Valid)
		} else {
			require.Equal(t, lastErrorMessage, arg.LastErrorMessage.String)
			require.True(t, arg.LastErrorMessage.Valid)
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.ResponseSnapshot, &payload))
		checkPayload(payload)
		return db.ExternalPaymentCommand{ID: 8801, ExternalObjectKey: arg.ExternalObjectKey, CommandStatus: arg.CommandStatus}, nil
	})
}

func expectSettlementApplicationQueryFact(t *testing.T, store *mockdb.MockStore, paymentConfig db.MerchantPaymentConfig, applicationNo string, terminalStatus string, verifyResult string, dedupeSuffix string, ownerPaymentConfigID int64, checkPayload func(map[string]any)) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilitySettlement, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectSettlement, arg.ExternalObjectType)
		require.Equal(t, paymentConfig.SubMchID, arg.ExternalObjectKey)
		require.Equal(t, applicationNo, arg.ExternalSecondaryKey.String)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.True(t, arg.BusinessOwner.Valid)
		require.Equal(t, settlementFactBusinessObjectMerchantPaymentConfig, arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, paymentConfig.ID, arg.BusinessObjectID.Int64)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, verifyResult, arg.UpstreamState)
		require.Equal(t, terminalStatus, arg.TerminalStatus)
		require.Equal(t, "CNY", arg.Currency)
		require.Equal(t, fmt.Sprintf("wechat:query:ecommerce:settlement_application:%s:%s:%s:%s", paymentConfig.SubMchID, applicationNo, verifyResult, dedupeSuffix), arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		checkPayload(payload)
		return db.ExternalPaymentFact{ID: 9901, DedupeKey: arg.DedupeKey, IsTerminal: arg.IsTerminal, TerminalStatus: arg.TerminalStatus}, nil
	})
	if ownerPaymentConfigID > 0 {
		store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             9901,
			Consumer:           settlementFactConsumerDomain,
			BusinessObjectType: settlementFactBusinessObjectMerchantPaymentConfig,
			BusinessObjectID:   ownerPaymentConfigID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).Return(db.ExternalPaymentFactApplication{
			ID:                 9911,
			FactID:             9901,
			Consumer:           settlementFactConsumerDomain,
			BusinessObjectType: settlementFactBusinessObjectMerchantPaymentConfig,
			BusinessObjectID:   ownerPaymentConfigID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}, nil)
	}
}

func expectSettlementApplicationTrackingApply(t *testing.T, store *mockdb.MockStore, applicationID int64, paymentConfig db.MerchantPaymentConfig, applicationNo string, verifyResult string) {
	t.Helper()
	rawResource, err := json.Marshal(map[string]any{
		"merchant_id":    paymentConfig.MerchantID,
		"sub_mch_id":     paymentConfig.SubMchID,
		"application_no": applicationNo,
		"verify_result":  verifyResult,
	})
	require.NoError(t, err)
	terminalStatus := db.ExternalPaymentTerminalStatusProcessing
	if verifyResult == "AUDIT_SUCCESS" {
		terminalStatus = db.ExternalPaymentTerminalStatusSuccess
	}
	if verifyResult == "AUDIT_FAIL" {
		terminalStatus = db.ExternalPaymentTerminalStatusFailed
	}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), applicationID).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             9901,
		Consumer:           settlementFactConsumerDomain,
		BusinessObjectType: settlementFactBusinessObjectMerchantPaymentConfig,
		BusinessObjectID:   paymentConfig.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(9901)).Return(db.ExternalPaymentFact{
		ID:                   9901,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilitySettlement,
		ExternalObjectType:   db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:    paymentConfig.SubMchID,
		ExternalSecondaryKey: pgtype.Text{String: applicationNo, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: settlementFactBusinessObjectMerchantPaymentConfig, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentConfig.ID, Valid: true},
		UpstreamState:        verifyResult,
		TerminalStatus:       terminalStatus,
		IsTerminal:           terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          rawResource,
	}, nil)
	store.EXPECT().GetMerchantPaymentConfigBySubMchID(gomock.Any(), paymentConfig.SubMchID).Return(paymentConfig, nil)
	store.EXPECT().UpdateMerchantPaymentConfigSettlementApplication(gomock.Any(), db.UpdateMerchantPaymentConfigSettlementApplicationParams{
		MerchantID:                             paymentConfig.MerchantID,
		LatestSettlementApplicationNo:          pgtype.Text{String: applicationNo, Valid: true},
		LatestSettlementApplicationSubmittedAt: pgtype.Timestamptz{},
	}).Return(paymentConfig, nil)
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 9901, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             9901,
		Consumer:           settlementFactConsumerDomain,
		BusinessObjectType: settlementFactBusinessObjectMerchantPaymentConfig,
		BusinessObjectID:   paymentConfig.ID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
	}, nil)
}

func expectSettlementAccountQueryFact(t *testing.T, store *mockdb.MockStore, paymentConfig db.MerchantPaymentConfig, latestApplicationNo string, terminalStatus string, verifyResult string, dedupeSuffix string, ownerApplymentID int64, checkPayload func(map[string]any)) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilitySettlement, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectSettlement, arg.ExternalObjectType)
		require.Equal(t, paymentConfig.SubMchID, arg.ExternalObjectKey)
		if latestApplicationNo == "" {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		} else {
			require.Equal(t, latestApplicationNo, arg.ExternalSecondaryKey.String)
			require.True(t, arg.ExternalSecondaryKey.Valid)
		}
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.True(t, arg.BusinessOwner.Valid)
		require.Equal(t, settlementFactBusinessObjectMerchantPaymentConfig, arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, paymentConfig.ID, arg.BusinessObjectID.Int64)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, verifyResult, arg.UpstreamState)
		require.Equal(t, terminalStatus, arg.TerminalStatus)
		require.Equal(t, "CNY", arg.Currency)
		require.Equal(t, fmt.Sprintf("wechat:query:ecommerce:settlement_account:%s:%s:%s", paymentConfig.SubMchID, verifyResult, dedupeSuffix), arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		checkPayload(payload)
		return db.ExternalPaymentFact{ID: 9902, DedupeKey: arg.DedupeKey, IsTerminal: arg.IsTerminal, TerminalStatus: arg.TerminalStatus}, nil
	})
	if ownerApplymentID > 0 {
		store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
			FactID:             9902,
			Consumer:           settlementFactConsumerDomain,
			BusinessObjectType: settlementFactBusinessObjectApplyment,
			BusinessObjectID:   ownerApplymentID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}).Return(db.ExternalPaymentFactApplication{
			ID:                 9912,
			FactID:             9902,
			Consumer:           settlementFactConsumerDomain,
			BusinessObjectType: settlementFactBusinessObjectApplyment,
			BusinessObjectID:   ownerApplymentID,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		}, nil)
	}
}

func expectSettlementVerificationApplicationApply(t *testing.T, store *mockdb.MockStore, applicationID int64, applyment db.EcommerceApplyment, verifyResult string, expectedStatus string, expectedFailReason string) {
	t.Helper()
	rawResource, err := json.Marshal(map[string]any{
		"merchant_id":           applyment.SubjectID,
		"sub_mch_id":            applyment.SubMchID.String,
		"latest_application_no": "APP_OLD_001",
		"verify_result":         verifyResult,
		"verify_fail_reason":    expectedFailReason,
		"owner_applyment_id":    applyment.ID,
	})
	require.NoError(t, err)
	terminalStatus := db.ExternalPaymentTerminalStatusProcessing
	if verifyResult == "VERIFY_SUCCESS" {
		terminalStatus = db.ExternalPaymentTerminalStatusSuccess
	}
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), applicationID).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             9902,
		Consumer:           settlementFactConsumerDomain,
		BusinessObjectType: settlementFactBusinessObjectApplyment,
		BusinessObjectID:   applyment.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(9902)).Return(db.ExternalPaymentFact{
		ID:                 9902,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilitySettlement,
		ExternalObjectType: db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:  applyment.SubMchID.String,
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		UpstreamState:      verifyResult,
		TerminalStatus:     terminalStatus,
		IsTerminal:         terminalStatus != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:        rawResource,
	}, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), applyment.ID).Return(applyment, nil)
	store.EXPECT().UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).DoAndReturn(func(_ any, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
		require.Equal(t, applyment.ID, arg.ID)
		require.False(t, arg.SettlementVerifyFirstTradeAt.Valid)
		require.False(t, arg.SettlementVerifyLastCheckedAt.Valid)
		require.False(t, arg.SettlementVerifyCheckCount.Valid)
		require.True(t, arg.SettlementVerifyStatus.Valid)
		require.Equal(t, expectedStatus, arg.SettlementVerifyStatus.String)
		require.True(t, arg.SettlementVerifyFailReason.Valid)
		require.Equal(t, expectedFailReason, arg.SettlementVerifyFailReason.String)
		applyment.SettlementVerifyStatus = pgtype.Text{String: expectedStatus, Valid: true}
		applyment.SettlementVerifyFailReason = pgtype.Text{String: expectedFailReason, Valid: true}
		return applyment, nil
	})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 9902, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             9902,
		Consumer:           settlementFactConsumerDomain,
		BusinessObjectType: settlementFactBusinessObjectApplyment,
		BusinessObjectID:   applyment.ID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
	}, nil)
}

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

func TestGetMerchantSettlementAccountEcommerceClientUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.getMerchantSettlementAccount(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "ecommerce client not configured", resp.Error)
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
		ID:                            101,
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
		Return(&wechatcontracts.SubMerchantSettlementResponse{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			BankName:      "工商银行北京分行",
			AccountNumber: "6222****8888",
			VerifyResult:  "VERIFY_SUCCESS",
		}, nil)
	applyment := db.EcommerceApplyment{
		ID:          301,
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
		SubMchID:    pgtype.Text{String: paymentConfig.SubMchID, Valid: true},
	}
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).
		Return(applyment, nil)
	expectSettlementAccountQueryFact(t, store, paymentConfig, "APP_OLD_001", db.ExternalPaymentTerminalStatusSuccess, "VERIFY_SUCCESS", "APP_OLD_001", applyment.ID, func(payload map[string]any) {
		require.Equal(t, float64(merchant.ID), payload["merchant_id"])
		require.Equal(t, paymentConfig.SubMchID, payload["sub_mch_id"])
		require.Equal(t, "APP_OLD_001", payload["latest_application_no"])
		require.Equal(t, "VERIFY_SUCCESS", payload["verify_result"])
		require.Equal(t, "6222****8888", payload["account_number"])
		require.Equal(t, float64(applyment.ID), payload["owner_applyment_id"])
	})
	expectSettlementVerificationApplicationApply(t, store, 9912, applyment, "VERIFY_SUCCESS", "success", "")

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

func TestGetMerchantSettlementAccountVerifyingRecordsProcessingFact(t *testing.T) {
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
		ID:         104,
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
		Return(&wechatcontracts.SubMerchantSettlementResponse{
			AccountType:      "ACCOUNT_TYPE_BUSINESS",
			AccountBank:      "工商银行",
			AccountNumber:    "6222****8888",
			VerifyResult:     "VERIFYING",
			VerifyFailReason: "",
		}, nil)
	applyment := db.EcommerceApplyment{
		ID:          304,
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
		SubMchID:    pgtype.Text{String: paymentConfig.SubMchID, Valid: true},
	}
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).
		Return(applyment, nil)
	expectSettlementAccountQueryFact(t, store, paymentConfig, "", db.ExternalPaymentTerminalStatusProcessing, "VERIFYING", "current", applyment.ID, func(payload map[string]any) {
		require.Equal(t, "VERIFYING", payload["verify_result"])
		require.Equal(t, "", payload["latest_application_no"])
		require.Equal(t, float64(applyment.ID), payload["owner_applyment_id"])
	})
	expectSettlementVerificationApplicationApply(t, store, 9912, applyment, "VERIFYING", "verifying", "")

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
	require.Equal(t, "VERIFYING", resp.Account.VerifyResult)
	require.Equal(t, "微信提现卡正在校验中，暂时无法提现，请稍后查看结果", resp.StatusDesc)
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
		ID:         102,
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
		Return(&wechatcontracts.SubMerchantSettlementResponse{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			AccountNumber: "622202******8888",
			VerifyResult:  "VERIFY_SUCCESS",
		}, nil)
	applyment := db.EcommerceApplyment{
		ID:          302,
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
		SubMchID:    pgtype.Text{String: paymentConfig.SubMchID, Valid: true},
	}
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).
		Return(applyment, nil)
	expectSettlementAccountQueryFact(t, store, paymentConfig, "", db.ExternalPaymentTerminalStatusSuccess, "VERIFY_SUCCESS", "current", applyment.ID, func(payload map[string]any) {
		require.Equal(t, float64(applyment.ID), payload["owner_applyment_id"])
		require.Equal(t, "622202******8888", payload["account_number"])
	})
	expectSettlementVerificationApplicationApply(t, store, 9912, applyment, "VERIFY_SUCCESS", "success", "")

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
		ID:         102,
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
		ID:         101,
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			BankName:      "中国工商银行北京分行",
			BankBranchID:  "402713354941",
			AccountNumber: "wx_encrypted_account_number",
			AccountName:   "wx_encrypted_account_name",
		}).
		Return(&wechatcontracts.ModifySubMerchantSettlementResponse{
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
	expectSettlementModifyCommand(t, store, paymentConfig, db.ExternalPaymentCommandStatusAccepted, "102329389XXXX", "", "", func(payload map[string]any) {
		require.Equal(t, paymentConfig.SubMchID, payload["sub_mch_id"])
		require.Equal(t, "102329389XXXX", payload["application_no"])
		require.Equal(t, "ACCOUNT_TYPE_BUSINESS", payload["account_type"])
		require.Equal(t, "工商银行", payload["account_bank"])
		require.Equal(t, "中国工商银行北京分行", payload["bank_name"])
		require.Equal(t, "402713354941", payload["bank_branch_id"])
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			AccountNumber: "wx_encrypted_account_number",
		}).
		Times(1).
		Return(&wechatcontracts.ModifySubMerchantSettlementResponse{ApplicationNo: "APP_NO_NAME_001"}, nil)
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "建设银行",
			AccountNumber: "wx_encrypted_account_number",
			AccountName:   "wx_encrypted_account_name",
		}).
		Times(1).
		Return(&wechatcontracts.ModifySubMerchantSettlementResponse{ApplicationNo: "APP_IGNORE_HINT_001"}, nil)
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
		Return(&wechatcontracts.CapitalBankListResponse{
			TotalCount: 1,
			Count:      1,
			Data: []wechatcontracts.CapitalBank{{
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
		ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{
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
	paymentConfig := db.MerchantPaymentConfig{ID: 102, MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Times(1).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechatcontracts.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechatcontracts.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "商户权限异常"}))
	expectSettlementModifyCommand(t, store, paymentConfig, db.ExternalPaymentCommandStatusRejected, "", "NO_AUTH", "商户权限异常", func(payload map[string]any) {
		require.Equal(t, paymentConfig.SubMchID, payload["sub_mch_id"])
		require.Equal(t, "ACCOUNT_TYPE_BUSINESS", payload["account_type"])
		require.Equal(t, "建设银行", payload["account_bank"])
		require.Equal(t, "NO_AUTH", payload["error_code"])
		require.Equal(t, "商户权限异常", payload["error_message"])
	})

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
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechatcontracts.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechatcontracts.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQENCY_LIMIT", Message: "频率超限"}))

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
	ecommerce.EXPECT().ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).Times(1).Return(&wechatcontracts.CapitalBankListResponse{TotalCount: 1, Count: 1, Data: []wechatcontracts.CapitalBank{{BankAlias: "建设银行", AccountBank: "建设银行"}}}, nil)
	ecommerce.EXPECT().EncryptSensitiveData("6222020202020202").Times(1).Return("wx_encrypted_account_number", nil)
	ecommerce.EXPECT().EncryptSensitiveData("新的开户名").Times(1).Return("wx_encrypted_account_name", nil)
	ecommerce.EXPECT().ModifySubMerchantSettlement(gomock.Any(), paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{AccountType: "ACCOUNT_TYPE_BUSINESS", AccountBank: "建设银行", AccountNumber: "wx_encrypted_account_number", AccountName: "wx_encrypted_account_name"}).Times(1).Return(nil, fmt.Errorf("modify sub merchant settlement: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "无效请求", Detail: "你的开户名称与主体名称不一致，请修改后重试"}))

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
		ID:         102,
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
		Return(&wechatcontracts.QuerySubMerchantSettlementApplicationResponse{
			AccountName:      "张*",
			AccountType:      "ACCOUNT_TYPE_BUSINESS",
			AccountBank:      "工商银行",
			AccountNumber:    "62***78",
			VerifyResult:     "AUDIT_SUCCESS",
			VerifyFinishTime: "2015-05-20T13:29:35+08:00",
		}, nil)
	expectSettlementApplicationQueryFact(t, store, paymentConfig, "102329389XXXX", db.ExternalPaymentTerminalStatusSuccess, "AUDIT_SUCCESS", "2015-05-20T13:29:35+08:00", paymentConfig.ID, func(payload map[string]any) {
		require.Equal(t, float64(merchant.ID), payload["merchant_id"])
		require.Equal(t, paymentConfig.SubMchID, payload["sub_mch_id"])
		require.Equal(t, "102329389XXXX", payload["application_no"])
		require.Equal(t, "AUDIT_SUCCESS", payload["verify_result"])
		require.Equal(t, "2015-05-20T13:29:35+08:00", payload["verify_finish_time"])
	})
	expectSettlementApplicationTrackingApply(t, store, 9911, paymentConfig, "102329389XXXX", "AUDIT_SUCCESS")

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

func TestGetMerchantSettlementApplicationAuditingRecordsProcessingFact(t *testing.T) {
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
		ID:         103,
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
		QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_AUDITING", "").
		Times(1).
		Return(&wechatcontracts.QuerySubMerchantSettlementApplicationResponse{
			AccountName:   "张*",
			AccountType:   "ACCOUNT_TYPE_BUSINESS",
			AccountBank:   "工商银行",
			AccountNumber: "62***78",
			VerifyResult:  "AUDITING",
		}, nil)
	expectSettlementApplicationQueryFact(t, store, paymentConfig, "APP_AUDITING", db.ExternalPaymentTerminalStatusProcessing, "AUDITING", "current", paymentConfig.ID, func(payload map[string]any) {
		require.Equal(t, "APP_AUDITING", payload["application_no"])
		require.Equal(t, "AUDITING", payload["verify_result"])
		require.Equal(t, "", payload["verify_finish_time"])
	})
	expectSettlementApplicationTrackingApply(t, store, 9911, paymentConfig, "APP_AUDITING", "AUDITING")

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_AUDITING", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp settlementApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "AUDITING", resp.VerifyResult)
	require.Equal(t, "62***78", resp.AccountNumber)
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

func TestGetMerchantSettlementApplicationWechatNoAuth(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	ecommerce.EXPECT().QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_NO_AUTH", "").Times(1).Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "服务商没有权限查询"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_NO_AUTH", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementApplicationQueryNoAuth.Code, resp.Code)
	require.Equal(t, ErrSettlementApplicationQueryNoAuth.Message, resp.Error)
}

func TestGetMerchantSettlementApplicationWechatFrequencyLimit(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	ecommerce.EXPECT().QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_LIMIT", "").Times(1).Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQENCY_LIMIT", Message: "频率超限"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_LIMIT", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrSettlementApplicationQueryFrequencyLimit.Code, resp.Code)
	require.Equal(t, ErrSettlementApplicationQueryFrequencyLimit.Message, resp.Error)
}

func TestGetMerchantSettlementApplicationWechatParamError(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	ecommerce.EXPECT().QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_PARAM", "").Times(1).Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "参数错误"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_PARAM", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, errSettlementApplicationQueryWechatParamError.Error(), resp.Message)
}

func TestGetMerchantSettlementApplicationWechatInvalidRequest(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 1, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true, CreatedAt: time.Now()}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_123", Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Times(1).Return(paymentConfig, nil)
	ecommerce.EXPECT().QuerySubMerchantSettlementApplication(gomock.Any(), paymentConfig.SubMchID, "APP_INVALID", "").Times(1).Return(nil, fmt.Errorf("query sub merchant settlement application: %w", &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "无效请求"}))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/settlement-account/applications/APP_INVALID", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, errSettlementApplicationQueryWechatInvalidRequest.Error(), resp.Message)
}
