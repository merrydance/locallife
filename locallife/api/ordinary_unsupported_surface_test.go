package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockosp "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOrdinaryServiceProviderGatesMerchantFundManagementRoutes(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          701,
		OwnerUserID: user.ID,
		RegionID:    11,
		Name:        "普通服务商商户",
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "MerchantAccountBalance", method: http.MethodGet, path: "/v1/merchant/finance/account/balance"},
		{name: "MerchantWithdrawalList", method: http.MethodGet, path: "/v1/merchant/finance/account/withdrawals"},
		{name: "MerchantWithdrawalDetail", method: http.MethodGet, path: "/v1/merchant/finance/account/withdrawals/17"},
		{name: "MerchantWithdrawalCreate", method: http.MethodPost, path: "/v1/merchant/finance/account/withdraw", body: `{}`},
		{name: "MerchantCancelWithdrawEligibility", method: http.MethodGet, path: "/v1/merchant/finance/account/cancel-withdraw/eligibility"},
		{name: "MerchantCancelWithdrawList", method: http.MethodGet, path: "/v1/merchant/finance/account/cancel-withdraw/applications"},
		{name: "MerchantCancelWithdrawDetail", method: http.MethodGet, path: "/v1/merchant/finance/account/cancel-withdraw/applications/21"},
		{name: "MerchantCancelWithdrawCreate", method: http.MethodPost, path: "/v1/merchant/finance/account/cancel-withdraw/applications", body: `{}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectResolveSingleOwnedMerchant(store, user.ID, merchant)

			server := newTestServer(t, store)
			server.SetOrdinaryServiceProviderClientForTest(mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl))

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			var resp APIResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, CodeServiceUnavail, resp.Code)
			require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
		})
	}
}

func TestPlatformEcommerceFundManagementRoutesStayDisabledWhenOrdinaryClientMissing(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance?account_type=operation", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
}

func TestOrdinaryServiceProviderGatesOperatorSubsidyRoutes(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:           801,
		UserID:       user.ID,
		RegionID:     22,
		Name:         "普通服务商运营商",
		ContactName:  "运营员",
		ContactPhone: "13800138000",
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	testCases := []struct {
		name string
		path string
		body string
	}{
		{name: "CreateSubsidy", path: "/v1/operators/me/payment-orders/901/subsidies", body: `{}`},
		{name: "ReturnSubsidy", path: "/v1/operators/me/payment-orders/901/subsidies/return", body: `{}`},
		{name: "CancelSubsidy", path: "/v1/operators/me/payment-orders/901/subsidies/cancel"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListUserRoles(gomock.Any(), user.ID).
				Return([]db.UserRole{{
					UserID:          user.ID,
					Role:            RoleOperator,
					Status:          "active",
					RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
				}}, nil)
			store.EXPECT().
				GetOperatorByUser(gomock.Any(), user.ID).
				Return(operator, nil)

			server := newTestServer(t, store)
			server.SetOrdinaryServiceProviderClientForTest(mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl))

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			var resp APIResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, CodeServiceUnavail, resp.Code)
			require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
		})
	}
}

func TestOrdinaryServiceProviderGatesPlatformAbnormalRefundRoute(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl))

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/platform/refunds/99/apply-abnormal-refund", strings.NewReader(`{"type":"MERCHANT_BANK_CARD"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, ordinaryServiceProviderUnsupportedFundsMessage, resp.Message)
}

func TestOrdinaryServiceProviderGatesProfitSharingReceiverLifecycleRepair(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl))

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/platform/profit-sharing/receiver-lifecycle/repair", strings.NewReader(`{"owner_type":"operator","owner_id":66,"desired_state":"present"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "普通服务商模式不支持平台收付通分账接收方预热或修复；普通服务商分账接收方会按支付单和子商户号自动同步，如支付、退款或分账被限制，请联系平台管理员查看普通服务商商户管控诊断", resp.Message)
}
