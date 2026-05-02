package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
	require.Equal(t, "商户收付通资金管理服务未配置；普通服务商模式请前往微信支付商户平台/商家助手处理资金操作", resp.Error)
}

func TestGetMerchantAccountBalanceAPIOrdinaryServiceProviderGateSkipsLegacyConfigLookup(t *testing.T) {
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

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func TestGetMerchantAccountBalanceAPIOrdinaryServiceProviderGateSkipsLegacyDayEndBalance(t *testing.T) {
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

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance?date=2026-04-05&account_type=deposit", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func TestGetMerchantAccountBalanceAPIOrdinaryServiceProviderGateRunsBeforeLegacyDayEndValidation(t *testing.T) {
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

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance?date=2026-04-05&account_type=fees", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func TestGetMerchantAccountBalanceAPIOrdinaryServiceProviderGateSkipsLegacyNoAuthMapping(t *testing.T) {
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

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
}

func requireOrdinaryUnsupportedFundsAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"amount":1200,"remark":"测试提现","out_request_no":"MW-20260425-001"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals/88", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"amount":1200,"remark":"测试提现","out_request_no":"MW-20260415-001"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/withdrawals/89", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	requireOrdinaryUnsupportedFundsAPIResponse(t, recorder)
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
