package api

import (
	"encoding/json"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetPlatformAccountBalanceAPI(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "RealtimeOK",
			query: "?account_type=operation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

				ecommerce.EXPECT().
					QueryPlatformFundBalance(gomock.Any(), "OPERATION").
					Return(&wechat.PlatformFundBalanceResponse{
						AvailableAmount: 32100,
						PendingAmount:   12,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp platformAccountBalanceResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "OPERATION", resp.AccountType)
				require.Equal(t, int64(32100), resp.AvailableAmount)
				require.Equal(t, int64(12), resp.PendingAmount)
			},
		},
		{
			name:  "DayEndOK",
			query: "?date=2026-04-05&account_type=fees",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

				ecommerce.EXPECT().
					QueryPlatformFundDayEndBalance(gomock.Any(), "FEES", "2026-04-05").
					Return(&wechat.PlatformFundBalanceResponse{
						AvailableAmount: 1200,
						PendingAmount:   3,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp platformAccountBalanceResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "FEES", resp.AccountType)
				require.Equal(t, "2026-04-05", resp.BalanceDate)
				require.Equal(t, int64(1200), resp.AvailableAmount)
			},
		},
		{
			name:  "InvalidAccountType",
			query: "?account_type=deposit",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
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
			ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerce)

			server := newTestServer(t, store)
			server.SetEcommerceClientForTest(ecommerce)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance"+tc.query, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetPlatformAccountBalanceAPINoAuthReturnsExplicitMessage(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	ecommerce.EXPECT().
		QueryPlatformFundBalance(gomock.Any(), wechatcontracts.FundManagementAccountTypeBasic).
		Return(nil, &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: wechaterrorcodes.FundManagementCodeNoAuth, Message: "no auth"})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "微信侧暂无该账户查询权限，请联系管理员检查收付通配置", resp.Message)
}
