package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
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

func TestRecordMembershipRechargeForMerchantBadAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	_, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       10,
		TargetMerchantID: 10,
		UserID:           20,
		RechargeAmount:   0,
		IdempotencyKey:   "recharge-1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "recharge amount must be greater than zero", reqErr.Err.Error())
}

func TestRecordMembershipRechargeForMerchantForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	_, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       10,
		TargetMerchantID: 11,
		UserID:           20,
		RechargeAmount:   100,
		IdempotencyKey:   "recharge-1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized for this merchant", reqErr.Err.Error())
}

func TestRecordMembershipRechargeForMerchantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 20, UserID: 30}).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	_, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       20,
		TargetMerchantID: 20,
		UserID:           30,
		RechargeAmount:   100,
		IdempotencyKey:   "recharge-1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "membership not found", reqErr.Err.Error())
}

func TestRecordMembershipRechargeForMerchantSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, MerchantID: 20, UserID: 30, Balance: 1000}
	updatedMembership := membership
	updatedMembership.Balance = 2100
	updatedMembership.TotalRecharged = 1100
	rule := db.RechargeRule{ID: 41, BonusAmount: 100}
	transaction := db.MembershipTransaction{ID: 51, MembershipID: membership.ID, Amount: 1100, PrincipalAmount: 1000, BonusAmount: 100}
	user := db.User{ID: membership.UserID, FullName: "会员用户"}

	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: membership.MerchantID, UserID: membership.UserID}).
		Return(membership, nil)
	store.EXPECT().
		GetUser(gomock.Any(), membership.UserID).
		Return(user, nil)
	store.EXPECT().
		GetMembershipRechargeTransactionByIdempotencyKey(gomock.Any(), gomock.Any()).
		Return(db.MembershipTransaction{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: membership.MerchantID, RechargeAmount: 1000}).
		Return(rule, nil)
	store.EXPECT().
		RechargeTx(gomock.Any(), db.RechargeTxParams{
			MembershipID:   membership.ID,
			RechargeAmount: 1000,
			BonusAmount:    100,
			RechargeRuleID: &rule.ID,
			Notes:          "线下微信收款",
			IdempotencyKey: "recharge-1",
		}).
		Return(db.RechargeTxResult{Membership: updatedMembership, Transaction: transaction}, nil)

	result, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       membership.MerchantID,
		TargetMerchantID: membership.MerchantID,
		UserID:           membership.UserID,
		RechargeAmount:   1000,
		Notes:            "线下微信收款",
		IdempotencyKey:   "recharge-1",
	})

	require.NoError(t, err)
	require.Equal(t, updatedMembership.ID, result.Membership.ID)
	require.Equal(t, transaction.ID, result.Transaction.ID)
	require.Equal(t, int64(100), result.BonusAmount)
	require.NotNil(t, result.RechargeRuleID)
	require.Equal(t, rule.ID, *result.RechargeRuleID)
	require.Equal(t, user.ID, result.User.ID)
}

func TestRecordMembershipRechargeForMerchantDuplicateKeyReturnsExistingTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, MerchantID: 20, UserID: 30, Balance: 2100, TotalRecharged: 1100}
	user := db.User{ID: membership.UserID, FullName: "会员用户"}
	rechargeRuleID := int64(41)
	existingTransaction := db.MembershipTransaction{
		ID:              51,
		MembershipID:    membership.ID,
		Type:            "recharge",
		Amount:          1100,
		PrincipalAmount: 1000,
		BonusAmount:     100,
		RechargeRuleID:  pgtype.Int8{Int64: rechargeRuleID, Valid: true},
		Notes:           pgtype.Text{String: "线下微信收款", Valid: true},
		IdempotencyKey:  pgtype.Text{String: "recharge-1", Valid: true},
	}

	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: membership.MerchantID, UserID: membership.UserID}).
		Return(membership, nil)
	store.EXPECT().
		GetUser(gomock.Any(), membership.UserID).
		Return(user, nil)
	store.EXPECT().
		GetMembershipRechargeTransactionByIdempotencyKey(gomock.Any(), gomock.Any()).
		Return(existingTransaction, nil)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Return(membership, nil)

	result, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       membership.MerchantID,
		TargetMerchantID: membership.MerchantID,
		UserID:           membership.UserID,
		RechargeAmount:   1000,
		Notes:            "线下微信收款",
		IdempotencyKey:   "recharge-1",
	})

	require.NoError(t, err)
	require.Equal(t, existingTransaction.ID, result.Transaction.ID)
	require.Equal(t, int64(100), result.BonusAmount)
	require.NotNil(t, result.RechargeRuleID)
	require.Equal(t, rechargeRuleID, *result.RechargeRuleID)
	require.Equal(t, user.ID, result.User.ID)
}

func TestRecordMembershipRechargeForMerchantUniqueConflictReturnsExistingTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	membership := db.MerchantMembership{ID: 10, MerchantID: 20, UserID: 30, Balance: 2100, TotalRecharged: 1100}
	user := db.User{ID: membership.UserID, FullName: "会员用户"}
	rule := db.RechargeRule{ID: 41, BonusAmount: 100}
	existingTransaction := db.MembershipTransaction{
		ID:              51,
		MembershipID:    membership.ID,
		Type:            "recharge",
		Amount:          1100,
		PrincipalAmount: 1000,
		BonusAmount:     100,
		RechargeRuleID:  pgtype.Int8{Int64: rule.ID, Valid: true},
		Notes:           pgtype.Text{String: "线下微信收款", Valid: true},
		IdempotencyKey:  pgtype.Text{String: "recharge-1", Valid: true},
	}

	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: membership.MerchantID, UserID: membership.UserID}).
		Return(membership, nil)
	store.EXPECT().
		GetUser(gomock.Any(), membership.UserID).
		Return(user, nil)
	store.EXPECT().
		GetMembershipRechargeTransactionByIdempotencyKey(gomock.Any(), db.GetMembershipRechargeTransactionByIdempotencyKeyParams{MembershipID: membership.ID, IdempotencyKey: pgtype.Text{String: "recharge-1", Valid: true}}).
		Return(db.MembershipTransaction{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetMatchingRechargeRule(gomock.Any(), db.GetMatchingRechargeRuleParams{MerchantID: membership.MerchantID, RechargeAmount: 1000}).
		Return(rule, nil)
	store.EXPECT().
		RechargeTx(gomock.Any(), db.RechargeTxParams{
			MembershipID:   membership.ID,
			RechargeAmount: 1000,
			BonusAmount:    100,
			RechargeRuleID: &rule.ID,
			Notes:          "线下微信收款",
			IdempotencyKey: "recharge-1",
		}).
		Return(db.RechargeTxResult{}, db.ErrUniqueViolation)
	store.EXPECT().
		GetMembershipRechargeTransactionByIdempotencyKey(gomock.Any(), db.GetMembershipRechargeTransactionByIdempotencyKeyParams{MembershipID: membership.ID, IdempotencyKey: pgtype.Text{String: "recharge-1", Valid: true}}).
		Return(existingTransaction, nil)
	store.EXPECT().
		GetMerchantMembership(gomock.Any(), membership.ID).
		Return(membership, nil)

	result, err := RecordMembershipRechargeForMerchant(context.Background(), store, MerchantMembershipRechargeInput{
		MerchantID:       membership.MerchantID,
		TargetMerchantID: membership.MerchantID,
		UserID:           membership.UserID,
		RechargeAmount:   1000,
		Notes:            "线下微信收款",
		IdempotencyKey:   "recharge-1",
	})

	require.NoError(t, err)
	require.Equal(t, existingTransaction.ID, result.Transaction.ID)
	require.Equal(t, int64(100), result.BonusAmount)
	require.NotNil(t, result.RechargeRuleID)
	require.Equal(t, rule.ID, *result.RechargeRuleID)
}
