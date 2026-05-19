package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeAPIBaofuWithdrawClient struct {
	balanceReqs  []baofucontracts.BalanceQueryRequest
	withdrawReqs []baofucontracts.WithdrawRequest
	balanceRes   *baofucontracts.BalanceResult
	withdrawRes  *baofucontracts.WithdrawResult
	balanceErr   error
	withdrawErr  error
}

func (c *fakeAPIBaofuWithdrawClient) QueryBalance(_ context.Context, req baofucontracts.BalanceQueryRequest) (*baofucontracts.BalanceResult, error) {
	c.balanceReqs = append(c.balanceReqs, req)
	return c.balanceRes, c.balanceErr
}

func (c *fakeAPIBaofuWithdrawClient) CreateWithdraw(_ context.Context, req baofucontracts.WithdrawRequest) (*baofucontracts.WithdrawResult, error) {
	c.withdrawReqs = append(c.withdrawReqs, req)
	return c.withdrawRes, c.withdrawErr
}

func (c *fakeAPIBaofuWithdrawClient) QueryWithdraw(context.Context, baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error) {
	return nil, nil
}

func configureBaofuWithdrawServiceForAPITest(server *Server, store db.Store, client *fakeAPIBaofuWithdrawClient) {
	server.baofuWithdrawService = logic.NewBaofuWithdrawService(store, client, logic.BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
}

func activeBaofuWithdrawalBinding(ownerType string, ownerID int64) db.BaofuAccountBinding {
	return db.BaofuAccountBinding{
		ID:          ownerID + 1000,
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "SECRET_CONTRACT_" + ownerType, Valid: true},
	}
}

func TestBaofuWithdrawalBalanceRoutesReturnStableUnavailableWhenServiceMissing(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	platformAdmin, _ := randomUser(t)
	operatorUser, _ := randomUser(t)
	operator := randomOperator(operatorUser.ID)
	riderUser, _ := randomUser(t)
	rider := randomRider(riderUser.ID)
	rider.Status = "approved"

	testCases := []struct {
		name       string
		method     string
		path       string
		userID     int64
		buildStubs func(store *mockdb.MockStore)
	}{
		{
			name:   "merchant balance",
			method: http.MethodGet,
			path:   "/v1/merchant/finance/baofu-withdrawal/balance",
			userID: merchantOwner.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
		},
		{
			name:   "platform balance",
			method: http.MethodGet,
			path:   "/v1/platform/finance/baofu-withdrawal/balance",
			userID: platformAdmin.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), platformAdmin.ID).
					Return([]db.UserRole{{UserID: platformAdmin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
		},
		{
			name:   "operator balance",
			method: http.MethodGet,
			path:   "/v1/operators/me/finance/baofu-withdrawal/balance",
			userID: operatorUser.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), operatorUser.ID).
					Return([]db.UserRole{{UserID: operatorUser.ID, Role: RoleOperator, Status: "active"}}, nil)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), operatorUser.ID).
					Return(operator, nil)
			},
		},
		{
			name:   "rider income balance",
			method: http.MethodGet,
			path:   "/v1/rider/income/baofu-withdrawal/balance",
			userID: riderUser.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), riderUser.ID).
					Return(rider, nil)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(tc.method, tc.path, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.userID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			var resp APIResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, CodeServiceUnavail, resp.Code)
			require.Equal(t, "提现服务暂不可用，请稍后再试", resp.Message)
		})
	}
}

func TestBaofuWithdrawalBalanceRoutesUseServerResolvedOwnerScope(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	platformAdmin, _ := randomUser(t)
	operatorUser, _ := randomUser(t)
	operator := randomOperator(operatorUser.ID)
	riderUser, _ := randomUser(t)
	rider := randomRider(riderUser.ID)
	rider.Status = "approved"

	testCases := []struct {
		name       string
		path       string
		userID     int64
		ownerType  string
		ownerID    int64
		buildStubs func(store *mockdb.MockStore)
	}{
		{
			name:      "merchant",
			path:      "/v1/merchant/finance/baofu-withdrawal/balance",
			userID:    merchantOwner.ID,
			ownerType: db.BaofuAccountOwnerTypeMerchant,
			ownerID:   merchant.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
		},
		{
			name:      "platform",
			path:      "/v1/platform/finance/baofu-withdrawal/balance",
			userID:    platformAdmin.ID,
			ownerType: db.BaofuAccountOwnerTypePlatform,
			ownerID:   platformBaofuAccountOwnerID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), platformAdmin.ID).
					Return([]db.UserRole{{UserID: platformAdmin.ID, Role: RoleAdmin, Status: "active"}}, nil)
			},
		},
		{
			name:      "operator",
			path:      "/v1/operators/me/finance/baofu-withdrawal/balance",
			userID:    operatorUser.ID,
			ownerType: db.BaofuAccountOwnerTypeOperator,
			ownerID:   operator.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), operatorUser.ID).
					Return([]db.UserRole{{UserID: operatorUser.ID, Role: RoleOperator, Status: "active"}}, nil)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), operatorUser.ID).
					Return(operator, nil)
			},
		},
		{
			name:      "rider income",
			path:      "/v1/rider/income/baofu-withdrawal/balance",
			userID:    riderUser.ID,
			ownerType: db.BaofuAccountOwnerTypeRider,
			ownerID:   rider.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), riderUser.ID).
					Return(rider, nil)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)
			binding := activeBaofuWithdrawalBinding(tc.ownerType, tc.ownerID)
			store.EXPECT().
				GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
					OwnerType: tc.ownerType,
					OwnerID:   tc.ownerID,
				}).
				Return(binding, nil)

			client := &fakeAPIBaofuWithdrawClient{
				balanceRes: &baofucontracts.BalanceResult{
					ContractNo:         binding.ContractNo.String,
					AvailableAmountFen: 2600,
					PendingAmountFen:   300,
					LedgerAmountFen:    3200,
					FrozenAmountFen:    300,
				},
			}
			server := newTestServer(t, store)
			configureBaofuWithdrawServiceForAPITest(server, store, client)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, tc.path, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.userID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var resp baofuWithdrawalBalanceResponse
			requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
			require.Equal(t, int64(2600), resp.AvailableAmount)
			require.Equal(t, int64(300), resp.PendingAmount)
			require.Equal(t, int64(3200), resp.LedgerAmount)
			require.Equal(t, int64(300), resp.FrozenAmount)
			require.True(t, resp.CanWithdraw)
			require.Len(t, client.balanceReqs, 1)
			require.Equal(t, binding.ContractNo.String, client.balanceReqs[0].ContractNo)
			require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
		})
	}
}

func TestListBaofuWithdrawalsReturnsStablePaginationAndStatusText(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	createdAt := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(3 * time.Minute)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		CountBaofuWithdrawalOrdersByOwner(gomock.Any(), db.CountBaofuWithdrawalOrdersByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(int64(21), nil)
	store.EXPECT().
		ListBaofuWithdrawalOrdersByOwner(gomock.Any(), db.ListBaofuWithdrawalOrdersByOwnerParams{
			OwnerType:   db.BaofuAccountOwnerTypeMerchant,
			OwnerID:     merchant.ID,
			OffsetCount: 20,
			LimitCount:  20,
		}).
		Return([]db.BaofuWithdrawalOrder{
			{
				ID:           91,
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				OwnerID:      merchant.ID,
				OutRequestNo: "MBW202605191000000001",
				Amount:       1200,
				Status:       db.BaofuWithdrawalStatusSucceeded,
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
			},
		}, nil)

	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, &fakeAPIBaofuWithdrawClient{})

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/baofu-withdrawal/withdrawals?page=2&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp baofuWithdrawalsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(21), resp.Total)
	require.Equal(t, int32(2), resp.Page)
	require.Equal(t, int32(20), resp.Limit)
	require.Equal(t, int64(2), resp.TotalPages)
	require.Len(t, resp.Withdrawals, 1)
	require.Equal(t, int64(91), resp.Withdrawals[0].ID)
	require.Equal(t, "提现成功", resp.Withdrawals[0].StatusText)
	require.Equal(t, createdAt.Format(time.RFC3339), resp.Withdrawals[0].CreatedAt)
	require.Equal(t, updatedAt.Format(time.RFC3339), resp.Withdrawals[0].UpdatedAt)
}

func TestGetBaofuWithdrawalDoesNotLeakCrossOwnerOrder(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuWithdrawalOrder(gomock.Any(), int64(55)).
		Return(db.BaofuWithdrawalOrder{
			ID:        55,
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   9901,
			Amount:    1200,
			Status:    db.BaofuWithdrawalStatusProcessing,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)

	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, &fakeAPIBaofuWithdrawClient{})

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/baofu-withdrawal/withdrawals/55", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeNotFound, resp.Code)
}

func TestGetBaofuWithdrawalReturnsOwnedOrderWithoutProviderSecrets(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	createdAt := time.Date(2026, 5, 19, 11, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuWithdrawalOrder(gomock.Any(), int64(56)).
		Return(db.BaofuWithdrawalOrder{
			ID:              56,
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			OutRequestNo:    "MBW202605191100000001",
			BaofuWithdrawNo: pgtype.Text{String: "SECRET_BAOFU_WITHDRAW_NO", Valid: true},
			Amount:          1800,
			Status:          db.BaofuWithdrawalStatusReturned,
			RawSnapshot:     []byte(`{"contractNo":"SECRET_CONTRACT","state":"3"}`),
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		}, nil)

	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, &fakeAPIBaofuWithdrawClient{})

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/baofu-withdrawal/withdrawals/56", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(56), resp.Withdrawal.ID)
	require.Equal(t, "提现退票", resp.Withdrawal.StatusText)
	require.NotContains(t, recorder.Body.String(), "SECRET_BAOFU_WITHDRAW_NO")
	require.NotContains(t, recorder.Body.String(), "SECRET_CONTRACT")
}

func TestCreateMerchantBaofuWithdrawalManagerForbidden(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleStaffMerchant(store, manager.ID, merchant)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     manager.ID,
		}).
		Return("manager", nil)

	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, &fakeAPIBaofuWithdrawClient{})

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestCreateBaofuWithdrawalRejectsInvalidAmountBeforeProviderCall(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	client := &fakeAPIBaofuWithdrawClient{}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":99,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, client.balanceReqs)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalMapsBalanceQueryFailureToBadGateway(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := activeBaofuWithdrawalBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(binding, nil)

	client := &fakeAPIBaofuWithdrawClient{balanceErr: errors.New("provider timeout")}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "提现账户余额暂不可确认，请稍后刷新", resp.Message)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalRejectsAmountAboveAvailableBalance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := activeBaofuWithdrawalBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(binding, nil)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 1000},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeConflict, resp.Code)
	require.Equal(t, "可提现金额不足", resp.Message)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalSubmitsProviderRequestAndReturnsProcessingOrder(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := activeBaofuWithdrawalBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID)
	createdAt := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	var capturedOutRequestNo string

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Times(1).
		Return(binding, nil)
	store.EXPECT().
		CreateBaofuWithdrawalOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderParams) (db.BaofuWithdrawalOrder, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeMerchant, arg.OwnerType)
			require.Equal(t, merchant.ID, arg.OwnerID)
			require.Equal(t, binding.ID, arg.AccountBindingID)
			require.Equal(t, int64(1200), arg.Amount)
			require.Equal(t, db.BaofuWithdrawalStatusProcessing, arg.Status)
			require.True(t, strings.HasPrefix(arg.OutRequestNo, "MBW"), arg.OutRequestNo)
			capturedOutRequestNo = arg.OutRequestNo
			return db.BaofuWithdrawalOrder{
				ID:               91,
				OwnerType:        arg.OwnerType,
				OwnerID:          arg.OwnerID,
				AccountBindingID: arg.AccountBindingID,
				OutRequestNo:     arg.OutRequestNo,
				Amount:           arg.Amount,
				Status:           arg.Status,
				CreatedAt:        createdAt,
				UpdatedAt:        createdAt,
			}, nil
		})
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
			require.Equal(t, capturedOutRequestNo, arg.ExternalObjectKey)
			return db.ExternalPaymentCommand{ID: 901}, nil
		})
	store.EXPECT().
		UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateBaofuWithdrawalOrderToProcessingParams) (db.BaofuWithdrawalOrder, error) {
			require.Equal(t, int64(91), arg.ID)
			require.Equal(t, "BF_WITHDRAW_001", arg.BaofuWithdrawNo.String)
			return db.BaofuWithdrawalOrder{
				ID:              91,
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         merchant.ID,
				OutRequestNo:    capturedOutRequestNo,
				BaofuWithdrawNo: arg.BaofuWithdrawNo,
				Amount:          1200,
				Status:          db.BaofuWithdrawalStatusProcessing,
				CreatedAt:       createdAt,
				UpdatedAt:       createdAt,
			}, nil
		})

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes:  &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{BaofuWithdrawNo: "BF_WITHDRAW_001", Raw: []byte(`{"state":"1"}`)},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, client.withdrawReqs, 1)
	require.Equal(t, "PAYOUT_MER", client.withdrawReqs[0].MerchantID)
	require.Equal(t, "PAYOUT_TER", client.withdrawReqs[0].TerminalID)
	require.Equal(t, binding.ContractNo.String, client.withdrawReqs[0].ContractNo)
	require.Equal(t, capturedOutRequestNo, client.withdrawReqs[0].TransSerialNo)
	require.Equal(t, int64(1200), client.withdrawReqs[0].AmountFen)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", client.withdrawReqs[0].NotifyURL)

	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(91), resp.Withdrawal.ID)
	require.Equal(t, capturedOutRequestNo, resp.Withdrawal.OutRequestNo)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, resp.Withdrawal.Status)
	require.Equal(t, "提现处理中", resp.Withdrawal.StatusText)
	require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
	require.NotContains(t, recorder.Body.String(), "BF_WITHDRAW_001")
}
