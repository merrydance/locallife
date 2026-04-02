package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantPackagingPolicyAPI(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: owner.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}
	policy := db.MerchantPackagingPolicy{
		MerchantID:           merchant.ID,
		ApplicableOrderTypes: []string{"takeout", "takeaway"},
		CandidateDishIds:     []int64{11},
	}

	testCases := []struct {
		name          string
		userID        int64
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OwnerOK",
			userID: owner.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(owner.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantPackagingPolicy(gomock.Any(), merchant.ID).
					Times(1).
					Return(policy, nil)

				store.EXPECT().
					GetDishesByIDsAll(gomock.Any(), []int64{11}).
					Times(1).
					Return([]db.GetDishesByIDsAllRow{{
						ID:          11,
						MerchantID:  merchant.ID,
						Name:        "收费餐盒",
						Price:       100,
						IsAvailable: true,
						IsOnline:    true,
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp packagingPolicyResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, merchant.ID, resp.MerchantID)
				require.Equal(t, []string{"takeout", "takeaway"}, resp.ApplicableOrderTypes)
				require.Equal(t, []int64{11}, resp.CandidateDishIDs)
				require.Len(t, resp.CandidateDishes, 1)
			},
		},
		{
			name:   "ManagerForbidden",
			userID: manager.ID,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleStaffMerchant(store, manager.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodGet, "/v1/merchants/me/packaging-policy", nil)
			require.NoError(t, err)
			addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, tc.userID, time.Minute)

			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateMerchantPackagingPolicyAPI(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          1,
		RegionID:    1,
		OwnerUserID: owner.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
		CreatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(owner.ID)).
		Times(1).
		Return(merchant, nil)

	store.EXPECT().
		GetDishesByIDsAll(gomock.Any(), []int64{11, 12}).
		Times(1).
		Return([]db.GetDishesByIDsAllRow{
			{ID: 11, MerchantID: merchant.ID, Name: "免费餐盒", Price: 0, IsAvailable: true, IsOnline: true},
			{ID: 12, MerchantID: merchant.ID, Name: "收费餐盒", Price: 100, IsAvailable: true, IsOnline: true},
		}, nil)

	store.EXPECT().
		UpsertMerchantPackagingPolicy(gomock.Any(), gomock.Eq(db.UpsertMerchantPackagingPolicyParams{
			MerchantID:           merchant.ID,
			ApplicableOrderTypes: []string{"takeout"},
			CandidateDishIds:     []int64{11, 12},
		})).
		Times(1).
		Return(db.MerchantPackagingPolicy{
			MerchantID:           merchant.ID,
			ApplicableOrderTypes: []string{"takeout"},
			CandidateDishIds:     []int64{11, 12},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(updatePackagingPolicyRequest{
		ApplicableOrderTypes: []string{"takeout"},
		CandidateDishIDs:     []int64{11, 12},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/packaging-policy", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp packagingPolicyResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, []string{"takeout"}, resp.ApplicableOrderTypes)
	require.Equal(t, []int64{11, 12}, resp.CandidateDishIDs)
	require.Len(t, resp.CandidateDishes, 2)
}
