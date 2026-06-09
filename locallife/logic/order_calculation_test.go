package logic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCalculateOrderPreview_EmptyCart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{}, db.ErrRecordNotFound)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: 1, MerchantID: 2, OrderType: "takeout"},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "cart is empty", reqErr.Err.Error())
}

func TestCalculateOrderPreview_InvalidCustomizations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 10}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 1, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 100, Valid: true}, Quantity: 1, Customizations: []byte("invalid")},
		}, nil)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: 1, MerchantID: 2, OrderType: "takeout"},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "invalid customizations in cart", reqErr.Err.Error())
}

func TestCalculateOrderPreview_WithVoucher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	voucherID := int64(3)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 20}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(20)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, RegionID: 9}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(db.GetUserVoucherRow{
			ID:                voucherID,
			UserID:            userID,
			MerchantID:        merchantID,
			Status:            "unused",
			ExpiresAt:         time.Now().Add(time.Hour),
			MinOrderAmount:    500,
			AllowedOrderTypes: []string{"takeout"},
			Amount:            200,
			Name:              "Promo",
		}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(2).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 1000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(db.GetUserVoucherRow{
			ID:                voucherID,
			UserID:            userID,
			MerchantID:        merchantID,
			Status:            "unused",
			ExpiresAt:         time.Now().Add(time.Hour),
			MinOrderAmount:    500,
			AllowedOrderTypes: []string{"takeout"},
			Amount:            200,
			Name:              "Promo",
		}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeout", UserVoucherID: &voucherID},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, int32(defaultDeliveryDistance), distance)
			return DeliveryFeeComputation{Fee: 500, Discount: 0}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(1000), result.Subtotal)
	require.Equal(t, int64(500), result.DeliveryFee)
	require.Equal(t, int64(200), result.DiscountAmount)
	require.Equal(t, int64(1300), result.TotalAmount)
	foundVoucher := false
	for _, promo := range result.Promotions {
		if promo.Type == "voucher" {
			foundVoucher = true
			break
		}
	}
	require.True(t, foundVoucher)
}

func TestCalculateCartPreview_TakeawayIgnoresDeliveryLocation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeaway,
		}).
		Times(1).
		Return(db.Cart{ID: 30}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(30)).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  2,
		}}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 2000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	feeCalled := false
	result, err := CalculateCartPreview(
		context.Background(),
		store,
		nil,
		db.Merchant{
			ID:        merchantID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.0),
			Longitude: numericFromFloat(120.0),
		},
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			feeCalled = true
			return DeliveryFeeComputation{Fee: 999, Discount: 100}, nil
		},
		CartPreviewInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeaway,
			AddressID:  &addressID,
		},
	)

	require.NoError(t, err)
	require.False(t, feeCalled)
	require.Equal(t, int64(2000), result.Subtotal)
	require.Equal(t, int64(0), result.DeliveryFee)
	require.Equal(t, int64(0), result.DeliveryFeeDiscount)
	require.Equal(t, int32(0), result.DeliveryDistance)
	require.Equal(t, int64(2000), result.Promotion.TotalAmount)
}

func TestCalculateOrderPreview_RejectsVoucherWhenMerchantDiscountCannotStack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	voucherID := int64(3)
	now := time.Now()

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 22}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(22)).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  1,
		}}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, RegionID: 9}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(db.GetUserVoucherRow{
			ID:                voucherID,
			UserID:            userID,
			MerchantID:        merchantID,
			Status:            "unused",
			ExpiresAt:         now.Add(time.Hour),
			MinOrderAmount:    500,
			AllowedOrderTypes: []string{"takeout"},
			Amount:            200,
			Name:              "Promo",
		}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{{
			ID:                  1,
			Name:                "会员日满减",
			MinOrderAmount:      1000,
			DiscountAmount:      300,
			ValidFrom:           now.Add(-time.Hour),
			ValidUntil:          now.Add(time.Hour),
			CanStackWithVoucher: false,
		}}, nil)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeout", UserVoucherID: &voucherID},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, int32(defaultDeliveryDistance), distance)
			return DeliveryFeeComputation{Fee: 500, Discount: 0}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "当前活动不可与所选优惠券叠加", reqErr.Err.Error())
}

func TestCalculateOrderPreview_SuggestsVoucher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 21}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(21)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1500, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, RegionID: 9}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 1500,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{
			{ID: 11, Name: "推荐券", Amount: 300, AllowedOrderTypes: []string{"takeout"}},
		}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: "takeout"},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, int32(defaultDeliveryDistance), distance)
			return DeliveryFeeComputation{Fee: 500, Discount: 0}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(1500), result.Subtotal)
	require.Equal(t, int64(2000), result.TotalAmount)
	require.NotNil(t, result.SuggestedVoucher)
	require.Equal(t, int64(11), result.SuggestedVoucher.ID)
	require.Len(t, result.VoucherTrials, 1)
	require.Equal(t, int64(1700), result.VoucherTrials[0].TrialPayable)
}

func TestCalculateOrderPreview_RejectsForeignAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(99)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 30}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(30)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, RegionID: 9}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{ID: addressID, UserID: userID + 1, RegionID: 9}, nil)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			AddressID:  &addressID,
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "address does not belong to you", reqErr.Err.Error())
}
