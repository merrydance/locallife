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

func TestGetPlatformBaofuSettlementStatusAPI_IncludesSanitizedReadiness(t *testing.T) {
	admin, _ := randomUser(t)
	binding := db.BaofuAccountBinding{
		ID:           5301,
		OwnerType:    db.BaofuAccountOwnerTypePlatform,
		OwnerID:      platformBaofuAccountOwnerID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "PF5301ABC", Valid: true},
		SharingMerID: pgtype.Text{String: "PS5301XYZ", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypePlatform,
			OwnerID:   platformBaofuAccountOwnerID,
		}).
		Times(1).
		Return(binding, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/settlement-account/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "PF5301ABC")
	require.NotContains(t, recorder.Body.String(), "PS5301XYZ")
	require.NotContains(t, recorder.Body.String(), "contractNo")
	require.NotContains(t, recorder.Body.String(), "sharingMerId")

	var resp platformBaofuSettlementStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.SettlementAccount)
	require.Equal(t, "ready", resp.SettlementAccount.State)
	require.Equal(t, "结算账户可用", resp.SettlementAccount.Label)
	require.True(t, resp.SettlementAccount.PaymentReady)
	require.Equal(t, "PF5****ABC", resp.MaskedContractNo)
	require.Equal(t, "PS5****XYZ", resp.MaskedSharingMerID)
}
