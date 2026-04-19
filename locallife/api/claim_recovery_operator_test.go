package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetOperatorClaimRecoveryAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	managedRegion := randomRegion()
	operator.RegionID = managedRegion.ID
	unmanagedRegion := randomRegion()
	if unmanagedRegion.ID == managedRegion.ID {
		unmanagedRegion.ID = managedRegion.ID + 1
	}

	claim := db.GetClaimForAppealRow{
		ID:         1,
		MerchantID: 200,
		RegionID:   unmanagedRegion.ID,
		CreatedAt:  time.Now(),
	}
	recovery := db.ClaimRecovery{
		ID:               11,
		ClaimID:          claim.ID,
		OrderID:          88,
		ResponsibleParty: "merchant",
		RecoveryTarget:   pgtype.Text{String: "merchant", Valid: true},
		RecoveryAmount:   500,
		Status:           "pending",
		DueAt:            time.Now().Add(24 * time.Hour),
		UpdatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "ManagedAcrossMultipleRegions",
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, managedRegion.ID, unmanagedRegion.ID)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(recovery, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response claimRecoveryResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, recovery.ID, response.ID)
				require.Equal(t, claim.ID, response.ClaimID)
			},
		},
		{
			name: "ForbiddenWhenClaimRegionUnmanaged",
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, managedRegion.ID)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/operator/claims/1/recovery", nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
