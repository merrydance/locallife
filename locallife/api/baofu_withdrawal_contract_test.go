package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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
		ID:           ownerID + 1000,
		OwnerType:    ownerType,
		OwnerID:      ownerID,
		AccountType:  baofuWithdrawalAccountTypeForOwner(ownerType),
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "SECRET_CONTRACT_" + ownerType, Valid: true},
		SharingMerID: pgtype.Text{String: "SECRET_SHARE_" + ownerType, Valid: true},
	}
}

func activeBaofuWithdrawalBindingWithoutFeeMember(ownerType string, ownerID int64) db.BaofuAccountBinding {
	binding := activeBaofuWithdrawalBinding(ownerType, ownerID)
	binding.SharingMerID = pgtype.Text{}
	return binding
}

func expectBaofuWithdrawalLocalReservedAmount(store *mockdb.MockStore, binding db.BaofuAccountBinding, reservedAmountFen int64) {
	call := store.EXPECT().
		GetBaofuWithdrawalAccountGuardByOwner(gomock.Any(), db.GetBaofuWithdrawalAccountGuardByOwnerParams{
			OwnerType:        binding.OwnerType,
			OwnerID:          binding.OwnerID,
			AccountBindingID: binding.ID,
		})
	if reservedAmountFen <= 0 {
		call.Return(db.BaofuWithdrawalAccountGuard{}, db.ErrRecordNotFound)
		return
	}
	call.Return(db.BaofuWithdrawalAccountGuard{
		OwnerType:         binding.OwnerType,
		OwnerID:           binding.OwnerID,
		AccountBindingID:  binding.ID,
		ReservedAmountFen: reservedAmountFen,
	}, nil)
}

func expectNoBaofuWithdrawalIdempotency(store *mockdb.MockStore, ownerType string, ownerID int64, idempotencyKey string) {
	store.EXPECT().
		GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
			OwnerType: ownerType,
			OwnerID:   ownerID,
			IdempotencyKey: pgtype.Text{
				String: idempotencyKey,
				Valid:  true,
			},
		}).
		Return(db.BaofuWithdrawalOrder{}, db.ErrRecordNotFound)
}

func baofuWithdrawalIdempotencyHashForTest(ownerType string, ownerID int64, amountFen int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("owner_type=%s\nowner_id=%d\namount_fen=%d", strings.TrimSpace(ownerType), ownerID, amountFen)))
	return fmt.Sprintf("sha256:%x", sum)
}

func baofuWithdrawalAccountTypeForOwner(ownerType string) string {
	switch ownerType {
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		return db.BaofuAccountTypePersonal
	default:
		return db.BaofuAccountTypeBusiness
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
	rider.Status = "active"

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
			expectBaofuWithdrawalLocalReservedAmount(store, binding, 0)

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
			require.Equal(t, binding.AccountType, client.balanceReqs[0].AccountType)
			require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
		})
	}
}

func TestBaofuWithdrawalBalanceDeductsLocalReservedAmount(t *testing.T) {
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
	expectBaofuWithdrawalLocalReservedAmount(store, binding, 1150)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{
			ContractNo:         binding.ContractNo.String,
			AvailableAmountFen: 1200,
			PendingAmountFen:   300,
			LedgerAmountFen:    1500,
			FrozenAmountFen:    0,
		},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/baofu-withdrawal/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp baofuWithdrawalBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(50), resp.AvailableAmount)
	require.Equal(t, int64(300), resp.PendingAmount)
	require.False(t, resp.CanWithdraw)
	require.Equal(t, "可提现金额不足", resp.DisabledReason)
	require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
}

func TestRiderBaofuWithdrawalBalanceReturnsUnopenedStateWhenBindingMissing(t *testing.T) {
	riderUser, _ := randomUser(t)
	rider := randomRider(riderUser.ID)
	rider.Status = "approved"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), riderUser.ID).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 2600},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/income/baofu-withdrawal/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, riderUser.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp baofuWithdrawalBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, resp.AccountStatus)
	require.Equal(t, "结算账户未开通", resp.StatusDesc)
	require.Zero(t, resp.AvailableAmount)
	require.Zero(t, resp.PendingAmount)
	require.Zero(t, resp.LedgerAmount)
	require.Zero(t, resp.FrozenAmount)
	require.Equal(t, baofuWithdrawalMinAmountFen, resp.MinWithdrawAmount)
	require.Equal(t, baofuWithdrawalMaxAmountFen, resp.MaxWithdrawAmount)
	require.False(t, resp.CanWithdraw)
	require.Equal(t, "请先开通结算账户后再提现", resp.DisabledReason)
	require.Empty(t, client.balanceReqs)
}

func TestMerchantFinanceOverviewAndBaofuWithdrawalBalanceUseSeparateTruthSources(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	startAt, err := time.Parse("2006-01-02", "2026-06-01")
	require.NoError(t, err)
	endAt, err := time.Parse("2006-01-02", "2026-06-10")
	require.NoError(t, err)
	endAt = endAt.Add(24*time.Hour - time.Nanosecond)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantFinanceOverview(gomock.Any(), db.GetMerchantFinanceOverviewParams{
			MerchantID: merchant.ID,
			StartAt:    startAt,
			EndAt:      endAt,
		}).
		Times(1).
		Return(db.GetMerchantFinanceOverviewRow{
			CompletedOrders:                 12,
			PendingOrders:                   2,
			TotalGmv:                        600000,
			TotalMerchantReceivableAmount:   500000,
			TotalPlatformServiceFeeAmount:   60000,
			TotalPaymentChannelFeeAmount:    5000,
			PendingMerchantReceivableAmount: 7000,
		}, nil)
	store.EXPECT().
		GetMerchantPromotionExpenses(gomock.Any(), db.GetMerchantPromotionExpensesParams{
			MerchantID: merchant.ID,
			StartAt:    startAt,
			EndAt:      endAt,
		}).
		Times(1).
		Return(db.GetMerchantPromotionExpensesRow{
			PromoOrderCount: 3,
			TotalDiscount:   20000,
		}, nil)
	store.EXPECT().
		SumMerchantSettlementAdjustments(gomock.Any(), db.SumMerchantSettlementAdjustmentsParams{
			MerchantID: merchant.ID,
			StartAt:    startAt,
			EndAt:      endAt,
		}).
		Times(1).
		Return(int64(15000), nil)
	binding := activeBaofuWithdrawalBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Times(1).
		Return(binding, nil)
	expectBaofuWithdrawalLocalReservedAmount(store, binding, 0)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{
			ContractNo:         binding.ContractNo.String,
			AvailableAmountFen: 12345,
			PendingAmountFen:   678,
			LedgerAmountFen:    99999,
			FrozenAmountFen:    111,
		},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	overviewRecorder := httptest.NewRecorder()
	overviewRequest, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/overview?start_date=2026-06-01&end_date=2026-06-10", nil)
	require.NoError(t, err)
	addAuthorization(t, overviewRequest, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)
	server.router.ServeHTTP(overviewRecorder, overviewRequest)

	require.Equal(t, http.StatusOK, overviewRecorder.Code)
	require.Empty(t, client.balanceReqs)
	var overview map[string]interface{}
	requireUnmarshalAPIResponseData(t, overviewRecorder.Body.Bytes(), &overview)
	require.Equal(t, float64(515000), overview["total_merchant_receivable_amount"])
	require.Equal(t, float64(7000), overview["pending_merchant_receivable_amount"])
	require.Equal(t, float64(495000), overview["net_income"])
	require.NotContains(t, overviewRecorder.Body.String(), "available_amount")

	balanceRecorder := httptest.NewRecorder()
	balanceRequest, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/baofu-withdrawal/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, balanceRequest, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)
	server.router.ServeHTTP(balanceRecorder, balanceRequest)

	require.Equal(t, http.StatusOK, balanceRecorder.Code)
	require.Len(t, client.balanceReqs, 1)
	require.Equal(t, "COLLECT_MER", client.balanceReqs[0].MerchantID)
	require.Equal(t, binding.ContractNo.String, client.balanceReqs[0].ContractNo)
	var balance baofuWithdrawalBalanceResponse
	requireUnmarshalAPIResponseData(t, balanceRecorder.Body.Bytes(), &balance)
	require.Equal(t, int64(12345), balance.AvailableAmount)
	require.Equal(t, int64(678), balance.PendingAmount)
	require.Equal(t, int64(99999), balance.LedgerAmount)
	require.Equal(t, int64(111), balance.FrozenAmount)
	require.NotContains(t, balanceRecorder.Body.String(), "total_merchant_receivable_amount")
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
	require.Equal(t, "提现已退回", resp.Withdrawal.StatusText)
	require.Equal(t, "资金已退回至宝付结算账户，请刷新可提现余额后按需重新申请", resp.Withdrawal.SyncMessage)
	require.NotContains(t, recorder.Body.String(), "SECRET_BAOFU_WITHDRAW_NO")
	require.NotContains(t, recorder.Body.String(), "SECRET_CONTRACT")
}

func TestCreateMerchantBaofuWithdrawalRequiresIdempotencyKey(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)

	client := &fakeAPIBaofuWithdrawClient{}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Idempotency-Key header is required")
	require.Empty(t, client.balanceReqs)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateMerchantBaofuWithdrawalReplaysSameIdempotencyKey(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	createdAt := time.Date(2026, 5, 19, 12, 35, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
			IdempotencyKey: pgtype.Text{
				String: "withdraw-replay-1",
				Valid:  true,
			},
		}).
		Return(db.BaofuWithdrawalOrder{
			ID:                     199,
			OwnerType:              db.BaofuAccountOwnerTypeMerchant,
			OwnerID:                merchant.ID,
			AccountBindingID:       77,
			OutRequestNo:           "MBW_EXISTING_REPLAY",
			Amount:                 1200,
			Status:                 db.BaofuWithdrawalStatusProcessing,
			IdempotencyKey:         pgtype.Text{String: "withdraw-replay-1", Valid: true},
			IdempotencyRequestHash: pgtype.Text{String: baofuWithdrawalIdempotencyHashForTest(db.BaofuAccountOwnerTypeMerchant, merchant.ID, 1200), Valid: true},
			CreatedAt:              createdAt,
			UpdatedAt:              createdAt,
		}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)

	client := &fakeAPIBaofuWithdrawClient{}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "withdraw-replay-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, client.balanceReqs)
	require.Empty(t, client.withdrawReqs)
	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(199), resp.Withdrawal.ID)
	require.Equal(t, "MBW_EXISTING_REPLAY", resp.Withdrawal.OutRequestNo)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, resp.Withdrawal.Status)
}

func TestCreateMerchantBaofuWithdrawalRejectsConflictingIdempotencyKey(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
			IdempotencyKey: pgtype.Text{
				String: "withdraw-conflict-1",
				Valid:  true,
			},
		}).
		Return(db.BaofuWithdrawalOrder{
			ID:                     200,
			OwnerType:              db.BaofuAccountOwnerTypeMerchant,
			OwnerID:                merchant.ID,
			Amount:                 1200,
			Status:                 db.BaofuWithdrawalStatusProcessing,
			IdempotencyKey:         pgtype.Text{String: "withdraw-conflict-1", Valid: true},
			IdempotencyRequestHash: pgtype.Text{String: baofuWithdrawalIdempotencyHashForTest(db.BaofuAccountOwnerTypeMerchant, merchant.ID, 1200), Valid: true},
		}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)

	client := &fakeAPIBaofuWithdrawClient{}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1300,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "withdraw-conflict-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Empty(t, client.balanceReqs)
	require.Empty(t, client.withdrawReqs)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeConflict, resp.Code)
	require.Equal(t, "当前状态已变化，请刷新页面确认后重试", resp.Message)
}

func TestCreateMerchantBaofuWithdrawalManagerCanCreate(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := activeBaofuWithdrawalBinding(db.BaofuAccountOwnerTypeMerchant, merchant.ID)
	createdAt := time.Date(2026, 5, 19, 12, 30, 0, 0, time.UTC)
	var capturedOutRequestNo string

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
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(binding, nil)
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "manager-withdraw-1")
	store.EXPECT().
		CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeMerchant, arg.WithdrawalOrder.OwnerType)
			require.Equal(t, merchant.ID, arg.WithdrawalOrder.OwnerID)
			require.Equal(t, int64(1200), arg.WithdrawalOrder.Amount)
			require.Equal(t, pgtype.Text{String: "manager-withdraw-1", Valid: true}, arg.WithdrawalOrder.IdempotencyKey)
			require.True(t, arg.WithdrawalOrder.IdempotencyRequestHash.Valid)
			require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
			capturedOutRequestNo = arg.WithdrawalOrder.OutRequestNo
			return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
				WithdrawalOrder: db.BaofuWithdrawalOrder{
					ID:                     191,
					OwnerType:              arg.WithdrawalOrder.OwnerType,
					OwnerID:                arg.WithdrawalOrder.OwnerID,
					AccountBindingID:       arg.WithdrawalOrder.AccountBindingID,
					OutRequestNo:           arg.WithdrawalOrder.OutRequestNo,
					Amount:                 arg.WithdrawalOrder.Amount,
					Status:                 arg.WithdrawalOrder.Status,
					IdempotencyKey:         arg.WithdrawalOrder.IdempotencyKey,
					IdempotencyRequestHash: arg.WithdrawalOrder.IdempotencyRequestHash,
					CreatedAt:              createdAt,
					UpdatedAt:              createdAt,
				},
				SubmittedCommand: db.ExternalPaymentCommand{ID: 1901},
			}, nil
		})

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "manager-withdraw-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, client.withdrawReqs)
	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(191), resp.Withdrawal.ID)
	require.Equal(t, capturedOutRequestNo, resp.Withdrawal.OutRequestNo)
	require.Equal(t, "unknown", resp.Withdrawal.SyncState)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Withdrawal.SyncMessage)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Message)
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
	request.Header.Set("Idempotency-Key", "invalid-amount-1")
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
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "balance-failure-1")
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
	request.Header.Set("Idempotency-Key", "balance-failure-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeBadGateway, resp.Code)
	require.Equal(t, "提现账户余额暂不可确认，请稍后刷新", resp.Message)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalRejectsMissingBindingBeforeProviderCall(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "missing-binding-1")
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).
		Times(0)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 5000},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "missing-binding-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeConflict, resp.Code)
	require.Equal(t, "结算账户未开通，暂不能提现", resp.Message)
	require.Empty(t, client.balanceReqs)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalRejectsMissingFeeMemberBeforeProviderCall(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := activeBaofuWithdrawalBindingWithoutFeeMember(db.BaofuAccountOwnerTypeMerchant, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "missing-fee-member-1")
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		}).
		Return(binding, nil)

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "missing-fee-member-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeConflict, resp.Code)
	require.Equal(t, "结算账户状态异常，请联系平台处理", resp.Message)
	require.Empty(t, client.balanceReqs)
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
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "insufficient-balance-1")
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
	request.Header.Set("Idempotency-Key", "insufficient-balance-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeConflict, resp.Code)
	require.Equal(t, "可提现金额不足", resp.Message)
	require.Empty(t, client.withdrawReqs)
}

func TestCreateBaofuWithdrawalPersistsSubmittedCommandAndReturnsAccepted(t *testing.T) {
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
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "withdraw-success-1")
	store.EXPECT().
		CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeMerchant, arg.WithdrawalOrder.OwnerType)
			require.Equal(t, merchant.ID, arg.WithdrawalOrder.OwnerID)
			require.Equal(t, binding.ID, arg.WithdrawalOrder.AccountBindingID)
			require.Equal(t, int64(1200), arg.WithdrawalOrder.Amount)
			require.Equal(t, db.BaofuWithdrawalStatusProcessing, arg.WithdrawalOrder.Status)
			require.True(t, strings.HasPrefix(arg.WithdrawalOrder.OutRequestNo, "MBW"), arg.WithdrawalOrder.OutRequestNo)
			require.Equal(t, pgtype.Text{String: "withdraw-success-1", Valid: true}, arg.WithdrawalOrder.IdempotencyKey)
			require.True(t, arg.WithdrawalOrder.IdempotencyRequestHash.Valid)
			require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
			capturedOutRequestNo = arg.WithdrawalOrder.OutRequestNo
			return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
				WithdrawalOrder: db.BaofuWithdrawalOrder{
					ID:                     91,
					OwnerType:              arg.WithdrawalOrder.OwnerType,
					OwnerID:                arg.WithdrawalOrder.OwnerID,
					AccountBindingID:       arg.WithdrawalOrder.AccountBindingID,
					OutRequestNo:           arg.WithdrawalOrder.OutRequestNo,
					Amount:                 arg.WithdrawalOrder.Amount,
					Status:                 arg.WithdrawalOrder.Status,
					IdempotencyKey:         arg.WithdrawalOrder.IdempotencyKey,
					IdempotencyRequestHash: arg.WithdrawalOrder.IdempotencyRequestHash,
					CreatedAt:              createdAt,
					UpdatedAt:              createdAt,
				},
				SubmittedCommand: db.ExternalPaymentCommand{ID: 901},
			}, nil
		})

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "withdraw-success-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, client.withdrawReqs)

	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(91), resp.Withdrawal.ID)
	require.Equal(t, capturedOutRequestNo, resp.Withdrawal.OutRequestNo)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, resp.Withdrawal.Status)
	require.Equal(t, "提现处理中", resp.Withdrawal.StatusText)
	require.Equal(t, "unknown", resp.Withdrawal.SyncState)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Withdrawal.SyncMessage)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Message)
	require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
	require.NotContains(t, recorder.Body.String(), "BF_WITHDRAW_001")
}

func TestCreateBaofuWithdrawalDefersProviderRejectedAcceptanceToWorker(t *testing.T) {
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
		Return(binding, nil)
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "withdraw-rejected-1")
	store.EXPECT().
		CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
			require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
			capturedOutRequestNo = arg.WithdrawalOrder.OutRequestNo
			return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
				WithdrawalOrder: db.BaofuWithdrawalOrder{
					ID:                     91,
					OwnerType:              arg.WithdrawalOrder.OwnerType,
					OwnerID:                arg.WithdrawalOrder.OwnerID,
					AccountBindingID:       arg.WithdrawalOrder.AccountBindingID,
					OutRequestNo:           arg.WithdrawalOrder.OutRequestNo,
					Amount:                 arg.WithdrawalOrder.Amount,
					Status:                 arg.WithdrawalOrder.Status,
					IdempotencyKey:         arg.WithdrawalOrder.IdempotencyKey,
					IdempotencyRequestHash: arg.WithdrawalOrder.IdempotencyRequestHash,
					CreatedAt:              createdAt,
					UpdatedAt:              createdAt,
				},
				SubmittedCommand: db.ExternalPaymentCommand{ID: 902},
			}, nil
		})

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{
			BaofuWithdrawNo: "BF_WITHDRAW_REJECTED",
			UpstreamState:   "2",
			Status:          db.BaofuWithdrawalStatusFailed,
			Remark:          "余额不足",
			Raw:             []byte(`{"state":"2","transRemark":"余额不足"}`),
		},
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "withdraw-rejected-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, client.withdrawReqs)
	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(91), resp.Withdrawal.ID)
	require.Equal(t, capturedOutRequestNo, resp.Withdrawal.OutRequestNo)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, resp.Withdrawal.Status)
	require.Equal(t, "unknown", resp.Withdrawal.SyncState)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Withdrawal.SyncMessage)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Message)
	require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
	require.NotContains(t, recorder.Body.String(), "BF_WITHDRAW_REJECTED")
	require.NotContains(t, recorder.Body.String(), "余额不足")
}

func TestCreateBaofuWithdrawalDefersProviderUnknownResultToWorker(t *testing.T) {
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
		Return(binding, nil)
	expectNoBaofuWithdrawalIdempotency(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "withdraw-unknown-1")
	store.EXPECT().
		CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
			require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
			capturedOutRequestNo = arg.WithdrawalOrder.OutRequestNo
			return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
				WithdrawalOrder: db.BaofuWithdrawalOrder{
					ID:                     91,
					OwnerType:              arg.WithdrawalOrder.OwnerType,
					OwnerID:                arg.WithdrawalOrder.OwnerID,
					AccountBindingID:       arg.WithdrawalOrder.AccountBindingID,
					OutRequestNo:           arg.WithdrawalOrder.OutRequestNo,
					Amount:                 arg.WithdrawalOrder.Amount,
					Status:                 arg.WithdrawalOrder.Status,
					IdempotencyKey:         arg.WithdrawalOrder.IdempotencyKey,
					IdempotencyRequestHash: arg.WithdrawalOrder.IdempotencyRequestHash,
					CreatedAt:              createdAt,
					UpdatedAt:              createdAt,
				},
				SubmittedCommand: db.ExternalPaymentCommand{ID: 903},
			}, nil
		})

	client := &fakeAPIBaofuWithdrawClient{
		balanceRes:  &baofucontracts.BalanceResult{ContractNo: binding.ContractNo.String, AvailableAmountFen: 5000},
		withdrawErr: errors.New("provider timeout"),
	}
	server := newTestServer(t, store)
	configureBaofuWithdrawServiceForAPITest(server, store, client)

	body := []byte(`{"amount":1200,"remark":"测试提现"}`)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/baofu-withdrawal/withdraw", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "withdraw-unknown-1")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, client.withdrawReqs)
	var resp baofuWithdrawalCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(91), resp.Withdrawal.ID)
	require.Equal(t, capturedOutRequestNo, resp.Withdrawal.OutRequestNo)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, resp.Withdrawal.Status)
	require.Equal(t, "unknown", resp.Withdrawal.SyncState)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Withdrawal.SyncMessage)
	require.Equal(t, baofuWithdrawalSubmittedSyncMessage, resp.Message)
	require.NotContains(t, recorder.Body.String(), binding.ContractNo.String)
	require.NotContains(t, recorder.Body.String(), "provider timeout")
}
