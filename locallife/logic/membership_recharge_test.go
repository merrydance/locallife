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

func TestPrepareMembershipRechargeNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	_, err := PrepareMembershipRecharge(context.Background(), store, MembershipRechargeInput{
		UserID:         1,
		MembershipID:   10,
		RechargeAmount: 100,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "membership not found", reqErr.Err.Error())
}

func TestPrepareMembershipRechargeForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembership{ID: 10, UserID: 2}, nil)

	_, err := PrepareMembershipRecharge(context.Background(), store, MembershipRechargeInput{
		UserID:         1,
		MembershipID:   10,
		RechargeAmount: 100,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}

func TestPrepareMembershipRechargeRuleFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, UserID: 1, MerchantID: 20}

	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: membership.MerchantID, RechargeAmount: 100}).
		Times(1).
		Return(db.RechargeRule{ID: 30, BonusAmount: 50}, nil)

	result, err := PrepareMembershipRecharge(context.Background(), store, MembershipRechargeInput{
		UserID:         membership.UserID,
		MembershipID:   membership.ID,
		RechargeAmount: 100,
	})

	require.NoError(t, err)
	require.Equal(t, membership.ID, result.Membership.ID)
	require.Equal(t, int64(50), result.BonusAmount)
	require.NotNil(t, result.RechargeRuleID)
	require.Equal(t, int64(30), *result.RechargeRuleID)
}

func TestPrepareMembershipRechargeRuleNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, UserID: 1, MerchantID: 20}

	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: membership.MerchantID, RechargeAmount: 100}).
		Times(1).
		Return(db.RechargeRule{}, db.ErrRecordNotFound)

	result, err := PrepareMembershipRecharge(context.Background(), store, MembershipRechargeInput{
		UserID:         membership.UserID,
		MembershipID:   membership.ID,
		RechargeAmount: 100,
	})

	require.NoError(t, err)
	require.Equal(t, membership.ID, result.Membership.ID)
	require.Equal(t, int64(0), result.BonusAmount)
	require.Nil(t, result.RechargeRuleID)
}

func TestPrepareMembershipRechargeRuleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, UserID: 1, MerchantID: 20}

	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Times(1).
		Return(membership, nil)
	store.EXPECT().
		GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: membership.MerchantID, RechargeAmount: 100}).
		Times(1).
		Return(db.RechargeRule{}, errors.New("match rule error"))

	_, err := PrepareMembershipRecharge(context.Background(), store, MembershipRechargeInput{
		UserID:         membership.UserID,
		MembershipID:   membership.ID,
		RechargeAmount: 100,
	})

	require.Error(t, err)
}
