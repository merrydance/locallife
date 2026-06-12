package logic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
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

func TestCalculateCartPreview_ClampsRouteDistanceToMinimum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 32}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(32)).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  1,
		}}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
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
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchantAvgPrepareTime(gomock.Any(), gomock.Any()).
		Times(1).
		Return(int64(0), db.ErrRecordNotFound)

	result, err := CalculateCartPreview(
		context.Background(),
		store,
		&fakeMapClient{route: &maps.RouteResult{Distance: 1, Duration: 60}},
		merchant,
		func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, int32(minDeliveryDistanceMeters), distance)
			return DeliveryFeeComputation{Fee: 500}, nil
		},
		CartPreviewInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			AddressID:  &addressID,
		},
	)
	require.NoError(t, err)
	require.Equal(t, int32(minDeliveryDistanceMeters), result.DeliveryDistance)
	require.Equal(t, int64(500), result.DeliveryFee)
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

func TestCalculateOrderPreview_RejectsAddressWithoutLocation(t *testing.T) {
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
		Return(db.Cart{ID: 33}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(33)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{
			ID:        merchantID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.0),
			Longitude: numericFromFloat(120.0),
		}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{ID: addressID, UserID: userID, RegionID: 9}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		AnyTimes().
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

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
			return DeliveryFeeComputation{Fee: 500}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "invalid address or merchant location", reqErr.Err.Error())
}

func TestCalculateOrderPreview_UsesRouteDistanceFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	userLat := 30.02
	userLng := 120.02
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 31}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(31)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
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
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	expectedDistance := int32(float64(algorithm.HaversineDistance(
		algorithm.Location{Latitude: userLat, Longitude: userLng},
		algorithm.Location{Latitude: 30.0, Longitude: 120.0},
	)) * 1.4)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			Latitude:   &userLat,
			Longitude:  &userLng,
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, expectedDistance, distance)
			return DeliveryFeeComputation{Fee: 500}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(500), result.DeliveryFee)
}

func TestCalculateOrderPreview_FallsBackToRouteDistanceEstimateWhenMapFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	userLat := 30.02
	userLng := 120.02
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 31}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(31)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
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
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	expectedDistance := int32(float64(algorithm.HaversineDistance(
		algorithm.Location{Latitude: userLat, Longitude: userLng},
		algorithm.Location{Latitude: 30.0, Longitude: 120.0},
	)) * 1.4)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		&fakeMapClient{err: errors.New("route unavailable")},
		OrderCalculationInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			Latitude:   &userLat,
			Longitude:  &userLng,
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, expectedDistance, distance)
			return DeliveryFeeComputation{Fee: 500}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(500), result.DeliveryFee)
}

func TestCalculateOrderPreview_ClampsRouteDistanceToMinimum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	userLat := 30.02
	userLng := 120.02
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 53}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(53)).
		Times(1).
		Return([]db.ListCartItemsRow{
			{DishID: pgtype.Int8{Int64: 5, Valid: true}, DishName: pgtype.Text{String: "Dish", Valid: true}, DishPrice: pgtype.Int8{Int64: 1000, Valid: true}, Quantity: 1},
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
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
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		&fakeMapClient{route: &maps.RouteResult{Distance: 1, Duration: 60}},
		OrderCalculationInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			Latitude:   &userLat,
			Longitude:  &userLng,
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(_ context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, int32(minDeliveryDistanceMeters), distance)
			return DeliveryFeeComputation{Fee: 500}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(500), result.DeliveryFee)
}

func TestCalculateCartPreview_PropagatesDeliveryFeeCalculatorError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 40}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(40)).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  1,
		}}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)

	_, err := CalculateCartPreview(
		context.Background(),
		store,
		&fakeMapClient{err: errors.New("route unavailable")},
		merchant,
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{}, errors.New("fee unavailable")
		},
		CartPreviewInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			AddressID:  &addressID,
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "fee unavailable")
}

func TestCalculateCartPreview_RejectsDeliverySuspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)
	merchant := db.Merchant{
		ID:        merchantID,
		RegionID:  9,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 41}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(41)).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  1,
		}}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)

	_, err := CalculateCartPreview(
		context.Background(),
		store,
		&fakeMapClient{err: errors.New("route unavailable")},
		merchant,
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{Suspended: true, SuspendReason: "policy"}, nil
		},
		CartPreviewInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			AddressID:  &addressID,
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "policy", reqErr.Err.Error())
}

func TestCalculateOrderPreview_PropagatesDeliveryFeeCalculatorError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 50}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(50)).
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
		Return(db.Merchant{
			ID:        merchantID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.0),
			Longitude: numericFromFloat(120.0),
		}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		&fakeMapClient{err: errors.New("route unavailable")},
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: db.OrderTypeTakeout, AddressID: &addressID},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{}, errors.New("fee unavailable")
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "fee unavailable")
}

func TestCalculateOrderPreview_RejectsDeliverySuspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 51}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(51)).
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
		Return(db.Merchant{
			ID:        merchantID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.0),
			Longitude: numericFromFloat(120.0),
		}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  9,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)

	_, err := CalculateOrderPreview(
		context.Background(),
		store,
		&fakeMapClient{err: errors.New("route unavailable")},
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: db.OrderTypeTakeout, AddressID: &addressID},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(context.Context, int64, int64, int32, int64) (DeliveryFeeComputation, error) {
			return DeliveryFeeComputation{Suspended: true, SuspendReason: "policy"}, nil
		},
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "policy", reqErr.Err.Error())
}

func TestCalculateOrderPreview_UsesMerchantRegionForDeliveryFeeConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	userID := int64(1)
	merchantID := int64(2)
	addressID := int64(3)
	merchantRegionID := int64(9)
	addressRegionID := int64(99)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Cart{ID: 52}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(52)).
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
		Return(db.Merchant{
			ID:        merchantID,
			RegionID:  merchantRegionID,
			Latitude:  numericFromFloat(30.0),
			Longitude: numericFromFloat(120.0),
		}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    userID,
			RegionID:  addressRegionID,
			Latitude:  numericFromFloat(30.01),
			Longitude: numericFromFloat(120.01),
		}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
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
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
			MerchantID: merchantID,
			UserID:     userID,
		}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	result, err := CalculateOrderPreview(
		context.Background(),
		store,
		nil,
		OrderCalculationInput{UserID: userID, MerchantID: merchantID, OrderType: db.OrderTypeTakeout, AddressID: &addressID},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return json.RawMessage{}, 0, nil
		},
		func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
			require.Equal(t, merchantRegionID, regionID)
			return DeliveryFeeComputation{Fee: 500}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(500), result.DeliveryFee)
}
