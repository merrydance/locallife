package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	api "github.com/merrydance/locallife/api"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type integrationMerchantAccountBalanceResponse struct {
	SubMchID           string `json:"sub_mch_id"`
	AvailableAmount    int64  `json:"available_amount"`
	PendingAmount      int64  `json:"pending_amount"`
	WithdrawableAmount int64  `json:"withdrawable_amount"`
	AccountType        string `json:"account_type"`
	BalanceDate        string `json:"balance_date"`
	AccountStatus      string `json:"account_status"`
	StatusDesc         string `json:"status_desc"`
}

type integrationPlatformAccountBalanceResponse struct {
	AccountType     string `json:"account_type"`
	BalanceDate     string `json:"balance_date"`
	AvailableAmount int64  `json:"available_amount"`
	PendingAmount   int64  `json:"pending_amount"`
}

func TestMerchantAccountDayEndBalanceIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	region := createIntegrationRegion(t, store)
	owner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, owner.ID, region.ID)

	_, err := store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_finance_integration_001",
		Status:     "active",
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockEcommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	mockEcommerceClient.EXPECT().
		QueryEcommerceFundDayEndBalance(gomock.Any(), "sub_mch_finance_integration_001", "2026-04-05", "DEPOSIT").
		Return(&wechat.EcommerceFundBalanceResponse{
			SubMchID:        "sub_mch_finance_integration_001",
			AvailableAmount: 5600,
			PendingAmount:   40,
			AccountType:     "DEPOSIT",
		}, nil)
	server.SetEcommerceClientForTest(mockEcommerceClient)
	defer server.SetEcommerceClientForTest(nil)

	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/balance?date=2026-04-05&account_type=deposit", nil)
	require.NoError(t, err)
	addAuthorization(t, req, integrationTokenMaker, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp integrationMerchantAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "sub_mch_finance_integration_001", resp.SubMchID)
	require.Equal(t, int64(5600), resp.AvailableAmount)
	require.Equal(t, int64(40), resp.PendingAmount)
	require.Equal(t, int64(5600), resp.WithdrawableAmount)
	require.Equal(t, "DEPOSIT", resp.AccountType)
	require.Equal(t, "2026-04-05", resp.BalanceDate)
	require.Equal(t, "active", resp.AccountStatus)
	require.Equal(t, "收付通账户已激活", resp.StatusDesc)
}

func TestPlatformAccountBalanceIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	adminUser := createIntegrationUser(t, store)
	_, err := store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          adminUser.ID,
		Role:            api.RoleAdmin,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockEcommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	mockEcommerceClient.EXPECT().
		QueryPlatformFundBalance(gomock.Any(), "OPERATION").
		Return(&wechat.PlatformFundBalanceResponse{
			AvailableAmount: 43210,
			PendingAmount:   321,
		}, nil)
	mockEcommerceClient.EXPECT().
		QueryPlatformFundDayEndBalance(gomock.Any(), "FEES", "2026-04-05").
		Return(&wechat.PlatformFundBalanceResponse{
			AvailableAmount: 2100,
			PendingAmount:   12,
		}, nil)
	server.SetEcommerceClientForTest(mockEcommerceClient)
	defer server.SetEcommerceClientForTest(nil)

	realtimeReq, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance?account_type=operation", nil)
	require.NoError(t, err)
	addAuthorization(t, realtimeReq, integrationTokenMaker, adminUser.ID, time.Minute)

	realtimeRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(realtimeRecorder, realtimeReq)
	require.Equal(t, http.StatusOK, realtimeRecorder.Code)

	var realtimeResp integrationPlatformAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, realtimeRecorder.Body.Bytes(), &realtimeResp)
	require.Equal(t, "OPERATION", realtimeResp.AccountType)
	require.Equal(t, int64(43210), realtimeResp.AvailableAmount)
	require.Equal(t, int64(321), realtimeResp.PendingAmount)
	require.Equal(t, "", realtimeResp.BalanceDate)

	dayEndReq, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance?date=2026-04-05&account_type=fees", nil)
	require.NoError(t, err)
	addAuthorization(t, dayEndReq, integrationTokenMaker, adminUser.ID, time.Minute)

	dayEndRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(dayEndRecorder, dayEndReq)
	require.Equal(t, http.StatusOK, dayEndRecorder.Code)

	var dayEndResp integrationPlatformAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, dayEndRecorder.Body.Bytes(), &dayEndResp)
	require.Equal(t, "FEES", dayEndResp.AccountType)
	require.Equal(t, "2026-04-05", dayEndResp.BalanceDate)
	require.Equal(t, int64(2100), dayEndResp.AvailableAmount)
	require.Equal(t, int64(12), dayEndResp.PendingAmount)
}
