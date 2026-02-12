package logic

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateRechargeRuleInvalidDates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := CreateRechargeRule(context.Background(), store, CreateRechargeRuleInput{
		MerchantID:     10,
		RechargeAmount: 100,
		BonusAmount:    10,
		ValidFrom:      time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC),
		ValidUntil:     time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "valid_until must be after valid_from", reqErr.Err.Error())
}

func TestListMerchantRechargeRulesForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := ListMerchantRechargeRules(context.Background(), store, MerchantRechargeRulesInput{
		MerchantID:       1,
		TargetMerchantID: 2,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized for this merchant", reqErr.Err.Error())
}

func TestUpdateRechargeRuleNotFound(t *testing.T) {
	ruleID := int64(10)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{}, db.ErrRecordNotFound)

	_, err := UpdateRechargeRuleForMerchant(context.Background(), store, UpdateRechargeRuleInput{
		MerchantID:       1,
		TargetMerchantID: 1,
		RuleID:           ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "rule not found", reqErr.Err.Error())
}

func TestUpdateRechargeRuleInvalidDates(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)
	validFrom := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	invalidUntil := validFrom.Add(-time.Minute)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID, ValidFrom: validFrom, ValidUntil: validFrom.Add(time.Hour)}, nil)

	_, err := UpdateRechargeRuleForMerchant(context.Background(), store, UpdateRechargeRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
		ValidUntil:       &invalidUntil,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "valid_until must be after valid_from", reqErr.Err.Error())
}

func TestUpdateRechargeRuleSuccess(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)
	rechargeAmount := int64(100)
	bonusAmount := int64(20)
	isActive := true
	validFrom := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	validUntil := validFrom.Add(2 * time.Hour)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID, ValidFrom: validFrom, ValidUntil: validFrom.Add(time.Hour)}, nil)
	store.EXPECT().
		UpdateRechargeRule(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdateRechargeRuleParams) (db.RechargeRule, error) {
			require.Equal(t, ruleID, arg.ID)
			require.Equal(t, rechargeAmount, arg.RechargeAmount.Int64)
			require.True(t, arg.RechargeAmount.Valid)
			require.Equal(t, bonusAmount, arg.BonusAmount.Int64)
			require.True(t, arg.BonusAmount.Valid)
			require.Equal(t, isActive, arg.IsActive.Bool)
			require.True(t, arg.IsActive.Valid)
			require.Equal(t, validFrom, arg.ValidFrom.Time)
			require.True(t, arg.ValidFrom.Valid)
			require.Equal(t, validUntil, arg.ValidUntil.Time)
			require.True(t, arg.ValidUntil.Valid)
			return db.RechargeRule{ID: ruleID, MerchantID: merchantID}, nil
		})

	_, err := UpdateRechargeRuleForMerchant(context.Background(), store, UpdateRechargeRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
		RechargeAmount:   &rechargeAmount,
		BonusAmount:      &bonusAmount,
		IsActive:         &isActive,
		ValidFrom:        &validFrom,
		ValidUntil:       &validUntil,
	})

	require.NoError(t, err)
}

func TestDeleteRechargeRuleForbidden(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID + 1}, nil)

	err := DeleteRechargeRuleForMerchant(context.Background(), store, DeleteRechargeRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "rule not found", reqErr.Err.Error())
}

func TestDeleteRechargeRuleSuccess(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID}, nil)
	store.EXPECT().
		DeleteRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(nil)

	err := DeleteRechargeRuleForMerchant(context.Background(), store, DeleteRechargeRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
	})

	require.NoError(t, err)
}

func TestUpdateRechargeRuleForbidden(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID}, nil)

	_, err := UpdateRechargeRuleForMerchant(context.Background(), store, UpdateRechargeRuleInput{
		MerchantID:       merchantID + 1,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}

func TestUpdateRechargeRuleWrongMerchant(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID}, nil)

	_, err := UpdateRechargeRuleForMerchant(context.Background(), store, UpdateRechargeRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID + 1,
		RuleID:           ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "rule not found", reqErr.Err.Error())
}

func TestDeleteRechargeRuleForbiddenByContext(t *testing.T) {
	ruleID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRechargeRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.RechargeRule{ID: ruleID, MerchantID: merchantID}, nil)

	err := DeleteRechargeRuleForMerchant(context.Background(), store, DeleteRechargeRuleInput{
		MerchantID:       merchantID + 1,
		TargetMerchantID: merchantID,
		RuleID:           ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}

func TestGetPublicRechargeRulesNotFound(t *testing.T) {
	merchantID := int64(10)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{}, db.ErrRecordNotFound)

	_, err := GetPublicRechargeRules(context.Background(), store, merchantID)

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "merchant not found", reqErr.Err.Error())
}

func TestGetPublicRechargeRulesSuccess(t *testing.T) {
	merchantID := int64(10)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID}, nil)
	store.EXPECT().
		ListActiveRechargeRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.RechargeRule{{ID: 1, MerchantID: merchantID}}, nil)

	_, err := GetPublicRechargeRules(context.Background(), store, merchantID)
	require.NoError(t, err)
}

func TestListActiveRechargeRulesForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := ListActiveRechargeRules(context.Background(), store, MerchantRechargeRulesInput{
		MerchantID:       1,
		TargetMerchantID: 2,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized for this merchant", reqErr.Err.Error())
}
