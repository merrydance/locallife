package logic

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMembershipForUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	_, err := GetMembershipForUser(context.Background(), store, MembershipAccessInput{
		UserID:       1,
		MembershipID: 10,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "membership not found", reqErr.Err.Error())
}

func TestGetMembershipForUserForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembership{ID: 10, UserID: 2}, nil)

	_, err := GetMembershipForUser(context.Background(), store, MembershipAccessInput{
		UserID:       1,
		MembershipID: 10,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}

func TestGetMembershipForUserSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, UserID: 1}
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Times(1).
		Return(membership, nil)

	result, err := GetMembershipForUser(context.Background(), store, MembershipAccessInput{
		UserID:       membership.UserID,
		MembershipID: membership.ID,
	})

	require.NoError(t, err)
	require.Equal(t, membership.ID, result.ID)
}

func TestListMembershipTransactionsForUserSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, UserID: 1}
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		ListMembershipTransactions(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.MembershipTransaction{{ID: 1, MembershipID: membership.ID}}, nil)

	transactions, err := ListMembershipTransactionsForUser(context.Background(), store, MembershipTransactionsInput{
		UserID:       membership.UserID,
		MembershipID: membership.ID,
		Limit:        10,
		Offset:       0,
	})

	require.NoError(t, err)
	require.Len(t, transactions, 1)
}
