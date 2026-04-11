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

func TestValidateMembershipPayment(t *testing.T) {
	userID := int64(10)
	merchantID := int64(20)

	baseMembership := db.MerchantMembership{MerchantID: merchantID, UserID: userID, Balance: 1000}

	testCases := []struct {
		name       string
		input      MembershipPaymentInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, membership *db.MerchantMembership, err error)
	}{
		{
			name:  "InvalidOrderType",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "takeout"},
			check: func(t *testing.T, _ *db.MerchantMembership, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "外卖和预定订单暂不支持余额支付", reqErr.Err.Error())
			},
		},
		{
			name:  "MembershipNotFound",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(db.MerchantMembership{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ *db.MerchantMembership, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "会员卡不存在", reqErr.Err.Error())
			},
		},
		{
			name:  "SettingsDisallow",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(baseMembership, nil)
				settings := db.MerchantMembershipSetting{MerchantID: merchantID, BalanceUsableScenes: []string{"takeaway"}}
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(settings, nil)
			},
			check: func(t *testing.T, _ *db.MerchantMembership, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "该商户暂不支持余额支付", reqErr.Err.Error())
			},
		},
		{
			name:  "SettingsDisallowButRulesEnabled",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", RulesEngineEnabled: true},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(baseMembership, nil)
				settings := db.MerchantMembershipSetting{MerchantID: merchantID, BalanceUsableScenes: []string{"takeaway"}}
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(settings, nil)
			},
			check: func(t *testing.T, membership *db.MerchantMembership, err error) {
				require.NoError(t, err)
				require.NotNil(t, membership)
			},
		},
		{
			name:  "SettingsMissing",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(baseMembership, nil)
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantMembershipSetting{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, membership *db.MerchantMembership, err error) {
				require.NoError(t, err)
				require.NotNil(t, membership)
			},
		},
		{
			name:  "BalanceInsufficient",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				membership := baseMembership
				membership.Balance = 0
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(membership, nil)
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantMembershipSetting{MerchantID: merchantID, BalanceUsableScenes: []string{"dine_in"}}, nil)
			},
			check: func(t *testing.T, _ *db.MerchantMembership, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "会员余额不足", reqErr.Err.Error())
			},
		},
		{
			name:  "Success",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(baseMembership, nil)
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantMembershipSetting{MerchantID: merchantID, BalanceUsableScenes: []string{"dine_in"}}, nil)
			},
			check: func(t *testing.T, membership *db.MerchantMembership, err error) {
				require.NoError(t, err)
				require.NotNil(t, membership)
				require.Equal(t, int64(1000), membership.Balance)
			},
		},
		{
			name:  "MembershipStoreError",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(db.MerchantMembership{}, errors.New("boom"))
			},
			check: func(t *testing.T, _ *db.MerchantMembership, err error) {
				require.Error(t, err)
				require.Equal(t, "boom", err.Error())
			},
		},
		{
			name:  "SettingsStoreError",
			input: MembershipPaymentInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchantID,
						UserID:     userID,
					}).
					Times(1).
					Return(baseMembership, nil)
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantMembershipSetting{}, errors.New("boom"))
			},
			check: func(t *testing.T, membership *db.MerchantMembership, err error) {
				require.NoError(t, err)
				require.NotNil(t, membership)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			membership, err := ValidateMembershipPayment(context.Background(), store, tc.input)
			tc.check(t, membership, err)
		})
	}
}
