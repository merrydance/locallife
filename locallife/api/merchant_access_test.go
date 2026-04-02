package api

import (
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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

func expectResolveOwnedMerchants(store *mockdb.MockStore, userID int64, merchants []db.Merchant) {
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return(merchants, nil)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(userID)).
		AnyTimes().
		Return([]db.Merchant{}, nil)
}
