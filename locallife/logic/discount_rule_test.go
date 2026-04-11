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

func TestCreateDiscountRuleValidation(t *testing.T) {
	merchantID := int64(10)
	base := CreateDiscountRuleInput{
		MerchantID:     merchantID,
		Name:           "rule",
		MinOrderAmount: 100,
		DiscountAmount: 10,
		ValidFrom:      time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
		ValidUntil:     time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC),
	}

	cases := []struct {
		name  string
		input CreateDiscountRuleInput
		err   string
		code  int
	}{
		{
			name:  "InvalidDateRange",
			input: func() CreateDiscountRuleInput { v := base; v.ValidUntil = v.ValidFrom.Add(-time.Hour); return v }(),
			err:   "valid_until must be after valid_from",
			code:  400,
		},
		{
			name:  "InvalidDiscountAmount",
			input: func() CreateDiscountRuleInput { v := base; v.DiscountAmount = 100; return v }(),
			err:   "discount_amount must be less than min_order_amount",
			code:  400,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			_, err := CreateDiscountRule(context.Background(), store, tc.input)
			reqErr := assertRequestError(t, err)
			require.Equal(t, tc.code, reqErr.Status)
			require.Equal(t, tc.err, reqErr.Err.Error())
		})
	}
}

func TestCreateDiscountRuleSuccess(t *testing.T) {
	merchantID := int64(10)
	stackingGroup := "group-a"
	now := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		CreateDiscountRule(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateDiscountRuleParams) (db.DiscountRule, error) {
			require.Equal(t, merchantID, arg.MerchantID)
			require.Equal(t, "rule", arg.Name)
			require.True(t, arg.Description.Valid)
			require.Equal(t, "desc", arg.Description.String)
			require.True(t, arg.StackingGroup.Valid)
			require.Equal(t, stackingGroup, arg.StackingGroup.String)
			return db.DiscountRule{ID: 1, MerchantID: merchantID}, nil
		})

	_, err := CreateDiscountRule(context.Background(), store, CreateDiscountRuleInput{
		MerchantID:             merchantID,
		Name:                   "rule",
		Description:            "desc",
		MinOrderAmount:         100,
		DiscountAmount:         20,
		CanStackWithVoucher:    true,
		CanStackWithMembership: false,
		StackingGroup:          &stackingGroup,
		ValidFrom:              now,
		ValidUntil:             now.Add(time.Hour),
	})

	require.NoError(t, err)
}

func TestGetDiscountRuleForMerchant(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)

	cases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, rule db.DiscountRule, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiscountRule(gomock.Any(), ruleID).
					Times(1).
					Return(db.DiscountRule{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.DiscountRule, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "discount rule not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiscountRule(gomock.Any(), ruleID).
					Times(1).
					Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID + 1}, nil)
			},
			check: func(t *testing.T, _ db.DiscountRule, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "insufficient permissions for this merchant", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiscountRule(gomock.Any(), ruleID).
					Times(1).
					Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID}, nil)
			},
			check: func(t *testing.T, rule db.DiscountRule, err error) {
				require.NoError(t, err)
				require.Equal(t, ruleID, rule.ID)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			rule, err := GetDiscountRuleForMerchant(context.Background(), store, DiscountRuleAccessInput{
				MerchantID: merchantID,
				RuleID:     ruleID,
			})
			tc.check(t, rule, err)
		})
	}
}

func TestGetBestDiscountRuleNotFound(t *testing.T) {
	merchantID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetBestDiscountRule(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.DiscountRule{}, db.ErrRecordNotFound)

	_, err := GetBestDiscountRule(context.Background(), store, BestDiscountRuleInput{
		MerchantID:       merchantID,
		TargetMerchantID: merchantID,
		OrderAmount:      100,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "no applicable discount rule found", reqErr.Err.Error())
}

func TestUpdateDiscountRuleForMerchant(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)
	name := "new"
	desc := "desc"
	minAmount := int64(200)
	discountAmount := int64(20)
	stackingGroup := "group"
	validFrom := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	validUntil := validFrom.Add(2 * time.Hour)
	isActive := true
	canStackVoucher := true
	canStackMember := false

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID}, nil)
	store.EXPECT().
		UpdateDiscountRule(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdateDiscountRuleParams) (db.DiscountRule, error) {
			require.Equal(t, ruleID, arg.ID)
			require.Equal(t, name, arg.Name.String)
			require.True(t, arg.Name.Valid)
			require.Equal(t, desc, arg.Description.String)
			require.True(t, arg.Description.Valid)
			require.Equal(t, minAmount, arg.MinOrderAmount.Int64)
			require.Equal(t, discountAmount, arg.DiscountAmount.Int64)
			require.Equal(t, stackingGroup, arg.StackingGroup.String)
			require.True(t, arg.StackingGroup.Valid)
			require.Equal(t, validFrom, arg.ValidFrom.Time)
			require.True(t, arg.ValidFrom.Valid)
			require.Equal(t, validUntil, arg.ValidUntil.Time)
			require.True(t, arg.ValidUntil.Valid)
			require.True(t, arg.IsActive.Valid)
			require.Equal(t, isActive, arg.IsActive.Bool)
			require.True(t, arg.CanStackWithVoucher.Valid)
			require.Equal(t, canStackVoucher, arg.CanStackWithVoucher.Bool)
			require.True(t, arg.CanStackWithMembership.Valid)
			require.Equal(t, canStackMember, arg.CanStackWithMembership.Bool)
			return db.DiscountRule{ID: ruleID, MerchantID: merchantID}, nil
		})

	_, err := UpdateDiscountRuleForMerchant(context.Background(), store, UpdateDiscountRuleInput{
		MerchantID:             merchantID,
		RuleID:                 ruleID,
		Name:                   &name,
		Description:            &desc,
		MinOrderAmount:         &minAmount,
		DiscountAmount:         &discountAmount,
		CanStackWithVoucher:    &canStackVoucher,
		CanStackWithMembership: &canStackMember,
		StackingGroup:          &stackingGroup,
		ValidFrom:              &validFrom,
		ValidUntil:             &validUntil,
		IsActive:               &isActive,
	})

	require.NoError(t, err)
}

func TestDeleteDiscountRuleForMerchant(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID}, nil)
	store.EXPECT().
		DeleteDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(nil)

	err := DeleteDiscountRuleForMerchant(context.Background(), store, DeleteDiscountRuleInput{
		MerchantID: merchantID,
		RuleID:     ruleID,
	})

	require.NoError(t, err)
}

func TestListMerchantDiscountRulesForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := ListMerchantDiscountRules(context.Background(), store, ListMerchantDiscountRulesInput{
		MerchantID:       1,
		TargetMerchantID: 2,
		Limit:            10,
		Offset:           0,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "insufficient permissions for this merchant", reqErr.Err.Error())
}

func TestApplicableDiscountRulesForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	_, err := GetApplicableDiscountRules(context.Background(), store, ApplicableDiscountRulesInput{
		MerchantID:       1,
		TargetMerchantID: 2,
		OrderAmount:      100,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "insufficient permissions for this merchant", reqErr.Err.Error())
}

func TestUpdateDiscountRuleForbidden(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID + 1}, nil)

	_, err := UpdateDiscountRuleForMerchant(context.Background(), store, UpdateDiscountRuleInput{
		MerchantID: merchantID,
		RuleID:     ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}

func TestDeleteDiscountRuleNotFound(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.DiscountRule{}, db.ErrRecordNotFound)

	err := DeleteDiscountRuleForMerchant(context.Background(), store, DeleteDiscountRuleInput{
		MerchantID: merchantID,
		RuleID:     ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "discount rule not found", reqErr.Err.Error())
}

func TestDeleteDiscountRuleForbidden(t *testing.T) {
	merchantID := int64(10)
	ruleID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiscountRule(gomock.Any(), ruleID).
		Times(1).
		Return(db.DiscountRule{ID: ruleID, MerchantID: merchantID + 1}, nil)

	err := DeleteDiscountRuleForMerchant(context.Background(), store, DeleteDiscountRuleInput{
		MerchantID: merchantID,
		RuleID:     ruleID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not authorized", reqErr.Err.Error())
}
