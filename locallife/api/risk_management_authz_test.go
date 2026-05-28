package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRiskManagementAdminOnlyRoutesRejectNonAdmin(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "SuspendMerchant",
			method: http.MethodPatch,
			path:   "/v1/food-safety/merchants/123/suspend",
			body:   `{"merchant_id":123,"reason":"security regression test","duration_hours":1,"admin_id":12345}`,
		},
		{
			name:   "TriggerFraudDetection",
			method: http.MethodPost,
			path:   "/v1/fraud/detect",
			body:   `{"claim_id":1}`,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListUserRoles(gomock.Any(), user.ID).
				Times(1).
				Return([]db.UserRole{{UserID: user.ID, Role: RoleCustomer, Status: "active"}}, nil)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}

func TestRiskManagementAdminOnlyRoutesAllowAdminThroughMiddleware(t *testing.T) {
	admin, _ := randomUser(t)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "SuspendMerchant",
			method: http.MethodPatch,
			path:   "/v1/food-safety/merchants/123/suspend",
			body:   `{"merchant_id":456,"reason":"security regression test","duration_hours":1,"admin_id":12345}`,
		},
		{
			name:   "TriggerFraudDetection",
			method: http.MethodPost,
			path:   "/v1/fraud/detect",
			body:   `{}`,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListUserRoles(gomock.Any(), admin.ID).
				Times(1).
				Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
