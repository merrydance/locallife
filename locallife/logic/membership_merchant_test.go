package logic

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListMerchantMembersForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := ListMerchantMembers(context.Background(), store, MerchantMembersInput{
		MerchantID:       1,
		TargetMerchantID: 2,
		Limit:            10,
		Offset:           0,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized for this merchant", reqErr.Err.Error())
}

func TestListMerchantMembersSuccessReturnsTotal(t *testing.T) {
	merchantID := int64(10)
	rows := []db.ListMerchantMembersRow{{ID: 1, MerchantID: merchantID, UserID: 20}}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListMerchantMembers(gomock.Any(), db.ListMerchantMembersParams{MerchantID: merchantID, Limit: 5, Offset: 5}).
		Times(1).
		Return(rows, nil)
	store.EXPECT().
		CountMerchantMembers(gomock.Any(), merchantID).
		Times(1).
		Return(int64(12), nil)

	result, err := ListMerchantMembers(context.Background(), store, MerchantMembersInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		Limit:            5,
		Offset:           5,
	})

	require.NoError(t, err)
	require.Equal(t, int64(12), result.Total)
	require.Len(t, result.Members, 1)
}

func TestGetMerchantMemberDetailNotFound(t *testing.T) {
	merchantID := int64(10)
	userID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	_, err := GetMerchantMemberDetail(context.Background(), store, MerchantMemberDetailInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		UserID:           userID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "membership not found", reqErr.Err.Error())
}

func TestGetMerchantMemberDetailSuccess(t *testing.T) {
	merchantID := int64(10)
	userID := int64(20)
	membership := db.MerchantMembership{ID: 30, MerchantID: merchantID, UserID: userID}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Times(1).
		Return(db.User{ID: userID, FullName: "Test"}, nil)
	store.EXPECT().
		ListMembershipTransactions(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.MembershipTransaction{{ID: 1, MembershipID: membership.ID}}, nil)

	result, err := GetMerchantMemberDetail(context.Background(), store, MerchantMemberDetailInput{
		MerchantID:        merchantID,
		TargetMerchantID:  merchantID,
		UserID:            userID,
		TransactionsLimit: 20,
	})

	require.NoError(t, err)
	require.Equal(t, membership.ID, result.Membership.ID)
	require.Equal(t, userID, result.User.ID)
	require.Len(t, result.Transactions, 1)
}
