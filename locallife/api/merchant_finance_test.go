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

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectMerchantWithdrawQueryFact(t *testing.T, store *mockdb.MockStore, record db.WithdrawalRecord, accountInfo merchantWithdrawAccountInfo, withdrawID string, wechatStatus string, reason string, applicationID int64) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
		require.Equal(t, accountInfo.OutRequestNo, arg.ExternalObjectKey)
		require.Equal(t, withdrawID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.Equal(t, merchantWithdrawFactBusinessObject, arg.BusinessObjectType.String)
		require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, wechatStatus, arg.UpstreamState)
		require.Equal(t, merchantWithdrawTerminalStatus(wechatStatus), arg.TerminalStatus)
		require.Equal(t, merchantWithdrawQueryFactDedupeKey(accountInfo.OutRequestNo, wechatStatus, withdrawID, reason), arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		require.EqualValues(t, record.ID, payload["withdrawal_record_id"])
		require.Equal(t, accountInfo.SubMchID, payload["sub_mch_id"])
		require.Equal(t, accountInfo.OutRequestNo, payload["out_request_no"])
		require.Equal(t, withdrawID, payload["withdraw_id"])
		require.Equal(t, wechatStatus, payload["wechat_status"])
		require.Equal(t, reason, payload["reason"])
		return db.ExternalPaymentFact{ID: 9901, DedupeKey: arg.DedupeKey, IsTerminal: arg.IsTerminal, TerminalStatus: arg.TerminalStatus}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             9901,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             9901,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
}

func expectMerchantWithdrawFactApplySuccess(t *testing.T, store *mockdb.MockStore, applicationID int64, factID int64, currentRecord db.WithdrawalRecord, outRequestNo string, withdrawID string, wechatStatus string, reason string, updatedRecord *db.WithdrawalRecord) {
	t.Helper()
	rawResource, err := json.Marshal(map[string]any{
		"withdrawal_record_id": currentRecord.ID,
		"out_request_no":       outRequestNo,
		"withdraw_id":          withdrawID,
		"wechat_status":        wechatStatus,
		"reason":               reason,
		"amount":               currentRecord.Amount,
	})
	require.NoError(t, err)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), applicationID).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   currentRecord.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), factID).Return(db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: pgtype.Text{String: withdrawID, Valid: withdrawID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantWithdrawFactBusinessObject, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: currentRecord.ID, Valid: true},
		UpstreamState:        wechatStatus,
		TerminalStatus:       merchantWithdrawTerminalStatus(wechatStatus),
		IsTerminal:           merchantWithdrawTerminalStatus(wechatStatus) != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          rawResource,
	}, nil)
	store.EXPECT().GetWithdrawalRecord(gomock.Any(), currentRecord.ID).Return(currentRecord, nil)
	if updatedRecord != nil {
		reasonArg := pgtype.Text{}
		if reason != "" {
			reasonArg = pgtype.Text{String: reason, Valid: true}
		}
		store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), db.UpdateWithdrawalStatusParams{
			ID:          currentRecord.ID,
			Status:      mapWechatWithdrawStatus(wechatStatus),
			Reason:      reasonArg,
			ClearReason: reason == "" && currentRecord.Reason.Valid,
		}).Return(*updatedRecord, nil)
	}
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: factID, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   currentRecord.ID,
		Status:             db.ExternalPaymentFactApplicationStatusApplied,
	}, nil)
}

func expectMerchantWithdrawFactApplyFailure(t *testing.T, store *mockdb.MockStore, applicationID int64, factID int64, currentRecord db.WithdrawalRecord, outRequestNo string, withdrawID string, wechatStatus string, reason string, updateErr error) {
	t.Helper()
	rawResource, err := json.Marshal(map[string]any{
		"withdrawal_record_id": currentRecord.ID,
		"out_request_no":       outRequestNo,
		"withdraw_id":          withdrawID,
		"wechat_status":        wechatStatus,
		"reason":               reason,
		"amount":               currentRecord.Amount,
	})
	require.NoError(t, err)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), applicationID).Return(db.ExternalPaymentFactApplication{
		ID:                 applicationID,
		FactID:             factID,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessObject,
		BusinessObjectID:   currentRecord.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), factID).Return(db.ExternalPaymentFact{
		ID:                   factID,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: pgtype.Text{String: withdrawID, Valid: withdrawID != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantWithdrawFactBusinessObject, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: currentRecord.ID, Valid: true},
		UpstreamState:        wechatStatus,
		TerminalStatus:       merchantWithdrawTerminalStatus(wechatStatus),
		IsTerminal:           merchantWithdrawTerminalStatus(wechatStatus) != db.ExternalPaymentTerminalStatusProcessing,
		RawResource:          rawResource,
	}, nil)
	store.EXPECT().GetWithdrawalRecord(gomock.Any(), currentRecord.ID).Return(currentRecord, nil)
	reasonArg := pgtype.Text{}
	if reason != "" {
		reasonArg = pgtype.Text{String: reason, Valid: true}
	}
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), db.UpdateWithdrawalStatusParams{
		ID:          currentRecord.ID,
		Status:      mapWechatWithdrawStatus(wechatStatus),
		Reason:      reasonArg,
		ClearReason: reason == "" && currentRecord.Reason.Valid,
	}).Return(db.WithdrawalRecord{}, updateErr)
	store.EXPECT().MarkExternalPaymentFactApplicationFailed(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationFailedParams{})).Return(db.ExternalPaymentFactApplication{
		ID:     applicationID,
		Status: db.ExternalPaymentFactApplicationStatusFailed,
	}, nil)
}

func TestGetMerchantAccountBalanceAPIEcommerceClientUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.getMerchantAccountBalance(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "ecommerce client not configured", resp.Error)
}

func TestGetMerchantAccountBalanceAPINotConfigured(t *testing.T) {
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
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "not_configured", resp.AccountStatus)
	require.Equal(t, int64(0), resp.AvailableAmount)
	require.Equal(t, "", resp.SubMchID)
}

func TestGetMerchantAccountBalanceAPI_WithDayEndBalance(t *testing.T) {
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
		SubMchID:   "sub_mch_merchant_001",
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
		QueryEcommerceFundDayEndBalance(gomock.Any(), paymentConfig.SubMchID, "2026-04-05", "DEPOSIT").
		Return(&wechat.EcommerceFundBalanceResponse{
			SubMchID:        paymentConfig.SubMchID,
			AvailableAmount: 45678,
			PendingAmount:   9,
			AccountType:     "DEPOSIT",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance?date=2026-04-05&account_type=deposit", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, paymentConfig.SubMchID, resp.SubMchID)
	require.Equal(t, "DEPOSIT", resp.AccountType)
	require.Equal(t, "2026-04-05", resp.BalanceDate)
	require.Equal(t, int64(45678), resp.AvailableAmount)
	require.Equal(t, int64(45678), resp.WithdrawableAmount)
	require.Equal(t, "active", resp.AccountStatus)
}

func TestGetMerchantAccountBalanceAPI_InvalidDayEndAccountType(t *testing.T) {
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
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance?date=2026-04-05&account_type=fees", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMerchantAccountBalanceAPI_NoAuthReturnsExplicitMessage(t *testing.T) {
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
		SubMchID:   "sub_mch_merchant_001",
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
		QueryEcommerceFundBalanceByAccountType(gomock.Any(), paymentConfig.SubMchID, wechatcontracts.FundManagementAccountTypeBasic).
		Return(nil, &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: wechaterrorcodes.FundManagementCodeNoAuth, Message: "no auth"})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadGateway, recorder.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "微信侧暂无该账户查询权限，请联系管理员检查收付通配置", resp.Message)
}

func TestListMerchantAccountWithdrawalsAPIInactiveConfig(t *testing.T) {
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
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantWithdrawalsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "inactive", resp.AccountStatus)
	require.Len(t, resp.Withdrawals, 0)
	require.Equal(t, int64(0), resp.Total)
}

func TestCreateMerchantAccountWithdrawAPIManagerForbidden(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: owner.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	expectResolveSingleStaffMerchant(store, manager.ID, merchant)

	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     manager.ID,
		})).
		Times(1).
		Return("manager", nil)

	server := newTestServer(t, store)

	body := []byte(`{"amount":1000,"remark":"test withdraw"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestCreateMerchantAccountWithdrawAPIRecordsAcceptedCommand(t *testing.T) {
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
		GetWithdrawalRecordByOutRequestNo(gomock.Any(), gomock.Any()).
		Return(db.WithdrawalRecord{}, db.ErrRecordNotFound)

	store.EXPECT().
		CreateWithdrawalRecord(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateWithdrawalRecordParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(1200), arg.Amount)
			require.Equal(t, "pending", arg.Status)
			require.Equal(t, merchantWithdrawChannel, arg.Channel)
			require.Equal(t, "MW-20260425-001", arg.OutRequestNo.String)
			return db.WithdrawalRecord{
				ID:           77,
				UserID:       user.ID,
				Amount:       arg.Amount,
				Status:       arg.Status,
				Channel:      arg.Channel,
				AccountInfo:  arg.AccountInfo,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				OutRequestNo: pgtype.Text{String: arg.OutRequestNo.String, Valid: arg.OutRequestNo.Valid},
			}, nil
		})

	ecommerce.EXPECT().
		CreateEcommerceWithdraw(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg *wechatcontracts.EcommerceWithdrawRequest) (*wechatcontracts.EcommerceWithdrawCreateResponse, error) {
			require.Equal(t, paymentConfig.SubMchID, arg.SubMchID)
			require.Equal(t, "MW-20260425-001", arg.OutRequestNo)
			require.Equal(t, int64(1200), arg.Amount)
			return &wechatcontracts.EcommerceWithdrawCreateResponse{
				SubMchID:     paymentConfig.SubMchID,
				OutRequestNo: arg.OutRequestNo,
				WithdrawID:   "withdraw_merchant_001",
			}, nil
		})

	store.EXPECT().
		UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(77), arg.ID)
			info := parseMerchantWithdrawAccountInfo(arg.AccountInfo)
			require.Equal(t, merchant.ID, info.MerchantID)
			require.Equal(t, paymentConfig.SubMchID, info.SubMchID)
			require.Equal(t, "MW-20260425-001", info.OutRequestNo)
			require.Equal(t, "withdraw_merchant_001", info.WithdrawID)
			return db.WithdrawalRecord{
				ID:           77,
				UserID:       user.ID,
				Amount:       1200,
				Status:       "pending",
				Channel:      merchantWithdrawChannel,
				AccountInfo:  arg.AccountInfo,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				OutRequestNo: pgtype.Text{String: "MW-20260425-001", Valid: true},
			}, nil
		})

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreateWithdraw, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
			require.Equal(t, "withdrawal_record", arg.BusinessObjectType.String)
			require.Equal(t, int64(77), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
			require.Equal(t, "MW-20260425-001", arg.ExternalObjectKey)
			require.Equal(t, "withdraw_merchant_001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.False(t, arg.LastErrorCode.Valid)
			snapshot := string(arg.ResponseSnapshot)
			require.Contains(t, snapshot, "MW-20260425-001")
			require.Contains(t, snapshot, "withdraw_merchant_001")
			require.NotContains(t, snapshot, "测试提现")
			return db.ExternalPaymentCommand{ID: 9701}, nil
		})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"amount":1200,"remark":"测试提现","out_request_no":"MW-20260425-001"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusCreated, recorder.Code)

	var resp merchantWithdrawCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(77), resp.Withdrawal.ID)
	require.Equal(t, "pending", resp.Withdrawal.Status)
	require.Equal(t, "MW-20260425-001", resp.Withdrawal.OutRequestNo)
	require.Equal(t, "withdraw_merchant_001", resp.Withdrawal.WithdrawID)
}

func TestListMerchantAccountWithdrawalsAPIManagerCanReadOwnRecords(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: owner.ID,
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
	record := db.WithdrawalRecord{
		ID:        10,
		UserID:    manager.ID,
		Amount:    5000,
		Status:    "success",
		Channel:   "wechat_ecommerce_fund",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleStaffMerchant(store, manager.ID, merchant)

	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     manager.ID,
		})).
		Times(1).
		Return("manager", nil)

	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Times(1).
		Return(paymentConfig, nil)

	store.EXPECT().
		ListWithdrawalRecords(gomock.Any(), gomock.Eq(db.ListWithdrawalRecordsParams{
			UserID:  manager.ID,
			Channel: "wechat_ecommerce_fund",
			Limit:   20,
			Offset:  0,
		})).
		Times(1).
		Return([]db.WithdrawalRecord{record}, nil)

	store.EXPECT().
		CountWithdrawalRecords(gomock.Any(), gomock.Eq(db.CountWithdrawalRecordsParams{
			UserID:  manager.ID,
			Channel: "wechat_ecommerce_fund",
		})).
		Times(1).
		Return(int64(1), nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantWithdrawalsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "active", resp.AccountStatus)
	require.Len(t, resp.Withdrawals, 1)
	require.Equal(t, record.ID, resp.Withdrawals[0].ID)
	require.Equal(t, manager.ID, record.UserID)
}

func TestGetMerchantAccountWithdrawalAPI_PersistsWithdrawIDFromWechat(t *testing.T) {
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

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: "MW1001",
	})
	require.NoError(t, err)

	store.EXPECT().
		GetWithdrawalRecord(gomock.Any(), int64(88)).
		Return(db.WithdrawalRecord{
			ID:          88,
			UserID:      user.ID,
			Amount:      1200,
			Status:      "pending",
			Channel:     "wechat_ecommerce_fund",
			AccountInfo: accountInfoBytes,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}, nil)

	ecommerce.EXPECT().
		QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), paymentConfig.SubMchID, "MW1001").
		Return(&wechat.EcommerceWithdrawResponse{
			SubMchID:     paymentConfig.SubMchID,
			OutRequestNo: "MW1001",
			WithdrawID:   "withdraw_test_merchant_001",
			Amount:       1200,
			Status:       "CREATE_SUCCESS",
		}, nil)

	updatedAccountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: "MW1001",
		WithdrawID:   "withdraw_test_merchant_001",
	})
	require.NoError(t, err)
	updatedRecord := db.WithdrawalRecord{
		ID:          88,
		UserID:      user.ID,
		Amount:      1200,
		Status:      "pending",
		Channel:     "wechat_ecommerce_fund",
		AccountInfo: updatedAccountInfoBytes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().
		UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(88), arg.ID)
			info := parseMerchantWithdrawAccountInfo(arg.AccountInfo)
			require.Equal(t, "withdraw_test_merchant_001", info.WithdrawID)
			return updatedRecord, nil
		})
	expectMerchantWithdrawQueryFact(t, store, updatedRecord, merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: "MW1001",
		WithdrawID:   "withdraw_test_merchant_001",
	}, "withdraw_test_merchant_001", "CREATE_SUCCESS", "", 9911)
	expectMerchantWithdrawFactApplySuccess(t, store, 9911, 9901, updatedRecord, "MW1001", "withdraw_test_merchant_001", "CREATE_SUCCESS", "", nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals/88", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantWithdrawItem
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "withdraw_test_merchant_001", resp.WithdrawID)
}

func TestCreateMerchantAccountWithdrawAPIReturnsPendingConfirmationWhenWechatCreateAndQueryFail(t *testing.T) {
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
		GetWithdrawalRecordByOutRequestNo(gomock.Any(), gomock.Any()).
		Return(db.WithdrawalRecord{}, db.ErrRecordNotFound)

	store.EXPECT().
		CreateWithdrawalRecord(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.CreateWithdrawalRecordParams) (db.WithdrawalRecord, error) {
			return db.WithdrawalRecord{
				ID:          77,
				UserID:      user.ID,
				Amount:      arg.Amount,
				Status:      arg.Status,
				Channel:     arg.Channel,
				AccountInfo: arg.AccountInfo,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				OutRequestNo: pgtype.Text{
					String: arg.OutRequestNo.String,
					Valid:  arg.OutRequestNo.Valid,
				},
			}, nil
		})

	ecommerce.EXPECT().
		CreateEcommerceWithdraw(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("create timeout"))

	ecommerce.EXPECT().
		QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), paymentConfig.SubMchID, "MW-20260415-001").
		Return(nil, errors.New("query timeout"))

	store.EXPECT().
		UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(77), arg.ID)
			require.Equal(t, "pending", arg.Status)
			require.True(t, arg.Reason.Valid)
			require.Equal(t, "withdraw request submitted, awaiting wechat confirmation", arg.Reason.String)
			accountInfoBytes, marshalErr := json.Marshal(merchantWithdrawAccountInfo{
				MerchantID:   merchant.ID,
				SubMchID:     paymentConfig.SubMchID,
				OutRequestNo: "MW-20260415-001",
				Remark:       "测试提现",
			})
			require.NoError(t, marshalErr)
			return db.WithdrawalRecord{
				ID:          77,
				UserID:      user.ID,
				Amount:      1200,
				Status:      "pending",
				Channel:     merchantWithdrawChannel,
				AccountInfo: accountInfoBytes,
				Reason:      pgtype.Text{String: arg.Reason.String, Valid: true},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				OutRequestNo: pgtype.Text{
					String: "MW-20260415-001",
					Valid:  true,
				},
			}, nil
		})

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreateWithdraw, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
			require.Equal(t, "withdrawal_record", arg.BusinessObjectType.String)
			require.Equal(t, int64(77), arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
			require.Equal(t, "MW-20260415-001", arg.ExternalObjectKey)
			require.False(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
			require.True(t, arg.LastErrorMessage.Valid)
			require.Contains(t, arg.LastErrorMessage.String, "create timeout")
			snapshot := string(arg.ResponseSnapshot)
			require.Contains(t, snapshot, "MW-20260415-001")
			require.Contains(t, snapshot, "query timeout")
			return db.ExternalPaymentCommand{ID: 9702}, nil
		})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"amount":1200,"remark":"测试提现","out_request_no":"MW-20260415-001"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusAccepted, recorder.Code)

	var resp merchantWithdrawCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, merchantWithdrawSyncStatePendingConfirmation, resp.Withdrawal.SyncState)
	require.Equal(t, "微信提现已提交，但微信侧结果暂未确认，系统将继续同步状态。", resp.Withdrawal.SyncMessage)
	require.Equal(t, "withdraw request submitted, awaiting wechat confirmation", resp.Withdrawal.Reason)
	require.Nil(t, resp.Wechat)
}

func TestGetMerchantAccountWithdrawalAPIReturnsStaleSyncStateWhenWechatQueryFails(t *testing.T) {
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

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   merchant.ID,
		SubMchID:     paymentConfig.SubMchID,
		OutRequestNo: "MW1002",
	})
	require.NoError(t, err)

	store.EXPECT().
		GetWithdrawalRecord(gomock.Any(), int64(89)).
		Return(db.WithdrawalRecord{
			ID:          89,
			UserID:      user.ID,
			Amount:      1200,
			Status:      "pending",
			Channel:     merchantWithdrawChannel,
			AccountInfo: accountInfoBytes,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}, nil)

	ecommerce.EXPECT().
		QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), paymentConfig.SubMchID, "MW1002").
		Return(nil, errors.New("wechat timeout"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals/89", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantWithdrawItem
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, merchantWithdrawSyncStateStale, resp.SyncState)
	require.Equal(t, "微信提现状态同步失败，当前展示的是本地缓存结果，请稍后刷新。", resp.SyncMessage)
	require.Equal(t, "pending", resp.Status)
	require.Equal(t, "MW1002", resp.OutRequestNo)
}

func TestGetMerchantFinanceOverviewAPI(t *testing.T) {
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

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				startAt, err := time.Parse("2006-01-02", "2025-11-01")
				require.NoError(t, err)
				endAt, err := time.Parse("2006-01-02", "2025-11-30")
				require.NoError(t, err)
				endAt = endAt.Add(24*time.Hour - time.Nanosecond)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantFinanceOverview(gomock.Any(), gomock.Eq(db.GetMerchantFinanceOverviewParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(db.GetMerchantFinanceOverviewRow{
						CompletedOrders:  100,
						PendingOrders:    5,
						TotalGmv:         1000000,
						TotalIncome:      950000,
						TotalPlatformFee: 20000,
						TotalOperatorFee: 30000,
						PendingIncome:    50000,
					}, nil)

				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Eq(db.GetMerchantPromotionExpensesParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(db.GetMerchantPromotionExpensesRow{
						PromoOrderCount: 10,
						TotalDiscount:   10000,
					}, nil)

				store.EXPECT().
					SumMerchantSettlementAdjustments(gomock.Any(), gomock.Eq(db.SumMerchantSettlementAdjustmentsParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp financeOverviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(100), resp.CompletedOrders)
				require.Equal(t, int64(1000000), resp.TotalGMV)
				require.Equal(t, int64(50000), resp.TotalServiceFee)
				require.Equal(t, int64(940000), resp.NetIncome) // 950000 - 10000
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MissingStartDate",
			query: "?end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "?start_date=2025/11/01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "StartDateAfterEndDate",
			query: "?start_date=2025-12-01&end_date=2025-11-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "DateRangeExceeds365Days",
			query: "?start_date=2024-01-01&end_date=2025-12-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/overview" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantFinanceOrdersAPI(t *testing.T) {
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

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				startAt, err := time.Parse("2006-01-02", "2025-11-01")
				require.NoError(t, err)
				endAt, err := time.Parse("2006-01-02", "2025-11-30")
				require.NoError(t, err)
				endAt = endAt.Add(24*time.Hour - time.Nanosecond)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.ListMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Offset:     0,
						Limit:      20,
					})).
					Times(1).
					Return([]db.ListMerchantFinanceOrdersRow{
						{
							ID:                 1,
							PaymentOrderID:     100,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							Status:             "finished",
							CreatedAt:          time.Now(),
							OrderID:            pgtype.Int8{Int64: 1000, Valid: true},
						},
					}, nil)

				store.EXPECT().
					CountMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.CountMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["orders"])
				require.Equal(t, float64(1), resp["total"])
			},
		},
		{
			name:  "WithPagination",
			query: "?start_date=2025-11-01&end_date=2025-11-30&page=2&limit=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				startAt, err := time.Parse("2006-01-02", "2025-11-01")
				require.NoError(t, err)
				endAt, err := time.Parse("2006-01-02", "2025-11-30")
				require.NoError(t, err)
				endAt = endAt.Add(24*time.Hour - time.Nanosecond)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.ListMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
						Offset:     10,
						Limit:      10,
					})).
					Times(1).
					Return([]db.ListMerchantFinanceOrdersRow{}, nil)

				store.EXPECT().
					CountMerchantFinanceOrders(gomock.Any(), gomock.Eq(db.CountMerchantFinanceOrdersParams{
						MerchantID: merchant.ID,
						StartAt:    startAt,
						EndAt:      endAt,
					})).
					Times(1).
					Return(int64(15), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, float64(2), resp["page"])
				require.Equal(t, float64(10), resp["limit"])
				require.Equal(t, float64(2), resp["total_pages"])
			},
		},
		{
			name:  "InvalidPageNumberNegative",
			query: "?start_date=2025-11-01&end_date=2025-11-30&page=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLimitExceedsMax",
			query: "?start_date=2025-11-01&end_date=2025-11-30&limit=101",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/orders" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantServiceFeesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		RegionID:    1,
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				startAt, err := time.Parse("2006-01-02", "2025-11-01")
				require.NoError(t, err)
				endAt, err := time.Parse("2006-01-02", "2025-11-30")
				require.NoError(t, err)
				endAt = endAt.Add(24*time.Hour - time.Nanosecond)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantServiceFeeDetail(
						gomock.Any(),
						gomock.Eq(db.GetMerchantServiceFeeDetailParams{MerchantID: merchant.ID, StartAt: startAt, EndAt: endAt}),
					).
					Times(1).
					Return([]db.GetMerchantServiceFeeDetailRow{
						{
							Date:        pgtype.Date{Time: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderSource: "takeout",
							OrderCount:  50,
							TotalAmount: 500000,
							PlatformFee: 10000,
							OperatorFee: 15000,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["details"])
				require.Equal(t, float64(10000), resp["total_platform_fee"])
				require.Equal(t, float64(15000), resp["total_operator_fee"])
				require.Equal(t, float64(25000), resp["total_service_fee"])
			},
		},
		{
			name:  "NoAuthorization",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加授权
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/service-fees" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantPromotionExpensesAPI(t *testing.T) {
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

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantPromotionOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantPromotionOrdersRow{
						{
							ID:                  1,
							OrderNo:             "ORD20251115001",
							OrderType:           "takeout",
							Subtotal:            10000,
							DeliveryFee:         500,
							DeliveryFeeDiscount: 500,
							TotalAmount:         10000,
							CreatedAt:           time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantPromotionOrders(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantPromotionExpenses(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantPromotionExpensesRow{
						PromoOrderCount: 1,
						TotalDiscount:   500,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["orders"])
				require.Equal(t, float64(1), resp["total_promo_orders"])
				require.Equal(t, float64(500), resp["total_promo_amount"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/promotions" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantDailyFinanceAPI(t *testing.T) {
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

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantDailyFinance(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetMerchantDailyFinanceRow{
						{
							Date:           pgtype.Date{Time: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC), Valid: true},
							OrderCount:     100,
							TotalGmv:       1000000,
							MerchantIncome: 950000,
							TotalFee:       50000,
						},
					}, nil)

				store.EXPECT().
					ListMerchantDailySettlementAdjustments(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantDailySettlementAdjustmentsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["daily_stats"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/daily" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantSettlementsAPI(t *testing.T) {
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

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_NoStatusFilter",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantSettlements(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ProfitSharingOrder{
						{
							ID:                 1,
							PaymentOrderID:     100,
							MerchantID:         merchant.ID,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							OutOrderNo:         "PSO20251115001",
							Status:             "finished",
							CreatedAt:          time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantSettlements(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantProfitSharingStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantProfitSharingStatsRow{
						TotalOrders:             1,
						TotalAmount:             10000,
						TotalMerchantAmount:     9500,
						TotalPlatformCommission: 200,
						TotalOperatorCommission: 300,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["settlements"])
				require.Equal(t, float64(1), resp["total"])
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "?start_date=2025-11-01&end_date=2025-11-30&status=finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantSettlementsByStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ProfitSharingOrder{
						{
							ID:                 1,
							PaymentOrderID:     100,
							MerchantID:         merchant.ID,
							OrderSource:        "takeout",
							TotalAmount:        10000,
							PlatformCommission: 200,
							OperatorCommission: 300,
							MerchantAmount:     9500,
							OutOrderNo:         "PSO20251115001",
							Status:             "finished",
							CreatedAt:          time.Now(),
						},
					}, nil)

				store.EXPECT().
					CountMerchantSettlementsByStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					GetMerchantProfitSharingStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantProfitSharingStatsRow{
						TotalOrders:             1,
						TotalAmount:             10000,
						TotalMerchantAmount:     9500,
						TotalPlatformCommission: 200,
						TotalOperatorCommission: 300,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["settlements"])
			},
		},
		{
			name:  "InvalidStatus",
			query: "?start_date=2025-11-01&end_date=2025-11-30&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "?start_date=2025-11-01&end_date=2025-11-30",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/finance/settlements" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
