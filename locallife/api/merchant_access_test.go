package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectResolveSingleOwnedMerchant(store *mockdb.MockStore, userID int64, merchant db.Merchant) {
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{merchant}, nil)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{}, nil)
}

func expectResolveSingleStaffMerchant(store *mockdb.MockStore, userID int64, merchant db.Merchant) {
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{}, nil)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{merchant}, nil)
}

func expectResolveNoAccessibleMerchants(store *mockdb.MockStore, userID int64) {
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{}, nil)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{}, nil)
}

func expectNoMerchantAccessResolution(store *mockdb.MockStore) {
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Any()).
		Times(0)
}

func TestListAccessibleMerchantsExcludesPendingStaffRole(t *testing.T) {
	userID := int64(101)
	merchant := db.Merchant{ID: 202, OwnerUserID: 303, Name: "Pending Merchant", Status: "active", RegionID: 1}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Eq(userID)).
		Times(1).
		Return([]db.Merchant{}, nil)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(userID)).
		Times(1).
		Return([]db.Merchant{merchant}, nil)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{MerchantID: merchant.ID, UserID: userID})).
		Times(1).
		Return(db.MerchantStaffRolePending, nil)

	server := newTestServer(t, store)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodGet, "/v1/merchant/devices", nil)

	merchants, err := server.listAccessibleMerchants(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, merchants)
}
