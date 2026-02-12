package logic

import (
	"context"
	"errors"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAdjustMemberBalanceZeroAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := AdjustMemberBalance(context.Background(), store, AdjustMemberBalanceInput{
		MerchantID:       1,
		TargetMerchantID: 1,
		UserID:           2,
		Amount:           0,
		Notes:            "test",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "amount cannot be zero", reqErr.Err.Error())
}

func TestAdjustMemberBalanceForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := AdjustMemberBalance(context.Background(), store, AdjustMemberBalanceInput{
		MerchantID:       1,
		TargetMerchantID: 2,
		UserID:           2,
		Amount:           10,
		Notes:            "test",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized for this merchant", reqErr.Err.Error())
}

func TestAdjustMemberBalanceNotFound(t *testing.T) {
	merchantID := int64(10)
	userID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	_, err := AdjustMemberBalance(context.Background(), store, AdjustMemberBalanceInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		UserID:           userID,
		Amount:           10,
		Notes:            "test",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "membership not found", reqErr.Err.Error())
}

func TestAdjustMemberBalanceInsufficient(t *testing.T) {
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
		AdjustMemberBalanceTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.AdjustMemberBalanceTxResult{}, errors.New("余额不足"))

	_, err := AdjustMemberBalance(context.Background(), store, AdjustMemberBalanceInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		UserID:           userID,
		Amount:           -10,
		Notes:            "test",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "余额不足", reqErr.Err.Error())
}

func TestAdjustMemberBalanceSuccess(t *testing.T) {
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
		AdjustMemberBalanceTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.AdjustMemberBalanceTxResult{Membership: membership}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Times(1).
		Return(db.User{ID: userID, FullName: "Test"}, nil)

	result, err := AdjustMemberBalance(context.Background(), store, AdjustMemberBalanceInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		UserID:           userID,
		Amount:           10,
		Notes:            "test",
	})

	require.NoError(t, err)
	require.Equal(t, membership.ID, result.Membership.ID)
	require.Equal(t, userID, result.User.ID)
}
