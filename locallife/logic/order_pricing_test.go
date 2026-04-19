package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetBestDiscountAmount(t *testing.T) {
	merchantID := int64(10)

	testCases := []struct {
		name       string
		subtotal   int64
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, amount int64, err error)
	}{
		{
			name:     "NoRules",
			subtotal: 1000,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchantID).
					Times(1).
					Return([]db.DiscountRule{}, nil)
			},
			check: func(t *testing.T, amount int64, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), amount)
			},
		},
		{
			name:     "BestMatch",
			subtotal: 5000,
			buildStubs: func(store *mockdb.MockStore) {
				now := time.Now()
				rules := []db.DiscountRule{
					{MinOrderAmount: 1000, DiscountAmount: 200, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
					{MinOrderAmount: 4000, DiscountAmount: 800, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
					{MinOrderAmount: 6000, DiscountAmount: 1200, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
				}
				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchantID).
					Times(1).
					Return(rules, nil)
			},
			check: func(t *testing.T, amount int64, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(800), amount)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			amount, err := GetBestDiscountAmount(context.Background(), store, merchantID, tc.subtotal)
			tc.check(t, amount, err)
		})
	}
}

func TestResolveMerchantDiscount(t *testing.T) {
	merchantID := int64(10)
	now := time.Now()

	t.Run("SelectsBestMatchingRuleWithinStackingGroup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListActiveDiscountRules(gomock.Any(), merchantID).
			Times(1).
			Return([]db.DiscountRule{
				{ID: 1, Name: "满20减2", MinOrderAmount: 2000, DiscountAmount: 200, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), CanStackWithVoucher: true},
				{ID: 2, Name: "满50减8", MinOrderAmount: 5000, DiscountAmount: 800, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), CanStackWithVoucher: true},
			}, nil)

		result, err := ResolveMerchantDiscount(context.Background(), store, OrderContext{MerchantID: merchantID, Subtotal: 3000})
		require.NoError(t, err)
		require.Equal(t, int64(200), result.DiscountAmount)
		require.True(t, result.AllowWithVoucher)
	})

	t.Run("MarksVoucherAsBlockedWhenRuleDisallowsStacking", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			ListActiveDiscountRules(gomock.Any(), merchantID).
			Times(1).
			Return([]db.DiscountRule{
				{
					ID:                  3,
					Name:                "会员日满减",
					MinOrderAmount:      1000,
					DiscountAmount:      300,
					ValidFrom:           now.Add(-time.Hour),
					ValidUntil:          now.Add(time.Hour),
					CanStackWithVoucher: false,
					StackingGroup:       pgtype.Text{String: "default", Valid: true},
				},
			}, nil)

		result, err := ResolveMerchantDiscount(context.Background(), store, OrderContext{MerchantID: merchantID, Subtotal: 3000})
		require.NoError(t, err)
		require.Equal(t, int64(300), result.DiscountAmount)
		require.False(t, result.AllowWithVoucher)
	})
}

func TestValidateVoucher(t *testing.T) {
	userID := int64(1)
	merchantID := int64(2)
	voucherID := int64(3)

	baseVoucher := db.GetUserVoucherRow{
		ID:                voucherID,
		UserID:            userID,
		Status:            "unused",
		ExpiresAt:         time.Now().Add(2 * time.Hour),
		MerchantID:        merchantID,
		Amount:            500,
		MinOrderAmount:    1000,
		AllowedOrderTypes: []string{"takeaway"},
	}

	testCases := []struct {
		name       string
		input      VoucherValidationInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result VoucherValidationResult, err error)
	}{
		{
			name:  "NilVoucherID",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000},
			check: func(t *testing.T, result VoucherValidationResult, err error) {
				require.NoError(t, err)
				require.Nil(t, result.UserVoucherID)
				require.Equal(t, int64(0), result.VoucherAmount)
			},
		},
		{
			name:  "VoucherNotFound",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(db.GetUserVoucherRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "优惠券不存在", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherNotOwned",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				voucher := baseVoucher
				voucher.UserID = userID + 1
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(voucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "优惠券不属于您", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherUsed",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				voucher := baseVoucher
				voucher.Status = "used"
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(voucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "优惠券已使用或已过期", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherExpired",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				voucher := baseVoucher
				voucher.ExpiresAt = time.Now().Add(-2 * time.Hour)
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(voucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "优惠券已过期", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherMerchantMismatch",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				voucher := baseVoucher
				voucher.MerchantID = merchantID + 1
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(voucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "该优惠券不能在此商户使用", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherMinSpend",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 500, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(baseVoucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "未达到最低消费 10 元", reqErr.Err.Error())
			},
		},
		{
			name:  "VoucherOrderTypeNotAllowed",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(baseVoucher, nil)
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "该代金券不适用于此订单类型", reqErr.Err.Error())
			},
		},
		{
			name:  "StoreError",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(db.GetUserVoucherRow{}, errors.New("boom"))
			},
			check: func(t *testing.T, _ VoucherValidationResult, err error) {
				require.Error(t, err)
				require.Equal(t, "boom", err.Error())
			},
		},
		{
			name:  "Success",
			input: VoucherValidationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000, UserVoucherID: &voucherID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserVoucher(gomock.Any(), voucherID).
					Times(1).
					Return(baseVoucher, nil)
			},
			check: func(t *testing.T, result VoucherValidationResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.UserVoucherID)
				require.Equal(t, voucherID, *result.UserVoucherID)
				require.Equal(t, int64(500), result.VoucherAmount)
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

			result, err := ValidateVoucher(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
