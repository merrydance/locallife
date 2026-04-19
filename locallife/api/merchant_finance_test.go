package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	store.EXPECT().
		UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(88), arg.ID)
			info := parseMerchantWithdrawAccountInfo(arg.AccountInfo)
			require.Equal(t, "withdraw_test_merchant_001", info.WithdrawID)
			return db.WithdrawalRecord{
				ID:          88,
				UserID:      user.ID,
				Amount:      1200,
				Status:      "pending",
				Channel:     "wechat_ecommerce_fund",
				AccountInfo: arg.AccountInfo,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		})

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
