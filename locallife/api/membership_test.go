package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBuildRechargeRuleStatusResponse(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		rule      db.RechargeRule
		wantCode  string
		wantLabel string
		wantTheme string
	}{
		{
			name:      "Inactive",
			rule:      db.RechargeRule{IsActive: false, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
			wantCode:  "inactive",
			wantLabel: "已停用",
			wantTheme: "default",
		},
		{
			name:      "Expired",
			rule:      db.RechargeRule{IsActive: true, ValidFrom: now.Add(-2 * time.Hour), ValidUntil: now.Add(-time.Hour)},
			wantCode:  "expired",
			wantLabel: "已过期",
			wantTheme: "danger",
		},
		{
			name:      "Scheduled",
			rule:      db.RechargeRule{IsActive: true, ValidFrom: now.Add(time.Hour), ValidUntil: now.Add(2 * time.Hour)},
			wantCode:  "scheduled",
			wantLabel: "未开始",
			wantTheme: "warning",
		},
		{
			name:      "Active",
			rule:      db.RechargeRule{IsActive: true, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
			wantCode:  "active",
			wantLabel: "生效中",
			wantTheme: "success",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusCode, statusLabel, statusTheme := buildRechargeRuleStatusResponse(tc.rule, now)
			require.Equal(t, tc.wantCode, statusCode)
			require.Equal(t, tc.wantLabel, statusLabel)
			require.Equal(t, tc.wantTheme, statusTheme)
		})
	}
}

func TestGetMembershipSettingsAPIOwnerSuccess(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	settings := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"takeaway"},
		AllowWithVoucher:    false,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 70,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), owner.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(settings, nil)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchants/me/membership-settings", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response membershipSettingsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, merchant.ID, response.MerchantID)
	require.Equal(t, []string{"dine_in", "takeaway"}, response.BalanceUsableScenes)
	require.Equal(t, []string{"takeaway"}, response.BonusUsableScenes)
	require.False(t, response.AllowWithVoucher)
	require.True(t, response.AllowWithDiscount)
	require.Equal(t, int32(70), response.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsAPIFullOwnerSuccess(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	existing := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), owner.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(existing, nil)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertMerchantMembershipSettingsParams) (db.MerchantMembershipSetting, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, []string{"dine_in", "takeaway"}, arg.BalanceUsableScenes)
			require.Equal(t, []string{"takeaway"}, arg.BonusUsableScenes)
			require.False(t, arg.AllowWithVoucher)
			require.False(t, arg.AllowWithDiscount)
			require.Equal(t, int32(65), arg.MaxDeductionPercent)
			return db.MerchantMembershipSetting{
				MerchantID:          arg.MerchantID,
				BalanceUsableScenes: arg.BalanceUsableScenes,
				BonusUsableScenes:   arg.BonusUsableScenes,
				AllowWithVoucher:    arg.AllowWithVoucher,
				AllowWithDiscount:   arg.AllowWithDiscount,
				MaxDeductionPercent: arg.MaxDeductionPercent,
			}, nil
		})

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"balance_usable_scenes": []string{"dine_in", "takeaway"},
		"bonus_usable_scenes":   []string{"takeaway"},
		"allow_with_voucher":    false,
		"allow_with_discount":   false,
		"max_deduction_percent": 65,
	})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/membership-settings", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response membershipSettingsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, merchant.ID, response.MerchantID)
	require.Equal(t, []string{"dine_in", "takeaway"}, response.BalanceUsableScenes)
	require.Equal(t, []string{"takeaway"}, response.BonusUsableScenes)
	require.False(t, response.AllowWithVoucher)
	require.False(t, response.AllowWithDiscount)
	require.Equal(t, int32(65), response.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsAPIPartialUpdatePreservesExistingFields(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	existing := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"takeaway"},
		BonusUsableScenes:   []string{},
		AllowWithVoucher:    false,
		AllowWithDiscount:   false,
		MaxDeductionPercent: 60,
	}
	nextPercent := int32(80)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), owner.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(existing, nil)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertMerchantMembershipSettingsParams) (db.MerchantMembershipSetting, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, []string{"takeaway"}, arg.BalanceUsableScenes)
			require.Empty(t, arg.BonusUsableScenes)
			require.False(t, arg.AllowWithVoucher)
			require.False(t, arg.AllowWithDiscount)
			require.Equal(t, nextPercent, arg.MaxDeductionPercent)
			return db.MerchantMembershipSetting{
				MerchantID:          arg.MerchantID,
				BalanceUsableScenes: arg.BalanceUsableScenes,
				BonusUsableScenes:   arg.BonusUsableScenes,
				AllowWithVoucher:    arg.AllowWithVoucher,
				AllowWithDiscount:   arg.AllowWithDiscount,
				MaxDeductionPercent: arg.MaxDeductionPercent,
			}, nil
		})

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{"max_deduction_percent": nextPercent})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/membership-settings", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response membershipSettingsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, []string{"takeaway"}, response.BalanceUsableScenes)
	require.Empty(t, response.BonusUsableScenes)
	require.False(t, response.AllowWithVoucher)
	require.False(t, response.AllowWithDiscount)
	require.Equal(t, nextPercent, response.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsAPIExplicitEmptyScenesPreserved(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	existing := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), owner.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(existing, nil)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertMerchantMembershipSettingsParams) (db.MerchantMembershipSetting, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.NotNil(t, arg.BalanceUsableScenes)
			require.Empty(t, arg.BalanceUsableScenes)
			require.NotNil(t, arg.BonusUsableScenes)
			require.Empty(t, arg.BonusUsableScenes)
			require.True(t, arg.AllowWithVoucher)
			require.True(t, arg.AllowWithDiscount)
			require.Equal(t, int32(100), arg.MaxDeductionPercent)
			return db.MerchantMembershipSetting{
				MerchantID:          arg.MerchantID,
				BalanceUsableScenes: arg.BalanceUsableScenes,
				BonusUsableScenes:   arg.BonusUsableScenes,
				AllowWithVoucher:    arg.AllowWithVoucher,
				AllowWithDiscount:   arg.AllowWithDiscount,
				MaxDeductionPercent: arg.MaxDeductionPercent,
			}, nil
		})

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"balance_usable_scenes": []string{},
		"bonus_usable_scenes":   []string{},
	})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/membership-settings", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response membershipSettingsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.BalanceUsableScenes)
	require.Empty(t, response.BalanceUsableScenes)
	require.NotNil(t, response.BonusUsableScenes)
	require.Empty(t, response.BonusUsableScenes)
	require.True(t, response.AllowWithVoucher)
	require.True(t, response.AllowWithDiscount)
	require.Equal(t, int32(100), response.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsAPIRejectsInvalidScene(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"balance_usable_scenes": []string{"takeout"},
	})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/membership-settings", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUpdateMembershipSettingsAPIRejectsInvalidMaxDeductionPercent(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	merchant.Status = "active"
	merchant.RegionID = 1

	testCases := []struct {
		name    string
		percent int32
	}{
		{name: "Zero", percent: 0},
		{name: "AboveMaximum", percent: 101},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
			store.EXPECT().
				GetMerchantByOwner(gomock.Any(), gomock.Any()).
				Times(0)
			store.EXPECT().
				GetMerchantMembershipSettings(gomock.Any(), gomock.Any()).
				Times(0)
			store.EXPECT().
				UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
				Times(0)

			server := newTestServer(t, store)
			body, err := json.Marshal(map[string]any{
				"max_deduction_percent": tc.percent,
			})
			require.NoError(t, err)
			request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/membership-settings", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestJoinMembershipAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"merchant_id": merchant.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetMerchant to verify merchant exists
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				// Mock JoinMembershipTx
				arg := db.JoinMembershipTxParams{
					MerchantID: merchant.ID,
					UserID:     user.ID,
				}
				membership := db.MerchantMembership{
					ID:             1,
					MerchantID:     merchant.ID,
					UserID:         user.ID,
					Balance:        0,
					TotalRecharged: 0,
					TotalConsumed:  0,
					CreatedAt:      time.Now(),
				}
				store.EXPECT().
					JoinMembershipTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.JoinMembershipTxResult{Membership: membership}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"merchant_id": merchant.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					JoinMembershipTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: map[string]interface{}{
				"merchant_id": merchant.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
				store.EXPECT().
					JoinMembershipTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidMerchantID",
			body: map[string]interface{}{
				"merchant_id": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					JoinMembershipTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/memberships"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRechargeMembershipAPI(t *testing.T) {
	user, _ := randomUser(t)
	membership := randomMembership(user.ID)
	rechargeRule := randomRechargeRule(membership.MerchantID)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "PaymentPaused",
			body: map[string]interface{}{
				"membership_id":   membership.ID,
				"recharge_amount": 10000, // 100元
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(membership.ID)).
					Times(1).
					Return(membership, nil)

				store.EXPECT().
					GetMatchingRechargeRule(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rechargeRule, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				require.Contains(t, recorder.Body.String(), "会员线上充值已停用，请联系商户线下充值后入账")
			},
		},
		{
			name: "InvalidAmount_Negative",
			body: map[string]interface{}{
				"membership_id":   membership.ID,
				"recharge_amount": -1000, // 负数
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidAmount_Zero",
			body: map[string]interface{}{
				"membership_id":   membership.ID,
				"recharge_amount": 0,
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "MembershipNotFound",
			body: map[string]interface{}{
				"membership_id":   999,
				"recharge_amount": 10000,
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.MerchantMembership{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "UnauthorizedAccess",
			body: map[string]interface{}{
				"membership_id":   membership.ID,
				"recharge_amount": 10000,
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// Different user trying to recharge
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+999, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetMerchantMembership returns membership belonging to different user
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(membership.ID)).
					Times(1).
					Return(membership, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			// Create mock payment client
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

			// Create server with mock payment client
			server := newTestServerWithPayment(t, store, paymentClient)

			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/memberships/recharge"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRecordMemberRechargeAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	memberUser, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	membership := randomMembership(memberUser.ID)
	membership.MerchantID = merchant.ID
	rule := randomRechargeRule(merchant.ID)
	updatedMembership := membership
	updatedMembership.Balance = membership.Balance + 11000
	updatedMembership.TotalRecharged = membership.TotalRecharged + 11000
	transaction := db.MembershipTransaction{ID: 91, MembershipID: membership.ID}

	testCases := []struct {
		name           string
		merchantID     int64
		userID         int64
		body           map[string]interface{}
		idempotencyKey string
		setupAuth      func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs     func(store *mockdb.MockStore)
		checkResponse  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "MissingIdempotencyKey",
			merchantID:     merchant.ID,
			userID:         memberUser.ID,
			idempotencyKey: "",
			body: map[string]interface{}{
				"recharge_amount": 10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "Idempotency-Key header is required")
			},
		},
		{
			name:           "OK",
			merchantID:     merchant.ID,
			userID:         memberUser.ID,
			idempotencyKey: "recharge-api-1",
			body: map[string]interface{}{
				"recharge_amount": 10000,
				"notes":           "线下微信收款",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchant.ID, UserID: memberUser.ID}).
					Return(membership, nil)
				store.EXPECT().
					GetUser(gomock.Any(), memberUser.ID).
					Return(memberUser, nil)
				store.EXPECT().
					GetMembershipRechargeTransactionByIdempotencyKey(gomock.Any(), gomock.Any()).
					Return(db.MembershipTransaction{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: merchant.ID, RechargeAmount: 10000}).
					Return(rule, nil)
				store.EXPECT().
					RechargeTx(gomock.Any(), db.RechargeTxParams{
						MembershipID:   membership.ID,
						RechargeAmount: 10000,
						BonusAmount:    1000,
						RechargeRuleID: &rule.ID,
						Notes:          "线下微信收款",
						IdempotencyKey: "recharge-api-1",
					}).
					Return(db.RechargeTxResult{Membership: updatedMembership, Transaction: transaction}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var rsp merchantMemberRechargeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &rsp)
				require.Equal(t, memberUser.ID, rsp.UserID)
				require.Equal(t, updatedMembership.Balance, rsp.Balance)
				require.Equal(t, int64(10000), rsp.RechargeAmount)
				require.Equal(t, int64(1000), rsp.BonusAmount)
				require.Equal(t, int64(11000), rsp.TotalCredited)
				require.NotNil(t, rsp.RechargeRuleID)
				require.Equal(t, rule.ID, *rsp.RechargeRuleID)
			},
		},
		{
			name:           "Forbidden_NotMerchant",
			merchantID:     merchant.ID,
			userID:         memberUser.ID,
			idempotencyKey: "recharge-api-1",
			body: map[string]interface{}{
				"recharge_amount": 10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, otherUser.ID)
				store.EXPECT().GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:           "MembershipNotFound",
			merchantID:     merchant.ID,
			userID:         memberUser.ID,
			idempotencyKey: "recharge-api-1",
			body: map[string]interface{}{
				"recharge_amount": 10000,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchant.ID, UserID: memberUser.ID}).
					Return(db.MerchantMembership{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/merchants/%d/members/%d/recharge", tc.merchantID, tc.userID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			if tc.idempotencyKey != "" {
				request.Header.Set("Idempotency-Key", tc.idempotencyKey)
			}

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestAdjustMemberBalanceAPIRequiresIdempotencyKey(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	memberUser, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
	store.EXPECT().GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(map[string]interface{}{
		"amount": int64(1000),
		"notes":  "人工补偿",
	})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/merchants/%d/members/%d/balance", merchant.ID, memberUser.ID)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Idempotency-Key header is required")
}

func TestAdjustMemberBalanceAPIForwardsIdempotencyKey(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	memberUser, _ := randomUser(t)
	membership := randomMembership(memberUser.ID)
	membership.MerchantID = merchant.ID
	updatedMembership := membership
	updatedMembership.Balance = 1000
	updatedMembership.PrincipalBalance = 1000

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchant.ID, UserID: memberUser.ID}).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		AdjustMemberBalanceTx(gomock.Any(), db.AdjustMemberBalanceTxParams{
			MembershipID:   membership.ID,
			Amount:         1000,
			Notes:          "人工补偿",
			IdempotencyKey: "adjust-api-1",
		}).
		Times(1).
		Return(db.AdjustMemberBalanceTxResult{Membership: updatedMembership}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), memberUser.ID).
		Times(1).
		Return(memberUser, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(map[string]interface{}{
		"amount": int64(1000),
		"notes":  "人工补偿",
	})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/merchants/%d/members/%d/balance", merchant.ID, memberUser.ID)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", " adjust-api-1 ")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var rsp merchantMemberResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &rsp)
	require.Equal(t, memberUser.ID, rsp.UserID)
	require.Equal(t, int64(1000), rsp.Balance)
}

func TestGetMembershipAPI(t *testing.T) {
	user, _ := randomUser(t)
	membership := randomMembership(user.ID)

	testCases := []struct {
		name          string
		membershipID  int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "OK",
			membershipID: membership.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(membership.ID)).
					Times(1).
					Return(membership, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:         "UnauthorizedAccess",
			membershipID: membership.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// Different user trying to access
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+999, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(membership.ID)).
					Times(1).
					Return(membership, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:         "NotFound",
			membershipID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.MerchantMembership{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/memberships/%d", tc.membershipID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper functions
func randomMembership(userID int64) db.MerchantMembership {
	return db.MerchantMembership{
		ID:             1,
		MerchantID:     1,
		UserID:         userID,
		Balance:        0,
		TotalRecharged: 0,
		TotalConsumed:  0,
		CreatedAt:      time.Now(),
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func randomRechargeRule(merchantID int64) db.RechargeRule {
	return db.RechargeRule{
		ID:             1,
		MerchantID:     merchantID,
		RechargeAmount: 10000, // 充100
		BonusAmount:    1000,  // 送10
		IsActive:       true,
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:      time.Now(),
	}
}

func TestCreateRechargeRuleAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherUser, _ := randomUser(t)
	wrongMerchantID := merchant.ID + 1

	testCases := []struct {
		name          string
		merchantID    int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			body: map[string]interface{}{
				"recharge_amount": 10000,
				"bonus_amount":    1000,
				"valid_from":      time.Now().Format(time.RFC3339),
				"valid_until":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					CreateRechargeRule(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomRechargeRule(merchant.ID), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name:       "Forbidden_NotMerchant",
			merchantID: merchant.ID,
			body: map[string]interface{}{
				"recharge_amount": 10000,
				"bonus_amount":    1000,
				"valid_from":      time.Now().Format(time.RFC3339),
				"valid_until":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, otherUser.ID)

				store.EXPECT().
					CreateRechargeRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "Forbidden_WrongMerchant",
			merchantID: wrongMerchantID,
			body: map[string]interface{}{
				"recharge_amount": 10000,
				"bonus_amount":    1000,
				"valid_from":      time.Now().Format(time.RFC3339),
				"valid_until":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					CreateRechargeRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "BadRequest_InvalidTimeRange",
			merchantID: merchant.ID,
			body: map[string]interface{}{
				"recharge_amount": 10000,
				"bonus_amount":    1000,
				"valid_from":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
				"valid_until":     time.Now().Format(time.RFC3339), // 结束时间早于开始时间
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					CreateRechargeRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/merchants/%d/recharge-rules", tc.merchantID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteRechargeRuleAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	rule := randomRechargeRule(merchant.ID)
	otherUser, _ := randomUser(t)

	testCases := []struct {
		name          string
		merchantID    int64
		ruleID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			ruleID:     rule.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetRechargeRule(gomock.Any(), gomock.Eq(rule.ID)).
					Times(1).
					Return(rule, nil)

				store.EXPECT().
					DeleteRechargeRule(gomock.Any(), gomock.Eq(rule.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "RuleNotFound",
			merchantID: merchant.ID,
			ruleID:     999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetRechargeRule(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.RechargeRule{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "Forbidden_NotOwner",
			merchantID: merchant.ID,
			ruleID:     rule.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, otherUser.ID)

				store.EXPECT().
					GetRechargeRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchants/%d/recharge-rules/%d", tc.merchantID, tc.ruleID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMembershipTransactionsAPI(t *testing.T) {
	user, _ := randomUser(t)
	membership := randomMembership(user.ID)
	otherUser, _ := randomUser(t)

	// 注意：由于 GIN 的 ShouldBindUri + ShouldBindQuery 绑定到同一结构体时可能存在问题
	// OK 场景需要在集成测试中覆盖。这里仅测试边界条件

	testCases := []struct {
		name          string
		membershipID  int64
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "NoAuthorization",
			membershipID: membership.ID,
			query:        "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Should not reach store calls
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:         "MissingPageID",
			membershipID: membership.ID,
			query:        "?page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Should fail at query binding
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/memberships/%d/transactions%s", tc.membershipID, tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}

	_ = otherUser // 保留用于可能的后续测试扩展
}
