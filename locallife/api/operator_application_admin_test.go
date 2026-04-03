package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

func TestListPendingOperatorApplicationsAdminAPI(t *testing.T) {
	admin, _ := randomUser(t)
	now := time.Now()
	submittedAt := pgtype.Timestamptz{Time: now, Valid: true}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   RoleAdmin,
						Status: "active",
					}}, nil)

				store.EXPECT().
					ListPendingOperatorApplications(gomock.Any(), db.ListPendingOperatorApplicationsParams{
						Limit:  20,
						Offset: 0,
					}).
					Return([]db.ListPendingOperatorApplicationsRow{{
						ID:                     101,
						UserID:                 202,
						RegionID:               303,
						Name:                   pgtype.Text{String: "测试运营商", Valid: true},
						ContactName:            pgtype.Text{String: "联系人甲", Valid: true},
						ContactPhone:           pgtype.Text{String: "13800138000", Valid: true},
						LegalPersonName:        pgtype.Text{String: "法人甲", Valid: true},
						RequestedContractYears: 2,
						Status:                 "submitted",
						CreatedAt:              now,
						SubmittedAt:            submittedAt,
						ApplicantName:          pgtype.Text{String: "提交人甲", Valid: true},
						ApplicantPhone:         pgtype.Text{String: "13900139000", Valid: true},
						RegionName:             "测试区域",
						RegionCode:             "CN-TEST",
					}}, nil)

				store.EXPECT().
					CountPendingOperatorApplications(gomock.Any()).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listPendingOperatorApplicationsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Applications, 1)
				require.Equal(t, "提交人甲", resp.Applications[0].ApplicantName)
				require.Equal(t, "13900139000", resp.Applications[0].ApplicantPhone)
				require.Equal(t, "联系人甲", resp.Applications[0].ContactName)
				require.False(t, resp.HasMore)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/admin/operators/applications?page=1&limit=20", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
