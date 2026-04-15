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
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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
			name: "OK",
			body: map[string]interface{}{
				"membership_id":   membership.ID,
				"recharge_amount": 10000, // 100元
				"payment_method":  "wechat",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetMerchantMembership
				store.EXPECT().
					GetMerchantMembership(gomock.Any(), gomock.Eq(membership.ID)).
					Times(1).
					Return(membership, nil)

				// Mock GetMatchingRechargeRule
				store.EXPECT().
					GetMatchingRechargeRule(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rechargeRule, nil)

				// Mock GetUser for wechat openid
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)

				// Mock idempotency check: no existing pending payment order with prepay_id
				store.EXPECT().
					GetPendingPaymentOrderByUserAndBusinessType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)

				// Mock CreateEcommercePaymentTx
				store.EXPECT().
					CreateEcommercePaymentTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateEcommercePaymentTxResult{
						PaymentOrder: db.PaymentOrder{
							ID:           1,
							UserID:       user.ID,
							OutTradeNo:   "MBRC123",
							BusinessType: "membership_recharge",
							Amount:       100 * fenPerYuan,
						},
						CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 1},
						SubMchID:             "sub_mch_001",
					}, nil)

				// Mock UpdatePaymentOrderPrepayId
				store.EXPECT().
					UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, nil)

				// Mock UpdateCombinedPaymentOrderPrepay
				store.EXPECT().
					UpdateCombinedPaymentOrderPrepay(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CombinedPaymentOrder{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)

			// Create server with mock payment client
			server := newTestServerWithPayment(t, store, paymentClient)

			if tc.name == "OK" {
				// Mock ecommerce client with CreateCombineOrder for successful case
				ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
				ecommerceClient.EXPECT().
					CreateCombineOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechatcontracts.CombineOrderResponse{PrepayID: "prepay_id_test"}, &wechat.JSAPIPayParams{
						TimeStamp: "123456",
						NonceStr:  "random",
						Package:   "prepay_id=test",
						SignType:  "RSA",
						PaySign:   "sign",
					}, nil)
				server.SetEcommerceClientForTest(ecommerceClient)
			}
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
			merchantID: 999, // 不同的商户ID
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
