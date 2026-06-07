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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	expectedMerchantStaffPermissionDenied        = &APIError{Code: 40364, Message: "当前员工角色无权执行该商户操作，请联系店主或管理员处理"}
	expectedMerchantRecoveryOwnerRequired        = &APIError{Code: 40365, Message: "仅店主或授权管理员可查看或处理索赔追偿"}
	expectedMerchantRecoveryPaymentOwnerRequired = &APIError{Code: 40366, Message: "仅店主可发起追偿支付"}
	expectedMerchantRiskAccessDenied             = &APIError{Code: 40367, Message: "仅店主或管理员可查看与本店交易相关顾客的风险提示"}
	expectedMerchantMembershipSettingsOwnerOnly  = &APIError{Code: 40371, Message: "仅店主可修改会员设置"}
)

func requireAPIErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expected *APIError) {
	t.Helper()

	var body ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	if body.Error == "" {
		requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &body)
	}
	require.Equal(t, expected.Code, body.Code)
	require.Equal(t, expected.Message, body.Error)
}

func staffAuthzMerchant(userID int64) db.Merchant {
	merchant := randomMerchant(userID + 1000)
	merchant.Status = "active"
	merchant.RegionID = 1
	return merchant
}

func expectStaffRole(store *mockdb.MockStore, userID int64, merchant db.Merchant, role string) {
	expectResolveSingleStaffMerchant(store, userID, merchant)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     userID,
		}).
		Times(1).
		Return(role, nil)
}

func TestMerchantClaimRecoveryRoutesDenyLowPrivilegeStaff(t *testing.T) {
	user, _ := randomUser(t)
	merchant := staffAuthzMerchant(user.ID)

	tests := []struct {
		name        string
		method      string
		path        string
		body        any
		expectedErr *APIError
		buildStubs  func(store *mockdb.MockStore)
	}{
		{
			name:        "ListClaimsCashierDenied",
			method:      http.MethodGet,
			path:        "/v1/merchant/claims?page_id=1&page_size=10",
			expectedErr: expectedMerchantRecoveryOwnerRequired,
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "cashier")
				store.EXPECT().ListMerchantClaimsForMerchant(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:        "GetRecoveryChefDenied",
			method:      http.MethodGet,
			path:        "/v1/merchant/recoveries/77",
			expectedErr: expectedMerchantRecoveryOwnerRequired,
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "chef")
				store.EXPECT().GetClaimRecoveryContextByID(gomock.Any(), int64(77)).Times(0)
			},
		},
		{
			name:        "PayRecoveryManagerDenied",
			method:      http.MethodPost,
			path:        "/v1/merchant/recoveries/77/pay",
			expectedErr: expectedMerchantRecoveryPaymentOwnerRequired,
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "manager")
				store.EXPECT().GetClaimRecoveryContextByID(gomock.Any(), int64(77)).Times(0)
			},
		},
		{
			name:        "CreateRecoveryDisputeCashierDenied",
			method:      http.MethodPost,
			path:        "/v1/merchant/recovery-disputes",
			body:        gin.H{"claim_id": 88, "reason": "门店已核验餐品完整"},
			expectedErr: expectedMerchantRecoveryOwnerRequired,
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "cashier")
				store.EXPECT().GetClaimRecoveryContextByClaimID(gomock.Any(), int64(88)).Times(0)
			},
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			auditWriter := &auditSpyWriter{}
			server.auditWriter = auditWriter

			var body *bytes.Reader
			if tc.body == nil {
				body = bytes.NewReader(nil)
			} else {
				data, err := json.Marshal(tc.body)
				require.NoError(t, err)
				body = bytes.NewReader(data)
			}
			request, err := http.NewRequest(tc.method, tc.path, body)
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusForbidden, recorder.Code)
			requireAPIErrorCode(t, recorder, tc.expectedErr)
			require.NotEmpty(t, auditWriter.Entries())
		})
	}
}

func TestMerchantProfileWriteRoutesDenyLowPrivilegeStaff(t *testing.T) {
	user, _ := randomUser(t)
	merchant := staffAuthzMerchant(user.ID)

	tests := []struct {
		name        string
		method      string
		path        string
		body        any
		expectedErr *APIError
		buildStubs  func(store *mockdb.MockStore)
	}{
		{
			name:   "PatchMerchantDenied",
			method: http.MethodPatch,
			path:   "/v1/merchants/me",
			body:   gin.H{"name": "Updated Name", "version": merchant.Version},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "cashier")
				store.EXPECT().UpdateMerchant(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:   "PatchStatusDenied",
			method: http.MethodPatch,
			path:   "/v1/merchants/me/status",
			body:   gin.H{"is_open": false},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "chef")
				store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:   "PatchShopImagesDenied",
			method: http.MethodPatch,
			path:   "/v1/merchants/me/shop-images",
			body: gin.H{
				"storefront_images":  []string{"uploads/merchant/storefront.jpg"},
				"environment_images": []string{"uploads/merchant/env.jpg"},
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "cashier")
				store.EXPECT().UpdateMerchantShopImages(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:   "PutBusinessHoursDenied",
			method: http.MethodPut,
			path:   "/v1/merchants/me/business-hours",
			body: gin.H{"hours": []gin.H{{
				"day_of_week": 1,
				"open_time":   "09:00",
				"close_time":  "21:00",
				"is_closed":   false,
			}}},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "cashier")
				store.EXPECT().SetBusinessHoursTx(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:   "PutTagsDenied",
			method: http.MethodPut,
			path:   "/v1/merchants/me/tags",
			body:   gin.H{"tag_ids": []int64{1, 2}},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "chef")
				store.EXPECT().GetTag(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:   "PutMembershipSettingsDenied",
			method: http.MethodPut,
			path:   "/v1/merchants/me/membership-settings",
			body: gin.H{
				"allow_with_voucher": true,
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectStaffRole(store, user.ID, merchant, "manager")
				store.EXPECT().UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).Times(0)
			},
			expectedErr: expectedMerchantMembershipSettingsOwnerOnly,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			auditWriter := &auditSpyWriter{}
			server.auditWriter = auditWriter

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)
			request, err := http.NewRequest(tc.method, tc.path, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusForbidden, recorder.Code)
			if tc.expectedErr != nil {
				requireAPIErrorCode(t, recorder, tc.expectedErr)
			} else {
				requireAPIErrorCode(t, recorder, expectedMerchantStaffPermissionDenied)
			}
			require.NotEmpty(t, auditWriter.Entries())
		})
	}
}

func TestMerchantRiskRequiresManagerAndExistingMerchantRelationship(t *testing.T) {
	user, _ := randomUser(t)
	merchant := staffAuthzMerchant(user.ID)
	targetUserID := int64(90210)

	t.Run("CashierDeniedBeforeRiskLookup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		expectStaffRole(store, user.ID, merchant, "cashier")
		store.EXPECT().HasUserOrderedFromMerchant(gomock.Any(), gomock.Any()).Times(0)
		store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Times(0)

		server := newTestServer(t, store)
		auditWriter := &auditSpyWriter{}
		server.auditWriter = auditWriter

		request, err := http.NewRequest(http.MethodGet, "/v1/merchant/risk/users/90210", nil)
		require.NoError(t, err)
		addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

		recorder := httptest.NewRecorder()
		server.router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusForbidden, recorder.Code)
		requireAPIErrorCode(t, recorder, expectedMerchantRiskAccessDenied)
		require.NotEmpty(t, auditWriter.Entries())
	})

	t.Run("ManagerDeniedWhenTargetNeverOrderedFromMerchant", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		expectStaffRole(store, user.ID, merchant, "manager")
		store.EXPECT().
			HasUserOrderedFromMerchant(gomock.Any(), db.HasUserOrderedFromMerchantParams{
				UserID:     targetUserID,
				MerchantID: merchant.ID,
			}).
			Times(1).
			Return(false, nil)
		store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Times(0)

		server := newTestServer(t, store)
		auditWriter := &auditSpyWriter{}
		server.auditWriter = auditWriter

		request, err := http.NewRequest(http.MethodGet, "/v1/merchant/risk/users/90210", nil)
		require.NoError(t, err)
		addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

		recorder := httptest.NewRecorder()
		server.router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusForbidden, recorder.Code)
		requireAPIErrorCode(t, recorder, expectedMerchantRiskAccessDenied)
		require.NotEmpty(t, auditWriter.Entries())
	})

	t.Run("ManagerCanReadRelatedUserRisk", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		expectStaffRole(store, user.ID, merchant, "manager")
		store.EXPECT().
			HasUserOrderedFromMerchant(gomock.Any(), db.HasUserOrderedFromMerchantParams{
				UserID:     targetUserID,
				MerchantID: merchant.ID,
			}).
			Times(1).
			Return(true, nil)
		store.EXPECT().
			GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{
				EntityType: "user",
				EntityID:   targetUserID,
			}).
			Times(1).
			Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)

		server := newTestServer(t, store)
		request, err := http.NewRequest(http.MethodGet, "/v1/merchant/risk/users/90210", nil)
		require.NoError(t, err)
		addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

		recorder := httptest.NewRecorder()
		server.router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		var resp merchantUserRiskResponse
		requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
		require.Equal(t, targetUserID, resp.UserID)
		require.False(t, resp.HasBlock)
	})
}
