package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockosp "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func applymentCatalogRequest(t *testing.T, method string, url string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	ctx.Request = request
	return recorder, ctx
}

func TestListApplymentBanks_OrdinaryServiceProviderClientUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(nil)

	recorder, ctx := applymentCatalogRequest(t, http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_PRIVATE")

	server.listApplymentBanks(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "普通服务商开户银行目录暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", response.Error)
}

func TestApplymentBankCatalogValidationErrorsReturnActionableChinese(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	tests := []struct {
		name      string
		url       string
		setupPath func(*gin.Context)
		call      func(*gin.Context)
		wantError string
	}{
		{
			name:      "missing account type",
			url:       "/v1/merchant/applyment/banks",
			call:      server.listApplymentBanks,
			wantError: "请选择开户账户类型后再查询银行目录",
		},
		{
			name:      "missing account number",
			url:       "/v1/merchant/applyment/banks/search-by-bank-account?account_type=ACCOUNT_TYPE_PRIVATE",
			call:      server.searchApplymentBanksByAccount,
			wantError: "请填写开户账户类型和银行卡号后再识别开户银行",
		},
		{
			name:      "invalid province code",
			url:       "/v1/merchant/applyment/areas/provinces/bad/cities",
			setupPath: func(ctx *gin.Context) { ctx.Params = gin.Params{{Key: "province_code", Value: "bad"}} },
			call:      server.listApplymentCities,
			wantError: "省份编码无效，请返回省份列表重新选择",
		},
		{
			name:      "missing bank alias",
			url:       "/v1/merchant/applyment/banks//branches?city_code=10",
			setupPath: func(ctx *gin.Context) { ctx.Params = gin.Params{{Key: "bank_alias_code", Value: ""}} },
			call:      server.listApplymentBankBranches,
			wantError: "银行别名编码缺失，请返回银行列表重新选择",
		},
		{
			name:      "missing city code",
			url:       "/v1/merchant/applyment/banks/ABC/branches",
			setupPath: func(ctx *gin.Context) { ctx.Params = gin.Params{{Key: "bank_alias_code", Value: "ABC"}} },
			call:      server.listApplymentBankBranches,
			wantError: "城市编码无效，请返回城市列表重新选择后再查询支行",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder, ctx := applymentCatalogRequest(t, http.MethodGet, tt.url)
			if tt.setupPath != nil {
				tt.setupPath(ctx)
			}

			tt.call(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			var response ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tt.wantError, response.Error)
			require.NotContains(t, response.Error, "request_id")
			require.NotContains(t, response.Error, "sql")
			require.NotContains(t, response.Error, "provider")
		})
	}
}

func TestListApplymentBanksProviderErrorReturnsActionableChinese(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	ordinaryClient.EXPECT().
		ListPersonalBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Return(nil, errors.New("provider request_id=req_123 sql connection failed"))

	recorder, ctx := applymentCatalogRequest(t, http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_PRIVATE")

	server.listApplymentBanks(ctx)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "普通服务商开户银行目录查询失败，请稍后重试；如持续失败请联系平台管理员检查微信支付银行目录服务日志", response.Error)
	require.NotContains(t, response.Error, "request_id")
	require.NotContains(t, response.Error, "sql")
	require.NotContains(t, response.Error, "provider")
}

func TestSearchApplymentBanksBusinessAccountSkipsProviderLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	recorder, ctx := applymentCatalogRequest(t, http.MethodGet, "/v1/merchant/applyment/banks/search-by-bank-account?account_type=ACCOUNT_TYPE_BUSINESS&account_number=123")

	server.searchApplymentBanksByAccount(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applymentBankSearchResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Total)
	require.Equal(t, []applymentBankOption{}, response.Matches)
}

func TestListApplymentBanksSuccessUsesOrdinaryProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockosp.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)

	ordinaryClient.EXPECT().
		ListCorporateBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Return(&ospcontracts.CapitalBankListResponse{
			Data: []ospcontracts.CapitalBank{
				{
					BankAlias:       "测试银行",
					BankAliasCode:   "TEST",
					AccountBank:     "测试银行",
					AccountBankCode: 1001,
					NeedBankBranch:  true,
				},
			},
			Count:      1,
			TotalCount: 1,
		}, nil)

	recorder, ctx := applymentCatalogRequest(t, http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_BUSINESS")

	server.listApplymentBanks(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applymentBankListResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Total)
	require.Equal(t, "测试银行", response.Banks[0].BankAlias)
}
