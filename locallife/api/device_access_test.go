package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantDeviceAccessAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          util.RandomInt(1, 100),
		OwnerUserID: util.RandomInt(1000, 2000),
		Name:        "Test Merchant",
		Status:      "active",
		RegionID:    1,
	}

	testCases := []struct {
		name            string
		userID          int64
		buildStubs      func(store *mockdb.MockStore)
		expectedManage  bool
		expectedRole    string
		expectedMessage string
	}{
		{
			name:   "OwnerCanManage",
			userID: merchant.OwnerUserID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchant.OwnerUserID, merchant)
			},
			expectedManage: true,
			expectedRole:   "owner",
		},
		{
			name:   "ManagerCanManage",
			userID: user.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleStaffMerchant(store, user.ID, merchant)
				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("manager", nil)
			},
			expectedManage: true,
			expectedRole:   "manager",
		},
		{
			name:   "CashierDenied",
			userID: user.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleStaffMerchant(store, user.ID, merchant)
				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("cashier", nil)
			},
			expectedManage:  false,
			expectedRole:    "cashier",
			expectedMessage: "打印设备和后厨协同设置仅支持老板或店长管理",
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
			request, err := http.NewRequest(http.MethodGet, "/v1/merchant/devices/access", nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.userID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)

			var resp map[string]any
			requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
			require.EqualValues(t, merchant.ID, resp["merchant_id"])
			require.Equal(t, merchant.Name, resp["merchant_name"])
			require.Equal(t, tc.expectedRole, resp["staff_role"])
			require.Equal(t, tc.expectedManage, resp["can_manage"])

			allowedRoles, ok := resp["allowed_roles"].([]any)
			require.True(t, ok)
			require.Len(t, allowedRoles, len(merchantDeviceManageAllowedRoles))
			for idx, role := range merchantDeviceManageAllowedRoles {
				require.Equal(t, role, allowedRoles[idx])
			}

			if tc.expectedMessage != "" {
				require.Equal(t, tc.expectedMessage, resp["block_reason"])
			} else {
				_, exists := resp["block_reason"]
				require.False(t, exists)
			}
		})
	}
}

func TestGetMerchantDeviceAccessAPIForbiddenWhenNoMerchantAssociation(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveNoAccessibleMerchants(store, user.ID)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/devices/access", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var body ErrorResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &body)
	require.Equal(t, "you are not associated with any merchant", body.Error)
}
